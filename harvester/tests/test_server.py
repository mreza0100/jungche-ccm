"""Tests for the customized fetch MCP server (trafilatura + on-disk cache)."""

import hashlib
import json
import math
import zipfile
from contextlib import asynccontextmanager
from pathlib import Path

import anyio
import pytest
from mcp.client.session import ClientSession
from mcp.shared.memory import create_client_server_memory_streams
from mcp.types import TextContent
from pydantic import ValidationError

import harvester.cache as cache_mod
import harvester.server as srv
from harvester import convert, detect, dispatch, images, net, oa
from harvester.cache import split_frontmatter
from harvester.describe import describe_fetch_result, describe_size_result
from harvester.tokens import estimate_tokens
from harvester.detect import _sniff_kind
from harvester.html import (
    extract_content_from_html,
    extract_metadata_block,
    tidy_markdown,
)
from harvester.net import (
    get_robots_txt_url,
    is_private_host,
    looks_like_challenge,
)
from harvester.server import Archive, FetchImage, Fetch, FindWorks, Search


# ── fixtures ──────────────────────────────────────────────────────────────────

@pytest.fixture(autouse=True)
def isolated_cache(tmp_path, monkeypatch):
    """Redirect the cache to a per-test tmp dir so no test touches the real cache dir."""
    fetch_dir = tmp_path / ".cache"
    fetch_dir.mkdir()
    monkeypatch.setenv("WEBFETCH_DIR", str(fetch_dir))
    return fetch_dir


# ── shared result builder ─────────────────────────────────────────────────────

def _result(body: str = "", *, cache_status: str = "miss", method: str = "local-trafilatura",
            http_status: int | None = 200, error_kind: str | None = None,
            challenge: bool = False, bytes: int = 123,
            content_chars: int | None = None) -> dict:
    """Build a get_or_fetch-style result dict for describe_fetch_result tests."""
    return {
        "cache_status": cache_status,
        "method": method,
        "md_path": Path("/tmp/x.md"),
        "body": body,
        "bytes": bytes,
        "content_chars": content_chars,
        "http_status": http_status,
        "error_kind": error_kind,
        "challenge": challenge,
    }


# ── detect.detect_kind ───────────────────────────────────────────────────────

class TestDetectKind:
    """detect_kind infers the processing route from URL/path extension."""

    def test_pdf(self):
        assert detect.detect_kind("https://example.com/paper.pdf") == "pdf"

    def test_docx(self):
        assert detect.detect_kind("/tmp/report.docx") == "docx"

    def test_xlsx(self):
        assert detect.detect_kind("https://x.com/data.xlsx?v=1") == "xlsx"

    def test_pptx(self):
        assert detect.detect_kind("file:///tmp/slides.pptx") == "pptx"

    def test_csv(self):
        assert detect.detect_kind("/tmp/data.csv") == "csv"

    def test_json(self):
        assert detect.detect_kind("https://api.example.com/v1/items.json") == "json"

    def test_zip(self):
        assert detect.detect_kind("/tmp/data.zip") == "zip"

    def test_7z(self):
        assert detect.detect_kind("https://x.com/archive.7z") == "7z"

    def test_rar(self):
        assert detect.detect_kind("/tmp/backup.rar") == "rar"

    def test_tar(self):
        assert detect.detect_kind("/tmp/src.tar") == "tar"

    def test_tar_gz(self):
        assert detect.detect_kind("/tmp/src.tar.gz") == "tar"

    def test_tgz(self):
        assert detect.detect_kind("https://example.com/release.tgz") == "tar"

    def test_tar_bz2(self):
        assert detect.detect_kind("/tmp/src.tar.bz2") == "tar"

    def test_tar_xz(self):
        assert detect.detect_kind("/tmp/src.tar.xz") == "tar"

    def test_txz(self):
        assert detect.detect_kind("/tmp/src.txz") == "tar"

    def test_png(self):
        assert detect.detect_kind("https://cdn.example.com/fig1.png") == "image"

    def test_jpg(self):
        assert detect.detect_kind("https://cdn.example.com/photo.jpg") == "image"

    def test_gif(self):
        assert detect.detect_kind("/tmp/anim.gif") == "image"

    def test_svg(self):
        assert detect.detect_kind("https://cdn.example.com/logo.svg") == "image"

    def test_html_extension(self):
        assert detect.detect_kind("https://example.com/page.html") == "html"

    def test_htm_extension(self):
        assert detect.detect_kind("https://example.com/page.htm") == "html"

    def test_extensionless_defaults_html(self):
        assert detect.detect_kind("https://arxiv.org/pdf/1706.03762") == "html"

    def test_query_stripped_before_ext_check(self):
        assert detect.detect_kind("https://example.com/doc.pdf?token=abc") == "pdf"

    def test_fragment_stripped_before_ext_check(self):
        assert detect.detect_kind("https://example.com/page.html#section") == "html"

    def test_trailing_slash_defaults_html(self):
        assert detect.detect_kind("https://example.com/section/") == "html"


# ── detect.deny_reason ───────────────────────────────────────────────────────

class TestDenyReason:
    """deny_reason refuses credentials/keys; leaves ordinary files alone."""

    # --- files that MUST be refused ---

    def test_refuses_dot_env(self, tmp_path):
        p = tmp_path / ".env"
        assert detect.deny_reason(p) is not None

    def test_refuses_dot_env_local(self, tmp_path):
        p = tmp_path / ".env.local"
        assert detect.deny_reason(p) is not None

    def test_refuses_pem(self, tmp_path):
        p = tmp_path / "server.pem"
        assert detect.deny_reason(p) is not None

    def test_refuses_key_file(self, tmp_path):
        p = tmp_path / "mykey.key"
        assert detect.deny_reason(p) is not None

    def test_refuses_id_rsa(self, tmp_path):
        p = tmp_path / "id_rsa"
        assert detect.deny_reason(p) is not None

    def test_refuses_id_ed25519(self, tmp_path):
        p = tmp_path / "id_ed25519"
        assert detect.deny_reason(p) is not None

    def test_refuses_inside_ssh_dir(self, tmp_path):
        d = tmp_path / ".ssh"
        d.mkdir()
        p = d / "config"
        assert detect.deny_reason(p) is not None

    def test_refuses_inside_aws_dir(self, tmp_path):
        d = tmp_path / ".aws"
        d.mkdir()
        p = d / "credentials"
        assert detect.deny_reason(p) is not None

    def test_refuses_private_key_marker_in_name(self, tmp_path):
        p = tmp_path / "deploy_rsa"
        assert detect.deny_reason(p) is not None

    # --- confinement: system roots + sensitive locations (LFI hardening) ---

    def test_refuses_proc_self_environ(self):
        # /proc/self/environ leaks the harvester process's whole env (API keys/proxy creds).
        assert detect.deny_reason(Path("/proc/self/environ")) is not None

    def test_refuses_etc_passwd(self):
        assert detect.deny_reason(Path("/etc/passwd")) is not None

    def test_refuses_sys_and_dev(self):
        assert detect.deny_reason(Path("/sys/kernel/notes")) is not None
        assert detect.deny_reason(Path("/dev/mem")) is not None

    def test_refuses_ssh_id_rsa_under_home(self):
        assert detect.deny_reason(Path("~/.ssh/id_rsa")) is not None

    def test_refuses_config_gcloud(self):
        # ~/.config holds gcloud / gh credentials — the working LFI exfil target.
        assert detect.deny_reason(Path("~/.config/gcloud/credentials.db")) is not None

    def test_refuses_symlink_into_etc(self, tmp_path):
        # A symlink with an innocent name must not smuggle a system secret past the name checks.
        link = tmp_path / "report.pdf"
        try:
            link.symlink_to("/etc/passwd")
        except OSError:
            import pytest as _pytest
            _pytest.skip("symlinks unsupported on this platform")
        assert detect.deny_reason(link) is not None

    # --- files that MUST be allowed ---

    def test_allows_ordinary_docx(self, tmp_path):
        p = tmp_path / "loginov_report.docx"
        assert detect.deny_reason(p) is None

    def test_allows_pdf_with_report_name(self, tmp_path):
        p = tmp_path / "loginov_report.pdf"
        assert detect.deny_reason(p) is None

    def test_allows_ordinary_txt(self, tmp_path):
        p = tmp_path / "notes.txt"
        assert detect.deny_reason(p) is None

    def test_allows_markdown_file(self, tmp_path):
        p = tmp_path / "README.md"
        assert detect.deny_reason(p) is None

    def test_allows_file_literally_named_pdf(self, tmp_path):
        # a file whose name contains "pdf" as a substring in a safe position
        p = tmp_path / "x.docx"
        assert detect.deny_reason(p) is None


