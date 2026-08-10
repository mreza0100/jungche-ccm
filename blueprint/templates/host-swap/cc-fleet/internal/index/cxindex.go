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

	"hostops/cc-fleet/internal/store"
)

type cxIndexRecord struct {
	ID         string `json:"id"`
	ThreadName string `json:"thread_name"`
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

	names := make(map[string]string)
	if size >= 0 {
		_, bytesRead, err := readCompleteLines(path, 0, func(line []byte) {
			var record cxIndexRecord
			if err := json.Unmarshal(line, &record); err == nil && record.ID != "" {
				names[record.ID] = record.ThreadName
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
			if err := tx.UpsertCxName(ctx, store.CxName{
				ID:         id,
				ThreadName: names[id],
			}); err != nil {
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
