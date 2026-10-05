#!/usr/bin/env python3
"""MFAで抑揚学習用JSONLを整列する。

  prepare  各モーラを1語として、MFA用コーパスと辞書を作る。
  import   整列結果をJSONLへ戻す。

区間はUTAUのノートと同じ「母音開始〜次の母音開始」。休止前はモーラ末で区切る。
"""

from __future__ import annotations

import argparse
import json
import shutil
import unicodedata
from pathlib import Path

import pyopenjtalk

VOWELS = {"a": "a", "i": "i", "u": "ɯ", "e": "e", "o": "o", "A": "a", "I": "i", "U": "ɯ", "E": "e", "O": "o"}
CONSONANTS = {
    "k": "k", "g": "ɡ", "s": "s", "sh": "ɕ", "z": "z", "j": "dʑ", "t": "t", "ch": "tɕ", "ts": "ts", "d": "d",
    "n": "n", "h": "h", "f": "ɸ", "b": "b", "p": "p", "m": "m", "y": "j", "r": "ɾ", "w": "w", "v": "v",
    "dy": "dʲ", "ty": "tʲ", "ky": "c", "gy": "ɟ", "ny": "ɲ", "hy": "ç", "my": "mʲ", "ry": "ɾʲ", "by": "bʲ", "py": "pʲ",
}
# い段の前の子音は、MFAの日本語モデルでは口蓋化した音素になる。
BEFORE_I = {"k": "c", "g": "ɟ", "n": "ɲ", "h": "ç", "m": "mʲ", "r": "ɾʲ", "b": "bʲ", "p": "pʲ"}
# 語（モーラ）の中で母音の始まりとみなす音素。撥音・促音はそれ自体を始まりとする。
ONSET_PHONES = {"a", "i", "ɯ", "e", "o", "aː", "iː", "ɯː", "eː", "oː", "i̥", "ɯ̥", "ɴ", "ʔ"}


def to_katakana(text: str) -> str:
    return "".join(chr(ord(c) + 0x60) if 0x3041 <= ord(c) <= 0x3096 else c for c in text)


def mora_phones(mora: str, previous_vowel: str) -> list[str] | None:
    if mora == "ー":
        return [previous_vowel] if previous_vowel else None
    if mora == "っ":
        return ["ʔ"]
    if mora == "ん":
        return ["ɴ"]
    labels = pyopenjtalk.g2p(to_katakana(mora)).split()
    phones = []
    for index, label in enumerate(labels):
        if label in VOWELS:
            phones.append(VOWELS[label])
        elif label == "N":
            phones.append("ɴ")
        elif label == "cl":
            phones.append("ʔ")
        elif label in CONSONANTS:
            following = labels[index + 1] if index + 1 < len(labels) else ""
            phones.append(BEFORE_I.get(label, CONSONANTS[label]) if following in ("i", "I") else CONSONANTS[label])
        else:
            return None
    return phones or None


def word_name(record_id: str, index: int) -> str:
    return f"{record_id.lower()}_{index:03d}"


def prepare(args: argparse.Namespace) -> int:
    corpus = Path(args.out) / "corpus"
    if corpus.exists():
        shutil.rmtree(corpus)
    corpus.mkdir(parents=True)
    dictionary, prepared, skipped = [], 0, 0
    for path in args.dataset:
        for line in Path(path).read_text(encoding="utf-8").splitlines():
            record = json.loads(line)
            words, entries, previous = [], [], ""
            for index, token in enumerate(record["tokens"]):
                if token.get("pause"):
                    previous = ""
                    continue
                phones = mora_phones(unicodedata.normalize("NFC", token["mora"]), previous)
                if phones is None:
                    words = []
                    break
                vowels = [phone for phone in phones if phone in VOWELS.values()]
                previous = vowels[-1] if vowels else previous
                words.append(word_name(record["id"], index))
                entries.append(f"{words[-1]}\t{' '.join(phones)}")
            if not words:
                skipped += 1
                continue
            shutil.copyfile(record["audio_path"], corpus / f"{record['id']}.wav")
            (corpus / f"{record['id']}.lab").write_text(" ".join(words), encoding="utf-8")
            dictionary.extend(entries)
            prepared += 1
    (Path(args.out) / "dictionary.dict").write_text("\n".join(dictionary) + "\n", encoding="utf-8")
    (Path(args.out) / "config.yaml").write_text("tokenization: simple\n", encoding="utf-8")
    print(f"prepared {prepared} utterances ({skipped} skipped), {len(dictionary)} mora words: {args.out}")
    return 0


