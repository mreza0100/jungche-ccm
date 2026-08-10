package mcpserv

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"hostops/cc-fleet/internal/naming"
)

const maxFindExcerpt = 8 << 10

type searchable struct {
	id       string
	path     string
	engine   string
	name     string
	dir      string
	mtimeNS  int64
	metadata string
}

func (current *backend) find(
	ctx context.Context,
	input FindInput,
) (FindOutput, error) {
	query := strings.TrimSpace(input.Excerpt)
	if query == "" {
		return FindOutput{}, fmt.Errorf("excerpt must not be empty")
	}
	if len(query) > maxFindExcerpt {
		return FindOutput{}, fmt.Errorf(
			"excerpt is %d bytes; maximum is %d",
			len(query),
			maxFindExcerpt,
		)
	}
	limit := input.Limit
	if limit == 0 {
		limit = 10
	}
	if limit < 1 || limit > 50 {
		return FindOutput{}, fmt.Errorf("limit must be between 1 and 50")
	}
	if err := current.index(ctx); err != nil {
		return FindOutput{}, err
	}
	rows, err := current.searchableRows(ctx)
	if err != nil {
		return FindOutput{}, err
	}
	lowerQuery := strings.ToLower(query)
	type ranked struct {
		row       searchable
		rank      int
		confirmed bool
		excerpt   string
	}
	matches := make([]ranked, 0)
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return FindOutput{}, err
		}
		metadataMatch := strings.Contains(strings.ToLower(row.metadata), lowerQuery)
		confirmed, excerpt, err := confirmLiteral(ctx, row.path, query)
		if err != nil && !os.IsNotExist(err) {
			continue
		}
		if !metadataMatch && !confirmed {
			continue
		}
		rank := 1
		if strings.Contains(strings.ToLower(row.name), lowerQuery) {
			rank = 0
		} else if !metadataMatch {
			rank = 2
		}
		matches = append(matches, ranked{
			row:       row,
			rank:      rank,
			confirmed: confirmed,
			excerpt:   excerpt,
		})
	}
	sort.Slice(matches, func(left, right int) bool {
		if matches[left].rank != matches[right].rank {
			return matches[left].rank < matches[right].rank
		}
		if matches[left].row.mtimeNS != matches[right].row.mtimeNS {
			return matches[left].row.mtimeNS > matches[right].row.mtimeNS
		}
		return matches[left].row.id < matches[right].row.id
	})
	if len(matches) > limit {
		matches = matches[:limit]
	}
	candidates := make([]FindCandidate, 0, len(matches))
	for _, match := range matches {
		date := ""
		if match.row.mtimeNS > 0 {
			date = time.Unix(0, match.row.mtimeNS).UTC().Format(time.RFC3339)
		}
		candidates = append(candidates, FindCandidate{
			ID:        match.row.id,
			Path:      match.row.path,
			Engine:    match.row.engine,
			Name:      match.row.name,
			Dir:       match.row.dir,
			Date:      date,
			Excerpt:   match.excerpt,
			Confirmed: match.confirmed,
		})
	}
	return FindOutput{Candidates: candidates, Count: len(candidates)}, nil
}

func (current *backend) searchableRows(
	ctx context.Context,
) ([]searchable, error) {
	transcripts, err := current.database.Transcripts(ctx)
	if err != nil {
		return nil, err
	}
	rollouts, err := current.database.Rollouts(ctx)
	if err != nil {
		return nil, err
	}
	names, err := current.database.CxNames(ctx)
	if err != nil {
		return nil, err
	}
	rows := make([]searchable, 0, len(transcripts)+len(rollouts))
	for _, transcript := range transcripts {
		name := naming.DisplayName(
			transcript.CustomTitle,
			transcript.AITitle,
			transcript.FirstPrompt,
		)
		rows = append(rows, searchable{
			id:      transcript.UUID,
			path:    transcript.Path,
			engine:  "cc",
			name:    name,
			dir:     transcript.CWD,
			mtimeNS: transcript.MTimeNS,
			metadata: strings.Join([]string{
				transcript.UUID,
				name,
				transcript.FirstPrompt,
				transcript.LastPrompt,
				transcript.CWD,
			}, "\n"),
		})
	}
	for _, rollout := range rollouts {
		name := naming.CxName(
			rollout.ID,
			rollout.SessionID,
			rollout.ParentThread,
			names,
			rollout.FirstPrompt,
		)
		rows = append(rows, searchable{
			id:      rollout.ID,
			path:    rollout.Path,
			engine:  "cx",
			name:    name,
			dir:     rollout.CWD,
			mtimeNS: rollout.MTimeNS,
			metadata: strings.Join([]string{
				rollout.ID,
				rollout.SessionID,
				name,
				rollout.FirstPrompt,
				rollout.CWD,
			}, "\n"),
		})
	}
	return rows, nil
}

func confirmLiteral(
	ctx context.Context,
	path, query string,
) (bool, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, "", err
	}
	defer file.Close()
	reader := bufio.NewReaderSize(file, 64<<10)
	lowerNeedle := bytes.ToLower([]byte(query))
	overlap := len(lowerNeedle) - 1
	if overlap < 0 {
		overlap = 0
	}
	tail := make([]byte, 0, overlap)
	for {
		if err := ctx.Err(); err != nil {
			return false, "", err
		}
		chunk := make([]byte, 64<<10)
		count, readErr := reader.Read(chunk)
		if count > 0 {
			window := append(append([]byte(nil), tail...), chunk[:count]...)
			lowerWindow := bytes.ToLower(window)
			if index := bytes.Index(lowerWindow, lowerNeedle); index >= 0 {
				start := maxInt(0, index-80)
				end := minInt(len(window), index+len(lowerNeedle)+80)
				return true, strings.TrimSpace(string(window[start:end])), nil
			}
			if overlap > 0 {
				start := maxInt(0, len(window)-overlap)
				tail = append(tail[:0], window[start:]...)
			}
		}
		if readErr == io.EOF {
			return false, "", nil
		}
		if readErr != nil {
			return false, "", readErr
		}
	}
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
