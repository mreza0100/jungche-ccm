package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"hostops/pfm/internal/hide"
)

// bbPrompt is the whole prompt that closes a chat. The match is the ENTIRE
// trimmed prompt and nothing else: "/bb doesn't work!" is a sentence ABOUT the
// command and has to reach the model like any other sentence, so a substring
// match is never used here.
const bbPrompt = "/bb"

// bbBlockPrompt is the exit code a UserPromptSubmit hook returns to block the
// prompt it just read. Blocking is the point: `/bb` closes the chat
// mechanically, so no turn is taken, it costs nothing, and the model never
// gets the chance to reinterpret, defer, or "helpfully" rephrase it.
const bbBlockPrompt = 2

// runBB is the UserPromptSubmit hook. It reads the harness's JSON payload on
// stdin and, for a bare /bb, hides and closes this chat itself.
//
// Everything outside that one intentional block fails OPEN: a hook that errors
// on a prompt it does not own would EAT that prompt, and a lost prompt is
// worse than a chat left open. So an unreadable stdin, a payload that is not
// JSON, and any prompt that is not exactly /bb all exit 0 in silence.
func runBB(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := newFlagSet("chat bb", "usage: pfm chat bb < hook-payload.json", stderr)
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
		Prompt string `json:"prompt"`
	}
	if err := json.Unmarshal(payload, &hook); err != nil {
		return 0
	}
	if strings.TrimSpace(hook.Prompt) != bbPrompt {
		return 0
	}

	// hide --self identifies THIS chat from the TMUX and TMUX_PANE the hook
	// inherits, and --exit hands the closing choreography to a detached
	// finisher, so the hook returns at once instead of waiting for a chat to
	// die around it. It is the same one implementation the picker's ⌃X and
	// the `hide` subcommand use (K3).
	database, manager, code := openHideManager(stderr)
	if code != 0 {
		fmt.Fprintln(stderr, "bb: the chat store is unreachable — chat left open")
		return bbBlockPrompt
	}
	defer database.Close()
	if _, err := manager.Hide(context.Background(), hide.Request{
		Self:        true,
		Exit:        true,
		Environment: hide.Environment(),
	}); err != nil {
		fmt.Fprintf(stderr, "bb: %v — chat left open\n", err)
		return bbBlockPrompt
	}
	// stderr is what the operator sees on a blocked prompt; stdout on
	// UserPromptSubmit is injected into the model's context, and this chat is
	// closing, so it says nothing there.
	fmt.Fprintln(stderr, "bye-bye — hidden from cc-ls, closing")
	return bbBlockPrompt
}
