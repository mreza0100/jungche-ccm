package mcpserv

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"hostops/pfm/internal/naming"
)

const (
	maxNeedles    = 5
	minNeedleRune = 20
)

// extractNeedles mirrors the CLI's excerpt shaping rules. The actual find
// operation is supplied by SharedOperations; this helper remains useful to
// callers that inspect the shaping contract in isolation.
func extractNeedles(excerpt string) []string {
	type candidate struct {
		text   string
		length int
	}
	candidates := make([]candidate, 0)
	seen := make(map[string]struct{})
	for _, line := range strings.Split(excerpt, "\n") {
		line = strings.TrimSuffix(line, "\r")
		line = strings.TrimLeft(line, " \t>#*-")
		line = strings.TrimRight(line, " \t")
		length := utf8.RuneCountInString(line)
		if length < minNeedleRune {
			continue
		}
		if _, duplicate := seen[line]; duplicate {
			continue
		}
		seen[line] = struct{}{}
		candidates = append(candidates, candidate{text: line, length: length})
	}
	sort.SliceStable(candidates, func(left, right int) bool {
		return candidates[left].length > candidates[right].length
	})
	if len(candidates) > maxNeedles {
		candidates = candidates[:maxNeedles]
	}
	needles := make([]string, 0, len(candidates))
	for _, item := range candidates {
		needles = append(needles, item.text)
	}
	return needles
}

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
	if current.operations.Find == nil {
		return FindOutput{}, fmt.Errorf("chat_find shared CLI operation is not configured")
	}
	return current.operations.Find(ctx, input)
}

// searchableRows is shared by source resolution for chat_last/status and is
// deliberately not a search implementation. Transcript finding itself is
// owned by the CLI callback above.
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
			id: transcript.UUID, path: transcript.Path, engine: "cc", name: name,
			dir: transcript.CWD, mtimeNS: transcript.MTimeNS,
			metadata: strings.Join([]string{
				transcript.UUID, name, transcript.FirstPrompt,
				transcript.LastPrompt, transcript.CWD,
			}, "\n"),
		})
	}
	for _, rollout := range rollouts {
		name := naming.CxName(
			rollout.ID, rollout.SessionID, rollout.ParentThread,
			names, rollout.FirstPrompt,
		)
		rows = append(rows, searchable{
			id: rollout.ID, path: rollout.Path, engine: "cx", name: name,
			dir: rollout.CWD, mtimeNS: rollout.MTimeNS,
			metadata: strings.Join([]string{
				rollout.ID, rollout.SessionID, name,
				rollout.FirstPrompt, rollout.CWD,
			}, "\n"),
		})
	}
	return rows, nil
}
