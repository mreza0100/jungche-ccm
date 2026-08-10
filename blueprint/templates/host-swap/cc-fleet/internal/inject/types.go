// Package inject implements guarded, socket-scoped live chat delivery.
package inject

import (
	"context"
	"time"

	"hostops/cc-fleet/internal/resolve"
)

const (
	// CodexInlineMax is chat.sh's hard limit for inline Codex delivery.
	CodexInlineMax = 1500
	// AbsoluteMessageMax keeps adversarial MCP arguments from reaching tmux.
	AbsoluteMessageMax = 256 << 10
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
	LockRoot         string
	Sender           *Sender
	DisableSignature bool
}

// Dependencies are injectable for jailed and adversarial tests.
type Dependencies struct {
	Resolver Resolver
	Tmux     Tmux
	Options  Options
}
