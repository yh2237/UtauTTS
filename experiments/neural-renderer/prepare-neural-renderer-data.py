#!/usr/bin/env python3
"""JSUTから疑似ボイスバンクindexを生成する。"""

from __future__ import annotations

import argparse
import json
import sys
from collections import Counter
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent))
from neural_renderer_common import SILENCE_PHONES, diphone_key, parse_hts_label, split_for, wav_info  # noqa: E402


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--jsut", default="data/jsut/basic5000")
    parser.add_argument("--labels", default=".tmp-jsut-label/labels/basic5000")
    parser.add_argument("--out", default="out/neural-renderer/jsut-index.json")
    parser.add_argument("--limit", type=int, default=0)
    args = parser.parse_args()

    root = Path(args.jsut)
    labels = Path(args.labels)
    wav_paths = sorted((root / "wav").glob("*.wav"))
    if args.limit > 0:
        wav_paths = wav_paths[: args.limit]
    if not wav_paths:
        raise SystemExit(f"WAV not found: {root / 'wav'}")

    utterances = []
    failures = []
    for wav_path in wav_paths:
        try:
            sample_rate, frames = wav_info(wav_path)
            segments = parse_hts_label(labels / f"{wav_path.stem}.lab", sample_rate)
            if not segments:
                raise ValueError("empty label")
            utterances.append(
                {
                    "id": wav_path.stem,
                    "split": split_for(wav_path.stem),
                    "audio_path": str(wav_path.resolve()),
                    "sample_rate": sample_rate,
                    "frames": frames,
                    "segments": segments,
                }
            )
        except (OSError, ValueError) as error:
            failures.append({"id": wav_path.stem, "error": str(error)})

    train_phones = Counter(
        segment["phone"]
        for utterance in utterances
        if utterance["split"] == "train"
        for segment in utterance["segments"]
        if segment["phone"] not in SILENCE_PHONES
    )
    train_diphones = Counter(
        diphone_key(segments[position], segments[position + 1])
        for utterance in utterances
        if utterance["split"] == "train"
        for segments in (utterance["segments"],)
        for position in range(len(segments) - 1)
    )
    missing = Counter()
    missing_diphones = Counter()
    evaluated = Counter()
    evaluated_diphones = Counter()
    for utterance in utterances:
        if utterance["split"] == "train":
            continue
        for segment in utterance["segments"]:
            phone = segment["phone"]
            if phone in SILENCE_PHONES:
                continue
            evaluated[utterance["split"]] += 1
            if train_phones[phone] == 0:
                missing[f"{utterance['split']}:{phone}"] += 1
        segments = utterance["segments"]
        for position in range(len(segments) - 1):
            key = diphone_key(segments[position], segments[position + 1])
            evaluated_diphones[utterance["split"]] += 1
            if train_diphones[key] == 0:
                missing_diphones[f"{utterance['split']}:{key}"] += 1

    report = {
        "utterances": len(utterances),
        "splits": dict(Counter(item["split"] for item in utterances)),
        "train_phone_types": len(train_phones),
        "train_diphone_types": len(train_diphones),
        "validation_phones": evaluated["validation"],
        "test_phones": evaluated["test"],
        "missing_candidates": dict(sorted(missing.items())),
        "validation_diphones": evaluated_diphones["validation"],
        "test_diphones": evaluated_diphones["test"],
        "missing_diphone_candidates": dict(sorted(missing_diphones.items())),
        "failures": failures,
    }
    result = {
        "version": 1,
        "corpus": "JSUT BASIC5000",
        "split": "sha256-80-10-10-v1",
        "utterances": utterances,
        "report": report,
    }
    output = Path(args.out)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps(report, ensure_ascii=False))
    print(f"wrote {output}")


if __name__ == "__main__":
    main()
