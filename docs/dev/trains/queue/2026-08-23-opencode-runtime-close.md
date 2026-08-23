# Close-out: `train/opencode-runtime` → merge to `main`

Status: QUEUED · Refined: 2026-08-23 by Professor, from two reviews of the branch (`tmp/review-opencode-runtime-2026-08-22.md`, G1/C1/C2/C3) and a re-inspection of the worktree's uncommitted fixes at 10:40 · Project: pfm + blueprint gates · Fenced wave, closing · Executor: the OpenCode session that built the wave, in `.worktrees/opencode-runtime/`.

Written to be executed without judgment calls. Every command is given; every expected output is given; when reality differs, § 6 says what to do. You build and verify; **you never commit** — the git writes are handed to a gitter session at two HANDOFF points, in the exact words provided.

---

## 0. Before you touch anything

### 0.1 Where you are
- `cd .worktrees/opencode-runtime` — every path below is relative to this directory. `git branch --show-current` must print `train/opencode-runtime`. If it prints anything else, stop: `BLOCKED — wrong branch: <output>`.
- Dirty set expected (verified 10:40; if yours differs, continue — but quote `git status --short` in the report):
  ```
   M .claude/commands/pcm.md            M .opencode/command/pcm.md        M docs/commands/pcm/references/audit-scopes.md
   D .claude/scripts/build-codex.mjs    M .opencode/opencode.jsonc        D harvester/AGENTS.md
   M .claude/scripts/build-opencode.mjs M AGENTS.md                       M pfm/internal/store/store.go
   M .claude/scripts/dev.sh             M CLAUDE.md                       M scripts/check-codex-markers.mjs
   M .codex/config.toml
  ```
