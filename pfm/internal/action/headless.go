package action

import (
	"errors"
	"fmt"
	"strings"

	pfmconfig "hostops/pfm/internal/config"
	"hostops/pfm/internal/store"
)

// HeadlessWidth and HeadlessHeight are the geometry a detached chat is born
// with. A headless pane has no client to size it, and tmux's 80x24 default
// truncates the statusline every label lookup reads (chat.sh's cross-socket
// scan, pfm's own crumbs) — a chat nobody can address is not a teammate.
const (
	HeadlessWidth  = 220
	HeadlessHeight = 50
)

// headlessHygiene adds CODEX_THREAD_ID to the launch-environment strip. The
// interactive routes are eval'd by the user's own shell, which never carries
// one; `run` is called from inside other chats and from scripts, where an
// inherited thread id would make the new chat answer `whoami` — and therefore
// `kill --self` — with its PARENT's identity.
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
	Config         pfmconfig.Config
	// Model and Effort pin the seat's tier at birth. A seat that inherits
	// whatever the account config holds that day is a seat whose cost and
	// quality nobody chose.
	Model  string
	Effort string
}

// claudeEfforts is the roster claude's own --help states. An unknown value is
// refused here rather than at the engine, where it would surface as a chat
// that died at birth for no stated reason.
var claudeEfforts = map[string]struct{}{
	"low": {}, "medium": {}, "high": {}, "xhigh": {}, "max": {},
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
		machine := normalizedMachineConfig(request.Config, request.Home)
		if _, found := machine.Account(request.PrimaryAccount); !found {
			return HeadlessPlan{}, fmt.Errorf(
				"primary account %d is not in the configured roster",
				request.PrimaryAccount,
			)
		}
		if request.Effort != "" {
			if _, known := claudeEfforts[strings.ToLower(request.Effort)]; !known {
				return HeadlessPlan{}, fmt.Errorf(
					"unknown Claude effort %q (want low, medium, high, xhigh or max)",
					request.Effort,
				)
			}
		}
		arguments := []string{"--name", request.Name}
		if request.Model != "" {
			arguments = append(arguments, "--model", request.Model)
		}
		if request.Effort != "" {
			arguments = append(arguments, "--effort", strings.ToLower(request.Effort))
		}
		if request.Prompt != "" {
			arguments = append(arguments, request.Prompt)
		}
		return HeadlessPlan{
			Run: claudeCommandWith(
				headlessHygiene,
				request.Home,
				request.PrimaryAccount,
				request.Cache1H,
				machine,
				arguments...,
			),
			PromptOnCommandLine: true,
		}, nil
	case store.CodexEngine:
		machine := normalizedMachineConfig(request.Config, request.Home)
		// No name and no prompt on the command line: Codex has no launch flag
		// for a thread name (codex 0.147 --help), so the name is set through
		// the TUI's own rename, and a prompt given here would have the model
		// working before that can land.
		codexArguments := make([]string, 0, 4)
		if request.Model != "" {
			codexArguments = append(codexArguments, "--model", request.Model)
		}
		if request.Effort != "" {
			// Codex takes reasoning effort as a config override, not a flag
			// (codex --help: -c key=value); the value is quoted as TOML.
			codexArguments = append(
				codexArguments,
				"-c",
				`model_reasoning_effort="`+strings.ToLower(request.Effort)+`"`,
			)
		}
		return HeadlessPlan{
			Run: codexCommandWithAccount(headlessHygiene, machine, request.PrimaryAccount, codexArguments...),
		}, nil
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
