"""Tests for harvester.mirror and its integration points in server.py."""

import pytest

from harvester import convert, dispatch, images, mirror, net, oa


# ── fixtures ──────────────────────────────────────────────────────────────────

@pytest.fixture(autouse=True)
def isolated_cache(tmp_path, monkeypatch):
    """Redirect the cache to a per-test tmp dir."""
    fetch_dir = tmp_path / ".cache"
    fetch_dir.mkdir()
    monkeypatch.setenv("WEBFETCH_DIR", str(fetch_dir))
    return fetch_dir


@pytest.fixture(autouse=True)
def stub_oa_chain(monkeypatch):
    """Keep mirror tests hermetic: the OA resolver must never hit the network here.

    Tests that exercise the OA chain itself live in test_oa.py and override these.
    """
    async def _empty(*args, **kwargs):
        return []

    monkeypatch.setattr(oa, "resolve_doi", _empty)
    monkeypatch.setattr(oa, "resolve_title", _empty)
    monkeypatch.setattr(oa, "resolve_book", _empty)


# ── extract_doi ───────────────────────────────────────────────────────────────

class TestExtractDoi:
    def test_matches_bare_doi(self):
        assert mirror.extract_doi("10.1073/pnas.2302738120") == "10.1073/pnas.2302738120"

    def test_matches_doi_inside_url(self):
        doi = mirror.extract_doi("https://pnas.org/doi/10.1073/pnas.2302738120")
        assert doi == "10.1073/pnas.2302738120"

    def test_matches_doi_inside_sentence(self):
        doi = mirror.extract_doi("See doi 10.1038/s41586-023-05881-4 for details.")
        assert doi == "10.1038/s41586-023-05881-4"

    def test_strips_trailing_period(self):
        assert mirror.extract_doi("10.1073/pnas.2302738120.") == "10.1073/pnas.2302738120"

    def test_strips_trailing_comma(self):
        assert mirror.extract_doi("10.1073/pnas.2302738120,") == "10.1073/pnas.2302738120"

    def test_strips_trailing_semicolon(self):
        assert mirror.extract_doi("10.1073/pnas.2302738120;") == "10.1073/pnas.2302738120"

    def test_strips_trailing_closing_paren(self):
        assert mirror.extract_doi("(10.1073/pnas.2302738120)") == "10.1073/pnas.2302738120"

    def test_strips_trailing_closing_angle(self):
        assert mirror.extract_doi("10.1073/pnas.2302738120>") == "10.1073/pnas.2302738120"

    def test_rejects_plain_text(self):
        assert mirror.extract_doi("hello world") is None

    def test_rejects_url_without_doi(self):
        assert mirror.extract_doi("https://example.com/page") is None

    def test_rejects_empty_string(self):
        assert mirror.extract_doi("") is None

    def test_rejects_partial_10_prefix(self):
        # "10." alone is not a valid DOI
        assert mirror.extract_doi("version 10.2 of the spec") is None

    def test_case_insensitive_match(self):
        # DOI registrant suffixes can have mixed case
        doi = mirror.extract_doi("10.1016/J.CELL.2023.01.001")
        assert doi == "10.1016/J.CELL.2023.01.001"

    def test_returns_first_doi_when_multiple(self):
        text = "First 10.1073/pnas.111 and second 10.1038/s41586-023-05881-4"
        assert mirror.extract_doi(text) == "10.1073/pnas.111"

    def test_doi_with_hyphens_and_underscores(self):
        doi = mirror.extract_doi("10.1186/s12859-023-05301-w")
        assert doi == "10.1186/s12859-023-05301-w"


# ── URL construction (pure, no network) ──────────────────────────────────────

class TestUrlConstruction:
    def test_pmc_article_url(self):
        url = mirror.pmc_article_url("PMC10450651")
        assert url == "https://pmc.ncbi.nlm.nih.gov/articles/PMC10450651/"

    def test_pmc_article_url_arbitrary_id(self):
        assert "PMC99999" in mirror.pmc_article_url("PMC99999")

    def test_europepmc_figures_zip_url(self):
        url = mirror.europepmc_figures_zip_url("PMC10450651")
        assert url == "https://www.ebi.ac.uk/europepmc/webservices/rest/PMC10450651/supplementaryFiles"

    def test_europepmc_figures_zip_url_arbitrary_id(self):
        assert "PMC42" in mirror.europepmc_figures_zip_url("PMC42")

    def test_pmc_article_url_ends_with_slash(self):
        assert mirror.pmc_article_url("PMC1").endswith("/")


# ── wayback_raw_url (httpx mocked) ───────────────────────────────────────────

