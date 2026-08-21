"""Open-access resolver: a scholarly identifier → ordered candidate free-fulltext URLs.

Given a DOI (or arXiv id, or an OSF/SocArXiv DOI, or a paper title) this queries the LEGAL
public scholarly APIs — Unpaywall, OpenAlex, Semantic Scholar, Crossref, Europe PMC, CORE,
DOAJ, arXiv, OSF — and returns an ordered list of `Candidate` URLs pointing at a free, legal
full-text copy (PDF preferred). The caller fetches + converts each candidate through the
normal harvester ladder; this module ONLY finds links, it never downloads article content.

Every network call is polite (one request, a descriptive UA + contact email), short-timeout,
and NEVER raises — a dead or oddly-shaped source yields `[]` and a warning log. Endpoints +
JSON paths were verified live against real DOIs on 2026-06-28; per-source quirks are noted
inline. No shadow libraries — only API-sanctioned sources.
"""
from __future__ import annotations

import asyncio
import os
import re
from dataclasses import dataclass, field
from typing import TYPE_CHECKING, Callable
from urllib.parse import quote

from .log import get_logger

if TYPE_CHECKING:
    from httpx import AsyncClient

log = get_logger("oa")

# Polite-pool contact for scholarly APIs (NCBI/Crossref/Unpaywall). Empty by default so the
# published tool sends no personal contact; operators opt in via HARVESTER_CONTACT_EMAIL.
CONTACT_EMAIL = os.environ.get("HARVESTER_CONTACT_EMAIL", "")
_UA = f"harvester-mcp/1.0 (mailto:{CONTACT_EMAIL})" if CONTACT_EMAIL else "harvester-mcp/1.0"

# Optional API keys — every source still works keyless, just rate-limited / less full-text.
CORE_API_KEY = os.environ.get("CORE_API_KEY", "")
S2_API_KEY = os.environ.get("SEMANTIC_SCHOLAR_API_KEY", "")

# DOI prefixes that route to a deterministic specialist resolver (skip the generic chain).
_ARXIV_PREFIX = "10.48550/arxiv."
_OSF_PREFIXES = {
    "10.31235": "socarxiv", "10.31234": "psyarxiv", "10.31219": "osf",
    "10.31730": "africarxiv", "10.35542": "edarxiv", "10.33767": "sportrxiv",
}

# Lower priority = tried first. Base per source; adjusted for kind / oa_status / version.
_SOURCE_BASE = {
    "arxiv": 0, "osf": 2, "citation_pdf_url": 4,
    "unpaywall": 10, "openalex": 12, "semanticscholar": 14, "europepmc": 16,
    "crossref": 30, "core": 40, "doaj": 45,
    # books
    "gutenberg": 5, "oapen": 8, "internetarchive": 18, "doab": 22, "googlebooks": 50,
}

GOOGLE_BOOKS_API_KEY = os.environ.get("GOOGLE_BOOKS_API_KEY", "")
# ISBN-13 (978/979 + 9 digits + check) or ISBN-10 (9 digits + check), hyphens/spaces stripped.
_ISBN_RE = re.compile(r"^(?:97[89])?\d{9}[\dX]$")


@dataclass(order=True)
class Candidate:
    """One ordered candidate free-fulltext URL. Only `priority` participates in sorting."""
    priority: int
    url: str = field(compare=False)
    source: str = field(compare=False)
    oa_status: str = field(default="", compare=False)
    version: str = field(default="", compare=False)
    kind_hint: str = field(default="pdf", compare=False)
    note: str = field(default="", compare=False)


def _score(source: str, oa_status: str = "", version: str = "", kind: str = "pdf") -> int:
    base = _SOURCE_BASE.get(source, 50)
    if kind != "pdf":
        base += 8  # a landing/html page is worse than a direct PDF
    if oa_status == "bronze":
        base += 6  # free-but-unlicensed publisher copy — may be re-paywalled
    if version and version != "publishedVersion":
        base += 2
    return base


def _bare_doi(text: str) -> str | None:
    """Pull a bare DOI out of a DOI string or a doi.org URL."""
    m = re.search(r"10\.\d{4,9}/[-._;()/:A-Za-z0-9]+", text or "")
    return m.group(0).rstrip(".,;:)>]") if m else None


def _norm_tokens(s: str) -> set[str]:
    return set(re.findall(r"[a-z0-9]+", (s or "").lower()))


def _similar(a: str, b: str) -> float:
    """Jaccard token overlap — a cheap guard against noisy title→DOI matches."""
    ta, tb = _norm_tokens(a), _norm_tokens(b)
    if not ta or not tb:
        return 0.0
    return len(ta & tb) / len(ta | tb)


