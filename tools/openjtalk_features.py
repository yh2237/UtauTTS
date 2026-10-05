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
    return _analyze(pyopenjtalk, text)
