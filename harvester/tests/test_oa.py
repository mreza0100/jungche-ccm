"""Unit tests for harvester.oa — the open-access resolver.

Hermetic: a FakeClient returns canned JSON (the shapes verified live 2026-06-28) so NO test
touches the network. One focused test per source confirms the parse + the key edge case;
resolve_doi/title/book cover routing, dedup, and ordering. Live API smoke checks (one per
source) live in test_oa_integration.py and are skipped unless RUN_OA_INTEGRATION=1.
"""
import pytest

from harvester import dispatch, oa


# ── fakes ───────────────────────────────────────────────────────────────────────

class FakeResp:
    def __init__(self, status=200, json_data=None, text=""):
        self.status_code = status
        self._json = json_data
        self.text = text

    def json(self):
        if self._json is None:
            raise ValueError("no json")
        return self._json


class FakeClient:
    """Routes a request URL to a canned response by substring match (first hit wins)."""

    def __init__(self, routes):
        self.routes = routes  # list of (substr, FakeResp)
        self.calls = []

    async def get(self, url, **kw):
        self.calls.append(("GET", url))
        for sub, resp in self.routes:
            if sub in url:
                return resp
        return FakeResp(404, None, "not found")

    async def post(self, url, **kw):
        self.calls.append(("POST", url, kw.get("json")))
        for sub, resp in self.routes:
            if sub in url:
                return resp
        return FakeResp(404, None, "")


# ── helpers ──────────────────────────────────────────────────────────────────────

class TestNormalizeIsbn:
    def test_isbn13(self):
        assert oa.normalize_isbn("978-0-262-30098-8") == "9780262300988"

    def test_isbn10_with_x(self):
        assert oa.normalize_isbn("0-201-61622-X") == "020161622X"  # valid ISBN-10 check digit X

    def test_rejects_title(self):
        assert oa.normalize_isbn("Frankenstein") is None

    def test_rejects_short_number(self):
        assert oa.normalize_isbn("12345") is None

    def test_rejects_bad_checksum(self):
        assert oa.normalize_isbn("978-0-262-30098-7") is None  # wrong check digit

    def test_rejects_all_same_digit(self):
        assert oa.normalize_isbn("0000000000") is None and oa.normalize_isbn("9781111111111") is None


class TestBareDoi:
    def test_from_url(self):
        assert oa._bare_doi("https://doi.org/10.1038/s41586-020-2649-2") == "10.1038/s41586-020-2649-2"

    def test_strips_trailing_punct(self):
        assert oa._bare_doi("see 10.1371/journal.pone.0173664).") == "10.1371/journal.pone.0173664"

    def test_none_on_plain_text(self):
        assert oa._bare_doi("not a doi") is None


class TestSimilar:
    def test_identical(self):
        assert oa._similar("Array programming with NumPy", "Array programming with NumPy") == 1.0

    def test_low_overlap(self):
        assert oa._similar("Array programming with NumPy", "A totally different paper") < 0.3


class TestScoreOrdering:
    def test_arxiv_beats_unpaywall_beats_core(self):
        assert oa._score("arxiv") < oa._score("unpaywall") < oa._score("core")

    def test_bronze_penalised(self):
        assert oa._score("unpaywall", "bronze") > oa._score("unpaywall", "gold")

    def test_landing_worse_than_pdf(self):
        assert oa._score("openalex", kind="html") > oa._score("openalex", kind="pdf")


class TestExtractMetaLinks:
    def test_citation_pdf_and_doi(self):
        page = (
            '<meta name="citation_pdf_url" content="https://pub.example/x.pdf">'
            '<meta name="citation_doi" content="10.1234/abc">'
            '<meta name="citation_title" content="A Great Paper">'
        )
        doi, pdf, title = oa.extract_meta_links(page)
        assert pdf == "https://pub.example/x.pdf"
        assert doi == "10.1234/abc"
        assert title == "A Great Paper"

    def test_reversed_attr_order(self):
        page = '<meta content="https://pub.example/y.pdf" name="citation_pdf_url"/>'
        _doi, pdf, _t = oa.extract_meta_links(page)
        assert pdf == "https://pub.example/y.pdf"

    def test_none_when_absent(self):
        assert oa.extract_meta_links("<html><body>nothing</body></html>") == (None, None, None)


