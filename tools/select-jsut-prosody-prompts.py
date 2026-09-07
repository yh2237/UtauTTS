#!/usr/bin/env python3
"""JSUT BASIC5000から抑揚調整用の文章を決定的に選抜する。"""

from __future__ import annotations

import argparse
import hashlib
import json
import math
import re
import statistics
import sys
from collections import Counter
from dataclasses import dataclass
from pathlib import Path

import pyopenjtalk

from openjtalk_feature_common import analyze


CATEGORY_WEIGHTS = {
    "mora": 0.4,
    "mora_pair": 0.25,
    "phrase_edge": 0.8,
    "accent": 6.0,
    "phrase_length": 3.0,
    "sentence_length": 2.5,
    "phrase_count": 3.0,
    "punctuation": 6.0,
    "special": 2.0,
    "pos": 0.5,
}


@dataclass(frozen=True)
class Candidate:
    id: str
    text: str
    reading: str
    features: frozenset[str]
    mora_count: int
    phrase_count: int


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def length_bin(value: int, boundaries: tuple[int, ...]) -> str:
    for boundary in boundaries:
        if value <= boundary:
            return f"le{boundary}"
    return f"gt{boundaries[-1]}"


def feature_name(category: str, value: str | int) -> str:
    return f"{category}:{value}"


def candidate_features(text: str, tokens: list[dict]) -> tuple[frozenset[str], int, int]:
    speech = [token for token in tokens if not token.get("pause")]
    morae = [str(token.get("mora", "")) for token in speech]
    features: set[str] = set()
    for mora in morae:
        features.add(feature_name("mora", mora))
        if "っ" in mora:
            features.add(feature_name("special", "sokuon"))
        if "ん" in mora:
            features.add(feature_name("special", "hatsuon"))
        if "ー" in mora:
            features.add(feature_name("special", "long_vowel"))
        if any(character in mora for character in "ゃゅょぁぃぅぇぉ"):
            features.add(feature_name("special", "contracted"))
    for left, right in zip(morae, morae[1:]):
        features.add(feature_name("mora_pair", f"{left}>{right}"))

    phrases: list[list[dict]] = []
    current: list[dict] = []
    for token in tokens:
        if token.get("pause"):
            if current:
                phrases.append(current)
                current = []
            continue
        if token.get("accent_phrase_start") and current:
            phrases.append(current)
            current = []
        current.append(token)
    if current:
        phrases.append(current)

    for phrase in phrases:
        phrase_length = int(phrase[0].get("accent_phrase_length", len(phrase)))
        nucleus = int(phrase[0].get("accent_nucleus", 0))
        nucleus_bin = "heiban" if nucleus == 0 else length_bin(nucleus, (1, 2, 3, 5, 8))
        features.add(feature_name("phrase_length", length_bin(phrase_length, (2, 4, 6, 9, 13))))
        features.add(feature_name("accent", f"{length_bin(phrase_length, (2, 4, 6, 9, 13))}/{nucleus_bin}"))
        features.add(feature_name("phrase_edge", f"start={phrase[0].get('mora', '')}"))
        features.add(feature_name("phrase_edge", f"end={phrase[-1].get('mora', '')}"))
        for token in phrase:
            features.add(feature_name("pos", str(token.get("pos", "*"))))
            features.add(feature_name("pos", f"group={token.get('pos_group1', '*')}"))

    features.add(feature_name("sentence_length", length_bin(len(morae), (12, 20, 30, 42, 58, 75))))
    features.add(feature_name("phrase_count", length_bin(len(phrases), (1, 2, 3, 5, 8))))
    comma_count = text.count("、") + text.count(",")
    features.add(feature_name("punctuation", f"comma={min(comma_count, 3)}"))
    if text.rstrip().endswith(("？", "?")):
        features.add(feature_name("punctuation", "question"))
    elif text.rstrip().endswith(("！", "!")):
        features.add(feature_name("punctuation", "exclamation"))
    else:
        features.add(feature_name("punctuation", "statement"))
    if re.search(r"[0-9０-９]", text):
        features.add(feature_name("special", "number"))
    return frozenset(features), len(morae), len(phrases)


