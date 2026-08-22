# harvester

A multi-format MCP **fetch + search** server. It converts web pages, PDFs, Office documents,
archives, and local files to clean Markdown; resolves scholarly identifiers (DOI/ISBN/title) to a
free, legal full-text copy; caches every artifact on disk under a type-partitioned `.cache/` tree;
and escalates through a four-stage wall-bypass ladder before surfacing an error.

> [!CAUTION]
> This server can access local files and local/internal IP addresses, and it reaches out to the
> open web. It may represent a security risk. Run it with the least privilege you can, and ensure it
> does not expose sensitive data. (Credential and key files are refused — see
> [Local-path support](#local-path-support-and-deny-paths).)

- **Repo:** <https://github.com/mreza0100/harvester-web-mcp>
- **Package name:** `harvester-mcp` (not yet on PyPI — install from source or from GitHub)
- **Command / module:** `harvester` (`python -m harvester`)

---

## What it does

**One API, many formats.** Pass a URL, a local path, or a scholarly identifier — `harvester`
detects the format, routes it to the right converter, caches the Markdown result, and returns the
full content. A second call on the same input is served from cache with no refetch.

**Wall-bypass ladder.** For an HTML page the server tries these methods in order, stopping as soon
as it gets rich content:

1. **httpx** — standard HTTP with trafilatura extraction (`favor_recall`); article images are
   downloaded locally and `![](remote)` links are rewritten to local cache paths when
   `HARVESTER_LOCALIZE_IMAGES=1` is set.
2. **curl_cffi Chrome impersonation** — replays a real Chrome TLS/JA3 fingerprint (pinned
   ≥0.16.1: current TLS + HTTP/2 fingerprints); bypasses many bot-detection CDNs.
3. **Jina Reader** (`https://r.jina.ai/<url>`) — external headless extraction; skipped for
   private/internal hosts. Keyless = 20 req/min; set `JINA_API_KEY` for ~500 req/min.
4. **defuddle.md reader** (`https://defuddle.md/<url>`) — a second keyless reader on different
   infrastructure, tried before the browser rung.
5. **Browser rung** (`HARVESTER_BROWSER=1`, needs the optional `[browser]` extra) — renders the
   page in a real system Chrome via Patchright; passes passive bot walls including Cloudflare's
   managed challenge. Every request the page makes is SSRF-checked at the route layer.
6. **Open-access mirror** — extracts a DOI from the URL (or scrapes a `citation_pdf_url` /
   `citation_doi` meta tag from the blocked landing page), then resolves through Europe PMC PDF →
   the PMC OA service direct PDF → PMC article HTML → the full open-access chain (Unpaywall¹,
   OpenAlex, Semantic Scholar, Europe PMC, OpenAIRE, Zenodo, eLife, PLOS, NBER, CORE, DOAJ,
   arXiv/ar5iv/OSF) → a Wayback Machine snapshot. This is the headline path for walled academic
   publishers.

¹ Unpaywall now requires an operator email per call; keyless runs skip it cleanly — set
`HARVESTER_CONTACT_EMAIL` to enable it.

PDF/Office/archive downloads share the spirit of the ladder: an empty or 403 download is retried
with curl_cffi impersonation, and a walled publisher PDF URL that carries an extractable DOI pivots
to the open-access mirror before giving up. A `.pdf` URL that serves an HTML wall page instead of a
PDF is not a dead end either: the (already-downloaded) HTML is scraped for a `citation_pdf_url` /
`citation_doi` tag and that candidate is tried before the mirror chain; a PDF that downloads fine
but converts to empty text (scanned/corrupt) also pivots to the mirror chain before erroring, on
the theory that a bad copy at one URL doesn't mean no open-access copy exists anywhere.

Hard IP-reputation blocks (publisher DRM, a paywall requiring authentication) are not solvable
without a residential exit node — that is out of scope, and the server surfaces a clear error
message when it applies.

---

## Format matrix

| Input | Converter | Cache subdir |
|---|---|---|
| Web page / HTML | trafilatura (`favor_recall`; image refs kept as URLs) | `.cache/html/` |
| PDF URL or file (header-sniffed for extensionless URLs) | pymupdf4llm | `.cache/pdf/` |
| DOCX, XLSX, PPTX | Docling | `.cache/docx/` `.cache/xlsx/` `.cache/pptx/` |
| CSV | MarkItDown | `.cache/csv/` |
| JSON | literal pretty-print in a ` ```json ``` ` block | `.cache/json/` |
| Image (JPG/PNG/GIF/WebP/BMP/SVG/TIFF) | saved locally, path returned for vision | `.cache/<ext>/` |
| Archive (.zip / .tar(.gz/.bz2/.xz) / .7z / .rar) | safe listing, member on request | `.cache/<kind>/` |
| Archive member — call `archive(source, member="path/to/member")` (internally addressed `source::member`) | routed by member extension | `.cache/archive_member/` |
| Local path or `file://` URL | same routing, no network | same tree |

**Images:** `fetch` never downloads image binaries and never OCRs — `![](remote-url)` references
are left as URLs in the Markdown. To VIEW a figure, pass its URL to the `fetchImage` tool, which
saves the bytes locally and returns a path readable by vision tools.

**Header-sniff routing:** content-type and magic bytes override the URL extension, so
`https://arxiv.org/pdf/1706.03762` (no `.pdf`) is correctly routed to pymupdf4llm. A `.pdf` URL
that actually serves an HTML wall is rejected rather than fed to the PDF parser as a silent wrong
document.

**DOI / ISBN fast-path:** bare DOIs (`10.xxxx/...`), `doi:` prefixes, `doi.org` URLs, bare ISBNs
and `isbn:` strings skip the HTTP ladder and go straight to the open-access resolver. Bare
PMID/PMCID identifiers resolve through Europe PMC as well.

---

## Tools

The server exposes six tools (`search` appears only when a search backend is configured — see
below).

### `fetch`

Converts one or many **sources** to clean Markdown, returned in input order.

- `sources` (array of strings, **required**, 1–50 items) — each item is a **location** (web URL,
  local path, or `file://` URL → web page / PDF / DOCX / XLSX / PPTX / CSV / JSON) or an
  **unambiguous identifier** (a DOI as `10.xxxx/…`, `doi:…`, or a `doi.org` URL; a book as
  `isbn:9780262300988` or a bare ISBN; a PMID or PMCID). Identifiers are resolved to a free, legal
  copy and then converted. Mix kinds in one batch; a failing item returns a descriptive per-item
  error naming the next tool/argument to try, and the rest still return.
- `refresh` (boolean, optional, default **false**) — bypass the cache entirely (including the
  negative cache for a previously-failed source), re-fetch, overwrite the cached artifact, and
  return the new content. Normally unnecessary: a cached web page, PDF, or other volatile document
  older than `HARVESTER_CACHE_TTL` is re-fetched automatically; images and archive members are
  served from cache until a refresh is explicitly requested.
- `size_only` (boolean, optional, default **false**) — when true, fetch + cache the full content as
  normal but return only `{size, chars, path}` per source (`size` = estimated TOKENS via an
  over-counting heuristic, `chars` = raw character count, `path` = cache file). Reuses the same
  cache entry a normal fetch would (no duplicate download); slice the cached `path` from disk.

`fetch` returns document Markdown only (image references left as URLs). Pass it an image, an archive,
or a bare title and it points you at the right sibling tool (`fetchImage`, `archive`, `findWorks`)
instead of guessing. Each result carries a short header (source, cache_status, method, bytes,
tokens, fetched_at, cache path) followed by the content. Content past
`HARVESTER_MAX_INLINE_CHARS` is truncated inline with a note naming the cache path and the char
offset to resume from — the cache file always holds the COMPLETE text, truncated or not.

### `findWorks`

The scholarly counterpart of web search: a TITLE or free-text bibliographic query → a ranked list
of candidate works (papers + books), each with a ready-to-use `fetch:` handle. It downloads
nothing — you pick a candidate and pass its handle to `fetch` (the `findWorks → fetch` pattern). Papers
come from OpenAlex; books from Open Library + Project Gutenberg.

- `query` (string, **required**) — a paper/book title or free-text query.
- `limit` (integer, optional, default **8**, range 1–25) — maximum candidate works to return.

### `search`

An open-web search — title · URL · snippet · engine — to triage; retrieve the URLs you want with
`fetch` (the `search → fetch` pattern). Returns links and snippets, not cleaned content.

- `query` (string, **required**) — the web search query.
- `count` (integer, optional, default **8**, range 1–20) — maximum results.
- `lang` (string, optional, default `""`) — language/locale to bias the search (e.g. `zh`, `ja`,
  `pt-BR`); routes the query to that language's native engines to reach non-English literature.
- `engines` (string, optional, default `""`) — comma-separated SearXNG engines to restrict to
  (e.g. `google,brave` or `naver,yahoo`). Omit for the default aggregated set.

**Backends (see [Web search](#web-search)):** the tool is shown only when a backend is configured.

### `fetchImage`

Fetches one or many images and returns their LOCAL FILE PATHS to read with vision — figures,
photos, charts, scanned pages. Images are saved, not OCR'd; open the returned path to see the
content.

- `sources` (array of strings, **required**, 1–50 items) — image URLs or local image paths. Each
  is saved under `.cache/<ext>/` (e.g. `.cache/png/`, `.cache/jpg/`) and its path is returned in
  order.

### `archive`

Safely browses a single archive — `.zip` / `.tar(.gz/.bz2/.xz)` / `.7z` / `.rar` — by URL or local
path. Two-step, like `findWorks → fetch`.

- `source` (string, **required**) — URL or local path of the archive.
- `member` (string, optional, default `null`) — omit to get the SAFE member listing (names + sizes;
  nothing is extracted to disk). Give a member name from that listing to fetch just that member,
  converted to Markdown. Path-traversal and symlink members are refused; member-count/size caps are
  enforced; the archive is never auto-extracted.

### `searchCache`

Searches every page already cached under `.cache/` for a regex pattern — recall what you have
already fetched without re-crawling. It locates WHICH cached pages match; it does not return their
text. Returns each hit's source URL, match count, a sample line, and the cached `md_path` — `fetch`
the source again (served from cache) or read `md_path` directly for the content.

- `pattern` (string, **required**) — regex pattern.
- `max_results` (integer, optional, default **50**, range 1–1000) — maximum matching pages.
- `ignore_case` (boolean, optional, default **true**) — case-insensitive search.

---

## Prompts

- `fetch` — fetch a URL or local path and return its extracted Markdown, rendered the same way as
  the `fetch` tool (header + truncation note on success, a descriptive error naming the next move
  on failure). Argument: `url` (required). Images/archives redirect to `fetchImage`/`archive`,
  same as the tool.

---

## Web search

The `search` tool has two backends. It is **always advertised** so MCP clients see a stable
six-tool surface; with no backend configured a call returns a clear "set `SEARXNG_URL` /
`BRAVE_API_KEY`" message instead of the tool vanishing mid-session.

- **`SEARXNG_URL`** (primary) — base URL of a self-hosted [SearXNG](https://searxng.org)
  (e.g. `http://127.0.0.1:8888`). Free, self-hosted, aggregates 200+ engines (less single-engine /
  SEO bias than a plain Google search), and routes per-language to regional engines — the
  multilingual lever behind the `lang` / `engines` parameters.
- **`BRAVE_API_KEY`** (managed fallback) — a Brave Search API token, used when SearXNG is
  unconfigured, down, or blocked for a locale.
- **`HARVESTER_DISABLE_SEARCH`** — set to `1`/`true`/`yes`/`on` to force-hide the `search` tool
  entirely, for deployments that want no web search at all.

The tool runs when `SEARXNG_URL` and/or `BRAVE_API_KEY` is set (SearXNG first, then Brave); it is
listed regardless unless `HARVESTER_DISABLE_SEARCH` is truthy.

---

## Local-path support and deny-paths

Local files and `file://` URLs are fully supported, routed by the same detector with no network
access. Credential and key files are refused — the deny list covers:

- **Directories:** `.ssh`, `.gnupg`, `.aws`, `.password-store`, `.docker`.
- **Names:** `id_rsa`, `id_ed25519`, `id_dsa`, `id_ecdsa`, `credentials`, `.netrc`, `.pgpass`,
  `.htpasswd`, `shadow`, `master.key`.
- **Suffixes:** `.pem`, `.key`, `.p12`, `.pfx`, `.keystore`, `.jks`, `.asc`, `.gpg`, `.kdbx`,
  `.ppk`, `.env` (and `.env.*`), plus any name containing a private-key marker
  (`_rsa`, `_ed25519`, `_dsa`, `_ecdsa`).

The check runs on both a path and its symlink target, so an innocently named symlink can't smuggle
a secret. Outbound web fetches also apply an SSRF guard that refuses private / internal / link-local
hosts (including obfuscated IP forms).

---

## Cache

The cache lives at `.cache/` at the project root (the directory containing `pyproject.toml`).
Every artifact is type-partitioned:

```
.cache/
  html/<slug>__<sha1>.md
  pdf/<slug>__<sha1>.md       (and the source .pdf alongside)
  docx/  xlsx/  pptx/  csv/  json/
  png/   jpg/   gif/   webp/  …     (images and image-bearing kinds)
  archive_member/<slug>__<sha1>.md
```

Slugs are filesystem-safe and suffixed with a 10-char SHA-1 of the input for collision safety.

**Location.** Resolved in this order:

1. `WEBFETCH_DIR` — legacy override, a full path, still honoured verbatim, takes precedence.
2. `HARVESTER_CACHE_DIR` — a directory name (or path). A **relative** value resolves inside the
   project root, so the cache stays with harvester rather than wherever the process was launched
   from; an **absolute** value is used verbatim.
3. `.cache/` at the project root — the default.

**Expiry.** Artifacts of a VOLATILE kind — a web page, a PDF, an Office document (DOCX/XLSX/PPTX),
CSV, JSON, or TXT — go stale after `HARVESTER_CACHE_TTL` seconds (default `86400`, one day) and are
re-fetched on the next request. `HARVESTER_CACHE_TTL=0` disables expiry entirely, so every cached
artifact is served until explicitly refreshed. **Images and archive members never expire** — they
are content addressed by URL, not publications that get edited in place, and re-downloading a
byte-identical file on a timer would burn bandwidth for nothing. Age is read from the `fetched_at`
frontmatter written at cache time, not the file's mtime, so touching a file never silently extends
its life.

Failing fetches are held in a short-lived in-memory negative cache (default 120 s, see
`HARVESTER_NEG_TTL`) so a dead source is not re-hammered; a repeat call inside the window returns
the same error annotated with the seconds remaining before a retry is worth attempting. A
transient failure (timeout, connection error, or an HTTP 429) instead gets the much shorter
`HARVESTER_NEG_TTL_TRANSIENT` window, since it's likely to clear up on its own soon. A DOI-type
input is canonicalized before it hits the cache, so a bare DOI, a `doi:` prefix, and a
`https://doi.org/…` URL for the same work share one entry instead of each independently
re-running (and re-failing) the same open-access chain. Concurrent calls for the same input share
a single in-flight fetch.

**Bypassing the cache.** Pass `refresh: true` to the `fetch` tool to skip the cache entirely
(including the negative cache), re-fetch, overwrite the cached artifact, and return the new
content — see [`fetch`](#fetch).

A successful fetch that needed more than one rescue attempt (e.g. the initial request was
walled and curl_cffi impersonation or the open-access mirror chain won instead) records which
rungs were tried in the cache frontmatter and the result header (`rungs: direct,
chrome-impersonation, ...`); a terminal error names every rung it walked before giving up, so a
dead source's error is itself the evidence that re-fetching won't help.

---

## Installation

The package is **not yet published to PyPI**. Install from source or straight from GitHub.

### From source

```bash
git clone https://github.com/mreza0100/harvester-web-mcp
cd harvester-web-mcp
uv sync            # install dependencies into the local venv
uv run harvester   # run the server (stdio)
```

### From GitHub

```bash
# run without cloning
uvx --from git+https://github.com/mreza0100/harvester-web-mcp harvester

# or install into an environment
pip install git+https://github.com/mreza0100/harvester-web-mcp
python -m harvester
```

### Once published to PyPI

```bash
uvx --from harvester-mcp harvester     # or: pip install harvester-mcp
```

---

## Configuration

Add to your MCP client config (the `git+https` form needs no clone):

```json
{
  "mcpServers": {
    "harvester": {
      "command": "uvx",
      "args": ["--from", "git+https://github.com/mreza0100/harvester-web-mcp", "harvester"]
    }
  }
}
```

To relocate the cache and enable web search, pass environment variables:

```json
{
  "mcpServers": {
    "harvester": {
      "command": "uvx",
      "args": ["--from", "git+https://github.com/mreza0100/harvester-web-mcp", "harvester"],
      "env": {
        "WEBFETCH_DIR": "/path/to/cache",
        "SEARXNG_URL": "http://127.0.0.1:8888"
      }
    }
  }
}
```

### Environment variables

| Variable | Default | Effect |
|---|---|---|
| `WEBFETCH_DIR` | unset | Legacy override: relocate the on-disk cache to this full path verbatim. Takes precedence over `HARVESTER_CACHE_DIR`. |
| `HARVESTER_CACHE_DIR` | `.cache` | Name (or path) of the cache directory. A relative value resolves inside the project root; an absolute value is used verbatim. Ignored when `WEBFETCH_DIR` is set. |
| `HARVESTER_CACHE_TTL` | `86400` (1 day) | Seconds before a cached web page/PDF/Office/CSV/JSON/TXT artifact is considered stale and re-fetched. `0` disables expiry. Images and archive members never expire regardless of this setting. |
| `HARVESTER_MAX_INLINE_CHARS` | `50000` | Cap on the Markdown returned inline per source; the cache file always holds the full text (read the returned path for everything). |
| `HARVESTER_NEG_TTL` | `120` | Seconds a failed fetch is remembered in the negative cache. |
| `HARVESTER_NEG_TTL_TRANSIENT` | `15` | Shorter negative-cache TTL for a transient failure (timeout, connection error, HTTP 429). |
| `HARVESTER_PDF_OCR` | off | Run Tesseract OCR on image-only PDF pages (slow; off by default for fast text-layer extraction). A scanned PDF fetched from the web always gets ONE automatic OCR pass when its text layer converts to empty, regardless of this setting — this flag turns OCR on for every PDF. |
| `HARVESTER_PDF_LAYOUT` | off | Run the pymupdf4llm layout model for higher table/heading fidelity (slow). |
| `HARVESTER_LOCALIZE_IMAGES` | off | When set, `fetch` downloads the figures a successfully fetched page references into the cache and rewrites the markdown links to local paths (read them with vision). Off by default: image refs stay as URLs and are viewed via `fetchImage`. |
| `HARVESTER_BROWSER` | off | Enable the real-browser wall-bypass rung: when a page survives httpx + Chrome-impersonation + Jina, it is rendered in a system Chrome via [Patchright](https://github.com/Kaliiiiiiiiii-Vinyasa/patchright) (passes passive bot walls incl. Cloudflare's managed challenge; never solves CAPTCHAs). Needs the optional `browser` extra: `uv sync --extra browser` / `pip install harvester-mcp[browser]`, then `patchright install chrome`. |
| `JINA_API_KEY` | empty | Optional free [Jina Reader](https://jina.ai) key. Keyless works at 20 req/min; the key raises the ceiling to ~500 req/min. Never required. |
| `HARVESTER_LOG_FILE` | unset | Write logs to this file. |
| `HARVESTER_LOG_LEVEL` | `INFO` | Log level. |
| `SEARXNG_URL` | empty | Self-hosted SearXNG base URL — primary `search` backend (enables the tool). |
| `BRAVE_API_KEY` | empty | Brave Search API token — fallback `search` backend (enables the tool). |
| `HARVESTER_DISABLE_SEARCH` | off | Force-hide the `search` tool even when a backend is configured. |
| `HARVESTER_CONTACT_EMAIL` | **empty** | Polite-pool contact for scholarly APIs. Empty by default, so the published tool sends **no** contact; operators opt in. When set, it is sent as a `mailto:` UA suffix and `?email=` / `?mailto=` parameter — better rate limits. **Unpaywall now hard-requires a real email per call** (placeholder addresses are rejected with HTTP 422), so setting this also ENABLES the Unpaywall source; keyless runs skip it cleanly. All other sources work without it. |
| `CORE_API_KEY` | empty | Optional [CORE](https://core.ac.uk) API key. |
| `SEMANTIC_SCHOLAR_API_KEY` | empty | Optional Semantic Scholar API key. |
| `GOOGLE_BOOKS_API_KEY` | empty | Optional Google Books API key. |
| `HARVESTER_LOCAL_ROOTS` | empty (unconfined) | `os.pathsep`-separated directories that local reads are confined to. Empty means the stdio default: any path the deny-list allows. The `http` transport sets this to the cache root automatically — see [Remote transport](#remote-transport-claudeai-custom-connector). |
| `HARVESTER_AUTH_PASSPHRASE` | empty | `http` transport: operator passphrase for the OAuth consent screen. Setting this (or `HARVESTER_STATIC_TOKEN`) turns authentication on. |
| `HARVESTER_STATIC_TOKEN` | empty | `http` transport: fixed bearer token accepted verbatim, for non-browser clients. |
| `HARVESTER_STATE_DIR` | `.harvester-state/` beside the cache | `http` transport: where OAuth clients and refresh-token digests are stored. Deliberately outside the cache root. |

Every scholarly source works **keyless** — the optional API keys only raise rate limits and
full-text coverage; nothing is hard-gated behind a key.

### CLI flags

| Flag | Effect |
|---|---|
| `--user-agent <UA>` | Override the default User-Agent (`Mozilla/5.0 (compatible; harvester/1.0)`). |
| `--proxy-url <URL>` | Route all outbound requests through this proxy (e.g. `http://proxy:8080`). |
| `--ignore-robots-txt` | No-op, kept for compatibility — robots.txt is never enforced. |
| `--transport stdio\|http` | `stdio` (default) for a local MCP client; `http` serves Streamable HTTP for remote clients. |
| `--host <addr>` | `http` only: bind address for the external (authenticated when credentials are set) gateway (default `127.0.0.1`). |
| `--port <n>` | `http` only: bind port for the external gateway (default `8081`). |
| `--public-url <url>` | `http` only: externally visible origin. Must match the URL entered in the client — OAuth resource metadata is compared exactly. |
| `--internal-host <addr>` | `http` only: bind address for the unauthenticated internal gateway (default `127.0.0.1`). |
| `--internal-port <n>` | `http` only: port for the unauthenticated internal gateway onto the same harvester instance (default `8082`; `0` disables it). Only started when the external gateway has credentials configured. |
| `--allow-unauthenticated` | `http` only: permit binding an unauthenticated server to a non-loopback address. |

---

## Remote transport (claude.ai custom connector)

`--transport http` serves the **same** tools over Streamable HTTP at `/mcp`, so a remote client such
as a claude.ai custom connector can reach them. Tool behaviour is identical to stdio; the threat
model is not, and four things change with it.

**Local reads are confined to the cache.** `fetchImage` and `archive` hand back cache paths for the
model to read, so local paths keep working — but only inside `.cache/`. Everything else on the disk
is refused by construction (`detect.LOCAL_ROOTS`), not by a denylist that has to anticipate every
sensitive filename. The deny-list still applies within the cache.

**Authentication is optional, but never accidental.** With no passphrase and no static token the
server runs open, which is right for a loopback/internal instance:

```bash
harvester --transport http --port 8081            # open, loopback only
```

Binding an unauthenticated server to a non-loopback address is refused unless you pass
`--allow-unauthenticated` — harvester fetches arbitrary URLs, so an open instance is an open proxy.

Set a passphrase (and optionally a static token) and it becomes a full OAuth 2.1 authorization
server — Dynamic Client Registration, PKCE `S256`, refresh-token rotation, revocation:

```bash
export HARVESTER_AUTH_PASSPHRASE='…'      # typed on the consent screen
export HARVESTER_STATIC_TOKEN='…'         # optional: fixed bearer for curl / Claude Code
harvester --transport http --host 0.0.0.0 --port 8081 \
          --public-url https://harvester.example.com
```

**The public URL must match exactly.** The protected-resource metadata `resource` field is compared
verbatim against the URL entered in the client, so `--public-url` has to be the externally visible
origin — not the bind address.

**Two gateways onto one instance.** `--transport http` can start a second, always-unauthenticated
gateway alongside the external one, on `--internal-host`/`--internal-port` (default
`127.0.0.1:8082`; `0` disables it). Both gateways sit in front of the SAME harvester — the cache,
the in-flight dedup, and the negative cache in `dispatch` are shared — so the internal gateway is
for a local/tailnet caller that shouldn't have to carry a bearer token on every call, not a second
server. It is started only when the external gateway actually has credentials configured: with no
credentials the external gateway is already open, so a second open door would add exposure and no
capability, and it is skipped instead (logged, never silently dropped). The split is enforced by
**separate sockets, not by URL path** — both gateways serve `/mcp`, but on different ports, so
publishing the external port through a tunnel can never expose the internal one; there is no path
on the public port that reaches it.

```bash
harvester --transport http --host 0.0.0.0 --port 8081 --public-url https://harvester.example.com \
          --internal-host 127.0.0.1 --internal-port 8082   # unauthenticated, loopback only
```

| Endpoint | Purpose |
|---|---|
| `/mcp` | Streamable HTTP (GET/POST/DELETE). Returns `401` + `WWW-Authenticate: Bearer resource_metadata="…"` when unauthenticated. |
| `/.well-known/oauth-protected-resource/mcp` | RFC 9728 resource metadata — points at the authorization server. |
| `/.well-known/oauth-authorization-server` | RFC 8414 metadata — advertises `S256` and the registration endpoint. |
| `/authorize` · `/token` · `/register` · `/revoke` | OAuth 2.1 endpoints. |
| `/consent` | The passphrase form. |
| `/healthz` | Liveness, plus whether auth is on. |

Add it in Claude under **Settings → Connectors → Add custom connector** with the `/mcp` URL. Claude
registers itself via DCR, sends you to `/consent`, and stores the resulting tokens.

### Deployment: systemd (native) or container

The systemd path is the simpler default — one process, no image to build, no ~3GB of torch/CUDA
pulled in for docling — because local-file reads are already confined at the application layer
(`detect.set_local_roots`), not by a container filesystem boundary; running natively just means that
application-layer confinement is the only lock, with no container-level backstop behind it.

```ini
# /etc/systemd/system/harvester-mcp.service
[Service]
Type=simple
WorkingDirectory=/path/to/harvester
EnvironmentFile=/path/to/harvester/.env.remote
ExecStart=/path/to/harvester/.venv/bin/python -m harvester --transport http --host 127.0.0.1 \
    --port 8081 --public-url ${HARVESTER_PUBLIC_URL} --internal-host 127.0.0.1 --internal-port 8082
Restart=always
RestartSec=5
```

Harden the unit with `NoNewPrivileges=true`, `ProtectSystem=strict`, and `ReadWritePaths=` scoped to
the cache dir, `HARVESTER_STATE_DIR`, and wherever logs land — nothing else this process touches
needs to write. Expose the **external** gateway (`8081`) only through a tunnel that terminates TLS
(Tailscale Funnel, Cloudflare Tunnel, a reverse proxy) rather than publishing the port; never do the
same for the **internal** gateway (`8082`) — it carries no authentication, by design, for
local/tailnet callers only.

**Container, as a secondary option:** `compose.remote.yaml` runs it with no host bind mounts,
loopback-only published ports, a read-only root filesystem, and dropped capabilities. Cache and auth
state live in separate named volumes — separate because the cache is remotely readable through
`fetch` and the credential store must not sit inside it.

```bash
cp .env.remote.example .env.remote      # fill in passphrase / token / public URL
docker compose -f compose.remote.yaml --env-file .env.remote up -d --build
```

Expose the **external** gateway (`8081`) with a tunnel that terminates TLS (Tailscale Funnel,
Cloudflare Tunnel, or a reverse proxy) rather than publishing the port — the compose file binds
`127.0.0.1` on purpose, and a Docker `0.0.0.0` publish would be internet-facing despite a
default-deny firewall. The **internal** gateway (`8082`) is also loopback-published for the same
reason, but it must never be given to the tunnel — it carries no authentication, by design, for
local/tailnet callers only.

---

## Development

```bash
uv sync
uv run pytest -q
uv run ruff check .
uv run pyright src/
```

---

## License

MIT. See [LICENSE](LICENSE). This project builds on MIT-licensed fetch-server scaffolding; the
original copyright notice is retained in the license file.
