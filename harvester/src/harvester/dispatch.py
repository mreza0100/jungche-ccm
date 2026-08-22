"""The dispatcher: route ONE input (URL / local path / `archive::member`) to its handler,
walk the wall-bypass ladder for HTML, and build the per-item result dict. Never raises —
failures come back as {"error": ...}.
"""

import asyncio
import os
import re
import tempfile
import time
from contextvars import ContextVar
from enum import Enum
from pathlib import Path
from typing import NamedTuple
from urllib.parse import unquote, urlparse

from . import cache, convert, detect, html, images, mirror, net, oa, safe_archive, search, stats
from .cache import CACHE_MIN_BODY, THIN_MIN_CHARS
from .log import get_logger

log = get_logger("dispatch")

# Opt-in figure localisation for `fetch` (HARVESTER_LOCALIZE_IMAGES=1): download every image a
# successfully fetched page references into the cache and rewrite the markdown links to local
# paths the model can read with vision. Off by default — `fetch`'s contract stays "image refs
# remain URLs"; see AGENTS.md § Conventions (images are bifurcated by `media`).
_LOCALIZE_IMAGES = convert._envflag("HARVESTER_LOCALIZE_IMAGES")

# Opt-in real-browser rung (HARVESTER_BROWSER=1) for the residual bot-wall class that survives
# httpx + curl_cffi + Jina. Needs the optional `browser` extra (patchright + a system Chrome);
# without it the rung is a no-op and the ladder is unchanged.
_BROWSER_ENABLED = convert._envflag("HARVESTER_BROWSER")

_URL_RE = re.compile(r"^https?://", re.IGNORECASE)
_SCHEME_RE = re.compile(r"^[a-z][a-z0-9+.-]*://", re.IGNORECASE)
# Bare scholarly identifiers handled by get_or_fetch's no-URL/no-DOI branch.
_PMCID_RE = re.compile(r"^PMC\d+$", re.IGNORECASE)
_PMID_RE = re.compile(r"^\d{7,9}$")

# ── negative-result cache + in-flight dedup (see get_or_fetch) ──────────────────
# A failing source must not be re-hammered: one DOI was fetched 18× in a real run.
DEFAULT_NEG_TTL = 120.0
_NEG_CACHE: dict[str, tuple[float, dict, float]] = {}
_INFLIGHT: dict[str, "asyncio.Future[dict]"] = {}

# Request-scoped force-refresh, set once by `get_or_fetch`.
#
# A ContextVar rather than a parameter threaded through the handlers on purpose: the cache is
# consulted at SIX depths (html, doc, doi-mirror, and the three binary download paths), several of
# them reached through recursive calls. A parameter would have to be passed correctly at every one
# of those sites, and a single missed site fails SILENTLY — `refresh=True` that quietly serves the
# cache anyway. Reading it from the context means every present and future cache site honours it by
# construction. asyncio copies the context into each task, so concurrent fetches with different
# refresh values never see each other's setting.
_FORCE_REFRESH: ContextVar[bool] = ContextVar("harvester_force_refresh", default=False)


def force_refresh() -> bool:
    """True when the in-flight request asked to bypass the cache."""
    return _FORCE_REFRESH.get()


def _may_serve(md_path: Path, kind: str, meta: "dict | None" = None) -> bool:
    """Whether a cached artifact may be served: refresh off AND not aged out."""
    if force_refresh():
        log.info("refresh requested — bypassing cache for %s", md_path)
        return False
    return not cache.is_stale(md_path, kind, meta)


def _may_reuse_binary(bin_path: Path) -> bool:
    """Whether an already-downloaded binary may be reused.

    Binaries (the source PDF/image/archive) carry no frontmatter, so there is no TTL to apply —
    only an explicit refresh forces a re-download.
    """
    if not bin_path.exists():
        return False
    if force_refresh():
        log.info("refresh requested — re-downloading %s", bin_path)
        return False
    return True


def _env_float(name: str, default: float) -> float:
    """Read a float env var ONCE at import time — module-level constants below are the value;
    tests monkeypatch the constant itself (see CLAUDE.md § Config), not the environment."""
    raw = os.environ.get(name)
    if raw is None:
        return default
    try:
        return float(raw)
    except ValueError:
        log.warning("invalid %s=%r; using default %s", name, raw, default)
        return default


# R7: a timeout/connect/429 failure is likely to clear up soon — don't hold a live source dead
# for the full negative-cache window. Read once at import time (unlike HARVESTER_NEG_TTL, which
# stays dynamic via `_neg_ttl()` for backward compatibility with existing tests/behavior).
HARVESTER_NEG_TTL_TRANSIENT = _env_float("HARVESTER_NEG_TTL_TRANSIENT", 15.0)
_TRANSIENT_ERROR_KINDS = {"timeout", "connect"}


def _neg_ttl() -> float:
    """Negative-cache TTL in seconds (env HARVESTER_NEG_TTL); invalid → default."""
    raw = os.environ.get("HARVESTER_NEG_TTL")
    if raw is None:
        return DEFAULT_NEG_TTL
    try:
        return float(raw)
    except ValueError:
        log.warning("invalid HARVESTER_NEG_TTL=%r; using default %s", raw, DEFAULT_NEG_TTL)
        return DEFAULT_NEG_TTL


def _is_transient_error(result: dict) -> bool:
    """True for a failure likely to resolve on its own soon (R7): a slow/blocked connection or a
    rate limit. Relies on `http_status`/`error_kind` being carried on the error dict (`_net_error`
    sets both); an error with neither (a content-classification failure, e.g. a challenge or a
    'no open-access copy' exhaustion) is never transient — it won't look different in 15s."""
    if result.get("http_status") == 429:
        return True
    return result.get("error_kind") in _TRANSIENT_ERROR_KINDS


def _neg_cache_key(item: str, media: str) -> str:
    """The negative-cache/in-flight key for *item*, canonicalizing DOI forms (R7) so a bare DOI,
    a ``doi:`` prefix, and a ``doi.org`` URL share ONE entry instead of each independently
    re-running (and re-failing) the same OA chain."""
    doi = _extract_doi_from_input(item)
    ident = f"doi:{doi}" if doi else item
    return f"{media}\x00{ident}"


def _neg_cache_get(key: str) -> dict | None:
    """Return a COPY of a fresh cached error dict for *key* (annotated), evicting it if stale."""
    entry = _NEG_CACHE.get(key)
    if entry is None:
        return None
    ts, result, ttl = entry
    age = time.monotonic() - ts
    if age >= ttl:
        _NEG_CACHE.pop(key, None)
        return None
    cached = dict(result)
    if cached.get("error"):
        retry_after = max(0, round(ttl - age))
        cached["error"] = f"{cached['error']} (recently failed; cached — retry after {retry_after}s)"
    return cached


def _neg_cache_evict_stale() -> None:
    """Drop expired negative-cache entries so the dict can't grow without bound."""
    now = time.monotonic()
    for k in [k for k, (ts, _r, ttl) in _NEG_CACHE.items() if now - ts >= ttl]:
        _NEG_CACHE.pop(k, None)


# ── rescue-graph: outcome classifier + Attempt trace (Slice 2) ──────────────────
# Replaces the scattered ad-hoc "thin or challenge" / "no bytes" checks that used to be
# re-derived slightly differently in each branch — one function, one vocabulary, so
# `_html_result`, `_doc_result`, and the OA-candidate paths all agree on what a rung's result
# WAS. See harvester-redesign.md §1.

class Outcome(str, Enum):
    OK = "ok"
    THIN = "thin"
    CHALLENGE = "challenge"
    HTTP_4XX_WITH_BODY = "http_4xx_with_body"
    WRONG_KIND = "wrong_kind"
    EMPTY_CONVERT = "empty_convert"
    DEAD_NET = "dead_net"
    NOT_FOUND_PAGE = "not_found_page"


class Attempt(NamedTuple):
    """One rescue-graph rung actually tried: which rung, what target URL/id, what it yielded."""
    rung: str
    target: str
    outcome: Outcome


def classify_outcome(
    *,
    body_len: int = 0,
    content_chars: int | None = None,
    status: int | None = None,
    error_kind: str | None = None,
    challenge: bool = False,
    not_found: bool = False,
    wrong_kind: bool = False,
    empty_convert: bool = False,
    thin_min: int = THIN_MIN_CHARS,
) -> Outcome:
    """Classify one fetch/convert attempt into a single Outcome.

    Precedence (most specific/actionable first): an explicit `empty_convert`/`wrong_kind` flag
    from the caller (already definitively known, e.g. a magic-byte mismatch) wins outright; else
    `dead_net` when there are literally no bytes to work with (a real connection failure, or a
    response with an empty body regardless of status — R1 keeps ≥400 bodies, so an EMPTY 4xx
    still lands here, matching pre-R1 behavior); else a detected bot/challenge page; a known
    not-found page; a ≥400 status that DID carry a body (R1's `http_4xx_with_body`); a body too
    short to be useful (`thin`); otherwise `ok`.
    """
    if empty_convert:
        return Outcome.EMPTY_CONVERT
    if wrong_kind:
        return Outcome.WRONG_KIND
    if body_len <= 0 and (status is None or status >= 400):
        return Outcome.DEAD_NET
    if challenge:
        return Outcome.CHALLENGE
    if not_found:
        return Outcome.NOT_FOUND_PAGE
    if status is not None and status >= 400:
        return Outcome.HTTP_4XX_WITH_BODY
    chars = content_chars if content_chars is not None else body_len
    if chars < thin_min:
        return Outcome.THIN
    return Outcome.OK


