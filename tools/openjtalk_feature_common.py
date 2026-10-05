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


def is_auxiliary_after_te(previous, node):
    return (
        previous.get("pos") == "助詞"
        and previous.get("pos_group1") == "接続助詞"
        and previous.get("string") in ("て", "で")
        and node.get("pos") == "動詞"
        and node.get("pos_group1") == "非自立"
    )


def is_accented_sahen_verb(previous, node):
    """accは補助動詞を連結した後の句全体の核。"""
    return (
        previous.get("pos") == "名詞"
        and previous.get("pos_group1") == "サ変接続"
        and node.get("pos") == "動詞"
        and node.get("orig") == "する"
        and int(node.get("acc", 0)) > 0
    )


def _chain_phrases(nodes, joins):
    head, length = None, 0
    for index, node in enumerate(nodes):
        if int(node.get("mora_size", 0)) == 0 or node.get("string") in PUNCTUATION:
            head, length = None, 0
            continue
        chain_flag = int(node.get("chain_flag", 0))
        if head is not None and chain_flag != 1 and joins(nodes[index - 1], node):
            node["chain_flag"] = 1
            if int(head.get("acc", 0)) == 0 and int(node.get("acc", 0)) > 0:
                head["acc"] = length + int(node["acc"])
        elif head is None or chain_flag != 1:
            head, length = node, 0
        length += sum(1 for item in split_morae(node.get("pron", "")) if not item["pause"])


def chain_accent_phrases(nodes):
    """分けすぎた補助動詞・サ変の句をつなぐ。
    前句に核があれば後句の核を消し、なければ後句の核を残す。
    サ変の判定には連結後の核が必要なため、最後に処理する。
    """
    nodes = [dict(node) for node in nodes]
    _chain_phrases(nodes, is_auxiliary_after_te)
    _chain_phrases(nodes, is_accented_sahen_verb)
    return nodes


CASE_PARTICLES_BEFORE_TOPIC = ("に", "で", "と", "へ", "から", "まで", "より")


def is_topic_particle(node):
    return node.get("pos") == "助詞" and node.get("pos_group1") == "係助詞" and node.get("string") in ("は", "も")


def takes_accent_before_topic(previous):
    if previous.get("pos") != "助詞":
        return False
    if previous.get("pos_group1") == "格助詞":
        return previous.get("string") in CASE_PARTICLES_BEFORE_TOPIC
    if previous.get("pos_group1") == "接続助詞":
        return previous.get("string") in ("て", "で")
    return False


def accent_before_topic_particles(nodes):
    """平板句の「には」「ても」などは、係助詞の直前に核を置く。"""
    head, length = None, 0
    for index, node in enumerate(nodes):
        if int(node.get("mora_size", 0)) == 0 or node.get("string") in PUNCTUATION:
            head, length = None, 0
            continue
        if head is None or int(node.get("chain_flag", 0)) != 1:
            head, length = node, 0
        elif (int(head.get("acc", 0)) == 0 and length > 1 and is_topic_particle(node)
              and takes_accent_before_topic(nodes[index - 1])):
            head["acc"] = length
        length += sum(1 for item in split_morae(node.get("pron", "")) if not item["pause"])


def refine_accent_phrases(nodes):
    """Open JTalkの句と核を、聴取とjsut-label（人手のアクセント）で確かめた規則で直す。"""
    nodes = chain_accent_phrases(nodes)
    accent_before_topic_particles(nodes)
    return nodes


def analyze(frontend, text):
    nodes = refine_accent_phrases(frontend.run_frontend(text))
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
