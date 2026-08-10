// Package shared is the fleet's ONE state store, co-owned with the zsh half.
//
// cc-db.sh (bash) and this binary (Go) are two front ends over the same two
// files: the SQLite database at ~/.cc/fleet.db and the carrier file at
// ~/.claude/.cc-ls-hidden. Everything a person can decide — which chats are
// hidden, which teammates a chat spawned, which account is primary — lives
// here, so neither half can hold an opinion the other cannot see. This binary's
// own database (paths.Values.DB) stays what it always was: a derived cache of
// transcripts and rollouts, rebuildable by a rescan.
//
// THE TWO-WRITER LAW (cc-db.sh:110-115). The carrier file is not a legacy
// leftover — it is the hidden set as it stands between picker runs, because the
// picker is the last thing a run does and a ⌃X toggle can only be carried in
// that file. So every writer of the hidden table writes the file too, and a
// read is the UNION of both. Only an explicit unhide removes a row: hides are
// permanent.
//
// DEGRADED MODE. No SQLite, an unopenable database, a read-only directory: the
// store keeps working against the carrier file alone and reports the reason
// through Degraded. cc-db.sh's fallback is the same (cc-db.sh:17-18) for the
// same reason — the picker must never be down because a database is unhappy.
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

	"hostops/cc-fleet/internal/paths"

	_ "modernc.org/sqlite"
)

const (
	driverName = "sqlite"

	// PrimaryAccountKey is cc-db.sh's meta row for the primary Claude account
	// (cc-db.sh:264, :271).
	PrimaryAccountKey = "primary_account"

	// KindNew and KindPane are the two children kinds cc-db.sh's CHECK
	// constraint admits (cc-db.sh:81). `new` is a detached teammate on its own
	// tmux socket, `pane` is a teammate sharing this chat's server as
	// "<socket>\t<pane>" (chat.sh:435, chat.sh:1362).
	KindNew  = "new"
	KindPane = "pane"
)

// schemaDDL is cc-db.sh's cmd_init, transcribed (cc-db.sh:72-102).
//
// All of it, not just the three tables Go touches: creating the file with a
// partial schema would flip cc-db.sh's have_db to true while `chat-load` still
// queried a table that did not exist, and the zsh picker would cold-parse every
// transcript on every run. Whichever half creates the database, the other finds
// the schema it expects.
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

// Store is a handle on the shared database plus its carrier file.
type Store struct {
	db       *sql.DB
	path     string
	carrier  string
	degraded error
}

