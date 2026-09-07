#!/usr/bin/env python3
"""GUIで採用した抑揚調整からGo互換の補正モデルを学習する。"""

from __future__ import annotations

import argparse
import copy
import hashlib
import json
import math
import random
import subprocess
from dataclasses import dataclass
from pathlib import Path

import numpy as np
import torch
from torch import nn
from torch.nn import functional as F


LIMIT_CENTS = 120.0


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def normalized_group(record: dict) -> str:
    text = "".join(str(record.get("text", "")).split())
    reading = "".join(str(record.get("reading", "")).split())
    return hashlib.sha256(f"{text}\n{reading}".encode("utf-8")).hexdigest()


def load_records(paths: list[Path], base_hash: str) -> list[dict]:
    latest: dict[str, dict] = {}
    for path in paths:
        with path.open("r", encoding="utf-8-sig") as source:
            for line_number, line in enumerate(source, 1):
                if not line.strip():
                    continue
                try:
                    record = json.loads(line)
                except json.JSONDecodeError as error:
                    raise ValueError(f"{path}:{line_number}: invalid JSON: {error}") from error
                if record.get("version") != 1 or record.get("status") != "accepted" or not record.get("accepted"):
                    continue
                count = len(record.get("morae", []))
                for name in ("features", "base_points_cents", "manual_offsets_cents", "edit_mask"):
                    if len(record.get(name, [])) != count:
                        raise ValueError(f"{path}:{line_number}: {name} length mismatch")
                if record.get("base_model", {}).get("sha256") != base_hash:
                    raise ValueError(f"{path}:{line_number}: base model hash mismatch")
                latest[normalized_group(record)] = record
    records = list(latest.values())
    if len(records) < 3:
        raise ValueError("at least three accepted, distinct prompts are required")
    frontends = {
        (record.get("frontend", {}).get("feature_version"), record.get("frontend", {}).get("dictionary_fingerprint"))
        for record in records
    }
    if len(frontends) != 1:
        raise ValueError("datasets use different frontend or dictionary fingerprints")
    return records


def deterministic_split(records: list[dict]) -> dict[str, list[dict]]:
    ordered = sorted(records, key=lambda item: normalized_group(item))
    bucket = lambda item: int(normalized_group(item)[:8], 16) % 10
    test = [record for record in ordered if bucket(record) == 0]
    validation = [record for record in ordered if bucket(record) == 1]
    train = [record for record in ordered if bucket(record) >= 2]
    if not test:
        candidate = next((record for record in ordered if record not in validation), ordered[0])
        test.append(candidate)
        if candidate in validation:
            validation.remove(candidate)
        if candidate in train:
            train.remove(candidate)
    if not validation:
        candidate = next((record for record in ordered if record not in test), ordered[-1])
        validation.append(candidate)
        if candidate in test:
            test.remove(candidate)
        if candidate in train:
            train.remove(candidate)
    if not train:
        source = validation if len(validation) > 1 else test
        train.append(source.pop())
    return {"train": train, "validation": validation, "test": test}


def phrase_ids(record: dict) -> list[int]:
    result: list[int] = []
    phrase = -1
    morae, frames = record["morae"], record["features"]
    for index, mora in enumerate(morae):
        if mora.get("pause", False):
            result.append(-1)
            continue
        start = index == 0 or morae[index - 1].get("pause", False)
        if float(frames[index].get("accent_phrase_start", 0)) > 0:
            start = True
        if start:
            phrase += 1
        result.append(phrase)
    return result


def add_categorical(result: dict[str, float], prefix: str, mora: dict | None, edge: str = "") -> None:
    if mora is None:
        result[f"{prefix}=<{edge}>"] = 1.0
    elif mora.get("pause", False):
        result[f"{prefix}=<PAUSE>"] = 1.0
    else:
        result[f"{prefix}={mora.get('mora', '')}"] = 1.0
        result[f"{prefix}_vowel={mora.get('vowel', '')}"] = 1.0


