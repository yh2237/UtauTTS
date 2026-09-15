#!/usr/bin/env python3
"""Train a compact transition residual TCN and export portable JSON."""

from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path

import numpy as np
import torch
from torch import nn


def load(path: Path, bins: int):
    rows = [json.loads(line) for line in path.read_text(encoding="utf-8").splitlines() if line.strip()]
    phones = sorted({r["left_phone"] for r in rows} | {r["right_phone"] for r in rows})
    phone_index = {p: i for i, p in enumerate(phones)}
    xs, ys, ids = [], [], []
    for row in rows:
        left, right = row["left_anchor"], row["right_anchor"]
        base = [left["rms_db"] / 60, right["rms_db"] / 60]
        lf, rf = left.get("f0_hz", 0), right.get("f0_hz", 0)
        base += [np.log(max(lf, 1)) / 7, np.log(max(rf, 1)) / 7, float(lf > 0), float(rf > 0)]
        base += [v / 30 for v in left["spectrum_db"]]
        base += [v / 30 for v in right["spectrum_db"]]
        onehot = [0.0] * (2 * len(phones))
        onehot[phone_index[row["left_phone"]]] = 1
        onehot[len(phones) + phone_index[row["right_phone"]]] = 1
        frames = row["frames"]
        indexes = np.linspace(0, len(frames) - 1, bins).round().astype(int)
        x, y = [], []
        for pos, index in enumerate(indexes):
            p = pos / max(1, bins - 1)
            x.append(base + onehot + [p, p * p, p * p * p])
            frame = frames[index]
            y.append([frame["rms_residual_db"] / 20] + [v / 20 for v in frame["spectrum_residual_db"]])
        xs.append(x)
        ys.append(y)
        ids.append(row["utterance_id"])
    return np.asarray(xs, np.float32), np.asarray(ys, np.float32), ids, phones


class TransitionTCN(nn.Module):
    def __init__(self, inputs: int, hidden: int, outputs: int):
        super().__init__()
        self.input = nn.Conv1d(inputs, hidden, 1)
        self.conv1 = nn.Conv1d(hidden, hidden, 3, padding=1)
        self.conv2 = nn.Conv1d(hidden, hidden, 3, padding=1)
        self.output = nn.Conv1d(hidden, outputs, 1)

    def forward(self, x):
        h = torch.relu(self.input(x))
        h = h + torch.relu(self.conv1(h))
        h = h + torch.relu(self.conv2(h))
        return self.output(h)


def layer(layer):
    return {"weight": layer.weight.detach().cpu().numpy().tolist(), "bias": layer.bias.detach().cpu().numpy().tolist()}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--input", required=True)
    parser.add_argument("--out", required=True)
    parser.add_argument("--bins", type=int, default=15)
    parser.add_argument("--hidden", type=int, default=24)
    parser.add_argument("--epochs", type=int, default=20)
    parser.add_argument("--id", default="jsut-cv-transition-tcn-v1")
    parser.add_argument("--display-name", default="JSUT CV transition TCN v1")
    parser.add_argument("--description", default="JSUT由来の単独音CV境界補正モデル")
    args = parser.parse_args()
    torch.manual_seed(20260915)
    x, y, ids, phones = load(Path(args.input), args.bins)
    validation = np.asarray([hashlib.sha256(i.encode()).digest()[0] % 10 == 0 for i in ids])
    train = ~validation
    if not train.any() or not validation.any():
        raise ValueError("empty train or validation split")
    model = TransitionTCN(x.shape[2], args.hidden, y.shape[2])
    optimizer = torch.optim.AdamW(model.parameters(), lr=2e-3, weight_decay=1e-4)
    tx = torch.from_numpy(x[train]).transpose(1, 2)
    ty = torch.from_numpy(y[train]).transpose(1, 2)
    for epoch in range(args.epochs):
        order = torch.randperm(len(tx))
        model.train()
        for start in range(0, len(order), 128):
            indexes = order[start:start + 128]
            prediction = model(tx[indexes])
            loss = torch.nn.functional.smooth_l1_loss(prediction, ty[indexes])
            optimizer.zero_grad()
            loss.backward()
            optimizer.step()
    model.eval()
    with torch.no_grad():
        vx = torch.from_numpy(x[validation]).transpose(1, 2)
        target = y[validation]
        predicted = model(vx).transpose(1, 2).numpy()
    baseline_spectrum = float(np.abs(target[:, :, 1:] * 20).mean())
    model_spectrum = float(np.abs((target[:, :, 1:] - predicted[:, :, 1:]) * 20).mean())
    baseline_rms = float(np.abs(target[:, :, 0] * 20).mean())
    model_rms = float(np.abs((target[:, :, 0] - predicted[:, :, 0]) * 20).mean())
    payload = {
        "version": 1, "kind": "jsut_cv_transition_tcn", "id": args.id,
        "display_name": args.display_name, "description": args.description,
        "position_bins": args.bins, "phones": phones, "input_size": x.shape[2],
        "hidden_size": args.hidden, "output_size": y.shape[2], "output_scale": 20.0,
        "layers": {"input": layer(model.input), "conv1": layer(model.conv1), "conv2": layer(model.conv2), "output": layer(model.output)},
        "validation": {"examples": int(validation.sum()), "baseline_spectrum_mae": baseline_spectrum,
                       "model_spectrum_mae": model_spectrum, "baseline_rms_mae": baseline_rms, "model_rms_mae": model_rms},
        "provenance": "Generated from JSUT BASIC5000 audio and jsut-label HTS labels; timing is Julius-estimated",
        "data_notice": "licenses/JSUT-DATA-AND-LABELS.txt",
        "license": "UtauTTS policy for JSUT-derived prosody models",
        "license_notice": "licenses/PROSODY-MODELS.txt",
    }
    Path(args.out).parent.mkdir(parents=True, exist_ok=True)
    Path(args.out).write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(f"train={int(train.sum())} validation={int(validation.sum())} spectrum {baseline_spectrum:.4f} -> {model_spectrum:.4f} dB rms {baseline_rms:.4f} -> {model_rms:.4f} dB")


if __name__ == "__main__":
    main()
