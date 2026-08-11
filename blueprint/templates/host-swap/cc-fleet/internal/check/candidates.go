package check

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"hostops/cc-fleet/internal/store"

	// The legacy picker's name cache is a SQLite database; reading it needs the
	// same driver the fleet store already links.
	_ "modernc.org/sqlite"
)

const legacyCacheDriver = "sqlite"

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

// CodexRolloutFileIDs returns every conversation id that owns a real rollout
// file, with no newest-N bound. The legacy picker's Codex source is a scan of
// those files, so a conversation absent from this set is one the reference
// script cannot list by any path — that is the fact behind codex-store-only,
// and it is read from the filesystem rather than inferred from the diff.
func CodexRolloutFileIDs(codexRoot string) (map[string]struct{}, error) {
	candidates, err := codexCandidates(codexRoot)
	if err != nil {
		return nil, err
	}
	result := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		if candidate.id != "" {
			result[candidate.id] = struct{}{}
		}
	}
	return result, nil
}

// CodexMachineSpawnedIDs returns every Codex thread id the state store
// classifies as machine-spawned — threads.source='exec' and never renamed
// (CodexThread.MachineSpawned, store/codexstate.go). A workflow lane or agent
// runner started such a thread, never a person at a picker, so Go rows it BG
// and the default listing suppresses it; the legacy picker has no such
// concept and lists it like any other resumable rollout. Read fresh from the
// read-only state store on every check run — never inferred from the diff —
// so the class this feeds re-verifies each absolved row against live
// evidence instead of trusting the diff's shape.
func CodexMachineSpawnedIDs(
	ctx context.Context,
	codexRoot string,
) (map[string]struct{}, error) {
	files, err := store.CodexStateFiles(codexRoot)
	if err != nil {
		return nil, fmt.Errorf("locate Codex state stores: %w", err)
	}
	threads, err := store.ReadCodexThreads(ctx, files)
	if err != nil {
		return nil, fmt.Errorf("read Codex state stores: %w", err)
	}
	result := make(map[string]struct{})
	for _, thread := range threads {
		if thread.MachineSpawned() {
			result[thread.ID] = struct{}{}
		}
	}
	return result, nil
}

// ClaudeTranscriptIDs mirrors the legacy picker's own store scan: every
// transcript file under a Claude root, at the depth `cc_find_meta "$store"
// -maxdepth 2 -name '*.jsonl'` walks. The legacy names its live-agent rows by
// intersecting the running processes with THIS set, so a session id absent from
// it is one the reference script cannot list however alive the process is.
func ClaudeTranscriptIDs(roots []string) (map[string]struct{}, error) {
	result := make(map[string]struct{})
	for _, root := range roots {
		if root == "" {
			continue
		}
		entries, err := os.ReadDir(root)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read Claude store %q: %w", root, err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				project, err := os.ReadDir(filepath.Join(root, entry.Name()))
				if err != nil {
					continue
				}
				for _, file := range project {
					collectTranscriptID(result, file.Name())
				}
				continue
			}
			collectTranscriptID(result, entry.Name())
		}
	}
	return result, nil
}

func collectTranscriptID(into map[string]struct{}, name string) {
	if filepath.Ext(name) != ".jsonl" {
		return
	}
	into[strings.TrimSuffix(name, ".jsonl")] = struct{}{}
}

// LegacyCacheEmptyCWDIDs returns the transcripts whose row in the legacy
// picker's own persistent name cache carries an EMPTY working directory.
//
// The cache (cc-db.sh's `chat` table) is written once per transcript and then
// only ever grown: _cc_metac's incremental branch adds the appended prompts and
// copies the cached cwd forward untouched (cc-fleet.zsh:672-704). A chat first
// seen before it had a parseable user record therefore keeps an empty cwd for
// life, and the picker renders its project as "?" while the transcript on disk
// names it. This reads the cache read-only so the class can point at the stale
// row instead of trusting the shape of the disagreement.
func LegacyCacheEmptyCWDIDs(
	ctx context.Context,
	databasePath string,
) (map[string]struct{}, error) {
	result := make(map[string]struct{})
	if databasePath == "" {
		return result, nil
	}
	if _, err := os.Stat(databasePath); errors.Is(err, fs.ErrNotExist) {
		return result, nil
	} else if err != nil {
		return nil, fmt.Errorf("stat legacy name cache %q: %w", databasePath, err)
	}
	database, err := sql.Open(
		legacyCacheDriver,
		"file:"+databasePath+"?mode=ro&_pragma=busy_timeout(2000)",
	)
	if err != nil {
		return nil, fmt.Errorf("open legacy name cache %q: %w", databasePath, err)
	}
	database.SetMaxOpenConns(1)
	defer database.Close()
	rows, err := database.QueryContext(
		ctx,
		"SELECT uuid FROM chat WHERE cwd IS NULL OR cwd = ''",
	)
	if err != nil {
		// A box whose legacy store predates the chat table has no cache to be
		// stale; that is an empty answer, not a broken check.
		return result, nil
	}
	defer rows.Close()
	for rows.Next() {
		var uuid string
		if err := rows.Scan(&uuid); err != nil {
			return nil, fmt.Errorf("read legacy name cache row: %w", err)
		}
		if uuid != "" {
			result[uuid] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan legacy name cache: %w", err)
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
