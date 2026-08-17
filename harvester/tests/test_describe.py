"""Tests for harvester.describe — the inline-body cap and its truncation note (FIX P0)."""

import json
import math
from pathlib import Path

from harvester import cache as cache_mod
from harvester.describe import (
    DEFAULT_MAX_INLINE_CHARS,
    describe_fetch_result,
    describe_size_result,
)
from harvester.tokens import estimate_tokens


def _ok_result(body: str, *, content_chars: int | None = None) -> dict:
    """A success result dict shaped like dispatch._ok/_hit (see dispatch.py)."""
    return {
        "cache_status": "miss",
        "method": "local-trafilatura",
        "md_path": Path("/tmp/cache/example.md"),
        "body": body,
        "bytes": len(body),
        "content_chars": content_chars if content_chars is not None else len(body.strip()),
        "http_status": 200,
        "error_kind": None,
        "challenge": False,
    }


class TestInlineCap:
    def test_body_over_default_cap_is_truncated_with_note(self):
        body = "A" * (DEFAULT_MAX_INLINE_CHARS + 10_000)
        out = describe_fetch_result("https://example.com/big", _ok_result(body)).text
        # The full body must NOT be inlined.
        assert body not in out
        # The truncation note names the cap, the true length, the md_path, and searchCache.
        assert f"first {DEFAULT_MAX_INLINE_CHARS} of {len(body)} chars" in out
        assert "/tmp/cache/example.md" in out
        assert "searchCache" in out

    def test_truncation_note_is_truthful_no_summarised_no_dead_recovery(self):
        """The note must claim COMPLETE (not "summarised") text at the cache path, name the
        char offset to resume from, and state plainly that searchCache finds pages, not text —
        it must NOT suggest "re-fetch a narrower target" (that recovery path does nothing)."""
        body = "A" * (DEFAULT_MAX_INLINE_CHARS + 10_000)
        out = describe_fetch_result("https://example.com/big", _ok_result(body)).text
        assert "COMPLETE text is at" in out
        assert f"read that file from char {DEFAULT_MAX_INLINE_CHARS}" in out
        assert "does not return text" in out
        assert "summarised" not in out.lower()
        assert "re-fetch a narrower target" not in out

    def test_body_under_cap_is_returned_unchanged(self):
        body = "word " * 200  # 1000 chars: over THIN_MIN_CHARS, under the cap
        out = describe_fetch_result("https://example.com/small", _ok_result(body)).text
        assert body in out
        assert "truncated" not in out

    def test_env_override_changes_threshold(self, monkeypatch):
        monkeypatch.setenv("HARVESTER_MAX_INLINE_CHARS", "100")
        body = "word " * 200  # 1000 chars, content_chars defaults to 1000 (not thin)
        out = describe_fetch_result("https://example.com/x", _ok_result(body)).text
        assert f"first 100 of {len(body)} chars" in out
        assert body not in out

    def test_cap_disabled_when_zero(self, monkeypatch):
        monkeypatch.setenv("HARVESTER_MAX_INLINE_CHARS", "0")
        body = "B" * (DEFAULT_MAX_INLINE_CHARS + 5_000)
        out = describe_fetch_result("https://example.com/full", _ok_result(body)).text
        assert body in out
        assert "truncated" not in out