def _rungs_summary(trace: "list[Attempt]") -> str:
    """Raw, comma-joined rung names for cache frontmatter / the result header — informational
    provenance, so left un-aggregated (unlike `_rungs_phrase`, used only in terminal errors)."""
    return ", ".join(a.rung for a in trace)


def _rungs_phrase(trace: "list[Attempt]") -> str:
    """A compact 'direct, chrome-impersonation, jina, oa-mirror(2 sources), wayback' phrase for
    terminal error text — consecutive `oa:*` candidate rungs collapse into one `oa-mirror(N
    source(s))` segment so an exhausted 7-source OA ladder doesn't read as noise."""
    parts: list[str] = []
    i, n = 0, len(trace)
    while i < n:
        rung = trace[i].rung
        if rung.startswith("oa:"):
            j = i
            count = 0
            while j < n and trace[j].rung.startswith("oa:"):
                count += 1
                j += 1
            parts.append(f"oa-mirror({count} source{'s' if count != 1 else ''})")
            i = j
        else:
            parts.append(rung)
            i += 1
    return ", ".join(parts)


def _with_rungs(message: str, trace: "list[Attempt]") -> str:
    """Append the rescue-graph trace to a terminal error — only once more than one rung ran (a
    single-rung failure already names its own cause; the trace only adds value once the model
    might otherwise think 're-fetching' is worth a turn)."""
    if len(trace) <= 1:
        return message
    return f"{message} Rungs tried: {_rungs_phrase(trace)} — re-fetching will not help."


def _finalize_with_rungs(res: "dict | None", trace: "list[Attempt]") -> "dict | None":
    """Patch the accumulated rescue-graph trace into an artifact written by a helper that had no
    visibility into `trace` itself (`_handle_binary_doc` → `_doc_result`/`_image_result`, called
    from `_candidate_to_result`). A no-op when there's nothing to add or the path isn't a cached
    `.md` artifact with frontmatter (e.g. a bare image path)."""
    if not res or res.get("error") or len(trace) <= 1:
        return res
    md_path = res.get("md_path")
    if not isinstance(md_path, Path) or md_path.suffix != ".md":
        return res
    try:
        text = md_path.read_text(encoding="utf-8", errors="ignore")
    except OSError:
        return res
    lines = text.splitlines(keepends=True)
    if not lines or lines[0].rstrip("\n") != "---":
        return res
    end_idx = None
    for i in range(1, len(lines)):
        if lines[i].rstrip("\n") == "---":
            end_idx = i
            break
    if end_idx is None or any(ln.startswith("rungs:") for ln in lines[1:end_idx]):
        return res
    lines.insert(end_idx, f"rungs: {_rungs_summary(trace)}\n")
    try:
        md_path.write_text("".join(lines), encoding="utf-8")
    except OSError as e:
        log.warning("could not annotate rungs onto %s: %s", md_path, e)
    return res


# ── result builders ───────────────────────────────────────────────────────────
def _hit(md_path: Path, method: str, body: str) -> dict:
    return {
        "cache_status": "hit", "method": method, "md_path": md_path, "body": body,
        "bytes": md_path.stat().st_size, "content_chars": len(body.strip()),
        "http_status": None, "error_kind": None, "challenge": False,
    }


def _ok(md_path, method: str, body: str, content_chars: int | None = None,
        http_status=None, error_kind=None, challenge=False, cache_status="miss") -> dict:
    try:
        size = os.path.getsize(md_path) if not isinstance(md_path, Path) else md_path.stat().st_size
    except OSError:
        size = len(body)
    return {
        "cache_status": cache_status, "method": method, "md_path": md_path, "body": body,
        "bytes": size, "content_chars": content_chars if content_chars is not None else len(body.strip()),
        "http_status": http_status, "error_kind": error_kind, "challenge": challenge,
    }


def _net_error(
    key: str, status: int | None, error_kind: str | None, trace: "list[Attempt] | None" = None
) -> dict:
    """Build the {"error": ...} dict for a download that returned no usable bytes.

    Shares its wording with `describe.describe_fetch_result` via `net.failure_message` so the
    same failure kind reads identically whether it's caught here (download-time) or classified
    later at render time. Carries `http_status`/`error_kind` on the dict itself so `get_or_fetch`
    can pick a negative-cache TTL (R7) without re-deriving them from the message text; embeds the
    rescue-graph trace (R2/R3 etc. rungs already walked) when more than one rung was tried.
    """
    msg = net.failure_message(key, status, error_kind)
    if msg is None:
        msg = f"Could not download {key} — try `search` for an alternative source."
    if trace:
        msg = _with_rungs(msg, trace)
    log.warning("net error %s status=%s kind=%s", key, status, error_kind)
    return {"error": msg, "body": "", "http_status": status, "error_kind": error_kind}


def _format_listing(members, key: str) -> str:
    lines = [
        f"# Archive: {key}", "",
        f"{len(members)} member(s). Fetch one with `archive(source={key!r}, "
        'member="<name>")` — pick a name from the table below.', "",
        "| name | size (bytes) | type |", "| --- | --- | --- |",
    ]
    for m in members:
        typ = "dir" if m.is_dir else ("symlink" if m.is_symlink else "file")
        safe_name = m.name.replace("|", "\\|")  # don't let a '|' in a name break the table
        lines.append(f"| {safe_name} | {m.uncompressed_size} | {typ} |")
    return "\n".join(lines) + "\n"


# ── per-kind handlers ─────────────────────────────────────────────────────────
async def _handle_binary_doc(
    data: bytes, src: str, key: str, kind: str, user_agent: str, proxy_url: str | None
) -> dict:
    """Save already-downloaded binary content and route to the correct local handler.

    Avoids a second HTTP download by writing the bytes to the cache path first, then
    calling the appropriate local converter. Used when `_html_result` discovers that an
    extensionless URL (e.g. https://arxiv.org/pdf/1706.03762) is actually a PDF/etc.
    """
    log.info("binary-doc sniff %s kind=%s (%d bytes)", key, kind, len(data))
    if kind in detect.ARCHIVE_KINDS:
        ext = ".tar" if kind == "tar" else f".{kind}"
        bin_path = cache.cache_file(key, kind, ext)
        bin_path.write_bytes(data)
        return await _archive_result(str(bin_path), key, kind, "", True, user_agent, proxy_url)

    if kind == "image":
        head = data[:16]
        if head.startswith(b"\xff\xd8\xff"):
            ext = ".jpg"
        elif head.startswith(b"\x89PNG\r\n\x1a\n"):
            ext = ".png"
        elif head.startswith((b"GIF87a", b"GIF89a")):
            ext = ".gif"
        elif len(head) >= 12 and head[:4] == b"RIFF" and head[8:12] == b"WEBP":
            ext = ".webp"
        else:
            ext = ".png"
        img_path = cache.cache_file(key, ext.lstrip("."), ext)
        img_path.write_bytes(data)
        local_path = str(img_path)
        body = (
            f"![{Path(key).name}]({local_path})\n\n"
            f"*Image saved locally at `{local_path}` — read it directly for the visual content "
            f"(OCR/markdown cannot convey a figure).*\n"
        )
        return _ok(local_path, "image", body, content_chars=THIN_MIN_CHARS + 1)

    # DOC kinds: pdf, docx, xlsx, pptx, csv, json, epub
    _doc_ext = {"pdf": ".pdf", "docx": ".docx", "xlsx": ".xlsx", "pptx": ".pptx", "csv": ".csv",
                "json": ".json", "epub": ".epub"}
    ext = _doc_ext.get(kind, f".{kind}")
    bin_path = cache.cache_file(key, kind, ext)
    bin_path.write_bytes(data)
    return await _doc_result(str(bin_path), key, kind, True, user_agent, proxy_url)


