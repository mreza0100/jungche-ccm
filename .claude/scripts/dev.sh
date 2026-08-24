#!/usr/bin/env bash
set -euo pipefail

# dev.sh — the single entry point for building, testing, and inspecting this
# repo's three projects. /dev drives it; agents call it directly.
#
# WHAT THIS SCRIPT REPORTS WHEN IT IS ITSELF BROKEN:
#   - a missing toolchain (go/node/npm) is TOOLCHAIN-MISSING and exits non-zero.
#     It is NEVER reported as a pass or a skip: "we could not look" and "there is
#     nothing wrong" must not print the same word.
#   - a project with no dependencies installed is NOT-INSTALLED, not "clean".
#   - an unknown project or command exits 2 with usage — never a silent no-op
#     that a caller could read as success.
#   - every command's own exit status propagates; nothing is swallowed with `|| true`.

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

PROJECTS=(blueprint pfm walker)

# project -> directory
proj_dir() {
  case "$1" in
    blueprint) echo "blueprint" ;;
    pfm)  echo "pfm" ;;
    walker)    echo "engines/wave-walker/engine" ;;
    *) return 1 ;;
  esac
}

RED=$'\033[31m'; GREEN=$'\033[32m'; YELLOW=$'\033[33m'; DIM=$'\033[2m'; OFF=$'\033[0m'
[[ -t 1 ]] || { RED=""; GREEN=""; YELLOW=""; DIM=""; OFF=""; }

ok()   { printf '%s  PASS%s  %s\n' "$GREEN" "$OFF" "$*"; }
bad()  { printf '%s  FAIL%s  %s\n' "$RED" "$OFF" "$*"; }
warn() { printf '%s  WARN%s  %s\n' "$YELLOW" "$OFF" "$*"; }
info() { printf '%s        %s%s\n' "$DIM" "$*" "$OFF"; }
head_() { printf '\n%s──%s %s\n' "$DIM" "$OFF" "$*"; }

FAILURES=0
fail_step() { bad "$*"; FAILURES=$((FAILURES + 1)); }

need_tool() { # need_tool <bin> <project>
  if ! command -v "$1" >/dev/null 2>&1; then
    fail_step "$2: TOOLCHAIN-MISSING — '$1' not on PATH; this project could not be checked"
    return 1
  fi
}

node_installed() { # node_installed <dir> <project>
  if [[ ! -f "$1/node_modules/.package-lock.json" ]]; then
    fail_step "$2: NOT-INSTALLED — no node_modules; run '$(basename "$0") install $2'"
    return 1
  fi
}

run() { # run <label> -- <cmd...>
  local label="$1"; shift
  [[ "${1:-}" == "--" ]] && shift
  info "\$ $*"
  if "$@"; then ok "$label"; else fail_step "$label (exit $?)"; fi
}

repo_git() {
  if [[ -n "${PFM_DEV_REPO_GIT_DIR:-}" && -n "${PFM_DEV_REPO_WORK_TREE:-}" ]]; then
    git --git-dir="$PFM_DEV_REPO_GIT_DIR" --work-tree="$PFM_DEV_REPO_WORK_TREE" \
      -c safe.directory="$PFM_DEV_REPO_WORK_TREE" "$@"
  else
    git "$@"
  fi
}

# ─── status ──────────────────────────────────────────────────────────────────

