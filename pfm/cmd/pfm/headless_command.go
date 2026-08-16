package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	"hostops/pfm/internal/compose"
	"hostops/pfm/internal/headless"
	"hostops/pfm/internal/inject"
	"hostops/pfm/internal/paths"
	"hostops/pfm/internal/resolve"
	"hostops/pfm/internal/store"
	"hostops/pfm/internal/transcript"
)

// Exit codes the headless family answers with. A consumer scripts against
// these, so they are a contract: 0 only ever means the chat was found and the
// verb did what it says.
const (
	codeUnknownChat = 4
	codeDeadChat    = 3
	// codeAwaitTimeout says the message was delivered and the chat is still
	// working — a different fact from every failure, and the one a caller
	// retries rather than escalates.
	codeAwaitTimeout = 5
	// codeUndelivered says nothing reached the model. The chat may be fine;
	// the message is not in it.
	codeUndelivered = 6
)

func runHeadless(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "run":
			args[0] = "new"
		case "transcript":
			args[0] = "read"
		}
	}
	return runChat(args, os.Stdin, stdout, stderr)
}

func runChat(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printChatUsage(stderr)
		return 2
	}
	verb, rest := args[0], args[1:]
	switch verb {
	case "new":
		return runRun(rest, stdout, stderr)
	case "open":
		return runChatOpen(rest, stdout, stderr)
	case "read":
		return runChatRead(rest, stdin, stdout, stderr)
	case "last":
		return runHeadlessLast(rest, stdout, stderr)
	case "status":
		return runHeadlessStatus(rest, stdout, stderr)
	case "stream":
		return runHeadlessStream(rest, stdout, stderr)
	case "inject":
		return runHeadlessInject(rest, stdout, stderr)
	case "ask":
		return runHeadlessAsk(rest, stdout, stderr)
	case "watch":
		return runHeadlessWatch(rest, stdout, stderr)
	case "capture":
		return runChatCapture(rest, stdout, stderr)
	case "recover":
		return runChatRecover(rest, stdout, stderr)
	case "name":
		return runChatName(rest, stdout, stderr)
	case "hide":
		return runChatHide(rest, stdout, stderr)
	case "unhide":
		return runChatUnhide(rest, stdout, stderr)
	case "end":
		return runChatEnd(rest, stdout, stderr)
	case "swap":
		return runChatSwap(rest, stdout, stderr)
	case "whoami":
		// Compatibility for the installed chat.sh shim. The settled public
		// command is `pfm whoami`, but the shim must remain safe while callers
		// still invoke `chat.sh whoami` during the staged cutover.
		return runWhoami(rest, stdout, stderr)
	case "find", "save", "load", "branch", "history", "ls":
		return runChatSatellite(verb, rest, stdin, stdout, stderr)
	case "modal":
		return runChatModal(rest, stdout, stderr)
	case "group":
		return runChatGroup(rest, stdin, stdout, stderr)
	case "resolve":
		return runChatResolve(rest, stdout, stderr)
	case "help", "-h", "--help":
		printChatUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "pfm chat: unknown command %q\n", verb)
		printChatUsage(stderr)
		return 2
	}
}

func printChatUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: pfm chat <command> [options]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "commands:")
	fmt.Fprintln(w, "  new         start a named chat on its own server")
	fmt.Fprintln(w, "  open        open a chat by name, socket, or id")
	fmt.Fprintln(w, "  status      one line (or --json) on a chat's state")
	fmt.Fprintln(w, "  last        the chat's last assistant message")
	fmt.Fprintln(w, "  read        read the chat's transcript")
	fmt.Fprintln(w, "  stream      follow the transcript as it is written")
	fmt.Fprintln(w, "  inject      deliver a message to the chat")
	fmt.Fprintln(w, "  ask         deliver a message and wait for the answer")
	fmt.Fprintln(w, "  watch       block, reporting IDLE / EXIT / DEAD")
	fmt.Fprintln(w, "  capture     print a live chat's tmux scrollback")
	fmt.Fprintln(w, "  recover     rebuild a Codex conversation from its rollout")
	fmt.Fprintln(w, "  name        rename a chat and converge its window")
	fmt.Fprintln(w, "  hide        hide a chat, optionally closing it")
	fmt.Fprintln(w, "  unhide      remove a chat hide")
	fmt.Fprintln(w, "  end         end a chat's tmux server")
	fmt.Fprintln(w, "  swap        reboot a Claude chat in place under another account/cache mode")
	fmt.Fprintln(w, "  find/save/load/branch/history/ls/group/resolve")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "exit codes: 0 done · 2 usage · 3 chat dead · 4 no such chat")
	fmt.Fprintln(w, "            5 answer timed out · 6 message not delivered")
}

