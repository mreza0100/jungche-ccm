"""
Adversarial tests for safe_archive.py.

Every test builds a real archive in tmp_path and asserts the security property.
No network access, no side effects outside the test's tmp_path.
"""

from __future__ import annotations

import io
import tarfile
import zipfile
from pathlib import Path

import py7zr
import pytest

import harvester.safe_archive as sa
from harvester.safe_archive import (
    ArchiveError,
    Member,
    _validate_name,
    list_archive,
    read_archive_member,
)


# ---------------------------------------------------------------------------
# 1. Normal ZIP — list returns members; read_member returns exact bytes
# ---------------------------------------------------------------------------

class TestNormalZip:
    def test_list_returns_all_members(self, tmp_path):
        arc = tmp_path / "normal.zip"
        with zipfile.ZipFile(arc, "w") as zf:
            zf.writestr("hello.txt", b"hello world")
            zf.writestr("sub/foo.txt", b"foo content")
            zf.writestr("empty.dat", b"")
        members = list_archive(str(arc))
        names = {m.name for m in members}
        assert "hello.txt" in names
        assert "sub/foo.txt" in names
        assert "empty.dat" in names

    def test_read_member_exact_bytes(self, tmp_path):
        arc = tmp_path / "normal.zip"
        with zipfile.ZipFile(arc, "w") as zf:
            zf.writestr("hello.txt", b"hello world")
            zf.writestr("sub/foo.txt", b"foo content")
        assert read_archive_member(str(arc), "hello.txt") == b"hello world"
        assert read_archive_member(str(arc), "sub/foo.txt") == b"foo content"

    def test_member_dataclass_fields(self, tmp_path):
        arc = tmp_path / "fields.zip"
        with zipfile.ZipFile(arc, "w", compression=zipfile.ZIP_DEFLATED) as zf:
            zf.writestr("data.bin", b"x" * 1000)
        members = list_archive(str(arc))
        m = next(x for x in members if x.name == "data.bin")
        assert isinstance(m, Member)
        assert m.uncompressed_size == 1000
        assert not m.is_dir
        assert not m.is_symlink

    def test_read_missing_member_raises(self, tmp_path):
        arc = tmp_path / "a.zip"
        with zipfile.ZipFile(arc, "w") as zf:
            zf.writestr("real.txt", b"ok")
        with pytest.raises(ArchiveError, match="not found"):
            read_archive_member(str(arc), "ghost.txt")


# ---------------------------------------------------------------------------
# 2. ZIP path traversal — listing raises; read_member raises (never escapes)
# ---------------------------------------------------------------------------

class TestZipPathTraversal:
    def _make_traversal_zip(self, arc: Path) -> None:
        """Build a zip with a path-traversal member name directly."""
        with zipfile.ZipFile(arc, "w") as zf:
            zf.writestr("../evil.txt", b"owned")

    def test_listing_raises_on_dotdot(self, tmp_path):
        arc = tmp_path / "traversal.zip"
        self._make_traversal_zip(arc)
        with pytest.raises(ArchiveError, match=r"\.\."):
            list_archive(str(arc))

    def test_read_raises_on_dotdot(self, tmp_path):
        arc = tmp_path / "traversal.zip"
        self._make_traversal_zip(arc)
        with pytest.raises(ArchiveError):
            read_archive_member(str(arc), "../evil.txt")

    def test_nested_dotdot_refused(self, tmp_path):
        arc = tmp_path / "nested.zip"
        with zipfile.ZipFile(arc, "w") as zf:
            zf.writestr("a/../../b.txt", b"bad")
        with pytest.raises(ArchiveError, match=r"\.\."):
            list_archive(str(arc))

    def test_absolute_path_in_read_refused(self, tmp_path):
        arc = tmp_path / "ok.zip"
        with zipfile.ZipFile(arc, "w") as zf:
            zf.writestr("safe.txt", b"ok")
        with pytest.raises(ArchiveError, match="absolute"):
            read_archive_member(str(arc), "/etc/passwd")


# ---------------------------------------------------------------------------
# 3. ZIP symlink — is_symlink True; read_member refuses
# ---------------------------------------------------------------------------