// Open returns a usable Store in every case. When the database cannot be
// opened or initialized the Store degrades to the carrier file and records why;
// callers report Degraded and carry on.
func Open(ctx context.Context, values paths.Values) *Store {
	store := &Store{
		path:    values.SharedDB,
		carrier: values.HiddenCarrier,
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

// applyPragmas matches what cc-db.sh asks of every statement it runs: WAL, so
// two chats queue instead of one erasing the other's edit, and a busy timeout,
// so a concurrent sqlite3 CLI writer is waited out rather than failed on
// (cc-db.sh:53-57).
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

// Carrier reports the carrier file pathname.
func (s *Store) Carrier() string { return s.carrier }

// Degraded reports why the database half is unavailable, or nil.
func (s *Store) Degraded() error { return s.degraded }

// Close releases the database handle. The carrier file holds no handle.
func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Hide records a permanent hide: the database row first, then the carrier file,
// which is cmd_hidden_add's order (cc-db.sh:129-138).
//
// The carrier append is NOT conditional on the database write succeeding, and
// that is deliberate — cc-db.sh runs `have_db && _q "INSERT…"` and then
// `_leg_add` unconditionally, so a hide survives a busy or missing database.
// The returned error reports the database half; the hide is already durable.
func (s *Store) Hide(ctx context.Context, id string, hiddenAt int64) error {
	var writeError error
	if s.db != nil {
		// COALESCE, not assignment: a second hide must not blank a payload an
		// earlier one supplied, whichever order they land in (cc-db.sh:131-136).
		if _, err := s.db.ExecContext(ctx, `
INSERT INTO hidden(uuid,hidden_at,at_payload) VALUES(?,?,NULL)
ON CONFLICT(uuid) DO UPDATE SET
  hidden_at=excluded.hidden_at,
  at_payload=COALESCE(excluded.at_payload, hidden.at_payload)`,
			id,
			hiddenAt,
		); err != nil {
			writeError = fmt.Errorf("record shared hide %q: %w", id, err)
		}
	}
	if err := carrierAdd(s.carrier, id); err != nil {
		return errors.Join(writeError, err)
	}
	return writeError
}

// Unhide is cmd_hidden_del (cc-db.sh:139-143): delete the row, then rewrite the
// carrier file without the id. Unhide is the ONLY removal in this package.
func (s *Store) Unhide(ctx context.Context, id string) error {
	var writeError error
	if s.db != nil {
		if _, err := s.db.ExecContext(
			ctx,
			"DELETE FROM hidden WHERE uuid=?",
			id,
		); err != nil {
			writeError = fmt.Errorf("remove shared hide %q: %w", id, err)
		}
	}
	if err := carrierDelete(s.carrier, id); err != nil {
		return errors.Join(writeError, err)
	}
	return writeError
}

// HiddenAt returns the effective hidden set: every database row unioned with
// every carrier id. A carrier-only id reports hidden_at 0 — the file records no
// time, and inventing one would claim knowledge the fleet does not have.
func (s *Store) HiddenAt(ctx context.Context) (map[string]int64, error) {
	hidden := make(map[string]int64)
	if s.db != nil {
		rows, err := s.db.QueryContext(
			ctx,
			"SELECT uuid, hidden_at FROM hidden",
		)
		if err != nil {
			return nil, fmt.Errorf("query shared hides: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			var hiddenAt int64
			if err := rows.Scan(&id, &hiddenAt); err != nil {
				return nil, fmt.Errorf("scan shared hide: %w", err)
			}
			hidden[id] = hiddenAt
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("iterate shared hides: %w", err)
		}
	}
	ids, err := readCarrier(s.carrier)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		if _, found := hidden[id]; !found {
			hidden[id] = 0
		}
	}
	return hidden, nil
}

// DatabaseHiddenIDs returns only the rows the database holds, sorted. It exists
// for `legacy export`, which rewrites the carrier file from the store.
func (s *Store) DatabaseHiddenIDs(ctx context.Context) ([]string, error) {
	if s.db == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, "SELECT uuid FROM hidden ORDER BY uuid")
	if err != nil {
		return nil, fmt.Errorf("query shared hides: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan shared hide: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate shared hides: %w", err)
	}
	return ids, nil
}

// CarrierIDs returns the carrier file's ids in file order.
func (s *Store) CarrierIDs() ([]string, error) {
	return readCarrier(s.carrier)
}

// RewriteCarrier replaces the carrier file with ids under the same lock
// _leg_del takes. `legacy export` is the only caller.
func (s *Store) RewriteCarrier(ids []string) error {
	return carrierRewrite(s.carrier, ids)
}

// Children returns the teammates a chat spawned, as cmd_child_list does
// (cc-db.sh:287-294). The second result is false when the shared children table
// is unreachable, which is the caller's cue to fall back to the flat files an
// older jail may still hold.
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

// AddChild is cmd_child_add (cc-db.sh:278-286). Nothing in the Go tree spawns
// teammates yet; it exists so the reaper's tests can seed the table the way
// chat.sh does.
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

// ClearChildren is cmd_child_clear (cc-db.sh:296-303): the reaper's counterpart
// to removing the flat file once every teammate is down.
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

// SetMeta writes one meta row with cc-db.sh's updated_at stamp (cc-db.sh:271).
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

// PrimaryAccount reports the primary Claude account exactly as cmd_primary_get
// resolves it (cc-db.sh:262-267): the database value first, the ~/.claude-primary
// mirror when the database has none, and not-found when neither parses. The
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
