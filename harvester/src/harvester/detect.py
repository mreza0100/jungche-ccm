"""Pure (no-network) format detection and security guards.

Nothing here touches the network or heavy converter deps — it only inspects names, magic
bytes, and content-types. Importing this module stays cheap.
"""

import os
from pathlib import Path

from .log import get_logger

log = get_logger("detect")

IMAGE_EXTS = {".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".tif", ".tiff", ".svg"}

_EXT_KIND = {
    ".pdf": "pdf", ".docx": "docx", ".xlsx": "xlsx", ".pptx": "pptx",
    ".csv": "csv", ".json": "json", ".zip": "zip", ".7z": "7z", ".rar": "rar",
    ".htm": "html", ".html": "html",
}
_TAR_SUFFIXES = (".tar.gz", ".tgz", ".tar.bz2", ".tbz2", ".tar.xz", ".txz", ".tar")
ARCHIVE_KINDS = {"zip", "tar", "7z", "rar"}
DOC_KINDS = {"pdf", "docx", "xlsx", "pptx", "csv", "json"}


def detect_kind(location: str) -> str:
    """Best-effort source kind from a URL/path by extension. Defaults to 'html'."""
    name = location.split("?", 1)[0].split("#", 1)[0].lower().rstrip("/")
    if name.endswith(_TAR_SUFFIXES):
        return "tar"
    ext = os.path.splitext(name)[1]
    if ext in _EXT_KIND:
        return _EXT_KIND[ext]
    if ext in IMAGE_EXTS:
        return "image"
    if ext in {".gz", ".bz2", ".xz"}:
        return "tar"
    return "html"


def image_ext(location: str, default: str = ".png") -> str:
    """The image file extension for a URL/path (used as the cache subdir name too)."""
    ext = os.path.splitext(location.split("?", 1)[0].split("#", 1)[0])[1].lower()
    return ext if ext in IMAGE_EXTS else default


def sniff_magic(head: bytes) -> str | None:
    """Identify a file by leading magic bytes (for local files with no/odd extension)."""
    if head.startswith(b"%PDF-"):
        return "pdf"
    if head.startswith(b"PK\x03\x04"):
        return "zip"  # also docx/xlsx/pptx — caller refines by extension
    if head.startswith(b"7z\xbc\xaf\x27\x1c"):
        return "7z"
    if head.startswith(b"Rar!\x1a\x07"):
        return "rar"
    if head.startswith(b"\x1f\x8b"):
        return "tar"  # gzip
    if head.startswith(b"BZh"):
        return "tar"  # bzip2
    if head.startswith(b"\xfd7zXZ\x00"):
        return "tar"  # xz
    if head.startswith(b"\xff\xd8\xff"):
        return "image"  # jpeg
    if head.startswith(b"\x89PNG\r\n\x1a\n"):
        return "image"  # png
    if head.startswith((b"GIF87a", b"GIF89a")):
        return "image"
    if head[:4] == b"RIFF" and head[8:12] == b"WEBP":
        return "image"
    return None


# ── security: confine local reads, refuse secrets/keys ────────────────────────
# System roots that must never be read regardless of file name — /proc/self/environ leaks the
# harvester process's entire environment (API keys/proxy creds); /etc holds passwd/shadow; etc.
_SYSTEM_ROOTS = ("/proc", "/sys", "/dev", "/etc")
_DENY_DIR_PARTS = {".ssh", ".gnupg", ".aws", ".password-store", ".docker", ".config", ".kube"}
_DENY_NAMES = {"id_rsa", "id_ed25519", "id_dsa", "id_ecdsa", "credentials",
               ".netrc", ".pgpass", ".htpasswd", "shadow", "master.key",
               "passwd", ".git-credentials", ".bash_history"}
_DENY_SUFFIXES = (".pem", ".key", ".p12", ".pfx", ".keystore", ".jks",
                  ".asc", ".gpg", ".kdbx", ".ppk", ".env")
_DENY_KEY_MARKERS = ("_rsa", "_ed25519", "_dsa", "_ecdsa")

def _parse_roots(raw: str) -> tuple[Path, ...]:
    """Parse an os.pathsep-separated root list into resolved dirs; unparseable entries are dropped
    with a log line rather than silently widening (or narrowing) confinement."""
    roots: list[Path] = []
    for part in raw.split(os.pathsep):
        part = part.strip()
        if not part:
            continue
        try:
            roots.append(Path(part).expanduser().resolve())
        except (OSError, RuntimeError) as e:
            log.warning("ignoring unresolvable HARVESTER_LOCAL_ROOTS entry %r: %s", part, e)
    return tuple(roots)


# Confinement roots for LOCAL reads. Empty (the default) = unconfined, which is right for the stdio
# server whose caller already owns the machine. The remote/Streamable-HTTP deployment sets this to
# the harvester cache root (see `remote.build_app`), so a remote caller can still read back the
# artifacts harvester itself produced — `fetchImage` and `archive` hand out cache paths and the
# model reads them — while the rest of the filesystem is structurally out of reach.
#
# This is a CONFINEMENT, not a denylist: everything outside the roots is refused by default, so a
# file nobody thought to name is still unreachable. The denylist below stays on INSIDE the roots.
LOCAL_ROOTS: tuple[Path, ...] = _parse_roots(os.environ.get("HARVESTER_LOCAL_ROOTS", ""))


def set_local_roots(*roots: Path) -> None:
    """Confine local reads to `roots` (resolved). No args = unconfined. Called by `remote.build_app`
    so the HTTP deployment is confined without depending on env-var ordering at import time."""
    global LOCAL_ROOTS
    LOCAL_ROOTS = tuple(r.expanduser().resolve() for r in roots)
    log.info("local read confinement: %s", [str(r) for r in LOCAL_ROOTS] or "unconfined")


def deny_reason(path: Path) -> str | None:
    """Return a refusal reason if reading `path` would expose a secret/system file, else None.

    Confinement-minded, not a bare denylist: the path is canonicalized first (expanduser +
    realpath, resolving symlinks) so a symlink with an innocent name can't smuggle a secret. It is
    then refused if it resolves (a) into a system root (/proc, /sys, /dev, /etc), (b) inside a
    sensitive directory (~/.ssh, ~/.aws, ~/.gnupg, ~/.config, ~/.kube, …), or (c) onto a known
    credential/secret file name or suffix. Ordinary documents pass through untouched.

    When `LOCAL_ROOTS` is non-empty the path must ALSO resolve inside one of those roots; the
    denylist still applies within them.
    """
    try:
        rp = path.expanduser().resolve()
    except (OSError, RuntimeError) as e:  # symlink loop / unresolvable → refuse, don't read
        log.warning("deny_reason could not resolve %s: %s — refusing", path, e)
        return "refusing to read an unresolvable path"
    # (0) confinement — checked on the RESOLVED path, so a symlink pointing OUT of a root is caught
    # (the cache dir is writable by the fetch path; a planted symlink must not become an escape).
    if LOCAL_ROOTS and not any(rp == root or root in rp.parents for root in LOCAL_ROOTS):
        return ("refusing to read outside this server's permitted directory — "
                "it serves only its own cache; pass a URL or an identifier instead")
    # (a) system roots — checked on the RESOLVED path so a symlink INTO /proc or /etc is caught.
    rp_str = str(rp)
    for root in _SYSTEM_ROOTS:
        if rp_str == root or rp_str.startswith(root + os.sep):
            return f"refusing to read inside a system directory ({root})"
    # (b) sensitive directories anywhere in the resolved path
    hit = {p.lower() for p in rp.parts} & _DENY_DIR_PARTS
    if hit:
        return f"refusing to read inside a sensitive directory ({sorted(hit)[0]})"
    # (c) credential/secret file names + suffixes
    name = rp.name
    low = name.lower()
    if low in _DENY_NAMES:
        return f"refusing to read a sensitive file ({name})"
    if low.endswith(_DENY_SUFFIXES):
        return f"refusing to read a credential/secret file ({name})"
    if low == ".env" or low.startswith(".env."):
        return f"refusing to read an environment/secret file ({name})"
    if any(k in low for k in _DENY_KEY_MARKERS):
        return f"refusing to read a private key ({name})"
    return None


# ── content-type → kind mapping for binary/non-HTML responses ────────────────
_CT_BINARY_MAP: dict[str, str] = {
    "application/pdf": "pdf",
    "application/vnd.openxmlformats-officedocument.wordprocessingml.document": "docx",
    "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": "xlsx",
    "application/vnd.openxmlformats-officedocument.presentationml.presentation": "pptx",
    "application/zip": "zip",
    "application/x-zip-compressed": "zip",
    "application/x-zip": "zip",
    "application/x-7z-compressed": "7z",
    "application/x-rar-compressed": "rar",
    "application/vnd.rar": "rar",
    "application/x-tar": "tar",
    "application/gzip": "tar",
    "application/x-gzip": "tar",
    "application/x-bzip2": "tar",
    "application/x-xz": "tar",
}
_HTML_LIKE_CT = frozenset({
    "text/html", "text/plain", "application/xhtml+xml",
    "application/xml", "text/xml", "text/markdown",
    # NOTE: empty string is intentionally excluded — an absent Content-Type falls through
    # to magic-byte sniffing rather than being assumed as HTML.
})


# ── plain-text vs. HTML routing (for the text bodies _sniff_kind passes through) ──
# Extensions and Content-Types whose bodies are RAW text — never run them through trafilatura
# (it assumes markup and strips prose to nothing). HTML CT/extensions are the explicit opposite.
_PLAIN_TEXT_EXTS = {".txt", ".text", ".md", ".markdown", ".rst", ".log", ".tex", ".org"}
_PLAIN_TEXT_CTS = {"text/plain", "text/markdown", "text/x-markdown", "text/x-rst"}
_HTML_CTS = {"text/html", "application/xhtml+xml", "application/xml", "text/xml"}
# Structural tags that mark a body as real HTML — each needs a literal '<', so plain prose
# (no angle-bracket tags) never trips them; used only when CT/extension give no verdict.
_HTML_MARKERS = ("<html", "<!doctype html", "<head", "<body", "<div", "<table",
                 "<article", "<section", "<span", "<p>", "<p ")


def is_plain_text(name: str, content_type: str = "", sample: str = "") -> bool:
    """True when a text body should be kept VERBATIM rather than run through trafilatura.

    trafilatura expects HTML markup; handed raw prose (a Gutenberg .txt, an OCR dump, a source
    file, a README) it extracts nothing, so the full document is lost and `size_only` reports 0/0.
    Decision order: an HTML/XML Content-Type or .htm(l) extension forces extraction; a text/plain|
    markdown Content-Type or a plain-text extension keeps it verbatim UNLESS the body is obviously
    mis-served HTML; with no decisive signal, keep it verbatim only when it has no HTML structure.
    """
    ct_base = (content_type or "").split(";")[0].strip().lower()
    low = (sample or "")[:4096].lower()
    looks_html = any(m in low for m in _HTML_MARKERS)
    if ct_base in _HTML_CTS:
        return False
    ext = os.path.splitext(name.split("?", 1)[0].split("#", 1)[0].lower().rstrip("/"))[1]
    if ext in (".htm", ".html"):
        return False
    if ct_base in _PLAIN_TEXT_CTS or ext in _PLAIN_TEXT_EXTS:
        return not looks_html  # honor the plain-text signal unless the body is clearly HTML
    return bool((sample or "").strip()) and not looks_html


def _sniff_kind(content_type: str, head: bytes) -> str | None:
    """Return the true non-HTML kind (e.g. 'pdf', 'zip', 'image') from Content-Type +
    magic bytes, or None when the response is genuinely HTML/text.

    Priority: explicit CT > openxmlformats sub-type > image/* > HTML CT (returns None) >
    magic bytes fallback when CT is absent/unknown.
    """
    ct_base = content_type.split(";")[0].strip().lower()
    if ct_base in _CT_BINARY_MAP:
        return _CT_BINARY_MAP[ct_base]
    if "openxmlformats-officedocument" in ct_base:
        if "wordprocessingml" in ct_base:
            return "docx"
        if "spreadsheetml" in ct_base:
            return "xlsx"
        if "presentationml" in ct_base:
            return "pptx"
        return "zip"
    if ct_base.startswith("image/"):
        return "image"
    if ct_base in ("application/json", "text/json", "application/ld+json"):
        return "json"
    if ct_base in ("text/csv", "application/csv"):
        return "csv"
    if ct_base in _HTML_LIKE_CT:
        return None  # unambiguously HTML/text — trust it, don't sniff further
    # Unknown or absent CT: fall back to magic bytes
    return sniff_magic(head)