class TestZipSymlink:
    def _make_symlink_zip(self, arc: Path, target: str = "/etc/passwd") -> None:
        """Build a zip with a Unix symlink entry via external_attr."""
        with zipfile.ZipFile(arc, "w") as zf:
            info = zipfile.ZipInfo("link.txt")
            info.create_system = 3           # Unix
            # 0xA1ED = S_IFLNK (0xA000) | 0x01ED (755 perms)
            info.external_attr = 0xA1ED0000  # shift mode into upper 16 bits
            zf.writestr(info, target)
            # Also include a safe member so we can confirm mixed archives work
            zf.writestr("safe.txt", b"safe content")

    def test_symlink_member_flagged(self, tmp_path):
        arc = tmp_path / "symlink.zip"
        self._make_symlink_zip(arc)
        members = list_archive(str(arc))
        link = next(m for m in members if m.name == "link.txt")
        assert link.is_symlink is True

    def test_safe_member_not_flagged(self, tmp_path):
        arc = tmp_path / "symlink.zip"
        self._make_symlink_zip(arc)
        members = list_archive(str(arc))
        safe = next(m for m in members if m.name == "safe.txt")
        assert safe.is_symlink is False

    def test_read_symlink_raises(self, tmp_path):
        arc = tmp_path / "symlink.zip"
        self._make_symlink_zip(arc)
        with pytest.raises(ArchiveError, match="symlink"):
            read_archive_member(str(arc), "link.txt")

    def test_read_safe_member_works(self, tmp_path):
        arc = tmp_path / "symlink.zip"
        self._make_symlink_zip(arc)
        data = read_archive_member(str(arc), "safe.txt")
        assert data == b"safe content"


# ---------------------------------------------------------------------------
# 4. tar.gz — symlink/hardlink refused; '../' refused; normal reads via extractfile
# ---------------------------------------------------------------------------

class TestTarGz:
    def _make_mixed_tgz(self, arc: Path) -> None:
        """Build a .tar.gz with a normal file, a symlink, and a hardlink."""
        with tarfile.open(str(arc), "w:gz") as tf:
            # Normal file
            content = b"normal content"
            buf = io.BytesIO(content)
            info = tarfile.TarInfo(name="normal.txt")
            info.size = len(content)
            tf.addfile(info, buf)
            # Symlink
            sym = tarfile.TarInfo(name="link.txt")
            sym.type = tarfile.SYMTYPE
            sym.linkname = "/etc/passwd"
            sym.size = 0
            tf.addfile(sym)
            # Hardlink
            lnk = tarfile.TarInfo(name="hardlink.txt")
            lnk.type = tarfile.LNKTYPE
            lnk.linkname = "normal.txt"
            lnk.size = 0
            tf.addfile(lnk)

    def test_symlink_flagged_in_listing(self, tmp_path):
        arc = tmp_path / "mixed.tar.gz"
        self._make_mixed_tgz(arc)
        members = list_archive(str(arc))
        sym = next(m for m in members if m.name == "link.txt")
        assert sym.is_symlink is True

    def test_hardlink_flagged_in_listing(self, tmp_path):
        arc = tmp_path / "mixed.tar.gz"
        self._make_mixed_tgz(arc)
        members = list_archive(str(arc))
        lnk = next(m for m in members if m.name == "hardlink.txt")
        assert lnk.is_symlink is True  # hardlinks also flagged

    def test_read_symlink_refused(self, tmp_path):
        arc = tmp_path / "mixed.tar.gz"
        self._make_mixed_tgz(arc)
        with pytest.raises(ArchiveError, match="symlink"):
            read_archive_member(str(arc), "link.txt")

    def test_read_hardlink_refused(self, tmp_path):
        arc = tmp_path / "mixed.tar.gz"
        self._make_mixed_tgz(arc)
        with pytest.raises(ArchiveError, match="symlink"):
            read_archive_member(str(arc), "hardlink.txt")

    def test_dotdot_member_refused_at_listing(self, tmp_path):
        arc = tmp_path / "dotdot.tar.gz"
        with tarfile.open(str(arc), "w:gz") as tf:
            buf = io.BytesIO(b"pwned")
            info = tarfile.TarInfo(name="../evil.txt")
            info.size = 5
            tf.addfile(info, buf)
        with pytest.raises(ArchiveError, match=r"\.\."):
            list_archive(str(arc))

    def test_normal_member_read_via_extractfile(self, tmp_path):
        """Critical: normal member reads correctly using extractfile() (in-memory)."""
        arc = tmp_path / "good.tar.gz"
        content = b"hello from tar.gz"
        with tarfile.open(str(arc), "w:gz") as tf:
            buf = io.BytesIO(content)
            info = tarfile.TarInfo(name="hello.txt")
            info.size = len(content)
            tf.addfile(info, buf)
        members = list_archive(str(arc))
        assert any(m.name == "hello.txt" for m in members)
        data = read_archive_member(str(arc), "hello.txt")
        assert data == content

    def test_plain_tar_works(self, tmp_path):
        arc = tmp_path / "plain.tar"
        content = b"plain tar content"
        with tarfile.open(str(arc), "w") as tf:
            buf = io.BytesIO(content)
            info = tarfile.TarInfo(name="plain.txt")
            info.size = len(content)
            tf.addfile(info, buf)
        assert read_archive_member(str(arc), "plain.txt") == content


