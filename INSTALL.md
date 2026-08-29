# INSTALL — Install Professor

Two independent things live in this repo. Install what you need.

- **`pfm`** — the host fleet CLI: statusline, `/reload`, the chat MCP server, multi-account tooling. Binary or source, touches only your `$HOME`, no project files.
- **Professor, the discipline layer** — `CLAUDE.md`, agents, commands, the pipeline. Installed into YOUR project through a Claude-guided interview.

Shortest path first.

---

## Runtime prerequisites for the `pfm` install paths

Paths 1 and 2 use the same host runtime. Both require Linux or macOS on `amd64` or `arm64`,
plus `tmux` ≥ 1.8, `git`, a POSIX `sh`, `bash`, `zsh`, and `sleep`. Linux also requires
`setsid`; macOS requires `ps`, `lsof`, and `launchctl`. `systemd` on Linux is optional when
user units are unavailable, but the scheduler surface cannot be enabled without it.

The `claude` and `codex` executables are not installed by `pfm`. Their self-doctors are optional
engine diagnostics even when accounts are configured: a broken engine capability stays visible,
but it cannot block unrelated host installation. Use `--skip-engine codex` to skip the Codex
probe and leave Codex mirror and hook surfaces unmanaged for this run.

## 1. Binary install — `pfm` only (2 minutes)

No clone, no Go toolchain. The installer's own assets (command cards, launcher shim, scheduler units) are embedded in the binary.