# ── detect.sniff_magic ───────────────────────────────────────────────────────

class TestSniffMagic:
    """sniff_magic identifies file type from magic bytes."""

    def test_pdf(self):
        assert detect.sniff_magic(b"%PDF-1.7 ....") == "pdf"

    def test_zip(self):
        assert detect.sniff_magic(b"PK\x03\x04" + b"\x00" * 12) == "zip"

    def test_7z(self):
        assert detect.sniff_magic(b"7z\xbc\xaf\x27\x1c" + b"\x00" * 10) == "7z"

    def test_rar(self):
        assert detect.sniff_magic(b"Rar!\x1a\x07" + b"\x00" * 10) == "rar"

    def test_gzip(self):
        assert detect.sniff_magic(b"\x1f\x8b" + b"\x00" * 14) == "tar"

    def test_jpeg(self):
        assert detect.sniff_magic(b"\xff\xd8\xff\xe0" + b"\x00" * 12) == "image"

    def test_png(self):
        assert detect.sniff_magic(b"\x89PNG\r\n\x1a\n" + b"\x00" * 8) == "image"

    def test_gif(self):
        assert detect.sniff_magic(b"GIF89a" + b"\x00" * 10) == "image"

    def test_unknown_returns_none(self):
        assert detect.sniff_magic(b"<html><head>") is None

    def test_too_short_returns_none(self):
        assert detect.sniff_magic(b"\x00\x01") is None


# ── convert.json_to_md ───────────────────────────────────────────────────────

class TestJsonToMd:
    """json_to_md wraps valid JSON in a fenced block and pretty-prints it."""

    def test_valid_object_is_pretty_printed(self):
        md = convert.json_to_md('{"b":2,"a":1}')
        assert md.startswith("```json\n")
        assert md.strip().endswith("```")
        # pretty-printed: keys are indented
        assert "  " in md

    def test_valid_array_is_fenced(self):
        md = convert.json_to_md("[1,2,3]")
        assert "```json" in md
        assert "1" in md

    def test_invalid_json_passed_through_raw(self):
        raw = "not valid json {{"
        md = convert.json_to_md(raw)
        assert "```json" in md
        assert raw in md

    def test_empty_object(self):
        md = convert.json_to_md("{}")
        assert "```json" in md
        assert "{}" in md


# ── _sniff_kind ───────────────────────────────────────────────────────────────

class TestSniffKind:
    """_sniff_kind picks the true kind from Content-Type + magic bytes."""

    def test_pdf_ct_returns_pdf(self):
        assert _sniff_kind("application/pdf", b"%PDF-") == "pdf"

    def test_pdf_magic_no_ct_returns_pdf(self):
        assert _sniff_kind("", b"%PDF-1.4 ....") == "pdf"

    def test_html_ct_returns_none(self):
        assert _sniff_kind("text/html; charset=utf-8", b"<html>") is None

    def test_text_plain_ct_returns_none(self):
        assert _sniff_kind("text/plain", b"hello world") is None

    def test_docx_ct_returns_docx(self):
        ct = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
        assert _sniff_kind(ct, b"PK\x03\x04") == "docx"

    def test_xlsx_ct_returns_xlsx(self):
        ct = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
        assert _sniff_kind(ct, b"PK\x03\x04") == "xlsx"

    def test_pptx_ct_returns_pptx(self):
        ct = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
        assert _sniff_kind(ct, b"PK\x03\x04") == "pptx"

    def test_zip_ct_returns_zip(self):
        assert _sniff_kind("application/zip", b"PK\x03\x04") == "zip"

    def test_7z_ct_returns_7z(self):
        assert _sniff_kind("application/x-7z-compressed", b"7z\xbc\xaf") == "7z"

    def test_image_ct_returns_image(self):
        assert _sniff_kind("image/png", b"\x89PNG") == "image"

    def test_unknown_ct_falls_back_to_magic(self):
        assert _sniff_kind("application/octet-stream", b"%PDF-1.6") == "pdf"

    def test_unknown_ct_no_magic_returns_none(self):
        assert _sniff_kind("application/octet-stream", b"\x00\x01\x02") is None


# ── content-sniff routing fix ─────────────────────────────────────────────────

class TestContentSniffRouting:
    """Extensionless PDF URLs (e.g. arxiv) must route to pdf:pymupdf4llm, not jina-reader."""

    async def test_pdf_bytes_route_to_pdf_converter(self, monkeypatch, tmp_path):
        """When fetch_bytes_with_meta returns PDF bytes for an extensionless URL,
        _html_result must hand off to _doc_result instead of trafilatura/Jina."""
        pdf_bytes = b"%PDF-1.4 fake\n%%EOF"

        async def fake_fetch_bytes_with_meta(url, ua, proxy=None):
            return pdf_bytes, 200, None, "application/pdf"

        captured = {}

        async def fake_doc_result(src, key, kind, local, ua, proxy):
            captured["kind"] = kind
            captured["local"] = local
            captured["key"] = key
            return {
                "cache_status": "miss", "method": "pdf:pymupdf4llm",
                "md_path": tmp_path / "out.md", "body": "# Attention Is All You Need\n\nContent.",
                "bytes": 100, "content_chars": 40, "http_status": None,
                "error_kind": None, "challenge": False,
            }

        monkeypatch.setattr(net, "fetch_bytes_with_meta", fake_fetch_bytes_with_meta)
        monkeypatch.setattr(dispatch, "_doc_result", fake_doc_result)

        result = await dispatch._html_result(
            "https://arxiv.org/pdf/1706.03762",
            "https://arxiv.org/pdf/1706.03762",
            False,
            "test-ua",
            None,
        )

        assert captured.get("kind") == "pdf", f"expected kind=pdf, got {captured.get('kind')!r}"
        assert captured.get("local") is True  # bytes were pre-saved, handler uses local path
        assert result["method"] == "pdf:pymupdf4llm"

    async def test_html_url_stays_on_html_path(self, monkeypatch, tmp_path):
        """A normal HTML page must NOT be re-routed — trafilatura stays in charge."""
        html_bytes = b"<html><head><title>Black hole</title></head><body><p>" + b"A" * 600 + b"</p></body></html>"

        async def fake_fetch_bytes_with_meta(url, ua, proxy=None):
            return html_bytes, 200, None, "text/html; charset=utf-8"

        jina_called = []

        async def fake_jina(url, ua, proxy=None):
            jina_called.append(url)
            return ""

        monkeypatch.setattr(net, "fetch_bytes_with_meta", fake_fetch_bytes_with_meta)
        monkeypatch.setattr(net, "fetch_jina", fake_jina)
        monkeypatch.setattr(images, "localize_html_images", lambda md, *a, **kw: __import__("asyncio").coroutine(lambda: md)())

        async def noop_localise(md, base_url, ua, proxy=None):
            return md

        monkeypatch.setattr(images, "localize_html_images", noop_localise)

        result = await dispatch._html_result(
            "https://en.wikipedia.org/wiki/Black_hole",
            "https://en.wikipedia.org/wiki/Black_hole",
            False,
            "test-ua",
            None,
        )
        # Method must be trafilatura-based, NOT a binary converter
        assert result["method"] in ("local-trafilatura", "jina-reader"), (
            f"expected html extraction method, got {result['method']!r}"
        )
        assert "pdf:pymupdf4llm" not in result["method"]


# ── archive :: member dispatch ─────────────────────────────────────────────────

class TestArchiveMemberDispatch:
    """The `archive::member` syntax fetches a single file from a local archive."""

    async def test_zip_member_json_dispatched(self, tmp_path):
        arc = tmp_path / "x.zip"
        with zipfile.ZipFile(arc, "w") as zf:
            zf.writestr("inner.json", '{"hello": "world"}')

        result = await dispatch.get_or_fetch(f"{arc}::inner.json", "test-ua", None)
        assert "error" not in result, f"unexpected error: {result.get('error')}"
        body = result.get("body", "")
        assert "hello" in body or "world" in body

    async def test_zip_member_txt_dispatched(self, tmp_path):
        arc = tmp_path / "docs.zip"
        with zipfile.ZipFile(arc, "w") as zf:
            zf.writestr("readme.txt", "Hello from archive!")

        result = await dispatch.get_or_fetch(f"{arc}::readme.txt", "test-ua", None)
        assert "error" not in result
        assert "Hello from archive!" in result.get("body", "")

    async def test_zip_listing_without_member(self, tmp_path):
        arc = tmp_path / "y.zip"
        with zipfile.ZipFile(arc, "w") as zf:
            zf.writestr("file1.txt", "content")
            zf.writestr("file2.txt", "more")

        result = await dispatch.get_or_fetch(str(arc), "test-ua", None)
        assert "error" not in result
        body = result.get("body", "")
        assert "file1.txt" in body
        assert "file2.txt" in body

    async def test_listing_teaches_archive_tool_call_not_double_colon(self, tmp_path):
        """The listing must teach `archive(source=..., member=...)` — the form `fetch` accepts
        — never the internal `source::member` addressing, which `fetch` refuses outright."""
        arc = tmp_path / "z.zip"
        with zipfile.ZipFile(arc, "w") as zf:
            zf.writestr("a.txt", "hi")

        result = await dispatch.get_or_fetch(str(arc), "test-ua", None)
        body = result.get("body", "")
        assert "archive(source=" in body and "member=" in body
        assert f"{arc}::" not in body


