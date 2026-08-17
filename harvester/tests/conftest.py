"""Shared test fixtures."""

import pytest

from harvester import dispatch


@pytest.fixture(autouse=True)
def isolated_cache(tmp_path, monkeypatch):
    """Redirect the cache to a per-test tmp dir so no test touches the real cache dir.

    Test modules that define their own `isolated_cache` override this one (identical behavior);
    modules without it inherit this so cache lookups stay hermetic.
    """
    fetch_dir = tmp_path / ".cache"
    fetch_dir.mkdir()
    monkeypatch.setenv("WEBFETCH_DIR", str(fetch_dir))
    return fetch_dir


@pytest.fixture(autouse=True)
def clear_dispatch_caches():
    """get_or_fetch keeps a module-level negative-result cache and in-flight map; clear them
    around every test so error results never leak across tests (and so the dedup/neg-cache
    tests start from a clean slate)."""
    dispatch._NEG_CACHE.clear()
    dispatch._INFLIGHT.clear()
    yield
    dispatch._NEG_CACHE.clear()
    dispatch._INFLIGHT.clear()
