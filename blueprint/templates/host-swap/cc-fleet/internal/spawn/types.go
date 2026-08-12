// Package spawn starts one detached, named fleet chat and reports what
// actually happened to it.
package spawn

import (
	"context"
	"time"
)

// SessionSpec is one detached tmux session to create.
type SessionSpec struct {
	Socket  string
	Session string
	Window  string
	CWD     string
	Run     string
	Width   int
	Height  int
}

// Tmux is the whole effect surface of a spawn: create the session, read the
// pane, type into it.
type Tmux interface {
	NewSession(ctx context.Context, spec SessionSpec) error
	Capture(ctx context.Context, socket, target string) (string, error)
	SendLiteral(ctx context.Context, socket, target, text string) error
	SendKey(ctx context.Context, socket, target, key string) error
}

// Request is one headless chat to start. Run comes from action.HeadlessRun,
// which owns the launch ceremony; this package owns only the session and the
// keystrokes.
type Request struct {
	Engine  string
	Name    string
	Socket  string
	CWD     string
	Run     string
	Prompt  string
	Width   int
	Height  int
	Timings Timings
}

// Timings bound every wait. Tests set them to microseconds; the CLI leaves
// them zero and takes Defaults.
type Timings struct {
	Poll  time.Duration
	Boot  time.Duration
	Step  time.Duration
	Typed time.Duration
}

// Defaults are the live timings. Boot is generous because a cold Codex or
// Claude TUI on a loaded box takes several seconds before it draws anything,
// and a spawn that gives up early would report a healthy chat as dead.
func Defaults() Timings {
	return Timings{
		Poll:  250 * time.Millisecond,
		Boot:  30 * time.Second,
		Step:  15 * time.Second,
		Typed: 150 * time.Millisecond,
	}
}

func (timings Timings) orDefaults() Timings {
	defaults := Defaults()
	if timings.Poll <= 0 {
		timings.Poll = defaults.Poll
	}
	if timings.Boot <= 0 {
		timings.Boot = defaults.Boot
	}
	if timings.Step <= 0 {
		timings.Step = defaults.Step
	}
	if timings.Typed < 0 {
		timings.Typed = defaults.Typed
	}
	return timings
}

// Result is what the caller reports to the user: where the chat lives, whether
// its name landed, and every step that did not go to plan.
type Result struct {
	Socket   string
	Session  string
	Window   string
	Name     string
	Named    bool
	Prompted bool
	Warnings []string
}
