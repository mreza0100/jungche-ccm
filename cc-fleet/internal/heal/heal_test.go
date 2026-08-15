package heal

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	_ "modernc.org/sqlite"
)

// codexJail builds a scratch Codex home: a state store, a history store, and
// the rollouts they point at. The real ~/.codex is never opened by a test —
// healing DELETES projection rows, and a fixture that could reach the live
// store is a fixture that can lose a chat.
type codexJail struct {
	root    string
	stores  Stores
	history *sql.DB
}

func newCodexJail(t *testing.T) *codexJail {
	t.Helper()
	root := t.TempDir()
	// Two generations on purpose: the newest is the live one, and picking the
	// wrong one heals a store Codex no longer reads.
	for _, name := range []string{
		"state_1.sqlite",
		"thread_history_1.sqlite",
	} {
		if err := os.WriteFile(filepath.Join(root, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	statePath := filepath.Join(root, "state_2.sqlite")
	historyPath := filepath.Join(root, "thread_history_2.sqlite")

	state := openJailDB(t, statePath)
	execJail(t, state, `CREATE TABLE threads (
		id TEXT PRIMARY KEY,
		rollout_path TEXT
	)`)
	if err := state.Close(); err != nil {
		t.Fatal(err)
	}

	history := openJailDB(t, historyPath)
	execJail(t, history, `CREATE TABLE thread_history_projection_state (
		thread_id TEXT PRIMARY KEY,
		next_rollout_byte_offset INTEGER,
		next_rollout_ordinal INTEGER
	)`)
	execJail(t, history, `CREATE TABLE thread_items (
		thread_id TEXT, ordinal INTEGER
	)`)
	execJail(t, history, `CREATE TABLE thread_turns (
		thread_id TEXT, ordinal INTEGER
	)`)
	t.Cleanup(func() { _ = history.Close() })

	return &codexJail{
		root:    root,
		stores:  Stores{State: statePath, History: historyPath, Root: root},
		history: history,
	}
}

func openJailDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	database, err := sql.Open(driverName, "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	database.SetMaxOpenConns(1)
	return database
}

func execJail(t *testing.T, database *sql.DB, statement string, args ...any) {
	t.Helper()
	if _, err := database.Exec(statement, args...); err != nil {
		t.Fatalf("%s: %v", statement, err)
	}
}

// addThread writes a rollout of `records` JSONL lines and registers the
// thread, returning the byte offset of each record.
func (jail *codexJail) addThread(t *testing.T, id string, records int) []int64 {
	t.Helper()
	path := filepath.Join(jail.root, "sessions", "rollout-"+id+".jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	var content strings.Builder
	offsets := make([]int64, 0, records)
	for index := 0; index < records; index++ {
		offsets = append(offsets, int64(content.Len()))
		content.WriteString(fmt.Sprintf(
			`{"ordinal":%d,"type":"event_msg","payload":{"n":%d}}`+"\n",
			index,
			index,
		))
	}
	if err := os.WriteFile(path, []byte(content.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	state := openJailDB(t, jail.stores.State)
	defer state.Close()
	execJail(
		t,
		state,
		"INSERT INTO threads(id, rollout_path) VALUES(?, ?)",
		id,
		path,
	)
	return append(offsets, int64(content.Len()))
}

func (jail *codexJail) setCursor(t *testing.T, id string, offset, ordinal int64) {
	t.Helper()
	execJail(
		t,
		jail.history,
		"INSERT INTO thread_history_projection_state("+
			"thread_id, next_rollout_byte_offset, next_rollout_ordinal) VALUES(?,?,?)",
		id,
		offset,
		ordinal,
	)
	execJail(
		t,
		jail.history,
		"INSERT INTO thread_items(thread_id, ordinal) VALUES(?,?)",
		id,
		ordinal,
	)
	execJail(
		t,
		jail.history,
		"INSERT INTO thread_turns(thread_id, ordinal) VALUES(?,?)",
		id,
		ordinal,
	)
}

func (jail *codexJail) projectionRows(t *testing.T, id string) int {
	t.Helper()
	total := 0
	for _, table := range []string{
		"thread_history_projection_state",
		"thread_items",
		"thread_turns",
	} {
		var count int
		if err := jail.history.QueryRow(
			"SELECT count(*) FROM "+table+" WHERE thread_id = ?",
			id,
		).Scan(&count); err != nil {
			t.Fatal(err)
		}
		total += count
	}
	return total
}

// FindStores must land on the NEWEST generation of each store: Codex leaves
// the old ones behind, and healing one it no longer reads changes nothing
// while reporting success.
func TestFindStoresTakesTheNewestGeneration(t *testing.T) {
	jail := newCodexJail(t)
	stores, err := FindStores(jail.root)
	if err != nil {
		t.Fatalf("FindStores() error = %v", err)
	}
	if filepath.Base(stores.State) != "state_2.sqlite" {
		t.Fatalf("state store = %s, want state_2.sqlite", stores.State)
	}
	if filepath.Base(stores.History) != "thread_history_2.sqlite" {
		t.Fatalf("history store = %s, want thread_history_2.sqlite", stores.History)
	}
	if _, err := FindStores(filepath.Join(jail.root, "nowhere")); err == nil {
		t.Fatal("FindStores() accepted a Codex home that is not there")
	}
}

// The verdicts are the whole diagnosis, and each one has a distinct cause.
func TestSweepJudgesEveryCursorShape(t *testing.T) {
	jail := newCodexJail(t)
	const (
		caughtUp   = "11111111-1111-4111-8111-111111111111"
		consistent = "22222222-2222-4222-8222-222222222222"
		wedged     = "33333333-3333-4333-8333-333333333333"
		midline    = "44444444-4444-4444-8444-444444444444"
		noRollout  = "55555555-5555-4555-8555-555555555555"
	)
	caughtUpOffsets := jail.addThread(t, caughtUp, 3)
	consistentOffsets := jail.addThread(t, consistent, 3)
	wedgedOffsets := jail.addThread(t, wedged, 3)
	midlineOffsets := jail.addThread(t, midline, 3)

	jail.setCursor(t, caughtUp, caughtUpOffsets[3], 3)
	jail.setCursor(t, consistent, consistentOffsets[1], 1)
	// The 0.146.1 shape: the offset advanced, the ordinal did not.
	jail.setCursor(t, wedged, wedgedOffsets[2], 1)
	jail.setCursor(t, midline, midlineOffsets[1]+5, 1)
	jail.setCursor(t, noRollout, 0, 0)
	state := openJailDB(t, jail.stores.State)
	execJail(
		t,
		state,
		"INSERT INTO threads(id, rollout_path) VALUES(?, ?)",
		noRollout,
		filepath.Join(jail.root, "sessions", "gone.jsonl"),
	)
	if err := state.Close(); err != nil {
		t.Fatal(err)
	}

	report, err := Sweep(context.Background(), jail.stores, "")
	if err != nil {
		t.Fatalf("Sweep() error = %v", err)
	}
	want := map[string]Verdict{
		caughtUp:   VerdictCaughtUp,
		consistent: VerdictConsistent,
		wedged:     VerdictWedged,
		midline:    VerdictMidline,
		noRollout:  VerdictNoRollout,
	}
	if len(report.Threads) != len(want) {
		t.Fatalf("Sweep() returned %d threads, want %d", len(report.Threads), len(want))
	}
	for _, thread := range report.Threads {
		if thread.Verdict != want[thread.ID] {
			t.Fatalf(
				"%s = %s (%s), want %s",
				thread.ID,
				thread.Verdict,
				thread.Detail,
				want[thread.ID],
			)
		}
	}
	if report.Totals[VerdictWedged] != 1 || report.Totals[VerdictMidline] != 1 {
		t.Fatalf("totals = %v", report.Totals)
	}
}

// A sweep is read-only: reporting must never change a projection.
func TestSweepChangesNothing(t *testing.T) {
	jail := newCodexJail(t)
	const wedged = "33333333-3333-4333-8333-333333333333"
	offsets := jail.addThread(t, wedged, 3)
	jail.setCursor(t, wedged, offsets[2], 1)
	if _, err := Sweep(context.Background(), jail.stores, ""); err != nil {
		t.Fatal(err)
	}
	if rows := jail.projectionRows(t, wedged); rows != 3 {
		t.Fatalf("a report deleted projection rows: %d remain, want 3", rows)
	}
}

// --apply heals exactly the broken threads, backs up first, and leaves every
// healthy projection alone.
func TestApplyHealsOnlyBrokenThreads(t *testing.T) {
	jail := newCodexJail(t)
	const (
		healthy = "22222222-2222-4222-8222-222222222222"
		wedged  = "33333333-3333-4333-8333-333333333333"
	)
	healthyOffsets := jail.addThread(t, healthy, 3)
	wedgedOffsets := jail.addThread(t, wedged, 3)
	jail.setCursor(t, healthy, healthyOffsets[1], 1)
	jail.setCursor(t, wedged, wedgedOffsets[2], 1)

	runner, err := New(jail.root, func() time.Time {
		return time.Unix(1800000000, 0)
	})
	if err != nil {
		t.Fatal(err)
	}
	report, err := runner.Run(context.Background(), Options{Apply: true})
	if err != nil {
		t.Fatalf("Run(--apply) error = %v", err)
	}
	if len(report.Healed) != 1 || report.Healed[0] != wedged {
		t.Fatalf("healed = %v, want [%s]", report.Healed, wedged)
	}
	if rows := jail.projectionRows(t, wedged); rows != 0 {
		t.Fatalf("the wedged projection still holds %d rows", rows)
	}
	if rows := jail.projectionRows(t, healthy); rows != 3 {
		t.Fatalf("a healthy projection lost rows: %d remain, want 3", rows)
	}
	if report.BackupDir == "" {
		t.Fatal("a heal ran with no backup")
	}
	if _, err := os.Stat(filepath.Join(
		report.BackupDir,
		filepath.Base(jail.stores.History),
	)); err != nil {
		t.Fatalf("the history store was not backed up: %v", err)
	}

	// Idempotent: the healed thread has no cursor left, so a second run finds
	// nothing to do and writes nothing.
	second, err := runner.Run(context.Background(), Options{Apply: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Healed) != 0 {
		t.Fatalf("a second run healed %v", second.Healed)
	}
	if second.BackupDir != "" {
		t.Fatal("a second run took a backup with nothing to heal")
	}
}

// A thread another seat is holding is never healed: Codex carries that
// cursor in memory and would write it back over the repair.
func TestLiveThreadsAreSkipped(t *testing.T) {
	jail := newCodexJail(t)
	const wedged = "33333333-3333-4333-8333-333333333333"
	offsets := jail.addThread(t, wedged, 3)
	jail.setCursor(t, wedged, offsets[2], 1)

	locks := filepath.Join(jail.root, "thread-writer-locks")
	if err := os.MkdirAll(locks, 0o700); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(locks, wedged+".lock")
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		t.Fatalf("hold the writer lock: %v", err)
	}
	if !Live(jail.root, wedged) {
		t.Fatal("a held writer lock did not read as live")
	}

	runner, err := New(jail.root, nil)
	if err != nil {
		t.Fatal(err)
	}
	report, err := runner.Run(context.Background(), Options{Apply: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Healed) != 0 {
		t.Fatalf("a live thread was healed: %v", report.Healed)
	}
	if len(report.SkippedLive) != 1 || report.SkippedLive[0] != wedged {
		t.Fatalf("skipped-live = %v, want [%s]", report.SkippedLive, wedged)
	}
	if rows := jail.projectionRows(t, wedged); rows != 3 {
		t.Fatalf("a live thread's projection was deleted anyway (%d rows left)", rows)
	}

	// Released, the same thread heals.
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_UN); err != nil {
		t.Fatal(err)
	}
	if Live(jail.root, wedged) {
		t.Fatal("a released lock still reads as live")
	}
	report, err = runner.Run(context.Background(), Options{Apply: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Healed) != 1 {
		t.Fatalf("healed = %v after the lock was released", report.Healed)
	}
}

// Thread is the pre-resume shape: it repairs a broken thread, says so in one
// line, and is completely silent about everything else — including a Codex
// home that does not exist, because a resume must never fail over a repair.
func TestThreadIsSilentUnlessItRepairs(t *testing.T) {
	jail := newCodexJail(t)
	const (
		healthy = "22222222-2222-4222-8222-222222222222"
		wedged  = "33333333-3333-4333-8333-333333333333"
	)
	healthyOffsets := jail.addThread(t, healthy, 3)
	wedgedOffsets := jail.addThread(t, wedged, 3)
	jail.setCursor(t, healthy, healthyOffsets[1], 1)
	jail.setCursor(t, wedged, wedgedOffsets[2], 1)

	ctx := context.Background()
	if message := Thread(ctx, jail.root, healthy); message != "" {
		t.Fatalf("a healthy thread reported %q", message)
	}
	if rows := jail.projectionRows(t, wedged); rows != 3 {
		t.Fatalf("healing one thread touched another")
	}
	message := Thread(ctx, jail.root, wedged)
	if !strings.Contains(message, wedged) ||
		!strings.Contains(message, string(VerdictWedged)) {
		t.Fatalf("Thread() = %q, want a line naming the thread and its verdict", message)
	}
	if rows := jail.projectionRows(t, wedged); rows != 0 {
		t.Fatalf("Thread() reported a heal but left %d rows", rows)
	}
	if message := Thread(ctx, filepath.Join(jail.root, "nowhere"), wedged); message != "" {
		t.Fatalf("a missing Codex home reported %q instead of staying silent", message)
	}
	if message := Thread(ctx, jail.root, ""); message != "" {
		t.Fatalf("an empty thread id reported %q", message)
	}
}
