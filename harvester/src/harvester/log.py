"""Central logging for harvester.

Logs are written to a ROTATING FILE at `<project-root>/tmp/logs/harvester.log`
(never to stdout — stdout is the MCP stdio JSON-RPC channel and any stray byte there
corrupts the protocol; and a file survives the run for post-mortem inspection). The
level comes from the HARVESTER_LOG_LEVEL env var (default "INFO"); the file path can be
overridden with HARVESTER_LOG_FILE. If the log dir cannot be created/written (e.g. an
installed read-only wheel), we fall back to stderr so logging never crashes the server.

Use `get_logger("<module>")` to obtain a `harvester.<module>` child logger; children
propagate to the single handler installed on `harvester`.
"""

import logging
import os
import sys
from logging.handlers import RotatingFileHandler
from pathlib import Path

_CONFIGURED = False

# Keep each log file bounded; rotate so a long-lived server can't fill the disk.
_MAX_BYTES = 5 * 1024 * 1024  # 5 MiB per file
_BACKUP_COUNT = 3  # harvester.log + .1 .2 .3


def _project_root() -> Path:
    """The dir containing pyproject.toml, walking up from this module.

    Mirrors cache._project_root() — duplicated (not imported) to keep log.py free of
    intra-package deps, since cache.py imports this module. Falls back to two levels up
    (src/harvester/ -> repo root) when no pyproject.toml is found (installed wheel).
    """
    here = Path(__file__).resolve()
    for parent in here.parents:
        if (parent / "pyproject.toml").is_file():
            return parent
    return here.parents[2]


def _log_file() -> Path:
    """Resolve the log file path: HARVESTER_LOG_FILE, else <project-root>/tmp/logs/harvester.log."""
    env = os.environ.get("HARVESTER_LOG_FILE")
    return Path(env) if env else _project_root() / "tmp" / "logs" / "harvester.log"


def _make_handler() -> logging.Handler:
    """A rotating file handler under tmp/logs/, falling back to stderr if that dir is unwritable."""
    try:
        path = _log_file()
        path.parent.mkdir(parents=True, exist_ok=True)
        return RotatingFileHandler(
            path, maxBytes=_MAX_BYTES, backupCount=_BACKUP_COUNT, encoding="utf-8"
        )
    except OSError as e:
        # Read-only install dir or similar — degrade to stderr rather than crash the server.
        h = logging.StreamHandler(sys.stderr)
        h.setLevel(logging.WARNING)
        print(f"harvester: file logging unavailable ({e}); using stderr", file=sys.stderr)
        return h


def _configure() -> None:
    """Install one file handler + level on the `harvester` parent logger (idempotent)."""
    global _CONFIGURED
    if _CONFIGURED:
        return
    root = logging.getLogger("harvester")
    handler = _make_handler()
    handler.setFormatter(
        logging.Formatter(
            "%(asctime)s %(levelname)s %(name)s: %(message)s", datefmt="%Y-%m-%d %H:%M:%S"
        )
    )
    root.addHandler(handler)
    root.setLevel(os.environ.get("HARVESTER_LOG_LEVEL", "INFO").upper())
    root.propagate = False  # never bubble to the root logger / stdout
    _CONFIGURED = True


def get_logger(name: str) -> logging.Logger:
    """Return the `harvester.<name>` child logger, writing to the rotating tmp/logs file."""
    _configure()
    return logging.getLogger("harvester." + name)