cmd_status() {
  head_ "toolchain"
  for t in go node npm git jq; do
    if command -v "$t" >/dev/null 2>&1; then
      ok "$t — $(command -v "$t")"
    else
      warn "$t — MISSING (projects needing it report TOOLCHAIN-MISSING, not a pass)"
    fi
  done

  head_ "projects"
  for p in "${PROJECTS[@]}"; do
    local d; d="$(proj_dir "$p")"
    if [[ ! -d "$d" ]]; then fail_step "$p — directory $d/ is MISSING"; continue; fi
    case "$p" in
      blueprint)
        ok "$p — $d/ ($(find "$d" -type f -not -name refresh-map.json | wc -l | tr -d ' ') shipped files, no build)" ;;
      pfm)
        ok "$p — $d/ (go $(sed -n 's/^go //p' "$d/go.mod" | head -1))" ;;
      walker)
        if [[ -d "$d/node_modules" ]]; then
          ok "$p — $d/ (npm, deps installed)"
        else
          warn "$p — $d/ (npm, NOT-INSTALLED — 'dev.sh install $p')"
        fi ;;
    esac
  done

  head_ "git"
  local dirty; dirty=$(repo_git status --porcelain | wc -l | tr -d ' ')
  info "branch $(repo_git rev-parse --abbrev-ref HEAD) @ $(repo_git rev-parse --short HEAD) — $dirty changed file(s)"
  info "version $(cat VERSION 2>/dev/null || echo '?') — newest tag $(repo_git describe --tags --abbrev=0 2>/dev/null || echo 'none')"

  head_ "install"
  for f in .professor/VERSION .professor/manifest.json CLAUDE.md AGENTS.md .claude/settings.json; do
    [[ -e "$f" ]] && ok "$f" || warn "$f — absent"
  done
}

# ─── per-project actions ─────────────────────────────────────────────────────

act_blueprint() { # the shipped product: mechanical gates, no build
  local action="$1"
  case "$action" in
    install|build|typecheck) info "blueprint: no $action step (markdown + shell)" ;;
    verify|test|all)
      head_ "blueprint — leak gate"
      local changed
      changed=$(repo_git status --porcelain -- blueprint scripts README.md INSTALL.md CHANGELOG.md releases \
                | awk '{print $NF}' | grep -v '/$' || true)
      if [[ -z "$changed" ]]; then
        info "no changed blueprint/public files — scanning the whole tracked blueprint tree instead"
        # shellcheck disable=SC2046
        if repo_git ls-files blueprint README.md INSTALL.md | xargs scripts/leak-check.sh --files; then
          ok "leak-check clean (full tracked scan)"
        else
          fail_step "leak-check FAILED — brand / PII / machine-path string in a public file"
        fi
      else
        # shellcheck disable=SC2086
        if scripts/leak-check.sh --files $changed; then
          ok "leak-check clean ($(wc -w <<<"$changed") changed file(s))"
        else
          fail_step "leak-check FAILED — brand / PII / machine-path string in a changed public file"
        fi
      fi

      head_ "blueprint — placeholder registry"
      # Scope: markdown templates only. Shell/JS templates use {VAR} for their own
      # runtime values, which are not install placeholders and never will be.
      local used unregistered out
      used=$(grep -rhoE '\{[A-Z][A-Z0-9_]+\}' --include='*.md' blueprint 2>/dev/null | sort -u || true)
      if [[ -z "$used" ]]; then
        fail_step "placeholder scan produced NO tokens at all — the SCAN is broken, not the templates"
      else
        unregistered=$(comm -23 <(printf '%s\n' "$used") \
                                <(grep -ohE '\{[A-Z][A-Z0-9_]+\}' docs/PLACEHOLDERS.md | sort -u))
        if [[ -z "$unregistered" ]]; then
          ok "every markdown-template token is registered in PLACEHOLDERS.md ($(wc -l <<<"$used") tokens)"
        else
          if [[ -n "${PFM_DEV_FENCE:-}" ]]; then
            out="$(mktemp)"
          else
            out="tmp/blueprint-unregistered-tokens.txt"
            mkdir -p tmp
          fi
          printf '%s\n' "$unregistered" > "$out"
          warn "$(wc -l <<<"$unregistered") of $(wc -l <<<"$used") markdown-template tokens are absent from PLACEHOLDERS.md"
          info "most frequent 10 (full list: $out):"
          grep -rhoE '\{[A-Z][A-Z0-9_]+\}' --include='*.md' blueprint \
            | grep -xFf "$out" | sort | uniq -c | sort -rn | head -10 \
            | while read -r n tok; do info "  ${n}x  $tok"; done
        fi
      fi

      head_ "blueprint — codex generated-marker claim"
      # The blueprint's shipped JS compiler and this repo's `pfm codex build`
      # write the same $HOME/.codex outputs on adopter hosts. A copy that stops
      # claiming the other's marker reports its files STALE forever; the gate
      # also reconciles every marked file's declared source against disk, so a
      # fossil generated from a deleted file fails BY NAME.
      if node scripts/check-codex-markers.mjs; then
        ok "compiler marker claims hold; every marked file has a live source"
      else
        fail_step "codex marker claim FAILED — a stranded marker or an orphaned generated file (see output)"
      fi

      head_ "blueprint — opencode mirror"
      # The OpenCode mirror must be current AND valid: check re-derives every
      # output from the Claude sources; doctor additionally parses each artifact.
      if need_tool node blueprint && node .claude/scripts/build-opencode.mjs check \
        && node .claude/scripts/build-opencode.mjs doctor | tail -1; then
        ok "opencode mirror current and parseable"
      else
        fail_step "opencode mirror FAILED — run: node .claude/scripts/build-opencode.mjs generate"
      fi

      head_ "blueprint — isolated-fence mount preflight"
      if bash infra/fence-preflight-test.sh; then
        ok "Docker Desktop mount targets are prepared before the read-only worktree bind"
      else
        fail_step "isolated-fence mount preflight FAILED — nested volume targets are not safely prepared"
      fi

      head_ "blueprint — self-hosted manifest"
      if bash infra/check-self-hosted-manifest.sh "$REPO_ROOT" blueprint pfm engines/wave-walker/engine; then
        ok "self-hosted manifest version, roster, and hashes match the repository"
      else
        fail_step "self-hosted manifest FAILED — its install ledger is stale or unreadable"
      fi

      ;;
    *) return 0 ;;
  esac
}

