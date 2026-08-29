---
name: h:gh
description: Host-local GitHub CLI bridge — `gh` is installed on this machine. Use it for GitHub operations (PRs, issues, releases, repo API), including the /pfm:release blueprint publish.
---

# GH — GitHub CLI Bridge

Host fact, not project prose — a machine-scope command, shared across every project on this host, not a per-repo copy. Assumes the `gh` CLI is installed and authenticated; if it isn't, say so and stop rather than guessing at credentials.
