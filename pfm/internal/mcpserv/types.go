// Package mcpserv exposes the pfm chat family over stdio MCP.
package mcpserv

import (
	pfmengine "hostops/pfm/internal/engine"
)

// LSInput selects the fleet view returned by chat_ls.
type LSInput struct {
	All     bool   `json:"all,omitempty" jsonschema:"include killed, background, and uncapped rows"`
	Killed  bool   `json:"killed,omitempty" jsonschema:"return killed rows only"`
	Project string `json:"project,omitempty" jsonschema:"case-insensitive project or directory filter"`
}

// ChatRow is one structured live or resumable fleet row.
type ChatRow struct {
	Session string       `json:"session"`
	ID      string       `json:"id"`
	Engine  pfmengine.ID `json:"engine"`
	State   string       `json:"state"`
	Dir     string       `json:"dir"`
	Project string       `json:"project"`
	Name    string       `json:"name"`
	Account int          `json:"account,omitempty"`
	Kind    string       `json:"kind"`
	Killed  bool         `json:"killed,omitempty"`
	Socket  string       `json:"socket,omitempty"`
	Pane    string       `json:"pane,omitempty"`
}

// LSOutput is chat_ls's structured response.
type LSOutput struct {
	Rows        []ChatRow `json:"rows"`
	Count       int       `json:"count"`
	KilledCount int       `json:"killed_count"`
}

// ResolveInput selects one chat.sh resolution namespace.
type ResolveInput struct {
	Kind string `json:"kind" jsonschema:"resolution namespace: label, session, or cxwin"`
	Name string `json:"name" jsonschema:"exact label, session, or Codex window name"`
}

// ResolveOutput mirrors chat.sh return codes 0, 1, and 2.
type ResolveOutput struct {
	Status     string `json:"status"`
	Code       int    `json:"code"`
	SocketPath string `json:"socket_path,omitempty"`
	Pane       string `json:"pane,omitempty"`
	Candidates string `json:"candidates,omitempty"`
}

// InjectInput requests guarded live delivery.
type InjectInput struct {
	Target   string   `json:"target" jsonschema:"live session, Claude label, Codex thread name, self, or tmux pane"`
	Message  string   `json:"message" jsonschema:"message to type and submit"`
	ForceNow bool     `json:"force_now,omitempty" jsonschema:"interrupt a busy target with Escape before delivery"`
	Then     []string `json:"then,omitempty" jsonschema:"follow-up steers delivered by a detached waiter after the primary turn settles to idle; in order, one settled turn apart. A /compact message REQUIRES at least one, and no steer may itself start with /compact"`
}

// InjectOutput is a stable MCP representation of inject.Result.
type InjectOutput struct {
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
	AutoFilePath  string `json:"auto_file_path,omitempty"`
	LiteralChunks int    `json:"literal_chunks,omitempty"`
}

// SelfCompactInput safely compacts the requesting chat and carries the turns
// that must resume work after compaction. Focus is retained in the tool-call
// history for the compactor AND composed onto the delivered command
// ("/compact " + focus) — the single-line, control-character-free
// validation is exactly what makes that concatenation safe. A target known
// to be Codex still receives the bare command: an earlier investigation
// recorded that Codex accepts no inline arguments on /compact, and nothing
// here re-tests it, so that constraint is held rather than assumed away.
// See chatSelfCompact for the full reasoning and what would retire it.
//
// Focus and Then are the ONLY things that survive. A caller holding durable
// state of its own — a ledger, a handoff file, a chat-specific memory — writes
// to it before calling, because nothing here can carry that state across.
type SelfCompactInput struct {
	Focus string   `json:"focus" jsonschema:"single-line compact focus authored after inspecting the requesting chat's current context and in-flight work"`
	Then  []string `json:"then" jsonschema:"mandatory post-compact steers, delivered in order one settled turn apart; no steer may start with /compact"`
}

// KeysInput requests tmux keypresses for one resolved live chat. Keys are
// key names unless Literal is explicitly true; this mirrors `pfm chat keys`.
type KeysInput struct {
	Target  string   `json:"target" jsonschema:"live session, Claude label, Codex thread name, self, or tmux pane"`
	Keys    []string `json:"keys" jsonschema:"tmux key names to press"`
	Literal bool     `json:"literal,omitempty" jsonschema:"type each key as literal text instead of pressing it"`
	DelayMS int      `json:"delay_ms,omitempty" jsonschema:"pause between keys in milliseconds; default 120"`
	Capture bool     `json:"capture,omitempty" jsonschema:"return the pane after the keys land"`
}

