#!/usr/bin/env python3
"""JSUTとHTSラベルから境界補修用manifestを生成する。"""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import wave
from dataclasses import asdict, dataclass
from pathlib import Path


PHONE_PATTERN = re.compile(r"-([^+]+)\+")
SILENCES = {"sil", "pau", "xx"}


@dataclass(frozen=True)
class Segment:
    start_ticks: int
    end_ticks: int
    phone: str


@dataclass(frozen=True)
class Record:
    version: int
    id: str
    audio_path: str
    sample_rate: int
    boundary_sample: int
    left_phone: str
    right_phone: str
    split: str


def parse_label(path: Path) -> list[Segment]:
    result = []
    for number, raw in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
        parts = raw.split(maxsplit=2)
        if len(parts) != 3:
            raise ValueError(f"{path}:{number}: invalid HTS label")
        match = PHONE_PATTERN.search(parts[2])
        if match is None:
            raise ValueError(f"{path}:{number}: phone not found")
        result.append(Segment(int(parts[0]), int(parts[1]), match.group(1)))
    return result


def validation_split(utterance_id: str) -> str:
    value = hashlib.sha256(utterance_id.encode("utf-8")).digest()[0]
    return "validation" if value < 26 else "train"


def sample_at(ticks: int, sample_rate: int) -> int:
    return round(ticks * sample_rate / 10_000_000)


def build_records(
    wav_path: Path,
    label_path: Path,
    context_ms: float,
) -> list[Record]:
    with wave.open(str(wav_path), "rb") as source:
        if source.getnchannels() != 1 or source.getsampwidth() != 2:
            raise ValueError(f"unsupported WAV format: {wav_path}")
        sample_rate = source.getframerate()
        frames = source.getnframes()
    segments = parse_label(label_path)
    margin = round(context_ms * sample_rate / 1000)
    split = validation_split(wav_path.stem)
    result = []
    for index in range(1, len(segments)):
        left, right = segments[index - 1], segments[index]
        if left.phone in SILENCES or right.phone in SILENCES:
            continue
        boundary = sample_at(right.start_ticks, sample_rate)
        if boundary < margin or boundary+margin >= frames:
            continue
        result.append(
            Record(
                version=1,
                id=f"{wav_path.stem}:{index:03d}",
                audio_path=str(wav_path.resolve()),
                sample_rate=sample_rate,
                boundary_sample=boundary,
                left_phone=left.phone,
                right_phone=right.phone,
                split=split,
            )
        )
    return result


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--jsut", default="data/jsut/basic5000")
    parser.add_argument("--labels", default=".tmp-jsut-label/labels/basic5000")
    parser.add_argument("--out", default="out/boundary-refiner/jsut-boundaries.jsonl")
    parser.add_argument("--limit", type=int, default=0)
    parser.add_argument("--context-ms", type=float, default=120.0)
    args = parser.parse_args()

    root = Path(args.jsut)
    labels = Path(args.labels)
    output = Path(args.out)
    wav_paths = sorted((root / "wav").glob("*.wav"))
    if args.limit > 0:
        wav_paths = wav_paths[: args.limit]
    if not wav_paths:
        raise SystemExit(f"WAV not found: {root / 'wav'}")

    records = []
    failures = []
    for wav_path in wav_paths:
        label_path = labels / f"{wav_path.stem}.lab"
        try:
            records.extend(build_records(wav_path, label_path, args.context_ms))
        except (OSError, ValueError) as error:
            failures.append({"id": wav_path.stem, "error": str(error)})

    output.parent.mkdir(parents=True, exist_ok=True)
    with output.open("w", encoding="utf-8", newline="\n") as destination:
        for record in records:
            destination.write(json.dumps(asdict(record), ensure_ascii=False) + "\n")
    report = {
        "version": 1,
        "utterances": len(wav_paths),
        "records": len(records),
        "train": sum(record.split == "train" for record in records),
        "validation": sum(record.split == "validation" for record in records),
        "failures": failures,
    }
    report_path = output.with_suffix(output.suffix + ".report.json")
    report_path.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps(report, ensure_ascii=False))
    print(f"wrote {output}")


if __name__ == "__main__":
    main()
