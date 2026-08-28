#!/usr/bin/env python3
"""別発話のphone候補だけで固定制御baselineを生成する。"""

from __future__ import annotations

import argparse
import functools
import html
import json
import math
import os
import sys
from pathlib import Path

import torch

sys.path.insert(0, str(Path(__file__).parent))
from neural_renderer_common import (  # noqa: E402
    SILENCE_PHONES,
    build_candidate_pool,
    build_diphone_pool,
    diphone_key,
    estimate_f0_median,
    fold_f0,
    load_index,
    read_pcm16,
    resample,
    write_pcm16,
)


@functools.lru_cache(maxsize=64)
def cached_wav(path: str) -> tuple[int, torch.Tensor]:
    return read_pcm16(path)


def choose_candidate(target_utterance: dict, target: dict, candidates: list[tuple[dict, dict]]) -> tuple[dict, dict]:
    target_duration = target["end_sample"] - target["start_sample"]

    def rank(item: tuple[dict, dict]) -> tuple[float, str, int]:
        utterance, segment = item
        if utterance["id"] == target_utterance["id"]:
            return math.inf, utterance["id"], segment["index"]
        context_penalty = 0
        if segment["left_phone"] != target["left_phone"]:
            context_penalty += 2
        if segment["right_phone"] != target["right_phone"]:
            context_penalty += 2
        source_duration = segment["end_sample"] - segment["start_sample"]
        duration_penalty = abs(math.log(max(1, source_duration) / max(1, target_duration)))
        return context_penalty + duration_penalty, utterance["id"], segment["index"]

    selected = min(candidates, key=rank)
    if selected[0]["id"] == target_utterance["id"]:
        raise ValueError(f"candidate leakage for {target_utterance['id']}:{target['index']}")
    return selected


def source_values(
    source: torch.Tensor,
    source_rate: int,
    source_segment: dict,
    output_positions: torch.Tensor,
    target_start: int,
    target_end: int,
    output_rate: int,
) -> torch.Tensor:
    target_duration = max(1, target_end - target_start)
    progress = (output_positions - target_start) / target_duration
    source_start = float(source_segment["start_sample"])
    source_end = float(source_segment["end_sample"])
    positions = source_start + progress * (source_end - source_start)
    positions = positions.clamp(0, source.numel() - 1)
    left = positions.floor().long()
    right = (left + 1).clamp_max(source.numel() - 1)
    fraction = positions - left
    return source[left] * (1 - fraction) + source[right] * fraction


def controlled_source_values(
    source: torch.Tensor,
    output_positions: torch.Tensor,
    target_start: int,
    target_end: int,
    source_start: float | torch.Tensor,
    source_end: float | torch.Tensor,
    midpoint_fraction: float | torch.Tensor,
    gain_db: float | torch.Tensor,
) -> torch.Tensor:
    """linearを中心に、source側の中点だけを緩やかに動かして読み出す。"""
    target_duration = max(1, target_end - target_start)
    progress = (output_positions - target_start) / target_duration
    source_midpoint = source_start + (source_end - source_start) * midpoint_fraction
    left_positions = source_start + progress * 2 * (source_midpoint - source_start)
    right_positions = source_midpoint + (progress - 0.5) * 2 * (source_end - source_midpoint)
    positions = torch.where(progress <= 0.5, left_positions, right_positions)
    positions = positions.clamp(0, source.numel() - 1)
    left = positions.floor().long()
    right = (left + 1).clamp_max(source.numel() - 1)
    fraction = positions - left
    values = source[left] * (1 - fraction) + source[right] * fraction
    gain = torch.pow(values.new_tensor(10.0), torch.as_tensor(gain_db, device=values.device) / 20)
    return values * gain


def shift_values(values: torch.Tensor, shift: int) -> torch.Tensor:
    """波形内容を小数処理なしのsample単位でずらし、端は境界値で補う。"""
    positions = (torch.arange(values.numel(), device=values.device) + shift).clamp(0, values.numel() - 1)
    return values[positions]


def best_phase_shift(reference: torch.Tensor, candidate: torch.Tensor, radius: int) -> int:
    """正規化相関が最大になる±radius内の位相ずれを返す。"""
    if reference.numel() < 16 or candidate.numel() != reference.numel() or radius <= 0:
        return 0
    centered_reference = reference - reference.mean()
    reference_energy = float(centered_reference.square().sum())
    if reference_energy < 1e-7:
        return 0
    best_score = -math.inf
    best_shift = 0
    for shift in range(-radius, radius + 1):
        shifted = shift_values(candidate, shift)
        shifted = shifted - shifted.mean()
        energy = float(shifted.square().sum())
        if energy < 1e-7:
            continue
        score = float(torch.dot(centered_reference, shifted)) / math.sqrt(reference_energy * energy)
        if score > best_score:
            best_score = score
            best_shift = shift
    return best_shift


