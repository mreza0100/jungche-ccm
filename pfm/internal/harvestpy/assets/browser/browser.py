"""Opt-in real-browser rung worker: Patchright + system Chrome.

Ported near-verbatim from the retired Python harvester's browser rung
(`git show fac3319:harvester/src/harvester/net.py` L593-672). The rung never
solves anything interactive (no Turnstile/CAPTCHA solver); it passes PASSIVE
bot walls by rendering in a real Chrome whose CDP automation handshake is
hidden by Patchright.

Protocol (JSON lines over stdin/stdout), one request serialized at a time:

  Go -> worker:   {"op":"fetch","url":"https://…","proxy":"http://…|null","timeout_ms":45000}
                  {"op":"smoke"}
  worker -> Go:   zero or more guard asks before the final line:
                  {"ask":"fetchable","url":"https://…"}
  Go -> worker:   {"allow":true}
                  {"allow":false,"reason":"refusing private/internal host …"}
  worker -> Go:   exactly one final line:
                  {"ok":true,"html":"…","status":403,"headless":false}
                  {"ok":false,"error":"patchright not installed"}

SSRF: Go owns the fetchable decision (harvest.AssertFetchable) — the route
guard below ASKS for every URL Chrome touches (initial navigation, every
redirect, every subresource, every XHR via context.route("**/*", …)) and
aborts blocked targets BEFORE Chrome ever connects.
"""

import asyncio
import json
import os.path
import shutil
import sys


# Only binaries `channel="chrome"` can actually launch — patchright's chrome
# channel means GOOGLE Chrome; a chromium-only host must report MISSING here,
# not pass smoke and fail every launch.
CHROME_CANDIDATES = [
    "google-chrome",
    "google-chrome-stable",
    "/usr/bin/google-chrome",
    "/usr/bin/google-chrome-stable",
    "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
]


def chrome_binary():
    """Resolve a system Chrome without launching anything."""
    for candidate in CHROME_CANDIDATES:
        found = shutil.which(candidate) or (
            candidate if os.path.isfile(candidate) else None
        )
        if found:
            return found
    return None


def browser_route_guard(ask_fetchable):
    """The per-request SSRF chokepoint for the browser rung, as a Playwright/Patchright
    route handler. Same contract as the retired net.py guard: EVERY url Chrome touches —
    redirects, subresources, XHR — is re-validated through the ask protocol, and blocked
    targets are aborted before Chrome ever connects. Extracted from fetch_browser so it
    is testable WITHOUT a browser under CI."""
    async def _ssrf_route_guard(route) -> None:
        target = str(route.request.url)
        try:
            allowed, reason = await ask_fetchable(target)
        except Exception as e:  # noqa: BLE001 — a broken ask channel must ABORT, never continue
            print(f"browser route guard RAISED for {redact(target)}: {e}", file=sys.stderr)
            await route.abort("guard-error")
            return
        if not allowed:
            print(f"browser route refused (ssrf) {redact(target)}: {reason}", file=sys.stderr)
            await route.abort("blocked")
            return
        await route.continue_()

    return _ssrf_route_guard


def redact(url):
    """Strip query string and fragment — refused URLs are logged, and their
    query strings can carry session tokens or internal host details."""
    return url.split("?", 1)[0].split("#", 1)[0]


def serialized_ask(raw_ask):
    """Wrap an ask callable so only ONE ask exchange is ever in flight.

    Playwright dispatches concurrent route handlers in parallel (measured on
    patchright 1.62.1: two handlers both entered while one was still awaiting
    its reply). The ask protocol is a strictly ordered stdin/stdout exchange
    with no correlation id — two interleaved exchanges would let an ALLOW
    meant for one URL satisfy the handler for another. A lock makes the
    protocol safe under concurrency.
    """
    lock = asyncio.Lock()

    async def guarded_ask(target):
        async with lock:
            return await raw_ask(target)

    return guarded_ask


