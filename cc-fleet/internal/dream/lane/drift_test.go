package lane

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnnotateDriftUsesLiveWorktreeNotBaseOrganRepository(t *testing.T) {
	repository := newDriftRepository(t)
	maps := filepath.Join(repository.root, "organ", "maps")
	mustMkdir(t, maps)
	mustWrite(t, filepath.Join(maps, "subject.md"), canonicalDriftMap(
		"Subject", "anchor.txt", repository.anchorHash, "stable.txt", repository.stableHash,
	))
	surface := "- Subject -> maps/subject.md\n"

	base, err := AnnotateDrift(repository.base, maps, surface)
	if err != nil {
		t.Fatalf("AnnotateDrift(base) error = %v", err)
	}
	if base.Surface != surface || base.DriftedMaps != 0 || base.Fallback {
		t.Fatalf("base result = %#v", base)
	}

	worktree, err := AnnotateDrift(repository.worktree, maps, surface)
	if err != nil {
		t.Fatalf("AnnotateDrift(worktree) error = %v", err)
	}
	want := "- Subject -> maps/subject.md ⚠ DRIFTED (1/2 anchors moved: anchor.txt)\n"
	if worktree.Surface != want || worktree.DriftedMaps != 1 || worktree.Fallback {
		t.Fatalf("worktree result = %#v, want surface %q", worktree, want)
	}
}

func TestAnnotateDriftTreatsMissingAnchorAsMoved(t *testing.T) {
	repository := newDriftRepository(t)
	maps := t.TempDir()
	mustWrite(t, filepath.Join(maps, "missing.md"), canonicalDriftMap(
		"Missing", "absent.txt", "0123456789ab", "stable.txt", repository.stableHash,
	))
	surface := "- Missing -> maps/missing.md"
	result, err := AnnotateDrift(repository.base, maps, surface)
	if err != nil {
		t.Fatalf("AnnotateDrift() error = %v", err)
	}
	want := surface + " ⚠ DRIFTED (1/2 anchors moved: absent.txt)"
	if result.Surface != want {
		t.Fatalf("Surface = %q, want %q", result.Surface, want)
	}
}

func TestAnnotateDriftFallsBackByteExactOnAnyMalformedMap(t *testing.T) {
	repository := newDriftRepository(t)
	maps := t.TempDir()
	// The referenced map is drifted in the worktree. An unrelated malformed map
	// must suppress every annotation rather than leak a partial result.
	mustWrite(t, filepath.Join(maps, "a-subject.md"), canonicalDriftMap(
		"Subject", "anchor.txt", repository.anchorHash, "stable.txt", repository.stableHash,
	))
	mustWrite(t, filepath.Join(maps, "z-malformed.md"), "# Malformed\n\n## Question\n\nquestion\n")
	original := "- Subject -> maps/a-subject.md\n"
	result, err := AnnotateDrift(repository.worktree, maps, original)
	if err == nil {
		t.Fatal("AnnotateDrift() error = nil")
	}
	if !result.Fallback || result.Surface != original || result.DriftedMaps != 0 {
		t.Fatalf("fallback result = %#v, original = %q", result, original)
	}
	assertErrorContains(t, err, "parse drift map")
}

func TestAnnotateDriftFallsBackByteExactOnMissingAnchorsAndGitFailure(t *testing.T) {
	repository := newDriftRepository(t)
	surface := "- Subject -> maps/subject.md\n"

	t.Run("missing anchors", func(t *testing.T) {
		maps := t.TempDir()
		body := strings.Split(canonicalDriftMap(
			"Subject", "anchor.txt", repository.anchorHash, "stable.txt", repository.stableHash,
		), "## Anchors")[0] + "## Anchors\n"
		mustWrite(t, filepath.Join(maps, "subject.md"), body)
		result, err := AnnotateDrift(repository.base, maps, surface)
		if err == nil || !result.Fallback || result.Surface != surface {
			t.Fatalf("AnnotateDrift(missing anchors) = %#v, %v", result, err)
		}
		assertErrorContains(t, err, "missing anchors")
	})

	t.Run("git mechanism", func(t *testing.T) {
		maps := t.TempDir()
		mustWrite(t, filepath.Join(maps, "subject.md"), canonicalDriftMap(
			"Subject", "anchor.txt", repository.anchorHash, "stable.txt", repository.stableHash,
		))
		result, err := AnnotateDrift(t.TempDir(), maps, surface)
		if err == nil || !result.Fallback || result.Surface != surface {
			t.Fatalf("AnnotateDrift(git failure) = %#v, %v", result, err)
		}
		assertErrorContains(t, err, "verify live worktree Git HEAD")
	})
}

