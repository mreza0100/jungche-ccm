// Package index incrementally indexes Claude transcripts and Codex rollouts.
package index

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"hostops/cc-fleet/internal/paths"
	"hostops/cc-fleet/internal/store"
)

const (
	codexParserVersionKey = "codex_parser_version"
	codexParserVersion    = "4"

	claudeParserVersionKey = "claude_parser_version"
	claudeParserVersion    = "1"
)

// Options controls one indexing pass.
type Options struct {
	Full         bool
	PriorityCWD  string
	PriorityOnly bool
}

// Counters describes the work performed by one indexing pass.
type Counters struct {
	FilesSeen       int
	FilesSkipped    int
	DeltaParsed     int
	FullParsed      int
	Deleted         int
	RowsTouched     int
	BytesRead       int64
	CxNamesReloaded bool
	// CodexThreads counts the listed conversations the Codex state store
	// vouched for, and CodexRowsCreated the rows this pass had to create for
	// conversations the rollout directory never showed.
	CodexThreads     int
	CodexRowsCreated int
}

// Indexer incrementally mirrors transcript stores into SQLite.
type Indexer struct {
	database *store.Store
	paths    paths.Values
}

// New resolves the jailed or default host paths used by an Indexer.
func New(database *store.Store) (*Indexer, error) {
	if database == nil {
		return nil, fmt.Errorf("index store is nil")
	}
	resolved, err := paths.Resolve()
	if err != nil {
		return nil, fmt.Errorf("resolve index paths: %w", err)
	}
	return &Indexer{database: database, paths: resolved}, nil
}

