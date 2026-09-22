"""Open JTalk（pyopenjtalk）からモーラ単位の言語特徴への変換。

共通変換は :mod:`openjtalk_feature_common` にある。本モジュールは
pyopenjtalkフロントエンドを束ね、学習・準備スクリプトが ``analyze(text)`` を
使い続けられるようにするだけ。
"""

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