def _match(query: str, title: str) -> float:
    """Title-match score with a containment bonus — a short query fully inside a verbose title
    (e.g. 'Frankenstein' in 'Frankenstein; or, The Modern Prometheus') scores high, not 0.2."""
    j = _similar(query, title)
    qt, tt = _norm_tokens(query), _norm_tokens(title)
    if qt and qt <= tt:
        return max(j, 0.85)
    return j


def _best_location_handle(w: dict) -> str:
    """A fetchable handle (pdf url / landing page / derived arXiv pdf) from an OpenAlex work's
    locations — so a paper with no top-level DOI/oa_url (RAG, BERT) is still findable."""
    locs = [loc for loc in [w.get("best_oa_location"), w.get("primary_location"),
                            *(w.get("locations") or [])] if isinstance(loc, dict)]
    for loc in locs:
        if loc.get("pdf_url"):
            return loc["pdf_url"]
    for loc in locs:
        m = re.search(r"arxiv\.org/abs/([^/?#]+)", loc.get("landing_page_url") or "", re.IGNORECASE)
        if m:
            return f"https://arxiv.org/pdf/{m.group(1)}"
    for loc in locs:
        if loc.get("landing_page_url"):
            return loc["landing_page_url"]
    return ""


async def _get_json(client: "AsyncClient", url: str, headers: dict | None = None, timeout: int = 15):
    """GET → parsed JSON (dict/list) or None. Never raises; logs every failure."""
    h = {"User-Agent": _UA}
    if headers:
        h.update(headers)
    try:
        r = await client.get(url, headers=h, timeout=timeout, follow_redirects=True)
    except Exception as e:
        log.warning("oa GET failed %s: %s", url, e)
        return None
    if r.status_code >= 400:
        log.info("oa %s -> HTTP %d", url, r.status_code)
        return None
    try:
        return r.json()
    except Exception as e:
        log.warning("oa non-JSON from %s: %s", url, e)
        return None


# ── per-source resolvers (each → list[Candidate], never raises) ──────────────────

async def from_unpaywall(doi: str, client: "AsyncClient") -> list[Candidate]:
    data = await _get_json(client, f"https://api.unpaywall.org/v2/{quote(doi)}?email={CONTACT_EMAIL}")
    if not isinstance(data, dict) or not data.get("is_oa"):
        return []
    status = data.get("oa_status") or ""
    out: list[Candidate] = []
    best = data.get("best_oa_location") or {}
    if best.get("url_for_pdf"):
        out.append(Candidate(_score("unpaywall", status, best.get("version") or "", "pdf"),
                             best["url_for_pdf"], "unpaywall", status, best.get("version") or "", "pdf"))
    elif best.get("url"):
        out.append(Candidate(_score("unpaywall", status, best.get("version") or "", "html"),
                             best["url"], "unpaywall", status, best.get("version") or "", "html"))
    for loc in (data.get("oa_locations") or []):
        u = loc.get("url_for_pdf")
        if u:
            out.append(Candidate(_score("unpaywall", status, loc.get("version") or "", "pdf") + 5,
                                 u, "unpaywall", status, loc.get("version") or "", "pdf"))
    return out


async def from_openalex(doi: str, client: "AsyncClient") -> list[Candidate]:
    data = await _get_json(
        client, f"https://api.openalex.org/works/https://doi.org/{quote(doi)}?mailto={CONTACT_EMAIL}"
    )
    if not isinstance(data, dict):
        return []
    oa = data.get("open_access") or {}
    status = oa.get("oa_status") or ""
    out: list[Candidate] = []
    if oa.get("oa_url"):
        out.append(Candidate(_score("openalex", status, "", "pdf"),
                             oa["oa_url"], "openalex", status, "", "pdf"))
    for loc in (data.get("locations") or []):
        if loc.get("is_oa") and loc.get("pdf_url"):
            out.append(Candidate(_score("openalex", status, loc.get("version") or "", "pdf") + 4,
                                 loc["pdf_url"], "openalex", status, loc.get("version") or "", "pdf"))
    return out


