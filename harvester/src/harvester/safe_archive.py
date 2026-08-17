"""
safe_archive.py — Hostile-archive-safe list+read-on-demand for the harvester MCP.

Design: LIST-FIRST, READ-ONE-MEMBER-ON-DEMAND, all content read INTO MEMORY.
Never calls extractall(); never builds filesystem paths for zip/tar members.

Formats: .zip, .tar / .tar.gz / .tar.bz2 / .tar.xz, .7z, .rar

Security properties:
  - Path traversal blocked: absolute paths, '..' components, null bytes rejected.
  - Symlinks refused (zip external_attr, tar issym/islnk, rar is_symlink).
  - Zip-bomb limits: MAX_MEMBERS, MAX_TOTAL_BYTES, MAX_FILE_BYTES, MAX_RATIO.
  - tar: uses extractfile() (in-memory), NOT extract()/extractall().
    Sidesteps CVE-2025-4517 / CVE-2024-12718.
  - 7z: extracts single target to TemporaryDirectory, verifies realpath stays inside.
  - rar: uses rarfile pure-Python listing; extraction degrades gracefully if
    unar/unrar binary is absent.
"""

from __future__ import annotations

import os
import tarfile
import tempfile
import unicodedata
import zipfile
from dataclasses import dataclass
from pathlib import Path

# ---------------------------------------------------------------------------
# Limits — module-level so tests can monkeypatch
# ---------------------------------------------------------------------------
MAX_MEMBERS: int = 1_000
MAX_TOTAL_BYTES: int = 1 * 1024**3  # 1 GiB
MAX_FILE_BYTES: int = 100 * 1024**2  # 100 MiB
MAX_RATIO: int = 100              # uncompressed/compressed; flags zip-bombs
MAX_NAME_LEN: int = 255


# ---------------------------------------------------------------------------
# Public data model
# ---------------------------------------------------------------------------

@dataclass
class Member:
    name: str
    compressed_size: int     # 0 if unknown (e.g. tar stream-compressed)
    uncompressed_size: int
    is_dir: bool
    is_symlink: bool


class ArchiveError(Exception):
    """Raised for any archive safety violation or unsupported operation."""


# ---------------------------------------------------------------------------
# Internal helpers
# ---------------------------------------------------------------------------

def _validate_name(name: str) -> str:
    """NFC-normalize then assert safety; returns the normalized name."""
    name = unicodedata.normalize("NFC", name)
    if "\x00" in name:
        raise ArchiveError(f"Member name contains null byte: {name!r}")
    if os.path.isabs(name):
        raise ArchiveError(f"Member name is absolute path: {name!r}")
    parts = Path(name).parts
    if ".." in parts:
        raise ArchiveError(f"Member name contains '..': {name!r}")
    if len(name) > MAX_NAME_LEN:
        raise ArchiveError(
            f"Member name too long ({len(name)} > {MAX_NAME_LEN}): {name[:80]!r}…"
        )
    return name


def _check_caps(members: list[Member]) -> None:
    """Enforce listing-phase limits (count, total bytes, per-member ratio)."""
    if len(members) > MAX_MEMBERS:
        raise ArchiveError(
            f"Archive has {len(members)} members (limit {MAX_MEMBERS})"
        )
    total = sum(m.uncompressed_size for m in members if not m.is_dir)
    if total > MAX_TOTAL_BYTES:
        raise ArchiveError(
            f"Archive total uncompressed size {total} bytes "
            f"exceeds limit {MAX_TOTAL_BYTES}"
        )
    for m in members:
        if m.compressed_size > 0:
            ratio = m.uncompressed_size / m.compressed_size
            if ratio > MAX_RATIO:
                raise ArchiveError(
                    f"Member {m.name!r}: compression ratio {ratio:.1f}x "
                    f"exceeds {MAX_RATIO}x (zip-bomb?)"
                )


def _cap_read(data: bytes, name: str) -> bytes:
    """Assert read bytes fit in MAX_FILE_BYTES (announced sizes are untrusted)."""
    if len(data) > MAX_FILE_BYTES:
        raise ArchiveError(
            f"Member {name!r} expanded to {len(data)} bytes, "
            f"exceeding MAX_FILE_BYTES={MAX_FILE_BYTES}"
        )
    return data


