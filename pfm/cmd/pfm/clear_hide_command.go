package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"hostops/pfm/internal/index"
)

// runClearHide is the fail-open /clear hook for both engines. Claude supplies
// the completed id directly in SessionEnd(reason=clear). Codex supplies the
// replacement id later in SessionStart(source=clear), so pfm resolves the
// completed id from the same pane's previously observed binding. Every
// unrelated or malformed payload returns 0 without output.
func runClearHide(args []string, stdin io.Reader, stderr io.Writer) int {
	flags := newFlagSet(
		"internal clear-hide",
		"usage: pfm internal clear-hide < hook-payload.json",
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
		return runCodexClearHide(hook.Source, hook.SessionID, hook.CWD, stderr)
	}
	if hook.Event != "SessionEnd" || hook.Reason != "clear" {
		return 0
	}

	database, manager, code := openHideManager(stderr)
	if code != 0 {
		fmt.Fprintln(stderr, "pfm internal clear-hide: store unavailable (fail-open)")
		return 0
	}
	defer database.Close()
	ctx := context.Background()
	transcript, found, err := database.Transcript(ctx, hook.SessionID)
	if err != nil {
		fmt.Fprintf(stderr, "pfm internal clear-hide: resolve fleet session (fail-open): %v\n", err)
		return 0
	}
	if !found {
		return 0
	}
	indexer, err := index.New(database)
	if err != nil {
		fmt.Fprintf(stderr, "pfm internal clear-hide: prepare transcript refresh (fail-open): %v\n", err)
		return 0
	}
	if _, err := indexer.Run(ctx, index.Options{
		PriorityCWD: transcript.CWD, PriorityOnly: true,
	}); err != nil {
		fmt.Fprintf(stderr, "pfm internal clear-hide: refresh transcript baseline (fail-open): %v\n", err)
		return 0
	}
	if _, found, err := manager.HideCleared(ctx, hook.SessionID); err != nil {
		fmt.Fprintf(stderr, "pfm internal clear-hide: record hide (fail-open): %v\n", err)
	} else if !found {
		return 0
	}
	return 0
}

func runCodexClearHide(source, sessionID, cwd string, stderr io.Writer) int {
	tmux := strings.SplitN(os.Getenv("TMUX"), ",", 2)[0]
	socket := filepath.Base(tmux)
	pane := os.Getenv("TMUX_PANE")
	if tmux == "" || pane == "" {
		return 0
	}
	database, manager, code := openHideManager(stderr)
	if code != 0 {
		fmt.Fprintln(stderr, "pfm internal clear-hide: store unavailable (fail-open)")
		return 0
	}
	defer database.Close()
	ctx := context.Background()
	previous, found, err := manager.CodexPaneBinding(ctx, socket, pane)
	if err != nil {
		fmt.Fprintf(stderr, "pfm internal clear-hide: resolve Codex pane binding (fail-open): %v\n", err)
		return 0
	}
	if source == "clear" && found && previous != sessionID {
		priorityCWD := cwd
		if rollout, indexed, lookupErr := database.Rollout(ctx, previous); lookupErr != nil {
			fmt.Fprintf(stderr, "pfm internal clear-hide: resolve Codex thread (fail-open): %v\n", lookupErr)
			return 0
		} else if indexed && rollout.CWD != "" {
			priorityCWD = rollout.CWD
		}
		indexer, indexErr := index.New(database)
		if indexErr != nil {
			fmt.Fprintf(stderr, "pfm internal clear-hide: prepare Codex refresh (fail-open): %v\n", indexErr)
			return 0
		}
		if _, indexErr = indexer.Run(ctx, index.Options{
			PriorityCWD: priorityCWD,
		}); indexErr != nil {
			fmt.Fprintf(stderr, "pfm internal clear-hide: refresh Codex baseline (fail-open): %v\n", indexErr)
			return 0
		}
		if _, hidden, hideErr := manager.HideClearedCodex(ctx, previous); hideErr != nil {
			fmt.Fprintf(stderr, "pfm internal clear-hide: record Codex hide (fail-open): %v\n", hideErr)
			return 0
		} else if !hidden {
			fmt.Fprintln(stderr, "pfm internal clear-hide: previous Codex thread was not indexed (fail-open)")
		}
	}
	if err := manager.BindCodexPane(ctx, socket, pane, sessionID); err != nil {
		fmt.Fprintf(stderr, "pfm internal clear-hide: bind Codex pane (fail-open): %v\n", err)
	}
	return 0
}
