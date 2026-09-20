#!/usr/bin/env python3
"""Attach CTC-derived mora timings to Kokoro frame-TCN records.

The CTC network and vocabulary are from the official Kokoro-Align project.
Only the final conversion from aligned phones to UtauTTS mora records is local.
"""

from __future__ import annotations

import argparse
import json
import sys
import wave
from functools import lru_cache
from pathlib import Path

import numpy as np
import pyopenjtalk
import torch
from torch.nn.utils.rnn import pack_sequence

KOKORO_ALIGN_ROOT = Path(__file__).resolve().parents[1] / "data" / "kokoro-align"
sys.path.insert(0, str(KOKORO_ALIGN_ROOT))
from kokoro_align.align import ctc_best_path  # noqa: E402
from kokoro_align.encoder import encode_text, v2i  # noqa: E402
from kokoro_align.train import AudioToChar, DEFAULT_PARAMS  # noqa: E402


SAMPLE_RATE = 22050
N_FFT = 512
HOP = N_FFT // 2
N_MELS = 40
N_MFCC = 40


def read_wav(path: Path) -> np.ndarray:
    with wave.open(str(path), "rb") as source:
        if source.getnchannels() != 1 or source.getsampwidth() != 2 or source.getframerate() != SAMPLE_RATE:
            raise ValueError(f"unexpected WAV format: {path}")
        raw = source.readframes(source.getnframes())
    return np.frombuffer(raw, dtype="<i2").astype(np.float32) / 32768.0


def hz_to_mel(hz: np.ndarray) -> np.ndarray:
    return 2595.0 * np.log10(1.0 + hz / 700.0)


def mel_to_hz(mel: np.ndarray) -> np.ndarray:
    return 700.0 * (10.0 ** (mel / 2595.0) - 1.0)


