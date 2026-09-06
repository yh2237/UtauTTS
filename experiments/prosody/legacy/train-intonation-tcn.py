#!/usr/bin/env python3
"""Train a compact sequence intonation model and export it as portable JSON."""

import argparse
import json
import math
import random
import sys
from pathlib import Path

import torch
from torch import nn
from torch.nn import functional as F

sys.path.insert(0, str(Path(__file__).resolve().parents[3] / "tools"))
from torch_device import device_description, move_batch, resolve_device


def fnv1a(text: str) -> int:
    value = 2166136261
    for byte in text.encode("utf-8"):
        value ^= byte
        value = (value * 16777619) & 0xFFFFFFFF
    return value


def token_features(tokens, position):
    current = tokens[position]
    denominator = max(1, len(tokens) - 1)
    pos = position / denominator
    result = {"bias": 1.0, "position": pos, "position2": pos * pos, "from_end": 1.0 - pos}
    if position == 0 or tokens[position - 1].get("pause", False):
        result["phrase_start"] = 1.0
    if position == len(tokens) - 1 or tokens[position + 1].get("pause", False):
        result["phrase_end"] = 1.0
    add_categorical(result, "mora", current)
    if position > 0:
        add_categorical(result, "prev", tokens[position - 1])
    else:
        result["prev=<BOS>"] = 1.0
    if position + 1 < len(tokens):
        add_categorical(result, "next", tokens[position + 1])
    else:
        result["next=<EOS>"] = 1.0
    if "accent_phrase_position" in current:
        phrase_length = max(1, int(current["accent_phrase_length"]))
        phrase_position = int(current["accent_phrase_position"])
        nucleus = int(current["accent_nucleus"])
        result["accent_position"] = phrase_position / phrase_length
        result["accent_from_end"] = (phrase_length - phrase_position) / phrase_length
        result["accent_nucleus_position"] = nucleus / phrase_length
        result["accent_high"] = float(bool(current["accent_high"]))
        result["accent_phrase_start"] = float(bool(current["accent_phrase_start"]))
        result["accent_phrase_end"] = float(bool(current["accent_phrase_end"]))
        result["word_start"] = float(bool(current["word_start"]))
        result["word_end"] = float(bool(current["word_end"]))
        result[f"pos={current.get('pos', '*')}"] = 1.0
        result[f"pos_group1={current.get('pos_group1', '*')}"] = 1.0
        if nucleus == 0:
            result["accent_type=heiban"] = 1.0
        elif phrase_position < nucleus:
            result["accent_type=before"] = 1.0
        elif phrase_position == nucleus:
            result["accent_type=nucleus"] = 1.0
        else:
            result["accent_type=after"] = 1.0
    return result


def add_categorical(result, prefix, token):
    if token.get("pause", False):
        result[f"{prefix}=<PAUSE>"] = 1.0
        return
    result[f"{prefix}={token.get('mora', '')}"] = 1.0
    result[f"{prefix}_vowel={token.get('vowel', '')}"] = 1.0


def load_records(path):
    records = []
    with Path(path).open("r", encoding="utf-8") as source:
        for line_number, line in enumerate(source, 1):
            if not line.strip():
                continue
            record = json.loads(line)
            if record.get("version") != 1:
                raise ValueError(f"{path}:{line_number}: unsupported dataset version")
            records.append(record)
    train = [record for record in records if fnv1a(record["id"]) % 10 != 0]
    validation = [record for record in records if fnv1a(record["id"]) % 10 == 0]
    if not train or not validation:
        raise ValueError("dataset must have non-empty deterministic train and validation splits")
    return train, validation


def add_openjtalk_features(records):
    from openjtalk_features import analyze

    result = []
    for record in records:
        try:
            _, annotated = analyze(record["text"])
        except Exception:
            continue
        tokens = record["tokens"]
        if len(annotated) != len(tokens):
            continue
        if any(bool(left.get("pause")) != bool(right.get("pause")) for left, right in zip(annotated, tokens)):
            continue
        copied = dict(record)
        copied["tokens"] = []
        for token, linguistic in zip(tokens, annotated):
            enriched = dict(token)
            for name, value in linguistic.items():
                if name not in {"mora", "pause"}:
                    enriched[name] = value
            copied["tokens"].append(enriched)
        result.append(copied)
    return result


