#!/usr/bin/env python3
"""diphone再構成で生じるF0差と境界ジャンプを監査する。"""

from __future__ import annotations

import argparse
import json
import math
import statistics
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent))
from neural_renderer_common import estimate_f0_median, load_index, read_pcm16  # noqa: E402


def midpoint(segment: dict) -> int:
    return round((segment["start_sample"] + segment["end_sample"]) / 2)


def cents(left: float, right: float) -> float:
    if left <= 0 or right <= 0:
        return 0.0
    return 1200 * math.log2(left / right)


def summary(values: list[float]) -> dict:
    if not values:
        return {"count": 0}
    absolute = sorted(abs(value) for value in values)
    return {
        "count": len(values),
        "mean_abs_cents": statistics.mean(absolute),
        "median_abs_cents": statistics.median(absolute),
        "p90_abs_cents": absolute[min(len(absolute) - 1, round(0.9 * (len(absolute) - 1)))],
        "maximum_abs_cents": max(absolute),
        "over_100_cents": sum(value > 100 for value in absolute),
        "over_200_cents": sum(value > 200 for value in absolute),
    }


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--dataset", default="out/neural-renderer/jsut-index.json")
    parser.add_argument("--render-dir", default="out/neural-renderer/diphone-duration-first")
    parser.add_argument("--out", default="out/neural-renderer/diphone-duration-first/pitch-audit.json")
    args = parser.parse_args()

    index = load_index(args.dataset)
    utterances = {item["id"]: item for item in index["utterances"]}
    details = []
    f0_errors = []
    rendered_jumps = []
    target_jumps = []
    records = [
        json.loads(path.read_text(encoding="utf-8"))
        for path in sorted(Path(args.render_dir).glob("[0-9][0-9][0-9]-*.json"))
    ]
    aggregate_path = Path(args.render_dir) / "report.json"
    if not records and aggregate_path.is_file():
        aggregate = json.loads(aggregate_path.read_text(encoding="utf-8"))
        records = [
            {"target": item["id"], "decisions": item["decisions"]}
            for item in aggregate["utterances"]
        ]
    for record in records:
        target_utterance = utterances[record["target"]]
        target_rate, target_wave = read_pcm16(target_utterance["audio_path"])
        previous_rendered_f0 = 0.0
        previous_target_f0 = 0.0
        for decision in record["decisions"]:
            if "fallback" in decision:
                continue
            source_utterance = utterances[decision["source_utterance_id"]]
            source_segments = source_utterance["segments"]
            source_left = source_segments[decision["source_left_index"]]
            source_right = source_segments[decision["source_right_index"]]
            source_rate, source_wave = read_pcm16(source_utterance["audio_path"])
            source_start, source_end = midpoint(source_left), midpoint(source_right)
            target_left = target_utterance["segments"][decision["target_index"]]
            target_right = target_utterance["segments"][decision["target_index"] + 1]
            target_start, target_end = midpoint(target_left), midpoint(target_right)
            source_f0 = decision.get(
                "source_f0_hz",
                estimate_f0_median(source_wave[source_start:source_end], source_rate),
            )
            target_f0 = decision.get(
                "target_f0_hz",
                estimate_f0_median(target_wave[target_start:target_end], target_rate),
            )
            source_seconds = (source_end - source_start) / source_rate
            target_seconds = (target_end - target_start) / target_rate
            rendered_f0 = decision.get(
                "rendered_f0_hz",
                source_f0 * source_seconds / max(1e-9, target_seconds),
            )
            error = cents(rendered_f0, target_f0) if rendered_f0 > 0 and target_f0 > 0 else 0.0
            if rendered_f0 > 0 and target_f0 > 0:
                f0_errors.append(error)
            if previous_rendered_f0 > 0 and rendered_f0 > 0:
                rendered_jumps.append(cents(rendered_f0, previous_rendered_f0))
            if previous_target_f0 > 0 and target_f0 > 0:
                target_jumps.append(cents(target_f0, previous_target_f0))
            if rendered_f0 > 0:
                previous_rendered_f0 = rendered_f0
            if target_f0 > 0:
                previous_target_f0 = target_f0
            details.append({
                "target": record["target"],
                "target_index": decision["target_index"],
                "diphone": decision["diphone"],
                "source_f0_hz": source_f0,
                "rendered_f0_hz": rendered_f0,
                "target_f0_hz": target_f0,
                "f0_error_cents": error,
            })
    report = {
        "version": 1,
        "render_dir": str(Path(args.render_dir)),
        "f0_error": summary(f0_errors),
        "rendered_adjacent_jump": summary(rendered_jumps),
        "target_adjacent_jump": summary(target_jumps),
        "items": details,
    }
    output = Path(args.out)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({key: value for key, value in report.items() if key != "items"}, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
