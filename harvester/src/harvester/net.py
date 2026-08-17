"""The network layer: raw/byte fetchers, curl_cffi impersonation, Jina Reader, and the
private-host guard. Every function is resilient — it never raises, returning ""/b""/None
and an error_kind on failure. httpx and curl_cffi are imported lazily.
"""

import asyncio
import ipaddress
import socket
from typing import Tuple
from urllib.parse import urljoin, urlparse, urlunparse

from .log import get_logger

log = get_logger("net")

DEFAULT_USER_AGENT_AUTONOMOUS = "Mozilla/5.0 (compatible; harvester/1.0)"
DEFAULT_USER_AGENT_MANUAL = "Mozilla/5.0 (compatible; harvester/1.0)"

MAX_DOWNLOAD_BYTES = 50 * 1024 * 1024

# Redirect handling for the curl_cffi paths (httpx follows in-library under the request hook).
_REDIRECT_CODES = {301, 302, 303, 307, 308}
_MAX_REDIRECTS = 10


class FetchNotAllowed(Exception):
    """Raised by the SSRF/scheme chokepoint when a URL must not be fetched.

    A non-http(s) scheme, a private/internal host (lexically or after DNS resolution), or an
    unparseable URL. Every fetch primitive converts this into its own resilient sentinel
    (""/b""/None) rather than letting it escape.
    """

HTTP_STATUS_MEANINGS = {
    400: "bad request", 401: "unauthorized", 403: "forbidden", 404: "page not found",
    405: "method not allowed", 408: "request timeout", 410: "gone", 429: "too many requests",
    500: "internal server error", 502: "bad gateway", 503: "service unavailable",
    504: "gateway timeout",
}
CONNECTION_ERROR_REASONS = {
    "timeout": "the server did not respond in time (connection timed out)",
    "dns": "DNS resolution failed (host not found)",
    "connect": "the connection failed (refused or host unreachable)",
}


def failure_message(item: str, status: int | None, error_kind: str | None,
                     challenge: bool = False) -> str | None:
    """One shared sentence for a net-level fetch failure, naming the next move.

    Shared by `dispatch._net_error` (a download that returned no bytes at all) and
    `describe.describe_fetch_result` (rendering a finished dispatch result) so the wording for
    the same failure kind never drifts between the two call sites. Returns None when *status*/
    *error_kind*/*challenge* don't describe a classifiable net failure — the caller supplies its
    own context-specific fallback (e.g. "no bytes downloaded" vs. "fetched but empty").
    """
    if error_kind == "invalid":
        return f"Invalid URL: {item} — check it for typos, or use `search` to find the source."
    if error_kind in CONNECTION_ERROR_REASONS:
        return (f"Could not reach {item}: {CONNECTION_ERROR_REASONS[error_kind]}. Retry later, "
                "or use `search` to find an alternative copy.")
    if challenge:
        return (f"{item} is behind a bot/Cloudflare challenge — content not retrievable from "
                "this datacenter server. Use `search` to find a mirror or alternative copy.")
    if status is not None and status >= 400:
        meaning = HTTP_STATUS_MEANINGS.get(status, "request failed")
        note = " — likely a bot-block or rate limit" if status in (403, 429, 503) else ""
        return (f"{item} returned HTTP {status} ({meaning}){note}. Use `search` to find an "
                "alternative copy, or `findWorks` if it is a scholarly title.")
    return None


def get_robots_txt_url(url: str) -> str:
    parsed = urlparse(url)
    return urlunparse((parsed.scheme, parsed.netloc, "/robots.txt", "", "", ""))


def _ext_from_content_type(ct: str) -> str:
    return {
        "image/jpeg": ".jpg", "image/jpg": ".jpg", "image/png": ".png",
        "image/gif": ".gif", "image/webp": ".webp", "image/bmp": ".bmp",
        "image/tiff": ".tiff", "image/svg+xml": ".svg",
    }.get(ct.split(";")[0].strip().lower(), ".png")


