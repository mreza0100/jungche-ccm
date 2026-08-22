"""Tests for the harvester-perfection wave:

* EPUB end-to-end — detection (ext/content-type/magic), conversion, and the OA-candidate
  un-rejection that previously threw found books away as 'archive-shaped' bytes
* OCR escalation rung — a remote PDF whose text layer converts empty gets ONE forced-OCR pass
* Image-localisation wiring — HARVESTER_LOCALIZE_IMAGES=1 rewrites figure links to local paths
* findWorks widening — Crossref + Semantic Scholar candidates, similarity-gated and deduped
* stats scoreboard — record_fetch/summarize roundtrip, malformed-line tolerance
"""

import json

from harvester import convert, detect, dispatch, net, oa, stats


# ── fixtures ─────────────────────────────────────────────────────────────────────

def _minimal_epub_bytes(title: str = "T", chapter: str = "World of epubs.") -> bytes:
    """Build a real EPUB in memory (mimetype stored FIRST, uncompressed — the spec order)."""
    import zipfile
    import io
    buf = io.BytesIO()
    with zipfile.ZipFile(buf, "w") as z:
        z.writestr("mimetype", "application/epub+zip", compress_type=zipfile.ZIP_STORED)
        z.writestr("META-INF/container.xml",
                   '<?xml version="1.0"?><container version="1.0" '
                   'xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles>'
                   '<rootfile full-path="OEBPS/content.opf" '
                   'media-type="application/oebps-package+xml"/></rootfiles></container>')
        z.writestr("OEBPS/content.opf",
                   '<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" '
                   'version="3.0" unique-identifier="id"><metadata '
                   'xmlns:dc="http://purl.org/dc/elements/1.1/">'
                   f'<dc:title>{title}</dc:title>'
                   '<dc:identifier id="id">x</dc:identifier></metadata><manifest>'
                   '<item id="c1" href="c1.xhtml" media-type="application/xhtml+xml"/></manifest>'
                   '<spine><itemref idref="c1"/></spine></package>')
        z.writestr("OEBPS/c1.xhtml",
                   f"<html><body><h1>Hello</h1><p>{chapter}</p></body></html>")
    return buf.getvalue()


EPUB_BYTES = _minimal_epub_bytes()


# ── EPUB detection ───────────────────────────────────────────────────────────────

class TestEpubDetection:
    def test_detect_kind_by_extension(self):
        assert detect.detect_kind("https://library.oapen.org/book.epub") == "epub"
        assert detect.detect_kind("/tmp/something.epub?download=1") == "epub"

    def test_sniff_kind_by_content_type(self):
        assert detect._sniff_kind("application/epub+zip", EPUB_BYTES[:16]) == "epub"

    def test_looks_like_epub_true(self):
        assert detect.looks_like_epub(EPUB_BYTES) is True

    def test_looks_like_epub_false_for_plain_zip(self):
        import zipfile
        import io
        buf = io.BytesIO()
        with zipfile.ZipFile(buf, "w") as z:
            z.writestr("a.txt", "not a book")
        assert detect.looks_like_epub(buf.getvalue()) is False

    def test_epub_is_a_doc_kind_not_an_archive(self):
        assert "epub" in detect.DOC_KINDS
        assert "epub" not in detect.ARCHIVE_KINDS


class TestEpubConversion:
    def test_epub_to_md(self):
        body = _convert_bytes()
        assert "Hello" in body
        assert "World of epubs." in body


def _convert_bytes() -> str:
    import tempfile
    import os
    fd, p = tempfile.mkstemp(suffix=".epub")
    try:
        os.write(fd, EPUB_BYTES)
        os.close(fd)
        return convert.epub_to_md(p)
    finally:
        os.unlink(p)


