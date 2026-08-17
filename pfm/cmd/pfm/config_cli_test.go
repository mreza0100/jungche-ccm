package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hostops/pfm/internal/config"
)

func TestConfigCLIRejectsGlobalConfigSyntaxAndLoadErrors(t *testing.T) {
	root := jailTest(t)
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{
			name: "missing path",
			args: []string{"--config"},
			want: "--config requires a path",
		},
		{
			name: "empty equals path",
			args: []string{"--config="},
			want: "--config requires a path",
		},
		{
			name: "duplicate path",
			args: []string{"--config", "one.json", "--config=two.json"},
			want: "--config may be specified only once",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run(test.args, &stdout, &stderr); code != 2 {
				t.Fatalf("run(%q) code=%d stdout=%q stderr=%q, want usage error", test.args, code, stdout.String(), stderr.String())
			}
			if stdout.Len() != 0 || !strings.Contains(stderr.String(), test.want) || !strings.Contains(stderr.String(), "usage:") {
				t.Fatalf("run(%q) stdout=%q stderr=%q, want %q and usage", test.args, stdout.String(), stderr.String(), test.want)
			}
		})
	}

	path := filepath.Join(root, "machine.json")
	if err := os.WriteFile(path, []byte(`{"version":1`), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--config", path, "mcp", "ls"}, &stdout, &stderr); code != 1 {
		t.Fatalf("run(malformed config) code=%d stdout=%q stderr=%q, want config failure", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "pfm: config: parse config "+path) || !strings.Contains(stderr.String(), "byte ") {
		t.Fatalf("run(malformed config) stdout=%q stderr=%q, want path and byte offset", stdout.String(), stderr.String())
	}
}

