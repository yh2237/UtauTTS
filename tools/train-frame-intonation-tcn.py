#!/usr/bin/env python3
"""Train a frame-level (10 ms) intonation TCN and export portable JSON.

The mora-level trainer in :mod:`train-intonation-tcn` learns one value per
token.  This trainer keeps the same small residual TCN and sparse linguistic
feature representation, but expands each JSUT token to a 10 ms frame grid and
uses an F0 track measured from the corresponding ``audio_path``. The target
is a smoothed utterance-relative log-F0 command in cents. Pitch is interpolated
across unvoiced consonants, while pause frames are excluded from the loss so
training and text-only inference share the same mask.

The preferred F0 extractor is the ``F0`` entry point exported by the local
WORLDLINE DLL.  The pure-Python/numpy autocorrelation extractor is deliberately
kept as a fallback so that dataset preparation and small smoke tests do not
depend on a particular native build.

The exported model is inference-oriented JSON; it does not contain a Python
pickle or a torch checkpoint.  Its ``frame_pitch`` object mirrors the
``sequence_pitch`` object produced by the older trainer and adds the frame
period and cent bounds needed by a Go implementation.
"""

from __future__ import annotations

import argparse
import ctypes
import json
import math
import random
import struct
import sys
import wave
from pathlib import Path
from typing import Callable, Iterable, Sequence

import numpy as np
import torch
from torch import nn
from torch.nn import functional as F

from torch_device import device_description, move_batch, resolve_device


FRAME_MS = 10.0
# 初期モデルはレンダラーの耐性確認前なので抑揚幅を控えめにする。
DEFAULT_LOW_CENTS = -250.0
DEFAULT_HIGH_CENTS = 250.0
DEFAULT_FMIN_HZ = 50.0
DEFAULT_FMAX_HZ = 600.0


def fnv1a(text: str) -> int:
    """Return the stable FNV-1a hash used for all utterance splits."""

    value = 2166136261
    for byte in text.encode("utf-8"):
        value ^= byte
        value = (value * 16777619) & 0xFFFFFFFF
    return value


def deterministic_split(records: Sequence[dict]) -> tuple[list[dict], list[dict]]:
    """Split utterances by id, keeping the split independent of file order.

    Normal JSUT corpora use ``hash(id) % 10 == 0`` as validation.  The small
    fallback for a tiny synthetic corpus keeps the command useful in smoke
    tests while remaining deterministic and disjoint whenever there are at
    least two records.
    """

    train = [record for record in records if fnv1a(str(record["id"])) % 10 != 0]
    validation = [record for record in records if fnv1a(str(record["id"])) % 10 == 0]
    if not train and len(records) > 1:
        ordered = sorted(records, key=lambda item: (fnv1a(str(item["id"])), str(item["id"])))
        validation = ordered[:1]
        train = ordered[1:]
    elif not validation and len(records) > 1:
        ordered = sorted(records, key=lambda item: (fnv1a(str(item["id"])), str(item["id"])))
        validation = ordered[:1]
        held_out = {id(item) for item in validation}
        train = [item for item in records if id(item) not in held_out]
    elif len(records) == 1:
        # 1発話では分離できないため、学習JSONへリークを明記して兼用する。
        train = list(records)
        validation = list(records)
    if not train or not validation:
        raise ValueError("dataset must contain at least one train and validation utterance")
    return train, validation


def load_records(path: str | Path, limit: int = 0) -> tuple[list[dict], list[dict]]:
    """Read version-1 JSUT JSONL and return deterministic train/validation."""

    records: list[dict] = []
    source_path = Path(path)
    with source_path.open("r", encoding="utf-8") as source:
        for line_number, line in enumerate(source, 1):
            if not line.strip():
                continue
            try:
                record = json.loads(line)
            except json.JSONDecodeError as error:
                raise ValueError(f"{path}:{line_number}: invalid JSON: {error}") from error
            if record.get("version") != 1:
                raise ValueError(f"{path}:{line_number}: unsupported dataset version")
            if not record.get("id"):
                raise ValueError(f"{path}:{line_number}: missing id")
            if not isinstance(record.get("tokens"), list) or not record["tokens"]:
                raise ValueError(f"{path}:{line_number}: missing tokens")
            if not record.get("audio_path"):
                raise ValueError(f"{path}:{line_number}: missing audio_path")
            records.append(record)
    if not records:
        raise ValueError(f"{path}: dataset is empty")
    if limit > 0:
        records = records[:limit]
    return deterministic_split(records)


def _add_categorical(result: dict[str, float], prefix: str, token: dict) -> None:
    if token.get("pause", False):
        result[f"{prefix}=<PAUSE>"] = 1.0
        return
    result[f"{prefix}={token.get('mora', '')}"] = 1.0
    result[f"{prefix}_vowel={token.get('vowel', '')}"] = 1.0


def token_features(tokens: Sequence[dict], position: int) -> dict[str, float]:
    """Build the reproducible categorical/accent features shared with Go.

    A frame receives the features of the mora containing its midpoint.  The
    feature names intentionally match the older trainer: a Go inference path
    can generate the same sparse vector without importing this script.
    """

    current = tokens[position]
    denominator = max(1, len(tokens) - 1)
    token_position = position / denominator
    result: dict[str, float] = {
        "bias": 1.0,
        "position": token_position,
        "position2": token_position * token_position,
        "from_end": 1.0 - token_position,
    }
    if position == 0 or tokens[position - 1].get("pause", False):
        result["phrase_start"] = 1.0
    if position == len(tokens) - 1 or tokens[position + 1].get("pause", False):
        result["phrase_end"] = 1.0
    _add_categorical(result, "mora", current)
    if position > 0:
        _add_categorical(result, "prev", tokens[position - 1])
    else:
        result["prev=<BOS>"] = 1.0
    if position + 1 < len(tokens):
        _add_categorical(result, "next", tokens[position + 1])
    else:
        result["next=<EOS>"] = 1.0

    # Open JTalkがなくても特徴名を固定し、未注釈値は0とする。
    phrase_length = max(1, int(current.get("accent_phrase_length", len(tokens))))
    phrase_position = int(current.get("accent_phrase_position", position + 1))
    nucleus = int(current.get("accent_nucleus", 0))
    result["accent_position"] = phrase_position / phrase_length
    result["accent_from_end"] = (phrase_length - phrase_position) / phrase_length
    result["accent_nucleus_position"] = nucleus / phrase_length
    result["accent_high"] = float(bool(current.get("accent_high", False)))
    result["accent_phrase_start"] = float(bool(current.get("accent_phrase_start", position == 0)))
    result["accent_phrase_end"] = float(
        bool(current.get("accent_phrase_end", position == len(tokens) - 1))
    )
    result["word_start"] = float(bool(current.get("word_start", False)))
    result["word_end"] = float(bool(current.get("word_end", False)))
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