# ---------------------------------------------------------------------------
# 5. .7z — list + read one member
# ---------------------------------------------------------------------------

class TestSevenZip:
    def test_list_and_read(self, tmp_path):
        arc = tmp_path / "test.7z"
        content = b"seven zip content"
        with py7zr.SevenZipFile(str(arc), mode="w") as szf:
            szf.writef(io.BytesIO(content), "hello.txt")
        members = list_archive(str(arc))
        assert any(m.name == "hello.txt" for m in members)
        data = read_archive_member(str(arc), "hello.txt")
        assert data == content

    def test_multiple_members(self, tmp_path):
        arc = tmp_path / "multi.7z"
        with py7zr.SevenZipFile(str(arc), mode="w") as szf:
            szf.writef(io.BytesIO(b"alpha"), "a.txt")
            szf.writef(io.BytesIO(b"beta"), "b.txt")
        members = list_archive(str(arc))
        names = {m.name for m in members}
        assert "a.txt" in names and "b.txt" in names
        assert read_archive_member(str(arc), "a.txt") == b"alpha"
        assert read_archive_member(str(arc), "b.txt") == b"beta"

    def test_missing_member_raises(self, tmp_path):
        arc = tmp_path / "test.7z"
        with py7zr.SevenZipFile(str(arc), mode="w") as szf:
            szf.writef(io.BytesIO(b"data"), "real.txt")
        with pytest.raises(ArchiveError):
            read_archive_member(str(arc), "ghost.txt")


# ---------------------------------------------------------------------------
# 6. Cap enforcement — monkeypatched limits keep fixtures small
# ---------------------------------------------------------------------------

class TestCapEnforcement:
    def test_too_many_members(self, tmp_path, monkeypatch):
        monkeypatch.setattr(sa, "MAX_MEMBERS", 3)
        arc = tmp_path / "big.zip"
        with zipfile.ZipFile(arc, "w") as zf:
            for i in range(5):
                zf.writestr(f"file{i}.txt", b"x")
        with pytest.raises(ArchiveError, match="members"):
            list_archive(str(arc))

    def test_total_bytes_exceeded(self, tmp_path, monkeypatch):
        monkeypatch.setattr(sa, "MAX_TOTAL_BYTES", 10)
        arc = tmp_path / "big.zip"
        with zipfile.ZipFile(arc, "w") as zf:
            zf.writestr("file.txt", b"x" * 20)
        with pytest.raises(ArchiveError, match="total uncompressed"):
            list_archive(str(arc))

    def test_single_file_too_large_on_read(self, tmp_path, monkeypatch):
        monkeypatch.setattr(sa, "MAX_FILE_BYTES", 5)
        arc = tmp_path / "big.zip"
        # Must also patch MAX_TOTAL_BYTES so listing passes
        monkeypatch.setattr(sa, "MAX_TOTAL_BYTES", 1024 ** 3)
        with zipfile.ZipFile(arc, "w") as zf:
            zf.writestr("file.txt", b"x" * 10)
        with pytest.raises(ArchiveError):
            read_archive_member(str(arc), "file.txt")

    def test_single_file_too_large_announced_refused_early(self, tmp_path, monkeypatch):
        """read_zip refuses if announced file_size already exceeds cap."""
        monkeypatch.setattr(sa, "MAX_FILE_BYTES", 5)
        monkeypatch.setattr(sa, "MAX_TOTAL_BYTES", 1024 ** 3)
        arc = tmp_path / "big.zip"
        with zipfile.ZipFile(arc, "w") as zf:
            zf.writestr("file.txt", b"x" * 10)
        with pytest.raises(ArchiveError):
            read_archive_member(str(arc), "file.txt")

    def test_compression_ratio_zip_bomb(self, tmp_path, monkeypatch):
        monkeypatch.setattr(sa, "MAX_RATIO", 2)
        # Create a highly-compressible entry
        arc = tmp_path / "bomb.zip"
        with zipfile.ZipFile(arc, "w", compression=zipfile.ZIP_DEFLATED) as zf:
            zf.writestr("bomb.txt", b"a" * 10_000)
        with pytest.raises(ArchiveError, match="ratio"):
            list_archive(str(arc))

    def test_too_many_members_tar(self, tmp_path, monkeypatch):
        monkeypatch.setattr(sa, "MAX_MEMBERS", 2)
        arc = tmp_path / "many.tar"
        with tarfile.open(str(arc), "w") as tf:
            for i in range(4):
                buf = io.BytesIO(b"x")
                info = tarfile.TarInfo(name=f"f{i}.txt")
                info.size = 1
                tf.addfile(info, buf)
        with pytest.raises(ArchiveError, match="members"):
            list_archive(str(arc))

    def test_too_many_members_7z(self, tmp_path, monkeypatch):
        monkeypatch.setattr(sa, "MAX_MEMBERS", 2)
        arc = tmp_path / "many.7z"
        with py7zr.SevenZipFile(str(arc), mode="w") as szf:
            for i in range(4):
                szf.writef(io.BytesIO(b"x"), f"f{i}.txt")
        with pytest.raises(ArchiveError, match="members"):
            list_archive(str(arc))

    def test_single_file_too_large_announced_refused_early_7z(self, tmp_path, monkeypatch):
        """read_7z refuses on the announced size BEFORE szf.extract writes the bomb to disk.

        Mirrors test_single_file_too_large_announced_refused_early (zip). Sabotages extract() so
        the test fails loudly if the cap is enforced post-extraction (the original 7z bug).
        """
        monkeypatch.setattr(sa, "MAX_FILE_BYTES", 5)
        arc = tmp_path / "big.7z"
        with py7zr.SevenZipFile(str(arc), mode="w") as szf:
            szf.writef(io.BytesIO(b"x" * 100), "big.txt")

        def boom_extract(self, *a, **k):
            raise AssertionError("extract() must NOT run — the cap must fire before extraction")

        monkeypatch.setattr(py7zr.SevenZipFile, "extract", boom_extract)
        with pytest.raises(ArchiveError, match="announced"):
            read_archive_member(str(arc), "big.txt")


