#!/usr/bin/env python3
"""モーラ長のTCNを学習し、既存の抑揚モデル（frame_pitch）と組み合わせたGo推論用JSONを出力する。

教師は合成時と同じノート区間（母音の始まり〜次の母音の始まり）のモーラ長。強制アラインメント
（tools/align-intonation-mfa.py）で作ったJSONLを使う。出力は発話ごとの中央値に対する倍率で、
基準長は合成（plan.durationFor）と同じく撥音0.9倍・長音1.2倍・それ以外1.0倍。
推論時の倍率はlow〜highに制限し、句末（休止・文末の直前）はphrase_final_low以上、
文頭・休止の直後はphrase_start_high以下にする。
"""

from __future__ import annotations

import argparse
import importlib.util
import json
import math
import random
import sys
from pathlib import Path

import numpy as np
import torch
from torch import nn
from torch.nn import functional as F

TOOLS = Path(__file__).resolve().parent
sys.path.insert(0, str(TOOLS))
_spec = importlib.util.spec_from_file_location("frame_trainer", TOOLS / "train-frame-intonation-tcn.py")
trainer = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(trainer)


def baseline_factor(token: dict) -> float:
    if str(token.get("vowel", "")) == "n":
        return 0.9
    if str(token.get("mora", "")) == "ー":
        return 1.2
    return 1.0


def prepare(records, feature_index):
    prepared = []
    for record in records:
        tokens = record["tokens"]
        sequence, logs, mask = [], [], []
        for position, token in enumerate(tokens):
            sequence.append([(feature_index[name], float(value)) for name, value in trainer.token_features(tokens, position).items()
                             if name in feature_index and math.isfinite(float(value))])
            duration = float(token.get("end_ms", 0)) - float(token.get("start_ms", 0))
            valid = not token.get("pause") and duration > 0
            logs.append(math.log(duration / (120.0 * baseline_factor(token))) if valid else 0.0)
            mask.append(valid)
        if any(mask):
            prepared.append((sequence, logs, mask))
    return prepared


class MoraDurationTCN(nn.Module):
    def __init__(self, inputs, hidden, dilations):
        super().__init__()
        self.input = nn.Linear(inputs, hidden)
        self.layers = nn.ModuleList([nn.Conv1d(hidden, hidden, 3, dilation=int(d)) for d in dilations])
        self.output = nn.Linear(hidden, 1)
        self.dilations = tuple(int(d) for d in dilations)

    def forward(self, values):
        state = torch.tanh(self.input(values)).transpose(1, 2)
        for layer, dilation in zip(self.layers, self.dilations):
            state = torch.tanh(state + layer(F.pad(state, (dilation, dilation))))
        return self.output(state.transpose(1, 2)).squeeze(-1)


def batches(records, feature_count, batch_size, rng):
    order = list(range(len(records)))
    rng.shuffle(order)
    for offset in range(0, len(order), batch_size):
        selected = [records[i] for i in order[offset:offset + batch_size]]
        length = max(len(item[0]) for item in selected)
        values = torch.zeros((len(selected), length, feature_count))
        targets = torch.zeros((len(selected), length))
        mask = torch.zeros((len(selected), length), dtype=torch.bool)
        for row, (sequence, expected, valid) in enumerate(selected):
            for position, sparse in enumerate(sequence):
                for column, value in sparse:
                    values[row, position, column] = value
            targets[row, :len(expected)] = torch.tensor(expected)
            mask[row, :len(valid)] = torch.tensor(valid)
        yield values, targets, mask


def centered(values, mask):
    centers = [values[row][mask[row]].median() if mask[row].any() else values.new_zeros(()) for row in range(values.shape[0])]
    return values - torch.stack(centers).unsqueeze(1)


def loss_fn(predicted, targets, mask, low, high):
    predicted = centered(predicted, mask)
    targets = centered(targets, mask).clamp(math.log(low), math.log(high))
    absolute = F.smooth_l1_loss(predicted[mask], targets[mask], beta=0.1)
    pairs = mask[:, 1:] & mask[:, :-1]
    delta = F.smooth_l1_loss((predicted[:, 1:] - predicted[:, :-1])[pairs], (targets[:, 1:] - targets[:, :-1])[pairs], beta=0.1)
    return absolute + 0.35 * delta


