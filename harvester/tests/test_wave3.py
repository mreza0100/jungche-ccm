"""Tests for wave-perfection-3: reader redundancy + PMC OA-service rescue.

* net.fetch_defuddle — second keyless reader rung (frontmatter envelope stripped)
* mirror.pmc_oa_pdf_url — NCBI oa.fcgi ftp→https/deprecated/ rewrite (their own API
  hands out dead links)
* dispatch._pmcid_to_result — PMC OA-service PDF fallback after Europe PMC fails
* oa.arxiv_candidates — ar5iv HTML insurance copy behind the direct PDF
"""

from harvester import convert, dispatch, mirror, net, oa


# ── defuddle reader ──────────────────────────────────────────────────────────────

class TestDefuddleReader:
    def test_frontmatter_envelope_stripped(self):
        raw = '---\ntitle: "T"\nsource: "u"\nword_count: 17\n---\n\nBody line.\n'
        assert net._strip_reader_frontmatter(raw).strip() == "Body line."

    def test_body_without_envelope_passes_through(self):
        assert net._strip_reader_frontmatter("# Just markdown") == "# Just markdown"

    async def test_rung_order_jina_then_defuddle_then_browser(self, monkeypatch):
        """Ladder order: chrome-impersonation → jina → defuddle → browser. Defuddle winning
        must stop the ladder before the browser rung fires."""
        order: list[str] = []

        async def fake_html_bytes(url, ua, proxy):
            return b"<html></html>", 200, None, "text/html"

        async def fake_imp(url, proxy):
            order.append("chrome")
            return "", None

        async def fake_jina(url, ua, proxy=None):
            order.append("jina")
            return ""

        async def fake_defuddle(url, ua, proxy=None):
            order.append("defuddle")
            return "defuddled body " * 120

        async def fake_browser(url, proxy=None, timeout_ms=45000):
            order.append("browser")
            return "", None

        monkeypatch.setattr(net, "fetch_bytes_with_meta", fake_html_bytes)
        monkeypatch.setattr(net, "fetch_impersonated", fake_imp)
        monkeypatch.setattr(net, "fetch_jina", fake_jina)
        monkeypatch.setattr(net, "fetch_defuddle", fake_defuddle)
        monkeypatch.setattr(net, "fetch_browser", fake_browser)

        res = await dispatch.get_or_fetch("https://walled2.example/page", "ua")
        assert order[:3] == ["chrome", "jina", "defuddle"], f"got {order}"
        assert "browser" not in order, "a defuddle win must make the browser rung unnecessary"
        assert res["method"] == "defuddle-reader"
        assert "error" not in res


# ── PMC OA service (oa.fcgi) rescue ─────────────────────────────────────────────

_OAFCGI_XML = (
    '<OA><request id="PMC10450651"><records returned-count="1">'
    '<record id="PMC10450651"><link format="tgz" updated="2025-04-04" '
    'href="ftp://ftp.ncbi.nlm.nih.gov/pub/pmc/oa_package/67/4a/PMC10450651.tar.gz"/>'
    '<link format="pdf" updated="2025-04-04" '
    'href="ftp://ftp.ncbi.nlm.nih.gov/pub/pmc/oa_pdf/b8/d6/pnas.202302738.PMC10450651.pdf"/>'
    "</record></records></OA>"
)


class _XmlResp:
    def __init__(self, text):
        self.text = text
        self.status_code = 200


class _XmlClient:
    async def get(self, url, **kw):
        return _XmlResp(_OAFCGI_XML)


class TestPmcOaPdfUrl:
    async def test_rewrites_dead_ftp_to_live_deprecated_https(self):
        url = await mirror.pmc_oa_pdf_url("PMC10450651", _XmlClient())
        assert url == ("https://ftp.ncbi.nlm.nih.gov/pub/pmc/deprecated/"
                       "oa_pdf/b8/d6/pnas.202302738.PMC10450651.pdf"), (
            "the ftp href NCBI hands out is DEAD; only the deprecated/ https path works")

    async def test_no_pdf_link_returns_none(self):
        class C:
            async def get(self, url, **kw):
                return _XmlResp('<OA><records><record id="PMC1"/></records></OA>')
        assert await mirror.pmc_oa_pdf_url("PMC1", C()) is None


class TestPmcidOaFallback:
    async def test_europepmc_fail_falls_through_to_oa_service(self, monkeypatch):
        """Europe PMC render endpoint dead → oa.fcgi direct PDF rescues the PMCID."""
        order: list[str] = []

        async def fake_epmc_pdf(pmcid, client):
            order.append("europepmc")
            return b""  # Europe PMC has nothing today

        async def fake_oa_url(pmcid, client):
            order.append("oa-fcgi")
            return "https://ftp.ncbi.nlm.nih.gov/pub/pmc/deprecated/x.pdf"

        async def fake_download(url, ua, proxy):
            return b"%PDF-1.7 real article bytes", 200, None

        def fake_pdf_to_md(path, force_ocr=False):
            return "PNAS ARTICLE " * 120

        monkeypatch.setattr(mirror, "europepmc_pdf", fake_epmc_pdf)
        monkeypatch.setattr(mirror, "pmc_oa_pdf_url", fake_oa_url)
        monkeypatch.setattr(net, "download_bytes", fake_download)
        monkeypatch.setattr(convert, "pdf_to_md", fake_pdf_to_md)

        res = await dispatch._pmcid_to_result("PMC10450651", "10.9999/fake", "ua", None)
        assert order == ["europepmc", "oa-fcgi"], f"got {order}"
        assert res and res["method"] == "mirror:pmc-oa-pdf"
        assert "PNAS ARTICLE" in res["body"]
        # no Europe PMC figures note on a non-Europe-PMC copy
        assert "Figures available as a ZIP" not in res["body"]


# ── ar5iv insurance candidate ────────────────────────────────────────────────────

class TestArxivAr5iv:
    def test_arxiv_candidates_include_ar5iv_behind_pdf(self):
        cands = oa.arxiv_candidates("1706.03762")
        assert [c.url for c in cands] == [
            "https://arxiv.org/pdf/1706.03762",
            "https://ar5iv.labs.arxiv.org/html/1706.03762",
        ]
        assert cands[0].kind_hint == "pdf" and cands[1].kind_hint == "html"
        assert cands[0].priority <= cands[1].priority, "PDF must be tried first"
