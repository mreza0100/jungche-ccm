package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestResolvePathUsesAbsoluteXDGOrHomeConfig(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")

	t.Run("absolute XDG", func(t *testing.T) {
		xdg := filepath.Join(t.TempDir(), "xdg")
		t.Setenv("XDG_CONFIG_HOME", xdg)
		want := filepath.Join(xdg, "pfm", "config.json")
		if got := ResolvePath(home); got != want {
			t.Fatalf("ResolvePath(%q) = %q, want %q", home, got, want)
		}
	})

	for _, tc := range []struct {
		name string
		xdg  string
	}{
		{name: "relative XDG", xdg: "relative-config"},
		{name: "empty XDG", xdg: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", tc.xdg)
			want := filepath.Join(home, ".config", "pfm", "config.json")
			if got := ResolvePath(home); got != want {
				t.Fatalf("ResolvePath(%q) = %q, want %q", home, got, want)
			}
		})
	}
}

func TestDefaultsWithDiscoveryRoots(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	roots := []string{
		filepath.Join(home, ".cc", "one", "projects"),
		filepath.Join(home, ".cc", "two", "projects"),
	}

	got := Defaults(home, roots)
	if got.Version != Version {
		t.Fatalf("Version = %d, want %d", got.Version, Version)
	}
	wantAccounts := []Account{
		{ID: 1, ConfigDir: filepath.Join(home, ".cc", "one"), ProjectDir: filepath.Join(home, ".cc", "one", "projects"), Implicit: true},
		{ID: 2, ConfigDir: filepath.Join(home, ".cc", "two"), ProjectDir: filepath.Join(home, ".cc", "two", "projects")},
	}
	if !reflect.DeepEqual(got.Accounts, wantAccounts) {
		t.Fatalf("Accounts = %#v, want %#v", got.Accounts, wantAccounts)
	}
	if got.Claude != (Claude{PermissionMode: PermissionBypass, Binary: "claude"}) {
		t.Fatalf("Claude = %#v, want bypass/claude defaults", got.Claude)
	}
	if got.Codex != (Codex{Yolo: true, Binary: "codex"}) {
		t.Fatalf("Codex = %#v, want yolo/codex defaults", got.Codex)
	}
	if !reflect.DeepEqual(got.MCPServers, map[string]MCPServer{"chat": {Enabled: false}}) {
		t.Fatalf("MCPServers = %#v, want chat disabled", got.MCPServers)
	}
	for _, key := range []string{
		"version", "accounts", "claude.permissionMode", "claude.binary", "codex.yolo", "codex.binary", "mcp.servers.chat.enabled",
	} {
		if got.Source(key) != SourceDefault {
			t.Errorf("Source(%q) = %q, want %q", key, got.Source(key), SourceDefault)
		}
	}
}

func TestDefaultsWithoutDiscoveryRootsUseThreeAccounts(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	got := Defaults(home, nil)
	want := []Account{
		{ID: 1, ConfigDir: filepath.Join(home, ".cc", "1"), ProjectDir: filepath.Join(home, ".cc", "1", "projects"), Implicit: true},
		{ID: 2, ConfigDir: filepath.Join(home, ".cc", "2"), ProjectDir: filepath.Join(home, ".cc", "2", "projects")},
		{ID: 3, ConfigDir: filepath.Join(home, ".cc", "3"), ProjectDir: filepath.Join(home, ".cc", "3", "projects")},
	}
	if !reflect.DeepEqual(got.Accounts, want) {
		t.Fatalf("Accounts = %#v, want %#v", got.Accounts, want)
	}
}