def prepare(records, feature_index=None):
    if feature_index is None:
        names = set()
        for record in records:
            for position in range(len(record["tokens"])):
                names.update(token_features(record["tokens"], position))
        feature_index = {name: index for index, name in enumerate(sorted(names))}
    prepared = []
    for record in records:
        sequence = []
        targets = []
        mask = []
        for position, token in enumerate(record["tokens"]):
            sparse = [(feature_index[name], value) for name, value in token_features(record["tokens"], position).items() if name in feature_index]
            ratio = float(token.get("pitch_ratio", 0.0))
            valid = not token.get("pause", False) and ratio > 0
            sequence.append(sparse)
            targets.append(math.log(min(1.4, max(0.7, ratio))) if valid else 0.0)
            mask.append(valid)
        prepared.append((sequence, targets, mask))
    return prepared, feature_index


class IntonationTCN(nn.Module):
    def __init__(self, inputs, hidden, dilations):
        super().__init__()
        self.input = nn.Linear(inputs, hidden)
        self.layers = nn.ModuleList([nn.Conv1d(hidden, hidden, 3, dilation=dilation) for dilation in dilations])
        self.output = nn.Linear(hidden, 1)
        self.dilations = dilations

    def forward(self, values):
        state = torch.tanh(self.input(values)).transpose(1, 2)
        for layer, dilation in zip(self.layers, self.dilations):
            convolved = layer(F.pad(state, (dilation, dilation)))
            state = torch.tanh(state + convolved)
        return self.output(state.transpose(1, 2)).squeeze(-1)


def batches(records, feature_count, batch_size, rng):
    order = list(range(len(records)))
    rng.shuffle(order)
    for offset in range(0, len(order), batch_size):
        selected = [records[index] for index in order[offset:offset + batch_size]]
        length = max(len(item[0]) for item in selected)
        values = torch.zeros((len(selected), length, feature_count), dtype=torch.float32)
        targets = torch.zeros((len(selected), length), dtype=torch.float32)
        mask = torch.zeros((len(selected), length), dtype=torch.bool)
        for row, (sequence, expected, valid) in enumerate(selected):
            for position, sparse in enumerate(sequence):
                for column, value in sparse:
                    values[row, position, column] = value
            targets[row, :len(expected)] = torch.tensor(expected)
            mask[row, :len(valid)] = torch.tensor(valid)
        yield values, targets, mask


def centered(values, mask):
    count = mask.sum(dim=1, keepdim=True).clamp_min(1)
    center = (values * mask).sum(dim=1, keepdim=True) / count
    return values - center


def sequence_loss(predicted, targets, mask, bounded_target=None):
    predicted = centered(predicted, mask)
    # 発話内の有声平均を除き、推論時に捨てるオフセットを学習させない。
    targets = centered(targets, mask)
    if bounded_target is not None:
        low, high = bounded_target
        targets = targets.clamp(math.log(low), math.log(high))
    absolute = (predicted[mask] - targets[mask]).abs().mean()
    pair_mask = mask[:, 1:] & mask[:, :-1]
    if pair_mask.any():
        predicted_delta = predicted[:, 1:] - predicted[:, :-1]
        target_delta = targets[:, 1:] - targets[:, :-1]
        delta = (predicted_delta[pair_mask] - target_delta[pair_mask]).abs().mean()
        return absolute + 0.35 * delta
    return absolute


@torch.no_grad()
def evaluate(model, records, feature_count, batch_size, low, high, device=torch.device("cpu")):
    errors = []
    model.eval()
    for values, targets, mask in batches(records, feature_count, batch_size, random.Random(0)):
        values, targets, mask = move_batch(device, values, targets, mask)
        predicted = model(values)
        for row in range(values.shape[0]):
            valid = mask[row]
            if not valid.any():
                continue
            speech = predicted[row][valid]
            speech = speech - speech.median()
            speech = speech.clamp(math.log(low), math.log(high))
            errors.extend((speech - targets[row][valid]).abs().mul(1200 / math.log(2)).detach().cpu().tolist())
    return sum(errors) / len(errors)


