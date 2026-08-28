#!/usr/bin/env python3
"""WORLDの有声部とraw linearの子音部を組み合わせる。"""

from __future__ import annotations

import argparse
import html
import json
import os
import re
import sys
from pathlib import Path

import torch

sys.path.insert(0, str(Path(__file__).parent))
from neural_renderer_common import load_index, read_pcm16, resample, write_pcm16  # noqa: E402


VOICED_NUCLEI = {"a", "i", "u", "e", "o"}
ID_PATTERN = re.compile(r"^\d{3}-(.+)-03-worldline\.wav$")


def protect_interval(mask: torch.Tensor, start: int, end: int, fade: int) -> None:
    start = max(0, min(mask.numel(), start))
    end = max(start, min(mask.numel(), end))
    mask[start:end] = 1
    left_start = max(0, start - fade)
    if start > left_start:
        ramp = torch.linspace(0, 1, start - left_start + 1)[:-1]
        mask[left_start:start] = torch.maximum(mask[left_start:start], ramp)
    right_end = min(mask.numel(), end + fade)
    if right_end > end:
        ramp = torch.linspace(1, 0, right_end - end + 1)[1:]
        mask[end:right_end] = torch.maximum(mask[end:right_end], ramp)


def write_html(path: Path, rows: list[dict]) -> None:
    parts = [
        "<!doctype html><meta charset='utf-8'><title>WORLD consonant protection</title>",
        "<style>body{font-family:sans-serif;max-width:960px;margin:30px auto}section{margin:28px 0}audio{display:block;width:100%;margin:5px 0 12px}</style>",
        "<h1>WORLD母音＋raw子音</h1>",
        "<p>hybridは母音だけWORLDを使い、それ以外の音素をraw linearへ戻しています。</p>",
    ]
    for row in rows:
        parts.append(f"<section><h2>{html.escape(row['id'])}</h2>")
        for key, label in (
            ("natural", "自然音声"),
            ("raw", "raw linear / F0 oracle / phase-align"),
            ("worldline", "WORLD / F0 calibrated"),
            ("hybrid", "hybrid / WORLD vowels + raw consonants"),
        ):
            parts.append(f"<label>{label}</label><audio controls src='{row[key]}'></audio>")
        parts.append(f"<p>raw比率: {row['raw_percent']:.1f}%</p></section>")
    path.write_text("\n".join(parts), encoding="utf-8")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--dataset", default="out/neural-renderer/jsut-index.json")
    parser.add_argument("--worldline-dir", default="out/neural-renderer/diphone-worldline-f0-calibrated")
    parser.add_argument("--raw-dir", default="out/neural-renderer/diphone-f0-oracle-phase")
    parser.add_argument("--out", default="out/neural-renderer/diphone-worldline-hybrid")
    parser.add_argument("--fade-ms", type=float, default=3.0)
    args = parser.parse_args()

    index = load_index(args.dataset)
    utterances = {item["id"]: item for item in index["utterances"]}
    output = Path(args.out)
    output.mkdir(parents=True, exist_ok=True)
    rows = []
    for worldline_path in sorted(Path(args.worldline_dir).glob("*-03-worldline.wav")):
        match = ID_PATTERN.match(worldline_path.name)
        if match is None or match.group(1) not in utterances:
            continue
        utterance_id = match.group(1)
        utterance = utterances[utterance_id]
        prefix = worldline_path.name.removesuffix("-03-worldline.wav")
        raw_path = Path(args.raw_dir) / (prefix + "-02-baseline.wav")
        natural_path = Path(args.worldline_dir) / (prefix + "-01-natural.wav")
        if not raw_path.is_file() or not natural_path.is_file():
            raise FileNotFoundError(raw_path if not raw_path.is_file() else natural_path)
        worldline_rate, worldline = read_pcm16(worldline_path)
        raw_rate, raw = read_pcm16(raw_path)
        raw = resample(raw, worldline.numel()) if raw_rate != worldline_rate or raw.numel() != worldline.numel() else raw
        mask = torch.zeros(worldline.numel())
        fade = max(1, round(args.fade_ms * worldline_rate / 1000))
        for segment in utterance["segments"]:
            if segment["phone"] in VOICED_NUCLEI:
                continue
            start = round(segment["start_sample"] * worldline_rate / utterance["sample_rate"])
            end = round(segment["end_sample"] * worldline_rate / utterance["sample_rate"])
            protect_interval(mask, start, end, fade)
        hybrid = (worldline * (1 - mask) + raw * mask).clamp(-1, 1)
        hybrid_name = prefix + "-04-hybrid.wav"
        write_pcm16(output / hybrid_name, worldline_rate, hybrid)
        rows.append({
            "id": utterance_id,
            "natural": Path(os.path.relpath(natural_path.resolve(), output.resolve())).as_posix(),
            "raw": Path(os.path.relpath(raw_path.resolve(), output.resolve())).as_posix(),
            "worldline": Path(os.path.relpath(worldline_path.resolve(), output.resolve())).as_posix(),
            "hybrid": hybrid_name,
            "raw_percent": 100 * float(mask.mean()),
        })
    if not rows:
        raise SystemExit("no WORLD files found")
    write_html(output / "index.html", rows)
    (output / "report.json").write_text(
        json.dumps({"version": 1, "fade_ms": args.fade_ms, "utterances": rows}, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )
    print(json.dumps({"written": len(rows), "mean_raw_percent": sum(row["raw_percent"] for row in rows) / len(rows)}, ensure_ascii=False))
    print(f"wrote {output / 'index.html'}")


if __name__ == "__main__":
    main()