# ---------------------------------------------------------------------------
# 7. Name validation — NUL byte, absolute path, long name
# ---------------------------------------------------------------------------

class TestNameValidation:
    def test_nul_byte_raises(self):
        with pytest.raises(ArchiveError, match="null byte"):
            _validate_name("file\x00.txt")

    def test_absolute_path_raises(self):
        with pytest.raises(ArchiveError, match="absolute"):
            _validate_name("/etc/passwd")

    def test_dotdot_raises(self):
        with pytest.raises(ArchiveError, match=r"\.\."):
            _validate_name("../outside.txt")

    def test_nested_dotdot_raises(self):
        with pytest.raises(ArchiveError, match=r"\.\."):
            _validate_name("a/b/../../etc/passwd")

    def test_name_too_long_raises(self):
        with pytest.raises(ArchiveError, match="too long"):
            _validate_name("a" * 256)

    def test_name_exactly_at_limit_ok(self):
        name = "a" * 255
        assert _validate_name(name) == name

    def test_nfc_normalization_applied(self):
        # Compose: e + combining acute = é (NFC)
        decomposed = "café"   # 'e' + combining acute accent
        composed = "café"           # é precomposed
        result = _validate_name(decomposed)
        assert result == composed

    def test_read_zip_with_nul_name(self, tmp_path):
        arc = tmp_path / "nul.zip"
        with zipfile.ZipFile(arc, "w") as zf:
            zf.writestr("safe.txt", b"ok")
        with pytest.raises(ArchiveError, match="null byte"):
            read_archive_member(str(arc), "safe\x00.txt")

    def test_read_zip_with_absolute_name(self, tmp_path):
        arc = tmp_path / "abs.zip"
        with zipfile.ZipFile(arc, "w") as zf:
            zf.writestr("safe.txt", b"ok")
        with pytest.raises(ArchiveError, match="absolute"):
            read_archive_member(str(arc), "/etc/passwd")

    def test_read_tar_with_nul_name(self, tmp_path):
        arc = tmp_path / "nul.tar"
        with tarfile.open(str(arc), "w") as tf:
            buf = io.BytesIO(b"ok")
            info = tarfile.TarInfo(name="safe.txt")
            info.size = 2
            tf.addfile(info, buf)
        with pytest.raises(ArchiveError, match="null byte"):
            read_archive_member(str(arc), "safe\x00.txt")
