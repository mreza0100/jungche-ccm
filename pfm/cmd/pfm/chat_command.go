package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"hostops/pfm/internal/deps"
	pfmengine "hostops/pfm/internal/engine"
	"hostops/pfm/internal/headless"
	"hostops/pfm/internal/inject"
	"hostops/pfm/internal/kill"
	"hostops/pfm/internal/paths"
	"hostops/pfm/internal/recovery"
	"hostops/pfm/internal/resolve"
)

var chatUUIDPattern = regexp.MustCompile(
	`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`,
)

func runChatRead(args []string, stdin io.Reader, stdout, stderr io.Writer, runtimes ...commandRuntime) int {
	if len(args) > 0 {
		if info, err := os.Stat(args[0]); err == nil && info.Mode().IsRegular() &&
			filepath.Ext(args[0]) != ".jsonl" {
			return runChatReadExcerpt(args, stdout, stderr, runtimes...)
		}
	}
	return runHeadlessTranscript(args, stdout, stderr, runtimes...)
}

func runChatOpen(
	args []string,
	stdout, stderr io.Writer,
	runtime commandRuntime,
) int {
	flags := newFlagSet("chat open", "usage: pfm chat open <target>", stderr)
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 1 {
		flags.Usage()
		return 2
	}
	chat, code := headlessTarget(context.Background(), flags.Arg(0), stdout, stderr, false, runtime)
	if code != 0 {
		return code
	}
	return openID(context.Background(), chat.ID, stdout, stderr, runtime)
}

func runChatKill(args []string, stdout, stderr io.Writer, runtimes ...commandRuntime) int {
	flags := newFlagSet("chat kill", "usage: pfm chat kill <target> [--exit]", stderr)
	exit := flags.Bool("exit", false, "gracefully close after killing")
	targets, code, ok := parseFlagsAnywhere(flags, args)
	if !ok {
		return code
	}
	if len(targets) != 1 {
		flags.Usage()
		return 2
	}
	if targets[0] == "self" || targets[0] == "me" {
		// Codex tool shells are served by app-server and carry no TMUX. Resolve
		// their CODEX_THREAD_ID through the fleet store, then preserve the live
		// row's immutable socket and pane for the detached exit finisher.
		if os.Getenv("TMUX") == "" && os.Getenv(resolve.CodexThreadEnv) != "" {
			chat, found, err := resolveChat(context.Background(), "self", io.Discard, runtimes...)
			if err != nil {
				fmt.Fprintf(stderr, "pfm chat kill: %v\n", err)
				return 1
			}
			if !found {
				fmt.Fprintln(stderr, "pfm chat kill: this Codex chat has no live fleet seat")
				return codeUnknownChat
			}
			return runResolvedChatKill(chat, *exit, stdout, stderr, runtimes...)
		}
		killArgs := []string{"--self"}
		if *exit {
			killArgs = append(killArgs, "--exit")
		}
		return runKill(killArgs, stdout, stderr, runtimes...)
	}
	target := targets[0]
	id := target
	if !chatUUIDPattern.MatchString(target) {
		chat, found, err := resolveChat(context.Background(), target, io.Discard, runtimes...)
		if err != nil {
			fmt.Fprintf(stderr, "pfm chat kill: %v\n", err)
			return 1
		}
		if !found {
			fmt.Fprintf(stdout, "%s\tnot-found\n", target)
			fmt.Fprintf(stderr, "pfm chat: no chat named %q\n", target)
			return codeUnknownChat
		}
		id = chat.ID
	}
	killArgs := make([]string, 0, 2)
	if *exit {
		killArgs = append(killArgs, "--exit")
	}
	killArgs = append(killArgs, id)
	var killOut, killErr strings.Builder
	code = runKill(killArgs, &killOut, &killErr, runtimes...)
	_, _ = io.WriteString(stdout, killOut.String())
	_, _ = io.WriteString(stderr, killErr.String())
	if code != 0 && strings.Contains(killErr.String(), "not indexed") {
		return codeUnknownChat
	}
	return code
}

func runResolvedChatKill(
	chat headless.Chat,
	exit bool,
	stdout, stderr io.Writer,
	runtimes ...commandRuntime,
) int {
	database, manager, code := openKillManager(stderr, runtimes...)
	if code != 0 {
		return code
	}
	defer database.Close()
	target, err := manager.Kill(context.Background(), kill.Request{
		ID:          chat.ID,
		Engine:      chat.Engine,
		RolloutPath: chat.Path,
		SocketName:  chat.Socket,
		PaneID:      chat.Pane,
		Exit:        exit,
	})
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat kill: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "killed %s\n", target.ID)
	return 0
}

