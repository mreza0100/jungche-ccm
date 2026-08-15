package legacy

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"hostops/pfm/internal/paths"
	"hostops/pfm/internal/store"
)

func TestImportHidesEveryKnownIDAndLeavesSourcesUntouched(t *testing.T) {
	root, database := legacyJail(t)
	ctx := context.Background()
	claudeDir := filepath.Join(root, "claude", "project")
	if err := os.MkdirAll(claudeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	first := `{"type":"user","message":{"content":"first"}}` + "\n"
	realDelta := `{"type":"user","message":{"content":"new prompt"}}` + "\n"
	noiseDelta := `{"type":"assistant","message":{"content":"noise"}}` + "\n"
	activePath := filepath.Join(claudeDir, "active.jsonl")
	quietPath := filepath.Join(claudeDir, "quiet.jsonl")
	if err := os.WriteFile(activePath, []byte(first+realDelta), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(quietPath, []byte(first+noiseDelta), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, transcript := range []store.Transcript{
		{UUID: "active", Path: activePath, Size: int64(len(first + realDelta)), PromptCount: 2},
		{UUID: "quiet", Path: quietPath, Size: int64(len(first + noiseDelta)), PromptCount: 1},
	} {
		if err := database.UpsertTranscript(ctx, transcript); err != nil {
			t.Fatal(err)
		}
	}

	hiddenPath := filepath.Join(root, "home", ".claude", ".cc-ls-hidden")
	if err := os.MkdirAll(filepath.Dir(hiddenPath), 0o700); err != nil {
		t.Fatal(err)
	}
	hiddenContent := []byte("quiet\n\nactive\nunknown\n")
	baselineContent := []byte("active\t" + strconv.Itoa(len(first)) +
		"\n\nquiet\t" + strconv.Itoa(len(first)) + "\n")
	if err := os.WriteFile(hiddenPath, hiddenContent, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hiddenPath+".at", baselineContent, 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := Import(ctx, database)
	if err != nil {
		t.Fatal(err)
	}
	// All three ids import — the union is the contract — and the one no index
	// row claims is reported as unknown so `hidden --prune-orphans` can offer
	// to clear it.
	if result.Imported != 3 || result.Unknown != 1 {
		t.Fatalf("Import() = %#v", result)
	}
	rows, err := database.HiddenChats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// "active" grew past its legacy byte baseline and still imports hidden.
	if len(rows) != 3 ||
		rows[0].ID != "active" || rows[0].BaselinePrompts != nil ||
		rows[1].ID != "quiet" || rows[1].BaselinePrompts != nil ||
		rows[2].ID != "unknown" || rows[2].Engine != "" {
		t.Fatalf("hidden rows = %#v", rows)
	}
	for path, want := range map[string][]byte{
		hiddenPath:         hiddenContent,
		hiddenPath + ".at": baselineContent,
	} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != string(want) {
			t.Fatalf("legacy source %s changed: %q err=%v", path, got, err)
		}
	}
}

func TestExportWritesSortedHideListAndNoByteBaselines(t *testing.T) {
	root, database := legacyJail(t)
	ctx := context.Background()
	if err := database.UpsertTranscript(ctx, store.Transcript{
		UUID: "zeta",
		Path: "/jail/zeta.jsonl",
		Size: 123,
	}); err != nil {
		t.Fatal(err)
	}
	if err := database.UpsertRollout(ctx, store.Rollout{
		ID:   "alpha",
		Path: "/jail/alpha.jsonl",
		Size: 456,
	}); err != nil {
		t.Fatal(err)
	}
	for _, hidden := range []store.Hidden{
		{ID: "zeta", Engine: "cc"},
		{ID: "alpha", Engine: "cx"},
	} {
		if err := database.Hide(ctx, hidden); err != nil {
			t.Fatal(err)
		}
	}
	hiddenPath := filepath.Join(root, "home", ".claude", ".cc-ls-hidden")
	if err := os.MkdirAll(filepath.Dir(hiddenPath), 0o700); err != nil {
		t.Fatal(err)
	}
	// A stale .at from before the ratchet was retired must be neither
	// rewritten nor removed: nothing reads it, so Export ignores it.
	stale := []byte("gamma\t7\n")
	if err := os.WriteFile(hiddenPath+".at", stale, 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := Export(ctx, database)
	if err != nil {
		t.Fatal(err)
	}
	if result.Exported != 2 {
		t.Fatalf("Export() = %#v", result)
	}
	assertLegacyContent(t, hiddenPath, "alpha\nzeta\n")
	assertLegacyContent(t, hiddenPath+".at", string(stale))
}

func TestExportNeverCreatesTheByteBaselineFile(t *testing.T) {
	root, database := legacyJail(t)
	ctx := context.Background()
	if err := database.UpsertTranscript(ctx, store.Transcript{
		UUID: "zeta",
		Path: "/jail/zeta.jsonl",
		Size: 123,
	}); err != nil {
		t.Fatal(err)
	}
	if err := database.Hide(ctx, store.Hidden{
		ID:     "zeta",
		Engine: "cc",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := Export(ctx, database); err != nil {
		t.Fatal(err)
	}
	baselinePath := filepath.Join(
		root,
		"home",
		".claude",
		".cc-ls-hidden.at",
	)
	if _, err := os.Stat(baselinePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Export wrote the retired %s: err=%v", baselinePath, err)
	}
}

func TestImportCodexChildHideCanonicalizesToLineageRoot(t *testing.T) {
	root, database := legacyJail(t)
	ctx := context.Background()
	for _, rollout := range []store.Rollout{
		{
			ID:          "root-thread",
			Path:        filepath.Join(root, "codex", "root.jsonl"),
			Size:        100,
			MTimeNS:     100,
			UserThread:  true,
			SessionID:   "root-thread",
			LineageRoot: "root-thread",
			PromptCount: 7,
			FirstPrompt: "WEB",
		},
		{
			ID:           "child-rollout",
			Path:         filepath.Join(root, "codex", "child.jsonl"),
			Size:         120,
			MTimeNS:      200,
			UserThread:   true,
			SessionID:    "root-thread",
			ParentThread: "root-thread",
			LineageRoot:  "root-thread",
			PromptCount:  7,
			FirstPrompt:  "WEB",
		},
	} {
		if err := database.UpsertRollout(ctx, rollout); err != nil {
			t.Fatal(err)
		}
	}
	hiddenPath := filepath.Join(root, "home", ".claude", ".cc-ls-hidden")
	if err := os.MkdirAll(filepath.Dir(hiddenPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hiddenPath, []byte("child-rollout\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := Import(ctx, database)
	if err != nil {
		t.Fatal(err)
	}
	if result.Imported != 1 {
		t.Fatalf("Import() = %#v", result)
	}
	// The root is what the picker hides and unhides, so the child id
	// canonicalizes to it. Both stay hidden: dropping the child id would be an
	// unhide, and an import never unhides.
	hidden, err := database.HiddenChats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(hidden) != 2 ||
		hidden[0].ID != "child-rollout" ||
		hidden[1].ID != "root-thread" ||
		hidden[1].BaselinePrompts != nil {
		t.Fatalf("lineage import hides = %#v", hidden)
	}
	// The canonicalized root is appended beside the child it came from: every
	// writer of the shared table writes the carrier file too.
	assertLegacyContent(t, hiddenPath, "child-rollout\nroot-thread\n")
}

func legacyJail(t *testing.T) (string, *store.Store) {
	t.Helper()
	root := t.TempDir()
	for _, directory := range []string{
		filepath.Join(root, "home"),
		filepath.Join(root, "claude"),
		filepath.Join(root, "codex"),
		filepath.Join(root, "sid"),
		filepath.Join(root, "tmux"),
		filepath.Join(root, "proc"),
	} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(paths.EnvDB, filepath.Join(root, "fleet.db"))
	t.Setenv(paths.EnvHome, filepath.Join(root, "home"))
	t.Setenv(paths.EnvClaudeRoots, filepath.Join(root, "claude"))
	t.Setenv(paths.EnvCodexRoot, filepath.Join(root, "codex"))
	t.Setenv(paths.EnvSIDDir, filepath.Join(root, "sid"))
	t.Setenv(paths.EnvTmuxDir, filepath.Join(root, "tmux"))
	t.Setenv(paths.EnvProcRoot, filepath.Join(root, "proc"))
	database, err := store.Open()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return root, database
}

func assertLegacyContent(t *testing.T, path, want string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != want {
		t.Fatalf("%s = %q, want %q", path, content, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("%s mode = %o", path, info.Mode().Perm())
	}
}
