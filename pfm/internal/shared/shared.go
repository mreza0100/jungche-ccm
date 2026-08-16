// Package shared is the fleet's authoritative state store.
//
// The SQLite database at ~/.cc/fleet.db holds every operator decision: hidden
// chats, spawned teammates, and the primary account. The transcript database
// (paths.Values.DB) remains a derived cache that a rescan can rebuild.
package shared

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"hostops/pfm/internal/paths"

	_ "modernc.org/sqlite"
)

const (
	driverName = "sqlite"

	// PrimaryAccountKey is the meta row for the primary Claude account.
	PrimaryAccountKey = "primary_account"

	// KindNew and KindPane are the two children kinds the schema admits. `new`
	// is a detached teammate on its own tmux socket; `pane` is a teammate
	// sharing this chat's server as "<socket>\t<pane>".
	KindNew  = "new"
	KindPane = "pane"
)

// schemaDDL initializes the complete operator-state schema atomically.
const schemaDDL = `
CREATE TABLE IF NOT EXISTS meta(key TEXT PRIMARY KEY, val TEXT NOT NULL, updated_at INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS hidden(
  uuid TEXT PRIMARY KEY,
  hidden_at INTEGER NOT NULL,
  at_payload INTEGER
);
CREATE TABLE IF NOT EXISTS children(
  kind TEXT NOT NULL CHECK(kind IN ('new','pane')),
  key  TEXT NOT NULL,
  val  TEXT,
  created_at INTEGER NOT NULL,
  PRIMARY KEY(kind, key, val)
);
CREATE TABLE IF NOT EXISTS swap_event(id INTEGER PRIMARY KEY, at INTEGER NOT NULL, line TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS chat(
  uuid    TEXT PRIMARY KEY,
  mtime   INTEGER NOT NULL DEFAULT 0,
  bytes   INTEGER NOT NULL DEFAULT 0,
  cwd     TEXT,
  label   TEXT,
  prompts INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS chat_mtime ON chat(mtime);
INSERT OR IGNORE INTO meta(key,val,updated_at) VALUES('schema_version','1',strftime('%s','now'));
`

// Store is a handle on the shared database.
type Store struct {
	db       *sql.DB
	path     string
	degraded error
}

// Open records a database initialization failure in Degraded. Operations then
// return that failure instead of pretending an operator decision was stored.
func Open(ctx context.Context, values paths.Values) *Store {
	store := &Store{
		path: values.SharedDB,
	}
	db, err := openDatabase(ctx, values.SharedDB)
	if err != nil {
		store.degraded = err
		return store
	}
	store.db = db
	return store
}

func openDatabase(ctx context.Context, path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create shared state directory: %w", err)
	}
	db, err := sql.Open(driverName, path)
	if err != nil {
		return nil, fmt.Errorf("open shared state database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := applyPragmas(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.ExecContext(ctx, schemaDDL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initialize shared state schema: %w", err)
	}
	return db, nil
}

// applyPragmas enables WAL so concurrent chats queue instead of erasing one
// another's edits, plus a busy timeout for concurrent writers.
func applyPragmas(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, "PRAGMA busy_timeout=10000"); err != nil {
		return fmt.Errorf("set shared busy_timeout: %w", err)
	}
	var journalMode string
	if err := db.QueryRowContext(
		ctx,
		"PRAGMA journal_mode=WAL",
	).Scan(&journalMode); err != nil {
		return fmt.Errorf("enable shared WAL: %w", err)
	}
	if !strings.EqualFold(journalMode, "wal") {
		return fmt.Errorf("enable shared WAL: journal_mode is %q", journalMode)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA synchronous=NORMAL"); err != nil {
		return fmt.Errorf("set shared synchronous mode: %w", err)
	}
	return nil
}

// SetBusyTimeout changes how long a statement waits for a concurrent writer
// before giving up. The default is the ten seconds applyPragmas sets; the
// busy-policy fixture shortens it so it can reach the give-up path without
// stalling a test for ten seconds.
func (s *Store) SetBusyTimeout(ctx context.Context, milliseconds int) error {
	if s.db == nil {
		return nil
	}
	if _, err := s.db.ExecContext(
		ctx,
		fmt.Sprintf("PRAGMA busy_timeout=%d", milliseconds),
	); err != nil {
		return fmt.Errorf("set shared busy_timeout: %w", err)
	}
	return nil
}

// Path reports the shared database pathname.
func (s *Store) Path() string { return s.path }

// Degraded reports why the database half is unavailable, or nil.
func (s *Store) Degraded() error { return s.degraded }

// Close releases the database handle.
func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

// HiddenRecord is one operator hide. A nil AtPayload is permanent; a prompt
// baseline is the clear-hide ratchet and expires when that transcript grows.
type HiddenRecord struct {
	HiddenAt  int64
	AtPayload *int64
}

