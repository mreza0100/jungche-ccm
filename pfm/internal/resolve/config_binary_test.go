package resolve

import (
	"strings"
	"testing"
)

func TestResolverUsesConfiguredClaudeBasenameForSplitSessions(t *testing.T) {
	customClaude := "/opt/tools/claude enterprise"
	resolver, err := New(fakeTmux{}, Binaries{Claude: customClaude})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	outcome := resolver.resolveSession("split", []Pane{
		{
			SocketPath:     "/jail/cc-configured",
			SessionName:    "split",
			PaneID:         "%1",
			CurrentCommand: customClaude,
		},
		{
			SocketPath:     "/jail/cc-configured",
			SessionName:    "split",
			PaneID:         "%2",
			CurrentCommand: "bash",
		},
	})
	if outcome.Code != 0 || outcome.Stdout != "/jail/cc-configured\t%1\n" {
		t.Fatalf("configured split resolution = %+v", outcome)
	}
}

func TestResolverUsesConfiguredCodexBasenameForWindowNames(t *testing.T) {
	customCodex := "/opt/tools/codex safe"
	resolver, err := New(fakeTmux{}, Binaries{Codex: customCodex})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	outcome := resolver.resolveCxWindow("configured thread", []Pane{{
		SocketPath:     "/jail/custom-engine",
		PaneID:         "%9",
		CurrentCommand: customCodex,
		WindowName:     "configured thread",
	}})
	if outcome.Code != 0 || outcome.Stdout != "/jail/custom-engine\t%9\n" {
		t.Fatalf("configured Codex window resolution = %+v", outcome)
	}
}

func TestEngineCommandBasenamesAcceptConfiguredPaths(t *testing.T) {
	customClaude := "/opt/tools/claude enterprise"
	customCodex := "/opt/tools/codex safe"
	for _, test := range []struct {
		name    string
		command string
		check   func(string, ...string) bool
		binary  string
	}{
		{name: "claude", command: customClaude, check: isClaudeCommand, binary: customClaude},
		{name: "codex", command: customCodex, check: isCodexCommand, binary: customCodex},
	} {
		t.Run(test.name, func(t *testing.T) {
			if !test.check(test.command, test.binary) {
				t.Fatalf("configured command %q was not recognized", test.command)
			}
		})
	}
	if isCodexCommand(customCodex) {
		t.Fatal("custom Codex basename was accepted without its configured policy")
	}
}

func TestResolverStillReturnsNoMatchForUnknownConfiguredCommand(t *testing.T) {
	resolver := &Resolver{tmux: fakeTmux{}}
	outcome := resolver.resolveSession("split", []Pane{
		{SocketPath: "/jail/cc", SessionName: "split", PaneID: "%1", CurrentCommand: "custom-claude"},
		{SocketPath: "/jail/cc", SessionName: "split", PaneID: "%2", CurrentCommand: "bash"},
	})
	if outcome.Code != 2 || !strings.Contains(outcome.Stderr, "0 running claude") {
		t.Fatalf("unknown command classification = %+v", outcome)
	}
}
