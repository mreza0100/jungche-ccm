// Package inject implements guarded, socket-scoped live chat delivery.
package inject

import (
	"context"
	"time"

	"hostops/cc-fleet/internal/resolve"
)

const (
	// CodeBusy is the verdict a caller retries rather than escalates: the pane
	// was working, so nothing was typed. Everything else means the message
	// will never arrive without a change of plan.
	CodeBusy = 7
	// CodexInlineMax is chat.sh's hard limit for inline Codex delivery.
	CodexInlineMax = 1500
	// AbsoluteMessageMax keeps adversarial MCP arguments from reaching tmux.
	AbsoluteMessageMax = 256 << 10
	// CompactFocusMax is chat.sh's COMPACT_FOCUS_MAX: a longer /compact body is
	// typed as a bracketed paste, the TUI collapses it, and the compaction
	// never fires.
	CompactFocusMax = 600
	// FullScrollback asks Capture for the entire retained buffer, chat.sh's
	// `capture-pane -S -`, instead of the visible fold.
	FullScrollback = -1
)

// Target is one resolved tmux destination.
type Target struct {
	SocketPath string `json:"socket_path"`
	Pane       string `json:"pane"`
	Engine     string `json:"engine"`
}

// Request describes one live injection.
type Request struct {
	Target   string
	Message  string
	ForceNow bool
	// Then carries follow-up steers delivered, in order, by a DETACHED waiter
	// once the primary turn settles busy -> idle-stable (chat.sh --then).
	Then []string
	// Chain marks this delivery as a hop of an existing steer chain, so the
	// waiter's log is appended to instead of truncated (chat.sh CHAT_THEN_CHAIN).
	Chain bool
}

// Result is a non-ambiguous delivery verdict.
type Result struct {
	Status        string `json:"status"`
	Code          int    `json:"code"`
	Message       string `json:"message"`
	SocketPath    string `json:"socket_path,omitempty"`
	Pane          string `json:"pane,omitempty"`
	Proof         string `json:"proof,omitempty"`
	Busy          bool   `json:"busy,omitempty"`
	Interrupted   bool   `json:"interrupted,omitempty"`
	DraftStashed  bool   `json:"draft_stashed,omitempty"`
	Typed         bool   `json:"typed,omitempty"`
	SubmitRetries int    `json:"submit_retries,omitempty"`
	Steers        int    `json:"steers,omitempty"`
	SteerLog      string `json:"steer_log,omitempty"`
	Unsigned      bool   `json:"unsigned,omitempty"`
}

// Sender is appended to every non-command plain-text delivery.
type Sender struct {
	Session string
	Label   string
	UUID    string
}

// Resolver is the existing chat.sh-compatible namespace resolver.
type Resolver interface {
	Resolve(context.Context, resolve.Kind, string) (resolve.Outcome, error)
}

// SelfIdentifier answers who the SENDER is — chat.sh's self_tmux, including
// its ancestry recovery — so an inject from a codex-origin shell still signs.
type SelfIdentifier interface {
	Identify(ctx context.Context) (resolve.Identity, error)
}

// Tmux supplies every operation used in the guarded critical section.
type Tmux interface {
	Capture(
		ctx context.Context,
		socketPath, target string,
		styled bool,
		scrollback int,
	) (string, error)
	SendLiteral(ctx context.Context, socketPath, target, text string) error
	SendKey(ctx context.Context, socketPath, target, key string) error
	CancelCopyMode(ctx context.Context, socketPath, target string) error
	PaneInMode(ctx context.Context, socketPath, target string) (bool, error)
	CurrentSession(ctx context.Context, socketPath string) (string, error)
	// WindowName backs chat.sh's codex label fallback: a codex chat has no 🔖
	// statusline, so its human thread name is the tmux window name.
	WindowName(ctx context.Context, socketPath, target string) (string, error)
}

// ThenSpawner starts the detached waiter that delivers --then steers. It must
// outlive the caller: for a self-inject the waiter waits on the very turn that
// spawned it, so a synchronous wait would deadlock.
type ThenSpawner interface {
	Spawn(ctx context.Context, request SteerSpawn) error
}

// SteerSpawn is one detached follow-up delivery.
type SteerSpawn struct {
	SocketPath string
	Target     string
	Steers     []string
	LogPath    string
	Append     bool
}

// Options controls bounded retries. Zero values select chat.sh defaults.
type Options struct {
	Poll             time.Duration
	EnterGap         time.Duration
	EnterSettle      time.Duration
	ProofSettle      time.Duration
	BusyTries        int
	InterruptTries   int
	StashTries       int
	SettleTries      int
	EnterTries       int
	Scrollback       int
	ProofLines       int
	LockTimeout      time.Duration
	LockPoll         time.Duration
	LockMaxHold      time.Duration
	CodexInlineMax   int
	AbsoluteByteMax  int
	CompactFocusMax  int
	LockRoot         string
	Sender           *Sender
	DisableSignature bool

	// --then waiter cadence, mirroring chat.sh's __then subcommand.
	ThenMin        time.Duration
	ThenBusyTries  int
	ThenIdlePoll   time.Duration
	ThenIdleTries  int
	ThenIdleStable int
	ThenSettle     time.Duration
	ThenLogRoot    string
}

// Dependencies are injectable for jailed and adversarial tests.
type Dependencies struct {
	Resolver   Resolver
	Tmux       Tmux
	Spawner    ThenSpawner
	Identifier SelfIdentifier
	Options    Options
}
