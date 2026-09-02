package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// reloadInterceptRun runs the reload FRONT in-process — the same
// runChatReloadWithRuntime a human's own `~/.local/bin/pfm chat reload` call
// would hit. A package var, not a direct call, so a test can swap in a fake
// front and assert on exactly the argument slice the hook derived from the
// human's typed prompt without spawning a worker or touching a live pane.
var reloadInterceptRun = runChatReloadWithRuntime

// runReloadIntercept is the UserPromptSubmit hook body for
// `pfm internal reload-intercept`. A prompt beginning with "/reload" is the
// human asking for the exact reboot the `/reload` command body would type into
// Bash — running it here, before the model ever sees the prompt, saves the
// whole turn the model would otherwise spend re-typing the identical call.
//
// Claude Code's UserPromptSubmit contract: exit 0 lets the prompt through
// (stdout is added as context); exit 2 BLOCKS the prompt, erases it, and
// shows stderr to the human. A matched "/reload …" prompt always exits 2 —
// the point is that it never reaches the model, whether the reload it ran
// succeeded or failed; the human reads the front's own result on stderr
// instead. Codex has no UserPromptSubmit hook, so a Codex seat's `/reload`
// still goes through the model and the command body — this hook is a
// Claude-only shortcut, not the only path.
func runReloadIntercept(stdin io.Reader, stderr io.Writer, runtime commandRuntime) int {
	var payload struct {
		Prompt string `json:"prompt"`
	}
	if err := json.NewDecoder(stdin).Decode(&payload); err != nil {
		fmt.Fprintf(stderr, "pfm internal reload-intercept: decode hook payload: %v\n", err)
		return 0
	}
	trimmed := strings.TrimSpace(payload.Prompt)
	if trimmed != "/reload" && !strings.HasPrefix(trimmed, "/reload ") {
		return 0
	}
	remainder := strings.TrimPrefix(trimmed, "/reload")
	words, err := splitReloadWords(remainder)
	if err != nil {
		fmt.Fprintf(stderr, "reload: %v\n", err)
		return 2
	}
	// The same buffer serves as both stdout and stderr so the front's success
	// line ("reload scheduled in place (log …)") and any validation error it
	// writes to stderr land in ONE combined transcript, in the order the
	// front itself produced them.
	var captured bytes.Buffer
	reloadInterceptRun(words, &captured, &captured, runtime)
	fmt.Fprintf(stderr, "reload: %s", captured.String())
	return 2
}

// splitReloadWords tokenizes a human-typed `/reload …` prompt with the same
// quoting rules splitGeneratedShellWords decodes for a generated command
// line: double and single quotes, a backslash escape recognized only inside
// double quotes, and an unterminated quote reported as an error rather than
// silently swallowed into the next word. A dedicated name keeps this call
// site self-documenting without a second parser to drift from the first.
func splitReloadWords(s string) ([]string, error) {
	return splitGeneratedShellWords(s)
}
