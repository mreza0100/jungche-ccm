package action

import (
	"context"
	"io"

	"hostops/pfm/internal/compose"
)

// Route is the legacy picker action letter.
type Route byte

const (
	NewClaude    Route = 'N'
	NewCodex     Route = 'C'
	Live         Route = 'L'
	Agent        Route = 'A'
	ResumeClaude Route = 'R'
	ResumeCodex  Route = 'X'
)

// Request is every value needed to synthesize and prepare one selected row.
type Request struct {
	Row            compose.Row
	PrimaryAccount int
	Cache1H        bool
	Bunker         bool
	Home           string
	FreshSocket    string
	CurrentTMUX    string
}

// CodexServer is a detached server that must exist before its attach line is
// emitted.
type CodexServer struct {
	Socket string
	CWD    string
	Run    string
}

// Plan contains the pure run string and the one eval line. Line never carries
// a trailing newline; the CLI writer owns the sole terminating newline.
type Plan struct {
	Route       Route
	Run         string
	Line        string
	CodexServer *CodexServer
}

// Pane is the tmux state needed by solo and self-switch.
type Pane struct {
	PaneID         string
	TTY            string
	SessionName    string
	WindowName     string
	WindowIndex    int
	CurrentCommand string
}

// TmuxClient contains only action mutations and prerequisite probes.
type TmuxClient interface {
	ListPanes(ctx context.Context, socket string) ([]Pane, error)
	SocketAlive(ctx context.Context, socket string) bool
	KillPane(ctx context.Context, socket, paneID string) error
	KillServer(ctx context.Context, socket string) error
	SetWindowSizeLatest(ctx context.Context, socket string) error
	SelectWindow(ctx context.Context, socket string, windowIndex int) error
	CreateCodexServer(ctx context.Context, server CodexServer) error
}

// Process is one candidate for _cc_solo's stray-Claude sweep.
type Process struct {
	PID  int
	Argv []string
	TTY  string
}

// ProcessTable supplies process snapshots and TERM without exposing /proc to
// the action core.
type ProcessTable interface {
	Processes(ctx context.Context) ([]Process, error)
	Terminate(pid int) error
}

// Gate asks on /dev/tty whether a mismatched live Claude chat should reboot.
type Gate interface {
	Confirm(ctx context.Context, request GateRequest) (bool, error)
}

// GateRequest is the live birth environment versus this picker invocation.
type GateRequest struct {
	Name           string
	BirthAccount   int
	PrimaryAccount int
	BirthCache1H   bool
	WantCache1H    bool
}

// CommandRunner invokes the unchanged satellite argv contracts.
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) error
}

// Dependencies replace every effect boundary in jailed tests.
type Dependencies struct {
	Tmux      TmuxClient
	Processes ProcessTable
	Gate      Gate
	Runner    CommandRunner
	Stderr    io.Writer
	// Heal repairs a Codex thread's history projection before it is resumed,
	// reporting in one line what it repaired and saying nothing when there
	// was nothing to repair. It is injected rather than imported because the
	// repair reads Codex's own SQLite stores, and action must stay free of
	// the store layer. A nil Heal simply skips the check.
	Heal HealFunc
}

// HealFunc is the pre-resume Codex projection repair.
type HealFunc func(ctx context.Context, threadID string) string

// Executor performs preparations and returns the only line the caller may
// write to stdout.
type Executor struct {
	tmux      TmuxClient
	processes ProcessTable
	gate      Gate
	runner    CommandRunner
	stderr    io.Writer
	sidDir    string
	heal      HealFunc
}
