package index

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hostops/cc-fleet/internal/compose"
	"hostops/cc-fleet/internal/paths"
	"hostops/cc-fleet/internal/store"
)

type codexStateThread struct {
	ID               string
	RolloutPath      string
	CWD              string
	Title            string
	FirstUserMessage string
	Name             string
	ThreadSource     string
	HistoryMode      string
	CreatedAt        int64
	UpdatedAt        int64
	TokensUsed       int64
	Archived         bool
}

// buildCodexState writes a scratch Codex state store using the real schema
// captured from a live 0.146.1 store.
func buildCodexState(t *testing.T, path string, threads ...codexStateThread) {
	t.Helper()

	schema, err := os.ReadFile(testdataPath(t, "codex-state-schema.sql"))
	if err != nil {
		t.Fatalf("read Codex state schema: %v", err)
	}
	database, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("create scratch Codex state store: %v", err)
	}
	defer database.Close()
	if _, err := database.Exec(string(schema)); err != nil {
		t.Fatalf("apply Codex state schema: %v", err)
	}
	for _, thread := range threads {
		historyMode := thread.HistoryMode
		if historyMode == "" {
			historyMode = "legacy"
		}
		archived := 0
		if thread.Archived {
			archived = 1
		}
		var name any
		if thread.Name != "" {
			name = thread.Name
		}
		if _, err := database.Exec(`
INSERT INTO threads (
  id, rollout_path, created_at, updated_at, source, model_provider, cwd,
  title, sandbox_policy, approval_mode, tokens_used, archived,
  first_user_message, preview, thread_source, recency_at, history_mode, name
) VALUES (?, ?, ?, ?, 'cli', 'openai', ?, ?, 'workspace-write', 'on-request',
  ?, ?, ?, '', ?, ?, ?, ?)`,
			thread.ID,
			thread.RolloutPath,
			thread.CreatedAt,
			thread.UpdatedAt,
			thread.CWD,
			thread.Title,
			thread.TokensUsed,
			archived,
			thread.FirstUserMessage,
			thread.ThreadSource,
			thread.UpdatedAt,
			historyMode,
			name,
		); err != nil {
			t.Fatalf("insert scratch Codex thread %q: %v", thread.ID, err)
		}
	}
}

func execCodexState(t *testing.T, path, statement string, args ...any) {
	t.Helper()

	database, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open scratch Codex state store: %v", err)
	}
	defer database.Close()
	if _, err := database.Exec(statement, args...); err != nil {
		t.Fatalf("update scratch Codex state store: %v", err)
	}
}

type codexStateFixture struct {
	codexRoot       string
	statePath       string
	fileRolloutPath string
}

