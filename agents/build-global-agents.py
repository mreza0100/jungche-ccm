#!/usr/bin/env python3
"""Emit the global Codex agent TOMLs from their Claude .md sources.

The host-level twin of the repo's .claude/scripts/build-codex.mjs. Same escaping
rules, deliberately — a TOML basic string cannot hold a raw `"`, and an agent
description that quotes its own trigger phrases ("walker fast", "map it now")
will break the parser at Codex startup and take the whole role down with it.

Escaping mirrors build-codex.mjs:151-153 exactly:
  basic string     -> backslash doubled, then `"` -> `\\"`
  multi-line basic -> backslash doubled, then a literal `\"\"\"` neutralised

Run after editing any ~/.professor/agents/*.md. Idempotent.
"""
import pathlib
import re
import sys

AGENTS = pathlib.Path(__file__).resolve().parent


def toml_escape(s: str) -> str:
    """For a TOML basic string — build-codex.mjs:151."""
    return s.replace("\\", "\\\\").replace('"', '\\"')


def toml_escape_multiline(s: str) -> str:
    """For a TOML multi-line basic string — build-codex.mjs:153."""
    return s.replace("\\", "\\\\").replace('"""', '\\"\\"\\"')


def emit(md_path: pathlib.Path) -> pathlib.Path:
    text = md_path.read_text()
    m = re.match(r"\A---\n(.*?)\n---\n(.*)\Z", text, re.S)
    if not m:
        sys.exit(f"{md_path}: no frontmatter")
    fm, body = m.group(1), m.group(2).strip()

    name_m = re.search(r"^name:\s*(.+)$", fm, re.M)
    desc_m = re.search(r"^description:\s*(.+)$", fm, re.M)
    if not name_m or not desc_m:
        sys.exit(f"{md_path}: frontmatter needs both name: and description:")
    name, desc = name_m.group(1).strip(), desc_m.group(1).strip()

    # Codex has no Agent tool — the lead spawns children through spawn_agent.
    body = body.replace(
        "children are Explore+haiku (never\nyour own type)",
        "children are spawned via spawn_agent as the `explorer` role (never your own type)",
    )

    out = AGENTS / f"{name}.toml"
    out.write_text(
        f'name = "{toml_escape(name)}"\n'
        f'description = "{toml_escape(desc)}"\n'
        f'developer_instructions = """\n{toml_escape_multiline(body)}\n"""\n'
    )
    return out


def main() -> None:
    written = [emit(md) for md in sorted(AGENTS.glob("*.md"))]
    if not written:
        sys.exit(f"no agent .md files in {AGENTS}")

    # An emitter that ships an unparseable artifact has done nothing useful.
    import tomllib

    for out in written:
        with out.open("rb") as fh:
            tomllib.load(fh)
        print(f"{out.name}: {out.stat().st_size} B, parses clean")


if __name__ == "__main__":
    main()