class TestWaybackRawUrl:
    async def test_constructs_id_url_from_timestamp(self):
        class FakeResp:
            def json(self):
                return {
                    "archived_snapshots": {
                        "closest": {
                            "available": True,
                            "timestamp": "20230601123456",
                            "url": "https://web.archive.org/web/20230601123456/https://example.com/",
                        }
                    }
                }

        class FakeClient:
            async def get(self, url, **kw):
                return FakeResp()

        result = await mirror.wayback_raw_url("https://example.com/", FakeClient())  # type: ignore[arg-type]
        assert result == "https://web.archive.org/web/20230601123456id_/https://example.com/"

    async def test_returns_none_when_not_available(self):
        class FakeResp:
            def json(self):
                return {"archived_snapshots": {"closest": {"available": False}}}

        class FakeClient:
            async def get(self, url, **kw):
                return FakeResp()

        result = await mirror.wayback_raw_url("https://example.com/", FakeClient())  # type: ignore[arg-type]
        assert result is None

    async def test_returns_none_on_empty_snapshots(self):
        class FakeResp:
            def json(self):
                return {"archived_snapshots": {}}

        class FakeClient:
            async def get(self, url, **kw):
                return FakeResp()

        result = await mirror.wayback_raw_url("https://example.com/", FakeClient())  # type: ignore[arg-type]
        assert result is None

    async def test_returns_none_on_exception(self):
        class FakeClient:
            async def get(self, url, **kw):
                raise RuntimeError("network error")

        result = await mirror.wayback_raw_url("https://example.com/", FakeClient())  # type: ignore[arg-type]
        assert result is None

    async def test_id_modifier_is_present(self):
        """The `id_` modifier must appear between the timestamp and the original URL."""
        class FakeResp:
            def json(self):
                return {
                    "archived_snapshots": {
                        "closest": {"available": True, "timestamp": "20200101000000"}
                    }
                }

        class FakeClient:
            async def get(self, url, **kw):
                return FakeResp()

        result = await mirror.wayback_raw_url("https://x.example/p", FakeClient())  # type: ignore[arg-type]
        assert result is not None
        assert "20200101000000id_/" in result


# ── doi_to_pmcid (httpx mocked) ──────────────────────────────────────────────

class TestDoiToPmcid:
    async def test_returns_pmcid_from_records(self):
        class FakeResp:
            def json(self):
                return {"records": [{"pmcid": "PMC10450651", "doi": "10.1073/pnas.2302738120"}]}

        class FakeClient:
            async def get(self, url, **kw):
                return FakeResp()

        result = await mirror.doi_to_pmcid("10.1073/pnas.2302738120", FakeClient())  # type: ignore[arg-type]
        assert result == "PMC10450651"

    async def test_returns_none_when_no_pmcid(self):
        class FakeResp:
            def json(self):
                return {"records": [{"doi": "10.1234/no-pmc"}]}

        class FakeClient:
            async def get(self, url, **kw):
                return FakeResp()

        result = await mirror.doi_to_pmcid("10.1234/no-pmc", FakeClient())  # type: ignore[arg-type]
        assert result is None

    async def test_returns_none_on_empty_records(self):
        class FakeResp:
            def json(self):
                return {"records": []}

        class FakeClient:
            async def get(self, url, **kw):
                return FakeResp()

        result = await mirror.doi_to_pmcid("10.1234/x", FakeClient())  # type: ignore[arg-type]
        assert result is None

    async def test_returns_none_on_exception(self):
        class FakeClient:
            async def get(self, url, **kw):
                raise RuntimeError("timeout")

        result = await mirror.doi_to_pmcid("10.1234/x", FakeClient())  # type: ignore[arg-type]
        assert result is None


# ── europepmc_pdf (httpx mocked) ─────────────────────────────────────────────

class TestEuropepmcPdf:
    async def test_returns_bytes_when_pdf(self):
        pdf_bytes = b"%PDF-1.4 fake content\n%%EOF"

        class FakeResp:
            content = pdf_bytes

        class FakeClient:
            async def get(self, url, **kw):
                return FakeResp()

        result = await mirror.europepmc_pdf("PMC10450651", FakeClient())  # type: ignore[arg-type]
        assert result == pdf_bytes

    async def test_returns_empty_when_not_pdf(self):
        class FakeResp:
            content = b"<html>not a pdf</html>"

        class FakeClient:
            async def get(self, url, **kw):
                return FakeResp()

        result = await mirror.europepmc_pdf("PMC99999", FakeClient())  # type: ignore[arg-type]
        assert result == b""

    async def test_returns_empty_on_exception(self):
        class FakeClient:
            async def get(self, url, **kw):
                raise RuntimeError("connection error")

        result = await mirror.europepmc_pdf("PMC99999", FakeClient())  # type: ignore[arg-type]
        assert result == b""