async def fetch_raw(url: str, user_agent: str, proxy_url: str | None = None) -> str:
    """Fetch the raw page body. Resilient: returns "" on a connection error."""
    from httpx import HTTPError
    async with _client(proxy_url) as client:
        try:
            response = await client.get(
                url, follow_redirects=True, headers={"User-Agent": user_agent}, timeout=30
            )
        except FetchNotAllowed as e:
            log.warning("fetch_raw refused %s: %s", url, e)
            return ""
        except HTTPError as e:
            log.warning("fetch_raw httpx error %s: %s", url, e)
            return ""
        log.debug("fetch_raw %s -> %d (%d bytes)", url, response.status_code, len(response.text))
        return response.text


async def fetch_raw_status(
    url: str, user_agent: str, proxy_url: str | None = None
) -> Tuple[str, int | None, str | None]:
    """Fetch raw text + diagnostics. Returns (text, http_status, error_kind). Never raises."""
    from httpx import (
        ConnectError, HTTPError, InvalidURL, TimeoutException, UnsupportedProtocol,
    )
    async with _client(proxy_url) as client:
        try:
            response = await client.get(
                url, follow_redirects=True, headers={"User-Agent": user_agent}, timeout=30
            )
        except FetchNotAllowed as e:
            log.warning("fetch_raw_status refused %s: %s", url, e)
            return "", None, "blocked"
        except (InvalidURL, UnsupportedProtocol):
            log.warning("fetch_raw_status invalid url %s", url)
            return "", None, "invalid"
        except TimeoutException:
            log.warning("fetch_raw_status timeout %s", url)
            return "", None, "timeout"
        except ConnectError as e:
            s = str(e).lower()
            dns_markers = (
                "name or service not known", "nodename nor servname", "getaddrinfo",
                "name resolution", "no address associated", "temporary failure in name resolution",
            )
            kind = "dns" if any(m in s for m in dns_markers) else "connect"
            log.warning("fetch_raw_status %s %s: %s", kind, url, e)
            return "", None, kind
        except HTTPError as e:
            log.warning("fetch_raw_status connect error %s: %s", url, e)
            return "", None, "connect"
        log.debug("fetch_raw_status %s -> %d", url, response.status_code)
        return response.text, response.status_code, None


async def _stream_capped(client, url, user_agent) -> Tuple[bytes, int | None, str | None, str]:
    """GET streaming, stopping at MAX_DOWNLOAD_BYTES so an oversized body is never fully buffered.

    Returns (data, status, error_kind, content_type). Never raises.
    """
    from httpx import (
        ConnectError, HTTPError, InvalidURL, Timeout, TimeoutException, UnsupportedProtocol,
    )
    timeout = Timeout(60.0, connect=10.0)  # short connect → closed/filtered ports fail fast
    try:
        async with client.stream(
            "GET", url, follow_redirects=True, headers={"User-Agent": user_agent}, timeout=timeout
        ) as response:
            # R1: a ≥400 response body is KEPT, not discarded — a soft-404/403-with-content still
            # needs to reach extraction + meta-scrape (the dispatch layer decides what to do with
            # it). INVARIANT for callers: only the primary-URL HTML ladder (_html_result) may
            # treat a ≥400 body as potential content; every rescue/binary path (OA candidates,
            # doc/image/archive downloads) treats ≥400 as that rung's FAILURE — see the
            # "R1 ripple guard" comments in dispatch.py. A genuinely empty error body still ends
            # up as empty `data`, so callers keyed on "no bytes" see the same thing as before.
            ct = response.headers.get("content-type", "")
            chunks: list[bytes] = []
            total = 0
            async for chunk in response.aiter_bytes():
                chunks.append(chunk)
                total += len(chunk)
                if total >= MAX_DOWNLOAD_BYTES:
                    log.info("stream %s hit %d-byte cap — truncating", url, MAX_DOWNLOAD_BYTES)
                    break
            data = b"".join(chunks)[:MAX_DOWNLOAD_BYTES]
            if response.status_code >= 400:
                log.info("stream %s -> HTTP %d (%d bytes kept)", url, response.status_code, len(data))
            log.debug("stream %s -> %d (%d bytes, ct=%s)", url, response.status_code, len(data), ct)
            return data, response.status_code, None, ct
    except FetchNotAllowed as e:
        log.warning("stream refused %s: %s", url, e)
        return b"", None, "blocked", ""
    except (InvalidURL, UnsupportedProtocol):
        log.warning("stream invalid url %s", url)
        return b"", None, "invalid", ""
    except TimeoutException:
        log.warning("stream timeout %s", url)
        return b"", None, "timeout", ""
    except ConnectError as e:
        s = str(e).lower()
        dns = any(m in s for m in ("name or service not known", "getaddrinfo", "name resolution"))
        kind = "dns" if dns else "connect"
        log.warning("stream %s %s: %s", kind, url, e)
        return b"", None, kind, ""
    except HTTPError as e:
        log.warning("stream connect error %s: %s", url, e)
        return b"", None, "connect", ""


