// Package chatload enumerates and reads one complete text-file set for the
// CLI and MCP chat_load surfaces.
package chatload

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

type File struct {
	Path  string
	Lines int
	Bytes int
	Text  string
}

type Result struct {
	Files      []File
	Warnings   []string
	TotalLines int
	TotalBytes int
}

// Load reads every non-empty text file beneath targets. maxBytes is a total
// response ceiling; zero leaves the CLI enumeration unbounded. Crossing the
// ceiling fails the whole operation instead of returning a sampled file set.
func Load(targets []string, maxBytes int) (Result, error) {
	if len(targets) == 0 {
		return Result{}, errors.New("at least one file or directory is required")
	}
	unique := make(map[string]struct{})
	result := Result{}
	for _, target := range targets {
		info, err := os.Stat(target)
		if errors.Is(err, fs.ErrNotExist) {
			result.Warnings = append(result.Warnings, fmt.Sprintf("not found: %s", target))
			continue
		}
		if err != nil {
			return Result{}, fmt.Errorf("stat %s: %w", target, err)
		}
		if !info.IsDir() {
			unique[filepath.Clean(target)] = struct{}{}
			continue
		}
		if err := filepath.WalkDir(target, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() && path != target && skippedDir(entry.Name()) {
				return filepath.SkipDir
			}
			if !entry.IsDir() {
				unique[filepath.Clean(path)] = struct{}{}
			}
			return nil
		}); err != nil {
			return Result{}, fmt.Errorf("walk %s: %w", target, err)
		}
	}
	paths := make([]string, 0, len(unique))
	for path := range unique {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			return Result{}, fmt.Errorf("read %s: %w", path, err)
		}
		if len(raw) == 0 || bytes.IndexByte(raw, 0) >= 0 {
			continue
		}
		if maxBytes > 0 && result.TotalBytes+len(raw) > maxBytes {
			return Result{}, fmt.Errorf(
				"complete text set exceeds max_bytes %d at %s (%d bytes before this file, %d in this file)",
				maxBytes, path, result.TotalBytes, len(raw),
			)
		}
		lines := bytes.Count(raw, []byte{'\n'})
		result.Files = append(result.Files, File{
			Path: path, Lines: lines, Bytes: len(raw), Text: string(raw),
		})
		result.TotalLines += lines
		result.TotalBytes += len(raw)
	}
	return result, nil
}

func skippedDir(name string) bool {
	switch name {
	case ".git", "node_modules", ".venv", "__pycache__":
		return true
	default:
		return false
	}
}
