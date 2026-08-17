"""Render one get_or_fetch result as MCP TextContent: success → header + full body;
failure → one clear, descriptive error line.
"""

import json
import os

from mcp.types import TextContent

from .cache import THIN_MIN_CHARS, read_frontmatter
from .log import get_logger
from .net import failure_message
from .tokens import estimate_tokens

log = get_logger("describe")

DEFAULT_MAX_INLINE_CHARS = 50000


def _max_inline_chars() -> int:
    """Inline-body cap in chars (env HARVESTER_MAX_INLINE_CHARS); the cache file keeps the full text."""
    raw = os.environ.get("HARVESTER_MAX_INLINE_CHARS")
    if raw is None:
        return DEFAULT_MAX_INLINE_CHARS
    try:
        return int(raw)
    except ValueError:
        log.warning("invalid HARVESTER_MAX_INLINE_CHARS=%r; using default %d", raw, DEFAULT_MAX_INLINE_CHARS)
        return DEFAULT_MAX_INLINE_CHARS


def _tokens_and_fetched_at(result: dict, body: str) -> "tuple[int, str]":
    """Read `tokens` (token_count) and `fetched_at` from the cache frontmatter when available.

    Both are already computed and written by `cache._write_md` at fetch time; reading them back
    means the header never re-derives a number that could drift from what's on disk. Falls back
    to a fresh estimate / "unknown" for artifacts with no frontmatter (images, archive listings).
    """
    md_path = result.get("md_path")
    meta = read_frontmatter(md_path) if md_path else {}
    try:
        tokens = int(meta["token_count"])
    except (KeyError, ValueError, TypeError):
        tokens = estimate_tokens(body)
    fetched_at = meta.get("fetched_at") or "unknown"
    return tokens, fetched_at


def _rungs_suffix(result: dict) -> str:
    """` / rungs: direct, chrome-impersonation, jina` when the artifact's frontmatter recorded a
    Slice 2 rescue-graph trace of more than one rung — omitted for the common single-rung case
    so the header stays quiet when there's nothing rescue-worthy to report."""
    md_path = result.get("md_path")
    meta = read_frontmatter(md_path) if md_path else {}
    rungs = meta.get("rungs")
    return f" / rungs: {rungs}" if rungs else ""


def describe_fetch_result(item: str, result: "dict | BaseException") -> TextContent:
    """Render one result as TextContent: success → header + full body; failure → one clear line."""
    if isinstance(result, BaseException):
        detail = str(result).strip() or "unknown error"
        log.warning("describe %s exception: %s: %s", item, type(result).__name__, detail)
        return TextContent(type="text", text=f"# {item}\nERROR: could not fetch — {type(result).__name__}: {detail}")

    if result.get("error"):
        log.info("describe %s error: %s", item, result["error"])
        return TextContent(type="text", text=f"# {item}\nERROR: {result['error']}")

    body = result.get("body") or ""
    stripped = body.strip()
    status = result.get("http_status")
    error_kind = result.get("error_kind")
    challenge = bool(result.get("challenge"))
    core_chars = result.get("content_chars")
    thin = (core_chars if core_chars is not None else len(stripped)) < THIN_MIN_CHARS
    negative = challenge or error_kind is not None or (status is not None and status >= 400)

    if stripped and not (thin and negative):
        tokens, fetched_at = _tokens_and_fetched_at(result, body)
        header = (
            f"# {item}\n"
            f"cache_status: {result['cache_status']} / method: {result['method']} / "
            f"bytes: {result['bytes']} / tokens: {tokens} / fetched_at: {fetched_at} / "
            f"path: {result['md_path']}{_rungs_suffix(result)}"
        )
        cap = _max_inline_chars()
        if cap > 0 and len(body) > cap:
            note = (
                f"\n\n— [truncated: first {cap} of {len(body)} chars. COMPLETE text is at "
                f"{result['md_path']} — read that file from char {cap} for the rest. "
                "`searchCache` locates WHICH cached pages match a pattern; it does not return "
                "text.]"
            )
            body = body[:cap] + note
        return TextContent(type="text", text=f"{header}\n\n{body}")

    msg = failure_message(item, status, error_kind, challenge)
    if msg is None:
        msg = (f"Fetched {item} but no readable content could be extracted (JS-rendered or "
               "bot-blocked — not retrievable from this datacenter IP). Use `search` to find "
               "an alternative copy, or `findWorks` if it is a scholarly title.")
    log.info("describe %s -> failure: %s", item, msg)
    return TextContent(type="text", text=f"# {item}\nERROR: {msg}")


def describe_size_result(item: str, result: "dict | BaseException") -> TextContent:
    """Render a `fetch(size_only=True)` probe: NO body — just the size + cache path, as JSON.

    The full content was still fetched and cached (same entry a normal fetch reuses); the probe
    returns only what a scheduler needs to plan reader windows: `size` (estimated TOKENS via the
    over-counting heuristic), raw `chars`, and the cache `path` to slice from disk. A failed /
    walled / thin source falls back to the normal error rendering so it stays visible.
    """
    if isinstance(result, BaseException) or (isinstance(result, dict) and result.get("error")):
        return describe_fetch_result(item, result)
    body = result.get("body") or ""
    if not body.strip():
        # Content was fetched but extracted to nothing — never report a silent size 0; surface it
        # as an explicit error so a scheduler doesn't treat an empty stub as a zero-token document.
        log.info("size_only %s -> empty body, reporting as error", item)
        return describe_fetch_result(item, {"error": (
            f"Fetched {item} but it yielded no readable content (empty after extraction) — "
            "nothing to size. Use `search` to find an alternative copy, or `findWorks` if it "
            "is a scholarly title."), "body": ""})
    tokens = estimate_tokens(body)  # over-counting heuristic; safe to budget against
    payload = {
        "source": item,
        "size": tokens,         # estimated TOKENS — the budget number a scheduler plans against
        "tokens": tokens,       # explicit alias of `size`
        "token_count": tokens,  # matches the `token_count` field written to cache frontmatter
        "chars": len(body),     # raw character count
        "path": str(result.get("md_path") or ""),
        "cache_status": result.get("cache_status"),
    }
    log.info("size_only %s -> tokens=%d chars=%d", item, tokens, payload["chars"])
    return TextContent(type="text", text=json.dumps(payload, ensure_ascii=False))
