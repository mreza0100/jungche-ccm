package installer

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	pfmconfig "hostops/pfm/internal/config"
)

const (
	mcpCredentialName = "mcp-auth-token"
	mcpOwnershipName  = "mcp-ownership.json"
	mcpFenceBegin     = "# BEGIN pfm mcp_servers — installer-owned"
	mcpFenceEnd       = "# END pfm mcp_servers — installer-owned"
)

type mcpOwnership struct {
	Clients []string `json:"clients"`
}

func (installer *engine) mcpAnyEnabled() bool {
	for _, enabled := range installer.options.MCPEnabled {
		if enabled {
			return true
		}
	}
	return false
}

func (installer *engine) mcpCredentialPath() string {
	return filepath.Join(installer.managedRoot, mcpCredentialName)
}

func (installer *engine) mcpOwnershipPath() string {
	return filepath.Join(installer.managedRoot, mcpOwnershipName)
}

func (installer *engine) wireMCP() error {
	if installer.options.Mode == ModeUninstall {
		return installer.removeMCPClientRegistrations()
	}
	if !installer.mcpAnyEnabled() {
		return installer.removeMCPClientRegistrations()
	}
	if err := installer.removeLegacyMCPConfigAuth(); err != nil {
		return err
	}
	names := enabledMCPNames(installer.options.MCPEnabled)
	wiredNames, err := installer.writeMCPClientJSON(names)
	if err != nil {
		return err
	}
	if err := installer.writeMCPCodeConfig(wiredNames); err != nil {
		return err
	}
	if err := installer.removeLegacyMCPCredential(); err != nil {
		return err
	}
	return installer.writeMCPOwnership(wiredNames)
}

func enabledMCPNames(servers map[string]bool) []string {
	names := make([]string, 0, len(servers))
	for name, enabled := range servers {
		if enabled && (name == "chat" || name == "harvester") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func (installer *engine) removeLegacyMCPCredential() error {
	path := installer.mcpCredentialPath()
	if _, err := os.Lstat(path); errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect retired MCP credential %s: %w", path, err)
	}
	return installer.change("remove retired "+path, func() error { return os.Remove(path) })
}

func isHex(value string) bool {
	_, err := hex.DecodeString(value)
	return err == nil
}

func (installer *engine) writeMCPClientJSON(names []string) ([]string, error) {
	path := filepath.Join(installer.options.Home, ".mcp.json")
	document, existed, err := readJSONObject(path)
	if err != nil {
		return nil, fmt.Errorf("read MCP client config %s: %w", path, err)
	}
	previouslyOwned := installer.previouslyOwnedMCPClients()
	servers, _ := document["mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
		document["mcpServers"] = servers
	}
	changed := false
	wired := make([]string, 0, len(names))
	for _, name := range names {
		wanted := map[string]any{
			"type": "http",
			"url":  installer.mcpURL(name),
		}
		if current, present := servers[name]; present && !sameJSONValue(current, wanted) {
			registration, object := current.(map[string]any)
			if !object || !previouslyOwned[name] || !installer.isPFMHTTPClient(name, registration) {
				installer.skip("preserve conflicting manual MCP client " + name)
				continue
			}
		}
		if !sameJSONValue(servers[name], wanted) {
			servers[name] = wanted
			changed = true
		}
		wired = append(wired, name)
	}
	if !changed {
		installer.ok(path + " wiring")
		return wired, nil
	}
	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode MCP client config %s: %w", path, err)
	}
	err = installer.change(changeDescription(path, existed), func() error {
		if existed {
			backup := availableBackup(path, installer.stamp)
			if err := copyBackup(path, backup); err != nil {
				return err
			}
		}
		return atomicWrite(path, append(encoded, '\n'), 0o600)
	})
	return wired, err
}

func (installer *engine) previouslyOwnedMCPClients() map[string]bool {
	owned := make(map[string]bool)
	raw, err := os.ReadFile(installer.mcpOwnershipPath())
	if err != nil {
		return owned
	}
	var ownership mcpOwnership
	if json.Unmarshal(raw, &ownership) != nil {
		return owned
	}
	for _, name := range ownership.Clients {
		owned[name] = true
	}
	return owned
}

func (installer *engine) isPFMHTTPClient(name string, registration map[string]any) bool {
	if registration["type"] != "http" || registration["url"] != installer.mcpURL(name) {
		return false
	}
	if len(registration) == 2 {
		return true
	}
	if len(registration) != 3 {
		return false
	}
	// Recognize only PFM's retired exact bearer shape so an owned registration
	// can be migrated. Foreign headers remain a manual conflict.
	headers, ok := registration["headers"].(map[string]any)
	if !ok || len(headers) != 1 {
		return false
	}
	authorization, ok := headers["Authorization"].(string)
	if !ok || !strings.HasPrefix(authorization, "Bearer ") {
		return false
	}
	token := strings.TrimPrefix(authorization, "Bearer ")
	return len(token) == 64 && isHex(token)
}

