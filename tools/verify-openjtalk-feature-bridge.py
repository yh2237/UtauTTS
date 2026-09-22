#!/usr/bin/env python3
"""単体フロントエンドを出力済みpyopenjtalk特徴と照合する。"""

import argparse
import json
import subprocess
from pathlib import Path


def analyze(helper, dictionary, text):
    process = subprocess.run(
        [helper, "--dictionary", dictionary],
        input=json.dumps({"text": text}, ensure_ascii=False).encode("utf-8"),
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=True,
    )
    return json.loads(process.stdout)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--helper", required=True)
    parser.add_argument("--dictionary", required=True)
    parser.add_argument("--corpus")
    args = parser.parse_args()
    if args.corpus and Path(args.corpus).is_file():
        cases = json.loads(Path(args.corpus).read_text(encoding="utf-8"))["cases"]
        for case in cases:
            actual = analyze(args.helper, args.dictionary, case["text"])
            if actual["reading"] != case["reading"] or actual["features"] != case["features"]:
                raise RuntimeError(f"feature mismatch for {case['id']}")
        print(f"verified {len(cases)} exported Open JTalk feature cases")
        return
    actual = analyze(args.helper, args.dictionary, "駅前の図書館で本を借ります。")
    if not actual.get("reading") or len(actual.get("morae", [])) != len(actual.get("features", [])):
        raise RuntimeError("invalid smoke response")
    print(f"verified arbitrary text with {len(actual['features'])} feature frames")


if __name__ == "__main__":
    main()
