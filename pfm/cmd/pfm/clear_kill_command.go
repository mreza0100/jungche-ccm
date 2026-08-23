package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"hostops/pfm/internal/index"
)

// runClearKill is the fail-open /clear hook for both engines. Claude supplies
// the completed id directly in SessionEnd(reason=clear). Codex supplies the
// replacement id later in SessionStart(source=clear), so pfm resolves the
// completed id from the same pane's previously observed binding or the same
// Codex parent process outside tmux. The inherited CODEX_THREAD_ID remains a
// compatibility fallback for hook runners that preserve it. Every unrelated
// or malformed payload returns 0 without output.
func runClearKill(args []string, stdin io.Reader, stderr io.Writer, runtimes ...commandRuntime) int {
	flags := newFlagSet(
		"internal clear-kill",
		"usage: pfm internal clear-kill < hook-payload.json",
		stderr,
	)
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 {
		flags.Usage()
		return 2
	}
	payload, err := io.ReadAll(stdin)
	if err != nil {
		return 0
	}
	var hook struct {
		Event     string `json:"hook_event_name"`
		Reason    string `json:"reason"`
		Source    string `json:"source"`
		SessionID string `json:"session_id"`
		CWD       string `json:"cwd"`
	}
	if json.Unmarshal(payload, &hook) != nil || hook.SessionID == "" {
		return 0
	}
	if hook.Event == "SessionStart" &&
		(hook.Source == "startup" || hook.Source == "resume" || hook.Source == "clear") {
		return runCodexClearKill(
			hook.Source, hook.SessionID, hook.CWD, strconv.Itoa(os.Getppid()), stderr, runtimes...,
		)
	}
	if hook.Event != "SessionEnd" || hook.Reason != "clear" {
		return 0
	}

	database, manager, code := openKillManager(stderr, runtimes...)
	if code != 0 {
		fmt.Fprintln(stderr, "pfm internal clear-kill: store unavailable (fail-open)")
		return 0
	}
	defer database.Close()
	ctx := context.Background()
	transcript, found, err := database.Transcript(ctx, hook.SessionID)
	if err != nil {
		fmt.Fprintf(stderr, "pfm internal clear-kill: resolve fleet session (fail-open): %v\n", err)
		return 0
	}
	if !found {
		return 0
	}
	runtime, runtimeErr := optionalCommandRuntime(runtimes)
	if runtimeErr != nil {
		fmt.Fprintf(stderr, "pfm internal clear-kill: config unavailable (fail-open): %v\n", runtimeErr)
		return 0
	}
	indexer, err := index.NewWithRoots(database, runtime.Paths, runtime.Paths.Roots)
	if err != nil {
		fmt.Fprintf(stderr, "pfm internal clear-kill: prepare transcript refresh (fail-open): %v\n", err)
		return 0
	}
	if _, err := indexer.Run(ctx, index.Options{
		PriorityCWD: transcript.CWD, PriorityOnly: true,
	}); err != nil {
		fmt.Fprintf(stderr, "pfm internal clear-kill: refresh transcript baseline (fail-open): %v\n", err)
		return 0
	}
	if _, found, err := manager.KillCleared(ctx, hook.SessionID); err != nil {
		fmt.Fprintf(stderr, "pfm internal clear-kill: record kill (fail-open): %v\n", err)
	} else if !found {
		return 0
	}
	return 0
}

func runCodexClearKill(source, sessionID, cwd, parent string, stderr io.Writer, runtimes ...commandRuntime) int {
	tmux := strings.SplitN(os.Getenv("TMUX"), ",", 2)[0]
	socket := filepath.Base(tmux)
	pane := os.Getenv("TMUX_PANE")
	hasPane := tmux != "" && pane != ""
	hasParent := strings.TrimSpace(parent) != ""
	inheritedID := strings.TrimSpace(os.Getenv("CODEX_THREAD_ID"))
	if !hasPane && !hasParent && (source != "clear" || inheritedID == "" || inheritedID == sessionID) {
		return 0
	}
	database, manager, code := openKillManager(stderr, runtimes...)
	if code != 0 {
		fmt.Fprintln(stderr, "pfm internal clear-kill: store unavailable (fail-open)")
		return 0
	}
	defer database.Close()
	ctx := context.Background()
	var previous string
	found := false
	if hasPane {
		var err error
		previous, found, err = manager.CodexPaneBinding(ctx, socket, pane)
		if err != nil {
			fmt.Fprintf(stderr, "pfm internal clear-kill: resolve Codex pane binding (fail-open): %v\n", err)
			return 0
		}
	} else if hasParent {
		var err error
		previous, found, err = manager.CodexProcessBinding(ctx, parent)
		if err != nil {
			fmt.Fprintf(stderr, "pfm internal clear-kill: resolve Codex process binding (fail-open): %v\n", err)
			return 0
		}
	}
	if source == "clear" && !found && inheritedID != "" && inheritedID != sessionID {
		previous = inheritedID
		found = true
	}
	if source == "clear" && found && previous != sessionID {
		priorityCWD := cwd
		if rollout, indexed, lookupErr := database.Rollout(ctx, previous); lookupErr != nil {
			fmt.Fprintf(stderr, "pfm internal clear-kill: resolve Codex thread (fail-open): %v\n", lookupErr)
			return 0
		} else if indexed && rollout.CWD != "" {
			priorityCWD = rollout.CWD
		}
		runtime, runtimeErr := optionalCommandRuntime(runtimes)
		if runtimeErr != nil {
			fmt.Fprintf(stderr, "pfm internal clear-kill: config unavailable (fail-open): %v\n", runtimeErr)
			return 0
		}
		indexer, indexErr := index.NewWithRoots(database, runtime.Paths, runtime.Paths.Roots)
		if indexErr != nil {
			fmt.Fprintf(stderr, "pfm internal clear-kill: prepare Codex refresh (fail-open): %v\n", indexErr)
			return 0
		}
		if _, indexErr = indexer.Run(ctx, index.Options{
			PriorityCWD: priorityCWD,
		}); indexErr != nil {
			fmt.Fprintf(stderr, "pfm internal clear-kill: refresh Codex baseline (fail-open): %v\n", indexErr)
			return 0
		}
		if _, killed, killErr := manager.KillClearedCodex(ctx, previous); killErr != nil {
			fmt.Fprintf(stderr, "pfm internal clear-kill: record Codex kill (fail-open): %v\n", killErr)
			return 0
		} else if !killed {
			fmt.Fprintln(stderr, "pfm internal clear-kill: previous Codex thread was not indexed (fail-open)")
		}
	}
	if hasPane {
		if err := manager.BindCodexPane(ctx, socket, pane, sessionID); err != nil {
			fmt.Fprintf(stderr, "pfm internal clear-kill: bind Codex pane (fail-open): %v\n", err)
		}
	} else if hasParent {
		if err := manager.BindCodexProcess(ctx, parent, sessionID); err != nil {
			fmt.Fprintf(stderr, "pfm internal clear-kill: bind Codex process (fail-open): %v\n", err)
		}
	}
	return 0
}
