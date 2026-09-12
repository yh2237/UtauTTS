#!/usr/bin/env python3
"""Export normalized standard-Japanese F0 contours from OpenJTalk speech."""

from __future__ import annotations

import argparse
import importlib.util
import json
import math
import sys
from pathlib import Path

import numpy as np
import pyopenjtalk


def load_trainer(root: Path):
    path = root / "tools" / "train-frame-intonation-tcn.py"
    spec = importlib.util.spec_from_file_location("utautts_frame_trainer", path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"cannot load {path}")
    module = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
    return module


def speech_mask(f0: np.ndarray, minimum_pause_frames: int = 8) -> np.ndarray:
    """Keep consonant gaps inside phrases, but split long acoustic pauses."""

    result = np.ones(len(f0), dtype=bool)
    index = 0
    while index < len(f0):
        if f0[index] > 0:
            index += 1
            continue
        end = index + 1
        while end < len(f0) and f0[end] <= 0:
            end += 1
        if end - index >= minimum_pause_frames:
            result[index:end] = False
        index = end
    return result


def normalize_contour(values: np.ndarray, mask: np.ndarray, p99_cents: float, maximum_cents: float) -> np.ndarray:
    valid = mask & (values > 0)
    result = np.zeros(len(values), dtype=np.float64)
    if not valid.any():
        return result
    center = float(np.median(np.log(values[valid])))
    result[mask] = 1200.0 / math.log(2.0) * (np.log(np.maximum(values[mask], 1e-6)) - center)
    observed = float(np.percentile(np.abs(result[mask]), 99))
    if observed > p99_cents:
        result[mask] *= p99_cents / observed
    return np.clip(result, -maximum_cents, maximum_cents)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--corpus", required=True)
    parser.add_argument("--world-engine", dest="worldline", help="path to utautts-world-engine library")
    parser.add_argument("--out", required=True)
    parser.add_argument("--frame-ms", type=float, default=10.0)
    parser.add_argument("--mora-ms", type=float, default=140.0)
    parser.add_argument("--pause-ms", type=float, default=180.0)
    parser.add_argument("--p99-cents", type=float, default=110.0)
    parser.add_argument("--max-cents", type=float, default=140.0)
    args = parser.parse_args()

    root = Path(__file__).resolve().parents[1]
    sys.path.insert(0, str(root / "tools"))
    from openjtalk_features import analyze

    trainer = load_trainer(root)
    worldline = trainer.load_worldline(args.worldline, method=1)
    corpus = json.loads(Path(args.corpus).read_text(encoding="utf-8"))
    cases = []
    for item in corpus["cases"]:
        samples, sample_rate = pyopenjtalk.tts(str(item["text"]))
        f0 = worldline.extract(np.asarray(samples, dtype=np.float64), int(sample_rate), args.frame_ms)
        mask = speech_mask(f0, max(2, int(round(80.0 / args.frame_ms))))
        macro = trainer.macro_log_f0(
            f0, args.frame_ms, speech_mask=mask, max_gap_ms=30.0, smooth_ms=50.0
        )
        cents = normalize_contour(macro, mask, args.p99_cents, args.max_cents)

        reading, tokens = analyze(str(item["text"]))
        target_ms = sum(args.pause_ms if token.get("pause", False) else args.mora_ms for token in tokens)
        target_count = max(2, int(math.ceil(target_ms / args.frame_ms)) + 1)
        source_position = np.linspace(0.0, 1.0, len(cents))
        target_position = np.linspace(0.0, 1.0, target_count)
        warped = np.interp(target_position, source_position, cents)
        cases.append({
            "id": str(item["id"]),
            "text": str(item["text"]),
            "reading": reading,
            "frame_ms": args.frame_ms,
            "cents": [round(float(value), 6) for value in warped],
        })

    output = {
        "version": 1,
        "name": "openjtalk-standard-reference-v1",
        "source": "pyopenjtalk.tts + UtauTTS WORLD Harvest F0",
        "p99_cents": args.p99_cents,
        "max_cents": args.max_cents,
        "cases": cases,
    }
    path = Path(args.out)
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(output, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(f"wrote {len(cases)} OpenJTalk reference contours to {path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