# ── empty-input error must not advertise the always-failing title:"..." form ────

class TestEmptyInputError:
    """`fetch("")` must point at `findWorks` for titles, not advertise `title:"..."` — that
    form always errors (titles are ambiguous; `fetch` never guesses)."""

    async def test_empty_string_points_at_findworks_not_title_syntax(self):
        result = await dispatch.get_or_fetch("", "test-ua", None)
        assert "error" in result
        assert "findWorks" in result["error"]
        assert 'title:"' not in result["error"]

    async def test_whitespace_only_points_at_findworks_not_title_syntax(self):
        result = await dispatch.get_or_fetch("   ", "test-ua", None)
        assert "error" in result
        assert "findWorks" in result["error"]
        assert 'title:"' not in result["error"]


# ── _image_result ──────────────────────────────────────────────────────────────

class TestImageResult:
    """_image_result for a local image returns a body referencing the local path."""

    async def test_local_png_body_references_path(self, tmp_path):
        img = tmp_path / "diagram.png"
        img.write_bytes(b"\x89PNG\r\n\x1a\n" + b"\x00" * 100)

        result = await dispatch._image_result(str(img), str(img), True, "test-ua", None)
        assert "error" not in result
        body = result.get("body", "")
        assert str(img) in body
        assert result["method"] == "image"

    async def test_local_jpg_body_references_path(self, tmp_path):
        img = tmp_path / "photo.jpg"
        img.write_bytes(b"\xff\xd8\xff\xe0" + b"\x00" * 100)

        result = await dispatch._image_result(str(img), str(img), True, "test-ua", None)
        body = result.get("body", "")
        assert str(img) in body

    async def test_image_result_content_chars_over_threshold(self, tmp_path):
        # content_chars must be above THIN_MIN_CHARS so describe_fetch_result doesn't flag it
        img = tmp_path / "fig.png"
        img.write_bytes(b"\x89PNG\r\n\x1a\n" + b"\x00" * 100)

        result = await dispatch._image_result(str(img), str(img), True, "test-ua", None)
        assert result["content_chars"] > cache_mod.THIN_MIN_CHARS


# ── localize_html_images ──────────────────────────────────────────────────────

class TestLocalizeHtmlImages:
    """localize_html_images rewrites remote image URLs to local paths in markdown."""

    async def test_rewrites_image_url_to_local_path(self, monkeypatch, tmp_path):
        """When download succeeds, the remote URL in `![](url)` is replaced by a local path."""
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
                return FakeResponse()

        import httpx
        monkeypatch.setattr(httpx, "AsyncClient", lambda **kw: FakeClient())

        md = "![fig](https://example.com/fig.png)\n\nSome text."
        result = await images.localize_html_images(md, "https://example.com/page", "test-ua")
        # The remote URL should be replaced by a local filesystem path
        assert "https://example.com/fig.png" not in result
        assert "![fig](" in result
        # The local path must be absolute
        rewritten = result.split("![fig](")[1].split(")")[0]
        assert rewritten.startswith("/")

    async def test_data_uri_skipped(self, monkeypatch):
        """data: URIs are never downloaded — left untouched."""
        calls = []

        class FakeClient:
            async def __aenter__(self):
                return self

            async def __aexit__(self, *_):
                pass

            async def get(self, url, **kw):
                calls.append(url)
                raise Exception("should not be called")

        import httpx
        monkeypatch.setattr(httpx, "AsyncClient", lambda **kw: FakeClient())

        data_uri = "data:image/png;base64,abc123=="
        md = f"![inline]({data_uri})\n\ntext"
        result = await images.localize_html_images(md, "https://example.com/", "test-ua")
        assert data_uri in result  # left untouched
        assert not calls

    async def test_no_images_returns_unchanged(self, monkeypatch):
        md = "# Just a heading\n\nNo images here."
        result = await images.localize_html_images(md, "https://example.com/", "test-ua")
        assert result == md


# ── cache path API ────────────────────────────────────────────────────────────

class TestCacheFile:
    """cache.cache_file produces the expected slug + extension under the kind subdir."""

    def test_slug_strips_scheme_and_sanitizes(self):
        url = "https://example.com/foo/bar?x=1"
        path = cache_mod.cache_file(url, "html", ".md")
        h = hashlib.sha1(url.encode("utf-8")).hexdigest()[:10]
        assert path.name == f"example.com_foo_bar_x_1__{h}.md"
        assert path.parent.name == "html"

    def test_distinct_urls_distinct_slugs(self):
        p1 = cache_mod.cache_file("https://example.com/a", "html", ".md")
        p2 = cache_mod.cache_file("https://example.com/b", "html", ".md")
        assert p1.name != p2.name

    def test_kind_creates_subdir(self, tmp_path):
        path = cache_mod.cache_file("https://x.com/doc.pdf", "pdf", ".md")
        assert path.parent.name == "pdf"
        assert path.parent.exists()

    def test_collapses_underscore_runs_in_slug(self):
        url = "http://a.b//c///?=="
        path = cache_mod.cache_file(url, "html", ".md")
        # No double underscores in the slug portion (before the hash suffix)
        slug_part = path.name.split("__")[0]
        assert "__" not in slug_part


# ── existing tests (unchanged from prior version) ─────────────────────────────

class TestExtractMetadataBlock:
    """extract_metadata_block recovers author/date/title the body extractor drops."""

    _HTML = (
        "<!DOCTYPE html><html><head>"
        "<title>A Study of Widgets</title>"
        '<meta name="author" content="Ada Lovelace">'
        '<meta name="citation_author" content="Ada Lovelace">'
        '<meta name="citation_author" content="Alan Turing">'
        '<meta name="citation_publication_date" content="2023-05-01">'
        '<meta property="og:site_name" content="Journal of Widgets">'
        "</head><body><article><p>Widgets are quite useful indeed, and so on.</p>"
        "</article></body></html>"
    )

    def test_recovers_author_and_title(self):
        block = extract_metadata_block(self._HTML)
        assert block, "expected a non-empty metadata block"
        assert "**Authors:**" in block
        assert "Lovelace" in block
        assert "**Title:**" in block
        assert block.endswith("---\n\n")

    def test_empty_html_yields_no_block(self):
        assert extract_metadata_block("") == ""

    def test_block_is_body_safe(self):
        block = extract_metadata_block(self._HTML)
        assert all(
            line.startswith("**") or line in ("", "---") for line in block.splitlines()
        )

    def test_junk_author_is_dropped(self):
        html = self._HTML.replace(
            '<meta name="author" content="Ada Lovelace">'
            '<meta name="citation_author" content="Ada Lovelace">'
            '<meta name="citation_author" content="Alan Turing">',
            '<meta name="author" content="Username">'
            '<meta name="citation_author" content="Username">',
        )
        block = extract_metadata_block(html)
        assert "**Title:**" in block
        assert "**Authors:**" not in block

    def test_real_author_with_junk_substring_is_kept(self):
        html = self._HTML.replace("Ada Lovelace", "Loginov, Ivan").replace("Alan Turing", "Loginov, Ivan")
        block = extract_metadata_block(html)
        assert "**Authors:**" in block
        assert "Loginov" in block


def test_describe_thin_guard_uses_content_chars():
    """A blocked page whose prepended metadata inflates the body must STILL be classified
    as a failure — the thin check reads content_chars, not the inflated body length."""
    inflated = "**Authors:** " + "x" * 600  # >500 chars, but it is all header
    out = describe_fetch_result(
        "https://blocked.example",
        _result(inflated, content_chars=10, challenge=True),
    )
    assert out.text.startswith("# https://blocked.example\nERROR")


