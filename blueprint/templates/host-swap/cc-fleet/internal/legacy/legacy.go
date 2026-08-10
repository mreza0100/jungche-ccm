// Package legacy converts the retired flat-file hide list to and from the
// SQLite representation without modifying source files during import.
package legacy

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"hostops/cc-fleet/internal/paths"
	"hostops/cc-fleet/internal/store"
)

const ImportDoneMeta = "legacy_import_done"

// Result summarizes a legacy conversion.
type Result struct {
	Imported int
	// AlreadyActive stays zero: a hide is permanent, so a legacy id whose
	// chat grew still imports as hidden. The counter survives for the
	// command's output format.
	AlreadyActive int
	Unknown       int
	Exported      int
}

// Import converts ~/.claude/.cc-ls-hidden. Every listed id that is indexed
// imports as hidden, whatever the retired .at byte baselines claim. The legacy
// files are opened read-only and left byte-for-byte untouched.
func Import(ctx context.Context, database *store.Store) (Result, error) {
	if database == nil {
		return Result{}, errors.New("legacy store is nil")
	}
	resolved, err := paths.Resolve()
	if err != nil {
		return Result{}, err
	}
	hiddenPath := filepath.Join(resolved.Home, ".claude", ".cc-ls-hidden")
	ids, err := readHiddenIDs(hiddenPath)
	if err != nil {
		return Result{}, err
	}

	result := Result{}
	now := time.Now().Unix()
	for _, id := range ids {
		targetID, engine, found, err := indexedTarget(ctx, database, id)
		if err != nil {
			return result, err
		}
		if !found {
			result.Unknown++
			continue
		}
		if err := database.Hide(ctx, store.Hidden{
			ID:       targetID,
			Engine:   engine,
			HiddenAt: now,
		}); err != nil {
			return result, err
		}
		result.Imported++
	}
	if err := database.SetMeta(ctx, ImportDoneMeta, "1"); err != nil {
		return result, err
	}
	return result, nil
}

// Export atomically rewrites the two legacy hide files from current SQLite
// state. The .at file keeps carrying the targets' current sizes for any old
// reader, even though import no longer consults it.
func Export(ctx context.Context, database *store.Store) (Result, error) {
	if database == nil {
		return Result{}, errors.New("legacy store is nil")
	}
	resolved, err := paths.Resolve()
	if err != nil {
		return Result{}, err
	}
	rows, err := database.HiddenChats(ctx)
	if err != nil {
		return Result{}, err
	}
	sort.Slice(rows, func(left, right int) bool {
		return rows[left].ID < rows[right].ID
	})

	var hiddenContent strings.Builder
	var baselineContent strings.Builder
	for _, row := range rows {
		hiddenContent.WriteString(row.ID)
		hiddenContent.WriteByte('\n')
		size := int64(0)
		if row.Engine == "cc" {
			if transcript, found, err := database.Transcript(ctx, row.ID); err != nil {
				return Result{}, err
			} else if found {
				size = transcript.Size
			}
		} else if row.Engine == "cx" {
			if lineage, found, err := database.CodexLineage(
				ctx,
				row.ID,
			); err != nil {
				return Result{}, err
			} else if found {
				size = lineage.Newest.Size
			} else if rollout, found, err := database.Rollout(
				ctx,
				row.ID,
			); err != nil {
				return Result{}, err
			} else if found {
				size = rollout.Size
			}
		}
		fmt.Fprintf(&baselineContent, "%s\t%d\n", row.ID, size)
	}

	hiddenPath := filepath.Join(resolved.Home, ".claude", ".cc-ls-hidden")
	if err := atomicWrite(hiddenPath, []byte(hiddenContent.String())); err != nil {
		return Result{}, err
	}
	if err := atomicWrite(hiddenPath+".at", []byte(baselineContent.String())); err != nil {
		return Result{}, err
	}
	return Result{Exported: len(rows)}, nil
}

func readHiddenIDs(path string) ([]string, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read legacy hidden file: %w", err)
	}
	seen := make(map[string]struct{})
	var ids []string
	for _, line := range strings.Split(string(content), "\n") {
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

func atomicWrite(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create legacy directory: %w", err)
	}
	file, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create legacy temporary file: %w", err)
	}
	tempPath := file.Name()
	defer os.Remove(tempPath)
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return err
	}
	if _, err := file.Write(content); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace legacy file: %w", err)
	}
	return nil
}
