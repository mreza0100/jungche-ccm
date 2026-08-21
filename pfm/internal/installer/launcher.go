package installer

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type LauncherState string

const (
	LauncherOK        LauncherState = "ok"
	LauncherMissing   LauncherState = "missing"
	LauncherDisplaced LauncherState = "displaced"
)

type ClaudeLauncherStatus struct {
	State  LauncherState
	Target string
}

func managedClaudeLauncher(home string) string {
	return filepath.Join(home, ".local", "share", "pfm", "install", "bin", "claude")
}

func canonicalClaudeLauncher(home string) string {
	return filepath.Join(home, ".local", "bin", "claude")
}

func claudeLauncherStatePath(home string) string {
	return filepath.Join(home, ".local", "share", "pfm", "install", "launcher.state")
}

func InspectClaudeLauncher(home string) (ClaudeLauncherStatus, error) {
	canonical := canonicalClaudeLauncher(home)
	info, err := os.Lstat(canonical)
	if errors.Is(err, fs.ErrNotExist) {
		return ClaudeLauncherStatus{State: LauncherMissing}, nil
	}
	if err != nil {
		return ClaudeLauncherStatus{}, fmt.Errorf("inspect canonical Claude launcher: %w", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return ClaudeLauncherStatus{State: LauncherDisplaced, Target: canonical}, nil
	}
	target, err := os.Readlink(canonical)
	if err != nil {
		return ClaudeLauncherStatus{}, fmt.Errorf("read canonical Claude launcher: %w", err)
	}
	if filepath.Clean(target) == filepath.Clean(managedClaudeLauncher(home)) {
		managedInfo, statErr := os.Stat(target)
		if statErr != nil {
			return ClaudeLauncherStatus{}, fmt.Errorf("inspect managed Claude launcher: %w", statErr)
		}
		if !managedInfo.Mode().IsRegular() || managedInfo.Mode().Perm()&0o111 == 0 {
			return ClaudeLauncherStatus{}, fmt.Errorf("managed Claude launcher is not executable: %s", target)
		}
		return ClaudeLauncherStatus{State: LauncherOK, Target: target}, nil
	}
	return ClaudeLauncherStatus{State: LauncherDisplaced, Target: target}, nil
}

// RepairClaudeLauncher is the fast SessionStart repair path. A correct link
// costs one lstat and readlink; a displaced native symlink is recorded before
// it is atomically replaced.
func RepairClaudeLauncher(home string) (bool, error) {
	status, err := InspectClaudeLauncher(home)
	if err != nil {
		return false, err
	}
	if status.State == LauncherOK {
		return false, nil
	}
	managed := managedClaudeLauncher(home)
	info, err := os.Stat(managed)
	if err != nil {
		return false, fmt.Errorf("inspect managed Claude launcher: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return false, fmt.Errorf("managed Claude launcher is not executable: %s", managed)
	}
	canonical := canonicalClaudeLauncher(home)
	if status.State == LauncherDisplaced {
		current, statErr := os.Lstat(canonical)
		if statErr != nil {
			return false, fmt.Errorf("inspect displaced Claude launcher: %w", statErr)
		}
		if current.Mode()&os.ModeSymlink == 0 {
			return false, fmt.Errorf("refuse to replace non-symlink Claude binary: %s", canonical)
		}
		if err := atomicWrite(claudeLauncherStatePath(home), []byte(status.Target+"\n"), 0o600); err != nil {
			return false, fmt.Errorf("record displaced Claude target: %w", err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(canonical), 0o700); err != nil {
		return false, fmt.Errorf("create canonical binary directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(canonical), ".claude-launcher-")
	if err != nil {
		return false, fmt.Errorf("reserve Claude launcher link: %w", err)
	}
	temporaryPath := temporary.Name()
	if err := temporary.Close(); err != nil {
		_ = os.Remove(temporaryPath)
		return false, err
	}
	if err := os.Remove(temporaryPath); err != nil {
		return false, err
	}
	defer os.Remove(temporaryPath)
	if err := os.Symlink(managed, temporaryPath); err != nil {
		return false, fmt.Errorf("create managed Claude launcher link: %w", err)
	}
	if err := os.Rename(temporaryPath, canonical); err != nil {
		return false, fmt.Errorf("publish managed Claude launcher link: %w", err)
	}
	return true, nil
}

func (installer *engine) wireClaudeLauncher() error {
	status, err := InspectClaudeLauncher(installer.options.Home)
	if err != nil {
		return err
	}
	if status.State == LauncherOK {
		installer.ok(canonicalClaudeLauncher(installer.options.Home))
		return nil
	}
	description := "link " + canonicalClaudeLauncher(installer.options.Home) + " -> " + managedClaudeLauncher(installer.options.Home)
	return installer.change(description, func() error {
		_, err := RepairClaudeLauncher(installer.options.Home)
		return err
	})
}

func (installer *engine) unwireClaudeLauncher() error {
	home := installer.options.Home
	status, err := InspectClaudeLauncher(home)
	if err != nil {
		return err
	}
	if status.State != LauncherOK {
		installer.skip(canonicalClaudeLauncher(home) + " is not the installed launcher")
		return nil
	}
	statePath := claudeLauncherStatePath(home)
	content, readErr := os.ReadFile(statePath)
	if readErr != nil && !errors.Is(readErr, fs.ErrNotExist) {
		return fmt.Errorf("read displaced Claude target: %w", readErr)
	}
	displaced := strings.TrimSpace(string(content))
	description := "remove " + canonicalClaudeLauncher(home)
	if displaced != "" {
		description = "restore " + canonicalClaudeLauncher(home) + " -> " + displaced
	}
	return installer.change(description, func() error {
		if err := os.Remove(canonicalClaudeLauncher(home)); err != nil {
			return err
		}
		if displaced != "" {
			if err := os.Symlink(displaced, canonicalClaudeLauncher(home)); err != nil {
				return err
			}
		}
		if err := os.Remove(statePath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	})
}
