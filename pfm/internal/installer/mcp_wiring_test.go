package installer

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMCPSystemdUnitStartsAtLogin(t *testing.T) {
	raw, err := readAsset("systemd/pfm-mcp.service")
	if err != nil {
		t.Fatal(err)
	}
	unit := string(raw)
	for _, want := range []string{"[Install]", "WantedBy=default.target"} {
		if !strings.Contains(unit, want) {
			t.Fatalf("pfm-mcp.service missing %q; enabled Harvester would not return after login:\n%s", want, unit)
		}
	}
}

func TestMCPInstallWiresConfigDrivenUnauthenticatedLoopbackClients(t *testing.T) {
	home := t.TempDir()
	canonical := filepath.Join(home, ".claude")
	secondary := filepath.Join(home, "account-two")
	writeFixture(t, filepath.Join(canonical, "settings.json"), `{}`)
	writeFixture(t, filepath.Join(secondary, "settings.json"), `{}`)
	configPath := filepath.Join(home, ".config", "pfm", "config.json")
	writeFixture(t, configPath, `{"version":2,"mcp":{"servers":{"chat":{"enabled":true}},"http":{"port":8456}}}`)
	runner := &fakeRunner{manager: true}

	options := Options{
		Mode:          ModeApply,
		Home:          home,
		ConfigDir:     canonical,
		ConfigDirs:    []string{canonical, secondary},
		MCPEnabled:    map[string]bool{"chat": true, "harvester": false},
		MCPPort:       8456,
		MCPConfigPath: configPath,
		Runner:        runner,
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
	if _, err := os.Stat(credential); !os.IsNotExist(err) {
		t.Fatalf("credential file exists in an unauthenticated MCP install: %v", err)
	}
	var clients map[string]any
	if err := json.Unmarshal([]byte(readFixture(t, filepath.Join(home, ".mcp.json"))), &clients); err != nil {
		t.Fatal(err)
	}
	clientJSON := readFixture(t, filepath.Join(home, ".mcp.json"))
	if !strings.Contains(clientJSON, "127.0.0.1:8456/mcp/chat") {
		t.Fatalf("client registration missing chat URL: %s", clientJSON)
	}
	for _, forbidden := range []string{"Authorization", "Bearer", "headers"} {
		if strings.Contains(clientJSON, forbidden) {
			t.Fatalf("client registration retained MCP authentication %q: %s", forbidden, clientJSON)
		}
	}
	if codex := readFixture(t, filepath.Join(home, ".codex", "config.toml")); strings.Contains(codex, "Authorization") || strings.Contains(codex, "Bearer") {
		t.Fatalf("Codex registration retained MCP authentication: %s", codex)
	}
	if config := readFixture(t, configPath); strings.Contains(config, "authToken") {
		t.Fatalf("PFM config retained MCP authentication: %s", config)
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "systemd", "user", mcpUnitName)); err != nil {
		t.Fatalf("MCP systemd unit missing: %v", err)
	}
	if calls := strings.Join(runner.calls, "\n"); !strings.Contains(calls, "systemctl --user restart "+mcpUnitName) {
		t.Fatalf("MCP daemon was not restarted after complete client wiring:\n%s", calls)
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

func TestMCPInstallRemovesLegacyCredentialAndAuthHeadersEverywhere(t *testing.T) {
	home := t.TempDir()
	canonical := filepath.Join(home, ".claude")
	writeFixture(t, filepath.Join(canonical, "settings.json"), `{}`)
	configPath := filepath.Join(home, ".config", "pfm", "config.json")
	credentialPath := filepath.Join(home, ".local", "share", "pfm", "install", mcpCredentialName)
	legacyToken := strings.Repeat("a", 64)
	writeFixture(t, configPath, `{"version":2,"mcp":{"servers":{"chat":{"enabled":true}},"authToken":"`+legacyToken+`"}}`)
	writeFixture(t, credentialPath, legacyToken+"\n")
	writeFixture(t, filepath.Join(home, ".local", "share", "pfm", "install", mcpOwnershipName), `{"credential":"`+credentialPath+`","clients":["chat"]}`)
	writeFixture(t, filepath.Join(home, ".mcp.json"), `{"mcpServers":{"chat":{"type":"http","url":"http://127.0.0.1:8377/mcp/chat","headers":{"Authorization":"Bearer `+legacyToken+`"}}}}`)
	writeFixture(t, filepath.Join(home, ".codex", "config.toml"), mcpFenceBegin+"\n"+
		"[mcp_servers.chat]\n"+
		"url = \"http://127.0.0.1:8377/mcp/chat\"\n"+
		"[mcp_servers.chat.headers]\n"+
		"Authorization = \"Bearer "+legacyToken+"\"\n"+
		mcpFenceEnd+"\n")
	options := Options{
		Mode: ModeApply, Home: home, ConfigDir: canonical,
		ConfigDirs: []string{canonical}, MCPEnabled: map[string]bool{"chat": true},
		MCPPort: 8377, MCPConfigPath: configPath, Force: true, Runner: &fakeRunner{},
	}
	if _, err := Run(context.Background(), options); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(credentialPath); !os.IsNotExist(err) {
		t.Fatalf("legacy credential remains: %v", err)
	}
	for _, path := range []string{
		configPath,
		filepath.Join(home, ".mcp.json"),
		filepath.Join(home, ".codex", "config.toml"),
	} {
		raw := readFixture(t, path)
		for _, forbidden := range []string{legacyToken, "authToken", "Authorization", "Bearer"} {
			if strings.Contains(raw, forbidden) {
				t.Fatalf("%s retained legacy MCP authentication %q: %s", path, forbidden, raw)
			}
		}
	}
}

func TestMCPManualConflictIsNotClaimedOrRemoved(t *testing.T) {
	home := t.TempDir()
	canonical := filepath.Join(home, ".claude")
	writeFixture(t, filepath.Join(canonical, "settings.json"), `{}`)
	manual := `{"mcpServers":{"harvester":{"type":"stdio","command":"manual-harvester"}}}`
	writeFixture(t, filepath.Join(home, ".mcp.json"), manual)
	options := Options{
		Mode: ModeApply, Home: home, ConfigDir: canonical,
		ConfigDirs: []string{canonical},
		MCPEnabled: map[string]bool{"chat": true, "harvester": true},
		MCPPort:    8377, Runner: &fakeRunner{},
	}
	if _, err := Run(context.Background(), options); err != nil {
		t.Fatal(err)
	}
	var ownership mcpOwnership
	ownershipPath := filepath.Join(home, ".local", "share", "pfm", "install", mcpOwnershipName)
	if err := json.Unmarshal([]byte(readFixture(t, ownershipPath)), &ownership); err != nil {
		t.Fatal(err)
	}
	if strings.Join(ownership.Clients, ",") != "chat" {
		t.Fatalf("owned clients=%v, want only the client PFM actually wired", ownership.Clients)
	}
	codexConfig := readFixture(t, filepath.Join(home, ".codex", "config.toml"))
	if !strings.Contains(codexConfig, "[mcp_servers.chat]") {
		t.Fatalf("Codex registration omitted owned chat client: %s", codexConfig)
	}
	if strings.Contains(codexConfig, "[mcp_servers.harvester]") {
		t.Fatalf("Codex registration claimed manually conflicting Harvester client: %s", codexConfig)
	}

	if _, err := Run(context.Background(), Options{
		Mode: ModeUninstall, Home: home, ConfigDir: canonical,
		ConfigDirs: []string{canonical}, MCPEnabled: options.MCPEnabled,
		Runner: &fakeRunner{},
	}); err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal([]byte(readFixture(t, filepath.Join(home, ".mcp.json"))), &document); err != nil {
		t.Fatal(err)
	}
	servers, _ := document["mcpServers"].(map[string]any)
	if _, ok := servers["harvester"]; !ok {
		t.Fatal("uninstall removed the conflicting manual Harvester registration")
	}
	if _, ok := servers["chat"]; ok {
		t.Fatal("uninstall retained PFM's owned chat registration")
	}
}

func TestInspectHarvesterClientCutoverNamesHealthyLegacyAndUnreadableStates(t *testing.T) {
	home := t.TempDir()
	writeFixture(t, filepath.Join(home, ".mcp.json"), `{"mcpServers":{"harvester":{"type":"http","url":"http://127.0.0.1:8377/mcp/harvester"}}}`)
	writeFixture(t, filepath.Join(home, ".codex", "config.toml"), "[mcp_servers.harvester]\ncommand = \"uv\"\nargs = [\"--directory\", \"/fixture/harvester\", \"run\", \"harvester\"]\n")

	reports := InspectHarvesterClientCutover(home, 8377)
	if len(reports) != 2 || reports[0].Client != "claude" || reports[0].State != MCPClientPFM || reports[0].Error != nil {
		t.Fatalf("Claude cutover report=%#v, want healthy PFM route", reports)
	}
	if reports[1].Client != "codex" || reports[1].State != MCPClientLegacyStandalone || reports[1].Error != nil {
		t.Fatalf("Codex cutover report=%#v, want legacy standalone route", reports)
	}

	writeFixture(t, filepath.Join(home, ".codex", "config.toml"), "broken = [\n")
	reports = InspectHarvesterClientCutover(home, 8377)
	if reports[1].State != MCPClientUnreadable || reports[1].Error == nil || !strings.Contains(reports[1].Error.Error(), "config.toml") {
		t.Fatalf("Codex unreadable report=%#v, want path-bearing parse error", reports[1])
	}
}
