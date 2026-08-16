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

// orphanedHiddenSource is the single definition of an orphaned hide: a chat in
// the shared store's hidden set that resolves to no transcript and no rollout,
// directly or through a Codex lineage root. Counts, the listing, and the prune
// all read it, so doctor and `hidden --prune-orphans` can never disagree.
//
// It reads the effective mirror, so a concurrent external hide counts here too;
// every caller refills that mirror first. The engine test the v1 query carried
// is gone with the column: an id matching a lineage root is a live Codex hide
// whatever engine anyone once recorded for it.
const orphanedHiddenSource = `
FROM ` + effectiveHidden + ` h
WHERE NOT EXISTS (SELECT 1 FROM transcripts t WHERE t.uuid=h.uuid)
  AND NOT EXISTS (
    SELECT 1 FROM rollouts r
    WHERE r.id=h.uuid OR r.lineage_root=h.uuid
  )`

// QuickCheck runs SQLite's bounded integrity probe.
func (s *Store) QuickCheck(ctx context.Context) (string, error) {
	var result string
	if err := s.db.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&result); err != nil {
		return "", fmt.Errorf("SQLite quick_check: %w", err)
	}
	return result, nil
}

// Counts reports table sizes and hides whose target row vanished. Hidden and
// OrphanedHides come from the shared store; the rest are this binary's cache.
func (s *Store) Counts(ctx context.Context) (RowCounts, error) {
	if err := s.syncEffectiveHidden(ctx); err != nil {
		return RowCounts{}, err
	}
	var counts RowCounts
	queries := []struct {
		query string
		value *int
	}{
		{"SELECT count(*) FROM transcripts", &counts.Transcripts},
		{"SELECT count(*) FROM rollouts", &counts.Rollouts},
		{"SELECT count(*) FROM cx_names", &counts.CxNames},
		{"SELECT count(*) FROM " + effectiveHidden, &counts.Hidden},
		{"SELECT count(*) " + orphanedHiddenSource, &counts.OrphanedHides},
	}
	for _, query := range queries {
		if err := s.db.QueryRowContext(ctx, query.query).Scan(query.value); err != nil {
			return RowCounts{}, fmt.Errorf("count fleet rows: %w", err)
		}
	}
	return counts, nil
}

// OrphanedHides lists the hides Counts reports as OrphanedHides, ordered by
// chat ID. The engine is derived, and for an orphan it derives empty by
// definition: no index row is left to name it.
func (s *Store) OrphanedHides(ctx context.Context) ([]Hidden, error) {
	if err := s.syncEffectiveHidden(ctx); err != nil {
		return nil, err
	}
	ids, err := s.orphanedHideIDs(ctx)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	hiddenAt, err := s.state.HiddenAt(ctx)
	if err != nil {
		return nil, err
	}
	orphans := make([]Hidden, 0, len(ids))
	for _, id := range ids {
		orphans = append(orphans, Hidden{ID: id, HiddenAt: hiddenAt[id]})
	}
	return orphans, nil
}

func (s *Store) orphanedHideIDs(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(
		ctx,
		"SELECT h.uuid "+orphanedHiddenSource+" ORDER BY h.uuid",
	)
	if err != nil {
		return nil, fmt.Errorf("query orphaned hidden chats: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan orphaned hidden chat: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate orphaned hidden chats: %w", err)
	}
	return ids, nil
}

// DeleteOrphanedHides removes exactly the hides OrphanedHides lists and reports
// how many were removed.
//
// This is the one path besides an unhide that takes a row out of the shared
// store, and it is gated behind `hidden --prune-orphans --confirm`. Each removal
// goes through the ordinary unhide, so the carrier file loses the id too: half a
// removal is the drift this store exists to end. There is no transaction around
// the set: holding its write lock across hundreds of rows would stall every
// other chat.
func (s *Store) DeleteOrphanedHides(ctx context.Context) (int, error) {
	if err := s.syncEffectiveHidden(ctx); err != nil {
		return 0, err
	}
	ids, err := s.orphanedHideIDs(ctx)
	if err != nil {
		return 0, err
	}
	deleted := 0
	for _, id := range ids {
		if err := s.state.Unhide(ctx, id); err != nil {
			return deleted, err
		}
		deleted++
	}
	return deleted, nil
}
