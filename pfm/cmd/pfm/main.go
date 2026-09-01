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

	"hostops/pfm/internal/config"
	pfmengine "hostops/pfm/internal/engine"
	"hostops/pfm/internal/kill"
	"hostops/pfm/internal/mcpserv"
	"hostops/pfm/internal/store"
)

var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	configPath, args, err := splitGlobalConfig(args)
	if err != nil {
		fmt.Fprintf(stderr, "pfm: %v\n", err)
		printUsage(stderr)
		return 2
	}
	runtime, err := loadCommandRuntime(configPath)
	if err != nil {
		if !diagnosticCommand(args) {
			fmt.Fprintf(stderr, "pfm: config: %v\n", err)
			return 1
		}
		runtime, err = loadDiagnosticRuntime(configPath)
		if err != nil {
			fmt.Fprintf(stderr, "pfm: config: %v\n", err)
			return 1
		}
	}
	if len(args) == 0 {
		return runLS(nil, stdout, stderr, runtime)
	}

	switch args[0] {
	case "version", "--version":
		return runVersion(args[1:], stdout, stderr)
	case "ls":
		return runLS(args[1:], stdout, stderr, runtime)
	case "chat":
		return runChatWithRuntime(args[1:], os.Stdin, stdout, stderr, runtime)
	case "harvest":
		return runHarvest(args[1:], stdout, stderr, runtime)
	case "headless":
		fmt.Fprintln(stderr, "pfm: 'headless' is deprecated; use 'pfm chat'")
		return runHeadless(args[1:], stdout, stderr, runtime)
	case "index":
		return runIndex(args[1:], stdout, stderr, runtime)
	case "doctor":
		return runDoctor(args[1:], stdout, stderr, runtime)
	case "config":
		return runConfig(args[1:], stdout, stderr, runtime)
	case "dream":
		return runDreamConfigured(args[1:], os.Stdin, stdout, stderr, runtime)
	case "revive":
		return runRevive(args[1:], stdout, stderr, runtime)
	case "reap":
		return runReap(args[1:], stdout, stderr, runtime)
	case "archive":
		return runArchive(args[1:], stdout, stderr, runtime)
	case "heal":
		return runHeal(args[1:], stdout, stderr)
	case "name-sync":
		return runNameSync(args[1:], stdout, stderr, runtime)
	case "statusline":
		return runStatuslineWithRuntime(args[1:], os.Stdin, stdout, stderr, runtime)
	case "usage-hook":
		return runUsageHookWithRuntime(args[1:], stdout, stderr, runtime)
	case "install":
		return runInstall(args[1:], stdout, stderr, runtime)
	case "uninstall":
		return runUninstall(args[1:], stdout, stderr, runtime)
	case "update":
		return runUpdate(args[1:], stdout, stderr, runtime)
	case "init":
		return runInit(args[1:], stdout, stderr, runtime)
	case "whoami":
		return runWhoami(args[1:], stdout, stderr, runtime)
	case "issues":
		return runIssues(args[1:], stdout, stderr, runtime)
	case "mcp":
		return runMCP(args[1:], stdout, stderr, runtime)
	case pfmengine.MustLookup(pfmengine.Codex).LongName:
		return runCodex(args[1:], stdout, stderr, runtime)
	case "internal":
		return runInternal(args[1:], stdout, stderr, runtime)
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "pfm: unknown command %q\n", args[0])
		printUsage(stderr)
		return 2
	}
}

func diagnosticCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}
	if args[0] == "doctor" {
		return true
	}
	return args[0] == "config" && len(args) > 1 && (args[1] == "show" || args[1] == "validate")
}