async def download_bytes(
    url: str, user_agent: str, proxy_url: str | None = None
) -> Tuple[bytes, int | None, str | None]:
    """Download binary content (streamed, capped). Returns (data, http_status, error_kind)."""
    async with _client(proxy_url) as client:
        data, status, error_kind, _ct = await _stream_capped(client, url, user_agent)
        return data, status, error_kind


async def fetch_bytes_with_meta(
    url: str, user_agent: str, proxy_url: str | None = None
) -> Tuple[bytes, int | None, str | None, str]:
    """Download binary content (streamed, capped). Returns (data, http_status, error_kind, content_type).

    Never raises. Returns (b"", ...) on any error. Exposes Content-Type so callers can detect
    binary documents served at extensionless URLs (e.g. arxiv /pdf/... endpoints).
    """
    async with _client(proxy_url) as client:
        return await _stream_capped(client, url, user_agent)


def _strip_jina_envelope(text: str) -> str:
    """r.jina.ai prefixes 'Title:/URL Source:/Published Time:/Markdown Content:' scaffolding — keep
    only the body so it doesn't leak into cached documents."""
    marker = "Markdown Content:"
    idx = text.find(marker)
    if idx != -1:
        return text[idx + len(marker):].lstrip("\n")
    # No marker — strip a leading run of envelope header lines if present.
    lines = text.splitlines()
    i = 0
    while i < len(lines) and lines[i].split(":", 1)[0] in (
            "Title", "URL Source", "Published Time", "Markdown Content", "Warning"):
        i += 1
    return "\n".join(lines[i:]).lstrip("\n") if i else text


# Strong, specific bot-wall phrases — multi-word, so safe to flag at ANY body length.
_CHALLENGE_PHRASES = (
    "just a moment",
    "checking your browser",
    "checking your browser before",
    "cf-browser-verification",
    "cf-chl-",  # Cloudflare challenge token / script markers
    "are you a robot",
    "confirm you are a human",
    "enable javascript and cookies",
    "captcha challenge",
    "completing the captcha",
    "verify you are human",
    "verifying you are human",
)
# Weak cues — common enough in real prose, so only treat them as a wall when the body is SHORT
# (a genuine interstitial is tiny; a real article that merely mentions the word "captcha" once,
# or whose prose says "attention required", is thousands of chars long). "attention required" is
# the Cloudflare 1020/block-page title but also plausible English, so it lives here, length-gated.
_CHALLENGE_WEAK = ("captcha", "cloudflare", "turnstile", "attention required")
_CHALLENGE_WEAK_MAX_LEN = 4000  # a few KB


def looks_like_challenge(html: str) -> bool:
    """Heuristic: does this raw HTML look like a Cloudflare / bot-wall interstitial?

    Strong multi-word phrases (e.g. "are you a robot", "enable javascript and cookies") flag at
    any length; bare single-word cues ("captcha", "cloudflare") flag only in a SHORT body, so a
    long real article that mentions the word in passing is not misread as a wall.
    """
    low = html.lower()
    if any(p in low for p in _CHALLENGE_PHRASES):
        return True
    if len(html) <= _CHALLENGE_WEAK_MAX_LEN and any(w in low for w in _CHALLENGE_WEAK):
        return True
    return False