act_pfm() {
  local action="$1" d; d="$(proj_dir pfm)"
  need_tool go pfm || return 0
  case "$action" in
    install) run "pfm: go mod download" -- go -C "$d" mod download ;;
    build)   run "pfm: go build" -- go -C "$d" build ./... ;;
    typecheck|verify) run "pfm: go vet" -- go -C "$d" vet ./... ;;
    test)    run "pfm: go test" -- go -C "$d" test ./... ;;
    all)     act_pfm build; act_pfm verify; act_pfm test ;;
  esac
}

act_npm() { # act_npm <project> <action> [extra script...]
  local p="$1" action="$2" d; d="$(proj_dir "$p")"
  need_tool npm "$p" || return 0
  case "$action" in
    install)
      if [[ -f "$d/package-lock.json" ]]; then
        run "$p: npm ci" -- npm --prefix "$d" ci
      else
        run "$p: npm install" -- npm --prefix "$d" install
      fi ;;
    *)
      node_installed "$d" "$p" || return 0
      case "$action" in
        build)     run "$p: npm run build" -- npm --prefix "$d" run build ;;
        typecheck) run "$p: npm run typecheck" -- npm --prefix "$d" run typecheck ;;
        verify)
          if npm --prefix "$d" run 2>/dev/null | grep -q '^  verify'; then
            run "$p: npm run verify" -- npm --prefix "$d" run verify
          else
            info "$p: no verify script"
          fi ;;
        test)      run "$p: npm test" -- npm --prefix "$d" test ;;
      esac ;;
  esac
}

act_walker() {
  local action="$1"
  case "$action" in
    all) act_npm walker build; act_npm walker verify; act_npm walker typecheck; act_npm walker test ;;
    *) act_npm walker "$action" ;;
  esac
}

dispatch() { # dispatch <project> <action>
  case "$1" in
    blueprint) act_blueprint "$2" ;;
    pfm)  act_pfm "$2" ;;
    walker)    act_walker "$2" ;;
  esac
}

