package installer

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ClaudeUserRegistry is Claude's user scope: the default account stores it
// beside .claude; an explicit alternate config directory stores it inside.
func ClaudeUserRegistry(home, configDir string, implicit bool) string {
	if implicit || filepath.Clean(configDir) == filepath.Join(home, ".claude") {
		return filepath.Join(home, ".claude.json")
	}
	return filepath.Join(configDir, ".claude.json")
}

func (installer *engine) writeMCPClientJSON(names []string) ([]string, error) {
	ownership := mcpOwnership{Registrations: map[string]map[string]any{}}
	raw, err := os.ReadFile(installer.mcpOwnershipPath())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &ownership); err != nil {
			return nil, fmt.Errorf("decode MCP ownership: %w", err)
		}
	}
	if ownership.Registrations == nil {
		ownership.Registrations = map[string]map[string]any{}
	}
	if ownership.Pending == nil {
		ownership.Pending = map[string]map[string]any{}
	}
	for _, receipts := range []*map[string]map[string]any{&ownership.Registrations, &ownership.Pending} {
		canonical := map[string]map[string]any{}
		for path, entries := range *receipts {
			physical := physicalSettingsPath(path)
			if canonical[physical] == nil {
				canonical[physical] = map[string]any{}
			}
			for name, registration := range entries {
				if prior, exists := canonical[physical][name]; exists && !sameJSONValue(prior, registration) {
					return nil, fmt.Errorf("conflicting MCP ownership aliases for %s in %s", name, physical)
				}
				canonical[physical][name] = registration
			}
		}
		*receipts = canonical
	}
	wantedPaths := map[string]bool{}
	if len(names) > 0 {
		registries := installer.options.ClaudeRegistries
		if registries == nil {
			for _, dir := range installer.claudeConfigDirs() {
				registries = append(registries, ClaudeUserRegistry(installer.options.Home, dir, false))
			}
		}
		for _, path := range registries {
			if strings.TrimSpace(path) != "" {
				wantedPaths[physicalSettingsPath(path)] = true
			}
		}
	}
	paths := map[string]bool{}
	for path := range wantedPaths {
		paths[path] = true
	}
	for path := range ownership.Registrations {
		paths[path] = true
	}
	for path := range ownership.Pending {
		paths[path] = true
	}
	legacy := physicalSettingsPath(filepath.Join(installer.options.Home, ".mcp.json"))
	if len(ownership.Clients) > 0 {
		paths[legacy] = true
	}
	ordered := make([]string, 0, len(paths))
	for path := range paths {
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)
	for _, path := range ordered {
		original, existed, err := readMCPFile(path)
		document := map[string]any{}
		if err == nil && existed {
			err = json.Unmarshal(original, &document)
			if err == nil && document == nil {
				err = errors.New("registry must be an object")
			}
		}
		if err != nil {
			return nil, fmt.Errorf("read MCP registry %s: %w", path, err)
		}
		servers := map[string]any{}
		if value, present := document["mcpServers"]; present {
			var ok bool
			servers, ok = value.(map[string]any)
			if !ok || servers == nil {
				return nil, fmt.Errorf("MCP registry %s: mcpServers must be an object", path)
			}
		}
		before, _ := json.Marshal(document)
		owned := ownership.Registrations[path]
		if owned == nil {
			owned = map[string]any{}
		}
		for name, registration := range ownership.Pending[path] {
			if sameJSONValue(servers[name], registration) {
				owned[name] = registration
			}
		}
		// Migrate the names-only predecessor ledger only at its historical path.
		if path == legacy {
			for _, name := range ownership.Clients {
				if registration, ok := servers[name].(map[string]any); ok && installer.isPFMClient(name, registration) {
					owned[name] = registration
				}
			}
		}
		next := map[string]any{}
		wanted := map[string]bool{}
		if wantedPaths[path] {
			for _, name := range names {
				wanted[name] = true
			}
		}
		for name, registration := range owned {
			current, present := servers[name]
			if present && sameJSONValue(current, registration) {
				if wanted[name] {
					next[name] = registration
				} else {
					delete(servers, name)
				}
			}
		}
		for name := range wanted {
			registration := installer.mcpClientRegistration(name)
			_, present := servers[name]
			_, ours := next[name]
			if present && !ours {
				installer.skip("preserve conflicting manual MCP client " + name + " in " + path)
				continue
			}
			servers[name] = registration
			next[name] = registration
		}
		if len(servers) > 0 || document["mcpServers"] != nil {
			document["mcpServers"] = servers
		}
		after, _ := json.Marshal(document)
		// Keep the last receipt until the registry write succeeds. Pending exact
		// values cover a crash between that write and the final receipt commit.
		ownership.Pending[path] = next
		if string(before) != string(after) {
			if err := installer.saveMCPOwnership(ownership); err != nil {
				return nil, err
			}
			encoded, err := json.MarshalIndent(document, "", "  ")
			if err != nil {
				return nil, err
			}
			if err := installer.change(changeDescription(path, existed), func() error {
				return installer.writeMCPFile(path, original, append(encoded, '\n'), existed)
			}); err != nil {
				return nil, err
			}
		} else {
			installer.ok(path + " wiring")
		}
		if len(next) > 0 {
			ownership.Registrations[path] = next
		} else {
			delete(ownership.Registrations, path)
		}
		delete(ownership.Pending, path)
		if path == legacy {
			ownership.Clients = nil
		}
		if err := installer.saveMCPOwnership(ownership); err != nil {
			return nil, err
		}
	}
	if err := installer.saveMCPOwnership(ownership); err != nil {
		return nil, err
	}
	return names, nil
}

func (installer *engine) saveMCPOwnership(ownership mcpOwnership) error {
	path := installer.mcpOwnershipPath()
	if len(ownership.Clients) == 0 && len(ownership.Registrations) == 0 && len(ownership.Pending) == 0 {
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			return nil
		} else if err != nil {
			return err
		}
		return installer.change("remove "+path, func() error { return os.Remove(path) })
	}
	encoded, err := json.MarshalIndent(ownership, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	if sameFile(path, encoded, 0600) {
		return nil
	}
	return installer.change("write "+path, func() error { return atomicWrite(path, encoded, 0600) })
}
