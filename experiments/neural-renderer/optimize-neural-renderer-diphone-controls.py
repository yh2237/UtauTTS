#!/usr/bin/env python3
"""diphone baselineへoracle制御を適用し、Phase 1の品質上限を調べる。"""

from __future__ import annotations

import argparse
import functools
import html
import importlib.util
import json
import math
import sys
from pathlib import Path

import torch
from torch.nn import functional as F

sys.path.insert(0, str(Path(__file__).parent))
from neural_renderer_common import build_diphone_pool, diphone_key, load_index, read_pcm16, resample, write_pcm16  # noqa: E402


def load_baseline_module():
    path = Path(__file__).with_name("render-neural-renderer-baseline.py")
    spec = importlib.util.spec_from_file_location("utautts_neural_renderer_baseline_oracle", path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"could not load {path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


baseline = load_baseline_module()


@functools.lru_cache(maxsize=64)
def cached_wav(path: str) -> tuple[int, torch.Tensor]:
    return read_pcm16(path)


def reconstruction_loss(rendered: torch.Tensor, target: torch.Tensor) -> torch.Tensor:
    total = rendered.new_zeros(())
    count = 0
    for size in (64, 128, 256, 512):
        if target.numel() <= size // 2:
            continue
        window = torch.hann_window(size, device=rendered.device)
        actual = torch.stft(rendered, size, size // 4, window=window, return_complex=True)
        expected = torch.stft(target, size, size // 4, window=window, return_complex=True)
        total = total + F.l1_loss(torch.log1p(actual.abs()), torch.log1p(expected.abs()))
        count += 1
    spectral = total / max(1, count)
    window_size = max(2, min(target.numel(), 120))
    stride = max(1, window_size // 2)
    actual_energy = F.avg_pool1d(rendered.square().view(1, 1, -1), window_size, stride=stride)
    target_energy = F.avg_pool1d(target.square().view(1, 1, -1), window_size, stride=stride)
    envelope = F.l1_loss(torch.sqrt(actual_energy + 1e-6), torch.sqrt(target_energy + 1e-6))
    return spectral + 0.5 * envelope


def constrained_controls(
    parameters: torch.Tensor,
    source_rate: int,
    source_start: int,
    source_end: int,
    maximum_correction_ms: float,
    maximum_gain_db: float,
) -> tuple[torch.Tensor, torch.Tensor, torch.Tensor, torch.Tensor, dict[str, torch.Tensor]]:
    duration = max(1, source_end - source_start)
    correction_limit = min(maximum_correction_ms * source_rate / 1000, duration * 0.15)
    start_correction = torch.tanh(parameters[0]) * correction_limit
    end_correction = torch.tanh(parameters[1]) * correction_limit
    controlled_start = parameters.new_tensor(float(source_start)) + start_correction
    controlled_end = parameters.new_tensor(float(source_end)) + end_correction
    midpoint_fraction = 0.5 + 0.16 * torch.tanh(parameters[2])
    gain_db = maximum_gain_db * torch.tanh(parameters[3])
    controls = {
        "start_correction_ms": start_correction * 1000 / source_rate,
        "end_correction_ms": end_correction * 1000 / source_rate,
        "midpoint_fraction": midpoint_fraction,
        "gain_db": gain_db,
    }
    return controlled_start, controlled_end, midpoint_fraction, gain_db, controls


def render_unit(
    source: torch.Tensor,
    source_rate: int,
    source_start: int,
    source_end: int,
    positions: torch.Tensor,
    target_start: int,
    target_end: int,
    parameters: torch.Tensor,
    maximum_correction_ms: float,
    maximum_gain_db: float,
) -> tuple[torch.Tensor, dict[str, torch.Tensor]]:
    start, end, midpoint, gain_db, controls = constrained_controls(
        parameters, source_rate, source_start, source_end, maximum_correction_ms, maximum_gain_db
    )
    values = baseline.controlled_source_values(
        source, positions, target_start, target_end, start, end, midpoint, gain_db
    )
    return values.clamp(-1, 1), controls


def unit_weight(samples: int, fade: int, has_left: bool, has_right: bool) -> torch.Tensor:
    weight = torch.ones(samples)
    if has_left:
        count = min(samples, fade)
        weight[:count] = torch.sin(torch.linspace(0, math.pi / 2, count)).square()
    if has_right:
        count = min(samples, fade)
        weight[-count:] = torch.sin(torch.linspace(math.pi / 2, 0, count)).square()
    return weight


def optimize_utterance(utterance: dict, pool: dict, args: argparse.Namespace) -> tuple[torch.Tensor, torch.Tensor, torch.Tensor, list[dict]]:
    target_rate, target_wave = cached_wav(utterance["audio_path"])
    output_frames = round(utterance["frames"] * args.sample_rate / target_rate)
    natural = resample(target_wave, output_frames)
    fixed_accumulator = torch.zeros(output_frames)
    oracle_accumulator = torch.zeros(output_frames)
    weights = torch.zeros(output_frames)
    fade = max(1, round(args.fade_ms * args.sample_rate / 1000))
    decisions = []
    segments = utterance["segments"]
    for position in range(len(segments) - 1):
        target_left, target_right = segments[position], segments[position + 1]
        key = diphone_key(target_left, target_right)
        candidates = pool.get(key, [])
        if not candidates:
            continue
        source_utterance, source_left, source_right = baseline.choose_diphone_candidate(
            utterance, target_left, target_right, candidates
        )
        target_start = round(baseline.segment_midpoint(target_left) * args.sample_rate / target_rate)
        target_end = round(baseline.segment_midpoint(target_right) * args.sample_rate / target_rate)
        if target_end - target_start < 32:
            continue
        render_start = max(0, target_start - fade // 2)
        render_end = min(output_frames, target_end + fade // 2)
        positions = torch.arange(render_start, render_end, dtype=torch.float32)
        core_positions = torch.arange(target_start, target_end, dtype=torch.float32)
        target = natural[target_start:target_end]
        source_rate, source_wave = cached_wav(source_utterance["audio_path"])
        source_start = baseline.segment_midpoint(source_left)
        source_end = baseline.segment_midpoint(source_right)
        parameters = torch.zeros(4, requires_grad=True)
        optimizer = torch.optim.Adam([parameters], lr=args.learning_rate)
        with torch.no_grad():
            fixed_core, _ = render_unit(
                source_wave, source_rate, source_start, source_end, core_positions,
                target_start, target_end, parameters, args.max_correction_ms, args.max_gain_db,
            )
            fixed_loss = float(reconstruction_loss(fixed_core, target))
        best_loss = fixed_loss
        best_parameters = parameters.detach().clone()
        for _ in range(args.steps):
            optimizer.zero_grad(set_to_none=True)
            rendered, _ = render_unit(
                source_wave, source_rate, source_start, source_end, core_positions,
                target_start, target_end, parameters, args.max_correction_ms, args.max_gain_db,
            )
            loss = reconstruction_loss(rendered, target)
            objective = loss + args.control_penalty * parameters.square().mean()
            objective.backward()
            optimizer.step()
            if float(loss.detach()) < best_loss:
                best_loss = float(loss.detach())
                best_parameters = parameters.detach().clone()
        with torch.no_grad():
            fixed, _ = render_unit(
                source_wave, source_rate, source_start, source_end, positions,
                target_start, target_end, torch.zeros(4), args.max_correction_ms, args.max_gain_db,
            )
            oracle, controls = render_unit(
                source_wave, source_rate, source_start, source_end, positions,
                target_start, target_end, best_parameters, args.max_correction_ms, args.max_gain_db,
            )
        weight = unit_weight(
            render_end - render_start, fade, target_start > 0, target_end < output_frames
        )
        fixed_accumulator[render_start:render_end] += fixed * weight
        oracle_accumulator[render_start:render_end] += oracle * weight
        weights[render_start:render_end] += weight
        decisions.append({
            "target_index": position,
            "diphone": key,
            "source_utterance_id": source_utterance["id"],
            "source_left_index": source_left["index"],
            "fixed_loss": fixed_loss,
            "oracle_loss": best_loss,
            "improvement_percent": 100 * (fixed_loss - best_loss) / max(1e-9, fixed_loss),
            "controls": {name: float(value) for name, value in controls.items()},
        })
    denominator = weights.clamp_min(1e-6)
    fixed = torch.where(weights > 1e-6, fixed_accumulator / denominator, torch.zeros_like(weights))
    oracle = torch.where(weights > 1e-6, oracle_accumulator / denominator, torch.zeros_like(weights))
    return natural, fixed.clamp(-1, 1), oracle.clamp(-1, 1), decisions


def write_html(path: Path, rows: list[dict]) -> None:
    parts = [
        "<!doctype html><meta charset='utf-8'><title>Phase 1 diphone oracle</title>",
        "<style>body{font-family:sans-serif;max-width:960px;margin:30px auto}section{margin:28px 0}audio{display:block;width:100%;margin:5px 0 12px}</style>",
        "<h1>Phase 1 diphone oracle</h1>",
        "<p>候補選択は同一です。oracleだけが正解波形を見てcrop、緩やかなtime warp、gainを調整しています。</p>",
    ]
    for row in rows:
        parts.append(f"<section><h2>{html.escape(row['id'])}</h2>")
        for key, label in (("natural", "自然音声"), ("fixed", "diphone + linear"), ("oracle", "oracle制御")):
            parts.append(f"<label>{label}</label><audio controls src='{row[key]}'></audio>")
        parts.append(f"<p>unit平均改善: {row['mean_improvement_percent']:.1f}%</p></section>")
    path.write_text("\n".join(parts), encoding="utf-8")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--dataset", default="out/neural-renderer/jsut-index.json")
    parser.add_argument("--split", choices=("validation", "test"), default="validation")
    parser.add_argument("--out", default="out/neural-renderer/diphone-oracle")
    parser.add_argument("--limit", type=int, default=6)
    parser.add_argument("--sample-rate", type=int, default=24_000)
    parser.add_argument("--fade-ms", type=float, default=10.0)
    parser.add_argument("--steps", type=int, default=100)
    parser.add_argument("--learning-rate", type=float, default=0.04)
    parser.add_argument("--max-correction-ms", type=float, default=20.0)
    parser.add_argument("--max-gain-db", type=float, default=2.0)
    parser.add_argument("--control-penalty", type=float, default=0.003)
    args = parser.parse_args()

    index = load_index(args.dataset)
    pool = build_diphone_pool(index)
    utterances = [item for item in index["utterances"] if item["split"] == args.split][: args.limit]
    if not utterances:
        raise SystemExit(f"no {args.split} utterances")
    output = Path(args.out)
    output.mkdir(parents=True, exist_ok=True)
    rows = []
    report = {"version": 1, "split": args.split, "utterances": []}
    for number, utterance in enumerate(utterances, 1):
        natural, fixed, oracle, decisions = optimize_utterance(utterance, pool, args)
        prefix = f"{number:03d}-{utterance['id']}"
        names = {
            "natural": prefix + "-01-natural.wav",
            "fixed": prefix + "-02-linear.wav",
            "oracle": prefix + "-03-oracle.wav",
        }
        write_pcm16(output / names["natural"], args.sample_rate, natural)
        write_pcm16(output / names["fixed"], args.sample_rate, fixed)
        write_pcm16(output / names["oracle"], args.sample_rate, oracle)
        mean_improvement = sum(item["improvement_percent"] for item in decisions) / max(1, len(decisions))
        item = {"id": utterance["id"], "units": len(decisions), "mean_improvement_percent": mean_improvement, "decisions": decisions}
        report["utterances"].append(item)
        rows.append({**names, **item})
        print(json.dumps({"id": utterance["id"], "units": len(decisions), "mean_improvement_percent": mean_improvement}, ensure_ascii=False))
    write_html(output / "index.html", rows)
    (output / "report.json").write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(f"wrote {output / 'index.html'}")


if __name__ == "__main__":
    main()