def _ip_is_blocked(addr: "ipaddress._BaseAddress") -> bool:
    """True if an IP is loopback / RFC-1918 / link-local / reserved / unspecified / multicast —
    i.e. never a legitimate public fetch target."""
    return (addr.is_loopback or addr.is_private or addr.is_link_local
            or addr.is_reserved or addr.is_unspecified or addr.is_multicast)


def _host_to_ip(host: str) -> "ipaddress._BaseAddress | None":
    """Parse *host* as a LITERAL IP — including OBFUSCATED forms (integer 2852039166, hex 0x...,
    octal) a naive string check misses — or None if it's a name needing DNS resolution."""
    try:
        return ipaddress.ip_address(host)  # dotted v4 / bracketless v6
    except ValueError:
        try:
            if host.startswith("0x"):
                return ipaddress.ip_address(int(host, 16))
            if host.isdigit():
                return ipaddress.ip_address(int(host))
            if "." not in host and ":" not in host and host.startswith("0") and host != "0":
                return ipaddress.ip_address(int(host, 8))  # octal
        except (ValueError, OverflowError):
            return None
    return None


def is_private_host(url: str) -> bool:
    """Return True if *url* targets a loopback, RFC-1918, link-local, or internal host (LEXICAL).

    Covers: 127.x / ::1 loopback, 10.x / 172.16–31.x / 192.168.x RFC-1918, 169.254.x link-local,
    `.ts.net` / `.local` / `.internal` internal TLDs, and any URL that contains embedded credentials.
    This is a cheap string-only filter — it does NOT resolve DNS; `assert_fetchable` adds the
    getaddrinfo check that also defeats DNS-rebinding. Private hosts must never be sent to an
    external proxy like Jina Reader.
    """
    try:
        parsed = urlparse(url)
    except Exception as e:
        log.warning("is_private_host could not parse %s: %s — treating as private", url, e)
        return True  # malformed — be safe

    # Credentials in the URL → don't forward to an external service
    if parsed.username or parsed.password:
        return True

    host = (parsed.hostname or "").lower().strip("[]")
    if not host:
        return True  # no resolvable host → treat as private

    # Internal-only TLD suffixes
    for suffix in (".ts.net", ".local", ".internal"):
        if host.endswith(suffix):
            return True

    # Well-known loopback hostnames
    if host in ("localhost", "localhost.localdomain", "ip6-localhost", "ip6-loopback"):
        return True

    addr = _host_to_ip(host)
    if addr is not None:
        return _ip_is_blocked(addr)

    return False


def _check_fetchable(url: str) -> None:
    """Sync core of the SSRF/scheme chokepoint (does a BLOCKING getaddrinfo). Raises
    `FetchNotAllowed` if *url* is not a safe public http(s) target. See `assert_fetchable`.
    """
    try:
        parsed = urlparse(url)
    except Exception as e:
        raise FetchNotAllowed(f"unparseable URL {url!r}: {e}")

    scheme = (parsed.scheme or "").lower()
    if scheme not in ("http", "https"):
        raise FetchNotAllowed(f"scheme {scheme or '(none)'!r} not allowed (http/https only): {url!r}")

    # Cheap lexical filter first — credentials, internal TLDs, literal/obfuscated private IPs.
    if is_private_host(url):
        raise FetchNotAllowed(f"private/internal host refused: {url!r}")

    host = (parsed.hostname or "").lower().strip("[]")
    if not host:
        raise FetchNotAllowed(f"no host in URL {url!r}")

    # A literal public IP was already validated by is_private_host — no DNS needed.
    if _host_to_ip(host) is not None:
        return

    # Resolve the NAME and reject if ANY address is private — this is what kills DNS-rebinding
    # (a public hostname whose A/AAAA record points at 169.254.169.254 / 127.0.0.1 / RFC-1918).
    try:
        infos = socket.getaddrinfo(host, None)
    except OSError as e:
        # Unresolvable — there is nothing to fetch; let the real fetch surface the DNS error
        # rather than masking it as an SSRF refusal.
        log.debug("assert_fetchable could not resolve %s: %s", host, e)
        return
    for info in infos:
        ip_str = str(info[4][0]).split("%", 1)[0]  # strip any IPv6 scope id
        try:
            addr = ipaddress.ip_address(ip_str)
        except ValueError:
            continue
        if _ip_is_blocked(addr):
            raise FetchNotAllowed(f"host {host!r} resolves to blocked IP {ip_str}: {url!r}")


