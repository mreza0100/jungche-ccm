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
	if len(args) == 0 {
		printHeadlessUsage(stderr)
		return 2
	}
	verb, rest := args[0], args[1:]
	switch verb {
	case "run":
		return runRun(rest, stdout, stderr)
	case "transcript":
		return runHeadlessTranscript(rest, stdout, stderr)
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
	case "ls":
		return runHeadlessLS(rest, stdout, stderr)
	case "help", "-h", "--help":
		printHeadlessUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "pfm headless: unknown command %q\n", verb)
		printHeadlessUsage(stderr)
		return 2
	}
}

func printHeadlessUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: pfm headless <command> [options]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "commands:")
	fmt.Fprintln(w, "  run         start a named chat detached on its own server")
	fmt.Fprintln(w, "  ls          list headless-reachable chats")
	fmt.Fprintln(w, "  status      one line (or --json) on a chat's state")
	fmt.Fprintln(w, "  last        the chat's last assistant message")
	fmt.Fprintln(w, "  transcript  read the chat's transcript")
	fmt.Fprintln(w, "  stream      follow the transcript as it is written")
	fmt.Fprintln(w, "  inject      deliver a message to the chat")
	fmt.Fprintln(w, "  ask         deliver a message and wait for the answer")
	fmt.Fprintln(w, "  watch       block, reporting IDLE / EXIT / DEAD")
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
		fmt.Fprintf(stderr, "pfm headless: %v\n", err)
		return headless.Chat{}, 2
	}
	if !found {
		if asJSON {
			writeJSON(stdout, headless.Missing(name))
		} else {
			fmt.Fprintf(stdout, "%s\t%s\n", name, headless.StateMissing)
		}
		fmt.Fprintf(stderr, "pfm headless: no chat named %q\n", name)
		return headless.Chat{}, codeUnknownChat
	}
	return chat, 0
}

func runHeadlessStatus(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet(
		"headless status",
		"usage: pfm headless status <name> [--json]",
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
		fmt.Fprintf(stderr, "pfm headless status: %v\n", err)
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
		"headless transcript",
		"usage: pfm headless transcript <name> [--tail N] [--condensed] [--json]",
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
		fmt.Fprintf(stderr, "pfm headless transcript: %q has not written a transcript yet\n", chat.Name)
		return codeDeadChat
	}
	entries, truncated, err := transcript.Tail(ctx, chat.Path, chat.Engine, *tail, 0)
	if err != nil {
		fmt.Fprintf(stderr, "pfm headless transcript: %v\n", err)
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
		"headless last",
		"usage: pfm headless last <name>",
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
		fmt.Fprintf(stderr, "pfm headless last: %q has not written a transcript yet\n", chat.Name)
		return codeDeadChat
	}
	// A wide window, then the newest assistant entry within it: the last thing
	// SAID, however many tool calls have happened since.
	entries, _, err := transcript.Tail(ctx, chat.Path, chat.Engine, 200, 0)
	if err != nil {
		fmt.Fprintf(stderr, "pfm headless last: %v\n", err)
		return 1
	}
	entry, found := transcript.Last(entries, transcript.RoleAssistant)
	if !found {
		fmt.Fprintf(stderr, "pfm headless last: %q has not answered yet\n", chat.Name)
		return codeDeadChat
	}
	fmt.Fprintln(stdout, entry.Text)
	return 0
}

func runHeadlessStream(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet(
		"headless stream",
		"usage: pfm headless stream <name> [--filter REGEX] [--margin N] "+
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
			fmt.Fprintf(stderr, "pfm headless stream: bad --filter: %v\n", err)
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
		fmt.Fprintf(stderr, "pfm headless stream: %q has not written a transcript yet\n", chat.Name)
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
			fmt.Fprintf(stderr, "pfm headless stream: %s is gone\n", name)
			return codeDeadChat
		}
		fmt.Fprintf(stderr, "pfm headless stream: %v\n", err)
		return 1
	}
	return 0
}

func runHeadlessInject(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet(
		"headless inject",
		"usage: pfm headless inject <name> <message>",
		stderr,
	)
	force := flags.Bool("now", false, "interrupt a working chat instead of waiting")
	// Only the flags BEFORE the name are parsed: everything after it is the
	// message, verbatim. A message may legitimately start with a dash, and an
	// order silently eaten as a flag is an order never delivered.
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() < 2 {
		flags.Usage()
		return 2
	}
	ctx := context.Background()
	chat, code := headlessTarget(ctx, flags.Arg(0), stdout, stderr, false)
	if code != 0 {
		return code
	}
	if !chat.Live {
		fmt.Fprintf(stderr, "pfm headless inject: %q is not running\n", chat.Name)
		return codeDeadChat
	}
	engine, err := inject.New(inject.Dependencies{})
	if err != nil {
		fmt.Fprintf(stderr, "pfm headless inject: %v\n", err)
		return 1
	}
	// The socket is addressed directly: this command already knows exactly
	// which seat it resolved, and a second name lookup inside the injector
	// could land on a different one.
	result, err := engine.Inject(ctx, inject.Request{
		Target:   chat.Socket,
		Message:  strings.Join(flags.Args()[1:], " "),
		ForceNow: *force,
	})
	if err != nil {
		fmt.Fprintf(stderr, "pfm headless inject: %v\n", err)
		return 1
	}
	if result.Unsigned {
		// The recipient is told the message is unsigned; the SENDER is the one
		// who can do something about it, and only if the reason reaches them.
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
	fmt.Fprintln(stdout, result.Message)
	return result.Code
}

func runHeadlessWatch(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet(
		"headless watch",
		"usage: pfm headless watch <name> [--idle-after SECS] "+
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
		fmt.Fprintf(stderr, "pfm headless watch: %v\n", err)
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
			fmt.Fprintf(stderr, "pfm headless watch: hook failed: %v\n", err)
		}
		return nil
	}
}

func runHeadlessLS(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet(
		"headless ls",
		"usage: pfm headless ls [--json]",
		stderr,
	)
	asJSON := flags.Bool("json", false, "emit a JSON array")
	names, code, ok := parseFlagsAnywhere(flags, args)
	if !ok {
		return code
	}
	if len(names) != 0 {
		flags.Usage()
		return 2
	}
	ctx := context.Background()
	database, err := store.Open(store.WithWarningWriter(io.Discard))
	if err != nil {
		fmt.Fprintf(stderr, "pfm headless ls: %v\n", err)
		return 1
	}
	defer database.Close()
	scan, err := scanFleet(
		ctx,
		database,
		scanRequest{View: compose.AllView, ReadOnly: true},
		io.Discard,
	)
	if err != nil {
		fmt.Fprintf(stderr, "pfm headless ls: %v\n", err)
		return 1
	}
	statuses := make([]headless.Status, 0, len(scan.Output.Rows))
	now := time.Now()
	for _, row := range scan.Output.Rows {
		if !isLiveKind(row.Kind) || row.ID == "" {
			continue
		}
		status, err := headless.Inspect(ctx, chatFromRow(row), now)
		if err != nil {
			continue
		}
		statuses = append(statuses, status)
	}
	if *asJSON {
		writeJSON(stdout, statuses)
		return 0
	}
	for _, status := range statuses {
		fmt.Fprintln(stdout, status.Line())
	}
	return 0
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
