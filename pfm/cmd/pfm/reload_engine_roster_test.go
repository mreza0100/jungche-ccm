package main

import (
	"strings"
	"testing"

	pfmconfig "hostops/pfm/internal/config"
)

func TestValidateReloadAccountUsesTheSeatEngineRoster(t *testing.T) {
	machine := pfmconfig.Config{
		Version:       pfmconfig.Version,
		CodexAccounts: []pfmconfig.CodexAccount{{ID: 3, Home: "/codex/3"}},
	}
	if _, err := validateReloadAccount(machine, "cx", 3); err != nil {
		t.Fatalf("Codex account rejected: %v", err)
	}
	if _, err := validateReloadAccount(machine, "cc", 1); err == nil ||
		!strings.Contains(err.Error(), "no Claude accounts configured") {
		t.Fatalf("empty Claude roster error = %v", err)
	}
	if _, err := validateReloadAccount(machine, "cx", 4); err == nil ||
		!strings.Contains(err.Error(), "Codex account 4") {
		t.Fatalf("off-roster Codex error = %v", err)
	}
}
