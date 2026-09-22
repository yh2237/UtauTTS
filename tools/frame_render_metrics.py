"""レンダリング空間の輪郭指標。Goのフレームヘッド後処理を反映する。

予測と教師の両方を変換する。これは有界輪郭の一致度を測るもので、
知覚的な自然さや合成音声のF0ではない。
"""
import math
import numpy as np


def render_contour(values, speech, frame_ms=10, strength=.32, smoothing_ms=20,
                   p99=75, maximum=90, low=-250, high=250):
    values = np.asarray(values, dtype=float).copy()
    speech = np.asarray(speech, dtype=bool)
    if not speech.any():
        return np.zeros_like(values)
    values -= np.median(values[speech])
    values[~speech] = 0
    sigma = smoothing_ms / frame_ms
    radius = max(1, math.ceil(3 * sigma))
    kernel = np.exp(-.5 * (np.arange(-radius, radius + 1) / sigma) ** 2)
    kernel /= kernel.sum()
    start = 0
    while start < len(values):
        if not speech[start]:
            start += 1
            continue
        end = start + 1
        while end < len(values) and speech[end]:
            end += 1
        values[start:end] = np.convolve(np.pad(values[start:end], radius, mode="edge"), kernel, mode="valid")
        start = end
    values *= strength
    selected = np.sort(np.abs(values[speech]))
    observed = selected[max(0, math.ceil(.99 * len(selected)) - 1)]
    if observed > p99:
        values *= p99 / observed
    return np.clip(np.clip(values, -maximum, maximum), low, high)