class TestEpubCandidateUnReject:
    async def test_zip_shaped_candidate_converts_as_epub(self, monkeypatch):
        """R5 regression: an OA candidate serving EPUB bytes was rejected WRONG_KIND; now it
        must be converted and returned as real content."""
        async def fake_fetch(url, ua, proxy):
            return EPUB_BYTES, 200, None, "application/zip"

        async def fake_imp(url, proxy):
            return b"", None

        monkeypatch.setattr(net, "fetch_bytes_with_meta", fake_fetch)
        monkeypatch.setattr(net, "download_impersonated", fake_imp)

        cand = oa.Candidate(oa._score("oapen"), "https://library.oapen.org/handle/book.epub",
                            "oapen", "gold", "", "pdf")
        res = await dispatch._candidate_to_result(cand, "isbn:testbook", "ua", None)
        assert res is not None, "an EPUB candidate must convert, not be rejected as archive bytes"
        assert "error" not in res
        assert "Hello" in res.get("body", "")
        # cached under the IDENTIFIER key, method names the epub converter
        assert res["method"] == "book:markitdown"

    async def test_plain_zip_still_rejected(self, monkeypatch):
        import zipfile
        import io
        buf = io.BytesIO()
        with zipfile.ZipFile(buf, "w") as z:
            z.writestr("data.bin", "\x00" * 64)

        async def fake_fetch(url, ua, proxy):
            return buf.getvalue(), 200, None, "application/zip"

        async def fake_imp(url, proxy):
            return b"", None

        monkeypatch.setattr(net, "fetch_bytes_with_meta", fake_fetch)
        monkeypatch.setattr(net, "download_impersonated", fake_imp)

        cand = oa.Candidate(10, "https://example.org/data.zip", "core", "", "", "pdf")
        res = await dispatch._candidate_to_result(cand, "key-zip", "ua", None)
        assert res is None, "a NON-epub zip candidate must still be rejected (R5 holds)"
        # and the artifact must NOT have been cached as a silent success
        assert not list(__import__("pathlib").Path(dispatch.cache.cache_root() / "archive_member")
                        .glob("*key-zip*"))


# ── OCR escalation rung ──────────────────────────────────────────────────────────

class TestOcrEscalation:
    async def test_empty_text_layer_gets_one_ocr_pass(self, monkeypatch):
        """A remote PDF whose text layer is empty (a scan) escalates to one forced-OCR pass
        AFTER the mirror chain gives up — OCR is the LAST rung, never a shortcut past OA."""
        ladder: list[str] = []

        async def fake_download(url, ua, proxy):
            return b"%PDF-1.7 scanned pages only", 200, None

        async def fake_imp(url, proxy):
            return b"", None

        async def fake_mirror(*a, **kw):
            ladder.append("mirror")
            return None

        def fake_pdf_to_md(path, force_ocr=False):
            ladder.append(f"ocr:{force_ocr}")
            return "OCR TEXT " * 120 if force_ocr else ""

        monkeypatch.setattr(net, "download_bytes", fake_download)
        monkeypatch.setattr(net, "download_impersonated", fake_imp)
        monkeypatch.setattr(dispatch, "_try_mirror_for_url", fake_mirror)
        monkeypatch.setattr(convert, "pdf_to_md", fake_pdf_to_md)

        res = await dispatch.get_or_fetch("https://journals.example/scanned-paper.pdf", "ua")
        assert "error" not in res, "OCR rung should have rescued the scanned PDF"
        assert res["method"] == "pdf:pymupdf4llm-ocr"
        assert ladder == ["ocr:False", "mirror", "ocr:True"], (
            "ladder order must be fast extract FIRST, then mirror, then forced OCR — "
            f"got {ladder}")
        assert "OCR TEXT" in res["body"]

    async def test_failed_ocr_falls_through_to_error(self, monkeypatch):
        async def fake_download(url, ua, proxy):
            return b"%PDF-1.7 garbage", 200, None

        async def fake_imp(url, proxy):
            return b"", None

        async def fake_mirror(*a, **kw):
            return None

        def fake_pdf_to_md(path, force_ocr=False):
            return ""

        monkeypatch.setattr(net, "download_bytes", fake_download)
        monkeypatch.setattr(net, "download_impersonated", fake_imp)
        monkeypatch.setattr(convert, "pdf_to_md", fake_pdf_to_md)
        monkeypatch.setattr(dispatch, "_try_mirror_for_url", fake_mirror)

        res = await dispatch.get_or_fetch("https://journals.example/broken.pdf", "ua")
        assert "error" in res
        # the error names the OCR attempt instead of telling you to enable what already ran
        assert "OCR pass was already attempted" in res["error"]


