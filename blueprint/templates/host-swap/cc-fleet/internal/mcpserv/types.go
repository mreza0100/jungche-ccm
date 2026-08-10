// Package mcpserv exposes the cc-fleet chat family over stdio MCP.
package mcpserv

// LSInput selects the fleet view returned by chat_ls.
type LSInput struct {
	All     bool   `json:"all,omitempty" jsonschema:"include hidden, background, and uncapped rows"`
	Hidden  bool   `json:"hidden,omitempty" jsonschema:"return hidden rows only"`
	Project string `json:"project,omitempty" jsonschema:"case-insensitive project or directory filter"`
}

// ChatRow is one structured live or resumable fleet row.
type ChatRow struct {
	Session string `json:"session"`
	ID      string `json:"id"`
	Engine  string `json:"engine"`
	State   string `json:"state"`
	Dir     string `json:"dir"`
	Project string `json:"project"`
	Name    string `json:"name"`
	Account int    `json:"account,omitempty"`
	Kind    string `json:"kind"`
	Hidden  bool   `json:"hidden,omitempty"`
	Socket  string `json:"socket,omitempty"`
	Pane    string `json:"pane,omitempty"`
}

// LSOutput is chat_ls's structured response.
type LSOutput struct {
	Rows        []ChatRow `json:"rows"`
	Count       int       `json:"count"`
	HiddenCount int       `json:"hidden_count"`
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
	Target   string `json:"target" jsonschema:"live session, Claude label, Codex thread name, self, or tmux pane"`
	Message  string `json:"message" jsonschema:"message to type and submit"`
	ForceNow bool   `json:"force_now,omitempty" jsonschema:"interrupt a busy target with Escape before delivery"`
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
}

// CaptureInput requests a visible pane snapshot.
type CaptureInput struct {
	Target    string `json:"target" jsonschema:"live session, Claude label, Codex thread name, self, or tmux pane"`
	TailLines int    `json:"tail_lines,omitempty" jsonschema:"return at most this many non-empty lines; default 40"`
}

// CaptureOutput is chat_capture's structured response.
type CaptureOutput struct {
	Status     string `json:"status"`
	Code       int    `json:"code"`
	SocketPath string `json:"socket_path,omitempty"`
	Pane       string `json:"pane,omitempty"`
	Text       string `json:"text,omitempty"`
	Message    string `json:"message,omitempty"`
}

// FindInput searches indexed transcript files.
type FindInput struct {
	Excerpt string `json:"excerpt" jsonschema:"literal name or prompt excerpt to find"`
	Limit   int    `json:"limit,omitempty" jsonschema:"maximum candidates, default 10 and maximum 50"`
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
}

// FindOutput is chat_find's structured response.
type FindOutput struct {
	Candidates []FindCandidate `json:"candidates"`
	Count      int             `json:"count"`
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