# ---------------------------------------------------------------------------
# ZIP — stdlib zipfile
# ---------------------------------------------------------------------------

def _is_zip_symlink(info: zipfile.ZipInfo) -> bool:
    """
    Unix external_attr upper 16 bits encode the Unix mode.
    S_IFLNK = 0xA000; check the file-type nibble only.
    Guard on create_system == 3 (Unix); Windows external_attr has different semantics.
    """
    if info.create_system != 3:
        return False
    unix_mode = info.external_attr >> 16
    return (unix_mode & 0xF000) == 0xA000


def list_zip(path: str) -> list[Member]:
    members: list[Member] = []
    with zipfile.ZipFile(path, "r") as zf:
        for info in zf.infolist():
            name = _validate_name(info.filename)
            members.append(Member(
                name=name,
                compressed_size=info.compress_size,
                uncompressed_size=info.file_size,
                is_dir=info.is_dir(),
                is_symlink=_is_zip_symlink(info),
            ))
    _check_caps(members)
    return members


def read_zip(path: str, name: str) -> bytes:
    name = _validate_name(name)
    with zipfile.ZipFile(path, "r") as zf:
        try:
            info = zf.getinfo(name)
        except KeyError:
            # Unicode-normalization mismatch: the listing shows the NFC name, but the archive may
            # store NFD — match by normalized name before giving up.
            info = next((i for i in zf.infolist()
                         if unicodedata.normalize("NFC", i.filename) == name), None)
            if info is None:
                raise ArchiveError(f"Member not found: {name!r}")
        if _is_zip_symlink(info):
            raise ArchiveError(f"Refusing to read symlink member: {name!r}")
        if info.is_dir():
            raise ArchiveError(f"Member is a directory: {name!r}")
        if info.compress_size > 0 and info.file_size / info.compress_size > MAX_RATIO:
            raise ArchiveError(
                f"Member {name!r}: compression ratio {info.file_size / info.compress_size:.0f}x "
                f"exceeds {MAX_RATIO}x (zip-bomb?)")
        if info.file_size > MAX_FILE_BYTES:
            raise ArchiveError(
                f"Member {name!r} announced size {info.file_size} "
                f"exceeds MAX_FILE_BYTES={MAX_FILE_BYTES}"
            )
        with zf.open(info) as fh:
            # Read cap+1 to detect if actual bytes exceed the limit
            data = fh.read(MAX_FILE_BYTES + 1)
    return _cap_read(data, name)


# ---------------------------------------------------------------------------
# TAR (tar, tar.gz, tar.bz2, tar.xz) — stdlib tarfile
# CRITICAL: uses extractfile() for in-memory reads, never extract()/extractall().
# Sidesteps CVE-2025-4517 / CVE-2024-12718.
# ---------------------------------------------------------------------------

def list_tar(path: str) -> list[Member]:
    members: list[Member] = []
    with tarfile.open(path, "r:*") as tf:
        for m in tf:
            name = _validate_name(m.name)
            members.append(Member(
                name=name,
                compressed_size=0,   # tar doesn't expose per-member compressed size
                uncompressed_size=m.size,
                is_dir=m.isdir(),
                is_symlink=m.issym() or m.islnk(),
            ))
    _check_caps(members)
    return members


def read_tar(path: str, name: str) -> bytes:
    name = _validate_name(name)
    with tarfile.open(path, "r:*") as tf:
        try:
            m = tf.getmember(name)
        except KeyError:
            m = next((mem for mem in tf.getmembers()
                      if unicodedata.normalize("NFC", mem.name) == name), None)
            if m is None:
                raise ArchiveError(f"Member not found: {name!r}")
        if m.issym() or m.islnk():
            raise ArchiveError(
                f"Refusing to read symlink/hardlink member: {name!r}"
            )
        if m.isdir():
            raise ArchiveError(f"Member is a directory: {name!r}")
        if m.size > MAX_FILE_BYTES:
            raise ArchiveError(
                f"Member {name!r} announced size {m.size} "
                f"exceeds MAX_FILE_BYTES={MAX_FILE_BYTES}"
            )
        fh = tf.extractfile(m)  # in-memory; never writes to filesystem
        if fh is None:
            raise ArchiveError(
                f"extractfile() returned None for {name!r} "
                "(not a regular file?)"
            )
        data = fh.read(MAX_FILE_BYTES + 1)
    return _cap_read(data, name)


