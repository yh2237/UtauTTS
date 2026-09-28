#!/usr/bin/env python3
"""AISHELL-3の音節整列とF0から中国語の声調補正を学習する。"""

from __future__ import annotations

import argparse
import io
import json
import math
import re
import sys
import wave
from collections import Counter
from pathlib import Path

import numpy as np

from world_engine_f0 import WorldEngineF0

KNOTS = (0.25, 0.40, 0.55, 0.70, 0.85)
PROJECT_ROOT = Path(__file__).resolve().parents[1]
FEATURES = (["bias", "position", "position2", "phrase_start", "phrase_end"]
            + [f"tone_{i}" for i in range(1, 6)]
            + [f"prev_tone_{i}" for i in range(1, 6)]
            + [f"next_tone_{i}" for i in range(1, 6)])
INTERVAL = re.compile(r'intervals \[\d+\]:\s*xmin = ([\d.]+)\s*xmax = ([\d.]+)\s*text = "([^"]*)"')


def tone_points(tone: int, final: bool, previous: int):
    if tone == 1:
        return ((0, 145), (1, 145))
    if tone == 2:
        return ((0, -20), (0.3, -55), (1, 145))
    if tone == 3:
        if final:
            return ((0, -35), (0.55, -145), (1, 65))
        return ((0, -35), (0.65, -145), (1, -115))
    if tone == 4:
        return ((0, 145), (0.2, 115), (1, -145))
    end = {1: -100, 2: -55, 3: 65, 4: -130, 5: -35}.get(previous, -35)
    return ((0, end + 25), (1, end))


def baseline(tone: int, final: bool, previous: int, progress: float) -> float:
    points = tone_points(tone, final, previous)
    for (left_x, left_y), (right_x, right_y) in zip(points, points[1:]):
        if progress <= right_x:
            ratio = max(0., min(1., (progress - left_x) / (right_x - left_x)))
            ratio = ratio * ratio * (3 - 2 * ratio)
            return left_y + (right_y - left_y) * ratio
    return points[-1][1]


def features(tones: list[int], index: int, starts: list[bool], ends: list[bool]) -> np.ndarray:
    position = index / max(1, len(tones) - 1)
    values = {"bias": 1., "position": position, "position2": position * position,
              "phrase_start": float(starts[index]), "phrase_end": float(ends[index]),
              f"tone_{tones[index]}": 1.}
    if index and not starts[index]:
        values[f"prev_tone_{tones[index - 1]}"] = 1.
    if index + 1 < len(tones) and not ends[index]:
        values[f"next_tone_{tones[index + 1]}"] = 1.
    return np.asarray([values.get(name, 0.) for name in FEATURES], dtype=np.float64)


def decode_wav(data: bytes) -> tuple[np.ndarray, int]:
    with wave.open(io.BytesIO(data)) as source:
        rate = source.getframerate()
        width = source.getsampwidth()
        channels = source.getnchannels()
        raw = source.readframes(source.getnframes())
    if width != 2:
        raise ValueError(f"unsupported WAV sample width: {width}")
    signal = np.frombuffer(raw, dtype="<i2").astype(np.float64) / 32768.
    return signal.reshape(-1, channels).mean(axis=1), rate


def textgrid_syllables(path: Path) -> list[tuple[float, float, str]]:
    raw = path.read_text(encoding="utf-8")
    words = raw.split('name = "words"', 1)[1].split('item [2]:', 1)[0]
    return [(float(start), float(end), label) for start, end, label in INTERVAL.findall(words)]


def sample_f0(f0: np.ndarray, start: float, end: float, progress: float) -> float:
    center = start + (end - start) * progress
    first = max(0, round((center - 0.025) * 100))
    last = min(len(f0), round((center + 0.025) * 100) + 1)
    voiced = f0[first:last]
    voiced = voiced[(voiced > 55) & (voiced < 650)]
    return float(np.median(voiced)) if len(voiced) else 0.


def collect(parquet: Path, alignments: Path, limit_per_speaker: int, engine: WorldEngineF0):
    import pyarrow.parquet as pq

    table = pq.ParquetFile(parquet)
    counts = Counter()
    rows = []
    skipped = Counter()
    for group in range(table.num_row_groups):
        for record in table.read_row_group(group).to_pylist():
            audio = record["audio"]
            utterance = Path(audio["path"]).stem
            speaker = utterance[:7]
            if counts[speaker] >= limit_per_speaker:
                continue
            grid = alignments / speaker / (utterance + ".TextGrid")
            if not grid.exists():
                skipped["missing_alignment"] += 1
                continue
            intervals = [(a, b, label) for a, b, label in textgrid_syllables(grid) if label]
            labels = [label for _, _, label in intervals]
            expected = record["pinyin"].split()
            if labels != expected or len(labels) < 3:
                skipped["label_mismatch"] += 1
                continue
            tones = [int(label[-1]) if label[-1].isdigit() else 0 for label in labels]
            if any(tone not in range(1, 6) for tone in tones):
                skipped["invalid_tone"] += 1
                continue
            samples, rate = decode_wav(audio["bytes"])
            f0 = engine.extract(samples, rate, 10.)
            starts = [i == 0 or intervals[i][0] - intervals[i - 1][1] > 0.12 for i in range(len(intervals))]
            ends = [i == len(intervals) - 1 or intervals[i + 1][0] - intervals[i][1] > 0.12 for i in range(len(intervals))]
            measured = np.asarray([[sample_f0(f0, a, b, k) for k in KNOTS] for a, b, _ in intervals])
            valid = measured > 0
            if valid.sum() < max(4, len(tones)):
                skipped["unvoiced"] += 1
                continue
            log_f0 = np.zeros_like(measured)
            log_f0[valid] = 1200 * np.log2(measured[valid] / np.median(measured[valid]))
            rule = np.asarray([[baseline(tone, ends[i], tones[i - 1] if i and not starts[i] else 5, k)
                                for k in KNOTS] for i, tone in enumerate(tones)])
            rule -= np.median(rule[valid])
            residual = log_f0 - rule
            rows.append((speaker, utterance, np.stack([features(tones, i, starts, ends) for i in range(len(tones))]), residual, valid, rule, log_f0))
            counts[speaker] += 1
    return rows, dict(counts), dict(skipped)