def record_features(record: dict) -> tuple[list[dict[str, float]], list[int]]:
    morae = record["morae"]
    base = [float(value) for value in record["base_points_cents"]]
    ids = phrase_ids(record)
    ranges: dict[int, tuple[float, float]] = {}
    for index, phrase in enumerate(ids):
        if phrase < 0:
            continue
        low, high = ranges.get(phrase, (base[index], base[index]))
        ranges[phrase] = min(low, base[index]), max(high, base[index])
    denominator = max(1, len(morae) - 1)
    result: list[dict[str, float]] = []
    for index, mora in enumerate(morae):
        position = index / denominator
        values: dict[str, float] = {
            "bias": 1.0,
            "position": position,
            "position2": position * position,
            "from_end": 1.0 - position,
        }
        if index == 0 or morae[index - 1].get("pause", False):
            values["phrase_start"] = 1.0
        if index + 1 == len(morae) or morae[index + 1].get("pause", False):
            values["phrase_end"] = 1.0
        add_categorical(values, "mora", mora)
        add_categorical(values, "prev", morae[index - 1] if index > 0 else None, "BOS")
        add_categorical(values, "next", morae[index + 1] if index + 1 < len(morae) else None, "EOS")
        values.update({name: float(value) for name, value in record["features"][index].items()})
        previous = base[index - 1] if index > 0 and ids[index - 1] == ids[index] else base[index]
        following = base[index + 1] if index + 1 < len(base) and ids[index + 1] == ids[index] else base[index]
        values.update(
            {
                "base_pitch_cents": base[index] / LIMIT_CENTS,
                "base_prev_cents": previous / LIMIT_CENTS,
                "base_next_cents": following / LIMIT_CENTS,
                "base_delta_prev": (base[index] - previous) / LIMIT_CENTS,
                "base_delta_next": (following - base[index]) / LIMIT_CENTS,
                "base_second_difference": (following - 2 * base[index] + previous) / LIMIT_CENTS,
                "base_near_render_limit": abs(base[index]) / 90.0,
            }
        )
        if ids[index] >= 0:
            low, high = ranges[ids[index]]
            values["base_phrase_min"] = low / LIMIT_CENTS
            values["base_phrase_max"] = high / LIMIT_CENTS
            values["base_phrase_range"] = (high - low) / LIMIT_CENTS
        result.append(values)
    return result, ids


@dataclass
class PreparedRecord:
    record: dict
    values: torch.Tensor
    targets: torch.Tensor
    weights: torch.Tensor
    edited: torch.Tensor
    speech: torch.Tensor
    phrases: torch.Tensor


def feature_vocabulary(records: list[dict]) -> list[str]:
    names: set[str] = set()
    for record in records:
        rows, _ = record_features(record)
        for row in rows:
            names.update(row)
    return sorted(names)


def prepare(record: dict, feature_index: dict[str, int]) -> PreparedRecord:
    rows, phrases = record_features(record)
    values = torch.zeros((len(rows), len(feature_index)), dtype=torch.float32)
    for row_index, row in enumerate(rows):
        for name, value in row.items():
            column = feature_index.get(name)
            if column is not None and value:
                values[row_index, column] = value
    speech = torch.tensor([not mora.get("pause", False) for mora in record["morae"]], dtype=torch.bool)
    edited = torch.tensor(record["edit_mask"], dtype=torch.bool) & speech
    targets = torch.tensor(
        [max(-LIMIT_CENTS, min(LIMIT_CENTS, float(value))) / LIMIT_CENTS for value in record["manual_offsets_cents"]],
        dtype=torch.float32,
    )
    weights = torch.where(edited, 1.0, 0.15) * speech.float()
    return PreparedRecord(record, values, targets, weights, edited, speech, torch.tensor(phrases, dtype=torch.long))


