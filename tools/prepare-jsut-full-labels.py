"""Build frame-TCN records from the actual JSUT phone/context labels.

No inferred text/phone alignment or nearest-phone substitution is used.
Unknown phone sequences and malformed accent fields fail the build.
"""
import argparse
import json
import re
from pathlib import Path

import pyopenjtalk


def phone_map():
    result = {("N",): "ん", ("cl",): "っ"}
    kana = list("あいうえおかきくけこがぎぐげござじずぜぞさしすせそたちつてとだぢづでどなにぬねのはひふへほばびぶべぼぱぴぷぺぽまみむめもやゆよらりるれろわ")
    kana += [c + v for c in "きぎしじちにひびぴみり" for v in "ゃゅょ"]
    kana += ["ふぁ", "ふぃ", "ふぇ", "ふぉ", "てぃ", "でぃ", "とぅ", "どぅ", "しぇ", "じぇ", "ちぇ", "つぁ", "つぃ", "つぇ", "つぉ", "うぃ", "うぇ", "うぉ", "ゔぁ", "ゔぃ", "ゔ", "ゔぇ", "ゔぉ"]
    for text in kana:
        phones = tuple(pyopenjtalk.g2p(text).split())
        result.setdefault(phones, text)
    result[("h", "a")] = "は"
    result[("h", "e")] = "へ"
    result[("w", "a")] = "わ"
    result[("dy", "u")] = "でゅ"
    result[("ty", "u")] = "てゅ"
    return result


def convert(text, mapping):
    tokens, pending = [], []
    previous_end = 0
    for line in text.splitlines():
        start, end, context = line.split()
        start, end = int(start), int(end)
        if start != previous_end or end <= start:
            raise ValueError("noncontiguous or invalid time interval")
        previous_end = end
        phone = re.search(r"-([^+]+)\+", context).group(1)
        if phone == "sil":
            if pending:
                raise ValueError("unfinished phone sequence")
            continue
        if phone == "pau":
            if pending:
                raise ValueError("pause inside mora")
            tokens.append(dict(mora="", pause=True, start_ms=start / 10000, end_ms=end / 10000, duration_ms=(end-start)/10000))
            continue
        pending.append((phone, start))
        if phone not in {"a", "i", "u", "e", "o", "A", "I", "U", "E", "O", "N", "cl"}:
            continue
        key = tuple(p.lower() if p in "AIUEO" else p for p, _ in pending)
        if key not in mapping:
            raise ValueError(f"unknown phone sequence {key}")
        a = re.search(r"/A:([^/]+)", context).group(1).split("+")
        f = re.search(r"/F:(\d+)_(\d+)", context)
        position, length, nucleus = int(a[1]), int(f[1]), int(f[2])
        if not 1 <= position <= length or not 0 <= nucleus <= length:
            raise ValueError("invalid accent context")
        tokens.append(dict(mora=mapping[key], pause=False,
                           start_ms=pending[0][1]/10000, end_ms=end/10000,
                           duration_ms=(end-pending[0][1])/10000,
                           accent_phrase_position=position, accent_phrase_length=length,
                           accent_nucleus=nucleus,
                           accent_high=(position == 1 if nucleus == 1 else position > 1 and (nucleus == 0 or position <= nucleus)),
                           accent_phrase_start=position == 1, accent_phrase_end=position == length,
                           pos="*", pos_group1="*", word_start=False, word_end=False))
        pending = []
    if pending or not tokens:
        raise ValueError("incomplete labels")
    return tokens


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--labels", required=True)
    parser.add_argument("--corpus", required=True)
    parser.add_argument("--out", required=True)
    args = parser.parse_args()
    corpus = Path(args.corpus)
    transcripts = dict(line.split(":", 1) for line in (corpus / "transcript_utf8.txt").read_text(encoding="utf-8").splitlines())
    mapping = phone_map()
    records = []
    for index in range(1, 5001):
        key = f"BASIC5000_{index:04d}"
        wav = (corpus / "wav" / (key + ".wav")).resolve()
        if not wav.is_file():
            raise ValueError(f"missing audio {wav}")
        try:
            tokens = convert((Path(args.labels) / (key + ".lab")).read_text(encoding="utf-8"), mapping)
        except Exception as error:
            raise ValueError(f"{key}: {error}") from error
        records.append(dict(version=1, id=key, text=transcripts[key], audio_path=str(wav),
                            tokens=tokens, accent_source="jsut_context_labels"))
    out = Path(args.out)
    out.parent.mkdir(parents=True, exist_ok=True)
    with out.open("x", encoding="utf-8") as stream:
        for record in records:
            stream.write(json.dumps(record, ensure_ascii=False) + "\n")
    print(f"wrote {len(records)} records without dropped utterances: {out}")


if __name__ == "__main__":
    main()
