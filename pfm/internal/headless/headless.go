// Package headless answers questions about a spawned chat from its
// TRANSCRIPT, never from its tmux pane.
//
// The rule behind every function here: a chat that is gone must never look
// like a chat that is quiet. Pane-scraping loses whatever scrolled, cannot
// tell a crashed engine from a thinking one, and breaks the day either engine
// repaints — so liveness comes from the socket and content comes from the
// file, and both are reported explicitly.
package headless

import (
	"context"
	"fmt"
	"time"

	pfmengine "hostops/pfm/internal/engine"
	"hostops/pfm/internal/transcript"
)

// The states a chat can be in, as reported by status and watch.
const (
	StateWorking = "working"
	StateIdle    = "idle"
	StateDead    = "dead"
	StateMissing = "not-found"
)

// Chat is one resolved seat: who it is, where its record lives, and whether a
// server is still holding it up.
type Chat struct {
	Name    string
	ID      string
	Engine  pfmengine.ID
	Path    string
	CWD     string
	Socket  string
	Session string
	Pane    string
	Live    bool
}

// Status is the machine-readable verdict. Field names are a contract — a
// consumer scripts against them.
type Status struct {
	Name          string       `json:"name"`
	State         string       `json:"state"`
	IdleSeconds   int64        `json:"idle_seconds"`
	Engine        pfmengine.ID `json:"engine"`
	Model         string       `json:"model,omitempty"`
	CWD           string       `json:"cwd,omitempty"`
	SessionID     string       `json:"session_id,omitempty"`
	Socket        string       `json:"socket,omitempty"`
	ContextPct    float64      `json:"context_pct,omitempty"`
	Last          string       `json:"last,omitempty"`
	Summary       string       `json:"summary,omitempty"`
	SummaryCached bool         `json:"summary_cached,omitempty"`
}

// SummaryLine is the human status suffix. Cached summaries say so at the
// label, while structured output carries SummaryCached separately.
func (status Status) SummaryLine() string {
	label := "summary"
	if status.SummaryCached {
		label = "summary(cached)"
	}
	return label + ": " + status.Summary
}

// Alive reports whether the chat is a running seat, which is the only
// distinction a caller may act on without reading the state string.
func (status Status) Alive() bool {
	return status.State == StateWorking || status.State == StateIdle
}

// Line is the one-line human rendering.
func (status Status) Line() string {
	line := fmt.Sprintf(
		"%s\t%s\tidle=%ds\t%s",
		status.Name,
		status.State,
		status.IdleSeconds,
		status.Engine,
	)
	if status.Model != "" {
		line += "\t" + status.Model
	}
	if status.ContextPct > 0 {
		line += fmt.Sprintf("\tctx=%.0f%%", status.ContextPct)
	}
	if status.Last != "" {
		line += "\t" + status.Last
	}
	return line
}

// Inspect derives the status of a resolved chat at the given instant.
//
// working vs idle is decided by the TRANSCRIPT, not by a timer: a chat whose
// newest record is a tool call or a human turn owes an answer, and one whose
// newest record is the assistant speaking has delivered it. A long tool run
// therefore reads as working however quiet the file goes, which is the honest
// answer and the one a watcher must not mistake for finished.
func Inspect(
	ctx context.Context,
	chat Chat,
	now time.Time,
) (Status, error) {
	status := Status{
		Name:      chat.Name,
		Engine:    chat.Engine,
		CWD:       chat.CWD,
		SessionID: chat.ID,
		Socket:    chat.Socket,
		State:     StateDead,
	}
	if chat.Path == "" {
		// No transcript at all: a seat that never wrote a word. It is alive
		// only if its server is, and it has nothing to report either way.
		if chat.Live {
			status.State = StateWorking
		}
		return status, nil
	}
	meta, err := transcript.ReadMeta(chat.Path, string(chat.Engine))
	if err != nil {
		return status, fmt.Errorf("read chat transcript metadata %s: %w", chat.Path, err)
	}
	status.Model = meta.Model
	status.ContextPct = meta.ContextPercent()
	if meta.ModifiedUnixNS > 0 {
		idle := now.UnixNano() - meta.ModifiedUnixNS
		if idle < 0 {
			idle = 0
		}
		status.IdleSeconds = idle / int64(time.Second)
	}

	entries, _, err := transcript.Tail(ctx, chat.Path, string(chat.Engine), 1, transcript.TextCap)
	if err == nil && len(entries) > 0 {
		status.Last = transcript.Condensed(entries[len(entries)-1])
		if chat.Live {
			if entries[len(entries)-1].Role == transcript.RoleAssistant {
				status.State = StateIdle
			} else {
				status.State = StateWorking
			}
		}
	} else if chat.Live {
		status.State = StateWorking
	}
	return status, nil
}

// Missing is the status of a name nothing answers to. It is a value, not an
// error, so every caller renders the same shape — and it is never silent.
func Missing(name string) Status {
	return Status{Name: name, State: StateMissing}
}
