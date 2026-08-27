# pfm — one entry point for building, gating, installing, and keeping the
# fleet's own processes honest.
#
# This file does NOT reimplement the repo gates. `.claude/scripts/dev.sh` owns
# `test`, `iso`, `verify`, and friends for every project; duplicating them here
# would create a second definition of "green" that drifts from the first. The
# targets below delegate to it, and add only what it does not own: the host
# mirror install, and the stale-process problem that install creates.
#
# THE STALE-PROCESS PROBLEM, stated once because every target below exists
# because of it: replacing ~/.local/bin/pfm does not touch a process already
# running the old image. Linux keeps the deleted inode alive for it, so a
# long-lived `pfm mcp serve` keeps serving yesterday's tool descriptions and a
# days-old `pfm ls` picker keeps writing yesterday's decisions over today's
# fixes. Both fail SILENTLY — the binary on disk is new, the behaviour is old,
# and nothing in between says a word. `make install` therefore restarts the
# daemon every time and reports every other stale process it can see.

SHELL := /usr/bin/env bash
# Real exit codes: a failing command in a pipe fails the target, instead of the
# last command in the pipe deciding the verdict.
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

REPO    := $(abspath $(dir $(abspath $(lastword $(MAKEFILE_LIST))))..)
DEV     := $(REPO)/.claude/scripts/dev.sh
BIN     := $(HOME)/.local/bin/pfm
LOCAL   := pfm.dev
UNIT    := pfm-mcp.service

.PHONY: help build fmt fmt-check vet test iso gate install host-install \
        mcp-restart mcp-status stale kill-stale clean

help: ## List every target
	@echo "pfm targets — run as: make <target>"
	@echo
	@grep -hE '^[a-z-]+:.*?## ' $(MAKEFILE_LIST) \
	  | sort \
	  | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'
	@echo
	@echo "Gates delegate to $(DEV) — this file never redefines them."

# ---------------------------------------------------------------- build ----

build: ## Compile to ./pfm.dev (a local artifact, never the shipped path)
	go build -o $(LOCAL) ./cmd/pfm

fmt: ## Rewrite formatting in place
	gofmt -w .

fmt-check: ## Fail if anything is unformatted, naming the files
	@unformatted="$$(gofmt -l . | grep -v '^$$' || true)"; \
	if [ -n "$$unformatted" ]; then \
	  echo "UNFORMATTED:"; echo "$$unformatted"; exit 1; \
	fi; \
	echo "gofmt: clean"

vet: ## go vet the whole module
	go vet ./...

# ----------------------------------------------------------------- gate ----
# Both delegate. dev.sh is the single definition of this project's gate.

test: ## Host test suite (delegates to dev.sh)
	bash $(DEV) test pfm

iso: ## Containerised test suite (delegates to dev.sh iso)
	bash $(DEV) iso test pfm

gate: fmt-check vet test iso ## Everything a change must pass before it merges
	@echo "gate: all passed"

# -------------------------------------------------------------- install ----

install: host-install mcp-restart stale ## Build, replace the host binary, restart the daemon, report stale processes
	@echo
	@echo "install: complete — host binary replaced and the MCP daemon restarted."

host-install: ## Replace ~/.local/bin/pfm, keeping the previous build
	@tmp="$$(mktemp -t pfm.XXXXXX)"; \
	go build -o "$$tmp" ./cmd/pfm; \
	if [ -x "$(BIN)" ]; then cp -f "$(BIN)" "$(BIN).prev"; fi; \
	mkdir -p "$$(dirname "$(BIN)")"; \
	mv -f "$$tmp" "$(BIN)"; \
	echo "installed: $(BIN) (previous kept at $(BIN).prev)"
	@# Never write onto the running path directly: an executing binary gives
	@# ETXTBSY. Building elsewhere and renaming over it is atomic and lets any
	@# process already running keep its own image — which is exactly why the
	@# stale sweep below has to exist.

# ---------------------------------------------------------------- mcp ------