class TestDescribeSizeResult:
    """`fetch(size_only=True)` rendering: {size, chars, path} as JSON, NO body."""

    def test_success_returns_size_chars_path_no_body(self):
        body = "hello world " * 200  # Latin prose
        out = describe_size_result("https://example.com/big", _ok_result(body)).text
        payload = json.loads(out)
        assert payload["source"] == "https://example.com/big"
        assert payload["chars"] == len(body)
        assert payload["size"] == estimate_tokens(body)
        assert payload["size"] == math.ceil(len(body) / 2)  # over-counting heuristic
        # The token estimate is exposed under explicit aliases too (and matches the frontmatter field).
        assert payload["tokens"] == payload["size"]
        assert payload["token_count"] == payload["size"]
        assert payload["path"] == "/tmp/cache/example.md"
        # The body itself is never inlined in a size probe.
        assert "hello world" not in out

    def test_empty_body_reports_error_not_zero(self):
        # A fetched-but-empty result must never come back as a silent size 0.
        out = describe_size_result("https://example.com/stub", _ok_result("")).text
        assert out.startswith("# https://example.com/stub\nERROR")
        assert "no readable content" in out
        # No JSON payload with a zero size.
        assert '"size": 0' not in out

    def test_error_dict_falls_back_to_error_rendering(self):
        out = describe_size_result("https://x.example/", {"error": "paywalled", "body": ""}).text
        assert out.startswith("# https://x.example/\nERROR")
        assert "paywalled" in out

    def test_exception_falls_back_to_error_rendering(self):
        out = describe_size_result("https://x.example/", ConnectionError("reset by peer")).text
        assert "ERROR" in out
        assert "ConnectionError" in out


class TestHeaderEnrichment:
    """The result header gains `tokens` and `fetched_at`, read from the cache frontmatter that
    `cache._write_md` already writes — never re-derived so it can't drift from what's on disk."""

    def test_header_includes_tokens_and_fetched_at_from_real_frontmatter(self, tmp_path):
        md_path = tmp_path / "doc.md"
        body = "word " * 400  # 2000 chars
        cache_mod._write_md(md_path, "https://example.com/x", "local-trafilatura", body)
        meta, written_body = cache_mod.split_frontmatter(md_path.read_text(encoding="utf-8"))

        # describe_fetch_result reads md_path off the result dict — point it at the real file.
        result = _ok_result(written_body)
        result["md_path"] = md_path
        out = describe_fetch_result("https://example.com/x", result).text

        assert f"tokens: {meta['token_count']}" in out
        assert f"fetched_at: {meta['fetched_at']}" in out

    def test_header_falls_back_gracefully_with_no_frontmatter(self, tmp_path):
        """A cache artifact with no frontmatter (e.g. an image path) must not blow up the
        header — tokens fall back to a fresh estimate, fetched_at to "unknown"."""
        body = "word " * 50
        result = _ok_result(body)
        result["md_path"] = tmp_path / "does-not-exist.png"
        out = describe_fetch_result("https://example.com/img", result).text
        assert f"tokens: {estimate_tokens(body)}" in out
        assert "fetched_at: unknown" in out


class TestFailureMessageNamesNextMove:
    """Every terminal error the model sees must name a concrete next tool/argument — Slice 1
    deliverable 7. describe.py derives these through the shared `net.failure_message` helper."""

    def test_invalid_url_names_search(self):
        out = describe_fetch_result(
            "not a url", _ok_result("", content_chars=0) | {"http_status": None, "error_kind": "invalid"}
        ).text
        assert "Invalid URL" in out
        assert "`search`" in out

    def test_connection_error_names_search(self):
        result = _ok_result("", content_chars=0) | {"http_status": None, "error_kind": "timeout"}
        out = describe_fetch_result("https://slow.example/", result).text
        assert "timed out" in out.lower()
        assert "`search`" in out

    def test_challenge_names_search(self):
        result = _ok_result("", content_chars=0) | {"http_status": 200, "challenge": True}
        out = describe_fetch_result("https://walled.example/", result).text
        assert "challenge" in out.lower()
        assert "`search`" in out

    def test_http_404_names_search_or_findworks(self):
        result = _ok_result("tiny", content_chars=4) | {"http_status": 404}
        out = describe_fetch_result("https://x.example/missing", result).text
        assert "HTTP 404" in out
        assert "`search`" in out and "`findWorks`" in out

    def test_thin_extraction_names_search_and_findworks(self):
        result = _ok_result("", content_chars=0) | {"http_status": 200}
        out = describe_fetch_result("https://js-only.example/", result).text
        assert "no readable content" in out.lower()
        assert "`search`" in out and "`findWorks`" in out