async def from_semanticscholar(doi: str, client: "AsyncClient") -> list[Candidate]:
    headers = {"x-api-key": S2_API_KEY} if S2_API_KEY else None
    data = await _get_json(
        client,
        f"https://api.semanticscholar.org/graph/v1/paper/DOI:{quote(doi)}"
        "?fields=openAccessPdf,externalIds,isOpenAccess",
        headers=headers,
    )
    if not isinstance(data, dict):
        return []
    out: list[Candidate] = []
    pdf = data.get("openAccessPdf") or {}
    if pdf.get("url"):
        status = (pdf.get("status") or "").lower()
        out.append(Candidate(_score("semanticscholar", status, "", "pdf"),
                             pdf["url"], "semanticscholar", status, "", "pdf"))
    ext = data.get("externalIds") or {}
    if ext.get("ArXiv"):  # arXiv copy is a guaranteed free PDF
        out.append(Candidate(_score("arxiv"), f"https://arxiv.org/pdf/{ext['ArXiv']}",
                             "arxiv", "green", "submittedVersion", "pdf"))
    if ext.get("PubMedCentral"):
        pmcid = str(ext["PubMedCentral"])
        if not pmcid.upper().startswith("PMC"):
            pmcid = f"PMC{pmcid}"
        out.append(Candidate(_score("europepmc"), f"https://europepmc.org/articles/{pmcid}?pdf=render",
                             "europepmc", "green", "", "pdf"))
    return out


async def from_crossref(doi: str, client: "AsyncClient") -> list[Candidate]:
    data = await _get_json(client, f"https://api.crossref.org/works/{quote(doi)}?mailto={CONTACT_EMAIL}")
    if not isinstance(data, dict):
        return []
    msg = data.get("message") or {}
    out: list[Candidate] = []
    for link in (msg.get("link") or []):
        if link.get("content-type") == "application/pdf" and link.get("URL"):
            out.append(Candidate(_score("crossref"), link["URL"], "crossref", "", "", "pdf"))
    return out


async def from_core(doi: str, client: "AsyncClient") -> list[Candidate]:
    headers = {"Authorization": f"Bearer {CORE_API_KEY}"} if CORE_API_KEY else None
    data = await _get_json(client, f"https://api.core.ac.uk/v3/works/{quote(doi)}", headers=headers)
    if not isinstance(data, dict):
        return []
    out: list[Candidate] = []
    if data.get("downloadUrl"):
        out.append(Candidate(_score("core"), data["downloadUrl"], "core", "", "", "pdf",
                             note="CORE copy — verify it matches the paper"))
    for su in (data.get("sourceFulltextUrls") or []):
        if su:
            out.append(Candidate(_score("core") + 1, su, "core", "", "", "pdf"))
    return out


async def from_doaj(doi: str, client: "AsyncClient") -> list[Candidate]:
    data = await _get_json(client, f"https://doaj.org/api/search/articles/doi:{quote(doi)}")
    if not isinstance(data, dict):
        return []
    out: list[Candidate] = []
    for res in (data.get("results") or [])[:1]:
        for link in ((res.get("bibjson") or {}).get("link") or []):
            if link.get("type") == "fulltext" and link.get("url"):
                out.append(Candidate(_score("doaj", "", "", "html"), link["url"], "doaj", "", "", "html"))
    return out


async def from_europepmc(doi: str, client: "AsyncClient") -> list[Candidate]:
    from . import mirror
    pmcid = await mirror.doi_to_pmcid(doi, client)
    if not pmcid:
        return []
    return [Candidate(_score("europepmc"), f"https://europepmc.org/articles/{pmcid}?pdf=render",
                     "europepmc", "green", "", "pdf", note=pmcid)]


async def from_osf(doi: str, client: "AsyncClient") -> list[Candidate]:
    # OSF DOI form: 10.NNNNN/osf.io/{GUID}; filter[doi] is broken, so parse the GUID.
    m = re.search(r"osf\.io/([A-Za-z0-9_]+)", doi, re.IGNORECASE)
    if not m:
        return []
    guid = m.group(1)
    data = await _get_json(client, f"https://api.osf.io/v2/preprints/{guid}/",
                           headers={"Accept": "application/json"})
    if not isinstance(data, dict):
        return []
    rel = (((data.get("data") or {}).get("relationships") or {}).get("primary_file") or {}).get("data") or {}
    fid = rel.get("id")
    if fid:
        return [Candidate(_score("osf"), f"https://osf.io/download/{fid}/", "osf", "green", "", "pdf")]
    return []


def arxiv_candidates(arxiv_id: str) -> list[Candidate]:
    aid = arxiv_id.strip().lstrip("/")
    return [Candidate(_score("arxiv"), f"https://arxiv.org/pdf/{aid}", "arxiv", "green", "", "pdf")]


# ── page-scraped <meta> links (the rabbit hole on a blocked publisher page) ──────
_META_PDF_RE = re.compile(
    r'<meta[^>]+name=["\']citation_pdf_url["\'][^>]+content=["\']([^"\']+)["\']', re.IGNORECASE)