# ---------------------------------------------------------------------------
# 7Z — py7zr >= 1.1.3
# Single-member extraction to TemporaryDirectory; realpath sandbox check.
# ---------------------------------------------------------------------------

def list_7z(path: str) -> list[Member]:
    try:
        import py7zr
    except ImportError:
        raise ArchiveError("py7zr package not installed; cannot read .7z archives")
    members: list[Member] = []
    with py7zr.SevenZipFile(path, mode="r") as szf:
        for info in szf.list():
            name = _validate_name(info.filename)
            members.append(Member(
                name=name,
                compressed_size=info.compressed or 0,
                uncompressed_size=info.uncompressed or 0,
                is_dir=info.is_directory,
                is_symlink=False,  # py7zr list() does not expose symlink type
            ))
    _check_caps(members)
    return members


def read_7z(path: str, name: str) -> bytes:
    try:
        import py7zr
    except ImportError:
        raise ArchiveError("py7zr package not installed; cannot read .7z archives")
    name = _validate_name(name)
    # Cap BEFORE extraction: unlike zip/tar (which read only MAX_FILE_BYTES+1 bytes), szf.extract
    # writes the WHOLE expansion to disk first — a few-KB 7z entry that announces tens of GB would
    # exhaust the disk before any post-extract size check. Gate on the announced size + ratio here.
    with py7zr.SevenZipFile(path, mode="r") as szf:
        info = next((i for i in szf.list()
                     if unicodedata.normalize("NFC", i.filename) == name), None)
    if info is None:
        raise ArchiveError(f"Member not found: {name!r}")
    if info.is_directory:
        raise ArchiveError(f"Member is a directory: {name!r}")
    announced = info.uncompressed or 0
    if announced > MAX_FILE_BYTES:
        raise ArchiveError(
            f"Member {name!r} announced size {announced} "
            f"exceeds MAX_FILE_BYTES={MAX_FILE_BYTES}"
        )
    compressed = info.compressed or 0
    if compressed > 0 and announced / compressed > MAX_RATIO:
        raise ArchiveError(
            f"Member {name!r}: compression ratio {announced / compressed:.0f}x "
            f"exceeds {MAX_RATIO}x (zip-bomb?)"
        )
    with tempfile.TemporaryDirectory() as tmpdir:
        with py7zr.SevenZipFile(path, mode="r") as szf:
            szf.extract(targets=[name], path=tmpdir)
        target = os.path.join(tmpdir, name)
        real = os.path.realpath(target)
        real_tmp = os.path.realpath(tmpdir)
        # Sandbox: resolved path must be strictly inside tmpdir
        if not (real.startswith(real_tmp + os.sep) or real == real_tmp):
            raise ArchiveError(
                f"Path traversal detected for {name!r}: "
                f"resolved to {real!r} outside {real_tmp!r}"
            )
        if not os.path.isfile(real):
            raise ArchiveError(
                f"Member not found or is not a regular file: {name!r}"
            )
        size = os.path.getsize(real)
        if size > MAX_FILE_BYTES:
            raise ArchiveError(
                f"Member {name!r} size {size} exceeds MAX_FILE_BYTES={MAX_FILE_BYTES}"
            )
        with open(real, "rb") as fh:
            data = fh.read(MAX_FILE_BYTES + 1)
    return _cap_read(data, name)


# ---------------------------------------------------------------------------
# RAR — rarfile (pure-Python listing; extraction requires unar/unrar binary)
# ---------------------------------------------------------------------------

def list_rar(path: str) -> list[Member]:
    try:
        import rarfile
    except ImportError:
        raise ArchiveError(
            "rarfile package not installed; cannot read .rar archives"
        )
    members: list[Member] = []
    with rarfile.RarFile(path, "r") as rf:
        for info in rf.infolist():
            name = _validate_name(info.filename)
            is_symlink = False
            if hasattr(info, "is_symlink"):
                is_symlink = bool(info.is_symlink())
            members.append(Member(
                name=name,
                compressed_size=info.compress_size,
                uncompressed_size=info.file_size,
                is_dir=info.is_dir(),
                is_symlink=is_symlink,
            ))
    _check_caps(members)
    return members


