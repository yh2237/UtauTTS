#!/usr/bin/env python3
"""raw consonantを維持し、母音だけTD-PSOLAで連続F0へ合わせる。"""

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
    estimate_f0_median,
    load_index,
    read_pcm16,
    resample,
    td_psola,
    write_pcm16,
)


VOWELS = {"a", "i", "u", "e", "o"}


def load_baseline_module():
    path = Path(__file__).with_name("render-neural-renderer-baseline.py")
    spec = importlib.util.spec_from_file_location("utautts_neural_renderer_psola", path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"could not load {path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


baseline = load_baseline_module()


@functools.lru_cache(maxsize=64)
def cached_wav(path: str) -> tuple[int, torch.Tensor]:
    return read_pcm16(path)


def replace_region(values: torch.Tensor, start: int, replacement: torch.Tensor, fade: int) -> None:
    end = min(values.numel(), start + replacement.numel())
    start = max(0, start)
    if end <= start:
        return
    replacement = replacement[: end - start]
    weight = torch.ones(replacement.numel())
    count = min(fade, replacement.numel() // 2)
    if count > 0:
        weight[:count] = torch.linspace(0, 1, count)
        weight[-count:] = torch.linspace(1, 0, count)
    values[start:end] = values[start:end] * (1 - weight) + replacement * weight


def render(utterance: dict, decisions: list[dict], utterances: dict[str, dict], args: argparse.Namespace):
    target_rate, target_wave = cached_wav(utterance["audio_path"])
    output_frames = round(utterance["frames"] * args.sample_rate / target_rate)
    natural = resample(target_wave, output_frames)
    accumulator = torch.zeros(output_frames)
    weights = torch.zeros(output_frames)
    fade = max(1, round(args.fade_ms * args.sample_rate / 1000))
    region_fade = max(1, round(args.region_fade_ms * args.sample_rate / 1000))
    output_decisions = []
    target_segments = utterance["segments"]
    for decision in decisions:
        if "fallback" in decision:
            continue
        position = decision["target_index"]
        target_left, target_right = target_segments[position], target_segments[position + 1]
        source_utterance = utterances[decision["source_utterance_id"]]
        source_segments = source_utterance["segments"]
        source_left = source_segments[decision["source_left_index"]]
        source_right = source_segments[decision["source_right_index"]]
        source_rate, source_wave = cached_wav(source_utterance["audio_path"])
        target_start = round(baseline.segment_midpoint(target_left) * args.sample_rate / target_rate)
        target_end = round(baseline.segment_midpoint(target_right) * args.sample_rate / target_rate)
        render_start = max(0, target_start - fade // 2)
        render_end = min(output_frames, target_end + fade // 2)
        positions = torch.arange(render_start, render_end, dtype=torch.float32)
        source_start = baseline.segment_midpoint(source_left)
        source_end = baseline.segment_midpoint(source_right)
        values = baseline.source_values(
            source_wave, source_rate, {"start_sample": source_start, "end_sample": source_end},
            positions, target_start, target_end, args.sample_rate,
        )
        psola_regions = 0
        for target_segment, source_segment, side in (
            (target_left, source_left, "left"),
            (target_right, source_right, "right"),
        ):
            if target_segment["phone"] not in VOWELS:
                continue
            if side == "left":
                source_region_start = baseline.segment_midpoint(source_segment)
                source_region_end = source_segment["end_sample"]
                target_region_start = baseline.segment_midpoint(target_segment)
                target_region_end = target_segment["end_sample"]
            else:
                source_region_start = source_segment["start_sample"]
                source_region_end = baseline.segment_midpoint(source_segment)
                target_region_start = target_segment["start_sample"]
                target_region_end = baseline.segment_midpoint(target_segment)
            source_region = source_wave[source_region_start:source_region_end]
            source_samples = max(2, round(source_region.numel() * args.sample_rate / source_rate))
            source_region = resample(source_region, source_samples)
            target_region_start_out = round(target_region_start * args.sample_rate / target_rate)
            target_region_end_out = round(target_region_end * args.sample_rate / target_rate)
            target_samples = max(2, target_region_end_out - target_region_start_out)
            target_region = target_wave[target_region_start:target_region_end]
            source_f0 = estimate_f0_median(source_region, args.sample_rate)
            target_f0 = estimate_f0_median(target_region, target_rate)
            if source_f0 <= 0 or target_f0 <= 0:
                continue
            rendered_region = td_psola(
                source_region, target_samples, args.sample_rate, source_f0, target_f0
            )
            replace_region(values, target_region_start_out - render_start, rendered_region, region_fade)
            psola_regions += 1
        unit_weight = torch.ones(render_end - render_start)
        if target_start > 0:
            count = min(unit_weight.numel(), fade)
            unit_weight[:count] = torch.sin(torch.linspace(0, math.pi / 2, count)).square()
        if target_end < output_frames:
            count = min(unit_weight.numel(), fade)
            unit_weight[-count:] = torch.sin(torch.linspace(math.pi / 2, 0, count)).square()
        rendered_f0 = float(decision.get("target_f0_hz", 0))
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
        output_decisions.append({
            "target_index": position, "diphone": decision["diphone"],
            "psola_regions": psola_regions, "phase_shift_samples": phase_shift,
        })
    rendered = torch.where(weights > 1e-6, accumulator / weights.clamp_min(1e-6), torch.zeros_like(weights))
    return natural, rendered.clamp(-1, 1), output_decisions


def write_html(path: Path, rows: list[dict]) -> None:
    parts = [
        "<!doctype html><meta charset='utf-8'><title>TD-PSOLA comparison</title>",
        "<style>body{font-family:sans-serif;max-width:960px;margin:30px auto}section{margin:28px 0}audio{display:block;width:100%;margin:5px 0 12px}</style>",
        "<h1>raw linear対TD-PSOLA</h1>",
        "<p>候補と子音波形は同じです。PSOLA版だけが母音周期を正解F0へ並べ直しています。</p>",
    ]
    for row in rows:
        parts.append(f"<section><h2>{html.escape(row['id'])}</h2>")
        for key, label in (("natural", "自然音声"), ("raw", "raw linear / F0 oracle / phase-align"), ("psola", "TD-PSOLA vowels / raw consonants")):
            parts.append(f"<label>{label}</label><audio controls src='{row[key]}'></audio>")
        parts.append("</section>")
    path.write_text("\n".join(parts), encoding="utf-8")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--dataset", default="out/neural-renderer/jsut-index.json")
    parser.add_argument("--decisions-dir", default="out/neural-renderer/diphone-f0-oracle-phase")
    parser.add_argument("--out", default="out/neural-renderer/diphone-psola-f0-oracle")
    parser.add_argument("--sample-rate", type=int, default=24_000)
    parser.add_argument("--fade-ms", type=float, default=10.0)
    parser.add_argument("--region-fade-ms", type=float, default=2.0)
    parser.add_argument("--limit", type=int, default=6)
    args = parser.parse_args()

    index = load_index(args.dataset)
    utterances = {item["id"]: item for item in index["utterances"]}
    paths = sorted(Path(args.decisions_dir).glob("[0-9][0-9][0-9]-*.json"))[: args.limit]
    output = Path(args.out)
    output.mkdir(parents=True, exist_ok=True)
    rows = []
    for number, path in enumerate(paths, 1):
        record = json.loads(path.read_text(encoding="utf-8"))
        utterance = utterances[record["target"]]
        natural, rendered, decisions = render(utterance, record["decisions"], utterances, args)
        prefix = f"{number:03d}-{utterance['id']}"
        natural_name = prefix + "-01-natural.wav"
        psola_name = prefix + "-03-psola.wav"
        write_pcm16(output / natural_name, args.sample_rate, natural)
        write_pcm16(output / psola_name, args.sample_rate, rendered)
        raw_path = Path(args.decisions_dir).resolve() / (prefix + "-02-baseline.wav")
        if not raw_path.is_file():
            raise FileNotFoundError(raw_path)
        rows.append({
            "id": utterance["id"], "natural": natural_name, "psola": psola_name,
            "raw": Path(os.path.relpath(raw_path, output.resolve())).as_posix(),
        })
        (output / (prefix + ".json")).write_text(
            json.dumps({"version": 1, "target": utterance["id"], "decisions": decisions}, ensure_ascii=False, indent=2) + "\n",
            encoding="utf-8",
        )
        print(json.dumps({"id": utterance["id"], "psola_regions": sum(item["psola_regions"] for item in decisions)}, ensure_ascii=False))
    write_html(output / "index.html", rows)
    print(f"wrote {output / 'index.html'}")


if __name__ == "__main__":
    main()