// Hide records a permanent hide in SQLite. An explicit hide always wins over
// an earlier clear-hide ratchet by clearing its prompt baseline.
func (s *Store) Hide(ctx context.Context, id string, hiddenAt int64) error {
	if s.db == nil {
		return fmt.Errorf("record shared hide %q: %w", id, s.degraded)
	}
	if _, err := s.db.ExecContext(ctx, `
INSERT INTO hidden(uuid,hidden_at,at_payload) VALUES(?,?,NULL)
ON CONFLICT(uuid) DO UPDATE SET
  hidden_at=excluded.hidden_at,
  at_payload=NULL`,
		id,
		hiddenAt,
	); err != nil {
		return fmt.Errorf("record shared hide %q: %w", id, err)
	}
	return nil
}

// HideUntilPrompt records the current prompt count for a /clear hide. A
// permanent hide is never weakened; repeated clear events retain the greatest
// observed baseline so delivery/index races cannot unhide too early.
func (s *Store) HideUntilPrompt(
	ctx context.Context,
	id string,
	hiddenAt, baseline int64,
) error {
	if s.db == nil {
		return fmt.Errorf("record shared clear hide %q: %w", id, s.degraded)
	}
	if _, err := s.db.ExecContext(ctx, `
INSERT INTO hidden(uuid,hidden_at,at_payload) VALUES(?,?,?)
ON CONFLICT(uuid) DO UPDATE SET
  hidden_at=CASE WHEN hidden.at_payload IS NULL THEN hidden.hidden_at ELSE excluded.hidden_at END,
  at_payload=CASE
    WHEN hidden.at_payload IS NULL THEN NULL
    WHEN excluded.at_payload > hidden.at_payload THEN excluded.at_payload
    ELSE hidden.at_payload
  END`,
		id,
		hiddenAt,
		baseline,
	); err != nil {
		return fmt.Errorf("record shared clear hide %q: %w", id, err)
	}
	return nil
}

// Unhide deletes one hide row.
func (s *Store) Unhide(ctx context.Context, id string) error {
	if s.db == nil {
		return fmt.Errorf("remove shared hide %q: %w", id, s.degraded)
	}
	if _, err := s.db.ExecContext(
		ctx,
		"DELETE FROM hidden WHERE uuid=?",
		id,
	); err != nil {
		return fmt.Errorf("remove shared hide %q: %w", id, err)
	}
	return nil
}

// UnhideIfPayload expires only the clear-hide version the caller observed.
// A concurrent explicit hide or later /clear survives the conditional delete.
func (s *Store) UnhideIfPayload(
	ctx context.Context,
	id string,
	payload int64,
) (bool, error) {
	if s.db == nil {
		return false, fmt.Errorf("expire shared clear hide %q: %w", id, s.degraded)
	}
	result, err := s.db.ExecContext(
		ctx,
		"DELETE FROM hidden WHERE uuid=? AND at_payload=?",
		id,
		payload,
	)
	if err != nil {
		return false, fmt.Errorf("expire shared clear hide %q: %w", id, err)
	}
	removed, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("count expired shared clear hide %q: %w", id, err)
	}
	return removed == 1, nil
}