// resolveChat finds a chat by name, id, or socket over a live compose pass —
// the same rows the picker shows, so a chat the user can see is a chat these
// commands can address. Ambiguity is refused rather than guessed at.
// warn is where gather's probe warnings go. The read verbs pass io.Discard:
// they answer about ONE chat, and a warning about somebody else's dead socket
// is noise in the middle of a machine-facing answer. `run` keeps them.
func resolveChat(
	ctx context.Context,
	name string,
	warn io.Writer,
) (headless.Chat, bool, error) {
	if name == "self" || name == "me" {
		identifier, err := resolve.NewWhoami(resolve.WhoamiDependencies{})
		if err != nil {
			return headless.Chat{}, false, err
		}
		identity, err := identifier.Identify(ctx)
		if err != nil {
			seat, found := codexSeatIdentity(ctx)
			if !found {
				return headless.Chat{}, false, err
			}
			identity = seat
		}
		switch {
		case identity.ID != "":
			name = identity.ID
		case identity.SocketName != "":
			name = identity.SocketName
		default:
			name = identity.Session
		}
	}
	database, err := store.Open(store.WithWarningWriter(warn))
	if err != nil {
		return headless.Chat{}, false, err
	}
	defer database.Close()
	scan, err := scanFleet(
		ctx,
		database,
		scanRequest{View: compose.AllView, ReadOnly: true},
		warn,
	)
	if err != nil {
		return headless.Chat{}, false, err
	}
	return matchChat(scan.Output.Rows, name)
}

func matchChat(rows []compose.Row, name string) (headless.Chat, bool, error) {
	exact := make([]compose.Row, 0, 2)
	folded := make([]compose.Row, 0, 2)
	for _, row := range rows {
		switch {
		case row.ID == name || row.Socket == name || row.Name == name:
			exact = append(exact, row)
		case strings.EqualFold(row.Name, name) ||
			(len(name) >= 8 && strings.HasPrefix(row.ID, name)):
			folded = append(folded, row)
		}
	}
	candidates := exact
	if len(candidates) == 0 {
		candidates = folded
	}
	if len(candidates) == 0 {
		return headless.Chat{}, false, nil
	}
	if len(candidates) > 1 {
		// A live row and its own resume row are the same conversation; prefer
		// the live one rather than calling the pair ambiguous.
		live := make([]compose.Row, 0, len(candidates))
		for _, row := range candidates {
			if isLiveKind(row.Kind) {
				live = append(live, row)
			}
		}
		if len(live) == 1 {
			candidates = live
		}
	}
	if len(candidates) > 1 {
		names := make([]string, 0, len(candidates))
		for _, row := range candidates {
			names = append(names, fmt.Sprintf("%s (%s)", row.Name, row.ID))
		}
		sort.Strings(names)
		return headless.Chat{}, false, fmt.Errorf(
			"%q matches %d chats: %s",
			name,
			len(candidates),
			strings.Join(names, ", "),
		)
	}
	return chatFromRow(candidates[0]), true, nil
}

func isLiveKind(kind compose.Kind) bool {
	return kind == compose.LiveClaude ||
		kind == compose.LiveCodex ||
		kind == compose.LiveSplit ||
		kind == compose.Agent ||
		kind == compose.Booting
}

func chatFromRow(row compose.Row) headless.Chat {
	return headless.Chat{
		Name:    row.Name,
		ID:      row.ID,
		Engine:  compose.EngineForKind(row.Kind),
		Path:    row.Path,
		CWD:     row.CWD,
		Socket:  row.Socket,
		Session: row.SessionName,
		Pane:    row.PaneID,
		Live:    isLiveKind(row.Kind),
	}
}

