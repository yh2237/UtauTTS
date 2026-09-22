"""Open JTalkのnode列をUtauTTSのモーラ特徴へ変換する。"""

from __future__ import annotations

import unicodedata


PUNCTUATION = {"、", "。", "？", "！", ",", ".", "?", "!"}
SMALL_KANA = set("ぁぃぅぇぉゃゅょゎゕゖ")

# frontend.ParseKanaのvowelOfと同じ母音対応。学習特徴をGo推論と揃える。
VOWEL_GROUPS = (
    ("あかがさざただなはばぱまやらわぁゃゎ", "a"),
    ("いきぎしじちぢにひびぴみりゐぃ", "i"),
    ("うくぐすずつづぬふぶぷむゆるゔぅゅ", "u"),
    ("えけげせぜてでねへべぺめれゑぇ", "e"),
    ("おこごそぞとどのほぼぽもよろをぉょ", "o"),
)


def to_hiragana(character):
    code = ord(character)
    if 0x30A1 <= code <= 0x30F6:
        return chr(code - 0x60)
    return character


def vowel_of(character, fallback=""):
    for characters, vowel in VOWEL_GROUPS:
        if character in characters:
            return vowel
    if character == "ん":
        return "n"
    if character == "っ":
        return "cl"
    return fallback


def split_morae(reading):
    result = []
    normalized = unicodedata.normalize("NFC", reading.replace("'", "").replace("’", ""))
    for character in normalized:
        if character.isspace() or character in PUNCTUATION:
            if result and not result[-1]["pause"]:
                result.append({"mora": "", "vowel": "", "pause": True})
            continue
        mora = to_hiragana(character)
        if mora in SMALL_KANA and result and not result[-1]["pause"]:
            result[-1]["mora"] += mora
            result[-1]["vowel"] = vowel_of(mora, result[-1]["vowel"])
            continue
        if mora == "ー":
            previous = result[-1]["vowel"] if result and not result[-1]["pause"] else ""
            result.append({"mora": "ー", "vowel": previous, "pause": False})
            continue
        result.append({"mora": mora, "vowel": vowel_of(mora), "pause": False})
    return result


def is_high(position, accent):
    if accent == 1:
        return position == 1
    if accent > 1:
        return 2 <= position <= accent
    return position >= 2


def sparse_features(token):
    if token.get("pause") or "accent_phrase_position" not in token:
        return {}
    phrase_length = max(1, int(token["accent_phrase_length"]))
    phrase_position = int(token["accent_phrase_position"])
    nucleus = int(token["accent_nucleus"])
    result = {
        "accent_position": phrase_position / phrase_length,
        "accent_from_end": (phrase_length - phrase_position) / phrase_length,
        "accent_nucleus_position": nucleus / phrase_length,
        "accent_high": float(bool(token["accent_high"])),
        "accent_phrase_start": float(bool(token["accent_phrase_start"])),
        "accent_phrase_end": float(bool(token["accent_phrase_end"])),
        "word_start": float(bool(token["word_start"])),
        "word_end": float(bool(token["word_end"])),
        f"pos={token.get('pos', '*')}": 1.0,
        f"pos_group1={token.get('pos_group1', '*')}": 1.0,
    }
    if nucleus == 0:
        result["accent_type=heiban"] = 1.0
    elif phrase_position < nucleus:
        result["accent_type=before"] = 1.0
    elif phrase_position == nucleus:
        result["accent_type=nucleus"] = 1.0
    else:
        result["accent_type=after"] = 1.0
    return result


def analyze(frontend, text):
    nodes = frontend.run_frontend(text)
    reading_parts = []
    result = []
    index = 0
    while index < len(nodes):
        node = nodes[index]
        if int(node.get("mora_size", 0)) == 0 or node.get("string") in PUNCTUATION:
            reading_parts.append(node.get("string", "、"))
            if result and not result[-1].get("pause"):
                result.append({"mora": "", "pause": True})
            index += 1
            continue

        phrase_nodes = []
        while index < len(nodes):
            current = nodes[index]
            if int(current.get("mora_size", 0)) == 0 or current.get("string") in PUNCTUATION:
                break
            if phrase_nodes and int(current.get("chain_flag", 0)) != 1:
                break
            pronunciation = current.get("pron", "").replace("'", "").replace("’", "")
            morae = [item for item in split_morae(pronunciation) if not item["pause"]]
            phrase_nodes.append((current, morae))
            reading_parts.append(pronunciation)
            index += 1

        phrase_length = sum(len(morae) for _, morae in phrase_nodes)
        accent = int(phrase_nodes[0][0].get("acc", 0))
        phrase_position = 0
        for current, morae in phrase_nodes:
            for word_position, mora in enumerate(morae, 1):
                phrase_position += 1
                result.append(
                    {
                        "mora": mora["mora"],
                        "vowel": mora["vowel"],
                        "pause": False,
                        "accent_phrase_position": phrase_position,
                        "accent_phrase_length": phrase_length,
                        "accent_nucleus": accent,
                        "accent_high": is_high(phrase_position, accent),
                        "accent_phrase_start": phrase_position == 1,
                        "accent_phrase_end": phrase_position == phrase_length,
                        "word_start": word_position == 1,
                        "word_end": word_position == len(morae),
                        "pos": current.get("pos", "*"),
                        "pos_group1": current.get("pos_group1", "*"),
                    }
                )
    return "".join(reading_parts), result
