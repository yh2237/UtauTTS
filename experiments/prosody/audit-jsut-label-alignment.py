#!/usr/bin/env python3
"""Audit mora alignment coverage of sarulab-speech/jsut-label labels."""

import argparse
import json
import re
from pathlib import Path


PHONE = re.compile(r"-([^+]+)\+")
MORA_NUCLEI = {"a", "i", "u", "e", "o", "A", "I", "U", "E", "O", "N", "cl"}


def label_tokens(path):
    result = []
    entries = []
    for line in path.read_text(encoding="utf-8").splitlines():
        parts = line.split(maxsplit=2)
        if len(parts) != 3:
            continue
        match = PHONE.search(parts[2])
        if not match:
            continue
        phone = match.group(1)
        entries.append((phone, int(parts[0]), int(parts[1])))
    for index, (phone, start, end) in enumerate(entries):
        if phone == "pau":
            result.append({"pause": True, "start": start, "end": end})
        elif phone == "sil" and index == len(entries) - 1:
            result.append({"pause": True, "start": start, "end": end})
        elif phone in MORA_NUCLEI:
            result.append({"pause": False, "end": end})
    return result


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--dataset", required=True)
    parser.add_argument("--labels", required=True)
    args = parser.parse_args()
    label_root = Path(args.labels)
    records = labels_found = aligned = 0
    failures = []
    with Path(args.dataset).open("r", encoding="utf-8") as source:
        for line in source:
            if not line.strip():
                continue
            record = json.loads(line)
            records += 1
            path = label_root / f"{record['id']}.lab"
            if not path.exists():
                failures.append({"id": record["id"], "reason": "missing label"})
                continue
            labels_found += 1
            observed = label_tokens(path)
            expected = record["tokens"]
            if len(observed) == len(expected) and all(bool(left["pause"]) == bool(right.get("pause")) for left, right in zip(observed, expected)):
                aligned += 1
            elif len(failures) < 20:
                failures.append({"id": record["id"], "reason": f"labels {len(observed)}, targets {len(expected)}"})
    print(json.dumps({
        "records": records,
        "labels_found": labels_found,
        "aligned": aligned,
        "alignment_rate": aligned / records if records else 0,
        "sample_failures": failures,
    }, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
