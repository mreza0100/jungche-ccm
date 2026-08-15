package index

import (
	"bytes"
	"encoding/json"
	"strings"

	"hostops/pfm/internal/naming"
	"hostops/pfm/internal/store"
)

type claudeRecord struct {
	Type             string `json:"type"`
	CWD              string `json:"cwd"`
	CustomTitle      string `json:"customTitle"`
	AgentName        string `json:"agentName"`
	AITitle          string `json:"aiTitle"`
	SessionKind      string `json:"sessionKind"`
	Entrypoint       string `json:"entrypoint"`
	PromptSource     string `json:"promptSource"`
	IsCompactSummary bool   `json:"isCompactSummary"`
	Message          struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

func parseClaude(
	file diskFile,
	start int64,
	transcript store.Transcript,
) (store.Transcript, int64, error) {
	parsedOffset, bytesRead, err := readCompleteLines(file.Path, start, func(line []byte) {
		if !relevantClaudeLine(line) {
			return
		}

		var record claudeRecord
		if err := json.Unmarshal(line, &record); err != nil {
			return
		}
		if record.SessionKind == "bg" || sdkSpawned(record) {
			transcript.IsBG = true
		}

		switch record.Type {
		case "custom-title", "agent-name":
			transcript.CustomTitle = record.CustomTitle
			if transcript.CustomTitle == "" {
				transcript.CustomTitle = record.AgentName
			}
		case "ai-title":
			transcript.AITitle = record.AITitle
		case "user":
			if record.IsCompactSummary {
				return
			}
			prompt := naming.FlattenPromptText(record.Message.Content)
			if prompt == "" || naming.IsJunkPrompt(prompt) {
				return
			}
			if transcript.CWD == "" {
				transcript.CWD = record.CWD
			}
			if transcript.FirstPrompt == "" {
				transcript.FirstPrompt = prompt
			}
			transcript.LastPrompt = prompt
			transcript.PromptCount++
		}
	})
	if err != nil {
		return store.Transcript{}, bytesRead, err
	}

	transcript.UUID = file.ID
	transcript.Path = file.Path
	transcript.Size = file.Size
	transcript.MTimeNS = file.MTimeNS
	transcript.ParsedOffset = parsedOffset
	return transcript, bytesRead, nil
}

// sdkSpawned reports whether the Agent SDK drove this session instead of a
// human at a terminal. Workflow and subagent runs are launched through the SDK,
// which stamps every user record it writes with an sdk entrypoint and prompt
// source; an interactive chat carries "cli" and "typed" there. Those sessions
// are machine work, so they belong with the background rows rather than the
// picker's real chats.
func sdkSpawned(record claudeRecord) bool {
	if record.Type != "user" {
		return false
	}
	return record.PromptSource == "sdk" ||
		strings.HasPrefix(record.Entrypoint, "sdk-")
}

func relevantClaudeLine(line []byte) bool {
	return bytes.Contains(line, []byte(`"custom-title"`)) ||
		bytes.Contains(line, []byte(`"agent-name"`)) ||
		bytes.Contains(line, []byte(`"ai-title"`)) ||
		bytes.Contains(line, []byte(`"type":"user"`)) ||
		bytes.Contains(line, []byte(`"sessionKind":"bg"`)) ||
		bytes.Contains(line, []byte(`"isCompactSummary"`))
}