# ── integration: DOI input → europepmc PDF path ──────────────────────────────

class TestDoiInputIntegration:
    """get_or_fetch with a DOI-type input resolves via mirror."""

    _PDF_BYTES = b"%PDF-1.4 fake content\n%%EOF"

    async def _setup_mocks(self, monkeypatch, pmcid: str = "PMC10450651") -> None:
        async def fake_doi_to_pmcid(doi: str, client):
            return pmcid

        async def fake_europepmc_pdf(pm: str, client):
            return self._PDF_BYTES

        def fake_pdf_to_md(path: str) -> str:
            return "# Open Access Paper\n\n" + "word " * 200

        monkeypatch.setattr(mirror, "doi_to_pmcid", fake_doi_to_pmcid)
        monkeypatch.setattr(mirror, "europepmc_pdf", fake_europepmc_pdf)
        monkeypatch.setattr(convert, "pdf_to_md", fake_pdf_to_md)

    async def test_bare_doi_returns_mirror_pdf(self, monkeypatch):
        await self._setup_mocks(monkeypatch)
        result = await dispatch.get_or_fetch("10.1073/pnas.2302738120", "test-ua", None)
        assert "error" not in result, result.get("error")
        assert result["method"] == "mirror:europepmc-pdf"
        # Figures URL embedded in body
        assert "supplementaryFiles" in result["body"]

    async def test_doi_prefix_returns_mirror_pdf(self, monkeypatch):
        await self._setup_mocks(monkeypatch)
        result = await dispatch.get_or_fetch("doi:10.1073/pnas.2302738120", "test-ua", None)
        assert "error" not in result, result.get("error")
        assert result["method"] == "mirror:europepmc-pdf"

    async def test_doi_org_url_returns_mirror_pdf(self, monkeypatch):
        await self._setup_mocks(monkeypatch)
        result = await dispatch.get_or_fetch(
            "https://doi.org/10.1073/pnas.2302738120", "test-ua", None
        )
        assert "error" not in result, result.get("error")
        assert result["method"] == "mirror:europepmc-pdf"

    async def test_doi_body_includes_pmcid_figures_url(self, monkeypatch):
        await self._setup_mocks(monkeypatch, pmcid="PMC10450651")
        result = await dispatch.get_or_fetch("10.1073/pnas.2302738120", "test-ua", None)
        assert "PMC10450651" in result["body"]

    async def test_doi_no_pmcid_falls_back_to_wayback(self, monkeypatch):
        async def fake_doi_to_pmcid(doi: str, client):
            return None  # no PMCID

        rich_text = "word " * 200

        async def fake_wayback_raw_url(url: str, client):
            return "https://web.archive.org/web/20230601id_/https://doi.org/10.9999/x"

        async def fake_fetch_raw(url: str, ua: str, proxy=None):
            return f"<html><body><article><p>{rich_text}</p></article></body></html>"

        monkeypatch.setattr(mirror, "doi_to_pmcid", fake_doi_to_pmcid)
        monkeypatch.setattr(mirror, "wayback_raw_url", fake_wayback_raw_url)
        monkeypatch.setattr(net, "fetch_raw", fake_fetch_raw)

        result = await dispatch.get_or_fetch("10.9999/x", "test-ua", None)
        assert "error" not in result, result.get("error")
        assert result["method"] == "mirror:wayback"

    async def test_doi_no_pmcid_no_wayback_returns_error(self, monkeypatch):
        async def fake_doi_to_pmcid(doi: str, client):
            return None

        async def fake_wayback_raw_url(url: str, client):
            return None

        monkeypatch.setattr(mirror, "doi_to_pmcid", fake_doi_to_pmcid)
        monkeypatch.setattr(mirror, "wayback_raw_url", fake_wayback_raw_url)

        result = await dispatch.get_or_fetch("10.9999/notfound", "test-ua", None)
        assert "error" in result
        err = result["error"]
        assert "10.9999/notfound" in err and "open-access" in err and "paywalled" in err


# ── integration: walled URL with embedded DOI → mirror fallback ──────────────