func (installer *engine) writeMCPCodeConfig(names []string) error {
	path := filepath.Join(installer.options.Home, ".codex", "config.toml")
	raw, err := os.ReadFile(path)
	existed := true
	if errors.Is(err, fs.ErrNotExist) {
		existed = false
		raw = nil
	} else if err != nil {
		return fmt.Errorf("read Codex MCP config %s: %w", path, err)
	}
	var lines []string
	if len(raw) != 0 {
		lines = strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	}
	start, end := -1, -1
	for index, line := range lines {
		if line == mcpFenceBegin {
			start = index
		}
		if line == mcpFenceEnd && start >= 0 {
			end = index
		}
	}
	kept := lines
	if start >= 0 && end >= start {
		kept = append(append([]string{}, lines[:start]...), lines[end+1:]...)
	}
	generated := []string{mcpFenceBegin}
	for _, name := range names {
		generated = append(generated,
			"[mcp_servers."+name+"]",
			"url = \""+installer.mcpURL(name)+"\"",
		)
	}
	generated = append(generated, mcpFenceEnd)
	wantedLines := append(kept, generated...)
	wanted := strings.TrimRight(strings.Join(wantedLines, "\n"), "\n") + "\n"
	if string(raw) == wanted {
		installer.ok(path + " wiring")
		return nil
	}
	return installer.change(changeDescription(path, existed), func() error {
		if existed {
			backup := availableBackup(path, installer.stamp)
			if err := copyBackup(path, backup); err != nil {
				return err
			}
		}
		return atomicWrite(path, []byte(wanted), 0o600)
	})
}

func (installer *engine) mcpURL(name string) string {
	return fmt.Sprintf("http://127.0.0.1:%d/mcp/%s", installer.options.MCPPort, name)
}

func (installer *engine) writeMCPOwnership(names []string) error {
	content, err := json.MarshalIndent(mcpOwnership{
		Clients: names,
	}, "", "  ")
	if err != nil {
		return err
	}
	if sameFile(installer.mcpOwnershipPath(), append(content, '\n'), 0o600) {
		installer.ok(installer.mcpOwnershipPath())
		return nil
	}
	return installer.change("write "+installer.mcpOwnershipPath(), func() error {
		return atomicWrite(installer.mcpOwnershipPath(), append(content, '\n'), 0o600)
	})
}

func (installer *engine) removeMCPClientRegistrations() error {
	ownershipRaw, err := os.ReadFile(installer.mcpOwnershipPath())
	if errors.Is(err, fs.ErrNotExist) {
		installer.skip("MCP ownership ledger absent; no JSON clients removed")
		if err := installer.removeMCPCodeConfig(); err != nil {
			return err
		}
		if err := installer.removeLegacyMCPConfigAuth(); err != nil {
			return err
		}
		return installer.removeLegacyMCPCredential()
	}
	if err != nil {
		return fmt.Errorf("read MCP ownership ledger: %w", err)
	}
	var ownership mcpOwnership
	if err := json.Unmarshal(ownershipRaw, &ownership); err != nil {
		return fmt.Errorf("decode MCP ownership ledger: %w", err)
	}
	path := filepath.Join(installer.options.Home, ".mcp.json")
	document, existed, err := readJSONObject(path)
	if err != nil {
		return err
	}
	servers, _ := document["mcpServers"].(map[string]any)
	changed := false
	for _, name := range ownership.Clients {
		if _, ok := servers[name]; ok {
			delete(servers, name)
			changed = true
		}
	}
	if changed {
		encoded, err := json.MarshalIndent(document, "", "  ")
		if err != nil {
			return err
		}
		if err := installer.change(changeDescription(path, existed), func() error {
			if existed {
				if err := copyBackup(path, availableBackup(path, installer.stamp)); err != nil {
					return err
				}
			}
			return atomicWrite(path, append(encoded, '\n'), 0o600)
		}); err != nil {
			return err
		}
	}
	if err := installer.removeMCPCodeConfig(); err != nil {
		return err
	}
	if err := installer.removeLegacyMCPConfigAuth(); err != nil {
		return err
	}
	if err := installer.removeLegacyMCPCredential(); err != nil {
		return err
	}
	return installer.change("remove "+installer.mcpOwnershipPath(), func() error { return os.Remove(installer.mcpOwnershipPath()) })
}

func (installer *engine) removeLegacyMCPConfigAuth() error {
	if installer.options.MCPConfigPath == "" {
		return nil
	}
	effective, err := pfmconfig.Load(installer.options.MCPConfigPath, installer.options.Home, nil)
	if err != nil {
		return fmt.Errorf("load MCP config for legacy auth cleanup: %w", err)
	}
	changed, err := pfmconfig.MCPAuthTokenPresent(effective)
	if err != nil {
		return fmt.Errorf("plan retired MCP installer credential removal: %w", err)
	}
	if !changed {
		return nil
	}
	return installer.change("remove retired MCP authToken from "+installer.options.MCPConfigPath, func() error {
		_, err := pfmconfig.RemoveMCPAuthToken(effective)
		return err
	})
}

func (installer *engine) removeMCPCodeConfig() error {
	path := filepath.Join(installer.options.Home, ".codex", "config.toml")
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read Codex MCP config for removal: %w", err)
	}
	lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	start, end := -1, -1
	for index, line := range lines {
		if line == mcpFenceBegin {
			start = index
		}
		if line == mcpFenceEnd && start >= 0 {
			end = index
		}
	}
	if start < 0 || end < start {
		return nil
	}
	kept := append(append([]string{}, lines[:start]...), lines[end+1:]...)
	wanted := strings.TrimRight(strings.Join(kept, "\n"), "\n")
	if wanted != "" {
		wanted += "\n"
	}
	return installer.change("rewrite "+path+" (remove pfm MCP registration)", func() error {
		return atomicWrite(path, []byte(wanted), 0o600)
	})
}

func readJSONObject(path string) (map[string]any, bool, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]any{}, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		return nil, true, err
	}
	return document, true, nil
}

func sameJSONValue(left, right any) bool {
	a, err := json.Marshal(left)
	if err != nil {
		return false
	}
	b, err := json.Marshal(right)
	return err == nil && string(a) == string(b)
}
