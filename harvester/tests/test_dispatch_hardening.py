"""Tests for the dispatch hardening fixes:

* P1 — negative-result cache + in-flight dedup in get_or_fetch
* P2 — publisher-403 PDF → DOI → open-access pivot in _doc_result
* P2b — detected bot/Cloudflare challenge is never cached/returned as content
* P3 — bare PMID / PMCID routing
* P4a — PubMed search-results URL → clean redirect
* P4b — favicon / icon skip in image localisation
"""

import asyncio

from harvester import convert, dispatch, images, mirror, net, oa


async def _noop_localise(md, *a, **kw):
    return md


# ── P1: negative-result cache + in-flight dedup ─────────────────────────────────

class TestNegativeCacheAndDedup:
    async def test_failure_is_cached_second_call_skips_fetch(self, monkeypatch):
        calls = 0

        async def fake_dispatch(item, ua, proxy, media):
            nonlocal calls
            calls += 1
            return {"error": f"boom for {item}", "body": ""}

        monkeypatch.setattr(dispatch, "_dispatch_one", fake_dispatch)

        r1 = await dispatch.get_or_fetch("https://neg.example/fail", "ua")
        r2 = await dispatch.get_or_fetch("https://neg.example/fail", "ua")
        assert calls == 1, "second failing call must hit the negative cache, not re-fetch"
        assert "boom" in r1["error"]
        assert "recently failed; cached" in r2["error"]

    async def test_success_is_not_cached(self, monkeypatch):
        calls = 0

        async def fake_dispatch(item, ua, proxy, media):
            nonlocal calls
            calls += 1
            return {"body": "ok content", "method": "x", "cache_status": "miss",
                    "md_path": "/tmp/x.md", "bytes": 10, "content_chars": 10,
                    "http_status": 200, "error_kind": None, "challenge": False}

        monkeypatch.setattr(dispatch, "_dispatch_one", fake_dispatch)

        await dispatch.get_or_fetch("https://ok.example/page", "ua")
        await dispatch.get_or_fetch("https://ok.example/page", "ua")
        assert calls == 2, "successes must never be negative-cached"

    async def test_concurrent_calls_dedup_to_one_fetch(self, monkeypatch):
        calls = 0

        async def fake_dispatch(item, ua, proxy, media):
            nonlocal calls
            calls += 1
            await asyncio.sleep(0.02)  # keep the fetch in flight while the twin arrives
            return {"error": "slow fail", "body": ""}

        monkeypatch.setattr(dispatch, "_dispatch_one", fake_dispatch)

        results = await asyncio.gather(
            dispatch.get_or_fetch("https://race.example/x", "ua"),
            dispatch.get_or_fetch("https://race.example/x", "ua"),
        )
        assert calls == 1, "concurrent calls for the same item must share one fetch"
        assert all(r["error"] == "slow fail" for r in results)

    async def test_allow_and_deny_media_do_not_alias(self, monkeypatch):
        seen = []

        async def fake_dispatch(item, ua, proxy, media):
            seen.append(media)
            return {"error": f"err {media}", "body": ""}

        monkeypatch.setattr(dispatch, "_dispatch_one", fake_dispatch)

        await dispatch.get_or_fetch("https://m.example/x", "ua", None, "allow")
        await dispatch.get_or_fetch("https://m.example/x", "ua", None, "deny")
        assert seen == ["allow", "deny"], "different media must not share a cache/in-flight key"

    async def test_expired_entry_refetches(self, monkeypatch):
        calls = 0

        async def fake_dispatch(item, ua, proxy, media):
            nonlocal calls
            calls += 1
            return {"error": "transient", "body": ""}

        clock = {"t": 1000.0}
        monkeypatch.setattr(dispatch.time, "monotonic", lambda: clock["t"])
        monkeypatch.setenv("HARVESTER_NEG_TTL", "10")
        monkeypatch.setattr(dispatch, "_dispatch_one", fake_dispatch)

        await dispatch.get_or_fetch("https://ttl.example/x", "ua")
        clock["t"] += 11  # advance past the TTL
        await dispatch.get_or_fetch("https://ttl.example/x", "ua")
        assert calls == 2, "an expired negative-cache entry must re-fetch"

    def test_invalid_neg_ttl_falls_back_to_default(self, monkeypatch):
        monkeypatch.setenv("HARVESTER_NEG_TTL", "not-a-number")
        assert dispatch._neg_ttl() == dispatch.DEFAULT_NEG_TTL

    async def test_neg_cache_annotation_includes_retry_after_seconds(self, monkeypatch):
        """Slice 1 deliverable 7: the neg-cache annotation must tell the model how long until
        retrying is worth it, computed from the TTL remainder — not just "cached" with no ETA."""
        async def fake_dispatch(item, ua, proxy, media):
            return {"error": "boom", "body": ""}

        clock = {"t": 1000.0}
        monkeypatch.setattr(dispatch.time, "monotonic", lambda: clock["t"])
        monkeypatch.setenv("HARVESTER_NEG_TTL", "20")
        monkeypatch.setattr(dispatch, "_dispatch_one", fake_dispatch)

        await dispatch.get_or_fetch("https://retry.example/x", "ua")
        clock["t"] += 5  # 15s left in the 20s window
        r2 = await dispatch.get_or_fetch("https://retry.example/x", "ua")

        assert "retry after 15s" in r2["error"]


# ── P2: publisher-403 PDF → DOI → OA pivot in _doc_result ───────────────────────

class TestDocResultMirrorPivot:
    _FAKE_SUCCESS = {
        "body": "# Open Access\n\n" + "word " * 200, "method": "mirror:europepmc-pdf",
        "cache_status": "miss", "md_path": "/tmp/x.md", "bytes": 1000,
        "content_chars": 1000, "http_status": None, "error_kind": None, "challenge": False,
    }

    async def _no_bytes(self, monkeypatch):
        async def fake_download_bytes(url, ua, proxy=None):
            return b"", 403, None

        async def fake_download_impersonated(url, proxy=None):
            return b"", 403

        monkeypatch.setattr(net, "download_bytes", fake_download_bytes)
        monkeypatch.setattr(net, "download_impersonated", fake_download_impersonated)

    async def test_403_pdf_pivots_to_mirror(self, monkeypatch):
        await self._no_bytes(monkeypatch)
        called = []

        async def fake_mirror(src, key, ua, proxy, trace=None, page_html=None):
            called.append(src)
            return dict(self._FAKE_SUCCESS)

        monkeypatch.setattr(dispatch, "_try_mirror_for_url", fake_mirror)

        url = "https://www.tandfonline.com/doi/pdf/10.1080/12345.2023.1"
        result = await dispatch._doc_result(url, url, "pdf", False, "ua", None)
        assert called, "the mirror pivot must be attempted on a 403 publisher PDF"
        assert result["method"] == "mirror:europepmc-pdf"
        assert "error" not in result

    async def test_403_pdf_no_mirror_returns_net_error(self, monkeypatch):
        await self._no_bytes(monkeypatch)

        async def fake_mirror(src, key, ua, proxy, trace=None, page_html=None):
            return None

        monkeypatch.setattr(dispatch, "_try_mirror_for_url", fake_mirror)

        url = "https://www.sciencedirect.com/science/article/pii/S0001/pdf"
        result = await dispatch._doc_result(url, url, "pdf", False, "ua", None)
        assert "error" in result
        assert "403" in result["error"] or "forbidden" in result["error"].lower()


