package installer

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMCPInstallWiresConfigDrivenSettingsCredentialAndClients(t *testing.T) {
	home := t.TempDir()
	canonical := filepath.Join(home, ".claude")
	secondary := filepath.Join(home, "account-two")
	writeFixture(t, filepath.Join(canonical, "settings.json"), `{}`)
	writeFixture(t, filepath.Join(secondary, "settings.json"), `{}`)
	configPath := filepath.Join(home, ".config", "pfm", "config.json")
	writeFixture(t, configPath, `{"version":2,"mcp":{"servers":{"chat":{"enabled":true}},"http":{"port":8456}}}`)

	options := Options{
		Mode:          ModeApply,
		Home:          home,
		ConfigDir:     canonical,
		ConfigDirs:    []string{canonical, secondary},
		MCPEnabled:    map[string]bool{"chat": true, "harvester": false},
		MCPPort:       8456,
		MCPConfigPath: configPath,
		Runner:        &fakeRunner{},
	}
	if _, err := Run(context.Background(), options); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(canonical, "settings.json"),
		filepath.Join(secondary, "settings.json"),
	} {
		raw := readFixture(t, path)
		for _, command := range []string{
			home + "/.local/bin/pfm dream hook agent-inject",
			home + "/.local/bin/pfm dream hook nudge",
			home + "/.local/bin/pfm internal explore-deny",
			home + "/.local/bin/pfm internal epic-inject",
		} {
			if !strings.Contains(raw, command) {
				t.Fatalf("%s missing installer hook %q", path, command)
			}
		}
		var document map[string]any
		if err := json.Unmarshal([]byte(raw), &document); err != nil {
			t.Fatal(err)
		}
		if document["cleanupPeriodDays"] != float64(36500) {
			t.Fatalf("%s cleanupPeriodDays=%v", path, document["cleanupPeriodDays"])
		}
	}
	credential := filepath.Join(home, ".local", "share", "pfm", "install", mcpCredentialName)
	info, err := os.Stat(credential)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("credential info=%v err=%v, want 0600", info, err)
	}
	token := strings.TrimSpace(readFixture(t, credential))
	if len(token) != 64 || !isHex(token) {
		t.Fatalf("credential=%q, want 64 hex characters", token)
	}
	var clients map[string]any
	if err := json.Unmarshal([]byte(readFixture(t, filepath.Join(home, ".mcp.json"))), &clients); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readFixture(t, filepath.Join(home, ".mcp.json")), "127.0.0.1:8456/mcp/chat") {
		t.Fatalf("client registration missing chat URL: %s", readFixture(t, filepath.Join(home, ".mcp.json")))
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "systemd", "user", mcpUnitName)); err != nil {
		t.Fatalf("MCP systemd unit missing: %v", err)
	}

	if report, err := Run(context.Background(), options); err != nil || report.Changed != 0 {
		t.Fatalf("second apply report=%#v err=%v", report, err)
	}
	if _, err := Run(context.Background(), Options{
		Mode:       ModeUninstall,
		Home:       home,
		ConfigDir:  canonical,
		ConfigDirs: []string{canonical, secondary},
		MCPEnabled: options.MCPEnabled,
		Runner:     &fakeRunner{},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(credential); !os.IsNotExist(err) {
		t.Fatalf("uninstall retained credential: %v", err)
	}
}
