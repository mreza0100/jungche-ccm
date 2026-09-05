package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pfmconfig "hostops/pfm/internal/config"
	"hostops/pfm/internal/deps"
	pfmengine "hostops/pfm/internal/engine"
	"hostops/pfm/internal/installer"
	"hostops/pfm/internal/paths"
)

func TestDoctorWarnsWhenLegacyHarvesterClientsStillOwnTheRoute(t *testing.T) {
	root := jailTest(t)
	home := filepath.Join(root, "home")
	if err := os.WriteFile(
		filepath.Join(home, ".mcp.json"),
		[]byte(`{"mcpServers":{"harvester":{"type":"stdio","command":"uv","args":["--directory","/fixture/legacy-harvester","run","harvester"]}}}`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(home, ".codex", "config.toml"),
		[]byte("[mcp_servers.harvester]\ncommand = \"uv\"\nargs = [\"--directory\", \"/fixture/legacy-harvester\", \"run\", \"harvester\"]\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	runtime, err := loadCommandRuntime("")
	if err != nil {
		t.Fatal(err)
	}
	runtime.Config.CodexAccounts = []pfmconfig.CodexAccount{{ID: 1, Home: filepath.Join(home, ".codex")}}
	var stdout, stderr bytes.Buffer
	if code := runDoctor(nil, &stdout, &stderr, runtime); code != 1 {
		t.Fatalf("doctor code=%d stdout=%q stderr=%q, want warning exit", code, stdout.String(), stderr.String())
	}
	for _, client := range []string{"claude", "codex"} {
		want := "doctor: mcp client=" + client + " harvester=legacy-standalone"
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("doctor output missing %q:\n%s", want, stdout.String())
		}
	}
	if strings.Contains(stdout.String(), "/fixture/legacy-harvester") {
		t.Fatalf("doctor leaked registration paths instead of a bounded diagnosis:\n%s", stdout.String())
	}
}

func TestDoctorReportsHarvesterCutoverForModernForeignAndUnreadableClients(t *testing.T) {
	root := jailTest(t)
	home := filepath.Join(root, "home")
	codexPath := filepath.Join(home, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(codexPath), 0o700); err != nil {
		t.Fatal(err)
	}
	write := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	run := func(t *testing.T) string {
		t.Helper()
		runtime, err := loadCommandRuntime("")
		if err != nil {
			t.Fatal(err)
		}
		runtime.Config.CodexAccounts = []pfmconfig.CodexAccount{{ID: 1, Home: filepath.Join(home, ".codex")}}
		var stdout, stderr bytes.Buffer
		if code := runDoctor(nil, &stdout, &stderr, runtime); code != 0 {
			t.Fatalf("doctor code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
		return stdout.String()
	}

	t.Run("modern no-auth loopback routes are complete", func(t *testing.T) {
		write(filepath.Join(home, ".mcp.json"), `{"mcpServers":{"harvester":{"type":"http","url":"http://127.0.0.1:8377/mcp/harvester"}}}`)
		write(codexPath, "[mcp_servers.harvester]\nurl = \"http://127.0.0.1:8377/mcp/harvester\"\n")
		output := run(t)
		if !strings.Contains(output, "doctor: mcp client-cutover=complete") {
			t.Fatalf("modern no-auth routes were not reported complete:\n%s", output)
		}
	})

	t.Run("loopback routes with retired authentication are incomplete", func(t *testing.T) {
		write(filepath.Join(home, ".mcp.json"), `{"mcpServers":{"harvester":{"type":"http","url":"http://127.0.0.1:8377/mcp/harvester","headers":{"Authorization":"Bearer retired"}}}}`)
		write(codexPath, "[mcp_servers.harvester]\nurl = \"http://127.0.0.1:8377/mcp/harvester\"\n[mcp_servers.harvester.headers]\nAuthorization = \"Bearer retired\"\n")
		runtime, err := loadCommandRuntime("")
		if err != nil {
			t.Fatal(err)
		}
		runtime.Config.CodexAccounts = []pfmconfig.CodexAccount{{ID: 1, Home: filepath.Join(home, ".codex")}}
		var stdout, stderr bytes.Buffer
		if code := runDoctor(nil, &stdout, &stderr, runtime); code != 1 {
			t.Fatalf("doctor code=%d stdout=%q stderr=%q, want retired-auth warnings", code, stdout.String(), stderr.String())
		}
		for _, client := range []string{"claude", "codex"} {
			want := "doctor: mcp client=" + client + " harvester=foreign-registration warning=consumer cutover incomplete"
			if !strings.Contains(stdout.String(), want) {
				t.Fatalf("retired-auth route output missing %q:\n%s", want, stdout.String())
			}
		}
	})

	t.Run("foreign routes are warnings for both clients", func(t *testing.T) {
		write(filepath.Join(home, ".mcp.json"), `{"mcpServers":{"harvester":{"type":"http","url":"https://foreign.invalid/mcp"}}}`)
		write(codexPath, "[mcp_servers.harvester]\nurl = \"https://foreign.invalid/mcp\"\n")
		runtime, err := loadCommandRuntime("")
		if err != nil {
			t.Fatal(err)
		}
		runtime.Config.CodexAccounts = []pfmconfig.CodexAccount{{ID: 1, Home: filepath.Join(home, ".codex")}}
		var stdout, stderr bytes.Buffer
		if code := runDoctor(nil, &stdout, &stderr, runtime); code != 1 {
			t.Fatalf("doctor code=%d stdout=%q stderr=%q, want foreign-route warnings", code, stdout.String(), stderr.String())
		}
		for _, client := range []string{"claude", "codex"} {
			want := "doctor: mcp client=" + client + " harvester=foreign-registration warning=consumer cutover incomplete"
			if !strings.Contains(stdout.String(), want) {
				t.Fatalf("foreign route output missing %q:\n%s", want, stdout.String())
			}
		}
	})

	t.Run("malformed Claude JSON is unreadable", func(t *testing.T) {
		write(filepath.Join(home, ".mcp.json"), `{"mcpServers":`)
		write(codexPath, "[mcp_servers.harvester]\nurl = \"http://127.0.0.1:8377/mcp/harvester\"\n")
		runtime, err := loadCommandRuntime("")
		if err != nil {
			t.Fatal(err)
		}
		runtime.Config.CodexAccounts = []pfmconfig.CodexAccount{{ID: 1, Home: filepath.Join(home, ".codex")}}
		var stdout, stderr bytes.Buffer
		if code := runDoctor(nil, &stdout, &stderr, runtime); code != 1 {
			t.Fatalf("doctor code=%d stdout=%q stderr=%q, want unreadable warning", code, stdout.String(), stderr.String())
		}
		if !strings.Contains(stdout.String(), "doctor: mcp client=claude harvester=unreadable error=") {
			t.Fatalf("malformed Claude JSON was not distinguished from absence:\n%s", stdout.String())
		}
	})

	t.Run("malformed Codex TOML is unreadable", func(t *testing.T) {
		write(filepath.Join(home, ".mcp.json"), `{"mcpServers":{"harvester":{"type":"http","url":"http://127.0.0.1:8377/mcp/harvester"}}}`)
		write(codexPath, "[mcp_servers.harvester\n")
		runtime, err := loadCommandRuntime("")
		if err != nil {
			t.Fatal(err)
		}
		runtime.Config.CodexAccounts = []pfmconfig.CodexAccount{{ID: 1, Home: filepath.Join(home, ".codex")}}
		var stdout, stderr bytes.Buffer
		if code := runDoctor(nil, &stdout, &stderr, runtime); code != 1 {
			t.Fatalf("doctor code=%d stdout=%q stderr=%q, want unreadable warning", code, stdout.String(), stderr.String())
		}
		if !strings.Contains(stdout.String(), "doctor: mcp client=codex harvester=unreadable error=") {
			t.Fatalf("malformed Codex TOML was not distinguished from absence:\n%s", stdout.String())
		}
	})
}

func TestDoctorEnumeratesExternalDependenciesAndInstalledHooks(t *testing.T) {
	t.Setenv("HARVESTER_BROWSER", "") // golden doctor output must not depend on the ambient opt-in gate (review-2 S3)
	jailTest(t)
	runtime, err := loadCommandRuntime("")
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := runDoctor(nil, &stdout, &stderr, runtime); code != 0 {
		t.Fatalf("doctor code=%d, want clean injected probes\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
	}
	for _, wanted := range []string{
		"doctor: dep tmux ",
		"doctor: hook claude[1] ",
	} {
		if !strings.Contains(stdout.String(), wanted) {
			t.Fatalf("doctor output missing %q:\n%s", wanted, stdout.String())
		}
	}
	if strings.Contains(stdout.String(), "doctor: hook codex[") {
		t.Fatalf("doctor invented a Codex hook for an empty roster:\n%s", stdout.String())
	}
}

func TestDependencyDoctorRowsKeepMissingBrokenAndSkippedDistinct(t *testing.T) {
	saved := dependencyProbeOverride
	t.Cleanup(func() { dependencyProbeOverride = saved })
	entries := []deps.Entry{
		{Name: "tmux", Required: true, MinVersion: "1.8"},
		{Name: "ps", Required: true, Platforms: []string{"darwin"}},
		{Name: "codex", Required: true, InstallHint: "install configured Codex"},
		{Name: "claude", Required: true},
	}
	dependencyProbeOverride = func(context.Context, []deps.Entry, deps.ProbeOptions) []deps.Result {
		return []deps.Result{
			{Entry: entries[0], State: deps.StateOK, Path: "/fixture/tmux", Version: "3.4"},
			{Entry: entries[1], State: deps.StateSkipped, Error: "not this platform"},
			{Entry: entries[2], State: deps.StateMissing},
			{Entry: entries[3], State: deps.StateBroken, Path: "/fixture/claude", Error: "exit status 1", Raw: "damaged install\nmore"},
		}
	}
	var output bytes.Buffer
	if warnings := printDependencyDoctor(context.Background(), &output, entries, deps.ProbeOptions{}); warnings != 2 {
		t.Fatalf("warnings=%d, want 2\n%s", warnings, output.String())
	}
	want := strings.Join([]string{
		"doctor: dep tmux path=/fixture/tmux version=3.4 min=1.8 ok",
		"doctor: dep ps platform=darwin skipped (not this platform)",
		"doctor: dep codex path=(none) MISSING required — install: install configured Codex",
		`doctor: dep claude path=/fixture/claude broken error=exit status 1 raw="damaged install"`,
		"",
	}, "\n")
	if output.String() != want {
		t.Fatalf("dependency rows:\n%s\nwant:\n%s", output.String(), want)
	}
}

func TestInstallPreflightRefusesRequiredDependencyBeforeInstallerRuns(t *testing.T) {
	savedProbe, savedInstaller := dependencyProbeOverride, runInstaller
	t.Cleanup(func() {
		dependencyProbeOverride = savedProbe
		runInstaller = savedInstaller
	})
	dependencyProbeOverride = func(_ context.Context, entries []deps.Entry, _ deps.ProbeOptions) []deps.Result {
		for _, entry := range entries {
			if entry.Name == "tmux" {
				return []deps.Result{{Entry: entry, State: deps.StateMissing}}
			}
		}
		t.Fatal("tmux registry entry missing")
		return nil
	}
	called := false
	runInstaller = func(context.Context, installer.Options) (installer.Report, error) {
		called = true
		return installer.Report{}, nil
	}
	home := t.TempDir()
	runtime := commandRuntime{
		Config: pfmconfig.Config{Claude: pfmconfig.Claude{Binary: "claude"}, Codex: pfmconfig.Codex{Binary: "codex"}},
		Paths:  paths.Values{Home: home, Roots: map[pfmengine.ID][]string{pfmengine.Codex: {filepath.Join(home, ".codex")}}},
	}
	var stdout, stderr bytes.Buffer
	if code := runInstall([]string{"--yes", "--skip-harvest"}, &stdout, &stderr, runtime); code != 1 {
		t.Fatalf("install code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if called || !strings.Contains(stdout.String(), "doctor: dep tmux path=(none) MISSING required") || !strings.Contains(stderr.String(), "required dependency preflight failed") {
		t.Fatalf("called=%t stdout=%s stderr=%s", called, stdout.String(), stderr.String())
	}
}

func TestInstallPreflightDoesNotRefuseBrokenOptionalEngine(t *testing.T) {
	savedProbe, savedInstaller := dependencyProbeOverride, runInstaller
	t.Cleanup(func() {
		dependencyProbeOverride = savedProbe
		runInstaller = savedInstaller
	})
	dependencyProbeOverride = func(_ context.Context, entries []deps.Entry, _ deps.ProbeOptions) []deps.Result {
		for _, entry := range entries {
			if entry.Name == "codex" {
				return []deps.Result{{Entry: entry, State: deps.StateBroken, Path: "/fixture/codex", Error: "self-doctor failed: auth missing"}}
			}
		}
		t.Fatal("codex registry entry missing")
		return nil
	}
	called := false
	runInstaller = func(context.Context, installer.Options) (installer.Report, error) {
		called = true
		return installer.Report{}, nil
	}
	home := t.TempDir()
	runtime := commandRuntime{
		Config: pfmconfig.Config{
			Codex:         pfmconfig.Codex{Binary: "codex"},
			CodexAccounts: []pfmconfig.CodexAccount{{ID: 1, Home: filepath.Join(home, ".codex")}},
		},
		Paths: paths.Values{Home: home},
	}
	var stdout, stderr bytes.Buffer
	if code := runInstall([]string{"--yes", "--skip-harvest"}, &stdout, &stderr, runtime); code != 0 {
		t.Fatalf("install code=%d stdout=%s stderr=%s, want optional Codex failure to remain non-blocking", code, stdout.String(), stderr.String())
	}
	if !called || !strings.Contains(stdout.String(), "dep codex") || !strings.Contains(stdout.String(), "auth missing") {
		t.Fatalf("called=%t stdout=%s stderr=%s, want a visible optional failure followed by install", called, stdout.String(), stderr.String())
	}
}

func TestInstallPreflightFailureStillPreviewsInDryRun(t *testing.T) {
	savedProbe, savedInstaller := dependencyProbeOverride, runInstaller
	t.Cleanup(func() {
		dependencyProbeOverride = savedProbe
		runInstaller = savedInstaller
	})
	dependencyProbeOverride = func(_ context.Context, entries []deps.Entry, _ deps.ProbeOptions) []deps.Result {
		for _, entry := range entries {
			if entry.Name == "tmux" {
				return []deps.Result{{Entry: entry, State: deps.StateMissing}}
			}
		}
		t.Fatal("tmux registry entry missing")
		return nil
	}
	called := false
	runInstaller = func(_ context.Context, options installer.Options) (installer.Report, error) {
		if options.Mode != installer.ModeDryRun {
			t.Errorf("install mode=%v, want dry run", options.Mode)
		}
		called = true
		return installer.Report{}, nil
	}
	home := t.TempDir()
	runtime := commandRuntime{
		Config: pfmconfig.Config{Claude: pfmconfig.Claude{Binary: "claude"}, Codex: pfmconfig.Codex{Binary: "codex"}},
		Paths:  paths.Values{Home: home, Roots: map[pfmengine.ID][]string{pfmengine.Codex: {filepath.Join(home, ".codex")}}},
	}
	var stdout, stderr bytes.Buffer
	if code := runInstall([]string{"--skip-harvest"}, &stdout, &stderr, runtime); code != 1 {
		t.Fatalf("install code=%d, want 1\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
	}
	if !called {
		t.Fatal("read-only preview never ran — a fresh machine gets no plan at all")
	}
	if !strings.Contains(stdout.String(), "doctor: dep tmux path=(none) MISSING required") || !strings.Contains(stderr.String(), "required dependency preflight failed") {
		t.Fatalf("missing preflight report:\nstdout=%s\nstderr=%s", stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String(), "if you agree, run again") {
		t.Fatalf("apply confirmation offered despite failed preflight:\n%s", stdout.String())
	}
}

func TestHookDoctorRowsCountMissingBrokenAndDriftWarnings(t *testing.T) {
	saved := hookProbeOverride
	t.Cleanup(func() { hookProbeOverride = saved })
	home := t.TempDir()
	hookProbeOverride = func(string, pfmconfig.Config) []installer.HookProbeResult {
		return []installer.HookProbeResult{
			{Hook: installer.ExpectedHook{Target: "claude[1]", File: filepath.Join(home, ".claude", "settings.json"), Event: "SessionEnd", Name: "clear-kill"}, State: "ok"},
			{Hook: installer.ExpectedHook{Target: "codex", File: filepath.Join(home, ".codex", "hooks.json"), Event: "SessionStart", Name: "clear-kill"}, State: "missing"},
			{Hook: installer.ExpectedHook{Target: "claude[2]", File: filepath.Join(home, ".cc", "2", "settings.json"), Event: "UserPromptSubmit", Name: "usage"}, State: "broken", Error: "parse error"},
			{Hook: installer.ExpectedHook{Target: "ownership", File: filepath.Join(home, "ledger.json"), Event: "SessionEnd", Name: "unexpected"}, State: "drift", Error: "ledger owns 1 hook absent from expectations"},
		}
	}
	var output bytes.Buffer
	if warnings := printHookDoctor(&output, home, pfmconfig.Config{}); warnings != 3 {
		t.Fatalf("warnings=%d output=%s", warnings, output.String())
	}
	for _, wanted := range []string{
		"doctor: hook claude[1] settings.json SessionEnd clear-kill ok",
		"doctor: hook codex hooks.json SessionStart clear-kill MISSING — run pfm install",
		"doctor: hook claude[2] settings.json UserPromptSubmit usage broken error=parse error",
		"doctor: hook ownership ledger.json SessionEnd unexpected drift error=ledger owns 1 hook absent from expectations",
	} {
		if !strings.Contains(output.String(), wanted) {
			t.Errorf("output missing %q:\n%s", wanted, output.String())
		}
	}
}