# ── P3: bare PMID / PMCID routing ───────────────────────────────────────────────

class TestPmidPmcidRouting:
    _PDF_BYTES = b"%PDF-1.4 fake content\n%%EOF"

    def _mock_pmc_pdf_chain(self, monkeypatch):
        async def fake_europepmc_pdf(pmcid, client):
            return self._PDF_BYTES

        def fake_pdf_to_md(path):
            return "# PMC Paper\n\n" + "word " * 200

        monkeypatch.setattr(mirror, "europepmc_pdf", fake_europepmc_pdf)
        monkeypatch.setattr(convert, "pdf_to_md", fake_pdf_to_md)

    async def test_bare_pmid_routes_to_resolver(self, monkeypatch):
        async def fake_pmid_to_pmcid(pmid, client):
            return "PMC11632837"

        monkeypatch.setattr(mirror, "pmid_to_pmcid", fake_pmid_to_pmcid)
        self._mock_pmc_pdf_chain(monkeypatch)

        result = await dispatch.get_or_fetch("30220343", "ua")
        assert "error" not in result, result.get("error")
        assert result["method"] == "mirror:europepmc-pdf"
        assert "local file not found" not in str(result)

    async def test_bare_pmid_no_pmcid_returns_clean_error(self, monkeypatch):
        async def fake_pmid_to_pmcid(pmid, client):
            return None

        monkeypatch.setattr(mirror, "pmid_to_pmcid", fake_pmid_to_pmcid)

        result = await dispatch.get_or_fetch("30220343", "ua")
        assert "error" in result
        assert "PubMed ID" in result["error"]
        assert "findWorks" in result["error"]
        assert "local file not found" not in result["error"]

    async def test_pmcid_routes_as_pmcid(self, monkeypatch):
        self._mock_pmc_pdf_chain(monkeypatch)
        result = await dispatch.get_or_fetch("PMC11632837", "ua")
        assert "error" not in result, result.get("error")
        assert result["method"] == "mirror:europepmc-pdf"

    async def test_pmcid_not_found_returns_clean_error(self, monkeypatch):
        async def fake_europepmc_pdf(pmcid, client):
            return b""

        async def fake_fetch_raw(url, ua, proxy=None):
            return ""

        monkeypatch.setattr(mirror, "europepmc_pdf", fake_europepmc_pdf)
        monkeypatch.setattr(net, "fetch_raw", fake_fetch_raw)

        result = await dispatch.get_or_fetch("PMC99999999", "ua")
        assert "error" in result
        assert "PMCID" in result["error"]
        assert "findWorks" in result["error"]

    async def test_pmcid_lowercase_is_recognised(self, monkeypatch):
        seen = []

        async def fake_pmcid_to_result(pmcid, key, ua, proxy):
            seen.append(pmcid)
            return {"body": "ok", "method": "mirror:pmc-html", "cache_status": "miss",
                    "md_path": "/tmp/x.md", "bytes": 2, "content_chars": 600,
                    "http_status": None, "error_kind": None, "challenge": False}

        monkeypatch.setattr(dispatch, "_pmcid_to_result", fake_pmcid_to_result)
        result = await dispatch.get_or_fetch("pmc11632837", "ua")
        assert "error" not in result
        assert seen == ["PMC11632837"], "PMCID must be upper-cased before resolving"


# ── P4a: PubMed search URL → redirect ───────────────────────────────────────────

class TestPubmedSearchUrl:
    def test_is_pubmed_search_url_positive(self):
        assert dispatch._is_pubmed_search_url("https://pubmed.ncbi.nlm.nih.gov/?term=cancer")
        assert dispatch._is_pubmed_search_url("https://pubmed.ncbi.nlm.nih.gov/search/?term=x")

    def test_is_pubmed_search_url_negative(self):
        # A specific-article URL must NOT be treated as a search URL.
        assert not dispatch._is_pubmed_search_url("https://pubmed.ncbi.nlm.nih.gov/30220343/")
        assert not dispatch._is_pubmed_search_url("https://example.com/?term=x")

    async def test_search_url_returns_find_hint(self, monkeypatch):
        async def boom(*a, **kw):
            raise AssertionError("must not fetch a PubMed search URL")

        # If routing is correct, the network is never touched.
        monkeypatch.setattr(net, "fetch_bytes_with_meta", boom)
        result = await dispatch.get_or_fetch(
            "https://pubmed.ncbi.nlm.nih.gov/?term=glp1+weight", "ua")
        assert "error" in result
        assert "findWorks" in result["error"] and "search" in result["error"].lower()


# ── P4b: favicon / icon skip ────────────────────────────────────────────────────

class TestIconSkip:
    def test_is_icon_asset(self):
        assert images._is_icon_asset("https://x.com/favicon.ico")
        assert images._is_icon_asset("https://x.com/static/favicon-32x32.png")
        assert images._is_icon_asset("https://x.com/assets/sprite.svg")
        assert images._is_icon_asset("/img/icon.ico?v=2")
        assert not images._is_icon_asset("https://x.com/figures/fig1.png")
        assert not images._is_icon_asset("https://x.com/photo.jpg")

    async def test_favicon_is_not_downloaded(self, monkeypatch):
        requested = []
        png_data = b"\x89PNG\r\n\x1a\n" + b"\x00" * 100

        class FakeResponse:
            status_code = 200
            content = png_data
            headers = {"content-type": "image/png"}

        class FakeClient:
            async def __aenter__(self):
                return self

            async def __aexit__(self, *_):
                pass

            async def get(self, url, **kw):
                requested.append(url)
                return FakeResponse()

        import httpx
        monkeypatch.setattr(httpx, "AsyncClient", lambda **kw: FakeClient())

        md = ("![icon](https://example.com/favicon.ico)\n\n"
              "![fig](https://example.com/fig.png)\n")
        result = await images.localize_html_images(md, "https://example.com/p", "ua")
        assert not any("favicon" in u for u in requested), "favicon must be skipped pre-download"
        assert any("fig.png" in u for u in requested), "the real figure must still download"
        assert "https://example.com/favicon.ico" in result  # left untouched


# ── P2b: unresolved bot/Cloudflare challenge is poison, never cached/returned ────

