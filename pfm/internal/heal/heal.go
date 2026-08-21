// Package heal repairs Codex thread-history projections so a resumed seat
// opens WHOLE instead of amnesiac at its first prompt.
//
// Codex ≥0.146 renders a resumed paginated thread from a projection of its
// rollout held in thread_history_<N>.sqlite, advanced by a stored (byte
// offset, ordinal) cursor. Codex 0.146.1 desynced that cursor on every
// paginated thread — the offset advanced past a token_count record and the
// ordinal did not — and later versions refuse to project past the
// inconsistency, forever. The thread then resumes showing only its first turn
// while the rollout on disk is complete.
//
// Deleting that thread's projection rows makes Codex rebuild the whole
// projection from the rollout at the next resume (measured: 110 MB in under
// 20 s), so the chat opens exactly as it was closed. Nothing else is touched,
// the rollout is never written, and the store is copied before any delete.
package heal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	_ "modernc.org/sqlite"
)

// driverName is the pure-Go SQLite driver the whole engine uses.
const driverName = "sqlite"

// Verdict is one thread's projection state.
type Verdict string

const (
	// VerdictCaughtUp means the projection has consumed the whole rollout.
	VerdictCaughtUp Verdict = "CAUGHT_UP"
	// VerdictConsistent means the cursor points at a record whose ordinal is
	// the one the projection expects: healthy, mid-file.
	VerdictConsistent Verdict = "CONSISTENT"
	// VerdictWedged means the record at the cursor carries a DIFFERENT
	// ordinal than the projection expects. Codex will never project past it.
	VerdictWedged Verdict = "WEDGED"
	// VerdictMidline means the cursor points inside a record rather than at
	// the start of one — the same dead end by another route.
	VerdictMidline Verdict = "MIDLINE"
	// VerdictNoRollout means the thread's rollout file is gone, so there is
	// nothing to project and nothing to repair.
	VerdictNoRollout Verdict = "NO_ROLLOUT"
)

// Broken reports whether a verdict is one healing fixes.
func (verdict Verdict) Broken() bool {
	return verdict == VerdictWedged || verdict == VerdictMidline
}

// ThreadState is one thread's cursor, its rollout, and the verdict on them.
type ThreadState struct {
	ID          string
	RolloutPath string
	Offset      int64
	Ordinal     int64
	Size        int64
	Verdict     Verdict
	Detail      string
}

// Report is one sweep or one heal.
type Report struct {
	Threads []ThreadState
	Totals  map[Verdict]int
	// Healed and SkippedLive are filled by a heal run.
	Healed      []string
	SkippedLive []string
	BackupDir   string
}

// Stores are the two SQLite files a heal reads: Codex's thread registry and
// its history projection.
type Stores struct {
	State   string
	History string
	Root    string
}

// FindStores locates the newest generation of each store under codexRoot.
// Codex leaves older generations behind when it migrates, and the highest N is
// the live one.
func FindStores(codexRoot string) (Stores, error) {
	if codexRoot == "" {
		return Stores{}, errors.New("no Codex home to heal")
	}
	entries, err := os.ReadDir(codexRoot)
	if errors.Is(err, fs.ErrNotExist) {
		return Stores{}, fmt.Errorf("no Codex home at %s", codexRoot)
	}
	if err != nil {
		return Stores{}, fmt.Errorf("read Codex home %q: %w", codexRoot, err)
	}
	stores := Stores{Root: codexRoot}
	stateGeneration, historyGeneration := -1, -1
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if generation, ok := storeGeneration(entry.Name(), "state_"); ok &&
			generation > stateGeneration {
			stateGeneration = generation
			stores.State = filepath.Join(codexRoot, entry.Name())
		}
		if generation, ok := storeGeneration(entry.Name(), "thread_history_"); ok &&
			generation > historyGeneration {
			historyGeneration = generation
			stores.History = filepath.Join(codexRoot, entry.Name())
		}
	}
	if stores.State == "" {
		return Stores{}, fmt.Errorf("no state_N.sqlite under %s", codexRoot)
	}
	if stores.History == "" {
		return Stores{}, fmt.Errorf("no thread_history_N.sqlite under %s", codexRoot)
	}
	return stores, nil
}

func storeGeneration(name, prefix string) (int, bool) {
	rest, found := strings.CutPrefix(name, prefix)
	if !found {
		return 0, false
	}
	rest, found = strings.CutSuffix(rest, ".sqlite")
	if !found {
		return 0, false
	}
	number, err := strconv.Atoi(rest)
	if err != nil || number < 0 {
		return 0, false
	}
	return number, true
}

