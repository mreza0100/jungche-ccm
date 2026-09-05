package action

import (
	"strings"
	"testing"

	"hostops/pfm/internal/compose"
	pfmconfig "hostops/pfm/internal/config"
)

func TestNewClaudeUsesNativeConfiguredSpawn(t *testing.T) {
	home := t.TempDir()
	machine := configuredMachinePolicy(home)
	machine.Claude.SystemPrompt = pfmconfig.SystemPromptProfessor
	prompt := "fresh prompt with '$HOME' and $(touch nope)"
	request := Request{
		Row: compose.Row{
			Kind: compose.NewClaude,
			CWD:  "/work/project with spaces",
		},
		PrimaryAccount: 42,
		Cache1H:        false,
		Bunker:         true,
		Home:           home,
		FreshSocket:    "cc-native-42",
		Config:         machine,
		Prompt:         prompt,
	}
	plan, err := Synthesize(request)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"CLAUDE_CONFIG_DIR=" + Quote(machine.Accounts[0].ConfigDir),
		"FORCE_PROMPT_CACHING_5M=1",
		Quote(machine.Claude.Binary),
		Quote(prompt),
		"--system-prompt-file " + Quote(ProfessorPromptPath(home)),
	} {
		if !strings.Contains(plan.Run, want) {
			t.Fatalf("native fresh run %q lacks %q", plan.Run, want)
		}
	}
	if strings.Contains(plan.Run, "skip-permissions") {
		t.Fatalf("prompted account received autonomy bypass: %q", plan.Run)
	}
	if got, want := plan.Line, newSessionLine(request.FreshSocket, request.Row.CWD, plan.Run, true); got != want {
		t.Fatalf("native fresh line = %q, want bunker tmux line %q", got, want)
	}
	for _, retired := range []string{" cc42", "_cc_run", "CC_ARM_1H"} {
		if strings.Contains(plan.Line, retired) || strings.Contains(plan.Run, retired) {
			t.Fatalf("native fresh action retained retired shell surface %q: %#v", retired, plan)
		}
	}
}

func TestNewClaudeNativeSpawnPreservesBypassLeanAndCachePolicy(t *testing.T) {
	home := t.TempDir()
	machine := configuredMachinePolicy(home)
	machine.Claude.PermissionMode = pfmconfig.PermissionBypass
	machine.Claude.SystemPrompt = pfmconfig.SystemPromptLean
	request := Request{
		Row:            compose.Row{Kind: compose.NewClaude, CWD: "/work/project"},
		PrimaryAccount: 42,
		Cache1H:        true,
		Home:           home,
		FreshSocket:    "cc-native-policy",
		Config:         machine,
	}
	plan, err := Synthesize(request)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"ENABLE_PROMPT_CACHING_1H=1",
		"CLAUDE_CODE_SIMPLE_SYSTEM_PROMPT=1",
		autonomyFlags,
	} {
		if !strings.Contains(plan.Run, want) {
			t.Fatalf("native fresh run %q lacks %q", plan.Run, want)
		}
	}
	if strings.Contains(plan.Run, "--system-prompt-file") {
		t.Fatalf("lean prompt mode also received professor prompt file: %q", plan.Run)
	}
	if strings.HasPrefix(plan.Line, "TMUX= exec ") {
		t.Fatalf("non-bunker fresh launch used exec prefix: %q", plan.Line)
	}
}

func TestNewClaudeRequiresFreshSocket(t *testing.T) {
	_, err := Synthesize(Request{
		Row:            compose.Row{Kind: compose.NewClaude, CWD: "/work/project"},
		PrimaryAccount: 1,
		Home:           "/home/test",
		Config:         testMachineConfig("/home/test"),
	})
	if err == nil || !strings.Contains(err.Error(), "fresh socket") {
		t.Fatalf("missing fresh socket error = %v", err)
	}
}
