---
name: dev
description: "Dev environment management. Invoked as $dev [subcommand]. Subcommands — start (default), kill, restart, status, log, snapshot. Manages local dev stack."
---

Read `.claude/commands/dev.md` in full — it is your complete role manual. Follow it verbatim.

**Argument:** optional subcommand (defaults to start).

All infrastructure operations go through the Makefile: `make -C {INFRA_PROJECT} <target>`. Never run docker compose, psql, or aws sqs directly.