# ── image localisation wiring ────────────────────────────────────────────────────

_MD_WITH_FIGURE = ("Prose paragraph. " * 60
                   + "\n\n![Figure 1](https://imgs.example/fig1.png)\n\n"
                   + "More prose. " * 60)


class TestImageLocalisationWiring:
    async def test_flag_on_rewrites_links(self, monkeypatch):
        monkeypatch.setattr(dispatch, "_LOCALIZE_IMAGES", True)

        async def fake_fetch(url, ua, proxy):
            return b"<html><body>page</body></html>", 200, None, "text/html"

        def fake_extract(raw):
            return _MD_WITH_FIGURE

        async def fake_localize(md, base_url, ua, proxy=None):
            return md.replace("https://imgs.example/fig1.png", "/cache/png/fig1.png")

        monkeypatch.setattr(net, "fetch_bytes_with_meta", fake_fetch)
        monkeypatch.setattr(dispatch.html, "extract_content_from_html", fake_extract)
        monkeypatch.setattr(dispatch.images, "localize_html_images", fake_localize)

        res = await dispatch.get_or_fetch("https://articles.example/paper", "ua")
        assert "error" not in res
        assert "/cache/png/fig1.png" in res["body"]
        assert "https://imgs.example/fig1.png" not in res["body"]

    async def test_default_off_leaves_remote_links(self, monkeypatch):
        assert dispatch._LOCALIZE_IMAGES is False, "default build ships with localisation OFF"

        called = []

        async def fake_localize(md, base_url, ua, proxy=None):
            called.append(base_url)
            return md

        async def fake_fetch(url, ua, proxy):
            return b"<html><body>page</body></html>", 200, None, "text/html"

        def fake_extract(raw):
            return _MD_WITH_FIGURE

        monkeypatch.setattr(net, "fetch_bytes_with_meta", fake_fetch)
        monkeypatch.setattr(dispatch.html, "extract_content_from_html", fake_extract)
        monkeypatch.setattr(dispatch.images, "localize_html_images", fake_localize)

        res = await dispatch.get_or_fetch("https://articles.example/paper2", "ua")
        assert "error" not in res
        assert not called, "with the flag off, localisation must never run"
        assert "https://imgs.example/fig1.png" in res["body"]

    async def test_localization_failure_keeps_remote_links(self, monkeypatch):
        monkeypatch.setattr(dispatch, "_LOCALIZE_IMAGES", True)

        async def fake_localize(md, base_url, ua, proxy=None):
            raise RuntimeError("network gone")

        async def fake_fetch(url, ua, proxy):
            return b"<html><body>page</body></html>", 200, None, "text/html"

        def fake_extract(raw):
            return _MD_WITH_FIGURE

        monkeypatch.setattr(net, "fetch_bytes_with_meta", fake_fetch)
        monkeypatch.setattr(dispatch.html, "extract_content_from_html", fake_extract)
        monkeypatch.setattr(dispatch.images, "localize_html_images", fake_localize)

        res = await dispatch.get_or_fetch("https://articles.example/paper3", "ua")
        assert "error" not in res, "a localisation crash must not fail the fetch"
        assert "https://imgs.example/fig1.png" in res["body"], "remote links survive"


# ── findWorks widening ───────────────────────────────────────────────────────────

