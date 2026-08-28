#!/usr/bin/env python3
"""音響join costを使い、発話全体で連続するdiphone候補列を選ぶ。"""

from __future__ import annotations

import argparse
import functools
import html
import importlib.util
import json
import math
import os
import statistics
import sys
from dataclasses import dataclass
from pathlib import Path

import torch

sys.path.insert(0, str(Path(__file__).parent))
from neural_renderer_common import (  # noqa: E402
    build_diphone_pool,
    diphone_key,
    estimate_f0_median,
    fold_f0,
    load_index,
    read_pcm16,
    write_pcm16,
)


def load_baseline_module():
    path = Path(__file__).with_name("render-neural-renderer-baseline.py")
    spec = importlib.util.spec_from_file_location("utautts_neural_renderer_viterbi", path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"could not load {path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


baseline = load_baseline_module()


@dataclass
class State:
    utterance: dict
    left: dict
    right: dict
    node_cost: float
    rendered_f0: float
    target_f0: float
    head_spectrum: torch.Tensor
    tail_spectrum: torch.Tensor
    head_rms_db: float
    tail_rms_db: float


@functools.lru_cache(maxsize=128)
def cached_wav(path: str) -> tuple[int, torch.Tensor]:
    return read_pcm16(path)


@functools.lru_cache(maxsize=16384)
def boundary_feature(path: str, center: int, side: str) -> tuple[torch.Tensor, float]:
    sample_rate, wave = cached_wav(path)
    length = max(64, round(0.015 * sample_rate))
    if side == "head":
        values = wave[center : min(wave.numel(), center + length)]
    else:
        values = wave[max(0, center - length) : center]
    if values.numel() < 16:
        return torch.zeros(129), -140.0
    fixed = torch.nn.functional.interpolate(
        values.view(1, 1, -1), size=512, mode="linear", align_corners=False
    ).view(-1)
    windowed = (fixed - fixed.mean()) * torch.hann_window(512)
    spectrum = torch.log(torch.fft.rfft(windowed).abs() + 1e-4)[:129]
    spectrum = (spectrum - spectrum.mean()) / spectrum.std().clamp_min(1e-4)
    rms_db = 20 * math.log10(max(1e-7, float(torch.sqrt(values.square().mean()))))
    return spectrum, rms_db


def target_f0(utterance: dict, left: dict, right: dict) -> float:
    sample_rate, wave = cached_wav(utterance["audio_path"])
    return fold_f0(
        estimate_f0_median(
            wave[baseline.segment_midpoint(left) : baseline.segment_midpoint(right)], sample_rate
        ),
        baseline.cached_utterance_f0(utterance["audio_path"]),
    )


def states_for(
    utterance: dict,
    target_left: dict,
    target_right: dict,
    candidates: list[tuple[dict, dict, dict]],
    preselect: int,
    state_count: int,
    strict_context: bool,
    wanted_f0_override: float = 0.0,
    adaptive_context: bool = False,
    context_penalty: float = 0.12,
    adaptive_context_levels: int = 1,
) -> list[State]:
    target_rate = utterance["sample_rate"]
    target_seconds = (
        baseline.segment_midpoint(target_right) - baseline.segment_midpoint(target_left)
    ) / target_rate
    wanted_f0 = wanted_f0_override or target_f0(utterance, target_left, target_right)
    ranked = []
    for source_utterance, left, right in candidates:
        if source_utterance["id"] == utterance["id"]:
            continue
        source_seconds = (
            baseline.segment_midpoint(right) - baseline.segment_midpoint(left)
        ) / source_utterance["sample_rate"]
        duration_cost = abs(math.log(max(1e-9, source_seconds / target_seconds)))
        context_cost = int(left["left_phone"] != target_left["left_phone"]) + int(
            right["right_phone"] != target_right["right_phone"]
        )
        ranked.append((duration_cost, context_cost, source_utterance["id"], left["index"], source_utterance, left, right))
    minimum_context = min((item[1] for item in ranked), default=0)
    if strict_context and ranked:
        ranked_all = ranked
        strict_ranked = [item for item in ranked if item[1] == minimum_context]
        if adaptive_context:
            ranked = sorted(strict_ranked)[:preselect]
            for level in range(1, adaptive_context_levels + 1):
                relaxed_ranked = [item for item in ranked_all if item[1] == minimum_context + level]
                ranked += sorted(relaxed_ranked)[:preselect]
        else:
            ranked = strict_ranked
    states = []
    for duration_cost, context_cost, _, _, source_utterance, left, right in sorted(ranked)[:preselect]:
        source_start = baseline.segment_midpoint(left)
        source_end = baseline.segment_midpoint(right)
        source_seconds = (source_end - source_start) / source_utterance["sample_rate"]
        source_f0 = baseline.cached_diphone_f0(source_utterance["audio_path"], source_start, source_end)
        rendered_f0 = fold_f0(
            source_f0 * source_seconds / max(1e-9, target_seconds), wanted_f0
        )
        if wanted_f0 > 0 and rendered_f0 > 0:
            pitch_cost = abs(1200 * math.log2(rendered_f0 / wanted_f0)) / 100
        elif wanted_f0 <= 0 and rendered_f0 <= 0:
            pitch_cost = 0.0
        else:
            pitch_cost = 8.0
        head_spectrum, head_rms = boundary_feature(source_utterance["audio_path"], source_start, "head")
        tail_spectrum, tail_rms = boundary_feature(source_utterance["audio_path"], source_end, "tail")
        states.append(State(
            source_utterance, left, right,
            pitch_cost + duration_cost + context_penalty * max(0, context_cost - minimum_context),
            rendered_f0, wanted_f0, head_spectrum, tail_spectrum, head_rms, tail_rms,
        ))
    states.sort(key=lambda state: (state.node_cost, state.utterance["id"], state.left["index"]))
    return states[:state_count]


def join_cost(previous: State, current: State, args: argparse.Namespace) -> float:
    spectral = float(torch.mean(torch.abs(previous.tail_spectrum - current.head_spectrum)))
    energy = abs(previous.tail_rms_db - current.head_rms_db) / 12
    pitch_transition = 0.0
    if previous.rendered_f0 > 0 and current.rendered_f0 > 0:
        actual = 1200 * math.log2(current.rendered_f0 / previous.rendered_f0)
        wanted = 0.0
        if previous.target_f0 > 0 and current.target_f0 > 0:
            wanted = 1200 * math.log2(current.target_f0 / previous.target_f0)
        pitch_transition = abs(actual - wanted) / 200
    continuity = (
        previous.utterance["id"] == current.utterance["id"]
        and previous.left["index"] + 1 == current.left["index"]
    )
    jump_excess = 0.0
    if previous.rendered_f0 > 0 and current.rendered_f0 > 0 and args.max_pitch_jump_cents > 0:
        actual_jump = abs(1200 * math.log2(current.rendered_f0 / previous.rendered_f0))
        jump_excess = max(0.0, actual_jump - args.max_pitch_jump_cents) / 100
    return (
        args.spectrum_weight * spectral
        + args.energy_weight * energy
        + args.pitch_transition_weight * pitch_transition
        + args.jump_excess_weight * jump_excess
        - (args.continuity_bonus if continuity else 0.0)
    )


def smooth_f0_targets(values: list[float], window: int) -> list[float]:
    if window <= 1:
        return values
    radius = window // 2
    result = []
    for index, value in enumerate(values):
        neighbors = [
            item for item in values[max(0, index - radius) : index + radius + 1]
            if item > 0
        ]
        result.append(statistics.median(neighbors) if neighbors else value)
    return result


def select_path(lattice: list[list[State]], args: argparse.Namespace) -> list[State]:
    costs = [state.node_cost for state in lattice[0]]
    backpointers: list[list[int]] = []
    for position in range(1, len(lattice)):
        next_costs, pointers = [], []
        for current in lattice[position]:
            options = [
                costs[index] + current.node_cost + join_cost(previous, current, args)
                for index, previous in enumerate(lattice[position - 1])
            ]
            pointer = min(range(len(options)), key=options.__getitem__)
            next_costs.append(options[pointer])
            pointers.append(pointer)
        costs = next_costs
        backpointers.append(pointers)
    index = min(range(len(costs)), key=costs.__getitem__)
    indices = [index]
    for pointers in reversed(backpointers):
        index = pointers[index]
        indices.append(index)
    indices.reverse()
    return [states[index] for states, index in zip(lattice, indices)]


def mean_path_join(path: list[State], args: argparse.Namespace) -> float:
    if len(path) < 2:
        return 0.0
    return sum(join_cost(path[index - 1], path[index], args) for index in range(1, len(path))) / (len(path) - 1)


def write_html(path: Path, rows: list[dict]) -> None:
    parts = [
        "<!doctype html><meta charset='utf-8'><title>Viterbi unit selection</title>",
        "<style>body{font-family:sans-serif;max-width:960px;margin:30px auto}section{margin:28px 0}audio{display:block;width:100%;margin:5px 0 12px}</style>",
        "<h1>Viterbi系列選択比較</h1>",
        "<p>波形処理はraw linear＋位相整列で共通です。</p>",
    ]
    for row in rows:
        parts.append(f"<section><h2>{html.escape(row['id'])}</h2>")
        for key, label in (("natural", "自然音声"), ("local", row["comparison_label"]), ("viterbi", row["viterbi_label"])):
            parts.append(f"<label>{label}</label><audio controls src='{row[key]}'></audio>")
        parts.append(f"<p>join cost: local {row['local_join']:.3f} / Viterbi {row['viterbi_join']:.3f}</p></section>")
    path.write_text("\n".join(parts), encoding="utf-8")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--dataset", default="out/neural-renderer/jsut-index.json")
    parser.add_argument("--split", choices=("validation", "test"), default="validation")
    parser.add_argument("--out", default="out/neural-renderer/diphone-viterbi")
    parser.add_argument("--compare-dir", default="out/neural-renderer/diphone-f0-oracle-phase")
    parser.add_argument("--compare-suffix", default="-02-baseline.wav")
    parser.add_argument("--compare-label", default="local F0 oracle / phase-align")
    parser.add_argument("--limit", type=int, default=6)
    parser.add_argument("--sample-rate", type=int, default=24_000)
    parser.add_argument("--fade-ms", type=float, default=10.0)
    parser.add_argument("--preselect", type=int, default=24)
    parser.add_argument("--states", type=int, default=8)
    parser.add_argument("--spectrum-weight", type=float, default=1.2)
    parser.add_argument("--energy-weight", type=float, default=0.5)
    parser.add_argument("--pitch-transition-weight", type=float, default=0.8)
    parser.add_argument("--continuity-bonus", type=float, default=0.6)
    parser.add_argument("--strict-context", action="store_true")
    parser.add_argument("--adaptive-context", action="store_true")
    parser.add_argument("--adaptive-context-levels", type=int, default=1)
    parser.add_argument("--context-penalty", type=float, default=0.12)
    parser.add_argument("--smooth-target-window", type=int, default=1)
    parser.add_argument("--max-pitch-jump-cents", type=float, default=0.0)
    parser.add_argument("--jump-excess-weight", type=float, default=0.0)
    args = parser.parse_args()

    index = load_index(args.dataset)
    pool = build_diphone_pool(index)
    targets = [item for item in index["utterances"] if item["split"] == args.split][: args.limit]
    output = Path(args.out)
    output.mkdir(parents=True, exist_ok=True)
    rows, report_items = [], []
    for number, utterance in enumerate(targets, 1):
        lattice = []
        raw_targets = [
            target_f0(utterance, *utterance["segments"][position : position + 2])
            for position in range(len(utterance["segments"]) - 1)
        ]
        wanted_targets = smooth_f0_targets(raw_targets, args.smooth_target_window)
        for position in range(len(utterance["segments"]) - 1):
            left, right = utterance["segments"][position : position + 2]
            states = states_for(
                utterance, left, right, pool[diphone_key(left, right)],
                args.preselect, args.states, args.strict_context, wanted_targets[position],
                args.adaptive_context, args.context_penalty, args.adaptive_context_levels,
            )
            if not states:
                raise RuntimeError(f"empty lattice at {utterance['id']}:{position}")
            lattice.append(states)
        selected = select_path(lattice, args)
        local = [states[0] for states in lattice]
        overrides = {position: (state.utterance, state.left, state.right) for position, state in enumerate(selected)}
        natural, rendered, decisions = baseline.render_diphone_utterance(
            utterance, pool, args.sample_rate, args.fade_ms, "linear",
            "duration-first", True, overrides,
        )
        prefix = f"{number:03d}-{utterance['id']}"
        natural_name = prefix + "-01-natural.wav"
        viterbi_name = prefix + "-03-viterbi.wav"
        write_pcm16(output / natural_name, args.sample_rate, natural)
        write_pcm16(output / viterbi_name, args.sample_rate, rendered)
        local_path = Path(args.compare_dir).resolve() / (prefix + args.compare_suffix)
        if not local_path.is_file():
            raise FileNotFoundError(local_path)
        item = {
            "id": utterance["id"], "local_join": mean_path_join(local, args),
            "viterbi_join": mean_path_join(selected, args),
            "continuous_edges": sum(
                selected[i - 1].utterance["id"] == selected[i].utterance["id"]
                and selected[i - 1].left["index"] + 1 == selected[i].left["index"]
                for i in range(1, len(selected))
            ),
            "smooth_target_window": args.smooth_target_window,
            "adaptive_context": args.adaptive_context,
            "decisions": decisions,
        }
        report_items.append(item)
        rows.append({
            **item, "natural": natural_name, "viterbi": viterbi_name,
            "local": Path(os.path.relpath(local_path, output.resolve())).as_posix(),
            "comparison_label": args.compare_label,
            "viterbi_label": (
                "Viterbi / adaptive context" if args.adaptive_context
                else "Viterbi / strict context" if args.strict_context
                else "Viterbi acoustic path"
            ),
        })
        print(json.dumps({key: value for key, value in item.items() if key != "decisions"}, ensure_ascii=False))
    write_html(output / "index.html", rows)
    (output / "report.json").write_text(
        json.dumps({"version": 1, "utterances": report_items}, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )
    print(f"wrote {output / 'index.html'}")


if __name__ == "__main__":
    main()
