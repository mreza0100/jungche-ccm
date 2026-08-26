package mcpserv

import (
	"context"
	pfmengine "hostops/pfm/internal/engine"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"hostops/pfm/internal/paths"
	"hostops/pfm/internal/transcript"
)

// newFixtureService keeps the package's older protocol tests on an explicit
// shared-operation seam. Production uses the command package's callbacks;
// these callbacks only provide the deterministic transcript fixture behavior
// needed by tests that construct mcpserv directly.
func newFixtureService(t *testing.T) *Service {
	t.Helper()
	resolved, err := paths.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewConfigured("test", io.Discard, Runtime{
		Paths:                resolved,
		Operations:           fixtureSharedOperations(),
		AllowAmbientIdentity: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func fixtureSharedOperations() SharedOperations {
	return SharedOperations{
		Find: func(ctx context.Context, input FindInput) (FindOutput, error) {
			query := strings.TrimSpace(input.Excerpt)
			if query == "" {
				return FindOutput{}, io.ErrUnexpectedEOF
			}
			needles := extractNeedles(query)
			if len(needles) == 0 {
				needles = []string{query}
			}
			files := fixtureTranscriptFiles()
			self := os.Getenv("CLAUDE_CODE_SESSION_ID")
			candidates := make([]FindCandidate, 0)
			for _, path := range files {
				id := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
				if !input.IncludeSelf && id == self {
					continue
				}
				if err := ctx.Err(); err != nil {
					return FindOutput{}, err
				}
				content, err := os.ReadFile(path)
				if err != nil {
					return FindOutput{}, err
				}
				hits := 0
				for _, needle := range needles {
					if strings.Contains(string(content), needle) {
						hits++
					}
				}
				if hits != 0 {
					candidates = append(candidates, FindCandidate{
						ID: id, Path: path, Engine: string(pfmengine.Claude),
						Hits: hits, Confirmed: true,
					})
				}
			}
			sort.Slice(candidates, func(left, right int) bool {
				if candidates[left].Hits != candidates[right].Hits {
					return candidates[left].Hits > candidates[right].Hits
				}
				return candidates[left].ID < candidates[right].ID
			})
			limit := input.Limit
			if limit == 0 {
				limit = 10
			}
			if len(candidates) > limit {
				candidates = candidates[:limit]
			}
			output := FindOutput{Candidates: candidates, Count: len(candidates), Needles: needles}
			if self != "" && !input.IncludeSelf {
				output.SelfID = self
			}
			return output, nil
		},
		Read: func(ctx context.Context, input ReadInput) (ReadOutput, error) {
			lastN := input.LastN
			if lastN == 0 {
				lastN = 20
			}
			maxBytes := input.MaxBytes
			if maxBytes == 0 {
				maxBytes = 64 << 10
			}
			path := ""
			for _, candidate := range fixtureTranscriptFiles() {
				id := strings.TrimSuffix(filepath.Base(candidate), filepath.Ext(candidate))
				if id == input.Source || filepath.Clean(candidate) == filepath.Clean(input.Source) {
					path = candidate
					break
				}
			}
			if path == "" {
				return ReadOutput{}, io.ErrUnexpectedEOF
			}
			entries, truncated, err := transcript.Tail(ctx, path, string(pfmengine.Claude), lastN, 0)
			if err != nil {
				return ReadOutput{}, err
			}
			turns := make([]Turn, 0, len(entries))
			bytes := 0
			for index := len(entries) - 1; index >= 0; index-- {
				available := maxBytes - bytes
				if available <= 0 {
					truncated = true
					break
				}
				entry := entries[index]
				text := entry.Text
				if len(text) > available {
					text = transcript.Truncate(text, available)
					truncated = true
				}
				bytes += len(text)
				turns = append(turns, Turn{Role: entry.Role, Text: text, Timestamp: entry.Timestamp})
			}
			for left, right := 0, len(turns)-1; left < right; left, right = left+1, right-1 {
				turns[left], turns[right] = turns[right], turns[left]
			}
			return ReadOutput{
				ID: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), Path: path,
				Engine: string(pfmengine.Claude), Turns: turns, Count: len(turns),
				Truncated: truncated, Bytes: bytes,
			}, nil
		},
	}
}

func fixtureTranscriptFiles() []string {
	root := os.Getenv(paths.EnvClaudeRoots)
	var files []string
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err == nil && entry != nil && !entry.IsDir() && filepath.Ext(path) == ".jsonl" {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	return files
}
