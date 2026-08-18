#!/usr/bin/env python3
"""Pinned Harvester conversion worker.

The worker is intentionally a small JSON-lines process.  Go owns process
lifetime and filesystem policy; this file owns the exact Python conversion
implementation.  No network operation is performed here.
"""

from __future__ import annotations

import contextlib
import json
import os
import pathlib
import sys


def _quiet_stdout():
    return contextlib.redirect_stdout(sys.stderr)


def _clean(value: object) -> str:
    if not value:
        return ""
    if isinstance(value, (list, tuple)):
        value = ", ".join(str(x) for x in value if x)
    return " ".join(str(value).split()).strip()


_JUNK_AUTHORS = frozenset(
    {
        "username", "user name", "user", "login", "log in", "sign in",
        "signin", "sign up", "signup", "register", "log out", "logout",
        "admin", "administrator", "guest", "search", "menu", "subscribe",
        "newsletter",
    }
)


def html_metadata(raw: str) -> str:
    from trafilatura.metadata import extract_metadata

    try:
        meta = extract_metadata(raw)
    except Exception as exc:
        print(f"metadata extraction failed: {exc}", file=sys.stderr)
        return ""
    if meta is None:
        return ""
    author = _clean(getattr(meta, "author", ""))
    if author.casefold() in _JUNK_AUTHORS:
        author = ""
    fields = (
        ("Title", _clean(getattr(meta, "title", ""))),
        ("Authors", author),
        ("Published", _clean(getattr(meta, "date", ""))),
        ("Source", _clean(getattr(meta, "sitename", ""))),
        ("License", _clean(getattr(meta, "license", ""))),
    )
    lines = [f"**{label}:** {value}" for label, value in fields if value]
    return "\n".join(lines) + "\n\n---\n\n" if lines else ""


def convert_html(path: pathlib.Path) -> str:
    import trafilatura

    # Local HTML follows the old dispatch path, which decodes malformed bytes
    # with errors ignored before trafilatura sees the document.
    raw = path.read_bytes().decode("utf-8", errors="ignore")
    with _quiet_stdout():
        body = trafilatura.extract(
            raw,
            output_format="markdown",
            favor_recall=True,
            include_links=True,
            include_images=True,
        ) or ""
    return tidy_markdown(html_metadata(raw) + body)


def tidy_markdown(markdown: str) -> str:
    lines = [line.rstrip() for line in markdown.splitlines()]
    output: list[str] = []
    index = 0
    while index < len(lines):
        if lines[index] == "":
            end = index
            while end < len(lines) and lines[end] == "":
                end += 1
            output.extend([""] if end - index >= 3 else [""] * (end - index))
            index = end
        else:
            output.append(lines[index])
            index += 1
    while output and output[0] == "":
        output.pop(0)
    while output and output[-1] == "":
        output.pop()
    return "\n".join(output) + "\n" if output else ""


def convert_pdf(path: pathlib.Path, request: dict) -> tuple[str, dict]:
    import pymupdf4llm

    # The old Harvester snapshots these flags when the long-lived process
    # starts. Per-request changes would be a behavior drift.
    layout = _PDF_LAYOUT
    ocr = _PDF_OCR
    if hasattr(pymupdf4llm, "use_layout"):
        try:
            pymupdf4llm.use_layout(layout)
        except Exception as exc:
            print(f"pymupdf4llm.use_layout({layout}) failed: {exc}", file=sys.stderr)
            # The old converter logs this warning and continues with the
            # library's current layout mode.
    kwargs = {"use_ocr": ocr} if layout else {}
    with _quiet_stdout():
        try:
            result = pymupdf4llm.to_markdown(str(path), **kwargs)
        except TypeError:
            result = pymupdf4llm.to_markdown(str(path))
    return result if isinstance(result, str) else "", {
        "ocr": "enabled" if ocr else "disabled",
        "layout": "enabled" if layout else "disabled",
        "models": "requested" if (layout or ocr) else "not-requested",
    }


def convert_office(path: pathlib.Path) -> str:
    with _quiet_stdout():
        return _docling().convert(str(path)).document.export_to_markdown() or ""


