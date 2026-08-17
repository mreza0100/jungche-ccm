"""The on-disk cache layer.

Default root is `.cache/` at the PROJECT ROOT (the dir holding pyproject.toml). The directory is
configurable — `HARVESTER_CACHE_DIR` sets the name (relative names land inside the project root, so
the cache stays with harvester), and the legacy `WEBFETCH_DIR` still overrides the full path. The
cache is type-partitioned: every artifact lives under a per-kind subdir, e.g. `.cache/html/<slug>.md`,
`.cache/pdf/<slug>.md`, `.cache/png/<slug>.png`.

Freshness: a cached artifact of a VOLATILE kind (a web page, a PDF, an Office document — anything
whose source can change under a stable URL) goes stale after `HARVESTER_CACHE_TTL` seconds and is
re-fetched. Images and archive members are content, not publications: they are keyed by URL and do
not expire. `refresh=True` on a fetch bypasses the cache entirely regardless of kind.
"""

import hashlib
import os
import re
from datetime import datetime, timezone
from pathlib import Path
from typing import Tuple

from .log import get_logger
from .tokens import estimate_tokens

log = get_logger("cache")

# Treat a cached/extracted body shorter than this as "thin" or "trivial".
THIN_MIN_CHARS = 500
# A cached HTML body must exceed this to count as a usable hit (else re-fetch).
CACHE_MIN_BODY = 200

DEFAULT_CACHE_DIRNAME = ".cache"
DEFAULT_CACHE_TTL = 86400          # 1 day

# Kinds whose SOURCE can change under a stable URL — a page is republished, a PDF is revised, a
# spreadsheet is updated. These expire. Images and archive members are omitted on purpose: they are
# binary content addressed by URL, not publications that get edited in place, and re-downloading
# them on a timer would burn bandwidth for a byte-identical file.
VOLATILE_KINDS = frozenset({"html", "pdf", "docx", "xlsx", "pptx", "csv", "json", "txt"})


def cache_ttl() -> int:
    """Seconds before a volatile cached artifact is considered stale (env HARVESTER_CACHE_TTL).

    `0` disables expiry entirely (every cached artifact is served until explicitly refreshed).
    """
    raw = os.environ.get("HARVESTER_CACHE_TTL")
    if raw is None or not raw.strip():
        return DEFAULT_CACHE_TTL
    try:
        ttl = int(raw)
    except ValueError:
        log.warning("invalid HARVESTER_CACHE_TTL=%r; using default %d", raw, DEFAULT_CACHE_TTL)
        return DEFAULT_CACHE_TTL
    if ttl < 0:
        log.warning("negative HARVESTER_CACHE_TTL=%r; using default %d", raw, DEFAULT_CACHE_TTL)
        return DEFAULT_CACHE_TTL
    return ttl


def _fetched_at(md_path: Path, meta: "dict | None") -> float | None:
    """Epoch seconds the artifact was fetched, from frontmatter, falling back to file mtime.

    Returns None only when neither source is readable — the caller then treats the entry as FRESH
    rather than stale, so an unreadable timestamp can never turn a working cache into a refetch
    storm against someone else's server.
    """
    stamp = (meta or {}).get("fetched_at")
    if stamp:
        try:
            return datetime.strptime(stamp, "%Y-%m-%dT%H:%M:%SZ").replace(
                tzinfo=timezone.utc
            ).timestamp()
        except ValueError:
            log.debug("unparseable fetched_at %r in %s; falling back to mtime", stamp, md_path)
    try:
        return md_path.stat().st_mtime
    except OSError as e:
        log.debug("cannot stat %s: %s", md_path, e)
        return None


def is_stale(md_path: Path, kind: str, meta: "dict | None" = None) -> bool:
    """True when this cached artifact should be re-fetched because it has aged out.

    Only VOLATILE_KINDS expire; everything else is always fresh. Age comes from the `fetched_at`
    frontmatter written at cache time, so touching a file does not silently extend its life.

    `meta` is optional purely to save a re-read when the caller already parsed the frontmatter.
    Omitting it reads the header rather than falling through to mtime: mtime on a just-written file
    is always "now", so a caller who forgot the argument would get "fresh forever" — a disabled
    expiry that looks exactly like a working one.
    """
    if kind not in VOLATILE_KINDS:
        return False
    ttl = cache_ttl()
    if ttl <= 0:
        return False
    if meta is None:
        meta = read_frontmatter(md_path)
    when = _fetched_at(md_path, meta)
    if when is None:
        return False
    age = datetime.now(timezone.utc).timestamp() - when
    if age <= ttl:
        return False
    log.info("cache stale %s kind=%s age=%ds ttl=%ds — refetching", md_path, kind, int(age), ttl)
    return True


def _project_root() -> Path:
    """The dir containing pyproject.toml, found by walking up from this module.

    Robust to layout changes — no hardcoded parent index. Falls back to two levels up
    (src/harvester/ -> repo root) when no pyproject.toml is found (e.g. an installed wheel).
    """
    here = Path(__file__).resolve()
    for parent in here.parents:
        if (parent / "pyproject.toml").is_file():
            return parent
    return here.parents[2]


