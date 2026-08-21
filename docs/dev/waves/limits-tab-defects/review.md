# Limits tab defects — inline review

## Review input

- Manifest: `docs/dev/waves/limits-tab-defects/manifest.md`
- Project: `pfm`
- Code commit: `630fab1659b11fcfdd51566de1a2820bf35fc447`
- Review mode: single-context inline walk, as ordered by the user

## Coverage

The review followed each changed writer to its visible Limits-tab consumer:

- numeric account discovery → materialized config accounts/skips → command wiring → limits sampler → UI row;
- usage API window keys → canonical descriptors → stats and statusline renderers;
- Codex cache path → dedicated sampler source → visible usage/error row;
- zero-config action tests → explicit config-owned roster fixtures.

The CONFIG-OWNERSHIP, HONEST-ABSENCE, and LEAK-LINE registry entries were checked against the diff. No guarded file or invariant registry change is required: the existing CONFIG-OWNERSHIP law already names the fixed hardcoded-count defect class.

## Remediation completed during review

- Every numeric `.cc` entry now receives a named validity verdict, including broken or non-directory entries.
- A persistent 401/403 becomes a named credential skip whether the one-shot refresh succeeds or fails; non-auth fetch failures remain visible account errors.
- Account directory construction and display labels were moved into `internal/config`; downstream packages no longer reconstruct `.cc/N` policy.
- Statusline window decoding ignores non-window metadata without losing valid unknown windows.
- Limit names and status text are control-character sanitized before terminal rendering.
- Codex success, Codex cache failure, non-auth account failure, and statusline unknown-window behavior gained direct regressions.

## Verification

- RED observed through `./.claude/scripts/dev.sh iso test pfm` before implementation: new config, paths, usage-window, sampler, command-wiring, and UI contracts failed.
- Final full suite: `./.claude/scripts/dev.sh iso test pfm` — PASS, fence proof `container=e62f0311d6af HOME=/root work=/worktree`.
- Final verifier: `./.claude/scripts/dev.sh iso verify pfm` — PASS, fence proof `container=106ed6f6fc54 HOME=/root work=/worktree`.
- `git diff --check` — PASS.

## Verdict

PASS — no open code action items and no user-owned deferrals. Publication remains outside this wave; no push was requested.
