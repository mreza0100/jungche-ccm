package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	goRuntime "runtime"
	"strconv"
	"strings"
	"time"

	"hostops/pfm/internal/config"
	"hostops/pfm/internal/gather"
	"hostops/pfm/internal/harvest"
	"hostops/pfm/internal/harvestpy"
	"hostops/pfm/internal/paths"
	"hostops/pfm/internal/store"
)

type harvestDoctor interface {
	Inspect(string, harvestpy.Platform) (harvestpy.EnvironmentDigest, error)
	Check(context.Context, string, harvestpy.Platform) (harvestpy.CheckReport, error)
}

type pinnedHarvestDoctor struct{}

// harvestDoctorOverride is nil in production. The command-package TestMain
// supplies a complete no-network fixture so existing doctor tests exercise
// fleet health without requiring a user-managed Python environment.
var harvestDoctorOverride harvestDoctor

func (pinnedHarvestDoctor) Inspect(root string, platform harvestpy.Platform) (harvestpy.EnvironmentDigest, error) {
	return harvestpy.Inspect(root, platform)
}

func (pinnedHarvestDoctor) Check(ctx context.Context, root string, platform harvestpy.Platform) (harvestpy.CheckReport, error) {
	return harvestpy.Check(ctx, root, platform)
}

func runDoctor(
	args []string,
	stdout, stderr io.Writer,
	runtime commandRuntime,
) int {
	flags := newFlagSet("doctor", "usage: pfm doctor", stderr)
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 {
		flags.Usage()
		return 2
	}
	resolved := runtime.Paths
	warnings := 0
	if runtime.ConfigError != nil {
		fmt.Fprintf(stdout, "doctor: config error=%v\n", runtime.ConfigError)
		warnings++
	}
	printDoctorConfig(stdout, runtime)
	if mcpConfigured(runtime) {
		status, daemonErr := mcpDaemonReachability(runtime)
		if daemonErr != nil {
			warnings++
			fmt.Fprintf(stdout, "doctor: mcp daemon=unreachable error=%v\n", daemonErr)
		} else {
			fmt.Fprintf(stdout, "doctor: mcp daemon=running pid=%d since=%s endpoint=%s\n", status.PID, status.StartTime, status.Endpoint)
			if status.PFMVersion != version {
				warnings++
				fmt.Fprintf(stdout, "doctor: mcp daemon=version-skew daemon=%s client=%s\n", status.PFMVersion, version)
			}
		}
	}
	database, err := store.Open(store.WithWarningWriter(stderr))
	if err != nil {
		fmt.Fprintf(stdout, "doctor: unhealthy database: %v\n", err)
		return 1
	}
	defer database.Close()
	ctx := context.Background()
	pathWarnings := pfmPathWarnings(resolved.Home, os.Getenv("PATH"))
	for _, warning := range pathWarnings {
		fmt.Fprintf(stdout, "doctor: warning %s\n", warning)
		if strings.HasPrefix(warning, "pfm_path_resolves=") || strings.HasPrefix(warning, "pfm_hash_mismatch=") {
			fmt.Fprintf(stdout, "doctor: remediation: put %s first on PATH and remove or rebuild stale pfm copies\n", filepath.Join(resolved.Home, ".local", "bin"))
		}
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

	// The process table is probed by READING it, not by stat'ing /proc. macOS has
	// no /proc and never will — pfm reads its process table through sysctl there
	// — so a directory check reports a healthy machine as broken, and on Linux it
	// proves less than the read does: a /proc that exists but denies the read is
	// the failure that actually matters.
	if pids, procErr := gather.NewProcFS(resolved.ProcRoot).PIDs(); procErr != nil {
		warnings++
		fmt.Fprintf(stdout, "doctor: warning process_table unreadable: %v\n", procErr)
	} else {
		fmt.Fprintf(stdout, "doctor: process_table readable pids=%d\n", len(pids))
	}

	rootWarnings := 0
	roots := append([]string(nil), resolved.ClaudeRoots...)
	roots = append(roots, resolved.CodexRoot)
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
	warnings += printHarvestPythonDoctor(ctx, stdout, resolved.Home, harvestpy.Platform{}, configuredHarvestDoctor())
	warnings += printHarvestCacheDoctor(stdout)
	if warnings != 0 {
		fmt.Fprintf(stdout, "doctor: warnings=%d\n", warnings)
		return 1
	}
	fmt.Fprintln(stdout, "doctor: clean")
	return 0
}

func printHarvestCacheDoctor(stdout io.Writer) int {
	root := harvest.CacheRoot()
	entries := 0
	walkErr := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			entries++
		}
		return nil
	})
	if os.IsNotExist(walkErr) {
		walkErr = nil
	}
	ttl := 24 * time.Hour
	ttlText := ttl.String()
	if raw := strings.TrimSpace(os.Getenv("HARVESTER_CACHE_TTL")); raw != "" {
		seconds, err := strconv.Atoi(raw)
		if err != nil || seconds < 0 {
			walkErr = errors.Join(walkErr, fmt.Errorf("invalid HARVESTER_CACHE_TTL=%q", raw))
		} else if seconds == 0 {
			ttlText = "never"
		} else {
			ttl = time.Duration(seconds) * time.Second
			ttlText = ttl.String()
		}
	}
	if walkErr != nil {
		fmt.Fprintf(stdout, "doctor: harvester_cache dir=%s entries=%d ttl=%s error=%v\n", root, entries, ttlText, walkErr)
		return 1
	}
	fmt.Fprintf(stdout, "doctor: harvester_cache dir=%s entries=%d ttl=%s\n", root, entries, ttlText)
	return 0
}