def _fallback_accent(tokens: Sequence[dict]) -> list[dict]:
    """Fill stable accent fields when pyopenjtalk is not installed/aligned."""

    result: list[dict] = []
    phrase: list[dict] = []

    def flush() -> None:
        if not phrase:
            return
        length = len(phrase)
        for index, token in enumerate(phrase, 1):
            copied = dict(token)
            copied.update(
                {
                    "accent_phrase_position": index,
                    "accent_phrase_length": length,
                    "accent_nucleus": 0,
                    "accent_high": index >= 2,
                    "accent_phrase_start": index == 1,
                    "accent_phrase_end": index == length,
                    "word_start": index == 1,
                    "word_end": index == length,
                    "pos": copied.get("pos", "*"),
                    "pos_group1": copied.get("pos_group1", "*"),
                }
            )
            result.append(copied)
        phrase.clear()

    for token in tokens:
        if token.get("pause", False):
            flush()
            result.append(dict(token))
        else:
            phrase.append(dict(token))
    flush()
    return result


def _ensure_accent_fields(tokens: Sequence[dict]) -> list[dict]:
    """Fill only absent accent fields, preserving an Open JTalk analysis."""

    fallback = _fallback_accent(tokens)
    result = []
    for source, defaults in zip(tokens, fallback):
        copied = dict(source)
        for name, value in defaults.items():
            copied.setdefault(name, value)
        result.append(copied)
    return result


def add_openjtalk_features(
    records: Sequence[dict],
    enabled: bool = True,
    stats: dict | None = None,
    min_alignment_rate: float = 0.60,
) -> list[dict]:
    """Annotate records with Open JTalk accent features when available.

    JSUT files already contain mora boundaries.  Open JTalk is only accepted
    when its mora and pause sequence aligns exactly.  Mismatched records are
    skipped (never silently replaced by fallback accents); the aggregate
    alignment rate must clear the configured minimum.
    """

    if not enabled:
        if stats is not None:
            stats.update({"alignment_records": 0, "alignment_moras": 0, "skipped_records": 0, "alignment_rate": 0.0, "fallback_records": len(records)})
        return [dict(record, tokens=_fallback_accent(record["tokens"])) for record in records]
    try:
        from openjtalk_features import analyze
    except Exception as error:
        raise RuntimeError(
            "--openjtalk-accent requires pyopenjtalk and Open JTalk; "
            "use --no-openjtalk-accent only for a fallback run"
        ) from error

    result: list[dict] = []
    accent_types: dict[str, int] = {}
    pos_values: dict[str, int] = {}
    pos_groups: dict[str, int] = {}
    aligned_moras = 0
    skipped_records = 0
    for record in records:
        source_tokens = record["tokens"]
        try:
            _, annotated = analyze(record["text"])
        except Exception as error:
            raise RuntimeError(f"Open JTalk analysis failed for {record.get('id', '<unknown>')}") from error
        aligned = len(annotated) == len(source_tokens) and all(
            bool(left.get("pause")) == bool(right.get("pause"))
            and str(left.get("mora", "")) == str(right.get("mora", ""))
            for left, right in zip(annotated, source_tokens)
        )
        if not aligned:
            skipped_records += 1
            continue
        annotated_tokens = []
        for source, linguistic in zip(source_tokens, annotated):
            copied = dict(source)
            for name, value in linguistic.items():
                if name not in {"mora", "pause"}:
                    copied[name] = value
            annotated_tokens.append(copied)
            if not copied.get("pause", False):
                aligned_moras += 1
                nucleus = int(copied.get("accent_nucleus", 0))
                position = int(copied.get("accent_phrase_position", 0))
                kind = "heiban" if nucleus == 0 else ("before" if position < nucleus else ("nucleus" if position == nucleus else "after"))
                accent_types[kind] = accent_types.get(kind, 0) + 1
                pos = str(copied.get("pos", "*"))
                group = str(copied.get("pos_group1", "*"))
                pos_values[pos] = pos_values.get(pos, 0) + 1
                pos_groups[group] = pos_groups.get(group, 0) + 1
        result.append(dict(record, tokens=annotated_tokens))
    if stats is not None:
        stats.update(
            {
                "alignment_records": len(result),
                "alignment_moras": aligned_moras,
                "skipped_records": skipped_records,
                "alignment_rate": len(result) / max(1, len(records)),
                "fallback_records": 0,
                "accent_type_counts": accent_types,
                "pos_counts": pos_values,
                "pos_group1_counts": pos_groups,
            }
        )
    alignment_rate = len(result) / max(1, len(records))
    if records and alignment_rate < min_alignment_rate:
        raise ValueError(
            f"Open JTalk alignment rate {alignment_rate:.3f} is below minimum {min_alignment_rate:.3f}"
        )
    return result


def _resolve_audio_path(path: str | Path, dataset_path: str | Path | None = None, audio_root: str | Path | None = None) -> Path:
    raw = Path(str(path))
    candidates: list[Path] = []
    if raw.is_absolute():
        candidates.append(raw)
    else:
        # POSIX実行時もJSONL内のWindowsパスを解決する。
        normalized = Path(str(path).replace("\\", "/"))
        candidates.extend([raw, normalized])
        if audio_root:
            candidates.extend([Path(audio_root) / raw, Path(audio_root) / normalized])
        if dataset_path:
            base = Path(dataset_path).resolve().parent
            candidates.extend([base / raw, base / normalized, base.parent / raw, base.parent / normalized])
            for parent in [base, *base.parents]:
                candidates.extend([parent / raw, parent / normalized])
    for candidate in candidates:
        if candidate.exists() and candidate.is_file():
            return candidate.resolve()
    # 同名の別ファイルを選ばず、エラー表示に最も有用な候補を返す。
    return candidates[0] if candidates else raw


