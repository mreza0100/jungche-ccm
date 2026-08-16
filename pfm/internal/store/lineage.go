package store

import (
	"context"
	"fmt"
	"sort"
)

// CodexLineage is one user-visible Codex conversation. Newest supplies the
// row's path, project, size, and activity. PromptCount is the maximum
// cumulative user-prompt count across its rollout files, so merely creating a
// resumed sibling does not inflate the conversation's prompt count.
type CodexLineage struct {
	RootID      string
	Newest      Rollout
	PromptCount int64
	MemberIDs   []string
}

// ResolveCodexLineages returns stable conversation groups and a root lookup
// for every rollout, including non-user descendants used by parent chains.
func ResolveCodexLineages(
	rollouts []Rollout,
) ([]CodexLineage, map[string]string) {
	byID := make(map[string]Rollout, len(rollouts))
	ids := make([]string, 0, len(rollouts))
	for _, rollout := range rollouts {
		if rollout.ID == "" {
			continue
		}
		byID[rollout.ID] = rollout
		ids = append(ids, rollout.ID)
	}
	sort.Strings(ids)

	roots := make(map[string]string, len(byID))
	for _, id := range ids {
		resolveCodexRoot(id, byID, roots)
	}

	groups := make(map[string]*CodexLineage)
	for _, rollout := range rollouts {
		if !rollout.UserThread || rollout.ID == "" {
			continue
		}
		root := roots[rollout.ID]
		if root == "" {
			root = rollout.ID
		}
		lineage := groups[root]
		if lineage == nil {
			lineage = &CodexLineage{RootID: root}
			groups[root] = lineage
		}
		lineage.MemberIDs = append(lineage.MemberIDs, rollout.ID)
		if rollout.PromptCount > lineage.PromptCount {
			lineage.PromptCount = rollout.PromptCount
		}
		if lineage.Newest.ID == "" ||
			rollout.MTimeNS > lineage.Newest.MTimeNS ||
			(rollout.MTimeNS == lineage.Newest.MTimeNS &&
				rollout.ID > lineage.Newest.ID) {
			lineage.Newest = rollout
		}
	}

	rootIDs := make([]string, 0, len(groups))
	for root := range groups {
		rootIDs = append(rootIDs, root)
	}
	sort.Strings(rootIDs)
	lineages := make([]CodexLineage, 0, len(rootIDs))
	for _, root := range rootIDs {
		lineage := groups[root]
		sort.Strings(lineage.MemberIDs)
		lineage.Newest.LineageRoot = root
		lineage.Newest.PromptCount = lineage.PromptCount
		lineages = append(lineages, *lineage)
	}
	return lineages, roots
}

func resolveCodexRoot(
	start string,
	byID map[string]Rollout,
	roots map[string]string,
) string {
	if root := roots[start]; root != "" {
		return root
	}
	path := make([]string, 0, 4)
	positions := make(map[string]int)
	current := start
	root := ""
	for {
		if known := roots[current]; known != "" {
			root = known
			break
		}
		if position, cycle := positions[current]; cycle {
			root = current
			for _, id := range path[position:] {
				if id < root {
					root = id
				}
			}
			break
		}
		positions[current] = len(path)
		path = append(path, current)

		rollout, found := byID[current]
		if !found {
			root = current
			break
		}
		next := lineageParent(rollout)
		if next == "" {
			root = current
			break
		}
		current = next
	}
	for _, id := range path {
		roots[id] = root
	}
	return root
}

func lineageParent(rollout Rollout) string {
	if rollout.SessionID != "" {
		if rollout.SessionID == rollout.ID {
			return ""
		}
		return rollout.SessionID
	}
	if rollout.ParentThread != "" && rollout.ParentThread != rollout.ID {
		return rollout.ParentThread
	}
	return ""
}

func initialLineageRoot(rollout Rollout) string {
	if rollout.LineageRoot != "" {
		return rollout.LineageRoot
	}
	if rollout.SessionID != "" {
		return rollout.SessionID
	}
	if rollout.ParentThread != "" {
		return rollout.ParentThread
	}
	return rollout.ID
}

// CodexLineage returns the conversation containing id.
func (s *Store) CodexLineage(
	ctx context.Context,
	id string,
) (CodexLineage, bool, error) {
	rollouts, err := s.Rollouts(ctx)
	if err != nil {
		return CodexLineage{}, false, err
	}
	lineages, roots := ResolveCodexLineages(rollouts)
	root := roots[id]
	if root == "" {
		root = id
	}
	for _, lineage := range lineages {
		if lineage.RootID == root {
			return lineage, true, nil
		}
	}
	return CodexLineage{}, false, nil
}

