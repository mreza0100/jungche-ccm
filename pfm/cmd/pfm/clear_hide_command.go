package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"hostops/pfm/internal/index"
)

// runClearHide is the fail-open Claude SessionEnd hook. It owns exactly
// reason=clear, refreshes a previously indexed fleet transcript before taking
// its prompt baseline, then records the ratcheted hide. Every unrelated or
// malformed payload returns 0 without output.
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
		SessionID string `json:"session_id"`
	}
	if json.Unmarshal(payload, &hook) != nil ||
		hook.Event != "SessionEnd" || hook.Reason != "clear" || hook.SessionID == "" {
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
