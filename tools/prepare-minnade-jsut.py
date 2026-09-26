#!/usr/bin/env python3
"""みんなで作るJSUTコーパスbasic5000を学習入力のmetadata.csv+wavs/へ整える。

配布ZIPの「02 台本テキスト」と「01 音声データ」を使い、指定したID範囲について
``metadata.csv``（``id|text|reading``）と``wavs/<id>.wav``（元WAVへのハードリンク、
不可ならコピー）を出力する。学習用JSONLは ``prepare-intonation-frame-data.py`` が作る。

ID範囲は ``BASIC5000_0001`` 形式の連番。夢前黎さんの担当は既定の0001-0600。
"""

from __future__ import annotations

import argparse
import os
import shutil
import sys
from pathlib import Path


def find_directory(root: Path, needle: str, *, require_suffix: str | None = None) -> Path:
    for path in sorted(root.rglob("*")):
        if not path.is_dir() or needle not in path.name:
            continue
        if require_suffix is None or any(path.glob(f"*{require_suffix}")):
            return path
    raise SystemExit(f"directory containing {needle!r} was not found below {root}")


def parse_original(path: Path) -> dict[str, str]:
    texts: dict[str, str] = {}
    for line in path.read_text(encoding="utf-8").splitlines():
        stripped = line.rstrip()
        if stripped.startswith("BASIC5000_") and ":" in stripped:
            key, _, value = stripped.partition(":")
            texts[key] = value.strip()
    return texts


def parse_reading(path: Path) -> tuple[dict[str, str], dict[str, str]]:
    texts: dict[str, str] = {}
    kanas: dict[str, str] = {}
    current = None
    for line in path.read_text(encoding="utf-8").splitlines():
        stripped = line.rstrip()
        if stripped.startswith("BASIC5000_") and stripped.endswith(":"):
            current = stripped[:-1]
        elif current is not None and "text_level0:" in stripped:
            texts[current] = stripped.split("text_level0:", 1)[1].strip()
        elif current is not None and "kana_level0:" in stripped:
            kanas[current] = stripped.split("kana_level0:", 1)[1].strip()
    return texts, kanas


def link_or_copy(source: Path, destination: Path) -> None:
    destination.parent.mkdir(parents=True, exist_ok=True)
    if destination.exists():
        destination.unlink()
    try:
        os.link(source, destination)
    except OSError:
        shutil.copy2(source, destination)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", required=True, help="extracted corpus root (nested folders are searched)")
    parser.add_argument("--out", required=True, help="output directory for metadata.csv and wavs/")
    parser.add_argument("--start", type=int, default=1, help="first sentence number (inclusive)")
    parser.add_argument("--end", type=int, default=600, help="last sentence number (inclusive)")
    parser.add_argument("--audio-dir", help="explicit directory holding BASIC5000_*.wav (overrides discovery)")
    args = parser.parse_args()

    root = Path(args.root).resolve()
    if not root.is_dir():
        raise SystemExit(f"corpus root was not found: {root}")
    audio_dir = Path(args.audio_dir).resolve() if args.audio_dir else find_directory(root, "01 音声データ", require_suffix=".wav")
    text_dir = find_directory(root, "02 台本テキスト")
    original = next(text_dir.rglob("【全文】【オリジナル】*.txt"), None)
    reading = next(text_dir.rglob("【全文】【読み仮名】*.txt"), None)
    if original is None or reading is None:
        raise SystemExit("original/reading script text was not found")

    original_texts = parse_original(original)
    reading_texts, reading_kanas = parse_reading(reading)

    out = Path(args.out).resolve()
    wavs = out / "wavs"
    wavs.mkdir(parents=True, exist_ok=True)
    rows: list[tuple[str, str, str]] = []
    missing_audio: list[str] = []
    for number in range(args.start, args.end + 1):
        key = f"BASIC5000_{number:04d}"
        text = reading_texts.get(key) or original_texts.get(key)
        if not text:
            continue
        wav = audio_dir / f"{key}.wav"
        if not wav.is_file():
            missing_audio.append(key)
            continue
        link_or_copy(wav, wavs / f"{key}.wav")
        rows.append((key, text, reading_kanas.get(key, "")))
    if not rows:
        raise SystemExit("no sentences were collected for the requested range")
    metadata = out / "metadata.csv"
    with metadata.open("w", encoding="utf-8", newline="") as stream:
        for key, text, reading in rows:
            stream.write(f"{key}|{text}|{reading}\n")
    print(f"wrote {len(rows)} sentences to {metadata}")
    print(f"audio source: {audio_dir}")
    if missing_audio:
        print(f"missing audio for {len(missing_audio)} ids: {', '.join(missing_audio[:10])}", file=sys.stderr)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