def read_rar(path: str, name: str) -> bytes:
    try:
        import rarfile
    except ImportError:
        raise ArchiveError(
            "rarfile package not installed; cannot read .rar archives"
        )
    name = _validate_name(name)
    with rarfile.RarFile(path, "r") as rf:
        try:
            info = rf.getinfo(name)
        except rarfile.NoRarEntry:
            raise ArchiveError(f"Member not found: {name!r}")
        is_symlink = False
        if hasattr(info, "is_symlink"):
            is_symlink = bool(info.is_symlink())
        if is_symlink:
            raise ArchiveError(f"Refusing to read symlink member: {name!r}")
        if info.is_dir():
            raise ArchiveError(f"Member is a directory: {name!r}")
        if info.file_size > MAX_FILE_BYTES:
            raise ArchiveError(
                f"Member {name!r} announced size {info.file_size} "
                f"exceeds MAX_FILE_BYTES={MAX_FILE_BYTES}"
            )
        try:
            data = rf.read(name)
        except rarfile.NeedFirstVolume as exc:
            raise ArchiveError(f"Multi-volume RAR not supported: {exc}") from exc
        except rarfile.BadRarFile as exc:
            raise ArchiveError(f"RAR extraction failed: {exc}") from exc
        except Exception as exc:
            msg = str(exc).lower()
            if "unrar" in msg or "unar" in msg or "tool" in msg:
                raise ArchiveError(
                    "RAR extraction requires unar or unrar binary. "
                    f"Install with: apt-get install unrar-free  ({exc})"
                ) from exc
            raise ArchiveError(f"RAR read error: {exc}") from exc
    return _cap_read(data, name)


# ---------------------------------------------------------------------------
# Format detection — magic bytes first, extension fallback for tar variants
# ---------------------------------------------------------------------------

_MAGIC: dict[bytes, str] = {
    b"PK\x03\x04": "zip",
    b"PK\x05\x06": "zip",  # empty zip
    b"7z\xbc\xaf'\x1c": "7z",
    b"Rar!\x1a\x07\x00": "rar",       # RAR4
    b"Rar!\x1a\x07\x01\x00": "rar",   # RAR5
}

_TAR_EXTENSIONS = {".tar", ".tgz", ".gz", ".bz2", ".xz", ".tbz2", ".tbz", ".txz"}


def _detect_format(path: str) -> str:
    try:
        with open(path, "rb") as fh:
            header = fh.read(8)
    except OSError:
        header = b""
    for magic, fmt in _MAGIC.items():
        if header.startswith(magic):
            return fmt
    # gzip / bzip2 / xz magic → a compressed tar (inner tar magic is hidden) — covers .tbz2/.txz/etc.
    if header.startswith((b"\x1f\x8b", b"BZh", b"\xfd7zXZ\x00")):
        return "tar"
    ext = Path(path).suffix.lower()
    if ext in _TAR_EXTENSIONS:
        return "tar"
    raise ArchiveError(f"Unrecognized archive format: {path!r}")


# ---------------------------------------------------------------------------
# Public top-level API
# ---------------------------------------------------------------------------

def list_archive(path: str) -> list[Member]:
    """
    List all members of an archive safely (no extraction).

    Raises ArchiveError on any security violation or unsupported format.
    """
    fmt = _detect_format(path)
    dispatch = {"zip": list_zip, "tar": list_tar, "7z": list_7z, "rar": list_rar}
    return dispatch[fmt](path)


def read_archive_member(path: str, name: str) -> bytes:
    """
    Read a single named member from an archive into memory.

    Raises ArchiveError if the member is a symlink, hardlink, directory,
    absolute path, path-traversal attempt, or exceeds size limits.
    Never writes anything to the filesystem (zip/tar); uses a sandboxed
    TemporaryDirectory for 7z (single target only).
    """
    fmt = _detect_format(path)
    dispatch = {"zip": read_zip, "tar": read_tar, "7z": read_7z, "rar": read_rar}
    return dispatch[fmt](path, name)