func setupCodexStateFixture(t *testing.T) codexStateFixture {
	t.Helper()

	root := t.TempDir()
	codexRoot := filepath.Join(root, "codex")
	sessions := filepath.Join(codexRoot, "sessions", "2026", "01", "01")
	if err := os.MkdirAll(sessions, 0o700); err != nil {
		t.Fatal(err)
	}
	fileRollout := filepath.Join(
		sessions,
		"rollout-2026-01-01T00-00-00-file-thread.jsonl",
	)
	writeLines(t, fileRollout,
		`{"type":"session_meta","payload":{"id":"file-thread","thread_source":"user","cwd":"/work/kept"}}`,
		`{"type":"event_msg","payload":{"type":"user_message","message":"kept first prompt"}}`,
		`{"type":"event_msg","payload":{"type":"user_message","message":"kept second prompt"}}`,
	)
	// The rollout file alone looks like a top-level chat; only the state store
	// knows Codex spawned it as a subagent.
	writeLines(t, filepath.Join(
		sessions,
		"rollout-2026-01-01T00-00-02-hidden-subagent.jsonl",
	),
		`{"type":"session_meta","payload":{"id":"hidden-subagent","cwd":"/work/kept"}}`,
		`{"type":"event_msg","payload":{"type":"user_message","message":"delegated work"}}`,
	)
	writeLines(t, filepath.Join(codexRoot, "session_index.jsonl"),
		`{"id":"file-thread","thread_name":"SESSION INDEX NAME"}`,
		`{"id":"legacy-named","thread_name":"ONLY IN SESSION INDEX"}`,
	)

	statePath := filepath.Join(codexRoot, "state_5.sqlite")
	buildCodexState(t, statePath,
		codexStateThread{
			ID:               "file-thread",
			RolloutPath:      fileRollout,
			CWD:              "/work/kept",
			Title:            "kept first prompt",
			FirstUserMessage: "kept first prompt",
			Name:             "STORE NAME",
			ThreadSource:     "user",
			CreatedAt:        100,
			UpdatedAt:        200,
			TokensUsed:       10,
		},
		codexStateThread{
			ID: "store-only",
			RolloutPath: filepath.Join(
				sessions,
				"rollout-2026-01-01T00-00-01-store-only.jsonl",
			),
			CWD:              "/work/paginated",
			FirstUserMessage: "paginated first prompt",
			Name:             "PAGINATED NAME",
			ThreadSource:     "user",
			HistoryMode:      "paginated",
			CreatedAt:        300,
			UpdatedAt:        400,
			TokensUsed:       99,
		},
		codexStateThread{
			ID: "hidden-subagent",
			RolloutPath: filepath.Join(
				sessions,
				"rollout-2026-01-01T00-00-02-hidden-subagent.jsonl",
			),
			CWD:          "/work/kept",
			Title:        "delegated work",
			ThreadSource: "subagent",
			CreatedAt:    150,
			UpdatedAt:    150,
		},
	)

	t.Setenv("TMUX_TMPDIR", filepath.Join(root, "t"))
	t.Setenv(paths.EnvDB, filepath.Join(root, "state", "fleet.db"))
	t.Setenv(paths.EnvSIDDir, filepath.Join(root, "sid"))
	t.Setenv(paths.EnvClaudeRoots, filepath.Join(root, "claude"))
	t.Setenv(paths.EnvCodexRoot, codexRoot)
	t.Setenv(paths.EnvTmuxDir, filepath.Join(root, "tmux"))
	t.Setenv(paths.EnvHome, filepath.Join(root, "home"))

	return codexStateFixture{
		codexRoot:       codexRoot,
		statePath:       statePath,
		fileRolloutPath: fileRollout,
	}
}

func writeLines(t *testing.T, path string, lines ...string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		path,
		[]byte(strings.Join(lines, "\n")+"\n"),
		0o600,
	); err != nil {
		t.Fatalf("write fixture %q: %v", path, err)
	}
}

func codexRows(t *testing.T, database *store.Store) []compose.Row {
	t.Helper()

	ctx := context.Background()
	rollouts, err := database.Rollouts(ctx)
	if err != nil {
		t.Fatalf("Rollouts() error = %v", err)
	}
	names, err := database.CxNames(ctx)
	if err != nil {
		t.Fatalf("CxNames() error = %v", err)
	}
	hidden, err := database.HiddenChats(ctx)
	if err != nil {
		t.Fatalf("HiddenChats() error = %v", err)
	}
	output := compose.Compose(compose.Input{
		Rollouts: rollouts,
		CxNames:  names,
		Hidden:   hidden,
		Options:  compose.Options{View: compose.AllView},
	})
	codex := make([]compose.Row, 0, len(output.Rows))
	for _, row := range output.Rows {
		if row.Kind == compose.ResumeCodex {
			codex = append(codex, row)
		}
	}
	return codex
}