def export_model(model, feature_index, args, records, tokens, validation_mae):
    feature_names = [None] * len(feature_index)
    for name, index in feature_index.items():
        feature_names[index] = name
    layers = []
    for dilation, layer in zip(model.dilations, model.layers):
        layers.append({
            "dilation": dilation,
            "weights": layer.weight.detach().cpu().double().tolist(),
            "bias": layer.bias.detach().cpu().double().tolist(),
        })
    return {
        "id": str(args.model_id or Path(args.out).stem),
        "display_name": str(args.display_name or Path(args.out).stem),
        "description": str(args.description or "Learned intonation model"),
        "recommended_renderers": list(args.recommended_renderer),
        "version": 6 if args.bounded_target else (5 if args.openjtalk_accent else 4),
        "feature_version": 1,
        "mode": "intonation_tcn_accent_bounded" if args.bounded_target else ("intonation_tcn_accent" if args.openjtalk_accent else "intonation_tcn"),
        "duration_weights": {},
        "sequence_pitch": {
            "feature_names": feature_names,
            "input_weights": model.input.weight.detach().cpu().double().tolist(),
            "input_bias": model.input.bias.detach().cpu().double().tolist(),
            "layers": layers,
            "output_weight": model.output.weight.detach().cpu().double().squeeze(0).tolist(),
            "output_bias": float(model.output.bias.detach().cpu()),
            "low": args.low,
            "high": args.high,
        },
        "metrics": {"records": args.validation_records, "tokens": args.validation_tokens, "pitch_mae_cents": validation_mae},
        "training": {"records": records, "tokens": tokens, "epochs": args.epochs, "learning_rate": args.learning_rate, "seed": args.seed, "device": str(args.resolved_device)},
    }


def vowel_of(mora):
    last = mora[-1:] if mora else ""
    for vowel, characters in {
        "a": "あかがさざただなはばぱまやらわぁゃゎ",
        "i": "いきぎしじちぢにひびぴみりゐぃ",
        "u": "うくぐすずつづぬふぶぷむゆるゔぅゅ",
        "e": "えけげせぜてでねへべぺめれゑぇ",
        "o": "おこごそぞとどのほぼぽもよろをぉょ",
        "n": "ん",
        "cl": "っ",
    }.items():
        if last in characters:
            return vowel
    return ""