def read_wav(path: str | Path) -> tuple[np.ndarray, int]:
    """Read PCM WAV into mono float32 samples in [-1, 1]."""

    with wave.open(str(path), "rb") as source:
        channels = source.getnchannels()
        sample_width = source.getsampwidth()
        sample_rate = source.getframerate()
        count = source.getnframes()
        raw = source.readframes(count)
    if channels <= 0 or sample_rate <= 0:
        raise ValueError(f"{path}: invalid WAV format")
    if sample_width == 1:
        values = (np.frombuffer(raw, dtype=np.uint8).astype(np.float32) - 128.0) / 128.0
    elif sample_width == 2:
        values = np.frombuffer(raw, dtype="<i2").astype(np.float32) / 32768.0
    elif sample_width == 3:
        packed = np.frombuffer(raw, dtype=np.uint8).reshape(-1, 3)
        integers = (
            packed[:, 0].astype(np.int32)
            | (packed[:, 1].astype(np.int32) << 8)
            | (packed[:, 2].astype(np.int32) << 16)
        )
        integers = np.where(integers & 0x800000, integers - 0x1000000, integers)
        values = integers.astype(np.float32) / 8388608.0
    elif sample_width == 4:
        values = np.frombuffer(raw, dtype="<i4").astype(np.float32) / 2147483648.0
    else:
        raise ValueError(f"{path}: unsupported PCM sample width {sample_width}")
    if len(values) % channels:
        values = values[: len(values) - (len(values) % channels)]
    if channels > 1:
        values = values.reshape(-1, channels).mean(axis=1)
    return np.asarray(values, dtype=np.float32), sample_rate


class WorldlineF0:
    """Small ctypes adapter for worldline.dll's exported F0 function."""

    def __init__(self, library_path: str | Path, method: int = 1):
        self.path = Path(library_path)
        loader = getattr(ctypes, "WinDLL", ctypes.CDLL)
        self.library = loader(str(self.path))
        self.method = int(method)
        self._f0 = self.library.F0
        self._f0.argtypes = [
            ctypes.POINTER(ctypes.c_float),
            ctypes.c_int,
            ctypes.c_int,
            ctypes.c_double,
            ctypes.c_int,
            ctypes.POINTER(ctypes.POINTER(ctypes.c_double)),
        ]
        self._f0.restype = ctypes.c_int

    def extract(self, samples: np.ndarray, sample_rate: int, frame_ms: float = FRAME_MS) -> np.ndarray:
        values = np.ascontiguousarray(samples, dtype=np.float32)
        if values.size == 0:
            return np.zeros(0, dtype=np.float64)
        sample_buffer = (ctypes.c_float * int(values.size)).from_buffer_copy(values)
        output = ctypes.POINTER(ctypes.c_double)()
        count = int(
            self._f0(
                sample_buffer,
                int(values.size),
                int(sample_rate),
                float(frame_ms),
                self.method,
                ctypes.byref(output),
            )
        )
        if not output:
            return np.zeros(0, dtype=np.float64)
        try:
            if count <= 0:
                return np.zeros(0, dtype=np.float64)
            return np.ctypeslib.as_array(output, shape=(count,)).copy()
        finally:
            # F0バッファを解放し、大規模学習時の蓄積を防ぐ。
            ole32 = getattr(getattr(ctypes, "windll", None), "ole32", None)
            free = getattr(ole32, "CoTaskMemFree", None)
            if free is not None:
                free.argtypes = [ctypes.c_void_p]
                free.restype = None
                free(output)


def load_worldline(path: str | Path | None = None, method: int = 1) -> WorldlineF0 | None:
    """Try a configured/local worldline library, returning None on failure."""

    candidates: list[Path] = []
    if path:
        candidates.append(Path(path))
    here = Path(__file__).resolve().parents[1]
    candidates.extend(
        [
            here / "release" / "UtauTTS" / "runtime" / "worldline.dll",
            here / "release" / "UtauTTS-Server" / "runtime" / "worldline.dll",
            here / "release" / "UtauTTS" / "worldline.dll",
            here / "release" / "UtauTTS-Server" / "worldline.dll",
        ]
    )
    for candidate in candidates:
        if not candidate.exists():
            continue
        try:
            return WorldlineF0(candidate, method)
        except (OSError, AttributeError, ctypes.ArgumentError):
            continue
    return None


def _autocorrelation_pitch(windowed: np.ndarray, sample_rate: int, fmin: float, fmax: float) -> float:
    if len(windowed) < 4:
        return 0.0
    energy = float(np.dot(windowed, windowed))
    if energy <= 1e-10:
        return 0.0
    minimum_lag = max(1, int(sample_rate / fmax))
    maximum_lag = min(len(windowed) - 2, int(sample_rate / fmin))
    if maximum_lag <= minimum_lag:
        return 0.0
    # ネイティブ処理失敗時も全JSUTを処理できるようFFT自己相関を使う。
    fft_size = 1 << (2 * len(windowed) - 1).bit_length()
    spectrum = np.fft.rfft(windowed, fft_size)
    correlation = np.fft.irfft(spectrum * np.conj(spectrum), fft_size)[: len(windowed)]
    squared = windowed * windowed
    prefix = np.concatenate(([0.0], np.cumsum(squared)))
    lags = np.arange(minimum_lag, maximum_lag + 1, dtype=np.int64)
    left_energy = prefix[len(windowed)] - prefix[lags]
    right_energy = prefix[len(windowed) - lags]
    denominator = np.sqrt(np.maximum(1e-20, left_energy * right_energy))
    correlations = correlation[lags] / denominator
    best_offset = int(np.argmax(correlations))
    best = float(correlations[best_offset])
    # 低相関は無声として除外し、voiced maskのノイズを抑える。
    if best < 0.30:
        return 0.0
    lag = float(minimum_lag + best_offset)
    if 0 < best_offset < len(correlations) - 1:
        left, middle, right = correlations[best_offset - 1 : best_offset + 2]
        denominator = left - 2.0 * middle + right
        if abs(denominator) > 1e-9:
            lag += 0.5 * (left - right) / denominator
    return float(sample_rate / max(1.0, lag))


