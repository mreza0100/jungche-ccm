package mcpserv

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"hostops/cc-fleet/internal/naming"
)

const (
	defaultReadTurns = 20
	maxReadTurns     = 200
	defaultReadBytes = 64 << 10
	maxReadBytes     = 1 << 20
)

type transcriptRecord struct {
	Type             string          `json:"type"`
	Timestamp        string          `json:"timestamp"`
	Summary          string          `json:"summary"`
	IsSidechain      bool            `json:"isSidechain"`
	IsCompactSummary bool            `json:"isCompactSummary"`
	Message          messageRecord   `json:"message"`
	Payload          payloadRecord   `json:"payload"`
	Content          json.RawMessage `json:"content"`
	Role             string          `json:"role"`
}

type messageRecord struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type payloadRecord struct {
	Type      string          `json:"type"`
	Message   json.RawMessage `json:"message"`
	Role      string          `json:"role"`
	Content   json.RawMessage `json:"content"`
	Timestamp string          `json:"timestamp"`
}

func (current *backend) read(
	ctx context.Context,
	input ReadInput,
) (ReadOutput, error) {
	if strings.TrimSpace(input.Source) == "" {
		return ReadOutput{}, fmt.Errorf("source must not be empty")
	}
	lastN := input.LastN
	if lastN == 0 {
		lastN = defaultReadTurns
	}
	if lastN < 1 || lastN > maxReadTurns {
		return ReadOutput{}, fmt.Errorf("last_n must be between 1 and %d", maxReadTurns)
	}
	maxBytes := input.MaxBytes
	if maxBytes == 0 {
		maxBytes = defaultReadBytes
	}
	if maxBytes < 1 || maxBytes > maxReadBytes {
		return ReadOutput{}, fmt.Errorf(
			"max_bytes must be between 1 and %d",
			maxReadBytes,
		)
	}
	if err := current.index(ctx); err != nil {
		return ReadOutput{}, err
	}
	source, err := current.resolveReadSource(ctx, input.Source)
	if err != nil {
		return ReadOutput{}, err
	}
	turns, truncated, err := extractTurns(
		ctx,
		source.path,
		source.engine,
		lastN,
		maxBytes,
	)
	if err != nil {
		return ReadOutput{}, err
	}
	bytes := 0
	for _, turn := range turns {
		bytes += len(turn.Text)
	}
	return ReadOutput{
		ID:        source.id,
		Path:      source.path,
		Engine:    source.engine,
		Turns:     turns,
		Count:     len(turns),
		Truncated: truncated,
		Bytes:     bytes,
	}, nil
}

func (current *backend) resolveReadSource(
	ctx context.Context,
	value string,
) (searchable, error) {
	rows, err := current.searchableRows(ctx)
	if err != nil {
		return searchable{}, err
	}
	clean := filepath.Clean(value)
	for _, row := range rows {
		if row.id == value || filepath.Clean(row.path) == clean {
			return row, nil
		}
	}
	// Codex callers sometimes know the session id rather than rollout id.
	rollouts, err := current.database.Rollouts(ctx)
	if err != nil {
		return searchable{}, err
	}
	for _, rollout := range rollouts {
		if rollout.SessionID == value {
			for _, row := range rows {
				if row.id == rollout.ID {
					return row, nil
				}
			}
		}
	}
	return searchable{}, fmt.Errorf("source %q is not an indexed transcript", value)
}

func extractTurns(
	ctx context.Context,
	path, engine string,
	lastN, maxBytes int,
) ([]Turn, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer file.Close()
	reader := bufio.NewReaderSize(file, 64<<10)
	ring := make([]Turn, 0, lastN)
	totalTurns := 0
	for {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		line, readErr := reader.ReadBytes('\n')
		if len(line) > 0 {
			if turn, ok := extractTurn(line, engine); ok {
				totalTurns++
				if len(turn.Text) > maxBytes {
					turn.Text = truncateUTF8(turn.Text, maxBytes)
				}
				if len(ring) == lastN {
					copy(ring, ring[1:])
					ring[len(ring)-1] = turn
				} else {
					ring = append(ring, turn)
				}
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, false, readErr
		}
	}

	kept := make([]Turn, 0, len(ring))
	used := 0
	truncated := totalTurns > len(ring)
	for index := len(ring) - 1; index >= 0; index-- {
		available := maxBytes - used
		if available <= 0 {
			truncated = true
			break
		}
		turn := ring[index]
		if len(turn.Text) > available {
			turn.Text = truncateUTF8(turn.Text, available)
			truncated = true
		}
		used += len(turn.Text)
		kept = append(kept, turn)
	}
	for left, right := 0, len(kept)-1; left < right; left, right = left+1, right-1 {
		kept[left], kept[right] = kept[right], kept[left]
	}
	return kept, truncated, nil
}

func extractTurn(line []byte, engine string) (Turn, bool) {
	var record transcriptRecord
	if err := json.Unmarshal(line, &record); err != nil || record.IsSidechain {
		return Turn{}, false
	}
	if engine == "cx" {
		return extractCodexTurn(record)
	}
	switch record.Type {
	case "summary":
		if record.Summary == "" {
			return Turn{}, false
		}
		return Turn{
			Role:      "summary",
			Text:      record.Summary,
			Timestamp: record.Timestamp,
		}, true
	case "user", "assistant":
		if record.IsCompactSummary {
			return Turn{}, false
		}
		text := visibleContent(record.Message.Content)
		if text == "" {
			return Turn{}, false
		}
		if record.Type == "user" && naming.IsJunkPrompt(text) {
			return Turn{}, false
		}
		return Turn{
			Role:      record.Type,
			Text:      text,
			Timestamp: record.Timestamp,
		}, true
	default:
		return Turn{}, false
	}
}

func extractCodexTurn(record transcriptRecord) (Turn, bool) {
	role := ""
	var content json.RawMessage
	switch record.Payload.Type {
	case "user_message":
		role = "user"
		content = record.Payload.Message
	case "agent_message":
		role = "assistant"
		content = record.Payload.Message
	case "message":
		role = record.Payload.Role
		content = record.Payload.Content
	default:
		if record.Type == "user_message" {
			role = "user"
			content = record.Message.Content
			if len(content) == 0 {
				content = record.Content
			}
		}
	}
	if role != "user" && role != "assistant" {
		return Turn{}, false
	}
	text := visibleContent(content)
	if text == "" || (role == "user" && naming.IsJunkPrompt(text)) {
		return Turn{}, false
	}
	timestamp := record.Timestamp
	if timestamp == "" {
		timestamp = record.Payload.Timestamp
	}
	return Turn{Role: role, Text: text, Timestamp: timestamp}, true
}

func visibleContent(raw json.RawMessage) string {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		if strings.HasPrefix(text, "<system-reminder>") {
			return ""
		}
		return text
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return ""
	}
	texts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		switch block.Type {
		case "text", "input_text", "output_text":
			if block.Text != "" && !strings.HasPrefix(block.Text, "<system-reminder>") {
				texts = append(texts, block.Text)
			}
		}
	}
	return strings.Join(texts, "\n\n")
}

func truncateUTF8(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	value = value[:limit]
	for !utf8.ValidString(value) && len(value) > 0 {
		value = value[:len(value)-1]
	}
	return value
}
