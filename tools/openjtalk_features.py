"""Open JTalk (pyopenjtalk) to mora-level linguistic feature conversion.

The shared conversion lives in :mod:`openjtalk_feature_common`.  This module only
binds the pyopenjtalk frontend so training and preparation scripts keep using
``analyze(text)``.
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
    """Return ``(reading, tokens)`` for ``text`` using the pyopenjtalk frontend."""

    return _analyze(pyopenjtalk, text)