async def _html_result(
    src, key, local, user_agent, proxy_url, trace: "list[Attempt] | None" = None
) -> dict:
    trace = trace if trace is not None else []
    md_path = cache.cache_file(key, "html", ".md")
    if md_path.exists():
        meta, body = cache.split_frontmatter(md_path.read_text(encoding="utf-8", errors="ignore"))
        if len(body.strip()) > CACHE_MIN_BODY and _may_serve(md_path, "html", meta):
            log.info("cache hit html %s", key)
            return _hit(md_path, meta.get("method", "cached"), body)

    if local:
        raw = Path(src).read_text(encoding="utf-8", errors="ignore")
        http_status, error_kind, challenge, ct = None, None, False, ""
    else:
        data, http_status, error_kind, ct = await net.fetch_bytes_with_meta(src, user_agent, proxy_url)
        # Extensionless URL sniff: if the server returns a binary document (e.g. arxiv PDF),
        # route to the correct LOCAL converter instead of trafilatura + Jina.
        if data:
            true_kind = detect._sniff_kind(ct, data[:16])
            if true_kind:
                return await _handle_binary_doc(data, src, key, true_kind, user_agent, proxy_url)
        raw = data.decode("utf-8", errors="replace") if data else ""
        challenge = net.looks_like_challenge(raw)

    # (a2) plain text (Gutenberg .txt, OCR dumps, source files, READMEs) is NOT HTML — trafilatura
    # would strip it to an empty stub (size_only then reports 0/0). Keep it verbatim and cache it,
    # so the real document survives and a repeat fetch is a cache hit.
    if raw.strip() and detect.is_plain_text(key, ct, raw):
        body = raw
        content_chars = len(body.strip())
        cache._write_md(md_path, key, "plain-text", body)
        log.info("plain-text %s chars=%d", key, content_chars)
        return _ok(md_path, "plain-text", body, content_chars=content_chars,
                   http_status=http_status, error_kind=error_kind)

    # (b) trafilatura extract from the initial response
    body = html.extract_content_from_html(raw)
    method = "local-trafilatura"
    content_chars = len(body.strip())
    if not local:
        trace.append(Attempt("direct", src, classify_outcome(
            body_len=len(raw), content_chars=content_chars, status=http_status,
            error_kind=error_kind, challenge=challenge)))
    log.info("html %s httpx -> %s chars=%d challenge=%s", key, method, content_chars, challenge)

    # (c) thin or Cloudflare challenge → retry with curl_cffi Chrome impersonation
    if not local and (content_chars < THIN_MIN_CHARS or challenge):
        log.info("html %s thin/challenge -> curl_cffi", key)
        cffi_text, _cffi_status = await net.fetch_impersonated(src, proxy_url)
        if cffi_text:
            cffi_body = html.extract_content_from_html(cffi_text)
            cffi_chars = len(cffi_body.strip())
            cffi_challenge = net.looks_like_challenge(cffi_text)
            trace.append(Attempt("chrome-impersonation", src, classify_outcome(
                body_len=len(cffi_text), content_chars=cffi_chars, status=_cffi_status,
                challenge=cffi_challenge)))
            if cffi_chars > content_chars:
                raw = cffi_text
                body = cffi_body
                content_chars = cffi_chars
                challenge = cffi_challenge
                method = "curl_cffi-trafilatura"
                log.info("html %s curl_cffi won chars=%d", key, content_chars)
        else:
            trace.append(Attempt("chrome-impersonation", src, Outcome.DEAD_NET))

    # (d) still thin → Jina Reader as final fallback (skips private hosts)
    if not local and content_chars < THIN_MIN_CHARS and not net.is_private_host(src):
        log.info("html %s still thin -> jina", key)
        jina_body = await net.fetch_jina(src, user_agent, proxy_url)
        jina_chars = len(jina_body.strip())
        if jina_chars > content_chars:
            body = jina_body
            content_chars = jina_chars
            method = "jina-reader"
            challenge = False  # Jina bypassed the wall
            trace.append(Attempt("jina", src, classify_outcome(
                body_len=len(jina_body), content_chars=jina_chars, status=200 if jina_body else None)))
            log.info("html %s jina won chars=%d", key, content_chars)
        else:
            trace.append(Attempt("jina", src, classify_outcome(
                body_len=len(jina_body), content_chars=jina_chars, status=200 if jina_body else None)))

    # (d1) defuddle.md — a second keyless reader beside Jina (different infra, different
    # blocks); same thin/challenge trigger, tried before the expensive browser rung.
    if not local and (content_chars < THIN_MIN_CHARS or challenge) and not net.is_private_host(src):
        log.info("html %s still thin/challenge -> defuddle", key)
        d_body = await net.fetch_defuddle(src, user_agent, proxy_url)
        d_chars = len(d_body.strip())
        if d_chars > content_chars:
            body = d_body
            content_chars = d_chars
            method = "defuddle-reader"
            challenge = False  # the reader bypassed the wall
            trace.append(Attempt("defuddle", src, Outcome.OK))
            log.info("html %s defuddle won chars=%d", key, content_chars)
        else:
            trace.append(Attempt("defuddle", src, classify_outcome(
                body_len=len(d_body), content_chars=d_chars,
                status=200 if d_body else None)))

    # (d2) browser rung (HARVESTER_BROWSER=1): render in a real system Chrome via Patchright.
    # Passes passive bot walls (TLS/H2 fingerprint + Cloudflare managed challenge) that no
    # HTTP-client trick can. Sits after Jina (cheap) and before the mirror chain (last resort).
    if (not local and _BROWSER_ENABLED
            and (content_chars < THIN_MIN_CHARS or challenge)
            and not net.is_private_host(src)):
        log.info("html %s still thin/challenge -> browser rung", key)
        b_html, b_status = await net.fetch_browser(src, proxy_url)
        if b_html:
            b_body = html.extract_content_from_html(b_html)
            b_challenge = net.looks_like_challenge(b_html)
            if len(b_body.strip()) > content_chars:
                raw, body, content_chars = b_html, b_body, len(b_body.strip())
                method = "browser-chrome"
                challenge = b_challenge
                trace.append(Attempt("browser", src, classify_outcome(
                    body_len=len(b_html), content_chars=content_chars, status=b_status,
                    challenge=b_challenge)))
                log.info("html %s browser rung won chars=%d", key, content_chars)
            else:
                trace.append(Attempt("browser", src, Outcome.THIN))
        else:
            trace.append(Attempt("browser", src, Outcome.DEAD_NET))

    # (e) mirror fallback: scholarly DOI embedded in URL, or Wayback snapshot
    # Only when the full ladder still yields thin content or a bot challenge.
    if not local and (content_chars < THIN_MIN_CHARS or challenge) and not net.is_private_host(src):
        log.info("html %s ladder exhausted -> mirror fallback", key)
        # R4: hand the mirror the best HTML text already in scope (httpx or curl_cffi, whichever
        # is current in `raw`) so it doesn't re-fetch a page that just proved to be walled.
        _mirror_res = await _try_mirror_for_url(src, key, user_agent, proxy_url, trace, page_html=raw)
        if _mirror_res:
            return _mirror_res

    # A bot/Cloudflare challenge that survived the WHOLE ladder and the OA mirror fallback is
    # poison: it would be cached + returned as a "success" (the live S0306987718301051 captcha
    # bug). Never cache it; return a clean error (which P1 then negative-caches).
    if not local and challenge:
        log.info("html %s -> unresolved bot/Cloudflare challenge", key)
        msg = _with_rungs(
            f"{key} is behind a bot/Cloudflare challenge (e.g. 'Are you a robot?') and no "
            "open-access copy was found — content not retrievable. Use `search` to find a "
            "mirror or alternative copy.", trace)
        return {"error": msg, "body": ""}

    # R6: a thin body from a ≥400 status ("soft 404") is a failure, not a near-empty success — it
    # must be classified as an error (neg-cached, no artifact written) symmetrically with every
    # other failure, instead of silently writing/returning a near-empty "ok" artifact.
    if not local and content_chars < THIN_MIN_CHARS and http_status is not None and http_status >= 400:
        log.info("html %s -> thin HTTP %s page after full ladder", key, http_status)
        return _net_error(key, http_status, error_kind, trace)

    # arXiv's "no article / invalid identifier" page extracts as a short success body — reject it
    # so a nonexistent id isn't returned (and cached) as a silent wrong document.
    if not local and _is_not_found_page(src, body):
        log.info("html %s -> not-found/error page", key)
        msg = _with_rungs(
            f"{key} returned a 'not found / invalid identifier' page — the "
            "resource does not exist. Double-check the identifier, or use "
            "`findWorks`/`search` to locate the correct one.", trace)
        return {"error": msg, "body": ""}

    # (f) metadata prefix, tidy, cache write. Image refs are LEFT as URLs — `fetch` never
    # downloads image binaries and never OCRs; the model views one on demand via `fetchImage`.
    meta_block = html.extract_metadata_block(raw)
    if meta_block:
        body = meta_block + body
    body = html.tidy_markdown(body)

    # Opt-in figure localisation (HARVESTER_LOCALIZE_IMAGES=1): rewrite `![](remote)` links to
    # local cache paths so the model can view figures directly. Only for a real (non-local,
    # non-challenge) fetch; a localization failure leaves the remote links untouched.
    if not local and not challenge and _LOCALIZE_IMAGES and "![" in body:
        try:
            localized = await images.localize_html_images(body, src, user_agent, proxy_url)
            if localized != body:
                log.info("html %s localized %d -> %d chars of image links", key, len(body), len(localized))
                body = localized
        except Exception as e:
            log.warning("image localisation failed %s (links left remote): %s", key, e)

    extra = {"rungs": _rungs_summary(trace)} if len(trace) > 1 else None
    cache._write_md(md_path, key, method, body, extra=extra)
    log.info("html %s done method=%s chars=%d", key, method, content_chars)
    return _ok(md_path, method, body, content_chars=content_chars,
               http_status=http_status, error_kind=error_kind, challenge=challenge)


