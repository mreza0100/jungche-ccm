"""Token-size estimation via an over-counting character heuristic — no tiktoken dependency.

`fetch(size_only=True)` and the cache frontmatter report a size in tokens by calling
`estimate_tokens`. It deliberately OVER-counts (never under-counts Claude's real tokenizer)
so the number is safe to budget against: a reader told "N tokens" is never handed more than N.
Three regimes, chosen by content:

* East-Asian (CJK / kana / Hangul) present -> chars * 1.3  (these tokenize ~1 token/char)
* code / markup heavy                      -> chars / 1.8  (denser than prose)
* Latin prose (default)                    -> chars / 2    (prose ~3.5-4 chars/token)

The estimate always rounds UP, so it never reports fewer tokens than the regime implies.
"""

import math
import re

# Any Hiragana/Katakana/CJK-ideograph/Hangul/full-width char -> treat the doc as East-Asian dense.
_CJK_RE = re.compile(
    "[぀-ヿ㐀-䶿一-鿿가-힯豈-﫿＀-￯]"
)
# Punctuation / symbol density that signals code or markup rather than prose.
_SYMBOL_RE = re.compile(r"[{}\[\]()<>;=+\-*/\\|&^%$#@~`_]")
_SYMBOL_DENSITY = 0.05


def estimate_tokens(text: str) -> int:
    """Over-counting token estimate for *text* (see module docstring); always rounds up."""
    n = len(text)
    if n == 0:
        return 0
    if _CJK_RE.search(text):
        return math.ceil(n * 1.3)
    if len(_SYMBOL_RE.findall(text)) / n >= _SYMBOL_DENSITY:
        return math.ceil(n / 1.8)
    return math.ceil(n / 2)
