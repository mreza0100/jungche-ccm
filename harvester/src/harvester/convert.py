"""File -> Markdown converters.

Nothing here touches the network — it only converts local files. Heavy converter deps
(docling, pymupdf4llm, markitdown) are imported lazily so importing this module stays cheap.
Format detection and security guards live in `detect.py`.
"""

import contextlib
import json
import os
import sys
from pathlib import Path

from .log import get_logger

log = get_logger("convert")


@contextlib.contextmanager
def _quiet_stdout():
    """Redirect stdout → stderr for the duration.

    The heavy converters (pymupdf4llm, Docling, MarkItDown) print progress and warnings to
    STDOUT — which is the MCP stdio JSON-RPC channel; a single stray byte there corrupts the
    protocol. Send any such chatter to stderr instead.
    """
    with contextlib.redirect_stdout(sys.stderr):
        yield

# This build of pymupdf4llm is SLOW by default: it (a) runs Tesseract OCR on every image-ish
# page and (b) runs a per-page layout MODEL — together 10-15 s/PDF, minutes for a scanned book,
# and fatal for batch fetches. Default BOTH off for fast text-layer extraction (~3-5 s/PDF):
#   HARVESTER_PDF_OCR=1     — OCR genuinely scanned PDFs (image-only pages).
#   HARVESTER_PDF_LAYOUT=1  — run the layout model for higher table/heading fidelity.
# (A scanned PDF without OCR yields thin text, so the OA resolver falls through to a text copy.)
def _envflag(name: str) -> bool:
    return os.environ.get(name, "").lower() in ("1", "true", "yes", "on")


_PDF_OCR = _envflag("HARVESTER_PDF_OCR")
_PDF_LAYOUT = _envflag("HARVESTER_PDF_LAYOUT")

# ── file -> markdown converters (heavy deps imported lazily) ───────────────────
_DOCLING = None


def _docling():
    global _DOCLING
    if _DOCLING is None:
        from docling.document_converter import DocumentConverter
        _DOCLING = DocumentConverter()
    return _DOCLING


def pdf_to_md(path: str, force_ocr: bool = False) -> str:
    """PDF -> markdown. `force_ocr=True` runs the layout model + Tesseract for THIS call only —
    the dispatch OCR-escalation rung uses it when a PDF's text layer converts to empty (a scan),
    without turning the slow default on for every other fetch."""
    ocr = _PDF_OCR or force_ocr
    layout = _PDF_LAYOUT or force_ocr  # use_ocr only applies in layout mode — force it on too
    log.info("convert pdf:pymupdf4llm %s ocr=%s layout=%s", path, ocr, layout)
    import pymupdf4llm
    # The fork defaults the layout model ON (10s+/PDF). Force it off unless opted in.
    if hasattr(pymupdf4llm, "use_layout"):
        try:
            pymupdf4llm.use_layout(layout)
        except Exception as e:
            log.warning("pymupdf4llm.use_layout(%s) failed: %s", layout, e)
    # use_ocr only applies in layout mode; in legacy mode it's ignored (and warns). Pass it only
    # when it does something — legacy/classic extraction never OCRs anyway.
    kwargs = {"use_ocr": ocr} if layout else {}
    with _quiet_stdout():
        try:
            result = pymupdf4llm.to_markdown(path, **kwargs)
        except TypeError:
            # Upstream pymupdf4llm has no use_ocr kwarg (and doesn't OCR by default).
            result = pymupdf4llm.to_markdown(path)
    return result if isinstance(result, str) else ""


def office_to_md(path: str) -> str:
    """docx / xlsx / pptx -> markdown via Docling (best local table + heading fidelity)."""
    log.info("convert office:docling %s", path)
    with _quiet_stdout():
        return _docling().convert(path).document.export_to_markdown() or ""


def csv_to_md(path: str) -> str:
    log.info("convert csv:markitdown %s", path)
    from markitdown import MarkItDown
    with _quiet_stdout():
        return MarkItDown().convert(path).text_content or ""


def epub_to_md(path: str) -> str:
    """EPUB -> markdown via MarkItDown (verified live: spine-ordered chapters, headings kept).

    EPUBs are how OAPEN/DOAB/Gutenberg serve open books — without this converter those OA
    candidates were rejected as 'archive-shaped' bytes and the book was lost after being found.
    """
    log.info("convert epub:markitdown %s", path)
    from markitdown import MarkItDown
    with _quiet_stdout():
        return MarkItDown().convert(path).text_content or ""


def json_to_md(text: str) -> str:
    """Return the literal JSON, pretty-printed inside a fenced ```json block."""
    try:
        pretty = json.dumps(json.loads(text), indent=2, ensure_ascii=False)
    except Exception as e:
        log.warning("json_to_md parse failed, passing through raw: %s", e)
        pretty = text
    return "```json\n" + pretty.strip() + "\n```\n"


def convert_local_file(path: str, kind: str) -> str:
    """Convert a local document file to markdown by kind (SYNC — wrap in a thread).

    A converter that raises (corrupt PDF, broken office file, …) returns "" rather than leaking
    the raw library exception — the caller turns "" into a clean 'converted to empty' error.
    """
    converters = {
        "pdf": lambda: pdf_to_md(path),
        "docx": lambda: office_to_md(path),
        "xlsx": lambda: office_to_md(path),
        "pptx": lambda: office_to_md(path),
        "csv": lambda: csv_to_md(path),
        "epub": lambda: epub_to_md(path),
        "json": lambda: json_to_md(Path(path).read_text(encoding="utf-8", errors="ignore")),
    }
    fn = converters.get(kind)
    if fn is None:
        raise ValueError(f"no document converter for kind={kind!r}")
    try:
        return fn()
    except Exception as e:
        log.warning("convert %s (%s) failed: %s", path, kind, e)
        return ""