async def _doc_result(
    src, key, kind, local, user_agent, proxy_url, trace: "list[Attempt] | None" = None
) -> dict:
    trace = trace if trace is not None else []
    md_path = cache.cache_file(key, kind, ".md")
    if md_path.exists():
        meta, body = cache.split_frontmatter(md_path.read_text(encoding="utf-8", errors="ignore"))
        if body.strip() and _may_serve(md_path, kind, meta):
            log.info("cache hit %s %s", kind, key)
            return _hit(md_path, meta.get("method", "cached"), body)

    if local:
        path = src
        rung = "local"
    else:
        ext = os.path.splitext(key.split("?", 1)[0])[1] or "." + kind
        bin_path = cache.cache_file(key, kind, ext)
        rung = "direct"  # default; reassigned below to whichever rung actually supplied bytes
        if not _may_reuse_binary(bin_path):
            data, status, error_kind = await net.download_bytes(src, user_agent, proxy_url)
            # R1 ripple guard: only the primary-URL HTML ladder may treat a ≥400 body as
            # potential content — here a ≥400 body is an HTML error/wall page, not the document,
            # and the binary cache is PERMANENT, so writing it would poison every later call.
            # Exception: a literal %PDF served with a 4xx status IS the requested document when
            # the kind is pdf (the magic proves it; some hosts mis-status real files). The error
            # page's TEXT is kept — only for the citation_pdf_url/citation_doi meta-scrape below.
            error_page = ""
            if (data and status is not None and status >= 400
                    and not (kind == "pdf" and data.startswith(b"%PDF"))):
                error_page = data.decode("utf-8", errors="ignore")
                data = b""
            if not data:
                outcome = (Outcome.HTTP_4XX_WITH_BODY if error_page
                           else classify_outcome(body_len=0, status=status, error_kind=error_kind))
                trace.append(Attempt("direct", src, outcome))
                # Retry with curl_cffi Chrome impersonation (handles walled CDNs / 403s)
                log.info("doc %s download empty/http-error -> curl_cffi", key)
                imp_data, imp_status = await net.download_impersonated(src, proxy_url)
                if imp_data:
                    data, status, error_kind, rung = imp_data, imp_status, None, "chrome-impersonation"
                else:
                    trace.append(Attempt("chrome-impersonation", src, Outcome.DEAD_NET))
            if not data:
                # A walled publisher PDF URL (e.g. tandfonline/sciencedirect/cell) usually carries
                # an extractable DOI — pivot to the open-access mirror before giving up. The 4xx
                # error page (if any) feeds the meta-scrape without a re-fetch (R4).
                mirror_res = await _try_mirror_for_url(src, key, user_agent, proxy_url, trace,
                                                       page_html=error_page or None)
                if mirror_res:
                    return mirror_res
                return _net_error(key, status, error_kind, trace)
            # Verify the bytes match the declared kind — a .pdf URL that serves an HTML wall must
            # NOT be fed to pymupdf and returned as a "successful" PDF (silent wrong document).
            if kind == "pdf" and not data.startswith(b"%PDF"):
                sniffed = detect.sniff_magic(data[:16])
                if sniffed:  # genuinely a different binary type → convert it correctly
                    return await _handle_binary_doc(data, src, key, sniffed, user_agent, proxy_url)
                # R2: the bytes in hand are (probably) an HTML wall/notice page, not the PDF —
                # they likely carry a citation_pdf_url/citation_doi <meta> tag. Try that chain
                # (reusing these bytes, no re-fetch) before giving up.
                trace.append(Attempt(rung, src, Outcome.WRONG_KIND))
                page_text = data.decode("utf-8", errors="ignore")
                mirror_res = await _try_mirror_for_url(src, key, user_agent, proxy_url, trace, page_html=page_text)
                if mirror_res:
                    return mirror_res
                msg = _with_rungs(
                    f"{key} has a .pdf address but did not return a PDF ({len(data)} bytes of non-PDF "
                    "content — likely an HTML paywall/login wall or a bot-block). Use `search` to "
                    "find an open-access copy.", trace)
                return {"error": msg, "body": ""}
            if trace:  # only note the winning rung when an earlier one already failed
                trace.append(Attempt(rung, src, Outcome.OK))
            bin_path.write_bytes(data)
        path = str(bin_path)

    if local and kind == "pdf":  # a local .pdf must actually be a PDF, not HTML/text renamed
        try:
            with open(path, "rb") as fh:
                if not fh.read(5).startswith(b"%PDF"):
                    return {"error": (
                        f"{key} has a .pdf extension but is not a PDF file — check its actual "
                        "contents before fetching it again."), "body": ""}
        except OSError as e:
            return {"error": f"cannot read {key}: {e} — check the path and permissions.", "body": ""}

    method = {
        "pdf": "pdf:pymupdf4llm", "docx": "office:docling", "xlsx": "office:docling",
        "pptx": "office:docling", "csv": "csv:markitdown", "json": "json",
        "epub": "book:markitdown",
    }[kind]
    body = await asyncio.to_thread(convert.convert_local_file, path, kind)
    body = html.tidy_markdown(body)
    if not body.strip():
        log.warning("doc %s (%s) converted to empty markdown", key, kind)
        # R3: pivot to the mirror chain before erroring — a scanned/corrupt copy at one URL
        # doesn't mean no open-access copy exists anywhere.
        if not local:
            trace.append(Attempt(rung, src, Outcome.EMPTY_CONVERT))
            mirror_res = await _try_mirror_for_url(src, key, user_agent, proxy_url, trace)
            if mirror_res:
                return mirror_res
            # LAST resort for a remote PDF: ONE forced-OCR pass. An empty text layer almost
            # always means a scanned document, and every faster rescue (a text copy elsewhere)
            # just failed. Slow, so it fires exactly when nothing else worked; a missing/broken
            # OCR backend is logged and treated as an empty result, never an exception.
            if kind == "pdf":
                log.info("doc %s mirror chain exhausted -> OCR escalation rung", key)
                ocr_body = ""
                try:
                    ocr_body = html.tidy_markdown(
                        await asyncio.to_thread(convert.pdf_to_md, path, True))
                except Exception as e:
                    log.warning("doc %s OCR backend failed: %s", key, e)
                if ocr_body.strip():
                    trace.append(Attempt("ocr", src, Outcome.OK))
                    extra = {"rungs": _rungs_summary(trace)} if len(trace) > 1 else None
                    cache._write_md(md_path, key, "pdf:pymupdf4llm-ocr", ocr_body, extra=extra)
                    log.info("doc %s -> OCR rung won (%d chars)", key, len(ocr_body))
                    return _ok(md_path, "pdf:pymupdf4llm-ocr", ocr_body)
                trace.append(Attempt("ocr", src, Outcome.EMPTY_CONVERT))
        hint = ""
        if kind == "pdf":
            hint = (" — an OCR pass was already attempted on this copy and produced nothing, "
                    "so the file is likely corrupt or password-protected" if not local
                    else " — if it's a scanned/image-only PDF, set HARVESTER_PDF_OCR=1 to OCR it")
        msg = (
            f"Downloaded the {kind.upper()} from {key} but it converted to EMPTY text. It is "
            f"likely scanned/image-only, corrupt, or password-protected{hint}. Use `search` to "
            "find an alternative copy.")
        if not local:
            msg = _with_rungs(msg, trace)
        return {"error": msg[:700], "body": ""}
    extra = {"rungs": _rungs_summary(trace)} if len(trace) > 1 else None
    cache._write_md(md_path, key, method, body, extra=extra)
    log.info("doc %s done method=%s", key, method)
    return _ok(md_path, method, body)


async def _image_result(src, key, local, user_agent, proxy_url) -> dict:
    ext = detect.image_ext(key)
    if local:
        local_path = str(Path(src).resolve())
        cache_status = "local"
    else:
        img_path = cache.cache_file(key, ext.lstrip("."), ext)
        if not _may_reuse_binary(img_path):
            data, status, error_kind = await net.download_bytes(src, user_agent, proxy_url)
            # R1 ripple guard: a ≥400 body here is an HTML error page, not the image — treat it
            # as no-data (never write it under an image extension for a vision read).
            if data and status is not None and status >= 400:
                data = b""
            if not data:
                # Retry with curl_cffi Chrome impersonation (handles walled image CDNs)
                imp_data, imp_status = await net.download_impersonated(src, proxy_url)
                if imp_data:
                    data, status, error_kind = imp_data, imp_status, None
            if not data:
                return _net_error(key, status, error_kind)
            img_path.write_bytes(data)
        local_path = str(img_path)
        cache_status = "miss"
    body = (
        f"![{Path(key).name}]({local_path})\n\n"
        f"*Image saved locally at `{local_path}` — read it directly for the visual content "
        f"(OCR/markdown cannot convey a figure).*\n"
    )
    log.info("image %s -> %s (%s)", key, local_path, cache_status)
    # high content_chars so the short body is never misread as a "thin" failure
    return _ok(local_path, "image", body, content_chars=THIN_MIN_CHARS + 1, cache_status=cache_status)


async def _member_to_result(data: bytes, member: str, key: str) -> dict:
    label = f"{key}::{member}"
    mkind = detect.detect_kind(member)
    if mkind == "html":
        sniffed = detect.sniff_magic(data[:16])
        if sniffed:
            mkind = sniffed

    if mkind == "image":
        ext = detect.image_ext(member)
        img_path = cache.cache_file(label, ext.lstrip("."), ext)
        img_path.write_bytes(data)
        body = (
            f"![{member}]({img_path})\n\n"
            f"*Image extracted from the archive at `{img_path}` — read it directly.*\n"
        )
        return _ok(str(img_path), "archive:member:image", body, content_chars=THIN_MIN_CHARS + 1)

    if mkind in detect.DOC_KINDS:
        suffix = os.path.splitext(member)[1] or "." + mkind
        with tempfile.NamedTemporaryFile(suffix=suffix, delete=False) as tf:
            tf.write(data)
            tmp = tf.name
        try:
            body = await asyncio.to_thread(convert.convert_local_file, tmp, mkind)
        finally:
            os.unlink(tmp)
        body = html.tidy_markdown(body)
    else:
        text = data.decode("utf-8", errors="replace")
        body = html.tidy_markdown(html.extract_content_from_html(text) or text) if mkind == "html" else text

    md_path = cache.cache_file(label, "archive_member", ".md")
    cache._write_md(md_path, label, f"archive:member:{mkind}", body)
    log.info("archive member %s kind=%s", label, mkind)
    return _ok(md_path, f"archive:member:{mkind}", body)