def load_alignment(path: Path) -> tuple[list, list]:
    tiers = json.loads(path.read_text(encoding="utf-8"))["tiers"]
    words = [(float(s), float(e), label) for s, e, label in tiers["words"]["entries"] if label and label != "<eps>"]
    phones = [(float(s), float(e), label) for s, e, label in tiers["phones"]["entries"] if label]
    return words, phones


def import_alignments(args: argparse.Namespace) -> int:
    written, skipped = 0, 0
    with Path(args.out).open("w", encoding="utf-8") as stream:
        for path in args.dataset:
            for line in Path(path).read_text(encoding="utf-8").splitlines():
                record = json.loads(line)
                alignment = Path(args.alignments) / f"{record['id']}.json"
                if not alignment.exists():
                    skipped += 1
                    continue
                words, phones = load_alignment(alignment)
                spans = {}
                for start, end, label in words:
                    inside = [phone for phone in phones if phone[0] >= start - 1e-4 and phone[1] <= end + 1e-4]
                    onset = next((phone[0] for phone in reversed(inside) if phone[2] in ONSET_PHONES), start)
                    spans[int(label.rsplit("_", 1)[1])] = (start * 1000, end * 1000, onset * 1000)
                tokens = record["tokens"]
                speech = [index for index, token in enumerate(tokens) if not token.get("pause")]
                if any(index not in spans for index in speech):
                    skipped += 1
                    continue
                for order, index in enumerate(speech):
                    start, end, onset = spans[index]
                    following = speech[order + 1] if order + 1 < len(speech) else None
                    adjacent = following is not None and following == index + 1
                    token = tokens[index]
                    token["start_ms"] = onset
                    token["end_ms"] = spans[following][2] if adjacent else end
                    token["vowel_onset_ms"] = onset
                    token["consonant_onset_ms"] = start
                for index, token in enumerate(tokens):
                    if token.get("pause"):
                        previous = tokens[index - 1]["end_ms"] if index > 0 else 0.0
                        following = tokens[index + 1]["start_ms"] if index + 1 < len(tokens) else previous + 1.0
                        token["start_ms"], token["end_ms"] = previous, max(previous + 1.0, following)
                for token in tokens:
                    token["duration_ms"] = token["end_ms"] - token["start_ms"]
                if any(token["duration_ms"] <= 0 for token in tokens):
                    skipped += 1
                    continue
                record["alignment_source"] = "mfa_japanese_note"
                stream.write(json.dumps(record, ensure_ascii=False) + "\n")
                written += 1
    print(f"wrote {written} records ({skipped} skipped): {args.out}")
    return 0


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    commands = parser.add_subparsers(dest="command", required=True)
    prepare_parser = commands.add_parser("prepare", help="build an MFA corpus and per-mora dictionary")
    prepare_parser.add_argument("--out", required=True, help="output directory (corpus/, dictionary.dict, config.yaml)")
    prepare_parser.add_argument("dataset", nargs="+", help="intonation JSONL from go run ./cmd/tools/prepare-intonation-frame-data")
    import_parser = commands.add_parser("import", help="write MFA timings back into the JSONL")
    import_parser.add_argument("--alignments", required=True, help="MFA output directory (--output_format json)")
    import_parser.add_argument("--out", required=True, help="output JSONL")
    import_parser.add_argument("dataset", nargs="+", help="the same JSONL passed to prepare")
    args = parser.parse_args()
    return prepare(args) if args.command == "prepare" else import_alignments(args)


if __name__ == "__main__":
    raise SystemExit(main())
