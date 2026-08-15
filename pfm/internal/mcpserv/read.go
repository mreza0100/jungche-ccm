package mcpserv

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"hostops/pfm/internal/transcript"
)

const (
	defaultReadTurns = 20
	maxReadTurns     = 200
	defaultReadBytes = 64 << 10
	maxReadBytes     = 1 << 20
)

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

// extractTurn maps one parsed transcript entry onto mcpserv's Turn contract.
// Tool calls are deliberately dropped here: chat_read has always returned
// spoken turns only, and its consumers page through them by count.
func extractTurn(line []byte, engine string) (Turn, bool) {
	entry, ok := transcript.Parse(line, engine)
	if !ok || entry.Role == transcript.RoleTool {
		return Turn{}, false
	}
	return Turn{
		Role:      entry.Role,
		Text:      entry.Text,
		Timestamp: entry.Timestamp,
	}, true
}

func truncateUTF8(value string, limit int) string {
	return transcript.Truncate(value, limit)
}
