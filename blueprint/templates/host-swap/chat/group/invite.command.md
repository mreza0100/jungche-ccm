---
name: chat:group:invite
description: Invite another live chat to a group by tmux session name, 🔖 label, or codex thread name — sends a join invitation; the target subscribes ITSELF so membership carries its own identity. Trigger — /chat:group:invite {group} {label}.
argument-hint: "{group} {label}"
---

# Chat Group Invite

Run `$HOME/.local/bin/pfm chat group invite {group} {label}` and report whether the invitation landed. The target joins by running subscribe itself — self-registration keeps its member identity equal to its own whoami; a chat that never acts on the invite is not a member.