class TestFindWorksWidening:
    CROSSREF_JSON = {"message": {"items": [
        {"DOI": "10.1234/wide.find", "title": ["A Wide Finder Study"],
         "author": [{"given": "A.", "family": "Author"}],
         "issued": {"date-parts": [[2024]]}},
        {"DOI": "10.1234/unrelated", "title": ["Something Entirely Unrelated To The Query"],
         "author": [], "issued": {"date-parts": [[2020]]}},
    ]}}

    S2_JSON = {"data": [
        {"title": "A Wide Finder Study", "year": 2023,
         "authors": [{"name": "B. Author"}],
         "externalIds": {"DOI": "10.1234/wide.find", "ArXiv": "2401.00001"},
         "openAccessPdf": {"url": "https://s2.example/pdf"}},
        {"title": "No Handle Whatsoever", "year": 2022,
         "authors": [], "externalIds": {}, "openAccessPdf": None},
    ]}

    async def test_crossref_parse_and_gate(self):
        client = oa_tests_client({"/works?query.bibliographic": self.CROSSREF_JSON})
        out = await oa._find_crossref("wide finder study", client, 8)
        assert len(out) == 1, "the unrelated-title item must be similarity-gated away"
        c = out[0]
        assert c["fetch"] == "10.1234/wide.find" and c["source"] == "crossref"
        assert c["year"] == 2024 and "Author" in c["authors"]

    async def test_s2_prefers_doi_handle_and_marks_free(self):
        client = oa_tests_client({"paper/search": self.S2_JSON})
        out = await oa._find_s2("wide finder study", client, 8)
        assert len(out) == 1, "the no-handle paper must be dropped"
        c = out[0]
        assert c["fetch"] == "10.1234/wide.find"
        assert c["free"] == "green"

    async def test_find_works_dedupes_across_sources(self, monkeypatch):
        """The same work surfaced by OpenAlex AND Crossref AND S2 collapses to ONE candidate."""
        openalex = {"results": [{
            "id": "w1", "display_name": "A Wide Finder Study", "doi": "https://doi.org/10.1234/wide.find",
            "open_access": {"oa_status": "green", "oa_url": None},
            "locations": [], "authorships": [], "publication_year": 2024,
        }]}
        routes = {
            "api.openalex.org/works?filter=title.search": openalex,
            "query.bibliographic": self.CROSSREF_JSON,
            "paper/search": self.S2_JSON,
            "export.arxiv.org": "<opensearch><totalResults>0</totalResults></opensearch>",
            "openlibrary.org": {"docs": []},
            "gutendex.com": {"results": []},
        }
        client = oa_tests_client(routes)

        async def fake_arxiv_by_title(query, client):
            return []

        monkeypatch.setattr(oa, "_arxiv_by_title", fake_arxiv_by_title)
        out = await oa.find_works("wide finder study", client, limit=8)
        handles = [c["fetch"] for c in out]
        assert handles.count("10.1234/wide.find") == 1, (
            f"three sources found the same DOI; dedupe must keep exactly one — got {handles}")


def oa_tests_client(routes: dict) -> "object":
    """FakeClient keyed by substring → parsed-JSON value (test_oa.py's shape, dict form)."""
    class Resp:
        def __init__(self, data):
            self._d = data
            self.status_code = 200

        def json(self):
            if self._d is None:
                raise ValueError("no json")
            return self._d

    class C:
        def __init__(self):
            self.calls = []

        async def get(self, url, **kw):
            self.calls.append(("GET", url))
            for sub, data in routes.items():
                if sub in url:
                    return Resp(data)
            return Resp(None)

        async def post(self, url, **kw):
            self.calls.append(("POST", url))
            for sub, data in routes.items():
                if sub in url:
                    return Resp(data)
            return Resp(None)

    return C()


# ── stats scoreboard ─────────────────────────────────────────────────────────────