func runMCP(
	args []string,
	stdout, stderr io.Writer,
	runtime commandRuntime,
) int {
	// The installed wiring historically invokes bare `pfm mcp`; preserve that
	// argv as the chat server's serve action while making every new form named.
	if len(args) == 0 {
		args = []string{"chat", "serve"}
	}
	if len(args) == 1 && args[0] == "serve" {
		return runMCPServe(stdout, stderr, runtime)
	}
	if len(args) == 1 && args[0] == "ls" {
		for _, name := range config.RegisteredMCPServers() {
			server := runtime.Config.MCPServers[name]
			fmt.Fprintf(
				stdout,
				"%s\t%t\t%s\n",
				name,
				server.Enabled,
				runtime.Config.Source("mcp.servers."+name+".enabled"),
			)
		}
		return 0
	}
	if len(args) < 2 || (len(args) > 2 && !(args[0] == "harvester" && args[1] == "serve")) {
		fmt.Fprintln(stderr, "usage: pfm mcp ls | pfm mcp serve | pfm mcp <server> enable|disable|serve")
		return 2
	}
	name, action := args[0], args[1]
	server, registered := runtime.Config.MCPServers[name]
	if !registered {
		fmt.Fprintf(stderr, "pfm mcp: unknown server %q (run: pfm mcp ls)\n", name)
		return 2
	}
	if action == "enable" || action == "disable" {
		enabled := action == "enable"
		changed, err := config.SetMCPServer(runtime.Config, name, enabled)
		if err != nil {
			fmt.Fprintf(stderr, "pfm mcp %s %s: %v\n", name, action, err)
			return 1
		}
		state := "unchanged"
		if changed {
			state = "updated"
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", name, action+"d", state)
		return 0
	}
	if action != "serve" {
		fmt.Fprintln(stderr, "usage: pfm mcp ls | pfm mcp <server> enable|disable|serve")
		return 2
	}
	if !server.Enabled {
		fmt.Fprintf(
			stderr,
			"pfm mcp %s: disabled by config %s; enable it with: pfm --config %s mcp %s enable\n",
			name,
			runtime.Config.Path,
			runtime.Config.Path,
			name,
		)
		return 1
	}
	if name == "harvester" {
		return runHarvesterMCP(args[2:], stdout, stderr, runtime)
	}
	if name != "chat" {
		fmt.Fprintf(stderr, "pfm mcp %s: registered server has no implementation\n", name)
		return 1
	}
	chatRuntime := mcpRuntime(runtime)
	// Stdio is launched by one chat and inherits that caller deliberately. The
	// shared HTTP daemon uses mcpRuntime's fail-closed default instead.
	chatRuntime.AllowAmbientIdentity = true
	service, err := mcpserv.NewConfigured(version, stderr, chatRuntime)
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

func runKill(args []string, stdout, stderr io.Writer, runtimes ...commandRuntime) int {
	flags := newFlagSet(
		"chat kill",
		"usage: pfm chat kill [self | id] [--exit]",
		stderr,
	)
	self := flags.Bool("self", false, "kill the calling tmux chat")
	exit := flags.Bool("exit", false, "gracefully close after killing")
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() > 1 || (*self && flags.NArg() != 0) ||
		(!*self && flags.NArg() != 1) {
		flags.Usage()
		return 2
	}

	runtime, err := optionalCommandRuntime(runtimes)
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat kill: config: %v\n", err)
		return 1
	}
	database, manager, code := openKillManager(stderr, runtime)
	if code != 0 {
		return code
	}
	defer database.Close()
	ctx := context.Background()
	id := ""
	var engine pfmengine.ID
	rolloutPath := ""
	if flags.NArg() == 1 {
		id = flags.Arg(0)
		// The index may not have caught up with the row the picker already
		// shows — a fresh agent's transcript, a live Codex pane holding no
		// rollout file yet. A compose pass is the picker's own source of
		// truth for what exists right now, so resolving against it vouches
		// for exactly the ids the picker would let you ⌃X, and nothing else.
		engine, rolloutPath = resolveRowEngine(ctx, database, id, stderr, runtime)
	}
	target, err := manager.Kill(ctx, kill.Request{
		ID:          id,
		Engine:      engine,
		RolloutPath: rolloutPath,
		Self:        *self,
		Exit:        *exit,
		Environment: kill.Environment(),
	})
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat kill: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "killed %s\n", target.ID)
	return 0
}

