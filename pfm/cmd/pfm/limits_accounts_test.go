package main

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	pfmconfig "hostops/pfm/internal/config"
	"hostops/pfm/internal/paths"
)

func TestLimitAccountsKeepCodexIndependentFromClaudeRoster(t *testing.T) {
	home := t.TempDir()
	t.Setenv(paths.EnvHome, home)
	runtime := commandRuntime{
		Paths: paths.Values{Home: home},
		Config: pfmconfig.Config{
			Claude: pfmconfig.Claude{Binary: "claude"},
			Accounts: []pfmconfig.Account{
				{ID: 1, ConfigDir: pfmconfig.DefaultAccountDir(home, 1), Emoji: "🥇"},
				{ID: 2, ConfigDir: pfmconfig.DefaultAccountDir(home, 2), Emoji: "🥈"},
			},
			AccountSkips: []pfmconfig.AccountSkip{{
				ID: 4, ConfigDir: pfmconfig.DefaultAccountDir(home, 4), Reason: "no valid credentials",
			}},
		},
	}
	accounts := limitAccounts(runtime)
	if len(accounts) != 4 {
		t.Fatalf("limitAccounts()=%#v, want two Claude rows, one skip, and Codex", accounts)
	}
	if accounts[0].ID != 1 || accounts[1].ID != 2 || accounts[2].SkipReason != "no valid credentials" {
		t.Fatalf("Claude accounts/skips=%#v", accounts[:3])
	}
	codex := accounts[3]
	wantCache := filepath.Join(home, "tmp", "cc-gpt-usage-"+strconv.Itoa(os.Getuid())+".json")
	if codex.Engine != "codex" || codex.Label != "Codex" || codex.CodexCachePath != wantCache {
		t.Fatalf("Codex account=%#v, want independent cache row at %q", codex, wantCache)
	}
}
