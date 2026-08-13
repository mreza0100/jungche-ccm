---
name: dreamer
description: Run the files-only Dreamer night runner. Modes — /dreamer (autonomous after the signed first APPLY; otherwise supervise and HOLD), /dreamer supervise (DISTILL + gates + VERIFY, no APPLY), /dreamer apply {staging-dir} (apply a signed night), /dreamer lane {agent-type} (harvest one subagent type into its own lane). Triggers — "dreamer", "dream", "consolidate memory", the 🌙 nudge.
argument-hint: '[supervise|apply {staging-dir}|lane {agent-type}]'
---

# Dreamer CLI gateway

$ARGUMENTS

The engine is `/home/user/.professor/dreamer/dreamer-night.sh`. Run it; never reconstruct its flow in chat and never route through an agent, workflow, SDK, or the frozen `ENGINES/dreamers/` evidence.

Dispatch:

1. `apply {staging-dir}` → run `bash /home/user/.professor/dreamer/dreamer-night.sh apply {staging-dir}`.
2. `supervise` → run `bash /home/user/.professor/dreamer/dreamer-night.sh supervise`.
3. `lane {agent-type}` → add `--agent {agent-type}` to any of the above; the lane needs a profile at `lanes/{lane}.md` or the runner refuses.
4. No mode → if `/home/user/work/proja/.professor/stm/agents/explorer.md` exists, run `bash /home/user/.professor/dreamer/dreamer-night.sh`; otherwise run it with `supervise` and HOLD before APPLY.

Launch a supervised night through `cc-fleet headless run --name {repo}-{lane}-{date}-{time}` so it can be streamed while it works; a bare background process has no handle.

Report the runner's outcome and staging path. One seat attempt means one attempt: surface failures with preserved artifacts; never retry.
