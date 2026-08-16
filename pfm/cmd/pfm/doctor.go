package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"hostops/pfm/internal/gather"
	"hostops/pfm/internal/paths"
	"hostops/pfm/internal/store"
)

func runDoctor(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("doctor", "usage: pfm doctor", stderr)
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 {
		flags.Usage()
		return 2
	}
	resolved, err := paths.Resolve()
	if err != nil {
		fmt.Fprintf(stdout, "doctor: unhealthy paths: %v\n", err)
		return 1
	}
	database, err := store.Open(store.WithWarningWriter(stderr))
	if err != nil {
		fmt.Fprintf(stdout, "doctor: unhealthy database: %v\n", err)
		return 1
	}
	defer database.Close()
	ctx := context.Background()
	warnings := 0

	pathWarnings := pfmPathWarnings(resolved.Home, os.Getenv("PATH"))
	for _, warning := range pathWarnings {
		fmt.Fprintf(stdout, "doctor: warning %s\n", warning)
	}
	warnings += len(pathWarnings)
	if len(pathWarnings) == 0 {
		fmt.Fprintln(stdout, "doctor: path canonical")
	}

	version, err := database.UserVersion(ctx)
	if err != nil {
		fmt.Fprintf(stdout, "doctor: unhealthy user_version: %v\n", err)
		return 1
	}
	check, err := database.QuickCheck(ctx)
	if err != nil {
		fmt.Fprintf(stdout, "doctor: unhealthy integrity: %v\n", err)
		return 1
	}
	if version != store.SchemaVersion || check != "ok" {
		warnings++
	}
	fmt.Fprintf(
		stdout,
		"doctor: database user_version=%d expected=%d quick_check=%s\n",
		version,
		store.SchemaVersion,
		check,
	)

	// Hides live in the fleet's shared database, not this binary's cache.
	sharedState := "ok"
	if degraded := database.SharedDegraded(); degraded != nil {
		warnings++
		sharedState = degraded.Error()
	}
	fmt.Fprintf(
		stdout,
		"doctor: shared store=%s state=%s\n",
		database.SharedPath(),
		sharedState,
	)

	counts, err := database.Counts(ctx)
	if err != nil {
		fmt.Fprintf(stdout, "doctor: unhealthy row counts: %v\n", err)
		return 1
	}
	if counts.OrphanedHides != 0 {
		warnings++
	}
	fmt.Fprintf(
		stdout,
		"doctor: rows transcripts=%d rollouts=%d cx_names=%d hidden=%d orphaned_hidden=%d\n",
		counts.Transcripts,
		counts.Rollouts,
		counts.CxNames,
		counts.Hidden,
		counts.OrphanedHides,
	)

	walBytes := int64(0)
	if info, err := os.Stat(database.Path() + "-wal"); err == nil {
		walBytes = info.Size()
	} else if !os.IsNotExist(err) {
		warnings++
		fmt.Fprintf(stdout, "doctor: warning WAL stat: %v\n", err)
	}
	fmt.Fprintf(stdout, "doctor: wal_bytes=%d\n", walBytes)

	hideWarnings, err := metaCounter(ctx, database, "busy_hide_warnings")
	if err != nil {
		fmt.Fprintf(stdout, "doctor: unhealthy busy counter: %v\n", err)
		return 1
	}
	unhideWarnings, err := metaCounter(ctx, database, "busy_unhide_warnings")
	if err != nil {
		fmt.Fprintf(stdout, "doctor: unhealthy busy counter: %v\n", err)
		return 1
	}
	if hideWarnings != 0 || unhideWarnings != 0 {
		warnings++
	}
	fmt.Fprintf(
		stdout,
		"doctor: busy_warnings hide=%d unhide=%d\n",
		hideWarnings,
		unhideWarnings,
	)

	rootWarnings := 0
	roots := append([]string(nil), resolved.ClaudeRoots...)
	roots = append(roots, resolved.CodexRoot, resolved.ProcRoot)
	for _, root := range roots {
		info, err := os.Stat(root)
		if err != nil || !info.IsDir() {
			rootWarnings++
			fmt.Fprintf(stdout, "doctor: warning unreachable_root=%s\n", root)
		}
	}
	warnings += rootWarnings
	fmt.Fprintf(
		stdout,
		"doctor: roots reachable=%d total=%d\n",
		len(roots)-rootWarnings,
		len(roots),
	)

	crumbEntries, crumbInvalid, crumbErr := crumbHealth(resolved.SIDDir)
	if crumbErr != nil {
		warnings++
		fmt.Fprintf(stdout, "doctor: warning crumb_dir=%v\n", crumbErr)
	} else {
		if crumbInvalid != 0 {
			warnings++
		}
		fmt.Fprintf(
			stdout,
			"doctor: crumbs entries=%d invalid=%d\n",
			crumbEntries,
			crumbInvalid,
		)
	}
	if warnings != 0 {
		fmt.Fprintf(stdout, "doctor: warnings=%d\n", warnings)
		return 1
	}
	fmt.Fprintln(stdout, "doctor: clean")
	return 0
}

