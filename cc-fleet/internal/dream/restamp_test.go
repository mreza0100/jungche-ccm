package dream

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRestampUpdatesMovedAnchorsMechanically(t *testing.T) {
	repository := hookRepository(t)
	organRoot := filepath.Join(repository, ".professor", "stm")
	firstHash := hookGit(t, repository, "rev-parse", "HEAD:anchor.txt")[:12]
	secondHash := hookGit(t, repository, "rev-parse", "HEAD:stable.txt")[:12]
	mapPath := filepath.Join(organRoot, "maps", "subject.md")
	writeHookFile(t, mapPath, hookMap(firstHash, secondHash))

	writeHookFile(t, filepath.Join(repository, "anchor.txt"), "moved\n")
	hookGit(t, repository, "add", "anchor.txt")
	hookGit(t, repository, "commit", "-m", "move anchor")
	movedHash := hookGit(t, repository, "rev-parse", "HEAD:anchor.txt")[:12]

	// Every spelling an agent might copy from the surface resolves to the map.
	for _, spelling := range []string{"maps/subject.md", "subject.md", "subject", mapPath} {
		if _, err := resolveRestampTarget(spelling, organRoot); err != nil {
			t.Fatalf("resolveRestampTarget(%q) error = %v", spelling, err)
		}
	}

	output, err := Restamp("maps/subject.md", repository, time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Restamp() error = %v", err)
	}
	if !strings.Contains(output, "row(s) updated at HEAD") {
		t.Fatalf("Restamp output = %q", output)
	}
	body, err := os.ReadFile(mapPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), movedHash) || strings.Contains(string(body), firstHash) {
		t.Fatalf("restamped map still carries the stale hash: %s", body)
	}
	if !strings.Contains(string(body), "Provenance: 2026-08-14 · sid") {
		t.Fatalf("restamp did not re-date Provenance: %s", body)
	}

	// A second run is a no-op and says so — it never reports silence.
	again, err := Restamp("maps/subject.md", repository, time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Restamp(again) error = %v", err)
	}
	if !strings.Contains(again, "already current") {
		t.Fatalf("second Restamp output = %q", again)
	}
}

func TestRestampFailsLoudOnVanishedAnchorAndForeignPath(t *testing.T) {
	repository := hookRepository(t)
	organRoot := filepath.Join(repository, ".professor", "stm")
	firstHash := hookGit(t, repository, "rev-parse", "HEAD:anchor.txt")[:12]
	writeHookFile(t, filepath.Join(organRoot, "maps", "gone.md"),
		strings.Replace(hookMap(firstHash, "0123456789ab"), "`stable.txt`", "`vanished.txt`", 1))

	// A vanished anchor path is a judgment call (rewrite the map), never a
	// silent restamp.
	if _, err := Restamp("maps/gone.md", repository, time.Now()); err == nil ||
		!strings.Contains(err.Error(), "vanished.txt") {
		t.Fatalf("Restamp over a vanished anchor = %v", err)
	}

	// Restamp never writes outside this organ's maps directory.
	if _, err := Restamp("/etc/passwd", repository, time.Now()); err == nil {
		t.Fatal("Restamp accepted a path outside the organ")
	}
	if _, err := Restamp("../stm.md", repository, time.Now()); err == nil {
		t.Fatal("Restamp accepted a traversal name")
	}
}