func TestConfigCLIMCPListReportsConfiguredStateAndSource(t *testing.T) {
	root := jailTest(t)
	path := writeConfigFixture(t, root, `{
  "version": 1,
  "mcp": {"servers": {"chat": {"enabled": true}}}
}`)

	var stdout, stderr bytes.Buffer
	if code := run([]string{"--config", path, "mcp", "ls"}, &stdout, &stderr); code != 0 {
		t.Fatalf("run(mcp ls) code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if got, want := stdout.String(), "chat\ttrue\tfile\n"; got != want {
		t.Fatalf("run(mcp ls) stdout=%q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("run(mcp ls) stderr=%q, want empty", stderr.String())
	}
}

func TestConfigCLIDisabledMCPServeExplainsEnablePath(t *testing.T) {
	root := jailTest(t)
	path := writeConfigFixture(t, root, `{
  "version": 1,
  "mcp": {"servers": {"chat": {"enabled": false}}}
}`)

	var stdout, stderr bytes.Buffer
	if code := run([]string{"--config", path, "mcp", "chat", "serve"}, &stdout, &stderr); code != 1 {
		t.Fatalf("run(disabled mcp serve) code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	want := "pfm mcp chat: disabled by config " + path + "; enable it with: pfm --config " + path + " mcp chat enable"
	if stdout.Len() != 0 || strings.TrimSpace(stderr.String()) != want {
		t.Fatalf("run(disabled mcp serve) stdout=%q stderr=%q, want actionable message %q", stdout.String(), stderr.String(), want)
	}
}

func TestConfigCLIMCPEnableDisableAreIdempotentWithoutStartingStdio(t *testing.T) {
	root := jailTest(t)
	path := writeConfigFixture(t, root, `{
  "version": 1,
  "mcp": {"servers": {"chat": {"enabled": false}}}
}`)

	assertMCPToggle := func(action, wantState string) []byte {
		t.Helper()
		var stdout, stderr bytes.Buffer
		if code := run([]string{"--config", path, "mcp", "chat", action}, &stdout, &stderr); code != 0 {
			t.Fatalf("run(mcp chat %s) code=%d stdout=%q stderr=%q", action, code, stdout.String(), stderr.String())
		}
		want := "chat\t" + action + "d\t" + wantState + "\n"
		if stdout.String() != want || stderr.Len() != 0 {
			t.Fatalf("run(mcp chat %s) stdout=%q stderr=%q, want stdout %q and empty stderr", action, stdout.String(), stderr.String(), want)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return content
	}

	initial, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	assertMCPToggle("enable", "updated")
	beforeRepeatEnable := mustReadFile(t, path)
	afterEnable := assertMCPToggle("enable", "unchanged")
	if !bytes.Equal(beforeRepeatEnable, afterEnable) {
		t.Fatal("idempotent enable changed the config")
	}
	if bytes.Equal(initial, afterEnable) {
		t.Fatal("enable did not update the config")
	}

	assertMCPToggle("disable", "updated")
	beforeRepeatDisable := mustReadFile(t, path)
	afterDisable := assertMCPToggle("disable", "unchanged")
	if !bytes.Equal(beforeRepeatDisable, afterDisable) {
		t.Fatal("idempotent disable changed the config")
	}
	if bytes.Equal(afterEnable, afterDisable) {
		t.Fatal("disable did not update the config")
	}
}

func TestDoctorConfigRenderingShowsEffectiveValuesAndSources(t *testing.T) {
	root := jailTest(t)
	home := filepath.Join(root, "config-home")
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "doctor-machine.json")
	if err := os.WriteFile(path, []byte(`{
  "version": 1,
  "accounts": [{"id": 7, "configDir": "~/account-seven"}],
  "claude": {"permissionMode": "prompt", "binary": "claude-fixture"},
  "codex": {"yolo": false, "binary": "codex-fixture"},
  "mcp": {"servers": {"chat": {"enabled": true}}}
}`), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load(path, home, nil)
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}

	var stdout bytes.Buffer
	printDoctorConfig(&stdout, commandRuntime{Config: loaded})
	want := strings.Join([]string{
		"doctor: config path=" + path + " exists=true",
		"doctor: config version=1 (file)",
		"doctor: config accounts=7:" + filepath.Join(home, "account-seven") + " (file)",
		"doctor: config claude.permissionMode=prompt (file)",
		"doctor: config claude.binary=claude-fixture (file)",
		"doctor: config codex.yolo=false (file)",
		"doctor: config codex.binary=codex-fixture (file)",
		"doctor: config mcp.servers.chat.enabled=true (file)",
	}, "\n") + "\n"
	if stdout.String() != want {
		t.Fatalf("printDoctorConfig() = %q, want %q", stdout.String(), want)
	}
}

func TestConfiguredAccountRosterIsTheExactTranscriptSearchBoundary(t *testing.T) {
	root := jailTest(t)
	home := os.Getenv("HOME")
	configuredRoot := filepath.Join(home, "configured-account")
	configuredProject := filepath.Join(configuredRoot, "projects", "configured-project")
	legacyProject := filepath.Join(home, ".claude", "projects", "legacy-project")
	for _, directory := range []string{configuredProject, legacyProject} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	configuredNeedle := "configured transcript phrase that must be discoverable"
	legacyNeedle := "legacy transcript phrase that configured accounts must exclude"
	if err := os.WriteFile(
		filepath.Join(configuredProject, "configured-session.jsonl"),
		[]byte(`{"type":"user","message":{"content":"`+configuredNeedle+`"}}`+"\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(legacyProject, "legacy-session.jsonl"),
		[]byte(`{"type":"user","message":{"content":"`+legacyNeedle+`"}}`+"\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	configPath := writeConfigFixture(t, root, `{
  "version": 1,
  "accounts": [{"id": 9, "configDir": "`+configuredRoot+`"}]
}`)

	find := func(name, needle string) (int, string, string) {
		t.Helper()
		excerptPath := filepath.Join(root, name+".txt")
		if err := os.WriteFile(excerptPath, []byte(needle+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		var stdout, stderr bytes.Buffer
		code := run([]string{"--config", configPath, "chat", "find", excerptPath}, &stdout, &stderr)
		return code, stdout.String(), stderr.String()
	}

	if code, stdout, stderr := find("configured", configuredNeedle); code != 0 ||
		!strings.Contains(stdout, "configured-session") || strings.Contains(stdout+stderr, "legacy-session") {
		t.Fatalf("configured lookup code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if code, stdout, stderr := find("legacy", legacyNeedle); code != 2 || stdout != "" ||
		!strings.Contains(stderr, "no session contains the excerpt") {
		t.Fatalf("legacy lookup escaped configured boundary: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func writeConfigFixture(t *testing.T, root, content string) string {
	t.Helper()
	path := filepath.Join(root, "machine.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return content
}
