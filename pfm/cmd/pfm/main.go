// Command pfm manages live and resumable Claude and Codex chats.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"

	"hostops/pfm/internal/hide"
	"hostops/pfm/internal/mcpserv"
	"hostops/pfm/internal/paths"
	"hostops/pfm/internal/store"
)

var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return runLS(nil, stdout, stderr)
	}

	switch args[0] {
	case "version", "--version":
		return runVersion(args[1:], stdout, stderr)
	case "ls":
		return runLS(args[1:], stdout, stderr)
	case "chat":
		return runChat(args[1:], os.Stdin, stdout, stderr)
	case "headless":
		fmt.Fprintln(stderr, "pfm: 'headless' is deprecated; use 'pfm chat'")
		return runHeadless(args[1:], stdout, stderr)
	case "index":
		return runIndex(args[1:], stdout, stderr)
	case "doctor":
		return runDoctor(args[1:], stdout, stderr)
	case "dream":
		return runDream(args[1:], os.Stdin, stdout, stderr)
	case "revive":
		return runRevive(args[1:], stdout, stderr)
	case "reap":
		return runReap(args[1:], stdout, stderr)
	case "archive":
		return runArchive(args[1:], stdout, stderr)
	case "heal":
		return runHeal(args[1:], stdout, stderr)
	case "name-sync":
		return runNameSync(args[1:], stdout, stderr)
	case "statusline":
		return runStatusline(args[1:], os.Stdin, stdout, stderr)
	case "usage-hook":
		return runUsageHook(args[1:], stdout, stderr)
	case "install":
		return runInstall(args[1:], stdout, stderr)
	case "whoami":
		return runWhoami(args[1:], stdout, stderr)
	case "mcp":
		return runMCP(args[1:], stderr)
	case "internal":
		return runInternal(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "pfm: unknown command %q\n", args[0])
		printUsage(stderr)
		return 2
	}
}

func runMCP(args []string, stderr io.Writer) int {
	flags := newFlagSet("mcp", "usage: pfm mcp", stderr)
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 {
		flags.Usage()
		return 2
	}
	service, err := mcpserv.New(version, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "pfm mcp: %v\n", err)
		return 1
	}
	defer service.Close()
	if err := service.RunStdio(
		context.Background(),
		os.Stdin,
		os.Stdout,
	); err != nil {
		fmt.Fprintf(stderr, "pfm mcp: %v\n", err)
		return 1
	}
	return 0
}

func runVersion(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("version", "usage: pfm version", stderr)
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 {
		flags.Usage()
		return 2
	}
	fmt.Fprintf(stdout, "pfm %s\n", version)
	return 0
}

func runHide(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet(
		"chat hide",
		"usage: pfm chat hide [self | id] [--exit]",
		stderr,
	)
	self := flags.Bool("self", false, "hide the calling tmux chat")
	exit := flags.Bool("exit", false, "gracefully close after hiding")
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() > 1 || (*self && flags.NArg() != 0) ||
		(!*self && flags.NArg() != 1) {
		flags.Usage()
		return 2
	}

	database, manager, code := openHideManager(stderr)
	if code != 0 {
		return code
	}
	defer database.Close()
	ctx := context.Background()
	id := ""
	engine := ""
	rolloutPath := ""
	if flags.NArg() == 1 {
		id = flags.Arg(0)
		// The index may not have caught up with the row the picker already
		// shows — a fresh agent's transcript, a live Codex pane holding no
		// rollout file yet. A compose pass is the picker's own source of
		// truth for what exists right now, so resolving against it vouches
		// for exactly the ids the picker would let you ⌃X, and nothing else.
		engine, rolloutPath = resolveRowEngine(ctx, database, id, stderr)
	}
	target, err := manager.Hide(ctx, hide.Request{
		ID:          id,
		Engine:      engine,
		RolloutPath: rolloutPath,
		Self:        *self,
		Exit:        *exit,
		Environment: hide.Environment(),
	})
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat hide: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "hidden %s\n", target.ID)
	return 0
}