async def fetch_browser(url, ask_fetchable, proxy_url=None, timeout_ms=45_000):
    """Render *url* in a real system Chrome via Patchright; return (html, status, headless, error).

    Opt-in rung — the caller gates it behind HARVESTER_BROWSER=1 because a browser launch is
    ~100ms+ and needs Chrome installed. patchright is an OPTIONAL dependency; when absent
    this returns ("", None, "patchright not installed") and the ladder falls through,
    never raises.

    SSRF: the fetchable decision runs on the initial URL AND on EVERY request the page
    makes — a context.route() interceptor re-checks each request/redirect/subresource and
    aborts any that Go refuses (a public URL 302ing to 169.254.169.254 must die at the
    route layer; a one-shot pre-check alone would let Chrome walk straight past it).
    """
    try:
        from patchright.async_api import async_playwright  # type: ignore[import-not-found]
    except ImportError:
        return "", None, False, "patchright not installed"
    loop = asyncio.get_event_loop()

    async def ask(target):
        return await loop.run_in_executor(None, ask_fetchable, target)

    # One lock for BOTH the initial ask and every route-handler ask: a single
    # in-flight stdin/stdout exchange, no matter how handlers interleave.
    guarded_ask = serialized_ask(ask)

    allowed, reason = await guarded_ask(url)
    if not allowed:
        return "", None, False, reason or f"refused initial url {url}"

    proxy = {"server": proxy_url} if proxy_url else None

    async def _render(headless: bool):
        async with async_playwright() as p:  # type: ignore[attr-defined]
            browser = await p.chromium.launch(channel="chrome", headless=headless, proxy=proxy)
            try:
                context = await browser.new_context()
                await context.route("**/*", browser_route_guard(guarded_ask))

                page = await context.new_page()
                resp = await page.goto(url, timeout=timeout_ms, wait_until="domcontentloaded")
                try:
                    await page.wait_for_load_state("networkidle", timeout=15_000)
                except Exception as e:  # noqa: BLE001 — best-effort quiet-period wait
                    print(f"browser networkidle wait ended early for {redact(url)}: {e}", file=sys.stderr)
                html = await page.content()
                status = resp.status if resp else None
                print(f"browser rung {redact(url)} -> HTTP {status} ({len(html)} chars, headless={headless})",
                      file=sys.stderr)
                return html, status, headless
            finally:
                await browser.close()

    # Headed Chrome passes passive checks far more reliably than headless (research bench);
    # on a display-less host the headed launch fails → retry once headless before giving up.
    try:
        html, status, headless = await _render(headless=False)
        if html:
            return html, status, headless, None
    except Exception as e:  # noqa: BLE001 — fall through to the headless retry
        print(f"browser headed launch failed for {redact(url)} ({type(e).__name__}) — retrying headless",
              file=sys.stderr)
    try:
        html, status, headless = await _render(headless=True)
        return html, status, headless, None
    except Exception as e:  # noqa: BLE001 — the rung never raises past this boundary
        print(f"browser rung failed for {redact(url)}: {e}", file=sys.stderr)
        return "", None, False, str(e)


def smoke():
    """Report patchright importability and Chrome resolution WITHOUT launching a page."""
    try:
        from patchright.async_api import async_playwright  # noqa: F401

        patchright = True
        error = None
    except ImportError as e:
        patchright = False
        error = "patchright not installed"
    binary = chrome_binary()
    return {
        "ok": True,
        "patchright": patchright,
        "chrome_path": binary,
        "error": error,
    }


async def handle_fetch(request):
    html, status, headless, error = await fetch_browser(
        request.get("url", ""),
        lambda target: _blocking_ask(target),
        proxy_url=request.get("proxy"),
        timeout_ms=int(request.get("timeout_ms") or 45_000),
    )
    if error is not None and not html:
        return {"ok": False, "error": error}
    return {"ok": True, "html": html, "status": status, "headless": headless}


def _blocking_ask(target):
    """Emit one guard ask and block on exactly one stdin reply line."""
    sys.stdout.write(json.dumps({"ask": "fetchable", "url": target}) + "\n")
    sys.stdout.flush()
    line = sys.stdin.readline()
    if not line:
        raise RuntimeError("go closed the ask channel mid-guard")
    reply = json.loads(line)
    return bool(reply.get("allow")), reply.get("reason")


def main():
    for raw in sys.stdin:
        raw = raw.strip()
        if not raw:
            continue
        try:
            request = json.loads(raw)
        except json.JSONDecodeError as e:
            print(json.dumps({"ok": False, "error": f"bad request JSON: {e}"}), flush=True)
            continue
        op = request.get("op")
        if op == "smoke":
            try:
                response = smoke()
            except Exception as e:  # noqa: BLE001 — a broken patchright install is an answer, not a crash
                response = {"ok": False, "error": f"{type(e).__name__}: {e}"}
            print(json.dumps(response), flush=True)
            continue
        if op != "fetch":
            print(json.dumps({"ok": False, "error": f"unknown op {op!r}"}), flush=True)
            continue
        try:
            response = asyncio.run(handle_fetch(request))
        except Exception as e:  # noqa: BLE001 — a crash discards only this response
            response = {"ok": False, "error": f"{type(e).__name__}: {e}"}
        sys.stdout.write(json.dumps(response) + "\n")
        sys.stdout.flush()


if __name__ == "__main__":
    main()
