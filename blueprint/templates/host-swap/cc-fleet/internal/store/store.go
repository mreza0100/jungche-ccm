package store

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"hostops/cc-fleet/internal/paths"
	"hostops/cc-fleet/internal/shared"

	_ "modernc.org/sqlite"
)

const (
	// SchemaVersion is the newest database schema understood by this binary.
	SchemaVersion = 2

	driverName = "sqlite"
)

//go:embed schema.sql
var schemaV1 string

//go:embed migration_v2.sql
var schemaV2 string

var migrations = [...]string{
	schemaV1,
	schemaV2,
}

// Store is a single-connection handle to the cc-fleet SQLite database.
//
// It holds TWO databases, and the split is the whole point. `db` is this
// binary's private cache — transcripts, rollouts, Codex names — every row of it
// derived from files on disk and rebuildable by a rescan. `state` is the
// fleet's shared store, the same ~/.cc/fleet.db and ~/.claude/.cc-ls-hidden
// that cc-db.sh writes, and it holds what a PERSON decided: the hides, the
// teammates, the primary account. A decision that lived in the private cache
// was a decision the zsh half could not see, and re-bridging the two by hand is
// what this split ends.
type Store struct {
	db    *sql.DB
	state *shared.Store
	path  string

	warnMu sync.Mutex
	warn   io.Writer
}

type openOptions struct {
	warn io.Writer
}

// OpenOption customizes process-local Store behavior.
type OpenOption func(*openOptions)

// WithWarningWriter directs non-fatal busy warnings to w. The default is
// os.Stderr.
func WithWarningWriter(w io.Writer) OpenOption {
	return func(options *openOptions) {
		if w != nil {
			options.warn = w
		}
	}
}

// Open opens the database selected by internal/paths and applies migrations
// and connection policy.
func Open(options ...OpenOption) (*Store, error) {
	return OpenContext(context.Background(), options...)
}

// OpenContext is Open with caller-controlled cancellation.
func OpenContext(ctx context.Context, options ...OpenOption) (*Store, error) {
	resolved, err := paths.Resolve()
	if err != nil {
		return nil, fmt.Errorf("resolve store paths: %w", err)
	}

	settings := openOptions{warn: os.Stderr}
	for _, option := range options {
		option(&settings)
	}

	if err := os.MkdirAll(filepath.Dir(resolved.DB), 0o700); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	db, err := sql.Open(driverName, resolved.DB)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	store := &Store{
		db:    db,
		state: shared.Open(ctx, resolved),
		path:  resolved.DB,
		warn:  settings.warn,
	}
	if err := store.applyPragmas(ctx); err != nil {
		return nil, errors.Join(err, store.Close())
	}
	if err := store.migrate(ctx); err != nil {
		return nil, errors.Join(err, store.Close())
	}
	if degraded := store.state.Degraded(); degraded != nil {
		store.warningf(
			"WARNING: cc-fleet could not open the shared state store at %s: %v; "+
				"hides fall back to %s alone\n",
			store.state.Path(),
			degraded,
			store.state.Carrier(),
		)
	}
	if err := store.adoptLocalHides(ctx); err != nil {
		return nil, errors.Join(err, store.Close())
	}
	return store, nil
}

// SharedPath reports the shared state database this Store writes hides to.
func (s *Store) SharedPath() string { return s.state.Path() }

// CarrierPath reports the carrier file this Store writes hides through.
func (s *Store) CarrierPath() string { return s.state.Carrier() }

// SharedDegraded reports why the shared database half is unavailable, or nil.
func (s *Store) SharedDegraded() error { return s.state.Degraded() }

// Shared exposes the shared state store for the few callers that need it
// directly: the teammate reaper, and `legacy import`/`export`.
func (s *Store) Shared() *shared.Store { return s.state }

func (s *Store) applyPragmas(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, "PRAGMA busy_timeout=10000"); err != nil {
		return fmt.Errorf("set sqlite busy_timeout: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		return fmt.Errorf("enable sqlite foreign keys: %w", err)
	}

	var journalMode string
	if err := s.db.QueryRowContext(ctx, "PRAGMA journal_mode=WAL").Scan(&journalMode); err != nil {
		return fmt.Errorf("enable sqlite WAL: %w", err)
	}
	if !strings.EqualFold(journalMode, "wal") {
		return fmt.Errorf("enable sqlite WAL: journal_mode is %q", journalMode)
	}

	if _, err := s.db.ExecContext(ctx, "PRAGMA synchronous=NORMAL"); err != nil {
		return fmt.Errorf("set sqlite synchronous mode: %w", err)
	}
	return nil
}