func TestLoadAbsentReturnsDefaultsAndResolvedPath(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	path := filepath.Join(t.TempDir(), "nested", "machine.json")
	roots := []string{filepath.Join(home, ".cc", "1", "projects")}

	got, err := Load(path, home, roots)
	if err != nil {
		t.Fatalf("Load(absent) error = %v", err)
	}
	if got.Exists {
		t.Fatal("Load(absent) Exists = true, want false")
	}
	if got.Path != filepath.Clean(path) {
		t.Fatalf("Load(absent) Path = %q, want %q", got.Path, filepath.Clean(path))
	}
	if !reflect.DeepEqual(got.Accounts, Defaults(home, roots).Accounts) {
		t.Fatalf("Load(absent) accounts = %#v, want Defaults accounts", got.Accounts)
	}
}

func TestLoadConfiguredAccountsExpandHomeAndPreserveIDs(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	path := filepath.Join(t.TempDir(), "config.json")
	content := `{
  "version": 1,
  "accounts": [
    {"id": 9, "configDir": "~/account-nine"},
    {"id": 2, "configDir": "$HOME/account-two"},
    {"id": 17, "configDir": "/srv/claude/account-seventeen"}
  ],
  "claude": {"permissionMode": "prompt", "binary": "claude-custom"},
  "codex": {"yolo": false, "binary": "codex-custom"},
  "mcp": {"servers": {"chat": {"enabled": true}}}
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := Load(path, home, nil)
	if err != nil {
		t.Fatalf("Load(configured) error = %v", err)
	}
	want := []Account{
		{ID: 9, ConfigDir: filepath.Join(home, "account-nine"), ProjectDir: filepath.Join(home, "account-nine", "projects")},
		{ID: 2, ConfigDir: filepath.Join(home, "account-two"), ProjectDir: filepath.Join(home, "account-two", "projects")},
		{ID: 17, ConfigDir: "/srv/claude/account-seventeen", ProjectDir: "/srv/claude/account-seventeen/projects"},
	}
	if !reflect.DeepEqual(got.Accounts, want) {
		t.Fatalf("Accounts = %#v, want %#v", got.Accounts, want)
	}
	if got.AccountIDs() == nil || !reflect.DeepEqual(got.AccountIDs(), []int{9, 2, 17}) {
		t.Fatalf("AccountIDs = %#v, want [9 2 17]", got.AccountIDs())
	}
	if got.Claude != (Claude{PermissionMode: PermissionPrompt, Binary: "claude-custom"}) {
		t.Fatalf("Claude = %#v, want configured values", got.Claude)
	}
	if got.Codex != (Codex{Yolo: false, Binary: "codex-custom"}) {
		t.Fatalf("Codex = %#v, want configured values", got.Codex)
	}
	if !got.Exists || got.Source("accounts") != SourceFile || got.Source("mcp.servers.chat.enabled") != SourceFile {
		t.Fatalf("configured sources/exists = exists:%v accounts:%q chat:%q", got.Exists, got.Source("accounts"), got.Source("mcp.servers.chat.enabled"))
	}
	for _, key := range []string{"claude.permissionMode", "claude.binary", "codex.yolo", "codex.binary", "version"} {
		if got.Source(key) != SourceFile {
			t.Errorf("Source(%q) = %q, want %q", key, got.Source(key), SourceFile)
		}
	}
}

func TestLoadUsesResolvePathWhenPathIsEmpty(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	xdg := filepath.Join(root, "xdg")
	t.Setenv("XDG_CONFIG_HOME", xdg)
	path := filepath.Join(xdg, "pfm", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"version":1}`), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := Load("", home, nil)
	if err != nil {
		t.Fatalf("Load(empty path) error = %v", err)
	}
	if got.Path != path || !got.Exists {
		t.Fatalf("Load(empty path) = path %q exists %v, want %q true", got.Path, got.Exists, path)
	}
}