_META_PDF_RE2 = re.compile(
    r'<meta[^>]+content=["\']([^"\']+)["\'][^>]+name=["\']citation_pdf_url["\']', re.IGNORECASE)
_META_DOI_RE = re.compile(
    r'<meta[^>]+name=["\'](?:citation_doi|dc\.identifier|DC\.Identifier)["\'][^>]+content=["\']([^"\']+)["\']',
    re.IGNORECASE)
_META_TITLE_RE = re.compile(
    r'<meta[^>]+name=["\']citation_title["\'][^>]+content=["\']([^"\']+)["\']', re.IGNORECASE)


def extract_meta_links(page_html: str) -> tuple[str | None, str | None, str | None]:
    """From a (possibly paywalled) publisher page, scrape (doi, citation_pdf_url, title).

    Most journal landing pages expose a `citation_pdf_url` and `citation_doi` <meta> tag even
    when the body is gated — the key to going down the rabbit hole from a blocked URL.
    """
    pdf_m = _META_PDF_RE.search(page_html) or _META_PDF_RE2.search(page_html)
    doi_m = _META_DOI_RE.search(page_html)
    title_m = _META_TITLE_RE.search(page_html)
    doi = _bare_doi(doi_m.group(1)) if doi_m else None
    pdf = pdf_m.group(1) if pdf_m else None
    title = title_m.group(1) if title_m else None
    if pdf or doi:
        log.info("meta-scrape -> pdf=%s doi=%s", bool(pdf), doi)
    return doi, pdf, title


# ── title → DOI (noisy; guarded by a token-similarity check) ─────────────────────

async def _title_to_doi(title: str, client: "AsyncClient") -> str | None:
    data = await _get_json(
        client, f"https://api.openalex.org/works?filter=title.search:{quote(title)}"
        f"&per_page=5&mailto={CONTACT_EMAIL}")
    if isinstance(data, dict):
        for w in (data.get("results") or []):
            name = w.get("display_name") or w.get("title") or ""
            if _similar(title, name) >= 0.6 and w.get("doi"):
                doi = _bare_doi(w["doi"])
                if doi:
                    log.info("title→doi (openalex) %r -> %s", title, doi)
                    return doi
    data = await _get_json(
        client, f"https://api.crossref.org/works?query.bibliographic={quote(title)}"
        f"&rows=5&select=DOI,title&mailto={CONTACT_EMAIL}")
    items = (((data or {}).get("message") or {}).get("items")) or [] if isinstance(data, dict) else []
    for it in items:
        titles = it.get("title") or []
        name = titles[0] if titles else ""
        if _similar(title, name) >= 0.6 and it.get("DOI"):
            log.info("title→doi (crossref) %r -> %s", title, it["DOI"])
            return it["DOI"]
    log.info("title→doi: no confident match for %r", title)
    return None


async def _arxiv_by_title(title: str, client: "AsyncClient") -> list[Candidate]:
    url = f"https://export.arxiv.org/api/query?search_query=ti:%22{quote(title)}%22&max_results=3"
    try:
        r = await client.get(url, headers={"User-Agent": _UA}, timeout=20, follow_redirects=True)
        text = r.text
    except Exception as e:
        log.warning("arxiv title search failed: %s", e)
        return []
    # Similarity-gate the match: an arXiv ti: search is fuzzy and will happily return a paper
    # whose title merely CONTAINS the query — never substitute a different work for the asked one.
    for m in re.finditer(r"<entry>(.*?)</entry>", text, re.DOTALL):
        entry = m.group(1)
        tm = re.search(r"<title>(.*?)</title>", entry, re.DOTALL)
        im = re.search(r"<id>https?://arxiv\.org/abs/([^<]+)</id>", entry)
        if tm and im and _similar(title, re.sub(r"\s+", " ", tm.group(1)).strip()) >= 0.6:
            return arxiv_candidates(im.group(1).strip())
    log.info("arxiv title search: no confident match for %r", title)
    return []


# ── find: a title / free-text query → a ranked list of CANDIDATE works (no fetch) ──

def _fmt_authors(names: list[str]) -> str:
    names = [n for n in names if n]
    if not names:
        return ""
    if len(names) <= 3:
        return ", ".join(names)
    return f"{names[0]} et al."


