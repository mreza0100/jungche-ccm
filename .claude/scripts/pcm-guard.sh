#!/usr/bin/env bash
# transitional shim — hooks registered before the /pcm→/ptm rename still call this path; delete after every live session has restarted with the new settings.json
exec "$(dirname "$0")/ptm-guard.sh" "$@"
