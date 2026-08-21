# harvestpy: provisioning fails on every real host at the publish rename

Status: QUEUED · Refined: 2026-08-21 by CCC (reproduced on the host: `pfm install --yes` → `harvestpy post-publish smoke: … fork/exec …/<digest>/project/.venv/bin/python: no such file or directory`) · Project: pfm · Fenced wave

## Why (verified)

- Design of record: the harvester migration is Go shell (`pfm/internal/harvest`, legacy byte-parity tests) + ONE pinned, digest-verified Python converter worker (`pfm/internal/harvestpy`, `converter.go:43` "no Go fallback converter"). The worker's environment is built by `harvestpy/provision.go`.
- `provision.go:223-228`: `uv sync --frozen --no-install-project --project <staging>/project --python <staging>/python/bin/python3.x`. uv creates `.venv/bin/python` as an ABSOLUTE symlink to that interpreter path and writes `home = <staging>/python/bin` into `.venv/pyvenv.cfg`.
- `provision.go:229-260`: stat, `uv pip check`, inventory, and the first `Smoke` all run against the STAGING path — they pass.
- `provision.go:272`: `os.Rename(staging, final)` publishes the tree to `<envRoot>/<digest>`. The symlink and `pyvenv.cfg` still point at the vanished staging path.
- `provision.go:285-296`: the post-publish `Smoke` runs `final/project/.venv/bin/python` → ENOENT → `restoreOld()` removes the tree → `pfm install` exits 1; `pfm doctor` then reports interpreter/marker/lock/inventory/live_smoke broken. The comment at `:285` names the exact hazard and treats it as a check instead of preventing it.
- Host evidence 2026-08-21: `~/.local/state/pfm/harvest-python/env/linux-amd64/` empty, `cache/` 55 MB (uv + Python tarballs only), install log line: `pfm install: harvestpy provision linux-amd64: harvestpy post-publish smoke: harvestpy smoke subprocess: start harvestpy worker: fork/exec /…/env/linux-amd64/fee1a18a…/project/.venv/bin/python: no such file or directory`.

## Fix (one task)

### #1 — build in place; never move a venv

- Stage AT the final path: `staging := filepath.Join(envRoot, desired)` with an `INCOMPLETE` marker file written first (`writePrivate(filepath.Join(staging, "INCOMPLETE"), …)`). All uv/python paths are then final paths from the first byte, so uv's absolute links are correct forever. The repair/quarantine logic (`:262-271`) moves an EXISTING invalid `final` aside BEFORE staging begins (same backup naming), and `restoreOld()` keeps its meaning. Success = remove `INCOMPLETE`, write `environment.json`, `atomicCurrent`. Failure = `os.RemoveAll(staging)` (+ restore backup) — exactly today's cleanup semantics.
- `Inspect`/`Check` (`check.go`) treat a present `INCOMPLETE` marker as state `incomplete` with a named reason ("provisioning did not finish"), never as `ready` and never as absence.
- Keep BOTH smokes (pre-publish and final) — they now run on the same path; the second one is cheap and remains the judge.
- Delete the `:285-287` comment's hazard narrative; replace with one line stating the invariant: the environment is never renamed after `uv sync`.
- Do NOT switch to `uv venv --relocatable` or rewrite symlinks by hand — a moved venv is the defect class, not a thing to patch.

### Tests (RED first, JAIL — no network)

- `harvestpy_test.go`: a fake `Run` that, on `sync`, creates `.venv/bin/python` as an ABSOLUTE symlink to `<project root>/python/bin/python3` (mirroring uv) and a `pyvenv.cfg` with `home = <that dir>`; a fake converter smoke that `os.Stat`s the resolved symlink target. Against HEAD the provision fails at "post-publish smoke" — WATCH IT FAIL — and passes after the fix with the link resolving inside `<envRoot>/<digest>`.
- `INCOMPLETE` marker: provisioning interrupted after `uv sync` (fake Run returns an error on `pip check`) leaves no `<digest>` dir and no `current`; `Check` on a hand-planted `INCOMPLETE` dir reports `incomplete` with the reason string.
- Existing `provision` tests keep passing; `doctor_harvest_test.go` unchanged.

## Acceptance

- `dev.sh iso test pfm` + `iso verify pfm` green, fence proof quoted.
- Host proof (user-run after the mirror build): `pfm install --yes` completes with `harvestpy … state=ready`; `pfm doctor` shows interpreter/marker/lock/inventory/live_smoke ok and `warnings` drops by 5; `pfm harvest <a local .pdf>` converts.
- Walker with HONEST-ABSENCE armed (the `INCOMPLETE` state is a new named absence/error boundary).

## Out of scope

- Package plan size (3.1 GB cold download) and any converter feature work.
- Enabling the `harvester` MCP server in the user's config (user-owned toggle).