// Sweep reads every projection cursor and judges it. It opens both stores
// READ-ONLY, and never immutable: an immutable handle kills the -wal and
// would judge a store Codex is actively writing from a stale snapshot.
//
// only, when set, limits the sweep to one thread id.
func Sweep(ctx context.Context, stores Stores, only string) (Report, error) {
	rolloutByID, err := rolloutPaths(ctx, stores.State)
	if err != nil {
		return Report{}, err
	}
	history, err := openReadOnly(stores.History)
	if err != nil {
		return Report{}, err
	}
	defer history.Close()

	rows, err := history.QueryContext(
		ctx,
		"SELECT thread_id, next_rollout_byte_offset, next_rollout_ordinal "+
			"FROM thread_history_projection_state",
	)
	if err != nil {
		return Report{}, fmt.Errorf(
			"read the projection cursors in %q: %w",
			stores.History,
			err,
		)
	}
	defer rows.Close()

	report := Report{Totals: make(map[Verdict]int)}
	for rows.Next() {
		var state ThreadState
		if err := rows.Scan(&state.ID, &state.Offset, &state.Ordinal); err != nil {
			return Report{}, fmt.Errorf("scan a projection cursor: %w", err)
		}
		if only != "" && state.ID != only {
			continue
		}
		state.RolloutPath = rolloutByID[state.ID]
		state.Verdict, state.Detail, state.Size = classify(
			state.RolloutPath,
			state.Offset,
			state.Ordinal,
		)
		report.Totals[state.Verdict]++
		report.Threads = append(report.Threads, state)
	}
	if err := rows.Err(); err != nil {
		return Report{}, fmt.Errorf("iterate the projection cursors: %w", err)
	}
	sort.Slice(report.Threads, func(left, right int) bool {
		return report.Threads[left].ID < report.Threads[right].ID
	})
	return report, nil
}

// classify judges one cursor against the rollout it points into. It is the
// single definition of "wedged" (K3): the pre-resume guard and the reporting
// command must agree, or one of them heals a thread the other calls healthy.
func classify(
	rolloutPath string,
	offset, ordinal int64,
) (Verdict, string, int64) {
	if rolloutPath == "" {
		return VerdictNoRollout, "the state store records no rollout", 0
	}
	info, err := os.Stat(rolloutPath)
	if err != nil {
		return VerdictNoRollout, rolloutPath, 0
	}
	size := info.Size()
	if offset >= size {
		return VerdictCaughtUp, fmt.Sprintf("%d/%d", offset, size), size
	}
	file, err := os.Open(rolloutPath)
	if err != nil {
		return VerdictNoRollout, err.Error(), size
	}
	defer file.Close()

	if offset > 0 {
		previous := make([]byte, 1)
		if _, err := file.ReadAt(previous, offset-1); err != nil {
			return VerdictMidline,
				fmt.Sprintf("offset %d is unreadable in %d bytes", offset, size),
				size
		}
		if previous[0] != '\n' {
			return VerdictMidline,
				fmt.Sprintf("offset %d sits inside a record of %d bytes", offset, size),
				size
		}
	}
	line, err := readRecordAt(file, offset)
	if err != nil {
		return VerdictMidline,
			fmt.Sprintf("no readable record at offset %d", offset),
			size
	}
	var record struct {
		Ordinal *int64 `json:"ordinal"`
	}
	if err := json.Unmarshal(line, &record); err != nil {
		return VerdictMidline,
			fmt.Sprintf("unparseable record at offset %d", offset),
			size
	}
	fileOrdinal := int64(-1)
	if record.Ordinal != nil {
		fileOrdinal = *record.Ordinal
	}
	if fileOrdinal == ordinal {
		return VerdictConsistent,
			fmt.Sprintf("%d/%d ordinal %d", offset, size, ordinal),
			size
	}
	return VerdictWedged,
		fmt.Sprintf(
			"expects ordinal %d, the file has %d; %.1f MB unprojected",
			ordinal,
			fileOrdinal,
			float64(size-offset)/1e6,
		),
		size
}