// headlessTarget resolves a name for a verb that needs one, reporting the
// refusal itself. A name nothing answers to is never silent and never rc 0.
func headlessTarget(
	ctx context.Context,
	name string,
	stdout, stderr io.Writer,
	asJSON bool,
) (headless.Chat, int) {
	chat, found, err := resolveChat(ctx, name, io.Discard)
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat: %v\n", err)
		return headless.Chat{}, 2
	}
	if !found {
		if asJSON {
			writeJSON(stdout, headless.Missing(name))
		} else {
			fmt.Fprintf(stdout, "%s\t%s\n", name, headless.StateMissing)
		}
		fmt.Fprintf(stderr, "pfm chat: no chat named %q\n", name)
		return headless.Chat{}, codeUnknownChat
	}
	return chat, 0
}

func runHeadlessStatus(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet(
		"chat status",
		"usage: pfm chat status <target> [--json]",
		stderr,
	)
	asJSON := flags.Bool("json", false, "emit one JSON object")
	names, code, ok := parseFlagsAnywhere(flags, args)
	if !ok {
		return code
	}
	if len(names) != 1 {
		flags.Usage()
		return 2
	}
	ctx := context.Background()
	chat, code := headlessTarget(ctx, names[0], stdout, stderr, *asJSON)
	if code != 0 {
		return code
	}
	status, err := headless.Inspect(ctx, chat, time.Now())
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat status: %v\n", err)
		return 1
	}
	if *asJSON {
		writeJSON(stdout, status)
	} else {
		fmt.Fprintln(stdout, status.Line())
	}
	if !status.Alive() {
		return codeDeadChat
	}
	return 0
}

func runHeadlessTranscript(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet(
		"chat read",
		"usage: pfm chat read <target> [--tail N] [--condensed] [--json]",
		stderr,
	)
	tail := flags.Int("tail", 1, "how many entries to read, newest last")
	condensed := flags.Bool("condensed", false, "one T/A/U line per entry")
	asJSON := flags.Bool("json", false, "emit a JSON array of entries")
	names, code, ok := parseFlagsAnywhere(flags, args)
	if !ok {
		return code
	}
	if len(names) != 1 || *tail < 1 {
		flags.Usage()
		return 2
	}
	ctx := context.Background()
	chat, code := headlessTarget(ctx, names[0], stdout, stderr, *asJSON)
	if code != 0 {
		return code
	}
	if chat.Path == "" {
		fmt.Fprintf(stderr, "pfm chat read: %q has not written a transcript yet\n", chat.Name)
		return codeDeadChat
	}
	entries, truncated, err := transcript.Tail(ctx, chat.Path, chat.Engine, *tail, 0)
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat read: %v\n", err)
		return 1
	}
	switch {
	case *asJSON:
		writeJSON(stdout, map[string]any{
			"name":      chat.Name,
			"engine":    chat.Engine,
			"path":      chat.Path,
			"truncated": truncated,
			"entries":   entries,
		})
	case *condensed:
		for _, entry := range entries {
			fmt.Fprintln(stdout, transcript.Condensed(entry))
		}
	default:
		for _, entry := range entries {
			fmt.Fprintf(stdout, "%s: %s\n", entry.Role, entryText(entry))
		}
	}
	return 0
}

func runHeadlessLast(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet(
		"chat last",
		"usage: pfm chat last <target>",
		stderr,
	)
	names, code, ok := parseFlagsAnywhere(flags, args)
	if !ok {
		return code
	}
	if len(names) != 1 {
		flags.Usage()
		return 2
	}
	ctx := context.Background()
	chat, code := headlessTarget(ctx, names[0], stdout, stderr, false)
	if code != 0 {
		return code
	}
	if chat.Path == "" {
		fmt.Fprintf(stderr, "pfm chat last: %q has not written a transcript yet\n", chat.Name)
		return codeDeadChat
	}
	// A wide window, then the newest assistant entry within it: the last thing
	// SAID, however many tool calls have happened since.
	entries, _, err := transcript.Tail(ctx, chat.Path, chat.Engine, 200, 0)
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat last: %v\n", err)
		return 1
	}
	entry, found := transcript.Last(entries, transcript.RoleAssistant)
	if !found {
		fmt.Fprintf(stderr, "pfm chat last: %q has not answered yet\n", chat.Name)
		return codeDeadChat
	}
	fmt.Fprintln(stdout, entry.Text)
	return 0
}

