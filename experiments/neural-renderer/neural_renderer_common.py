"""ニューラル制御型renderer実験の共通処理。"""

from __future__ import annotations

import hashlib
import bisect
import json
import re
import sys
import wave
from array import array
from pathlib import Path

import torch
import numpy as np
from torch.nn import functional as F


PHONE_PATTERN = re.compile(r"-([^+]+)\+")
SILENCE_PHONES = {"sil", "pau", "xx"}
TICKS_PER_SECOND = 10_000_000


def split_for(utterance_id: str) -> str:
    value = int.from_bytes(hashlib.sha256(utterance_id.encode("utf-8")).digest()[:4], "little") % 100
    if value < 80:
        return "train"
    if value < 90:
        return "validation"
    return "test"


def parse_hts_label(path: Path, sample_rate: int) -> list[dict]:
    raw_segments = []
    for number, raw in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
        parts = raw.split(maxsplit=2)
        if len(parts) != 3:
            raise ValueError(f"{path}:{number}: invalid HTS label")
        match = PHONE_PATTERN.search(parts[2])
        if match is None:
            raise ValueError(f"{path}:{number}: phone not found")
        raw_segments.append((int(parts[0]), int(parts[1]), match.group(1)))
    result = []
    for index, (start, end, phone) in enumerate(raw_segments):
        result.append(
            {
                "index": index,
                "phone": phone,
                "left_phone": raw_segments[index - 1][2] if index > 0 else "xx",
                "right_phone": raw_segments[index + 1][2] if index + 1 < len(raw_segments) else "xx",
                "start_sample": round(start * sample_rate / TICKS_PER_SECOND),
                "end_sample": round(end * sample_rate / TICKS_PER_SECOND),
            }
        )
    return result


def wav_info(path: Path) -> tuple[int, int]:
    with wave.open(str(path), "rb") as source:
        if source.getnchannels() != 1 or source.getsampwidth() != 2:
            raise ValueError(f"unsupported WAV format: {path}")
        return source.getframerate(), source.getnframes()


def read_pcm16(path: str | Path) -> tuple[int, torch.Tensor]:
    with wave.open(str(path), "rb") as source:
        if source.getnchannels() != 1 or source.getsampwidth() != 2:
            raise ValueError(f"unsupported WAV format: {path}")
        sample_rate = source.getframerate()
        values = array("h")
        values.frombytes(source.readframes(source.getnframes()))
    if sys.byteorder != "little":
        values.byteswap()
    return sample_rate, torch.tensor(values, dtype=torch.float32).div_(32768)


def write_pcm16(path: str | Path, sample_rate: int, values: torch.Tensor) -> None:
    output = Path(path)
    output.parent.mkdir(parents=True, exist_ok=True)
    samples = array(
        "h",
        (
            max(-32768, min(32767, round(float(value) * 32767)))
            for value in values.detach().cpu().clamp(-1, 1)
        ),
    )
    if sys.byteorder != "little":
        samples.byteswap()
    with wave.open(str(output), "wb") as destination:
        destination.setnchannels(1)
        destination.setsampwidth(2)
        destination.setframerate(sample_rate)
        destination.writeframes(samples.tobytes())


def resample(values: torch.Tensor, samples: int) -> torch.Tensor:
    if values.numel() == samples:
        return values
    return F.interpolate(values.view(1, 1, -1), size=samples, mode="linear", align_corners=False).view(-1)