// readRecordAt returns the one JSONL record starting at offset.
func readRecordAt(file *os.File, offset int64) ([]byte, error) {
	const chunk = 64 * 1024
	buffer := make([]byte, 0, chunk)
	scratch := make([]byte, chunk)
	for {
		read, err := file.ReadAt(scratch, offset+int64(len(buffer)))
		if read > 0 {
			if index := indexByte(scratch[:read], '\n'); index >= 0 {
				return append(buffer, scratch[:index]...), nil
			}
			buffer = append(buffer, scratch[:read]...)
		}
		if err != nil {
			if len(buffer) > 0 {
				return buffer, nil
			}
			return nil, err
		}
	}
}

func indexByte(data []byte, target byte) int {
	for index, value := range data {
		if value == target {
			return index
		}
	}
	return -1
}

// rolloutPaths reads the thread → rollout mapping out of the state store.
func rolloutPaths(ctx context.Context, statePath string) (map[string]string, error) {
	state, err := openReadOnly(statePath)
	if err != nil {
		return nil, err
	}
	defer state.Close()
	rows, err := state.QueryContext(ctx, "SELECT id, rollout_path FROM threads")
	if err != nil {
		return nil, fmt.Errorf("read threads from %q: %w", statePath, err)
	}
	defer rows.Close()
	paths := make(map[string]string)
	for rows.Next() {
		var id string
		var path sql.NullString
		if err := rows.Scan(&id, &path); err != nil {
			return nil, fmt.Errorf("scan a thread row: %w", err)
		}
		paths[id] = path.String
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate thread rows: %w", err)
	}
	return paths, nil
}

func openReadOnly(path string) (*sql.DB, error) {
	database, err := sql.Open(
		driverName,
		"file:"+path+"?mode=ro&_pragma=busy_timeout(2000)",
	)
	if err != nil {
		return nil, fmt.Errorf("open %q read-only: %w", path, err)
	}
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	return database, nil
}

// Live reports whether Codex holds this thread's writer lock right now.
//
// A held lock means a running seat owns the thread and its in-memory cursor
// would race a heal, so the thread is left alone. A lock file that exists but
// is NOT held is the ordinary leftover of a closed seat.
func Live(codexRoot, threadID string) bool {
	path := filepath.Join(codexRoot, "thread-writer-locks", threadID+".lock")
	file, err := os.OpenFile(path, os.O_RDWR, 0o600)
	if err != nil {
		return false
	}
	defer file.Close()
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		return true
	}
	// The probe took the lock; give it straight back, because holding it is
	// itself the thing that would block a seat from opening.
	_ = unix.Flock(int(file.Fd()), unix.LOCK_UN)
	return false
}

// Backup copies the history store, and its -wal and -shm siblings, into a
// stamped directory beside it. It runs BEFORE any delete, every time: the
// projection rows are the only copy of work Codex would otherwise have to
// rebuild, and a rebuild is cheap only when the rollout is intact.
func Backup(stores Stores, now time.Time) (string, error) {
	destination := filepath.Join(
		stores.Root,
		"heal-backup-"+now.UTC().Format("20060102-150405"),
	)
	if err := os.MkdirAll(destination, 0o700); err != nil {
		return "", fmt.Errorf("create the heal backup directory: %w", err)
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		source := stores.History + suffix
		content, err := os.ReadFile(source)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("read %s for backup: %w", source, err)
		}
		target := filepath.Join(destination, filepath.Base(source))
		if err := os.WriteFile(target, content, 0o600); err != nil {
			return "", fmt.Errorf("write %s: %w", target, err)
		}
	}
	return destination, nil
}

// Delete removes one thread's projection so Codex rebuilds it from the
// rollout at the next resume. The three tables go in ONE immediate
// transaction: a projection state without its items is a thread that resumes
// empty, which is the very failure this repairs.
func Delete(ctx context.Context, stores Stores, threadID string) error {
	database, err := sql.Open(
		driverName,
		"file:"+stores.History+"?_pragma=busy_timeout(5000)",
	)
	if err != nil {
		return fmt.Errorf("open %q: %w", stores.History, err)
	}
	defer database.Close()
	database.SetMaxOpenConns(1)

	transaction, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin the heal transaction: %w", err)
	}
	defer func() { _ = transaction.Rollback() }()
	for _, table := range []string{
		"thread_history_projection_state",
		"thread_items",
		"thread_turns",
	} {
		if _, err := transaction.ExecContext(
			ctx,
			"DELETE FROM "+table+" WHERE thread_id = ?",
			threadID,
		); err != nil {
			return fmt.Errorf("clear %s for %s: %w", table, threadID, err)
		}
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit the heal for %s: %w", threadID, err)
	}
	return nil
}
