package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"hostops/pfm/internal/headless"
	"hostops/pfm/internal/inject"
	"hostops/pfm/internal/paths"
)

var chatUUIDPattern = regexp.MustCompile(
	`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`,
)

func runChatRead(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		if info, err := os.Stat(args[0]); err == nil && info.Mode().IsRegular() &&
			filepath.Ext(args[0]) != ".jsonl" {
			return runChatSatellite("read", args, stdin, stdout, stderr)
		}
	}
	return runHeadlessTranscript(args, stdout, stderr)
}

func runChatOpen(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("chat open", "usage: pfm chat open <target>", stderr)
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 1 {
		flags.Usage()
		return 2
	}
	chat, code := headlessTarget(context.Background(), flags.Arg(0), stdout, stderr, false)
	if code != 0 {
		return code
	}
	return openID(context.Background(), chat.ID, stdout, stderr)
}

func runChatHide(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("chat hide", "usage: pfm chat hide <target> [--exit]", stderr)
	exit := flags.Bool("exit", false, "gracefully close after hiding")
	targets, code, ok := parseFlagsAnywhere(flags, args)
	if !ok {
		return code
	}
	if len(targets) != 1 {
		flags.Usage()
		return 2
	}
	if targets[0] == "self" || targets[0] == "me" {
		hideArgs := []string{"--self"}
		if *exit {
			hideArgs = append(hideArgs, "--exit")
		}
		return runHide(hideArgs, stdout, stderr)
	}
	target := targets[0]
	id := target
	if !chatUUIDPattern.MatchString(target) {
		chat, found, err := resolveChat(context.Background(), target, io.Discard)
		if err != nil {
			fmt.Fprintf(stderr, "pfm chat hide: %v\n", err)
			return 1
		}
		if !found {
			fmt.Fprintf(stdout, "%s\tnot-found\n", target)
			fmt.Fprintf(stderr, "pfm chat: no chat named %q\n", target)
			return codeUnknownChat
		}
		id = chat.ID
	}
	hideArgs := make([]string, 0, 2)
	if *exit {
		hideArgs = append(hideArgs, "--exit")
	}
	hideArgs = append(hideArgs, id)
	var hideOut, hideErr strings.Builder
	code = runHide(hideArgs, &hideOut, &hideErr)
	_, _ = io.WriteString(stdout, hideOut.String())
	_, _ = io.WriteString(stderr, hideErr.String())
	if code != 0 && strings.Contains(hideErr.String(), "not indexed") {
		return codeUnknownChat
	}
	return code
}

func runChatUnhide(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("chat unhide", "usage: pfm chat unhide <target>", stderr)
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 1 {
		flags.Usage()
		return 2
	}
	target := flags.Arg(0)
	if !chatUUIDPattern.MatchString(target) {
		chat, found, err := resolveChat(context.Background(), target, io.Discard)
		if err != nil {
			fmt.Fprintf(stderr, "pfm chat unhide: %v\n", err)
			return 1
		}
		if !found {
			fmt.Fprintf(stdout, "%s\tnot-found\n", target)
			fmt.Fprintf(stderr, "pfm chat: no chat named %q\n", target)
			return codeUnknownChat
		}
		target = chat.ID
	}
	return runUnhide([]string{target}, stdout, stderr)
}

func runChatResolve(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("chat resolve", "usage: pfm chat resolve <target>", stderr)
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 1 {
		flags.Usage()
		return 2
	}
	chat, code := headlessTarget(context.Background(), flags.Arg(0), stdout, stderr, false)
	if code != 0 {
		return code
	}
	fmt.Fprintf(stdout, "%s\t%s\t%s\n", chat.Socket, chat.Session, chat.ID)
	return 0
}

func runChatCapture(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("chat capture", "usage: pfm chat capture <target>", stderr)
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 1 {
		flags.Usage()
		return 2
	}
	chat, code := headlessTarget(context.Background(), flags.Arg(0), stdout, stderr, false)
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

func runChatName(args []string, stdout, stderr io.Writer) int {
	return runChatNameWith(args, stdout, stderr, deliverChatName)
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
	engine, err := inject.New(inject.Dependencies{})
	if err != nil {
		return 1, "", err
	}
	target := chat.ID
	if target == "" {
		target = chat.Socket
	}
	result, err := engine.Inject(ctx, inject.Request{
		Target:  target,
		Message: "/rename " + name,
	})
	return result.Code, result.Message, err
}

func runChatNameWith(
	args []string,
	stdout, stderr io.Writer,
	deliver chatNameDelivery,
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
	chat, code := headlessTarget(context.Background(), flags.Arg(0), stdout, stderr, false)
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

func runChatEnd(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("chat end", "usage: pfm chat end <target>", stderr)
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 1 {
		flags.Usage()
		return 2
	}
	chat, code := headlessTarget(context.Background(), flags.Arg(0), stdout, stderr, false)
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
	command := exec.Command("tmux", "-S", socketPath, "kill-server")
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
	command := exec.CommandContext(ctx, "tmux", "-S", socketPath, "rename-window", "-t", target, name)
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

func runChatGroup(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "hook" {
		// A UserPromptSubmit receiver must never block the prompt it enriches.
		// The shell backend already follows this contract; keep it even when
		// locating or starting that transitional backend fails.
		_ = runChatScript("group.sh", args, stdin, stdout, io.Discard)
		return 0
	}
	return runChatScript("group.sh", args, stdin, stdout, stderr)
}

func runChatSatellite(
	verb string,
	args []string,
	stdin io.Reader,
	stdout, stderr io.Writer,
) int {
	if verb == "history" {
		return runChatScript("history.sh", args, stdin, stdout, stderr)
	}
	return runChatScript("chat-ops.sh", append([]string{verb}, args...), stdin, stdout, stderr)
}

func runChatScript(
	name string,
	args []string,
	stdin io.Reader,
	stdout, stderr io.Writer,
) int {
	path, err := locateChatScript(name)
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat: %v\n", err)
		return 1
	}
	command := exec.Command("bash", append([]string{path}, args...)...)
	command.Stdin = stdin
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return exit.ExitCode()
		}
		fmt.Fprintf(stderr, "pfm chat: run %s: %v\n", name, err)
		return 1
	}
	return 0
}

func locateChatScript(name string) (string, error) {
	candidates := make([]string, 0, 6)
	if home := os.Getenv("PFM_HOME"); home != "" {
		candidates = append(candidates, filepath.Join(home, "chat", name))
	}
	if resolved, err := paths.Resolve(); err == nil {
		candidates = append(candidates,
			filepath.Join(resolved.Home, ".claude", "commands", "chat", name),
			filepath.Join(resolved.Home, ".professor", "blueprint", "templates", "host-swap", "chat", name),
		)
	}
	candidates = append(candidates,
		filepath.Join("blueprint", "templates", "host-swap", "chat", name),
		filepath.Join("..", "blueprint", "templates", "host-swap", "chat", name),
	)
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && info.Mode().IsRegular() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("locate %s", name)
}
