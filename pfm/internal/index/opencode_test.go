package index

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"hostops/pfm/internal/paths"
	"hostops/pfm/internal/store"

	_ "modernc.org/sqlite"
)

// seedOpencodeStore builds the native OpenCode store shape: user prompts live
// in message/part JSON rows, never in PFM's optional session_input queue.
func seedOpencodeStore(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir opencode root: %v", err)
	}
	db, err := sql.Open("sqlite", filepath.Join(root, "opencode.db"))
	if err != nil {
		t.Fatalf("open fixture store: %v", err)
	}
	defer db.Close()
	script := `
CREATE TABLE project (id TEXT PRIMARY KEY, worktree TEXT NOT NULL);
CREATE TABLE session (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL,
  parent_id TEXT,
  directory TEXT NOT NULL,
  title TEXT NOT NULL,
  agent TEXT NOT NULL DEFAULT '',
  model TEXT NOT NULL DEFAULT '',
  tokens_input INTEGER NOT NULL DEFAULT 0,
  tokens_output INTEGER NOT NULL DEFAULT 0,
  cost REAL NOT NULL DEFAULT 0,
  time_created INTEGER NOT NULL,
  time_updated INTEGER NOT NULL,
  time_archived INTEGER
);
CREATE TABLE message (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL,
  time_created INTEGER NOT NULL,
  time_updated INTEGER NOT NULL,
  data TEXT NOT NULL
);
CREATE TABLE part (
  id TEXT PRIMARY KEY,
  message_id TEXT NOT NULL,
  session_id TEXT NOT NULL,
  time_created INTEGER NOT NULL,
  time_updated INTEGER NOT NULL,
  data TEXT NOT NULL
);
CREATE INDEX message_session_time_created_id_idx ON message (session_id, time_created, id);
CREATE INDEX part_message_id_id_idx ON part (message_id, id);
CREATE INDEX part_session_idx ON part (session_id);`
	if _, err := db.Exec(script); err != nil {
		t.Fatalf("seed schema: %v", err)
	}
	if _, err := db.Exec("INSERT INTO project (id, worktree) VALUES ('p1', '/work/nuts')"); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	sessions := []struct {
		id     string
		parent any
		title  string
		model  string
		cost   float64
		arch   any
	}{
		{"ses_root", nil, "Ramsey attack", `{"id":"m1","providerID":"prov"}`, 1.5, nil},
		{"ses_child", "ses_root", "subagent", "", 0, nil},
		{"ses_gone", nil, "old", "", 0, 500},
	}
	for _, session := range sessions {
		if _, err := db.Exec(
			"INSERT INTO session (id, project_id, parent_id, directory, title, agent, model, tokens_input, tokens_output, cost, time_created, time_updated, time_archived) VALUES (?, 'p1', ?, '/work/nuts/03-ramsey', ?, 'build', ?, 1, 1, ?, 10, 20, ?)",
			session.id, session.parent, session.title, session.model, session.cost, session.arch,
		); err != nil {
			t.Fatalf("seed session %s: %v", session.id, err)
		}
	}
	prompts := [][5]any{
		{"m1", "i1", "ses_root", "prove the bound", 100},
		{"m2", "i2", "ses_root", "now generalize", 200},
		{"m3", "i3", "ses_child", "child probe", 150},
	}
	for _, p := range prompts {
		if _, err := db.Exec(
			`INSERT INTO message (id, session_id, time_created, time_updated, data)
			 VALUES (?, ?, ?, ?, '{"role":"user"}')`,
			p[0], p[2], p[4], p[4],
		); err != nil {
			t.Fatalf("seed message %v: %v", p[0], err)
		}
		if _, err := db.Exec(
			`INSERT INTO part (id, message_id, session_id, time_created, time_updated, data)
			 VALUES (?, ?, ?, ?, ?, json_object('type','text','text',?))`,
			p[1], p[0], p[2], p[4], p[4], p[3],
		); err != nil {
			t.Fatalf("seed part %v: %v", p[1], err)
		}
	}
}