func mcpConfigured(runtime commandRuntime) bool {
	if runtime.Config.MCP.AuthToken != "" {
		return true
	}
	for _, server := range runtime.Config.MCPServers {
		if server.Enabled {
			return true
		}
	}
	return false
}

func configuredHarvestDoctor() harvestDoctor {
	if harvestDoctorOverride != nil {
		return harvestDoctorOverride
	}
	return pinnedHarvestDoctor{}
}

func printHarvestPythonDoctor(ctx context.Context, stdout io.Writer, home string, platform harvestpy.Platform, doctor harvestDoctor) int {
	if platform.GOOS == "" {
		platform.GOOS, platform.GOARCH = goRuntime.GOOS, goRuntime.GOARCH
	}
	root := filepath.Join(home, ".local", "state", "pfm", "harvest-python")
	current := harvestpy.RuntimeRoot(root, platform)
	interpreter := filepath.Join(current, "project", ".venv", "bin", "python")
	if _, err := os.Lstat(root); errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(stdout, "doctor: harvestpy skipped")
		return 0
	}
	warnings := 0

	plan, planErr := harvestpy.Plan(platform)
	if planErr != nil {
		fmt.Fprintf(stdout, "doctor: harvestpy pinned_version=(default) unavailable error=%v\n", planErr)
		warnings++
	} else {
		fmt.Fprintf(stdout, "doctor: harvestpy pinned_version=(default) %s\n", plan.PythonVersion)
		if len(plan.PackageBlockers) != 0 || plan.PackageDownloadStatus == "blocked-exact-lock" {
			warnings++
			fmt.Fprintf(stdout, "doctor: harvestpy package_plan=(default) blocked-exact-lock\n")
			for _, blocker := range plan.PackageBlockers {
				fmt.Fprintf(stdout, "doctor: harvestpy package_blocker=%s\n", blocker)
			}
		}
	}

	digest, inspectErr := doctor.Inspect(root, platform)
	report, checkErr := doctor.Check(ctx, root, platform)
	if report.Digest.Python != "" {
		digest = report.Digest
	}

	interpreterStatus, interpreterErr := harvestDoctorCheck(report, "interpreter", checkErr)
	if interpreterStatus {
		fmt.Fprintf(stdout, "doctor: harvestpy interpreter=(file) %s version=%s\n", interpreter, digest.Python)
	} else {
		warnings++
		fmt.Fprintf(stdout, "doctor: harvestpy interpreter=(file) %s broken error=%s\n", interpreter, interpreterErr)
	}
	if inspectErr != nil && digest.LockSHA256 == "" {
		warnings++
		fmt.Fprintf(stdout, "doctor: harvestpy marker=(file) broken error=%v\n", inspectErr)
	}

	lockOK, lockErr := harvestDoctorCheck(report, "lock_completeness", checkErr)
	if lockOK && digest.LockSHA256 != "" {
		fmt.Fprintf(stdout, "doctor: harvestpy lock=(file) complete digest=%s\n", digest.LockSHA256)
	} else {
		warnings++
		if lockErr == "" {
			lockErr = "lock digest or completeness check is missing"
		}
		fmt.Fprintf(stdout, "doctor: harvestpy lock=(file) incomplete error=%s\n", lockErr)
	}

	inventoryOK, inventoryErr := harvestDoctorCheck(report, "lock_completeness", checkErr)
	if inventoryOK && digest.InventorySHA256 != "" && digest.InventoryCount > 0 {
		fmt.Fprintf(stdout, "doctor: harvestpy inventory=(file) complete count=%d digest=%s\n", digest.InventoryCount, digest.InventorySHA256)
	} else {
		warnings++
		if inventoryErr == "" {
			inventoryErr = "installed inventory digest/count is missing"
		}
		fmt.Fprintf(stdout, "doctor: harvestpy inventory=(file) incomplete error=%s\n", inventoryErr)
	}

	smokeOK, smokeErr := harvestDoctorCheck(report, "live_smoke", checkErr)
	conversionOK, conversionErr := harvestDoctorCheck(report, "live_smoke_conversion", checkErr)
	if smokeOK && conversionOK {
		fmt.Fprintln(stdout, "doctor: harvestpy live_smoke=(file) healthy")
	} else {
		warnings++
		if smokeErr == "" {
			smokeErr = conversionErr
		}
		if smokeErr == "" {
			smokeErr = "live smoke check did not pass"
		}
		fmt.Fprintf(stdout, "doctor: harvestpy live_smoke=(file) broken error=%s\n", smokeErr)
	}
	return warnings
}