// RolloutLineage returns all indexed files in id's conversation.
func (s *Store) RolloutLineage(
	ctx context.Context,
	id string,
) ([]Rollout, error) {
	rollouts, err := s.Rollouts(ctx)
	if err != nil {
		return nil, err
	}
	_, roots := ResolveCodexLineages(rollouts)
	root := roots[id]
	if root == "" {
		root = id
	}
	family := make([]Rollout, 0)
	for _, rollout := range rollouts {
		if roots[rollout.ID] == root {
			family = append(family, rollout)
		}
	}
	return family, nil
}

// ReconcileCodexLineageRoots refreshes denormalized roots after an index pass.
func (s *Store) ReconcileCodexLineageRoots(ctx context.Context) error {
	rollouts, err := s.Rollouts(ctx)
	if err != nil {
		return err
	}
	_, roots := ResolveCodexLineages(rollouts)
	updates := make([]Rollout, 0)
	for _, rollout := range rollouts {
		root := roots[rollout.ID]
		if root != "" && root != rollout.LineageRoot {
			rollout.LineageRoot = root
			updates = append(updates, rollout)
		}
	}
	return s.Batch(ctx, len(updates), func(
		tx *ImmediateTx,
		start, end int,
	) error {
		for _, rollout := range updates[start:end] {
			if _, err := tx.ExecContext(
				ctx,
				"UPDATE rollouts SET lineage_root=? WHERE id=?",
				rollout.LineageRoot,
				rollout.ID,
			); err != nil {
				return fmt.Errorf(
					"update rollout %q lineage root: %w",
					rollout.ID,
					err,
				)
			}
		}
		return nil
	})
}

// migrateCodexLineageHides denormalizes every rollout's lineage root and
// collapses the v1 per-rollout Codex hides onto that root, keeping the newest
// hidden_at. The merged row's baseline_prompts is NULL because these are
// explicit permanent hides, so lineage repair must not invent /clear state.
func migrateCodexLineageHides(
	ctx context.Context,
	tx *ImmediateTx,
) error {
	rollouts, err := queryRollouts(ctx, tx)
	if err != nil {
		return err
	}
	_, roots := ResolveCodexLineages(rollouts)
	for _, rollout := range rollouts {
		root := roots[rollout.ID]
		if root == "" {
			root = initialLineageRoot(rollout)
		}
		if _, err := tx.ExecContext(
			ctx,
			"UPDATE rollouts SET lineage_root=? WHERE id=?",
			root,
			rollout.ID,
		); err != nil {
			return err
		}
	}

	rows, err := tx.QueryContext(ctx, `
SELECT id, engine, hidden_at, baseline_prompts
FROM hidden
WHERE engine='cx'
ORDER BY id`)
	if err != nil {
		return err
	}
	hides := make([]Hidden, 0)
	for rows.Next() {
		hidden, err := scanHidden(rows)
		if err != nil {
			_ = rows.Close()
			return err
		}
		hides = append(hides, hidden)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}

	type migratedHide struct {
		hidden Hidden
		ids    []string
	}
	migrated := make(map[string]*migratedHide)
	for _, hidden := range hides {
		root := roots[hidden.ID]
		if root == "" {
			continue
		}
		target := migrated[root]
		if target == nil {
			target = &migratedHide{hidden: Hidden{
				ID:       root,
				Engine:   CodexEngine,
				HiddenAt: hidden.HiddenAt,
			}}
			migrated[root] = target
		}
		target.ids = append(target.ids, hidden.ID)
		if hidden.HiddenAt > target.hidden.HiddenAt {
			target.hidden.HiddenAt = hidden.HiddenAt
		}
	}
	for _, target := range migrated {
		for _, id := range target.ids {
			if _, err := tx.ExecContext(
				ctx,
				"DELETE FROM hidden WHERE engine='cx' AND id=?",
				id,
			); err != nil {
				return err
			}
		}
		if err := tx.UpsertHidden(ctx, target.hidden); err != nil {
			return err
		}
	}
	return nil
}

func queryRollouts(
	ctx context.Context,
	db queryExecer,
) ([]Rollout, error) {
	rows, err := db.QueryContext(
		ctx,
		"SELECT "+rolloutColumns+" FROM rollouts ORDER BY id",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rollouts := make([]Rollout, 0)
	for rows.Next() {
		rollout, err := scanRollout(rows)
		if err != nil {
			return nil, err
		}
		rollouts = append(rollouts, rollout)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return rollouts, nil
}
