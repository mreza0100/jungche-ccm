package compose

import (
	"hostops/cc-fleet/internal/gather"
	"hostops/cc-fleet/internal/store"
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
	default:
		return "unknown"
	}
}

// View selects the default, all, or hidden-only row set.
type View uint8

const (
	DefaultView View = iota
	AllView
	HiddenView
)

// AccountRoot associates one transcript/config root with its fleet account.
type AccountRoot struct {
	Account int
	Path    string
}

// Options controls pure presentation choices.
type Options struct {
	View           View
	CurrentDir     string
	CurrentSocket  string
	PrimaryAccount int
	CodexAvailable bool
	Rotation       int
	NowNS          int64
}

// Input is the complete immutable input to one composition pass.
type Input struct {
	Snapshot     gather.Snapshot
	Transcripts  []store.Transcript
	Rollouts     []store.Rollout
	CxNames      map[string]string
	Hidden       []store.Hidden
	AccountRoots []AccountRoot
	Options      Options
}

// Row is one live, resumable, agent, or new-chat choice.
type Row struct {
	Kind        Kind
	ID          string
	Path        string
	ConfigDir   string
	Socket      string
	PaneID      string
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
	Hidden      bool
	BG          bool
	C1H         bool
	Attached    bool
	Here        bool
	ServerCount int
	SplitCount  int
}

// Output contains the rows plus project metadata. A hide is permanent, so a
// composition pass carries no persistence intent: it only reads the hide list.
type Output struct {
	Rows            []Row
	ProjectDirs     map[string]string
	ProjectOrder    []string
	HiddenCount     int
	SuppressedCount int

	includeNewClaude bool
	includeNewCodex  bool
	primaryAccount   int
	fallbackDir      string
}