@torch.no_grad()
def predict_corpus(model, corpus_path, feature_index, low, high, bounded_target=False):
    from openjtalk_features import analyze

    corpus = json.loads(Path(corpus_path).read_text(encoding="utf-8"))
    cases = []
    model.eval()
    device = next(model.parameters()).device
    for item in corpus["cases"]:
        reading, tokens = analyze(item["text"])
        previous_vowel = ""
        for token in tokens:
            if token["pause"]:
                token["vowel"] = ""
                previous_vowel = ""
            else:
                token["vowel"] = previous_vowel if token["mora"] == "ー" else vowel_of(token["mora"])
                previous_vowel = token["vowel"]
        values = torch.zeros((1, len(tokens), len(feature_index)), dtype=torch.float32)
        for position in range(len(tokens)):
            for name, value in token_features(tokens, position).items():
                if name in feature_index:
                    values[0, position, feature_index[name]] = value
        values = values.to(device=device, non_blocking=True)
        predicted = model(values)[0]
        speech = torch.tensor([not token["pause"] for token in tokens], dtype=torch.bool, device=device)
        center = predicted[speech].median() if speech.any() else predicted.new_zeros(())
        predicted = (predicted - center).clamp(math.log(low), math.log(high)).exp()
        predicted_values = predicted.detach().cpu().tolist()
        factors = [1.0 if token["pause"] else float(predicted_values[position]) for position, token in enumerate(tokens)]
        cases.append({"id": item["id"], "text": item["text"], "reading": reading, "pitch_factors": factors})
    name = "openjtalk-accent-tcn-v6-bounded" if bounded_target else "openjtalk-accent-tcn-v5"
    return {"version": 1, "name": name, "cases": cases}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--dataset", required=True)
    parser.add_argument("--out", default="out/prosody/intonation-tcn.json")
    parser.add_argument("--model-id", default="", help="stable plugin ID stored in the model")
    parser.add_argument("--display-name", default="", help="user-facing model name")
    parser.add_argument("--description", default="", help="user-facing model description")
    parser.add_argument("--recommended-renderer", action="append", default=[], help="compatible renderer ID; repeatable")
    parser.add_argument("--epochs", type=int, default=24)
    parser.add_argument("--learning-rate", type=float, default=0.002)
    parser.add_argument("--hidden", type=int, default=24)
    parser.add_argument("--batch-size", type=int, default=32)
    parser.add_argument("--device", default="auto", help="PyTorch device: auto, cpu, cuda, cuda:N, xpu, or mps")
    parser.add_argument("--seed", type=int, default=1)
    parser.add_argument("--low", type=float, default=0.97)
    parser.add_argument("--high", type=float, default=1.03)
    parser.add_argument("--openjtalk-accent", action="store_true")
    parser.add_argument("--bounded-target", action="store_true", help="center and clamp pitch targets to the inference range during training")
    parser.add_argument("--predict-corpus")
    parser.add_argument("--predict-out")
    args = parser.parse_args()
    random.seed(args.seed)
    torch.manual_seed(args.seed)
    torch.set_num_threads(max(1, min(8, torch.get_num_threads())))
    try:
        device = resolve_device(args.device)
    except ValueError as error:
        parser.error(str(error))
    args.resolved_device = str(device)
    print(f"training device: {device_description(device)}")

    train_raw, validation_raw = load_records(args.dataset)
    if args.openjtalk_accent:
        original_train, original_validation = len(train_raw), len(validation_raw)
        train_raw = add_openjtalk_features(train_raw)
        validation_raw = add_openjtalk_features(validation_raw)
        print(f"Open JTalk alignment: train {len(train_raw)}/{original_train}, validation {len(validation_raw)}/{original_validation}")
    train, feature_index = prepare(train_raw)
    validation, _ = prepare(validation_raw, feature_index)
    args.validation_records = len(validation)
    args.validation_tokens = sum(sum(item[2]) for item in validation)
    model = IntonationTCN(len(feature_index), args.hidden, [1, 2, 4]).to(device)
    optimizer = torch.optim.AdamW(model.parameters(), lr=args.learning_rate, weight_decay=1e-5)
    rng = random.Random(args.seed)
    for epoch in range(args.epochs):
        model.train()
        total = torch.zeros((), device=device)
        count = 0
        for values, targets, mask in batches(train, len(feature_index), args.batch_size, rng):
            values, targets, mask = move_batch(device, values, targets, mask)
            if not mask.any():
                continue
            optimizer.zero_grad()
            bounded = (args.low, args.high) if args.bounded_target else None
            loss = sequence_loss(model(values), targets, mask, bounded)
            loss.backward()
            torch.nn.utils.clip_grad_norm_(model.parameters(), 1.0)
            optimizer.step()
            total += loss.detach()
            count += 1
        print(f"epoch {epoch + 1:02d}/{args.epochs}: loss={float(total.cpu()) / max(1, count):.6f}")

    validation_mae = evaluate(model, validation, len(feature_index), args.batch_size, args.low, args.high, device)
    train_tokens = sum(sum(item[2]) for item in train)
    exported = export_model(model, feature_index, args, len(train), train_tokens, validation_mae)
    output = Path(args.out)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(exported, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(f"wrote {output} ({len(train)} train/{len(validation)} validation records, {validation_mae:.1f} cents MAE)")
    if args.predict_corpus or args.predict_out:
        if not args.predict_corpus or not args.predict_out:
            parser.error("--predict-corpus and --predict-out must be used together")
        contours = predict_corpus(model, args.predict_corpus, feature_index, args.low, args.high, args.bounded_target)
        contour_path = Path(args.predict_out)
        contour_path.parent.mkdir(parents=True, exist_ok=True)
        contour_path.write_text(json.dumps(contours, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        print(f"wrote {len(contours['cases'])} predicted contours to {contour_path}")


if __name__ == "__main__":
    main()
