#!/usr/bin/env bash
# cx-hide.sh [--exit] — the Codex twin, delegated to the Go engine. `cc-fleet hide --self`
# resolves a Codex chat via CODEX_THREAD_ID / pane argv / the state store and writes the hide
# on the lineage root. The pre-cutover implementation is archived in
# /tmp/cc-fleet-legacy-2026-08-11.tar.gz and in git history.
exec "$HOME/.local/bin/cc-fleet" hide --self "$@"