// The Codex state store decides which conversations exist: one that never
// wrote a rollout file is listed exactly once under its store name, one that
// did keeps its file-derived size while the store still names it, and a
// subagent the rollout file cannot self-identify stays out of the list.
func TestCodexStateStoreDrivesListingAndNames(t *testing.T) {
	setupCodexStateFixture(t)
	database := openIndexStore(t)
	t.Cleanup(func() { _ = database.Close() })
	indexer, err := New(database)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	ctx := context.Background()

	counters, err := indexer.Run(ctx, Options{})
	if err != nil {
		t.Fatalf("initial Run() error = %v", err)
	}
	if counters.CodexThreads != 2 || counters.CodexRowsCreated != 1 {
		t.Fatalf(
			"initial counters = %+v, want two listed threads and one store-only row",
			counters,
		)
	}

	storeOnly, found, err := database.Rollout(ctx, "store-only")
	if err != nil || !found {
		t.Fatalf("store-only Rollout() found = %v, error = %v", found, err)
	}
	if storeOnly.Size != 0 ||
		storeOnly.PromptCount != 1 ||
		!storeOnly.UserThread ||
		storeOnly.CWD != "/work/paginated" ||
		storeOnly.LineageRoot != "store-only" ||
		storeOnly.FirstPrompt != "paginated first prompt" ||
		storeOnly.MTimeNS != 400*1_000_000_000 {
		t.Fatalf("store-only row = %#v", storeOnly)
	}
	withFile, found, err := database.Rollout(ctx, "file-thread")
	if err != nil || !found {
		t.Fatalf("file-thread Rollout() found = %v, error = %v", found, err)
	}
	if withFile.Size <= 0 || withFile.PromptCount != 2 || !withFile.UserThread {
		t.Fatalf("file-thread row = %#v, want file-derived size and prompts", withFile)
	}
	subagent, found, err := database.Rollout(ctx, "hidden-subagent")
	if err != nil || !found {
		t.Fatalf("hidden-subagent Rollout() found = %v, error = %v", found, err)
	}
	if subagent.UserThread {
		t.Fatal("the state store classified hidden-subagent as a subagent; it is still listed")
	}

	names, err := database.CxNames(ctx)
	if err != nil {
		t.Fatalf("CxNames() error = %v", err)
	}
	if names["file-thread"] != "STORE NAME" {
		t.Fatalf(
			"file-thread name = %q, want the state store to outrank session_index",
			names["file-thread"],
		)
	}
	if names["store-only"] != "PAGINATED NAME" {
		t.Fatalf("store-only name = %q, want the state store name", names["store-only"])
	}
	if names["legacy-named"] != "ONLY IN SESSION INDEX" {
		t.Fatalf(
			"legacy-named name = %q, want session_index kept as the fallback",
			names["legacy-named"],
		)
	}

	rows := codexRows(t, database)
	if len(rows) != 2 {
		t.Fatalf("composed Codex rows = %#v, want exactly two", rows)
	}
	byID := map[string]compose.Row{rows[0].ID: rows[0], rows[1].ID: rows[1]}
	if row := byID["store-only"]; row.Name != "PAGINATED NAME" ||
		row.Project != "paginated" ||
		row.PromptCount != 1 {
		t.Fatalf("store-only row = %#v", row)
	}
	if row := byID["file-thread"]; row.Name != "STORE NAME" ||
		row.PromptCount != 2 {
		t.Fatalf("file-thread row = %#v", row)
	}

	warm, err := indexer.Run(ctx, Options{})
	if err != nil {
		t.Fatalf("warm Run() error = %v", err)
	}
	if warm.RowsTouched != 0 || warm.Deleted != 0 {
		t.Fatalf("warm counters = %+v, want an idempotent pass", warm)
	}
}

// A rename inside Codex reaches the picker, and archiving a conversation
// there retires the row the store alone was holding up.
func TestCodexStateRenameAndArchivePropagate(t *testing.T) {
	fixture := setupCodexStateFixture(t)
	database := openIndexStore(t)
	t.Cleanup(func() { _ = database.Close() })
	indexer, err := New(database)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	ctx := context.Background()
	if _, err := indexer.Run(ctx, Options{}); err != nil {
		t.Fatalf("initial Run() error = %v", err)
	}

	execCodexState(
		t,
		fixture.statePath,
		"UPDATE threads SET name = ? WHERE id = ?",
		"RENAMED IN CODEX",
		"store-only",
	)
	renamed, err := indexer.Run(ctx, Options{})
	if err != nil {
		t.Fatalf("rename Run() error = %v", err)
	}
	if renamed.RowsTouched != 1 {
		t.Fatalf("rename counters = %+v, want one name row touched", renamed)
	}
	names, err := database.CxNames(ctx)
	if err != nil {
		t.Fatalf("CxNames() error = %v", err)
	}
	if names["store-only"] != "RENAMED IN CODEX" {
		t.Fatalf("store-only name = %q, want the rename", names["store-only"])
	}

	execCodexState(
		t,
		fixture.statePath,
		"UPDATE threads SET archived = 1 WHERE id = ?",
		"store-only",
	)
	archived, err := indexer.Run(ctx, Options{})
	if err != nil {
		t.Fatalf("archive Run() error = %v", err)
	}
	if archived.Deleted != 1 || archived.CodexThreads != 1 {
		t.Fatalf("archive counters = %+v, want the store-only row retired", archived)
	}
	if _, found, err := database.Rollout(ctx, "store-only"); err != nil || found {
		t.Fatalf("archived Rollout() found = %v, error = %v; want false, nil", found, err)
	}
	rows := codexRows(t, database)
	if len(rows) != 1 || rows[0].ID != "file-thread" {
		t.Fatalf("composed Codex rows after archive = %#v", rows)
	}
}