func harvestDoctorCheck(report harvestpy.CheckReport, name string, checkErr error) (bool, string) {
	if status, ok := report.Checks[name]; ok {
		return status.OK, status.Error
	}
	if report.Healthy && checkErr == nil {
		return true, ""
	}
	if checkErr != nil {
		return false, checkErr.Error()
	}
	return false, "check did not report healthy"
}

func printDoctorConfig(stdout io.Writer, runtime commandRuntime) {
	fmt.Fprintf(
		stdout,
		"doctor: config path=%s exists=%t\n",
		runtime.Config.Path,
		runtime.Config.Exists,
	)
	fmt.Fprintf(stdout, "doctor: config version=%d (%s)\n", runtime.Config.Version, runtime.Config.Source("version"))
	fmt.Fprintf(stdout, "doctor: config theme=%s (%s)\n", runtime.Config.Theme, runtime.Config.Source("theme"))
	accounts := make([]string, 0, len(runtime.Config.Accounts))
	for _, account := range runtime.Config.Accounts {
		accounts = append(accounts, fmt.Sprintf("%d:%s", account.ID, account.ConfigDir))
	}
	fmt.Fprintf(stdout, "doctor: config accounts=%s (%s)\n", strings.Join(accounts, ","), runtime.Config.Source("accounts"))
	fmt.Fprintf(stdout, "doctor: config claude.permissionMode=%s (%s)\n", runtime.Config.Claude.PermissionMode, runtime.Config.Source("claude.permissionMode"))
	fmt.Fprintf(stdout, "doctor: config claude.binary=%s (%s)\n", runtime.Config.Claude.Binary, runtime.Config.Source("claude.binary"))
	fmt.Fprintf(stdout, "doctor: config codex.yolo=%t (%s)\n", runtime.Config.Codex.Yolo, runtime.Config.Source("codex.yolo"))
	fmt.Fprintf(stdout, "doctor: config codex.binary=%s (%s)\n", runtime.Config.Codex.Binary, runtime.Config.Source("codex.binary"))
	for _, name := range config.RegisteredMCPServers() {
		key := "mcp.servers." + name + ".enabled"
		fmt.Fprintf(stdout, "doctor: config %s=%t (%s)\n", key, runtime.Config.MCPServers[name].Enabled, runtime.Config.Source(key))
	}
	fmt.Fprintf(stdout, "doctor: config mcp.http.port=%d (%s)\n", runtime.Config.MCP.HTTP.Port, runtime.Config.Source("mcp.http.port"))
}

// pfmPathWarnings checks both precedence and byte identity. A copied binary
// later on PATH can become the next active binary after a shell/toolchain
// change, so checking command resolution alone is insufficient.
func pfmPathWarnings(home, pathEnvironment string) []string {
	canonical := filepath.Join(home, ".local", "bin", "pfm")
	canonical, _ = filepath.Abs(canonical)
	targetHome, _ := filepath.Abs(home)
	jailed := os.Getenv(paths.EnvHome) != "" || os.Getenv("PFM_DEV_FENCE") != ""
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
		if jailed {
			relative, err := filepath.Rel(targetHome, candidate)
			if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
				continue
			}
		}
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
	return crumbHealthWith(path, os.Stat, os.ReadDir)
}

func crumbHealthWith(
	path string,
	stat func(string) (os.FileInfo, error),
	readDir func(string) ([]os.DirEntry, error),
) (entries, invalid int, err error) {
	info, err := stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil
		}
		return 0, 0, err
	}
	if !info.IsDir() {
		return 0, 0, fmt.Errorf("%s is not a directory", path)
	}
	directory, err := readDir(path)
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
