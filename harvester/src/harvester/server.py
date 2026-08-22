"""The MCP plumbing: the harvester tools (`fetch`, `findWorks`, `search`, `fetchImage`,
`archive`, `searchCache`) and the `fetch` prompt.

All the real work lives in focused modules — this file only wires the MCP protocol to
`dispatch.get_or_fetch`, `describe.describe_fetch_result`, and `cache.search_cache`.
"""

import asyncio
from typing import Annotated

from mcp.server import Server
from mcp.server.stdio import stdio_server
from mcp.shared.exceptions import McpError
from mcp.types import (
    INVALID_PARAMS,
    ErrorData,
    GetPromptResult,
    Prompt,
    PromptArgument,
    PromptMessage,
    TextContent,
    Tool,
)
from pydantic import BaseModel, Field

from .cache import search_cache
from .describe import describe_fetch_result, describe_size_result
from .dispatch import find_sources, get_or_fetch, search_web
from .log import get_logger
from .net import DEFAULT_USER_AGENT_AUTONOMOUS
from .search import search_advertised, search_enabled

log = get_logger("server")


class Fetch(BaseModel):
    """Parameters for fetching/resolving one or many sources to Markdown."""

    sources: Annotated[
        list[str],
        Field(
            description=(
                "1–50 things to fetch, each returned as clean Markdown in the SAME order. Each is "
                "a LOCATION or an UNAMBIGUOUS identifier of a DOCUMENT:\n"
                "• URL / local path / file:// — web page, PDF, DOCX, XLSX, PPTX, CSV, JSON.\n"
                "• DOI — a bare DOI (10.xxxx/...), a 'doi:' prefix, or a doi.org URL → a free, legal copy.\n"
                "• Book by ISBN — isbn:9780262300988 (or a bare ISBN) → a free OA/public-domain copy.\n"
                "• PMID / PMCID — a bare PubMed ID (e.g. 30220343) or a PMC accession (PMC1234567) "
                "→ resolved via Europe PMC/PMC.\n"
                "Use a DIFFERENT tool for: a TITLE → `findWorks` (returns candidates to choose from); an "
                "IMAGE → `fetchImage` (returns a local path to read with vision); a ZIP/TAR/7z/RAR "
                "archive → `archive` (lists members, fetches one). `fetch` returns document markdown — "
                "pass it a title/image/archive and it points you to the right tool instead of guessing.\n"
                "Image references in the returned markdown stay as URLs — `fetch` never downloads image "
                "binaries; to VIEW one, pass its URL to `fetchImage`.\n"
                "Content past the inline cap is truncated with a note naming the cache file — the "
                "COMPLETE text is always saved there, whether or not the inline copy was cut. Mix "
                "kinds in one batch; a failing item returns a descriptive per-item error and the "
                "rest still return."
            ),
            min_length=1,
            max_length=50,
        ),
    ]
    refresh: Annotated[
        bool,
        Field(
            default=False,
            description=(
                "Force a fresh fetch: bypass the cache entirely, re-download, overwrite the cached "
                "artifact, and return the NEW content. Use it when a page may have changed since it "
                "was cached, or when a previous fetch returned something wrong or incomplete. "
                "Normally unnecessary — a cached web page, PDF, or document older than a day is "
                "re-fetched automatically; images and archives are served from cache until you ask "
                "for a refresh."
            ),
        ),
    ]
    size_only: Annotated[
        bool,
        Field(
            default=False,
            description=(
                "When true, fetch + cache the full content as normal but return NO body — just its "
                "SIZE and the cache file path: `{size, chars, path}` where `size` is an estimated "
                "TOKEN count (an over-counting heuristic) and `chars` is the raw character count. Use "
                "it to probe how big a source is before reading; then slice the cached `path` from "
                "disk. Reuses the same cache entry a normal fetch would (no duplicate download)."
            ),
        ),
    ]


class FindWorks(BaseModel):
    """Parameters for finding candidate works by title / free-text query (the scholarly WebSearch)."""

    query: Annotated[
        str,
        Field(description=(
            "A paper or book TITLE, or a free-text bibliographic query. Returns a ranked list of "
            "candidate works (papers + books), each with a `fetch:` handle — pick the right one and "
            "pass that handle to the `fetch` tool to retrieve it."
        )),
    ]
    limit: Annotated[
        int,
        Field(default=8, description="Maximum number of candidate works to return.", gt=0, le=25),
    ]


