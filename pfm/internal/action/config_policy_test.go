package action

import (
	"path/filepath"
	"strings"
	"testing"

	"hostops/pfm/internal/compose"
	pfmconfig "hostops/pfm/internal/config"
)

func configuredMachinePolicy(home string) pfmconfig.Config {
	configDir := filepath.Join(home, "profiles", "account 42")
	return pfmconfig.Config{
		Version: pfmconfig.Version,
		Accounts: []pfmconfig.Account{{
			ID:         42,
			ConfigDir:  configDir,
			ProjectDir: filepath.Join(configDir, "projects"),
		}},
		Claude: pfmconfig.Claude{
			PermissionMode: pfmconfig.PermissionPrompt,
			Binary:         "/opt/tools/claude enterprise",
		},
		Codex: pfmconfig.Codex{
			Yolo:   false,
			Binary: "/opt/tools/codex safe",
		},
		Sources: map[string]pfmconfig.Source{
			"accounts":              pfmconfig.SourceFile,
			"claude.permissionMode": pfmconfig.SourceFile,
			"claude.binary":         pfmconfig.SourceFile,
			"codex.yolo":            pfmconfig.SourceFile,
			"codex.binary":          pfmconfig.SourceFile,
		},
	}
}

func TestSynthesizeUsesConfiguredClaudeAccountAndPromptPolicy(t *testing.T) {
	home := t.TempDir()
	machine := configuredMachinePolicy(home)
	plan, err := Synthesize(Request{
		Row: compose.Row{
			Kind: compose.ResumeClaude,
			ID:   "11111111-1111-4111-8111-111111111111",
			CWD:  "/work/project",
		},
		PrimaryAccount: 42,
		Cache1H:        true,
		Home:           home,
		FreshSocket:    "cc-configured-42",
		Config:         machine,
	})
	if err != nil {
		t.Fatalf("Synthesize() error = %v", err)
	}
	for _, want := range []string{
		"CLAUDE_CONFIG_DIR=" + Quote(machine.Accounts[0].ConfigDir),
		"ENABLE_PROMPT_CACHING_1H=1",
		Quote(machine.Claude.Binary),
		Quote("--resume") + " " + Quote("11111111-1111-4111-8111-111111111111"),
	} {
		if !strings.Contains(plan.Run, want) {
			t.Fatalf("resume run %q lacks configured policy %q", plan.Run, want)
		}
	}
	if strings.Contains(plan.Run, "skip-permissions") {
		t.Fatalf("prompt permission policy still armed bypass flags: %q", plan.Run)
	}
}

func TestHeadlessRunUsesConfiguredClaudeAndCodexPolicy(t *testing.T) {
	home := t.TempDir()
	machine := configuredMachinePolicy(home)
	claude, err := HeadlessRun(HeadlessRequest{
		Engine:         "claude",
		Name:           "configured worker",
		CWD:            "/work/project",
		Prompt:         "inspect the configured account",
		Home:           home,
		PrimaryAccount: 42,
		Config:         machine,
	})
	if err != nil {
		t.Fatalf("HeadlessRun(Claude) error = %v", err)
	}
	if !claude.PromptOnCommandLine || !strings.Contains(claude.Run, Quote(machine.Claude.Binary)) {
		t.Fatalf("headless Claude plan = %#v", claude)
	}
	if strings.Contains(claude.Run, "skip-permissions") {
		t.Fatalf("headless Claude prompt policy still armed bypass flags: %q", claude.Run)
	}

	codex, err := HeadlessRun(HeadlessRequest{
		Engine: "codex",
		Name:   "configured codex",
		CWD:    "/work/project",
		Home:   home,
		Config: machine,
	})
	if err != nil {
		t.Fatalf("HeadlessRun(Codex) error = %v", err)
	}
	if !strings.Contains(codex.Run, Quote(machine.Codex.Binary)+" --sandbox workspace-write") {
		t.Fatalf("headless Codex plan = %q", codex.Run)
	}
	if strings.Contains(codex.Run, "dangerously-bypass-approvals-and-sandbox") {
		t.Fatalf("headless Codex yolo=false still armed bypass: %q", codex.Run)
	}
}

func TestSynthesizePickerUsesConfiguredCodexBinaryAndSandbox(t *testing.T) {
	home := t.TempDir()
	machine := configuredMachinePolicy(home)
	plan, err := Synthesize(Request{
		Row: compose.Row{
			Kind: compose.NewCodex,
			CWD:  "/work/project",
		},
		PrimaryAccount: 42,
		Home:           home,
		Config:         machine,
	})
	if err != nil {
		t.Fatalf("Synthesize() error = %v", err)
	}
	want := "(cd -- '/work/project' && " + hygiene + " " + Quote(machine.Codex.Binary) + " --sandbox workspace-write)"
	if plan.Line != want {
		t.Fatalf("new Codex picker line = %q, want %q", plan.Line, want)
	}
}