class TestUnresolvedChallenge:
    # The live sciencedirect captcha variant (~620 bytes) that was wrongly cached as success.
    _CAPTCHA_HTML = (
        "<html><head><title>Just a moment...</title></head><body>"
        "<h1>Are you a robot?</h1>"
        "<p>Please confirm you are a human by completing the captcha challenge below. "
        "Enable JavaScript and cookies to continue.</p>"
        "<p>Reference number: 1234567890</p><p>IP Address: 1.2.3.4</p>"
        "</body></html>"
    ).encode()
    _RICH_HTML = ("<html><body><article><p>" + "word " * 300 + "</p></article></body></html>").encode()

    async def test_challenge_survives_ladder_returns_error_and_no_cache(self, monkeypatch, isolated_cache):
        async def fake_bytes(url, ua, proxy=None):
            return self._CAPTCHA_HTML, 200, None, "text/html"

        async def fake_impersonated(url, proxy=None):
            return self._CAPTCHA_HTML.decode(), 200  # curl_cffi also hits the wall

        async def fake_jina(url, ua, proxy=None):
            return ""

        async def fake_defuddle(url, ua, proxy=None):
            return ""

        async def fake_mirror(src, key, ua, proxy, trace=None, page_html=None):
            return None  # no open-access copy

        monkeypatch.setattr(net, "fetch_bytes_with_meta", fake_bytes)
        monkeypatch.setattr(net, "fetch_impersonated", fake_impersonated)
        monkeypatch.setattr(net, "fetch_jina", fake_jina)
        monkeypatch.setattr(net, "fetch_defuddle", fake_defuddle)
        monkeypatch.setattr(dispatch, "_try_mirror_for_url", fake_mirror)
        monkeypatch.setattr(images, "localize_html_images", _noop_localise)

        url = "https://www.sciencedirect.com/science/article/pii/S0306987718301051"
        result = await dispatch._html_result(url, url, False, "ua", None)

        assert "error" in result, "a surviving challenge must be an error, not a success"
        assert result["body"] == ""
        assert "challenge" in result["error"].lower()
        # The poison body must NOT have been written to the cache.
        assert not list(isolated_cache.rglob("*.md")), "challenge body must never be cached"

    async def test_rich_content_after_challenge_is_cached_ok(self, monkeypatch, isolated_cache):
        async def fake_bytes(url, ua, proxy=None):
            return self._RICH_HTML, 200, None, "text/html"

        monkeypatch.setattr(net, "fetch_bytes_with_meta", fake_bytes)
        monkeypatch.setattr(images, "localize_html_images", _noop_localise)

        url = "https://example.com/real-article"
        result = await dispatch._html_result(url, url, False, "ua", None)

        assert "error" not in result, result.get("error")
        assert result["method"] == "local-trafilatura"
        assert list(isolated_cache.rglob("*.md")), "genuine content must be cached"


# ── plain-text passthrough: .txt / text/plain must NOT be trafilatura-stripped ──

class TestPlainTextPassthrough:
    """A Gutenberg-style .txt (text/plain) is kept VERBATIM, cached, and a repeat is a cache hit.

    Regression: trafilatura on raw prose returns "", which was cached as an empty 154-byte stub —
    making `size_only` report 0/0 and every repeat re-download.
    """

    _BIG_TEXT = ("CHAPTER I\n\n" + ("Napoleon and the war and peace of nations. " * 4000)).encode()

    async def test_plain_text_kept_verbatim_and_cached(self, monkeypatch, isolated_cache):
        calls = 0

        async def fake_bytes(url, ua, proxy=None):
            nonlocal calls
            calls += 1
            return self._BIG_TEXT, 200, None, "text/plain; charset=utf-8"

        monkeypatch.setattr(net, "fetch_bytes_with_meta", fake_bytes)

        url = "https://www.gutenberg.org/files/2600/2600-0.txt"
        result = await dispatch._html_result(url, url, False, "ua", None)

        assert "error" not in result, result.get("error")
        assert result["method"] == "plain-text"
        # Verbatim: the full text survives (no trafilatura stripping).
        assert "Napoleon" in result["body"]
        assert len(result["body"]) >= len(self._BIG_TEXT) - 16  # ~exact (decode only)
        # Cache file is the real document, not a header-only stub.
        cached = list(isolated_cache.rglob("*.md"))
        assert cached and cached[0].stat().st_size > 100_000
        assert "token_count:" in cached[0].read_text()

        # A repeat is a cache hit — no second download.
        again = await dispatch._html_result(url, url, False, "ua", None)
        assert again["cache_status"] == "hit"
        assert calls == 1, "repeat plain-text fetch must reuse the cache, not re-download"


class TestIsPlainText:
    """detect.is_plain_text routes raw text away from trafilatura, real HTML toward it."""

    def test_text_plain_content_type_is_verbatim(self):
        from harvester import detect
        assert detect.is_plain_text("https://x/file", "text/plain; charset=utf-8", "hello") is True

    def test_txt_extension_is_verbatim(self):
        from harvester import detect
        assert detect.is_plain_text("https://x/book.txt", "", "plain prose here") is True

    def test_html_content_type_is_extracted(self):
        from harvester import detect
        assert detect.is_plain_text("https://x/page", "text/html", "<html><body>hi</body></html>") is False

    def test_html_extension_is_extracted(self):
        from harvester import detect
        assert detect.is_plain_text("https://x/page.html", "", "<div>hi</div>") is False

    def test_mis_served_html_as_text_plain_is_extracted(self):
        from harvester import detect
        # text/plain but the body is obviously HTML → prefer extraction.
        assert detect.is_plain_text("https://x/p", "text/plain", "<html><body>x</body></html>") is False

    def test_no_signal_no_structure_is_verbatim(self):
        from harvester import detect
        assert detect.is_plain_text("https://x/unknown", "", "just some prose, no tags") is True


# ═══════════════════════════════════════════════════════════════════════════════
# Slice 2 — rescue-graph engine: outcome classifier, Attempt trace, R1–R7
# ═══════════════════════════════════════════════════════════════════════════════

# ── A: outcome classifier ────────────────────────────────────────────────────

