#!/usr/bin/env python3
"""Open JTalk frontend bridge for the portable UtauTTS runtime.

The executable accepts one request or a newline-delimited persistent stream.
"""

import argparse
import json
import sys

import openjtalk

from openjtalk_feature_common import analyze, sparse_features


def process(frontend, request):
    text = str(request.get("text", "")).strip()
    if not text:
        raise ValueError("text is empty")
    reading, tokens = analyze(frontend, text)
    return {
        "version": 1,
        "reading": reading,
        "morae": [token.get("mora", "") for token in tokens],
        "features": [sparse_features(token) for token in tokens],
    }


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--dictionary", required=True)
    parser.add_argument("--serve", action="store_true")
    args = parser.parse_args()
    frontend = openjtalk.OpenJTalk(dn_mecab=args.dictionary.encode("utf-8"))
    if args.serve:
        for line in sys.stdin:
            try:
                response = process(frontend, json.loads(line))
            except Exception as error:
                response = {"error": str(error)}
            json.dump(response, sys.stdout, ensure_ascii=False, separators=(",", ":"))
            sys.stdout.write("\n")
            sys.stdout.flush()
    else:
        json.dump(process(frontend, json.load(sys.stdin)), sys.stdout,
                  ensure_ascii=False, separators=(",", ":"))
        sys.stdout.write("\n")


if __name__ == "__main__":
    sys.stdin.reconfigure(encoding="utf-8")
    sys.stdout.reconfigure(encoding="utf-8")
    main()