// Run executes one complete Claude, Codex, and session-index pass.
func (indexer *Indexer) Run(ctx context.Context, options Options) (Counters, error) {
	var counters Counters

	claudeFiles, err := walkClaudeRoots(ctx, indexer.paths.ClaudeRoots)
	if err != nil {
		return counters, err
	}
	existingTranscripts, err := indexer.database.Transcripts(ctx)
	if err != nil {
		return counters, err
	}
	existingRollouts, err := indexer.database.Rollouts(ctx)
	if err != nil {
		return counters, err
	}
	claudeFiles = prioritizeClaudeFiles(
		claudeFiles,
		options.PriorityCWD,
		existingTranscripts,
		options.PriorityOnly,
	)
	var codexFiles []diskFile
	if !options.PriorityOnly {
		codexFiles, err = walkCodexRollouts(ctx, indexer.paths.CodexRoot)
		if err != nil {
			return counters, err
		}
	}
	counters.FilesSeen = len(claudeFiles) + len(codexFiles)

	storedCodexVersion, codexVersionFound, err := indexer.database.Meta(
		ctx,
		codexParserVersionKey,
	)
	if err != nil {
		return counters, err
	}
	forceCodexFull := options.Full ||
		!codexVersionFound ||
		storedCodexVersion != codexParserVersion

	storedClaudeVersion, claudeVersionFound, err := indexer.database.Meta(
		ctx,
		claudeParserVersionKey,
	)
	if err != nil {
		return counters, err
	}
	forceClaudeFull := options.Full ||
		!claudeVersionFound ||
		storedClaudeVersion != claudeParserVersion

	transcriptByPath := make(map[string]store.Transcript, len(existingTranscripts))
	for _, transcript := range existingTranscripts {
		transcriptByPath[transcript.Path] = transcript
	}
	rolloutByPath := make(map[string]store.Rollout, len(existingRollouts))
	rolloutByID := make(map[string]store.Rollout, len(existingRollouts))
	for _, rollout := range existingRollouts {
		rolloutByPath[rollout.Path] = rollout
		rolloutByID[rollout.ID] = rollout
	}

	transcriptUpdates := make([]store.Transcript, 0)
	presentTranscriptPaths := make(map[string]struct{}, len(claudeFiles))
	presentTranscriptIDs := make(map[string]struct{}, len(claudeFiles))
	for _, file := range claudeFiles {
		presentTranscriptPaths[file.Path] = struct{}{}
		presentTranscriptIDs[file.ID] = struct{}{}

		existing, found := transcriptByPath[file.Path]
		if shouldSkip(file, found, existing.Size, existing.MTimeNS, forceClaudeFull) {
			counters.FilesSkipped++
			continue
		}

		start := int64(0)
		base := store.Transcript{}
		if shouldDelta(file, found, existing.Size, existing.ParsedOffset, forceClaudeFull) {
			start = existing.ParsedOffset
			base = existing
			counters.DeltaParsed++
		} else {
			counters.FullParsed++
		}
		transcript, bytesRead, err := parseClaude(file, start, base)
		if err != nil {
			return counters, err
		}
		counters.BytesRead += bytesRead
		transcriptUpdates = append(transcriptUpdates, transcript)
	}

	rolloutUpdates := make([]store.Rollout, 0)
	presentRolloutPaths := make(map[string]struct{}, len(codexFiles))
	presentRolloutIDs := make(map[string]struct{}, len(codexFiles))
	for _, file := range codexFiles {
		presentRolloutPaths[file.Path] = struct{}{}
		existing, found := rolloutByPath[file.Path]
		if shouldSkip(file, found, existing.Size, existing.MTimeNS, forceCodexFull) {
			counters.FilesSkipped++
			presentRolloutIDs[existing.ID] = struct{}{}
			continue
		}

		start := int64(0)
		base := store.Rollout{}
		if shouldDelta(file, found, existing.Size, existing.ParsedOffset, forceCodexFull) {
			start = existing.ParsedOffset
			base = existing
			counters.DeltaParsed++
		} else {
			counters.FullParsed++
		}
		rollout, bytesRead, err := parseCodex(file, start, base)
		if err != nil {
			return counters, err
		}
		counters.BytesRead += bytesRead
		rolloutUpdates = append(rolloutUpdates, rollout)
		presentRolloutIDs[rollout.ID] = struct{}{}
	}

	// The Codex state store is consulted after the rollout files are parsed:
	// files supply sizes and prompt counts, the store supplies the population,
	// its classification, and the conversations that wrote no file at all.
	var codexThreads []store.CodexThread
	if !options.PriorityOnly {
		codexThreads, err = readCodexThreads(ctx, indexer.paths.CodexRoot)
		if err != nil {
			return counters, err
		}
		rolloutUpdates = reconcileCodexState(
			codexThreads,
			indexer.paths.CodexRoot,
			rolloutUpdates,
			rolloutByID,
			presentRolloutIDs,
			&counters,
		)
	}

	transcriptDeletes := make([]string, 0)
	rolloutDeletes := make([]string, 0)
	if !options.PriorityOnly {
		for _, transcript := range existingTranscripts {
			_, pathPresent := presentTranscriptPaths[transcript.Path]
			_, idPresent := presentTranscriptIDs[transcript.UUID]
			if !pathPresent && !idPresent {
				transcriptDeletes = append(transcriptDeletes, transcript.UUID)
			}
		}
		for _, rollout := range existingRollouts {
			_, pathPresent := presentRolloutPaths[rollout.Path]
			_, idPresent := presentRolloutIDs[rollout.ID]
			if !pathPresent && !idPresent {
				rolloutDeletes = append(rolloutDeletes, rollout.ID)
			}
		}
	}

	if err := writeTranscriptUpdates(ctx, indexer.database, transcriptUpdates); err != nil {
		return counters, err
	}
	if err := writeRolloutUpdates(ctx, indexer.database, rolloutUpdates); err != nil {
		return counters, err
	}
	if err := deleteTranscripts(ctx, indexer.database, transcriptDeletes); err != nil {
		return counters, err
	}
	if err := deleteRollouts(ctx, indexer.database, rolloutDeletes); err != nil {
		return counters, err
	}
	if !options.PriorityOnly {
		if err := indexer.database.ReconcileCodexLineageRoots(ctx); err != nil {
			return counters, fmt.Errorf("reconcile Codex lineage roots: %w", err)
		}
	}

	counters.Deleted = len(transcriptDeletes) + len(rolloutDeletes)
	counters.RowsTouched = len(transcriptUpdates) + len(rolloutUpdates) + counters.Deleted
	if !options.PriorityOnly {
		if err := reloadCxNames(
			ctx,
			indexer.database,
			indexer.paths.CodexRoot,
			&counters,
		); err != nil {
			return counters, err
		}
		// Store names are applied after the session_index mirror is rebuilt,
		// so a rename made inside Codex outranks the file it left behind.
		if err := reconcileCodexNames(
			ctx,
			indexer.database,
			codexThreads,
			&counters,
		); err != nil {
			return counters, err
		}
		if !codexVersionFound || storedCodexVersion != codexParserVersion {
			if err := indexer.database.SetMeta(
				ctx,
				codexParserVersionKey,
				codexParserVersion,
			); err != nil {
				return counters, err
			}
		}
		if !claudeVersionFound || storedClaudeVersion != claudeParserVersion {
			if err := indexer.database.SetMeta(
				ctx,
				claudeParserVersionKey,
				claudeParserVersion,
			); err != nil {
				return counters, err
			}
		}
	}
	return counters, nil
}

