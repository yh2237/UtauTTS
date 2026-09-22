#!/usr/bin/env python3
"""Kokoroのメタデータクリップをモノラル22.05 kHz PCM WAVとして抽出する。

上流メタデータは元音声のサンプルオフセットを保持する。元MP3は章ごとに
ffmpegで1回だけデコードし、公開オフセットをそのまま使って再分割しない。
"""

from __future__ import annotations

import argparse
import csv
import json
import subprocess
import wave
from pathlib import Path


SAMPLE_RATE = 22050


def decode_source(source: Path, output: Path) -> None:
    output.parent.mkdir(parents=True, exist_ok=True)
    if output.is_file() and output.stat().st_size > 44:
        return
    subprocess.run(
        ["ffmpeg", "-loglevel", "error", "-y", "-i", str(source), "-ac", "1",
         "-ar", str(SAMPLE_RATE), "-c:a", "pcm_s16le", str(output)],
        check=True,
    )


def read_pcm(path: Path) -> bytes:
    with wave.open(str(path), "rb") as source:
        if source.getnchannels() != 1 or source.getsampwidth() != 2 or source.getframerate() != SAMPLE_RATE:
            raise ValueError(f"unexpected decoded format: {path}")
        return source.readframes(source.getnframes())


def write_clip(path: Path, pcm: bytes, start: int, end: int) -> None:
    start = max(0, int(start))
    end = min(len(pcm) // 2, int(end))
    if end <= start:
        raise ValueError(f"invalid clip range {start}:{end}")
    path.parent.mkdir(parents=True, exist_ok=True)
    if path.is_file() and path.stat().st_size > 44:
        return
    with wave.open(str(path), "wb") as output:
        output.setnchannels(1)
        output.setsampwidth(2)
        output.setframerate(SAMPLE_RATE)
        output.writeframes(pcm[start * 2 : end * 2])


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--metadata", required=True, help="Kokoro metadata directory")
    parser.add_argument("--source-root", required=True, help="downloaded source MP3 directory")
    parser.add_argument("--out", required=True, help="output directory containing wavs/ and metadata.csv")
    parser.add_argument("--limit", type=int, default=0)
    args = parser.parse_args()

    metadata_root = Path(args.metadata).resolve()
    source_root = Path(args.source_root).resolve()
    output_root = Path(args.out).resolve()
    wav_root = output_root / "wavs"
    wav_root.mkdir(parents=True, exist_ok=True)
    rows = []
    count = 0
    for metadata_file in sorted(metadata_root.glob("*.metadata.txt")):
        dataset = metadata_file.name.removesuffix(".metadata.txt")
        source_dir = source_root / dataset
        decoded_dir = output_root / "decoded" / dataset
        decoded: dict[str, bytes] = {}
        for line in metadata_file.read_text(encoding="utf-8").splitlines():
            if not line.strip():
                continue
            parts = line.split("|")
            if len(parts) < 6:
                raise ValueError(f"invalid metadata row: {metadata_file}:{line}")
            clip_id, source_name, start, end, text, reading = parts[:6]
            if args.limit and count >= args.limit:
                break
            source = source_dir / source_name
            decoded_path = decoded_dir / (Path(source_name).stem + ".wav")
            if source_name not in decoded:
                if not source.is_file():
                    raise FileNotFoundError(source)
                decode_source(source, decoded_path)
                decoded[source_name] = read_pcm(decoded_path)
            clip_path = wav_root / f"{clip_id}.wav"
            write_clip(clip_path, decoded[source_name], int(start), int(end))
            rows.append((clip_id, text, reading))
            count += 1
        if args.limit and count >= args.limit:
            break
    if not rows:
        raise ValueError("no Kokoro clips were extracted")
    with (output_root / "metadata.csv").open("w", encoding="utf-8", newline="") as stream:
        writer = csv.writer(stream, delimiter="|", lineterminator="\n")
        writer.writerows(rows)
    print(json.dumps({"clips": len(rows), "output": str(output_root)}, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
