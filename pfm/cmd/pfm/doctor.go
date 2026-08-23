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

	"hostops/pfm/internal/action"
	"hostops/pfm/internal/ask"
	"hostops/pfm/internal/config"
	"hostops/pfm/internal/deps"
	pfmengine "hostops/pfm/internal/engine"
	"hostops/pfm/internal/gather"
	"hostops/pfm/internal/harvest"
	"hostops/pfm/internal/harvestpy"
	"hostops/pfm/internal/index"
	"hostops/pfm/internal/installer"
	"hostops/pfm/internal/paths"
	"hostops/pfm/internal/spawn"
	"hostops/pfm/internal/stats"
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

var dependencyProbeOverride func(context.Context, []deps.Entry, deps.ProbeOptions) []deps.Result
var hookProbeOverride func(string, config.Config) []installer.HookProbeResult

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
	flags := newFlagSet("doctor", "usage: pfm doctor [--verbose]", stderr)
	verbose := flags.Bool("verbose", false, "write raw dependency probe output under tmp/")
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
	warnings += printEngineDoctor(stdout, runtime.Config)
	warnings += printEngineCapabilities(stdout)
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
	launcher, launcherErr := installer.InspectClaudeLauncher(resolved.Home)
	if launcherErr != nil {
		warnings++
		fmt.Fprintf(stdout, "doctor: launcher: unreadable error=%v — run pfm install\n", launcherErr)
	} else {
		switch launcher.State {
		case installer.LauncherOK:
			fmt.Fprintln(stdout, "doctor: launcher: ok")
		case installer.LauncherMissing:
			warnings++
			fmt.Fprintln(stdout, "doctor: launcher: missing — run pfm install")
		case installer.LauncherDisplaced:
			warnings++
			fmt.Fprintf(stdout, "doctor: launcher: DISPLACED by %s — run pfm install\n", launcher.Target)
		default:
			warnings++
			fmt.Fprintf(stdout, "doctor: launcher: unknown state=%s — run pfm install\n", launcher.State)
		}
	}
	verboseDir := ""
	if *verbose {
		verboseDir = filepath.Join("tmp", "pfm-doctor")
	}
	warnings += printDependencyDoctor(ctx, stdout, deps.Registry(deps.Options{
		Home: resolved.Home, ClaudeBinary: runtime.Config.Claude.Binary, CodexBinary: runtime.Config.Codex.Binary,
		ClaudeAccounts: len(runtime.Config.Accounts), CodexAccounts: len(runtime.Config.CodexAccounts),
	}), deps.ProbeOptions{VerboseDir: verboseDir})
	warnings += printHookDoctor(stdout, resolved.Home, runtime.Config)

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

	// Kills live in the fleet's shared database, not this binary's cache.
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
	if counts.OrphanedKills != 0 {
		warnings++
	}
	fmt.Fprintf(
		stdout,
		"doctor: rows transcripts=%d rollouts=%d cx_names=%d killed=%d orphaned_killed=%d\n",
		counts.Transcripts,
		counts.Rollouts,
		counts.CxNames,
		counts.Killed,
		counts.OrphanedKills,
	)

	walBytes := int64(0)
	if info, err := os.Stat(database.Path() + "-wal"); err == nil {
		walBytes = info.Size()
	} else if !os.IsNotExist(err) {
		warnings++
		fmt.Fprintf(stdout, "doctor: warning WAL stat: %v\n", err)
	}
	fmt.Fprintf(stdout, "doctor: wal_bytes=%d\n", walBytes)

	killWarnings, err := metaCounter(ctx, database, "busy_kill_warnings")
	if err != nil {
		fmt.Fprintf(stdout, "doctor: unhealthy busy counter: %v\n", err)
		return 1
	}
	unkillWarnings, err := metaCounter(ctx, database, "busy_unkill_warnings")
	if err != nil {
		fmt.Fprintf(stdout, "doctor: unhealthy busy counter: %v\n", err)
		return 1
	}
	if killWarnings != 0 || unkillWarnings != 0 {
		warnings++
	}
	fmt.Fprintf(
		stdout,
		"doctor: busy_warnings kill=%d unkill=%d\n",
		killWarnings,
		unkillWarnings,
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
	roots := make([]string, 0, len(runtime.Config.Accounts)+len(runtime.Config.CodexAccounts))
	for _, account := range runtime.Config.Accounts {
		roots = append(roots, account.ProjectDir)
	}
	for _, account := range runtime.Config.CodexAccounts {
		roots = append(roots, account.Home)
	}
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

func printEngineDoctor(stdout io.Writer, machine config.Config) int {
	counts := machine.Engines()
	defaultEngine, err := machine.DefaultEngine()
	parts := make([]string, 0, len(pfmengine.All()))
	for _, id := range pfmengine.All() {
		parts = append(parts, fmt.Sprintf("%s=%d", id, counts[id]))
	}
	if err != nil {
		fmt.Fprintf(stdout, "doctor: roster %s default=none error=%v\n", strings.Join(parts, " "), err)
		return 1
	}
	fmt.Fprintf(stdout, "doctor: roster %s default=%s\n", strings.Join(parts, " "), defaultEngine)
	return 0
}

func printEngineCapabilities(stdout io.Writer) int {
	capabilities := []struct {
		name string
		ids  []pfmengine.ID
	}{
		{name: "index", ids: index.RegisteredSources()},
		{name: "launcher", ids: spawn.RegisteredLaunchers()},
		{name: "matcher", ids: gather.RegisteredMatchers()},
		{name: "usage", ids: stats.RegisteredUsageSources()},
		{name: "headless", ids: action.RegisteredPlanners()},
		{name: "ask", ids: ask.RegisteredRunners()},
	}
	parts := make([]string, 0, len(pfmengine.All()))
	warnings := 0
	for _, id := range pfmengine.All() {
		registered := make([]string, 0, len(capabilities))
		for _, capability := range capabilities {
			if containsEngine(capability.ids, id) {
				registered = append(registered, capability.name)
			}
		}
		if len(registered) == 0 {
			parts = append(parts, fmt.Sprintf("%s=NONE (descriptor only)", id))
			warnings++
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%s", id, strings.Join(registered, ",")))
	}
	fmt.Fprintf(stdout, "doctor: engines %s\n", strings.Join(parts, " "))
	return warnings
}

func containsEngine(ids []pfmengine.ID, want pfmengine.ID) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

func configuredDependencyProbe(ctx context.Context, entries []deps.Entry, options deps.ProbeOptions) []deps.Result {
	if dependencyProbeOverride != nil {
		return dependencyProbeOverride(ctx, entries, options)
	}
	return deps.Probe(ctx, entries, options)
}

func printDependencyDoctor(ctx context.Context, stdout io.Writer, entries []deps.Entry, options deps.ProbeOptions) int {
	warnings := 0
	for _, result := range configuredDependencyProbe(ctx, entries, options) {
		entry := result.Entry
		switch result.State {
		case deps.StateSkipped:
			platform := strings.Join(entry.Platforms, ",")
			if platform == "" {
				platform = "all"
			}
			fmt.Fprintf(stdout, "doctor: dep %s platform=%s skipped (%s)\n", entry.Name, platform, result.Error)
		case deps.StateMissing:
			requirement := "optional"
			if entry.Required {
				requirement = "required"
				warnings++
			}
			fmt.Fprintf(stdout, "doctor: dep %s path=(none) MISSING %s — install: %s\n", entry.Name, requirement, entry.InstallHint)
		case deps.StateBroken:
			if entry.Required {
				warnings++
			}
			raw := deps.FirstLine(result.Raw)
			if raw != "" && !strings.Contains(result.Error, "raw=") {
				fmt.Fprintf(stdout, "doctor: dep %s path=%s broken error=%s raw=%q\n", entry.Name, result.Path, result.Error, raw)
			} else {
				fmt.Fprintf(stdout, "doctor: dep %s path=%s broken error=%s\n", entry.Name, result.Path, result.Error)
			}
		case deps.StateOK:
			fmt.Fprintf(stdout, "doctor: dep %s path=%s", entry.Name, result.Path)
			if result.Version != "" {
				fmt.Fprintf(stdout, " version=%s", result.Version)
			}
			if entry.MinVersion != "" {
				fmt.Fprintf(stdout, " min=%s", entry.MinVersion)
			}
			if result.SelfDoctor != "" {
				fmt.Fprintf(stdout, " self_doctor=%s", result.SelfDoctor)
			}
			fmt.Fprintln(stdout, " ok")
		default:
			warnings++
			fmt.Fprintf(stdout, "doctor: dep %s broken error=unknown probe state %q\n", entry.Name, result.State)
		}
		if result.VerboseErr != "" {
			warnings++
			fmt.Fprintf(stdout, "doctor: dep %s verbose broken error=%s\n", entry.Name, result.VerboseErr)
		}
	}
	return warnings
}

func printHookDoctor(stdout io.Writer, home string, machine config.Config) int {
	var results []installer.HookProbeResult
	if hookProbeOverride != nil {
		results = hookProbeOverride(home, machine)
	} else {
		results = installer.ProbeExpectedHooks(home, machine)
	}
	warnings := 0
	for _, result := range results {
		hook := result.Hook
		file := filepath.Base(hook.File)
		if file == "." || file == "" {
			file = "(unknown)"
		}
		prefix := fmt.Sprintf("doctor: hook %s %s %s %s", hook.Target, file, hook.Event, hook.Name)
		switch result.State {
		case "ok":
			fmt.Fprintln(stdout, prefix+" ok")
		case "missing":
			warnings++
			fmt.Fprintln(stdout, prefix+" MISSING — run pfm install")
		case "broken":
			warnings++
			fmt.Fprintf(stdout, "%s broken error=%s\n", prefix, result.Error)
		case "drift":
			warnings++
			fmt.Fprintf(stdout, "%s drift error=%s\n", prefix, result.Error)
		case "stale":
			warnings++
			fmt.Fprintln(stdout, prefix+" stale — run pfm install")
		default:
			warnings++
			fmt.Fprintf(stdout, "%s broken error=unknown hook state %q\n", prefix, result.State)
		}
	}
	return warnings
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

	// "incomplete" is the word reserved for a marker left by an interrupted
	// provision (digest.State == "incomplete"); any other lock/inventory
	// failure is a live check failure against a provision that DID finish —
	// a decode error, a stale lock, a dependency drift — and that is
	// "broken", never "incomplete".
	harvestFailureWord := "broken"
	if digest.State == "incomplete" {
		harvestFailureWord = "incomplete"
	}

	lockOK, lockErr := harvestDoctorCheck(report, "lock_completeness", checkErr)
	if lockOK && digest.LockSHA256 != "" {
		fmt.Fprintf(stdout, "doctor: harvestpy lock=(file) complete digest=%s\n", digest.LockSHA256)
	} else {
		warnings++
		if lockErr == "" {
			lockErr = "lock digest or completeness check is missing"
		}
		fmt.Fprintf(stdout, "doctor: harvestpy lock=(file) %s error=%s\n", harvestFailureWord, lockErr)
	}

	inventoryOK, inventoryErr := harvestDoctorCheck(report, "lock_completeness", checkErr)
	if inventoryOK && digest.InventorySHA256 != "" && digest.InventoryCount > 0 {
		fmt.Fprintf(stdout, "doctor: harvestpy inventory=(file) complete count=%d digest=%s\n", digest.InventoryCount, digest.InventorySHA256)
	} else {
		warnings++
		if inventoryErr == "" {
			inventoryErr = "installed inventory digest/count is missing"
		}
		fmt.Fprintf(stdout, "doctor: harvestpy inventory=(file) %s error=%s\n", harvestFailureWord, inventoryErr)
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