async def _run_fetch_tool(monkeypatch, urls, fake_get_or_fetch, *, extra_args=None):
    """Drive the real `fetch` MCP tool end-to-end over in-memory streams, with
    get_or_fetch monkeypatched so no network is touched. Returns the content list.

    `extra_args` is merged into the tool call (e.g. {"size_only": True})."""
    monkeypatch.setattr(srv, "get_or_fetch", fake_get_or_fetch)
    async with create_client_server_memory_streams() as (client_streams, server_streams):
        client_read, client_write = client_streams
        server_read, server_write = server_streams

        @asynccontextmanager
        async def fake_stdio():
            yield server_read, server_write

        monkeypatch.setattr(srv, "stdio_server", fake_stdio)
        async with anyio.create_task_group() as tg:
            tg.start_soon(srv.serve)
            async with ClientSession(client_read, client_write) as session:
                await session.initialize()
                call_args = {"sources": urls, **(extra_args or {})}
                result = await session.call_tool("fetch", call_args)
            tg.cancel_scope.cancel()
        texts: list[str] = []
        for item in result.content:
            assert isinstance(item, TextContent)
            texts.append(item.text)
        return texts


class TestGetRobotsTxtUrl:
    """Tests for get_robots_txt_url function."""

    def test_simple_url(self):
        assert get_robots_txt_url("https://example.com/page") == "https://example.com/robots.txt"

    def test_url_with_path(self):
        assert (
            get_robots_txt_url("https://example.com/some/deep/path/page.html")
            == "https://example.com/robots.txt"
        )

    def test_url_with_query_params(self):
        assert (
            get_robots_txt_url("https://example.com/page?foo=bar&baz=qux")
            == "https://example.com/robots.txt"
        )

    def test_url_with_port(self):
        assert get_robots_txt_url("https://example.com:8080/page") == "https://example.com:8080/robots.txt"

    def test_http_url(self):
        assert get_robots_txt_url("http://example.com/page") == "http://example.com/robots.txt"


class TestExtractContentFromHtml:
    """Tests for the trafilatura-backed extractor."""

    def test_simple_html(self):
        html = """
        <html><head><title>Test Page</title></head>
        <body><article><h1>Hello World</h1>
        <p>This is a test paragraph with enough words to be kept by the extractor.</p>
        </article></body></html>
        """
        result = extract_content_from_html(html)
        assert "test paragraph" in result

    def test_empty_html_returns_empty_string(self):
        assert extract_content_from_html("") == ""


class TestSplitFrontmatter:
    """Tests for the YAML frontmatter splitter."""

    def test_splits_header_and_body(self):
        text = (
            "---\n"
            "url: https://example.com/x\n"
            "method: local-trafilatura\n"
            "raw_bytes: 1234\n"
            "---\n\n"
            "# Title\n\nBody text here."
        )
        meta, body = split_frontmatter(text)
        assert meta["url"] == "https://example.com/x"
        assert meta["method"] == "local-trafilatura"
        assert meta["raw_bytes"] == "1234"
        assert body.startswith("# Title")

    def test_no_frontmatter_returns_text(self):
        meta, body = split_frontmatter("just a body, no header")
        assert meta == {}
        assert body == "just a body, no header"


class TestFetchModel:
    """The Fetch input model takes a list of `sources` (1–50): URLs, paths, DOIs, titles, ISBNs."""

    def test_accepts_single_source_in_list(self):
        assert Fetch(sources=["https://example.com/"]).sources == ["https://example.com/"]

    def test_accepts_many_sources_and_preserves_order(self):
        sources = [f"https://example.com/{i}" for i in range(50)]
        assert Fetch(sources=sources).sources == sources

    def test_rejects_empty_list(self):
        with pytest.raises(ValidationError):
            Fetch(sources=[])

    def test_rejects_more_than_50(self):
        with pytest.raises(ValidationError):
            Fetch(sources=[f"https://example.com/{i}" for i in range(51)])

    def test_requires_sources_field(self):
        with pytest.raises(ValidationError):
            Fetch()  # type: ignore[call-arg]

    def test_old_urls_field_is_gone(self):
        with pytest.raises(ValidationError):
            Fetch(urls=["https://example.com/"])  # type: ignore[call-arg]


class TestDescribeFetchResult:
    """Per-URL rendering: full body on success, a descriptive error on every failure."""

    def test_success_returns_header_and_full_body_untruncated(self):
        body = "# Title\n\n" + ("word " * 400)
        tc = describe_fetch_result("https://ok.example/", _result(body))
        assert isinstance(tc, TextContent)
        assert tc.text.startswith("# https://ok.example/")
        assert "cache_status: miss" in tc.text
        assert body in tc.text
        assert "ERROR" not in tc.text

    def test_short_but_legit_page_is_success_not_an_error(self):
        body = "This domain is for use in illustrative examples."
        tc = describe_fetch_result("https://example.com/", _result(body, http_status=200))
        assert body in tc.text
        assert "ERROR" not in tc.text

    def test_timeout_is_described(self):
        tc = describe_fetch_result("https://x.example/", _result("", http_status=None, error_kind="timeout"))
        assert "ERROR" in tc.text
        assert "https://x.example/" in tc.text
        assert "timed out" in tc.text.lower()

    def test_dns_failure_is_described(self):
        tc = describe_fetch_result("https://nope.invalid/", _result("", http_status=None, error_kind="dns"))
        assert "dns" in tc.text.lower()

    def test_connection_failure_is_described(self):
        tc = describe_fetch_result("https://x.example/", _result("", http_status=None, error_kind="connect"))
        assert "connection failed" in tc.text.lower()

    def test_http_404_surfaces_code_and_meaning(self):
        tc = describe_fetch_result("https://x.example/missing", _result("tiny", http_status=404))
        assert "HTTP 404" in tc.text
        assert "page not found" in tc.text

    def test_http_403_adds_bot_block_note(self):
        tc = describe_fetch_result("https://x.example/", _result("tiny", http_status=403))
        assert "HTTP 403" in tc.text
        assert "bot-block or rate limit" in tc.text

    def test_cloudflare_challenge_is_described(self):
        tc = describe_fetch_result("https://x.example/", _result("", http_status=200, challenge=True))
        assert "challenge" in tc.text.lower()

    def test_nothing_extractable_is_described(self):
        tc = describe_fetch_result("https://x.example/", _result("", http_status=200))
        assert "no readable content" in tc.text.lower()

    def test_invalid_url_is_described(self):
        tc = describe_fetch_result("not a url", _result("", http_status=None, error_kind="invalid"))
        assert "Invalid URL" in tc.text

    def test_exception_is_wrapped_not_leaked(self):
        tc = describe_fetch_result("https://x.example/", ConnectionError("reset by peer"))
        assert "ERROR" in tc.text
        assert "ConnectionError" in tc.text
        assert "reset by peer" in tc.text

    def test_oversized_success_body_is_capped_with_a_note_naming_the_cache(self, monkeypatch):
        monkeypatch.setenv("HARVESTER_MAX_INLINE_CHARS", "1000")
        full = "x" * 5000
        tc = describe_fetch_result("https://big.example/", _result(full))
        # The 5000-char body must not survive inline...
        assert full not in tc.text
        # ...but the first `cap` chars and a truncation note must.
        assert "x" * 1000 in tc.text
        assert "truncated: first 1000 of 5000 chars" in tc.text
        # The note must name the real cache path so the source can be flagged as partial, tell
        # the model the file holds the COMPLETE text (not "summarised"), name the char offset to
        # resume reading from, and be honest that searchCache locates pages, not text.
        assert "/tmp/x.md" in tc.text
        assert "COMPLETE text is at" in tc.text
        assert "read that file from char 1000" in tc.text
        assert "searchCache" in tc.text and "does not return text" in tc.text
        assert "summarised" not in tc.text.lower()

    def test_under_cap_success_body_is_left_untruncated(self, monkeypatch):
        monkeypatch.setenv("HARVESTER_MAX_INLINE_CHARS", "1000")
        body = "y" * 500
        tc = describe_fetch_result("https://small.example/", _result(body))
        assert body in tc.text
        assert "truncated" not in tc.text


