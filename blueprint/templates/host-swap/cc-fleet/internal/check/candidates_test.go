package check

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCodexCandidateIDsReproducesNewestFileBound(t *testing.T) {
	root := t.TempDir()
	sessions := filepath.Join(root, "sessions", "2026", "01", "01")
	if err := os.MkdirAll(sessions, 0o700); err != nil {
		t.Fatal(err)
	}
	type fixture struct {
		id    string
		age   time.Duration
		bytes string
	}
	fixtures := []fixture{
		{id: "old-user", age: 3 * time.Hour, bytes: `{"thread_source":"user"}`},
		{id: "new-subagent", age: time.Hour, bytes: `{"thread_source":"subagent"}`},
		{id: "new-user", age: 2 * time.Hour, bytes: `{"thread_source":"user"}`},
		{id: "new-schema", age: 30 * time.Minute, bytes: `{"source":"vscode"}`},
	}
	now := time.Now()
	for _, fixture := range fixtures {
		path := filepath.Join(
			sessions,
			"rollout-2026-01-01T00-00-00-"+fixture.id+".jsonl",
		)
		if err := os.WriteFile(path, []byte(fixture.bytes+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		mtime := now.Add(-fixture.age)
		if err := os.Chtimes(path, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	ids, err := CodexCandidateIDs(root, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 3 {
		t.Fatalf("candidate IDs = %#v", ids)
	}
	if _, found := ids["new-subagent"]; !found {
		t.Fatal("subagent file was incorrectly removed before the legacy bound")
	}
	if _, found := ids["new-user"]; !found {
		t.Fatal("new user file missing")
	}
	if _, found := ids["old-user"]; found {
		t.Fatal("old file survived newest-two bound")
	}
	unrecognized, err := CodexLegacyUnrecognizedIDs(root, 3)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := unrecognized["new-schema"]; !found {
		t.Fatal("new interactive source schema was not verified as legacy-unrecognized")
	}
	if _, found := unrecognized["new-user"]; found {
		t.Fatal("legacy-recognized user source was marked unrecognized")
	}

	// The unbounded set is what tells a store-only thread from one the legacy
	// scan simply did not reach: every file on disk, no newest-N cut.
	files, err := CodexRolloutFileIDs(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != len(fixtures) {
		t.Fatalf("rollout file IDs = %#v, want one per fixture", files)
	}
	if _, found := files["old-user"]; !found {
		t.Fatal("the newest-N bound leaked into the unbounded rollout file set")
	}
	if _, found := files["never-written"]; found {
		t.Fatal("a thread with no rollout file was reported as having one")
	}
}

func TestClaudeTranscriptIDsMirrorTheLegacyStoreScan(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "account-1")
	second := filepath.Join(root, "account-2")
	project := filepath.Join(first, "-home-user-work-projc")
	for _, directory := range []string{project, second} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for path, content := range map[string]string{
		filepath.Join(project, "in-project.jsonl"):   "{}\n",
		filepath.Join(first, "at-root.jsonl"):        "{}\n",
		filepath.Join(second, "other-account.jsonl"): "{}\n",
		// Not a transcript: the legacy scan takes *.jsonl only.
		filepath.Join(project, "notes.txt"): "ignored\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// Depth 3 is past `-maxdepth 2`, so the legacy scan never sees it either.
	deep := filepath.Join(project, "nested")
	if err := os.MkdirAll(deep, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(deep, "too-deep.jsonl"),
		[]byte("{}\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	ids, err := ClaudeTranscriptIDs([]string{
		first,
		second,
		filepath.Join(root, "absent"),
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"in-project", "at-root", "other-account"}
	if len(ids) != len(want) {
		t.Fatalf("transcript IDs = %#v, want %v", ids, want)
	}
	for _, id := range want {
		if _, found := ids[id]; !found {
			t.Fatalf("transcript IDs missing %q: %#v", id, ids)
		}
	}
	for _, id := range []string{"too-deep", "notes"} {
		if _, found := ids[id]; found {
			t.Fatalf("transcript IDs picked up %q: %#v", id, ids)
		}
	}
}

func TestLegacyCacheEmptyCWDIDsReadsTheStaleRowsOnly(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "fleet.db")
	database, err := sql.Open(legacyCacheDriver, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
CREATE TABLE chat(
  uuid    TEXT PRIMARY KEY,
  mtime   INTEGER NOT NULL DEFAULT 0,
  bytes   INTEGER NOT NULL DEFAULT 0,
  cwd     TEXT,
  label   TEXT,
  prompts INTEGER NOT NULL DEFAULT 0
);
INSERT INTO chat(uuid, cwd) VALUES
  ('blank', ''),
  ('null', NULL),
  ('named', '/home/user/work/projc');
`); err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	ids, err := LegacyCacheEmptyCWDIDs(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 {
		t.Fatalf("empty-cwd IDs = %#v, want blank and null", ids)
	}
	for _, want := range []string{"blank", "null"} {
		if _, found := ids[want]; !found {
			t.Fatalf("empty-cwd IDs missing %q: %#v", want, ids)
		}
	}
	if _, found := ids["named"]; found {
		t.Fatal("a cached row with a real directory was reported stale")
	}

	// A box with no legacy cache has no stale rows, not a broken check.
	absent, err := LegacyCacheEmptyCWDIDs(
		context.Background(),
		filepath.Join(root, "absent.db"),
	)
	if err != nil || len(absent) != 0 {
		t.Fatalf("missing cache = %#v, err = %v", absent, err)
	}

	// So does a store that predates the chat table.
	older := filepath.Join(root, "older.db")
	legacy, err := sql.Open(legacyCacheDriver, older)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.Exec(
		"CREATE TABLE hidden(uuid TEXT PRIMARY KEY);",
	); err != nil {
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}
	tableless, err := LegacyCacheEmptyCWDIDs(context.Background(), older)
	if err != nil || len(tableless) != 0 {
		t.Fatalf("chat-less cache = %#v, err = %v", tableless, err)
	}
}
