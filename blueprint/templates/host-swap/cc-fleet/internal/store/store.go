package store

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"hostops/cc-fleet/internal/paths"

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
type Store struct {
	db   *sql.DB
	path string

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
		db:   db,
		path: resolved.DB,
		warn: settings.warn,
	}
	if err := store.applyPragmas(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := store.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

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

// Close closes the SQLite handle.
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) warningf(format string, args ...any) {
	s.warnMu.Lock()
	defer s.warnMu.Unlock()
	fmt.Fprintf(s.warn, format, args...)
}