# ── article sources ──────────────────────────────────────────────────────────────

class TestUnpaywall:
    async def test_best_url_for_pdf(self):
        c = FakeClient([("api.unpaywall.org", FakeResp(json_data={
            "is_oa": True, "oa_status": "gold",
            "best_oa_location": {"url_for_pdf": "https://plos.example/a.pdf", "version": "publishedVersion"},
            "oa_locations": [],
        }))])
        cands = await oa.from_unpaywall("10.1/x", c)
        assert cands and cands[0].url == "https://plos.example/a.pdf"
        assert cands[0].source == "unpaywall" and cands[0].oa_status == "gold"

    async def test_not_oa_returns_empty(self):
        c = FakeClient([("api.unpaywall.org", FakeResp(json_data={"is_oa": False, "best_oa_location": None}))])
        assert await oa.from_unpaywall("10.1/x", c) == []

    async def test_404_returns_empty(self):
        c = FakeClient([("api.unpaywall.org", FakeResp(status=404))])
        assert await oa.from_unpaywall("10.1/x", c) == []


class TestOpenAlex:
    async def test_oa_url(self):
        c = FakeClient([("api.openalex.org", FakeResp(json_data={
            "open_access": {"is_oa": True, "oa_status": "gold", "oa_url": "https://oa.example/b.pdf"},
            "locations": [],
        }))])
        cands = await oa.from_openalex("10.1/x", c)
        assert cands[0].url == "https://oa.example/b.pdf" and cands[0].source == "openalex"


class TestSemanticScholar:
    async def test_open_access_pdf(self):
        c = FakeClient([("api.semanticscholar.org", FakeResp(json_data={
            "openAccessPdf": {"url": "https://s2.example/c.pdf", "status": "HYBRID"},
            "externalIds": {"ArXiv": "2006.10256"},
        }))])
        cands = await oa.from_semanticscholar("10.1/x", c)
        urls = [c_.url for c_ in cands]
        assert "https://s2.example/c.pdf" in urls
        assert "https://arxiv.org/pdf/2006.10256" in urls  # externalId fallback

    async def test_empty_url_skipped(self):
        c = FakeClient([("api.semanticscholar.org", FakeResp(json_data={
            "openAccessPdf": {"url": "", "status": None}, "externalIds": {},
        }))])
        assert await oa.from_semanticscholar("10.1/x", c) == []


class TestCrossref:
    async def test_pdf_link(self):
        c = FakeClient([("api.crossref.org", FakeResp(json_data={"message": {"link": [
            {"content-type": "unspecified", "URL": "https://x/landing"},
            {"content-type": "application/pdf", "URL": "https://x/full.pdf"},
        ]}}))])
        cands = await oa.from_crossref("10.1/x", c)
        assert [c_.url for c_ in cands] == ["https://x/full.pdf"]

    async def test_no_pdf_link(self):
        c = FakeClient([("api.crossref.org", FakeResp(json_data={"message": {"link": [
            {"content-type": "unspecified", "URL": "https://x/landing"},
        ]}}))])
        assert await oa.from_crossref("10.1/x", c) == []


class TestCore:
    async def test_download_url(self):
        c = FakeClient([("api.core.ac.uk", FakeResp(json_data={
            "downloadUrl": "https://repo.example/d.pdf", "sourceFulltextUrls": [],
        }))])
        cands = await oa.from_core("10.1/x", c)
        assert cands[0].url == "https://repo.example/d.pdf" and cands[0].source == "core"


