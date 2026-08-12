package hide

import (
	"bufio"
	"encoding/json"
	"os"
)

// codexRolloutHeader is the minimal subset of a rollout file's session_meta
// line this package needs: the conversation the file resumes from. It
// deliberately reads far less than the indexer's own parseCodex — no prompt
// counting, no whole-file scan — because resolving one hide's target must
// stay fast, not reconcile a file the indexer has not reached yet.
type codexRolloutHeader struct {
	SessionID      string `json:"session_id"`
	ParentThread   string `json:"parent_thread"`
	ParentThreadID string `json:"parent_thread_id"`
	Payload        struct {
		SessionID      string `json:"session_id"`
		ParentThread   string `json:"parent_thread"`
		ParentThreadID string `json:"parent_thread_id"`
	} `json:"payload"`
}

// readCodexLineageParent reads a rollout file's own session_meta header for
// the conversation it resumes from — session_id first, then
// parent_thread_id — the ONLY place that link exists before the indexer has
// parsed the file into the fleet database. Codex always writes session_meta
// first, so a handful of lines is enough; a file with no such line within
// that reach is either a fresh top-level conversation (nothing to find) or
// one the index will classify properly once it gets there.
func readCodexLineageParent(path string) string {
	if path == "" {
		return ""
	}
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()

	const maxLines = 20
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for lines := 0; lines < maxLines && scanner.Scan(); lines++ {
		var header codexRolloutHeader
		if err := json.Unmarshal(scanner.Bytes(), &header); err != nil {
			continue
		}
		if parent := firstNonEmptyLineageField(
			header.SessionID,
			header.Payload.SessionID,
			header.ParentThread,
			header.ParentThreadID,
			header.Payload.ParentThread,
			header.Payload.ParentThreadID,
		); parent != "" {
			return parent
		}
	}
	return ""
}

func firstNonEmptyLineageField(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
