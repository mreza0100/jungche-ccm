package mcpserv

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"hostops/cc-fleet/internal/naming"
	"hostops/cc-fleet/internal/resolve"
)

// maxNeedleScanBytes bounds one transcript's needle scan so a pathological
// file cannot pin the server.
const maxNeedleScanBytes = 64 << 20

const (
	maxFindExcerpt = 8 << 10
	// chat.sh:455-456 keeps the five LONGEST excerpt lines of at least 20
	// characters as needles, and ranks a transcript by how many of them it
	// contains. A short line is not distinctive enough to vote.
	maxNeedles    = 5
	minNeedleRune = 20
)

// extractNeedles mirrors chat.sh's awk needle pass (chat.sh:455-456): strip a
// trailing CR, strip leading quote/list decoration and trailing blanks, keep
// lines of 20+ characters, and take the five longest, longest first.
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

// selfIDs are the transcripts belonging to the ASKING session. chat.sh drops
// its own transcript from every candidate list (chat.sh:462) — a session that
// pasted an excerpt of its own conversation would otherwise always find
// itself, which answers nothing.
func selfIDs(includeSelf bool) map[string]struct{} {
	excluded := make(map[string]struct{}, 2)
	if includeSelf {
		return excluded
	}
	for _, value := range []string{
		os.Getenv(resolve.ClaudeSessionEnv),
		os.Getenv(resolve.CodexThreadEnv),
	} {
		if value != "" {
			excluded[value] = struct{}{}
		}
	}
	return excluded
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
	needles := extractNeedles(query)
	excluded := selfIDs(input.IncludeSelf)
	type ranked struct {
		row       searchable
		rank      int
		hits      int
		confirmed bool
		excerpt   string
	}
	matches := make([]ranked, 0)
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return FindOutput{}, err
		}
		if _, self := excluded[row.id]; self {
			continue
		}
		if _, self := excluded[transcriptID(row.path)]; self {
			continue
		}
		metadataMatch := strings.Contains(strings.ToLower(row.metadata), lowerQuery)
		confirmed, excerpt, err := confirmLiteral(ctx, row.path, query)
		if err != nil && !os.IsNotExist(err) {
			continue
		}
		hits := 0
		if confirmed {
			// The whole excerpt is present verbatim, so every needle cut from
			// it is present too — a full-house vote, without re-reading.
			hits = len(needles)
		} else if len(needles) > 0 {
			counted, best, countErr := countNeedles(ctx, row.path, needles)
			if countErr != nil && !os.IsNotExist(countErr) {
				continue
			}
			hits = counted
			if excerpt == "" {
				excerpt = best
			}
		}
		if !metadataMatch && !confirmed && hits == 0 {
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
			hits:      hits,
			confirmed: confirmed,
			excerpt:   excerpt,
		})
	}
	// Votes first (chat.sh:462 `uniq -c | sort -rn`), then the existing
	// name/metadata rank, then recency, then a stable id tiebreak.
	sort.Slice(matches, func(left, right int) bool {
		if matches[left].hits != matches[right].hits {
			return matches[left].hits > matches[right].hits
		}
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
			Hits:      match.hits,
		})
	}
	selfID := ""
	for id := range excluded {
		if selfID == "" || id < selfID {
			selfID = id
		}
	}
	return FindOutput{
		Candidates: candidates,
		Count:      len(candidates),
		Needles:    needles,
		SelfID:     selfID,
	}, nil
}

// transcriptID is the session id a transcript file is named for, the form
// chat.sh excludes by (`grep -v "/$current.jsonl$"`).
func transcriptID(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// countNeedles counts how many distinct needles a transcript contains — one
// vote each, chat.sh's per-file tally — and returns the surrounding text of
// the first hit as the excerpt.
func countNeedles(
	ctx context.Context,
	path string,
	needles []string,
) (int, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, "", err
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, maxNeedleScanBytes))
	if err != nil {
		return 0, "", err
	}
	if err := ctx.Err(); err != nil {
		return 0, "", err
	}
	lowered := bytes.ToLower(content)
	hits := 0
	excerpt := ""
	for _, needle := range needles {
		index := bytes.Index(lowered, bytes.ToLower([]byte(needle)))
		if index < 0 {
			continue
		}
		hits++
		if excerpt == "" {
			start := maxInt(0, index-80)
			end := minInt(len(content), index+len(needle)+80)
			excerpt = strings.TrimSpace(string(content[start:end]))
		}
	}
	return hits, excerpt, nil
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