func runChatUnkill(args []string, stdout, stderr io.Writer, runtimes ...commandRuntime) int {
	flags := newFlagSet("chat unkill", "usage: pfm chat unkill <target>", stderr)
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 1 {
		flags.Usage()
		return 2
	}
	target := flags.Arg(0)
	if !chatUUIDPattern.MatchString(target) {
		chat, found, err := resolveChat(context.Background(), target, io.Discard, runtimes...)
		if err != nil {
			fmt.Fprintf(stderr, "pfm chat unkill: %v\n", err)
			return 1
		}
		if !found {
			fmt.Fprintf(stdout, "%s\tnot-found\n", target)
			fmt.Fprintf(stderr, "pfm chat: no chat named %q\n", target)
			return codeUnknownChat
		}
		target = chat.ID
	}
	return runUnkill([]string{target}, stdout, stderr, runtimes...)
}

func runChatResolve(args []string, stdout, stderr io.Writer, runtimes ...commandRuntime) int {
	flags := newFlagSet("chat resolve", "usage: pfm chat resolve <target>", stderr)
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 1 {
		flags.Usage()
		return 2
	}
	chat, code := headlessTarget(context.Background(), flags.Arg(0), stdout, stderr, false, runtimes...)
	if code != 0 {
		return code
	}
	fmt.Fprintf(stdout, "%s\t%s\t%s\n", chat.Socket, chat.Session, chat.ID)
	return 0
}

func runChatCapture(args []string, stdout, stderr io.Writer, runtimes ...commandRuntime) int {
	flags := newFlagSet("chat capture", "usage: pfm chat capture <target>", stderr)
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 1 {
		flags.Usage()
		return 2
	}
	chat, code := headlessTarget(context.Background(), flags.Arg(0), stdout, stderr, false, runtimes...)
	if code != 0 {
		return code
	}
	if !chat.Live {
		fmt.Fprintf(stderr, "pfm chat capture: %q is not running\n", chat.Name)
		return codeDeadChat
	}
	socketPath, err := chatSocketPath(chat.Socket)
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat capture: %v\n", err)
		return 1
	}
	target := chat.Pane
	if target == "" {
		target = chat.Session
	}
	if target == "" {
		target = chat.Socket
	}
	capture, err := (inject.CommandTmux{}).Capture(
		context.Background(), socketPath, target, true, inject.FullScrollback,
	)
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat capture: %v\n", err)
		return codeDeadChat
	}
	fmt.Fprint(stdout, capture)
	if !strings.HasSuffix(capture, "\n") {
		fmt.Fprintln(stdout)
	}
	return 0
}

func runChatRecover(args []string, stdout, stderr io.Writer, runtimes ...commandRuntime) int {
	flags := newFlagSet("chat recover", "usage: pfm chat recover <thread-id|rollout-path>", stderr)
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 1 {
		flags.Usage()
		return 2
	}
	_, err := optionalCommandRuntime(runtimes)
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat recover: load config: %v\n", err)
		return 1
	}
	resolved, err := paths.Resolve()
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat recover: resolve source paths: %v\n", err)
		return 1
	}
	codexRoot := firstRoot(resolved.Roots[pfmengine.Codex])
	result, err := recovery.Run(context.Background(), codexRoot, flags.Arg(0))
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat recover: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "messages=%d carried=%d malformed=%d\n", result.Messages, result.Carried, result.Malformed)
	fmt.Fprintf(stdout, "recovered thread %s\n", result.ThreadID)
	fmt.Fprintf(stdout, "  %s\n  %s\n  %s\n", filepath.Join(codexRoot, "recovered-"+result.ThreadID, "brief.md"), filepath.Join(codexRoot, "recovered-"+result.ThreadID, "compaction-memory.md"), filepath.Join(codexRoot, "recovered-"+result.ThreadID, "transcript.md"))
	fmt.Fprintf(stdout, "\nBrief a replacement seat with:\n  pfm chat inject <socket> 'RECOVERY: read %s, then compaction-memory.md, then the end of transcript.md.'\n", filepath.Join(codexRoot, "recovered-"+result.ThreadID, "brief.md"))
	return 0
}

func runChatName(args []string, stdout, stderr io.Writer, runtimes ...commandRuntime) int {
	deliver := func(ctx context.Context, chat headless.Chat, name string) (int, string, error) {
		return deliverChatNameWithRuntime(ctx, chat, name, runtimes...)
	}
	return runChatNameWith(args, stdout, stderr, deliver, runtimes...)
}

type chatNameDelivery func(
	context.Context,
	headless.Chat,
	string,
) (int, string, error)

func deliverChatName(
	ctx context.Context,
	chat headless.Chat,
	name string,
) (int, string, error) {
	return deliverChatNameWithRuntime(ctx, chat, name)
}