func TestReadOpencodeSessionsReadsTheFixtureStore(t *testing.T) {
	root := t.TempDir()
	seedOpencodeStore(t, root)

	sessions, err := ReadOpencodeSessions(context.Background(), root)
	if err != nil {
		t.Fatalf("read sessions: %v", err)
	}
	byID := make(map[string]int, len(sessions))
	for index, session := range sessions {
		byID[session.ID] = index
	}
	rootSession, found := byID["ses_root"]
	if !found {
		t.Fatalf("root session missing from %#v", sessions)
	}
	got := sessions[rootSession]
	want := struct {
		title       string
		projectDir  string
		model       string
		firstPrompt string
		promptCount int64
		costMilli   int64
	}{"Ramsey attack", "/work/nuts", "prov/m1", "prove the bound", 2, 1500}
	switch {
	case got.Title != want.title:
		t.Errorf("title = %q, want %q", got.Title, want.title)
	case got.ProjectDir != want.projectDir:
		t.Errorf("project dir = %q, want %q", got.ProjectDir, want.projectDir)
	case got.Model != want.model:
		t.Errorf("model = %q, want %q", got.Model, want.model)
	case got.FirstPrompt != want.firstPrompt:
		t.Errorf("first prompt = %q, want %q", got.FirstPrompt, want.firstPrompt)
	case got.PromptCount != want.promptCount:
		t.Errorf("prompt count = %d, want %d", got.PromptCount, want.promptCount)
	case got.CostMillicents != want.costMilli:
		t.Errorf("cost millicents = %d, want %d", got.CostMillicents, want.costMilli)
	}
	child, found := byID["ses_child"]
	if !found || sessions[child].ParentID != "ses_root" {
		t.Errorf("child session lost or unparented: %#v", sessions)
	}
}

func TestReadOpencodeSessionsCompactsNativeModelID(t *testing.T) {
	root := t.TempDir()
	seedOpencodeStore(t, root)

	db, err := sql.Open("sqlite", filepath.Join(root, "opencode.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE session SET model = '{"providerID":"openai","modelID":"gpt-5.6-sol"}' WHERE id = 'ses_root'`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	sessions, err := ReadOpencodeSessions(context.Background(), root)
	if err != nil {
		t.Fatalf("read sessions: %v", err)
	}
	for _, session := range sessions {
		if session.ID == "ses_root" {
			if session.Model != "openai/gpt-5.6-sol" {
				t.Fatalf("model = %q, want native provider/modelID", session.Model)
			}
			return
		}
	}
	t.Fatal("native model session missing")
}

func TestReadOpencodeSessionsMissingStoreIsQuietlyEmpty(t *testing.T) {
	sessions, err := ReadOpencodeSessions(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("missing store must not error: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("sessions = %#v, want empty", sessions)
	}
}

func openMirrorStore(t *testing.T) *store.Store {
	t.Helper()
	root := t.TempDir()
	t.Setenv(paths.EnvDB, filepath.Join(root, "fleet.db"))
	t.Setenv(paths.EnvSharedDB, filepath.Join(root, "shared.db"))
	database, err := store.Open()
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	return database
}

func TestSyncOpencodeMirrorReplacesAndDeletes(t *testing.T) {
	database := openMirrorStore(t)
	ctx := context.Background()
	root := t.TempDir()
	seedOpencodeStore(t, root)

	var counters Counters
	if err := syncOpencodeMirror(ctx, database, root, &counters); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	if counters.OcSessions != 3 {
		t.Fatalf("OcSessions = %d, want 3", counters.OcSessions)
	}
	stored, err := database.OcSessions(ctx)
	if err != nil {
		t.Fatalf("read mirror: %v", err)
	}

	// The source loses ses_child entirely; the next pass must drop it.
	db, err := sql.Open("sqlite", filepath.Join(root, "opencode.db"))
	if err != nil {
		t.Fatalf("reopen fixture: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec("DELETE FROM session WHERE id = 'ses_child'"); err != nil {
		t.Fatalf("delete child: %v", err)
	}
	db.Close()

	counters = Counters{}
	if err := syncOpencodeMirror(ctx, database, root, &counters); err != nil {
		t.Fatalf("second sync: %v", err)
	}
	stored, err = database.OcSessions(ctx)
	if err != nil {
		t.Fatalf("reread mirror: %v", err)
	}
	if len(stored) != 2 {
		t.Fatalf("mirror holds %d rows after deletion, want 2: %#v", len(stored), stored)
	}
	for _, row := range stored {
		if row.ID == "ses_child" {
			t.Fatalf("vanished session survived the mirror replace: %#v", stored)
		}
	}
}