class TestDoaj:
    async def test_fulltext_link(self):
        c = FakeClient([("doaj.org", FakeResp(json_data={"results": [{"bibjson": {"link": [
            {"type": "fulltext", "url": "https://journal.example/e"},
        ]}}]}))])
        cands = await oa.from_doaj("10.1/x", c)
        assert cands[0].url == "https://journal.example/e"


class TestOsf:
    async def test_primary_file_download(self):
        c = FakeClient([("api.osf.io", FakeResp(json_data={"data": {"relationships": {
            "primary_file": {"data": {"id": "abc123"}}}}}))])
        cands = await oa.from_osf("10.31235/osf.io/yvzcb", c)
        assert cands[0].url == "https://osf.io/download/abc123/"

    async def test_no_guid(self):
        assert await oa.from_osf("10.1/notosf", FakeClient([])) == []


class TestArxivCandidates:
    def test_url(self):
        assert oa.arxiv_candidates("1706.03762")[0].url == "https://arxiv.org/pdf/1706.03762"


# ── resolve_doi routing ──────────────────────────────────────────────────────────

class TestResolveDoi:
    async def test_arxiv_prefix_is_deterministic(self):
        c = FakeClient([])  # no API calls needed
        cands = await oa.resolve_doi("10.48550/arXiv.1706.03762", c)
        assert len(cands) == 1
        assert cands[0].url == "https://arxiv.org/pdf/1706.03762"
        assert c.calls == []  # short-circuit: no network

    async def test_osf_prefix_routes_to_osf(self):
        c = FakeClient([("api.osf.io", FakeResp(json_data={"data": {"relationships": {
            "primary_file": {"data": {"id": "zz"}}}}}))])
        cands = await oa.resolve_doi("10.31235/osf.io/yvzcb", c)
        assert cands[0].url == "https://osf.io/download/zz/"

    async def test_generic_chain_merges_and_sorts(self):
        c = FakeClient([
            ("api.unpaywall.org", FakeResp(json_data={
                "is_oa": True, "oa_status": "gold",
                "best_oa_location": {"url_for_pdf": "https://up.example/a.pdf"}, "oa_locations": []})),
            ("api.openalex.org", FakeResp(json_data={
                "open_access": {"oa_url": "https://up.example/a.pdf"}, "locations": []})),  # dup URL
            ("api.core.ac.uk", FakeResp(json_data={"downloadUrl": "https://repo.example/z.pdf"})),
        ])
        cands = await oa.resolve_doi("10.1/x", c)
        urls = [c_.url for c_ in cands]
        assert urls[0] == "https://up.example/a.pdf"  # unpaywall (lower score) first
        assert urls.count("https://up.example/a.pdf") == 1  # deduped across sources
        assert "https://repo.example/z.pdf" in urls  # core present, ranked last


class TestResolveTitle:
    async def test_title_to_doi_then_chain(self, monkeypatch):
        c = FakeClient([
            ("api.openalex.org/works?filter=title.search", FakeResp(json_data={"results": [
                {"display_name": "Array programming with NumPy", "doi": "https://doi.org/10.1038/s41586-020-2649-2"}]})),
        ])

        async def fake_resolve_doi(doi, client):
            assert doi == "10.1038/s41586-020-2649-2"
            return [oa.Candidate(0, "https://nature.example/numpy.pdf", "unpaywall")]

        monkeypatch.setattr(oa, "resolve_doi", fake_resolve_doi)
        cands = await oa.resolve_title("Array programming with NumPy", c)
        assert cands[0].url == "https://nature.example/numpy.pdf"

    async def test_noisy_title_rejected(self, monkeypatch):
        c = FakeClient([
            ("api.openalex.org", FakeResp(json_data={"results": [
                {"display_name": "Completely unrelated work", "doi": "https://doi.org/10.1/junk"}]})),
            ("api.crossref.org", FakeResp(json_data={"message": {"items": [
                {"title": ["Another unrelated thing"], "DOI": "10.1/junk2"}]}})),
            ("export.arxiv.org", FakeResp(text="<feed></feed>")),  # no <id>
        ])
        # similarity gate rejects both → falls to arxiv (no match) → []
        assert await oa.resolve_title("Array programming with NumPy", c) == []