// pfmPathWarnings checks both precedence and byte identity. A copied binary
// later on PATH can become the next active binary after a shell/toolchain
// change, so checking command resolution alone is insufficient.
func pfmPathWarnings(home, pathEnvironment string) []string {
	canonical := filepath.Join(home, ".local", "bin", "pfm")
	canonical, _ = filepath.Abs(canonical)
	canonicalHash, err := executableHash(canonical)
	if err != nil {
		return []string{fmt.Sprintf("pfm_canonical=%s error=%v", canonical, err)}
	}

	seen := make(map[string]bool)
	candidates := make([]string, 0)
	var warnings []string
	for _, directory := range filepath.SplitList(pathEnvironment) {
		if directory == "" {
			directory = "."
		}
		candidate, err := filepath.Abs(filepath.Join(directory, "pfm"))
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("pfm_path_entry=%s error=%v", directory, err))
			continue
		}
		candidate = filepath.Clean(candidate)
		if seen[candidate] {
			continue
		}
		seen[candidate] = true
		info, err := os.Stat(candidate)
		if err != nil {
			if !os.IsNotExist(err) {
				warnings = append(warnings, fmt.Sprintf("pfm_path_entry=%s error=%v", candidate, err))
			}
			continue
		}
		if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
			continue
		}
		candidates = append(candidates, candidate)
	}
	if len(candidates) == 0 {
		warnings = append(warnings, "pfm_path_resolves=not-found canonical="+canonical)
		return warnings
	}
	if candidates[0] != canonical {
		warnings = append(warnings, fmt.Sprintf(
			"pfm_path_resolves=%s canonical=%s",
			candidates[0],
			canonical,
		))
	}
	for _, candidate := range candidates {
		hash, err := executableHash(candidate)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("pfm_hash_read=%s error=%v", candidate, err))
			continue
		}
		if hash != canonicalHash {
			warnings = append(warnings, fmt.Sprintf(
				"pfm_hash_mismatch=%s canonical=%s",
				candidate,
				canonical,
			))
		}
	}
	return warnings
}

func executableHash(path string) ([sha256.Size]byte, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	return sha256.Sum256(content), nil
}

func metaCounter(
	ctx context.Context,
	database *store.Store,
	key string,
) (int64, error) {
	value, found, err := database.Meta(ctx, key)
	if err != nil || !found {
		return 0, err
	}
	count, err := strconv.ParseInt(value, 10, 64)
	if err != nil || count < 0 {
		return 0, fmt.Errorf("%s has invalid value %q", key, value)
	}
	return count, nil
}

func crumbHealth(path string) (entries, invalid int, err error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, 0, err
	}
	if !info.IsDir() {
		return 0, 0, fmt.Errorf("%s is not a directory", path)
	}
	directory, err := os.ReadDir(path)
	if err != nil {
		return 0, 0, err
	}
	for _, entry := range directory {
		entries++
		name := entry.Name()
		// A dot prefix marks sid bookkeeping rather than a crumb: the .lock
		// files and the open-lock directories the zsh creates with mkdir.
		if name == "" || name[0] == '.' {
			continue
		}
		if entry.IsDir() {
			invalid++
			continue
		}
		if _, _, ok := gather.ParseCrumbName(name); ok {
			continue
		}
		if filepath.Ext(name) == ".lock" ||
			nonFleetServerCrumb(name) ||
			knownSIDMetadata(name) {
			continue
		}
		invalid++
	}
	return entries, invalid, nil
}

// nonFleetServerCrumb reports whether a crumb names a tmux server the fleet
// deliberately excludes. The statusline writes a crumb for every Claude chat
// it sees, including chats on the vsct bunker and the revive dashboards, so
// those names are ordinary sid traffic rather than rot.
func nonFleetServerCrumb(name string) bool {
	socket := name
	if marker := strings.LastIndex(name, ".%"); marker >= 0 {
		socket = name[:marker]
	}
	return strings.HasPrefix(socket, "vsct") ||
		strings.HasPrefix(socket, "revive")
}

func knownSIDMetadata(name string) bool {
	const suffix = ".then-failed"
	if !strings.HasSuffix(name, suffix) {
		return false
	}
	socket := strings.TrimSuffix(name, suffix)
	_, paneID, ok := gather.ParseCrumbName(socket)
	return ok && paneID == ""
}