class TestStatsScoreboard:
    def test_record_and_summarize_roundtrip(self):
        stats.record_fetch("https://a.example/x", True, "jina-reader")
        stats.record_fetch("https://b.example/y", False, "timeout")
        stats.record_fetch("https://c.example/z", True, "jina-reader")
        buckets = stats.summarize()
        assert buckets["jina-reader"]["total"] == 2 and buckets["jina-reader"]["rate"] == 1.0
        assert buckets["timeout"]["total"] == 1 and buckets["timeout"]["rate"] == 0.0

    def test_malformed_lines_are_skipped_not_fatal(self):
        stats.record_fetch("https://a.example/x", True, "direct")
        with open(stats.stats_path(), "a", encoding="utf-8") as fh:
            fh.write("{not json\n")
            fh.write("[broken\n")
        stats.record_fetch("https://b.example/y", True, "direct")
        buckets = stats.summarize()
        assert buckets["direct"]["total"] == 2

    def test_summarize_empty_when_no_file(self, tmp_path, monkeypatch):
        monkeypatch.setenv("WEBFETCH_DIR", str(tmp_path / "empty-cache"))
        import importlib
        importlib.reload(stats)
        try:
            assert stats.summarize() == {}
        finally:
            importlib.reload(stats)  # restore the shared cache root for other tests

    async def test_stats_written_by_real_fetch(self, monkeypatch):
        """The scoreboard provably RUNS on the live dispatch path — an empty stats.jsonl after
        real fetches would mean the recorder never fired (the coincidence-detector failure)."""
        async def fake_dispatch(item, ua, proxy, media):
            return {"body": "fine", "method": "test-method", "cache_status": "miss",
                    "md_path": "/tmp/x.md", "bytes": 4, "content_chars": 4,
                    "http_status": 200, "error_kind": None, "challenge": False}

        monkeypatch.setattr(dispatch, "_dispatch_one", fake_dispatch)
        await dispatch.get_or_fetch("https://stats.example/live", "ua")

        lines = [json.loads(ln) for ln in stats.stats_path().read_text().splitlines() if ln.strip()]
        rec = [r for r in lines if r["item"] == "https://stats.example/live"]
        assert rec and rec[0]["ok"] is True and rec[0]["detail"] == "test-method"


# ── review walk 2: the browser SSRF route guard must execute under CI ───────────

class TestBrowserRouteGuard:
    async def test_private_redirect_target_is_aborted(self):
        """A public URL that 302s to a metadata endpoint dies at the ROUTE layer —
        the exact walk-1 Critical, now proven under CI with no browser installed."""
        from harvester.net import FetchNotAllowed, browser_route_guard

        class FakeRequest:
            url = "http://169.254.169.254/latest/meta-data/"

        class FakeRoute:
            def __init__(self):
                self.aborted = None
                self.continued = False

            @property
            def request(self):
                return FakeRequest()

            async def abort(self, reason):
                self.aborted = reason

            async def continue_(self):
                self.continued = True

        route = FakeRoute()
        await browser_route_guard()(route)
        assert route.aborted == "blocked" and not route.continued, (
            "a metadata-endpoint request must be aborted before Chrome connects")

    async def test_public_target_continues(self):
        from harvester.net import browser_route_guard

        class FakeRequest:
            url = "https://example.com/page"

        class FakeRoute:
            def __init__(self):
                self.aborted = None
                self.continued = False

            @property
            def request(self):
                return FakeRequest()

            async def abort(self, reason):
                self.aborted = reason

            async def continue_(self):
                self.continued = True

        route = FakeRoute()
        await browser_route_guard()(route)
        assert route.continued and route.aborted is None

    def test_fetch_browser_wires_the_guard(self, monkeypatch):
        """The production path must register THE guard on context.route(**/*)."""
        import inspect

        import harvester.net as net
        src = inspect.getsource(net.fetch_browser)
        assert 'context.route("**/*", browser_route_guard())' in src, (
            "fetch_browser must install the shared guard on every request")