class TestOutcomeClassifier:
    """classify_outcome is the ONE place that decides what a rung's result was — replacing the
    ad-hoc 'thin or challenge' checks that used to be re-derived per branch."""

    def test_dead_net_on_connection_failure(self):
        assert dispatch.classify_outcome(body_len=0, status=None, error_kind="timeout") == dispatch.Outcome.DEAD_NET

    def test_dead_net_on_real_empty_4xx(self):
        # A real empty 4xx (no bytes at all) classifies the same way it did pre-R1 — R1 only
        # changes what happens when a 4xx body actually carries content.
        assert dispatch.classify_outcome(body_len=0, status=404) == dispatch.Outcome.DEAD_NET

    def test_http_4xx_with_body(self):
        outcome = dispatch.classify_outcome(body_len=500, content_chars=600, status=403)
        assert outcome == dispatch.Outcome.HTTP_4XX_WITH_BODY

    def test_challenge_takes_priority_over_4xx_status(self):
        outcome = dispatch.classify_outcome(body_len=500, content_chars=600, status=403, challenge=True)
        assert outcome == dispatch.Outcome.CHALLENGE

    def test_thin_when_content_short(self):
        assert dispatch.classify_outcome(body_len=50, content_chars=50, status=200) == dispatch.Outcome.THIN

    def test_ok_when_rich_200(self):
        assert dispatch.classify_outcome(body_len=2000, content_chars=2000, status=200) == dispatch.Outcome.OK

    def test_explicit_flags_win_outright(self):
        assert dispatch.classify_outcome(empty_convert=True, status=200, content_chars=2000) == \
            dispatch.Outcome.EMPTY_CONVERT
        assert dispatch.classify_outcome(wrong_kind=True, status=200, content_chars=2000) == \
            dispatch.Outcome.WRONG_KIND


# ── B: Attempt trace rendering helpers ───────────────────────────────────────

class TestRungsHelpers:
    """_rungs_phrase / _rungs_summary / _with_rungs — the trace renderers, unit-tested directly
    so the aggregation rule doesn't depend on chasing a specific multi-branch dispatch flow."""

    def test_rungs_phrase_lists_non_oa_rungs_plainly(self):
        trace = [
            dispatch.Attempt("direct", "u", dispatch.Outcome.CHALLENGE),
            dispatch.Attempt("chrome-impersonation", "u", dispatch.Outcome.THIN),
            dispatch.Attempt("jina", "u", dispatch.Outcome.OK),
        ]
        assert dispatch._rungs_phrase(trace) == "direct, chrome-impersonation, jina"

    def test_rungs_phrase_groups_consecutive_oa_candidates(self):
        trace = [
            dispatch.Attempt("direct", "u", dispatch.Outcome.THIN),
            dispatch.Attempt("oa:unpaywall", "c1", dispatch.Outcome.DEAD_NET),
            dispatch.Attempt("oa:openalex", "c2", dispatch.Outcome.THIN),
            dispatch.Attempt("oa:core", "c3", dispatch.Outcome.DEAD_NET),
            dispatch.Attempt("wayback", "w1", dispatch.Outcome.NOT_FOUND_PAGE),
        ]
        assert dispatch._rungs_phrase(trace) == "direct, oa-mirror(3 sources), wayback"

    def test_rungs_summary_is_raw_comma_list(self):
        trace = [
            dispatch.Attempt("direct", "u", dispatch.Outcome.THIN),
            dispatch.Attempt("oa:unpaywall", "c1", dispatch.Outcome.OK),
        ]
        assert dispatch._rungs_summary(trace) == "direct, oa:unpaywall"

    def test_with_rungs_quiet_for_a_single_rung(self):
        trace = [dispatch.Attempt("direct", "u", dispatch.Outcome.DEAD_NET)]
        assert dispatch._with_rungs("boom", trace) == "boom"

    def test_with_rungs_embeds_phrase_for_multi_rung(self):
        trace = [
            dispatch.Attempt("direct", "u", dispatch.Outcome.THIN),
            dispatch.Attempt("wayback", "w", dispatch.Outcome.DEAD_NET),
        ]
        msg = dispatch._with_rungs("boom", trace)
        assert "Rungs tried: direct, wayback" in msg
        assert "re-fetching will not help" in msg


class TestRungsTraceInFrontmatterAndHeader:
    """On success, the trace is written into cache frontmatter and rendered by describe.py —
    ONLY when more than one rung ran, keeping the common single-rung header quiet."""

    async def test_single_rung_success_has_no_rungs_field(self, monkeypatch, isolated_cache):
        rich_html = ("<html><body><article><p>" + "word " * 200 + "</p></article></body></html>").encode()

        async def fake_bytes(url, ua, proxy=None):
            return rich_html, 200, None, "text/html"

        monkeypatch.setattr(net, "fetch_bytes_with_meta", fake_bytes)

        url = "https://example.com/one-shot"
        result = await dispatch._html_result(url, url, False, "ua", None)
        assert "error" not in result
        assert "rungs:" not in result["md_path"].read_text()

    async def test_multi_rung_success_writes_and_renders_rungs(self, monkeypatch, isolated_cache):
        thin_html = b"<html><body><p>tiny</p></body></html>"
        rich_html = ("<html><body><article><p>" + "word " * 200 + "</p></article></body></html>").encode()

        async def fake_bytes(url, ua, proxy=None):
            return thin_html, 200, None, "text/html"

        async def fake_impersonated(url, proxy=None):
            return rich_html.decode(), 200

        monkeypatch.setattr(net, "fetch_bytes_with_meta", fake_bytes)
        monkeypatch.setattr(net, "fetch_impersonated", fake_impersonated)

        url = "https://example.com/needs-cffi"
        result = await dispatch._html_result(url, url, False, "ua", None)
        assert "error" not in result
        assert "rungs: direct, chrome-impersonation" in result["md_path"].read_text()

        from harvester.describe import describe_fetch_result
        out = describe_fetch_result(url, result).text
        assert "rungs: direct, chrome-impersonation" in out


class TestDeadUrlErrorListsRungs:
    """'Done when' criterion: a dead URL's terminal error must list the rungs it walked."""

    _CAPTCHA_HTML = (
        "<html><head><title>Just a moment...</title></head><body>"
        "<h1>Are you a robot?</h1><p>Please confirm you are a human by completing the captcha "
        "challenge below. Enable JavaScript and cookies to continue.</p></body></html>"
    ).encode()

    async def test_challenge_error_names_every_rung_tried(self, monkeypatch, isolated_cache):
        async def fake_bytes(url, ua, proxy=None):
            return self._CAPTCHA_HTML, 200, None, "text/html"

        async def fake_impersonated(url, proxy=None):
            return self._CAPTCHA_HTML.decode(), 200

        async def fake_jina(url, ua, proxy=None):
            return ""

        async def fake_defuddle(url, ua, proxy=None):
            return ""

        async def fake_mirror(src, key, ua, proxy, trace=None, page_html=None):
            return None

        monkeypatch.setattr(net, "fetch_bytes_with_meta", fake_bytes)
        monkeypatch.setattr(net, "fetch_impersonated", fake_impersonated)
        monkeypatch.setattr(net, "fetch_jina", fake_jina)
        monkeypatch.setattr(net, "fetch_defuddle", fake_defuddle)
        monkeypatch.setattr(dispatch, "_try_mirror_for_url", fake_mirror)

        url = "https://www.dead-walled.example/paper"
        result = await dispatch._html_result(url, url, False, "ua", None)

        assert "error" in result
        assert "Rungs tried: direct, chrome-impersonation, jina" in result["error"]


