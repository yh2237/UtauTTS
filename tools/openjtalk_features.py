"""pyopenjtalkを共通変換へ渡し、既存のanalyze(text)の入口を保つ。"""

import pyopenjtalk

from openjtalk_feature_common import (  # noqa: F401
    PUNCTUATION,
    SMALL_KANA,
    VOWEL_GROUPS,
    is_high,
    split_morae,
    to_hiragana,
    vowel_of,
)
from openjtalk_feature_common import analyze as _analyze


def analyze(text):
    """pyopenjtalkフロントエンドで ``text`` の ``(reading, tokens)`` を返す。"""

    return _analyze(pyopenjtalk, text)