class TestTidyMarkdown:
    """Light, content-safe markdown tidy — vertical/trailing whitespace only."""

    def test_strips_trailing_whitespace(self):
        assert tidy_markdown("alpha   \nbeta\t\n") == "alpha\nbeta\n"

    def test_collapses_four_blank_lines_to_one(self):
        assert tidy_markdown("a\n\n\n\n\nb\n") == "a\n\nb\n"

    def test_strips_leading_and_trailing_blank_lines_one_final_newline(self):
        assert tidy_markdown("\n\n# Title\n\nbody\n\n\n") == "# Title\n\nbody\n"

    def test_preserves_links_brackets_and_inline_text_byte_identical(self):
        md = (
            "# Heading\n"
            "\n"
            "See [the docs](https://example.com/a_b?x=1) and `code[0]`.\n"
            "\n"
            "    indented line kept verbatim\n"
            "\n"
            "Brackets [x] and inline   double  spaces stay.\n"
        )
        assert tidy_markdown(md) == md

    def test_preserves_code_fence_contents_byte_identical(self):
        md = (
            "```python\n"
            "def f(x):\n"
            "    return  x + 1\n"
            "```\n"
        )
        assert tidy_markdown(md) == md

    def test_idempotent(self):
        md = "  \n# T\n\n\n\nlots\n\n\n\nof   \nblanks\n\n\n"
        once = tidy_markdown(md)
        assert tidy_markdown(once) == once

    def test_empty_input_stays_empty(self):
        assert tidy_markdown("") == ""
        assert tidy_markdown("\n\n  \n") == ""


class TestLooksLikeChallenge:
    """Cloudflare / bot-wall marker detection on raw HTML."""

    def test_detects_just_a_moment(self):
        assert looks_like_challenge("<html><title>Just a moment...</title></html>")

    def test_plain_page_is_not_a_challenge(self):
        assert not looks_like_challenge("<html><body><p>hello world</p></body></html>")

    def test_detects_are_you_a_robot_captcha(self):
        """The live sciencedirect captcha variant must be flagged (root cause A)."""
        captcha = (
            "Are you a robot? Please confirm you are a human by completing the captcha "
            "challenge. Enable JavaScript and cookies to continue. "
            "Reference number: 1234567890. IP Address: 1.2.3.4."
        )
        assert looks_like_challenge(captcha)

    def test_long_article_mentioning_captcha_is_not_a_challenge(self):
        """A long real article that mentions 'captcha'/'robot' once must NOT be flagged."""
        article = (
            "This survey of robot perception discusses how a captcha differs from a Turing test. "
        ) + ("word " * 2000)  # ~10 KB of real prose, single passing mention
        assert not looks_like_challenge(article)


class TestFetchToolBatch:
    """End-to-end through the real `fetch` MCP tool: batch in, full content out, in order."""

    async def test_batch_of_two_returns_two_results_in_order_with_full_body(self, monkeypatch):
        bodies = {
            "https://a.example/": "# A\n\n" + ("alpha " * 300),
            "https://b.example/": "# B\n\n" + ("beta " * 300),
        }

        async def fake(url, user_agent, proxy_url=None, media="allow", refresh=False):
            return _result(bodies[url], bytes=len(bodies[url]))

        urls = ["https://a.example/", "https://b.example/"]
        texts = await _run_fetch_tool(monkeypatch, urls, fake)

        assert len(texts) == 2
        assert texts[0].startswith("# https://a.example/")
        assert bodies["https://a.example/"] in texts[0]
        assert texts[1].startswith("# https://b.example/")
        assert bodies["https://b.example/"] in texts[1]

    async def test_failing_url_gets_descriptive_error_without_breaking_batch(self, monkeypatch):
        async def fake(url, user_agent, proxy_url=None, media="allow", refresh=False):
            if "boom" in url:
                raise ConnectionError("simulated connection reset")
            if "missing" in url:
                return _result("tiny", http_status=404)
            return _result("# OK\n\n" + ("good " * 300))

        urls = ["https://boom.example/", "https://x.example/missing", "https://ok.example/"]
        texts = await _run_fetch_tool(monkeypatch, urls, fake)

        assert len(texts) == 3
        assert "ERROR" in texts[0] and "ConnectionError" in texts[0]
        assert "HTTP 404" in texts[1] and "page not found" in texts[1]
        assert "good good" in texts[2] and "ERROR" not in texts[2]


class TestFetchSizeOnly:
    """`fetch(size_only=True)`: full content is still cached, but only {size, chars, path} is
    returned — no body. Errors fall back to the normal error rendering."""

    async def test_size_only_returns_size_and_path_without_body(self, monkeypatch):
        body = "# Doc\n\n" + ("alpha " * 500)  # Latin prose

        async def fake(url, user_agent, proxy_url=None, media="allow", refresh=False):
            return _result(body, bytes=len(body))

        texts = await _run_fetch_tool(
            monkeypatch, ["https://a.example/"], fake, extra_args={"size_only": True})

        assert len(texts) == 1
        payload = json.loads(texts[0])
        assert payload["chars"] == len(body)
        assert payload["size"] == math.ceil(len(body) / 2)  # over-counting heuristic
        assert payload["size"] == estimate_tokens(body)
        assert payload["path"] == "/tmp/x.md"  # the cache file path for the disk-slice reader
        # The body itself must NOT be inlined — only its size/path.
        assert "alpha alpha" not in texts[0]

    async def test_size_only_error_source_falls_back_to_error_rendering(self, monkeypatch):
        async def fake(url, user_agent, proxy_url=None, media="allow", refresh=False):
            return {"error": "behind a paywall", "body": ""}

        texts = await _run_fetch_tool(
            monkeypatch, ["https://walled.example/"], fake, extra_args={"size_only": True})

        assert len(texts) == 1
        assert "ERROR" in texts[0] and "behind a paywall" in texts[0]

    async def test_default_fetch_still_returns_body(self, monkeypatch):
        body = "# Doc\n\n" + ("beta " * 300)

        async def fake(url, user_agent, proxy_url=None, media="allow", refresh=False):
            return _result(body, bytes=len(body))

        texts = await _run_fetch_tool(monkeypatch, ["https://a.example/"], fake)
        assert body in texts[0]  # without size_only, the body comes back


class TestSizeOnlyReusesCache:
    """A `size_only` probe writes/reads the SAME cache entry a normal fetch uses — no
    duplicate file, no poisoned entry. (Dispatch caching is exercised end-to-end here.)"""

    async def test_probe_then_fetch_share_one_cache_file(self, monkeypatch):
        rich = ("<html><body><article><p>" + ("word " * 400) + "</p></article></body></html>").encode()

        async def fake_bytes(url, ua, proxy=None):
            return rich, 200, None, "text/html"

        monkeypatch.setattr(net, "fetch_bytes_with_meta", fake_bytes)

        url = "https://example.com/article"
        # 1) size_only probe — fetches + caches, returns size + path, no body.
        probe = await dispatch.get_or_fetch(url, "ua", None, media="deny")
        probe_tc = describe_size_result(url, probe)
        payload = json.loads(probe_tc.text)
        assert payload["size"] > 0 and payload["chars"] > 0
        probe_path = payload["path"]

        # 2) a normal fetch of the same source is a cache HIT on the SAME file.
        full = await dispatch.get_or_fetch(url, "ua", None, media="deny")
        assert full["cache_status"] == "hit"
        assert str(full["md_path"]) == probe_path  # one canonical cache entry, not a duplicate
        assert "word word" in full["body"]  # the body is intact (probe did not strip it on disk)


class TestTokenCountFrontmatter:
    """`_write_md` records a `token_count` in the YAML frontmatter (the size heuristic)."""

    def test_write_md_records_token_count(self, tmp_path):
        md_path = tmp_path / "doc.md"
        body = "word " * 400  # 2000 chars Latin prose
        cache_mod._write_md(md_path, "https://example.com/x", "local-trafilatura", body)
        meta, parsed = split_frontmatter(md_path.read_text(encoding="utf-8"))
        assert int(meta["token_count"]) == estimate_tokens(parsed)
        assert int(meta["token_count"]) == math.ceil(len(body) / 2)


# ── is_private_host ───────────────────────────────────────────────────────────

class TestIsPrivateHost:
    """is_private_host correctly classifies private/internal vs public hosts."""

    # --- hosts that MUST be private ---

    def test_localhost(self):
        assert is_private_host("http://localhost/foo")

    def test_loopback_ipv4(self):
        assert is_private_host("http://127.0.0.1:8080/")

    def test_loopback_ipv6(self):
        assert is_private_host("http://[::1]/")

    def test_rfc1918_10(self):
        assert is_private_host("http://10.0.0.1/api")

    def test_rfc1918_172(self):
        assert is_private_host("http://172.16.0.1/")

    def test_rfc1918_192_168(self):
        assert is_private_host("http://192.168.1.100/page")

    def test_link_local_169_254(self):
        assert is_private_host("http://169.254.1.1/")

    def test_tailscale_ts_net(self):
        assert is_private_host("http://internal-host.ts.net/")

    def test_dot_local(self):
        assert is_private_host("http://printer.local/")

    def test_dot_internal(self):
        assert is_private_host("http://api.corp.internal/v1")

    def test_url_with_credentials(self):
        assert is_private_host("http://user:pass@example.com/secret")

    # --- hosts that MUST be public ---

    def test_public_hostname(self):
        assert not is_private_host("https://example.com/page")

    def test_public_ip(self):
        assert not is_private_host("https://8.8.8.8/")

    def test_jina_reader_itself(self):
        assert not is_private_host("https://r.jina.ai/https://example.com/")


