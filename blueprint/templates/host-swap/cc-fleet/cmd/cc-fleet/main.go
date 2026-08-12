// Command cc-fleet manages live and resumable Claude and Codex chats.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"hostops/cc-fleet/internal/hide"
	"hostops/cc-fleet/internal/mcpserv"
	"hostops/cc-fleet/internal/resolve"
	"hostops/cc-fleet/internal/store"
)

var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}

	switch args[0] {
	case "version":
		return runVersion(args[1:], stdout, stderr)
	case "ls":
		return runLS(args[1:], stdout, stderr)
	case "open":
		return runOpen(args[1:], stdout, stderr)
	case "index":
		return runIndex(args[1:], stdout, stderr)
	case "doctor":
		return runDoctor(args[1:], stdout, stderr)
	case "revive":
		return runRevive(args[1:], stdout, stderr)
	case "legacy":
		return runLegacy(args[1:], stdout, stderr)
	case "hide":
		return runHide(args[1:], stdout, stderr)
	case "unhide":
		return runUnhide(args[1:], stdout, stderr)
	case "hidden":
		return runHidden(args[1:], stdout, stderr)
	case "resolve":
		return runResolve(args[1:], stdout, stderr)
	case "whoami":
		return runWhoami(args[1:], stdout, stderr)
	case "mcp":
		return runMCP(args[1:], stderr)
	case "internal":
		return runInternal(args[1:], stderr)
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "cc-fleet: unknown command %q\n", args[0])
		printUsage(stderr)
		return 2
	}
}

func runMCP(args []string, stderr io.Writer) int {
	flags := newFlagSet("mcp", "usage: cc-fleet mcp", stderr)
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 {
		flags.Usage()
		return 2
	}
	service, err := mcpserv.New(version, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "cc-fleet mcp: %v\n", err)
		return 1
	}
	defer service.Close()
	if err := service.RunStdio(
		context.Background(),
		os.Stdin,
		os.Stdout,
	); err != nil {
		fmt.Fprintf(stderr, "cc-fleet mcp: %v\n", err)
		return 1
	}
	return 0
}

func runVersion(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("version", "usage: cc-fleet version", stderr)
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 {
		flags.Usage()
		return 2
	}
	fmt.Fprintf(stdout, "cc-fleet %s\n", version)
	return 0
}

func runHide(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet(
		"hide",
		"usage: cc-fleet hide [--self | id] [--exit]",
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
		fmt.Fprintf(stderr, "cc-fleet hide: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "hidden %s\n", target.ID)
	return 0
}

func runUnhide(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("unhide", "usage: cc-fleet unhide id", stderr)
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
		fmt.Fprintf(stderr, "cc-fleet unhide: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "unhidden %s\n", flags.Arg(0))
	return 0
}

func runHidden(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet(
		"hidden",
		"usage: cc-fleet hidden [--prune-orphans [--yes]]",
		stderr,
	)
	pruneOrphans := flags.Bool(
		"prune-orphans",
		false,
		"report hides whose chat no longer exists",
	)
	confirm := flags.Bool(
		"yes",
		false,
		"with --prune-orphans, delete the reported rows instead of listing them",
	)
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 || (*confirm && !*pruneOrphans) {
		flags.Usage()
		return 2
	}
	database, manager, code := openHideManager(stderr)
	if code != 0 {
		return code
	}
	defer database.Close()
	if *pruneOrphans {
		return pruneOrphanedHides(
			context.Background(),
			database,
			*confirm,
			stdout,
			stderr,
		)
	}
	rows, err := manager.Hidden(context.Background())
	if err != nil {
		fmt.Fprintf(stderr, "cc-fleet hidden: %v\n", err)
		return 1
	}
	for _, row := range rows {
		fmt.Fprintf(stdout, "%s\t%s\t%d\n", row.ID, row.Engine, row.HiddenAt)
	}
	return 0
}

func runResolve(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet(
		"resolve",
		"usage: cc-fleet resolve label|session|cxwin name",
		stderr,
	)
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 2 {
		flags.Usage()
		return 2
	}
	resolver, err := resolve.New(nil)
	if err != nil {
		fmt.Fprintf(stderr, "cc-fleet resolve: %v\n", err)
		return 2
	}
	outcome, err := resolver.Resolve(
		context.Background(),
		resolve.Kind(flags.Arg(0)),
		flags.Arg(1),
	)
	if err != nil {
		fmt.Fprintf(stderr, "cc-fleet resolve: %v\n", err)
		return 2
	}
	_, _ = io.WriteString(stdout, outcome.Stdout)
	_, _ = io.WriteString(stderr, outcome.Stderr)
	return outcome.Code
}

func runInternal(args []string, stderr io.Writer) int {
	if len(args) != 0 && args[0] == "then" {
		return runInternalThen(args[1:], stderr)
	}
	if len(args) == 0 || args[0] != "hide-exit" {
		fmt.Fprintln(stderr, "usage: cc-fleet internal hide-exit|then [options]")
		return 2
	}
	flags := newFlagSet(
		"internal hide-exit",
		"usage: cc-fleet internal hide-exit --engine cc|cx --id id --path path --socket path --socket-name name --pane %id",
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
		fmt.Fprintf(stderr, "cc-fleet internal hide-exit: %v\n", err)
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
		fmt.Fprintf(stderr, "cc-fleet internal hide-exit: %v\n", err)
		return 1
	}
	return 0
}

func openHideManager(
	stderr io.Writer,
) (*store.Store, *hide.Manager, int) {
	database, err := store.Open(store.WithWarningWriter(stderr))
	if err != nil {
		fmt.Fprintf(stderr, "cc-fleet: %v\n", err)
		return nil, nil, 1
	}
	manager, err := hide.New(database, hide.Dependencies{})
	if err != nil {
		_ = database.Close()
		fmt.Fprintf(stderr, "cc-fleet: %v\n", err)
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

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: cc-fleet <command> [options]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "commands:")
	fmt.Fprintln(w, "  ls        list or pick fleet chats")
	fmt.Fprintln(w, "  open      open an indexed chat by id")
	fmt.Fprintln(w, "  index     refresh the transcript index")
	fmt.Fprintln(w, "  hide      hide a chat, optionally closing it")
	fmt.Fprintln(w, "  unhide    remove a chat hide")
	fmt.Fprintln(w, "  hidden    list or prune chat hides")
	fmt.Fprintln(w, "  resolve   resolve a chat.sh target")
	fmt.Fprintln(w, "  whoami    print this chat's own tmux session name")
	fmt.Fprintln(w, "  mcp       serve chat tools over stdio MCP")
	fmt.Fprintln(w, "  revive    list resumable chats by project")
	fmt.Fprintln(w, "  legacy    repair the hide carrier file against the shared store")
	fmt.Fprintln(w, "  doctor    inspect fleet database and jail health")
	fmt.Fprintln(w, "  version   print the cc-fleet version")
}