func TestLoadRejectsUnknownKeysAtEveryConfigLevel(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	cases := []struct {
		name string
		json string
		want string
	}{
		{name: "top level", json: `{"version":1,"mystery":true}`, want: "mystery"},
		{name: "claude", json: `{"version":1,"claude":{"mystery":true}}`, want: "mystery"},
		{name: "codex", json: `{"version":1,"codex":{"mystery":true}}`, want: "mystery"},
		{name: "account", json: `{"version":1,"accounts":[{"id":1,"configDir":"/opt/fixture/cc","projectDir":"/opt/fixture/projects"}]}`, want: "projectDir"},
		{name: "mcp object", json: `{"version":1,"mcp":{"mystery":true}}`, want: "mystery"},
		{name: "server object", json: `{"version":1,"mcp":{"servers":{"chat":{"enabled":false,"mystery":true}}}}`, want: "mystery"},
		{name: "unregistered server", json: `{"version":1,"mcp":{"servers":{"not-registered":{"enabled":true}}}}`, want: "mcp.servers.not-registered"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(path, []byte(tc.json), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := Load(path, home, nil)
			if err == nil {
				t.Fatalf("Load(%s) succeeded, want unknown-key error", tc.json)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want it to name %q", err, tc.want)
			}
		})
	}
}

func TestLoadMalformedJSONNamesPathAndByte(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	cases := []struct {
		name    string
		content string
	}{
		{name: "syntax", content: `{"version":1`},
		{name: "wrong type", content: `{"version":"one"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "machine.json")
			if err := os.WriteFile(path, []byte(tc.content), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := Load(path, home, nil)
			if err == nil {
				t.Fatal("Load malformed config succeeded")
			}
			message := err.Error()
			if !strings.Contains(message, "parse config "+path) {
				t.Errorf("error = %q, want path %q", message, path)
			}
			if !strings.Contains(message, "byte ") {
				t.Errorf("error = %q, want byte offset", message)
			}
		})
	}
}

func TestLoadRejectsInvalidAccountRoster(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	cases := []struct {
		name string
		json string
		want string
	}{
		{name: "empty", json: `{"version":1,"accounts":[]}`, want: "at least one account"},
		{name: "non-positive id", json: `{"version":1,"accounts":[{"id":0,"configDir":"/tmp/cc"}]}`, want: "positive"},
		{name: "duplicate id", json: `{"version":1,"accounts":[{"id":2,"configDir":"/tmp/a"},{"id":2,"configDir":"/tmp/b"}]}`, want: "duplicate"},
		{name: "relative path", json: `{"version":1,"accounts":[{"id":1,"configDir":"relative"}]}`, want: "must be absolute"},
		{name: "nul path", json: "{\"version\":1,\"accounts\":[{\"id\":1,\"configDir\":\"/tmp/a\\u0000b\"}]}", want: "must not contain NUL"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(path, []byte(tc.json), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := Load(path, home, nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Load error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestLoadMCPServersHaveIndependentDefaultsAndSources(t *testing.T) {
	registered := map[string]MCPServer{
		"chat":      {Enabled: false},
		"harvester": {Enabled: false},
	}
	home := filepath.Join(t.TempDir(), "home")
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"mcp":{"servers":{"harvester":{"enabled":true}}}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := loadWithMCPServers(path, home, nil, registered)
	if err != nil {
		t.Fatalf("Load(mcp) error = %v", err)
	}
	if got.MCPServers["chat"].Enabled {
		t.Fatal("chat became enabled when only harvester was configured")
	}
	if !got.MCPServers["harvester"].Enabled {
		t.Fatal("harvester did not take its configured enabled value")
	}
	if got.Source("mcp.servers.chat.enabled") != SourceDefault {
		t.Fatalf("chat source = %q, want default", got.Source("mcp.servers.chat.enabled"))
	}
	if got.Source("mcp.servers.harvester.enabled") != SourceFile {
		t.Fatalf("harvester source = %q, want file", got.Source("mcp.servers.harvester.enabled"))
	}
}

func TestSetMCPServerIsIndependentAtomicAndIdempotent(t *testing.T) {
	registered := map[string]MCPServer{
		"chat":      {Enabled: false},
		"harvester": {Enabled: false},
	}
	home := filepath.Join(t.TempDir(), "home")
	path := filepath.Join(t.TempDir(), "deep", "machine.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{
  "version": 1,
  "claude": {"binary": "claude-custom"},
  "mcp": {"servers": {"chat": {"enabled": false}, "harvester": {"enabled": false}}}
}`), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadWithMCPServers(path, home, nil, registered)
	if err != nil {
		t.Fatalf("initial Load error = %v", err)
	}

	changed, err := SetMCPServer(loaded, "chat", true)
	if err != nil || !changed {
		t.Fatalf("enable chat = changed:%v err:%v, want changed true", changed, err)
	}
	afterChat, err := loadWithMCPServers(path, home, nil, registered)
	if err != nil {
		t.Fatalf("Load(after chat) error = %v", err)
	}
	if !afterChat.MCPServers["chat"].Enabled || afterChat.MCPServers["harvester"].Enabled {
		t.Fatalf("after chat toggle MCPServers = %#v, want chat true and harvester false", afterChat.MCPServers)
	}
	if afterChat.Claude.Binary != "claude-custom" {
		t.Fatal("SetMCPServer discarded unrelated configuration")
	}
	beforeNoop, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	changed, err = SetMCPServer(afterChat, "chat", true)
	if err != nil || changed {
		t.Fatalf("repeat enable chat = changed:%v err:%v, want no-op", changed, err)
	}
	afterNoop, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(afterNoop, beforeNoop) {
		t.Fatal("idempotent SetMCPServer rewrote the config")
	}

	changed, err = SetMCPServer(afterChat, "harvester", true)
	if err != nil || !changed {
		t.Fatalf("enable harvester = changed:%v err:%v, want changed true", changed, err)
	}
	afterHarvester, err := loadWithMCPServers(path, home, nil, registered)
	if err != nil {
		t.Fatalf("Load(after harvester) error = %v", err)
	}
	if !afterHarvester.MCPServers["chat"].Enabled || !afterHarvester.MCPServers["harvester"].Enabled {
		t.Fatalf("after harvester toggle MCPServers = %#v, want both enabled", afterHarvester.MCPServers)
	}

	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".config.json.tmp-") {
			t.Errorf("atomic scratch file remains: %s", entry.Name())
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("config mode = %o, want 600", info.Mode().Perm())
	}
}