class TestWalledUrlWithDoiFallback:
    """_html_result triggers mirror when publisher URL embeds a DOI and page is walled."""

    _CF_HTML = b"<html><title>Just a moment...</title><body>Checking your browser</body></html>"
    _PDF_BYTES = b"%PDF-1.4 fake\n%%EOF"

    async def test_cf_challenge_with_doi_in_url_uses_mirror_pdf(self, monkeypatch):
        async def fake_bytes(url, ua, proxy=None):
            return self._CF_HTML, 200, None, "text/html"

        async def fake_impersonated(url, proxy=None):
            return "", None

        async def fake_jina(url, ua, proxy=None):
            return ""

        async def fake_defuddle(url, ua, proxy=None):
            return ""

        async def fake_doi_to_pmcid(doi: str, client):
            return "PMC99999"

        async def fake_europepmc_pdf(pmcid: str, client):
            return self._PDF_BYTES

        def fake_pdf_to_md(path: str) -> str:
            return "# Walled Paper\n\n" + "word " * 200

        monkeypatch.setattr(net, "fetch_bytes_with_meta", fake_bytes)
        monkeypatch.setattr(net, "fetch_impersonated", fake_impersonated)
        monkeypatch.setattr(net, "fetch_jina", fake_jina)
        monkeypatch.setattr(net, "fetch_defuddle", fake_defuddle)
        monkeypatch.setattr(mirror, "doi_to_pmcid", fake_doi_to_pmcid)
        monkeypatch.setattr(mirror, "europepmc_pdf", fake_europepmc_pdf)
        monkeypatch.setattr(convert, "pdf_to_md", fake_pdf_to_md)

        result = await dispatch._html_result(
            "https://pnas.org/doi/10.1073/pnas.2302738120",
            "https://pnas.org/doi/10.1073/pnas.2302738120",
            False, "test-ua", None,
        )
        assert "error" not in result, result.get("error")
        assert result["method"] == "mirror:europepmc-pdf"

    async def test_thin_response_with_doi_in_url_uses_mirror(self, monkeypatch):
        thin_html = b"<html><body><p>tiny</p></body></html>"

        async def fake_bytes(url, ua, proxy=None):
            return thin_html, 200, None, "text/html"

        async def fake_impersonated(url, proxy=None):
            return "", None

        async def fake_jina(url, ua, proxy=None):
            return ""

        async def fake_defuddle(url, ua, proxy=None):
            return ""

        async def fake_doi_to_pmcid(doi: str, client):
            return "PMC77777"

        async def fake_europepmc_pdf(pmcid: str, client):
            return self._PDF_BYTES

        def fake_pdf_to_md(path: str) -> str:
            return "# Full Text\n\n" + "word " * 200

        monkeypatch.setattr(net, "fetch_bytes_with_meta", fake_bytes)
        monkeypatch.setattr(net, "fetch_impersonated", fake_impersonated)
        monkeypatch.setattr(net, "fetch_jina", fake_jina)
        monkeypatch.setattr(net, "fetch_defuddle", fake_defuddle)
        monkeypatch.setattr(mirror, "doi_to_pmcid", fake_doi_to_pmcid)
        monkeypatch.setattr(mirror, "europepmc_pdf", fake_europepmc_pdf)
        monkeypatch.setattr(convert, "pdf_to_md", fake_pdf_to_md)

        result = await dispatch._html_result(
            "https://sciencedirect.com/science/article/pii/S0092867423010012?via=10.1016/j.cell.2023.01.001",
            "https://sciencedirect.com/science/article/pii/S0092867423010012?via=10.1016/j.cell.2023.01.001",
            False, "test-ua", None,
        )
        assert "error" not in result, result.get("error")
        assert result["method"] == "mirror:europepmc-pdf"

    async def test_mirror_not_triggered_for_rich_content(self, monkeypatch):
        """When the page loads fine, mirror must NOT be called."""
        rich_html = ("<html><body><article><p>" + "word " * 200 + "</p></article></body></html>").encode()

        async def fake_bytes(url, ua, proxy=None):
            return rich_html, 200, None, "text/html"

        doi_to_pmcid_called = []

        async def fake_doi_to_pmcid(doi: str, client):
            doi_to_pmcid_called.append(doi)
            return None

        async def noop_localise(md, base_url, ua, proxy=None):
            return md

        monkeypatch.setattr(net, "fetch_bytes_with_meta", fake_bytes)
        monkeypatch.setattr(mirror, "doi_to_pmcid", fake_doi_to_pmcid)
        monkeypatch.setattr(images, "localize_html_images", noop_localise)

        result = await dispatch._html_result(
            "https://pnas.org/doi/10.1073/pnas.2302738120",
            "https://pnas.org/doi/10.1073/pnas.2302738120",
            False, "test-ua", None,
        )
        assert result["method"] == "local-trafilatura"
        assert not doi_to_pmcid_called, "mirror must not be called when content is rich"


