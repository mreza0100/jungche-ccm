"""Live smoke tests for the OA resolver — ONE real call per API, just to prove the wiring.

These hit the network, so they are SKIPPED unless RUN_OA_INTEGRATION=1. They are deliberately
minimal (one input per source) — not a stress test. The hermetic unit coverage is in test_oa.py.

    RUN_OA_INTEGRATION=1 uv run pytest tests/test_oa_integration.py -q
"""
import os

import pytest

from harvester import oa

pytestmark = pytest.mark.skipif(
    not os.environ.get("RUN_OA_INTEGRATION"),
    reason="live network test — set RUN_OA_INTEGRATION=1 to run",
)

# Known-good inputs (verified live 2026-06-28).
PLOS = "10.1371/journal.pone.0173664"        # PLOS ONE, gold OA
PCBI = "10.1371/journal.pcbi.1003285"        # PLOS Comp Biol, in PMC
NATURE = "10.1038/s41586-020-2649-2"         # NumPy, Crossref pdf link
OSF_DOI = "10.31235/osf.io/yvzcb_v1"         # SocArXiv preprint
OAPEN_ISBN = "9780262300988"                 # Suber, "Open Access" (OAPEN)
DOAB_ISBN = "9783110730234"                  # DOAB-indexed OA book


async def _client():
    from httpx import AsyncClient
    return AsyncClient()


@pytest.mark.parametrize("fn,doi", [
    (oa.from_unpaywall, PLOS),
    (oa.from_openalex, PLOS),
    (oa.from_semanticscholar, PLOS),
    (oa.from_crossref, NATURE),
    (oa.from_doaj, PLOS),
    (oa.from_europepmc, PCBI),
    (oa.from_osf, OSF_DOI),
])
async def test_article_source_returns_a_candidate(fn, doi):
    async with await _client() as client:
        cands = await fn(doi, client)
    assert cands, f"{fn.__name__} returned no candidate for {doi}"
    assert cands[0].url.startswith("http")


async def test_core_runs_without_error():
    # CORE often has no copy (verified) — assert it RUNS and returns a list, not that it hits.
    async with await _client() as client:
        cands = await oa.from_core(PLOS, client)
    assert isinstance(cands, list)


@pytest.mark.parametrize("fn,ident", [
    (oa.from_oapen, OAPEN_ISBN),
    (oa.from_gutendex, "frankenstein shelley"),
    (oa.from_internetarchive, "Frankenstein Shelley"),
    (oa.from_doab, DOAB_ISBN),
])
async def test_book_source_returns_a_candidate(fn, ident):
    async with await _client() as client:
        cands = await fn(ident, client)
    assert cands, f"{fn.__name__} returned no candidate for {ident}"
    assert cands[0].url.startswith("http")


async def test_resolve_doi_arxiv_is_offline_deterministic():
    async with await _client() as client:
        cands = await oa.resolve_doi("10.48550/arXiv.1706.03762", client)
    assert cands[0].url == "https://arxiv.org/pdf/1706.03762"
