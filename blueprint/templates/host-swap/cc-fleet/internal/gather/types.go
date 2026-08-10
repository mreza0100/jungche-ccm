package gather

// Pane is one live tmux pane from a chat socket.
type Pane struct {
	Socket      string
	SessionName string
	WindowID    string
	WindowName  string
	PaneTitle   string
	CurrentPath string
	TTY         string
	PID         int
	PaneID      string
	Attached    bool
}

// TmuxProbe is the live pane result plus recoverable sweep diagnostics.
type TmuxProbe struct {
	Panes         []Pane
	CorpseSwept   []string
	ProbeWarnings []string
}

// Crumb maps a socket or pane to its current Claude transcript.
type Crumb struct {
	Filename       string
	Socket         string
	PaneID         string
	TranscriptPath string
}

// CrumbProbe contains accepted crumbs and sweep counters.
type CrumbProbe struct {
	Crumbs     []Crumb
	FilesSeen  int
	Accepted   int
	Ignored    int
	StaleSwept []string
}

// LiveCodex maps a live codex process and its first rollout onto a pane.
// ThreadID is the conversation the process was resolved to; it is the only
// identity a session that writes no rollout file has.
type LiveCodex struct {
	PID         int
	PanePID     int
	Socket      string
	PaneID      string
	RolloutPath string
	ThreadID    string
}

// ClaudeProcess maps one live Claude process onto its owning tmux pane.
type ClaudeProcess struct {
	PID     int
	PanePID int
	Socket  string
	PaneID  string
	TTY     string
}

// Agent is one non-primary live Claude session identity.
type Agent struct {
	PID       int
	PanePID   int
	Socket    string
	PaneID    string
	SessionID string
	ConfigDir string
	StartTime uint64
}

// WindowRename is an unapplied Codex window convergence action.
type WindowRename struct {
	Socket      string
	SessionName string
	WindowID    string
	CurrentName string
	TargetName  string
}

// Snapshot is one immutable-by-convention gather result.
type Snapshot struct {
	Panes           []Pane
	Crumbs          []Crumb
	Codex           []LiveCodex
	ClaudeProcesses []ClaudeProcess
	Agents          []Agent
	Cache1HSockets  []string
	Renames         []WindowRename
	CorpseSwept     []string
	StaleSwept      []string
	Warnings        []string
}
