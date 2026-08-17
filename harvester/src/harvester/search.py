"""Web-search backend for the `search` tool: SearXNG (self-hosted) with a Brave fallback.

`search` deliberately returns high-signal results (title + URL + snippet) for the agent to
TRIAGE and then retrieve through harvester's own `fetch` — it does NOT return cleaned content,
because `fetch` already does that. SearXNG is the primary backend (free, self-hosted, aggregates
many engines, routes per-language to regional engines — the multilingual lever); the Brave Search
API is the managed single-key fallback for downtime or blocked locales.

Configured by env:
  SEARXNG_URL   — base URL of a self-hosted SearXNG (e.g. http://127.0.0.1:8888)
  BRAVE_API_KEY — Brave Search API token (fallback)
Returns (results, backend): backend is "searxng" / "brave", or None when neither is configured.
"""
from __future__ import annotations

import os
from typing import TYPE_CHECKING, Tuple

from .log import get_logger

if TYPE_CHECKING:
    from httpx import AsyncClient

log = get_logger("search")

SEARXNG_URL = os.environ.get("SEARXNG_URL", "")
BRAVE_API_KEY = os.environ.get("BRAVE_API_KEY", "")
_UA = "harvester-mcp/1.0"


def _force_disabled() -> bool:
    """True when HARVESTER_DISABLE_SEARCH is set truthy — the operator opt-out for web search."""
    return os.environ.get("HARVESTER_DISABLE_SEARCH", "").strip().lower() in ("1", "true", "yes", "on")


def search_enabled() -> bool:
    """Whether the `search` BACKEND can actually run — a backend (SearXNG and/or Brave) is
    configured and the tool is not force-disabled. Governs call-time behavior, NOT tool visibility:
    when False the tool is still listed but returns a clear "set SEARXNG_URL/BRAVE_API_KEY" message.
    """
    if _force_disabled():
        return False
    return bool(SEARXNG_URL or BRAVE_API_KEY)


def search_advertised() -> bool:
    """Whether the `search` tool is LISTED at all. Always advertised so MCP clients (e.g. RR's
    scout/prospector) see a stable six-tool surface; with no backend a call returns a clear
    configure-me message rather than the tool vanishing. `HARVESTER_DISABLE_SEARCH=1` force-hides
    it for deployments that want no web search at all.
    """
    return not _force_disabled()


async def _searxng(query: str, client: "AsyncClient", count: int, lang: str, engines: str):
    """Query a self-hosted SearXNG. Returns list (possibly empty) or None if unconfigured/failed."""
    if not SEARXNG_URL:
        return None
    params = {"q": query, "format": "json", "safesearch": "0"}
    if lang:
        params["language"] = lang
    if engines:
        params["engines"] = engines
    try:
        r = await client.get(SEARXNG_URL.rstrip("/") + "/search", params=params,
                             headers={"User-Agent": _UA}, timeout=20)
        if r.status_code >= 400:
            log.warning("searxng HTTP %d for %r", r.status_code, query)
            return None
        data = r.json()
    except Exception as e:
        log.warning("searxng failed for %r: %s", query, e)
        return None
    out = [
        {"title": it.get("title") or "", "url": it["url"],
         "snippet": (it.get("content") or "")[:300],
         "engine": ",".join(it.get("engines") or []) or it.get("engine", "")}
        for it in (data.get("results") or []) if it.get("url")
    ][:count]
    log.info("searxng %r -> %d result(s)", query, len(out))
    return out


async def _brave(query: str, client: "AsyncClient", count: int, lang: str):
    """Query the Brave Search API. Returns list (possibly empty) or None if no key/failed."""
    if not BRAVE_API_KEY:
        return None
    params = {"q": query, "count": min(count, 20)}
    if lang:
        params["search_lang"] = lang
    headers = {"Accept": "application/json", "User-Agent": _UA,
               "X-Subscription-Token": BRAVE_API_KEY}
    try:
        r = await client.get("https://api.search.brave.com/res/v1/web/search",
                             params=params, headers=headers, timeout=20)
        if r.status_code >= 400:
            log.warning("brave HTTP %d for %r", r.status_code, query)
            return None
        data = r.json()
    except Exception as e:
        log.warning("brave failed for %r: %s", query, e)
        return None
    out = [
        {"title": it.get("title") or "", "url": it["url"],
         "snippet": (it.get("description") or "")[:300], "engine": "brave"}
        for it in ((data.get("web") or {}).get("results") or []) if it.get("url")
    ][:count]
    log.info("brave %r -> %d result(s)", query, len(out))
    return out


async def web_search(
    query: str, client: "AsyncClient", count: int = 8, lang: str = "", engines: str = ""
) -> Tuple[list[dict] | None, str | None]:
    """Try SearXNG, then Brave. Returns (results, backend).

    backend "searxng"/"brave" names which answered; (None, None) means NO backend is configured.
    A configured-but-empty backend returns ([], backend).
    """
    sx = await _searxng(query, client, count, lang, engines)
    if sx:
        return sx, "searxng"
    bv = await _brave(query, client, count, lang)
    if bv:
        return bv, "brave"
    if sx == []:
        return [], "searxng"
    if bv == []:
        return [], "brave"
    # Neither backend produced anything: distinguish "not configured" from "configured but failing".
    return None, ("error" if (SEARXNG_URL or BRAVE_API_KEY) else None)
