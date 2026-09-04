package action

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"hostops/pfm/internal/compose"
	pfmconfig "hostops/pfm/internal/config"
	"hostops/pfm/internal/paths"
)

// The Codex spawn path carries the same tmux.titles policy the Claude one
// does. A fleet where one engine seizes the terminal title and the other does
// not is worse than either choice made consistently.

func newCodexTitlesProbeServer(t *testing.T, socket string, titles *pfmconfig.TmuxTitles) string {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	root, err := os.MkdirTemp("/tmp", "pfmcxtitles")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	tmuxDir := filepath.Join(root, "tmux-"+strconv.Itoa(os.Getuid()))
	if err := os.MkdirAll(tmuxDir, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv(paths.EnvTmuxConf, "/dev/null")

	tmux := CommandTmux{TmuxDir: tmuxDir}
	if err := tmux.CreateCodexServer(context.Background(), CodexServer{
		Socket: socket, CWD: root, Run: "sleep 120", Titles: titles,
	}); err != nil {
		t.Fatalf("create Codex server: %v", err)
	}
	t.Cleanup(func() {
		kill := exec.Command("tmux", "-S", filepath.Join(tmuxDir, socket), "kill-server")
		kill.Env = append(os.Environ(), "TMUX=")
		_ = kill.Run()
	})
	return filepath.Join(tmuxDir, socket)
}

func showCodexOption(t *testing.T, socketPath string, arguments ...string) string {
	t.Helper()
	command := exec.Command("tmux", append([]string{"-S", socketPath}, arguments...)...)
	command.Env = append(os.Environ(), "TMUX=")
	output, err := command.Output()
	if err != nil {
		t.Fatalf("tmux %s: %v", strings.Join(arguments, " "), err)
	}
	return strings.TrimSpace(string(output))
}

func TestCodexServerAppliesTheTitleOptionsWhenPfmOwnsThem(t *testing.T) {
	titles := pfmconfig.DefaultTmuxTitles()
	socketPath := newCodexTitlesProbeServer(t, "probe-cx-1800000021-1-1", &titles)
	if got := showCodexOption(t, socketPath, "show-options", "-g", "set-titles"); got != "set-titles on" {
		t.Fatalf("set-titles = %q, want on", got)
	}
	want := `set-titles-string "` + pfmconfig.TmuxTitlesString + `"`
	if got := showCodexOption(t, socketPath, "show-options", "-g", "set-titles-string"); got != want {
		t.Fatalf("set-titles-string = %q, want %q", got, want)
	}
}

func TestCodexServerLeavesTheHostsTitleAloneWhenTitlesAreDisabled(t *testing.T) {
	socketPath := newCodexTitlesProbeServer(
		t, "probe-cx-1800000022-1-1", &pfmconfig.TmuxTitles{Enabled: false},
	)
	if got := showCodexOption(t, socketPath, "show-options", "-g", "set-titles"); got != "set-titles off" {
		t.Fatalf("set-titles = %q, want tmux's own default off", got)
	}
	if got := showCodexOption(t, socketPath, "show-window-options", "-g", "automatic-rename"); got != "automatic-rename off" {
		t.Fatalf("automatic-rename = %q, want off regardless of the title policy", got)
	}
}

// The policy travels with the PLAN, so every caller of Synthesize carries the
// machine config's answer without wiring it a second time.
func TestSynthesizeCarriesTheConfiguredTitlePolicyIntoTheCodexPlan(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		machine := testMachineConfig("/home/test")
		machine.Tmux = pfmconfig.Tmux{Titles: pfmconfig.TmuxTitles{Enabled: enabled}}
		plan, err := Synthesize(Request{
			Row: compose.Row{
				Kind: compose.ResumeCodex,
				ID:   "55555555-5555-4555-8555-555555555555",
				CWD:  "/work/codex",
			},
			PrimaryAccount: 1,
			Home:           "/home/test",
			FreshSocket:    "cx-901-1-1",
			Config:         machine,
		})
		if err != nil {
			t.Fatal(err)
		}
		if plan.CodexServer == nil || plan.CodexServer.Titles == nil {
			t.Fatalf("plan carries no tmux.titles policy: %#v", plan.CodexServer)
		}
		if plan.CodexServer.Titles.Enabled != enabled {
			t.Fatalf("plan title policy = %t, want %t", plan.CodexServer.Titles.Enabled, enabled)
		}
	}
}