class ResidualTCN(nn.Module):
    def __init__(self, feature_count: int, hidden: int):
        super().__init__()
        self.input = nn.Linear(feature_count, hidden)
        self.dilations = (1, 2, 4)
        self.weights = nn.ParameterList(
            [nn.Parameter(torch.empty(hidden, hidden, 3)) for _ in self.dilations]
        )
        self.biases = nn.ParameterList([nn.Parameter(torch.zeros(hidden)) for _ in self.dilations])
        self.output = nn.Linear(hidden, 1)
        for weight in self.weights:
            nn.init.kaiming_uniform_(weight, a=math.sqrt(5))
        nn.init.zeros_(self.output.weight)
        nn.init.zeros_(self.output.bias)

    def forward(self, values: torch.Tensor, phrases: torch.Tensor) -> torch.Tensor:
        state = torch.tanh(self.input(values))
        length = state.shape[0]
        for dilation, weights, bias in zip(self.dilations, self.weights, self.biases):
            update = state + bias
            for kernel, offset in enumerate((-dilation, 0, dilation)):
                source = torch.arange(length, device=state.device) + offset
                valid = (source >= 0) & (source < length)
                safe = source.clamp(0, max(0, length - 1))
                valid = valid & (phrases[safe] == phrases) & (phrases >= 0)
                projected = state[safe] @ weights[:, :, kernel].T
                update = update + projected * valid[:, None]
            state = torch.tanh(update)
        return self.output(state).squeeze(-1)


def loss_for(model: ResidualTCN, item: PreparedRecord) -> torch.Tensor:
    predicted = model(item.values, item.phrases)
    point_loss = F.smooth_l1_loss(predicted, item.targets, reduction="none", beta=0.2)
    point_loss = (point_loss * item.weights).sum() / item.weights.sum().clamp_min(1)
    pair = item.speech[1:] & item.speech[:-1] & (item.phrases[1:] == item.phrases[:-1])
    if bool(pair.any()):
        predicted_delta = predicted[1:] - predicted[:-1]
        target_delta = item.targets[1:] - item.targets[:-1]
        delta_loss = F.smooth_l1_loss(predicted_delta[pair], target_delta[pair], beta=0.2)
    else:
        delta_loss = predicted.new_tensor(0.0)
    if len(predicted) >= 3:
        triple = pair[1:] & pair[:-1]
        second = predicted[2:] - 2 * predicted[1:-1] + predicted[:-2]
        smooth_loss = second[triple].square().mean() if bool(triple.any()) else predicted.new_tensor(0.0)
    else:
        smooth_loss = predicted.new_tensor(0.0)
    zero_loss = predicted[item.speech].square().mean() if bool(item.speech.any()) else predicted.new_tensor(0.0)
    return point_loss + 0.15 * delta_loss + 0.01 * smooth_loss + 0.002 * zero_loss


def metrics(model: ResidualTCN | None, records: list[PreparedRecord]) -> dict:
    all_errors, edited_errors = [], []
    weighted_sum = weighted_count = 0.0
    with torch.no_grad():
        for item in records:
            predicted = torch.zeros_like(item.targets) if model is None else model(item.values, item.phrases) * LIMIT_CENTS
            expected = item.targets * LIMIT_CENTS
            errors = (predicted - expected).abs()
            all_errors.extend(errors[item.speech].tolist())
            edited_errors.extend(errors[item.edited].tolist())
            weighted_sum += float((errors * item.weights).sum())
            weighted_count += float(item.weights.sum())
    return {
        "mae_cents": float(np.mean(all_errors)) if all_errors else None,
        "edited_mae_cents": float(np.mean(edited_errors)) if edited_errors else None,
        "weighted_mae_cents": weighted_sum / weighted_count if weighted_count else None,
        "speech_points": len(all_errors),
        "edited_points": len(edited_errors),
    }


def train_seed(train: list[PreparedRecord], validation: list[PreparedRecord], feature_count: int, args, seed: int):
    random.seed(seed)
    np.random.seed(seed)
    torch.manual_seed(seed)
    model = ResidualTCN(feature_count, args.hidden)
    optimizer = torch.optim.AdamW(model.parameters(), lr=args.learning_rate, weight_decay=1e-4)
    best_state, best_score, best_epoch = None, float("inf"), 0
    rng = random.Random(seed)
    for epoch in range(1, args.epochs + 1):
        order = list(train)
        rng.shuffle(order)
        model.train()
        for item in order:
            optimizer.zero_grad(set_to_none=True)
            loss = loss_for(model, item)
            loss.backward()
            torch.nn.utils.clip_grad_norm_(model.parameters(), 2.0)
            optimizer.step()
        score = metrics(model, validation)["weighted_mae_cents"]
        if score is not None and score < best_score:
            best_score, best_epoch = score, epoch
            best_state = copy.deepcopy(model.state_dict())
    if best_state is not None:
        model.load_state_dict(best_state)
    return model.eval(), {"seed": seed, "best_epoch": best_epoch, "validation_weighted_mae_cents": best_score}