def load_candidates(path: Path, min_chars: int, max_chars: int) -> tuple[list[Candidate], list[dict]]:
    candidates = []
    rejected = []
    with path.open("r", encoding="utf-8-sig") as source:
        for line_number, line in enumerate(source, 1):
            line = line.rstrip("\r\n")
            if not line:
                continue
            if ":" not in line:
                rejected.append({"line": line_number, "reason": "missing separator"})
                continue
            prompt_id, text = line.split(":", 1)
            text = text.strip()
            if len(text) < min_chars or len(text) > max_chars:
                rejected.append({"id": prompt_id, "reason": "text length"})
                continue
            try:
                reading, tokens = analyze(pyopenjtalk, text)
            except Exception as error:
                rejected.append({"id": prompt_id, "reason": str(error)})
                continue
            features, mora_count, phrase_count = candidate_features(text, tokens)
            if not features or mora_count == 0:
                rejected.append({"id": prompt_id, "reason": "empty analysis"})
                continue
            candidates.append(Candidate(prompt_id, text, reading, features, mora_count, phrase_count))
    return candidates, rejected


def category(feature: str) -> str:
    return feature.split(":", 1)[0]


def coverage_score(
    features: frozenset[str], frequencies: Counter[str], selected_counts: Counter[str]
) -> float:
    grouped: dict[str, list[float]] = {}
    for feature in features:
        rarity = 1.0 / math.sqrt(max(1, frequencies[feature]))
        novelty = 1.0 / math.sqrt(1 + selected_counts[feature])
        grouped.setdefault(category(feature), []).append(rarity * novelty)
    return sum(
        CATEGORY_WEIGHTS.get(name, 1.0)
        * statistics.fmean(values)
        * (1.0 + 0.15 * math.log2(1 + len(values)))
        for name, values in grouped.items()
    )


def select_candidates(candidates: list[Candidate], count: int) -> list[Candidate]:
    frequencies = Counter(feature for item in candidates for feature in item.features)
    selected_counts: Counter[str] = Counter()
    remaining = {item.id: item for item in candidates}
    selected = []
    while remaining and len(selected) < count:
        best = None
        best_score = -1.0
        for item in remaining.values():
            score = coverage_score(item.features, frequencies, selected_counts)
            score /= 1.0 + max(0, item.mora_count - 35) / 60.0
            if score > best_score or (score == best_score and (best is None or item.id < best.id)):
                best, best_score = item, score
        if best is None:
            break
        selected.append(best)
        selected_counts.update(best.features)
        del remaining[best.id]
    return selected


def assign_packs(selected: list[Candidate], pack_count: int, pack_size: int) -> list[list[Candidate]]:
    global_frequency = Counter(feature for item in selected for feature in item.features)
    packs: list[list[Candidate]] = [[] for _ in range(pack_count)]
    pack_counts: list[Counter[str]] = [Counter() for _ in range(pack_count)]
    for item in selected:
        choices = [index for index, pack in enumerate(packs) if len(pack) < pack_size]
        index = max(
            choices,
            key=lambda candidate: (
                coverage_score(item.features, global_frequency, pack_counts[candidate]),
                -len(packs[candidate]),
                -candidate,
            ),
        )
        packs[index].append(item)
        pack_counts[index].update(item.features)
    return packs


def coverage_summary(items: list[Candidate]) -> dict:
    grouped: dict[str, set[str]] = {}
    for item in items:
        for feature in item.features:
            grouped.setdefault(category(feature), set()).add(feature)
    return {name: len(values) for name, values in sorted(grouped.items())}