- The branch is **5 commits behind `main`** (`git rev-list --count HEAD..main`). One of those five is the host-escape jail guard: after the rebase in HANDOFF 1, any test that reaches the operator's real home FAILS with `refusing to resolve the operator's real home directory inside a test`. That is a test bug to fix (§ 3.2), never a flake to retry.

### 0.2 What you may never do
- No `git commit`, `git push`, `git tag`, `git merge`, `git rebase`, `git stash`, `git reset`, `git add`, `git rm`, `git checkout`. The harness denies most of these; the repo law denies all of them to you. Every git write below is inside a HANDOFF block for a gitter session.
- No edit under `.claude/**`, `.opencode/**`, any `CLAUDE.md`, any `AGENTS.md` — the harness denies the edit tool there. **Running a compiler that regenerates a mirror (`node .claude/scripts/build-opencode.mjs generate`) is sanctioned; editing the output by hand is not.**
- No `go test` / `go build` / `go vet` on the host. Go runs only inside the fence (§ 0.3).
- Never write `/home/<anything>` into a tracked file.
- Never touch the installed `pfm` (`~/.local/bin/pfm`) or the real `~/.local/state/pfm/fleet.db`. The host mirror build is HANDOFF 2's, not yours.

### 0.3 Commands you will use
```
./.claude/scripts/dev.sh iso test pfm       # Go suite inside the container (~10 min); ends "all steps passed."
./.claude/scripts/dev.sh iso verify pfm     # vet + gofmt + build inside the container
./.claude/scripts/dev.sh verify blueprint   # host: codex marker gate + opencode mirror check/doctor (node only, no Go)
./scripts/leak-check.sh                     # host: the publication gate; exit 0 = clean
node scripts/check-codex-markers.mjs        # host: the marker gate alone, for the RED/GREEN proof
```
A green fence run prints one line `fence: container=<id> HOME=/root work=/worktree`. **Quote it** in every report that claims a fence result; a report without it is a host run.

---

## 1. What is already done on this branch (do not redo)

| Review item | State in the working tree (10:40) | Verdict |
|---|---|---|
| G1 — `harvester/AGENTS.md` resurrected | `D harvester/AGENTS.md` (deleted on disk, still in the index until HANDOFF 1 commits it) | done, pending commit |
| C1 — marker gate counts the fossil healthy | `scripts/check-codex-markers.mjs` now reconciles every tracked marked file against its declared sources and reports `ORPHAN <file> — none of its declared sources exist`; a failed `git ls-files` is its own FAIL line | done, **but never watched failing** (§ 3.1) and one silent skip remains (§ 2) |
| C2 — two live compilers | `.claude/scripts/build-codex.mjs` deleted; `CLAUDE.md`/`AGENTS.md` say `pfm codex build` is the single writer; `.claude/scripts/codex-sync.sh` calls it | done except one dangling pointer (§ 4, HANDOFF 0) |
| C3 — schema 7 one-way door | `store.go` `migrate` now refuses a newer DB with the fix and the `sqlite3 <db> ".backup …"` recovery in the error text | done (refuse-and-explain chosen) |

---

## 2. Task A — C1b: the gate's one remaining silent skip (≈ 10 min)

`scripts/check-codex-markers.mjs`, the tracked-file loop (today ~lines 86–92):
```js
  let content;
  try {
    content = readFileSync(file, 'utf8');
  } catch {
    continue; // unreadable tracked file — not this gate's subject
  }
```
This is why the gate passes on your tree right now: `harvester/AGENTS.md` is tracked, deleted on disk, `readFileSync` throws `ENOENT`, and the fossil is skipped without a word. Replace with exactly:
```js
  let content;
  try {
    content = readFileSync(file, 'utf8');
  } catch (e) {
    if (e.code === 'ENOENT') {
      // Tracked but gone from disk: a deletion awaiting its commit. Named,
      // counted, not failed — the commit that records it is the fix.
      deletedTracked.push(file);
      continue;
    }
    fail(`${file} — tracked but unreadable (${e.code}); this gate could not look`);
    continue;
  }
```
Add `const deletedTracked = [];` beside `let orphans = 0;`. Extend BOTH summary lines (the `FAILED` one and the `OK` one) with `; ${deletedTracked.length} tracked-but-deleted` and, when `deletedTracked.length > 0`, print one line before the summary: `console.error(`WARN tracked-but-deleted (awaiting commit): ${deletedTracked.join(', ')}`)`. Update the header comment's "what this reports when broken" sentence to name all three states: ORPHAN (fail), tracked-but-unreadable (fail), tracked-but-deleted (warn, counted).

**Proof, in this order (no git writes — file restores only):**
1. Current tree (two tracked files deleted on disk — the fossil AND the retired compiler): `node scripts/check-codex-markers.mjs` → must print `WARN tracked-but-deleted (awaiting commit): .claude/scripts/build-codex.mjs, harvester/AGENTS.md` and end `… 2 tracked-but-deleted`, exit 0. Before your edit it printed nothing about either — quote both runs. (The gate does not read a deleted file's HEAD content to decide whether it was marked: a file it cannot open is named as such, whatever it was.)
2. `git show HEAD:harvester/AGENTS.md > harvester/AGENTS.md` (restores the fossil to disk; `git show` is read-only). Run the gate → must print `FAIL ORPHAN harvester/AGENTS.md — none of its declared sources exist (harvester/CLAUDE.md); delete this fossil`, exit 1. **This is C1's RED: the gate catching G1 by name.** Quote it.
3. `rm harvester/AGENTS.md`. Run the gate → back to step 1's output (`2 tracked-but-deleted`), exit 0. Quote it.
4. `git status --short harvester/` must print exactly ` D harvester/AGENTS.md` — the same as before you started.

---

## 3. Task B — the fence, twice (≈ 15 min each run)

### 3.1 First run, on your current (unrebased) tree
```
./.claude/scripts/dev.sh iso verify pfm && ./.claude/scripts/dev.sh iso test pfm
./.claude/scripts/dev.sh verify blueprint
./scripts/leak-check.sh; echo "leak-check exit=$?"
```
Expected: both `iso` runs end `all steps passed.` with the fence line; `verify blueprint` ends `all steps passed.`; leak-check `exit=0`. Quote the fence line, the package tally (`grep -c '^ok' …` is fine), and the last line of each. Then stop at **HANDOFF 1**.

### 3.2 Second run, after HANDOFF 1 rebased you onto `main`
Same three commands. The jail guard is now in the tree. If any test prints `refusing to resolve the operator's real home directory inside a test`:
- that package lacks a jail. Fix it the way `pfm/internal/store/testmain_test.go` on `main` does: a `TestMain` calling `testjail.Run(m)`, or — if the package is one `testjail` itself imports — a self-contained `TestMain` that sets `paths.EnvHome` to a temp dir (copy `pfm/internal/deps/testmain_test.go`). Never set `PFM_TEST_REAL_HOME`.
- re-run; report the package and the fix. This is test code, in scope.
Then stop at **HANDOFF 2**.

---

## 4. HANDOFF 0 — Claude-side, not yours (the dangling pointer)

`.claude/commands/pcm.md:228` reads `… → \`build-codex.mjs generate\` (it compiles a \`.codex/agents/{name}.toml\`) → …`. The script is deleted on this branch; the live compiler is `pfm codex build`. The harness denies you this edit. **Report this line verbatim and continue** — a Claude session with `/pcm` makes the one-line change (`build-codex.mjs generate` → `pfm codex build`), its Stop hook recompiles the Codex mirror, and you then run `node .claude/scripts/build-opencode.mjs generate && node .claude/scripts/build-opencode.mjs doctor` so `.opencode/command/pcm.md` follows. If the Claude session has not done it by HANDOFF 1, HANDOFF 1 still proceeds; the pointer rides into the squash commit's "known debt" line and is fixed on `main` by `/pcm` afterwards.

---

## 5. The HANDOFF blocks (print each verbatim when you reach it, then WAIT)

### HANDOFF 1 — squash + rebase (gitter)
```
HANDOFF 1 — gitter: squash and rebase train/opencode-runtime
worktree: .worktrees/opencode-runtime   branch: train/opencode-runtime   base now: <git rev-parse --short main>
fence (unrebased): <the fence line>   iso test: <last line>   verify blueprint: <last line>   leak-check: exit=<n>
marker gate RED proof: <the FAIL ORPHAN line from § 2 step 2>
git status --short:
<paste>

gitter instructions:
1. In the worktree: `git log --oneline -2` must show `386e927 wip(opencode-runtime): …` on top of `1fba855`. If not, stop and report.
2. `git reset --soft 1fba855` — folds the wip checkpoint back into the index so the wave lands as ONE real commit. Then stage the pasted status with explicit pathspecs (`git add -- <each path>`; deletions via `git rm --cached -- <path>` or `git add -u -- <path>`), verify `git diff --cached --name-only` equals the pasted set plus the wip checkpoint's own files (`git show --stat --format= 386e927`), then one commit:
   feat(pfm): OpenCode as the third engine — oc_sessions mirror, schema 7, single Codex writer

   - index/opencode.go reads OpenCode's live SQLite store read-only in one WAL snapshot into oc_sessions (schema 7, additive); a newer DB is refused with the fix and the .backup recovery in the error
   - pfm codex build is the single writer of the Codex mirror; the repo-local build-codex.mjs is deleted (the blueprint's shipped copy stays for adopters)
   - scripts/check-codex-markers.mjs reconciles every tracked marked file against its declared sources: ORPHAN fails by name, tracked-but-unreadable fails, tracked-but-deleted warns and counts; watched failing on the resurrected harvester/AGENTS.md before that fossil was removed
   - harvester/AGENTS.md removed (its project was retired in 114b4dd; a modify/delete conflict had kept it)
   - known debt: .claude/commands/pcm.md still names `build-codex.mjs generate` once (line ~228) — /pcm fixes it on main

   Co-Authored-By: <the executing session's trailer>
3. `git rebase main` (no -i). On a conflict: stop and report the file list — do not resolve by keep-ours; the orchestrator decides per file.
4. Report: new HEAD hash, `git log --oneline main..HEAD` (must be exactly ONE commit), `git status --short` (must be empty).
```

### HANDOFF 2 — merge + host mirror build (gitter, then the orchestrator)
```
HANDOFF 2 — gitter: merge train/opencode-runtime into main
fence (rebased): <fence line>   iso test: <last line>   iso verify: <last line>   verify blueprint: <last line>   leak-check: exit=<n>
jails added in 3.2: <package list or "none">

gitter instructions:
1. In the LIVE checkout (repo root, branch main): `git merge --no-ff train/opencode-runtime -m "merge(train/opencode-runtime): OpenCode as the third engine — oc_sessions mirror, schema 7 refuse-and-explain, single Codex writer"`. The live checkout holds other sessions' uncommitted edits to CLAUDE.md/README.md/AGENTS.md/.claude/** — if git refuses the merge because of them, STOP and report; never stash.
2. Report the merge hash and `git log --oneline -3`.
3. Do NOT delete the worktree or the branch yet — the orchestrator verifies first.

orchestrator (a Claude session, on the user's word) — the host mirror build:
   cd pfm && go build -o ~/.local/bin/pfm ./cmd/pfm && pfm install --yes && pfm doctor
   — the installed pfm is the running daemon's binary; stop `pfm mcp serve` first if `cp` reports "Text file busy".
   — `pfm install --yes` regenerates .professor/manifest.json, which still lists the deleted build-codex.mjs hash.
   — schema: the first run of the new binary migrates the real fleet.db to 7. Take the backup FIRST:
     sqlite3 ~/.local/state/pfm/fleet.db ".backup ~/.local/state/pfm/fleet.db.bak-before-v7"
   Then: `pfm ls` shows the same chats as before; `pfm doctor` is green; an OpenCode chat row appears if one exists.
```

---

## 6. Stop and report instead of guessing

Print `BLOCKED — <reason>` with the command and its full output, then wait, when:
- § 0.1 branch or dirty-set check fails in a way § 0.1 does not cover;
- the § 2 RED proof does not print `FAIL ORPHAN harvester/AGENTS.md` (then the reconcile is not doing what the table in § 1 says — say what it printed);
- a fence run fails for any reason other than the jail-guard line handled in § 3.2;
- `verify blueprint` or `leak-check` is non-zero — quote the failing line; do not edit a file under `.claude/**` or `.opencode/**` to make it pass;
- the fence does not start (`docker` / `compose` error) — never fall back to a host run.

The report file: write `tmp/opencode-runtime-close.md` as you go (every quoted output, in order). `tmp/` is gitignored; the squash and merge messages above carry the durable summary.
