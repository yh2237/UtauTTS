#!/usr/bin/env python3
"""長い連続unitで接続回数を減らした再構成上限を生成する。"""

from __future__ import annotations

import argparse
import functools
import html
import importlib.util
import json
import math
import os
import sys
from pathlib import Path

import torch

sys.path.insert(0, str(Path(__file__).parent))
from neural_renderer_common import (  # noqa: E402
    build_ngram_pool,
    estimate_f0_median,
    load_index,
    ngram_key,
    read_pcm16,
    resample,
    write_pcm16,
)


def load_baseline_module():
    path = Path(__file__).with_name("render-neural-renderer-baseline.py")
    spec = importlib.util.spec_from_file_location("utautts_neural_renderer_long_units", path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"could not load {path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


baseline = load_baseline_module()


@functools.lru_cache(maxsize=64)
def cached_wav(path: str) -> tuple[int, torch.Tensor]:
    return read_pcm16(path)


@functools.lru_cache(maxsize=8192)
def cached_unit_f0(path: str, start: int, end: int) -> float:
    rate, wave = cached_wav(path)
    return estimate_f0_median(wave[start:end], rate)


def choose_f0_oracle(
    target_utterance: dict,
    target_segments: list[dict],
    candidates: list[tuple[dict, list[dict]]],
    target_wave: torch.Tensor,
    target_rate: int,
    shortlist_size: int,
) -> tuple[dict, list[dict]]:
    target_start = baseline.segment_midpoint(target_segments[0])
    target_end = baseline.segment_midpoint(target_segments[-1])
    target_seconds = max(1e-9, (target_end - target_start) / target_rate)
    target_f0 = estimate_f0_median(target_wave[target_start:target_end], target_rate)
    ranked = []
    for utterance, segments in candidates:
        if utterance["id"] == target_utterance["id"]:
            continue
        source_start = baseline.segment_midpoint(segments[0])
        source_end = baseline.segment_midpoint(segments[-1])
        source_seconds = max(1e-9, (source_end - source_start) / utterance["sample_rate"])
        duration_penalty = abs(math.log(source_seconds / target_seconds))
        context_penalty = int(segments[0]["left_phone"] != target_segments[0]["left_phone"]) + int(
            segments[-1]["right_phone"] != target_segments[-1]["right_phone"]
        )
        ranked.append((duration_penalty, context_penalty, utterance["id"], segments[0]["index"], utterance, segments))
    if not ranked:
        raise ValueError(f"no leakage-free candidate for {target_utterance['id']}:{target_segments[0]['index']}")
    shortlist = sorted(ranked)[:shortlist_size]
    if target_f0 <= 0:
        return shortlist[0][-2], shortlist[0][-1]
    scored = []
    for duration_penalty, context_penalty, _, _, utterance, segments in shortlist:
        source_start = baseline.segment_midpoint(segments[0])
        source_end = baseline.segment_midpoint(segments[-1])
        source_seconds = max(1e-9, (source_end - source_start) / utterance["sample_rate"])
        source_f0 = cached_unit_f0(utterance["audio_path"], source_start, source_end)
        rendered_f0 = source_f0 * source_seconds / target_seconds
        pitch_error = abs(1200 * math.log2(rendered_f0 / target_f0)) if rendered_f0 > 0 else math.inf
        scored.append((pitch_error, duration_penalty, context_penalty, utterance["id"], segments[0]["index"], utterance, segments))
    selected = min(scored)
    if not math.isfinite(selected[0]):
        return shortlist[0][-2], shortlist[0][-1]
    return selected[-2], selected[-1]


def render(utterance: dict, pools: dict[int, dict], args: argparse.Namespace) -> tuple[torch.Tensor, torch.Tensor, list[dict]]:
    target_rate, target_wave = cached_wav(utterance["audio_path"])
    output_frames = round(utterance["frames"] * args.sample_rate / target_rate)
    natural = resample(target_wave, output_frames)
    accumulator = torch.zeros(output_frames)
    weights = torch.zeros(output_frames)
    fade = max(1, round(args.fade_ms * args.sample_rate / 1000))
    decisions = []
    position = 0
    target_all = utterance["segments"]
    while position < len(target_all) - 1:
        size = min(args.max_unit_phones, len(target_all) - position)
        while size >= 2 and ngram_key(target_all[position : position + size]) not in pools[size]:
            size -= 1
        if size < 2:
            position += 1
            continue
        target_segments = target_all[position : position + size]
        key = ngram_key(target_segments)
        source_utterance, source_segments = choose_f0_oracle(
            utterance, target_segments, pools[size][key], target_wave, target_rate, args.shortlist
        )
        target_start = round(baseline.segment_midpoint(target_segments[0]) * args.sample_rate / target_rate)
        target_end = round(baseline.segment_midpoint(target_segments[-1]) * args.sample_rate / target_rate)
        render_start = max(0, target_start - fade // 2)
        render_end = min(output_frames, target_end + fade // 2)
        positions = torch.arange(render_start, render_end, dtype=torch.float32)
        source_rate, source_wave = cached_wav(source_utterance["audio_path"])
        source_start = baseline.segment_midpoint(source_segments[0])
        source_end = baseline.segment_midpoint(source_segments[-1])
        values = baseline.source_values(
            source_wave, source_rate, {"start_sample": source_start, "end_sample": source_end},
            positions, target_start, target_end, args.sample_rate,
        )
        unit_weight = torch.ones(render_end - render_start)
        if target_start > 0:
            count = min(unit_weight.numel(), fade)
            unit_weight[:count] = torch.sin(torch.linspace(0, math.pi / 2, count)).square()
        if target_end < output_frames:
            count = min(unit_weight.numel(), fade)
            unit_weight[-count:] = torch.sin(torch.linspace(math.pi / 2, 0, count)).square()
        source_seconds = (source_end - source_start) / source_rate
        target_seconds = (target_end - target_start) / args.sample_rate
        source_f0 = cached_unit_f0(source_utterance["audio_path"], source_start, source_end)
        rendered_f0 = source_f0 * source_seconds / max(1e-9, target_seconds)
        phase_shift = 0
        existing_weights = weights[render_start:render_end]
        overlap = (existing_weights > 1e-6) & (unit_weight > 1e-6)
        indices = torch.nonzero(overlap, as_tuple=False).flatten()
        if rendered_f0 >= 60 and indices.numel() >= 16:
            reference = accumulator[render_start:render_end][indices] / existing_weights[indices]
            radius = min(fade // 2, max(1, round(args.sample_rate / rendered_f0 / 2)))
            phase_shift = baseline.best_phase_shift(reference, values[indices], radius)
            values = baseline.shift_values(values, phase_shift)
        accumulator[render_start:render_end] += values * unit_weight
        weights[render_start:render_end] += unit_weight
        decisions.append({
            "target_index": position,
            "phones": size,
            "key": key,
            "source_utterance_id": source_utterance["id"],
            "source_index": source_segments[0]["index"],
            "phase_shift_samples": phase_shift,
        })
        position += size - 1
    result = torch.where(weights > 1e-6, accumulator / weights.clamp_min(1e-6), torch.zeros_like(weights))
    return natural, result.clamp(-1, 1), decisions


def write_html(path: Path, rows: list[dict]) -> None:
    parts = [
        "<!doctype html><meta charset='utf-8'><title>Long unit comparison</title>",
        "<style>body{font-family:sans-serif;max-width:960px;margin:30px auto}section{margin:28px 0}audio{display:block;width:100%;margin:5px 0 12px}</style>",
        "<h1>diphone対triphone</h1>",
        "<p>F0 oracleと位相整列は共通で、連続unitの長さだけが異なります。</p>",
    ]
    for row in rows:
        parts.append(f"<section><h2>{html.escape(row['id'])}</h2>")
        for key, label in (("natural", "自然音声"), ("comparison", "diphone"), ("baseline", "triphone")):
            parts.append(f"<label>{label}</label><audio controls src='{row[key]}'></audio>")
        parts.append(f"<p>接続数: diphone {row['diphone_joins']} / triphone {row['long_joins']}</p></section>")
    path.write_text("\n".join(parts), encoding="utf-8")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--dataset", default="out/neural-renderer/jsut-index.json")
    parser.add_argument("--split", choices=("validation", "test"), default="validation")
    parser.add_argument("--out", default="out/neural-renderer/triphone-f0-oracle-phase")
    parser.add_argument("--compare-dir", default="out/neural-renderer/diphone-f0-oracle-phase")
    parser.add_argument("--limit", type=int, default=6)
    parser.add_argument("--sample-rate", type=int, default=24_000)
    parser.add_argument("--fade-ms", type=float, default=10.0)
    parser.add_argument("--max-unit-phones", type=int, choices=(2, 3, 4), default=3)
    parser.add_argument("--shortlist", type=int, default=8)
    args = parser.parse_args()

    index = load_index(args.dataset)
    pools = {size: build_ngram_pool(index, size) for size in range(2, args.max_unit_phones + 1)}
    targets = [item for item in index["utterances"] if item["split"] == args.split][: args.limit]
    output = Path(args.out)
    output.mkdir(parents=True, exist_ok=True)
    rows, report_items = [], []
    for number, utterance in enumerate(targets, 1):
        natural, rendered, decisions = render(utterance, pools, args)
        prefix = f"{number:03d}-{utterance['id']}"
        natural_name = prefix + "-01-natural.wav"
        baseline_name = prefix + "-02-baseline.wav"
        write_pcm16(output / natural_name, args.sample_rate, natural)
        write_pcm16(output / baseline_name, args.sample_rate, rendered)
        comparison = Path(args.compare_dir).resolve() / baseline_name
        if not comparison.is_file():
            raise FileNotFoundError(comparison)
        diphone_joins = max(0, len(utterance["segments"]) - 2)
        item = {
            "id": utterance["id"], "units": len(decisions),
            "long_joins": max(0, len(decisions) - 1), "diphone_joins": diphone_joins,
            "decisions": decisions,
        }
        report_items.append(item)
        rows.append({
            **item, "natural": natural_name, "baseline": baseline_name,
            "comparison": Path(os.path.relpath(comparison, output.resolve())).as_posix(),
        })
        print(json.dumps({key: value for key, value in item.items() if key != "decisions"}, ensure_ascii=False))
    write_html(output / "index.html", rows)
    (output / "report.json").write_text(
        json.dumps({"version": 1, "max_unit_phones": args.max_unit_phones, "utterances": report_items}, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )
    print(f"wrote {output / 'index.html'}")


if __name__ == "__main__":
    main()