class TestFinalizeWithRungsBackfill:
    """_candidate_to_result's PDF/image success path delegates to _handle_binary_doc, which has
    no visibility into the accumulated trace — _finalize_with_rungs patches it in afterward."""

    async def test_pdf_candidate_success_backfills_rungs_after_earlier_failed_candidate(
        self, monkeypatch, isolated_cache
    ):
        calls = {"n": 0}

        async def fake_fetch_bytes_with_meta(url, ua, proxy=None):
            calls["n"] += 1
            if calls["n"] == 1:
                return b"", 404, None, ""  # first candidate: dead
            return b"%PDF-1.4 minimal", 200, None, "application/pdf"

        async def fake_download_impersonated(url, proxy=None):
            return b"", None

        def fake_convert(path, kind):
            return "# Paper\n\n" + "word " * 200

        monkeypatch.setattr(net, "fetch_bytes_with_meta", fake_fetch_bytes_with_meta)
        monkeypatch.setattr(net, "download_impersonated", fake_download_impersonated)
        monkeypatch.setattr(convert, "convert_local_file", fake_convert)

        cands = [
            oa.Candidate(10, "https://dead.example/a.pdf", "unpaywall", kind_hint="pdf"),
            oa.Candidate(20, "https://good.example/b.pdf", "core", kind_hint="pdf"),
        ]
        trace: list = []
        result = await dispatch._resolve_and_fetch(cands, "some-key", "test", "ua", None, trace)
        assert result and "error" not in result
        assert "rungs: oa:unpaywall, oa:core" in result["md_path"].read_text()


# ── R1: net layer keeps ≥400 response bodies ─────────────────────────────────

class TestR1KeepsBodyOnHttpError:
    """net._stream_capped no longer discards the response body on a ≥400 status — a soft-404/
    403-with-content must reach extraction, not come back as empty bytes."""

    async def test_403_with_body_is_kept(self, monkeypatch):
        body = b"<html><body>Access Denied but here is some real content</body></html>"

        class FakeResponse:
            status_code = 403
            headers = {"content-type": "text/html"}

            async def aiter_bytes(self):
                yield body

        class FakeStreamCtx:
            async def __aenter__(self):
                return FakeResponse()

            async def __aexit__(self, *a):
                return False

        class FakeClient:
            def stream(self, method, url, **kw):
                return FakeStreamCtx()

            async def __aenter__(self):
                return self

            async def __aexit__(self, *a):
                return False

        monkeypatch.setattr(net, "_client", lambda proxy_url=None, **kw: FakeClient())

        data, status, error_kind, ct = await net.fetch_bytes_with_meta("https://example.com/x", "ua")
        assert status == 403
        assert data == body
        assert error_kind is None

    async def test_real_empty_4xx_still_yields_no_bytes(self, monkeypatch):
        class FakeResponse:
            status_code = 404
            headers = {"content-type": "text/html"}

            async def aiter_bytes(self):
                if False:  # pragma: no cover - makes this an async generator with 0 items
                    yield b""

        class FakeStreamCtx:
            async def __aenter__(self):
                return FakeResponse()

            async def __aexit__(self, *a):
                return False

        class FakeClient:
            def stream(self, method, url, **kw):
                return FakeStreamCtx()

            async def __aenter__(self):
                return self

            async def __aexit__(self, *a):
                return False

        monkeypatch.setattr(net, "_client", lambda proxy_url=None, **kw: FakeClient())

        data, status, error_kind, ct = await net.fetch_bytes_with_meta("https://example.com/missing", "ua")
        assert status == 404
        assert data == b""


# ── R1 ripple: rescue/binary paths must treat a ≥400 body as that rung's failure ──
# Invariant: only the primary-URL HTML ladder (_html_result) may treat a ≥400 body as potential
# content; everywhere else a kept 4xx body is that rung's failure, never cached/published.

class TestR1RippleCandidateRejects4xxBody:
    """An OA candidate is a RESCUE URL — a rich 404/410 error page must never be cached as the
    identifier's content; it means 'try the next candidate'."""

    # >THIN_MIN_CHARS of plain prose (no HTML tags), so pre-fix it would pass the text branch.
    _RICH_404 = ("This work has been withdrawn from the repository. " * 30).encode()

    async def test_rich_404_candidate_rejected_not_cached(self, monkeypatch, isolated_cache):
        async def fake_fetch_bytes_with_meta(url, ua, proxy=None):
            return self._RICH_404, 404, None, "text/plain"

        async def fake_download_impersonated(url, proxy=None):
            return b"", None

        monkeypatch.setattr(net, "fetch_bytes_with_meta", fake_fetch_bytes_with_meta)
        monkeypatch.setattr(net, "download_impersonated", fake_download_impersonated)

        cand = oa.Candidate(0, "https://gone.example/paper", "unpaywall", kind_hint="html")
        trace: list = []
        result = await dispatch._candidate_to_result(cand, "10.1234/x", "ua", None, trace)

        assert result is None, "a 4xx body candidate must be rejected, not published"
        assert not list(isolated_cache.rglob("*.md")), "a 4xx body must never be cached as content"
        assert dispatch.Outcome.HTTP_4XX_WITH_BODY in [a.outcome for a in trace]

    async def test_4xx_body_still_tries_impersonation_rescue(self, monkeypatch, isolated_cache):
        """Pre-R1, a 403 yielded no bytes and fell to the curl_cffi retry — that rescue must
        survive R1: the 403 body is discarded and impersonation may still fetch the real PDF."""
        called = []

        async def fake_fetch_bytes_with_meta(url, ua, proxy=None):
            return b"<html><body>Access denied</body></html>", 403, None, "text/html"

        async def fake_download_impersonated(url, proxy=None):
            called.append(url)
            return b"%PDF-1.4 minimal", 200

        def fake_convert(path, kind):
            return "# Paper\n\n" + "word " * 200

        monkeypatch.setattr(net, "fetch_bytes_with_meta", fake_fetch_bytes_with_meta)
        monkeypatch.setattr(net, "download_impersonated", fake_download_impersonated)
        monkeypatch.setattr(convert, "convert_local_file", fake_convert)

        cand = oa.Candidate(0, "https://walled.example/a.pdf", "unpaywall", kind_hint="pdf")
        result = await dispatch._candidate_to_result(cand, "10.1234/y", "ua", None)
        assert called, "the impersonation retry must fire on a 4xx-with-body candidate"
        assert result and not result.get("error")

    async def test_pdf_magic_with_4xx_status_is_accepted(self, monkeypatch, isolated_cache):
        """DECISION (documented): a literal %PDF served WITH a 4xx status is accepted — the
        magic bytes prove it's the document; some mirrors mis-status real files."""
        async def fake_fetch_bytes_with_meta(url, ua, proxy=None):
            return b"%PDF-1.4 minimal", 404, None, "application/pdf"

        def fake_convert(path, kind):
            return "# Paper\n\n" + "word " * 200

        monkeypatch.setattr(net, "fetch_bytes_with_meta", fake_fetch_bytes_with_meta)
        monkeypatch.setattr(convert, "convert_local_file", fake_convert)

        cand = oa.Candidate(0, "https://odd.example/b.pdf", "core", kind_hint="pdf")
        result = await dispatch._candidate_to_result(cand, "10.1234/z", "ua", None)
        assert result and not result.get("error")