def export_head(model: ResidualTCN, feature_names: list[str]) -> dict:
    layers = []
    for dilation, weights, bias in zip(model.dilations, model.weights, model.biases):
        layers.append(
            {
                "dilation": dilation,
                "weights": weights.detach().double().tolist(),
                "bias": bias.detach().double().tolist(),
            }
        )
    return {
        "feature_names": feature_names,
        "input_weights": model.input.weight.detach().double().tolist(),
        "input_bias": model.input.bias.detach().double().tolist(),
        "layers": layers,
        "output_weight": (model.output.weight.detach().double().squeeze(0) * LIMIT_CENTS).tolist(),
        "output_bias": float(model.output.bias.detach()) * LIMIT_CENTS,
    }


def portable_predict(head: dict, item: PreparedRecord) -> np.ndarray:
    values = item.values.numpy().astype(np.float64)
    phrases = item.phrases.numpy()
    state = np.tanh(values @ np.asarray(head["input_weights"]).T + np.asarray(head["input_bias"]))
    for layer in head["layers"]:
        weights = np.asarray(layer["weights"])
        following = state + np.asarray(layer["bias"])
        dilation = int(layer["dilation"])
        for position in range(len(state)):
            for kernel, offset in enumerate((-dilation, 0, dilation)):
                source = position + offset
                if 0 <= source < len(state) and phrases[position] >= 0 and phrases[source] == phrases[position]:
                    following[position] += weights[:, :, kernel] @ state[source]
        state = np.tanh(following)
    return state @ np.asarray(head["output_weight"]) + float(head["output_bias"])


