package check

import (
	"bufio"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type rolloutCandidate struct {
	id      string
	path    string
	mtimeNS int64
}

// CodexCandidateIDs reproduces legacy's newest-N rollout-file bound using
// read-only filesystem metadata. Subagent files intentionally remain in the
// candidate population because legacy applies head before classification.
func CodexCandidateIDs(codexRoot string, limit int) (map[string]struct{}, error) {
	result := make(map[string]struct{})
	if limit <= 0 {
		return result, nil
	}
	candidates, err := codexCandidates(codexRoot)
	if err != nil {
		return nil, err
	}
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	for _, candidate := range candidates {
		if candidate.id != "" {
			result[candidate.id] = struct{}{}
		}
	}
	return result, nil
}

// CodexLegacyUnrecognizedIDs returns newest-bound files whose first line does
// not carry the legacy script's exact thread_source:user marker. The Go parser
// recognizes newer interactive source schemas; shadow parity must classify
// that intentional compatibility gap from filesystem evidence.
func CodexLegacyUnrecognizedIDs(
	codexRoot string,
	limit int,
) (map[string]struct{}, error) {
	result := make(map[string]struct{})
	if limit <= 0 {
		return result, nil
	}
	candidates, err := codexCandidates(codexRoot)
	if err != nil {
		return nil, err
	}
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	for _, candidate := range candidates {
		file, err := os.Open(candidate.path)
		if err != nil {
			return nil, err
		}
		firstLine, readErr := bufio.NewReader(file).ReadString('\n')
		closeErr := file.Close()
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if candidate.id != "" &&
			!strings.Contains(firstLine, `"thread_source":"user"`) {
			result[candidate.id] = struct{}{}
		}
	}
	return result, nil
}

func codexCandidates(codexRoot string) ([]rolloutCandidate, error) {
	root := filepath.Join(codexRoot, "sessions")
	candidates := make([]rolloutCandidate, 0)
	err := filepath.WalkDir(root, func(
		path string,
		entry fs.DirEntry,
		walkErr error,
	) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() ||
			!strings.HasPrefix(entry.Name(), "rollout-") ||
			filepath.Ext(entry.Name()) != ".jsonl" {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		candidates = append(candidates, rolloutCandidate{
			id:      rolloutID(entry.Name()),
			path:    path,
			mtimeNS: info.ModTime().UnixNano(),
		})
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sort.Slice(candidates, func(left, right int) bool {
		if candidates[left].mtimeNS != candidates[right].mtimeNS {
			return candidates[left].mtimeNS > candidates[right].mtimeNS
		}
		return candidates[left].path > candidates[right].path
	})
	return candidates, nil
}

func rolloutID(name string) string {
	stem := strings.TrimSuffix(name, ".jsonl")
	rest := strings.TrimPrefix(stem, "rollout-")
	if len(rest) > 20 &&
		rest[4] == '-' &&
		rest[7] == '-' &&
		rest[10] == 'T' &&
		rest[13] == '-' &&
		rest[16] == '-' &&
		rest[19] == '-' {
		return rest[20:]
	}
	return rest
}
