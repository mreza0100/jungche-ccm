package index

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"hostops/pfm/internal/paths"
	"hostops/pfm/internal/store"

	_ "modernc.org/sqlite"
)

// seedOpencodeStress builds a large, hostile session store: thousands of
// sessions, unicode/control-character titles, oversized prompts, malformed
// model JSON, NULL-heavy rows, and duplicate timestamps.
func seedOpencodeStress(t *testing.T, root string, count int) {
	t.Helper()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	db, err := sql.Open("sqlite", filepath.Join(root, "opencode.db"))
	if err != nil {
		t.Fatalf("open stress store: %v", err)
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
CREATE TABLE session_input (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL,
  prompt TEXT NOT NULL,
  delivery TEXT NOT NULL,
  admitted_seq INTEGER NOT NULL,
  time_created INTEGER NOT NULL
);`
	if _, err := db.Exec(script); err != nil {
		t.Fatalf("seed schema: %v", err)
	}
	// Production opencode.db runs in WAL mode (-wal/-shm siblings live beside
	// it); the stress fixture must match or its locking behavior tests a
	// database OpenCode never writes.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		t.Fatalf("enable WAL: %v", err)
	}
	if _, err := db.Exec("INSERT INTO project VALUES ('p1', '/stress/repo')"); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	hostileTitles := []string{
		"quote'and\"double",
		"unicode ∑ ≤ ≥ 🚀 ‱",
		"tab\tand\nnewline",
		strings.Repeat("long", 500),
		"",
		"; rm -rf / --no-preserve-root",
	}
	hostileModels := []string{
		`{"id":"m","providerID":"p"}`,
		`{malformed json`,
		``,
		`{"id":""}`,
		strings.Repeat("x", 10_000),
	}
	bigPrompt := strings.Repeat("prompt body ", 8192) // ~100KB
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	for i := 0; i < count; i++ {
		parent := any(nil)
		if i%7 == 3 { // every seventh row is a subagent child
			parent = fmt.Sprintf("ses_%d", i-1)
		}
		var archived any
		if i%11 == 5 {
			archived = int64(i * 100)
		}
		if _, err := tx.Exec(
			"INSERT INTO session (id, project_id, parent_id, directory, title, agent, model, tokens_input, tokens_output, cost, time_created, time_updated, time_archived) VALUES (?, 'p1', ?, ?, ?, 'build', ?, ?, ?, ?, ?, ?, ?)",
			fmt.Sprintf("ses_%d", i), parent,
			[]any{"/stress/repo", "", "/stress/repo/"}[i%3],
			hostileTitles[i%len(hostileTitles)],
			hostileModels[i%len(hostileModels)],
			i, i, float64(i%97)/7.0,
			int64(i), int64(i), // duplicate timestamps on purpose
			archived,
		); err != nil {
			t.Fatalf("seed session %d: %v", i, err)
		}
		prompt := bigPrompt
		if i%2 == 0 {
			prompt = hostileTitles[i%len(hostileTitles)]
		}
		if _, err := tx.Exec(
			"INSERT INTO session_input (id, session_id, prompt, delivery, admitted_seq, time_created) VALUES (?, ?, ?, 'text', 0, ?)",
			fmt.Sprintf("in_%d", i), fmt.Sprintf("ses_%d", i), prompt, int64(i),
		); err != nil {
			t.Fatalf("seed input %d: %v", i, err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

func TestStressOpencodeIndexSurvivesHostileStore(t *testing.T) {
	const count = 3000
	root := t.TempDir()
	seedOpencodeStress(t, root, count)

	for pass := 0; pass < 3; pass++ {
		sessions, err := ReadOpencodeSessions(context.Background(), root)
		if err != nil {
			t.Fatalf("pass %d read: %v", pass, err)
		}
		if len(sessions) != count {
			t.Fatalf("pass %d read %d sessions, want %d", pass, len(sessions), count)
		}
		children, archived := 0, 0
		for _, session := range sessions {
			if session.ParentID != "" {
				children++
			}
			if session.TimeArchivedMS != 0 {
				archived++
			}
			if len([]rune(session.FirstPrompt)) > 201 {
				t.Fatalf("first prompt escaped its clip: %d runes", len([]rune(session.FirstPrompt)))
			}
		}
		wantChildren := (count + 6) / 7 // ceil(count/7): i%7==3 pattern
		if children != countChildren(count) {
			t.Errorf("children = %d", children)
		}
		_ = wantChildren
		if archived != countArchived(count) {
			t.Errorf("archived = %d, computed %d", archived, countArchived(count))
		}
	}
}

func countChildren(n int) int {
	total := 0
	for i := 0; i < n; i++ {
		if i%7 == 3 {
			total++
		}
	}
	return total
}

func countArchived(n int) int {
	total := 0
	for i := 0; i < n; i++ {
		if i%11 == 5 {
			total++
		}
	}
	return total
}

// The mirror must converge under repeated full passes and concurrent readers:
// two goroutines indexing the same store race the temp-copy path.
func TestStressOpencodeMirrorConcurrentPasses(t *testing.T) {
	const count = 800
	root := t.TempDir()
	seedOpencodeStress(t, root, count)

	jail := t.TempDir()
	t.Setenv(paths.EnvDB, filepath.Join(jail, "fleet.db"))
	t.Setenv(paths.EnvSharedDB, filepath.Join(jail, "shared.db"))
	database, err := store.Open()
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer database.Close()

	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for worker := 0; worker < 4; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var counters Counters
			if err := syncOpencodeMirror(context.Background(), database, root, &counters); err != nil {
				errs <- err
				return
			}
			if counters.OcSessions != count {
				errs <- fmt.Errorf("OcSessions = %d, want %d", counters.OcSessions, count)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	stored, err := database.OcSessions(context.Background())
	if err != nil {
		t.Fatalf("read mirror: %v", err)
	}
	if len(stored) != count {
		t.Fatalf("mirror holds %d rows, want %d", len(stored), count)
	}
}

// A live OpenCode process checkpoints into the WAL while we copy; the reader
// must tolerate the database growing mid-copy without erroring or hanging.
func TestStressOpencodeReadWhileWriterActive(t *testing.T) {
	const (
		count      = 400
		liveWrites = 2000
	)
	root := t.TempDir()
	seedOpencodeStress(t, root, count)

	dbPath := filepath.Join(root, "opencode.db")
	stop := make(chan struct{})
	writerDone := make(chan error, 1)
	go func() {
		live, err := sql.Open("sqlite", dbPath)
		if err != nil {
			writerDone <- err
			return
		}
		defer live.Close()
		for round := 0; round < liveWrites; round++ {
			select {
			case <-stop:
				writerDone <- nil
				return
			default:
			}
			if _, err := live.Exec(
				"INSERT INTO session_input (id, session_id, prompt, delivery, admitted_seq, time_created) VALUES (?, 'ses_0', ?, 'text', 0, ?)",
				fmt.Sprintf("live_%d", round), strings.Repeat("w", 2000), int64(1_000_000+round),
			); err != nil {
				writerDone <- err
				return
			}
			// Yield the write lock: OpenCode writes in bursts between turns,
			// never as a zero-gap spin, and a spin starves the snapshot.
			time.Sleep(2 * time.Millisecond)
		}
		// Keep the writer connection live after the bounded write burst. An
		// unbounded producer makes the fixture itself grow faster than a reader
		// can finish under CPU contention, turning a concurrency probe into an
		// ever-expanding benchmark that times out by construction.
		<-stop
		writerDone <- nil
	}()

	reads := 0
	deadline := 40
	for round := 0; round < deadline; round++ {
		sessions, err := ReadOpencodeSessions(context.Background(), root)
		if err != nil {
			close(stop)
			t.Fatalf("concurrent read %d failed: %v", round, err)
		}
		if len(sessions) != count {
			close(stop)
			t.Fatalf("concurrent read %d saw %d sessions, want %d", round, len(sessions), count)
		}
		reads++
	}
	close(stop)
	if err := <-writerDone; err != nil {
		t.Fatalf("writer failed: %v", err)
	}
	if reads != deadline {
		t.Fatalf("completed %d reads, want %d", reads, deadline)
	}
}