# ── book sources ─────────────────────────────────────────────────────────────────

class TestOapen:
    async def test_search_item_bitstream(self):
        c = FakeClient([
            ("library.oapen.org/rest/search", FakeResp(json_data=[{"uuid": "u1", "name": "Open Access"}])),
            ("library.oapen.org/rest/items/u1", FakeResp(json_data={"bitstreams": [
                {"bundleName": "THUMBNAIL", "mimeType": "image/jpeg", "retrieveLink": "/x.jpg"},
                {"bundleName": "ORIGINAL", "mimeType": "application/pdf", "retrieveLink": "/rest/bitstreams/b1/retrieve"},
            ]})),
        ])
        cands = await oa.from_oapen("9780262300988", c)
        assert cands[0].url == "https://library.oapen.org/rest/bitstreams/b1/retrieve"


class TestInternetArchive:
    async def test_public_pdf(self):
        c = FakeClient([
            ("openlibrary.org/isbn/", FakeResp(json_data={"ocaid": "frank00shel"})),
            ("archive.org/metadata/frank00shel", FakeResp(json_data={
                "metadata": {}, "files": [{"name": "frank00shel.pdf"}, {"name": "frank00shel_djvu.txt"}]})),
        ])
        cands = await oa.from_internetarchive("9780262300988", c)
        urls = [c_.url for c_ in cands]
        assert "https://archive.org/download/frank00shel/frank00shel.pdf" in urls

    async def test_lending_only_refused(self):
        c = FakeClient([
            ("openlibrary.org/isbn/", FakeResp(json_data={"ocaid": "lend01"})),
            ("archive.org/metadata/lend01", FakeResp(json_data={
                "metadata": {"access-restricted-item": "true"}, "files": [{"name": "lend01.pdf"}]})),
        ])
        assert await oa.from_internetarchive("9780262033848", c) == []  # legality gate


class TestGutendex:
    async def test_title_to_text(self):
        c = FakeClient([("gutendex.com", FakeResp(json_data={"results": [{
            "media_type": "Text", "copyright": False, "title": "Frankenstein",
            "formats": {"text/html": "https://gutenberg.example/84.html",
                        "text/plain; charset=utf-8": "https://gutenberg.example/84.txt"},
        }]}))])
        cands = await oa.from_gutendex("frankenstein shelley", c)
        assert cands[0].url == "https://gutenberg.example/84.html"  # prefers html

    async def test_isbn_skipped(self):
        assert await oa.from_gutendex("9780262300988", FakeClient([])) == []


class TestDoab:
    async def test_downloadurl_that_is_doi_expands(self, monkeypatch):
        c = FakeClient([
            ("find-by-metadata-field", FakeResp(json_data=[{"uuid": "d1"}])),
            ("directory.doabooks.org/rest/items/d1", FakeResp(json_data={"bitstreams": [
                {"bundleName": "THUMBNAIL", "metadata": [
                    {"key": "oapen.identifier.downloadUrl", "value": "https://doi.org/10.1017/9781009072854"}]}]})),
        ])

        async def fake_resolve_doi(doi, client):
            assert doi == "10.1017/9781009072854"
            return [oa.Candidate(10, "https://pub.example/book.pdf", "unpaywall")]

        monkeypatch.setattr(oa, "resolve_doi", fake_resolve_doi)
        cands = await oa.from_doab("9783110730234", c)
        assert cands[0].url == "https://pub.example/book.pdf"


class TestGoogleBooks:
    async def test_no_key_returns_empty(self, monkeypatch):
        monkeypatch.setattr(oa, "GOOGLE_BOOKS_API_KEY", "")
        assert await oa.from_googlebooks("9780000000000", FakeClient([])) == []


