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
	// IsBG marks machine work — a workflow lane or agent runner started this
	// conversation, not a person. It is the Codex twin of Transcript.IsBG and
	// keeps such rows out of the default listing.
	IsBG bool
}

// CxName mirrors one Codex thread's name and where pfm learned it.
type CxName struct {
	ID         string
	ThreadName string
	// Source records which half of Codex's own bookkeeping supplied
	// ThreadName: the state store's threads.name (CxNameSourceStore) or a
	// session_index.jsonl rename record (CxNameSourceSessionIndex).
	Source string
	// RenamedAt is the epoch-nanosecond time session_index.jsonl recorded for
	// this rename, or 0 when no rename time is known. That covers every store
	// name — the state store keeps no rename clock at all — and a
	// session_index entry written before Codex began stamping updated_at.
	RenamedAt int64
}

// CxNameSourceStore and CxNameSourceSessionIndex are the two CxName.Source
// values. CxNameSourceSessionIndex is also the column default: a name
// written before this provenance existed reads back as an undated
// session_index entry, which is the safer assumption of the two — it can
// never outrank a dated rename, matching how such a row already behaved.
const (
	CxNameSourceStore        = "store"
	CxNameSourceSessionIndex = "session_index"
)

// Hidden records a permanent hide for a Claude or Codex chat: it lifts only
// on an explicit unhide.
//
// The row lives in the fleet's shared store, keyed by uuid and nothing else.
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
	// BaselinePrompts is the retired auto-unhide baseline in the
	// at_payload column. Nothing reads it and nothing writes it any more; it
	// survives on the type only so old rows and the shared schema are left
	// untouched.
	BaselinePrompts *int64
}
