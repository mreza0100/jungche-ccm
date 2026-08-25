# INSTALL — Install Professor

Two independent things live in this repo. Install what you need.

- **`pfm`** — the host fleet CLI: statusline, `/chat:*`, `/reload`, multi-account tooling. Binary or source, touches only your `$HOME`, no project files.
- **Professor, the discipline layer** — `CLAUDE.md`, agents, commands, the pipeline. Installed into YOUR project through a Claude-guided interview.

Shortest path first.

---

## 1. Binary install — `pfm` only (2 minutes)

No clone, no Go toolchain. The installer's own assets (command cards, launcher shim, scheduler units) are embedded in the binary.

Prerequisites: `linux` or `darwin`, `amd64` or `arm64`, `git` (to resolve the latest tag — or read it off the [Releases page](https://github.com/mreza0100/professor/releases) by hand). The installer's preflight also hard-requires these before any preview renders: `tmux` ≥ 1.8, `zsh`, `bash`, a POSIX `sh`, and on Linux `setsid` (`apt install tmux zsh bash` / `brew install tmux zsh` covers them). Optional, per feature: `systemd` (Linux user units) and the `claude`/`codex` CLIs.

```bash
REPO=mreza0100/professor
TAG=$(git ls-remote --tags --sort=-v:refname "https://github.com/${REPO}.git" 'v*' \
  | grep -v '\^{}' | head -1 | sed 's#.*/##')
OS=linux      # or darwin
ARCH=amd64    # or arm64
BINARY="pfm_${TAG}_${OS}_${ARCH}"
BASE_URL="https://github.com/${REPO}/releases/download/${TAG}"

curl -fLO "${BASE_URL}/${BINARY}"
curl -fLO "${BASE_URL}/SHA256SUMS"
awk -v file="${BINARY}" '$2 == file' SHA256SUMS > "${BINARY}.sha256"
if command -v sha256sum >/dev/null 2>&1; then
  sha256sum -c "${BINARY}.sha256"
else
  shasum -a 256 -c "${BINARY}.sha256"
fi

mkdir -p "$HOME/.local/bin"
install -m 0755 "${BINARY}" "$HOME/.local/bin/pfm"
```

The checksum only catches a corrupted/incomplete download — releases don't publish a separate signature.

Add `$HOME/.local/bin` to `PATH` if it isn't already, then:

```bash
pfm install             # preview — the default mode, no writes
pfm install --yes       # apply the preview
```

`pfm install --yes` wires seven surfaces, all under `$HOME`:

1. Staged assets — `~/.local/share/pfm/install/`
2. Command symlinks — `~/.claude/commands/` (`/reload`, the `/chat:*` family)
3. The `pfm-name-sync` scheduler — three systemd user units (Linux) or one launchd agent (macOS)
4. Every Claude account settings file it finds (`~/.claude/settings.json` and each `~/.cc/N/settings.json`) — adds the usage, group, and `/clear` `SessionEnd` hooks; adopts the statusline only if none is already set
5. `~/.codex/prompts/`, `~/.codex/skills/`, and `~/.codex/agents/` — Codex mirrors generated from
   the installed global Claude commands and host-global agent sources; only marker-owned command
   outputs are replaced or retired, while unmarked conflicts survive and stop the install by name
6. `~/.codex/hooks.json` — migrates surviving binary paths and removes retired clear-kill and
   Dream/STM hooks; it installs no automatic Codex hook
7. One source line appended to `~/.zshrc` — restart your shell (or `source ~/.zshrc`) for it to take effect

Every rewritten file is backed up before it's touched.

**Known gate — read before you run it.** On any host where a user `systemd` bus is already reachable (true for most already-logged-in Linux sessions, not just a host with a live fleet), `pfm install --yes` refuses with exit 97 and `live user systemd bus is reachable; run in a proven dead-bus jail`; the preview remains read-only. No flag bypasses this (checked `pfm install --help` and the installer source); a first-time install on a machine with no running user bus is unaffected. macOS gates only when the name-sync launch agent is actively mid-run: `launchctl bootout gui/$(id -u)/com.professor.pfm.name-sync` clears it, then retry.

---

## 2. Build from source — `pfm` only

```bash
git clone https://github.com/mreza0100/professor.git "$HOME/.professor"   # or wherever you keep it
go -C "$HOME/.professor/pfm" build -o "$HOME/.local/bin/pfm" ./cmd/pfm    # needs Go 1.24+
```

Then the same two commands as the binary path:

```bash
pfm install
pfm install --yes
```

Same seven surfaces, same rc-97 gate.

---

## 3. Full Professor adoption — the discipline layer

Everything above, plus `CLAUDE.md`, per-project agents, commands, docs scaffolding, and the whole pipeline — customized to your project through a Claude-guided interview. Nothing here duplicates what paths 1/2 already do; the interview invokes them for you if you opt into a host extra.

**Prerequisites:** Claude Code CLI, logged in. A git repository — if the project isn't one, Claude asks before `git init`. `jq` — required by the host installer and several hooks (`brew install jq` / `apt install jq`). Optional, per opt-in: `prettier` via `npx` (markdown format hook), `tmux` (VSCode launcher, host fleet), `node` (Codex mirror compiler), `gh`/`glab` (git-host skill). Ten to fifteen minutes of your attention.

Open Claude Code in your target project and paste:

```
Read https://raw.githubusercontent.com/mreza0100/professor/main/INSTALL.md and walk me through
the interactive install. Ask me each section's questions one at a time and wait for my answers
before proceeding. Do not assume — confirm everything.
```

Claude interviews you — structure, stack, disciplines, optional roles, persona, host extras — shows the full write plan (files, persona, doc re-homing), waits for you to type **"go"**, then generates. Ten to fifteen minutes, commits nothing.

**Guarantees, stated by the installer up front:**

- Never commits, pushes, or runs `git add` — files only; you review and commit.
- Never overwrites an existing `CLAUDE.md` / `.claude/` without asking (overwrite / merge / abort).
- Never installs an opt-in piece — Tier B roles, Codex, statusline, hooks, host fleet, themes, memory backup — without an explicit yes.
- Never touches a path outside the plan.

**Full protocol:** [`docs/SETUP.md`](./docs/SETUP.md) — the interview questions, pre-flight checks, existing-doc re-homing rules, generation steps, and verification. [`docs/PLACEHOLDERS.md`](./docs/PLACEHOLDERS.md) is the substitution law, [`docs/BLUEPRINT.md`](./docs/BLUEPRINT.md) the philosophy. Read all three before writing any file.

### Corrections to SETUP.md — this list wins until a release folds them in

1. **Smoke test:** `/wave:builder add-readme-section` no longer works — `/wave:builder` is orchestrated-only and refuses anything but a brief path. Verify with `/dev status` and a tiny `/jc` task instead.
2. **`AGENTS.md` is compiled, not symlinked.** If Codex is opted in, `scripts/build-codex.mjs` generates it from `CLAUDE.md`; never `ln -sf CLAUDE.md AGENTS.md`.
3. **`codex-mirror.sh` does not exist.** The Codex layer is `scripts/build-codex.mjs` (compiler) + `scripts/codex-sync.sh` (hooks).
4. **Skip the Council interview question.** No `/council` command ships; omit `council_panel` from the manifest.
5. **Clone at the latest tag**, not SETUP.md's hardcoded example — `git ls-remote --tags --sort=-v:refname <repo> 'v*'` gets the newest (same command as path 1 above).

---

## What gets written where

One writer per surface — the law that keeps the two installers from fighting over the same file.

| Surface                                        | Written by                            | Paths                                                                                                                                                                                               |
| ---------------------------------------------- | ------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Host fleet wiring                              | `pfm install` — the only writer       | `~/.local/share/pfm/install/`, `~/.claude/commands/`, the systemd/launchd scheduler units, every Claude account `settings.json`, `~/.codex/{prompts,skills,agents,hooks.json}`, one `~/.zshrc` line |
| Project discipline layer                       | The interview — the only writer       | `CLAUDE.md`, `.claude/`, `docs/`, `.professor/`, per-project `CLAUDE.md` + `.claude/`                                                                                                               |
| Host-level opt-ins chosen during the interview | `pfm install`, invoked on your behalf | Lands inside the six host-fleet surfaces above — the interview never writes them directly                                                                                                           |

`pfm install --config-dir DIR` retargets the `~/.claude`-rooted writes to a different config directory — the only supported override.

---

## Updating

**Any existing installation:** follow the
[v0.61.4 LLM upgrade runbook](releases/v0.61.4.md#llm-upgrade-runbook). Installations already on
`v0.60.1` or later use `pfm update --to v0.61.4`; `v0.60.0` or earlier must bootstrap the
checksum-verified v0.61.4 binary directly because the older updater cannot validate its own
replacement safely.

**`pfm` (machine layer, from `v0.60.1` onward):** `pfm update` consumes a tagged source-clone release
transactionally, then runs `install --yes` and `doctor`; `/pcm:update` remains the semantic
blueprint-content update. `pfm init` scaffolds a project from the clone recorded by install.

**The discipline layer (path 3):**

```
/pcm:update              # interactive update to the latest tag, replays your interview answers
/pcm:update check        # read-only preview
/pcm:update --to vX.Y.Z  # pin a specific release
```

Every file lands in one of three buckets: **auto-apply** (upstream changed, you didn't), **review** (both changed, or the change costs money — new hooks, env vars, model config are always reviewed), **manual** (migrations, new interview questions). Anything recorded in `.professor/drift.md` is a forced keep-local that overrides an auto-apply.

---

## Uninstall

**`pfm`:** `pfm uninstall` — removes the installed links and restores the pre-install backups, per `pfm uninstall --help`.

**The discipline layer:** no uninstall command exists anywhere in `blueprint/` or the shipped commands. Removing it is a manual `git` operation on your side — revert the install commit, or delete the written paths from the ownership table above.