class TestR1RippleDocRejects4xxBody:
    """A non-PDF-verified doc download with a ≥400 body must NOT write the (permanent) binary
    cache — it retries impersonation, pivots to the mirror with the error page's text, then
    errors with the rungs trace. A %PDF with a 4xx status is the documented exception."""

    _ERR_HTML = (b"<html><head>"
                 b'<meta name="citation_pdf_url" content="https://mirror.example/real.pdf">'
                 b"</head><body>Not found</body></html>")

    async def test_docx_4xx_body_not_written_and_pivots_with_page(self, monkeypatch, isolated_cache):
        async def fake_download_bytes(url, ua, proxy=None):
            return self._ERR_HTML, 404, None

        async def fake_download_impersonated(url, proxy=None):
            return b"", None

        captured = {}

        async def fake_mirror(src, key, ua, proxy, trace=None, page_html=None):
            captured["page_html"] = page_html
            return None

        monkeypatch.setattr(net, "download_bytes", fake_download_bytes)
        monkeypatch.setattr(net, "download_impersonated", fake_download_impersonated)
        monkeypatch.setattr(dispatch, "_try_mirror_for_url", fake_mirror)

        url = "https://gone.example/report.docx"
        result = await dispatch._doc_result(url, url, "docx", False, "ua", None)

        assert "error" in result
        assert "HTTP 404" in result["error"]
        assert "Rungs tried: direct, chrome-impersonation" in result["error"]
        assert not list(isolated_cache.rglob("*.docx")), "4xx body must not enter the binary cache"
        assert not list(isolated_cache.rglob("*.md")), "4xx body must not be converted/cached as text"
        assert captured["page_html"] and "citation_pdf_url" in captured["page_html"], (
            "the 4xx page's text must feed the meta-scrape pivot (no re-fetch)")

    async def test_csv_4xx_html_body_never_converted_to_success(self, monkeypatch, isolated_cache):
        """csv/json converters can turn an HTML error page into non-empty markdown — the gate
        must reject the body before conversion, not after."""
        async def fake_download_bytes(url, ua, proxy=None):
            return b"col_a,col_b\noops,404\n" + b"x," * 600, 410, None

        async def fake_download_impersonated(url, proxy=None):
            return b"", None

        async def fake_mirror(src, key, ua, proxy, trace=None, page_html=None):
            return None

        def boom_convert(path, kind):
            raise AssertionError("a 4xx body must never reach the converter")

        monkeypatch.setattr(net, "download_bytes", fake_download_bytes)
        monkeypatch.setattr(net, "download_impersonated", fake_download_impersonated)
        monkeypatch.setattr(dispatch, "_try_mirror_for_url", fake_mirror)
        monkeypatch.setattr(convert, "convert_local_file", boom_convert)

        url = "https://gone.example/data.csv"
        result = await dispatch._doc_result(url, url, "csv", False, "ua", None)
        assert "error" in result
        assert not list(isolated_cache.rglob("*.csv"))
        assert not list(isolated_cache.rglob("*.md"))

    async def test_pdf_magic_with_4xx_status_is_converted(self, monkeypatch, isolated_cache):
        """DECISION (documented): kind=pdf + %PDF magic + 4xx status → accept and convert."""
        async def fake_download_bytes(url, ua, proxy=None):
            return b"%PDF-1.4 minimal", 403, None

        def fake_convert(path, kind):
            return "# Paper\n\n" + "word " * 200

        monkeypatch.setattr(net, "download_bytes", fake_download_bytes)
        monkeypatch.setattr(convert, "convert_local_file", fake_convert)

        url = "https://odd.example/paper.pdf"
        result = await dispatch._doc_result(url, url, "pdf", False, "ua", None)
        assert "error" not in result, result.get("error")
        assert result["method"] == "pdf:pymupdf4llm"


class TestR1RippleImageRejects4xxBody:
    async def test_4xx_html_body_not_saved_as_image(self, monkeypatch, isolated_cache):
        async def fake_download_bytes(url, ua, proxy=None):
            return b"<html><body>Not Found</body></html>", 404, None

        async def fake_download_impersonated(url, proxy=None):
            return b"", None

        monkeypatch.setattr(net, "download_bytes", fake_download_bytes)
        monkeypatch.setattr(net, "download_impersonated", fake_download_impersonated)

        url = "https://gone.example/fig1.png"
        result = await dispatch._image_result(url, url, False, "ua", None)
        assert "error" in result
        assert not list(isolated_cache.rglob("*.png")), (
            "an HTML error page must never be saved under an image extension")


class TestR1RippleArchiveRejects4xxBody:
    async def test_4xx_html_body_not_saved_as_archive(self, monkeypatch, isolated_cache):
        async def fake_download_bytes(url, ua, proxy=None):
            return b"<html><body>Forbidden</body></html>", 403, None

        async def fake_download_impersonated(url, proxy=None):
            return b"", None

        monkeypatch.setattr(net, "download_bytes", fake_download_bytes)
        monkeypatch.setattr(net, "download_impersonated", fake_download_impersonated)

        url = "https://gone.example/data.zip"
        result = await dispatch._archive_result(url, url, "zip", "", False, "ua", None)
        assert "error" in result
        assert not list(isolated_cache.rglob("*.zip")), (
            "an HTML error page cached as a .zip would make safe_archive refuse it forever")


# ── R2: .pdf-serves-HTML pivots through meta-scrape + mirror ─────────────────