def convert_csv(path: pathlib.Path) -> str:
    from markitdown import MarkItDown

    with _quiet_stdout():
        return MarkItDown().convert(str(path)).text_content or ""


def convert_json(path: pathlib.Path) -> str:
    try:
        value = json.loads(path.read_text(encoding="utf-8", errors="ignore"))
        pretty = json.dumps(value, indent=2, ensure_ascii=False)
    except Exception as exc:
        print(f"JSON parse failed; passing through raw text: {exc}", file=sys.stderr)
        pretty = path.read_text(encoding="utf-8", errors="ignore")
    return "```json\n" + pretty.strip() + "\n```\n"


def _flag(name: str) -> bool:
    return os.environ.get(name, "").casefold() in {"1", "true", "yes", "on"}


_PDF_OCR = _flag("HARVESTER_PDF_OCR")
_PDF_LAYOUT = _flag("HARVESTER_PDF_LAYOUT")
_DOCLING = None


def _docling():
    global _DOCLING
    if _DOCLING is None:
        from docling.document_converter import DocumentConverter

        _DOCLING = DocumentConverter()
    return _DOCLING


def _kind(path: pathlib.Path, declared: str) -> str:
    if declared:
        return declared.casefold().lstrip(".")
    suffixes = [s.casefold().lstrip(".") for s in path.suffixes]
    if suffixes[-2:] == ["tar", "gz"]:
        return "tar"
    return suffixes[-1] if suffixes else ""


def convert(request: dict) -> dict:
    path = pathlib.Path(request["path"])
    declared = str(request.get("kind", ""))
    kind = _kind(path, declared)
    try:
        if kind in {"html", "htm"}:
            markdown, features = convert_html(path), {}
        elif kind == "pdf":
            markdown, features = convert_pdf(path, request)
        elif kind in {"docx", "xlsx", "pptx"}:
            markdown, features = convert_office(path), {}
        elif kind == "csv":
            markdown, features = convert_csv(path), {}
        elif kind == "json":
            markdown, features = convert_json(path), {}
        else:
            raise ValueError(f"unsupported conversion kind: {kind!r}")
    except Exception as exc:
        # convert_local_file in the old Harvester catches converter failures
        # and returns its clean EMPTY-text contract; never leak a library
        # traceback as the protocol error.
        print(f"conversion failed for kind={kind!r}: {type(exc).__name__}: {exc}", file=sys.stderr)
        markdown, features = "", {}
    markdown = tidy_markdown(markdown)
    return {"ok": True, "markdown": markdown, "kind": kind, "features": features}


def smoke() -> dict:
    modules = ("trafilatura", "pymupdf4llm", "docling", "markitdown")
    imports = {}
    for module in modules:
        try:
            loaded = __import__(module)
            imports[module] = {"ok": True, "version": str(getattr(loaded, "__version__", ""))}
        except Exception as exc:
            imports[module] = {"ok": False, "error": f"{type(exc).__name__}: {exc}"}
    conversion = ""
    if all(item["ok"] for item in imports.values()):
        import tempfile

        with tempfile.NamedTemporaryFile("w", suffix=".html", encoding="utf-8") as fixture:
            fixture.write("<html><body><h1>harvestpy smoke</h1><p>ok</p></body></html>")
            fixture.flush()
            conversion = convert_html(pathlib.Path(fixture.name))
    return {
        "ok": all(item["ok"] for item in imports.values()),
        "python": sys.version.split()[0],
        "imports": imports,
        "conversion": {"ok": bool(conversion), "chars": len(conversion), "marker": "harvestpy smoke" if "harvestpy smoke" in conversion else ""},
        "features": {"ocr": "disabled", "layout": "disabled", "models": "not-requested"},
    }


def main() -> int:
    for line in sys.stdin:
        if not line.strip():
            continue
        try:
            request = json.loads(line)
            result = smoke() if request.get("op") == "smoke" else convert(request)
        except Exception as exc:
            print(json.dumps({"ok": False, "error": f"{type(exc).__name__}: {exc}"}), flush=True)
            continue
        print(json.dumps(result, ensure_ascii=False), flush=True)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
