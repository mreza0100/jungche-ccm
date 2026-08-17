"""Unit tests for harvester.search — the SearXNG + Brave web-search backend (hermetic)."""
from harvester import search


class FakeResp:
    def __init__(self, status=200, json_data=None):
        self.status_code = status
        self._json = json_data

    def json(self):
        if self._json is None:
            raise ValueError("no json")
        return self._json


class FakeClient:
    def __init__(self, routes):
        self.routes = routes
        self.calls = []

    async def get(self, url, **kw):
        self.calls.append((url, kw.get("params")))
        for sub, resp in self.routes:
            if sub in url:
                return resp
        return FakeResp(404)


SEARX_JSON = {"results": [
    {"title": "A", "url": "https://a.example", "content": "snippet a", "engines": ["google", "brave"]},
    {"title": "B", "url": "https://b.example", "content": "snippet b", "engine": "ddg"},
    {"title": "no url — skipped"},
]}
BRAVE_JSON = {"web": {"results": [
    {"title": "C", "url": "https://c.example", "description": "desc c"},
]}}


class TestSearXNG:
    async def test_returns_results(self, monkeypatch):
        monkeypatch.setattr(search, "SEARXNG_URL", "http://localhost:8888")
        c = FakeClient([("localhost:8888/search", FakeResp(json_data=SEARX_JSON))])
        res, backend = await search.web_search("x", c)
        assert backend == "searxng"
        assert [r["url"] for r in res] == ["https://a.example", "https://b.example"]
        assert res[0]["engine"] == "google,brave"

    async def test_lang_and_engines_passed(self, monkeypatch):
        monkeypatch.setattr(search, "SEARXNG_URL", "http://localhost:8888")
        c = FakeClient([("localhost:8888/search", FakeResp(json_data=SEARX_JSON))])
        await search.web_search("x", c, lang="zh", engines="baidu")
        params = c.calls[0][1]
        assert params["language"] == "zh" and params["engines"] == "baidu"


class TestBackendSelection:
    async def test_unconfigured_returns_none(self, monkeypatch):
        monkeypatch.setattr(search, "SEARXNG_URL", "")
        monkeypatch.setattr(search, "BRAVE_API_KEY", "")
        res, backend = await search.web_search("x", FakeClient([]))
        assert res is None and backend is None

    async def test_brave_when_searxng_unset(self, monkeypatch):
        monkeypatch.setattr(search, "SEARXNG_URL", "")
        monkeypatch.setattr(search, "BRAVE_API_KEY", "k")
        c = FakeClient([("api.search.brave.com", FakeResp(json_data=BRAVE_JSON))])
        res, backend = await search.web_search("x", c)
        assert backend == "brave" and res[0]["url"] == "https://c.example"

    async def test_brave_fallback_when_searxng_down(self, monkeypatch):
        monkeypatch.setattr(search, "SEARXNG_URL", "http://localhost:8888")
        monkeypatch.setattr(search, "BRAVE_API_KEY", "k")
        c = FakeClient([
            ("localhost:8888/search", FakeResp(status=500)),
            ("api.search.brave.com", FakeResp(json_data=BRAVE_JSON)),
        ])
        _res, backend = await search.web_search("x", c)
        assert backend == "brave"

    async def test_searxng_empty_then_brave(self, monkeypatch):
        monkeypatch.setattr(search, "SEARXNG_URL", "http://localhost:8888")
        monkeypatch.setattr(search, "BRAVE_API_KEY", "k")
        c = FakeClient([
            ("localhost:8888/search", FakeResp(json_data={"results": []})),
            ("api.search.brave.com", FakeResp(json_data=BRAVE_JSON)),
        ])
        _res, backend = await search.web_search("x", c)
        assert backend == "brave"

    async def test_searxng_empty_no_brave_returns_empty(self, monkeypatch):
        monkeypatch.setattr(search, "SEARXNG_URL", "http://localhost:8888")
        monkeypatch.setattr(search, "BRAVE_API_KEY", "")
        c = FakeClient([("localhost:8888/search", FakeResp(json_data={"results": []}))])
        res, backend = await search.web_search("x", c)
        assert res == [] and backend == "searxng"


class TestSearchEnabled:
    """`search_enabled` reports backend availability (can a query actually run?), not visibility."""

    def test_disabled_when_no_backend(self, monkeypatch):
        monkeypatch.delenv("HARVESTER_DISABLE_SEARCH", raising=False)
        monkeypatch.setattr(search, "SEARXNG_URL", "")
        monkeypatch.setattr(search, "BRAVE_API_KEY", "")
        assert search.search_enabled() is False

    def test_enabled_with_searxng(self, monkeypatch):
        monkeypatch.delenv("HARVESTER_DISABLE_SEARCH", raising=False)
        monkeypatch.setattr(search, "SEARXNG_URL", "http://127.0.0.1:8888")
        monkeypatch.setattr(search, "BRAVE_API_KEY", "")
        assert search.search_enabled() is True

    def test_enabled_with_brave_only(self, monkeypatch):
        monkeypatch.delenv("HARVESTER_DISABLE_SEARCH", raising=False)
        monkeypatch.setattr(search, "SEARXNG_URL", "")
        monkeypatch.setattr(search, "BRAVE_API_KEY", "k")
        assert search.search_enabled() is True

    def test_force_disabled_even_with_backend(self, monkeypatch):
        monkeypatch.setattr(search, "SEARXNG_URL", "http://127.0.0.1:8888")
        monkeypatch.setattr(search, "BRAVE_API_KEY", "k")
        monkeypatch.setenv("HARVESTER_DISABLE_SEARCH", "1")
        assert search.search_enabled() is False


class TestSearchAdvertised:
    """`search_advertised` governs tool VISIBILITY: always on unless force-disabled."""

    def test_advertised_without_backend(self, monkeypatch):
        monkeypatch.delenv("HARVESTER_DISABLE_SEARCH", raising=False)
        monkeypatch.setattr(search, "SEARXNG_URL", "")
        monkeypatch.setattr(search, "BRAVE_API_KEY", "")
        assert search.search_advertised() is True

    def test_not_advertised_when_force_disabled(self, monkeypatch):
        monkeypatch.setenv("HARVESTER_DISABLE_SEARCH", "1")
        assert search.search_advertised() is False
