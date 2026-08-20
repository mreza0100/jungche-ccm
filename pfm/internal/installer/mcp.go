package installer

import (
	"crypto/rand"
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
	Credential string   `json:"credential"`
	Clients    []string `json:"clients"`
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
		if installer.options.Mode == ModeApply {
			return installer.removeMCPClientRegistrations()
		}
		installer.skip("MCP disabled; daemon units and client registrations will be removed on apply")
		return nil
	}
	if !installer.apply {
		installer.say("MCP enabled: would generate a 32-byte bearer credential, install daemon units, and register HTTP clients")
		return nil
	}
	token, err := installer.ensureMCPCredential()
	if err != nil {
		return err
	}
	if installer.options.MCPConfigPath != "" {
		effective, err := pfmconfig.Load(installer.options.MCPConfigPath, installer.options.Home, nil)
		if err != nil {
			return fmt.Errorf("load MCP config for installer credential: %w", err)
		}
		if _, err := pfmconfig.SetMCPAuthToken(effective, token); err != nil {
			return fmt.Errorf("store MCP installer credential: %w", err)
		}
	}
	names := enabledMCPNames(installer.options.MCPEnabled)
	if err := installer.writeMCPClientJSON(token, names); err != nil {
		return err
	}
	if err := installer.writeMCPCodeConfig(token, names); err != nil {
		return err
	}
	return installer.writeMCPOwnership(names)
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

func (installer *engine) ensureMCPCredential() (string, error) {
	path := installer.mcpCredentialPath()
	if !installer.options.Force {
		if raw, err := os.ReadFile(path); err == nil {
			token := strings.TrimSpace(string(raw))
			if len(token) == 64 && isHex(token) {
				installer.ok(path)
				return token, nil
			}
			return "", fmt.Errorf("invalid MCP credential at %s", path)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("read MCP credential %s: %w", path, err)
		}
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate MCP credential: %w", err)
	}
	token := hex.EncodeToString(raw)
	if err := installer.change("write "+path, func() error {
		return atomicWrite(path, []byte(token+"\n"), 0o600)
	}); err != nil {
		return "", err
	}
	return token, nil
}

func isHex(value string) bool {
	_, err := hex.DecodeString(value)
	return err == nil
}

func (installer *engine) writeMCPClientJSON(token string, names []string) error {
	path := filepath.Join(installer.options.Home, ".mcp.json")
	document, existed, err := readJSONObject(path)
	if err != nil {
		return fmt.Errorf("read MCP client config %s: %w", path, err)
	}
	servers, _ := document["mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
		document["mcpServers"] = servers
	}
	changed := false
	for _, name := range names {
		wanted := map[string]any{
			"type": "http",
			"url":  installer.mcpURL(name),
			"headers": map[string]any{
				"Authorization": "Bearer " + token,
			},
		}
		if current, ok := servers[name].(map[string]any); ok && !sameJSONValue(current, wanted) {
			installer.skip("preserve conflicting manual MCP client " + name)
			continue
		}
		if !sameJSONValue(servers[name], wanted) {
			servers[name] = wanted
			changed = true
		}
	}
	if !changed {
		installer.ok(path + " wiring")
		return nil
	}
	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return fmt.Errorf("encode MCP client config %s: %w", path, err)
	}
	return installer.change("rewrite "+path+" (backup preserved)", func() error {
		if existed {
			backup := availableBackup(path, installer.stamp)
			if err := copyBackup(path, backup); err != nil {
				return err
			}
		}
		return atomicWrite(path, append(encoded, '\n'), 0o600)
	})
}

func (installer *engine) writeMCPCodeConfig(token string, names []string) error {
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
			"[mcp_servers."+name+".headers]",
			"Authorization = \"Bearer "+token+"\"",
		)
	}
	generated = append(generated, mcpFenceEnd)
	wantedLines := append(kept, generated...)
	wanted := strings.TrimRight(strings.Join(wantedLines, "\n"), "\n") + "\n"
	if string(raw) == wanted {
		installer.ok(path + " wiring")
		return nil
	}
	return installer.change("rewrite "+path+" (backup preserved)", func() error {
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
		Credential: installer.mcpCredentialPath(),
		Clients:    names,
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
		installer.skip("MCP ownership ledger absent")
		return nil
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
		if err := installer.change("rewrite "+path+" (backup preserved)", func() error {
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
	if err := installer.change("remove "+ownership.Credential, func() error { return os.Remove(ownership.Credential) }); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return installer.change("remove "+installer.mcpOwnershipPath(), func() error { return os.Remove(installer.mcpOwnershipPath()) })
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
