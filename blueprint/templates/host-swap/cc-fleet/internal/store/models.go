package store

// ClaudeEngine and CodexEngine name the two chat engines. They are the source
// of the same pair in internal/hide, which aliases them rather than repeating
// the literals.
const (
	ClaudeEngine = "cc"
	CodexEngine  = "cx"
)

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
//
// The row itself lives in the fleet's shared store, keyed by uuid and nothing
// else — cc-db.sh's schema (cc-db.sh:75-79), which the zsh half writes too.
type Hidden struct {
	ID string
	// Engine is DERIVED at read time, not stored: "cc" when the id resolves to
	// an indexed transcript, "cx" when it resolves to a rollout or a Codex
	// lineage root, and empty when neither table knows it. An empty engine
	// means "hidden whatever the engine" — it never lifts a hide.
	Engine string
	// HiddenAt is the shared row's hidden_at, or 0 for a hide that reached only
	// the carrier file, which records no time.
	HiddenAt int64
	// BaselinePrompts is the retired auto-unhide baseline, cc-db.sh's
	// at_payload column. Nothing reads it and nothing writes it any more; it
	// survives on the type only so old rows and the shared schema are left
	// untouched.
	BaselinePrompts *int64
}