# ── integration: no-DOI walled URL → wayback attempt ─────────────────────────

class TestWalledUrlNoDoiWayback:
    """When a walled URL has no embedded DOI, the mirror tries Wayback."""

    _THIN_HTML = b"<html><body><p>tiny</p></body></html>"

    async def test_no_doi_url_tries_wayback(self, monkeypatch):
        rich_text = "word " * 200

        async def fake_bytes(url, ua, proxy=None):
            return self._THIN_HTML, 200, None, "text/html"

        async def fake_impersonated(url, proxy=None):
            return "", None

        async def fake_jina(url, ua, proxy=None):
            return ""

        async def fake_defuddle(url, ua, proxy=None):
            return ""

        async def fake_wayback_raw_url(url: str, client):
            return "https://web.archive.org/web/20230601000000id_/https://wall.example/page"

        async def fake_fetch_raw(url: str, ua: str, proxy=None):
            if "web.archive.org" in url:
                return f"<html><body><article><p>{rich_text}</p></article></body></html>"
            return ""

        monkeypatch.setattr(net, "fetch_bytes_with_meta", fake_bytes)
        monkeypatch.setattr(net, "fetch_impersonated", fake_impersonated)
        monkeypatch.setattr(net, "fetch_jina", fake_jina)
        monkeypatch.setattr(net, "fetch_defuddle", fake_defuddle)
        monkeypatch.setattr(mirror, "wayback_raw_url", fake_wayback_raw_url)
        monkeypatch.setattr(net, "fetch_raw", fake_fetch_raw)

        result = await dispatch._html_result(
            "https://wall.example/page",
            "https://wall.example/page",
            False, "test-ua", None,
        )
        assert "error" not in result, result.get("error")
        assert result["method"] == "mirror:wayback"

    async def test_private_host_skips_mirror(self, monkeypatch):
        """Mirror fallback must NOT fire for private/internal hosts."""
        async def fake_bytes(url, ua, proxy=None):
            return self._THIN_HTML, 200, None, "text/html"

        async def fake_impersonated(url, proxy=None):
            return "", None

        async def fake_jina(url, ua, proxy=None):
            return ""

        async def fake_defuddle(url, ua, proxy=None):
            return ""

        wayback_called = []

        async def fake_wayback_raw_url(url: str, client):
            wayback_called.append(url)
            return None

        monkeypatch.setattr(net, "fetch_bytes_with_meta", fake_bytes)
        monkeypatch.setattr(net, "fetch_impersonated", fake_impersonated)
        monkeypatch.setattr(net, "fetch_jina", fake_jina)
        monkeypatch.setattr(net, "fetch_defuddle", fake_defuddle)
        monkeypatch.setattr(mirror, "wayback_raw_url", fake_wayback_raw_url)

        await dispatch._html_result(
            "http://192.168.1.1/admin",
            "http://192.168.1.1/admin",
            False, "test-ua", None,
        )
        assert not wayback_called, "Mirror must NOT be called for private hosts"

    async def test_wayback_thin_does_not_override_existing_body(self, monkeypatch):
        """When wayback also returns thin content, we fall through to the original result."""
        async def fake_bytes(url, ua, proxy=None):
            return self._THIN_HTML, 200, None, "text/html"

        async def fake_impersonated(url, proxy=None):
            return "", None

        async def fake_jina(url, ua, proxy=None):
            return ""

        async def fake_defuddle(url, ua, proxy=None):
            return ""

        async def fake_wayback_raw_url(url: str, client):
            return "https://web.archive.org/web/20230601000000id_/https://wall.example/page"

        async def fake_fetch_raw(url: str, ua: str, proxy=None):
            return "<html><body><p>tiny</p></body></html>"

        monkeypatch.setattr(net, "fetch_bytes_with_meta", fake_bytes)
        monkeypatch.setattr(net, "fetch_impersonated", fake_impersonated)
        monkeypatch.setattr(net, "fetch_jina", fake_jina)
        monkeypatch.setattr(net, "fetch_defuddle", fake_defuddle)
        monkeypatch.setattr(mirror, "wayback_raw_url", fake_wayback_raw_url)
        monkeypatch.setattr(net, "fetch_raw", fake_fetch_raw)

        result = await dispatch._html_result(
            "https://wall.example/page",
            "https://wall.example/page",
            False, "test-ua", None,
        )
        # Mirror returned None (wayback also thin), so result is still from trafilatura path
        assert result["method"] != "mirror:wayback"
