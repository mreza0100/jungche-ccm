package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	"hostops/pfm/internal/shared"

	modernsqlite "modernc.org/sqlite"
)

const (
	sqliteBusyCode = 5
	busyRetryDelay = 25 * time.Millisecond

	// effectiveHidden is a per-connection mirror of the shared store's hidden
	// set — database rows unioned with carrier-file ids — so the cached
	// first-frame queries can still express "not hidden" as a join. It is
	// refilled from the shared store on every read that consults it, which is
	// what lets a concurrent hide show up with no sync step in between.
	effectiveHidden = "temp.effective_hidden"

	effectiveHiddenDDL = `
CREATE TABLE IF NOT EXISTS ` + effectiveHidden + `(
  uuid TEXT PRIMARY KEY,
  hidden_at INTEGER NOT NULL
)`
)

// Hide records a permanent hide in the shared store: the SQLite row, then the
// carrier file. A persistent SQLITE_BUSY is warned about and deliberately
// suppressed so hide choreography never aborts.
func (s *Store) Hide(ctx context.Context, hidden Hidden) error {
	return s.hiddenWrite(ctx, "hide", hidden.ID, func() error {
		return s.state.Hide(ctx, hidden.ID, hidden.HiddenAt)
	})
}

// Unhide removes a hide from the shared store and the carrier file. A
// persistent SQLITE_BUSY is warned about and deliberately suppressed so callers
// still exit successfully.
func (s *Store) Unhide(ctx context.Context, id string) error {
	return s.hiddenWrite(ctx, "unhide", id, func() error {
		return s.state.Unhide(ctx, id)
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

	// The carrier file was written either way, whether or not the row lands. A
	// hide survives this and an unhide does not: the row the delete missed is
	// still in the database, and the union will keep reporting it hidden.
	s.warningf(
		"WARNING: pfm could not %s %q in %s: SQLite remained busy after "+
			"retry; %s still carries the change but the database row was NOT "+
			"written\n",
		action,
		id,
		s.state.Path(),
		s.state.Carrier(),
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

// UpsertHidden inserts or replaces a row in the RETIRED local hidden table.
// Only the v1→v2 Codex lineage migration calls it, and only to tidy rows that
// the shared store adopts during migration; no live hide reaches this table.
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

// Hidden reports whether one chat is hidden, by exact chat ID.
func (s *Store) Hidden(ctx context.Context, id string) (Hidden, bool, error) {
	hiddenAt, err := s.state.HiddenAt(ctx)
	if err != nil {
		return Hidden{}, false, err
	}
	at, found := hiddenAt[id]
	if !found {
		return Hidden{}, false, nil
	}
	engines, err := s.deriveEngines(ctx, []string{id})
	if err != nil {
		return Hidden{}, false, err
	}
	return Hidden{ID: id, Engine: engines[id], HiddenAt: at}, true, nil
}

// HiddenChats returns the effective hidden set — shared database rows unioned
// with carrier-file ids — ordered by chat ID.
func (s *Store) HiddenChats(ctx context.Context) ([]Hidden, error) {
	hiddenAt, err := s.state.HiddenAt(ctx)
	if err != nil {
		return nil, err
	}
	ids := shared.SortedIDs(hiddenAt)
	engines, err := s.deriveEngines(ctx, ids)
	if err != nil {
		return nil, err
	}
	hiddenChats := make([]Hidden, 0, len(ids))
	for _, id := range ids {
		hiddenChats = append(hiddenChats, Hidden{
			ID:       id,
			Engine:   engines[id],
			HiddenAt: hiddenAt[id],
		})
	}
	if len(hiddenChats) == 0 {
		return nil, nil
	}
	return hiddenChats, nil
}

// deriveEngines answers "Claude or Codex?" from the index rather than from a
// stored column. The shared hidden table is keyed by uuid alone, so the engine
// is read back out of whichever index table claims the id:
// a transcript is "cc", a rollout or a Codex lineage root is "cx". An id
// neither table knows is an orphaned hide and keeps an empty engine, which
// compose reads as "hidden whatever the engine".
func (s *Store) deriveEngines(
	ctx context.Context,
	ids []string,
) (map[string]string, error) {
	engines := make(map[string]string, len(ids))
	if len(ids) == 0 {
		return engines, nil
	}
	for start := 0; start < len(ids); start += BatchSize {
		end := min(start+BatchSize, len(ids))
		chunk := ids[start:end]
		if err := s.deriveEngineChunk(ctx, chunk, engines); err != nil {
			return nil, err
		}
	}
	return engines, nil
}

func (s *Store) deriveEngineChunk(
	ctx context.Context,
	ids []string,
	engines map[string]string,
) error {
	marks := placeholders(len(ids))
	query := `
SELECT uuid, 'cc' FROM transcripts WHERE uuid IN (` + marks + `)
UNION ALL
SELECT id, 'cx' FROM rollouts WHERE id IN (` + marks + `)
UNION ALL
SELECT lineage_root, 'cx' FROM rollouts WHERE lineage_root IN (` + marks + `)
UNION ALL
SELECT id, 'cx' FROM cx_names WHERE id IN (` + marks + `)`
	// One bound id list per IN clause: four clauses, four copies.
	arguments := make([]any, 0, len(ids)*4)
	for clause := 0; clause < 4; clause++ {
		for _, id := range ids {
			arguments = append(arguments, id)
		}
	}

	rows, err := s.db.QueryContext(ctx, query, arguments...)
	if err != nil {
		return fmt.Errorf("derive hidden chat engines: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, engine string
		if err := rows.Scan(&id, &engine); err != nil {
			return fmt.Errorf("scan hidden chat engine: %w", err)
		}
		// A transcript wins a collision, whatever order the union arms come
		// back in: SQL does not promise that order, and an id that resolved
		// one way this run and the other way next run would flicker a chat in
		// and out of the listing.
		_, alreadyDerived := engines[id]
		if engine == ClaudeEngine || !alreadyDerived {
			engines[id] = engine
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate hidden chat engines: %w", err)
	}
	return nil
}

// scanHidden reads a row of the RETIRED local hidden table, which still has the
// engine and baseline_prompts columns. The v1→v2 Codex lineage migration and
// the orphan report are its only readers.
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

func placeholders(count int) string {
	if count <= 0 {
		return "NULL"
	}
	marks := make([]byte, 0, count*2-1)
	for index := 0; index < count; index++ {
		if index > 0 {
			marks = append(marks, ',')
		}
		marks = append(marks, '?')
	}
	return string(marks)
}

// syncEffectiveHidden refills the per-connection mirror of the shared hidden
// set. Every query that joins against effectiveHidden calls this first: the
// mirror is a query convenience, never a second copy of the truth.
func (s *Store) syncEffectiveHidden(ctx context.Context) error {
	hiddenAt, err := s.state.HiddenAt(ctx)
	if err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, effectiveHiddenDDL); err != nil {
		return fmt.Errorf("create effective hidden mirror: %w", err)
	}
	if _, err := s.db.ExecContext(
		ctx,
		"DELETE FROM "+effectiveHidden,
	); err != nil {
		return fmt.Errorf("clear effective hidden mirror: %w", err)
	}
	ids := make([]string, 0, len(hiddenAt))
	for id := range hiddenAt {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for start := 0; start < len(ids); start += BatchSize {
		end := min(start+BatchSize, len(ids))
		arguments := make([]any, 0, (end-start)*2)
		values := make([]byte, 0, (end-start)*8)
		for index := start; index < end; index++ {
			if index > start {
				values = append(values, ',')
			}
			values = append(values, "(?,?)"...)
			arguments = append(arguments, ids[index], hiddenAt[ids[index]])
		}
		if _, err := s.db.ExecContext(
			ctx,
			"INSERT INTO "+effectiveHidden+"(uuid,hidden_at) VALUES "+string(values),
			arguments...,
		); err != nil {
			return fmt.Errorf("fill effective hidden mirror: %w", err)
		}
	}
	return nil
}