@torch.no_grad()
def evaluate(model, records, feature_count, low, high, device):
    model.eval()
    errors, predicted_all, expected_all = [], [], []
    for values, targets, mask in batches(records, feature_count, 32, random.Random(0)):
        values, targets, mask = values.to(device), targets.to(device), mask.to(device)
        predicted = centered(model(values), mask).clamp(math.log(low), math.log(high))
        expected = centered(targets, mask).clamp(math.log(low), math.log(high))
        errors.append((predicted - expected)[mask].abs().cpu())
        predicted_all.append(predicted[mask].cpu())
        expected_all.append(expected[mask].cpu())
    errors = torch.cat(errors)
    predicted_all, expected_all = torch.cat(predicted_all).numpy(), torch.cat(expected_all).numpy()
    return {"log_mae": float(errors.mean()), "pred_std": float(predicted_all.std()), "target_std": float(expected_all.std()),
            "corr": float(np.corrcoef(predicted_all, expected_all)[0, 1]), "moras": int(len(predicted_all))}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dataset", required=True, help="MFAで整列したJSONL（tokensにstart_ms/end_ms）")
    parser.add_argument("--frame-model", required=True, help="組み合わせる抑揚モデルJSON（frame_pitchとライセンス表記を引き継ぐ）")
    parser.add_argument("--out", required=True)
    parser.add_argument("--id", required=True)
    parser.add_argument("--display-name", required=True)
    parser.add_argument("--description", default="")
    parser.add_argument("--default-priority", type=int, default=0)
    parser.add_argument("--epochs", type=int, default=40)
    parser.add_argument("--hidden", type=int, default=32)
    parser.add_argument("--dilations", default="1,2,4,8")
    parser.add_argument("--learning-rate", type=float, default=0.002)
    parser.add_argument("--low-factor", type=float, default=0.5)
    parser.add_argument("--high-factor", type=float, default=2.0)
    parser.add_argument("--phrase-final-low", type=float, default=1.0)
    parser.add_argument("--phrase-start-high", type=float, default=1.25)
    parser.add_argument("--exclude", default="", help="学習から外してテストに入れるID（カンマ区切り）")
    parser.add_argument("--seed", type=int, default=1)
    args = parser.parse_args()
    sys.stdout.reconfigure(encoding="utf-8")
    random.seed(args.seed)
    torch.manual_seed(args.seed)
    device = torch.device("cuda" if torch.cuda.is_available() else "cpu")

    excluded = set(filter(None, args.exclude.split(",")))
    train_raw, validation_raw = trainer.load_records(args.dataset, 0)
    held = lambda record: trainer.fnv1a(str(record["id"])) % 10 == 1 or str(record["id"]) in excluded  # noqa: E731
    test_raw = [r for r in train_raw + validation_raw if held(r)]
    train_raw = [r for r in train_raw if not held(r)]
    validation_raw = [r for r in validation_raw if not held(r)]
    train_raw, validation_raw, test_raw = (trainer.add_openjtalk_features(records, True, {}, min_alignment_rate=0.0)
                                           for records in (train_raw, validation_raw, test_raw))
    feature_index = trainer.mora_feature_index(train_raw + validation_raw)
    splits = {name: prepare(records, feature_index) for name, records in (("train", train_raw), ("validation", validation_raw), ("test", test_raw))}
    dilations = [int(v) for v in args.dilations.split(",")]
    model = MoraDurationTCN(len(feature_index), args.hidden, dilations).to(device)
    optimizer = torch.optim.AdamW(model.parameters(), lr=args.learning_rate, weight_decay=1e-5)
    rng = random.Random(args.seed)
    best, best_state, best_epoch = None, None, 0
    for epoch in range(args.epochs):
        model.train()
        for values, targets, mask in batches(splits["train"], len(feature_index), 16, rng):
            values, targets, mask = values.to(device), targets.to(device), mask.to(device)
            optimizer.zero_grad()
            loss = loss_fn(model(values), targets, mask, args.low_factor, args.high_factor)
            loss.backward()
            torch.nn.utils.clip_grad_norm_(model.parameters(), 1.0)
            optimizer.step()
        metrics = evaluate(model, splits["validation"], len(feature_index), args.low_factor, args.high_factor, device)
        if best is None or metrics["log_mae"] < best["log_mae"]:
            best, best_epoch = metrics, epoch + 1
            best_state = {key: value.detach().cpu().clone() for key, value in model.state_dict().items()}
        print(f"epoch {epoch + 1:02d}: validation {metrics}", flush=True)
    model.load_state_dict(best_state)
    test = evaluate(model, splits["test"], len(feature_index), args.low_factor, args.high_factor, device)
    print(f"best epoch {best_epoch}: validation {best} / test {test}", flush=True)

    names = [None] * len(feature_index)
    for name, index in feature_index.items():
        names[index] = name
    model = model.cpu()
    frame = json.loads(Path(args.frame_model).read_text(encoding="utf-8"))
    exported = {
        "id": args.id, "display_name": args.display_name, "description": args.description,
        "license": frame.get("license", ""), "license_notices": frame.get("license_notices", []),
        "provenance": {"pitch": frame.get("provenance"), "pitch_model": frame.get("id"),
                       "mora_duration": {"dataset": Path(args.dataset).name, "target": "note interval (vowel onset to next vowel onset)"}},
        "recommended_renderers": frame.get("recommended_renderers", ["utautts-world-phrase"]),
        "version": 10, "feature_version": 2, "mode": "prosody_multitask_tcn", "language": frame.get("language", "ja"),
        "outputs": {"pitch": True, "mora_duration": True}, "duration_weights": {},
        "mora_duration": {
            "feature_names": names,
            "input_weights": model.input.weight.detach().double().tolist(),
            "input_bias": model.input.bias.detach().double().tolist(),
            "layers": [{"dilation": d, "weights": layer.weight.detach().double().tolist(), "bias": layer.bias.detach().double().tolist()}
                       for d, layer in zip(model.dilations, model.layers)],
            "output_weight": model.output.weight.detach().double().squeeze(0).tolist(),
            "output_bias": float(model.output.bias.detach()),
            "low": args.low_factor, "high": args.high_factor,
            "phrase_final_low": args.phrase_final_low, "phrase_start_high": args.phrase_start_high,
        },
        "frame_pitch": frame["frame_pitch"],
        "metrics": {"mora_duration": {"validation": best, "test": test, "best_epoch": best_epoch}, "pitch": frame.get("metrics")},
        "training": {"mora_duration": {"records": len(splits["train"]), "epochs": args.epochs, "hidden": args.hidden, "dilations": dilations,
                                       "learning_rate": args.learning_rate, "seed": args.seed, "excluded": sorted(excluded),
                                       "baseline": "plan.durationFor (n 0.9, long vowel 1.2, else 1.0)"}},
    }
    if args.default_priority:
        exported["default_priority"] = args.default_priority
    Path(args.out).write_text(json.dumps(exported, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print("wrote", args.out)


if __name__ == "__main__":
    main()