def anchored_source_values(
    source: torch.Tensor,
    output_positions: torch.Tensor,
    target_start: int,
    target_anchor: int,
    target_end: int,
    source_start: int,
    source_anchor: int,
    source_end: int,
) -> torch.Tensor:
    left_duration = max(1, target_anchor - target_start)
    right_duration = max(1, target_end - target_anchor)
    left_progress = (output_positions - target_start) / left_duration
    right_progress = (output_positions - target_anchor) / right_duration
    left_positions = source_start + left_progress * (source_anchor - source_start)
    right_positions = source_anchor + right_progress * (source_end - source_anchor)
    positions = torch.where(output_positions <= target_anchor, left_positions, right_positions)
    positions = positions.clamp(0, source.numel() - 1)
    left = positions.floor().long()
    right = (left + 1).clamp_max(source.numel() - 1)
    fraction = positions - left
    return source[left] * (1 - fraction) + source[right] * fraction


def segment_midpoint(segment: dict) -> int:
    return round((segment["start_sample"] + segment["end_sample"]) / 2)


@functools.lru_cache(maxsize=8192)
def cached_diphone_f0(path: str, start: int, end: int) -> float:
    sample_rate, wave = cached_wav(path)
    return estimate_f0_median(wave[start:end], sample_rate)


@functools.lru_cache(maxsize=256)
def cached_utterance_f0(path: str) -> float:
    sample_rate, wave = cached_wav(path)
    return estimate_f0_median(wave, sample_rate)


def choose_diphone_candidate(
    target_utterance: dict,
    target_left: dict,
    target_right: dict,
    candidates: list[tuple[dict, dict, dict]],
    selection: str = "context-duration",
) -> tuple[dict, dict, dict]:
    target_duration = segment_midpoint(target_right) - segment_midpoint(target_left)

    def rank(item: tuple[dict, dict, dict]) -> tuple:
        utterance, left, right = item
        if utterance["id"] == target_utterance["id"]:
            return math.inf, utterance["id"], left["index"]
        context_penalty = 0
        if left["left_phone"] != target_left["left_phone"]:
            context_penalty += 2
        if right["right_phone"] != target_right["right_phone"]:
            context_penalty += 2
        source_duration = segment_midpoint(right) - segment_midpoint(left)
        duration_penalty = abs(math.log(max(1, source_duration) / max(1, target_duration)))
        if selection == "duration-first":
            return duration_penalty, context_penalty, utterance["id"], left["index"]
        return context_penalty + duration_penalty, utterance["id"], left["index"]

    selected = min(candidates, key=rank)
    if selected[0]["id"] == target_utterance["id"]:
        raise ValueError(f"diphone candidate leakage for {target_utterance['id']}:{target_left['index']}")
    return selected


def choose_f0_oracle_candidate(
    target_utterance: dict,
    target_left: dict,
    target_right: dict,
    candidates: list[tuple[dict, dict, dict]],
    target_wave: torch.Tensor,
    target_rate: int,
    shortlist_size: int = 8,
) -> tuple[dict, dict, dict]:
    """正解F0を使う診断専用の候補選択。製品runtimeでは使用しない。"""
    target_start = segment_midpoint(target_left)
    target_end = segment_midpoint(target_right)
    target_seconds = max(1e-9, (target_end - target_start) / target_rate)
    reference_f0 = estimate_f0_median(target_wave, target_rate)
    target_f0 = fold_f0(
        estimate_f0_median(target_wave[target_start:target_end], target_rate), reference_f0
    )

    ranked = []
    for item in candidates:
        utterance, left, right = item
        if utterance["id"] == target_utterance["id"]:
            continue
        source_seconds = max(1e-9, (segment_midpoint(right) - segment_midpoint(left)) / utterance["sample_rate"])
        duration_penalty = abs(math.log(source_seconds / target_seconds))
        context_penalty = int(left["left_phone"] != target_left["left_phone"]) + int(
            right["right_phone"] != target_right["right_phone"]
        )
        ranked.append((duration_penalty, context_penalty, utterance["id"], left["index"], item))
    if not ranked:
        raise ValueError(f"diphone candidate leakage for {target_utterance['id']}:{target_left['index']}")
    shortlist = [item[-1] for item in sorted(ranked)[:shortlist_size]]
    if target_f0 <= 0:
        return shortlist[0]

    scored = []
    for utterance, left, right in shortlist:
        source_start, source_end = segment_midpoint(left), segment_midpoint(right)
        source_f0 = cached_diphone_f0(utterance["audio_path"], source_start, source_end)
        source_seconds = max(1e-9, (source_end - source_start) / utterance["sample_rate"])
        if source_f0 <= 0:
            pitch_error = math.inf
        else:
            rendered_f0 = fold_f0(source_f0 * source_seconds / target_seconds, target_f0)
            pitch_error = abs(1200 * math.log2(rendered_f0 / target_f0))
        scored.append((pitch_error, utterance["id"], left["index"], (utterance, left, right)))
    selected = min(scored)[-1]
    if not math.isfinite(min(scored)[0]):
        return shortlist[0]
    return selected


