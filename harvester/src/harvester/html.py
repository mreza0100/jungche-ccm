"""HTML -> Markdown extraction, metadata recovery, and a conservative tidy pass.

trafilatura does the heavy lifting; everything here is pure (no network).
"""

import trafilatura
from trafilatura.metadata import extract_metadata

from .log import get_logger

log = get_logger("html")


def extract_content_from_html(html: str) -> str:
    """Extract the main content of an HTML page as Markdown via trafilatura.

    include_images=True emits ``![]()`` links for article images. `fetch` leaves these as
    remote URLs — it never downloads image binaries; the model views one on demand via the
    `fetchImage` tool.
    """
    return trafilatura.extract(
        html, output_format="markdown", favor_recall=True,
        include_links=True, include_images=True,
    ) or ""


# UI-chrome values trafilatura occasionally mistakes for an author (seen: PubMed abstract
# pages yield "Username"). Compared against the WHOLE author value only (case-insensitive),
# never as a substring — so a real byline like "Loginov, Ivan" or any author list is never cut.
_JUNK_AUTHOR_VALUES = frozenset({
    "username", "user name", "user", "login", "log in", "sign in", "signin",
    "sign up", "signup", "register", "log out", "logout", "admin",
    "administrator", "guest", "search", "menu", "subscribe", "newsletter",
})


def extract_metadata_block(html: str) -> str:
    """Recover page metadata (title, author, date, sitename, license) and render it as a
    compact markdown block to PREPEND to the body. Returns "" when nothing is found."""
    try:
        meta = extract_metadata(html)
    except Exception as e:
        log.warning("extract_metadata failed: %s", e)
        return ""
    if meta is None:
        return ""

    def clean(value) -> str:
        if not value:
            return ""
        if isinstance(value, (list, tuple)):
            value = ", ".join(str(x) for x in value if x)
        return " ".join(str(value).split()).strip()

    author = clean(getattr(meta, "author", ""))
    if author.lower() in _JUNK_AUTHOR_VALUES:
        author = ""  # whole-value junk only (e.g. "Username"); real bylines pass through

    fields = [
        ("Title", clean(getattr(meta, "title", ""))),
        ("Authors", author),
        ("Published", clean(getattr(meta, "date", ""))),
        ("Source", clean(getattr(meta, "sitename", ""))),
        ("License", clean(getattr(meta, "license", ""))),
    ]
    lines = [f"**{label}:** {value}" for label, value in fields if value]
    if not lines:
        return ""
    return "\n".join(lines) + "\n\n---\n\n"


def tidy_markdown(md: str) -> str:
    """Conservative, content-safe tidy — vertical/trailing whitespace ONLY. Idempotent."""
    lines = [line.rstrip() for line in md.splitlines()]
    out: list[str] = []
    i, n = 0, len(lines)
    while i < n:
        if lines[i] == "":
            j = i
            while j < n and lines[j] == "":
                j += 1
            run = j - i
            out.extend([""] if run >= 3 else [""] * run)
            i = j
        else:
            out.append(lines[i])
            i += 1
    while out and out[0] == "":
        out.pop(0)
    while out and out[-1] == "":
        out.pop()
    return "\n".join(out) + "\n" if out else ""