def git_commit() -> str:
    try:
        return subprocess.check_output(["git", "rev-parse", "HEAD"], text=True).strip()
    except (OSError, subprocess.CalledProcessError):
        return ""


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dataset", required=True, nargs="+", help="GUIから書き出したJSONL。複数指定可")
    parser.add_argument("--base-model", required=True, help="収集時に使ったframe-intonation-v8 JSON")
    parser.add_argument("--out", required=True, help="生成する自己完結型モデルJSON")
    parser.add_argument("--report", help="学習report。省略時はモデル名から生成")
    parser.add_argument("--model-id", help="GUI、CLI、Serverで使う一意なモデルID")
    parser.add_argument("--display-name", help="GUIに表示するモデル名")
    parser.add_argument("--description", help="モデルの用途や教師データの説明")
    parser.add_argument("--recommended-renderer", action="append", help="推奨Renderer ID。複数回指定可")
    parser.add_argument("--epochs", type=int, default=240, help="各seedの最大epoch数")
    parser.add_argument("--hidden", type=int, default=16, help="TCNのhidden unit数")
    parser.add_argument("--learning-rate", type=float, default=0.002, help="AdamWのlearning rate")
    parser.add_argument("--seeds", default="23,29,41", help="学習するseedのカンマ区切り一覧")
    args = parser.parse_args()

    dataset_paths, base_path, out_path = [Path(value) for value in args.dataset], Path(args.base_model), Path(args.out)
    report_path = Path(args.report) if args.report else out_path.with_name(out_path.stem + "-training-report.json")
    base_hash = sha256_file(base_path)
    base_model = json.loads(base_path.read_text(encoding="utf-8"))
    if base_model.get("version") != 8 or not isinstance(base_model.get("frame_pitch"), dict):
        raise ValueError("base model must be a frame-intonation v8 JSON")
    records = load_records(dataset_paths, base_hash)
    split = deterministic_split(records)
    feature_names = feature_vocabulary(split["train"])
    feature_index = {name: index for index, name in enumerate(feature_names)}
    prepared = {name: [prepare(record, feature_index) for record in values] for name, values in split.items()}

    candidates = []
    for seed in [int(value.strip()) for value in args.seeds.split(",") if value.strip()]:
        model, summary = train_seed(prepared["train"], prepared["validation"], len(feature_names), args, seed)
        summary["validation"] = metrics(model, prepared["validation"])
        candidates.append((model, summary))
    model, selected = min(candidates, key=lambda item: item[1]["validation_weighted_mae_cents"])
    split_metrics = {
        name: {"zero": metrics(None, items), "model": metrics(model, items)} for name, items in prepared.items()
    }
    residual_head = export_head(model, feature_names)
    parity_error = 0.0
    with torch.no_grad():
        for items in prepared.values():
            for item in items:
                expected = (model(item.values, item.phrases) * LIMIT_CENTS).numpy()
                parity_error = max(parity_error, float(np.max(np.abs(portable_predict(residual_head, item) - expected))))
    if parity_error > 1e-4:
        raise RuntimeError(f"portable inference mismatch: {parity_error:.8f} cents")
    model_id = str(args.model_id or out_path.stem).strip()
    if not model_id:
        raise ValueError("model ID must not be empty")
    model_json = {
        "id": model_id,
        "display_name": str(args.display_name or model_id),
        "description": str(args.description or "GUIで確認した手動抑揚補正をv8へ加える個人モデル"),
        "license": base_model.get("license", "UTAUTTS-JSUT-DERIVED-MODEL-TERMS-1.0"),
        "license_notice": base_model.get("license_notice", "licenses/PROSODY-MODELS.txt"),
        "provenance": base_model.get("provenance"),
        "recommended_renderers": args.recommended_renderer or base_model.get("recommended_renderers", []),
        "version": 11,
        "feature_version": 2,
        "mode": "intonation_frame_v8_manual_residual",
        "outputs": {"frame_pitch": True, "mora_pitch_residual": True},
        "duration_weights": {},
        "base_model": {"id": base_model.get("id", "frame-intonation-v8"), "sha256": base_hash},
        "frame_pitch": base_model["frame_pitch"],
        "mora_pitch_residual": residual_head,
        "residual_limits": {"low_cents": -LIMIT_CENTS, "high_cents": LIMIT_CENTS, "smoothing_ms": 20.0},
        "metrics": {
            "records": len(split["validation"]),
            "pitch_mae_cents": split_metrics["validation"]["model"]["weighted_mae_cents"],
            "baseline_pitch_mae_cents": split_metrics["validation"]["zero"]["weighted_mae_cents"],
        },
        "training": {
            "records": len(split["train"]),
            "tokens": sum(int(item.speech.sum()) for item in prepared["train"]),
            "epochs": args.epochs,
            "learning_rate": args.learning_rate,
            "seed": selected["seed"],
        },
    }
    report = {
        "version": 1,
        "dataset": {
            "inputs": [{"path": str(path), "sha256": sha256_file(path)} for path in dataset_paths],
            "accepted_distinct_records": len(records),
        },
        "base_model": {"path": str(base_path), "id": base_model.get("id"), "sha256": base_hash},
        "output": str(out_path),
        "model_id": model_id,
        "target": {"mode": "raw_manual_offset", "clip_cents": LIMIT_CENTS, "unchanged_weight": 0.15},
        "split": {name: [item["id"] for item in values] for name, values in split.items()},
        "feature_count": len(feature_names),
        "portable_max_abs_error_cents": parity_error,
        "candidates": [summary for _, summary in candidates],
        "selected": selected,
        "metrics": split_metrics,
        "git_commit": git_commit(),
        "torch": torch.__version__,
        "device": "cpu",
    }
    out_path.parent.mkdir(parents=True, exist_ok=True)
    report_path.parent.mkdir(parents=True, exist_ok=True)
    out_path.write_text(json.dumps(model_json, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    report_path.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({"model": str(out_path), "report": str(report_path), "selected": selected, "metrics": split_metrics}, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