class FetchImage(BaseModel):
    """Parameters for fetching one or many images to a local path for vision."""

    sources: Annotated[
        list[str],
        Field(
            description=(
                "1–50 image URLs or local image paths. Each is downloaded into the type-partitioned "
                "cache (`<cache>/png/`, `<cache>/jpg/`, …) and its LOCAL FILE PATH is returned in order — "
                "open that path with your vision to read the figure/photo. Images are NOT OCR'd."
            ),
            min_length=1,
            max_length=50,
        ),
    ]


class Archive(BaseModel):
    """Parameters for safely browsing one archive (.zip/.tar(.gz/.bz2/.xz)/.7z/.rar)."""

    source: Annotated[
        str,
        Field(description="URL or local path of a .zip / .tar(.gz/.bz2/.xz) / .7z / .rar archive."),
    ]
    member: Annotated[
        str | None,
        Field(
            default=None,
            description=(
                "Omit to get the SAFE member listing (names + sizes, nothing extracted). Give a member "
                "name from that listing to fetch just that one member, converted to Markdown."
            ),
        ),
    ]


class Search(BaseModel):
    """Parameters for a web search (SearXNG-backed, Brave fallback)."""

    query: Annotated[str, Field(description="The web search query.")]
    count: Annotated[
        int, Field(default=8, description="Maximum number of results to return.", gt=0, le=20)
    ]
    lang: Annotated[
        str,
        Field(default="", description=(
            "Optional language/locale to bias the search (e.g. 'zh', 'ja', 'pt-BR'). Set it to "
            "reach a NON-English literature — it routes the query to that language's engines."
        )),
    ]
    engines: Annotated[
        str,
        Field(default="", description=(
            "Optional comma-separated SearXNG engines to restrict to (e.g. 'google,brave' or "
            "'naver,yahoo'). Omit for the default aggregated set."
        )),
    ]


class SearchCache(BaseModel):
    """Parameters for searching the local fetch cache."""

    pattern: Annotated[
        str,
        Field(description="Regex pattern to search across every cached markdown body in the cache."),
    ]
    max_results: Annotated[
        int,
        Field(default=50, description="Maximum number of matching cached pages to return.", gt=0, le=1000),
    ]
    ignore_case: Annotated[
        bool,
        Field(default=True, description="Case-insensitive search."),
    ]


def _render_find(query: str, cands: list[dict]) -> str:
    """Render find() candidates WebSearch-style: title, a `fetch:` handle, a one-line meta snippet."""
    if not cands:
        return (f"No candidate works found for {query!r}. Try a plain WebSearch, or rephrase — "
                "a more exact title helps.")
    lines = [
        f"{len(cands)} candidate work(s) for {query!r} — pick one and call `fetch` with its "
        "`fetch:` value:",
        "",
    ]
    for i, c in enumerate(cands, 1):
        meta = " · ".join(str(x) for x in (
            c.get("kind"), c.get("authors") or None, c.get("year") or None,
            c.get("source"), c.get("free") or None, f"match {c.get('match')}",
        ) if x)
        lines.append(f"{i}. {c.get('title') or '(untitled)'}")
        lines.append(f"   fetch: {c.get('fetch')}")
        lines.append(f"   {meta}")
    return "\n".join(lines)


def _render_search(query: str, results: "list[dict] | None", backend: str | None) -> str:
    """Render web-search results: title, URL (the fetch handle), snippet, engine."""
    if backend is None:
        return ("No web-search backend is configured. Set SEARXNG_URL (self-hosted SearXNG) "
                "and/or BRAVE_API_KEY to enable the `search` tool.")
    if backend == "error":
        return ("The web-search backend(s) are configured but unreachable or failing right now — "
                "retry shortly, or check that SEARXNG_URL is up and BRAVE_API_KEY is valid.")
    if not results:
        return f"No results for {query!r} (via {backend}). Try different terms or a broader query."
    lines = [f"{len(results)} result(s) for {query!r} (via {backend}) — fetch the ones you want by URL:", ""]
    for i, r in enumerate(results, 1):
        lines.append(f"{i}. {r.get('title') or '(untitled)'}")
        lines.append(f"   {r.get('url')}")
        if r.get("snippet"):
            lines.append(f"   {r['snippet']}")
        if r.get("engine"):
            lines.append(f"   [{r['engine']}]")
    return "\n".join(lines)