def main() -> int:
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8")
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--transcript", required=True, type=Path)
    parser.add_argument("--out", required=True, type=Path)
    parser.add_argument("--report", type=Path)
    parser.add_argument("--supplement", type=Path, help="先頭へ追加するUtauTTS独自prompt pack")
    parser.add_argument("--count", type=int, default=300)
    parser.add_argument("--pack-size", type=int, default=50)
    parser.add_argument("--min-chars", type=int, default=8)
    parser.add_argument("--max-chars", type=int, default=72)
    args = parser.parse_args()
    if args.count < 3 or args.pack_size < 1 or args.count % args.pack_size:
        raise ValueError("count must be divisible by pack-size")

    candidates, rejected = load_candidates(args.transcript, args.min_chars, args.max_chars)
    if len(candidates) < args.count:
        raise ValueError(f"only {len(candidates)} usable prompts for requested {args.count}")
    selected = select_candidates(candidates, args.count)
    packs = assign_packs(selected, args.count // args.pack_size, args.pack_size)
    supplement = None
    if args.supplement:
        supplement = json.loads(args.supplement.read_text(encoding="utf-8"))
        if not supplement.get("id") or not supplement.get("name") or not supplement.get("prompts"):
            raise ValueError("invalid supplement prompt pack")
    output_packs = []
    if supplement:
        output_packs.append(supplement)
    output_packs.extend(
        {
            "id": f"basic5000-pack{index + 1:02d}",
            "name": f"BASIC5000 抑揚セット {index + 1}/{len(packs)}（50文）",
            "source": "jsut-basic5000",
            "prompts": [{"id": item.id, "text": item.text} for item in pack],
        }
        for index, pack in enumerate(packs)
    )
    prompt_set = {
        "id": "jsut-basic5000-prosody-v1",
        "version": 1,
        "language": "ja",
        "name": "JSUT BASIC5000 抑揚調整セット",
        "sources": [
            {
                "id": "utautts-original",
                "name": "UtauTTS original prompts",
                "license": "MIT",
            },
            {
                "id": "jsut-basic5000",
                "name": "JSUT corpus BASIC5000",
                "url": "https://sites.google.com/site/shinnosuketakamichi/publication/jsut",
                "license": "Mixed; see licenses/JSUT-DATA-AND-LABELS.txt",
                "licenses": [
                    {
                        "component": "TANAKA corpus text",
                        "license": "CC BY 2.0",
                        "url": "https://creativecommons.org/licenses/by/2.0/",
                    },
                    {
                        "component": "Japanese Wikipedia text",
                        "license": "CC BY-SA 3.0",
                        "url": "https://creativecommons.org/licenses/by-sa/3.0/",
                    },
                    {
                        "component": "JSUT original sentences",
                        "license": "CC BY-SA 4.0",
                        "url": "https://creativecommons.org/licenses/by-sa/4.0/",
                    },
                ],
                "license_note": "JSUT LICENCE.txtに従う混在データ。田中コーパス、Wikipedia、JSUT独自文を含む。個別の出典条件はlicenses/JSUT-DATA-AND-LABELS.txtを参照する。",
                "transcript_sha256": sha256_file(args.transcript),
                "selection_tool": "tools/select-jsut-prosody-prompts.py",
            },
        ],
        "packs": output_packs,
    }
    report = {
        "format": "utautts-jsut-prosody-selection",
        "format_version": 1,
        "input": {"path": str(args.transcript), "sha256": sha256_file(args.transcript)},
        "parameters": {
            "count": args.count,
            "pack_size": args.pack_size,
            "min_chars": args.min_chars,
            "max_chars": args.max_chars,
            "supplement": str(args.supplement) if args.supplement else "",
        },
        "candidate_count": len(candidates),
        "rejected_count": len(rejected),
        "selected_coverage": coverage_summary(selected),
        "packs": [
            {
                "id": f"pack{index + 1:02d}",
                "count": len(pack),
                "coverage": coverage_summary(pack),
                "prompt_ids": [item.id for item in pack],
            }
            for index, pack in enumerate(packs)
        ],
        "rejected": rejected,
    }
    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text(json.dumps(prompt_set, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    report_path = args.report or args.out.with_name(args.out.stem + "-report.json")
    report_path.parent.mkdir(parents=True, exist_ok=True)
    report_path.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({"out": str(args.out), "report": str(report_path), "selected": len(selected), "coverage": report["selected_coverage"]}, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