func runHeadlessStream(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet(
		"chat stream",
		"usage: pfm chat stream <target> [--filter REGEX] [--margin N] "+
			"[--from-start] [--raw] [--no-follow]",
		stderr,
	)
	filter := flags.String("filter", "", "keep only lines matching this regexp")
	margin := flags.Int("margin", 0, "lines of context on each side of a match")
	fromStart := flags.Bool("from-start", false, "replay the transcript before following")
	raw := flags.Bool("raw", false, "print full entry text instead of condensed lines")
	noFollow := flags.Bool("no-follow", false, "drain what exists and exit")
	names, code, ok := parseFlagsAnywhere(flags, args)
	if !ok {
		return code
	}
	if len(names) != 1 || *margin < 0 {
		flags.Usage()
		return 2
	}
	var pattern *regexp.Regexp
	if *filter != "" {
		compiled, err := regexp.Compile(*filter)
		if err != nil {
			fmt.Fprintf(stderr, "pfm chat stream: bad --filter: %v\n", err)
			return 2
		}
		pattern = compiled
	}
	ctx := context.Background()
	chat, code := headlessTarget(ctx, names[0], stdout, stderr, false)
	if code != 0 {
		return code
	}
	if chat.Path == "" {
		fmt.Fprintf(stderr, "pfm chat stream: %q has not written a transcript yet\n", chat.Name)
		return codeDeadChat
	}
	name := chat.Name
	err := headless.Stream(ctx, chat.Path, chat.Engine, headless.StreamOptions{
		Filter:    pattern,
		Margin:    *margin,
		FromStart: *fromStart,
		Follow:    !*noFollow,
		Raw:       *raw,
		Alive: func() bool {
			chat, found, err := resolveChat(context.Background(), name, io.Discard)
			return err == nil && found && chat.Live
		},
	}, stdout)
	if err != nil {
		if err == headless.ErrChatGone {
			fmt.Fprintf(stderr, "pfm chat stream: %s is gone\n", name)
			return codeDeadChat
		}
		fmt.Fprintf(stderr, "pfm chat stream: %v\n", err)
		return 1
	}
	return 0
}