def extract_f0_internal(
    samples: np.ndarray,
    sample_rate: int,
    frame_ms: float = FRAME_MS,
    fmin: float = DEFAULT_FMIN_HZ,
    fmax: float = DEFAULT_FMAX_HZ,
) -> np.ndarray:
    """Extract a conservative 10 ms F0 track using windowed autocorrelation."""

    hop = max(1, int(round(sample_rate * frame_ms / 1000.0)))
    window = max(hop * 4, int(round(sample_rate * 0.040)))
    if window % 2:
        window += 1
    half = window // 2
    padded = np.pad(np.asarray(samples, dtype=np.float64), (half, half), mode="constant")
    hamming = np.hamming(window)
    frame_count = max(1, int(math.ceil(len(samples) / hop)))
    result = np.zeros(frame_count, dtype=np.float64)
    frame_energy = np.zeros(frame_count, dtype=np.float64)
    for index in range(frame_count):
        center = index * hop + half
        segment = padded[center - half : center + half]
        if len(segment) != window:
            segment = np.pad(segment, (0, window - len(segment)))
        segment = segment - float(np.mean(segment))
        windowed = segment * hamming
        frame_energy[index] = float(np.sqrt(np.mean(windowed * windowed)))
        result[index] = _autocorrelation_pitch(windowed, sample_rate, fmin, fmax)
    nonzero_energy = frame_energy[frame_energy > 1e-8]
    if len(nonzero_energy):
        threshold = max(1e-5, float(np.percentile(nonzero_energy, 15)) * 0.35)
        result[frame_energy < threshold] = 0.0
    return result


def _interpolate_track(
    values: np.ndarray,
    source_times_ms: np.ndarray,
    query_ms: np.ndarray,
    frame_ms: float,
) -> np.ndarray:
    """Interpolate log-F0 only inside explicitly voiced islands.

    WORLD's exported array has no timestamp side channel, so its timestamps
    are constructed explicitly from the requested frame period.  Sampling by
    rounded indices can duplicate or skip frames when periods differ; this
    routine uses the actual source/query times and never bridges an unvoiced
    island.
    """

    values = np.asarray(values, dtype=np.float64)
    source_times_ms = np.asarray(source_times_ms, dtype=np.float64)
    query_ms = np.asarray(query_ms, dtype=np.float64)
    result = np.zeros(len(query_ms), dtype=np.float64)
    if len(values) == 0:
        return result
    if len(source_times_ms) != len(values):
        source_times_ms = np.arange(len(values), dtype=np.float64) * frame_ms
    valid = values > 0
    index = 0
    while index < len(values):
        if not valid[index]:
            index += 1
            continue
        end = index + 1
        while end < len(values) and valid[end]:
            end += 1
        island_times = source_times_ms[index:end]
        island_values = np.log(np.maximum(values[index:end], 1e-6))
        inside = (query_ms >= island_times[0]) & (query_ms <= island_times[-1])
        if inside.any():
            result[inside] = np.exp(np.interp(query_ms[inside], island_times, island_values))
        index = end
    return result


def _frame_token_index(tokens: Sequence[dict], time_ms: float) -> int:
    if not tokens:
        return -1
    for index, token in enumerate(tokens):
        start = float(token.get("start_ms", 0.0))
        end = float(token.get("end_ms", start + token.get("duration_ms", 0.0)))
        if start <= time_ms < max(start + 1e-6, end):
            return index
    if time_ms < float(tokens[0].get("start_ms", 0.0)):
        return 0
    return len(tokens) - 1


def utterance_frame_times(record: dict, frame_ms: float = FRAME_MS) -> np.ndarray:
    start = float(record.get("start_ms", record["tokens"][0].get("start_ms", 0.0)))
    end = float(record.get("end_ms", record["tokens"][-1].get("end_ms", start)))
    if not math.isfinite(start) or not math.isfinite(end) or end <= start:
        starts = [float(token.get("start_ms", 0.0)) for token in record["tokens"]]
        ends = [float(token.get("end_ms", start)) for token in record["tokens"]]
        start, end = min(starts, default=0.0), max(ends, default=start + frame_ms)
    count = max(1, int(math.ceil((end - start) / frame_ms)))
    return start + np.arange(count, dtype=np.float64) * frame_ms + frame_ms * 0.5


def extract_record_f0(
    record: dict,
    dataset_path: str | Path | None = None,
    audio_root: str | Path | None = None,
    frame_ms: float = FRAME_MS,
    worldline: WorldlineF0 | None = None,
    f0_provider: Callable[[np.ndarray, int, float], np.ndarray] | None = None,
) -> tuple[np.ndarray, np.ndarray, np.ndarray]:
    """Return frame times, F0 Hz, and frame token indices for one record."""

    audio_path = _resolve_audio_path(record["audio_path"], dataset_path, audio_root)
    if not audio_path.exists():
        raise FileNotFoundError(f"audio file not found: {audio_path} (from {record['audio_path']})")
    samples, sample_rate = read_wav(audio_path)
    frame_times = utterance_frame_times(record, frame_ms)
    if f0_provider is not None:
        track = np.asarray(f0_provider(samples, sample_rate, frame_ms), dtype=np.float64)
        # 注入providerは要求グリッドまたは音声全体の系列を返せる。
        if len(track) == len(frame_times):
            f0 = track
        else:
            f0 = _interpolate_track(track, np.arange(len(track), dtype=np.float64) * frame_ms, frame_times, frame_ms)
    elif worldline is not None:
        track = worldline.extract(samples, sample_rate, frame_ms)
        f0 = _interpolate_track(track, np.arange(len(track), dtype=np.float64) * frame_ms, frame_times, frame_ms)
    else:
        track = extract_f0_internal(samples, sample_rate, frame_ms)
        f0 = _interpolate_track(track, np.arange(len(track), dtype=np.float64) * frame_ms, frame_times, frame_ms)
    token_indices = np.array([_frame_token_index(record["tokens"], time) for time in frame_times], dtype=np.int64)
    for index, token_index in enumerate(token_indices):
        if token_index < 0 or record["tokens"][int(token_index)].get("pause", False):
            f0[index] = 0.0
    return frame_times, np.maximum(f0, 0.0), token_indices


def frame_features(
    tokens: Sequence[dict],
    token_index: int,
    frame_time_ms: float,
    utterance_start_ms: float,
    utterance_end_ms: float,
    question: bool = False,
) -> dict[str, float]:
    """Expand token features with normalized frame/mora progress."""

    if token_index < 0 or token_index >= len(tokens):
        token_index = max(0, min(len(tokens) - 1, token_index))
    result = token_features(tokens, token_index)
    token = tokens[token_index]
    start = float(token.get("start_ms", utterance_start_ms))
    end = float(token.get("end_ms", start + token.get("duration_ms", 0.0)))
    denominator = max(1.0, end - start)
    progress = min(1.0, max(0.0, (frame_time_ms - start) / denominator))
    total = max(1.0, utterance_end_ms - utterance_start_ms)
    position = min(1.0, max(0.0, (frame_time_ms - utterance_start_ms) / total))
    result["mora_progress"] = progress
    result["mora_progress2"] = progress * progress
    result["frame_position"] = position
    result["frame_from_end"] = 1.0 - position
    result["question_distance"] = 1.0 - position if question else 0.0
    result["final_distance"] = 1.0 - position
    return result