func deliverChatNameWithRuntime(
	ctx context.Context,
	chat headless.Chat,
	name string,
	runtimes ...commandRuntime,
) (int, string, error) {
	engine, err := newInjectEngine(runtimes...)
	if err != nil {
		return 1, "", err
	}
	// Inject through the live tmux namespace, not the transcript UUID. A newly
	// spawned Codex seat can be addressable before its rollout has been indexed;
	// converting that seat to its UUID makes the subsequent /rename miss it.
	target := chatNameInjectTarget(chat)
	result, err := engine.Inject(ctx, inject.Request{
		Target:  target,
		Message: "/rename " + name,
	})
	return result.Code, result.Message, err
}

func chatNameInjectTarget(chat headless.Chat) string {
	if chat.Session != "" {
		return chat.Session
	}
	return chat.Socket
}

func runChatNameWith(
	args []string,
	stdout, stderr io.Writer,
	deliver chatNameDelivery,
	runtimes ...commandRuntime,
) int {
	flags := newFlagSet("chat name", "usage: pfm chat name <target> <name>", stderr)
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() < 2 {
		flags.Usage()
		return 2
	}
	name := strings.TrimSpace(strings.Join(flags.Args()[1:], " "))
	if name == "" || strings.ContainsAny(name, "\r\n\x00") {
		fmt.Fprintln(stderr, "pfm chat name: name must be one non-empty line")
		return 2
	}
	chat, code := headlessTarget(context.Background(), flags.Arg(0), stdout, stderr, false, runtimes...)
	if code != 0 {
		return code
	}
	if !chat.Live {
		fmt.Fprintf(stderr, "pfm chat name: %q is not running\n", chat.Name)
		return codeDeadChat
	}
	code = applyChatName(context.Background(), chat, name, deliver, stderr)
	if code != 0 {
		return code
	}
	fmt.Fprintf(stdout, "named %s -> %s\n", chat.ID, name)
	return 0
}

func applyChatName(
	ctx context.Context,
	chat headless.Chat,
	name string,
	deliver chatNameDelivery,
	stderr io.Writer,
) int {
	resultCode, resultMessage, err := deliver(ctx, chat, name)
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat name: %v\n", err)
		return 1
	}
	if resultCode != 0 {
		fmt.Fprintf(stderr, "pfm chat name: %s\n", resultMessage)
		return resultCode
	}
	target := chat.Pane
	if target == "" {
		target = chat.Session
	}
	if err := renameChatWindow(ctx, chat.Socket, target, name); err != nil {
		fmt.Fprintf(stderr, "pfm chat name: rename delivered but window convergence failed: %v\n", err)
		return 1
	}
	return 0
}

func runChatEnd(args []string, stdout, stderr io.Writer, runtimes ...commandRuntime) int {
	flags := newFlagSet("chat end", "usage: pfm chat end <target>", stderr)
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 1 {
		flags.Usage()
		return 2
	}
	chat, code := headlessTarget(context.Background(), flags.Arg(0), stdout, stderr, false, runtimes...)
	if code != 0 {
		return code
	}
	if !chat.Live {
		fmt.Fprintf(stderr, "pfm chat end: %q is not running\n", chat.Name)
		return codeDeadChat
	}
	socketPath, err := chatSocketPath(chat.Socket)
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat end: %v\n", err)
		return 1
	}
	command := exec.Command(deps.Executable("tmux"), "-S", socketPath, "kill-server")
	if output, err := command.CombinedOutput(); err != nil {
		fmt.Fprintf(stderr, "pfm chat end: %v: %s\n", err, strings.TrimSpace(string(output)))
		return 1
	}
	fmt.Fprintf(stdout, "ended %s\n", chat.ID)
	return 0
}

func renameChatWindow(ctx context.Context, socket, target, name string) error {
	socketPath, err := chatSocketPath(socket)
	if err != nil {
		return err
	}
	if target == "" {
		target = socket
	}
	command := exec.CommandContext(ctx, deps.Executable("tmux"), "-S", socketPath, "rename-window", "-t", target, name)
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("tmux rename-window: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func chatSocketPath(socket string) (string, error) {
	if filepath.IsAbs(socket) {
		return socket, nil
	}
	resolved, err := paths.Resolve()
	if err != nil {
		return "", err
	}
	return filepath.Join(resolved.TmuxDir, socket), nil
}

func runChatSatellite(
	verb string,
	args []string,
	stdin io.Reader,
	stdout, stderr io.Writer,
	runtimes ...commandRuntime,
) int {
	switch verb {
	case "find":
		return runChatFind(args, stdout, stderr, runtimes...)
	case "save":
		return runChatSave(args, stdout, stderr, runtimes...)
	case "load":
		return runChatLoad(args, stdout, stderr)
	case "branch":
		return runChatBranch(args, stdout, stderr, runtimes...)
	case "ls":
		return runChatLS(args, stdout, stderr, runtimes...)
	case "history":
		return runChatHistory(args, stdout, stderr, runtimes...)
	default:
		fmt.Fprintf(stderr, "pfm chat: unsupported compatibility command %q\n", verb)
		return 2
	}
}
