package store

// Transcript is one indexed Claude transcript.
type Transcript struct {
	UUID         string
	Path         string
	Size         int64
	MTimeNS      int64
	ParsedOffset int64
	CWD          string
	CustomTitle  string
	AITitle      string
	FirstPrompt  string
	LastPrompt   string
	PromptCount  int64
	IsBG         bool
}

// Rollout is one indexed Codex rollout.
type Rollout struct {
	ID           string
	Path         string
	Size         int64
	MTimeNS      int64
	ParsedOffset int64
	CWD          string
	UserThread   bool
	SessionID    string
	ParentThread string
	LineageRoot  string
	FirstPrompt  string
	PromptCount  int64
}

// CxName mirrors one Codex session_index name.
type CxName struct {
	ID         string
	ThreadName string
}

// Hidden records a permanent hide for a Claude or Codex chat: it lifts only
// on an explicit unhide.
type Hidden struct {
	ID       string
	Engine   string
	HiddenAt int64
	// BaselinePrompts is the retired auto-unhide baseline. Nothing reads it
	// and new hides write NULL; the column survives so the schema and old
	// rows are left untouched.
	BaselinePrompts *int64
}