def build_feature_index(records: Sequence[dict], frame_ms: float = FRAME_MS) -> dict[str, int]:
    """Collect and lexicographically index every frame feature name."""

    names: set[str] = set()
    for record in records:
        times = utterance_frame_times(record, frame_ms)
        start = float(times[0] - frame_ms * 0.5)
        end = float(times[-1] + frame_ms * 0.5)
        for time in times:
            index = _frame_token_index(record["tokens"], float(time))
            names.update(
                frame_features(
                    record["tokens"], index, float(time), start, end,
                    question=("?" in str(record.get("text", "")) or "？" in str(record.get("text", ""))),
                )
            )
    return {name: index for index, name in enumerate(sorted(names))}


def mora_feature_index(records: Sequence[dict]) -> dict[str, int]:
    """Index only the mora-level token features (no frame/mora-progress names).

    ``build_feature_index`` additionally collects the frame-only columns that
    ``frame_features`` appends (mora_progress, frame_position, ...).  Those are
    never produced by ``token_features`` and are always zero for a mora-level
    head, so heads like the prosody multitask duration predictor should train
    and export against this smaller index instead.
    """

    names: set[str] = set()
    for record in records:
        tokens = record["tokens"]
        for position in range(len(tokens)):
            names.update(token_features(tokens, position))
    return {name: index for index, name in enumerate(sorted(names))}


def _record_target_cents(f0: np.ndarray, mask: np.ndarray, low_cents: float, high_cents: float, fallback: float = 200.0) -> np.ndarray:
    voiced = f0[mask & (f0 > 0)]
    baseline = float(np.median(voiced)) if len(voiced) else fallback
    baseline = max(1.0, baseline)
    target = np.zeros(len(f0), dtype=np.float64)
    valid = mask & (f0 > 0)
    target[valid] = 1200.0 * np.log2(np.maximum(1.0, f0[valid]) / baseline)
    if valid.any():
        target[valid] -= float(np.median(target[valid]))
    return np.clip(target, low_cents, high_cents)


def macro_log_f0(
    f0: np.ndarray,
    frame_ms: float,
    speech_mask: np.ndarray | None = None,
    max_gap_ms: float = 30.0,
    smooth_ms: float = 40.0,
) -> np.ndarray:
    """Interpolate a pitch command over speech and smooth each phrase.

    At inference there is no acoustic voiced mask.  Supplying ``speech_mask``
    therefore fills every non-pause island from its measured voiced frames,
    making training, evaluation and the Go runtime use the same mask.
    """

    values = np.asarray(f0, dtype=np.float64)
    result = np.zeros(len(values), dtype=np.float64)
    speech = np.ones(len(values), dtype=bool) if speech_mask is None else np.asarray(speech_mask, dtype=bool)
    valid = (values > 0) & speech
    if not valid.any():
        return result
    log_values = np.full(len(values), np.nan, dtype=np.float64)
    log_values[valid] = np.log(values[valid])
    index = 0
    while index < len(values):
        if not speech[index]:
            index += 1
            continue
        end = index + 1
        while end < len(values) and speech[end]:
            end += 1
        positions = np.arange(index, end)
        measured = positions[valid[index:end]]
        if len(measured):
            # 制御輪郭なので、無声区間と句端は最近傍の有声F0で補間する。
            phrase = np.interp(positions, measured, log_values[measured])
            width = min(len(phrase), max(1, int(round(smooth_ms / max(frame_ms, 1e-6)))))
            if width % 2 == 0:
                width = max(1, width - 1)
            if width > 1:
                pad = width // 2
                padded = np.pad(phrase, (pad, pad), mode="edge")
                phrase = np.convolve(padded, np.ones(width, dtype=np.float64) / width, mode="valid")
            result[index:end] = np.exp(phrase)
        index = end
    return result


def prepare(
    records: Sequence[dict],
    feature_index: dict[str, int] | None = None,
    *,
    dataset_path: str | Path | None = None,
    audio_root: str | Path | None = None,
    frame_ms: float = FRAME_MS,
    low_cents: float = DEFAULT_LOW_CENTS,
    high_cents: float = DEFAULT_HIGH_CENTS,
    target_scale: float = 1.0,
    worldline: WorldlineF0 | None = None,
    f0_provider: Callable[[np.ndarray, int, float], np.ndarray] | None = None,
) -> tuple[list[tuple[list[list[tuple[int, float]]], list[float], list[bool], list[float]]], dict[str, int]]:
    """Materialize sparse frame sequences and voiced-mask targets."""

    if low_cents >= high_cents:
        raise ValueError("low_cents must be smaller than high_cents")
    if feature_index is None:
        feature_index = build_feature_index(records, frame_ms)
    prepared = []
    for record in records:
        frame_times, f0, token_indices = extract_record_f0(
            record,
            dataset_path=dataset_path,
            audio_root=audio_root,
            frame_ms=frame_ms,
            worldline=worldline,
            f0_provider=f0_provider,
        )
        start = float(frame_times[0] - frame_ms * 0.5)
        end = float(frame_times[-1] + frame_ms * 0.5)
        speech_mask = np.array(
            [
                0 <= int(token_index) < len(record["tokens"])
                and not record["tokens"][int(token_index)].get("pause", False)
                for index, token_index in enumerate(token_indices)
            ],
            dtype=bool,
        )
        macro_f0 = macro_log_f0(f0, frame_ms, speech_mask=speech_mask)
        mask = speech_mask & (macro_f0 > 0)
        targets = _record_target_cents(
            macro_f0,
            mask,
            low_cents,
            high_cents,
            fallback=float(record.get("median_f0_hz", 200.0) or 200.0),
        )
        sequence: list[list[tuple[int, float]]] = []
        for time, token_index in zip(frame_times, token_indices):
            sparse = [
                (feature_index[name], float(value))
                for name, value in frame_features(
                    record["tokens"], int(token_index), float(time), start, end,
                    question=("?" in str(record.get("text", "")) or "？" in str(record.get("text", ""))),
                ).items()
                if name in feature_index and math.isfinite(float(value))
            ]
            sequence.append(sparse)
        prepared.append((sequence, (targets / max(1.0, target_scale)).astype(float).tolist(), mask.tolist(), frame_times.tolist()))
    return prepared, feature_index


