package professor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

const BaselineVersion = 1

type Baseline struct {
	Version   int                `json:"version"`
	Blueprint BlueprintPin       `json:"blueprint"`
	Files     map[string]FilePin `json:"files"`
	Ignored   []string           `json:"ignored,omitempty"`
}

type BlueprintPin struct {
	Version string `json:"version"`
	SHA     string `json:"sha"`
}

type FilePin struct {
	Template     string `json:"template"`
	TemplateHash string `json:"templateHash"`
	PinnedSHA    string `json:"pinnedSha"`
	PinnedAt     string `json:"pinnedAt"`
}

func BaselinePath(root string) string {
	return filepath.Join(root, ".professor", "baseline.json")
}

func Load(root string) (Baseline, error) {
	path := BaselinePath(root)
	raw, err := readStoreFile(path)
	if err != nil {
		return Baseline{}, fmt.Errorf("UNREADABLE %s: %w", path, err)
	}
	var baseline Baseline
	if err := json.Unmarshal(raw, &baseline); err != nil {
		return Baseline{}, fmt.Errorf("BASELINE-MALFORMED %s: %w", path, err)
	}
	if baseline.Version != BaselineVersion {
		return Baseline{}, fmt.Errorf("BASELINE-VERSION %d: unsupported", baseline.Version)
	}
	if baseline.Files == nil {
		baseline.Files = make(map[string]FilePin)
	}
	baseline.Ignored = normalizeIgnored(baseline.Ignored)
	return baseline, nil
}

// normalizeIgnored returns a sorted, duplicate-free copy of an Ignored list
// (nil for an empty result, so json:",omitempty" drops it cleanly).
func normalizeIgnored(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	sorted := append([]string(nil), values...)
	sort.Strings(sorted)
	deduped := make([]string, 0, len(sorted))
	for index, value := range sorted {
		if index == 0 || value != sorted[index-1] {
			deduped = append(deduped, value)
		}
	}
	return deduped
}

func Save(root string, baseline Baseline) (resultErr error) {
	if baseline.Version != BaselineVersion {
		return fmt.Errorf("BASELINE-VERSION %d: unsupported", baseline.Version)
	}
	if baseline.Files == nil {
		baseline.Files = make(map[string]FilePin)
	}
	baseline.Ignored = normalizeIgnored(baseline.Ignored)
	raw, err := json.MarshalIndent(baseline, "", "  ")
	if err != nil {
		return fmt.Errorf("encode baseline: %w", err)
	}
	raw = append(raw, '\n')
	path := BaselinePath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create baseline directory %s: %w", filepath.Dir(path), err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".baseline-")
	if err != nil {
		return fmt.Errorf("create baseline temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() {
		if removeErr := os.Remove(temporaryPath); removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
			resultErr = errors.Join(resultErr, fmt.Errorf("remove baseline temporary file %s: %w", temporaryPath, removeErr))
		}
	}()
	if err := temporary.Chmod(0o644); err != nil {
		return errors.Join(fmt.Errorf("chmod baseline temporary file: %w", err), temporary.Close())
	}
	if _, err := temporary.Write(raw); err != nil {
		return errors.Join(fmt.Errorf("write baseline temporary file: %w", err), temporary.Close())
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close baseline temporary file: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace baseline %s: %w", path, err)
	}
	return nil
}