// HiddenRecords returns the complete shared hide state keyed by chat id.
func (s *Store) HiddenRecords(ctx context.Context) (map[string]HiddenRecord, error) {
	if s.db == nil {
		return nil, fmt.Errorf("query shared hides: %w", s.degraded)
	}
	records := make(map[string]HiddenRecord)
	rows, err := s.db.QueryContext(
		ctx,
		"SELECT uuid, hidden_at, at_payload FROM hidden",
	)
	if err != nil {
		return nil, fmt.Errorf("query shared hides: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var hiddenAt int64
		var payload sql.NullInt64
		if err := rows.Scan(&id, &hiddenAt, &payload); err != nil {
			return nil, fmt.Errorf("scan shared hide: %w", err)
		}
		record := HiddenRecord{HiddenAt: hiddenAt}
		if payload.Valid {
			value := payload.Int64
			record.AtPayload = &value
		}
		records[id] = record
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate shared hides: %w", err)
	}
	return records, nil
}

// HiddenAt returns every hidden row keyed by chat id.
func (s *Store) HiddenAt(ctx context.Context) (map[string]int64, error) {
	records, err := s.HiddenRecords(ctx)
	if err != nil {
		return nil, err
	}
	hidden := make(map[string]int64, len(records))
	for id, record := range records {
		hidden[id] = record.HiddenAt
	}
	return hidden, nil
}

// Children returns the teammates a chat spawned. The second result is false
// when the shared children table is unreachable, which is the caller's cue to
// fall back to flat files from an older install.
func (s *Store) Children(
	ctx context.Context,
	kind, key string,
) ([]string, bool, error) {
	if s.db == nil {
		return nil, false, nil
	}
	rows, err := s.db.QueryContext(
		ctx,
		"SELECT val FROM children WHERE kind=? AND key=?",
		kind,
		key,
	)
	if err != nil {
		return nil, false, fmt.Errorf("query shared children: %w", err)
	}
	defer rows.Close()
	var values []string
	for rows.Next() {
		var value sql.NullString
		if err := rows.Scan(&value); err != nil {
			return nil, false, fmt.Errorf("scan shared child: %w", err)
		}
		if value.Valid && value.String != "" {
			values = append(values, value.String)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate shared children: %w", err)
	}
	return values, true, nil
}

// AddChild records one spawned teammate.
func (s *Store) AddChild(
	ctx context.Context,
	kind, key, value string,
	createdAt int64,
) error {
	if s.db == nil {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, `
INSERT OR IGNORE INTO children(kind,key,val,created_at) VALUES(?,?,?,?)`,
		kind,
		key,
		value,
		createdAt,
	); err != nil {
		return fmt.Errorf("record shared child: %w", err)
	}
	return nil
}

// ClearChildren removes a chat's teammate rows once every teammate is down.
func (s *Store) ClearChildren(ctx context.Context, kind, key string) error {
	if s.db == nil {
		return nil
	}
	if _, err := s.db.ExecContext(
		ctx,
		"DELETE FROM children WHERE kind=? AND key=?",
		kind,
		key,
	); err != nil {
		return fmt.Errorf("clear shared children: %w", err)
	}
	return nil
}

// Meta reads one meta row.
func (s *Store) Meta(ctx context.Context, key string) (string, bool, error) {
	if s.db == nil {
		return "", false, nil
	}
	var value string
	err := s.db.QueryRowContext(
		ctx,
		"SELECT val FROM meta WHERE key=?",
		key,
	).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read shared meta %q: %w", key, err)
	}
	return value, true, nil
}

// SetMeta writes one timestamped meta row.
func (s *Store) SetMeta(
	ctx context.Context,
	key, value string,
	updatedAt int64,
) error {
	if s.db == nil {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, `
INSERT INTO meta(key,val,updated_at) VALUES(?,?,?)
ON CONFLICT(key) DO UPDATE SET val=excluded.val, updated_at=excluded.updated_at`,
		key,
		value,
		updatedAt,
	); err != nil {
		return fmt.Errorf("write shared meta %q: %w", key, err)
	}
	return nil
}

// PrimaryAccount reports the database value first, then the
// ~/.claude-primary mirror when the database has none, and not-found when
// neither parses. The
// roster bound is the CALLER's to apply — action.MaxAccount is that constant,
// and this package cannot import it without a cycle.
//
// It opens the database read-only and never creates it: reading the primary
// account must not be the act that brings a state store into existence.
func PrimaryAccount(ctx context.Context, values paths.Values) (int, bool) {
	if account, found := primaryFromDatabase(ctx, values.SharedDB); found {
		return account, true
	}
	content, err := os.ReadFile(filepath.Join(values.Home, ".claude-primary"))
	if err != nil {
		return 0, false
	}
	account, err := strconv.Atoi(strings.TrimSpace(string(content)))
	if err != nil {
		return 0, false
	}
	return account, true
}

// SetPrimaryAccount updates the authoritative shared row and its statusline
// mirror. The database is written first; a mirror failure is returned loudly
// so a caller never reports a primary switch that only half landed.
func SetPrimaryAccount(
	ctx context.Context,
	values paths.Values,
	account int,
	updatedAt int64,
) error {
	if account < 1 {
		return fmt.Errorf("primary account must be positive, got %d", account)
	}
	state := Open(ctx, values)
	defer state.Close()
	if err := state.SetMeta(
		ctx,
		PrimaryAccountKey,
		strconv.Itoa(account),
		updatedAt,
	); err != nil {
		return err
	}

	path := filepath.Join(values.Home, ".claude-primary")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create primary mirror directory: %w", err)
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".claude-primary.tmp-*")
	if err != nil {
		return fmt.Errorf("create primary mirror scratch: %w", err)
	}
	tempPath := file.Name()
	defer os.Remove(tempPath)
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return fmt.Errorf("secure primary mirror scratch: %w", err)
	}
	if _, err := fmt.Fprintf(file, "%d\n", account); err != nil {
		_ = file.Close()
		return fmt.Errorf("write primary mirror: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close primary mirror: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("install primary mirror: %w", err)
	}
	return nil
}

func primaryFromDatabase(ctx context.Context, path string) (int, bool) {
	if _, err := os.Stat(path); err != nil {
		return 0, false
	}
	db, err := sql.Open(driverName, path)
	if err != nil {
		return 0, false
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(ctx, "PRAGMA busy_timeout=10000"); err != nil {
		return 0, false
	}
	var value string
	if err := db.QueryRowContext(
		ctx,
		"SELECT val FROM meta WHERE key=?",
		PrimaryAccountKey,
	).Scan(&value); err != nil {
		return 0, false
	}
	account, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, false
	}
	return account, true
}

// SortedIDs returns the keys of a hidden set in stable order.
func SortedIDs(hidden map[string]int64) []string {
	ids := make([]string, 0, len(hidden))
	for id := range hidden {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
