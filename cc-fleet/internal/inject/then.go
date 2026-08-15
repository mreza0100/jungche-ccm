package inject

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// DeliverThen is the waiter half of chat.sh's __then subcommand
// (chat.sh:1048-1085). It rides out the primary turn — typically a /compact
// compaction — and delivers the FIRST steer once the pane has been idle long
// enough to hold, passing the remainder along so the chain re-arms itself one
// confirmed delivery at a time: steer N+1 always waits out steer N's whole
// turn. It runs in a DETACHED process because for a self-inject the waiter
// waits on the very turn that spawned it.
func (engine *Engine) DeliverThen(
	ctx context.Context,
	socketPath, target string,
	steers []string,
) (Result, error) {
	if target == "" || len(steers) == 0 || steers[0] == "" {
		return refused(
			1,
			"then waiter needs a socket, a target, and at least one steer",
		), nil
	}
	if socketPath != "" {
		// The socket rides the environment so a pane-id target resolves on the
		// server that actually holds it (chat.sh:1082).
		if err := os.Setenv("CHAT_INJECT_SOCKET", socketPath); err != nil {
			return Result{}, err
		}
	}
	engine.waitForSettledTurn(ctx, socketPath, target)
	return engine.Inject(ctx, Request{
		Target:  target,
		Message: steers[0],
		Then:    steers[1:],
		Chain:   true,
	})
}

// waitForSettledTurn reproduces chat.sh:1062-1077: let the primary take hold,
// wait (bounded) for the pane to go busy, then wait for idle to hold STEADY —
// compaction shows brief stalls that would otherwise read as done — and settle.
func (engine *Engine) waitForSettledTurn(
	ctx context.Context,
	socketPath, target string,
) {
	sleepContext(ctx, engine.options.ThenMin)
	for attempt := 0; attempt < engine.options.ThenBusyTries; attempt++ {
		if engine.paneBusy(ctx, socketPath, target) {
			break
		}
		sleepContext(ctx, engine.options.Poll)
	}
	stable := 0
	for attempt := 0; attempt < engine.options.ThenIdleTries; attempt++ {
		if engine.paneBusy(ctx, socketPath, target) {
			stable = 0
		} else {
			stable++
		}
		if stable >= engine.options.ThenIdleStable {
			break
		}
		sleepContext(ctx, engine.options.ThenIdlePoll)
	}
	sleepContext(ctx, engine.options.ThenSettle)
}

func (engine *Engine) paneBusy(
	ctx context.Context,
	socketPath, target string,
) bool {
	capture, err := engine.tmux.Capture(ctx, socketPath, target, false, 0)
	if err != nil {
		// An unreadable pane is not busy; the delivery attempt reports the
		// dead pane truthfully instead of spinning here.
		return false
	}
	return IsBusy(capture)
}

// CommandThenSpawner starts this binary's own waiter under a detached process,
// mirroring chat.sh's `$(cc_detach) bash "$0" __then …` (chat.sh:946-948).
type CommandThenSpawner struct {
	Executable string
	Setsid     string
}

// Spawn launches the detached waiter and returns as soon as it is running.
func (spawner CommandThenSpawner) Spawn(
	ctx context.Context,
	request SteerSpawn,
) error {
	executable := spawner.Executable
	if executable == "" {
		var err error
		executable, err = os.Executable()
		if err != nil {
			return fmt.Errorf("resolve cc-fleet executable: %w", err)
		}
	}
	setsid := spawner.Setsid
	if setsid == "" {
		setsid = "setsid"
	}
	arguments := []string{
		"-f",
		executable,
		"internal",
		"then",
		"--socket",
		request.SocketPath,
		"--target",
		request.Target,
	}
	for _, steer := range request.Steers {
		arguments = append(arguments, "--steer", steer)
	}
	command := exec.CommandContext(ctx, setsid, arguments...)
	stated := []string{
		"CHAT_INJECT_SOCKET=" + request.SocketPath,
		"CHAT_THEN_CHAIN=1",
	}
	stated = append(stated, senderEnvironment(request.Sender)...)
	// Any inherited definition of these names is dropped first: os.Getenv
	// answers with the FIRST match, so appending over an inherited value would
	// leave the inherited one winning, and a chain hop would sign as whoever
	// spawned the hop before it.
	command.Env = append(withoutNames(os.Environ(), stated), stated...)
	null, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open null device for then waiter: %w", err)
	}
	defer null.Close()
	command.Stdin = null
	// A fresh chain truncates the log; a HOP appends — truncating on a hop
	// would wipe the chain's earlier hops while they are still being written.
	flags := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	if request.Append {
		flags = os.O_CREATE | os.O_WRONLY | os.O_APPEND
	}
	log, err := os.OpenFile(request.LogPath, flags, 0o600)
	if err != nil {
		command.Stdout = null
		command.Stderr = null
	} else {
		defer log.Close()
		command.Stdout = log
		command.Stderr = log
	}
	if err := command.Run(); err != nil {
		return fmt.Errorf("start detached then waiter: %w", err)
	}
	return nil
}

// senderEnvironment states the spawning chat's identity to the waiter. An
// empty field is left unset rather than exported empty, so a chat that could
// not derive its own identity hands down nothing and the waiter's message
// still says UNSIGNED out loud instead of signing as a nameless sender.
func senderEnvironment(sender Sender) []string {
	stated := make([]string, 0, 3)
	for _, pair := range [][2]string{
		{SenderSessionEnv, sender.Session},
		{SenderLabelEnv, sender.Label},
		{SenderIDEnv, sender.UUID},
	} {
		if pair[1] != "" {
			stated = append(stated, pair[0]+"="+pair[1])
		}
	}
	return stated
}

// withoutNames drops every definition of the names the given NAME=value pairs
// set, so the caller's own definitions are the only ones in the child.
func withoutNames(environment []string, pairs []string) []string {
	names := make(map[string]bool, len(pairs))
	for _, pair := range pairs {
		if name, _, ok := strings.Cut(pair, "="); ok {
			names[name] = true
		}
	}
	// The full set is dropped, not just the ones being set: a waiter that
	// inherits CHAT_SENDER_LABEL from an earlier hop while this hop states
	// only a session would sign with two different chats' fields.
	for _, name := range []string{
		SenderSessionEnv,
		SenderLabelEnv,
		SenderIDEnv,
	} {
		names[name] = true
	}
	kept := make([]string, 0, len(environment))
	for _, entry := range environment {
		name, _, _ := strings.Cut(entry, "=")
		if !names[name] {
			kept = append(kept, entry)
		}
	}
	return kept
}