func TestAnnotateDriftAcceptsLegacyMapEnvelopeAndCommaRanges(t *testing.T) {
	repository := newDriftRepository(t)
	maps := t.TempDir()
	body := "# Legacy\n\n## Load-bearing surface\n\nOld envelope.\n\n## Anchors\n\n" +
		"- `anchor.txt:1-2,4-5` — blob `" + repository.anchorHash + "`\n" +
		"- `stable.txt:1` — blob `" + repository.stableHash + "`\n"
	mustWrite(t, filepath.Join(maps, "legacy.md"), body)
	result, err := AnnotateDrift(repository.worktree, maps, "- Legacy -> maps/legacy.md\n")
	if err != nil {
		t.Fatal(err)
	}
	want := "- Legacy -> maps/legacy.md ⚠ DRIFTED (1/2 anchors moved: anchor.txt)\n"
	if result.Surface != want {
		t.Fatalf("legacy drift surface = %q, want %q", result.Surface, want)
	}
}

func TestAnnotateDriftFallsBackOnSurfacePointerWithoutMap(t *testing.T) {
	repository := newDriftRepository(t)
	original := "- Ghost -> maps/ghost.md\n"
	result, err := AnnotateDrift(repository.base, t.TempDir(), original)
	if err == nil || !result.Fallback || result.Surface != original {
		t.Fatalf("AnnotateDrift() = %#v, %v", result, err)
	}
	assertErrorContains(t, err, "surface points to missing map")
}

type driftRepository struct {
	root       string
	base       string
	worktree   string
	anchorHash string
	stableHash string
}

func newDriftRepository(t *testing.T) driftRepository {
	t.Helper()
	root := t.TempDir()
	base := filepath.Join(root, "base")
	worktree := filepath.Join(root, "worktree")
	mustMkdir(t, base)
	gitRun(t, root, "init", "-b", "main", base)
	mustWrite(t, filepath.Join(base, "anchor.txt"), "base anchor\n")
	mustWrite(t, filepath.Join(base, "stable.txt"), "stable\n")
	gitRun(t, base, "add", "anchor.txt", "stable.txt")
	gitRun(t, base, "commit", "-m", "base")
	anchorHash := gitRun(t, base, "rev-parse", "HEAD:anchor.txt")
	stableHash := gitRun(t, base, "rev-parse", "HEAD:stable.txt")
	gitRun(t, base, "worktree", "add", "-b", "live", worktree)
	mustWrite(t, filepath.Join(worktree, "anchor.txt"), "live worktree anchor\n")
	gitRun(t, worktree, "add", "anchor.txt")
	gitRun(t, worktree, "commit", "-m", "live")
	return driftRepository{
		root: root, base: base, worktree: worktree,
		anchorHash: anchorHash[:12], stableHash: stableHash[:12],
	}
}

func gitRun(t *testing.T, directory string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", arguments...)
	command.Dir = directory
	command.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Dream Test", "GIT_AUTHOR_EMAIL=dream@example.invalid",
		"GIT_COMMITTER_NAME=Dream Test", "GIT_COMMITTER_EMAIL=dream@example.invalid",
		"GIT_CONFIG_NOSYSTEM=1", "GIT_OPTIONAL_LOCKS=0",
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(arguments, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func canonicalDriftMap(title, firstPath, firstHash, secondPath, secondHash string) string {
	return "# " + title + "\n" +
		"\n## Question\n\nWhat moved?\n" +
		"\n## Answer\n\nThe worktree decides.\n" +
		"\n## Derivation trail\n\nA hermetic repository proved it.\n" +
		"\nProvenance: 2026-08-13 · sid 0123abcd\n" +
		"\n## Anchors\n\n" +
		"- `" + firstPath + "` — blob `" + firstHash + "`\n" +
		"- `" + secondPath + "` — blob `" + secondHash + "`\n"
}

func TestAnnotateDriftSkipsAgentSurfaceHeaderRow(t *testing.T) {
	repository := newDriftRepository(t)
	maps := filepath.Join(repository.root, "organ", "maps")
	mustMkdir(t, maps)
	mustWrite(t, filepath.Join(maps, "subject.md"), canonicalDriftMap(
		"Subject", "anchor.txt", repository.anchorHash, "stable.txt", repository.stableHash,
	))
	// The live shape: the dreamer writes AgentSurfaceHeader as line 1 of every
	// agents/{lane}.md. It carries no map pointer and must not abort annotation.
	surface := AgentSurfaceHeader + "\n- Subject -> maps/subject.md\n"

	result, err := AnnotateDrift(repository.worktree, maps, surface)
	if err != nil {
		t.Fatalf("AnnotateDrift() error = %v", err)
	}
	want := AgentSurfaceHeader + "\n- Subject -> maps/subject.md ⚠ DRIFTED (1/2 anchors moved: anchor.txt)\n"
	if result.Surface != want || result.DriftedMaps != 1 || result.Fallback {
		t.Fatalf("result = %#v, want surface %q", result, want)
	}
}