func prioritizeClaudeFiles(
	files []diskFile,
	cwd string,
	existing []store.Transcript,
	only bool,
) []diskFile {
	cwd = filepath.Clean(cwd)
	if cwd == "." || cwd == "" {
		if only {
			return nil
		}
		return files
	}
	knownPaths := make(map[string]struct{})
	for _, transcript := range existing {
		if filepath.Clean(transcript.CWD) == cwd {
			knownPaths[filepath.Clean(transcript.Path)] = struct{}{}
		}
	}
	encoded := strings.ReplaceAll(cwd, string(filepath.Separator), "-")
	if filepath.VolumeName(cwd) != "" {
		encoded = strings.ReplaceAll(encoded, ":", "-")
	}
	isPriority := func(file diskFile) bool {
		if _, found := knownPaths[filepath.Clean(file.Path)]; found {
			return true
		}
		return filepath.Base(filepath.Dir(file.Path)) == encoded
	}
	if only {
		selected := make([]diskFile, 0)
		for _, file := range files {
			if isPriority(file) {
				selected = append(selected, file)
			}
		}
		return selected
	}
	sort.SliceStable(files, func(left, right int) bool {
		leftPriority := isPriority(files[left])
		rightPriority := isPriority(files[right])
		if leftPriority != rightPriority {
			return leftPriority
		}
		return files[left].Path < files[right].Path
	})
	return files
}

func shouldSkip(
	file diskFile,
	found bool,
	oldSize int64,
	oldMTimeNS int64,
	full bool,
) bool {
	return !full &&
		found &&
		file.Size == oldSize &&
		file.MTimeNS == oldMTimeNS
}

// shouldDelta reports whether the previous parse of this file can be resumed.
// A row with no parsed bytes has no parse to continue — a Codex row the state
// store created before its rollout file existed is exactly that — so it is
// parsed in full instead of adding a whole file onto a carried-over base.
func shouldDelta(
	file diskFile,
	found bool,
	oldSize int64,
	parsedOffset int64,
	full bool,
) bool {
	return !full &&
		found &&
		oldSize > 0 &&
		file.Size > oldSize &&
		file.Size >= parsedOffset
}

func writeTranscriptUpdates(
	ctx context.Context,
	database *store.Store,
	updates []store.Transcript,
) error {
	return database.Batch(ctx, len(updates), func(tx *store.ImmediateTx, start, end int) error {
		for _, transcript := range updates[start:end] {
			if err := tx.UpsertTranscript(ctx, transcript); err != nil {
				return err
			}
		}
		return nil
	})
}

func writeRolloutUpdates(
	ctx context.Context,
	database *store.Store,
	updates []store.Rollout,
) error {
	return database.Batch(ctx, len(updates), func(tx *store.ImmediateTx, start, end int) error {
		for _, rollout := range updates[start:end] {
			if err := tx.UpsertRollout(ctx, rollout); err != nil {
				return err
			}
		}
		return nil
	})
}

func deleteTranscripts(
	ctx context.Context,
	database *store.Store,
	ids []string,
) error {
	return database.Batch(ctx, len(ids), func(tx *store.ImmediateTx, start, end int) error {
		for _, id := range ids[start:end] {
			if err := tx.DeleteTranscript(ctx, id); err != nil {
				return err
			}
		}
		return nil
	})
}

func deleteRollouts(
	ctx context.Context,
	database *store.Store,
	ids []string,
) error {
	return database.Batch(ctx, len(ids), func(tx *store.ImmediateTx, start, end int) error {
		for _, id := range ids[start:end] {
			if err := tx.DeleteRollout(ctx, id); err != nil {
				return err
			}
		}
		return nil
	})
}