# ── assert_fetchable: the single SSRF/scheme chokepoint ──────────────────────

class TestAssertFetchable:
    """assert_fetchable rejects non-http(s) schemes, internal hosts, and names that RESOLVE
    to a private IP (DNS-rebinding) — and allows ordinary public http(s) URLs."""

    async def test_blocks_non_http_schemes(self):
        for u in ("file:///etc/passwd", "gopher://h/1", "dict://h/x", "ftp://h/f", "data:text/x"):
            with pytest.raises(net.FetchNotAllowed):
                await net.assert_fetchable(u)

    async def test_blocks_metadata_loopback_and_internal(self):
        # All caught lexically (literal IPs / internal TLDs) before any DNS lookup.
        for u in ("http://169.254.169.254/latest/meta-data/iam/security-credentials/",
                  "http://127.0.0.1:6379/", "http://[::1]/", "http://node.ts.net/x",
                  "http://10.1.2.3/", "http://192.168.0.5/", "http://0x7f000001/"):
            with pytest.raises(net.FetchNotAllowed):
                await net.assert_fetchable(u)

    async def test_blocks_dns_rebind_to_private(self, monkeypatch):
        # A public hostname whose A-record points at link-local/RFC-1918 must be refused.
        monkeypatch.setattr(net.socket, "getaddrinfo",
                            lambda *a, **k: [(2, 1, 6, "", ("169.254.169.254", 0))])
        with pytest.raises(net.FetchNotAllowed):
            await net.assert_fetchable("http://rebind.example.com/")

    async def test_allows_public_host(self, monkeypatch):
        monkeypatch.setattr(net.socket, "getaddrinfo",
                            lambda *a, **k: [(2, 1, 6, "", ("93.184.216.34", 0))])
        await net.assert_fetchable("https://example.com/page")  # must NOT raise


# ── SSRF chokepoint on OA candidates (resolve_doi/book + scraped citation_pdf_url) ──

class TestCandidateSsrfGuard:
    """_candidate_to_result refuses any candidate whose URL is non-http(s) or an internal host,
    BEFORE any network fetch — covering OA-resolver candidates and the scraped citation_pdf_url."""

    def _block_all_fetch(self, monkeypatch):
        async def boom_bytes(*a, **k):
            raise AssertionError("a refused candidate must never reach fetch_bytes_with_meta")

        async def boom_imp(*a, **k):
            raise AssertionError("a refused candidate must never reach download_impersonated")

        monkeypatch.setattr(net, "fetch_bytes_with_meta", boom_bytes)
        monkeypatch.setattr(net, "download_impersonated", boom_imp)

    async def test_internal_and_scheme_candidates_refused(self, monkeypatch):
        self._block_all_fetch(monkeypatch)
        for url in ("http://169.254.169.254/latest/meta-data/",
                    "http://127.0.0.1:8080/paper.pdf",
                    "http://internal.ts.net/paper.pdf",
                    "file:///etc/passwd",
                    "gopher://evil/1"):
            cand = oa.Candidate(0, url, "unpaywall", kind_hint="pdf")
            assert await dispatch._candidate_to_result(cand, "k", "ua", None) is None, url

    async def test_scraped_citation_pdf_url_to_metadata_refused(self, monkeypatch):
        # The exact CRITICAL #1 vector: a citation_pdf_url scraped from an attacker page.
        self._block_all_fetch(monkeypatch)
        cand = oa.Candidate(0, "http://169.254.169.254/secrets.pdf", "citation_pdf_url",
                            kind_hint="pdf")
        assert await dispatch._candidate_to_result(cand, "k", "ua", None) is None


# ── SSRF chokepoint across HTTP redirects (per-hop revalidation) ──────────────

class TestRedirectSsrfGuard:
    """The httpx request event-hook re-validates EVERY hop, so a public URL that 302s to the
    cloud-metadata endpoint is refused; a fetch primitive converts that refusal to its sentinel."""

    async def test_redirect_to_metadata_is_refused(self, monkeypatch):
        import httpx
        monkeypatch.setattr(net.socket, "getaddrinfo",
                            lambda *a, **k: [(2, 1, 6, "", ("93.184.216.34", 0))])

        def handler(request):
            if request.url.host == "attacker.example":
                return httpx.Response(
                    302, headers={"location": "http://169.254.169.254/latest/meta-data/"})
            return httpx.Response(200, content=b"INTERNAL-LEAK")

        async with net._client(transport=httpx.MockTransport(handler)) as client:
            with pytest.raises(net.FetchNotAllowed):
                await client.get("http://attacker.example/", follow_redirects=True)

    async def test_primitive_returns_sentinel_on_private_resolution(self, monkeypatch):
        # net.fetch_raw must swallow the refusal and return "" (its resilient contract), not raise.
        monkeypatch.setattr(net.socket, "getaddrinfo",
                            lambda *a, **k: [(2, 1, 6, "", ("10.0.0.1", 0))])
        assert await net.fetch_raw("http://rebind.example.com/", "ua") == ""


# ── HTML fetch ladder escalation ─────────────────────────────────────────────

