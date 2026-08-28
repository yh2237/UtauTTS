#!/usr/bin/env python3
"""選択済みdiphoneをWORLDで連続F0へ再合成する診断比較を生成する。"""

from __future__ import annotations

import argparse
import html
import json
import math
import os
import statistics
import subprocess
import sys
from pathlib import Path

import torch

sys.path.insert(0, str(Path(__file__).parent))
from neural_renderer_common import (  # noqa: E402
    estimate_f0,
    estimate_f0_median,
    load_index,
    read_pcm16,
    resample,
    write_pcm16,
)


def midpoint(segment: dict) -> int:
    return round((segment["start_sample"] + segment["end_sample"]) / 2)


def stabilize(values: list[float]) -> list[float]:
    voiced = sorted(value for value in values if value > 0)
    reference = statistics.median(voiced) if voiced else 220.0
    folded = []
    for value in values:
        if value <= 0:
            folded.append(0.0)
            continue
        while value > reference * 1.6:
            value /= 2
        while value < reference / 1.6:
            value *= 2
        folded.append(value)
    result = folded[:]
    for index, value in enumerate(folded):
        if value <= 0:
            continue
        neighborhood = [item for item in folded[max(0, index - 2) : index + 3] if item > 0]
        if neighborhood:
            result[index] = statistics.median(neighborhood)
    voiced_indices = [index for index, value in enumerate(result) if value > 0]
    if not voiced_indices:
        return [reference] * len(result)
    for index, value in enumerate(result):
        if value > 0:
            continue
        nearest = min(voiced_indices, key=lambda other: abs(other - index))
        result[index] = result[nearest]
    return result


def f0_curve(wave: torch.Tensor, sample_rate: int, frames: int) -> list[float]:
    window = round(0.04 * sample_rate)
    half = window // 2
    values = []
    for frame in range(frames):
        center = round(frame * 0.01 * sample_rate)
        start = max(0, center - half)
        end = min(wave.numel(), start + window)
        start = max(0, end - window)
        values.append(estimate_f0(wave[start:end], sample_rate))
    return stabilize(values)


def envelope(length_ms: float, fade_ms: float) -> list[dict]:
    fade = min(fade_ms, max(1.0, length_ms * 0.25))
    return [
        {"x_ms": 0.0, "y": 0.0},
        {"x_ms": fade, "y": 1.0},
        {"x_ms": fade, "y": 1.0},
        {"x_ms": max(fade, length_ms - fade), "y": 1.0},
        {"x_ms": length_ms, "y": 0.0},
    ]


def make_manifest(
    utterance: dict,
    decisions: list[dict],
    utterances: dict[str, dict],
    target_wave: torch.Tensor,
    target_rate: int,
    args: argparse.Namespace,
    output_path: Path,
) -> dict:
    duration_ms = utterance["frames"] * 1000 / utterance["sample_rate"]
    curve = f0_curve(target_wave, target_rate, max(2, math.ceil(duration_ms / 10) + 2))
    units = []
    half_fade = args.fade_ms / 2
    for decision in decisions:
        if "fallback" in decision:
            continue
        source_utterance = utterances[decision["source_utterance_id"]]
        source_segments = source_utterance["segments"]
        left = source_segments[decision["source_left_index"]]
        right = source_segments[decision["source_right_index"]]
        source_rate = source_utterance["sample_rate"]
        source_start_ms = midpoint(left) * 1000 / source_rate
        source_end_ms = midpoint(right) * 1000 / source_rate
        target_start_ms = decision["target_start_sample"] * 1000 / args.decision_sample_rate
        target_end_ms = decision["target_end_sample"] * 1000 / args.decision_sample_rate
        position_ms = max(0.0, target_start_ms - half_fade)
        leading_trim = target_start_ms - half_fade - position_ms
        required_ms = max(20.0, target_end_ms - target_start_ms + args.fade_ms - leading_trim)
        offset_ms = max(0.0, source_start_ms - half_fade)
        source_length_ms = max(20.0, source_end_ms - source_start_ms + args.fade_ms)
        source_f0 = float(decision.get("source_f0_hz", 0))
        if source_f0 <= 0:
            source_f0 = statistics.median(curve)
        tone = round(69 + 12 * math.log2(source_f0 / 440))
        units.append({
            "source": source_utterance["audio_path"],
            "frq_path": "",
            "position_ms": position_ms,
            "skip_ms": 0.0,
            "length_ms": required_ms,
            "fade_in_ms": args.fade_ms,
            "fade_out_ms": args.fade_ms,
            "offset_ms": offset_ms,
            "required_length_ms": required_ms,
            "consonant_ms": 0.0,
            "cutoff_ms": -source_length_ms,
            "tone": tone,
            "consonant_velocity": 100.0,
            "pitch_start_ms": position_ms,
            "pitch_length_ms": required_ms,
            "volume": 100.0,
            "modulation": 0.0,
            "tempo": 120.0,
            "envelope": envelope(required_ms, args.fade_ms),
        })
    return {
        "engine": "classic-worldline-faithful",
        "worldline_path": str(Path(args.worldline).resolve()),
        "gpu_path": "",
        "output_path": str(output_path.resolve()),
        "sample_rate": args.sample_rate,
        "f0_curve": curve,
        "units": units,
    }


