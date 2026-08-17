"""Tests for harvester.tokens — the over-counting char→token heuristic (no tiktoken)."""

import math

from harvester.tokens import estimate_tokens


class TestEstimateTokens:
    def test_empty_is_zero(self):
        assert estimate_tokens("") == 0

    def test_latin_prose_uses_half(self):
        body = "word " * 200  # 1000 Latin chars
        assert estimate_tokens(body) == math.ceil(1000 / 2)

    def test_prose_rounds_up(self):
        assert estimate_tokens("abc") == math.ceil(3 / 2)  # 2, not 1

    def test_code_density_uses_1_8_divisor(self):
        code = "a=[1];b={k:v};c=(x)+y*z/w;" * 40  # symbol-dense
        n = len(code)
        assert estimate_tokens(code) == math.ceil(n / 1.8)
        # The denser divisor yields MORE tokens than the prose divisor would.
        assert estimate_tokens(code) > math.ceil(n / 2)

    def test_cjk_uses_1_3_multiplier(self):
        body = "中文测试" * 25  # 100 CJK chars
        assert estimate_tokens(body) == math.ceil(100 * 1.3)

    def test_korean_is_treated_as_east_asian(self):
        body = "가나다라" * 25  # 100 Hangul syllables
        assert estimate_tokens(body) == math.ceil(100 * 1.3)

    def test_cjk_detection_wins_over_symbol_density(self):
        # CJK present + symbols → still the ×1.3 (highest, safest) regime.
        body = "中{}[]" * 50
        assert estimate_tokens(body) == math.ceil(len(body) * 1.3)

    def test_cjk_estimate_exceeds_prose_for_same_length(self):
        n = 100
        assert estimate_tokens("中" * n) > estimate_tokens("a " * (n // 2))

    def test_never_under_counts_a_loose_real_lower_bound(self):
        # Real Claude prose is ~3.5–4 chars/token; n/4 is a generous lower bound the
        # over-counting heuristic must always stay at or above.
        for body in ("The quick brown fox. " * 100, "Lorem ipsum dolor sit amet " * 50):
            assert estimate_tokens(body) >= len(body) // 4

    def test_monotonic_in_length(self):
        small = estimate_tokens("word " * 100)
        large = estimate_tokens("word " * 200)
        assert large >= small