class TestFetchLadder:
    """_html_result escalates through httpx → curl_cffi → Jina when content is thin."""

    _THIN_HTML = b"<html><body><p>tiny</p></body></html>"
    # Rich HTML gives trafilatura enough to exceed THIN_MIN_CHARS
    _RICH_BODY = "word " * 120  # 600 chars
    _RICH_HTML = f"<html><body><article><p>{_RICH_BODY}</p></article></body></html>".encode()

    async def _noop_localise(self, md, base_url, ua, proxy=None):
        return md

    async def test_rich_httpx_does_not_escalate(self, monkeypatch, tmp_path):
        """A fat initial response must stay on local-trafilatura — no escalation."""
        async def fake_bytes(url, ua, proxy=None):
            return self._RICH_HTML, 200, None, "text/html"

        impersonated_called = []

        async def fake_impersonated(url, proxy=None):
            impersonated_called.append(url)
            return "", None

        jina_called = []

        async def fake_jina(url, ua, proxy=None):
            jina_called.append(url)
            return ""

        monkeypatch.setattr(net, "fetch_bytes_with_meta", fake_bytes)
        monkeypatch.setattr(net, "fetch_impersonated", fake_impersonated)
        monkeypatch.setattr(net, "fetch_jina", fake_jina)
        monkeypatch.setattr(images, "localize_html_images", self._noop_localise)

        result = await dispatch._html_result(
            "https://example.com/page", "https://example.com/page", False, "ua", None
        )

        assert result.get("method") == "local-trafilatura"
        assert not impersonated_called, "curl_cffi should NOT be called for rich content"
        assert not jina_called, "Jina should NOT be called for rich content"

    async def test_thin_escalates_to_curl_cffi(self, monkeypatch, tmp_path):
        """Thin httpx response → curl_cffi is tried; if richer, method becomes curl_cffi-trafilatura."""
        async def fake_bytes(url, ua, proxy=None):
            return self._THIN_HTML, 200, None, "text/html"

        async def fake_impersonated(url, proxy=None):
            return self._RICH_HTML.decode(), 200

        jina_called = []

        async def fake_jina(url, ua, proxy=None):
            jina_called.append(url)
            return ""

        monkeypatch.setattr(net, "fetch_bytes_with_meta", fake_bytes)
        monkeypatch.setattr(net, "fetch_impersonated", fake_impersonated)
        monkeypatch.setattr(net, "fetch_jina", fake_jina)
        monkeypatch.setattr(images, "localize_html_images", self._noop_localise)

        result = await dispatch._html_result(
            "https://example.com/wall", "https://example.com/wall", False, "ua", None
        )

        assert result.get("method") == "curl_cffi-trafilatura", (
            f"expected curl_cffi-trafilatura, got {result.get('method')!r}"
        )
        assert not jina_called, "Jina should NOT be called when curl_cffi succeeds"

    async def test_challenge_page_escalates_to_curl_cffi(self, monkeypatch, tmp_path):
        """A Cloudflare challenge triggers curl_cffi even if byte count is large enough."""
        cf_html = b"<html><title>Just a moment...</title><body>Checking your browser</body></html>"

        async def fake_bytes(url, ua, proxy=None):
            return cf_html, 200, None, "text/html"

        async def fake_impersonated(url, proxy=None):
            return self._RICH_HTML.decode(), 200

        monkeypatch.setattr(net, "fetch_bytes_with_meta", fake_bytes)
        monkeypatch.setattr(net, "fetch_impersonated", fake_impersonated)
        monkeypatch.setattr(net, "fetch_jina", lambda *a, **kw: "")
        monkeypatch.setattr(images, "localize_html_images", self._noop_localise)

        result = await dispatch._html_result(
            "https://cf.example/", "https://cf.example/", False, "ua", None
        )

        assert result.get("method") == "curl_cffi-trafilatura"

    async def test_curl_cffi_thin_escalates_to_jina(self, monkeypatch, tmp_path):
        """When both httpx and curl_cffi yield thin content, Jina is the final fallback."""
        async def fake_bytes(url, ua, proxy=None):
            return self._THIN_HTML, 200, None, "text/html"

        async def fake_impersonated(url, proxy=None):
            return "", None  # curl_cffi fails

        async def fake_jina(url, ua, proxy=None):
            return self._RICH_BODY  # Jina returns rich markdown

        monkeypatch.setattr(net, "fetch_bytes_with_meta", fake_bytes)
        monkeypatch.setattr(net, "fetch_impersonated", fake_impersonated)
        monkeypatch.setattr(net, "fetch_jina", fake_jina)
        monkeypatch.setattr(images, "localize_html_images", self._noop_localise)

        result = await dispatch._html_result(
            "https://example.com/js-only", "https://example.com/js-only", False, "ua", None
        )

        assert result.get("method") == "jina-reader", (
            f"expected jina-reader, got {result.get('method')!r}"
        )

    async def test_private_host_skips_jina(self, monkeypatch, tmp_path):
        """Jina must never be called for private / internal hosts."""
        async def fake_bytes(url, ua, proxy=None):
            return self._THIN_HTML, 200, None, "text/html"

        async def fake_impersonated(url, proxy=None):
            return "", None

        jina_called = []

        async def fake_jina(url, ua, proxy=None):
            jina_called.append(url)
            return ""

        monkeypatch.setattr(net, "fetch_bytes_with_meta", fake_bytes)
        monkeypatch.setattr(net, "fetch_impersonated", fake_impersonated)
        monkeypatch.setattr(net, "fetch_jina", fake_jina)
        monkeypatch.setattr(images, "localize_html_images", self._noop_localise)

        await dispatch._html_result(
            "http://192.168.1.1/admin", "http://192.168.1.1/admin", False, "ua", None
        )

        assert not jina_called, "Jina must NOT be called for a private host"

    async def test_binary_content_type_routes_to_local_converter(self, monkeypatch, tmp_path):
        """Content-Type: application/pdf on an extensionless URL must route to PDF converter."""
        pdf_bytes = b"%PDF-1.4 fake content\n%%EOF"

        async def fake_bytes(url, ua, proxy=None):
            return pdf_bytes, 200, None, "application/pdf"

        captured: dict = {}

        async def fake_doc_result(src, key, kind, local, ua, proxy):
            captured["kind"] = kind
            captured["local"] = local
            return {
                "cache_status": "miss", "method": "pdf:pymupdf4llm",
                "md_path": tmp_path / "out.md", "body": "# Doc\n\n" + "word " * 200,
                "bytes": 100, "content_chars": 1000, "http_status": None,
                "error_kind": None, "challenge": False,
            }

        monkeypatch.setattr(net, "fetch_bytes_with_meta", fake_bytes)
        monkeypatch.setattr(dispatch, "_doc_result", fake_doc_result)

        result = await dispatch._html_result(
            "https://arxiv.org/pdf/xyz", "https://arxiv.org/pdf/xyz", False, "ua", None
        )

        assert captured.get("kind") == "pdf"
        assert captured.get("local") is True
        assert result["method"] == "pdf:pymupdf4llm"


# ── the `find` tool: title/free-text → ranked candidate works (scholarly WebSearch) ──

async def _run_find_tool(monkeypatch, query, fake_find_sources):
    """Drive the real `find` MCP tool end-to-end over in-memory streams."""
    monkeypatch.setattr(srv, "find_sources", fake_find_sources)
    async with create_client_server_memory_streams() as (client_streams, server_streams):
        client_read, client_write = client_streams
        server_read, server_write = server_streams

        @asynccontextmanager
        async def fake_stdio():
            yield server_read, server_write

        monkeypatch.setattr(srv, "stdio_server", fake_stdio)
        async with anyio.create_task_group() as tg:
            tg.start_soon(srv.serve)
            async with ClientSession(client_read, client_write) as session:
                await session.initialize()
                tools = {t.name for t in (await session.list_tools()).tools}
                result = await session.call_tool("findWorks", {"query": query})
            tg.cancel_scope.cancel()
        return tools, [c.text for c in result.content if isinstance(c, TextContent)]


class TestFindModel:
    def test_defaults(self):
        f = FindWorks(query="entropy")
        assert f.query == "entropy" and f.limit == 8

    def test_rejects_bad_limit(self):
        with pytest.raises(ValidationError):
            FindWorks(query="x", limit=0)
        with pytest.raises(ValidationError):
            FindWorks(query="x", limit=999)

    def test_requires_query(self):
        with pytest.raises(ValidationError):
            FindWorks()  # type: ignore[call-arg]


class TestRenderFind:
    def test_websearch_style_listing(self):
        cands = [
            {"kind": "paper", "title": "A Mathematical Theory of Communication",
             "authors": "C. E. Shannon", "year": 1948, "fetch": "10.1002/x",
             "source": "openalex", "free": "closed", "match": 1.0},
            {"kind": "book", "title": "Frankenstein", "authors": "Mary Shelley", "year": 1818,
             "fetch": "isbn:9780000000000", "source": "openlibrary", "free": "public", "match": 0.3},
        ]
        out = srv._render_find("communication", cands)
        assert "1. A Mathematical Theory of Communication" in out
        assert "fetch: 10.1002/x" in out
        assert "fetch: isbn:9780000000000" in out
        assert "C. E. Shannon" in out and "1948" in out

    def test_empty(self):
        assert "No candidate works" in srv._render_find("nope", [])


class TestFindToolEndToEnd:
    async def test_find_is_registered_and_returns_candidates(self, monkeypatch):
        async def fake_find_sources(query, limit, proxy=None):
            return [{"kind": "paper", "title": "Hit", "authors": "A. Author", "year": 2020,
                     "fetch": "10.1/hit", "source": "openalex", "free": "gold", "match": 1.0}]

        tools, texts = await _run_find_tool(monkeypatch, "anything", fake_find_sources)
        assert "findWorks" in tools and "fetch" in tools  # both tools registered
        assert len(texts) == 1
        assert "fetch: 10.1/hit" in texts[0] and "Hit" in texts[0]


# ── fetchImage + archive: media split out of fetch ──────────────────────────

class TestFetchImageModel:
    def test_accepts_list(self):
        assert FetchImage(sources=["https://x/a.png"]).sources == ["https://x/a.png"]

    def test_rejects_empty(self):
        with pytest.raises(ValidationError):
            FetchImage(sources=[])

    def test_rejects_over_50(self):
        with pytest.raises(ValidationError):
            FetchImage(sources=[f"https://x/{i}.png" for i in range(51)])


class TestArchiveModel:
    def test_member_optional(self):
        a = Archive(source="https://x/a.zip")
        assert a.member is None

    def test_with_member(self):
        assert Archive(source="x.zip", member="dir/file.txt").member == "dir/file.txt"

    def test_requires_source(self):
        with pytest.raises(ValidationError):
            Archive()  # type: ignore[call-arg]


