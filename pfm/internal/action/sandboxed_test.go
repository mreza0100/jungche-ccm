package action

import (
	"strings"
	"testing"
)

func TestSandboxedCodexRunNeverUsesTheBypassBuilder(t *testing.T) {
	plan, err := SandboxedCodexRun(SandboxedCodexRequest{
		CWD: "/organ/tmp/staging/explorer-night",
		Config: []string{
			`model="gpt-5.6-luna"`,
			`model_reasoning_effort="xhigh"`,
			`approval_policy="never"`,
			`sandbox_mode="workspace-write"`,
			`features.apps=false`,
			`features.plugins=false`,
			`mcp_servers.example.enabled=false`,
		},
	})
	if err != nil {
		t.Fatalf("SandboxedCodexRun() error = %v", err)
	}
	if plan.PromptOnCommandLine {
		t.Fatal("a sandboxed Codex TUI must be prompted through the pane")
	}
	for _, forbidden := range []string{
		"--dangerously-bypass-approvals-and-sandbox",
		"--ignore-user-config",
		"--ephemeral",
		"CODEX_HOME=",
	} {
		if strings.Contains(plan.Run, forbidden) {
			t.Fatalf("sandboxed command contains %q: %s", forbidden, plan.Run)
		}
	}
	for _, required := range []string{
		"-u CODEX_THREAD_ID",
		"codex '--strict-config' '--sandbox' 'workspace-write'",
		"'--cd' '/organ/tmp/staging/explorer-night'",
		"'-c' 'model=\"gpt-5.6-luna\"'",
		"'-c' 'approval_policy=\"never\"'",
		"'-c' 'sandbox_mode=\"workspace-write\"'",
		"'-c' 'mcp_servers.example.enabled=false'",
	} {
		if !strings.Contains(plan.Run, required) {
			t.Fatalf("sandboxed command lacks %q: %s", required, plan.Run)
		}
	}
	// The pane root must BE the Codex launcher for the seat's process-tree
	// gate. dash (/bin/sh — cron's SHELL, hence tmux's default-shell there)
	// does not exec a sole trailing command the way zsh/bash do, so the plan
	// carries the exec itself; without this prefix every cron-launched seat
	// dies at the gate with pane root shape "sh".
	if !strings.HasPrefix(plan.Run, "exec env ") {
		t.Fatalf("sandboxed command must exec into the launcher, got: %s", plan.Run)
	}
}

func TestSandboxedCodexRunRefusesIncompleteValues(t *testing.T) {
	for _, test := range []struct {
		name    string
		request SandboxedCodexRequest
	}{
		{"no directory", SandboxedCodexRequest{Config: []string{"a=true"}}},
		{"nul directory", SandboxedCodexRequest{CWD: "/stage\x00bad", Config: []string{"a=true"}}},
		{"empty override", SandboxedCodexRequest{CWD: "/stage", Config: []string{""}}},
		{"not key value", SandboxedCodexRequest{CWD: "/stage", Config: []string{"features.apps"}}},
		{"nul override", SandboxedCodexRequest{CWD: "/stage", Config: []string{"a=\x00"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := SandboxedCodexRun(test.request); err == nil {
				t.Fatal("SandboxedCodexRun() accepted an incomplete request")
			}
		})
	}
}