class FrameIntonationTCN(nn.Module):
    """Compact residual dilated TCN operating on (batch, frame, feature)."""

    def __init__(self, inputs: int, hidden: int, dilations: Sequence[int] = (1, 2, 4, 8)):
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
    records: Sequence[tuple[list[list[tuple[int, float]]], list[float], list[bool], list[float]]],
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
        for row, (sequence, expected, valid, _times) in enumerate(selected):
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
    center = torch.stack(centers).unsqueeze(1)
    return values - center


def sequence_loss(
    predicted: torch.Tensor,
    targets: torch.Tensor,
    mask: torch.Tensor,
    bounded_target: tuple[float, float] | None = None,
    delta_weight: float = 0.35,
    target_scale: float = 1.0,
) -> torch.Tensor:
    """Speech-frame robust loss plus adjacent speech-frame delta loss."""

    if not bool(mask.any()):
        return predicted.sum() * 0.0
    predicted = centered(predicted, mask)
    targets = centered(targets, mask)
    if bounded_target is not None:
        low, high = bounded_target
        low /= max(1.0, target_scale)
        high /= max(1.0, target_scale)
        targets = targets.clamp(float(low), float(high))
        # 予測値のclampは勾配を止めるため、targetと出力時だけ制限する。
        predicted_for_loss = predicted
    else:
        predicted_for_loss = predicted
    absolute = F.smooth_l1_loss(predicted_for_loss[mask], targets[mask])
    pair_mask = mask[:, 1:] & mask[:, :-1]
    if not bool(pair_mask.any()):
        return absolute
    predicted_delta = predicted[:, 1:] - predicted[:, :-1]
    target_delta = targets[:, 1:] - targets[:, :-1]
    delta = F.smooth_l1_loss(predicted_delta[pair_mask], target_delta[pair_mask])
    return absolute + float(delta_weight) * delta


@torch.no_grad()
def evaluate(
    model: nn.Module,
    records: Sequence[tuple[list[list[tuple[int, float]]], list[float], list[bool], list[float]]],
    feature_count: int,
    batch_size: int,
    low_cents: float,
    high_cents: float,
    target_scale: float = 1.0,
    device: torch.device = torch.device("cpu"),
) -> float:
    model.eval()
    errors: list[float] = []
    for values, targets, mask in batches(records, feature_count, batch_size, random.Random(0)):
        values, targets, mask = move_batch(device, values, targets, mask)
        predicted = centered(model(values), mask).clamp(
            low_cents / max(1.0, target_scale), high_cents / max(1.0, target_scale)
        ) * max(1.0, target_scale)
        expected = targets * max(1.0, target_scale)
        for row in range(values.shape[0]):
            valid = mask[row]
            if bool(valid.any()):
                errors.extend((predicted[row][valid] - expected[row][valid]).abs().detach().cpu().tolist())
    return float(sum(errors) / len(errors)) if errors else 0.0


def _layers_for_export(model: FrameIntonationTCN) -> list[dict]:
    result = []
    for dilation, layer in zip(model.dilations, model.layers):
        result.append(
            {
                "dilation": int(dilation),
                "weights": layer.weight.detach().cpu().double().tolist(),
                "bias": layer.bias.detach().cpu().double().tolist(),
            }
        )
    return result


def export_model(
    model: FrameIntonationTCN,
    feature_index: dict[str, int],
    args: argparse.Namespace,
    train_records: int,
    train_frames: int,
    validation_frames: int,
    validation_voiced_frames: int,
    validation_mae: float,
    alignment_metadata: dict | None = None,
    train_tokens: int = 0,
) -> dict:
    feature_names = [None] * len(feature_index)
    for name, index in feature_index.items():
        feature_names[index] = name
    target_scale = max(1.0, float(getattr(args, "target_scale", 1.0)))
    output_weight = (model.output.weight.detach().cpu().double().squeeze(0) * target_scale).tolist()
    output_bias = float(model.output.bias.detach().cpu()) * target_scale
    frame_pitch = {
        "feature_names": feature_names,
        "input_weights": model.input.weight.detach().cpu().double().tolist(),
        "input_bias": model.input.bias.detach().cpu().double().tolist(),
        "layers": _layers_for_export(model),
        # 既存Goランタイム互換のため、正規化centを元の尺度へ戻す。
        "output_weight": output_weight,
        "output_bias": output_bias,
        "frame_ms": float(args.frame_ms),
        "low_cents": float(args.low_cents),
        "high_cents": float(args.high_cents),
        "centered": True,
        "target_scale_cents": target_scale,
        "render_strength": float(args.render_strength),
        "render_smoothing_ms": float(args.render_smoothing_ms),
        "render_p99_cents": float(args.render_p99_cents),
        "render_max_cents": float(args.render_max_cents),
    }
    return {
        "id": str(args.model_id or Path(args.out).stem),
        "display_name": str(args.display_name or Path(args.out).stem),
        "description": str(args.description or "Frame-level learned intonation model"),
        "license": "CC-BY-SA-4.0",
        "license_notice": "licenses/PROSODY-MODELS.txt",
        "provenance": {
            "training_corpus": "JSUT Japanese speech corpus",
            "training_corpus_license": "CC BY-SA 4.0",
            "source_notice": "licenses/JSUT-DATA-AND-LABELS.txt",
        },
        "recommended_renderers": list(args.recommended_renderer or ["utautts-world-phrase"]),
        "version": 8,
        "feature_version": 1,
        "mode": "intonation_frame_tcn_accent_bounded",
        "duration_weights": {},
        "frame_pitch": frame_pitch,
        "metrics": {
            "records": int(getattr(args, "validation_records", 0)),
            "frames": int(validation_frames),
            "voiced_frames": int(validation_voiced_frames),
            "pitch_mae_cents": float(validation_mae),
        },
        "training": {
            "records": int(train_records),
            "tokens": int(train_tokens),
            "frames": int(train_frames),
            "epochs": int(args.epochs),
            "learning_rate": float(args.learning_rate),
            "hidden": int(args.hidden),
            "batch_size": int(args.batch_size),
            "seed": int(args.seed),
            "device": str(getattr(args, "resolved_device", "cpu")),
            "openjtalk_accent": bool(args.openjtalk_accent),
            "f0_source": str(getattr(args, "f0_source", "auto")),
            "alignment": alignment_metadata or {},
        },
    }