def fit(rows, lam: float):
    xs = [[] for _ in KNOTS]
    ys = [[] for _ in KNOTS]
    for _, _, x, residual, valid, _, _ in rows:
        for k in range(len(KNOTS)):
            good = valid[:, k] & (np.abs(residual[:, k]) < 600)
            xs[k].append(x[good])
            ys[k].append(residual[good, k])
    weights = []
    for x_parts, y_parts in zip(xs, ys):
        x = np.concatenate(x_parts)
        y = np.concatenate(y_parts)
        regularizer = np.eye(len(FEATURES)) * lam
        regularizer[0, 0] = lam * 0.1
        weights.append(np.linalg.solve(x.T @ x + regularizer, x.T @ y))
    return np.stack(weights)


def evaluate(rows, weights, strength=0.65):
    errors = []
    base = []
    for _, _, x, _, valid, rule, target in rows:
        correction = np.clip(x @ weights.T, -100, 100) * strength
        errors.extend(np.abs((rule + correction - target)[valid]))
        base.extend(np.abs((rule - target)[valid]))
    return {"mae_cents": round(float(np.mean(errors)), 2), "rule_mae_cents": round(float(np.mean(base)), 2),
            "evaluated_knots": len(errors)}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--parquet", type=Path, default=PROJECT_ROOT / "data/aishell3/train-00000-of-00045.parquet")
    parser.add_argument("--alignments", type=Path, default=PROJECT_ROOT / "data/aishell3/aishell3_alignment_tone")
    parser.add_argument("--out", type=Path, default=PROJECT_ROOT / "out/tone-intonation-zh-v1/tone-intonation-zh-v1.json")
    parser.add_argument("--limit-per-speaker", type=int, default=250)
    parser.add_argument("--world-engine", type=Path, default=PROJECT_ROOT / "runtime/utautts-world-engine.dll")
    args = parser.parse_args()
    rows, counts, skipped = collect(args.parquet, args.alignments, args.limit_per_speaker, WorldEngineF0(args.world_engine))
    if len(counts) < 3:
        raise ValueError("at least three speakers are required for a held-out evaluation")
    speakers = sorted(counts)
    held_out = speakers[-1]
    train = [row for row in rows if row[0] != held_out]
    test = [row for row in rows if row[0] == held_out]
    if len(train) < 100 or len(test) < 50:
        raise ValueError("insufficient training or held-out utterances")
    candidates = [(evaluate(test, fit(train, lam)), lam) for lam in (25., 100., 400., 1200.)]
    metrics, lam = min(candidates, key=lambda pair: pair[0]["mae_cents"])
    weights = fit(train, lam)
    model = {
        "id": "tone-intonation-zh-v1", "display_name": "Mandarin Tone Intonation v1",
        "description": "AISHELL-3 learned correction to Mandarin tone contours",
        "license": "Apache License 2.0 (model weights)",
        "license_notices": ["licenses/TONE-INTONATION-ZH-V1.txt", "licenses/AISHELL-3-NOTICE.txt",
                            "licenses/PADDLESPEECH-AISHELL3-ALIGNMENT-NOTICE.txt", "licenses/APACHE-2.0.txt"],
        "language": "zh", "provenance": {
            "training_corpus": "AISHELL-3 subset (3 speakers, 750 utterances)",
            "training_data_kind": "natural-mandarin-speech",
            "corpus_url": "https://www.openslr.org/93/", "source_license": "Apache License 2.0",
            "alignment": "PaddleSpeech AISHELL-3 MFA2.x with tone",
            "alignment_url": "https://github.com/PaddlePaddle/PaddleSpeech/tree/develop/examples/aishell3/tts3",
        },
        "recommended_renderers": ["utautts-world-phrase"], "default_priority": 60,
        "version": 13, "feature_version": 1, "mode": "mandarin_intonation_v1",
        "mandarin_intonation": {"feature_names": FEATURES, "knots": KNOTS, "weights": weights.tolist(),
                                "strength": 0.65, "max_cents": 100},
        "metrics": {"records": len(test), "tokens": sum(len(row[2]) for row in test),
                    "pitch_mae_cents": metrics["mae_cents"], "baseline_pitch_mae_cents": metrics["rule_mae_cents"]},
        "training": {"records": len(train), "tokens": sum(len(row[2]) for row in train),
                     "corpus": "AISHELL-3", "f0_source": "utautts_world_harvest",
                     "alignment": "PaddleSpeech MFA2 with tone", "speakers": counts,
                     "speaker_splits": {"train": speakers[:-1], "validation": [held_out]},
                     "ridge_lambda": lam, "hyperparameter_selected_on_validation": True,
                     "evaluation_is_unbiased_test": False, "skipped": skipped,
                     "validation": metrics, "training_fit": evaluate(train, weights)},
    }
    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text(json.dumps(model, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({"speakers": counts, "skipped": skipped, "held_out_speaker": held_out,
                      "held_out": metrics, "ridge_lambda": lam, "model": str(args.out)}, ensure_ascii=False))


if __name__ == "__main__":
    main()