class TestResolveBook:
    async def test_isbn_aggregates_sources(self, monkeypatch):
        async def fake_oapen(q, c):
            return [oa.Candidate(8, "https://oapen.example/x.pdf", "oapen")]

        async def empty(q, c):
            return []

        monkeypatch.setattr(oa, "from_oapen", fake_oapen)
        monkeypatch.setattr(oa, "from_internetarchive", empty)
        monkeypatch.setattr(oa, "from_gutendex", empty)
        monkeypatch.setattr(oa, "from_doab", empty)
        monkeypatch.setattr(oa, "from_googlebooks", empty)
        cands = await oa.resolve_book("9780262300988", FakeClient([]))
        assert cands[0].url == "https://oapen.example/x.pdf"


# ── dispatch input routing (hybrid interface) ────────────────────────────────────

class TestDispatchRouting:
    @pytest.fixture(autouse=True)
    def capture_resolve(self, monkeypatch):
        self.captured = []

        async def fake_resolve_input(kind, value, key, ua, proxy):
            self.captured.append((kind, value))
            return {"method": f"oa:{kind}", "body": "x", "content_chars": 1,
                    "md_path": "/x", "cache_status": "miss"}

        monkeypatch.setattr(dispatch, "_resolve_input", fake_resolve_input)

    async def test_title_prefix_routes_to_find(self):
        r = await dispatch.get_or_fetch('title:"Array programming with NumPy"', "ua", None)
        assert "error" in r and "findworks" in r["error"].lower()
        assert self.captured == []  # a title is NOT fetched/resolved here

    async def test_isbn_prefix(self):
        await dispatch.get_or_fetch("isbn:978-0-262-30098-8", "ua", None)
        assert self.captured == [("isbn", "9780262300988")]  # normalized (hyphens stripped)

    async def test_invalid_isbn_prefix_rejected(self):
        r = await dispatch.get_or_fetch("isbn:0000000000", "ua", None)
        assert "error" in r and "ISBN" in r["error"]
        assert self.captured == []

    async def test_bare_isbn(self):
        await dispatch.get_or_fetch("9780262300988", "ua", None)
        assert self.captured == [("isbn", "9780262300988")]

    async def test_bare_title_routes_to_find(self):
        r = await dispatch.get_or_fetch("The Selfish Gene by Dawkins", "ua", None)
        assert "error" in r and "findworks" in r["error"].lower()
        assert self.captured == []

    async def test_existing_file_is_not_a_title(self, tmp_path, monkeypatch):
        monkeypatch.setenv("WEBFETCH_DIR", str(tmp_path / ".cache"))
        f = tmp_path / "real document.txt"
        f.write_text("hello")
        await dispatch.get_or_fetch(str(f), "ua", None)
        assert self.captured == []  # routed as a local file, NOT a title

    async def test_path_like_not_a_title(self):
        # a non-existent path-looking string is NOT sent to the title resolver
        await dispatch.get_or_fetch("./some/missing.pdf", "ua", None)
        assert self.captured == []

    async def test_image_redirects_under_media_deny(self):
        r = await dispatch.get_or_fetch("https://x.example/fig.png", "ua", None, media="deny")
        assert "error" in r and "fetchImage" in r["error"]
        assert self.captured == []

    async def test_archive_redirects_under_media_deny(self):
        r = await dispatch.get_or_fetch("https://x.example/data.zip", "ua", None, media="deny")
        assert "error" in r and "archive" in r["error"]

    async def test_doc_not_redirected_under_media_deny(self, monkeypatch):
        # a PDF is a document → media=deny must NOT redirect it
        async def fake_doc(src, key, kind, local, ua, proxy):
            return {"method": "pdf", "body": "x", "content_chars": 1, "md_path": "/x",
                    "cache_status": "miss"}
        monkeypatch.setattr(dispatch, "_doc_result", fake_doc)
        r = await dispatch.get_or_fetch("https://x.example/p.pdf", "ua", None, media="deny")
        assert "error" not in r and r["method"] == "pdf"


# ── find: title/free-text → ranked candidate works (the scholarly WebSearch) ─────