async def _find_papers(query: str, client: "AsyncClient", limit: int) -> list[dict]:
    data = await _get_json(
        client, f"https://api.openalex.org/works?filter=title.search:{quote(query)}"
        f"&per_page={limit}&mailto={CONTACT_EMAIL}")
    out: list[dict] = []
    for w in (data or {}).get("results", []) if isinstance(data, dict) else []:
        title = w.get("display_name") or ""
        doi = _bare_doi(w.get("doi") or "")
        oa = w.get("open_access") or {}
        fetch = doi or oa.get("oa_url") or _best_location_handle(w)  # dig locations for RAG/BERT-style records
        if not fetch:
            continue
        authors = _fmt_authors([(a.get("author") or {}).get("display_name", "")
                                for a in (w.get("authorships") or [])])
        out.append({
            "kind": "paper", "title": title, "authors": authors, "year": w.get("publication_year"),
            "fetch": fetch, "source": "openalex", "free": oa.get("oa_status") or "",
            "match": round(_match(query, title), 2),
        })
    return out


async def _find_arxiv(query: str, client: "AsyncClient", limit: int) -> list[dict]:
    """arXiv ti: search → a candidate (similarity-gated) so on-arXiv papers (RAG, BERT, …) that
    OpenAlex indexes thinly still surface as a find candidate."""
    out = []
    for c in await _arxiv_by_title(query, client):
        out.append({"kind": "paper", "title": query, "authors": "", "year": None,
                    "fetch": c.url, "source": "arxiv", "free": "green", "match": 0.9})
    return out


async def _find_crossref(query: str, client: "AsyncClient", limit: int) -> list[dict]:
    """Crossref bibliographic search → candidates for works OpenAlex indexes thinly or not at
    all (newly published, conference proceedings, non-OA-heavy fields). Similarity-gated like
    every title→work path — Crossref's relevance ranking happily returns near-misses."""
    data = await _get_json(
        client, f"https://api.crossref.org/works?query.bibliographic={quote(query)}"
        f"&rows={limit}&select=DOI,title,author,issued&mailto={CONTACT_EMAIL}")
    out: list[dict] = []
    for it in (data.get("message", {}).get("items") or []) if isinstance(data, dict) else []:
        titles = it.get("title") or []
        title = titles[0] if titles else ""
        doi = _bare_doi(it.get("DOI") or "")
        if not title or not doi:
            continue
        m = _match(query, title)
        if m < 0.5:
            continue
        issued = (((it.get("issued") or {}).get("date-parts") or [[None]])[0] or [None])[0]
        out.append({
            "kind": "paper", "title": title,
            "authors": _fmt_authors([
                " ".join(x for x in (a.get("given"), a.get("family")) if x)
                for a in (it.get("author") or [])]),
            "year": issued, "fetch": doi, "source": "crossref", "free": "",
            "match": round(m, 2),
        })
    return out


async def _find_s2(query: str, client: "AsyncClient", limit: int) -> list[dict]:
    """Semantic Scholar paper search → candidates with a DIRECT free-PDF handle when one exists
    (openAccessPdf / arXiv id), so a findable-and-free paper is fetchable in one step."""
    headers = {"x-api-key": S2_API_KEY} if S2_API_KEY else None
    data = await _get_json(
        client,
        f"https://api.semanticscholar.org/graph/v1/paper/search?query={quote(query)}"
        f"&limit={limit}&fields=title,year,authors,externalIds,openAccessPdf",
        headers=headers)
    out: list[dict] = []
    for p in (data.get("data") or []) if isinstance(data, dict) else []:
        title = p.get("title") or ""
        if not title:
            continue
        m = _match(query, title)
        if m < 0.5:
            continue
        ext = p.get("externalIds") or {}
        oa_pdf = (p.get("openAccessPdf") or {}).get("url") or ""
        doi = _bare_doi(ext.get("DOI") or "")
        if doi:
            fetch = doi
        elif ext.get("ArXiv"):
            fetch = f"https://arxiv.org/pdf/{ext['ArXiv']}"
        elif oa_pdf:
            fetch = oa_pdf
        else:
            continue  # no fetchable handle → useless as a candidate
        free = "green" if (oa_pdf or ext.get("ArXiv")) else ""
        out.append({
            "kind": "paper", "title": title,
            "authors": _fmt_authors([a.get("name", "") for a in (p.get("authors") or [])]),
            "year": p.get("year"), "fetch": fetch, "source": "semanticscholar",
            "free": free, "match": round(m, 2),
        })
    return out


