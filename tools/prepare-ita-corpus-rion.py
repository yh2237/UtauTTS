#!/usr/bin/env python3
"""Prepare ITA-Corpus-Rion Emotion recordings for frame-intonation training.

The corpus has no phone timings. Active speech bounds and Open JTalk morae are
used as an explicitly approximate alignment. Same-text speakers share an ID,
so a sentence never occurs in both training and validation.
"""
import argparse
import json
import math
import re
import sys
import wave
from pathlib import Path

import numpy as np

sys.path.insert(0, str(Path(__file__).resolve().parent))
from openjtalk_features import analyze


def read_wav(path):
    with wave.open(str(path), "rb") as source:
        if source.getnchannels() != 1:
            raise ValueError("expected mono PCM WAV")
        width, rate = source.getsampwidth(), source.getframerate()
        raw = source.readframes(source.getnframes())
    if width == 1:
        samples = [value - 128 for value in raw]
    elif width in (2, 3, 4):
        samples = [int.from_bytes(raw[i:i + width], "little", signed=True) for i in range(0, len(raw), width)]
    else:
        raise ValueError(f"unsupported PCM width: {width}")
    return rate, samples


def bounds(samples, rate):
    frame = max(1, round(rate * 0.02))
    energy = []
    for start in range(0, len(samples), frame):
        block = samples[start:start + frame]
        energy.append(math.sqrt(sum(x * x for x in block) / max(1, len(block))))
    threshold = max(80.0, max(energy, default=0.0) * 10 ** (-35.0 / 20.0))
    active = [i for i, value in enumerate(energy) if value >= threshold]
    if not active:
        raise ValueError("no active speech")
    start = max(0.0, active[0] * frame / rate * 1000.0 - 30.0)
    end = min(len(samples) / rate * 1000.0, (active[-1] + 1) * frame / rate * 1000.0 + 30.0)
    return start, end


def timed_tokens(text, start, end):
    reading, linguistic = analyze(text)
    speech = sum(not token.get("pause", False) for token in linguistic)
    pauses = sum(bool(token.get("pause", False)) for token in linguistic)
    pause_ms = min(180.0, (end - start) * 0.08) if pauses else 0.0
    mora_ms = (end - start - pauses * pause_ms) / max(1, speech)
    if mora_ms <= 20:
        raise ValueError("speech interval is too short")
    cursor, result = start, []
    for token in linguistic:
        duration = pause_ms if token.get("pause", False) else mora_ms
        result.append(dict(token, start_ms=cursor, end_ms=cursor + duration, duration_ms=duration))
        cursor += duration
    result[-1]["end_ms"] = end
    result[-1]["duration_ms"] = end - result[-1]["start_ms"]
    return reading, result


def load_transcript(path):
    rows = {}
    for line in path.read_text(encoding="utf-8").splitlines():
        key, fields = line.split(":", 1)
        text, reading = fields.split(",", 1)
        match = re.fullmatch(r"EMOTION100_(\d{3})", key)
        if not match:
            raise ValueError(f"invalid transcript ID: {key}")
        rows[int(match.group(1))] = (text, reading)
    if set(rows) != set(range(1, 101)):
        raise ValueError("expected EMOTION100_001 through EMOTION100_100")
    return rows


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--corpus", required=True)
    parser.add_argument("--out", required=True)
    parser.add_argument("--limit", type=int, default=0)
    parser.add_argument("--lower-duration-factor", type=float, default=0.45)
    parser.add_argument("--upper-duration-factor", type=float, default=2.20)
    args = parser.parse_args()
    if args.limit < 0 or not 0 < args.lower_duration_factor <= 1 <= args.upper_duration_factor:
        parser.error("invalid limit or duration factors")
    root = Path(args.corpus).resolve()
    texts = load_transcript(root / "emotion_transcript_utf8.txt")
    candidates = []
    for speaker in sorted(path for path in root.iterdir() if path.is_dir()):
        for audio in sorted(speaker.glob("*.wav")):
            if args.limit and len(candidates) >= args.limit:
                break
            match = re.fullmatch(r"ITA#(\d{1,3})\.wav", audio.name, re.IGNORECASE)
            if not match:
                continue
            number = int(match.group(1))
            rate, samples = read_wav(audio)
            start, end = bounds(samples, rate)
            text, source_reading = texts[number]
            reading, tokens = timed_tokens(text, start, end)
            candidates.append({"version": 1, "id": f"EMOTION100_{number:03d}",
                "record_id": f"{speaker.name}_EMOTION100_{number:03d}", "speaker": speaker.name,
                "text": text, "audio_path": str(audio.resolve()), "tokens": tokens,
                "start_ms": start, "end_ms": end, "accent_source": "openjtalk",
                "alignment_source": "uniform_mora_with_energy_bounds",
                "source_reading": source_reading, "openjtalk_reading": reading})
        if args.limit and len(candidates) >= args.limit:
            break
    if not candidates:
        raise ValueError("no usable recordings")
    grouped = {}
    for record in candidates:
        grouped.setdefault(record["id"], []).append(record["end_ms"] - record["start_ms"])
    medians = {key: float(np.median(values)) for key, values in grouped.items()}
    records = [record for record in candidates if medians[record["id"]] * args.lower_duration_factor <= record["end_ms"] - record["start_ms"] <= medians[record["id"]] * args.upper_duration_factor]
    output = Path(args.out)
    output.parent.mkdir(parents=True, exist_ok=True)
    with output.open("x", encoding="utf-8") as stream:
        for record in records:
            stream.write(json.dumps(record, ensure_ascii=False) + "\n")
    print(f"wrote {len(records)} records from {len(candidates)} candidates ({len(candidates) - len(records)} duration-filtered): {output}")


if __name__ == "__main__":
    main()