def mel_filterbank() -> torch.Tensor:
    points = mel_to_hz(np.linspace(hz_to_mel(np.array([0.0]))[0], hz_to_mel(np.array([SAMPLE_RATE / 2]))[0], N_MELS + 2))
    bins = np.floor((N_FFT + 1) * points / SAMPLE_RATE).astype(int)
    bank = np.zeros((N_MELS, N_FFT // 2 + 1), dtype=np.float32)
    for index in range(N_MELS):
        left, center, right = bins[index : index + 3]
        if center > left:
            bank[index, left:center] = (np.arange(left, center) - left) / (center - left)
        if right > center:
            bank[index, center:right] = (right - np.arange(center, right)) / (right - center)
    return torch.from_numpy(bank)


def dct_matrix() -> torch.Tensor:
    n = torch.arange(float(N_MELS)).unsqueeze(0)
    k = torch.arange(float(N_MFCC)).unsqueeze(1)
    result = torch.cos(torch.pi / N_MELS * (n + 0.5) * k)
    result[0] *= 1.0 / np.sqrt(2.0)
    return result * np.sqrt(2.0 / N_MELS)


MEL_BANK = mel_filterbank()
DCT = dct_matrix()


def mfcc(samples: np.ndarray) -> torch.Tensor:
    audio = torch.from_numpy(samples)
    if audio.numel() < N_FFT:
        audio = torch.nn.functional.pad(audio, (0, N_FFT - audio.numel()))
    window = torch.hann_window(N_FFT)
    spectrum = torch.stft(
        audio, n_fft=N_FFT, hop_length=HOP, win_length=N_FFT,
        window=window, center=True, pad_mode="reflect", return_complex=True,
    ).abs().square()
    mel = MEL_BANK @ spectrum
    log_mel = 10.0 * torch.log10(torch.clamp(mel, min=1.0e-10))
    log_mel -= torch.max(log_mel)
    log_mel = torch.clamp(log_mel, min=-80.0)
    return (DCT @ log_mel).transpose(0, 1).contiguous().float()


def _legacy_phone_groups(reading: str, tokens: list[dict] | None = None) -> list[list[str]]:
    groups: list[list[str]] = []
    current: list[str] = []
    vowels = set("aiueo")
    for raw in reading.split():
        token = raw.strip()
        if token in {"_"}:
            continue
        if token in {".", ",", "?", "!"}:
            if current:
                groups.append(current)
                current = []
            groups.append([])
            continue
        if token == "q":
            if current:
                groups.append(current)
                current = []
            groups.append([])
            continue
        current.append(token)
        if token == "N" or token.endswith(":") or token[-1:].lower() in vowels:
            groups.append(current)
            current = []
    if current:
        groups.append(current)
    if tokens is not None and len(groups) != len(tokens):
        # Kokoro writes a long vowel such as ``yo:`` as one phone group,
        # while Open JTalk exposes the prolonged sound mark (ー) as a second
        # mora.  Keep the CTC phones on the preceding mora and insert an
        # empty group for that extra mora; align_record splits the measured
        # span between the two morae below.
        non_long_tokens = [token for token in tokens if str(token.get("mora", "")) != "ー"]
        if len(non_long_tokens) == len(groups):
            expanded: list[list[str]] = []
            cursor = 0
            for token in tokens:
                if str(token.get("mora", "")) == "ー":
                    expanded.append([])
                else:
                    expanded.append(groups[cursor])
                    cursor += 1
            groups = expanded
    return groups


def label_bounds(path: np.ndarray, label_count: int) -> list[tuple[int, int]]:
    result = []
    for index in range(label_count):
        start_candidates = np.flatnonzero(path >= (2 * index + 1))
        end_candidates = np.flatnonzero(path >= (2 * index + 2))
        start = int(start_candidates[0]) if len(start_candidates) else 0
        end = int(end_candidates[0]) if len(end_candidates) else len(path)
        result.append((min(start, len(path)), max(start, min(end, len(path)))))
    return result


@lru_cache(maxsize=128)
def mora_phones(mora: str) -> tuple[str, ...]:
    if not mora or mora == "ー":
        return ()
    return tuple(pyopenjtalk.g2p(mora, kana=False).split())


def phone_groups(reading: str, tokens: list[dict]) -> list[list[str]]:
    """Map Kokoro phones onto the exact mora list produced by Open JTalk.

    A colon is a single CTC label but represents two vowel units for mora
    timing. Keeping an unconsumed unit on the source phone lets the following
    mora share that label without inventing a second CTC phone.
    """

    units: list[dict] = []
    for raw in reading.split():
        token = raw.strip()
        if token == "_":
            continue
        if token in {".", ",", "?", "!"}:
            units.append({"pause": True})
            continue
        if token == "q":
            units.append({"remaining": ["cl"], "label": None})
            continue
        base = token[:-1] if token.endswith(":") else token
        units.append({"remaining": [base, base] if token.endswith(":") else [base], "label": token})

    groups: list[list[str]] = []
    cursor = 0
    for token in tokens:
        mora = str(token.get("mora", ""))
        if token.get("pause", False):
            while cursor < len(units) and not units[cursor].get("pause", False) and not units[cursor].get("remaining"):
                cursor += 1
            while cursor < len(units) and units[cursor].get("pause", False):
                cursor += 1
            groups.append([])
            continue
        if mora == "ー":
            if cursor < len(units) and units[cursor].get("remaining"):
                units[cursor]["remaining"].pop(0)
                if not units[cursor]["remaining"]:
                    cursor += 1
            groups.append([])
            continue

        group: list[str] = []
        expected_phones = mora_phones(mora)
        if (
            mora == "へ"
            and expected_phones == ("e",)
            and cursor < len(units)
            and units[cursor].get("remaining", [])[:1] == ["h"]
            and cursor + 1 < len(units)
            and units[cursor + 1].get("remaining", [])[:1] == ["e"]
        ):
            # Open JTalk can expose へ as a particle (e), while the supplied
            # Kokoro sequence retains the h onset in this context.
            expected_phones = ("h", "e")
        for expected in expected_phones:
            if cursor < len(units) and units[cursor].get("pause", False):
                raise ValueError(f"unexpected punctuation before mora {mora!r}")
            if cursor >= len(units) or not units[cursor].get("remaining"):
                raise ValueError(f"not enough source phones for mora {mora!r}")
            actual = units[cursor]["remaining"][0]
            context_variant = (
                mora == "は" and {expected, actual} == {"h", "w"}
            ) or (
                mora == "へ" and {expected, actual} == {"h", "e"}
            )
            if actual != expected and not context_variant:
                raise ValueError(f"phone mismatch for mora {mora!r}: expected {expected!r}, got {actual!r}")
            units[cursor]["remaining"].pop(0)
            label = units[cursor].get("label")
            if label in v2i and label not in group:
                group.append(label)
            if not units[cursor]["remaining"]:
                cursor += 1
        groups.append(group)

    return groups


def ctc_viterbi_path(log_probs: np.ndarray, labels: np.ndarray) -> tuple[np.ndarray, np.ndarray]:
    """Find the best monotonic CTC path with the standard Viterbi recurrence.

    ``kokoro_align.align.ctc_best_path`` is useful for inspecting a small
    number of files, but its Python beam loop is unnecessarily expensive for
    the full Kokoro corpus.  This recurrence uses the same CTC state graph
    and the same official checkpoint while keeping the dynamic program in
    NumPy arrays.
    """

    expanded = np.zeros(len(labels) * 2 + 1, dtype=np.int32)
    expanded[1::2] = labels.astype(np.int32)
    time_count = int(log_probs.shape[0])
    state_count = int(len(expanded))
    if time_count == 0 or time_count < len(labels):
        raise ValueError(f"CTC sequence is too short: {time_count} frames for {len(labels)} labels")

    neg_inf = -np.inf
    scores = np.full(state_count, neg_inf, dtype=np.float64)
    scores[0] = float(log_probs[0, expanded[0]])
    if state_count > 1:
        scores[1] = float(log_probs[0, expanded[1]])
    back = np.full((time_count, state_count), -1, dtype=np.int32)

    states = np.arange(state_count, dtype=np.int32)
    for time in range(1, time_count):
        stay = scores
        advance = np.full(state_count, neg_inf, dtype=np.float64)
        advance[1:] = scores[:-1]
        skip = np.full(state_count, neg_inf, dtype=np.float64)
        can_skip = (expanded[2:] != 0) & (expanded[2:] != expanded[:-2])
        skip[2:][can_skip] = scores[:-2][can_skip]

        candidates = np.stack((stay, advance, skip), axis=0)
        choice = np.argmax(candidates, axis=0)
        best = candidates[choice, states]
        back[time] = states - choice
        scores = best + log_probs[time, expanded]

    if state_count == 1:
        final_state = 0
    else:
        final_state = state_count - 1 if scores[-1] >= scores[-2] else state_count - 2
    if not np.isfinite(scores[final_state]):
        raise ValueError("CTC Viterbi path could not consume the requested label sequence")

    path = np.empty(time_count, dtype=np.int32)
    path[-1] = final_state
    for time in range(time_count - 1, 0, -1):
        previous = int(back[time, path[time]])
        if previous < 0:
            raise ValueError("CTC Viterbi backtracking reached an invalid state")
        path[time - 1] = previous
    path_scores = log_probs[np.arange(time_count), expanded[path]]
    return path, path_scores


def align_record(record: dict, model: AudioToChar, beam_size: int, method: str) -> dict:
    samples = read_wav(Path(record["audio_path"]))
    features = mfcc(samples)
    tokens = record["tokens"]
    groups = phone_groups(str(record.get("source_reading", "")), tokens)
    if len(groups) != len(tokens):
        raise ValueError(f"token/group count mismatch: {len(tokens)} != {len(groups)}")
    flattened = [phone for group in groups for phone in group if phone in v2i]
    labels = encode_text(" ".join(flattened))
    if len(labels) == 0:
        raise ValueError("empty CTC label sequence")
    with torch.no_grad():
        logits, lengths = model(pack_sequence([features], enforce_sorted=False))
    length = int(lengths[0])
    log_probs = torch.log_softmax(logits[:length, 0], dim=-1).cpu().numpy()
    if method == "beam":
        path, _, scores = ctc_best_path(log_probs, labels, beam_size=beam_size, max_move=4)
    else:
        path, scores = ctc_viterbi_path(log_probs, labels)
    bounds = label_bounds(path, len(labels))
    spans: list[tuple[int, int] | None] = []
    cursor = 0
    for group in groups:
        count = sum(phone in v2i for phone in group)
        if count:
            start = bounds[cursor][0]
            end = bounds[cursor + count - 1][1]
            spans.append((start, end))
            cursor += count
        else:
            spans.append(None)
    for index, span in enumerate(spans):
        if span is not None:
            continue
        previous = next((spans[j][1] for j in range(index - 1, -1, -1) if spans[j] is not None), 0)
        following = next((spans[j][0] for j in range(index + 1, len(spans)) if spans[j] is not None), len(path))
        if tokens[index].get("pause", False):
            spans[index] = (previous, max(previous, following))
        elif str(tokens[index].get("mora", "")) == "ー" and index > 0 and spans[index - 1] is not None:
            # A prolonged sound mark shares the preceding CTC vowel. Split
            # that measured span rather than manufacturing a new phone.
            previous_start, previous_end = spans[index - 1]
            midpoint = previous_start + max(1, (previous_end - previous_start) // 2)
            spans[index - 1] = (previous_start, midpoint)
            spans[index] = (midpoint, previous_end)
        else:
            spans[index] = (previous, min(max(previous, following), previous + max(1, round(35.0 / (HOP * 1000.0 / SAMPLE_RATE)))))
    for index, token in enumerate(tokens):
        if token.get("pause", False) or spans[index][1] > spans[index][0]:
            continue
        # q (っ) has no CTC label. If the surrounding states meet exactly,
        # borrow one frame from the following mora so the closure remains a
        # usable feature interval instead of disappearing from the timeline.
        if index + 1 < len(spans):
            next_start, next_end = spans[index + 1]
            if next_end - next_start > 1:
                spans[index] = (next_start, next_start + 1)
                spans[index + 1] = (next_start + 1, next_end)
                continue
        if index > 0:
            previous_start, previous_end = spans[index - 1]
            if previous_end - previous_start > 1:
                spans[index - 1] = (previous_start, previous_end - 1)
                spans[index] = (previous_end - 1, previous_end)
    frame_ms = HOP * 1000.0 / SAMPLE_RATE
    copied_tokens = []
    for token, span in zip(tokens, spans):
        assert span is not None
        start, end = span
        copied = dict(token)
        copied["start_ms"] = start * frame_ms
        copied["end_ms"] = max(copied["start_ms"], end * frame_ms)
        copied["duration_ms"] = copied["end_ms"] - copied["start_ms"]
        copied_tokens.append(copied)
    result = dict(record)
    result["tokens"] = copied_tokens
    result["alignment_source"] = f"Kokoro-Align CTC v0.2 {method}"
    result["alignment_method"] = method
    result["alignment_score"] = float(np.mean(scores))
    result["alignment_frames"] = int(length)
    return result


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--input", required=True)
    parser.add_argument("--out", required=True)
    parser.add_argument("--checkpoint", required=True)
    parser.add_argument("--beam-size", type=int, default=64)
    parser.add_argument(
        "--method", choices=("viterbi", "beam"), default="viterbi",
        help="CTC path search method; viterbi is the fast full-corpus default",
    )
    parser.add_argument("--limit", type=int, default=0)
    args = parser.parse_args()
    model = AudioToChar(**DEFAULT_PARAMS)
    state = torch.load(args.checkpoint, map_location="cpu")
    model.load_state_dict(state["model"])
    model.eval()
    output = Path(args.out)
    output.parent.mkdir(parents=True, exist_ok=True)
    records = []
    with Path(args.input).open(encoding="utf-8") as stream:
        for line in stream:
            if line.strip():
                records.append(json.loads(line))
    if args.limit > 0:
        records = records[: args.limit]
    if not records:
        raise ValueError("no records")
    with output.open("x", encoding="utf-8") as stream:
        for index, record in enumerate(records, 1):
            aligned = align_record(record, model, args.beam_size, args.method)
            stream.write(json.dumps(aligned, ensure_ascii=False) + "\n")
            if index % 100 == 0:
                print(f"aligned {index}/{len(records)}", flush=True)
    print(f"wrote {len(records)} aligned records: {output}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
