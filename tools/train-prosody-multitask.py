#!/usr/bin/env python3
"""Train the mora-duration head of prosody-multitask-v1.

The v1 export combines this learned duration head with an existing v8 frame
pitch head.  Keeping the frame head as an input makes the first v1 dataset
iteration reproducible: duration learning can be evaluated independently,
while the runtime already exposes both outputs through one model file.

Duration targets are utterance-relative log ratios:

    log(observed_mora_ms / baseline_mora_ms)

The Go runtime removes the per-utterance median and exponentiates the result,
so the exported head predicts a stable relative factor rather than an
absolute duration that would ignore the user's tempo setting.
"""

from __future__ import annotations

import argparse
import importlib.util
import json
import math
import random
import sys
from pathlib import Path
from typing import Iterable, Sequence

import torch
from torch import nn
from torch.nn import functional as F


def _load_frame_training_module():
    path = Path(__file__).with_name("train-frame-intonation-tcn.py")
    spec = importlib.util.spec_from_file_location("utautts_frame_training", path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"could not load {path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


frame_training = _load_frame_training_module()
sys.path.insert(0, str(Path(__file__).parent))
from torch_device import device_description, move_batch, resolve_device  # noqa: E402


def load_model(path: str | Path) -> dict:
    with Path(path).open("r", encoding="utf-8") as source:
        model = json.load(source)
    if model.get("version") != 8 or model.get("mode") != "intonation_frame_tcn_accent_bounded":
        raise ValueError("--frame-model must be a frame-intonation-v8 JSON model")
    if not isinstance(model.get("frame_pitch"), dict):
        raise ValueError("--frame-model has no frame_pitch head")
    return model


def baseline_duration(token: dict, base_ms: float, pause_ms: float) -> float:
    if token.get("pause", False):
        return pause_ms
    vowel = str(token.get("vowel", ""))
    if vowel == "cl":
        return base_ms * 0.65
    if vowel == "n":
        return base_ms * 0.90
    mora = str(token.get("mora", ""))
    if mora == "ー":
        return base_ms * 1.20
    return base_ms


def duration_prepare(
    records: Sequence[dict],
    feature_index: dict[str, int],
    base_ms: float,
    pause_ms: float,
) -> list[tuple[list[list[tuple[int, float]]], list[float], list[bool]]]:
    prepared = []
    for record in records:
        sequence = []
        targets = []
        mask = []
        tokens = record["tokens"]
        for position, token in enumerate(tokens):
            sparse = [
                (feature_index[name], float(value))
                for name, value in frame_training.token_features(tokens, position).items()
                if name in feature_index and math.isfinite(float(value))
            ]
            observed = float(token.get("duration_ms", 0.0) or 0.0)
            baseline = baseline_duration(token, base_ms, pause_ms)
            valid = not token.get("pause", False) and observed > 0 and baseline > 0
            sequence.append(sparse)
            targets.append(math.log(max(1e-6, observed / baseline)) if valid else 0.0)
            mask.append(valid)
        if any(mask):
            prepared.append((sequence, targets, mask))
    return prepared


class MoraDurationTCN(nn.Module):
    def __init__(self, inputs: int, hidden: int, dilations: Sequence[int]):
        super().__init__()
        self.input = nn.Linear(inputs, hidden)
        self.layers = nn.ModuleList(
            [nn.Conv1d(hidden, hidden, 3, dilation=int(dilation)) for dilation in dilations]
        )
        self.output = nn.Linear(hidden, 1)
        self.dilations = tuple(int(dilation) for dilation in dilations)

    def forward(self, values: torch.Tensor) -> torch.Tensor:
        state = torch.tanh(self.input(values)).transpose(1, 2)
        for layer, dilation in zip(self.layers, self.dilations):
            convolved = layer(F.pad(state, (dilation, dilation)))
            state = torch.tanh(state + convolved)
        return self.output(state.transpose(1, 2)).squeeze(-1)


def batches(
    records: Sequence[tuple[list[list[tuple[int, float]]], list[float], list[bool]]],
    feature_count: int,
    batch_size: int,
    rng: random.Random,
) -> Iterable[tuple[torch.Tensor, torch.Tensor, torch.Tensor]]:
    order = list(range(len(records)))
    rng.shuffle(order)
    for offset in range(0, len(order), batch_size):
        selected = [records[index] for index in order[offset : offset + batch_size]]
        length = max(len(item[0]) for item in selected)
        values = torch.zeros((len(selected), length, feature_count), dtype=torch.float32)
        targets = torch.zeros((len(selected), length), dtype=torch.float32)
        mask = torch.zeros((len(selected), length), dtype=torch.bool)
        for row, (sequence, expected, valid) in enumerate(selected):
            for position, sparse in enumerate(sequence):
                for column, value in sparse:
                    values[row, position, column] = value
            targets[row, : len(expected)] = torch.tensor(expected, dtype=torch.float32)
            mask[row, : len(valid)] = torch.tensor(valid, dtype=torch.bool)
        yield values, targets, mask


def centered(values: torch.Tensor, mask: torch.Tensor) -> torch.Tensor:
    centers = []
    for row in range(values.shape[0]):
        selected = values[row][mask[row]]
        centers.append(selected.median() if selected.numel() else values[row].new_zeros(()))
    return values - torch.stack(centers).unsqueeze(1)


def sequence_loss(predicted: torch.Tensor, targets: torch.Tensor, mask: torch.Tensor, low: float, high: float) -> torch.Tensor:
    if not bool(mask.any()):
        return predicted.sum() * 0.0
    predicted = centered(predicted, mask)
    targets = centered(targets, mask).clamp(math.log(low), math.log(high))
    absolute = F.smooth_l1_loss(predicted[mask], targets[mask])
    pair_mask = mask[:, 1:] & mask[:, :-1]
    if not bool(pair_mask.any()):
        return absolute
    delta = F.smooth_l1_loss(
        (predicted[:, 1:] - predicted[:, :-1])[pair_mask],
        (targets[:, 1:] - targets[:, :-1])[pair_mask],
    )
    return absolute + 0.35 * delta


@torch.no_grad()
def evaluate(model, records, feature_count, batch_size, low, high, device) -> float:
    model.eval()
    errors = []
    for values, targets, mask in batches(records, feature_count, batch_size, random.Random(0)):
        values, targets, mask = move_batch(device, values, targets, mask)
        predicted = centered(model(values), mask).clamp(math.log(low), math.log(high))
        expected = centered(targets, mask)
        for row in range(values.shape[0]):
            if bool(mask[row].any()):
                errors.extend((predicted[row][mask[row]] - expected[row][mask[row]]).abs().cpu().tolist())
    return float(sum(errors) / len(errors)) if errors else 0.0


def export_duration_model(model, feature_index, args, records, validation_mae: float, frame_model: dict) -> dict:
    feature_names = [None] * len(feature_index)
    for name, index in feature_index.items():
        feature_names[index] = name
    duration_head = {
        "feature_names": feature_names,
        "input_weights": model.input.weight.detach().cpu().double().tolist(),
        "input_bias": model.input.bias.detach().cpu().double().tolist(),
        "layers": [
            {
                "dilation": int(dilation),
                "weights": layer.weight.detach().cpu().double().tolist(),
                "bias": layer.bias.detach().cpu().double().tolist(),
            }
            for dilation, layer in zip(model.dilations, model.layers)
        ],
        "output_weight": model.output.weight.detach().cpu().double().squeeze(0).tolist(),
        "output_bias": float(model.output.bias.detach().cpu()),
        "low": float(args.low_factor),
        "high": float(args.high_factor),
    }
    return {
        "id": args.model_id or Path(args.out).stem,
        "display_name": args.display_name or "Prosody multitask v1",
        "description": args.description or "Learned Japanese mora duration with v8 frame intonation",
        "license": frame_model.get("license", "UTAUTTS-JSUT-DERIVED-MODEL-TERMS-1.0"),
        "license_notice": frame_model.get("license_notice", "licenses/PROSODY-MODELS.txt"),
        "recommended_renderers": args.recommended_renderer or frame_model.get("recommended_renderers", []),
        "version": 10,
        "feature_version": 2,
        "mode": "prosody_multitask_tcn",
        "outputs": {"pitch": True, "mora_duration": True},
        "duration_weights": {},
        "mora_duration": duration_head,
        "frame_pitch": frame_model["frame_pitch"],
        "metrics": {
            "records": len(records),
            "tokens": sum(len(record["tokens"]) for record in records),
            "duration_log_ratio_mae": validation_mae,
        },
        "training": {
            "records": len(records),
            "tokens": sum(len(record["tokens"]) for record in records),
            "epochs": args.epochs,
            "learning_rate": args.learning_rate,
            "seed": args.seed,
        },
        "provenance": frame_model.get("provenance"),
    }


def main(argv: Sequence[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("dataset")
    parser.add_argument("--frame-model", default="models/frame-intonation-v8.json", help="existing frame-intonation-v8 JSON")
    parser.add_argument("--out", default="out/prosody/prosody-multitask-v1.json")
    parser.add_argument("--model-id", default="prosody-multitask-v1")
    parser.add_argument("--display-name", default="Prosody multitask v1")
    parser.add_argument("--description", default="Learned Japanese mora duration with v8 frame intonation")
    parser.add_argument("--recommended-renderer", action="append", default=[])
    parser.add_argument("--epochs", type=int, default=24)
    parser.add_argument("--learning-rate", type=float, default=0.002)
    parser.add_argument("--hidden", type=int, default=24)
    parser.add_argument("--batch-size", type=int, default=16)
    parser.add_argument("--device", default="auto")
    parser.add_argument("--limit", type=int, default=0)
    parser.add_argument("--seed", type=int, default=1)
    parser.add_argument("--base-duration-ms", type=float, default=140.0)
    parser.add_argument("--pause-duration-ms", type=float, default=180.0)
    parser.add_argument("--low-factor", type=float, default=0.55)
    parser.add_argument("--high-factor", type=float, default=1.8)
    accent = parser.add_mutually_exclusive_group()
    accent.add_argument("--openjtalk-accent", dest="openjtalk_accent", action="store_true")
    accent.add_argument("--no-openjtalk-accent", dest="openjtalk_accent", action="store_false")
    parser.set_defaults(openjtalk_accent=True)
    args = parser.parse_args(argv)
    if args.epochs < 0 or args.hidden <= 0 or args.batch_size <= 0:
        parser.error("epochs, hidden, and batch-size must be valid positive values")
    if args.base_duration_ms <= 0 or args.pause_duration_ms <= 0 or not 0 < args.low_factor < args.high_factor:
        parser.error("invalid duration baseline or factor bounds")

    random.seed(args.seed)
    torch.manual_seed(args.seed)
    device = resolve_device(args.device)
    print(f"training device: {device_description(device)}")

    frame_model = load_model(args.frame_model)
    train_raw, validation_raw = frame_training.load_records(args.dataset, args.limit)
    train_raw = frame_training.add_openjtalk_features(train_raw, args.openjtalk_accent, min_alignment_rate=0.0)
    validation_raw = frame_training.add_openjtalk_features(validation_raw, args.openjtalk_accent, min_alignment_rate=0.0)
    if not train_raw or not validation_raw:
        raise ValueError("Open JTalk alignment removed every utterance from a train or validation split")

    all_records = train_raw + validation_raw
    feature_index = frame_training.mora_feature_index(all_records)
    train = duration_prepare(train_raw, feature_index, args.base_duration_ms, args.pause_duration_ms)
    validation = duration_prepare(validation_raw, feature_index, args.base_duration_ms, args.pause_duration_ms)
    if not train or not validation:
        raise ValueError("dataset has no usable mora duration targets")

    model = MoraDurationTCN(len(feature_index), args.hidden, (1, 2, 4, 8)).to(device)
    optimizer = torch.optim.AdamW(model.parameters(), lr=args.learning_rate, weight_decay=1e-5)
    rng = random.Random(args.seed)
    for epoch in range(args.epochs):
        model.train()
        total = 0.0
        count = 0
        for values, targets, mask in batches(train, len(feature_index), args.batch_size, rng):
            values, targets, mask = move_batch(device, values, targets, mask)
            optimizer.zero_grad()
            loss = sequence_loss(model(values), targets, mask, args.low_factor, args.high_factor)
            loss.backward()
            torch.nn.utils.clip_grad_norm_(model.parameters(), 1.0)
            optimizer.step()
            total += float(loss.detach().cpu())
            count += 1
        print(f"epoch {epoch + 1:02d}/{args.epochs}: loss={total / max(1, count):.6f}")

    validation_mae = evaluate(model, validation, len(feature_index), args.batch_size, args.low_factor, args.high_factor, device)
    exported = export_duration_model(model, feature_index, args, all_records, validation_mae, frame_model)
    output = Path(args.out)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(exported, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(f"wrote {output} ({len(train)} train/{len(validation)} validation records, {validation_mae:.6f} log-ratio MAE)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