def cache_root() -> Path:
    """Root of the type-partitioned cache.

    Resolution order:
      1. `WEBFETCH_DIR` — a full path; the legacy override, still honoured verbatim.
      2. `HARVESTER_CACHE_DIR` — a directory NAME (or path). A relative value resolves inside the
         project root, so the cache stays with harvester rather than wherever it was launched from.
      3. `.cache/` at the project root.
    """
    env = os.environ.get("WEBFETCH_DIR")
    if env and env.strip():
        p = Path(env).expanduser()
    else:
        configured = os.environ.get("HARVESTER_CACHE_DIR", "").strip() or DEFAULT_CACHE_DIRNAME
        p = Path(configured).expanduser()
        if not p.is_absolute():
            p = _project_root() / p
    p.mkdir(parents=True, exist_ok=True)
    return p


# Backwards-compatible alias (older code/tests referenced harvester_dir()).
def harvester_dir() -> Path:
    return cache_root()


def slugify(key: str) -> str:
    """URL/path -> a filesystem-safe slug + 10-char sha1 of the key for collision safety.

    Strips any scheme, replaces every char outside [A-Za-z0-9._-] with '_', collapses runs,
    trims, caps the slug at 150 chars, and appends `__<sha1[:10]>`.
    """
    no_scheme = re.sub(r"^[a-z][a-z0-9+.-]*://", "", key, flags=re.IGNORECASE)
    slug = re.sub(r"[^A-Za-z0-9._-]", "_", no_scheme)
    slug = re.sub(r"_+", "_", slug).strip("_")
    h = hashlib.sha1(key.encode("utf-8")).hexdigest()[:10]
    return f"{slug[:150]}__{h}"


def cache_file(key: str, kind: str, ext: str = ".md") -> Path:
    """Path for a cached artifact: `<root>/<kind>/<slug><ext>`. Creates the subdir."""
    sub = cache_root() / kind
    sub.mkdir(parents=True, exist_ok=True)
    return sub / f"{slugify(key)}{ext}"


def split_frontmatter(text: str) -> Tuple[dict, str]:
    """Split a `---`-delimited YAML frontmatter header from the markdown body."""
    lines = text.splitlines(keepends=True)
    if not lines or lines[0].rstrip("\n") != "---":
        return {}, text
    meta: dict = {}
    i = 1
    while i < len(lines) and lines[i].rstrip("\n") != "---":
        line = lines[i].rstrip("\n")
        if ":" in line:
            k, _, v = line.partition(":")
            meta[k.strip()] = v.strip()
        i += 1
    body = "".join(lines[i + 1:]) if i < len(lines) else ""
    return meta, body.lstrip("\n")


def read_frontmatter(md_path: str | Path) -> dict:
    """Read just the YAML frontmatter of a cached artifact, without loading the whole body.

    Used by `describe.py` to enrich a result header with `tokens` / `fetched_at` straight from
    what `_write_md` already recorded. Returns `{}` (never raises) when the path doesn't exist,
    isn't readable as text, or carries no frontmatter block — callers must tolerate a missing
    key (non-`.md` cache artifacts like images/archives have no frontmatter at all).
    """
    try:
        with open(md_path, "r", encoding="utf-8", errors="ignore") as fh:
            head = fh.read(4096)  # the frontmatter block is always tiny; never read a huge body
    except OSError as e:
        log.debug("read_frontmatter cannot read %s: %s", md_path, e)
        return {}
    meta, _body = split_frontmatter(head)
    return meta


def _write_md(md_path: Path, key: str, method: str, body: str, extra: "dict[str, str] | None" = None) -> None:
    """Write a markdown artifact with a small YAML provenance header.

    `extra` adds optional additional frontmatter fields (e.g. `{"rungs": "direct, jina"}` — the
    Slice 2 rescue-graph trace) without changing the shape of an artifact that has none: existing
    readers (`split_frontmatter`/`read_frontmatter`) already parse arbitrary `key: value` lines,
    so old artifacts with no `extra` are byte-identical to before.
    """
    fetched_at = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    extra_lines = "".join(f"{k}: {v}\n" for k, v in (extra or {}).items())
    header = (
        "---\n"
        f"url: {key}\n"
        f"fetched_at: {fetched_at}\n"
        "source: harvester\n"
        f"method: {method}\n"
        f"token_count: {estimate_tokens(body)}\n"
        f"{extra_lines}"
        "---\n\n"
    )
    md_path.write_text(header + body, encoding="utf-8")
    log.debug("cache write %s method=%s chars=%d", md_path, method, len(body))


def search_cache(pattern: str, max_results: int = 50, ignore_case: bool = True) -> list[dict]:
    """Search every cached markdown body under the cache tree for `pattern` (regex)."""
    flags = re.IGNORECASE if ignore_case else 0
    try:
        rx = re.compile(pattern, flags)
    except re.error as e:
        log.warning("search_cache invalid regex %r: %s", pattern, e)
        raise ValueError(f"invalid regex pattern: {e}")
    results: list[dict] = []
    for md in sorted(cache_root().rglob("*.md")):
        try:
            text = md.read_text(encoding="utf-8", errors="ignore")
        except OSError as e:
            log.warning("search_cache cannot read %s: %s", md, e)
            continue
        _meta, body = split_frontmatter(text)
        hits = rx.findall(body)
        if not hits:
            continue
        sample = ""
        for line in body.splitlines():
            if rx.search(line):
                sample = line.strip()[:200]
                break
        results.append({
            "url": _meta.get("url") or str(md),
            "md_path": str(md),
            "matches": len(hits),
            "sample": sample,
        })
        if len(results) >= max_results:
            break
    log.info("search_cache /%s/ -> %d page(s)", pattern, len(results))
    return results
