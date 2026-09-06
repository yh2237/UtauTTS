#!/usr/bin/env python3
"""Measure alignment coverage between a prosody JSONL and Open JTalk morae."""

import argparse
import json
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "tools"))
from openjtalk_features import analyze


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--dataset", required=True)
    args = parser.parse_args()
    records = matched = same_morae = failures = 0
    with Path(args.dataset).open("r", encoding="utf-8") as source:
        for line in source:
            if not line.strip():
                continue
            record = json.loads(line)
            records += 1
            try:
                _, annotated = analyze(record["text"])
            except Exception:
                failures += 1
                continue
            targets = record["tokens"]
            if len(annotated) != len(targets):
                continue
            matched += 1
            if all(bool(left.get("pause")) == bool(right.get("pause")) and (left.get("pause") or left.get("mora") == right.get("mora")) for left, right in zip(annotated, targets)):
                same_morae += 1
    print(json.dumps({
        "records": records,
        "openjtalk_failures": failures,
        "length_aligned": matched,
        "length_alignment_rate": matched / records if records else 0,
        "exact_mora_aligned": same_morae,
        "exact_mora_alignment_rate": same_morae / records if records else 0,
    }, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
