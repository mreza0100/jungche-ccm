"""Fetch-outcome scoreboard: an append-only JSONL of every dispatch result.

Every `get_or_fetch` terminal result appends one line to `<cache_root>/stats.jsonl`:
``{"ts": ..., "item": ..., "ok": true|false, "detail": "method-or-error_kind"}``. Aggregated,
this answers the questions the per-artifact `rungs:` frontmatter cannot: WHICH sources win,
which failure kinds dominate, and what a source's real success rate is over time — so a dying
OA source or a newly walled publisher is VISIBLE instead of silently rotting.

Nothing here raises or blocks the fetch path: a failed append is logged and dropped. Run
`python -m harvester.stats` for the human-readable summary table.
"""
from __future__ import annotations

import json
from datetime import datetime, timezone
from pathlib import Path

from .cache import cache_root
from .log import get_logger

log = get_logger("stats")

STATS_FILENAME = "stats.jsonl"
_MAX_ITEM_LEN = 500
_MAX_DETAIL_LEN = 200


def stats_path() -> Path:
    """The JSONL file lives INSIDE the cache root, so it follows WEBFETCH_DIR/HARVESTER_CACHE_DIR."""
    return cache_root() / STATS_FILENAME


def record_fetch(item: str, ok: bool, detail: str = "") -> None:
    """Append one outcome line. Never raises — telemetry must never break a fetch."""
    entry = {
        "ts": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "item": (item or "")[:_MAX_ITEM_LEN],
        "ok": bool(ok),
        "detail": (detail or "")[:_MAX_DETAIL_LEN],
    }
    try:
        with open(stats_path(), "a", encoding="utf-8") as fh:
            fh.write(json.dumps(entry, ensure_ascii=False) + "\n")
    except OSError as e:
        log.warning("stats append failed: %s", e)


def summarize(last_n: int = 5000) -> dict[str, dict[str, float | int]]:
    """Aggregate the last *last_n* records by `detail` → {total, ok, rate}.

    Reads at most the tail of the file (a scoreboard that loads forever would defeat itself).
    Returns {} when nothing has been recorded yet — an EMPTY result means no data was ever
    written, which is distinguishable from a broken reader by the caller checking that
    `stats_path()` exists.
    """
    path = stats_path()
    try:
        size = path.stat().st_size
    except OSError as e:
        log.warning("stats summarize could not stat %s: %s", path, e)
        return {}
    try:
        with open(path, "rb") as fh:
            if size > last_n * 512:  # seek near the end; line boundaries re-split below
                fh.seek(max(0, size - last_n * 512))
                fh.readline()  # drop the (possibly partial) first line
            raw = fh.read()
    except OSError as e:
        log.warning("stats summarize could not read %s: %s", path, e)
        return {}
    buckets: dict[str, dict[str, float | int]] = {}
    seen = 0
    for line in raw.decode("utf-8", errors="ignore").splitlines():
        line = line.strip()
        if not line:
            continue
        try:
            rec = json.loads(line)
            ok = bool(rec.get("ok"))
            detail = str(rec.get("detail") or ("ok" if ok else "unknown"))
        except (json.JSONDecodeError, AttributeError) as e:
            log.debug("stats skipping malformed line %r: %s", line[:80], e)
            continue
        seen += 1
        b = buckets.setdefault(detail, {"total": 0, "ok": 0})
        b["total"] = int(b["total"]) + 1
        if ok:
            b["ok"] = int(b["ok"]) + 1
    for b in buckets.values():
        total = int(b["total"])
        b["rate"] = round(int(b["ok"]) / total, 3) if total else 0.0
    log.info("stats summarized %d record(s) -> %d detail bucket(s)", seen, len(buckets))
    return buckets


def main() -> None:  # pragma: no cover — thin CLI wrapper over summarize()
    buckets = summarize()
    path = stats_path()
    if not buckets:
        if not path.exists():
            print(f"No stats recorded yet ({path} does not exist). Fetch something first.")
        else:
            print(f"Stats file {path} exists but holds no parseable records.")
        raise SystemExit(1)
    width = max(len(k) for k in buckets)
    print(f"{'detail'.ljust(width)}  {'ok':>6}  {'total':>6}  {'rate':>6}")
    for k in sorted(buckets, key=lambda k: -int(buckets[k]['total'])):
        b = buckets[k]
        print(f"{k.ljust(width)}  {b['ok']:>6}  {b['total']:>6}  {b['rate']:>6.3f}")


if __name__ == "__main__":
    main()
