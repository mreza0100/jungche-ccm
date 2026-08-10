package index

import (
	"bytes"
	"encoding/json"

	"hostops/cc-fleet/internal/naming"
	"hostops/cc-fleet/internal/store"
)

type codexRecord struct {
	Type           string          `json:"type"`
	ID             string          `json:"id"`
	CWD            string          `json:"cwd"`
	SessionID      string          `json:"session_id"`
	ParentThread   string          `json:"parent_thread"`
	ParentThreadID string          `json:"parent_thread_id"`
	ThreadSource   string          `json:"thread_source"`
	Payload        codexPayload    `json:"payload"`
	Message        json.RawMessage `json:"message"`
}

type codexPayload struct {
	Type           string          `json:"type"`
	ID             string          `json:"id"`
	CWD            string          `json:"cwd"`
	SessionID      string          `json:"session_id"`
	ParentThread   string          `json:"parent_thread"`
	ParentThreadID string          `json:"parent_thread_id"`
	ThreadSource   string          `json:"thread_source"`
	Message        json.RawMessage `json:"message"`
}

func parseCodex(
	file diskFile,
	start int64,
	rollout store.Rollout,
) (store.Rollout, int64, error) {
	// A delta starts from a previously classified row. On a full parse the
	// session_meta line normally establishes the source before messages, but
	// the fallback remains user for older files without thread_source.
	sourceKnown := start > 0
	parsedOffset, bytesRead, err := readCompleteLines(file.Path, start, func(line []byte) {
		if !relevantCodexLine(line) {
			return
		}

		var record codexRecord
		if err := json.Unmarshal(line, &record); err != nil {
			return
		}
		if value := firstNonEmpty(record.CWD, record.Payload.CWD); value != "" {
			rollout.CWD = value
		}
		if value := firstNonEmpty(record.SessionID, record.Payload.SessionID); value != "" {
			rollout.SessionID = value
		}
		if value := firstNonEmpty(
			record.ParentThread,
			record.ParentThreadID,
			record.Payload.ParentThread,
			record.Payload.ParentThreadID,
		); value != "" {
			rollout.ParentThread = value
			// Newer Codex fork/subagent metadata may encode source as an
			// object instead of thread_source. A parent thread is independent
			// proof that this is not the top-level interactive rollout. An
			// explicit thread_source:user below still wins for older schemas
			// that legitimately carry both fields.
			rollout.UserThread = false
			sourceKnown = true
		}
		if source := firstNonEmpty(record.ThreadSource, record.Payload.ThreadSource); source != "" {
			rollout.UserThread = source == "user"
			sourceKnown = true
		}

		if record.Type != "user_message" && record.Payload.Type != "user_message" {
			return
		}
		if !sourceKnown {
			rollout.UserThread = true
		}
		rollout.PromptCount++
		message := record.Payload.Message
		if len(message) == 0 {
			message = record.Message
		}
		if prompt := naming.FlattenPromptText(message); rollout.FirstPrompt == "" && prompt != "" {
			rollout.FirstPrompt = prompt
		}
	})
	if err != nil {
		return store.Rollout{}, bytesRead, err
	}

	// The filename UUID is the rollout's stable identity. Later event records
	// may carry a parent/session/event ID; accepting one here collapses a user
	// rollout and its fork/subagent into one SQLite primary key, forcing the
	// displaced file to be fully reparsed on every warm pass.
	rollout.ID = file.ID
	rollout.Path = file.Path
	rollout.Size = file.Size
	rollout.MTimeNS = file.MTimeNS
	rollout.ParsedOffset = parsedOffset
	return rollout, bytesRead, nil
}

func relevantCodexLine(line []byte) bool {
	return bytes.Contains(line, []byte(`"session_meta"`)) ||
		bytes.Contains(line, []byte(`"thread_source"`)) ||
		bytes.Contains(line, []byte(`"session_id"`)) ||
		bytes.Contains(line, []byte(`"parent_thread`)) ||
		bytes.Contains(line, []byte(`"user_message"`))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