// The store's row is a placeholder for a file that may still arrive: when the
// rollout file finally appears the parsed file replaces the derived values
// instead of stacking on top of them.
func TestCodexStateRowYieldsToTheRolloutFileWhenItArrives(t *testing.T) {
	setupCodexStateFixture(t)
	database := openIndexStore(t)
	t.Cleanup(func() { _ = database.Close() })
	indexer, err := New(database)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	ctx := context.Background()
	if _, err := indexer.Run(ctx, Options{}); err != nil {
		t.Fatalf("initial Run() error = %v", err)
	}
	declared, found, err := database.Rollout(ctx, "store-only")
	if err != nil || !found {
		t.Fatalf("store-only Rollout() found = %v, error = %v", found, err)
	}

	writeLines(t, declared.Path,
		`{"type":"session_meta","payload":{"id":"store-only","thread_source":"user","cwd":"/work/paginated"}}`,
		`{"type":"event_msg","payload":{"type":"user_message","message":"paginated first prompt"}}`,
		`{"type":"event_msg","payload":{"type":"user_message","message":"paginated second prompt"}}`,
	)
	arrived, err := indexer.Run(ctx, Options{})
	if err != nil {
		t.Fatalf("post-arrival Run() error = %v", err)
	}
	if arrived.FullParsed != 1 || arrived.DeltaParsed != 0 {
		t.Fatalf("post-arrival counters = %+v, want a full parse of the new file", arrived)
	}
	parsed, found, err := database.Rollout(ctx, "store-only")
	if err != nil || !found {
		t.Fatalf("parsed Rollout() found = %v, error = %v", found, err)
	}
	if parsed.PromptCount != 2 || parsed.Size <= 0 || !parsed.UserThread {
		t.Fatalf("store-only row after its file arrived = %#v", parsed)
	}
	rows := codexRows(t, database)
	if len(rows) != 2 {
		t.Fatalf("composed Codex rows = %#v, want the conversation still listed once", rows)
	}
}

// A conversation the state store still vouches for keeps its row when its
// rollout file is deleted, instead of vanishing with the file.
func TestCodexStateKeepsThreadWhoseRolloutFileIsDeleted(t *testing.T) {
	fixture := setupCodexStateFixture(t)
	database := openIndexStore(t)
	t.Cleanup(func() { _ = database.Close() })
	indexer, err := New(database)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	ctx := context.Background()
	if _, err := indexer.Run(ctx, Options{}); err != nil {
		t.Fatalf("initial Run() error = %v", err)
	}
	if err := os.Remove(fixture.fileRolloutPath); err != nil {
		t.Fatalf("remove rollout file: %v", err)
	}
	if _, err := indexer.Run(ctx, Options{}); err != nil {
		t.Fatalf("post-delete Run() error = %v", err)
	}

	kept, found, err := database.Rollout(ctx, "file-thread")
	if err != nil || !found {
		t.Fatalf("file-thread Rollout() found = %v, error = %v; want the store's row", found, err)
	}
	if !kept.UserThread || kept.PromptCount != 2 {
		t.Fatalf("file-thread row after file deletion = %#v", kept)
	}
	rows := codexRows(t, database)
	if len(rows) != 2 {
		t.Fatalf("composed Codex rows = %#v, want both conversations", rows)
	}
}