def estimate_f0(frame: torch.Tensor, sample_rate: int) -> float:
    """短い有声区間のF0を自己相関から推定する。"""
    values = frame.detach().cpu().numpy().astype(np.float64, copy=False)
    if values.size < 32 or float(np.sqrt(np.mean(values * values))) < 0.003:
        return 0.0
    values = values - np.mean(values)
    if float(np.sqrt(np.mean(values * values))) < 0.003:
        return 0.0
    values = values * np.hanning(values.size)
    minimum_lag = max(2, sample_rate // 500)
    maximum_lag = min(values.size // 2, sample_rate // 60)
    if minimum_lag >= maximum_lag:
        return 0.0
    fft_size = 1 << (2 * values.size - 1).bit_length()
    spectrum = np.fft.rfft(values, fft_size)
    correlation = np.fft.irfft(spectrum * np.conjugate(spectrum), fft_size)[: maximum_lag + 1]
    if correlation[0] <= 1e-9:
        return 0.0
    correlation /= correlation[0]
    peaks = [
        lag for lag in range(minimum_lag + 1, maximum_lag)
        if correlation[lag] >= correlation[lag - 1] and correlation[lag] > correlation[lag + 1]
    ]
    if not peaks:
        return 0.0
    best = max(peaks, key=lambda lag: correlation[lag])
    if correlation[best] < 0.2:
        return 0.0
    strong = [lag for lag in peaks if correlation[lag] >= correlation[best] * 0.9]
    lag = float(min(strong))
    integer_lag = int(lag)
    if minimum_lag < integer_lag < maximum_lag:
        left, center, right = correlation[integer_lag - 1 : integer_lag + 2]
        denominator = left - 2 * center + right
        if abs(denominator) > 1e-12:
            lag += 0.5 * (left - right) / denominator
    return sample_rate / lag


def estimate_f0_median(wave: torch.Tensor, sample_rate: int) -> float:
    window = round(0.04 * sample_rate)
    hop = max(1, round(0.01 * sample_rate))
    if wave.numel() < window:
        return estimate_f0(wave, sample_rate)
    values = [
        estimate_f0(wave[start : start + window], sample_rate)
        for start in range(0, wave.numel() - window + 1, hop)
    ]
    voiced = sorted(value for value in values if value > 0)
    if not voiced:
        return 0.0
    middle = len(voiced) // 2
    if len(voiced) % 2:
        return voiced[middle]
    return (voiced[middle - 1] + voiced[middle]) / 2


def fold_f0(value: float, reference: float, octave_ratio: float = 1.6) -> float:
    """倍音・サブハーモニック誤検出をreferenceと同じoctaveへ寄せる。"""
    if value <= 0 or reference <= 0:
        return value
    while value > reference * octave_ratio:
        value /= 2
    while value < reference / octave_ratio:
        value *= 2
    return value


def pitch_marks(wave: torch.Tensor, sample_rate: int, f0: float) -> list[int]:
    if f0 <= 0 or wave.numel() < 16:
        return []
    period = sample_rate / f0
    center = wave.numel() // 2
    radius = max(2, round(period * 0.5))
    anchor_start = max(0, center - radius)
    anchor_end = min(wave.numel(), center + radius + 1)
    anchor = anchor_start + int(wave[anchor_start:anchor_end].abs().argmax())
    marks = [anchor]
    search = max(2, round(period * 0.22))
    current = anchor
    while current + period < wave.numel():
        expected = round(current + period)
        start, end = max(current + 1, expected - search), min(wave.numel(), expected + search + 1)
        if end <= start:
            break
        current = start + int(wave[start:end].abs().argmax())
        marks.append(current)
    current = anchor
    before = []
    while current - period >= 0:
        expected = round(current - period)
        start, end = max(0, expected - search), min(current, expected + search + 1)
        if end <= start:
            break
        current = start + int(wave[start:end].abs().argmax())
        before.append(current)
    return list(reversed(before)) + marks


def td_psola(
    source: torch.Tensor,
    target_samples: int,
    sample_rate: int,
    source_f0: float,
    target_f0: float,
) -> torch.Tensor:
    """一定F0の短い有声区間をpitch-synchronous overlap-addで伸縮する。"""
    if target_samples < 2 or source_f0 <= 0 or target_f0 <= 0:
        return resample(source, max(2, target_samples))
    marks = pitch_marks(source, sample_rate, source_f0)
    if len(marks) < 2:
        return resample(source, target_samples)
    target_period = sample_rate / target_f0
    target_marks = []
    position = target_period * 0.5
    while position < target_samples:
        target_marks.append(round(position))
        position += target_period
    if not target_marks:
        return resample(source, target_samples)
    output = torch.zeros(target_samples, dtype=source.dtype, device=source.device)
    weights = torch.zeros_like(output)
    grain_radius = max(4, round(sample_rate / source_f0 * 0.9))
    window = torch.hann_window(grain_radius * 2 + 1, periodic=False, device=source.device)
    for target_mark in target_marks:
        progress = target_mark / max(1, target_samples - 1)
        desired_source = progress * (source.numel() - 1)
        insertion = bisect.bisect_left(marks, desired_source)
        options = marks[max(0, insertion - 1) : min(len(marks), insertion + 1)]
        source_mark = min(options, key=lambda mark: abs(mark - desired_source))
        source_start, source_end = source_mark - grain_radius, source_mark + grain_radius + 1
        target_start, target_end = target_mark - grain_radius, target_mark + grain_radius + 1
        clip_left = max(0, -source_start, -target_start)
        clip_right = max(0, source_end - source.numel(), target_end - target_samples)
        length = grain_radius * 2 + 1 - clip_left - clip_right
        if length <= 0:
            continue
        source_slice = slice(source_start + clip_left, source_start + clip_left + length)
        target_slice = slice(target_start + clip_left, target_start + clip_left + length)
        grain_window = window[clip_left : clip_left + length]
        output[target_slice] += source[source_slice] * grain_window
        weights[target_slice] += grain_window
    fallback = resample(source, target_samples)
    return torch.where(weights > 1e-4, output / weights.clamp_min(1e-4), fallback)


def load_index(path: str | Path) -> dict:
    data = json.loads(Path(path).read_text(encoding="utf-8"))
    if data.get("version") != 1:
        raise ValueError("unsupported pseudo voicebank index")
    return data


def candidate_key(segment: dict) -> str:
    return str(segment["phone"])


def build_candidate_pool(index: dict, split: str = "train") -> dict[str, list[tuple[dict, dict]]]:
    result: dict[str, list[tuple[dict, dict]]] = {}
    for utterance in index["utterances"]:
        if utterance["split"] != split:
            continue
        for segment in utterance["segments"]:
            if segment["phone"] in SILENCE_PHONES:
                continue
            result.setdefault(candidate_key(segment), []).append((utterance, segment))
    for candidates in result.values():
        candidates.sort(key=lambda item: (item[0]["id"], item[1]["index"]))
    return result


def diphone_key(left: dict, right: dict) -> str:
    return f"{left['phone']}->{right['phone']}"


def build_diphone_pool(index: dict, split: str = "train") -> dict[str, list[tuple[dict, dict, dict]]]:
    result: dict[str, list[tuple[dict, dict, dict]]] = {}
    for utterance in index["utterances"]:
        if utterance["split"] != split:
            continue
        segments = utterance["segments"]
        for position in range(len(segments) - 1):
            left, right = segments[position], segments[position + 1]
            result.setdefault(diphone_key(left, right), []).append((utterance, left, right))
    for candidates in result.values():
        candidates.sort(key=lambda item: (item[0]["id"], item[1]["index"]))
    return result


def ngram_key(segments: list[dict]) -> str:
    return "->".join(str(segment["phone"]) for segment in segments)


def build_ngram_pool(index: dict, size: int, split: str = "train") -> dict[str, list[tuple[dict, list[dict]]]]:
    if size < 2:
        raise ValueError("ngram size must be at least 2")
    result: dict[str, list[tuple[dict, list[dict]]]] = {}
    for utterance in index["utterances"]:
        if utterance["split"] != split:
            continue
        segments = utterance["segments"]
        for position in range(len(segments) - size + 1):
            unit = segments[position : position + size]
            result.setdefault(ngram_key(unit), []).append((utterance, unit))
    for candidates in result.values():
        candidates.sort(key=lambda item: (item[0]["id"], item[1][0]["index"]))
    return result
