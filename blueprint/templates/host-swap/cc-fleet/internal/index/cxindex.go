package index

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"hostops/cc-fleet/internal/store"
)

type cxIndexRecord struct {
	ID         string `json:"id"`
	ThreadName string `json:"thread_name"`
	// UpdatedAt is the rename time Codex 0.147 began stamping on every
	// session_index.jsonl entry. Older entries carry none.
	UpdatedAt string `json:"updated_at"`
}

// parseCxRenameTime reads a session_index.jsonl entry's updated_at. An empty
// or unparsable value means no rename time is known for this entry, not that
// the rename happened at the Unix epoch — reconcileCodexNames treats the two
// cases identically (RenamedAt of 0), so returning 0 here is the correct
// "unknown" sentinel, not a wrong guess.
func parseCxRenameTime(raw string) int64 {
	if raw == "" {
		return 0
	}
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return 0
	}
	return parsed.UnixNano()
}

func reloadCxNames(
	ctx context.Context,
	database *store.Store,
	codexRoot string,
	counters *Counters,
) error {
	path := filepath.Join(codexRoot, "session_index.jsonl")
	size, mtimeNS := int64(-1), int64(-1)
	if info, err := os.Stat(path); err == nil {
		size = info.Size()
		mtimeNS = info.ModTime().UnixNano()
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("stat Codex session index: %w", err)
	}

	oldSize, sizeFound, err := database.Meta(ctx, "cx_index_size")
	if err != nil {
		return err
	}
	oldMTime, mtimeFound, err := database.Meta(ctx, "cx_index_mtime_ns")
	if err != nil {
		return err
	}
	sizeText := strconv.FormatInt(size, 10)
	mtimeText := strconv.FormatInt(mtimeNS, 10)
	if sizeFound && mtimeFound && oldSize == sizeText && oldMTime == mtimeText {
		return nil
	}

	names := make(map[string]store.CxName)
	if size >= 0 {
		_, bytesRead, err := readCompleteLines(path, 0, func(line []byte) {
			var record cxIndexRecord
			if err := json.Unmarshal(line, &record); err == nil && record.ID != "" {
				// The file is append-only, so the LAST entry for an id — the
				// one this overwrite leaves standing — is the freshest rename
				// intent, whether or not it carries a timestamp.
				names[record.ID] = store.CxName{
					ID:         record.ID,
					ThreadName: record.ThreadName,
					Source:     store.CxNameSourceSessionIndex,
					RenamedAt:  parseCxRenameTime(record.UpdatedAt),
				}
			}
		})
		if err != nil {
			return fmt.Errorf("parse Codex session index: %w", err)
		}
		counters.BytesRead += bytesRead
	}

	ids := make([]string, 0, len(names))
	for id := range names {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if err := database.WithImmediateTx(ctx, func(tx *store.ImmediateTx) error {
		if _, err := tx.ExecContext(ctx, "DELETE FROM cx_names"); err != nil {
			return fmt.Errorf("clear Codex names: %w", err)
		}
		for _, id := range ids {
			if err := tx.UpsertCxName(ctx, names[id]); err != nil {
				return err
			}
		}
		if err := tx.SetMeta(ctx, "cx_index_size", sizeText); err != nil {
			return err
		}
		return tx.SetMeta(ctx, "cx_index_mtime_ns", mtimeText)
	}); err != nil {
		return fmt.Errorf("replace Codex names: %w", err)
	}

	counters.CxNamesReloaded = true
	counters.RowsTouched += len(ids) + 2
	return nil
}
