package main

import (
	"strings"
	"testing"

	pfmconfig "hostops/pfm/internal/config"
)

func TestResolveRunEngineAccountUsesTheChosenRoster(t *testing.T) {
	machine := pfmconfig.Config{
		Version:       pfmconfig.Version,
		CodexAccounts: []pfmconfig.CodexAccount{{ID: 3, Home: "/codex/3"}, {ID: 8, Home: "/codex/8"}},
		Ask:           pfmconfig.AskConfig{Engine: "codex"},
	}

	engine, account, err := resolveRunEngineAccount("", 0, machine, 0)
	if err != nil || engine != "cx" || account != 3 {
		t.Fatalf("default = %q/%d error=%v, want cx/3", engine, account, err)
	}
	engine, account, err = resolveRunEngineAccount("codex", 8, machine, 0)
	if err != nil || engine != "cx" || account != 8 {
		t.Fatalf("explicit = %q/%d error=%v, want cx/8", engine, account, err)
	}
	_, _, err = resolveRunEngineAccount("claude", 1, machine, 0)
	if err == nil || !strings.Contains(err.Error(), "Claude account 1") {
		t.Fatalf("empty Claude roster error = %v", err)
	}
}