class TestR2PdfServesHtmlRescue:
    """A `.pdf` URL that serves an HTML wall page must try the citation_pdf_url/citation_doi
    chain (reusing the bytes already downloaded) before erroring — not error immediately."""

    _WALL_HTML = (b"<html><head>"
                  b'<meta name="citation_pdf_url" content="https://mirror.example/real.pdf">'
                  b"</head><body>Access Denied</body></html>")

    async def test_pivots_with_bytes_already_in_hand(self, monkeypatch):
        async def fake_download_bytes(url, ua, proxy=None):
            # A wall page served with 200 — exercises the wrong-kind (non-%PDF) branch;
            # a wall page served with 4xx takes the R1-ripple gate (tested separately below).
            return self._WALL_HTML, 200, None

        captured = {}

        async def fake_mirror(src, key, ua, proxy, trace=None, page_html=None):
            captured["page_html"] = page_html
            return {"body": "# rescued\n\n" + "word " * 200, "method": "oa:citation_pdf_url",
                    "cache_status": "miss", "md_path": "/tmp/x.md", "bytes": 900,
                    "content_chars": 900, "http_status": None, "error_kind": None, "challenge": False}

        monkeypatch.setattr(net, "download_bytes", fake_download_bytes)
        monkeypatch.setattr(dispatch, "_try_mirror_for_url", fake_mirror)

        url = "https://walled.example/paper.pdf"
        result = await dispatch._doc_result(url, url, "pdf", False, "ua", None)

        assert "error" not in result, result.get("error")
        assert result["method"] == "oa:citation_pdf_url"
        assert captured["page_html"] and "citation_pdf_url" in captured["page_html"], (
            "R2 must reuse the HTML bytes already downloaded, not discard them before mirroring")

    async def test_no_rescue_returns_error_naming_rungs(self, monkeypatch):
        async def fake_download_bytes(url, ua, proxy=None):
            return self._WALL_HTML, 200, None

        async def fake_mirror(src, key, ua, proxy, trace=None, page_html=None):
            if trace is not None:
                trace.append(dispatch.Attempt(
                    "oa:unpaywall", "https://mirror.example/x", dispatch.Outcome.DEAD_NET))
            return None

        monkeypatch.setattr(net, "download_bytes", fake_download_bytes)
        monkeypatch.setattr(dispatch, "_try_mirror_for_url", fake_mirror)

        url = "https://walled.example/paper.pdf"
        result = await dispatch._doc_result(url, url, "pdf", False, "ua", None)
        assert "error" in result
        assert "did not return a PDF" in result["error"]
        assert "oa-mirror(1 source)" in result["error"]


# ── R3: empty-convert PDF pivots to mirror before erroring ───────────────────

class TestR3EmptyConvertPivotsToMirror:
    async def test_empty_convert_pivots_to_mirror(self, monkeypatch):
        async def fake_download_bytes(url, ua, proxy=None):
            return b"%PDF-1.4 minimal", 200, None

        def fake_convert(path, kind):
            return ""  # scanned/image-only PDF -> empty text

        called = []

        async def fake_mirror(src, key, ua, proxy, trace=None, page_html=None):
            called.append(src)
            return {"body": "# rescued\n\n" + "word " * 200, "method": "oa:core",
                    "cache_status": "miss", "md_path": "/tmp/x.md", "bytes": 900,
                    "content_chars": 900, "http_status": None, "error_kind": None, "challenge": False}

        monkeypatch.setattr(net, "download_bytes", fake_download_bytes)
        monkeypatch.setattr(convert, "convert_local_file", fake_convert)
        monkeypatch.setattr(dispatch, "_try_mirror_for_url", fake_mirror)

        url = "https://scanned.example/paper.pdf"
        result = await dispatch._doc_result(url, url, "pdf", False, "ua", None)
        assert called == [url]
        assert result["method"] == "oa:core"
        assert "error" not in result

    async def test_no_rescue_keeps_ocr_hint(self, monkeypatch):
        """Since the OCR-escalation wave, a REMOTE pdf gets one automatic OCR pass before this
        error — so the message names the attempt, not the env flag (the HARVESTER_PDF_OCR hint
        now applies only to LOCAL files, which the escalation never sees)."""
        async def fake_download_bytes(url, ua, proxy=None):
            return b"%PDF-1.4 minimal", 200, None

        def fake_convert(path, kind):
            return ""

        async def fake_mirror(src, key, ua, proxy, trace=None, page_html=None):
            return None

        monkeypatch.setattr(net, "download_bytes", fake_download_bytes)
        monkeypatch.setattr(convert, "convert_local_file", fake_convert)
        monkeypatch.setattr(dispatch, "_try_mirror_for_url", fake_mirror)

        url = "https://scanned.example/paper.pdf"
        result = await dispatch._doc_result(url, url, "pdf", False, "ua", None)
        assert "error" in result
        assert "OCR pass was already attempted" in result["error"]
        assert "search" in result["error"].lower()

    async def test_local_pdf_still_names_ocr_flag(self, monkeypatch):
        """A LOCAL pdf never goes through the mirror/OCR escalation — its error keeps the
        'set HARVESTER_PDF_OCR=1' hint pointing at the operator's own control."""
        import tempfile
        import os
        fd, p = tempfile.mkstemp(suffix=".pdf")
        os.write(fd, b"%PDF-1.4 minimal")
        os.close(fd)

        def fake_convert(path, kind):
            return ""

        monkeypatch.setattr(convert, "convert_local_file", fake_convert)

        try:
            result = await dispatch._doc_result(p, p, "pdf", True, "ua", None)
        finally:
            os.unlink(p)
        assert "error" in result
        assert "HARVESTER_PDF_OCR" in result["error"]


# ── R4: meta-scrape reuses text already in scope, never plain httpx ──────────

class TestR4MetaScrapeReusesText:
    async def test_reuses_page_html_when_supplied_no_refetch(self, monkeypatch):
        async def boom(*a, **kw):
            raise AssertionError("must not re-fetch when page_html is already in hand")

        monkeypatch.setattr(net, "fetch_raw", boom)
        monkeypatch.setattr(net, "fetch_impersonated", boom)

        async def fake_candidate(cand, key, ua, proxy, trace=None):
            return {"body": "# ok\n\n" + "word " * 200, "method": f"oa:{cand.source}",
                    "cache_status": "miss", "md_path": "/tmp/x.md", "bytes": 900,
                    "content_chars": 900, "http_status": None, "error_kind": None, "challenge": False}

        monkeypatch.setattr(dispatch, "_candidate_to_result", fake_candidate)

        page_html = '<meta name="citation_pdf_url" content="https://mirror.example/real.pdf">'
        result = await dispatch._try_mirror_for_url(
            "https://walled.example/article", "https://walled.example/article", "ua", None,
            page_html=page_html)
        assert result and result["method"] == "oa:citation_pdf_url"

    async def test_no_page_html_uses_impersonation_not_plain_httpx(self, monkeypatch):
        async def boom(*a, **kw):
            raise AssertionError("must not use plain httpx fetch_raw for the meta-scrape")

        called = []

        async def fake_impersonated(url, proxy=None):
            called.append(url)
            return "", 403  # simulate still-blocked; nothing found, that's fine for this test

        async def fake_wayback(url, client):
            return None

        monkeypatch.setattr(net, "fetch_raw", boom)
        monkeypatch.setattr(net, "fetch_impersonated", fake_impersonated)
        monkeypatch.setattr(mirror, "wayback_raw_url", fake_wayback)

        url = "https://walled.example/article"
        result = await dispatch._try_mirror_for_url(url, url, "ua", None)
        assert called == [url]
        assert result is None


# ── R5: an OA candidate serving zip/epub bytes is rejected, never cached as text ──