// KeysOutput is the structured receipt for chat_keys.
type KeysOutput struct {
	Status     string   `json:"status"`
	Code       int      `json:"code"`
	SocketPath string   `json:"socket_path,omitempty"`
	Pane       string   `json:"pane,omitempty"`
	Count      int      `json:"count"`
	Keys       []string `json:"keys"`
	Text       string   `json:"text,omitempty"`
}

// CaptureInput requests a pane snapshot of the whole retained scrollback,
// bounded by the caller after the capture.
type CaptureInput struct {
	Target    string `json:"target" jsonschema:"live session, Claude label, Codex thread name, self, or tmux pane"`
	TailLines *int   `json:"tail_lines,omitempty" jsonschema:"return at most this many non-empty lines of the scrollback; default 40"`
	MaxBytes  *int   `json:"max_bytes,omitempty" jsonschema:"maximum returned text bytes, default 262144 and maximum 4194304"`
}

// CaptureOutput is chat_capture's structured response.
type CaptureOutput struct {
	Status     string `json:"status"`
	Code       int    `json:"code"`
	SocketPath string `json:"socket_path,omitempty"`
	Pane       string `json:"pane,omitempty"`
	Text       string `json:"text,omitempty"`
	Message    string `json:"message,omitempty"`
	Bytes      int    `json:"bytes,omitempty"`
	Truncated  bool   `json:"truncated,omitempty"`
}

// WhoamiInput takes no arguments: Codex identity arrives in the reserved MCP
// request metadata, while stdio callers retain environment/ancestry recovery.
type WhoamiInput struct{}

// WhoamiOutput is chat.sh whoami's answer plus the engine identity behind it.
type WhoamiOutput struct {
	Status     string `json:"status"`
	Session    string `json:"session,omitempty"`
	SocketPath string `json:"socket_path,omitempty"`
	SocketName string `json:"socket_name,omitempty"`
	Pane       string `json:"pane,omitempty"`
	Engine     string `json:"engine,omitempty"`
	ID         string `json:"id,omitempty"`
	Source     string `json:"source,omitempty"`
	Recovered  bool   `json:"recovered,omitempty"`
	Message    string `json:"message,omitempty"`
}

// FindInput searches indexed transcript files.
type FindInput struct {
	Excerpt     string `json:"excerpt" jsonschema:"literal name or prompt excerpt to find; a multi-line excerpt is split into needles and candidates are ranked by how many they hit"`
	Limit       int    `json:"limit,omitempty" jsonschema:"maximum candidates, default 10 and maximum 50"`
	IncludeSelf bool   `json:"include_self,omitempty" jsonschema:"also match the asking session's own transcript, which is excluded by default"`
}

// FindCandidate is one confirmed indexed transcript match.
type FindCandidate struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	Engine    string `json:"engine"`
	Name      string `json:"name"`
	Dir       string `json:"dir"`
	Date      string `json:"date"`
	Excerpt   string `json:"excerpt,omitempty"`
	Confirmed bool   `json:"confirmed"`
	Hits      int    `json:"hits"`
}

// FindOutput is chat_find's structured response.
type FindOutput struct {
	Candidates []FindCandidate `json:"candidates"`
	Count      int             `json:"count"`
	Needles    []string        `json:"needles,omitempty"`
	SelfID     string          `json:"self_id,omitempty"`
}

// ReadInput requests bounded recent transcript turns.
type ReadInput struct {
	Source   string `json:"source" jsonschema:"indexed Claude/Codex id or exact indexed path"`
	LastN    int    `json:"last_n,omitempty" jsonschema:"maximum recent turns, default 20 and maximum 200"`
	MaxBytes int    `json:"max_bytes,omitempty" jsonschema:"maximum returned text bytes, default 65536 and maximum 1048576"`
}

// Turn is one visible user, assistant, or summary transcript record.
type Turn struct {
	Role      string `json:"role"`
	Text      string `json:"text"`
	Timestamp string `json:"timestamp,omitempty"`
}

// ReadOutput is chat_read's bounded extraction.
type ReadOutput struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	Engine    string `json:"engine"`
	Turns     []Turn `json:"turns"`
	Count     int    `json:"count"`
	Truncated bool   `json:"truncated"`
	Bytes     int    `json:"bytes"`
}

// LastInput selects the newest assistant answer for a chat.
type LastInput struct {
	Target string `json:"target" jsonschema:"chat id, name, session, or tmux target"`
}

type LastOutput struct {
	Target string `json:"target"`
	Text   string `json:"text"`
}