class TestFindWorks:
    async def test_papers_from_openalex(self):
        c = FakeClient([
            ("api.openalex.org/works?filter=title.search", FakeResp(json_data={"results": [{
                "display_name": "A Mathematical Theory of Communication",
                "doi": "https://doi.org/10.1002/x", "publication_year": 1948,
                "open_access": {"oa_status": "closed", "oa_url": None},
                "authorships": [{"author": {"display_name": "C. E. Shannon"}}]}]})),
            ("openlibrary.org/search.json", FakeResp(json_data={"docs": []})),
            ("gutendex.com", FakeResp(json_data={"results": []})),
        ])
        cands = await oa.find_works("A Mathematical Theory of Communication", c)
        assert cands[0]["kind"] == "paper"
        assert cands[0]["fetch"] == "10.1002/x"
        assert cands[0]["authors"] == "C. E. Shannon" and cands[0]["year"] == 1948

    async def test_books_from_openlibrary_and_gutenberg(self):
        c = FakeClient([
            ("api.openalex.org", FakeResp(json_data={"results": []})),
            ("openlibrary.org/search.json", FakeResp(json_data={"docs": [{
                "title": "Frankenstein", "author_name": ["Mary Shelley"],
                "first_publish_year": 1818, "isbn": ["9780000000000"], "ebook_access": "public"}]})),
            ("gutendex.com", FakeResp(json_data={"results": [{
                "title": "Frankenstein", "copyright": False, "authors": [{"name": "Shelley, Mary"}],
                "formats": {"text/html": "https://gutenberg.example/84.html"}}]})),
        ])
        handles = [x["fetch"] for x in await oa.find_works("Frankenstein", c)]
        assert "isbn:9780000000000" in handles
        assert "https://gutenberg.example/84.html" in handles

    async def test_ranked_by_match(self):
        c = FakeClient([
            ("api.openalex.org", FakeResp(json_data={"results": [
                {"display_name": "Something else entirely", "doi": "https://doi.org/10.1234/b",
                 "open_access": {}, "authorships": []},
                {"display_name": "Entropy and Information", "doi": "https://doi.org/10.1234/a",
                 "open_access": {}, "authorships": []}]})),
            ("openlibrary.org", FakeResp(json_data={"docs": []})),
            ("gutendex.com", FakeResp(json_data={"results": []})),
        ])
        cands = await oa.find_works("Entropy and Information", c)
        assert cands[0]["fetch"] == "10.1234/a"  # best title match ranked first

    async def test_one_source_failing_does_not_sink_the_rest(self):
        c = FakeClient([
            ("api.openalex.org", FakeResp(status=500)),  # papers fail
            ("openlibrary.org/search.json", FakeResp(json_data={"docs": [{
                "title": "X", "isbn": ["9781111111111"], "author_name": ["A"]}]})),
            ("gutendex.com", FakeResp(json_data={"results": []})),
        ])
        cands = await oa.find_works("X", c)
        assert cands and cands[0]["fetch"] == "isbn:9781111111111"


class TestArxivByTitleGate:
    async def test_rejects_fuzzy_substring_match(self):
        # arXiv ti: search returns a paper whose title merely CONTAINS the query → must NOT substitute
        feed = ("<entry><title>Complexity and Second Moment of The Mathematical Theory of "
                "Communication</title><id>http://arxiv.org/abs/2101.00001v1</id></entry>")
        c = FakeClient([("export.arxiv.org", FakeResp(text=feed))])
        assert await oa._arxiv_by_title("A Mathematical Theory of Communication", c) == []

    async def test_accepts_exact_match(self):
        feed = ("<entry><title>Attention Is All You Need</title>"
                "<id>http://arxiv.org/abs/1706.03762v7</id></entry>")
        c = FakeClient([("export.arxiv.org", FakeResp(text=feed))])
        cands = await oa._arxiv_by_title("Attention Is All You Need", c)
        assert cands and cands[0].url == "https://arxiv.org/pdf/1706.03762v7"