async def _find_books(query: str, client: "AsyncClient", limit: int) -> list[dict]:
    out: list[dict] = []
    ol = await _get_json(
        client, f"https://openlibrary.org/search.json?q={quote(query)}"
        f"&fields=title,author_name,first_publish_year,isbn,ebook_access&limit={limit}")
    for d in (ol or {}).get("docs", []) if isinstance(ol, dict) else []:
        isbns = d.get("isbn") or []
        if not isbns:
            continue
        title = d.get("title") or ""
        out.append({
            "kind": "book", "title": title, "authors": _fmt_authors(d.get("author_name") or []),
            "year": d.get("first_publish_year"), "fetch": f"isbn:{isbns[0]}", "source": "openlibrary",
            "free": d.get("ebook_access") or "", "match": round(_match(query, title), 2),
        })
    gd = await _get_json(client, f"https://gutendex.com/books?search={quote(query)}")
    for r in ((gd or {}).get("results", []) if isinstance(gd, dict) else [])[:3]:
        if r.get("copyright"):
            continue
        fmts = r.get("formats") or {}
        url = next((v for k, v in fmts.items() if k.startswith("text/html")), None) \
            or next((v for k, v in fmts.items() if k.startswith("text/plain")), None)
        if not url:
            continue
        title = r.get("title") or ""
        out.append({
            "kind": "book", "title": title,
            "authors": _fmt_authors([a.get("name", "") for a in (r.get("authors") or [])]),
            "year": None, "fetch": url, "source": "gutenberg", "free": "pd",
            "match": round(_match(query, title), 2),
        })
    return out


async def find_works(query: str, client: "AsyncClient", limit: int = 8) -> list[dict]:
    """A title / free-text query → a ranked list of candidate works (papers + books).

    Each candidate carries a `fetch` handle (a DOI, an `isbn:` string, or a direct URL) that the
    caller passes to `fetch` to retrieve the chosen one. This module FINDS; it does not download.
    """
    query = query.strip().strip('"').strip("'")
    if not query:
        return []
    papers, arxiv, crossref, s2, books = await asyncio.gather(
        _safe_find(_find_papers, query, client, limit),
        _safe_find(_find_arxiv, query, client, limit),
        _safe_find(_find_crossref, query, client, limit),
        _safe_find(_find_s2, query, client, limit),
        _safe_find(_find_books, query, client, limit),
    )
    # Dedupe by fetch handle (keep the highest match per handle).
    best: dict[str, dict] = {}
    for c in papers + arxiv + crossref + s2 + books:
        k = c["fetch"]
        if k and (k not in best or c["match"] > best[k]["match"]):
            best[k] = c
    out = list(best.values())
    # Rank: highest title match first; among equal matches, an actually-free copy ranks above a closed one.
    free = {"pd", "gold", "green", "diamond", "hybrid", "bronze", "public"}
    out.sort(key=lambda c: (-c["match"], 0 if (c.get("free") or "").lower() in free else 1))
    log.info("find_works %r -> %d candidate(s)", query, len(out))
    return out[:limit]


async def _safe_find(fn: Callable, query: str, client: "AsyncClient", limit: int) -> list[dict]:
    try:
        return await fn(query, client, limit)
    except Exception as e:
        log.warning("find source %s failed for %r: %s", getattr(fn, "__name__", fn), query, e)
        return []


# ── public entry points ──────────────────────────────────────────────────────────

async def _safe(fn: Callable, doi: str, client: "AsyncClient") -> list[Candidate]:
    try:
        return await fn(doi, client)
    except Exception as e:
        log.warning("oa source %s failed for %s: %s", getattr(fn, "__name__", fn), doi, e)
        return []


def _dedupe(cands: list[Candidate]) -> list[Candidate]:
    seen: set[str] = set()
    out: list[Candidate] = []
    for c in sorted(cands):
        if c.url in seen:
            continue
        seen.add(c.url)
        out.append(c)
    return out


def doi_prefix(doi: str) -> str:
    return doi.split("/", 1)[0]


async def resolve_doi(doi: str, client: "AsyncClient") -> list[Candidate]:
    """A DOI → ordered candidate free-fulltext URLs. Specialist prefixes short-circuit."""
    doi = doi.strip()
    low = doi.lower()
    if low.startswith(_ARXIV_PREFIX):
        return _dedupe(arxiv_candidates(doi[len(_ARXIV_PREFIX):]))
    if doi_prefix(doi) in _OSF_PREFIXES:
        cands = await _safe(from_osf, doi, client)
        if cands:
            return _dedupe(cands)  # OSF is authoritative for its own DOIs
    sources = [from_unpaywall, from_openalex, from_semanticscholar,
               from_europepmc, from_crossref, from_core, from_doaj]
    results = await asyncio.gather(*[_safe(f, doi, client) for f in sources])
    cands = [c for sub in results for c in sub]
    log.info("resolve_doi %s -> %d candidate(s)", doi, len(cands))
    return _dedupe(cands)


