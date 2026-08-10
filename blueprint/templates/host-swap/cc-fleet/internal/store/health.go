package store

import (
	"context"
	"fmt"
)

// RowCounts is the stable database summary used by doctor and jailed
// idempotence checks.
type RowCounts struct {
	Transcripts   int
	Rollouts      int
	CxNames       int
	Hidden        int
	OrphanedHides int
}

// orphanedHiddenSource is the single definition of an orphaned hide row:
// a hidden row whose chat resolves to no transcript and no rollout, directly
// or through a Codex lineage root. Counts, the listing, and the prune all read
// it, so doctor and `hidden --prune-orphans` can never disagree.
const orphanedHiddenSource = `
FROM hidden h
WHERE NOT EXISTS (SELECT 1 FROM transcripts t WHERE t.uuid=h.id)
  AND NOT EXISTS (
    SELECT 1 FROM rollouts r
    WHERE r.id=h.id OR (h.engine='cx' AND r.lineage_root=h.id)
  )`

// QuickCheck runs SQLite's bounded integrity probe.
func (s *Store) QuickCheck(ctx context.Context) (string, error) {
	var result string
	if err := s.db.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&result); err != nil {
		return "", fmt.Errorf("SQLite quick_check: %w", err)
	}
	return result, nil
}

// Counts reports table sizes and hide rows whose target row vanished.
func (s *Store) Counts(ctx context.Context) (RowCounts, error) {
	var counts RowCounts
	queries := []struct {
		query string
		value *int
	}{
		{"SELECT count(*) FROM transcripts", &counts.Transcripts},
		{"SELECT count(*) FROM rollouts", &counts.Rollouts},
		{"SELECT count(*) FROM cx_names", &counts.CxNames},
		{"SELECT count(*) FROM hidden", &counts.Hidden},
		{"SELECT count(*) " + orphanedHiddenSource, &counts.OrphanedHides},
	}
	for _, query := range queries {
		if err := s.db.QueryRowContext(ctx, query.query).Scan(query.value); err != nil {
			return RowCounts{}, fmt.Errorf("count fleet rows: %w", err)
		}
	}
	return counts, nil
}

// OrphanedHides lists the rows Counts reports as OrphanedHides, ordered by
// chat ID.
func (s *Store) OrphanedHides(ctx context.Context) ([]Hidden, error) {
	rows, err := s.db.QueryContext(
		ctx,
		"SELECT h.id, h.engine, h.hidden_at, h.baseline_prompts "+
			orphanedHiddenSource+" ORDER BY h.id",
	)
	if err != nil {
		return nil, fmt.Errorf("query orphaned hidden chats: %w", err)
	}
	defer rows.Close()

	var orphans []Hidden
	for rows.Next() {
		hidden, err := scanHidden(rows)
		if err != nil {
			return nil, fmt.Errorf("scan orphaned hidden chat: %w", err)
		}
		orphans = append(orphans, hidden)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate orphaned hidden chats: %w", err)
	}
	return orphans, nil
}

// DeleteOrphanedHides removes exactly the rows OrphanedHides lists and reports
// how many were deleted.
func (s *Store) DeleteOrphanedHides(ctx context.Context) (int, error) {
	deleted := 0
	err := s.WithImmediateTx(ctx, func(tx *ImmediateTx) error {
		result, err := tx.ExecContext(
			ctx,
			"DELETE FROM hidden WHERE id IN (SELECT h.id "+orphanedHiddenSource+")",
		)
		if err != nil {
			return fmt.Errorf("delete orphaned hidden chats: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("count deleted orphaned hidden chats: %w", err)
		}
		deleted = int(affected)
		return nil
	})
	if err != nil {
		return 0, err
	}
	return deleted, nil
}
