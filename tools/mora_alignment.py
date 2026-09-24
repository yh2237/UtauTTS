"""アクセント誘導・時間長制約付きViterbiモーラ整列。

音素タイミングの無いコーパスでも
トークンごとのモーラ時間幅が必要。本モジュールはOpen JTalkのアクセント注釈を
弱い音響モデルとして境界を置く。有声フレームはモーラの高低ピッチ付近に来る
べきという前提。ハードな時間長制約が各モーラを妥当な長さに保ち、単純DTWが
階段状輪郭で作る退化パスを防ぐ。
"""

from __future__ import annotations

import importlib.util
import math
import sys
from collections import deque
from pathlib import Path

import numpy as np

_TRAINER = None


def load_trainer():
    """F0抽出と補間のため共通フレーム学習器を読み込む。"""

    global _TRAINER
    if _TRAINER is None:
        path = Path(__file__).resolve().parent / "train-frame-intonation-tcn.py"
        spec = importlib.util.spec_from_file_location("utautts_frame_trainer", path)
        if spec is None or spec.loader is None:
            raise RuntimeError(f"cannot load {path}")
        module = importlib.util.module_from_spec(spec)
        sys.modules[spec.name] = module
        spec.loader.exec_module(module)
        _TRAINER = module
    return _TRAINER


def viterbi_mora_bounds(
    tokens,
    f0,
    frame_ms: float = 10.0,
    high_cents: float = 70.0,
    min_ms: float = 60.0,
    max_ms: float = 300.0,
):
    """トークンごとのフレーム境界を返す（長さ ``len(tokens) + 1``）。"""

    voiced = f0 > 0
    base = float(np.median(f0[voiced])) if voiced.any() else 200.0
    cents = np.zeros(len(f0), dtype=np.float64)
    cents[voiced] = 1200.0 * np.log2(f0[voiced] / max(1.0, base))
    count, frames = len(tokens), len(f0)
    min_frames = max(1, int(round(min_ms / frame_ms)))
    max_frames = max(min_frames, int(round(max_ms / frame_ms)))
    if frames < count * min_frames or frames > count * max_frames:
        return [int(round(index * frames / count)) for index in range(count + 1)]
    infinite = 1e18
    dp = np.full(frames + 1, infinite, dtype=np.float64)
    dp[0] = 0.0
    back = np.full((count + 1, frames + 1), -1, dtype=np.int64)
    for index in range(count):
        if tokens[index].get("pause", False):
            frame_cost = np.where(voiced, 4.0, 0.0)
        else:
            expected = high_cents if tokens[index].get("accent_high") else -high_cents
            frame_cost = np.where(voiced, (cents - expected) ** 2 / 10000.0, 0.15)
        prefix = np.concatenate(([0.0], np.cumsum(frame_cost)))
        new = np.full(frames + 1, infinite, dtype=np.float64)
        window: deque = deque()
        for end in range(1, frames + 1):
            candidate = end - min_frames
            if candidate >= 0 and dp[candidate] < infinite:
                value = dp[candidate] - prefix[candidate]
                while window and window[-1][1] >= value:
                    window.pop()
                window.append((candidate, value))
            low = end - max_frames
            while window and window[0][0] < low:
                window.popleft()
            if window:
                new[end] = prefix[end] + window[0][1]
                back[index + 1, end] = window[0][0]
        dp = new
    if dp[frames] >= infinite:
        return [int(round(index * frames / count)) for index in range(count + 1)]
    bounds = [frames]
    cursor = frames
    for index in range(count, 0, -1):
        cursor = int(back[index, cursor])
        bounds.append(cursor)
    bounds.reverse()
    return bounds


def align_accent_viterbi(
    tokens,
    samples,
    rate,
    start,
    end,
    frame_ms: float = 10.0,
    worldline=None,
    high_cents: float = 70.0,
    min_ms: float = 60.0,
    max_ms: float = 300.0,
):
    """アクセント誘導・時間長制約付きViterbiで均等モーラタイミングを精緻化する。"""

    trainer = load_trainer()
    frame_count = max(1, int(math.ceil((end - start) / frame_ms)))
    frame_times = start + np.arange(frame_count, dtype=np.float64) * frame_ms + frame_ms * 0.5
    if worldline is not None:
        data = np.asarray(samples, dtype=np.float64)
        peak = float(np.max(np.abs(data))) if len(data) else 0.0
        if peak > 0:
            data = data / peak
        track = worldline.extract(data, rate, frame_ms)
    else:
        track = trainer.extract_f0_internal(np.asarray(samples, dtype=np.float64), rate, frame_ms)
    f0 = trainer._interpolate_track(
        track, np.arange(len(track), dtype=np.float64) * frame_ms, frame_times, frame_ms
    )
    bounds = viterbi_mora_bounds(tokens, f0, frame_ms, high_cents, min_ms, max_ms)
    result = [dict(token) for token in tokens]
    for index, token in enumerate(result):
        token["start_ms"] = start + bounds[index] * frame_ms
        token["end_ms"] = start + bounds[index + 1] * frame_ms
        token["duration_ms"] = token["end_ms"] - token["start_ms"]
    return result