# ─── iso — the container fence ───────────────────────────────────────────────
# Runs a command inside the pfm-dev container (infra/docker-compose.yml) with
# THIS checkout — the worktree this script belongs to — mounted at /work: a
# fresh machine per run (own HOME, own tmux, no published ports). Files are
# edited on the host; the container only builds and tests.
# First output line is the fence proof (container hostname + HOME + /work).
# BROKEN STATE: docker or the compose file missing = TOOLCHAIN-MISSING and a
# non-zero exit — never a host fallback; a run that cannot print its fence
# proof did not run inside the fence.
cmd_iso() { # cmd_iso <action> [project]
  local action="${1:-}" target="${2:-pfm}"
  need_tool docker iso || exit 1
  need_tool git iso || exit 1
  local compose="$REPO_ROOT/infra/docker-compose.yml"
  if [[ ! -f "$compose" ]]; then
    fail_step "iso: TOOLCHAIN-MISSING — $compose not found"; exit 1
  fi
  if ! bash "$REPO_ROOT/infra/prepare-fence-mounts.sh" "$REPO_ROOT"; then
    fail_step "iso: prepare nested Docker volume targets under $REPO_ROOT"; exit 1
  fi

  local git_common git_dir git_dir_relative
  git_common="$(git -C "$REPO_ROOT" rev-parse --git-common-dir)"
  if [[ "$git_common" != /* ]]; then git_common="$REPO_ROOT/$git_common"; fi
  git_common="$(cd "$git_common" && pwd -P)"
  git_dir="$(git -C "$REPO_ROOT" rev-parse --absolute-git-dir)"
  git_dir="$(cd "$git_dir" && pwd -P)"
  case "$git_dir" in
    "$git_common") git_dir_relative="." ;;
    "$git_common"/*) git_dir_relative="${git_dir#"$git_common"/}" ;;
    *) fail_step "iso: git directory $git_dir is outside common directory $git_common"; exit 1 ;;
  esac
  export PFM_DEV_WORKTREE="$REPO_ROOT"
  export PFM_DEV_GIT_COMMON="$git_common"
  export PFM_DEV_GIT_DIR_REL="$git_dir_relative"
  local proof='echo "fence: container=$(hostname) HOME=$HOME work=$(pwd)"'
  case "$action" in
    shell)
      docker compose -f "$compose" run --rm --build pfm-dev zsh -c "$proof; exec zsh -i" ;;
    e2e)
      docker compose -f "$compose" run --rm --build pfm-dev bash -c "$proof; go -C pfm test -count=1 -tags e2e -p 1 ./e2e/..." ;;
    install|build|typecheck|verify|test|all|status)
      docker compose -f "$compose" run --rm --build pfm-dev bash -c "$proof; ./.claude/scripts/dev.sh $action $target" ;;
    *)
      echo "usage: dev.sh iso {install|build|typecheck|verify|test|all|status|e2e|shell} [project]" >&2; exit 2 ;;
  esac
}

usage() {
  cat >&2 <<EOF
usage: dev.sh <command> [project]

commands:
  status                 toolchain, projects, git, install state (default)
  install                fetch dependencies
  build                  compile
  typecheck              vet / tsc --noEmit
  verify                 pre-test gates (go vet, walker's verify, blueprint's leak + token gates)
  test                   run the test suite
  all                    verify + build + test for the project
  iso <cmd> [project]    run any command above — plus e2e | shell — inside the
                         pfm-dev container fence (infra/), worktree mounted

projects: ${PROJECTS[*]} | all (default)

Every project that cannot be checked reports TOOLCHAIN-MISSING or NOT-INSTALLED
and exits non-zero. A skipped check is never a pass.
EOF
  exit 2
}

CMD="${1:-status}"
TARGET="${2:-all}"

case "$CMD" in
  status) cmd_status ;;
  install|build|test|typecheck|verify|all)
    if [[ "$TARGET" == "all" ]]; then
      for p in "${PROJECTS[@]}"; do head_ "$p :: $CMD"; dispatch "$p" "$CMD"; done
    else
      proj_dir "$TARGET" >/dev/null 2>&1 || { echo "unknown project: $TARGET" >&2; usage; }
      head_ "$TARGET :: $CMD"
      dispatch "$TARGET" "$CMD"
    fi ;;
  iso) cmd_iso "${2:-}" "${3:-pfm}" ;;
  -h|--help|help) usage ;;
  *) echo "unknown command: $CMD" >&2; usage ;;
esac

if (( FAILURES > 0 )); then
  printf '\n%s%d step(s) failed or could not run.%s\n' "$RED" "$FAILURES" "$OFF"
  exit 1
fi
printf '\n%sall steps passed.%s\n' "$GREEN" "$OFF"
