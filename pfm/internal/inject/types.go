// Package inject implements guarded, socket-scoped live chat delivery.
package inject

import (
	"context"
	"time"

	"hostops/pfm/internal/resolve"
)

const (
	// CodeDead means the target resolved to a live pane that disappeared or
	// became unreadable before delivery could complete.
	CodeDead = 3
	// CodeUnknown means no live target matched the requested namespace.
	CodeUnknown = 4
	// CodeUndelivered means the target exists but the guarded transaction did
	// not put a turn into its model input.
	CodeUndelivered = 6
	// CodeBusy is the verdict a caller retries rather than escalates: the pane
	// was working, so nothing was typed. It is internal retry telemetry; the
	// CLI maps it to CodeUndelivered and never exposes rc 7.
	CodeBusy = 7
	// ClaudeAutoFileMax and CodexAutoFileMax are 10% below the earliest
	// empirically observed composer failure for each engine. Claude's smaller
	// bracketed-paste edge is 801 characters; Codex's inline and paste edge is
	// 1001. TESTPLAN.md records the authentic probe method and both transports.
	ClaudeAutoFileMax = 720
	CodexAutoFileMax  = 900
	// CommandChunkRunes stays safely below both measured literal-paste edges.
	// Slash commands bypass auto-file pointers, so every command byte reaches
	// the TUI through paced literal sends and Enter lands only after the final
	// chunk.
	CommandChunkRunes = 512
	CommandChunkGap   = 50 * time.Millisecond
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
	// FileBacked says Message came from --file. It travels through tmux's
	// bracketed-paste transport so a long Codex body is not mistaken for an
	// inline composer whose visible head/tail cannot be proven.
	FileBacked bool
	// Then carries follow-up steers delivered, in order, by a DETACHED waiter
	// once the primary turn settles busy -> idle-stable (chat.sh --then).
	Then []string
	// Chain marks this delivery as a hop of an existing steer chain, so the
	// waiter's log is appended to instead of truncated (chat.sh CHAT_THEN_CHAIN).
	Chain bool
}

// Result is a non-ambiguous delivery verdict.
type Result struct {
	Status         string `json:"status"`
	Code           int    `json:"code"`
	Message        string `json:"message"`
	SocketPath     string `json:"socket_path,omitempty"`
	Pane           string `json:"pane,omitempty"`
	Proof          string `json:"proof,omitempty"`
	ResolutionNote string `json:"resolution_note,omitempty"`
	Busy           bool   `json:"busy,omitempty"`
	Interrupted    bool   `json:"interrupted,omitempty"`
	DraftStashed   bool   `json:"draft_stashed,omitempty"`
	Typed          bool   `json:"typed,omitempty"`
	SubmitRetries  int    `json:"submit_retries,omitempty"`
	Steers         int    `json:"steers,omitempty"`
	SteerLog       string `json:"steer_log,omitempty"`
	Unsigned       bool   `json:"unsigned,omitempty"`
	AutoFilePath   string `json:"auto_file_path,omitempty"`
	LiteralChunks  int    `json:"literal_chunks,omitempty"`
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
	SendPaste(ctx context.Context, socketPath, target, text string) error
	SendKey(ctx context.Context, socketPath, target, key string) error
	CancelCopyMode(ctx context.Context, socketPath, target string) error
	PaneInMode(ctx context.Context, socketPath, target string) (bool, error)
	// PaneCommand verifies and identifies the foreground Claude/Codex TUI
	// before Inject types into its mid-turn composer queue. Socket spelling is
	// an address, not proof of what process currently owns the pane.
	PaneCommand(ctx context.Context, socketPath, target string) (string, error)
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
	// Sender is the spawning chat's own identity, carried down because the
	// waiter runs detached and can derive none of its own.
	Sender Sender
}

// Options controls bounded retries. Zero values select chat.sh defaults.
type Options struct {
	Poll              time.Duration
	EnterGap          time.Duration
	EnterSettle       time.Duration
	ProofSettle       time.Duration
	BusyTries         int
	InterruptTries    int
	StashTries        int
	SettleTries       int
	EnterTries        int
	Scrollback        int
	ProofLines        int
	LockTimeout       time.Duration
	LockPoll          time.Duration
	LockMaxHold       time.Duration
	ClaudeAutoFileMax int
	CodexAutoFileMax  int
	CommandChunkRunes int
	CommandChunkGap   time.Duration
	LockRoot          string
	BodyRoot          string
	BodyMaxAge        time.Duration
	Now               func() time.Time
	Sender            *Sender
	DisableSignature  bool

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
	// ClaudeBinary and CodexBinary are the configured launch commands. Tmux
	// exposes only each foreground process basename, so Engine uses these
	// values to verify a live pane without assuming the defaults.
	ClaudeBinary  string
	CodexBinary   string
	AccountEmojis []string
	// CodexSeat is the last sender-identity rung. It maps this process's
	// CODEX_THREAD_ID to the live fleet seat after ambient tmux and ancestry
	// recovery both fail. Nil means that lookup is unavailable.
	CodexSeat SelfIdentifier
	Options   Options
}
