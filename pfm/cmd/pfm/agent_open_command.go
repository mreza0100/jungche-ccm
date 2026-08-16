package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"hostops/pfm/internal/agentopen"
	"hostops/pfm/internal/paths"
	"hostops/pfm/internal/shared"
)

// runInternalAgentOpen is deliberately absent from operator help. It is the
// argv target embedded by the picker for a tmux window, not a user command.
func runInternalAgentOpen(args []string, stderr io.Writer) int {
	flags := newFlagSet("internal agent-open", "usage: pfm internal agent-open --id id --cwd path [--config path]", stderr)
	id := flags.String("id", "", "session id")
	cwd := flags.String("cwd", "", "project directory")
	config := flags.String("config", "", "owning Claude config directory")
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 || *id == "" || *cwd == "" {
		flags.Usage()
		return 2
	}
	resolved, err := paths.Resolve()
	if err != nil {
		fmt.Fprintf(stderr, "pfm internal agent-open: %v\n", err)
		return 1
	}
	primary := 1
	if account, found := shared.PrimaryAccount(context.Background(), resolved); found && account >= 1 && account <= 2 {
		primary = account
	}
	opener := agentopen.New(agentopen.Dependencies{
		SIDDir:    resolved.SIDDir,
		Home:      resolved.Home,
		Commands:  agentopen.ExecCommands{Home: resolved.Home, Stdout: os.Stdout, Stderr: stderr},
		Processes: agentopen.RealProcesses{Root: resolved.ProcRoot},
		Tmux:      agentopen.RealTmux{Dir: resolved.TmuxDir, Stderr: stderr},
		Stderr:    stderr,
	})
	if err := opener.Open(context.Background(), agentopen.Request{ID: *id, CWD: *cwd, OwningConfig: *config, PrimaryAccount: primary, Cache1H: initialCache1H()}); err != nil {
		fmt.Fprintf(stderr, "pfm internal agent-open: %v\n", err)
		return 1
	}
	return 0
}