func runUnhide(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("chat unhide", "usage: pfm chat unhide id", stderr)
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 1 {
		flags.Usage()
		return 2
	}
	database, manager, code := openHideManager(stderr)
	if code != 0 {
		return code
	}
	defer database.Close()
	if err := manager.Unhide(context.Background(), flags.Arg(0)); err != nil {
		fmt.Fprintf(stderr, "pfm chat unhide: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "unhidden %s\n", flags.Arg(0))
	return 0
}

func runHidden(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet(
		"ls --hidden",
		"usage: pfm ls --hidden",
		stderr,
	)
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 {
		flags.Usage()
		return 2
	}
	database, manager, code := openHideManager(stderr)
	if code != 0 {
		return code
	}
	defer database.Close()
	rows, err := manager.Hidden(context.Background())
	if err != nil {
		fmt.Fprintf(stderr, "pfm ls --hidden: %v\n", err)
		return 1
	}
	for _, row := range rows {
		fmt.Fprintf(stdout, "%s\t%s\t%d\n", row.ID, row.Engine, row.HiddenAt)
	}
	return 0
}

func runInternal(args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 && args[0] == "clear-hide" {
		return runClearHide(args[1:], os.Stdin, stderr)
	}
	if len(args) != 0 && args[0] == "agent-open" {
		return runInternalAgentOpen(args[1:], stderr)
	}
	if len(args) != 0 && args[0] == "swap-run" {
		return runChatSwapWorker(args[1:], os.Stdout, stderr)
	}
	if len(args) != 0 && args[0] == "then" {
		return runInternalThen(args[1:], stderr)
	}
	if len(args) != 0 && args[0] == "primary-get" {
		resolved, err := paths.Resolve()
		if err != nil {
			fmt.Fprintf(stderr, "pfm internal primary-get: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, readPrimaryAccount(resolved))
		return 0
	}
	if len(args) != 0 && args[0] == "primary-set" {
		flags := newFlagSet(
			"internal primary-set",
			"usage: pfm internal primary-set <1|2>",
			stderr,
		)
		if code, ok := parseFlags(flags, args[1:]); !ok {
			return code
		}
		if flags.NArg() != 1 {
			flags.Usage()
			return 2
		}
		account, err := strconv.Atoi(flags.Arg(0))
		if err != nil {
			flags.Usage()
			return 2
		}
		resolved, err := paths.Resolve()
		if err != nil {
			fmt.Fprintf(stderr, "pfm internal primary-set: %v\n", err)
			return 1
		}
		if err := writePrimaryAccount(resolved, account); err != nil {
			fmt.Fprintf(stderr, "pfm internal primary-set: %v\n", err)
			return 1
		}
		return 0
	}
	if len(args) == 0 || args[0] != "hide-exit" {
		fmt.Fprintln(stderr, "usage: pfm internal clear-hide|hide-exit|then [options]")
		return 2
	}
	flags := newFlagSet(
		"internal hide-exit",
		"usage: pfm internal hide-exit --engine cc|cx --id id --path path --socket path --socket-name name --pane %id",
		stderr,
	)
	engine := flags.String("engine", "", "engine name")
	id := flags.String("id", "", "chat id")
	dataPath := flags.String("path", "", "transcript or rollout path")
	socket := flags.String("socket", "", "tmux socket path")
	socketName := flags.String("socket-name", "", "tmux socket basename")
	pane := flags.String("pane", "", "tmux pane id")
	if code, ok := parseFlags(flags, args[1:]); !ok {
		return code
	}
	if flags.NArg() != 0 || *engine == "" || *id == "" ||
		*socket == "" || *socketName == "" || *pane == "" {
		flags.Usage()
		return 2
	}
	database, err := store.Open(store.WithWarningWriter(stderr))
	if err != nil {
		fmt.Fprintf(stderr, "pfm internal hide-exit: %v\n", err)
		return 1
	}
	defer database.Close()
	finisher, err := hide.NewFinisher(database, hide.Dependencies{})
	if err == nil {
		err = finisher.Run(context.Background(), hide.ExitArgs{
			Engine:     *engine,
			ID:         *id,
			DataPath:   *dataPath,
			SocketPath: *socket,
			SocketName: *socketName,
			PaneID:     *pane,
		})
	}
	if err != nil {
		fmt.Fprintf(stderr, "pfm internal hide-exit: %v\n", err)
		return 1
	}
	return 0
}

func openHideManager(
	stderr io.Writer,
) (*store.Store, *hide.Manager, int) {
	database, err := store.Open(store.WithWarningWriter(stderr))
	if err != nil {
		fmt.Fprintf(stderr, "pfm: %v\n", err)
		return nil, nil, 1
	}
	manager, err := hide.New(database, hide.Dependencies{})
	if err != nil {
		_ = database.Close()
		fmt.Fprintf(stderr, "pfm: %v\n", err)
		return nil, nil, 1
	}
	return database, manager, 0
}

func newFlagSet(name, usage string, stderr io.Writer) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintln(stderr, usage)
	}
	return flags
}

func parseFlags(flags *flag.FlagSet, args []string) (int, bool) {
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0, false
		}
		return 2, false
	}
	return 0, true
}

// parseFlagsAnywhere accepts flags before OR after the positional arguments,
// because that is how the family is documented and how a human types it:
// `status seat --json` must not read as three positionals. Go's flag package
// stops at the first non-flag token, so the remainder is re-parsed until only
// positionals are left.
func parseFlagsAnywhere(
	flags *flag.FlagSet,
	args []string,
) ([]string, int, bool) {
	positional := make([]string, 0, 2)
	for {
		if code, ok := parseFlags(flags, args); !ok {
			return nil, code, false
		}
		rest := flags.Args()
		if len(rest) == 0 {
			return positional, 0, true
		}
		positional = append(positional, rest[0])
		args = rest[1:]
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: pfm <command> [options]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "operator commands:")
	fmt.Fprintln(w, "  ls        list or pick fleet chats")
	fmt.Fprintln(w, "  chat      operate on one chat: new, open, inject, ask, read, stream, name, hide, end")
	fmt.Fprintln(w, "  dream     build and inject repository memory organs")
	fmt.Fprintln(w, "  index     refresh the transcript index")
	fmt.Fprintln(w, "  whoami    print this chat's own tmux session name")
	fmt.Fprintln(w, "  revive    list resumable chats by project")
	fmt.Fprintln(w, "  reap      classify the socket graveyard; --apply reclaims it")
	fmt.Fprintln(w, "  archive   move hidden chats and old subagent transcripts out of sight, reversibly")
	fmt.Fprintln(w, "  heal      report or repair wedged Codex history projections")
	fmt.Fprintln(w, "  install   wire or remove the self-contained host integration")
	fmt.Fprintln(w, "  doctor    inspect fleet database and jail health")
	fmt.Fprintln(w, "  version   print the pfm version")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "wiring commands:")
	fmt.Fprintln(w, "  name-sync converge live chat window names")
	fmt.Fprintln(w, "  statusline render the native Claude status line")
	fmt.Fprintln(w, "  usage-hook the fail-open usage-limit prompt hook")
	fmt.Fprintln(w, "  mcp       serve chat tools over stdio MCP")
}
