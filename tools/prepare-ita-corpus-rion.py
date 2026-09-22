#!/usr/bin/env python3
"""Prepare ITA-Corpus-Rion Emotion recordings for frame-intonation training.

The corpus has no phone timings. Active speech bounds and Open JTalk morae are
used as an explicitly approximate alignment. Same-text speakers share an ID,
so a sentence never occurs in both training and validation.
"""
import argparse
import importlib.util
import json
import math
import re
import sys
import wave
from collections import deque
from pathlib import Path

import numpy as np

sys.path.insert(0, str(Path(__file__).resolve().parent))
from openjtalk_features import analyze, split_morae
from mora_alignment import align_accent_viterbi


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


def count_morae(reading):
    """Count spoken morae in a kana reading using the shared splitter."""

    return sum(1 for token in split_morae(reading) if not token.get("pause", False))


def _energy_envelope(samples, rate, start_ms, end_ms, frame_ms=10.0):
    hop = max(1, int(round(rate * frame_ms / 1000.0)))
    window = hop * 2
    count = max(1, int(math.ceil((end_ms - start_ms) / frame_ms)))
    envelope = np.zeros(count, dtype=np.float64)
    for index in range(count):
        begin = int((start_ms + index * frame_ms) / 1000.0 * rate)
        block = samples[begin:begin + window]
        if len(block):
            envelope[index] = math.sqrt(sum(value * value for value in block) / len(block))
    if len(envelope) > 3:
        envelope = np.convolve(envelope, np.ones(3, dtype=np.float64) / 3.0, mode="same")
    return envelope


def refine_alignment(tokens, samples, rate, start, end, frame_ms=10.0, strength=0.4):
    """Move uniform mora boundaries to nearby energy valleys.

    The corpus has no phone timings, so this only refines the uniform estimate;
    it never reorders morae and keeps each segment within ``strength`` of the
    uniform boundary.
    """

    result = [dict(token) for token in tokens]
    spoken = [index for index, token in enumerate(result) if not token.get("pause", False)]
    if len(spoken) < 2:
        return result
    envelope = _energy_envelope(samples, rate, start, end, frame_ms)
    step = (end - start) / len(spoken)
    bounds = [start + index * step for index in range(len(spoken) + 1)]
    bounds[-1] = end
    refined = [bounds[0]]
    for index in range(1, len(bounds) - 1):
        center = bounds[index]
        radius = step * strength
        low = max(refined[-1] + step * 0.3, center - radius)
        high = min(bounds[index + 1] - step * 0.3, center + radius)
        if high <= low:
            refined.append(center)
            continue
        first = max(0, int((low - start) / frame_ms))
        last = min(len(envelope), int((high - start) / frame_ms))
        if last <= first:
            refined.append(center)
            continue
        offset = first + int(np.argmin(envelope[first:last]))
        refined.append(start + offset * frame_ms)
    refined.append(bounds[-1])
    for position, token_index in enumerate(spoken):
        result[token_index]["start_ms"] = refined[position]
        result[token_index]["end_ms"] = refined[position + 1]
        result[token_index]["duration_ms"] = refined[position + 1] - refined[position]
    return result


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
    parser.add_argument("--alignment", choices=("uniform", "energy", "viterbi"), default="uniform",
                        help="uniform mora placement, energy-valley refinement, or accent-guided Viterbi")
    parser.add_argument("--require-reading-match", action="store_true",
                        help="skip recordings whose Open JTalk mora count differs from the official reading")
    parser.add_argument("--world-engine", help="UtauTTS WORLD engine used for Viterbi F0 (default: internal)")
    args = parser.parse_args()
    if args.limit < 0 or not 0 < args.lower_duration_factor <= 1 <= args.upper_duration_factor:
        parser.error("invalid limit or duration factors")
    worldline = None
    if args.alignment == "viterbi" and args.world_engine:
        from world_engine_f0 import WorldEngineF0

        worldline = WorldEngineF0(args.world_engine, 1)
    root = Path(args.corpus).resolve()
    texts = load_transcript(root / "emotion_transcript_utf8.txt")
    candidates = []
    reading_mismatches = 0
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
            if count_morae(reading) != count_morae(source_reading):
                reading_mismatches += 1
                if args.require_reading_match:
                    continue
            alignment_source = "uniform_mora_with_energy_bounds"
            if args.alignment == "energy":
                tokens = refine_alignment(tokens, samples, rate, start, end)
                alignment_source = "energy_valley_refined_mora"
            elif args.alignment == "viterbi":
                tokens = align_accent_viterbi(tokens, samples, rate, start, end, worldline=worldline)
                alignment_source = "accent_viterbi_duration_constrained"
            candidates.append({"version": 1, "id": f"EMOTION100_{number:03d}",
                "record_id": f"{speaker.name}_EMOTION100_{number:03d}", "speaker": speaker.name,
                "text": text, "audio_path": str(audio.resolve()), "tokens": tokens,
                "start_ms": start, "end_ms": end, "accent_source": "openjtalk",
                "alignment_source": alignment_source,
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
    print(f"wrote {len(records)} records from {len(candidates)} candidates "
          f"({len(candidates) - len(records)} duration-filtered, {reading_mismatches} reading-mismatched): {output}")


if __name__ == "__main__":
    main()