async def _archive_result(src, key, kind, member, local, user_agent, proxy_url) -> dict:
    if local:
        arc_path = src
        cache_status = "local"
    else:
        ext = os.path.splitext(key.split("?", 1)[0])[1] or ("." + ("tar" if kind == "tar" else kind))
        arc_cache = cache.cache_file(key, kind, ext)
        if not _may_reuse_binary(arc_cache):
            data, status, error_kind = await net.download_bytes(src, user_agent, proxy_url)
            # R1 ripple guard: a ≥400 body here is an HTML error page, not the archive — writing
            # it as a .zip/.tar would make safe_archive refuse the cached file forever after.
            if data and status is not None and status >= 400:
                data = b""
            if not data:
                # Retry with curl_cffi Chrome impersonation (handles walled archive downloads)
                imp_data, imp_status = await net.download_impersonated(src, proxy_url)
                if imp_data:
                    data, status, error_kind = imp_data, imp_status, None
            if not data:
                return _net_error(key, status, error_kind)
            arc_cache.write_bytes(data)
        arc_path = str(arc_cache)
        cache_status = "miss"

    if member:
        try:
            data = await asyncio.to_thread(safe_archive.read_archive_member, arc_path, member)
        except safe_archive.ArchiveError as e:
            return {"error": f"{e} — call `archive` on this source with no `member` to list what's inside.",
                    "body": ""}
        return await _member_to_result(data, member, key)

    try:
        members = await asyncio.to_thread(safe_archive.list_archive, arc_path)
    except safe_archive.ArchiveError as e:
        return {"error": (f"{e} — refusing to list this archive for safety. Try a different "
                          "source for its contents, or fetch individual files directly if "
                          "URLs are known."), "body": ""}
    log.info("archive %s -> %d member(s)", key, len(members))
    body = _format_listing(members, key)
    return _ok(arc_path, "archive:listing", body, content_chars=THIN_MIN_CHARS + 1,
               cache_status=cache_status)


def _file_url_to_path(u: str) -> str:
    return unquote(urlparse(u).path)


# ── mirror / DOI helpers ───────────────────────────────────────────────────────
def _extract_doi_from_input(item: str) -> str | None:
    """Return the DOI string if *item* is a DOI-type input, else None.

    Recognised forms: bare DOI (10.xxxx/...), ``doi:`` prefix,
    ``https://doi.org/<doi>`` and ``https://www.doi.org/<doi>``.
    """
    stripped = item.strip()
    if stripped.lower().startswith("doi:"):
        return mirror.extract_doi(stripped[4:])
    parsed = urlparse(stripped)
    if parsed.netloc.lower() in ("doi.org", "www.doi.org"):
        return mirror.extract_doi(parsed.path.lstrip("/"))
    # bare DOI: no URL scheme, starts with "10."
    if not _SCHEME_RE.match(stripped) and stripped.startswith("10."):
        return mirror.extract_doi(stripped)
    return None


async def _mirror_pdf_result(
    pdf_bytes: bytes, key: str, pmcid: str, user_agent: str, proxy_url: str | None,
    trace: "list[Attempt] | None" = None, method: str = "mirror:europepmc-pdf",
    figures_note: bool = True,
) -> dict | None:
    """Save *pdf_bytes* to the cache, convert to markdown, append figures note.

    Returns a result dict (method ``method``, default ``mirror:europepmc-pdf``) or None on
    failure. `figures_note=False` suppresses the Europe PMC figures ZIP pointer (it only
    makes sense for an actual Europe PMC copy).
    """
    trace = trace if trace is not None else []
    bin_path = cache.cache_file(key, "pdf", ".pdf")
    try:
        bin_path.write_bytes(pdf_bytes)
    except OSError as e:
        log.warning("mirror pdf cache write failed %s: %s", bin_path, e)
        return None
    body = await asyncio.to_thread(convert.pdf_to_md, str(bin_path))
    body = html.tidy_markdown(body)
    if not body.strip():
        log.warning("mirror pdf %s converted to empty markdown", key)
        return None
    if figures_note:
        figs_url = mirror.europepmc_figures_zip_url(pmcid)
        body += (
            f"\n\n---\n\n*Figures available as a ZIP: {figs_url}"
            " (fetch it to list+extract individual figures).*\n"
        )
    md_path = cache.cache_file(key, "pdf", ".md")
    extra = {"rungs": _rungs_summary(trace)} if len(trace) > 1 else None
    cache._write_md(md_path, key, method, body, extra=extra)
    log.info("mirror %s -> %s (%s)", key, method, pmcid)
    return _ok(md_path, method, body)


async def _pmcid_to_result(
    pmcid: str, key: str, user_agent: str, proxy_url: str | None,
    trace: "list[Attempt] | None" = None,
) -> dict | None:
    """PMCID → open-access content: Europe PMC full-text PDF, else the PMC article HTML.

    Returns a result dict, or None when neither yields usable content. The artifact is cached
    under *key*. Shared by the DOI mirror and the bare-PMID/PMCID routing.
    """
    trace = trace if trace is not None else []
    async with net._client(proxy_url) as client:
        pdf_bytes = await mirror.europepmc_pdf(pmcid, client)
    if pdf_bytes:
        result = await _mirror_pdf_result(pdf_bytes, key, pmcid, user_agent, proxy_url, trace)
        if result:
            trace.append(Attempt("oa:europepmc-pdf", pmcid, Outcome.OK))
            return result
        trace.append(Attempt("oa:europepmc-pdf", pmcid, Outcome.EMPTY_CONVERT))
    else:
        trace.append(Attempt("oa:europepmc-pdf", pmcid, Outcome.DEAD_NET))
    # Europe PMC had no renderable PDF — try the PMC OA service (oa.fcgi) for the direct
    # repository PDF (its ftp hrefs need the verified `deprecated/` rewrite; mirror.py).
    async with net._client(proxy_url) as client:
        oa_pdf_url = await mirror.pmc_oa_pdf_url(pmcid, client)
    if oa_pdf_url:
        data, status, _err = await net.download_bytes(oa_pdf_url, user_agent, proxy_url)
        if data and data.startswith(b"%PDF"):
            result = await _mirror_pdf_result(data, key, pmcid, user_agent, proxy_url, trace,
                                              method="mirror:pmc-oa-pdf", figures_note=False)
            if result:
                trace.append(Attempt("oa:pmc-oa-pdf", oa_pdf_url, Outcome.OK))
                return result
            trace.append(Attempt("oa:pmc-oa-pdf", oa_pdf_url, Outcome.EMPTY_CONVERT))
        else:
            trace.append(Attempt("oa:pmc-oa-pdf", oa_pdf_url,
                                 classify_outcome(body_len=len(data), status=status)))
    # PDF unavailable — try the PMC article HTML
    pmc_url = mirror.pmc_article_url(pmcid)
    pmc_raw = await net.fetch_raw(pmc_url, user_agent, proxy_url)
    if pmc_raw:
        pmc_body = html.extract_content_from_html(pmc_raw)
        pmc_chars = len(pmc_body.strip())
        if pmc_chars > THIN_MIN_CHARS:
            trace.append(Attempt("oa:pmc-html", pmc_url, Outcome.OK))
            pmc_body = html.tidy_markdown(pmc_body)
            md_path = cache.cache_file(key, "html", ".md")
            extra = {"rungs": _rungs_summary(trace)} if len(trace) > 1 else None
            cache._write_md(md_path, key, "mirror:pmc-html", pmc_body, extra=extra)
            log.info("pmcid %s -> pmc-html (%s)", pmcid, key)
            return _ok(md_path, "mirror:pmc-html", pmc_body)
        trace.append(Attempt("oa:pmc-html", pmc_url, classify_outcome(content_chars=pmc_chars, status=200)))
    else:
        trace.append(Attempt("oa:pmc-html", pmc_url, Outcome.DEAD_NET))
    return None


