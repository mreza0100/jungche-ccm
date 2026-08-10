package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	modernsqlite "modernc.org/sqlite"
)

const (
	sqliteBusyCode = 5
	busyRetryDelay = 25 * time.Millisecond
)

// Hide inserts or replaces a hide row. A persistent SQLITE_BUSY is warned
// about and deliberately suppressed so hide choreography never aborts.
func (s *Store) Hide(ctx context.Context, hidden Hidden) error {
	return s.hiddenWrite(ctx, "hide", hidden.ID, func() error {
		return upsertHidden(ctx, s.db, hidden)
	})
}

// Unhide removes a hide row. A persistent SQLITE_BUSY is warned about and
// deliberately suppressed so callers still exit successfully.
func (s *Store) Unhide(ctx context.Context, id string) error {
	return s.hiddenWrite(ctx, "unhide", id, func() error {
		if _, err := s.db.ExecContext(ctx, "DELETE FROM hidden WHERE id=?", id); err != nil {
			return fmt.Errorf("delete hidden chat %q: %w", id, err)
		}
		return nil
	})
}

func (s *Store) hiddenWrite(
	ctx context.Context,
	action string,
	id string,
	write func() error,
) error {
	err := write()
	if !isBusy(err) {
		return err
	}

	timer := time.NewTimer(busyRetryDelay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
	}

	err = write()
	if !isBusy(err) {
		return err
	}

	s.warningf(
		"WARNING: cc-fleet could not %s %q: SQLite remained busy after retry; "+
			"continuing without failing and the change was NOT saved\n",
		action,
		id,
	)
	counterCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_ = s.IncrementMeta(counterCtx, "busy_"+action+"_warnings")
	return nil
}

func isBusy(err error) bool {
	var sqliteError *modernsqlite.Error
	return errors.As(err, &sqliteError) &&
		sqliteError.Code()&0xff == sqliteBusyCode
}

// UpsertHidden inserts or replaces a hide row within tx. Busy handling is
// owned by the caller of the outer transaction.
func (tx *ImmediateTx) UpsertHidden(ctx context.Context, hidden Hidden) error {
	return upsertHidden(ctx, tx, hidden)
}

func upsertHidden(ctx context.Context, db queryExecer, hidden Hidden) error {
	_, err := execWrite(ctx, db, `
INSERT INTO hidden (id, engine, hidden_at, baseline_prompts)
VALUES (?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  engine=excluded.engine,
  hidden_at=excluded.hidden_at,
  baseline_prompts=excluded.baseline_prompts`,
		hidden.ID,
		hidden.Engine,
		hidden.HiddenAt,
		hidden.BaselinePrompts,
	)
	if err != nil {
		return fmt.Errorf("upsert hidden chat %q: %w", hidden.ID, err)
	}
	return nil
}

// Hidden returns a hide row by exact chat ID.
func (s *Store) Hidden(ctx context.Context, id string) (Hidden, bool, error) {
	hidden, err := scanHidden(s.db.QueryRowContext(
		ctx,
		`SELECT id, engine, hidden_at, baseline_prompts FROM hidden WHERE id=?`,
		id,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return Hidden{}, false, nil
	}
	if err != nil {
		return Hidden{}, false, fmt.Errorf("query hidden chat %q: %w", id, err)
	}
	return hidden, true, nil
}

// HiddenChats returns all hide rows ordered by chat ID.
func (s *Store) HiddenChats(ctx context.Context) ([]Hidden, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, engine, hidden_at, baseline_prompts FROM hidden ORDER BY id`,
	)
	if err != nil {
		return nil, fmt.Errorf("query hidden chats: %w", err)
	}
	defer rows.Close()

	var hiddenChats []Hidden
	for rows.Next() {
		hidden, err := scanHidden(rows)
		if err != nil {
			return nil, fmt.Errorf("scan hidden chat: %w", err)
		}
		hiddenChats = append(hiddenChats, hidden)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate hidden chats: %w", err)
	}
	return hiddenChats, nil
}

func scanHidden(row rowScanner) (Hidden, error) {
	var hidden Hidden
	var baseline sql.NullInt64
	err := row.Scan(
		&hidden.ID,
		&hidden.Engine,
		&hidden.HiddenAt,
		&baseline,
	)
	if baseline.Valid {
		hidden.BaselinePrompts = &baseline.Int64
	}
	return hidden, err
}