func TestSetMCPServerRejectsUnknownServerWithoutWriting(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	path := filepath.Join(t.TempDir(), "config.json")
	loaded, err := Load(path, home, nil)
	if err != nil {
		t.Fatal(err)
	}
	if changed, err := SetMCPServer(loaded, "not-registered", true); err == nil || changed {
		t.Fatalf("unknown server = changed:%v err:%v, want error and no change", changed, err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unknown server created config: stat error = %v", err)
	}
}

func TestRegisteredMCPServersIsSorted(t *testing.T) {
	got := RegisteredMCPServers()
	if !sort.StringsAreSorted(got) || !reflect.DeepEqual(got, []string{"chat"}) {
		t.Fatalf("RegisteredMCPServers() = %#v, want sorted production registry", got)
	}
}

func TestLoadRejectsMissingVersionAndWrongVersion(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	for _, tc := range []struct {
		name string
		json string
		want string
	}{
		{name: "missing", json: `{}`, want: "required key \"version\" is missing"},
		{name: "wrong", json: `{"version":99}`, want: "version must be 1, got 99"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(path, []byte(tc.json), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := Load(path, home, nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Load error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestSetMCPServerWritesValidJSON(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	path := filepath.Join(t.TempDir(), "new", "config.json")
	loaded, err := Load(path, home, nil)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := SetMCPServer(loaded, "chat", true)
	if err != nil || !changed {
		t.Fatalf("SetMCPServer = changed:%v err:%v", changed, err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(content, &decoded); err != nil {
		t.Fatalf("written config is not JSON: %v", err)
	}
	if got, ok := decoded["version"].(float64); !ok || got != Version {
		t.Fatalf("written version = %#v, want %d", decoded["version"], Version)
	}
}