# ── open-access resolver: candidate-URL → content ───────────────────────────────
async def _candidate_to_result(
    cand: "oa.Candidate", key: str, user_agent: str, proxy_url: str | None,
    trace: "list[Attempt] | None" = None,
) -> dict | None:
    """Fetch ONE oa.Candidate URL (httpx → curl_cffi), verify it's real content, convert it.

    Returns a result dict, or None when the candidate yields nothing usable (so the caller tries
    the next one). The artifact is cached under *key* (the identifier), not the candidate URL.
    """
    trace = trace if trace is not None else []
    rung = f"oa:{cand.source}"
    # SSRF/scheme chokepoint for EVERY candidate (OA resolvers + the scraped citation_pdf_url):
    # candidate URLs come from third-party JSON / attacker-influenced page <meta> tags and bypass
    # the dispatch-time guard. Cheap lexical first filter here; the net layer resolves DNS + every
    # redirect hop. Refusing here also stops a file:// candidate from reaching the curl_cffi retry.
    _scheme = urlparse(cand.url).scheme.lower()
    if _scheme not in ("http", "https") or net.is_private_host(cand.url):
        log.warning("oa candidate refused (scheme/host) %s (%s): %s", cand.source, cand.kind_hint, cand.url)
        return None  # a security refusal, not a rung "attempt" — no trace entry

    data, status, error_kind, ct = await net.fetch_bytes_with_meta(cand.url, user_agent, proxy_url)
    # R1 ripple guard: only the primary-URL HTML ladder may treat a ≥400 body as potential
    # content. A candidate is a RESCUE URL — a ≥400 status here means "this rung failed, try the
    # next candidate", never "publish this body under the identifier key". One exception: a
    # literal %PDF served with a 4xx status IS the document (the magic proves it; some mirrors
    # mis-status real files). Discarding the body restores the pre-R1 no-bytes flow, so the
    # curl_cffi impersonation retry below still gets its shot at the real content.
    discarded_4xx = False
    if data and status is not None and status >= 400 and not data[:5].startswith(b"%PDF"):
        discarded_4xx = True
        data = b""
    if not data:  # walled or an error page? retry with a Chrome TLS/JA3 fingerprint
        data, imp_status = await net.download_impersonated(cand.url, proxy_url)
        if data:
            status, error_kind, discarded_4xx = imp_status, None, False
        ct = ""
    if not data:
        outcome = (Outcome.HTTP_4XX_WITH_BODY if discarded_4xx
                   else classify_outcome(body_len=0, status=status, error_kind=error_kind))
        trace.append(Attempt(rung, cand.url, outcome))
        log.info("oa candidate %s (%s) -> no usable bytes (status=%s)", cand.source, cand.url, status)
        return None

    head = data[:16]
    ct_base = (ct or "").split(";")[0].strip().lower()
    kind = "pdf" if (data[:5].startswith(b"%PDF") or ct_base == "application/pdf") else detect.sniff_magic(head)

    if kind in ("pdf", "image"):
        try:
            res = await _handle_binary_doc(data, cand.url, key, kind, user_agent, proxy_url)
        except Exception as e:
            log.warning("oa candidate convert failed %s: %s", cand.url, e)
            trace.append(Attempt(rung, cand.url, Outcome.EMPTY_CONVERT))
            return None
        if res and not res.get("error") and (res.get("content_chars") or 0) > 0:
            trace.append(Attempt(rung, cand.url, Outcome.OK))
            log.info("oa candidate %s (%s) -> %s", cand.source, cand.url, kind)
            return _finalize_with_rungs(res, trace)
        trace.append(Attempt(rung, cand.url, Outcome.EMPTY_CONVERT))
        return None

    if kind in detect.ARCHIVE_KINDS:
        # R5: a candidate serving a zip/7z/rar/tar has no text converter in this ladder —
        # decoding it as UTF-8 below would silently cache binary noise as a "successful" text
        # artifact. ONE exception: an EPUB is zip-SHAPED but is a book, and OA book sources
        # (OAPEN/DOAB/Gutenberg) serve EPUB constantly — detect it by its uncompressed
        # `mimetype` member and convert it instead of throwing the found book away.
        if kind == "zip" and detect.looks_like_epub(data):
            log.info("oa candidate %s (%s) -> zip bytes are an EPUB — converting", cand.source, cand.url)
            try:
                res = await _handle_binary_doc(data, cand.url, key, "epub", user_agent, proxy_url)
            except Exception as e:
                log.warning("oa candidate epub convert failed %s: %s", cand.url, e)
                trace.append(Attempt(rung, cand.url, Outcome.EMPTY_CONVERT))
                return None
            if res and not res.get("error") and (res.get("content_chars") or 0) > 0:
                trace.append(Attempt(rung, cand.url, Outcome.OK))
                return _finalize_with_rungs(res, trace)
            trace.append(Attempt(rung, cand.url, Outcome.EMPTY_CONVERT))
            return None
        log.info("oa candidate %s (%s) -> archive-kind %s, no text converter — rejected",
                 cand.source, cand.url, kind)
        trace.append(Attempt(rung, cand.url, Outcome.WRONG_KIND))
        return None

    # Otherwise treat as HTML / plain text (publisher landing page, Gutenberg text, OCR .txt).
    # Defense-in-depth for the R1 invariant: a ≥400 TEXT body is never published as the
    # identifier's content. (Normally already discarded at the download step above — the only
    # 4xx bytes kept there are %PDF, which take the binary branch — but this branch must hold
    # the invariant locally too, so a future byte source can't reintroduce the leak.)
    if status is not None and status >= 400:
        trace.append(Attempt(rung, cand.url, Outcome.HTTP_4XX_WITH_BODY))
        log.info("oa candidate %s (%s) -> HTTP %s text body rejected", cand.source, cand.url, status)
        return None
    text = data.decode("utf-8", "ignore")
    if not text.strip() or net.looks_like_challenge(text):
        trace.append(Attempt(rung, cand.url, Outcome.CHALLENGE if text.strip() else Outcome.DEAD_NET))
        log.info("oa candidate %s (%s) -> challenge/empty", cand.source, cand.url)
        return None
    low = text.lower()
    is_html = "<html" in low or "<!doctype html" in low or "<body" in low
    body = text if (cand.kind_hint == "txt" or not is_html) else html.extract_content_from_html(text)
    if len(body.strip()) <= THIN_MIN_CHARS:
        trace.append(Attempt(rung, cand.url, Outcome.THIN))
        return None
    trace.append(Attempt(rung, cand.url, Outcome.OK))
    body = html.tidy_markdown(body)
    md_path = cache.cache_file(key, "html", ".md")
    extra = {"rungs": _rungs_summary(trace)} if len(trace) > 1 else None
    cache._write_md(md_path, key, f"oa:{cand.source}", body, extra=extra)
    log.info("oa candidate %s (%s) -> %d chars", cand.source, cand.url, len(body))
    return _ok(md_path, f"oa:{cand.source}", body)


async def _resolve_and_fetch(
    candidates: "list[oa.Candidate]", key: str, label: str, user_agent: str, proxy_url: str | None,
    trace: "list[Attempt] | None" = None,
) -> dict | None:
    """Try each candidate in priority order; return the first that yields real content, else None."""
    for cand in candidates:
        res = await _candidate_to_result(cand, key, user_agent, proxy_url, trace)
        if res:
            log.info("%s %s -> %s (%s)", label, key, cand.url, cand.source)
            return res
    return None


async def _resolve_input(
    kind: str, value: str, key: str, user_agent: str, proxy_url: str | None
) -> dict:
    """Resolve an ISBN/book identifier to content through the OA chain (books are unambiguous)."""
    async with net._client(proxy_url) as client:
        cands = await oa.resolve_book(value, client)
        res = await _resolve_and_fetch(cands, key, kind, user_agent, proxy_url)
    if res:
        return res
    log.info("%s resolver found no free full-text for %r", kind, value)
    return {"error": (
        f"No free, legal full text found for ISBN {value!r} (checked OAPEN, Internet Archive, "
        "Project Gutenberg, and DOAB). It may be an in-copyright book with no open edition — try a "
        "library, or use the `findWorks` tool with the title to see candidate editions."), "body": ""}


async def search_web(
    query: str, count: int = 8, lang: str = "", engines: str = "", proxy_url: str | None = None
) -> "tuple[list[dict] | None, str | None]":
    """Web search via SearXNG (then Brave). Returns (results, backend); (None, None) = unconfigured."""
    from httpx import AsyncClient
    async with AsyncClient(proxy=proxy_url) as client:
        return await search.web_search(query, client, count, lang, engines)


async def find_sources(query: str, limit: int = 8, proxy_url: str | None = None) -> list[dict]:
    """A title / free-text query → a ranked list of candidate works (papers + books).

    Like WebSearch, but for scholarship: returns candidates to CHOOSE from (each with a `fetch`
    handle), it does not download. The caller fetches the chosen handle with `get_or_fetch`.
    """
    from httpx import AsyncClient
    async with AsyncClient(proxy=proxy_url) as client:
        return await oa.find_works(query, client, limit)


# A title is ambiguous — `fetch` won't guess which work you mean; it routes you to `findWorks`.
_FIND_HINT = (
    "is a title — use the `findWorks` tool to list candidate works (it returns a fetch handle for "
    "each), then fetch the one you pick. `fetch` retrieves locations and UNAMBIGUOUS identifiers "
    "(URL, file path, DOI, ISBN), never a title."
)


def _looks_like_title(s: str) -> bool:
    """True if a bare (non-URL/DOI/ISBN) string should be resolved as a paper/book title."""
    if s.startswith((".", "~", "-")) or "\\" in s:
        return False  # path-like
    if detect.detect_kind(s) != "html":
        return False  # has a recognized file extension → a (missing) path
    if "/" in s and " " not in s:
        return False  # slashy with no spaces → a relative path (docs/foo), not a title
    return bool(re.search(r"[A-Za-z]", s))


def _is_not_found_page(url: str, body: str) -> bool:
    """True if a fetched HTML body is a known 'resource does not exist' page (e.g. arXiv's)."""
    low = body.lower()
    if "arxiv.org" in url.lower():
        return any(m in low for m in (
            "no article for this identifier", "invalid arxiv identifier",
            "article identifier", "no article found"))
    return False