def _simple_tokens(text: str) -> list[dict]:
    """Minimal deterministic fallback for prediction-only text corpora."""

    letters = [char for char in str(text) if not char.isspace()]
    return [
        {
            "mora": char,
            "vowel": "",
            "start_ms": index * 100.0,
            "end_ms": (index + 1) * 100.0,
            "duration_ms": 100.0,
            "pause": char in {",", ".", "、", "。"},
        }
        for index, char in enumerate(letters)
    ] or [{"mora": "", "vowel": "", "start_ms": 0.0, "end_ms": 100.0, "duration_ms": 100.0}]


def _prediction_tokens(item: dict) -> tuple[str, list[dict]]:
    if isinstance(item.get("tokens"), list) and item["tokens"]:
        tokens = [dict(token) for token in item["tokens"]]
        enriched = _ensure_accent_fields(tokens)
        return str(item.get("reading", item.get("text", ""))), enriched
    try:
        from openjtalk_features import analyze

        reading, tokens = analyze(str(item.get("text", "")))
        return reading, _ensure_accent_fields(tokens)
    except Exception:
        return str(item.get("reading", item.get("text", ""))), _fallback_accent(_simple_tokens(item.get("text", "")))


def _predict_timeline_tokens(tokens: list[dict], frame_ms: float) -> tuple[np.ndarray, np.ndarray]:
    if not tokens:
        return np.array([frame_ms * 0.5]), np.array([0], dtype=np.int64)
    cursor = 0.0
    for token in tokens:
        duration = float(token.get("duration_ms", 0.0) or 0.0)
        if duration <= 0.0:
            duration = 120.0 if token.get("pause", False) else 100.0
        token["start_ms"] = float(token.get("start_ms", cursor))
        token["end_ms"] = float(token.get("end_ms", token["start_ms"] + duration))
        cursor = max(cursor, token["end_ms"])
    count = max(1, int(math.ceil(cursor / frame_ms)))
    times = np.arange(count, dtype=np.float64) * frame_ms + frame_ms * 0.5
    indices = np.array([_frame_token_index(tokens, float(time)) for time in times], dtype=np.int64)
    return times, indices


@torch.no_grad()
def predict_corpus(
    model: FrameIntonationTCN,
    corpus_path: str | Path,
    feature_index: dict[str, int],
    frame_ms: float,
    low_cents: float,
    high_cents: float,
    target_scale: float = 1.0,
) -> dict:
    """Predict frame curves for ``{"cases": [...]}`` or JSUT JSONL."""

    path = Path(corpus_path)
    if path.suffix.lower() == ".jsonl":
        items = [json.loads(line) for line in path.read_text(encoding="utf-8").splitlines() if line.strip()]
    else:
        loaded = json.loads(path.read_text(encoding="utf-8"))
        items = loaded.get("cases", loaded if isinstance(loaded, list) else [])
    cases = []
    model.eval()
    device = next(model.parameters()).device
    for item in items:
        reading, tokens = _prediction_tokens(item)
        times, token_indices = _predict_timeline_tokens(tokens, frame_ms)
        start = 0.0
        end = float(times[-1] + frame_ms * 0.5)
        values = torch.zeros((1, len(times), len(feature_index)), dtype=torch.float32)
        for frame, (time, token_index) in enumerate(zip(times, token_indices)):
            for name, value in frame_features(
                tokens, int(token_index), float(time), start, end,
                question=("?" in str(item.get("text", "")) or "？" in str(item.get("text", ""))),
            ).items():
                if name in feature_index:
                    values[0, frame, feature_index[name]] = float(value)
        values = values.to(device=device, non_blocking=True)
        predicted = model(values)[0]
        mask = torch.tensor(
            [not tokens[int(index)].get("pause", False) for index in token_indices],
            dtype=torch.bool,
            device=device,
        )
        if bool(mask.any()):
            predicted = predicted - predicted[mask].median()
        predicted = (predicted * max(1.0, target_scale)).clamp(low_cents, high_cents)
        cases.append(
            {
                "id": str(item.get("id", f"case-{len(cases):04d}")),
                "text": str(item.get("text", "")),
                "reading": str(reading),
                "frame_ms": float(frame_ms),
                "cents": [round(float(value), 6) for value in predicted.detach().cpu().tolist()],
            }
        )
    return {"version": 1, "name": "intonation-frame-tcn-v8", "cases": cases}