Prerequisites: the shared runtime listed above and `git` (to resolve the latest tag — or read it off the [Releases page](https://github.com/mreza0100/professor/releases) by hand). The binary path does not require a Go toolchain or access to a Go module proxy.

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

For a filtered network that cannot reach a Go module host or proxy, use this binary path: it
only needs access to the release assets and the Git tag lookup above, not the module downloads
needed by a source build.

### Preview, optional components, and harvest footprint

Bare `pfm install` is a read-only preview. Review its planned writes and the harvest line before
applying the identical flag set with `--yes`:

```bash
pfm install --skip-harvest --skip-engine codex --skip-themes
pfm install --yes --skip-harvest --skip-engine codex --skip-themes
```

- `--skip-harvest` leaves the pinned harvestpy runtime unmanaged; it avoids the harvest download
  and its disk footprint. It does not hide a failed provision.
- `--skip-engine codex` suppresses the Codex dependency probe and Codex mirror/hooks. It does not
  alter Claude or OpenCode surfaces.
- `--skip-themes` suppresses source-fetched theme installation. Theme entries come from
  `templates/themes/sources.json` and the current Tokyo Night target is
  `~/.claude/themes/tokyo-night.json`.

The current embedded harvest plan is measured, not a promise for every host. On Linux `amd64`,
the cold package closure is about **3.1 GB** (3,106,174,573 bytes) to download and about
**5.8 GB** (5,786,939,761 bytes) installed. The uv and CPython bootstrap archives add roughly
57 MB, and temporary files or caches can require more free space. Other platforms and future
lock revisions vary; the preview is the authoritative plan for the host.

Theme fetch failures are visible nonfatal skips, so the rest of the install can continue. A
locally modified theme is preserved and reported as drift rather than overwritten. On uninstall,
only theme files recorded as installer-owned in the ownership ledger are removed.

Add `$HOME/.local/bin` to `PATH` if it isn't already, then:

```bash
pfm install             # preview — the default mode, no writes
pfm install --yes       # apply the preview
pfm install --vscode    # opt-in preview: make PFM the default VS Code terminal
pfm install --yes --vscode
```

`pfm install --yes` manages eight surfaces, all under `$HOME`; `--vscode` adds a ninth:

1. Staged assets — `~/.local/share/pfm/install/`
2. Command symlinks — `~/.claude/commands/` (`/reload`)
3. The `pfm-name-sync` scheduler — three systemd user units (Linux) or one launchd agent (macOS)
4. Every Claude account settings file it finds (`~/.claude/settings.json` and each `~/.cc/N/settings.json`) — adds the usage, group, and `/clear` `SessionEnd` hooks; adopts the statusline only if none is already set
5. `~/.codex/prompts/`, `~/.codex/skills/`, and `~/.codex/agents/` — Codex mirrors generated from
   the installed global Claude commands and host-global agent sources; only marker-owned command
   outputs are replaced or retired, while unmarked conflicts survive and stop the install by name
6. `~/.codex/hooks.json` — migrates surviving binary paths and removes retired clear-kill and
   Dream/STM hooks; it installs no automatic Codex hook
7. One source line appended to `~/.zshrc` — restart your shell (or `source ~/.zshrc`) for it to take effect
8. `~/.claude/themes/` — source-fetched themes declared by `templates/themes/sources.json`; a
   failed cosmetic fetch is reported and skipped without aborting the other surfaces
9. **Opt-in:** the VS Code user or remote-machine `settings.json` — adds a `PFM` terminal profile
   and selects it as the platform default. The profile opens a login zsh, then the installed shim
   opens the PFM picker at the shell's first prompt. PFM edits JSONC surgically, so comments and
   unrelated profiles survive; later installs retain ownership, and uninstall restores the prior
   default unless the operator changed it after installation.

Every rewritten file is backed up before it's touched.

**Known gate — read before you run it.** A mutating install refuses with exit 97 only while
PFM's name-sync job is actively running, so it cannot replace the job or binary mid-execution.
On Linux, wait or run `systemctl --user stop pfm-name-sync.service`; on macOS, wait or run
`launchctl bootout gui/$(id -u)/com.professor.pfm.name-sync`. The preview remains read-only.

---

## 2. Build from source — `pfm` only

```bash
REPO=mreza0100/professor
SOURCE_DIR="$HOME/.professor"
TAG=$(git ls-remote --tags --sort=-v:refname "https://github.com/${REPO}.git" 'v*' \
  | grep -v '\^{}' | head -1 | sed 's#.*/##')
git clone "https://github.com/${REPO}.git" "$SOURCE_DIR"
git -C "$HOME/.professor" checkout "$TAG"
GOPROXY=<proxy> go -C "$HOME/.professor/pfm" build -trimpath -ldflags "-X main.version=$TAG" -o "$HOME/.local/bin/pfm" ./cmd/pfm
```

Replace `<proxy>` with a Go module proxy reachable from your network (for example,
`https://proxy.golang.org`). The source path needs Go **1.24.13 or newer**, the floor declared
by `pfm/go.mod`, and access to the module host or proxy. The tag lookup and explicit checkout
keep the source and the binary on the same latest release; if the source directory already
exists, fetch and check out that tag there instead of cloning over it.

The source build has the same harvest cost and opt-outs as the
[preview/apply block above](#preview-optional-components-and-harvest-footprint). Then run the same
two commands as the binary path:

```bash
pfm install
pfm install --yes
```

Same eight base surfaces, the same optional VS Code surface, and the same rc-97 gate.

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
- Never installs an opt-in piece — Tier B roles, Codex, statusline, hooks, host fleet, memory backup — without an explicit yes.
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
| Host fleet wiring                              | `pfm install` — the only writer       | `~/.local/share/pfm/install/`, `~/.claude/commands/`, the systemd/launchd scheduler units, every Claude account `settings.json`, `~/.codex/{prompts,skills,agents,hooks.json}`, one `~/.zshrc` line, and the opt-in VS Code user/remote `settings.json` |
| Project discipline layer                       | The interview — the only writer       | `CLAUDE.md`, `.claude/`, `docs/`, `.professor/`, per-project `CLAUDE.md` + `.claude/`                                                                                                               |
| Host-level opt-ins chosen during the interview | `pfm install`, invoked on your behalf | Lands inside the host-fleet surfaces above — the interview never writes them directly                                                                                                           |
| Source-fetched themes (default; `--skip-themes` opts out) | `pfm install` | `~/.claude/themes/tokyo-night.json` and other targets declared by `templates/themes/sources.json`; exact ownership is recorded in the install ledger |

`pfm install --config-dir DIR` retargets the `~/.claude`-rooted writes to a different config directory — the only supported override.

### Codex homes are config-owned

An explicitly empty `codex.homes` array in the PFM machine config is authoritative: `"homes": []`
means no Codex home even if `~/.codex` exists and contains credentials. Non-empty entries may use
`~` or `$HOME/` and must name authenticated homes:

```json
{
  "version": 2,
  "codex": {
    "homes": [
      {"id": 1, "home": "~/.codex"}
    ]
  }
}
```

With an empty list, PFM does not fall back to the default home.

---

## Updating

**Any existing installation:** follow the
[v0.64.0 LLM upgrade runbook](releases/v0.64.0.md#llm-upgrade-runbook). Installations already on
`v0.60.1` or later use `pfm update --to v0.64.0`; `v0.60.0` or earlier must bootstrap the
checksum-verified v0.64.0 binary directly because the older updater cannot validate its own
replacement safely.

**`pfm` (machine layer, from `v0.60.1` onward):** `pfm update` consumes a tagged source-clone release
transactionally, then runs `install --yes` and `doctor`. `pfm init` scaffolds a project from the
clone recorded by install.

**The discipline layer (path 3):** updating an installed project's blueprint content is deliberate,
by-hand work — see `docs/SETUP.md` § "Staying current". Anything recorded in `.professor/drift.md`
is a forced keep-local that is never blindly overwritten.

---

## Uninstall

**`pfm`:** `pfm uninstall` — removes installer-owned links and theme files and restores the pre-install backups, per `pfm uninstall --help`. Locally modified theme files are preserved and reported rather than removed.

**The discipline layer:** no uninstall command exists anywhere in `templates/` or the shipped commands. Removing it is a manual `git` operation on your side — revert the install commit, or delete the written paths from the ownership table above.