async def resolve_title(title: str, client: "AsyncClient") -> list[Candidate]:
    """A paper title → candidate free-fulltext URLs (title→DOI→chain, then arXiv search)."""
    title = title.strip().strip('"').strip("'")
    doi = await _title_to_doi(title, client)
    if doi:
        cands = await resolve_doi(doi, client)
        if cands:
            return cands
    return await _arxiv_by_title(title, client)


# ── books: ISBN / title → free full-text (OAPEN · Internet Archive · Gutendex · DOAB · Google Books) ──

def _isbn13_ok(d: str) -> bool:
    if len(d) != 13 or not d.isdigit():
        return False
    return sum((1 if i % 2 == 0 else 3) * int(c) for i, c in enumerate(d)) % 10 == 0


def _isbn10_ok(d: str) -> bool:
    if len(d) != 10:
        return False
    total = 0
    for i, c in enumerate(d):
        if c == "X" and i == 9:
            v = 10
        elif c.isdigit():
            v = int(c)
        else:
            return False
        total += (10 - i) * v
    return total % 11 == 0


def normalize_isbn(s: str) -> str | None:
    """Strip hyphens/spaces → a CHECKSUM-VALID bare ISBN-10/13, else None.

    Rejects empty, wrong-length, bad-checksum (typos like 012345678X), and all-same-digit
    placeholders (0000000000) so a bogus ISBN can't fuzzy-match an unrelated book.
    """
    d = re.sub(r"[^0-9Xx]", "", s or "").upper()
    if not d or len(set(d.replace("X", ""))) <= 1:  # empty or all-one-digit → not a real ISBN
        return None
    if len(d) == 13:
        return d if _isbn13_ok(d) else None
    if len(d) == 10:
        return d if _isbn10_ok(d) else None
    return None


async def from_oapen(query: str, client: "AsyncClient") -> list[Candidate]:
    """OAPEN OA academic books: search → item → ORIGINAL/PDF bitstream retrieve link (unwalled)."""
    isbn = normalize_isbn(query)
    q = isbn if isbn else f"title:{query}"
    arr = await _get_json(client, f"https://library.oapen.org/rest/search?query={quote(q)}&limit=5",
                          headers={"Accept": "application/json"})
    if not isinstance(arr, list):
        return []
    for item in arr[:3]:
        uuid = item.get("uuid")
        if not uuid:
            continue
        detail = await _get_json(client, f"https://library.oapen.org/rest/items/{uuid}?expand=bitstreams",
                                 headers={"Accept": "application/json"})
        if not isinstance(detail, dict):
            continue
        for bs in (detail.get("bitstreams") or []):
            if (bs.get("bundleName") == "ORIGINAL" and bs.get("mimeType") == "application/pdf"
                    and bs.get("retrieveLink")):
                url = "https://library.oapen.org" + bs["retrieveLink"]
                return [Candidate(_score("oapen"), url, "oapen", "gold", "", "pdf", note=item.get("name", ""))]
    return []


async def from_internetarchive(query: str, client: "AsyncClient") -> list[Candidate]:
    """Open Library index → Internet Archive full text, gated on access-restricted-item (no lending copies)."""
    isbn = normalize_isbn(query)
    ocaid = None
    if isbn:
        data = await _get_json(client, f"https://openlibrary.org/isbn/{isbn}.json")
        if isinstance(data, dict):
            ocaid = data.get("ocaid")
    else:
        data = await _get_json(
            client, f"https://openlibrary.org/search.json?q={quote(query)}&fields=ia,ebook_access,title&limit=5")
        if isinstance(data, dict):
            for doc in (data.get("docs") or []):
                if doc.get("ebook_access") == "public" and doc.get("ia"):
                    ocaid = doc["ia"][0]
                    break
    if not ocaid:
        return []
    meta = await _get_json(client, f"https://archive.org/metadata/{ocaid}")
    if not isinstance(meta, dict):
        return []
    if (meta.get("metadata") or {}).get("access-restricted-item") == "true":
        log.info("internetarchive %s is lending-only — not served", ocaid)
        return []  # legality gate: controlled-lending items are NOT downloadable
    files = meta.get("files") or []
    names = [str(f.get("name", "")) for f in files]
    pdf = next((n for n in names if n.lower().endswith(".pdf") and not n.endswith("_encrypted.pdf")), None)
    txt = next((n for n in names if n.lower().endswith("_djvu.txt")), None)
    out: list[Candidate] = []
    if pdf:
        out.append(Candidate(_score("internetarchive"), f"https://archive.org/download/{ocaid}/{quote(pdf)}",
                             "internetarchive", "pd", "", "pdf"))
    if txt:
        out.append(Candidate(_score("internetarchive", "", "", "txt"),
                             f"https://archive.org/download/{ocaid}/{quote(txt)}",
                             "internetarchive", "pd", "", "txt"))
    return out


