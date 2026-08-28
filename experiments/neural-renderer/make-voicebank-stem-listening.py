#!/usr/bin/env python3
"""原音切り出しとWorldline unit stemの聴取ページを作る。"""

from __future__ import annotations

import argparse
import html
import json
import os
from pathlib import Path


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--plan", required=True)
    parser.add_argument("--stems", required=True)
    parser.add_argument("--phrase", required=True)
    parser.add_argument("--out", required=True)
    args = parser.parse_args()

    plan = json.loads(Path(args.plan).read_text(encoding="utf-8"))
    stem_path = Path(args.stems).resolve()
    stems = json.loads(stem_path.read_text(encoding="utf-8"))
    output = Path(args.out).resolve()
    output.parent.mkdir(parents=True, exist_ok=True)
    phrase = Path(os.path.relpath(Path(args.phrase).resolve(), output.parent)).as_posix()
    parts = [
        "<!doctype html><meta charset='utf-8'><title>Worldline unit stems</title>",
        "<style>body{font-family:sans-serif;max-width:1000px;margin:30px auto}"
        "section{border-top:1px solid #ccc;padding:18px 0}"
        "audio{display:block;width:100%;margin:4px 0 12px}code{overflow-wrap:anywhere}</style>",
        "<h1>Worldline unit stem比較</h1>",
        "<p>原音切り出し → Worldline処理全体 → 実際にmixへ使った区間の順です。</p>",
        f"<h2>合成音声</h2><audio controls src='{html.escape(phrase)}'></audio>",
    ]
    for filename, label in (
        ("mix-overlap-normalized.wav", "重なりゲイン正規化"),
        ("mix-overlap-sharp.wav", "優勢unit強調"),
    ):
        candidate = stem_path.parent / filename
        if candidate.is_file():
            relative_candidate = Path(os.path.relpath(candidate, output.parent)).as_posix()
            parts.append(f"<label>{label}</label><audio controls src='{relative_candidate}'></audio>")
    for record in stems["units"]:
        index = record["index"]
        unit = plan["units"][index]
        relative = lambda name: Path(os.path.relpath(stem_path.parent / name, output.parent)).as_posix()
        parts.extend([
            f"<section><h2>{index:03d}: {html.escape(unit['mora'])} / {html.escape(unit['alias'])}"
            f" <small>{html.escape(unit['role'])}</small></h2>",
            f"<p><code>{html.escape(unit['source'])}</code></p>",
            f"<label>原音切り出し</label><audio controls src='{relative(record['source_wav'])}'></audio>",
            f"<label>Worldline resample</label><audio controls src='{relative(record['raw_wav'])}'></audio>",
            f"<label>mix使用区間</label><audio controls src='{relative(record['visible_wav'])}'></audio>",
            "</section>",
        ])
    output.write_text("\n".join(parts), encoding="utf-8")
    print(f"wrote {output}")


if __name__ == "__main__":
    main()
