package action

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"hostops/cc-fleet/internal/compose"
)

func TestQuoteRoundTripsHostileWords(t *testing.T) {
	values := []string{
		"",
		"plain",
		"space dir",
		"single'quote",
		"$HOME $(touch nope) `touch nope2`; newline\nnext",
		"tail\n",
	}
	for _, value := range values {
		outputPath := filepath.Join(t.TempDir(), "word")
		script := "set -- " + Quote(value) +
			"; printf %s \"$1\" > " + Quote(outputPath)
		command := exec.Command("sh", "-c", script)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("sh round trip %q: %v: %s", value, err, output)
		}
		content, err := os.ReadFile(outputPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(content) != value {
			t.Fatalf("Quote(%q) round trip = %q", value, content)
		}
	}
}

func TestSynthesizeRoutesAndEnvHygiene(t *testing.T) {
	id := "11111111-1111-4111-8111-111111111111"
	request := Request{
		Row: compose.Row{
			Kind: compose.ResumeClaude,
			ID:   id,
			CWD:  "/work/a project's $(dir)",
		},
		PrimaryAccount: 2,
		Cache1H:        true,
		Bunker:         true,
		Home:           "/home/test",
		FreshSocket:    "cc-1700000000-123-456",
	}
	plan, err := Synthesize(request)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Route != ResumeClaude ||
		!strings.HasPrefix(plan.Line, "TMUX= exec tmux ") ||
		!strings.Contains(plan.Line, Quote(request.Row.CWD)) {
		t.Fatalf("resume plan = %#v", plan)
	}
	wantPrefix := hygiene +
		" CLAUDE_CONFIG_DIR='/home/test/.cc/2'" +
		" ENABLE_PROMPT_CACHING_1H=1 claude"
	if !strings.HasPrefix(plan.Run, wantPrefix) {
		t.Fatalf("resume run = %q, want prefix %q", plan.Run, wantPrefix)
	}
	// A resumed chat keeps full autonomy, on every account (cc-fleet.zsh:872).
	if !strings.Contains(
		plan.Run,
		"claude '--resume' "+Quote(id)+" "+autonomyFlags,
	) {
		t.Fatalf("resume run missed the autonomy flags: %q", plan.Run)
	}
	for _, name := range []string{
		"CLAUDE_CODE_SESSION_ID",
		"CLAUDECODE",
		"CLAUDE_CONFIG_DIR",
		"ENABLE_PROMPT_CACHING_1H",
		"FORCE_PROMPT_CACHING_5M",
		// CC_ENDPOINT_UNSET (cc-fleet.zsh:80-84) — a chat born inside another
		// chat must never inherit a translating proxy's endpoint.
		"ANTHROPIC_BASE_URL",
		"ANTHROPIC_AUTH_TOKEN",
		"ANTHROPIC_MODEL",
		"ANTHROPIC_SMALL_FAST_MODEL",
		"CLAUDE_CODE_AUTO_COMPACT_WINDOW",
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC",
		"CLAUDE_CODE_DISABLE_NONSTREAMING_FALLBACK",
		"CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY",
	} {
		if !strings.Contains(plan.Run, "-u "+name) {
			t.Fatalf("resume run missed env scrub %s: %q", name, plan.Run)
		}
	}

	request.Row = compose.Row{
		Kind: compose.NewClaude,
		CWD:  "/rotated/project",
	}
	request.Cache1H = false
	request.Bunker = false
	plan, err = Synthesize(request)
	if err != nil {
		t.Fatal(err)
	}
	// A fresh launch calls the shell launcher, whose _cc_run prepends the
	// autonomy flags — repeating them here would duplicate them in argv
	// (cc-fleet.zsh:138).
	if plan.Line != "(cd -- '/rotated/project' && CC_ARM_1H=0 ENABLE_PROMPT_CACHING_1H=0 cc2)" {
		t.Fatalf("new Claude line = %q", plan.Line)
	}
	if strings.Contains(plan.Line, "skip-permissions") {
		t.Fatalf("new Claude line duplicated the autonomy flags: %q", plan.Line)
	}
}

func TestSynthesizeRejectsAccountsOffTheRoster(t *testing.T) {
	// The roster is cc-db.sh's: primary-set refuses anything but 1 or 2, so an
	// account the fleet cannot launch must never reach a command line.
	for _, account := range []int{0, MaxAccount + 1, 9} {
		_, err := Synthesize(Request{
			Row: compose.Row{
				Kind: compose.NewClaude,
				CWD:  "/work/project",
			},
			PrimaryAccount: account,
			Home:           "/home/test",
		})
		if err == nil || !strings.Contains(err.Error(), "primary account") {
			t.Fatalf("account %d error = %v, want a roster rejection", account, err)
		}
	}
	for account := 1; account <= MaxAccount; account++ {
		if _, err := Synthesize(Request{
			Row: compose.Row{
				Kind: compose.NewClaude,
				CWD:  "/work/project",
			},
			PrimaryAccount: account,
			Home:           "/home/test",
		}); err != nil {
			t.Fatalf("account %d rejected: %v", account, err)
		}
	}
}