def _write_json(path: str | Path, value: dict) -> None:
    output = Path(path)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def main(argv: Sequence[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dataset", required=True, help="version-1 JSUT JSONL with token boundaries and audio_path")
    parser.add_argument("--out", default="out/prosody/intonation-frame-tcn-v7.json")
    parser.add_argument("--model-id", default="", help="stable plugin ID stored in the model")
    parser.add_argument("--display-name", default="", help="user-facing model name")
    parser.add_argument("--description", default="", help="user-facing model description")
    parser.add_argument("--recommended-renderer", action="append", default=[], help="compatible renderer ID; repeatable")
    parser.add_argument("--epochs", type=int, default=24)
    parser.add_argument("--learning-rate", type=float, default=0.002)
    parser.add_argument("--hidden", type=int, default=24)
    parser.add_argument("--batch-size", type=int, default=16)
    parser.add_argument("--device", default="auto", help="PyTorch device: auto, cpu, cuda, cuda:N, xpu, or mps")
    parser.add_argument("--limit", type=int, default=0, help="maximum utterances before deterministic split (0=all)")
    parser.add_argument("--seed", type=int, default=1)
    parser.add_argument("--frame-ms", type=float, default=FRAME_MS)
    parser.add_argument("--low-cents", type=float, default=DEFAULT_LOW_CENTS)
    parser.add_argument("--high-cents", type=float, default=DEFAULT_HIGH_CENTS)
    parser.add_argument("--render-strength", type=float, default=0.32, help="runtime contour scale applied after centering (default keeps ±250c model within about ±80c)")
    parser.add_argument("--render-smoothing-ms", type=float, default=20.0)
    parser.add_argument("--render-p99-cents", type=float, default=75.0)
    parser.add_argument("--render-max-cents", type=float, default=90.0)
    parser.add_argument("--worldline", help="path to worldline.dll; local release paths are tried automatically")
    parser.add_argument("--f0-method", type=int, default=1, help="worldline F0 method: 0=DIO, 1=Harvest, 2=PYIN")
    parser.add_argument("--audio-root", help="optional root used to resolve record audio_path")
    accent = parser.add_mutually_exclusive_group()
    accent.add_argument("--openjtalk-accent", dest="openjtalk_accent", action="store_true")
    accent.add_argument("--no-openjtalk-accent", dest="openjtalk_accent", action="store_false")
    parser.set_defaults(openjtalk_accent=True)
    parser.add_argument("--predict-corpus", help="JSON/JSONL corpus for optional frame-curve export")
    parser.add_argument("--predict-out", help="output JSON for --predict-corpus")
    args = parser.parse_args(argv)
    if args.frame_ms <= 0 or args.epochs < 0 or args.batch_size <= 0:
        parser.error("frame-ms, batch-size must be positive and epochs must be non-negative")
    if args.low_cents >= args.high_cents:
        parser.error("--low-cents must be smaller than --high-cents")
    if not 0 < args.render_strength <= 1:
        parser.error("--render-strength must be in (0, 1]")
    if args.render_smoothing_ms < 0 or args.render_p99_cents <= 0 or args.render_max_cents < args.render_p99_cents:
        parser.error("invalid renderer smoothing/p99/maximum safety settings")
    if bool(args.predict_corpus) != bool(args.predict_out):
        parser.error("--predict-corpus and --predict-out must be used together")

    random.seed(args.seed)
    np.random.seed(args.seed)
    torch.manual_seed(args.seed)
    torch.set_num_threads(max(1, min(8, torch.get_num_threads())))
    try:
        device = resolve_device(args.device)
    except ValueError as error:
        parser.error(str(error))
    args.resolved_device = str(device)
    print(f"training device: {device_description(device)}")

    train_raw, validation_raw = load_records(args.dataset, args.limit)
    train_alignment: dict = {}
    validation_alignment: dict = {}
    train_raw = add_openjtalk_features(train_raw, args.openjtalk_accent, train_alignment, min_alignment_rate=0.0)
    validation_raw = add_openjtalk_features(validation_raw, args.openjtalk_accent, validation_alignment, min_alignment_rate=0.0)
    if not train_raw or not validation_raw:
        raise ValueError("Open JTalk alignment removed every utterance from a train or validation split")
    total_input_records = len(train_raw) + len(validation_raw) + train_alignment.get("skipped_records", 0) + validation_alignment.get("skipped_records", 0)
    overall_alignment_rate = (len(train_raw) + len(validation_raw)) / max(1, total_input_records)
    if args.openjtalk_accent and overall_alignment_rate < 0.60:
        raise ValueError(f"Open JTalk alignment rate {overall_alignment_rate:.3f} is below minimum 0.600")
    alignment_metadata = {
        "train": train_alignment,
        "validation": validation_alignment,
        "records": len(train_raw) + len(validation_raw),
        "aligned_records": len(train_raw) + len(validation_raw),
        "skipped_records": train_alignment.get("skipped_records", 0) + validation_alignment.get("skipped_records", 0),
        "alignment_rate": overall_alignment_rate,
        "question_records": sum("?" in str(record.get("text", "")) or "？" in str(record.get("text", "")) for record in train_raw + validation_raw),
        "final_distance_records": len(train_raw) + len(validation_raw),
        "question_distance_feature_count": sum(
            len(utterance_frame_times(record, args.frame_ms))
            for record in train_raw + validation_raw
            if "?" in str(record.get("text", "")) or "？" in str(record.get("text", ""))
        ),
    }
    worldline = load_worldline(args.worldline, args.f0_method)
    args.f0_source = "worldline" if worldline is not None else "internal_autocorrelation"
    args.target_scale = max(1.0, abs(args.low_cents), abs(args.high_cents))
    if worldline is None:
        print("worldline.dll unavailable; using internal autocorrelation F0 extractor", file=sys.stderr)
    else:
        print(f"using worldline F0: {worldline.path}")

    train, feature_index = prepare(
        train_raw,
        dataset_path=args.dataset,
        audio_root=args.audio_root,
        frame_ms=args.frame_ms,
        low_cents=args.low_cents,
        high_cents=args.high_cents,
        target_scale=args.target_scale,
        worldline=worldline,
    )
    validation, _ = prepare(
        validation_raw,
        feature_index,
        dataset_path=args.dataset,
        audio_root=args.audio_root,
        frame_ms=args.frame_ms,
        low_cents=args.low_cents,
        high_cents=args.high_cents,
        target_scale=args.target_scale,
        worldline=worldline,
    )
    args.validation_records = len(validation)
    validation_frames = sum(len(item[1]) for item in validation)
    validation_voiced_frames = sum(sum(item[2]) for item in validation)
    model = FrameIntonationTCN(len(feature_index), args.hidden).to(device)
    optimizer = torch.optim.AdamW(model.parameters(), lr=args.learning_rate, weight_decay=1e-5)
    rng = random.Random(args.seed)
    for epoch in range(args.epochs):
        model.train()
        total = torch.zeros((), device=device)
        count = 0
        for values, targets, mask in batches(train, len(feature_index), args.batch_size, rng):
            values, targets, mask = move_batch(device, values, targets, mask)
            if not bool(mask.any()):
                continue
            optimizer.zero_grad()
            loss = sequence_loss(
                model(values), targets, mask, (args.low_cents, args.high_cents), target_scale=args.target_scale
            )
            loss.backward()
            torch.nn.utils.clip_grad_norm_(model.parameters(), 1.0)
            optimizer.step()
            total += loss.detach()
            count += 1
        print(f"epoch {epoch + 1:02d}/{args.epochs}: loss={float(total.cpu()) / max(1, count):.6f}")

    validation_mae = evaluate(
        model, validation, len(feature_index), args.batch_size, args.low_cents, args.high_cents,
        args.target_scale, device
    )
    train_frames = sum(len(item[1]) for item in train)
    exported = export_model(
        model,
        feature_index,
        args,
        len(train_raw),
        train_frames,
        validation_frames,
        validation_voiced_frames,
        validation_mae,
        alignment_metadata,
        sum(len(record["tokens"]) for record in train_raw),
    )
    _write_json(args.out, exported)
    print(
        f"wrote {args.out} ({len(train)} train/{len(validation)} validation records, "
        f"{validation_mae:.1f} cents MAE, {len(feature_index)} features)"
    )
    if args.predict_corpus and args.predict_out:
        contours = predict_corpus(
            model,
            args.predict_corpus,
            feature_index,
            args.frame_ms,
            args.low_cents,
            args.high_cents,
            args.target_scale,
        )
        _write_json(args.predict_out, contours)
        print(f"wrote {len(contours['cases'])} predicted frame contours to {args.predict_out}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
