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


# ── autouse live-network block ───────────────────────────────────────────────────
# No hermetic test may touch the live internet — a forgotten mock used to surface
# as a silent 30s hang against an external service (the defuddle leak, walk 2).
# The opt-in LIVE suites set the escape env vars and the block stands down.
import os

import pytest


@pytest.fixture(autouse=True)
def _block_live_network(monkeypatch):
    if os.environ.get("RUN_OA_INTEGRATION") or os.environ.get("RUN_BROWSER"):
        yield
        return

    import socket

    def _refuse(*args, **kwargs):
        raise AssertionError(
            "live network access attempted in a hermetic test — mock the "
            "transport (monkeypatch net.fetch_* / use a FakeClient), or run "
            "with RUN_OA_INTEGRATION=1 / RUN_BROWSER=1 for the live suites")

    # NOTE: getaddrinfo stays FREE — pure DNS resolution is part of the SSRF
    # guards under test, not a network side effect. Only real connections die.
    monkeypatch.setattr(socket.socket, "connect", _refuse)
    monkeypatch.setattr(socket, "create_connection", _refuse)
    yield
