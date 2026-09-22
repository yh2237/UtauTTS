"""Shared Open JTalk to mora-level linguistic feature conversion."""

import unicodedata

import pyopenjtalk


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
    for character in unicodedata.normalize("NFC", reading.replace("'", "").replace("’", "")):
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


def analyze(text):
    nodes = pyopenjtalk.run_frontend(text)
    reading_parts = []
    result = []
    index = 0
    while index < len(nodes):
        node = nodes[index]
        pronunciation = node.get("pron", "").replace("'", "").replace("’", "")
        if int(node.get("mora_size", 0)) == 0 or node.get("string") in PUNCTUATION:
            reading_parts.append(node.get("string", "、"))
            if result and not result[-1]["pause"]:
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
            current_pronunciation = current.get("pron", "").replace("'", "").replace("’", "")
            morae = [item for item in split_morae(current_pronunciation) if not item["pause"]]
            phrase_nodes.append((current, morae))
            reading_parts.append(current_pronunciation)
            index += 1

        phrase_length = sum(len(morae) for _, morae in phrase_nodes)
        accent = int(phrase_nodes[0][0].get("acc", 0))
        phrase_position = 0
        for current, morae in phrase_nodes:
            for word_position, mora in enumerate(morae, 1):
                phrase_position += 1
                result.append({
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
                })
    return "".join(reading_parts), result