async def assert_fetchable(url: str) -> None:
    """The single SSRF/scheme chokepoint. Raises `FetchNotAllowed` unless *url* is a safe public
    http(s) target:

      * scheme must be http or https (kills file://, gopher://, dict://, …);
      * the host must not be private/internal lexically (creds, .ts.net/.local/.internal,
        literal/obfuscated private IPs); and
      * the host must not RESOLVE (getaddrinfo) to any loopback/private/link-local/reserved/
        multicast/unspecified address — closing the DNS-rebinding variant.

    Call before every outbound fetch; the httpx request hook re-runs it on each redirect hop.
    """
    await asyncio.to_thread(_check_fetchable, url)


async def _ssrf_request_hook(request) -> None:
    """httpx request event-hook — validate the target (and EVERY redirect hop, since httpx fires
    this for each request in a redirect chain) so a 302 → internal/metadata host is refused."""
    await assert_fetchable(str(request.url))


def _client(proxy_url: str | None = None, **kwargs):
    """An httpx.AsyncClient with the SSRF request-hook installed on every request + redirect hop.

    Use this instead of constructing AsyncClient directly so no caller can forget the guard.
    """
    from httpx import AsyncClient
    hooks = dict(kwargs.pop("event_hooks", None) or {})
    hooks["request"] = [*hooks.get("request", []), _ssrf_request_hook]
    return AsyncClient(proxy=proxy_url, event_hooks=hooks, **kwargs)


async def fetch_jina(url: str, user_agent: str, proxy_url: str | None = None) -> str:
    """Fetch via Jina Reader (https://r.jina.ai/<url>).

    Skips private hosts so internal URLs are never leaked to the external service.
    Returns the markdown body, or "" on any error or 4xx response. Never raises.
    """
    if is_private_host(url):
        log.debug("jina skipped private host %s", url)
        return ""
    from httpx import HTTPError
    jina_url = f"https://r.jina.ai/{url}"
    try:
        async with _client(proxy_url) as client:
            response = await client.get(
                jina_url, follow_redirects=True,
                headers={"User-Agent": user_agent}, timeout=30,
            )
        if response.status_code >= 400:
            log.warning("jina %s -> HTTP %d", url, response.status_code)
            return ""
        log.debug("jina %s -> %d (%d chars)", url, response.status_code, len(response.text))
        return _strip_jina_envelope(response.text)
    except FetchNotAllowed as e:
        log.warning("jina refused %s: %s", url, e)
        return ""
    except HTTPError as e:
        log.warning("jina httpx error %s: %s", url, e)
        return ""
    except Exception as e:
        log.warning("jina unexpected error %s: %s", url, e)
        return ""


def _cffi_session_kwargs() -> dict:
    """curl_cffi Session kwargs that lock libcurl to http/https on the initial transfer AND on any
    redirect — defense-in-depth behind `assert_fetchable`'s scheme gate (blocks file://, gopher://,
    dict://, … at the libcurl layer)."""
    try:
        from curl_cffi import CurlOpt  # type: ignore[import-not-found]
        return {"curl_options": {CurlOpt.PROTOCOLS_STR: "https,http",
                                 CurlOpt.REDIR_PROTOCOLS_STR: "https,http"}}
    except Exception:  # pragma: no cover — older curl_cffi without CurlOpt
        return {}


