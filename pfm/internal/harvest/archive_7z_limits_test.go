package harvest

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This is the valid file_and_empty.7z fixture shipped by bodgit/sevenzip.
// Keeping it inline makes the reduced-limit tests independent of a host 7z
// executable and, importantly, exercises the announced-size check before a
// member is opened.
const validSevenZipFixture = "N3q8ryccAAQwP4SyFQAAAAAAAAA4AAAAAAAAAA+CMddIdXV1dWdlIGZpbGUgY29udGVudHMBBAYAAQkVAAcLAQABAQAMFQAABQIOAUAPAYARGQBsAGEAcgBnAGUAAABlAG0AcAB0AHkAAAAAAA=="

func TestSevenZipReducedMemberLimitIsCheckedBeforeRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture.7z")
	body, err := base64.StdEncoding.DecodeString(validSevenZipFixture)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	members, err := ListArchive(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) < 2 {
		t.Fatalf("valid 7z fixture members = %#v", members)
	}
	var target string
	for _, member := range members {
		if !member.IsDir && member.UncompressedSize > 0 {
			target = member.Name
			break
		}
	}
	if target == "" {
		t.Fatalf("valid 7z fixture has no non-empty member: %#v", members)
	}
	old := MaxArchiveFileBytes
	MaxArchiveFileBytes = 5
	t.Cleanup(func() { MaxArchiveFileBytes = old })
	_, err = ReadArchiveMember(path, target)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "file limit") {
		t.Fatalf("ReadArchiveMember with reduced file limit: %v", err)
	}
}

func TestSevenZipReducedMemberCountIsEnforced(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture.7z")
	body, err := base64.StdEncoding.DecodeString(validSevenZipFixture)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	old := MaxArchiveMembers
	MaxArchiveMembers = 1
	t.Cleanup(func() { MaxArchiveMembers = old })
	_, err = ListArchive(path)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "members") {
		t.Fatalf("ListArchive with reduced member limit: %v", err)
	}
}
