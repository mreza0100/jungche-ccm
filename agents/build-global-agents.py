#!/usr/bin/env python3
"""Emit the global Codex agent TOMLs from their Claude .md sources, then install both.

Host-level twin of a repo's .claude/scripts/build-codex.mjs. Same escaping rules,
deliberately — a TOML basic string cannot hold a raw `"`, and an agent description
that quotes its own trigger phrases will break the parser at Codex startup and take
the whole role down with it.

Run after editing any ~/.professor/agents/*.md. Idempotent.
"""
import os, pathlib, re, sys, tomllib

AGENTS = pathlib.Path(__file__).resolve().parent
# Honor the runtimes' own config-dir overrides. Hardcoding ~/.claude installs
# into a directory a session with CLAUDE_CONFIG_DIR set never reads, so every
# role lands "successfully" somewhere invisible and the chat reports the agent
# as not found — an install that cannot fail is not an install that worked.
CLAUDE_HOME = pathlib.Path(os.environ.get("CLAUDE_CONFIG_DIR") or pathlib.Path.home() / ".claude")
CODEX_HOME = pathlib.Path(os.environ.get("CODEX_HOME") or pathlib.Path.home() / ".codex")
INSTALL_MD = CLAUDE_HOME / "agents"
INSTALL_TOML = CODEX_HOME / "agents"


def esc(s: str) -> str:                 # TOML basic string
    return s.replace("\\", "\\\\").replace('"', '\\"')


def esc_ml(s: str) -> str:              # TOML multi-line basic string
    return s.replace("\\", "\\\\").replace('"""', '\\"\\"\\"')


def emit(md: pathlib.Path) -> pathlib.Path:
    m = re.match(r"\A---\n(.*?)\n---\n(.*)\Z", md.read_text(), re.S)
    if not m:
        sys.exit(f"{md}: no frontmatter")
    fm, body = m.group(1), m.group(2).strip()
    name = re.search(r"^name:\s*(.+)$", fm, re.M)
    desc = re.search(r"^description:\s*(.+)$", fm, re.M)
    if not (name and desc):
        sys.exit(f"{md}: frontmatter needs both name: and description:")

    # Codex has no Agent tool — leads fan out through spawn_agent roles.
    body = body.replace(
        "children are Explore+haiku (never\nyour own type)",
        "children are spawned via spawn_agent as the `explorer` role (never your own type)",
    ).replace(
        "dispatch `general-purpose` children\n(`model: sonnet`)",
        "dispatch children via spawn_agent as the `explorer` role",
    )

    out = AGENTS / f"{name.group(1).strip()}.toml"
    out.write_text(
        f'name = "{esc(name.group(1).strip())}"\n'
        f'description = "{esc(desc.group(1).strip())}"\n'
        f'developer_instructions = """\n{esc_ml(body)}\n"""\n'
    )
    return out


def install(src: pathlib.Path, dest_dir: pathlib.Path) -> pathlib.Path:
    """Install as a real file so the registry holds no dependency on this directory."""
    dest_dir.mkdir(parents=True, exist_ok=True)
    dest = dest_dir / src.name
    if dest.is_symlink():
        dest.unlink()
    dest.write_text(src.read_text())
    return dest


def main() -> None:
    written = [emit(md) for md in sorted(AGENTS.glob("*.md"))]
    if not written:
        sys.exit(f"no agent .md files in {AGENTS}")
    # An emitter that ships an unparseable artifact has done nothing useful.
    for out in written:
        with out.open("rb") as fh:
            tomllib.load(fh)
        print(f"{out.name}: {out.stat().st_size} B, parses clean")
    for md in sorted(AGENTS.glob("*.md")):
        print(f"installed {install(md, INSTALL_MD)}")
    for tm in sorted(AGENTS.glob("*.toml")):
        print(f"installed {install(tm, INSTALL_TOML)}")


if __name__ == "__main__":
    main()
