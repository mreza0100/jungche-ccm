"""Cache location, expiry, and force-refresh.

The failure these guard against is silent by nature: a cache that serves month-old content while
looking exactly like a successful fetch. Nothing in the response says "this is stale" — so the
staleness rule itself has to be pinned.
"""

import os
from datetime import datetime, timedelta, timezone
from pathlib import Path

import pytest

from harvester import cache, dispatch


def _write_artifact(root: Path, kind: str, key: str, body: str, age_hours: float = 0.0) -> Path:
    """Write a cached artifact whose frontmatter claims it was fetched `age_hours` ago."""
    md = cache.cache_file(key, kind, ".md")
    when = datetime.now(timezone.utc) - timedelta(hours=age_hours)
    md.write_text(
        "---\n"
        f"url: {key}\n"
        f"fetched_at: {when.strftime('%Y-%m-%dT%H:%M:%SZ')}\n"
        "source: harvester\n"
        "method: test\n"
        "---\n\n" + body,
        encoding="utf-8",
    )
    return md


# ── where the cache lives ────────────────────────────────────────────────────


def test_cache_dir_defaults_to_dot_cache(monkeypatch, tmp_path):
    monkeypatch.delenv("WEBFETCH_DIR", raising=False)
    monkeypatch.delenv("HARVESTER_CACHE_DIR", raising=False)
    assert cache.cache_root().name == ".cache"


def test_cache_dir_name_comes_from_env(monkeypatch, tmp_path):
    """The directory name is configuration, not a constant baked into the code."""
    monkeypatch.delenv("WEBFETCH_DIR", raising=False)
    monkeypatch.setenv("HARVESTER_CACHE_DIR", ".harvest-cache")
    root = cache.cache_root()
    assert root.name == ".harvest-cache"
    # A relative name stays INSIDE harvester rather than following the working directory.
    assert (root.parent / "pyproject.toml").is_file()


def test_absolute_cache_dir_is_used_verbatim(monkeypatch, tmp_path):
    monkeypatch.delenv("WEBFETCH_DIR", raising=False)
    monkeypatch.setenv("HARVESTER_CACHE_DIR", str(tmp_path / "elsewhere"))
    assert cache.cache_root() == tmp_path / "elsewhere"


def test_webfetch_dir_still_overrides(monkeypatch, tmp_path):
    """The legacy variable keeps working — the whole test suite relies on it."""
    monkeypatch.setenv("WEBFETCH_DIR", str(tmp_path / "legacy"))
    monkeypatch.setenv("HARVESTER_CACHE_DIR", ".ignored")
    assert cache.cache_root() == tmp_path / "legacy"


# ── expiry ───────────────────────────────────────────────────────────────────


@pytest.mark.parametrize("kind", sorted(cache.VOLATILE_KINDS))
def test_volatile_artifact_older_than_a_day_is_stale(kind, isolated_cache):
    md = _write_artifact(isolated_cache, kind, f"https://x.example/{kind}", "body", age_hours=25)
    assert cache.is_stale(md, kind, cache.read_frontmatter(md)) is True


@pytest.mark.parametrize("kind", sorted(cache.VOLATILE_KINDS))
def test_volatile_artifact_within_a_day_is_fresh(kind, isolated_cache):
    md = _write_artifact(isolated_cache, kind, f"https://x.example/{kind}", "body", age_hours=23)
    assert cache.is_stale(md, kind, cache.read_frontmatter(md)) is False


@pytest.mark.parametrize("kind", ["png", "jpg", "image", "archive_member", "zip"])
def test_static_kinds_never_expire(kind, isolated_cache):
    """Images and archive members are content addressed by URL, not publications that get edited."""
    md = _write_artifact(isolated_cache, kind, f"https://x.example/{kind}", "body", age_hours=24 * 365)
    assert cache.is_stale(md, kind, cache.read_frontmatter(md)) is False


def test_ttl_is_configurable(monkeypatch, isolated_cache):
    md = _write_artifact(isolated_cache, "html", "https://x.example/a", "body", age_hours=2)
    assert cache.is_stale(md, "html") is False
    monkeypatch.setenv("HARVESTER_CACHE_TTL", "3600")   # 1 hour
    assert cache.is_stale(md, "html") is True


def test_zero_ttl_disables_expiry(monkeypatch, isolated_cache):
    md = _write_artifact(isolated_cache, "html", "https://x.example/a", "body", age_hours=24 * 365)
    monkeypatch.setenv("HARVESTER_CACHE_TTL", "0")
    assert cache.is_stale(md, "html") is False


