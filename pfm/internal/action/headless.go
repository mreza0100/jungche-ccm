package action

import (
	"errors"
	"fmt"
	pfmengine "hostops/pfm/internal/engine"
	"strings"

	pfmconfig "hostops/pfm/internal/config"
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
	Engine         pfmengine.ID
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

var codexEfforts = map[string]struct{}{
	"minimal": {}, "low": {}, "medium": {}, "high": {}, "xhigh": {}, "max": {}, "ultra": {},
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

// HeadlessForkRequest is one detached fork of an existing engine session.
// SessionID is always explicit: a shared MCP daemon must never inherit the
// identity of whichever process launched it.
type HeadlessForkRequest struct {
	Engine         pfmengine.ID
	SessionID      string
	Name           string
	CWD            string
	Home           string
	PrimaryAccount int
	Cache1H        bool
	Model          string
	Config         pfmconfig.Config
}

// HeadlessFork synthesizes the command for a real Claude or Codex fork.
func HeadlessFork(request HeadlessForkRequest) (HeadlessPlan, error) {
	if request.SessionID == "" || strings.ContainsAny(request.SessionID, "\r\n\x00") {
		return HeadlessPlan{}, errors.New("a fork requires one safe session id")
	}
	if err := validateHeadlessRequest(HeadlessRequest{
		Name: request.Name, CWD: request.CWD, Home: request.Home,
	}); err != nil {
		return HeadlessPlan{}, err
	}
	machine := normalizedMachineConfig(request.Config, request.Home)
	switch request.Engine {
	case pfmengine.Claude:
		if _, found := machine.Account(request.PrimaryAccount); !found {
			return HeadlessPlan{}, fmt.Errorf("Claude account %d is not in the configured roster", request.PrimaryAccount)
		}
		arguments := []string{"--resume", request.SessionID, "--fork-session"}
		if request.Model != "" {
			arguments = append(arguments, "--model", request.Model)
		}
		arguments = append(arguments, "--name", request.Name)
		return HeadlessPlan{
			Run:                 claudeCommandWith(headlessHygiene, request.Home, request.PrimaryAccount, request.Cache1H, machine, arguments...),
			PromptOnCommandLine: true,
		}, nil
	case pfmengine.Codex:
		if _, found := machine.CodexAccountByID(request.PrimaryAccount); !found {
			return HeadlessPlan{}, fmt.Errorf("Codex account %d is not in the configured roster", request.PrimaryAccount)
		}
		arguments := make([]string, 0, 3)
		if request.Model != "" {
			arguments = append(arguments, "--model", request.Model)
		}
		arguments = append(arguments, "fork", request.SessionID)
		return HeadlessPlan{
			Run:                 codexCommandWithAccount(headlessHygiene, machine, request.PrimaryAccount, arguments...),
			PromptOnCommandLine: true,
		}, nil
	default:
		return HeadlessPlan{}, fmt.Errorf("chat branch supports Claude and Codex, not %q", request.Engine)
	}
}

// HeadlessRun synthesizes the engine command for a detached chat. It performs
// no I/O and no tmux work: the caller owns the session.
func HeadlessRun(request HeadlessRequest) (HeadlessPlan, error) {
	planner, err := PlannerFor(request.Engine)
	if err != nil {
		return HeadlessPlan{}, err
	}
	return planner.Plan(request)
}

func validateHeadlessRequest(request HeadlessRequest) error {
	if request.Name == "" {
		return errors.New("a headless chat requires a name")
	}
	if strings.ContainsAny(request.Name, "\n\r") {
		return errors.New("a chat name cannot contain newlines")
	}
	if request.CWD == "" {
		return errors.New("a headless chat requires a project directory")
	}
	if hasNUL(request.Name, request.CWD, request.Prompt, request.Home) {
		return errors.New("action values cannot contain NUL")
	}
	return nil
}

// PlanClaude contains the command synthesis used by Claude's planner.
func PlanClaude(request HeadlessRequest) (HeadlessPlan, error) {
	if err := validateHeadlessRequest(request); err != nil {
		return HeadlessPlan{}, err
	}
	machine := normalizedMachineConfig(request.Config, request.Home)
	if _, found := machine.Account(request.PrimaryAccount); !found {
		return HeadlessPlan{}, fmt.Errorf("primary account %d is not in the configured roster", request.PrimaryAccount)
	}
	if request.Effort != "" {
		if _, known := claudeEfforts[strings.ToLower(request.Effort)]; !known {
			return HeadlessPlan{}, fmt.Errorf("unknown Claude effort %q (want low, medium, high, xhigh or max)", request.Effort)
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
		Run:                 claudeCommandWith(headlessHygiene, request.Home, request.PrimaryAccount, request.Cache1H, machine, arguments...),
		PromptOnCommandLine: true,
	}, nil
}

// PlanCodex contains the command synthesis used by Codex's planner.
func PlanCodex(request HeadlessRequest) (HeadlessPlan, error) {
	if err := validateHeadlessRequest(request); err != nil {
		return HeadlessPlan{}, err
	}
	machine := normalizedMachineConfig(request.Config, request.Home)
	if _, found := machine.CodexAccountByID(request.PrimaryAccount); !found {
		return HeadlessPlan{}, fmt.Errorf("Codex account %d is not in the configured roster", request.PrimaryAccount)
	}
	if request.Effort != "" {
		if _, known := codexEfforts[strings.ToLower(request.Effort)]; !known {
			return HeadlessPlan{}, fmt.Errorf("unknown Codex effort %q (want minimal, low, medium, high, xhigh, max or ultra)", request.Effort)
		}
	}
	arguments := make([]string, 0, 4)
	if request.Model != "" {
		arguments = append(arguments, "--model", request.Model)
	}
	if request.Effort != "" {
		arguments = append(arguments, "-c", `model_reasoning_effort="`+strings.ToLower(request.Effort)+`"`)
	}
	return HeadlessPlan{Run: codexCommandWithAccount(headlessHygiene, machine, request.PrimaryAccount, arguments...)}, nil
}
