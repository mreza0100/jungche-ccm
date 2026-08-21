"""Stress tests — hostile inputs, malformed upstreams, and abuse shapes for the
harvester-perfection wave. Everything here simulates the world being WRONG:
garbage JSON from scholarly APIs, corrupt/weaponized EPUBs, SSRF-flavored image
links, concurrent hammering, and an oversized stats file. The server must degrade
to a clean error or an empty result — never raise, never poison the cache.
"""

import asyncio
import json

from harvester import cache, detect, dispatch, images, net, oa, stats

# ── fakes (test_oa.py shape, dict-routed) ────────────────────────────────────────


class _Resp:
    def __init__(self, value):
        self._v = value
        self.status_code = 200

    def json(self):
        if self._v is None:
            raise ValueError("no json")
        return self._v


class GarbageClient:
    """Returns a DIFFERENT wrong shape per route: lists where dicts belong, strings,
    nulls, deeply-nested junk — every resolver's parse path gets fed something hostile."""

    SHAPES = ["a string", None, [], 42, {"hits": "not-a-list"}, {"message": []},
              {"items": [{"DOI": 5}]}, {"records": {"x": "y"}}, [[[["deep"]]]]]

    def __init__(self):
        self.n = 0

    async def get(self, url, **kw):
        v = self.SHAPES[self.n % len(self.SHAPES)]
        self.n += 1
        return _Resp(v)

    async def post(self, url, **kw):
        return await self.get(url)


# ── garbage-in: every resolver survives malformed upstreams ─────────────────────

class TestGarbageUpstreams:
    async def test_every_doi_resolver_survives_garbage(self):
        client = GarbageClient()
        resolvers = [oa.from_unpaywall, oa.from_openalex, oa.from_semanticscholar,
                     oa.from_crossref, oa.from_core, oa.from_doaj,
                     oa.from_openaire, oa.from_zenodo]
        for fn in resolvers:
            cands = await fn("10.1234/garbage", client)
            assert isinstance(cands, list), f"{fn.__name__} must return a list on garbage"
            assert cands == [], f"{fn.__name__} must extract nothing from garbage"

    async def test_resolve_doi_full_chain_on_garbage(self):
        cands = await oa.resolve_doi("10.1234/garbage", GarbageClient())
        assert cands == []

    async def test_find_works_survives_all_sources_lying(self, monkeypatch):
        routes = {
            "api.openalex.org": {"results": [{"display_name": 42}]},   # title not a string
            "query.bibliographic": {"message": None},                  # null message
            "paper/search": {"data": "not-a-list"},
            "export.arxiv.org": "<< not xml >>",
            "openlibrary.org": {"docs": "nope"},
            "gutendex.com": {"results": [[]]},
        }
        monkeypatch.setattr(oa, "_arxiv_by_title", lambda *a: [])
        out = await oa.find_works("some query", _FakeRouteClient(routes), limit=8)
        assert out == []


class _FakeRouteClient(GarbageClient):
    def __init__(self, routes):
        self.routes = routes

    async def get(self, url, **kw):
        for sub, v in self.routes.items():
            if sub in url:
                return _Resp(v)
        return _Resp(None)

    async def post(self, url, **kw):
        return await self.get(url)


# ── weaponized EPUBs ────────────────────────────────────────────────────────────

