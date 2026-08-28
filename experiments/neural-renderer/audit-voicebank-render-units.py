#!/usr/bin/env python3
"""Classic faithfulの出力をunit単位で監査する。"""

from __future__ import annotations

import argparse
import json
import math
import statistics
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent))
from neural_renderer_common import estimate_f0_median, read_pcm16  # noqa: E402


def cents(value: float, reference: float) -> float:
    if value <= 0 or reference <= 0:
        return 0.0
    return 1200 * math.log2(value / reference)


def percentile(values: list[float], fraction: float) -> float:
    if not values:
        return 0.0
    ordered = sorted(values)
    return ordered[min(len(ordered) - 1, round(fraction * (len(ordered) - 1)))]


def summary(values: list[float]) -> dict:
    if not values:
        return {"count": 0}
    absolute = [abs(value) for value in values]
    return {
        "count": len(values),
        "median_abs_cents": statistics.median(absolute),
        "p90_abs_cents": percentile(absolute, 0.9),
        "maximum_abs_cents": max(absolute),
        "over_200_cents": sum(value > 200 for value in absolute),
    }


def source_usable_ms(unit: dict) -> float:
    sample_rate, wave = read_pcm16(unit["source"])
    duration_ms = wave.numel() * 1000 / sample_rate
    cutoff = unit.get("cutoff_ms", 0.0)
    end_ms = unit["offset_ms"] - cutoff if cutoff < 0 else duration_ms - cutoff
    return max(0.0, end_ms - unit["offset_ms"])


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--plan", required=True)
    parser.add_argument("--wav", required=True)
    parser.add_argument("--out", required=True)
    parser.add_argument("--stems", default="")
    args = parser.parse_args()

    plan = json.loads(Path(args.plan).read_text(encoding="utf-8"))
    sample_rate, rendered = read_pcm16(args.wav)
    stem_records = {}
    stem_directory = None
    if args.stems:
        stem_path = Path(args.stems)
        stem_report = json.loads(stem_path.read_text(encoding="utf-8"))
        stem_records = {item["index"]: item for item in stem_report["units"]}
        stem_directory = stem_path.parent
    items = []
    previous_output_f0 = 0.0
    previous_target_f0 = 0.0
    output_jumps = []
    target_jumps = []
    output_errors = []
    stem_visible_errors = []
    source_cache: dict[tuple[str, float, float, float], float] = {}

    for index, unit in enumerate(plan["units"]):
        if unit.get("silent"):
            continue
        start_ms = unit["note_start_ms"]
        duration_ms = unit["duration_ms"]
        margin_ms = min(25.0, duration_ms * 0.2)
        start = max(0, round((start_ms + margin_ms) * sample_rate / 1000))
        end = min(rendered.numel(), round((start_ms + duration_ms - margin_ms) * sample_rate / 1000))
        output_f0 = estimate_f0_median(rendered[start:end], sample_rate) if end > start else 0.0
        target_f0 = unit.get("target_f0_hz", 0.0)
        output_error = cents(output_f0, target_f0)
        if output_f0 > 0 and target_f0 > 0:
            output_errors.append(output_error)
        if previous_output_f0 > 0 and output_f0 > 0:
            output_jumps.append(cents(output_f0, previous_output_f0))
        if previous_target_f0 > 0 and target_f0 > 0:
            target_jumps.append(cents(target_f0, previous_target_f0))
        if output_f0 > 0:
            previous_output_f0 = output_f0
        if target_f0 > 0:
            previous_target_f0 = target_f0

        cache_key = (
            unit["source"], unit["offset_ms"], unit.get("cutoff_ms", 0.0),
            unit.get("consonant_ms", 0.0),
        )
        if cache_key not in source_cache:
            source_cache[cache_key] = source_usable_ms(unit)
        usable_ms = source_cache[cache_key]
        item = {
            "unit_index": index,
            "position": unit["position"],
            "role": unit["role"],
            "mora": unit["mora"],
            "alias": unit["alias"],
            "source_f0_hz": unit.get("source_f0_hz", 0.0),
            "target_f0_hz": target_f0,
            "output_f0_hz": output_f0,
            "output_f0_error_cents": output_error,
            "source_usable_ms": usable_ms,
            "output_span_ms": duration_ms,
            "unit_time_ratio": duration_ms / usable_ms if usable_ms > 0 else 0.0,
        }
        stem = stem_records.get(index)
        if stem is not None and stem_directory is not None:
            _, raw_stem = read_pcm16(stem_directory / stem["raw_wav"])
            _, visible_stem = read_pcm16(stem_directory / stem["visible_wav"])
            raw_f0 = estimate_f0_median(raw_stem, sample_rate)
            visible_f0 = estimate_f0_median(visible_stem, sample_rate)
            visible_error = cents(visible_f0, target_f0)
            item.update({
                "stem_resampled_f0_hz": raw_f0,
                "stem_visible_f0_hz": visible_f0,
                "stem_visible_f0_error_cents": visible_error,
            })
            if visible_f0 > 0 and target_f0 > 0:
                stem_visible_errors.append(visible_error)
        items.append(item)

    report = {
        "version": 1,
        "plan": str(Path(args.plan)),
        "wav": str(Path(args.wav)),
        "output_f0_error": summary(output_errors),
        "output_adjacent_jump": summary(output_jumps),
        "target_adjacent_jump": summary(target_jumps),
        "stem_visible_f0_error": summary(stem_visible_errors),
        "units": items,
    }
    output = Path(args.out)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({key: value for key, value in report.items() if key != "units"}, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