def _impersonated_fetch_sync(url: str, prx, timeout: int):
    """Sync curl_cffi fallback (no AsyncSession). Manual, bounded, per-hop-validated redirects.
    Returns the final Response, or None on refusal/failure."""
    try:
        import curl_cffi.requests as _cffi  # type: ignore[import-not-found]
    except ImportError:
        return None
    current = url
    for _ in range(_MAX_REDIRECTS + 1):
        try:
            _check_fetchable(current)
        except FetchNotAllowed as e:
            log.warning("curl_cffi(sync) refused %s: %s", current, e)
            return None
        try:
            r = _cffi.get(current, impersonate="chrome", proxies=prx, timeout=timeout,
                          allow_redirects=False)  # type: ignore[arg-type]
        except Exception as e:
            log.warning("curl_cffi(sync) failed %s: %s", current, e)
            return None
        if r.status_code in _REDIRECT_CODES and r.headers.get("location"):
            current = urljoin(current, r.headers["location"])
            continue
        return r
    log.warning("curl_cffi(sync) too many redirects %s", url)
    return None


async def _impersonated_fetch(url: str, proxy_url: str | None, timeout: int):
    """curl_cffi GET that runs `assert_fetchable` on the initial URL and EVERY redirect hop
    (manual, bounded — curl_cffi auto-follow would skip the per-hop host check), with libcurl
    protocols locked to http/https. Returns the final Response, or None on refusal/failure.
    """
    # curl_cffi ProxySpec expects {"http": ..., "https": ...} — structurally compatible
    # but pyright cannot verify it without the stubs' TypedDict, so we ignore arg-type below.
    prx = {"https": proxy_url, "http": proxy_url} if proxy_url else None
    try:
        from curl_cffi.requests import AsyncSession  # type: ignore[import-not-found]
    except ImportError:
        return await asyncio.to_thread(_impersonated_fetch_sync, url, prx, timeout)
    current = url
    try:
        async with AsyncSession(**_cffi_session_kwargs()) as session:  # type: ignore[attr-defined]
            for _ in range(_MAX_REDIRECTS + 1):
                await assert_fetchable(current)
                r = await session.get(current, impersonate="chrome", proxies=prx,  # type: ignore[arg-type]
                                      timeout=timeout, allow_redirects=False)
                if r.status_code in _REDIRECT_CODES and r.headers.get("location"):
                    current = urljoin(current, r.headers["location"])
                    continue
                return r
        log.warning("curl_cffi too many redirects %s", url)
        return None
    except FetchNotAllowed as e:
        log.warning("curl_cffi refused %s: %s", current, e)
        return None
    except Exception as e:
        log.warning("curl_cffi failed %s: %s", url, e)
        return None


async def fetch_impersonated(url: str, proxy_url: str | None = None) -> Tuple[str, int | None]:
    """Fetch with curl_cffi Chrome TLS/JA3 fingerprint impersonation.

    Never raises — returns ("", None) on any failure, refusal, or missing curl_cffi. A status of 0
    (a failed / non-HTTP transfer) is treated as failure, not success.
    """
    r = await _impersonated_fetch(url, proxy_url, 45)
    if r is None or not r.status_code:
        return "", (r.status_code if r is not None else None) or None
    log.debug("curl_cffi %s -> %s (%d chars)", url, r.status_code, len(r.text))
    return r.text, r.status_code


async def download_impersonated(url: str, proxy_url: str | None = None) -> Tuple[bytes, int | None]:
    """Download raw bytes with curl_cffi Chrome impersonation (for walled PDFs / CDN assets).

    Never raises — returns (b"", None) on any failure, refusal, or missing curl_cffi. A status of 0
    (a failed / non-HTTP transfer) and any >=400 are treated as failure.
    """
    r = await _impersonated_fetch(url, proxy_url, 60)
    if r is None:
        return b"", None
    if not r.status_code or r.status_code >= 400:
        log.warning("curl_cffi download %s -> HTTP %s", url, r.status_code)
        return b"", r.status_code or None
    log.debug("curl_cffi download %s -> %s (%d bytes)", url, r.status_code, len(r.content))
    return r.content[:MAX_DOWNLOAD_BYTES], r.status_code