def _is_pubmed_search_url(url: str) -> bool:
    """True for a PubMed *search/results* URL (not a specific article) — these 404 on fetch.

    Narrow on purpose: host pubmed.ncbi.nlm.nih.gov AND (a `/search` path OR a `term=` query).
    An article URL like .../30220343/ has neither, so it is left to fetch normally.
    """
    try:
        parsed = urlparse(url)
    except Exception:
        return False
    if (parsed.hostname or "").lower() != "pubmed.ncbi.nlm.nih.gov":
        return False
    return "/search" in parsed.path.lower() or "term=" in (parsed.query or "").lower()


def _path_exists(s: str) -> bool:
    """Path.exists() that can't raise — a very long string raises OSError(ENAMETOOLONG)."""
    try:
        return Path(s).expanduser().exists()
    except OSError:
        return False


def _is_file(p: Path) -> bool:
    try:
        return p.is_file()
    except OSError:
        return False


async def _doi_mirror_result(
    doi: str, key: str, user_agent: str, proxy_url: str | None
) -> dict:
    """Resolve a DOI to open-access content via Europe PMC, PMC HTML, or Wayback.

    Tries in order: Europe PMC PDF → PMC article HTML → Wayback snapshot of doi.org URL.
    """
    trace: "list[Attempt]" = []
    log.info("doi mirror %s", doi)
    # Cache-first: a repeated DOI must not re-download + re-convert (~40s) or write a second,
    # divergent artifact. DOI artifacts are keyed by the bare DOI (dedup across input forms).
    for ck in (doi, key):
        for kind in ("pdf", "html"):
            cached = cache.cache_file(ck, kind, ".md")
            if cached.exists():
                meta, body = cache.split_frontmatter(cached.read_text(encoding="utf-8", errors="ignore"))
                if body.strip() and _may_serve(cached, kind, meta):
                    log.info("doi mirror cache hit %s (%s)", doi, kind)
                    return _hit(cached, meta.get("method", "cached"), body)
    async with net._client(proxy_url) as client:
        pmcid = await mirror.doi_to_pmcid(doi, client)
        if pmcid:
            res = await _pmcid_to_result(pmcid, doi, user_agent, proxy_url, trace)
            if res:
                return res

        # Full OA chain: Unpaywall / OpenAlex / Semantic-Scholar / CORE / DOAJ / arXiv / OSF.
        oa_res = await _resolve_and_fetch(
            await oa.resolve_doi(doi, client), doi, "doi-oa", user_agent, proxy_url, trace)
        if oa_res:
            return oa_res

        # Still nothing free — fall back to a Wayback snapshot of the doi.org URL
        doi_url = f"https://doi.org/{doi}"
        wb_url = await mirror.wayback_raw_url(doi_url, client)
        if wb_url:
            wb_raw = await net.fetch_raw(wb_url, user_agent, proxy_url)
            wb_body = html.extract_content_from_html(wb_raw) if wb_raw else ""
            wb_chars = len(wb_body.strip())
            if wb_chars > THIN_MIN_CHARS:
                trace.append(Attempt("wayback", wb_url, Outcome.OK))
                wb_body = html.tidy_markdown(wb_body)
                md_path = cache.cache_file(doi, "html", ".md")
                extra = {"rungs": _rungs_summary(trace)} if len(trace) > 1 else None
                cache._write_md(md_path, doi, "mirror:wayback", wb_body, extra=extra)
                log.info("doi mirror %s -> wayback", doi)
                return _ok(md_path, "mirror:wayback", wb_body)
            trace.append(Attempt("wayback", wb_url, classify_outcome(
                content_chars=wb_chars, status=200 if wb_raw else None)))
        else:
            trace.append(Attempt("wayback", doi_url, Outcome.DEAD_NET))

    log.warning("doi mirror %s -> no open-access source", doi)
    msg = _with_rungs(
        f"Found DOI {doi}, but no free, legal full text exists in any open-access source (checked "
        "Unpaywall, OpenAlex, Semantic Scholar, Europe PMC, OpenAIRE, Zenodo, CORE, DOAJ, "
        "arXiv/OSF, and the Wayback "
        "Machine). The paper is likely paywalled — use `search` to find an author preprint or the "
        "publisher's page directly.", trace)
    return {"error": msg, "body": ""}


async def _try_mirror_for_url(
    src: str, key: str, user_agent: str, proxy_url: str | None,
    trace: "list[Attempt] | None" = None, page_html: str | None = None,
) -> dict | None:
    """Wall fallback: extract DOI from a publisher URL and resolve via mirror, or try Wayback.

    Returns a result dict on success, or None when no better content is found.
    Only called for non-private, non-local URLs after the full httpx/curl_cffi/Jina ladder.

    R4: *page_html*, when supplied, is text the CALLER already fetched (e.g. the httpx/curl_cffi
    body from the html ladder, or the mismatched bytes from a `.pdf`-serves-HTML doc download) —
    reused for the citation_pdf_url/citation_doi meta-scrape instead of re-fetching a page that
    just proved to be walled. Only when nothing was passed in do we fetch ourselves, and then via
    curl_cffi Chrome impersonation (not plain httpx, which would likely hit the same wall again).
    """
    trace = trace if trace is not None else []
    doi = mirror.extract_doi(src)
    meta_pdf = None
    if not doi:
        page = page_html
        if not page:
            page, _status = await net.fetch_impersonated(src, proxy_url)
            trace.append(Attempt("meta-scrape", src, Outcome.OK if page else Outcome.DEAD_NET))
        if page:
            m_doi, meta_pdf, _title = oa.extract_meta_links(page)
            doi = doi or m_doi
    if doi:
        log.info("mirror url %s doi=%s", key, doi)
    async with net._client(proxy_url) as client:
        if meta_pdf:
            res = await _candidate_to_result(
                oa.Candidate(oa._score("citation_pdf_url"), meta_pdf, "citation_pdf_url", kind_hint="pdf"),
                key, user_agent, proxy_url, trace)
            if res:
                log.info("mirror url %s -> citation_pdf_url", key)
                return res
        if doi:
            pmcid = await mirror.doi_to_pmcid(doi, client)
            if pmcid:
                res = await _pmcid_to_result(pmcid, key, user_agent, proxy_url, trace)
                if res:
                    return res

        if doi:
            oa_res = await _resolve_and_fetch(
                await oa.resolve_doi(doi, client), key, "mirror-oa", user_agent, proxy_url, trace)
            if oa_res:
                return oa_res

        # No DOI found (or all OA candidates failed) — try a Wayback snapshot
        wb_url = await mirror.wayback_raw_url(src, client)
        if wb_url:
            wb_raw = await net.fetch_raw(wb_url, user_agent, proxy_url)
            wb_body = html.extract_content_from_html(wb_raw) if wb_raw else ""
            wb_chars = len(wb_body.strip())
            if wb_chars > THIN_MIN_CHARS:
                trace.append(Attempt("wayback", wb_url, Outcome.OK))
                wb_body = html.tidy_markdown(wb_body)
                md_path = cache.cache_file(key, "html", ".md")
                extra = {"rungs": _rungs_summary(trace)} if len(trace) > 1 else None
                cache._write_md(md_path, key, "mirror:wayback", wb_body, extra=extra)
                log.info("mirror url %s -> wayback", key)
                return _ok(md_path, "mirror:wayback", wb_body)
            trace.append(Attempt("wayback", wb_url, classify_outcome(
                content_chars=wb_chars, status=200 if wb_raw else None)))
        else:
            trace.append(Attempt("wayback", src, Outcome.DEAD_NET))

    log.info("mirror url %s -> no better content", key)
    return None


