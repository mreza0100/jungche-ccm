# reap/busy: a stat error is not "not configured"

Status: QUEUED · Refined: 2026-08-21 by CCC (walker finding, W3 close — pre-existing, HIGH, HONEST-ABSENCE) · Project: pfm · Fenced wave

## Why (verified by the W3 walker, 2026-08-21)

- `pfm/internal/reap/busy.go:59-63`: every `os.Stat` error that is NOT `ErrNotExist` — permission denied, a broken mount, an I/O error — folds into the same silent "not configured" bucket as a genuinely absent directory. A reaper that cannot look reports "nothing configured", which is the coincidence-detector shape root `CLAUDE.md` forbids.

## Fix (one task)

- Split the branch: `errors.Is(err, fs.ErrNotExist)` → the existing not-configured path; any other error → a named error carrying the path and the `os.Stat` message, surfaced by the reaper's caller the same way its other probe failures surface (never a log line alone).
- RED-first (JAIL): a directory with mode `000` (or an unreadable parent) must produce the named error, watched failing against HEAD where it reads as not-configured.

## Acceptance

- `dev.sh iso test pfm` + `iso verify pfm` green with fence proof; walker with HONEST-ABSENCE armed.