func (s *Store) migrate(ctx context.Context) error {
	return s.WithImmediateTx(ctx, func(tx *ImmediateTx) error {
		version, err := userVersion(ctx, tx)
		if err != nil {
			return err
		}
		if version > SchemaVersion {
			return fmt.Errorf(
				"database schema version %d is newer than supported version %d",
				version,
				SchemaVersion,
			)
		}

		startingVersion := version
		for next := version + 1; next <= SchemaVersion; next++ {
			if _, err := tx.ExecContext(ctx, migrations[next-1]); err != nil {
				return fmt.Errorf("apply database migration %d: %w", next, err)
			}
			if _, err := tx.ExecContext(
				ctx,
				fmt.Sprintf("PRAGMA user_version=%d", next),
			); err != nil {
				return fmt.Errorf("set database schema version %d: %w", next, err)
			}
		}
		if startingVersion < 2 {
			if err := migrateCodexLineageHides(ctx, tx); err != nil {
				return fmt.Errorf("migrate Codex lineage hides: %w", err)
			}
		}
		return nil
	})
}

// adoptedHidesMeta marks the one-time move of hides out of this binary's
// private cache and into the fleet's shared store.
const adoptedHidesMeta = "shared_hidden_adopted"

// adoptLocalHides merges, once, the hides this binary used to keep to itself
// into the shared store and its carrier file, then stops consulting the local
// table for good.
//
// UNIONS ONLY, IN BOTH DIRECTIONS. Local rows become shared rows; carrier ids
// the shared database has never seen become shared rows too, so cc-db.sh —
// which reads the database alone once the file exists (cc-db.sh:106-109) —
// agrees with this binary about what is hidden. NOTHING is deleted: a hide is
// permanent and an unhide is the only removal, so a startup that could drop a
// row is a startup that could lose a decision.
//
// The v1 `hidden` table is left in place, populated, and unread. It costs a few
// kilobytes and it is the rollback: an older binary still finds its hides
// there. `cc-fleet doctor` reports it; nothing else looks at it.
func (s *Store) adoptLocalHides(ctx context.Context) error {
	done, found, err := s.Meta(ctx, adoptedHidesMeta)
	if err != nil {
		return err
	}
	if found && done == "1" {
		return nil
	}

	rows, err := s.db.QueryContext(ctx, "SELECT id, hidden_at FROM hidden")
	if err != nil {
		return fmt.Errorf("read hides awaiting adoption: %w", err)
	}
	adopted := make(map[string]int64)
	for rows.Next() {
		var id string
		var hiddenAt int64
		if err := rows.Scan(&id, &hiddenAt); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan hide awaiting adoption: %w", err)
		}
		adopted[id] = hiddenAt
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("read hides awaiting adoption: %w", err)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read hides awaiting adoption: %w", err)
	}

	existing, err := s.state.HiddenAt(ctx)
	if err != nil {
		return err
	}
	for _, id := range shared.SortedIDs(adopted) {
		if _, alreadyShared := existing[id]; alreadyShared {
			continue
		}
		if err := s.state.Hide(ctx, id, adopted[id]); err != nil {
			return err
		}
	}
	// A carrier id the database never recorded: hidden_at 0 because the file
	// keeps no time, and inventing "now" would date a years-old hide today.
	carrier, err := s.state.CarrierIDs()
	if err != nil {
		return err
	}
	databaseIDs, err := s.state.DatabaseHiddenIDs(ctx)
	if err != nil {
		return err
	}
	inDatabase := make(map[string]struct{}, len(databaseIDs))
	for _, id := range databaseIDs {
		inDatabase[id] = struct{}{}
	}
	for _, id := range carrier {
		if _, found := inDatabase[id]; found {
			continue
		}
		if err := s.state.Hide(ctx, id, 0); err != nil {
			return err
		}
		inDatabase[id] = struct{}{}
	}
	return s.SetMeta(ctx, adoptedHidesMeta, "1")
}

func userVersion(ctx context.Context, query rowQueryer) (int, error) {
	var version int
	if err := query.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return 0, fmt.Errorf("read database schema version: %w", err)
	}
	return version, nil
}

// UserVersion reports PRAGMA user_version.
func (s *Store) UserVersion(ctx context.Context) (int, error) {
	return userVersion(ctx, s.db)
}

// Path reports the database pathname resolved when the Store was opened.
func (s *Store) Path() string {
	return s.path
}

// Stats reports database/sql pool statistics.
func (s *Store) Stats() sql.DBStats {
	return s.db.Stats()
}

// Close closes both SQLite handles.
func (s *Store) Close() error {
	return errors.Join(s.db.Close(), s.state.Close())
}

func (s *Store) warningf(format string, args ...any) {
	s.warnMu.Lock()
	defer s.warnMu.Unlock()
	fmt.Fprintf(s.warn, format, args...)
}