async def from_gutendex(query: str, client: "AsyncClient") -> list[Candidate]:
    """Project Gutenberg (public-domain literary classics) by title+author — no ISBN lookup."""
    if normalize_isbn(query):
        return []
    data = await _get_json(client, f"https://gutendex.com/books?search={quote(query)}")
    if not isinstance(data, dict):
        return []
    for res in (data.get("results") or []):
        if res.get("media_type") not in (None, "Text") or res.get("copyright"):
            continue
        fmts = res.get("formats") or {}
        url, kind = None, "html"
        for mime, u in fmts.items():
            if mime.startswith("text/html"):
                url, kind = u, "html"
                break
        if not url:
            for mime, u in fmts.items():
                if mime.startswith("text/plain"):
                    url, kind = u, "txt"
                    break
        if url and "zip" not in url.lower():
            return [Candidate(_score("gutenberg", "", "", kind), url, "gutenberg", "pd", "", kind,
                             note=res.get("title", ""))]
    return []


async def from_doab(query: str, client: "AsyncClient") -> list[Candidate]:
    """DOAB OA-books discovery → downloadUrl; if that's a DOI, expand via the article chain."""
    isbn = normalize_isbn(query)
    arr = None
    try:
        if isbn:
            r = await client.post(
                "https://directory.doabooks.org/rest/items/find-by-metadata-field",
                json={"key": "oapen.relation.isbn", "value": isbn},
                headers={"User-Agent": _UA, "Content-Type": "application/json"},
                timeout=15, follow_redirects=True)
            arr = r.json() if r.status_code < 400 else None
        else:
            arr = await _get_json(client, f"https://directory.doabooks.org/rest/search?query={quote(query)}")
    except Exception as e:
        log.warning("doab lookup failed: %s", e)
        return []
    if not isinstance(arr, list) or not arr:
        return []
    uuid = arr[0].get("uuid")
    if not uuid:
        return []
    detail = await _get_json(
        client, f"https://directory.doabooks.org/rest/items/{uuid}?expand=metadata,bitstreams")
    if not isinstance(detail, dict):
        return []
    download = None
    pools = [m for bs in (detail.get("bitstreams") or []) for m in (bs.get("metadata") or [])]
    pools += (detail.get("metadata") or [])
    for m in pools:
        if m.get("key") == "oapen.identifier.downloadUrl" and m.get("value"):
            download = m["value"]
            break
    if not download:
        return []
    if "doi.org/" in download:  # publisher-hosted — route through the DOI chain for a real PDF
        doi = _bare_doi(download)
        if doi:
            cands = await resolve_doi(doi, client)
            if cands:
                return cands
    return [Candidate(_score("doab"), download, "doab", "gold", "", "pdf", note=detail.get("name", ""))]


async def from_googlebooks(query: str, client: "AsyncClient") -> list[Candidate]:
    """Google Books — public-domain PDF/EPUB links. Needs a key (keyless = 0-quota from datacenter IPs)."""
    if not GOOGLE_BOOKS_API_KEY:
        return []
    isbn = normalize_isbn(query)
    q = f"isbn:{isbn}" if isbn else query
    data = await _get_json(
        client, f"https://www.googleapis.com/books/v1/volumes?q={quote(q)}&country=US&key={GOOGLE_BOOKS_API_KEY}")
    if not isinstance(data, dict):
        return []
    for item in (data.get("items") or []):
        acc = item.get("accessInfo") or {}
        dl = (acc.get("pdf") or {}).get("downloadLink")
        if acc.get("publicDomain") and dl:
            return [Candidate(_score("googlebooks"), dl, "googlebooks", "pd", "", "pdf",
                             note="Google Books PD scan — byte-fetch may be browser-gated")]
    return []


async def resolve_book(query: str, client: "AsyncClient") -> list[Candidate]:
    """An ISBN or book title → ordered candidate free-fulltext URLs (OA academic + public-domain)."""
    query = query.strip().strip('"').strip("'")
    if len(query) < 2:  # empty/blank → no silent "arbitrary book" result
        return []
    sources = [from_oapen, from_internetarchive, from_gutendex, from_doab, from_googlebooks]
    results = await asyncio.gather(*[_safe(f, query, client) for f in sources])
    cands = [c for sub in results for c in sub]
    log.info("resolve_book %r -> %d candidate(s)", query, len(cands))
    return _dedupe(cands)
