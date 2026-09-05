package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pfmconfig "hostops/pfm/internal/config"
	pfmengine "hostops/pfm/internal/engine"
	"hostops/pfm/internal/paths"
)

// TestHarvestAskRunsBothConfiguredAdapters is the issue-6 regression: the
// documented subcommand did not exist, so -p was parsed as an unknown fetch
// flag and neither configured ask adapter could ever run.
func TestHarvestAskRunsBothConfiguredAdapters(t *testing.T) {
	for _, testCase := range []struct {
		name      string
		engine    pfmengine.ID
		model     string
		effort    string
		homeEnv   string
		wantArgs  string
		configure func(*pfmconfig.Config, string, string)
	}{
		{
			name:     "Claude",
			engine:   pfmengine.Claude,
			model:    "claude-fixture-model",
			effort:   "low",
			homeEnv:  "CLAUDE_CONFIG_DIR",
			wantArgs: "-p|--model|claude-fixture-model|--effort|low|--output-format|text|",
			configure: func(machine *pfmconfig.Config, binary, accountHome string) {
				machine.Claude = pfmconfig.Claude{Binary: binary}
				machine.Accounts = []pfmconfig.Account{{ID: 1, ConfigDir: accountHome}}
			},
		},
		{
			name:     "Codex",
			engine:   pfmengine.Codex,
			model:    "codex-fixture-model",
			effort:   "high",
			homeEnv:  "CODEX_HOME",
			wantArgs: "exec|--model|codex-fixture-model|-c|model_reasoning_effort=\"high\"|--ephemeral|--skip-git-repo-check|--color|never|-|",
			configure: func(machine *pfmconfig.Config, binary, accountHome string) {
				machine.Codex = pfmconfig.Codex{Binary: binary}
				machine.CodexAccounts = []pfmconfig.CodexAccount{{ID: 1, Home: accountHome}}
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			home := t.TempDir()
			installAskLauncherStub(t, home)
			accountHome := filepath.Join(home, "account")
			if err := os.MkdirAll(accountHome, 0o700); err != nil {
				t.Fatal(err)
			}
			source := filepath.Join(home, "source.txt")
			if err := os.WriteFile(source, []byte("the fixture answer is forty-two\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			capture := filepath.Join(home, "ask-input.txt")
			binary := filepath.Join(home, "ask-engine")
			script := "#!/bin/sh\n" +
				"printf 'home=%s\\n' \"${" + testCase.homeEnv + "-}\" > \"$PFM_ASK_CAPTURE\"\n" +
				"printf 'args=' >> \"$PFM_ASK_CAPTURE\"\n" +
				"printf '%s|' \"$@\" >> \"$PFM_ASK_CAPTURE\"\n" +
				"printf '\\n' >> \"$PFM_ASK_CAPTURE\"\n" +
				"cat >> \"$PFM_ASK_CAPTURE\"\n" +
				"printf 'fixture answer\\n'\n"
			if err := os.WriteFile(binary, []byte(script), 0o700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PFM_ASK_CAPTURE", capture)
			t.Setenv("WEBFETCH_DIR", filepath.Join(home, "cache"))

			machine := pfmconfig.Config{
				Ask: pfmconfig.AskConfig{
					Engine: testCase.engine,
					Prefs: map[pfmengine.ID]pfmconfig.EnginePrefs{
						testCase.engine: {Model: testCase.model, Effort: testCase.effort},
					},
				},
			}
			testCase.configure(&machine, binary, accountHome)
			runtime := commandRuntime{Config: machine, Paths: paths.Values{Home: home}}

			var stdout, stderr bytes.Buffer
			code := runHarvest([]string{"ask", "-p", "What is the fixture answer?", source}, &stdout, &stderr, runtime)
			if code != 0 {
				t.Fatalf("harvest ask code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			if got := stdout.String(); got != "fixture answer\n" {
				t.Fatalf("harvest ask stdout=%q", got)
			}
			raw, err := os.ReadFile(capture)
			if err != nil {
				t.Fatalf("read engine capture: %v", err)
			}
			prompt := string(raw)
			for _, want := range []string{
				"home=" + accountHome,
				"args=" + testCase.wantArgs,
				"source: " + source,
				"TASK: What is the fixture answer?",
				"EVIDENCE",
			} {
				if !strings.Contains(prompt, want) {
					t.Errorf("engine capture missing %q:\n%s", want, prompt)
				}
			}
		})
	}
}

func TestHarvestAskPreservesFailureReceiptsAndCleansThemUp(t *testing.T) {
	home := t.TempDir()
	installAskLauncherStub(t, home)
	accountHome := filepath.Join(home, "codex-home")
	if err := os.MkdirAll(accountHome, 0o700); err != nil {
		t.Fatal(err)
	}
	good := filepath.Join(home, "good.txt")
	if err := os.WriteFile(good, []byte("load-bearing local evidence\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(home, "missing.txt")
	promptCapture := filepath.Join(home, "prompt.txt")
	fileCapture := filepath.Join(home, "prepared-files.txt")
	binary := filepath.Join(home, "codex-fixture")
	script := "#!/bin/sh\n" +
		"cat > \"$PFM_ASK_PROMPT\"\n" +
		"sed -n 's/^[0-9][0-9]*\\. \\(.*\\) — source:.*$/\\1/p' \"$PFM_ASK_PROMPT\" | while IFS= read -r prepared; do\n" +
		"  printf '%s\\n' \"--- $prepared\" >> \"$PFM_ASK_FILES\"\n" +
		"  cat \"$prepared\" >> \"$PFM_ASK_FILES\"\n" +
		"done\n" +
		"printf 'receipt-aware answer\\n'\n"
	if err := os.WriteFile(binary, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PFM_ASK_PROMPT", promptCapture)
	t.Setenv("PFM_ASK_FILES", fileCapture)
	t.Setenv("WEBFETCH_DIR", filepath.Join(home, "cache"))
	runtime := commandRuntime{
		Config: pfmconfig.Config{
			Codex:         pfmconfig.Codex{Binary: binary},
			CodexAccounts: []pfmconfig.CodexAccount{{ID: 1, Home: accountHome}},
			Ask: pfmconfig.AskConfig{
				Engine: pfmengine.Codex,
				Prefs:  map[pfmengine.ID]pfmconfig.EnginePrefs{pfmengine.Codex: {Model: "fixture", Effort: "low"}},
			},
		},
		Paths: paths.Values{Home: home},
	}

	var stdout, stderr bytes.Buffer
	if code := runHarvest([]string{"ask", "-p", "Answer without hiding missing evidence", good, missing}, &stdout, &stderr, runtime); code != 0 {
		t.Fatalf("harvest ask code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	prepared, err := os.ReadFile(fileCapture)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"load-bearing local evidence",
		`"status": "unavailable"`,
		`"input": "` + missing + `"`,
		`"error": "refusing to read an unresolvable path"`,
	} {
		if !strings.Contains(string(prepared), want) {
			t.Errorf("prepared files omitted %q:\n%s", want, prepared)
		}
	}
	promptBytes, err := os.ReadFile(promptCapture)
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{good, missing} {
		if !strings.Contains(string(promptBytes), "source: "+source) {
			t.Errorf("prompt lost source label %q:\n%s", source, promptBytes)
		}
	}
	receiptRoot := filepath.Join(home, ".local", "state", "pfm", "harvest-ask")
	entries, err := os.ReadDir(receiptRoot)
	if err != nil {
		t.Fatalf("read receipt root: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("temporary receipts survived command: %v", entries)
	}
}

func TestHarvestAskValidatesBoundsBeforeEngineOrFetch(t *testing.T) {
	home := t.TempDir()
	installAskLauncherStub(t, home)
	source := filepath.Join(home, "source.txt")
	if err := os.WriteFile(source, []byte("fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fiftyOne := []string{"ask", "-p", "question"}
	for range 51 {
		fiftyOne = append(fiftyOne, source)
	}
	for _, testCase := range []struct {
		name string
		args []string
	}{
		{name: "missing prompt", args: []string{"ask", source}},
		{name: "missing sources", args: []string{"ask", "-p", "question"}},
		{name: "more than fifty sources", args: fiftyOne},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := runHarvest(testCase.args, &stdout, &stderr, commandRuntime{Paths: paths.Values{Home: home}}); code != 2 {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			if !strings.Contains(stderr.String(), "usage: pfm harvest ask") {
				t.Fatalf("validation failure omitted ask usage: %q", stderr.String())
			}
		})
	}
}

func TestHarvestAskAcceptsFiftySourcesAndFlagsAfterPositionals(t *testing.T) {
	home := t.TempDir()
	installAskLauncherStub(t, home)
	source := filepath.Join(home, "source.txt")
	if err := os.WriteFile(source, []byte("boundary evidence\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	accountHome := filepath.Join(home, "codex-home")
	if err := os.MkdirAll(accountHome, 0o700); err != nil {
		t.Fatal(err)
	}
	promptCapture := filepath.Join(home, "prompt.txt")
	binary := filepath.Join(home, "codex-fixture")
	script := "#!/bin/sh\ncat > \"$PFM_ASK_PROMPT\"\nprintf 'boundary answer\\n'\n"
	if err := os.WriteFile(binary, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PFM_ASK_PROMPT", promptCapture)
	t.Setenv("WEBFETCH_DIR", filepath.Join(home, "cache"))
	runtime := commandRuntime{
		Config: pfmconfig.Config{
			Codex:         pfmconfig.Codex{Binary: binary},
			CodexAccounts: []pfmconfig.CodexAccount{{ID: 1, Home: accountHome}},
			Ask:           pfmconfig.AskConfig{Engine: pfmengine.Codex},
		},
		Paths: paths.Values{Home: home},
	}
	args := []string{"--ask"}
	for range 50 {
		args = append(args, source)
	}
	args = append(args, "--engine", "codex", "-p", "Exercise the upper bound")
	var stdout, stderr bytes.Buffer
	if code := runHarvest(args, &stdout, &stderr, runtime); code != 0 {
		t.Fatalf("harvest ask code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	prompt, err := os.ReadFile(promptCapture)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(prompt), "\n50. ") || !strings.Contains(string(prompt), " — source: "+source+"\nTASK:") {
		t.Fatalf("upper-bound source missing from prompt:\n%s", prompt)
	}
}

func TestHarvestAskCleansFailureReceiptsWhenEngineFails(t *testing.T) {
	home := t.TempDir()
	installAskLauncherStub(t, home)
	accountHome := filepath.Join(home, "codex-home")
	if err := os.MkdirAll(accountHome, 0o700); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(home, "codex-fixture")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\ncat >/dev/null\nprintf 'fixture failure\\n' >&2\nexit 7\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WEBFETCH_DIR", filepath.Join(home, "cache"))
	runtime := commandRuntime{
		Config: pfmconfig.Config{
			Codex:         pfmconfig.Codex{Binary: binary},
			CodexAccounts: []pfmconfig.CodexAccount{{ID: 1, Home: accountHome}},
			Ask:           pfmconfig.AskConfig{Engine: pfmengine.Codex},
		},
		Paths: paths.Values{Home: home},
	}
	missing := filepath.Join(home, "missing.txt")
	var stdout, stderr bytes.Buffer
	if code := runHarvest([]string{"ask", "-p", "Fail after preparation", missing}, &stdout, &stderr, runtime); code != 1 {
		t.Fatalf("harvest ask code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "codex ask failed") || !strings.Contains(stderr.String(), "fixture failure") {
		t.Fatalf("engine failure lost context: %q", stderr.String())
	}
	receiptRoot := filepath.Join(home, ".local", "state", "pfm", "harvest-ask")
	entries, err := os.ReadDir(receiptRoot)
	if err != nil {
		t.Fatalf("read receipt root: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("temporary receipts survived failed engine: %v", entries)
	}
}

func TestPlainHarvestJSONRemainsBackwardCompatible(t *testing.T) {
	home := t.TempDir()
	installAskLauncherStub(t, home)
	source := filepath.Join(home, "plain.txt")
	if err := os.WriteFile(source, []byte("plain harvest remains plain\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WEBFETCH_DIR", filepath.Join(home, "cache"))
	var stdout, stderr bytes.Buffer
	if code := runHarvest([]string{source, "--json"}, &stdout, &stderr, commandRuntime{Paths: paths.Values{Home: home}}); code != 0 {
		t.Fatalf("plain harvest code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	for _, want := range []string{`"source": "` + source + `"`, "\"content\": \"plain harvest remains plain\\n\"", `"method": "local"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("plain JSON omitted %q:\n%s", want, stdout.String())
		}
	}
}

func installAskLauncherStub(t *testing.T, directory string) {
	t.Helper()
	script := "#!/bin/sh\n[ \"$1\" = internal ] && [ \"$2\" = codex-launch ] || exit 90\nshift 2\nexec \"$@\"\n"
	if err := os.WriteFile(filepath.Join(directory, "pfm"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
}