func runHeadlessInject(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet(
		"chat inject",
		"usage: pfm chat inject [--force-now] [--then STEER]... [--file PATH] <target> <message>",
		stderr,
	)
	var force bool
	var retiredNoSig bool
	flags.BoolVar(&force, "now", false, "interrupt a working chat instead of waiting")
	flags.BoolVar(&force, "force-now", false, "interrupt a working chat instead of waiting")
	flags.BoolVar(&retiredNoSig, "no-sig", false, "retired compatibility flag; signatures remain mandatory")
	messageFile := flags.String("file", "", "read the message from a file")
	var steers steerList
	flags.Var(&steers, "then", "follow-up steer; repeat for a chain")
	// Only the flags BEFORE the name are parsed: everything after it is the
	// message, verbatim. A message may legitimately start with a dash, and an
	// order silently eaten as a flag is an order never delivered.
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if retiredNoSig {
		fmt.Fprintln(stderr, "pfm chat inject: --no-sig is retired; delivery signatures remain enabled")
	}
	minimum := 2
	if *messageFile != "" {
		minimum = 1
	}
	if flags.NArg() < minimum {
		flags.Usage()
		return 2
	}
	message := strings.Join(flags.Args()[1:], " ")
	if *messageFile != "" {
		if flags.NArg() != 1 {
			flags.Usage()
			return 2
		}
		content, err := os.ReadFile(*messageFile)
		if err != nil {
			fmt.Fprintf(stderr, "pfm chat inject: read --file: %v\n", err)
			return 2
		}
		message = string(content)
	}
	if strings.TrimSpace(message) == "" {
		fmt.Fprintln(stderr, "pfm chat inject: refusing to inject an empty message")
		return 2
	}
	ctx := context.Background()
	engine, err := newInjectEngine()
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat inject: %v\n", err)
		return codeUndelivered
	}
	result, err := engine.Inject(ctx, inject.Request{
		Target:     flags.Arg(0),
		Message:    message,
		ForceNow:   force,
		FileBacked: *messageFile != "",
		Then:       steers,
	})
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat inject: %v\n", err)
		return codeUndelivered
	}
	if result.Code == inject.CodeUnknown && !strings.Contains(strings.ToLower(result.Message), "ambiguous") {
		target, found, resolveErr := resolveResumeTarget(flags.Arg(0))
		if resolveErr != nil {
			fmt.Fprintf(stderr, "pfm chat inject: %v\n", resolveErr)
			return codeUnknownChat
		}
		if found {
			resolved, pathErr := paths.Resolve()
			if pathErr != nil {
				fmt.Fprintf(stderr, "pfm chat inject: %v\n", pathErr)
				return codeUndelivered
			}
			if uuidSessionID(target.ID) {
				live, liveErr := runningClaudeSession(resolved.ProcRoot, target.ID)
				if liveErr != nil {
					fmt.Fprintf(stderr, "pfm chat inject: %v\n", liveErr)
					return codeUndelivered
				}
				if live {
					fmt.Fprintf(
						stderr,
						"pfm chat inject: %q is a LIVE session held by a running process with no resolvable tmux pane; transcript append refused\n",
						flags.Arg(0),
					)
					return codeDeadChat
				}
				crumb, live, crumbErr := liveCrumbSession(ctx, resolved, target.ID)
				if crumbErr != nil {
					fmt.Fprintf(stderr, "pfm chat inject: %v\n", crumbErr)
					return codeUndelivered
				}
				if live {
					fmt.Fprintf(
						stderr,
						"pfm chat inject: %q is a LIVE session at %s with no resolvable target on this path; transcript append refused\n",
						flags.Arg(0),
						crumb,
					)
					return codeDeadChat
				}
				config, live, registryErr := registeredDaemonSession(ctx, resolved, target.ID)
				if registryErr != nil {
					fmt.Fprintf(stderr, "pfm chat inject: %v\n", registryErr)
					return codeUndelivered
				}
				if live {
					fmt.Fprintf(
						stderr,
						"pfm chat inject: %q is a LIVE background agent under config %s; it reads its input stream, not transcript appends, so nothing was appended\n",
						flags.Arg(0),
						config,
					)
					return codeDeadChat
				}
			}
			if force {
				fmt.Fprintln(stderr, "pfm chat inject: --force-now ignored for a dormant RESUME target")
			}
			if len(steers) > 0 {
				fmt.Fprintf(stderr, "pfm chat inject: --then ignored for a dormant RESUME target (%d steer(s))\n", len(steers))
			}
			prepared, prepareErr := engine.PrepareForResume(
				ctx,
				target.ID,
				"cc",
				message,
			)
			if prepareErr != nil {
				fmt.Fprintf(stderr, "pfm chat inject: prepare resume body: %v\n", prepareErr)
				return codeUndelivered
			}
			receipt, appendErr := appendResumeInjection(ctx, resolved, target, prepared.Message)
			if appendErr != nil {
				fmt.Fprintf(stderr, "pfm chat inject: %v\n", appendErr)
				return codeUndelivered
			}
			if prepared.Unsigned {
				writeUnsignedInjectWarning(stderr)
			}
			if prepared.AutoFilePath != "" {
				fmt.Fprintf(
					stdout,
					"injected RESUME AUTO-FILE pointer %s (parent %s) into session %s — full body saved at %s; answered on next reopen; backup: %s\n",
					receipt.EventID,
					receipt.ParentID,
					receipt.SessionID,
					prepared.AutoFilePath,
					receipt.Backup,
				)
			} else {
				fmt.Fprintf(
					stdout,
					"injected RESUME user turn %s (parent %s) into session %s — answered on next reopen; backup: %s\n",
					receipt.EventID,
					receipt.ParentID,
					receipt.SessionID,
					receipt.Backup,
				)
			}
			for _, warning := range prepared.Warnings {
				fmt.Fprintf(stderr, "pfm chat inject: WARNING: %s\n", warning)
			}
			return 0
		}
	}
	if result.Unsigned {
		// The recipient is told the message is unsigned; the SENDER is the one
		// who can do something about it, and only if the reason reaches them.
		writeUnsignedInjectWarning(stderr)
	}
	return writeInjectResult(result, flags.Arg(0), stdout, stderr)
}