func runUnkill(args []string, stdout, stderr io.Writer, runtimes ...commandRuntime) int {
	flags := newFlagSet("chat unkill", "usage: pfm chat unkill id", stderr)
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 1 {
		flags.Usage()
		return 2
	}
	database, manager, code := openKillManager(stderr, runtimes...)
	if code != 0 {
		return code
	}
	defer database.Close()
	if err := manager.Unkill(context.Background(), flags.Arg(0)); err != nil {
		fmt.Fprintf(stderr, "pfm chat unkill: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "unkilled %s\n", flags.Arg(0))
	return 0
}

func runKilled(args []string, stdout, stderr io.Writer, runtimes ...commandRuntime) int {
	flags := newFlagSet(
		"ls --killed",
		"usage: pfm ls --killed",
		stderr,
	)
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 {
		flags.Usage()
		return 2
	}
	database, manager, code := openKillManager(stderr, runtimes...)
	if code != 0 {
		return code
	}
	defer database.Close()
	rows, err := manager.Killed(context.Background())
	if err != nil {
		fmt.Fprintf(stderr, "pfm ls --killed: %v\n", err)
		return 1
	}
	for _, row := range rows {
		fmt.Fprintf(stdout, "%s\t%s\t%d\n", row.ID, row.Engine, row.KilledAt)
	}
	return 0
}

func runInternal(
	args []string,
	stdout, stderr io.Writer,
	runtime commandRuntime,
) int {
	if len(args) != 0 && args[0] == "clear-kill" {
		return runClearKill(args[1:], os.Stdin, stderr, runtime)
	}
	if len(args) != 0 && args[0] == "agent-open" {
		return runInternalAgentOpen(args[1:], stderr, runtime)
	}
	if len(args) != 0 && args[0] == "launch" {
		return runInternalLaunch(args[1:], stdout, stderr, runtime)
	}
	if len(args) != 0 && args[0] == "launcher-repair" {
		return runInternalLauncherRepair(args[1:], stderr, runtime)
	}
	if len(args) != 0 && args[0] == "explore-deny" {
		return runExploreDeny(os.Stdin, stdout, stderr)
	}
	if len(args) != 0 && args[0] == "epic-inject" {
		return runEpicInject(os.Stdin, stdout, stderr)
	}
	if len(args) != 0 && args[0] == "reload-run" {
		return runChatReloadWorkerWithRuntime(args[1:], os.Stdout, stderr, runtime)
	}
	if len(args) != 0 && args[0] == "then" {
		return runInternalThen(args[1:], stderr, runtime)
	}
	if len(args) != 0 && args[0] == "update-check" {
		return runInternalUpdateCheck(args[1:], stderr)
	}
	if len(args) != 0 && args[0] == "primary-get" {
		fmt.Fprintln(stdout, readPrimaryAccount(runtime.Paths, runtime.Config))
		return 0
	}
	if len(args) != 0 && args[0] == "prompt-args" {
		return runInternalPromptArgs(args[1:], stdout, stderr, runtime)
	}
	if len(args) != 0 && args[0] == "primary-set" {
		flags := newFlagSet(
			"internal primary-set",
			"usage: pfm internal primary-set <account>",
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
		if err := writePrimaryAccount(runtime.Paths, runtime.Config, account); err != nil {
			fmt.Fprintf(stderr, "pfm internal primary-set: %v\n", err)
			return 1
		}
		return 0
	}
	if len(args) == 0 || args[0] != "kill-exit" {
		fmt.Fprintln(stderr, "usage: pfm internal clear-kill|kill-exit|then|update-check|explore-deny|epic-inject [options]")
		return 2
	}
	flags := newFlagSet(
		"internal kill-exit",
		"usage: pfm internal kill-exit --engine cc|cx --id id --path path --socket path --socket-name name --pane %id",
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
	engineID, err := pfmengine.Parse(*engine)
	if err != nil {
		fmt.Fprintf(stderr, "pfm internal kill-exit: %v\n", err)
		return 1
	}
	database, err := store.Open(store.WithWarningWriter(stderr))
	if err != nil {
		fmt.Fprintf(stderr, "pfm internal kill-exit: %v\n", err)
		return 1
	}
	defer database.Close()
	finisher, err := kill.NewFinisher(database, kill.Dependencies{
		Paths:       runtime.Paths,
		ClaudeRoots: runtime.Config.ProjectRoots(),
		CodexRoots:  codexHomes(runtime.Config),
	})
	if err == nil {
		err = finisher.Run(context.Background(), kill.ExitArgs{
			Engine:     engineID,
			ID:         *id,
			DataPath:   *dataPath,
			SocketPath: *socket,
			SocketName: *socketName,
			PaneID:     *pane,
		})
	}
	if err != nil {
		fmt.Fprintf(stderr, "pfm internal kill-exit: %v\n", err)
		return 1
	}
	return 0
}

func openKillManager(
	stderr io.Writer,
	runtimes ...commandRuntime,
) (*store.Store, *kill.Manager, int) {
	runtime, err := optionalCommandRuntime(runtimes)
	if err != nil {
		fmt.Fprintf(stderr, "pfm: config: %v\n", err)
		return nil, nil, 1
	}
	database, err := store.Open(store.WithWarningWriter(stderr))
	if err != nil {
		fmt.Fprintf(stderr, "pfm: %v\n", err)
		return nil, nil, 1
	}
	manager, err := kill.New(database, killDependencies(runtime))
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
	fmt.Fprintln(w, "usage: pfm [--config PATH] <command> [options]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "operator commands:")
	fmt.Fprintln(w, "  ls        list or pick fleet chats")
	fmt.Fprintln(w, "  chat      operate on one chat: new, open, inject, ask, read, stream, name, kill, end")
	fmt.Fprintln(w, "  harvest   fetch and convert URL, DOI, ISBN, PMID, PMCID, or local path")
	fmt.Fprintln(w, "  dream     build and inject repository memory organs")
	fmt.Fprintln(w, "  index     refresh the transcript index")
	fmt.Fprintln(w, "  whoami    print this chat's own tmux session name")
	fmt.Fprintln(w, "  issues    list servicedesk complaints filed through issue_servicedesk")
	fmt.Fprintln(w, "  revive    list resumable chats by project")
	fmt.Fprintln(w, "  reap      classify the socket graveyard; --apply reclaims it")
	fmt.Fprintln(w, "  archive   move killed chats and old subagent transcripts out of sight, reversibly")
	fmt.Fprintln(w, "  heal      report or repair wedged Codex history projections")
	fmt.Fprintln(w, "  install   wire or remove the self-contained host integration")
	fmt.Fprintln(w, "  uninstall remove the self-contained host integration")
	fmt.Fprintln(w, "  update    update the binary; check, adopt, pin, ignore, or drop project template baselines")
	fmt.Fprintln(w, "  init      scaffold project templates once and pin their baselines")
	fmt.Fprintln(w, "  config    initialize, inspect, or validate machine configuration")
	fmt.Fprintln(w, "  doctor    inspect fleet database and jail health")
	fmt.Fprintln(w, "  version   print the pfm version")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "wiring commands:")
	fmt.Fprintln(w, "  name-sync converge live chat window names")
	fmt.Fprintln(w, "  statusline render the native Claude status line")
	fmt.Fprintln(w, "  usage-hook the fail-open usage-limit prompt hook")
	fmt.Fprintln(w, "  mcp       list, configure, or serve registered MCP servers (stdio or loopback HTTP)")
	fmt.Fprintln(w, "  codex     compile or check the Codex project mirror")
}