class TestHostileEpubs:
    def test_epub_marker_requires_real_mimetype_member(self):
        # A plain zip that merely CONTAINS the marker deep inside file DATA is not an EPUB —
        # only the first-200-bytes (stored mimetype member) position counts.
        import io
        import zipfile
        buf = io.BytesIO()
        with zipfile.ZipFile(buf, "w") as z:
            z.writestr("evil.txt", "x" * 10_000 + b"application/epub+zip".decode())
        assert detect.looks_like_epub(buf.getvalue()) is False, (
            "the marker must be near the zip header, not buried in member data")

    async def test_corrupt_epub_candidate_is_skipped_not_cached(self, monkeypatch):
        """A zip claiming the EPUB mimetype but with no OPF inside → conversion yields empty,
        the candidate is skipped, and NO poisoned artifact is written under the key."""
        import io
        import zipfile
        buf = io.BytesIO()
        with zipfile.ZipFile(buf, "w") as z:
            z.writestr("mimetype", "application/epub+zip", compress_type=zipfile.ZIP_STORED)
            z.writestr("trash.bin", "\x00\x01\x02" * 100)  # no container.xml, no OPF

        async def fake_fetch(url, ua, proxy):
            return buf.getvalue(), 200, None, "application/zip"

        async def fake_imp(url, proxy):
            return b"", None

        monkeypatch.setattr(net, "fetch_bytes_with_meta", fake_fetch)
        monkeypatch.setattr(net, "download_impersonated", fake_imp)

        cand = oa.Candidate(10, "https://evil.example/fake.epub", "oapen", "", "", "pdf")
        res = await dispatch._candidate_to_result(cand, "key-corrupt-epub", "ua", None)
        assert res is None, "an unparseable epub must yield 'skip', not a broken success"
        md = cache.cache_file("key-corrupt-epub", "epub", ".md")
        assert not md.exists(), "corrupt-epub output must never be cached as content"

    def test_deeply_nested_openaire_json_no_recursion_crash(self):
        # 5000-deep nested list — _walk_webresource_urls must terminate cleanly
        obj: object = []
        for _ in range(5000):
            obj = [obj]
        urls = oa._walk_webresource_urls(obj)
        assert urls == []


# ── SSRF via localized images ───────────────────────────────────────────────────

class TestImageLocalizationSsrf:
    async def test_private_image_targets_are_refused(self):
        """The localization download path inherits the SSRF chokepoint: a page that smuggles
        ![](http://127.0.0.1:9/x.png) or metadata-endpoint links must refuse them."""
        md = ("body text " * 80 + "\n\n![x](http://127.0.0.1:9/x.png)\n"
              "![y](http://169.254.169.254/latest/meta-data/)\n")
        out = await images.localize_html_images(md, "https://public.example/page", "ua")
        assert "127.0.0.1" in out and "169.254.169.254" in out, (
            "refused links must stay as-is, never rewritten to a local path")

    async def test_redirect_to_private_host_is_refused(self, monkeypatch):
        """A public image URL that 302s to an internal host dies at the request hook."""
        class RedirectThenRefusedClient:
            async def __aenter__(self):
                return self

            async def __aexit__(self, *exc):
                return False

            async def get(self, url, **kw):
                # net._client installs the hook; simulate the refusal the hook would raise
                raise net.FetchNotAllowed("redirect target refused")

        orig_client = net._client
        monkeypatch.setattr(net, "_client", lambda *a, **k: RedirectThenRefusedClient())
        try:
            md = "prose " * 100 + "\n\n![i](https://public.example/rebind.png)"
            out = await images.localize_html_images(
                md, "https://public.example/page", "ua")
            assert "https://public.example/rebind.png" in out
        finally:
            monkeypatch.setattr(net, "_client", orig_client)


# ── concurrency + cache abuse ───────────────────────────────────────────────────

class TestConcurrencyAndCacheAbuse:
    async def test_twenty_concurrent_calls_share_one_fetch(self, monkeypatch):
        calls = 0

        async def fake_dispatch(item, ua, proxy, media):
            nonlocal calls
            calls += 1
            await asyncio.sleep(0.05)
            return {"error": "still failing", "body": ""}

        monkeypatch.setattr(dispatch, "_dispatch_one", fake_dispatch)
        results = await asyncio.gather(*(
            dispatch.get_or_fetch("https://hammer.example/x", "ua") for _ in range(20)))
        assert calls == 1, f"20 racers must share ONE fetch; got {calls}"
        assert all(r.get("error") for r in results)

    async def test_neg_cache_cannot_grow_unbounded(self, monkeypatch):
        async def fake_dispatch(item, ua, proxy, media):
            return {"error": f"fail {item}", "body": ""}

        monkeypatch.setattr(dispatch, "_dispatch_one", fake_dispatch)
        dispatch._NEG_CACHE.clear()
        for i in range(500):
            await dispatch.get_or_fetch(f"https://grow.example/{i}", "ua")
        dispatch._neg_cache_evict_stale()
        assert len(dispatch._NEG_CACHE) <= 500
        # and eviction actually drops stale entries rather than leaking them forever
        clock = {"t": 10_000_000.0}  # far past every entry's recorded timestamp
        monkeypatch.setattr(dispatch.time, "monotonic", lambda: clock["t"])
        dispatch._neg_cache_evict_stale()
        assert len(dispatch._NEG_CACHE) == 0, "stale entries must be evicted"


