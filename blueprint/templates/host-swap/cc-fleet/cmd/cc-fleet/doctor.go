package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"hostops/cc-fleet/internal/gather"
	"hostops/cc-fleet/internal/paths"
	"hostops/cc-fleet/internal/store"
)

func runDoctor(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("doctor", "usage: cc-fleet doctor", stderr)
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
		if entry.IsDir() {
			invalid++
			continue
		}
		if _, _, ok := gather.ParseCrumbName(entry.Name()); ok {
			continue
		}
		if filepath.Ext(entry.Name()) == ".lock" ||
			entry.Name()[0] == '.' ||
			knownSIDMetadata(entry.Name()) {
			continue
		}
		invalid++
	}
	return entries, invalid, nil
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