class TestToolRegistration:
    async def test_all_six_tools_registered_without_backend(self, monkeypatch):
        """`search` is always advertised — the full six-tool surface shows even with no backend."""
        from harvester import search as _search

        async def fake_find_sources(query, limit, proxy=None):
            return []

        monkeypatch.setattr(_search, "SEARXNG_URL", "")
        monkeypatch.setattr(_search, "BRAVE_API_KEY", "")
        monkeypatch.delenv("HARVESTER_DISABLE_SEARCH", raising=False)
        tools, _ = await _run_find_tool(monkeypatch, "x", fake_find_sources)
        assert tools == {"fetch", "findWorks", "search", "fetchImage", "archive", "searchCache"}

    async def test_search_registered_when_backend_configured(self, monkeypatch):
        """A configured backend (SearXNG/Brave) keeps `search` in the tool list."""
        from harvester import search as _search

        async def fake_find_sources(query, limit, proxy=None):
            return []

        monkeypatch.setattr(_search, "SEARXNG_URL", "http://127.0.0.1:8888")
        monkeypatch.delenv("HARVESTER_DISABLE_SEARCH", raising=False)
        tools, _ = await _run_find_tool(monkeypatch, "x", fake_find_sources)
        assert tools == {"fetch", "findWorks", "search", "fetchImage", "archive", "searchCache"}

    async def test_search_force_hidden(self, monkeypatch):
        """HARVESTER_DISABLE_SEARCH force-hides `search` even when a backend is configured."""
        from harvester import search as _search

        async def fake_find_sources(query, limit, proxy=None):
            return []

        monkeypatch.setattr(_search, "SEARXNG_URL", "http://127.0.0.1:8888")
        monkeypatch.setenv("HARVESTER_DISABLE_SEARCH", "1")
        tools, _ = await _run_find_tool(monkeypatch, "x", fake_find_sources)
        assert tools == {"fetch", "findWorks", "fetchImage", "archive", "searchCache"}
        assert "search" not in tools


async def _list_tools_raw(monkeypatch):
    """Drive the server's `list_tools` over in-memory streams, returning the raw `Tool` objects
    (name + description + inputSchema) — what a model actually reads, unlike the name-only set
    `_run_find_tool` returns."""
    async with create_client_server_memory_streams() as (client_streams, server_streams):
        client_read, client_write = client_streams
        server_read, server_write = server_streams

        @asynccontextmanager
        async def fake_stdio():
            yield server_read, server_write

        monkeypatch.setattr(srv, "stdio_server", fake_stdio)
        async with anyio.create_task_group() as tg:
            tg.start_soon(srv.serve)
            async with ClientSession(client_read, client_write) as session:
                await session.initialize()
                tools = (await session.list_tools()).tools
            tg.cancel_scope.cancel()
        return tools


class TestFetchToolDescriptionTruth:
    """Slice 1 deliverables 1 + 3: the `fetch` tool schema must document PMID/PMCID inputs and
    must not contain the "summarised" / shouty "FULL content" contradictions."""

    async def test_mentions_pmid_and_pmcid(self, monkeypatch):
        tools = await _list_tools_raw(monkeypatch)
        fetch_tool = next(t for t in tools if t.name == "fetch")
        sources_desc = fetch_tool.inputSchema["properties"]["sources"]["description"]
        assert "PMID" in fetch_tool.description
        assert "PMID" in sources_desc and "PMCID" in sources_desc

    async def test_no_summarised_or_shouty_full_content_claim(self, monkeypatch):
        tools = await _list_tools_raw(monkeypatch)
        fetch_tool = next(t for t in tools if t.name == "fetch")
        sources_desc = fetch_tool.inputSchema["properties"]["sources"]["description"]
        combined = fetch_tool.description + sources_desc
        assert "summarised" not in combined.lower()
        assert "FULL content" not in combined


class TestSearchModel:
    def test_defaults(self):
        s = Search(query="x")
        assert s.count == 8 and s.lang == "" and s.engines == ""

    def test_rejects_bad_count(self):
        with pytest.raises(ValidationError):
            Search(query="x", count=0)
        with pytest.raises(ValidationError):
            Search(query="x", count=99)


class TestRenderSearch:
    def test_no_backend_configured(self):
        assert "No web-search backend" in srv._render_search("q", None, None)

    def test_results_listed(self):
        out = srv._render_search(
            "q", [{"title": "T", "url": "https://u", "snippet": "s", "engine": "google"}], "searxng")
        assert "https://u" in out and "via searxng" in out and "T" in out

    def test_empty(self):
        assert "No results" in srv._render_search("q", [], "brave")


# ── `searchCache` tool: gains an `md_path` field per hit ─────────────────────────

async def _run_searchcache_tool(monkeypatch, pattern, fake_search_cache, *, extra_args=None):
    """Drive the real `searchCache` MCP tool end-to-end over in-memory streams."""
    monkeypatch.setattr(srv, "search_cache", fake_search_cache)
    async with create_client_server_memory_streams() as (client_streams, server_streams):
        client_read, client_write = client_streams
        server_read, server_write = server_streams

        @asynccontextmanager
        async def fake_stdio():
            yield server_read, server_write

        monkeypatch.setattr(srv, "stdio_server", fake_stdio)
        async with anyio.create_task_group() as tg:
            tg.start_soon(srv.serve)
            async with ClientSession(client_read, client_write) as session:
                await session.initialize()
                call_args = {"pattern": pattern, **(extra_args or {})}
                result = await session.call_tool("searchCache", call_args)
            tg.cancel_scope.cancel()
        return [c.text for c in result.content if isinstance(c, TextContent)]


class TestSearchCacheTool:
    """Slice 1 deliverable 4: `searchCache` output gains an `md_path` field per hit — cache.py
    already computes it, this pins that the SERVER RENDERING actually surfaces it."""

    async def test_hits_include_md_path(self, monkeypatch):
        def fake_search_cache(pattern, max_results, ignore_case):
            return [{"url": "https://x.example/a", "md_path": "/tmp/.cache/html/a__deadbeef00.md",
                     "matches": 2, "sample": "hello world"}]

        texts = await _run_searchcache_tool(monkeypatch, "hello", fake_search_cache)
        assert len(texts) == 1
        assert "https://x.example/a" in texts[0]
        assert "/tmp/.cache/html/a__deadbeef00.md" in texts[0]
        assert "md_path" in texts[0]

    async def test_no_matches_names_next_move(self, monkeypatch):
        def fake_search_cache(pattern, max_results, ignore_case):
            return []

        texts = await _run_searchcache_tool(monkeypatch, "nope", fake_search_cache)
        assert "No cached pages match" in texts[0]
        assert "`search`" in texts[0] or "`fetch`" in texts[0]


# ── the `fetch` prompt: same rendering + media contract as the `fetch` tool ──────

async def _run_fetch_prompt(monkeypatch, url, fake_get_or_fetch):
    """Drive the real `fetch` MCP prompt end-to-end over in-memory streams, with
    get_or_fetch monkeypatched so no network is touched."""
    monkeypatch.setattr(srv, "get_or_fetch", fake_get_or_fetch)
    async with create_client_server_memory_streams() as (client_streams, server_streams):
        client_read, client_write = client_streams
        server_read, server_write = server_streams

        @asynccontextmanager
        async def fake_stdio():
            yield server_read, server_write

        monkeypatch.setattr(srv, "stdio_server", fake_stdio)
        async with anyio.create_task_group() as tg:
            tg.start_soon(srv.serve)
            async with ClientSession(client_read, client_write) as session:
                await session.initialize()
                result = await session.get_prompt("fetch", {"url": url})
            tg.cancel_scope.cancel()
        return result


class TestFetchPromptAlignsWithTool:
    """Slice 1 deliverable 6: the `fetch` prompt must render through `describe_fetch_result`
    (so it keeps status/kind diagnostics, header, and truncation, not just a bare body/error)
    and must run `media="deny"` like the tool (redirecting images/archives instead of guessing)."""

    async def test_success_gets_the_same_header_as_the_tool(self, monkeypatch):
        body = "# Doc\n\n" + ("word " * 300)

        async def fake(url, user_agent, proxy_url=None, media="allow"):
            assert media == "deny", "the prompt must fetch with media='deny', same as the tool"
            return _result(body, bytes=len(body))

        result = await _run_fetch_prompt(monkeypatch, "https://ok.example/", fake)
        text = result.messages[0].content.text
        assert text.startswith("# https://ok.example/")
        assert "cache_status: miss" in text  # the describe_fetch_result header, not a bare body
        assert body in text

    async def test_media_deny_redirects_images_like_the_tool(self, monkeypatch):
        async def fake(url, user_agent, proxy_url=None, media="allow"):
            assert media == "deny"
            return {"error": f"{url} is an image — use the `fetchImage` tool, not `fetch`.", "body": ""}

        result = await _run_fetch_prompt(monkeypatch, "https://x.example/fig.png", fake)
        text = result.messages[0].content.text
        assert "ERROR" in text
        assert "fetchImage" in text

    async def test_failure_keeps_status_kind_diagnostics(self, monkeypatch):
        """Before this fix the prompt path built its own <error> tag and lost the status/kind
        classification describe_fetch_result derives (e.g. the HTTP code + meaning)."""
        async def fake(url, user_agent, proxy_url=None, media="allow"):
            return _result("tiny", http_status=404)

        result = await _run_fetch_prompt(monkeypatch, "https://x.example/missing", fake)
        text = result.messages[0].content.text
        assert "HTTP 404" in text and "page not found" in text
