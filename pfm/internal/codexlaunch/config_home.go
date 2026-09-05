package codexlaunch

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// readerHome adapts runtime-only config selectors for config/read. The query
// home is a sibling so ../ references retain their meaning; other account
// entries remain reachable through symlinks. The eventual launch uses the
// original environment, and no source config file is ever written.
func readerHome(options []string) ([]string, string, func(), error) {
	cleanup := func() {}
	var filtered []string
	profile := ""
	ignore := false
	for i := 0; i < len(options); i++ {
		switch options[i] {
		case "-p", "--profile":
			i++
			profile = options[i]
		case "--ignore-user-config":
			ignore = true
		default:
			filtered = append(filtered, options[i])
			if options[i] != "--strict-config" {
				i++
				filtered = append(filtered, options[i])
			}
		}
	}
	if profile == "" && !ignore {
		return filtered, "", cleanup, nil
	}
	if profile != "" && strings.IndexFunc(profile, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-')
	}) >= 0 {
		return nil, "", cleanup, fmt.Errorf("invalid Codex profile name")
	}
	home := os.Getenv("CODEX_HOME")
	if home == "" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return nil, "", cleanup, err
		}
		home = filepath.Join(userHome, ".codex")
	}
	home, err := filepath.Abs(home)
	if err != nil {
		return nil, "", cleanup, err
	}
	if os.Getenv("CODEX_HOME") != "" {
		home, err = filepath.EvalSymlinks(home)
		if err != nil {
			return nil, "", cleanup, fmt.Errorf("resolve Codex account home: %w", err)
		}
	}
	document := map[string]any{}
	if !ignore {
		if err := readConfigFile(filepath.Join(home, "config.toml"), document); err != nil {
			return nil, "", cleanup, err
		}
		profiles, _ := document["profiles"].(map[string]any)
		if _, exists := profiles[profile]; exists || document["profile"] == profile {
			return nil, "", cleanup, fmt.Errorf("Codex profile conflicts with legacy profile configuration")
		}
		overlay := map[string]any{}
		if err := readConfigFile(filepath.Join(home, profile+".config.toml"), overlay); err != nil {
			return nil, "", cleanup, err
		}
		mergeTables(document, overlay)
	}
	var encoded bytes.Buffer
	if err := toml.NewEncoder(&encoded).Encode(document); err != nil {
		return nil, "", cleanup, err
	}
	scratch, err := os.MkdirTemp(filepath.Dir(home), ".pfm-codex-config-")
	if err != nil {
		return nil, "", cleanup, err
	}
	cleanup = func() { _ = os.RemoveAll(scratch) }
	entries, err := os.ReadDir(home)
	if err != nil && !os.IsNotExist(err) {
		cleanup()
		return nil, "", func() {}, err
	}
	for _, entry := range entries {
		if entry.Name() == "config.toml" {
			continue
		}
		if err := os.Symlink(filepath.Join(home, entry.Name()), filepath.Join(scratch, entry.Name())); err != nil {
			cleanup()
			return nil, "", func() {}, err
		}
	}
	if err := os.WriteFile(filepath.Join(scratch, "config.toml"), encoded.Bytes(), 0600); err != nil {
		cleanup()
		return nil, "", func() {}, err
	}
	return filtered, scratch, cleanup, nil
}

func readConfigFile(path string, document map[string]any) error {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read Codex config: %w", err)
	}
	if _, err := toml.Decode(string(raw), &document); err != nil {
		return fmt.Errorf("parse Codex config %s: %w", path, err)
	}
	return nil
}

func mergeTables(base, overlay map[string]any) {
	for key, value := range overlay {
		left, leftOK := base[key].(map[string]any)
		right, rightOK := value.(map[string]any)
		if leftOK && rightOK {
			mergeTables(left, right)
		} else {
			base[key] = value
		}
	}
}