func writeInjectResult(
	result inject.Result,
	target string,
	stdout, stderr io.Writer,
) int {
	if result.ResolutionNote != "" {
		fmt.Fprintln(stderr, result.ResolutionNote)
	}
	if result.Code != 0 {
		if result.Code == inject.CodeUnknown {
			fmt.Fprintf(stderr, "pfm chat: no chat named %q\n", target)
			if strings.Contains(strings.ToLower(result.Message), "ambiguous") {
				fmt.Fprintln(stderr, result.Message)
			}
		} else {
			fmt.Fprintln(stderr, result.Message)
		}
		if result.Proof != "" {
			fmt.Fprintln(stderr, "--- delivery proof (FAILED): target pane tail ---")
			fmt.Fprintln(stderr, result.Proof)
			fmt.Fprintln(stderr, "--- end delivery proof ---")
		}
		switch result.Code {
		case inject.CodeUnknown:
			return codeUnknownChat
		case inject.CodeDead:
			return codeDeadChat
		default:
			return codeUndelivered
		}
	}
	fmt.Fprintln(stdout, result.Message)
	fmt.Fprintln(stdout, "--- delivery proof: target pane tail ---")
	fmt.Fprintln(stdout, result.Proof)
	fmt.Fprintln(stdout, "--- end delivery proof ---")
	return 0
}

func writeUnsignedInjectWarning(stderr io.Writer) {
	fmt.Fprintln(
		stderr,
		"WARNING: sent UNSIGNED — this process derived no identity of its"+
			" own. If it ran DETACHED (setsid/nohup/disowned), that is"+
			" why: detaching severs the process chain the handle is"+
			" recovered from. Send from the chat itself, or state it: "+
			inject.SenderSessionEnv+"=$(pfm whoami) "+
			inject.SenderLabelEnv+"=<label> <command>.",
	)
}

func runHeadlessWatch(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet(
		"chat watch",
		"usage: pfm chat watch <target> [--idle-after SECS] "+
			"[--on-idle CMD] [--on-exit CMD] [--once]",
		stderr,
	)
	idleAfter := flags.Int("idle-after", 0, "seconds of idle before IDLE is emitted")
	onIdle := flags.String("on-idle", "", "shell command to run on IDLE")
	onExit := flags.String("on-exit", "", "shell command to run on EXIT or DEAD")
	once := flags.Bool("once", false, "stop after the first IDLE")
	poll := flags.Int("poll", 2, "seconds between samples")
	names, code, ok := parseFlagsAnywhere(flags, args)
	if !ok {
		return code
	}
	if len(names) != 1 || *idleAfter < 0 || *poll < 1 {
		flags.Usage()
		return 2
	}
	name := names[0]
	ctx := context.Background()
	if _, code := headlessTarget(ctx, name, stdout, stderr, false); code != 0 {
		return code
	}
	watcher := headless.Watcher{
		Name: name,
		Resolve: func(ctx context.Context) (headless.Chat, bool, error) {
			return resolveChat(ctx, name, io.Discard)
		},
	}
	status, err := watcher.Watch(ctx, headless.WatchOptions{
		IdleAfter: time.Duration(*idleAfter) * time.Second,
		Poll:      time.Duration(*poll) * time.Second,
		Once:      *once,
		OnIdle:    hookRunner(*onIdle, stderr),
		OnExit:    hookRunner(*onExit, stderr),
	}, stdout)
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat watch: %v\n", err)
		return 1
	}
	if !status.Alive() {
		return codeDeadChat
	}
	return 0
}

// hookRunner runs a --on-idle/--on-exit command with the chat's facts in the
// environment, so a hook can act without re-resolving anything.
func hookRunner(command string, stderr io.Writer) func(headless.Status) error {
	if strings.TrimSpace(command) == "" {
		return nil
	}
	return func(status headless.Status) error {
		process := exec.Command("sh", "-c", command)
		process.Env = append(
			os.Environ(),
			"CC_CHAT_NAME="+status.Name,
			"CC_CHAT_STATE="+status.State,
			"CC_CHAT_ENGINE="+status.Engine,
			"CC_CHAT_SOCKET="+status.Socket,
			"CC_CHAT_SESSION_ID="+status.SessionID,
		)
		process.Stdout = stderr
		process.Stderr = stderr
		if err := process.Run(); err != nil {
			fmt.Fprintf(stderr, "pfm chat watch: hook failed: %v\n", err)
		}
		return nil
	}
}

func entryText(entry transcript.Entry) string {
	if entry.Role == transcript.RoleTool {
		return entry.Tool + " " + entry.Input
	}
	return entry.Text
}

func writeJSON(out io.Writer, value any) {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(value)
}
