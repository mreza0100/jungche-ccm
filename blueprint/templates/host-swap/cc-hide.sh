#!/usr/bin/env bash
# cc-hide.sh [--exit] — delegate to the Go engine. The full self-identification (whoami rungs),
# hide write (shared store + carrier), and the detached --exit choreography live in
# `cc-fleet hide --self` (internal/hide). The pre-cutover implementation is archived in
# /tmp/cc-fleet-legacy-2026-08-11.tar.gz and in git history.
exec "$HOME/.local/bin/cc-fleet" hide --self "$@"
