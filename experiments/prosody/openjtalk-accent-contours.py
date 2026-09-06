#!/usr/bin/env python3
"""Generate diagnostic mora pitch contours from Open JTalk accent labels."""

import argparse
import json
import math
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "tools"))
from openjtalk_features import analyze


def contour(text):
    reading, tokens = analyze(text)
    factors = []
    for token in tokens:
        if token["pause"]:
            factors.append(1.0)
            continue
        position = token["accent_phrase_position"]
        cents = (30.0 if token["accent_high"] else -30.0) - 1.5 * (position - 1)
        factors.append(math.pow(2.0, cents / 1200.0))

    speech_logs = [math.log(value) for value in factors if value != 1.0]
    if speech_logs:
        center = sorted(speech_logs)[len(speech_logs) // 2]
        factors = [1.0 if value == 1.0 else min(1.03, max(0.97, math.exp(math.log(value) - center))) for value in factors]
    return reading, factors


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--corpus", required=True)
    parser.add_argument("--out", required=True)
    args = parser.parse_args()
    corpus = json.loads(Path(args.corpus).read_text(encoding="utf-8"))
    cases = []
    for item in corpus["cases"]:
        reading, factors = contour(item["text"])
        cases.append({"id": item["id"], "text": item["text"], "reading": reading, "pitch_factors": factors})
    output = {"version": 1, "name": "openjtalk-accent-diagnostic-v1", "cases": cases}
    path = Path(args.out)
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(output, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(f"wrote {len(cases)} contours to {path}")


if __name__ == "__main__":
    main()