def test_invalid_ttl_falls_back_to_the_default(monkeypatch):
    monkeypatch.setenv("HARVESTER_CACHE_TTL", "not-a-number")
    assert cache.cache_ttl() == cache.DEFAULT_CACHE_TTL


def test_unreadable_timestamp_is_treated_as_fresh(isolated_cache, monkeypatch):
    """A missing/corrupt timestamp must not turn the cache into a refetch storm against a server
    that did nothing wrong. Degrade to serving, not to hammering."""
    md = cache.cache_file("https://x.example/a", "html", ".md")
    md.write_text("no frontmatter here", encoding="utf-8")
    monkeypatch.setattr(Path, "stat", lambda self: (_ for _ in ()).throw(OSError("nope")))
    assert cache.is_stale(md, "html", {}) is False


def test_frontmatter_wins_over_mtime(isolated_cache):
    """Touching a file must not silently extend its life — age comes from when it was FETCHED."""
    md = _write_artifact(isolated_cache, "html", "https://x.example/a", "body", age_hours=48)
    os.utime(md, None)   # mtime = now
    assert cache.is_stale(md, "html", cache.read_frontmatter(md)) is True


# ── force refresh ────────────────────────────────────────────────────────────


def test_refresh_context_defaults_off():
    assert dispatch.force_refresh() is False


def test_may_serve_respects_refresh(isolated_cache):
    md = _write_artifact(isolated_cache, "html", "https://x.example/a", "body", age_hours=1)
    assert dispatch._may_serve(md, "html", {}) is True
    token = dispatch._FORCE_REFRESH.set(True)
    try:
        assert dispatch._may_serve(md, "html", {}) is False
    finally:
        dispatch._FORCE_REFRESH.reset(token)


def test_may_reuse_binary_respects_refresh(isolated_cache):
    binary = cache.cache_file("https://x.example/a.pdf", "pdf", ".pdf")
    binary.write_bytes(b"%PDF-1.4")
    assert dispatch._may_reuse_binary(binary) is True
    token = dispatch._FORCE_REFRESH.set(True)
    try:
        assert dispatch._may_reuse_binary(binary) is False
    finally:
        dispatch._FORCE_REFRESH.reset(token)


async def test_refresh_bypasses_the_cached_artifact(isolated_cache, monkeypatch):
    """The end-to-end promise: refresh returns NEW content and overwrites the cached copy."""
    url = "https://x.example/page"
    _write_artifact(isolated_cache, "html", url, "STALE BODY " * 40)

    async def fake_fetch(src, user_agent, proxy_url=None):
        return "<html><body>" + ("FRESH BODY " * 60) + "</body></html>"

    monkeypatch.setattr(dispatch.net, "fetch_raw", fake_fetch)
    monkeypatch.setattr(
        dispatch.net, "fetch_bytes_with_meta",
        lambda *a, **k: _async(( b"<html><body>" + b"FRESH BODY " * 60 + b"</body></html>",
                                 200, None, "text/html")),
    )

    cached = await dispatch.get_or_fetch(url, "ua", media="deny")
    assert "STALE BODY" in cached["body"]

    fresh = await dispatch.get_or_fetch(url, "ua", media="deny", refresh=True)
    assert "FRESH BODY" in fresh["body"]
    assert "STALE BODY" not in fresh["body"]

    # ...and the cache now holds the new copy, so the NEXT plain fetch sees it too.
    again = await dispatch.get_or_fetch(url, "ua", media="deny")
    assert "FRESH BODY" in again["body"]


async def _async(value):
    return value


async def test_refresh_bypasses_the_negative_cache(isolated_cache, monkeypatch):
    """A caller explicitly retrying a failed source must not be handed the remembered failure."""
    url = "https://x.example/flaky"
    calls = {"n": 0}

    async def flaky(item, user_agent, proxy_url, media):
        calls["n"] += 1
        if calls["n"] == 1:
            return {"error": "boom", "body": ""}
        return {"body": "recovered", "cache_status": "miss", "method": "direct",
                "md_path": None, "error": None}

    monkeypatch.setattr(dispatch, "_dispatch_one", flaky)

    first = await dispatch.get_or_fetch(url, "ua")
    assert "boom" in first["error"]

    # Without refresh, the negative cache answers and the source is not retried.
    second = await dispatch.get_or_fetch(url, "ua")
    assert "boom" in second["error"]
    assert calls["n"] == 1

    third = await dispatch.get_or_fetch(url, "ua", refresh=True)
    assert third["body"] == "recovered"
    assert calls["n"] == 2