mcp-restart: ## Restart the MCP daemon so it serves the binary now on disk
	@if command -v systemctl >/dev/null 2>&1 && \
	    systemctl --user cat $(UNIT) >/dev/null 2>&1; then \
	  systemctl --user restart $(UNIT); \
	  sleep 1; \
	  state="$$(systemctl --user is-active $(UNIT) || true)"; \
	  if [ "$$state" != "active" ]; then \
	    echo "MCP-RESTART-FAILED: $(UNIT) is '$$state' after restart — the fleet has NO chat MCP server" >&2; \
	    exit 1; \
	  fi; \
	  pid="$$(systemctl --user show -p MainPID --value $(UNIT))"; \
	  echo "mcp: $(UNIT) restarted, pid $$pid"; \
	elif pgrep -f 'pfm mcp serve' >/dev/null 2>&1; then \
	  echo "mcp: no $(UNIT) unit; killing unsupervised 'pfm mcp serve' so the next client starts a fresh one"; \
	  pkill -f 'pfm mcp serve'; \
	else \
	  echo "mcp: no daemon running and no $(UNIT) unit — nothing to restart"; \
	fi
	@# Restarting unconditionally is deliberate. Deciding whether the daemon is
	@# stale needs process introspection that not every platform offers, and a
	@# restart is cheap, supervised, and idempotent — so the guarantee does not
	@# depend on a probe that might not be able to run.

mcp-status: ## Report the daemon and whether it is running a replaced binary
	@if command -v systemctl >/dev/null 2>&1 && \
	    systemctl --user cat $(UNIT) >/dev/null 2>&1; then \
	  echo "unit:  $$(systemctl --user is-active $(UNIT))"; \
	  pid="$$(systemctl --user show -p MainPID --value $(UNIT))"; \
	  echo "pid:   $$pid"; \
	  if [ -r "/proc/$$pid/exe" ]; then \
	    echo "image: $$(readlink /proc/$$pid/exe)"; \
	  else \
	    echo "image: UNKNOWN — cannot read /proc/$$pid/exe on this platform"; \
	  fi; \
	else \
	  echo "unit:  not installed"; \
	  pgrep -af 'pfm mcp serve' || echo "no 'pfm mcp serve' process running"; \
	fi

# --------------------------------------------------------------- stale -----

stale: ## List pfm processes still running a replaced binary
	@if [ ! -d /proc ]; then \
	  echo "STALE-UNKNOWN: this platform has no /proc, so stale pfm processes could NOT be enumerated." >&2; \
	  echo "  This is not a clean result. The MCP daemon was restarted regardless, so it is fresh;" >&2; \
	  echo "  any other long-lived pfm process (a 'pfm ls' picker) may still hold the old binary." >&2; \
	  exit 0; \
	fi; \
	found=0; \
	for pid in $$(pgrep -x pfm 2>/dev/null || true); do \
	  link="$$(readlink /proc/$$pid/exe 2>/dev/null || true)"; \
	  case "$$link" in \
	    *"(deleted)"*) \
	      found=$$((found + 1)); \
	      cmd="$$(tr '\0' ' ' < /proc/$$pid/cmdline 2>/dev/null | cut -c1-70)"; \
	      echo "STALE pid=$$pid  $$cmd";; \
	  esac; \
	done; \
	if [ "$$found" -eq 0 ]; then \
	  echo "stale: none — every running pfm process uses the binary now on disk"; \
	else \
	  echo "stale: $$found process(es) above are running a REPLACED binary."; \
	  echo "       They will keep behaving like the old build and can write stale"; \
	  echo "       state over new fixes. Close them, or run: make kill-stale"; \
	fi
	@# "none" and "could not look" are printed differently on purpose. A sweep
	@# that reports nothing found when it never managed to look is the exact
	@# shape this repo treats as a defect.

kill-stale: ## Kill pfm processes running a replaced binary (interactive pickers included)
	@if [ ! -d /proc ]; then \
	  echo "REFUSING: no /proc, so stale processes cannot be identified — killing pfm processes blind would take down live ones" >&2; \
	  exit 1; \
	fi; \
	killed=0; \
	for pid in $$(pgrep -x pfm 2>/dev/null || true); do \
	  link="$$(readlink /proc/$$pid/exe 2>/dev/null || true)"; \
	  case "$$link" in \
	    *"(deleted)"*) \
	      cmd="$$(tr '\0' ' ' < /proc/$$pid/cmdline 2>/dev/null | cut -c1-70)"; \
	      echo "killing pid=$$pid  $$cmd"; \
	      kill "$$pid" 2>/dev/null || echo "  could not signal pid=$$pid (already gone, or not ours)"; \
	      killed=$$((killed + 1));; \
	  esac; \
	done; \
	echo "kill-stale: signalled $$killed process(es)"
	@# Separate from `install` on purpose: a stale process may be a picker the
	@# operator is looking at right now. `install` names them; closing them is
	@# a decision someone makes, not a side effect of building.

clean: ## Remove local build artifacts
	rm -f $(LOCAL)
	@echo "clean: removed $(LOCAL)"
