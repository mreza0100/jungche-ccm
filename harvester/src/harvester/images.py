"""Article-image localisation: download every image a page references into the cache and
rewrite the markdown `![](remote)` links to local paths so the caller can read them with vision.
"""

import asyncio
import re
from urllib.parse import urljoin, urlsplit

from . import cache, detect, net
from .log import get_logger

log = get_logger("images")

# HTML image localisation guards.
IMG_DL_CAP = 50
IMG_MAX_BYTES = 10 * 1024 * 1024

_IMG_MD_RE = re.compile(r"!\[[^\]]*\]\(\s*([^)\s]+)")
_ICON_NAME_RE = re.compile(r"favicon|sprite", re.IGNORECASE)


def _is_icon_asset(url: str) -> bool:
    """True for a favicon / .ico / sprite asset — not real article content, skip the download."""
    base = urlsplit(url).path.rsplit("/", 1)[-1].lower()
    return base.endswith(".ico") or bool(_ICON_NAME_RE.search(base))


async def localize_html_images(
    md: str, base_url: str, user_agent: str, proxy_url: str | None = None
) -> str:
    """Download every image a page references into `<cache>/<ext>/` and rewrite the markdown
    `![](remote)` links to absolute LOCAL paths so the caller can read them with vision.

    Skips data: URIs; caps at IMG_DL_CAP images and IMG_MAX_BYTES each; on any failed
    download the original remote link is left untouched.
    """
    urls: list[str] = []
    seen: set[str] = set()
    for m in _IMG_MD_RE.finditer(md):
        u = m.group(1)
        if u.startswith("data:") or u in seen:
            continue
        seen.add(u)
        if _is_icon_asset(u):
            log.info("image skipped (icon/favicon): %s", u)
            continue
        urls.append(u)
        if len(urls) >= IMG_DL_CAP:
            break
    if not urls:
        return md

    mapping: dict[str, str] = {}
    failed = 0
    sem = asyncio.Semaphore(6)

    async with net._client(proxy_url) as client:
        async def one(u: str) -> None:
            nonlocal failed
            full = urljoin(base_url, u)
            try:
                async with sem:
                    r = await client.get(
                        full, follow_redirects=True,
                        headers={"User-Agent": user_agent}, timeout=30,
                    )
            except net.FetchNotAllowed as e:
                failed += 1
                log.warning("image refused (ssrf) %s: %s", full, e)
                return
            except Exception as e:
                failed += 1
                log.warning("image download failed %s: %s", full, e)
                return
            if r.status_code >= 400:
                failed += 1
                log.warning("image %s -> HTTP %d", full, r.status_code)
                return
            data = r.content
            if not data or len(data) > IMG_MAX_BYTES:
                failed += 1
                log.warning("image %s skipped (size=%d)", full, len(data))
                return
            ct = r.headers.get("content-type", "").lower()
            if not (ct.startswith("image/") or detect.sniff_magic(data[:16]) == "image"):
                failed += 1
                log.warning("image %s skipped (not an image, ct=%s)", full, ct)
                return
            ext = detect.image_ext(full, default=net._ext_from_content_type(ct))
            path = cache.cache_file(full, ext.lstrip("."), ext)
            try:
                path.write_bytes(data)
            except OSError as e:
                failed += 1
                log.warning("image cache write failed %s: %s", path, e)
                return
            mapping[u] = str(path)

        await asyncio.gather(*(one(u) for u in urls))

    log.info("images %s downloaded=%d failed/skipped=%d", base_url, len(mapping), failed)
    if not mapping:
        return md

    def repl(m: "re.Match") -> str:
        u = m.group(1)
        return m.group(0).replace(u, mapping[u], 1) if u in mapping else m.group(0)

    return _IMG_MD_RE.sub(repl, md)