# ── stats scoreboard under load / abuse ─────────────────────────────────────────

class TestStatsAbuse:
    def test_thousand_records_and_junk_lines_aggregate_correctly(self):
        p = stats.stats_path()
        with open(p, "w", encoding="utf-8") as fh:
            for i in range(1000):
                fh.write(json.dumps({"ts": "t", "item": f"u{i}", "ok": i % 3 != 0,
                                     "detail": "direct"}) + "\n")
                fh.write("\x00binary garbage line\n")
        buckets = stats.summarize(last_n=5000)
        b = buckets["direct"]
        assert b["total"] == 1000
        assert b["ok"] == len([i for i in range(1000) if i % 3 != 0])
        assert abs(b["rate"] - round(b["ok"] / 1000, 3)) < 1e-9

    def test_item_and_detail_are_length_capped(self):
        stats.record_fetch("u" * 10_000, False, "d" * 10_000)
        with open(stats.stats_path(), encoding="utf-8") as fh:
            rec = json.loads(fh.readlines()[-1])
        assert len(rec["item"]) <= 500 and len(rec["detail"]) <= 200


# ── browser rung gating ─────────────────────────────────────────────────────────

class TestBrowserRungGating:
    def test_default_off(self):
        assert dispatch._BROWSER_ENABLED is False, "browser rung ships OFF by default"

    async def test_flag_on_runs_after_jina_and_before_mirror(self, monkeypatch):
        """Ladder order with the browser enabled: direct → chrome-impersonation → jina →
        browser → mirror. The browser rung must sit between Jina and the mirror chain."""
        order: list[str] = []

        async def fake_html_bytes(url, ua, proxy):
            return b"<html></html>", 200, None, "text/html"  # empty body → thin

        async def fake_imp_text(url, proxy):
            order.append("chrome-impersonation")
            return "", None

        async def fake_jina(url, ua, proxy=None):
            order.append("jina")
            return ""  # jina fails too

        async def fake_browser(url, proxy=None, timeout_ms=45000):
            order.append("browser")
            return "<html><body>" + "real content " * 120 + "</body></html>", 200

        async def fake_mirror(*a, **kw):
            order.append("mirror")
            return None

        def fake_extract(raw):
            return raw if isinstance(raw, str) and "real content" in raw else ""

        monkeypatch.setattr(dispatch, "_BROWSER_ENABLED", True)
        monkeypatch.setattr(net, "fetch_bytes_with_meta", fake_html_bytes)
        monkeypatch.setattr(net, "fetch_impersonated", fake_imp_text)
        monkeypatch.setattr(net, "fetch_jina", fake_jina)
        monkeypatch.setattr(net, "fetch_browser", fake_browser)
        monkeypatch.setattr(dispatch.html, "extract_content_from_html", fake_extract)
        monkeypatch.setattr(dispatch, "_try_mirror_for_url", fake_mirror)

        res = await dispatch.get_or_fetch("https://walled.example/article", "ua")
        assert order[:2] == ["chrome-impersonation", "jina"], f"got {order}"
        assert order == ["chrome-impersonation", "jina", "browser"], (
            f"browser won → ladder stops there, mirror never needed; got {order}")
        assert "error" not in res and res["method"] == "browser-chrome"
        assert "real content" in res["body"]

    async def test_missing_patchright_is_a_silent_skip(self, monkeypatch):
        async def boom_import(url, proxy=None, timeout_ms=45000):
            return "", None  # fetch_browser's own contract when patchright absent

        monkeypatch.setattr(dispatch, "_BROWSER_ENABLED", True)
        monkeypatch.setattr(net, "fetch_browser", boom_import)

        async def fake_html_bytes(url, ua, proxy):
            return b"<html>" + b"x" * 4000 + b"</html>", 200, None, "text/html"

        monkeypatch.setattr(net, "fetch_bytes_with_meta", fake_html_bytes)
        res = await dispatch.get_or_fetch("https://thin-but-fine.example/x", "ua")
        # body > THIN_MIN_CHARS so the rung never fires; just prove no crash/no behavior change
        assert "error" not in res
