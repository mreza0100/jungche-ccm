// Package legacy repairs the two halves of the shared hidden set against each
// other: the carrier file ~/.claude/.cc-ls-hidden and the `hidden` table in
// ~/.cc/fleet.db.
//
// Both commands are near-no-ops in normal operation, and that is the point.
// Every hide now writes the database row AND the carrier line in the same call,
// so the two cannot drift on their own. They drift when something outside that
// path touches one of them — a database restored from backup, a carrier file
// edited by hand, a run where SQLite was unopenable and only the file was
// written. Import and Export are the repair for exactly those, kept for the day
// it happens and no longer part of any normal run.
package legacy

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"hostops/pfm/internal/shared"
	"hostops/pfm/internal/store"
)

const ImportDoneMeta = "legacy_import_done"

// Result summarizes a legacy conversion.
type Result struct {
	Imported int
	Unknown  int
	Exported int
}

// Import unions the carrier file into the shared hidden table: every id the
// file lists ends up as a row, and nothing is removed. A Codex id resolves to
// its lineage root first, because that is the id the picker hides and unhides.
//
// Unknown counts the ids no index row claims. They are imported all the same —
// the union is the whole contract, and an id in the carrier file is already
// hidden as far as every reader is concerned — but they are worth reporting,
// because that is what `hidden --prune-orphans` will offer to clear.
//
// The carrier file is never rewritten here. Ids it already holds re-append as
// no-ops; a canonicalized lineage root is appended beside the child id it came
// from, and the child id stays, because removing it is an unhide and Import is
// not an unhide.
func Import(ctx context.Context, database *store.Store) (Result, error) {
	if database == nil {
		return Result{}, errors.New("legacy store is nil")
	}
	state := database.Shared()
	ids, err := carrierIDs(state)
	if err != nil {
		return Result{}, err
	}

	result := Result{}
	now := time.Now().Unix()
	for _, id := range ids {
		targetID, _, found, err := indexedTarget(ctx, database, id)
		if err != nil {
			return result, err
		}
		if !found {
			result.Unknown++
			targetID = id
		}
		if err := state.Hide(ctx, targetID, now); err != nil {
			return result, err
		}
		result.Imported++
	}
	if err := database.SetMeta(ctx, ImportDoneMeta, "1"); err != nil {
		return result, err
	}
	return result, nil
}

// Export rewrites the carrier file from the shared hidden table under the
// carrier lock.
//
// It unions the file into the table first. A whole-file rewrite is the one
// operation here that can DELETE a hide, and a hide the file holds but the
// table does not is precisely the state a repair command is called for — an
// export that dropped it would destroy the thing it was run to save. After the
// union the rewrite is lossless by construction.
//
// Only the id list is written: the retired .at byte-baseline file fed the
// auto-unhide ratchet, which no longer exists, and nothing reads it.
func Export(ctx context.Context, database *store.Store) (Result, error) {
	if database == nil {
		return Result{}, errors.New("legacy store is nil")
	}
	state := database.Shared()
	if _, err := Import(ctx, database); err != nil {
		return Result{}, err
	}
	ids, err := state.DatabaseHiddenIDs(ctx)
	if err != nil {
		return Result{}, err
	}
	sort.Strings(ids)
	if err := state.RewriteCarrier(ids); err != nil {
		return Result{}, err
	}
	return Result{Exported: len(ids)}, nil
}

// carrierIDs reads the carrier file's ids, deduplicated and sorted so an import
// is deterministic whatever order the file grew in.
func carrierIDs(state *shared.Store) ([]string, error) {
	lines, err := state.CarrierIDs()
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(lines))
	ids := make([]string, 0, len(lines))
	for _, line := range lines {
		id := strings.TrimSpace(line)
		if id == "" {
			continue
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

func indexedTarget(
	ctx context.Context,
	database *store.Store,
	id string,
) (targetID, engine string, found bool, err error) {
	_, found, err = database.Transcript(ctx, id)
	if err != nil {
		return "", "", false, err
	}
	if found {
		return id, "cc", true, nil
	}
	_, found, err = database.Rollout(ctx, id)
	if err != nil {
		return "", "", false, err
	}
	if found {
		lineage, lineageFound, err := database.CodexLineage(ctx, id)
		if err != nil {
			return "", "", false, err
		}
		if lineageFound {
			return lineage.RootID, "cx", true, nil
		}
		return id, "cx", true, nil
	}
	return "", "", false, nil
}
