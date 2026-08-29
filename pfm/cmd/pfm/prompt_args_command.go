package main

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"hostops/pfm/internal/action"
	pfmconfig "hostops/pfm/internal/config"
)

// runInternalPromptArgs prints the extra launch words the shim's _cc_run must
// add for the account's claude.systemPrompt choice, one per line:
//
//	env K=V    — an environment assignment for the launch prefix
//	arg TOKEN  — one claude argv element
//
// Production prints nothing. Every failure is a one-line stderr reason with a
// nonzero exit and NO stdout — the shim fail-opens on nonzero, and a partial
// line protocol would inject half a flag pair.
func runInternalPromptArgs(args []string, stdout, stderr io.Writer, runtime commandRuntime) int {
	if len(args) > 1 {
		fmt.Fprintln(stderr, "usage: pfm internal prompt-args [account]")
		return 2
	}
	if runtime.ConfigError != nil {
		fmt.Fprintf(stderr, "pfm internal prompt-args: config unreadable: %v\n", runtime.ConfigError)
		return 1
	}
	account := 0
	if len(args) == 1 {
		parsed, err := strconv.Atoi(args[0])
		if err != nil {
			fmt.Fprintln(stderr, "usage: pfm internal prompt-args [account]")
			return 2
		}
		account = parsed
	} else {
		account = readPrimaryAccount(runtime.Paths, runtime.Config)
	}
	switch prefs := runtime.Config.EffectiveClaude(account); prefs.SystemPrompt {
	case "", pfmconfig.SystemPromptProduction:
		return 0
	case pfmconfig.SystemPromptLean:
		fmt.Fprintln(stdout, "env CLAUDE_CODE_SIMPLE_SYSTEM_PROMPT=1")
		return 0
	case pfmconfig.SystemPromptProfessor:
		path := action.ProfessorPromptPath(runtime.Paths.Home)
		if _, err := os.Stat(path); err != nil {
			fmt.Fprintf(stderr, "pfm internal prompt-args: staged professor prompt unreadable at %s (%v) — run pfm install\n", path, err)
			return 1
		}
		fmt.Fprintln(stdout, "arg --system-prompt-file")
		fmt.Fprintln(stdout, "arg "+path)
		return 0
	default:
		fmt.Fprintf(stderr, "pfm internal prompt-args: unknown systemPrompt %q\n", prefs.SystemPrompt)
		return 1
	}
}
