package installer

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func sameFile(path string, content []byte, mode fs.FileMode) bool {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != mode.Perm() {
		return false
	}
	existing, err := os.ReadFile(path)
	return err == nil && bytes.Equal(existing, content)
}

func atomicWrite(path string, content []byte, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create %s parent: %w", path, err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".pfm-install-*")
	if err != nil {
		return fmt.Errorf("create %s scratch: %w", path, err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode.Perm()); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set %s scratch mode: %w", path, err)
	}
	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write %s scratch: %w", path, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close %s scratch: %w", path, err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}

func resolvedLink(path string) (string, bool) {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return "", false
	}
	target, err := os.Readlink(path)
	if err != nil {
		return "", false
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(path), target)
	}
	return filepath.Clean(target), true
}

func newestBackup(path string) string {
	matches, _ := filepath.Glob(path + ".pre-professor-*")
	sort.Strings(matches)
	if len(matches) == 0 {
		return ""
	}
	return matches[len(matches)-1]
}

func availableBackup(path, stamp string) string {
	base := path + ".pre-professor-" + stamp
	if _, err := os.Lstat(base); errors.Is(err, fs.ErrNotExist) {
		return base
	}
	for index := 1; ; index++ {
		candidate := fmt.Sprintf("%s.%d", base, index)
		if _, err := os.Lstat(candidate); errors.Is(err, fs.ErrNotExist) {
			return candidate
		}
	}
}

func copyBackup(source, target string) error {
	content, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	return atomicWrite(target, content, info.Mode().Perm())
}

func sourceLine(path string) string {
	return `[[ -r "` + path + `" ]] && source "` + path + `"`
}

func rewriteZshrc(content, wanted string, uninstall bool) string {
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	result := make([]string, 0, len(lines)+3)
	inserted := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		generatedComment := trimmed == "# The shell launchers delegate to the pfm engine." ||
			strings.HasPrefix(trimmed, "# Professor fleet — launchers") ||
			strings.Contains(trimmed, "legacy oracle remains unsourced")
		fleetSource := !strings.HasPrefix(trimmed, "#") &&
			strings.Contains(trimmed, "source") &&
			(strings.Contains(trimmed, "pfm.zsh") || strings.Contains(trimmed, "cc-fleet.zsh"))
		if generatedComment {
			continue
		}
		if fleetSource {
			if !uninstall && !inserted {
				result = append(result, "# The shell launchers delegate to the pfm engine.", wanted)
				inserted = true
			}
			continue
		}
		result = append(result, line)
	}
	if !uninstall && !inserted {
		for len(result) > 0 && result[len(result)-1] == "" {
			result = result[:len(result)-1]
		}
		if len(result) > 0 {
			result = append(result, "")
		}
		result = append(result,
			"# Professor fleet — launchers + the cc-ls chat picker",
			"# The shell launchers delegate to the pfm engine.",
			wanted,
		)
	}
	for len(result) > 0 && result[len(result)-1] == "" {
		result = result[:len(result)-1]
	}
	if len(result) == 0 {
		return ""
	}
	return strings.Join(result, "\n") + "\n"
}