async def _dispatch_one(
    item: str, user_agent: str, proxy_url: str | None, media: str
) -> dict:
    """Dispatch ONE input (URL or local path, optionally `archive::member`) to its handler
    and return a result dict. Never raises — failures come back as {"error": ...}.

    `media="deny"` (used by the `fetch` tool) redirects images → `fetchImage` and archives →
    `archive`, keeping `fetch`'s contract pure (returns document markdown, not a path/listing).
    """
    log.info("get_or_fetch %s media=%s", item, media)
    base, sep, member = item.partition("::")
    member = member if sep else ""
    stripped = base.strip()
    low = stripped.lower()

    if not stripped:
        return {"error": (
            "empty input — pass a URL, a file path, a DOI, an ISBN, or a PMID/PMCID. For a "
            "TITLE, use the `findWorks` tool instead (it returns a fetch handle to pass here); "
            "`fetch` always errors on a bare title."), "body": ""}

    # A title is ambiguous — route it to the `findWorks` tool rather than guess which work it means.
    if low.startswith("title:"):
        val = stripped[len("title:"):].strip().strip('"').strip("'")
        log.info("routing -> find hint for title: %s", val)
        return {"error": f"{val!r} {_FIND_HINT}", "body": ""}
    # ISBN is unambiguous → fetch the book directly (after validating the checksum).
    if low.startswith("isbn:"):
        raw = stripped[len("isbn:"):].strip()
        norm = oa.normalize_isbn(raw)
        if not norm:
            return {"error": (
                f"{raw!r} is not a valid ISBN-10/13 (check the digits / check digit), or use "
                "the `findWorks` tool with the title instead."), "body": ""}
        log.info("routing -> book resolver (isbn): %s", norm)
        return await _resolve_input("isbn", norm, base, user_agent, proxy_url)

    # DOI fast-path: bare DOI (10.xxxx/...), doi: prefix, or doi.org URL
    _doi_str = _extract_doi_from_input(base)
    if _doi_str:
        log.info("routing %s -> doi mirror", base)
        return await _doi_mirror_result(_doi_str, base, user_agent, proxy_url)

    if low.startswith("file://"):
        local, src = True, _file_url_to_path(base)
    elif _URL_RE.match(base):
        local, src = False, base
    elif _SCHEME_RE.match(base):
        log.warning("unsupported URL scheme: %s", base)
        return {"error": (f"unsupported URL scheme in {base!r} — fetch handles http(s):// and "
                          "file:// URLs, local paths, DOIs, and ISBNs."), "body": ""}
    else:
        # Not a URL/scheme/DOI. A bare ISBN fetches a book directly; a bare title is ambiguous,
        # so point it at `findWorks`. Neither fires when the string is an existing local file/path.
        if not member and not _path_exists(stripped):
            bare_isbn = oa.normalize_isbn(stripped)
            if bare_isbn:
                log.info("routing -> book resolver (bare isbn): %s", bare_isbn)
                return await _resolve_input("isbn", bare_isbn, base, user_agent, proxy_url)
            # Bare scholarly IDs — checked BEFORE _looks_like_title because "PMC…" has letters
            # and would otherwise be misread as a title. A real local file already won above.
            if _PMCID_RE.match(stripped):
                pmcid = stripped.upper()
                log.info("routing -> pmcid resolver: %s", pmcid)
                res = await _pmcid_to_result(pmcid, base, user_agent, proxy_url)
                if res:
                    return res
                return {"error": (
                    f"{stripped} is a PMCID but no free full text was found in Europe PMC/PMC. "
                    "Try the `findWorks` tool."), "body": ""}
            if _PMID_RE.match(stripped):
                log.info("routing -> pmid resolver: %s", stripped)
                async with net._client(proxy_url) as client:
                    pmcid = await mirror.pmid_to_pmcid(stripped, client)
                res = await _pmcid_to_result(pmcid, base, user_agent, proxy_url) if pmcid else None
                if res:
                    return res
                return {"error": (
                    f"{stripped} looks like a PubMed ID but no open-access full text was found "
                    "(it may be abstract-only or paywalled). Try the `findWorks` tool with the title."),
                    "body": ""}
            if _looks_like_title(stripped):
                log.info("routing -> find hint for bare title: %s", stripped)
                return {"error": f"{stripped!r} {_FIND_HINT}", "body": ""}
        local, src = True, base

    # SSRF guard: never fetch a private / internal / link-local host (covers obfuscated IP forms).
    if not local and net.is_private_host(src):
        log.warning("refusing private/internal host: %s", src)
        return {"error": (
            f"refusing to fetch a private or internal host: {base} — harvester only fetches "
            "public internet resources; use the resource's public URL instead."), "body": ""}

    # A PubMed *search/results* URL is not an article — it 404s on fetch. Route to find/search.
    if not local and _is_pubmed_search_url(src):
        log.info("routing -> find/search hint for pubmed search url: %s", src)
        return {"error": (
            f"{base} is a PubMed search/results URL, not an article — use the `findWorks` tool (or "
            "`search`) to get candidate works, each with a fetch handle."), "body": ""}

    kind = detect.detect_kind(base)

    if local:
        p = Path(src).expanduser()
        try:
            rp = p.resolve()
        except OSError:
            rp = p
        # Deny on BOTH the path and its symlink target — a symlink with an innocent name must not
        # smuggle a secret (e.g. report.html → prod_creds.pem).
        reason = detect.deny_reason(p) or detect.deny_reason(rp)
        if reason:
            log.warning("deny %s: %s", p, reason)
            return {"error": reason, "body": ""}
        if not _is_file(rp):
            log.warning("local file not found: %s", base)
            return {"error": (
                f"local file not found: {base} — check the path for typos, or use `search`/"
                "`findWorks` if you meant to fetch it from the web instead."), "body": ""}
        if kind == "html":  # ambiguous local file — sniff magic bytes
            try:
                with rp.open("rb") as fh:
                    head = fh.read(16)
            except OSError as e:
                log.warning("cannot read %s: %s", rp, e)
                return {"error": f"cannot read {base}: {e} — check the path and permissions.", "body": ""}
            sniffed = detect.sniff_magic(head)
            if sniffed:
                kind = sniffed
        src = str(rp)

    log.info("dispatch %s kind=%s local=%s member=%r", base, kind, local, member)
    if media == "deny" and (kind == "image" or kind in detect.ARCHIVE_KINDS):
        tool = "fetchImage" if kind == "image" else "archive"
        what = "an image" if kind == "image" else f"a {kind} archive"
        log.info("fetch redirect: %s is %s -> %s tool", base, what, tool)
        return {"error": f"{base} is {what} — use the `{tool}` tool, not `fetch`.", "body": ""}
    try:
        if kind in detect.ARCHIVE_KINDS:
            return await _archive_result(src, base, kind, member, local, user_agent, proxy_url)
        if member:
            return {"error": (
                f"the '::' member syntax is only valid for archives, not {kind} — drop the "
                "'::member' suffix and fetch the source directly, or use "
                "`archive(source=..., member=...)` if it is actually an archive."), "body": ""}
        if kind == "image":
            return await _image_result(src, base, local, user_agent, proxy_url)
        if kind == "html":
            return await _html_result(src, base, local, user_agent, proxy_url)
        return await _doc_result(src, base, kind, local, user_agent, proxy_url)
    except Exception as e:  # converters/archive libs can raise — surface a clean per-item error
        log.exception("get_or_fetch %s failed: %s", base, e)
        return {"error": (
            f"{type(e).__name__}: {e} — try `search` for an alternative source, or retry."),
            "body": ""}


async def get_or_fetch(
    item: str, user_agent: str, proxy_url: str | None = None, media: str = "allow",
    refresh: bool = False,
) -> dict:
    """Dispatch ONE input to its handler (see `_dispatch_one`), with two guards against
    re-hammering a failing source:

    * **Negative cache** — a result with an ``"error"`` key is remembered for
      HARVESTER_NEG_TTL seconds (default 120; transient kinds — timeout/connect/HTTP 429 — get
      the shorter HARVESTER_NEG_TTL_TRANSIENT instead, R7); a repeat call inside the window
      returns a copy of that error instead of re-fetching. Successes are never cached. DOI-type
      inputs are canonicalized (bare DOI / ``doi:`` / ``doi.org`` URL share one entry) so the
      three forms of the same identifier don't each independently re-run the failed chain.
    * **In-flight dedup** — concurrent calls for the same (media, item) share ONE fetch.

    `refresh=True` bypasses BOTH guards and every on-disk cache site, re-fetches, overwrites the
    artifact, and returns the fresh copy. It bypasses the negative cache too: a caller explicitly
    asking to retry a source must not be handed the remembered failure from two minutes ago — that
    would make the flag look broken on exactly the source it exists for.

    Same signature/contract as the dispatch itself: never raises; failures are {"error": ...}.
    """
    key = _neg_cache_key(item, media)
    token = _FORCE_REFRESH.set(refresh)

    try:
        if not refresh:
            cached = _neg_cache_get(key)
            if cached is not None:
                log.info("get_or_fetch %s media=%s -> negative-cache hit", item, media)
                return cached

            # Only a non-refresh caller may join an in-flight fetch. A refresh must not be
            # satisfied by a request that is allowed to answer from cache.
            inflight = _INFLIGHT.get(key)
            if inflight is not None:
                log.info("get_or_fetch %s media=%s -> awaiting in-flight fetch", item, media)
                return await inflight
        else:
            _NEG_CACHE.pop(key, None)

        # Refreshing fetches run under their OWN in-flight key. Sharing the normal key would let a
        # refresh overwrite a concurrent plain request's future — and then pop that request's entry
        # on the way out, stranding whoever was awaiting it.
        return await _fetch_and_record(
            key, f"{key}#refresh" if refresh else key, item, user_agent, proxy_url, media
        )
    finally:
        _FORCE_REFRESH.reset(token)


async def _fetch_and_record(
    neg_key: str, inflight_key: str, item: str, user_agent: str,
    proxy_url: str | None, media: str,
) -> dict:
    """Run the dispatch under in-flight dedup, recording failures in the negative cache.

    `neg_key` identifies the SOURCE (a failure is worth remembering however it was requested);
    `inflight_key` identifies this request MODE, so refresh and non-refresh never share a future.
    """
    fut: "asyncio.Future[dict]" = asyncio.get_running_loop().create_future()
    _INFLIGHT[inflight_key] = fut
    try:
        result = await _dispatch_one(item, user_agent, proxy_url, media)
    except BaseException as e:  # _dispatch_one shouldn't raise, but never leak the future
        if not fut.done():
            fut.set_exception(e)
        raise
    else:
        if isinstance(result, dict) and result.get("error"):
            ttl = HARVESTER_NEG_TTL_TRANSIENT if _is_transient_error(result) else _neg_ttl()
            _NEG_CACHE[neg_key] = (time.monotonic(), result, ttl)
            _neg_cache_evict_stale()
        # Scoreboard (stats.jsonl): one line per terminal outcome, so source health and failure
        # kinds are aggregable over time. record_fetch never raises — telemetry can't break a fetch.
        if isinstance(result, dict):
            ok = not result.get("error")
            stats.record_fetch(
                item, ok,
                str(result.get("method") or "") if ok else str(result.get("error_kind") or "error"),
            )
        if not fut.done():
            fut.set_result(result)
        return result
    finally:
        _INFLIGHT.pop(inflight_key, None)