class TestR5CandidateBinaryRejected:
    async def test_zip_candidate_is_rejected_not_cached_as_text(self, monkeypatch, isolated_cache):
        zip_bytes = b"PK\x03\x04" + b"x" * 2000  # zip magic + filler, easily > THIN_MIN_CHARS if mis-decoded

        async def fake_fetch_bytes_with_meta(url, ua, proxy=None):
            return zip_bytes, 200, None, "application/epub+zip"

        monkeypatch.setattr(net, "fetch_bytes_with_meta", fake_fetch_bytes_with_meta)

        cand = oa.Candidate(0, "https://example.com/book.epub", "internetarchive", kind_hint="pdf")
        result = await dispatch._candidate_to_result(cand, "some-key", "ua", None)

        assert result is None, "an unconvertible archive-kind candidate must be rejected, not cached"
        assert not list(isolated_cache.rglob("*.md")), "no garbage text artifact may be written"


# ── R6: a thin ≥400 page is an error, neg-cached, no artifact ────────────────

class TestR6ThinFourOhFourIsError:
    async def _mocks(self, monkeypatch, calls: dict):
        async def fake_bytes(url, ua, proxy=None):
            calls["n"] = calls.get("n", 0) + 1
            return b"<html><body><p>Not Found</p></body></html>", 404, None, "text/html"

        async def fake_impersonated(url, proxy=None):
            return "", None

        async def fake_jina(url, ua, proxy=None):
            return ""

        async def fake_defuddle(url, ua, proxy=None):
            return ""

        async def fake_wayback(url, client):
            return None

        monkeypatch.setattr(net, "fetch_bytes_with_meta", fake_bytes)
        monkeypatch.setattr(net, "fetch_impersonated", fake_impersonated)
        monkeypatch.setattr(net, "fetch_jina", fake_jina)
        monkeypatch.setattr(net, "fetch_defuddle", fake_defuddle)
        monkeypatch.setattr(mirror, "wayback_raw_url", fake_wayback)

    async def test_thin_404_is_error_not_near_empty_success(self, monkeypatch, isolated_cache):
        await self._mocks(monkeypatch, {})

        url = "https://example.com/deleted-page"
        result = await dispatch._html_result(url, url, False, "ua", None)

        assert "error" in result, "a thin 4xx page must be a clean error, not a near-empty success"
        assert not list(isolated_cache.rglob("*.md")), "no artifact may be written for a thin 4xx"

    async def test_thin_404_is_negative_cached_symmetrically(self, monkeypatch, isolated_cache):
        calls: dict = {}
        await self._mocks(monkeypatch, calls)

        url = "https://example.com/deleted-page"
        r1 = await dispatch.get_or_fetch(url, "ua")
        r2 = await dispatch.get_or_fetch(url, "ua")
        assert "error" in r1 and "error" in r2
        assert calls["n"] == 1, "a thin-404 must be neg-cached, not re-fetched on the very next call"


# ── R7: transient TTL + DOI-form neg-cache canonicalization ──────────────────

class TestR7TransientTtlAndDoiCanonicalization:
    def test_default_transient_ttl_is_15s(self):
        assert dispatch.HARVESTER_NEG_TTL_TRANSIENT == 15.0

    async def test_timeout_error_expires_after_short_transient_ttl(self, monkeypatch):
        clock = {"t": 1000.0}
        monkeypatch.setattr(dispatch.time, "monotonic", lambda: clock["t"])
        monkeypatch.setattr(dispatch, "HARVESTER_NEG_TTL_TRANSIENT", 5.0)

        async def fake_dispatch(item, ua, proxy, media):
            return {"error": "could not reach it", "body": "", "http_status": None, "error_kind": "timeout"}

        monkeypatch.setattr(dispatch, "_dispatch_one", fake_dispatch)
        await dispatch.get_or_fetch("https://slow.example/x", "ua")

        clock["t"] += 6  # past the 5s transient TTL, well within the 120s default
        calls = 0

        async def fake_dispatch2(item, ua, proxy, media):
            nonlocal calls
            calls += 1
            return {"error": "could not reach it", "body": "", "http_status": None, "error_kind": "timeout"}

        monkeypatch.setattr(dispatch, "_dispatch_one", fake_dispatch2)
        await dispatch.get_or_fetch("https://slow.example/x", "ua")
        assert calls == 1, "a timeout must expire from the negative cache after the SHORT transient TTL"

    async def test_429_gets_short_transient_ttl(self, monkeypatch):
        clock = {"t": 2000.0}
        monkeypatch.setattr(dispatch.time, "monotonic", lambda: clock["t"])
        monkeypatch.setattr(dispatch, "HARVESTER_NEG_TTL_TRANSIENT", 5.0)

        async def fake_dispatch(item, ua, proxy, media):
            return {"error": "rate limited", "body": "", "http_status": 429, "error_kind": None}

        monkeypatch.setattr(dispatch, "_dispatch_one", fake_dispatch)
        await dispatch.get_or_fetch("https://api.example/x", "ua")

        clock["t"] += 6
        calls = 0

        async def fake_dispatch2(item, ua, proxy, media):
            nonlocal calls
            calls += 1
            return {"error": "rate limited", "body": "", "http_status": 429, "error_kind": None}

        monkeypatch.setattr(dispatch, "_dispatch_one", fake_dispatch2)
        await dispatch.get_or_fetch("https://api.example/x", "ua")
        assert calls == 1

    async def test_non_transient_error_keeps_full_default_ttl(self, monkeypatch):
        """A 403/permanent error must NOT expire early just because the transient TTL is short."""
        clock = {"t": 3000.0}
        monkeypatch.setattr(dispatch.time, "monotonic", lambda: clock["t"])
        monkeypatch.setattr(dispatch, "HARVESTER_NEG_TTL_TRANSIENT", 5.0)
        monkeypatch.setenv("HARVESTER_NEG_TTL", "120")
        calls = 0

        async def fake_dispatch(item, ua, proxy, media):
            nonlocal calls
            calls += 1
            return {"error": "forbidden", "body": "", "http_status": 403, "error_kind": None}

        monkeypatch.setattr(dispatch, "_dispatch_one", fake_dispatch)
        await dispatch.get_or_fetch("https://walled.example/x", "ua")

        clock["t"] += 6  # past the transient TTL, well within the 120s default
        await dispatch.get_or_fetch("https://walled.example/x", "ua")
        assert calls == 1, "a 403 must stay negative-cached past the short transient TTL"

    async def test_doi_forms_share_one_negative_cache_entry(self, monkeypatch):
        calls = 0

        async def fake_dispatch(item, ua, proxy, media):
            nonlocal calls
            calls += 1
            return {"error": "no open-access copy", "body": ""}

        monkeypatch.setattr(dispatch, "_dispatch_one", fake_dispatch)

        await dispatch.get_or_fetch("10.1234/example", "ua")
        await dispatch.get_or_fetch("doi:10.1234/example", "ua")
        await dispatch.get_or_fetch("https://doi.org/10.1234/example", "ua")
        assert calls == 1, "10.x/y, doi:10.x/y, and the doi.org URL must share one neg-cache entry"