def build_server(
    custom_user_agent: str | None = None,
    ignore_robots_txt: bool = True,
    proxy_url: str | None = None,
) -> Server:
    """Build the harvester MCP `Server` with every tool/prompt handler registered.

    Transport-free on purpose: `serve` runs it over stdio, `remote.build_app` runs the SAME
    instance over Streamable HTTP. Tool behaviour must never differ between the two.
    """
    server = Server("harvester")
    user_agent_autonomous = custom_user_agent or DEFAULT_USER_AGENT_AUTONOMOUS
    log.info("build_server ua=%s proxy=%s", user_agent_autonomous, proxy_url)

    @server.list_tools()
    async def list_tools() -> list[Tool]:
        tools = [
            Tool(
                name="fetch",
                description="""Convert one or many **sources** (`sources`, a list of 1–50) to clean Markdown, in input order, each cached on disk (type-partitioned: `html/`, `pdf/`, `png/`, …).

A *source* is either a **location** (where something lives) or an **identity** (what the thing is — you need not know where to find it). Harvester resolves both and hands back the content.

**Locations — fetched directly to document markdown:**
- Web URL / local path / `file://` → web page, PDF, DOCX, XLSX, PPTX, CSV, JSON, or EPUB.
- HTML via trafilatura. Image references stay as URLs in the markdown — `fetch` never downloads image binaries and never OCRs; to VIEW an image, pass its URL to `fetchImage`, which returns a local path to read with vision (deployments can opt into downloading figures locally with `HARVESTER_LOCALIZE_IMAGES=1`). PDF via pymupdf4llm (extensionless URLs like `arxiv.org/pdf/…` are header-sniffed; a scanned PDF whose text layer is empty gets one automatic OCR pass). DOCX/XLSX/PPTX via Docling, CSV/EPUB via MarkItDown, JSON pretty-printed. Credential/secret files are refused.
- An IMAGE → use the `fetchImage` tool. An ARCHIVE (.zip/.tar/.7z/.rar) → use the `archive` tool. `fetch` will redirect you if you pass one here.

**Identities — resolved to a free, legal copy, then converted:**
- **DOI** — `10.xxxx/…`, `doi:…`, or a `doi.org` URL.
- **Book by ISBN** — `isbn:9780262300988` (or a bare ISBN).
- **PMID / PMCID** — a bare PubMed ID (e.g. `30220343`) or a PMC accession (`PMC1234567`), resolved via Europe PMC/PMC.
Harvester runs the legal open-access chain — for papers: OpenAlex, Semantic Scholar, Europe PMC, OpenAIRE, Zenodo, eLife, PLOS, NBER, Crossref, CORE, DOAJ (+ Unpaywall when HARVESTER_CONTACT_EMAIL is set; arXiv/ar5iv & OSF/SocArXiv resolve by DOI prefix); for books: OAPEN, DOAB, Internet Archive, HathiTrust (full view), Project Gutenberg — returning the first copy that yields real content. Only API-sanctioned sources; no shadow libraries.
**Have only a TITLE?** Titles are ambiguous, so `fetch` won't guess — call the **`findWorks`** tool first (it lists candidate works), then fetch the one you choose by its DOI/URL.

**Wall-bypass — when ANY URL is blocked, it goes down the rabbit hole:** httpx → curl_cffi Chrome-impersonation → Jina Reader → defuddle.md → (opt-in) a real system Chrome via Patchright → then it extracts the DOI from the page/URL (or a `citation_pdf_url` meta tag) and runs the open-access chain → Wayback Machine. So a paywalled or bot-blocked publisher link still returns the open copy when one legally exists. Hard IP-reputation blocks need a residential exit — the server says so plainly.

**Sibling tools:** `search` (open-web search → URLs to fetch), `findWorks` (a title → candidate works to choose from), `fetchImage` (an image → a local path to read with vision), `archive` (browse a .zip/.tar/.7z/.rar), `searchCache` (search what you already fetched).

Returns the content of every source in the SAME order. Each result: a short header (source, cache_status, method, bytes, tokens, fetched_at, cache path) then the content — content past the inline cap is truncated with a note, but the cache path always holds the COMPLETE text. A failing source yields a descriptive per-item error naming what to try next; the rest still return. Set `size_only: true` to get just `{size, chars, path}` per source (full content still cached) — probe a source's size, then slice the cached path from disk.""",
                inputSchema=Fetch.model_json_schema(),
            ),
            Tool(
                name="findWorks",
                description="""Find scholarly papers and books by TITLE or free-text query — the scholarly counterpart of WebSearch. Returns a RANKED LIST of candidate works (papers + books), each with a ready-to-use `fetch:` handle; it does NOT download anything.

Use it whenever you have a TITLE or a fuzzy description rather than a URL / DOI / ISBN — `fetch` deliberately won't guess which work a title means, so `findWorks` shows you the matches and you choose. Each result lists: title · authors · year · kind (paper|book) · source · free-access status · a `fetch:` handle (a DOI, an `isbn:` string, or a direct URL). Then call `fetch` with the handle of the one you want.

Two-step pattern, exactly like WebSearch → WebFetch: **findWorks → fetch**. (Papers come from OpenAlex + Crossref + Semantic Scholar + arXiv; books from Open Library + Project Gutenberg.)""",
                inputSchema=FindWorks.model_json_schema(),
            ),
            Tool(
                name="search",
                description="""Search the open web — a stronger, privacy-respecting replacement for the built-in WebSearch. Returns ranked results (title · URL · snippet · engine); pick the URLs you want and retrieve them with `fetch`.

Backed by a self-hosted **SearXNG** that aggregates 200+ engines (less single-engine/SEO bias than a plain Google search), with the **Brave** Search API as fallback. It returns links + snippets to TRIAGE, not full content — that's `fetch`'s job (search → fetch, like findWorks → fetch).

**Multilingual:** set `lang` (e.g. `zh`, `ja`, `pt-BR`) to route the query to that language's native engines — the way to reach Chinese / Japanese / Brazilian / etc. web results that an English search never surfaces. Optionally restrict to specific `engines`.""",
                inputSchema=Search.model_json_schema(),
            ),
            Tool(
                name="fetchImage",
                description="""Fetch one or many images and return their LOCAL FILE PATHS to read with your vision — figures, photos, charts, scanned pages. Each `sources` item is an image URL or a local image path; the bytes are saved under `<cache>/<ext>/` and the path returned in order.

Images are NOT OCR'd or turned into text — you OPEN the returned path with your vision to see the content (a chart or photo carries information no caption can). Use this instead of `fetch` whenever the thing is a picture; `fetch` returns document markdown (image refs left as URLs) and will point you here for an image.""",
                inputSchema=FetchImage.model_json_schema(),
            ),
            Tool(
                name="archive",
                description="""Safely browse a single archive — `.zip` / `.tar(.gz/.bz2/.xz)` / `.7z` / `.rar` — given by URL or local path.

Two-step, like `findWorks` → `fetch`: call with NO `member` to get the SAFE member listing (names + sizes; nothing is extracted to disk). Then call again with one `member` name from that listing to fetch just that member, converted to Markdown. Path-traversal and symlink members are refused, member-count/size caps are enforced, and the archive is never auto-extracted. Use this instead of `fetch` for any archive.""",
                inputSchema=Archive.model_json_schema(),
            ),
            Tool(
                name="searchCache",
                description="""Search every page already cached on disk for a regex `pattern`, returning WHICH cached pages match — source URL, match count, a sample line, and the cached `md_path` — not their text. Recall what you have already fetched without re-crawling; to read a match's content, `fetch` the source again (served from cache) or read `md_path` directly from disk.""",
                inputSchema=SearchCache.model_json_schema(),
            ),
        ]
        # `search` is always advertised so MCP clients (e.g. RR's scout/prospector) get a stable
        # six-tool surface. With no backend configured a call returns a clear "set SEARXNG_URL/
        # BRAVE_API_KEY" message rather than the tool vanishing mid-session.
        # HARVESTER_DISABLE_SEARCH=1 force-hides it for deployments that want no web search at all.
        if not search_advertised():
            tools = [t for t in tools if t.name != "search"]
        return tools

    @server.list_prompts()
    async def list_prompts() -> list[Prompt]:
        return [
            Prompt(
                name="fetch",
                description="Fetch a URL or local path and convert its contents to markdown",
                arguments=[PromptArgument(name="url", description="URL or path to fetch", required=True)],
            )
        ]

    @server.call_tool()
    async def call_tool(name, arguments: dict) -> list[TextContent]:
        if name == "searchCache":
            try:
                gargs = SearchCache(**arguments)
            except ValueError as e:
                raise McpError(ErrorData(code=INVALID_PARAMS, message=str(e)))
            try:
                matches = search_cache(gargs.pattern, gargs.max_results, gargs.ignore_case)
            except ValueError as e:
                raise McpError(ErrorData(code=INVALID_PARAMS, message=str(e)))
            if not matches:
                return [TextContent(type="text", text=(
                    f"No cached pages match /{gargs.pattern}/. This only searches pages already "
                    "fetched — it does not search the web; use `search` or `fetch` a source first."
                ))]
            lines = [f"{len(matches)} cached page(s) match /{gargs.pattern}/ — this lists WHICH "
                     "pages match, it does not return their text; `fetch` the source or read "
                     "`md_path` directly for content:", ""]
            for m in matches:
                lines.append(f"- {m['url']}  ({m['matches']} matches)  md_path: {m['md_path']}")
                if m["sample"]:
                    lines.append(f"    {m['sample']}")
            return [TextContent(type="text", text="\n".join(lines))]

        if name == "findWorks":
            try:
                fargs = FindWorks(**arguments)
            except ValueError as e:
                raise McpError(ErrorData(code=INVALID_PARAMS, message=str(e)))
            cands = await find_sources(fargs.query, fargs.limit, proxy_url)
            log.info("findWorks tool: %r -> %d candidate(s)", fargs.query, len(cands))
            return [TextContent(type="text", text=_render_find(fargs.query, cands))]

        if name == "search" and not search_enabled():
            return [TextContent(type="text", text=(
                "The `search` tool is disabled: no web-search backend is configured "
                "(set SEARXNG_URL and/or BRAVE_API_KEY)."))]
        if name == "search":
            try:
                sargs = Search(**arguments)
            except ValueError as e:
                raise McpError(ErrorData(code=INVALID_PARAMS, message=str(e)))
            results, backend = await search_web(
                sargs.query, sargs.count, sargs.lang, sargs.engines, proxy_url)
            log.info("search tool: %r -> %s (%s)", sargs.query,
                     len(results) if results else 0, backend)
            return [TextContent(type="text", text=_render_search(sargs.query, results, backend))]

        if name == "fetchImage":
            try:
                iargs = FetchImage(**arguments)
            except ValueError as e:
                raise McpError(ErrorData(code=INVALID_PARAMS, message=str(e)))
            log.info("fetchImage tool: %d source(s)", len(iargs.sources))
            sem = asyncio.Semaphore(8)

            async def dl_one(u: str) -> dict:
                async with sem:
                    return await get_or_fetch(u, user_agent_autonomous, proxy_url, media="allow")

            results = await asyncio.gather(*(dl_one(u) for u in iargs.sources), return_exceptions=True)
            return [describe_fetch_result(u, r) for u, r in zip(iargs.sources, results)]

        if name == "archive":
            try:
                aargs = Archive(**arguments)
            except ValueError as e:
                raise McpError(ErrorData(code=INVALID_PARAMS, message=str(e)))
            item = f"{aargs.source}::{aargs.member}" if aargs.member else aargs.source
            log.info("archive tool: %s member=%r", aargs.source, aargs.member)
            result = await get_or_fetch(item, user_agent_autonomous, proxy_url, media="allow")
            return [describe_fetch_result(aargs.source, result)]

        try:
            args = Fetch(**arguments)
        except ValueError as e:
            raise McpError(ErrorData(code=INVALID_PARAMS, message=str(e)))

        log.info("fetch tool: %d source(s) size_only=%s refresh=%s",
                 len(args.sources), args.size_only, args.refresh)
        sem = asyncio.Semaphore(8)

        async def fetch_one(u: str) -> dict:
            async with sem:
                return await get_or_fetch(
                    u, user_agent_autonomous, proxy_url, media="deny", refresh=args.refresh
                )

        results = await asyncio.gather(*(fetch_one(u) for u in args.sources), return_exceptions=True)
        # size_only: full content is still fetched + cached, but return only {size, chars, path}.
        render = describe_size_result if args.size_only else describe_fetch_result
        return [render(u, r) for u, r in zip(args.sources, results)]

    @server.get_prompt()
    async def get_prompt(name: str, arguments: dict | None) -> GetPromptResult:
        if not arguments or "url" not in arguments:
            raise McpError(ErrorData(code=INVALID_PARAMS, message="URL is required"))
        url = arguments["url"]
        # Same contract as the `fetch` tool: media="deny" (redirect images/archives to their own
        # tools) and describe_fetch_result rendering (header + status/kind diagnostics on failure),
        # so the prompt path never loses information the tool path keeps.
        result = await get_or_fetch(url, user_agent_autonomous, proxy_url, media="deny")
        rendered = describe_fetch_result(url, result)
        return GetPromptResult(
            description=f"Contents of {url}",
            messages=[PromptMessage(role="user", content=rendered)],
        )

    return server


async def serve(
    custom_user_agent: str | None = None,
    ignore_robots_txt: bool = True,
    proxy_url: str | None = None,
) -> None:
    """Run the fetch MCP server over stdio (the local/default transport)."""
    server = build_server(custom_user_agent, ignore_robots_txt, proxy_url)
    log.info("serve start (stdio)")
    options = server.create_initialization_options()
    async with stdio_server() as (read_stream, write_stream):
        await server.run(read_stream, write_stream, options, raise_exceptions=False)