func TestScriptsResolveToTheInstalledBundle(t *testing.T) {
	// install.sh symlinks the bundle's scripts into ~/.claude/bin; the repo copy
	// under work/host-ops/ is a pre-move path that no longer exists on the host.
	plan, err := Synthesize(Request{
		Row: compose.Row{
			Kind: compose.Agent,
			ID:   "33333333-3333-4333-8333-333333333333",
			CWD:  "/work/project",
		},
		PrimaryAccount: 1,
		Home:           "/home/test",
		FreshSocket:    "cc-1700000001-123-456",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plan.Run, Quote("/home/test/.claude/bin/cc-agent-open.sh")) {
		t.Fatalf("agent run = %q", plan.Run)
	}
	if strings.Contains(plan.Run, "host-ops") {
		t.Fatalf("agent run still points at the pre-move tree: %q", plan.Run)
	}
}

func TestCodexLiveUsesOnlyVerifiedWindow(t *testing.T) {
	row := compose.Row{
		Kind:        compose.LiveCodex,
		Socket:      "cx-1700000000-123-456",
		SessionName: "cx-session",
		Name:        strings.Repeat("界", 30),
	}
	plan, err := Synthesize(Request{
		Row:            row,
		PrimaryAccount: 1,
		Home:           "/home/test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plan.Line, Quote("cx-session")) ||
		strings.Contains(plan.Line, "cx-session:") {
		t.Fatalf("unverified Codex attach line = %q", plan.Line)
	}
	row.WindowName = "already-converged"
	plan, err = Synthesize(Request{
		Row:            row,
		PrimaryAccount: 1,
		Home:           "/home/test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plan.Line, Quote("cx-session:already-converged")) {
		t.Fatalf("Codex converged attach line = %q", plan.Line)
	}
}

func TestAgentFailureNetFallsBackToSanitizedResume(t *testing.T) {
	root := t.TempDir()
	agentScript := filepath.Join(root, "agent-open")
	claudeScript := filepath.Join(root, "claude")
	resultPath := filepath.Join(root, "result")
	writeActionFile(t, agentScript, "#!/bin/sh\nexit 1\n", 0o700)
	writeActionFile(t, claudeScript, `#!/bin/sh
{
  printf 'argv=%s\n' "$*"
  printf 'sid=%s\n' "${CLAUDE_CODE_SESSION_ID-unset}"
  printf 'code=%s\n' "${CLAUDECODE-unset}"
  printf 'cfg=%s\n' "${CLAUDE_CONFIG_DIR-unset}"
  printf 'enable=%s\n' "${ENABLE_PROMPT_CACHING_1H-unset}"
  printf 'force=%s\n' "${FORCE_PROMPT_CACHING_5M-unset}"
  printf 'base=%s\n' "${ANTHROPIC_BASE_URL-unset}"
  printf 'token=%s\n' "${ANTHROPIC_AUTH_TOKEN-unset}"
  printf 'gateway=%s\n' "${CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY-unset}"
} > "$ACTION_RESULT"
`, 0o700)
	id := "22222222-2222-4222-8222-222222222222"
	plan, err := Synthesize(Request{
		Row: compose.Row{
			Kind:      compose.Agent,
			ID:        id,
			CWD:       "/work/agent",
			ConfigDir: "/home/test/.cc/2",
		},
		PrimaryAccount: 2,
		Cache1H:        true,
		Home:           "/home/test",
		FreshSocket:    "cc-1700000001-123-456",
		AgentScript:    agentScript,
	})
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("sh", "-c", plan.Run)
	command.Env = append(
		os.Environ(),
		"PATH="+root+":"+os.Getenv("PATH"),
		"ACTION_RESULT="+resultPath,
		"CLAUDE_CODE_SESSION_ID=poison",
		"CLAUDECODE=poison",
		"CLAUDE_CONFIG_DIR=/poison",
		"ENABLE_PROMPT_CACHING_1H=poison",
		"FORCE_PROMPT_CACHING_5M=poison",
		"ANTHROPIC_BASE_URL=http://127.0.0.1:9/proxy",
		"ANTHROPIC_AUTH_TOKEN=poison",
		"CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY=1",
	)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("run agent failure net: %v: %s", err, output)
	}
	content, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{
		"argv=--resume " + id + " " + autonomyFlags,
		"sid=unset",
		"code=unset",
		"cfg=/home/test/.cc/2",
		"enable=1",
		"force=unset",
		"base=unset",
		"token=unset",
		"gateway=unset",
		"",
	}, "\n")
	if string(content) != want {
		t.Fatalf("fallback result = %q, want %q", content, want)
	}
}

func TestSynthesizeRejectsNUL(t *testing.T) {
	_, err := Synthesize(Request{
		Row: compose.Row{
			Kind: compose.NewClaude,
			CWD:  "/work/a\x00b",
		},
		PrimaryAccount: 1,
	})
	if err == nil || !strings.Contains(err.Error(), "NUL") {
		t.Fatalf("NUL error = %v", err)
	}
}

func TestReaderGateUsesInjectedTerminalChannel(t *testing.T) {
	var output bytes.Buffer
	reboot, err := (ReaderGate{
		Reader: strings.NewReader("S"),
		Writer: &output,
	}).Confirm(context.Background(), GateRequest{
		Name:           "Builder",
		BirthAccount:   2,
		PrimaryAccount: 1,
		BirthCache1H:   false,
		WantCache1H:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reboot ||
		!strings.Contains(output.String(), "account  2 → 1") ||
		!strings.Contains(output.String(), "cache") {
		t.Fatalf("gate reboot=%v output=%q", reboot, output.String())
	}
}

func writeActionFile(
	t *testing.T,
	path, content string,
	mode os.FileMode,
) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}