def write_html(path: Path, rows: list[dict]) -> None:
    parts = [
        "<!doctype html><meta charset='utf-8'><title>WORLD continuity comparison</title>",
        "<style>body{font-family:sans-serif;max-width:960px;margin:30px auto}section{margin:28px 0}audio{display:block;width:100%;margin:5px 0 12px}</style>",
        "<h1>raw linear対WORLD連続F0</h1>",
        "<p>候補は同一です。WORLD版だけが正解F0軌跡でpitch同期再合成されています。</p>",
    ]
    for row in rows:
        parts.append(f"<section><h2>{html.escape(row['id'])}</h2>")
        for key, label in (("natural", "自然音声"), ("comparison", "raw linear / F0 oracle / phase-align"), ("worldline", "WORLD / F0 oracle")):
            parts.append(f"<label>{label}</label><audio controls src='{row[key]}'></audio>")
        parts.append("</section>")
    path.write_text("\n".join(parts), encoding="utf-8")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--dataset", default="out/neural-renderer/jsut-index.json")
    parser.add_argument("--decisions-dir", default="out/neural-renderer/diphone-f0-oracle-phase")
    parser.add_argument("--out", default="out/neural-renderer/diphone-worldline-f0-oracle")
    parser.add_argument("--worldline", default="out/neural-renderer/worldline.dll")
    parser.add_argument("--bridge", default="release/UtauTTS/runtime/utautts-worldline-bridge.exe")
    parser.add_argument("--sample-rate", type=int, default=44_100)
    parser.add_argument("--decision-sample-rate", type=int, default=24_000)
    parser.add_argument("--fade-ms", type=float, default=10.0)
    parser.add_argument("--limit", type=int, default=6)
    parser.add_argument("--no-f0-calibration", action="store_true")
    args = parser.parse_args()

    for required in (args.worldline, args.bridge):
        if not Path(required).is_file():
            raise FileNotFoundError(required)
    index = load_index(args.dataset)
    utterances = {item["id"]: item for item in index["utterances"]}
    decision_paths = sorted(Path(args.decisions_dir).glob("[0-9][0-9][0-9]-*.json"))[: args.limit]
    output = Path(args.out)
    manifest_dir = output / "manifests"
    manifest_dir.mkdir(parents=True, exist_ok=True)
    rows = []
    for number, decision_path in enumerate(decision_paths, 1):
        record = json.loads(decision_path.read_text(encoding="utf-8"))
        utterance = utterances[record["target"]]
        target_rate, target_wave = read_pcm16(utterance["audio_path"])
        prefix = f"{number:03d}-{utterance['id']}"
        bridge_output = output / (prefix + "-worldline-raw.wav")
        manifest = make_manifest(
            utterance, record["decisions"], utterances, target_wave, target_rate,
            args, bridge_output,
        )
        manifest_path = manifest_dir / (prefix + ".json")
        manifest_path.write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        completed = subprocess.run(
            [str(Path(args.bridge).resolve()), str(manifest_path.resolve())],
            check=False, capture_output=True, text=True,
        )
        if completed.returncode != 0:
            raise RuntimeError(f"worldline bridge failed for {utterance['id']}: {completed.stderr}")
        worldline_rate, worldline_wave = read_pcm16(bridge_output)
        calibration = 1.0
        if not args.no_f0_calibration:
            natural_f0 = estimate_f0_median(target_wave, target_rate)
            for _ in range(3):
                rendered_f0 = estimate_f0_median(worldline_wave, worldline_rate)
                if natural_f0 <= 0 or rendered_f0 <= 0:
                    break
                correction = max(0.8, min(1.25, natural_f0 / rendered_f0))
                if abs(correction - 1) < 0.01:
                    break
                calibration *= correction
                manifest["f0_curve"] = [value * correction for value in manifest["f0_curve"]]
                manifest_path.write_text(
                    json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
                )
                completed = subprocess.run(
                    [str(Path(args.bridge).resolve()), str(manifest_path.resolve())],
                    check=False, capture_output=True, text=True,
                )
                if completed.returncode != 0:
                    raise RuntimeError(f"calibrated worldline bridge failed for {utterance['id']}: {completed.stderr}")
                worldline_rate, worldline_wave = read_pcm16(bridge_output)
        target_samples = round(utterance["frames"] * args.sample_rate / utterance["sample_rate"])
        if worldline_wave.numel() < target_samples:
            worldline_wave = torch.cat((worldline_wave, torch.zeros(target_samples - worldline_wave.numel())))
        worldline_wave = worldline_wave[:target_samples]
        natural = resample(target_wave, target_samples)
        comparison_name = prefix + "-02-baseline.wav"
        comparison = Path(args.decisions_dir).resolve() / comparison_name
        if not comparison.is_file():
            raise FileNotFoundError(comparison)
        _, comparison_wave = read_pcm16(comparison)
        worldline_rms = float(torch.sqrt(worldline_wave.square().mean()))
        comparison_rms = float(torch.sqrt(comparison_wave.square().mean()))
        gain = max(0.25, min(4.0, comparison_rms / max(1e-9, worldline_rms)))
        worldline_wave = (worldline_wave * gain).clamp(-1, 1)
        natural_name = prefix + "-01-natural.wav"
        worldline_name = prefix + "-03-worldline.wav"
        write_pcm16(output / natural_name, args.sample_rate, natural)
        write_pcm16(output / worldline_name, worldline_rate, worldline_wave)
        rows.append({
            "id": utterance["id"], "natural": natural_name,
            "comparison": Path(os.path.relpath(comparison, output.resolve())).as_posix(),
            "worldline": worldline_name,
        })
        print(json.dumps({
            "id": utterance["id"], "units": len(manifest["units"]),
            "f0_calibration": calibration,
        }, ensure_ascii=False))
    write_html(output / "index.html", rows)
    print(f"wrote {output / 'index.html'}")


if __name__ == "__main__":
    main()
