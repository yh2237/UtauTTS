"""Train the v9.1 phrase-anchor residual intonation model.

This trainer reuses the v8 dataset/F0 frontend, but reduces every accent
phrase to four residual controls: phrase start, accent nucleus, phrase end,
and question/boundary rise.  The exported model is consumed by the Go
runtime and expanded to a smooth frame curve there.

v9.1 tightens the teacher signal: Open JTalk alignment is strict by default,
octave flips in the measured F0 are unwrapped before interpolation, and the
target is a smoothed log-F0 residual rather than a clipped raw frame track.
"""

from __future__ import annotations

import argparse
import importlib.util
import json
import math
from pathlib import Path

import numpy as np


ANCHORS = ("start", "nucleus", "end", "question")


def load_frame_trainer():
    path = Path(__file__).with_name("train-frame-intonation-tcn.py")
    spec = importlib.util.spec_from_file_location("frame_trainer_v8", path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"cannot load {path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def is_question(record: dict) -> bool:
    text = str(record.get("text", ""))
    return "?" in text or "？" in text


def accent_base(token: dict, accent_range: float, declination: float) -> float:
    value = 0.5 * accent_range if token.get("accent_high", False) else -0.5 * accent_range
    position = float(token.get("accent_position", 0.0))
    return value - declination * min(1.0, max(0.0, position))


def phrase_ranges(record: dict, token_indices: np.ndarray, trainer) -> list[list[int]]:
    ranges: list[list[int]] = []
    current: list[int] = []
    for frame, raw_index in enumerate(token_indices):
        index = int(raw_index)
        if index < 0 or record["tokens"][index].get("pause", False):
            if current:
                ranges.append(current)
                current = []
            continue
        starts = bool(record["tokens"][index].get("accent_phrase_start", index == 0))
        if current and starts:
            ranges.append(current)
            current = []
        current.append(frame)
    if current:
        ranges.append(current)
    return ranges


def median_or_zero(values: list[float]) -> float:
    return float(np.median(np.asarray(values, dtype=np.float64))) if values else 0.0


def unwrap_octaves(f0: np.ndarray, speech: np.ndarray) -> np.ndarray:
    """Remove isolated WORLD octave errors without flattening real movement.

    Adjacent voiced frames in ordinary Japanese speech do not jump an octave.
    For each speech island we therefore choose the octave-equivalent candidate
    closest to the previous voiced estimate.  Zero/unvoiced frames are kept
    untouched and are filled later by ``macro_log_f0``.
    """

    values = np.asarray(f0, dtype=np.float64)
    result = values.copy()
    mask = np.asarray(speech, dtype=bool)
    index = 0
    while index < len(values):
        if not mask[index]:
            index += 1
            continue
        end = index + 1
        while end < len(values) and mask[end]:
            end += 1
        valid = [position for position in range(index, end) if values[position] > 0]
        if len(valid) >= 2:
            seed_count = min(5, len(valid))
            previous = float(np.median(values[np.asarray(valid[:seed_count], dtype=np.int64)]))
            for position in valid:
                source = float(values[position])
                candidates = [source * (2.0 ** octave) for octave in range(-2, 3)]
                chosen = min(candidates, key=lambda candidate: abs(1200.0 * math.log2(candidate / previous)))
                result[position] = chosen
                previous = chosen
        index = end
    return result


def build_examples(
    records,
    trainer,
    dataset_path,
    audio_root,
    frame_ms,
    accent_range,
    declination,
    worldline,
    teacher_smoothing_ms,
):
    examples: list[tuple[dict[str, float], list[float]]] = []
    feature_names: set[str] = set()
    for record in records:
        times, f0, token_indices = trainer.extract_record_f0(
            record,
            dataset_path=dataset_path,
            audio_root=audio_root,
            frame_ms=frame_ms,
            worldline=worldline,
        )
        speech = np.asarray(
            [0 <= int(index) < len(record["tokens"]) and not record["tokens"][int(index)].get("pause", False) for index in token_indices],
            dtype=bool,
        )
        f0 = unwrap_octaves(f0, speech)
        macro = trainer.macro_log_f0(
            f0,
            frame_ms,
            speech_mask=speech,
            smooth_ms=teacher_smoothing_ms,
        )
        valid = speech & (macro > 0)
        if not valid.any():
            continue
        target = trainer._record_target_cents(macro, valid, -4800.0, 4800.0)
        for frames in phrase_ranges(record, token_indices, trainer):
            if len(frames) < 2:
                continue
            token_ids = sorted({int(token_indices[frame]) for frame in frames})
            token_ids = [index for index in token_ids if 0 <= index < len(record["tokens"]) and not record["tokens"][index].get("pause", False)]
            if not token_ids:
                continue
            vector: dict[str, float] = {}
            for index in token_ids:
                for name, value in trainer.token_features(record["tokens"], index).items():
                    vector[name] = vector.get(name, 0.0) + float(value)
            for name in vector:
                vector[name] /= len(token_ids)
                feature_names.add(name)

            start, end = frames[0], frames[-1]
            span = float(max(1, end - start))
            residual: list[float] = []
            progress: list[float] = []
            for frame in frames:
                index = int(token_indices[frame])
                if not valid[frame]:
                    continue
                residual.append(float(target[frame] - accent_base(record["tokens"][index], accent_range, declination)))
                progress.append((frame - start) / span)
            if len(residual) < 2:
                continue
            residual_array = np.asarray(residual)
            progress_array = np.asarray(progress)
            nucleus = 0.5
            nucleus_positions = [float(record["tokens"][index].get("accent_nucleus_position", 0.0)) for index in token_ids]
            nucleus_positions = [value for value in nucleus_positions if value > 0]
            if nucleus_positions:
                nucleus = min(0.95, max(0.05, float(np.median(nucleus_positions))))
            start_values = residual_array[progress_array <= 0.2]
            if len(start_values) == 0:
                start_values = residual_array[:1]
            nucleus_values = residual_array[np.abs(progress_array - nucleus) <= 0.12]
            if len(nucleus_values) == 0:
                nucleus_values = residual_array[[int(np.argmin(np.abs(progress_array - nucleus)))]]
            end_values = residual_array[progress_array >= 0.8]
            if len(end_values) == 0:
                end_values = residual_array[-1:]
            previous_values = residual_array[(progress_array >= 0.45) & (progress_array <= 0.7)]
            if len(previous_values) == 0:
                previous_values = residual_array[max(0, len(residual_array) // 2 - 1) : max(1, len(residual_array) // 2)]
            question = median_or_zero(end_values.tolist()) - median_or_zero(previous_values.tolist()) if is_question(record) else 0.0
            examples.append(
                (
                    vector,
                    [
                        median_or_zero(start_values.tolist()),
                        median_or_zero(nucleus_values.tolist()),
                        median_or_zero(end_values.tolist()),
                        question,
                    ],
                )
            )
    return examples, sorted(feature_names)


def fit_ridge(examples, feature_names, regularization: float):
    matrix = np.zeros((len(examples), len(feature_names)), dtype=np.float64)
    targets = np.zeros((len(examples), 4), dtype=np.float64)
    columns = {name: index for index, name in enumerate(feature_names)}
    for row, (features, target) in enumerate(examples):
        for name, value in features.items():
            matrix[row, columns[name]] = value
        targets[row] = target
    design = np.concatenate([np.ones((len(matrix), 1)), matrix], axis=1)
    penalty = np.eye(design.shape[1], dtype=np.float64) * regularization
    penalty[0, 0] = 0.0
    coefficients = np.linalg.solve(design.T @ design + penalty, design.T @ targets)
    return coefficients[1:].T, coefficients[0]


def evaluate(examples, feature_names, weights, bias):
    columns = {name: index for index, name in enumerate(feature_names)}
    errors = []
    for features, target in examples:
        vector = np.zeros(len(feature_names), dtype=np.float64)
        for name, value in features.items():
            if name in columns:
                vector[columns[name]] = value
        errors.extend(np.abs(weights @ vector + bias - np.asarray(target)).tolist())
    return float(np.mean(errors)) if errors else 0.0


def main(argv=None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dataset", required=True)
    parser.add_argument("--out", default="out/prosody/intonation-phrase-anchor-v9-1.json")
    parser.add_argument("--model-id", default="", help="stable plugin ID stored in the model")
    parser.add_argument("--display-name", default="", help="user-facing model name")
    parser.add_argument("--description", default="", help="user-facing model description")
    parser.add_argument("--recommended-renderer", action="append", default=[], help="compatible renderer ID; repeatable")
    parser.add_argument("--limit", type=int, default=0)
    parser.add_argument("--frame-ms", type=float, default=10.0)
    parser.add_argument("--teacher-smoothing-ms", type=float, default=40.0)
    parser.add_argument("--accent-range-cents", type=float, default=60.0)
    parser.add_argument("--declination-cents", type=float, default=20.0)
    parser.add_argument("--smoothing-ms", type=float, default=30.0)
    parser.add_argument("--p99-cents", type=float, default=75.0)
    parser.add_argument("--max-cents", type=float, default=90.0)
    parser.add_argument("--regularization", type=float, default=10.0)
    parser.add_argument("--worldline")
    parser.add_argument("--f0-method", type=int, default=1)
    parser.add_argument("--audio-root")
    parser.add_argument("--no-openjtalk-accent", action="store_true")
    parser.add_argument("--min-alignment-rate", type=float, default=0.60)
    args = parser.parse_args(argv)
    if (
        args.frame_ms <= 0
        or args.teacher_smoothing_ms <= 0
        or args.accent_range_cents <= 0
        or args.max_cents <= 0
        or args.p99_cents <= 0
        or not 0 <= args.min_alignment_rate <= 1
    ):
        parser.error("frame/safety values must be positive and alignment rate must be in 0..1")

    trainer = load_frame_trainer()
    train_raw, validation_raw = trainer.load_records(args.dataset, args.limit)
    alignment: dict = {}
    train_alignment: dict = {}
    validation_alignment: dict = {}
    train_raw = trainer.add_openjtalk_features(
        train_raw,
        not args.no_openjtalk_accent,
        train_alignment,
        min_alignment_rate=0.0,
    )
    validation_raw = trainer.add_openjtalk_features(
        validation_raw,
        not args.no_openjtalk_accent,
        validation_alignment,
        min_alignment_rate=0.0,
    )
    alignment.update({"train": train_alignment, "validation": validation_alignment})
    total_records = train_alignment.get("alignment_records", 0) + validation_alignment.get("alignment_records", 0)
    total_input_records = len(train_raw) + train_alignment.get("skipped_records", 0) + len(validation_raw) + validation_alignment.get("skipped_records", 0)
    overall_alignment_rate = total_records / max(1, total_input_records)
    if not args.no_openjtalk_accent and overall_alignment_rate < args.min_alignment_rate:
        raise ValueError(
            f"Open JTalk alignment rate {overall_alignment_rate:.3f} is below minimum "
            f"{args.min_alignment_rate:.3f}"
        )
    if not train_raw or not validation_raw:
        raise ValueError("Open JTalk alignment removed every training or validation record")
    worldline = trainer.load_worldline(args.worldline, args.f0_method)
    train, feature_names = build_examples(
        train_raw, trainer, args.dataset, args.audio_root, args.frame_ms,
        args.accent_range_cents, args.declination_cents, worldline,
        args.teacher_smoothing_ms,
    )
    validation, _ = build_examples(
        validation_raw, trainer, args.dataset, args.audio_root, args.frame_ms,
        args.accent_range_cents, args.declination_cents, worldline,
        args.teacher_smoothing_ms,
    )
    if not train:
        raise ValueError("no phrase examples were created")
    weights, bias = fit_ridge(train, feature_names, args.regularization)
    output = {
        "id": str(args.model_id or Path(args.out).stem),
        "display_name": str(args.display_name or Path(args.out).stem),
        "description": str(args.description or "Phrase-anchor intonation model"),
        "recommended_renderers": list(args.recommended_renderer or ["utautts-world-phrase"]),
        "version": 9,
        "feature_version": 1,
        "mode": "intonation_phrase_anchor_v9_1",
        "duration_weights": {},
        "phrase_pitch": {
            "feature_names": feature_names,
            "weights": weights.tolist(),
            "bias": bias.tolist(),
            "frame_ms": args.frame_ms,
            "teacher_smoothing_ms": args.teacher_smoothing_ms,
            "low_cents": -250.0,
            "high_cents": 250.0,
            "accent_range_cents": args.accent_range_cents,
            "declination_cents": args.declination_cents,
            "smoothing_ms": args.smoothing_ms,
            "p99_cents": args.p99_cents,
            "max_cents": args.max_cents,
        },
        "metrics": {
            "records": len(validation_raw),
            "phrase_examples": len(validation),
            "phrase_anchor_mae_cents": evaluate(validation, feature_names, weights, bias),
        },
        "training": {
            "records": len(train_raw),
            "phrase_examples": len(train),
            "regularization": args.regularization,
            "f0_source": "worldline" if worldline is not None else "internal_autocorrelation",
            "openjtalk_accent": not args.no_openjtalk_accent,
            "min_alignment_rate": args.min_alignment_rate,
            "overall_alignment_rate": overall_alignment_rate,
            "alignment": alignment,
        },
    }
    output_path = Path(args.out)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(json.dumps(output, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(f"wrote {args.out} ({len(train)} train/{len(validation)} validation phrases, {output['metrics']['phrase_anchor_mae_cents']:.1f} cents MAE)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
