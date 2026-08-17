"""Open-access mirror resolver: DOI → PubMed Central / Europe PMC.

All functions are polite (one request, descriptive UA, never raise —
return None / b"" on any failure).
"""
from __future__ import annotations

import os
import re
from typing import TYPE_CHECKING

from .log import get_logger

if TYPE_CHECKING:
    from httpx import AsyncClient

log = get_logger("mirror")

# Polite-pool contact — empty by default (published tool sends no personal email); operators
# opt in via HARVESTER_CONTACT_EMAIL.
CONTACT_EMAIL = os.environ.get("HARVESTER_CONTACT_EMAIL", "")
_MIRROR_UA = f"harvester-mcp/1.0 (mailto:{CONTACT_EMAIL})" if CONTACT_EMAIL else "harvester-mcp/1.0"
_EMAIL_PARAM = f"&email={CONTACT_EMAIL}" if CONTACT_EMAIL else ""

# DOI pattern per CrossRef spec: 10.XXXX/...
_DOI_RE = re.compile(r"10\.\d{4,9}/[-._;()/:A-Za-z0-9]+", re.IGNORECASE)
_DOI_TRAIL_RE = re.compile(r"[.,;:)>\]]+$")


def extract_doi(text: str) -> str | None:
    """Return the first DOI found in *text*, stripping trailing punctuation, or None."""
    m = _DOI_RE.search(text)
    if not m:
        return None
    doi = _DOI_TRAIL_RE.sub("", m.group(0))
    return doi or None


async def doi_to_pmcid(doi: str, client: "AsyncClient") -> str | None:
    """Resolve a DOI to a PubMed Central PMCID via the PMC ID Converter API.

    Returns e.g. "PMC10450651" or None on any failure.
    """
    url = (
        "https://pmc.ncbi.nlm.nih.gov/tools/idconv/api/v1/articles/"
        f"?ids={doi}&format=json&tool=harvester-mcp{_EMAIL_PARAM}"
    )
    try:
        r = await client.get(
            url, headers={"User-Agent": _MIRROR_UA}, timeout=15, follow_redirects=True
        )
        data = r.json()
        records = data.get("records") or []
        if records and records[0].get("pmcid"):
            pmcid = str(records[0]["pmcid"])
            log.info("doi_to_pmcid %s -> %s", doi, pmcid)
            return pmcid
    except Exception as e:
        log.warning("doi_to_pmcid failed for %s: %s", doi, e)
        return None
    log.info("doi_to_pmcid %s -> no PMCID", doi)
    return None


async def pmid_to_pmcid(pmid: str, client: "AsyncClient") -> str | None:
    """Resolve a PubMed ID to a PubMed Central PMCID via the PMC ID Converter API.

    Mirrors :func:`doi_to_pmcid` (same idconv endpoint, contact, and parsing). Returns e.g.
    "PMC10450651", or None on any failure (e.g. the article is abstract-only / paywalled).
    """
    url = (
        "https://pmc.ncbi.nlm.nih.gov/tools/idconv/api/v1/articles/"
        f"?ids={pmid}&format=json&tool=harvester-mcp{_EMAIL_PARAM}"
    )
    try:
        r = await client.get(
            url, headers={"User-Agent": _MIRROR_UA}, timeout=15, follow_redirects=True
        )
        data = r.json()
        records = data.get("records") or []
        if records and records[0].get("pmcid"):
            pmcid = str(records[0]["pmcid"])
            log.info("pmid_to_pmcid %s -> %s", pmid, pmcid)
            return pmcid
    except Exception as e:
        log.warning("pmid_to_pmcid failed for %s: %s", pmid, e)
        return None
    log.info("pmid_to_pmcid %s -> no PMCID", pmid)
    return None


async def europepmc_pdf(pmcid: str, client: "AsyncClient") -> bytes:
    """Fetch the full-text PDF for *pmcid* from Europe PMC (follows redirects).

    Tries the verified-live article render endpoint first, then the legacy getPdf API.
    (The old /europepmc/webservices/rest/{src}/{id}/fullTextPDF endpoint now 404s — don't use it.)
    Returns raw PDF bytes (starts with b"%PDF") or b"" on any failure.
    """
    urls = (
        f"https://europepmc.org/articles/{pmcid}?pdf=render",
        f"https://europepmc.org/api/getPdf?pmcid={pmcid}",
    )
    for url in urls:
        try:
            r = await client.get(
                url, headers={"User-Agent": _MIRROR_UA}, timeout=60, follow_redirects=True
            )
            data = r.content
            if data.startswith(b"%PDF"):
                log.info("europepmc_pdf %s -> %d bytes via %s", pmcid, len(data), url)
                return data
        except Exception as e:
            log.warning("europepmc_pdf failed for %s at %s: %s", pmcid, url, e)
    log.info("europepmc_pdf %s -> no PDF", pmcid)
    return b""


async def europepmc_fulltext_xml(pmcid: str, client: "AsyncClient") -> str:
    """Fetch the full-text XML for *pmcid* from Europe PMC.

    Returns the XML text or "" on any failure.
    """
    url = f"https://www.ebi.ac.uk/europepmc/webservices/rest/{pmcid}/fullTextXML"
    try:
        r = await client.get(
            url, headers={"User-Agent": _MIRROR_UA}, timeout=30, follow_redirects=True
        )
        return r.text
    except Exception as e:
        log.warning("europepmc_fulltext_xml failed for %s: %s", pmcid, e)
        return ""


def europepmc_figures_zip_url(pmcid: str) -> str:
    """URL for the supplementary files (figures) ZIP for *pmcid* on Europe PMC."""
    return f"https://www.ebi.ac.uk/europepmc/webservices/rest/{pmcid}/supplementaryFiles"


def pmc_article_url(pmcid: str) -> str:
    """URL for the unguarded HTML article page on PubMed Central."""
    return f"https://pmc.ncbi.nlm.nih.gov/articles/{pmcid}/"


async def wayback_raw_url(url: str, client: "AsyncClient") -> str | None:
    """Check the Wayback Machine for a snapshot of *url*.

    Returns the raw snapshot URL (with ``id_`` modifier to strip the toolbar) or None.
    """
    api = f"https://archive.org/wayback/available?url={url}"
    try:
        r = await client.get(
            api, headers={"User-Agent": _MIRROR_UA}, timeout=15, follow_redirects=True
        )
        data = r.json()
        closest = data.get("archived_snapshots", {}).get("closest", {})
        if not closest.get("available"):
            log.info("wayback %s -> no snapshot", url)
            return None
        timestamp = closest.get("timestamp", "")
        if not timestamp:
            log.info("wayback %s -> snapshot without timestamp", url)
            return None
        log.info("wayback %s -> snapshot %s", url, timestamp)
        return f"https://web.archive.org/web/{timestamp}id_/{url}"
    except Exception as e:
        log.warning("wayback_raw_url failed for %s: %s", url, e)
        return None