// StatusInput selects one chat for headless status inspection.
type StatusInput struct {
	Target  string `json:"target" jsonschema:"chat id, name, session, or tmux target"`
	Summary bool   `json:"summary,omitempty" jsonschema:"summarize the last human exchange; off by default"`
	Ask     bool   `json:"ask,omitempty" jsonschema:"answer the chat's current status from its live pane capture plus its last exchange; off by default, never cached"`
	Engine  string `json:"engine,omitempty" jsonschema:"summary engine override: claude or codex"`
	Model   string `json:"model,omitempty" jsonschema:"summary model override"`
}

type StatusOutput struct {
	Name          string  `json:"name"`
	State         string  `json:"state"`
	IdleSeconds   int64   `json:"idle_seconds"`
	Engine        string  `json:"engine"`
	Model         string  `json:"model,omitempty"`
	CWD           string  `json:"cwd,omitempty"`
	SessionID     string  `json:"session_id,omitempty"`
	Socket        string  `json:"socket,omitempty"`
	ContextPct    float64 `json:"context_pct,omitempty"`
	Last          string  `json:"last,omitempty"`
	Summary       string  `json:"summary,omitempty"`
	SummaryCached bool    `json:"summary_cached,omitempty"`
	Ask           string  `json:"ask,omitempty"`
}

// TargetInput is shared by chat actions whose CLI form takes one target.
type TargetInput struct {
	Target string `json:"target" jsonschema:"chat id, name, session, or tmux target"`
}

type NameInput struct {
	Target string `json:"target"`
	Name   string `json:"name"`
}

type NewInput struct {
	Name     string `json:"name"`
	Engine   string `json:"engine,omitempty"`
	CWD      string `json:"cwd,omitempty"`
	Account  int    `json:"account,omitempty"`
	Cache1H  bool   `json:"1h,omitempty"`
	Model    string `json:"model,omitempty"`
	Effort   string `json:"effort,omitempty"`
	Prompt   string `json:"prompt,omitempty"`
	Await    bool   `json:"await,omitempty"`
	Timeout  int    `json:"timeout,omitempty"`
	Settle   int    `json:"settle,omitempty"`
	Progress bool   `json:"progress,omitempty"`
	Attach   bool   `json:"attach,omitempty"`
}

type ActionOutput struct {
	Status  string `json:"status"`
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
}

type KillInput struct {
	Target string `json:"target"`
	Exit   bool   `json:"exit,omitempty"`
}

type SaveInput struct {
	Target     string `json:"target"`
	Transcript string `json:"transcript,omitempty"`
}

type BranchInput struct {
	Name string `json:"name,omitempty" jsonschema:"optional name for the detached fork"`
}

type GoalInput struct {
	Target string `json:"target,omitempty" jsonschema:"target chat; defaults to the requesting chat"`
	Goal   string `json:"goal" jsonschema:"already-compiled inline /goal body, one line and at most 4000 characters"`
}

type LoadInput struct {
	Paths    []string `json:"paths" jsonschema:"files or directories whose complete text set must be returned"`
	MaxBytes int      `json:"max_bytes,omitempty" jsonschema:"whole-result byte ceiling, default 1048576 and maximum 4194304"`
}

type LoadFile struct {
	Path  string `json:"path"`
	Lines int    `json:"lines"`
	Bytes int    `json:"bytes"`
	Text  string `json:"text"`
}

type LoadOutput struct {
	Files      []LoadFile `json:"files"`
	Warnings   []string   `json:"warnings,omitempty"`
	Count      int        `json:"count"`
	TotalLines int        `json:"total_lines"`
	TotalBytes int        `json:"total_bytes"`
}

// IssueInput is one agent complaint filed against Professor itself. Reporter
// identity is never accepted here — issue_servicedesk captures it the same
// way chat_inject captures a sender, so a model can complain but never say
// who is complaining.
type IssueInput struct {
	Title    string `json:"title" jsonschema:"short one-line issue title"`
	Detail   string `json:"detail" jsonschema:"full issue body: what went wrong, what was expected, and the surface it happened on"`
	Severity string `json:"severity,omitempty" jsonschema:"low, medium, or high; defaults to medium"`
	Area     string `json:"area,omitempty" jsonschema:"free text naming the command, agent, file, or surface this is about"`
}

// IssueOutput is issue_servicedesk's receipt: the filed issue's stable id.
type IssueOutput struct {
	Status string `json:"status"`
	ID     int64  `json:"id,omitempty"`
}
