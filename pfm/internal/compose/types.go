package compose

import (
	"hostops/pfm/internal/gather"
	"hostops/pfm/internal/store"
)

// Kind identifies the action and visual treatment for a row.
type Kind uint8

const (
	LiveClaude Kind = iota + 1
	LiveCodex
	LiveSplit
	Agent
	ResumeClaude
	ResumeCodex
	NewClaude
	NewCodex
	// Booting is a chat still sitting at a startup prompt (folder trust, MCP
	// approval): its pane and process are live but its statusline has not
	// written a SID crumb yet, so no transcript identity exists to key an
	// ordinary live row on. Not killable, not resumable — Enter is the only
	// live operation it supports (attach), matching the socket-only identity
	// it carries.
	Booting
)

func (kind Kind) String() string {
	switch kind {
	case LiveClaude:
		return "live-claude"
	case LiveCodex:
		return "live-codex"
	case LiveSplit:
		return "live-split"
	case Agent:
		return "agent"
	case ResumeClaude:
		return "resume-claude"
	case ResumeCodex:
		return "resume-codex"
	case NewClaude:
		return "new-claude"
	case NewCodex:
		return "new-codex"
	case Booting:
		return "booting"
	default:
		return "unknown"
	}
}

// View selects the default, all, or killed-only row set.
type View uint8

const (
	DefaultView View = iota
	AllView
	KilledView
)

// AccountRoot associates one transcript/config root with its fleet account.
type AccountRoot struct {
	Account int
	Path    string
}

// Options controls pure presentation choices.
type Options struct {
	View                View
	CurrentDir          string
	CurrentSocket       string
	PrimaryAccount      int
	CodexAccountIDs     []int
	PrimaryCodexAccount int
	NowNS               int64
}

// Input is the complete immutable input to one composition pass.
type Input struct {
	Snapshot     gather.Snapshot
	Transcripts  []store.Transcript
	Rollouts     []store.Rollout
	CxNames      map[string]string
	Killed       []store.Killed
	AccountRoots []AccountRoot
	CodexRoots   []AccountRoot
	Options      Options
}

// Row is one live, resumable, agent, or new-chat choice.
type Row struct {
	Kind      Kind
	ID        string
	Path      string
	ConfigDir string
	Socket    string
	PaneID    string
	// PanePIDs are the roots of this live socket's process tree. They are
	// presentation-neutral live facts used only by the lazy Stats sampler.
	PanePIDs    []int
	SessionName string
	WindowName  string
	Name        string
	LastPrompt  string
	Project     string
	CWD         string
	Size        int64
	PromptCount int64
	ActivityNS  int64
	AgeNS       int64
	Account     int
	Accounts    []int
	Killed      bool
	// NameKilled marks a row killed by its "_KILL…" label rather than by a
	// store row: the picker's kill key cannot toggle it, because the label —
	// not the killed table — is what keeps it out of the list.
	NameKilled  bool
	BG          bool
	C1H         bool
	Attached    bool
	Here        bool
	ServerCount int
	SplitCount  int
}

// Output contains the rows plus project metadata. Composition reads the kill
// list; the store owns persistence and expiry of /clear prompt baselines.
type Output struct {
	Rows            []Row
	ProjectDirs     map[string]string
	ProjectOrder    []string
	KilledCount     int
	SuppressedCount int

	includeNewClaude bool
	includeNewCodex  bool
	primaryAccount   int
	primaryCodex     int
	fallbackDir      string
}
