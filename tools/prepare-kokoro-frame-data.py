#!/usr/bin/env python3
"""Create frame-intonation JSONL from Kokoro Speech Dataset WAV files.

Kokoro provides utterance clips but no phone timestamps. This trial preparer
detects active audio bounds and distributes Open JTalk morae uniformly inside
them. Every record identifies this approximation; it is not forced alignment.
"""

from __future__ import annotations

import argparse
import csv
import json
import math
import sys
import wave
from pathlib import Path

import pyopenjtalk

sys.path.insert(0, str(Path(__file__).resolve().parent))
from openjtalk_features import analyze  # noqa: E402
from mora_alignment import align_accent_viterbi  # noqa: E402


def normalized_phones(value: str) -> list[str]:
    result = []
    for phone in value.split():
        if phone in {"_", "pau", ".", ",", "?", "!"}:
            continue
        if phone == "q":
            result.append("cl")
        elif phone.endswith(":"):
            vowel = phone[:-1].lower()
            result.extend([vowel, vowel])
        else:
            result.append(phone.lower() if phone in "AIUEO" else phone)
    return result


def wav_info(path: Path) -> tuple[int, list[int]]:
    with wave.open(str(path), "rb") as source:
        if source.getnchannels() != 1 or source.getsampwidth() != 2:
            raise ValueError("expected mono 16-bit PCM WAV")
        rate = source.getframerate()
        raw = source.readframes(source.getnframes())
    samples = [int.from_bytes(raw[i : i + 2], "little", signed=True) for i in range(0, len(raw), 2)]
    return rate, samples


def active_bounds(samples: list[int], rate: int) -> tuple[float, float]:
    frame = max(1, round(rate * 0.02))
    energies = []
    for start in range(0, len(samples), frame):
        block = samples[start : start + frame]
        energies.append(math.sqrt(sum(value * value for value in block) / max(1, len(block))))
    peak = max(energies, default=0.0)
    threshold = max(80.0, peak * (10.0 ** (-35.0 / 20.0)))
    active = [index for index, value in enumerate(energies) if value >= threshold]
    if not active:
        raise ValueError("no active speech detected")
    start_ms = max(0.0, (active[0] * frame / rate * 1000.0) - 30.0)
    end_ms = min(len(samples) / rate * 1000.0, ((active[-1] + 1) * frame / rate * 1000.0) + 30.0)
    return start_ms, end_ms


def timed_tokens(text: str, start_ms: float, end_ms: float) -> tuple[str, list[dict]]:
    # Kokoro uses spaces as morphological separators, not audible pauses.
    normalized_text = "".join(text.split())
    reading, linguistic = analyze(normalized_text)
    spoken = [token for token in linguistic if not token.get("pause", False)]
    if not spoken:
        raise ValueError("Open JTalk produced no morae")
    pause_count = sum(bool(token.get("pause", False)) for token in linguistic)
    available = end_ms - start_ms
    pause_ms = min(180.0, available * 0.08) if pause_count else 0.0
    mora_ms = (available - pause_ms * pause_count) / len(spoken)
    if mora_ms <= 20.0:
        raise ValueError("audio is too short for its mora count")
    cursor = start_ms
    result = []
    for token in linguistic:
        duration = pause_ms if token.get("pause", False) else mora_ms
        copied = dict(token)
        copied.update(start_ms=cursor, end_ms=cursor + duration, duration_ms=duration)
        result.append(copied)
        cursor += duration
    result[-1]["end_ms"] = end_ms
    result[-1]["duration_ms"] = end_ms - float(result[-1]["start_ms"])
    return reading, result


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--corpus", required=True, help="directory containing metadata.csv and wavs/")
    parser.add_argument("--out", required=True)
    parser.add_argument("--limit", type=int, default=0)
    parser.add_argument("--allow-reading-mismatch", action="store_true",
                        help="keep clips whose supplied reading differs from Open JTalk")
    parser.add_argument("--alignment", choices=("uniform", "viterbi"), default="viterbi",
                        help="uniform mora placement or accent-guided Viterbi alignment")
    parser.add_argument("--world-engine", help="UtauTTS WORLD engine used for Viterbi F0 (default: internal)")
    parser.add_argument("--high-cents", type=float, default=70.0, help="Viterbi accent emission scale")
    parser.add_argument("--min-ms", type=float, default=60.0, help="minimum mora duration")
    parser.add_argument("--max-ms", type=float, default=300.0, help="maximum mora duration")
    args = parser.parse_args()
    worldline = None
    if args.alignment == "viterbi" and args.world_engine:
        from world_engine_f0 import WorldEngineF0

        worldline = WorldEngineF0(args.world_engine, 1)
    corpus = Path(args.corpus).resolve()
    with (corpus / "metadata.csv").open(encoding="utf-8", newline="") as stream:
        rows = list(csv.reader(stream, delimiter="|"))
    if args.limit > 0:
        rows = rows[: args.limit]
    records, skipped = [], []
    for row in rows:
        if len(row) < 3:
            skipped.append((row[0] if row else "<empty>", "invalid metadata row"))
            continue
        utterance_id, text, supplied_reading = row[:3]
        normalized_text = "".join(text.split())
        audio = corpus / "wavs" / f"{utterance_id}.wav"
        try:
            if not audio.is_file():
                raise ValueError(f"missing audio: {audio.name}")
            openjtalk_phones = pyopenjtalk.g2p(normalized_text, kana=False)
            if not args.allow_reading_mismatch and normalized_phones(supplied_reading) != normalized_phones(openjtalk_phones):
                raise ValueError("supplied reading differs from Open JTalk")
            rate, samples = wav_info(audio)
            start_ms, end_ms = active_bounds(samples, rate)
            openjtalk_reading, tokens = timed_tokens(normalized_text, start_ms, end_ms)
            alignment_source = "uniform_mora_with_energy_bounds"
            if args.alignment == "viterbi":
                tokens = align_accent_viterbi(
                    tokens, samples, rate, start_ms, end_ms, worldline=worldline,
                    high_cents=args.high_cents, min_ms=args.min_ms, max_ms=args.max_ms,
                )
                alignment_source = "accent_viterbi_duration_constrained"
            records.append(
                {
                    "version": 1,
                    "id": utterance_id,
                    "text": normalized_text,
                    "source_text": text,
                    "audio_path": str(audio),
                    "tokens": tokens,
                    "accent_source": "openjtalk",
                    "alignment_source": alignment_source,
                    "source_reading": supplied_reading,
                    "openjtalk_reading": openjtalk_reading,
                    "openjtalk_phones": openjtalk_phones,
                }
            )
        except Exception as error:
            skipped.append((utterance_id, str(error)))
    if not records:
        raise ValueError("no usable Kokoro records")
    output = Path(args.out)
    output.parent.mkdir(parents=True, exist_ok=True)
    with output.open("x", encoding="utf-8") as stream:
        for record in records:
            stream.write(json.dumps(record, ensure_ascii=False) + "\n")
    print(f"wrote {len(records)} records; skipped {len(skipped)}: {output}")
    for utterance_id, reason in skipped[:10]:
        print(f"skip {utterance_id}: {reason}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
