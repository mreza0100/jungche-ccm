package hide

import (
	"context"
	"time"

	"hostops/cc-fleet/internal/gather"
	"hostops/cc-fleet/internal/store"
)

// ClaudeEngine and CodexEngine alias the store's engine names so the two
// packages cannot drift apart on the spelling of "cc".
const (
	ClaudeEngine = store.ClaudeEngine
	CodexEngine  = store.CodexEngine
)

// SelfEnvironment contains only the caller state used for self-identification.
type SelfEnvironment struct {
	TMUX            string
	TMUXPane        string
	ClaudeSessionID string
}

// Target is one hide identity and its optional live tmux address.
type Target struct {
	Engine     string
	ID         string
	DataPath   string
	SocketPath string
	SocketName string
	PaneID     string
}

// Request describes a public hide invocation.
type Request struct {
	ID          string
	Self        bool
	Exit        bool
	Environment SelfEnvironment
}

// ExitArgs are serialized onto the internal hide-exit argv.
type ExitArgs struct {
	Engine     string
	ID         string
	DataPath   string
	SocketPath string
	SocketName string
	PaneID     string
}

// TmuxClient abstracts all tmux mutation behind a jailed implementation.
type TmuxClient interface {
	PanePID(ctx context.Context, socketPath, paneID string) (int, error)
	PaneExists(ctx context.Context, socketPath, paneID string) bool
	SendLine(ctx context.Context, socketPath, paneID, line string) error
	KillPane(ctx context.Context, socketPath, paneID string) error
	KillServer(ctx context.Context, socketPath string) error
}

// ExitSpawner starts the detached internal finisher.
type ExitSpawner interface {
	Spawn(ctx context.Context, args ExitArgs) error
}

// Refresher updates indexed prompt counts after the engine flushes its file.
type Refresher interface {
	Refresh(ctx context.Context) error
}

// Dependencies replaces process/tmux/time boundaries in tests.
type Dependencies struct {
	ProcFS       gather.ProcFS
	Tmux         TmuxClient
	Spawner      ExitSpawner
	Now          func() time.Time
	Executable   string
	Delay        time.Duration
	PollEvery    time.Duration
	PollAttempts int
	Refresher    Refresher
}

// Manager performs public hide operations.
type Manager struct {
	database *store.Store
	proc     gather.ProcFS
	tmux     TmuxClient
	spawner  ExitSpawner
	now      func() time.Time
	paths    resolvedPaths
}

// Finisher performs the detached graceful-exit phase.
type Finisher struct {
	database     *store.Store
	tmux         TmuxClient
	refresher    Refresher
	now          func() time.Time
	delay        time.Duration
	pollEvery    time.Duration
	pollAttempts int
	paths        resolvedPaths
}

type resolvedPaths struct {
	home      string
	sidDir    string
	codexRoot string
	tmuxDir   string
}
