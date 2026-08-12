package action

import (
	"errors"
	"fmt"
	"strings"

	"hostops/cc-fleet/internal/store"
)

// HeadlessWidth and HeadlessHeight are the geometry a detached chat is born
// with. A headless pane has no client to size it, and tmux's 80x24 default
// truncates the statusline every label lookup reads (chat.sh's cross-socket
// scan, cc-fleet's own crumbs) — a chat nobody can address is not a teammate.
const (
	HeadlessWidth  = 220
	HeadlessHeight = 50
)

// headlessHygiene adds CODEX_THREAD_ID to the launch-environment strip. The
// interactive routes are eval'd by the user's own shell, which never carries
// one; `run` is called from inside other chats and from scripts, where an
// inherited thread id would make the new chat answer `whoami` — and therefore
// `hide --self` — with its PARENT's identity.
const headlessHygiene = hygiene + " -u CODEX_THREAD_ID"

// HeadlessRequest is one detached, named chat to start.
type HeadlessRequest struct {
	Engine         string
	Name           string
	CWD            string
	Prompt         string
	Home           string
	PrimaryAccount int
	Cache1H        bool
}

// HeadlessPlan is the pure result: the command the tmux session runs, and
// whether the prompt travelled on it.
type HeadlessPlan struct {
	Run string
	// PromptOnCommandLine is false when the prompt must be typed into the
	// running TUI instead — Codex is named through its own rename UI, and a
	// prompt on the command line would start a turn before that can happen.
	PromptOnCommandLine bool
}

// HeadlessRun synthesizes the engine command for a detached chat. It performs
// no I/O and no tmux work: the caller owns the session.
func HeadlessRun(request HeadlessRequest) (HeadlessPlan, error) {
	if request.Name == "" {
		return HeadlessPlan{}, errors.New("a headless chat requires a name")
	}
	if strings.ContainsAny(request.Name, "\n\r") {
		return HeadlessPlan{}, errors.New("a chat name cannot contain newlines")
	}
	if request.CWD == "" {
		return HeadlessPlan{}, errors.New(
			"a headless chat requires a project directory",
		)
	}
	if hasNUL(request.Name, request.CWD, request.Prompt, request.Home) {
		return HeadlessPlan{}, errors.New("action values cannot contain NUL")
	}

	// Normalized here as well as at the CLI: a caller that hands this the
	// engine name a human typed must not silently fall through to the error
	// branch, and one name for the engine is the whole point of the mapping.
	engine, known := NormalizeEngine(request.Engine)
	if !known {
		return HeadlessPlan{}, fmt.Errorf(
			"unsupported engine %q (want %q or %q)",
			request.Engine,
			store.ClaudeEngine,
			store.CodexEngine,
		)
	}
	switch engine {
	case store.ClaudeEngine:
		if request.PrimaryAccount < 1 || request.PrimaryAccount > MaxAccount {
			return HeadlessPlan{}, fmt.Errorf(
				"primary account must be 1-%d, got %d",
				MaxAccount,
				request.PrimaryAccount,
			)
		}
		arguments := []string{"--name", request.Name}
		if request.Prompt != "" {
			arguments = append(arguments, request.Prompt)
		}
		return HeadlessPlan{
			Run: claudeCommandWith(
				headlessHygiene,
				request.Home,
				request.PrimaryAccount,
				request.Cache1H,
				arguments...,
			),
			PromptOnCommandLine: true,
		}, nil
	case store.CodexEngine:
		// No name and no prompt on the command line: Codex has no launch flag
		// for a thread name (codex 0.147 --help), so the name is set through
		// the TUI's own rename, and a prompt given here would have the model
		// working before that can land.
		return HeadlessPlan{Run: codexCommandWith(headlessHygiene)}, nil
	}
	return HeadlessPlan{}, fmt.Errorf("unsupported engine %q", request.Engine)
}

// NormalizeEngine maps the spellings a human types onto the two engine names
// the rest of the tree uses.
func NormalizeEngine(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "cc", "claude":
		return store.ClaudeEngine, true
	case "cx", "codex":
		return store.CodexEngine, true
	default:
		return "", false
	}
}