def render_utterance(
    utterance: dict,
    candidates: dict[str, list[tuple[dict, dict]]],
    output_rate: int,
    fade_ms: float,
) -> tuple[torch.Tensor, torch.Tensor, list[dict]]:
    target_rate, target_wave = cached_wav(utterance["audio_path"])
    output_frames = round(utterance["frames"] * output_rate / target_rate)
    target_output = resample(target_wave, output_frames)
    accumulator = torch.zeros(output_frames)
    weights = torch.zeros(output_frames)
    fade = max(1, round(fade_ms * output_rate / 1000))
    decisions = []
    for target in utterance["segments"]:
        phone = target["phone"]
        if phone in SILENCE_PHONES:
            continue
        available = candidates.get(phone, [])
        if not available:
            decisions.append({"target_index": target["index"], "phone": phone, "fallback": "missing_candidate"})
            continue
        source_utterance, source_segment = choose_candidate(utterance, target, available)
        target_start = round(target["start_sample"] * output_rate / target_rate)
        target_end = round(target["end_sample"] * output_rate / target_rate)
        render_start = max(0, target_start - fade // 2)
        render_end = min(output_frames, target_end + fade // 2)
        positions = torch.arange(render_start, render_end, dtype=torch.float32)
        source_rate, source_wave = cached_wav(source_utterance["audio_path"])
        values = source_values(source_wave, source_rate, source_segment, positions, target_start, target_end, output_rate)
        unit_weight = torch.ones(render_end - render_start)
        if target_start > 0:
            fade_end = min(render_end, target_start + fade // 2)
            count = max(0, fade_end - render_start)
            if count > 0:
                phase = torch.linspace(0, math.pi / 2, count)
                unit_weight[:count] = torch.sin(phase).square()
        if target_end < output_frames:
            fade_start = max(render_start, target_end - fade // 2)
            count = max(0, render_end - fade_start)
            if count > 0:
                phase = torch.linspace(math.pi / 2, 0, count)
                unit_weight[-count:] = torch.sin(phase).square()
        accumulator[render_start:render_end] += values * unit_weight
        weights[render_start:render_end] += unit_weight
        decisions.append(
            {
                "target_index": target["index"],
                "phone": phone,
                "source_utterance_id": source_utterance["id"],
                "source_index": source_segment["index"],
                "source_phone": source_segment["phone"],
                "target_start_sample": target_start,
                "target_end_sample": target_end,
            }
        )
    rendered = torch.where(weights > 1e-6, accumulator / weights.clamp_min(1e-6), torch.zeros_like(accumulator))
    return target_output, rendered.clamp(-1, 1), decisions


def render_diphone_utterance(
    utterance: dict,
    candidates: dict[str, list[tuple[dict, dict, dict]]],
    output_rate: int,
    fade_ms: float,
    time_warp: str,
    selection: str = "context-duration",
    phase_align: bool = False,
    candidate_overrides: dict[int, tuple[dict, dict, dict]] | None = None,
) -> tuple[torch.Tensor, torch.Tensor, list[dict]]:
    target_rate, target_wave = cached_wav(utterance["audio_path"])
    target_reference_f0 = cached_utterance_f0(utterance["audio_path"])
    output_frames = round(utterance["frames"] * output_rate / target_rate)
    target_output = resample(target_wave, output_frames)
    accumulator = torch.zeros(output_frames)
    weights = torch.zeros(output_frames)
    fade = max(1, round(fade_ms * output_rate / 1000))
    decisions = []
    segments = utterance["segments"]
    for position in range(len(segments) - 1):
        target_left, target_right = segments[position], segments[position + 1]
        key = diphone_key(target_left, target_right)
        available = candidates.get(key, [])
        if not available:
            decisions.append({"target_index": position, "diphone": key, "fallback": "missing_candidate"})
            continue
        if candidate_overrides is not None and position in candidate_overrides:
            source_utterance, source_left, source_right = candidate_overrides[position]
        elif selection == "f0-oracle":
            source_utterance, source_left, source_right = choose_f0_oracle_candidate(
                utterance, target_left, target_right, available, target_wave, target_rate
            )
        else:
            source_utterance, source_left, source_right = choose_diphone_candidate(
                utterance, target_left, target_right, available, selection
            )
        target_start = round(segment_midpoint(target_left) * output_rate / target_rate)
        target_end = round(segment_midpoint(target_right) * output_rate / target_rate)
        render_start = max(0, target_start - fade // 2)
        render_end = min(output_frames, target_end + fade // 2)
        if render_end <= render_start or target_end <= target_start:
            decisions.append({"target_index": position, "diphone": key, "fallback": "invalid_target_span"})
            continue
        positions = torch.arange(render_start, render_end, dtype=torch.float32)
        source_rate, source_wave = cached_wav(source_utterance["audio_path"])
        source_start = segment_midpoint(source_left)
        source_end = segment_midpoint(source_right)
        source_duration_seconds = (source_end - source_start) / source_rate
        target_duration_seconds = (target_end - target_start) / output_rate
        pitch_shift_semitones = 12 * math.log2(
            max(1e-9, source_duration_seconds) / max(1e-9, target_duration_seconds)
        )
        source_f0 = cached_diphone_f0(source_utterance["audio_path"], source_start, source_end)
        target_source_start = segment_midpoint(target_left)
        target_source_end = segment_midpoint(target_right)
        target_f0 = fold_f0(
            estimate_f0_median(target_wave[target_source_start:target_source_end], target_rate),
            target_reference_f0,
        )
        rendered_f0 = fold_f0(
            source_f0 * source_duration_seconds / max(1e-9, target_duration_seconds),
            target_f0 if target_f0 > 0 else target_reference_f0,
        )
        target_anchor = round(target_right["start_sample"] * output_rate / target_rate)
        source_anchor = source_right["start_sample"]
        if time_warp == "phone-anchor":
            values = anchored_source_values(
                source_wave, positions, target_start, target_anchor, target_end,
                source_start, source_anchor, source_end,
            )
        else:
            values = source_values(
                source_wave, source_rate,
                {"start_sample": source_start, "end_sample": source_end},
                positions, target_start, target_end, output_rate,
            )
        unit_weight = torch.ones(render_end - render_start)
        if target_start > 0:
            count = min(unit_weight.numel(), fade)
            unit_weight[:count] = torch.sin(torch.linspace(0, math.pi / 2, count)).square()
        if target_end < output_frames:
            count = min(unit_weight.numel(), fade)
            unit_weight[-count:] = torch.sin(torch.linspace(math.pi / 2, 0, count)).square()
        phase_shift_samples = 0
        if phase_align and rendered_f0 >= 60:
            existing_weights = weights[render_start:render_end]
            overlap = (existing_weights > 1e-6) & (unit_weight > 1e-6)
            indices = torch.nonzero(overlap, as_tuple=False).flatten()
            if indices.numel() >= 16:
                reference = accumulator[render_start:render_end][indices] / existing_weights[indices]
                candidate_overlap = values[indices]
                radius = min(fade // 2, max(1, round(output_rate / rendered_f0 / 2)))
                phase_shift_samples = best_phase_shift(reference, candidate_overlap, radius)
                values = shift_values(values, phase_shift_samples)
        accumulator[render_start:render_end] += values * unit_weight
        weights[render_start:render_end] += unit_weight
        decisions.append(
            {
                "target_index": position,
                "diphone": key,
                "source_utterance_id": source_utterance["id"],
                "source_left_index": source_left["index"],
                "source_right_index": source_right["index"],
                "target_start_sample": target_start,
                "target_anchor_sample": target_anchor,
                "target_end_sample": target_end,
                "source_start_sample": source_start,
                "source_anchor_sample": source_anchor,
                "source_end_sample": source_end,
                "duration_ratio": target_duration_seconds / max(1e-9, source_duration_seconds),
                "pitch_shift_semitones": pitch_shift_semitones,
                "source_f0_hz": source_f0,
                "rendered_f0_hz": rendered_f0,
                "target_f0_hz": target_f0,
                "phase_shift_samples": phase_shift_samples,
            }
        )
    rendered = torch.where(weights > 1e-6, accumulator / weights.clamp_min(1e-6), torch.zeros_like(accumulator))
    return target_output, rendered.clamp(-1, 1), decisions


def write_html(path: Path, rows: list[dict]) -> None:
    parts = [
        "<!doctype html><meta charset='utf-8'><title>Neural renderer baseline</title>",
        "<style>body{font-family:sans-serif;max-width:960px;margin:30px auto}section{margin:28px 0}audio{display:block;width:100%;margin:5px 0 12px}</style>",
        "<h1>Neural renderer Phase 0 baseline</h1>",
        "<p>正解の自然音声と、別発話の同一phone候補だけで再構成した固定rendererです。</p>",
    ]
    for row in rows:
        parts.append(f"<section><h2>{html.escape(row['id'])}</h2>")
        parts.append(f"<label>自然音声</label><audio controls src='{row['target']}'></audio>")
        if row.get("comparison"):
            parts.append(
                f"<label>{html.escape(row.get('comparison_label', 'comparison'))}</label>"
                f"<audio controls src='{row['comparison']}'></audio>"
            )
        parts.append(f"<label>{html.escape(row['baseline_label'])}</label><audio controls src='{row['baseline']}'></audio>")
        parts.append("</section>")
    path.write_text("\n".join(parts), encoding="utf-8")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--dataset", default="out/neural-renderer/jsut-index.json")
    parser.add_argument("--split", choices=("validation", "test"), default="validation")
    parser.add_argument("--out", default="out/neural-renderer/baseline")
    parser.add_argument("--limit", type=int, default=12)
    parser.add_argument("--sample-rate", type=int, default=24_000)
    parser.add_argument("--fade-ms", type=float, default=10.0)
    parser.add_argument("--unit", choices=("phone", "diphone"), default="diphone")
    parser.add_argument("--time-warp", choices=("linear", "phone-anchor"), default="linear")
    parser.add_argument(
        "--selection", choices=("context-duration", "duration-first", "f0-oracle"),
        default="context-duration",
    )
    parser.add_argument("--compare-dir", default="")
    parser.add_argument("--compare-label", default="comparison")
    parser.add_argument("--phase-align", action="store_true")
    args = parser.parse_args()

    index = load_index(args.dataset)
    pool = build_diphone_pool(index) if args.unit == "diphone" else build_candidate_pool(index)
    targets = [item for item in index["utterances"] if item["split"] == args.split]
    if args.limit > 0:
        targets = targets[: args.limit]
    if not targets:
        raise SystemExit(f"no {args.split} utterances")
    output = Path(args.out)
    output.mkdir(parents=True, exist_ok=True)
    rows = []
    failures = []
    for number, utterance in enumerate(targets, 1):
        try:
            if args.unit == "diphone":
                target, rendered, decisions = render_diphone_utterance(
                    utterance, pool, args.sample_rate, args.fade_ms, args.time_warp,
                    args.selection, args.phase_align,
                )
            else:
                target, rendered, decisions = render_utterance(
                    utterance, pool, args.sample_rate, args.fade_ms
                )
            prefix = f"{number:03d}-{utterance['id']}"
            target_name = prefix + "-01-natural.wav"
            baseline_name = prefix + "-02-baseline.wav"
            write_pcm16(output / target_name, args.sample_rate, target)
            write_pcm16(output / baseline_name, args.sample_rate, rendered)
            (output / (prefix + ".json")).write_text(
                json.dumps({"version": 1, "target": utterance["id"], "decisions": decisions}, ensure_ascii=False, indent=2) + "\n",
                encoding="utf-8",
            )
            row = {
                "id": utterance["id"], "target": target_name, "baseline": baseline_name,
                "baseline_label": (
                    f"{args.time_warp} / {args.selection}" + (" / phase-align" if args.phase_align else "")
                    if args.unit == "diphone" else "phone baseline"
                ),
            }
            if args.compare_dir:
                comparison = Path(args.compare_dir).resolve() / baseline_name
                if comparison.is_file():
                    row["comparison"] = Path(os.path.relpath(comparison, output.resolve())).as_posix()
                    row["comparison_label"] = args.compare_label
            rows.append(row)
        except (OSError, ValueError) as error:
            failures.append({"id": utterance["id"], "error": str(error)})
    write_html(output / "index.html", rows)
    report = {
        "version": 1, "split": args.split, "unit": args.unit, "time_warp": args.time_warp,
        "selection": args.selection, "phase_align": args.phase_align,
        "written": len(rows), "failures": failures,
    }
    (output / "report.json").write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps(report, ensure_ascii=False))
    print(f"wrote {output / 'index.html'}")


if __name__ == "__main__":
    main()
