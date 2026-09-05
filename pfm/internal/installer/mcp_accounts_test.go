package installer

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMCPWiresActualClaudeRegistriesAndHonorsEmptyCodex(t *testing.T) {
	home := t.TempDir()
	primary := filepath.Join(home, ".claude")
	secondary := filepath.Join(home, "account-two")
	paths := []string{filepath.Join(home, ".claude.json"), filepath.Join(secondary, ".claude.json")}
	for _, path := range paths {
		writeFixture(t, path, `{"oauthAccount":{"accountUuid":"private"},"mcpServers":{"foreign":{"command":"custom"}}}`)
	}
	options := Options{Home: home, ConfigDir: primary, ConfigDirs: []string{primary, secondary}, CodexHomes: []string{}, Mode: ModeApply, Runner: &fakeRunner{}, Stdout: io.Discard, MCPEnabled: map[string]bool{"chat": true}, MCPPort: 8377}
	if _, err := Run(context.Background(), options); err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		var doc map[string]any
		if err := json.Unmarshal([]byte(readFixture(t, path)), &doc); err != nil {
			t.Fatal(err)
		}
		servers := doc["mcpServers"].(map[string]any)
		if servers["chat"] == nil || servers["foreign"] == nil || doc["oauthAccount"] == nil {
			t.Errorf("registry %s lost wiring or private state: %#v", path, doc)
		}
	}
	if _, err := os.Stat(filepath.Join(home, ".codex")); !os.IsNotExist(err) {
		t.Errorf("empty Codex roster wrote .codex: %v", err)
	}
	// A user replacement after installation is preserved on uninstall.
	replacement := `{"oauthAccount":{"accountUuid":"private"},"mcpServers":{"chat":{"command":"manual"},"foreign":{"command":"custom"}}}`
	writeFixture(t, paths[1], replacement)
	options.Mode = ModeUninstall
	if _, err := Run(context.Background(), options); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(readFixture(t, paths[0]), `"chat"`) {
		t.Error("owned primary registration survived uninstall")
	}
	if got := readFixture(t, paths[1]); got != replacement {
		t.Errorf("manual replacement was changed: %s", got)
	}
}
