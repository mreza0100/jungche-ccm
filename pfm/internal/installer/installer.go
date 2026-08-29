package installer

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	goRuntime "runtime"
	"sort"
	"strings"
	"syscall"

	"hostops/pfm/internal/codexgen"
	"hostops/pfm/internal/harvestpy"
	"hostops/pfm/internal/paths"
	"hostops/pfm/internal/shared"
)

type engine struct {
	options     Options
	report      Report
	apply       bool
	stamp       string
	managedRoot string
	outputErr   error
	planErrors  []error
}

type pinnedHarvestProvisioner struct{}

func (pinnedHarvestProvisioner) Plan(platform harvestpy.Platform) (harvestpy.InstallPlan, error) {
	return harvestpy.Plan(platform)
}

func (pinnedHarvestProvisioner) Provision(ctx context.Context, options harvestpy.ProvisionOptions) (harvestpy.ProvisionResult, error) {
	return harvestpy.Provision(ctx, options)
}

func (pinnedHarvestProvisioner) Check(ctx context.Context, root string, platform harvestpy.Platform) (harvestpy.CheckReport, error) {
	return harvestpy.Check(ctx, root, platform)
}

// NewHarvestProvisioner returns the production pinned-runtime adapter. Tests
// should inject their own HarvestProvisioner through Options instead.
func NewHarvestProvisioner() HarvestProvisioner { return pinnedHarvestProvisioner{} }

func Run(ctx context.Context, options Options) (Report, error) {
	options, err := normalize(options)
	if err != nil {
		return Report{}, fmt.Errorf("resolve installer options: %w", err)
	}
	if options.Mode > ModeUninstall {
		return Report{}, fmt.Errorf("unknown installer mode %d", options.Mode)
	}
	if options.Mode != ModeDryRun {
		if schedulerIsLaunchd {
			running, probed := launchAgentRunning(ctx, options.Runner)
			if running {
				return Report{}, ErrLaunchAgentRunning
			}
			if !probed {
				options.launchGateUnprobed = true
			}
		} else {
			running, probed := nameSyncServiceRunning(ctx, options.Runner)
			if running {
				return Report{}, ErrNameSyncRunning
			}
			if !probed {
				options.nameSyncGateUnprobed = true
			}
		}
	}

	installer := &engine{
		options:     options,
		apply:       options.Mode != ModeDryRun,
		stamp:       options.Now().Format("20060102-150405"),
		managedRoot: filepath.Join(options.Home, ".local", "share", "pfm", "install"),
	}
	installer.say("pfm install: home=%s config=%s", options.Home, options.ConfigDir)
	switch options.Mode {
	case ModeDryRun:
		installer.say("MODE: dry run — nothing will change")
	case ModeApply:
		installer.say("MODE: apply")
	case ModeUninstall:
		installer.say("MODE: uninstall")
	}
	installer.say("")
	if options.Mode == ModeApply {
		if err := installer.preflightInstall(ctx); err != nil {
			return installer.report, err
		}
	}
	if options.Mode == ModeUninstall {
		if err := installer.preflightUninstall(ctx); err != nil {
			return installer.report, err
		}
	}

	if options.Mode == ModeUninstall {
		err = installer.uninstall(ctx)
	} else {
		err = installer.install(ctx)
	}
	if len(installer.planErrors) != 0 {
		err = errors.Join(append([]error{err}, installer.planErrors...)...)
	}
	installer.say("")
	installer.say(
		"summary changed=%d ok=%d skipped=%d",
		installer.report.Changed,
		installer.report.OK,
		installer.report.Skipped,
	)
	if installer.outputErr != nil {
		err = errors.Join(err, installer.outputErr)
	}
	return installer.report, err
}

// nameSyncServiceRunning reports whether the Linux name-sync service is
// executing right now, and whether the question could be asked at all.
//
// `systemctl is-active --quiet` exits non-zero both when the service is
// inactive AND when the probe itself never ran: systemctl missing from PATH,
// the user bus unreachable, permission denied. Those are opposite answers to
// two different questions ("it is idle" vs "we do not know"), so the error is
// inspected rather than treated as one signal: an *exec.ExitError means
// systemctl started, ran to completion, and reported a status — inactive is
// proceed-safe, as before. Anything else means the probe never got an
// answer, and the caller must not read that silence as safety.
func nameSyncServiceRunning(ctx context.Context, runner CommandRunner) (running bool, probed bool) {
	err := runner.Run(ctx, "systemctl", "--user", "is-active", "--quiet", "pfm-name-sync.service")
	if err == nil {
		return true, true
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return false, true
	}
	return false, false
}

// preflightInstall executes the complete dry planner against the same host
// snapshot immediately before apply. Every planner stays read-only, but all
// conflicts and future paths are computed before installHarvest or asset
// staging can mutate the machine.
func (installer *engine) preflightInstall(ctx context.Context) error {
	options := installer.options
	options.Mode = ModeDryRun
	options.Stdout = io.Discard
	preview := &engine{
		options:     options,
		apply:       false,
		stamp:       installer.stamp,
		managedRoot: installer.managedRoot,
	}
	installErr := preview.install(ctx)
	if len(preview.planErrors) != 0 {
		installErr = errors.Join(append([]error{installErr}, preview.planErrors...)...)
	}
	if installErr != nil {
		return fmt.Errorf("preflight apply plan: %w", installErr)
	}
	if preview.outputErr != nil {
		return fmt.Errorf("preflight apply plan output: %w", preview.outputErr)
	}
	return nil
}

// preflightUninstall runs the complete removal planner before any installed
// launcher, command, hook, unit, or ownership artifact is touched. Codex
// command conflicts are therefore refusals, never half-uninstalled hosts.
func (installer *engine) preflightUninstall(ctx context.Context) error {
	options := installer.options
	options.Mode = ModeUninstall
	options.Stdout = io.Discard
	preview := &engine{
		options: options, apply: false, stamp: installer.stamp,
		managedRoot: installer.managedRoot,
	}
	uninstallErr := preview.uninstall(ctx)
	if len(preview.planErrors) != 0 {
		uninstallErr = errors.Join(append([]error{uninstallErr}, preview.planErrors...)...)
	}
	if uninstallErr != nil {
		return fmt.Errorf("preflight uninstall plan: %w", uninstallErr)
	}
	if preview.outputErr != nil {
		return fmt.Errorf("preflight uninstall plan output: %w", preview.outputErr)
	}
	return nil
}

func (installer *engine) install(ctx context.Context) error {
	if err := installer.installHarvest(ctx); err != nil {
		return err
	}
	installer.installThemes(ctx)
	assets, err := assetFiles()
	if err != nil {
		return fmt.Errorf("enumerate embedded install assets: %w", err)
	}
	systemdAssetChanged, err := installer.stageAssets(assets)
	if err != nil {
		return err
	}
	if err := installer.wireClaudeLauncher(); err != nil {
		return err
	}
	if err := installer.migrateOldState(); err != nil {
		return err
	}
	if err := installer.migrateLegacyCarrier(ctx); err != nil {
		return err
	}
	if err := installer.retirePredecessors(); err != nil {
		return err
	}
	if err := installer.retireBBInstall(); err != nil {
		return err
	}
	if err := installer.retireChatCommands(); err != nil {
		return err
	}
	if err := installer.wireCommands(assets); err != nil {
		return err
	}
	if err := installer.wireGlobalCommands(); err != nil {
		return err
	}
	if err := installer.wireGlobalSkills(); err != nil {
		return err
	}
	if err := installer.retireLegacySwapCommand(); err != nil {
		return err
	}
	if len(installer.codexHomes()) == 0 {
		installer.skip("no Codex accounts configured — command mirror has nothing to write")
		installer.skip("no Codex accounts configured — agent mirror has nothing to write")
	} else {
		if err := installer.reconcileCodexCommands(assets); err != nil {
			return err
		}
		if err := installer.wireCodexAgents(); err != nil {
			return err
		}
		if err := installer.retireOrphanCodexAgents(); err != nil {
			return err
		}
	}
	// The periodic name-sync has one job and two schedulers. Linux gets the
	// systemd units; macOS gets a launchd agent that carries both triggers.
	// Staging the other platform's files would leave an operator with a
	// ~/.config/systemd/user full of units nothing will ever read.
	if schedulerIsLaunchd {
		if installer.apply {
			if installer.options.launchGateUnprobed {
				installer.skip("launch-agent gate NOT probed (runner cannot read output); an apply during a name-sync run is not refused")
			} else {
				installer.ok("launch-agent gate: name-sync is not mid-execution")
			}
		}
		if err := installer.wireLaunchAgent(ctx); err != nil {
			return err
		}
		if err := installer.wireMCPLaunchAgent(ctx); err != nil {
			return err
		}
	} else {
		if installer.apply && installer.options.nameSyncGateUnprobed {
			installer.skip("name-sync gate NOT probed (systemctl is-active could not run); an apply during a name-sync run is not refused")
		}
		unitChanged, err := installer.wireUnits(ctx)
		if err != nil {
			return err
		}
		if systemdAssetChanged || unitChanged {
			installer.reloadUnits(ctx)
		}
	}
	if err := installer.wireSettings(); err != nil {
		return err
	}
	if err := installer.wireCodexHooks(); err != nil {
		return err
	}
	mcpErr := installer.wireMCP()
	// A host build replaces the binary without changing the unit file, and MCP
	// client wiring can change without changing either. enable --now leaves an
	// already-running process untouched, so always restart the enabled Linux
	// daemon after its complete config/client transaction has landed.
	if !schedulerIsLaunchd && installer.apply && installer.mcpAnyEnabled() && installer.userManagerAvailable(ctx) {
		installer.runSystemctl(ctx, "restart", mcpUnitName)
	}
	if mcpErr != nil {
		return mcpErr
	}
	if err := installer.wireShell(false); err != nil {
		return err
	}
	if err := installer.wireVSCode(); err != nil {
		return err
	}
	if err := installer.writeUpdateMetadata(); err != nil {
		return err
	}
	return nil
}

func (installer *engine) wireCodexAgents() error {
	sourceRepo, err := installer.globalSourceRepoRoot()
	if err != nil {
		return fmt.Errorf("resolve Codex global agents source repository: %w", err)
	}
	source := filepath.Join(sourceRepo, "templates", "global", "agents")
	if _, err := os.Stat(source); errors.Is(err, fs.ErrNotExist) {
		installer.skip("Codex global agents source absent at " + source)
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect Codex global agents source %s: %w", source, err)
	}
	plan, err := codexgen.RunGlobalAgents(codexgen.GlobalAgentsOptions{
		Home:       installer.options.Home,
		SourceRepo: sourceRepo,
		Mode:       codexgen.ModeCheck,
	})
	if err != nil {
		return fmt.Errorf("plan pfm codex agents: %w", err)
	}
	for _, action := range plan.Actions {
		message := action.Kind + " " + action.Path
		if action.Target != "" {
			message += " -> " + action.Target
		}
		if err := installer.change(message, nil); err != nil {
			return err
		}
	}
	for _, problem := range plan.Problems {
		installer.skip(problem)
	}
	if !installer.apply {
		return nil
	}
	result, err := codexgen.RunGlobalAgents(codexgen.GlobalAgentsOptions{Home: installer.options.Home, SourceRepo: sourceRepo, Mode: codexgen.ModeBuild})
	if err != nil {
		return fmt.Errorf("run pfm codex agents: %w", err)
	}
	installer.ok(fmt.Sprintf("Codex global agents compiled=%d linked=%d", len(result.Compiled), len(result.Installed)))
	return nil
}

// globalSourceRepoRoot resolves the clone every global agent/command/skill
// symlink target and conflict check is anchored on: this run's explicit
// --source-repo when set, else the marker a prior install recorded, else the
// documented default clone location (INSTALL.md's $HOME/.professor) so a
// first-ever install with neither still resolves to something concrete
// rather than refusing to plan at all.
func (installer *engine) globalSourceRepoRoot() (string, error) {
	if repo := strings.TrimSpace(installer.options.SourceRepo); repo != "" {
		abs, err := filepath.Abs(repo)
		if err != nil {
			return "", fmt.Errorf("resolve source repository %q: %w", repo, err)
		}
		return filepath.Clean(abs), nil
	}
	marker := SourceRepoPath(installer.options.Home)
	if _, err := os.Lstat(marker); errors.Is(err, fs.ErrNotExist) {
		return filepath.Clean(filepath.Join(installer.options.Home, ".professor")), nil
	} else if err != nil {
		return "", fmt.Errorf("inspect source repository marker: %w", err)
	}
	repo, err := ReadSourceRepoMarker(installer.options.Home)
	if err != nil {
		return "", err
	}
	return filepath.Clean(repo), nil
}

// retireOrphanCodexAgents deletes the exactly two known strays a retired
// "explorer" Codex agent left behind: its compiled TOML and any timestamped
// backup an earlier copy-based install wrote before this installer learned
// to symlink instead. Nothing else here is ever deleted.
func (installer *engine) retireOrphanCodexAgents() error {
	target := filepath.Join(installer.options.Home, ".codex", "agents", "explorer.toml")
	if err := installer.retire(target, "retired explorer agent"); err != nil {
		return err
	}
	matches, err := filepath.Glob(target + ".bak-*")
	if err != nil {
		return fmt.Errorf("glob retired explorer agent backups: %w", err)
	}
	for _, match := range matches {
		if err := installer.retire(match, "retired explorer agent backup"); err != nil {
			return err
		}
	}
	return nil
}

// wireGlobalCommands links every top-level entry of
// <sourceRepo>/templates/global/commands/ into {ConfigDir}/commands/: one
// file link per file entry, one whole-directory link per directory entry
// (wave/, quality/, h/, tokens/). An absent or empty source directory is
// reported and never an error — populating it is a parallel lane's job. The
// pfm-owned chat/* and reload.md links belong to a different owner and are
// never touched here.
func (installer *engine) wireGlobalCommands() error {
	sourceRepo, err := installer.globalSourceRepoRoot()
	if err != nil {
		return fmt.Errorf("resolve global commands source repository: %w", err)
	}
	source := filepath.Join(sourceRepo, "templates", "global", "commands")
	entries, err := os.ReadDir(source)
	if errors.Is(err, fs.ErrNotExist) {
		installer.skip("global commands source absent at " + source + " (0 entries)")
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect global commands source %s: %w", source, err)
	}
	if len(entries) == 0 {
		installer.skip("global commands source empty at " + source + " (0 entries)")
		return nil
	}
	target := filepath.Join(installer.options.ConfigDir, "commands")
	for _, entry := range entries {
		if err := installer.wireGlobalLink(
			filepath.Join(source, entry.Name()),
			filepath.Join(target, entry.Name()),
			sourceRepo,
			entry.IsDir(),
		); err != nil {
			return err
		}
	}
	return nil
}

// wireGlobalSkills links {Home}/.claude/skills/deep-rr to the in-tree
// engines/deep-rr skill directory. ghostwriter/vision-factory clone
// management is explicitly out of scope here — a different owner entirely.
func (installer *engine) wireGlobalSkills() error {
	sourceRepo, err := installer.globalSourceRepoRoot()
	if err != nil {
		return fmt.Errorf("resolve global skills source repository: %w", err)
	}
	source := filepath.Join(sourceRepo, "engines", "deep-rr")
	if _, err := os.Stat(filepath.Join(source, "SKILL.md")); errors.Is(err, fs.ErrNotExist) {
		installer.skip("SKILL-SOURCE-MISSING deep-rr (" + filepath.Join(source, "SKILL.md") + " absent)")
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect deep-rr skill source: %w", err)
	}
	target := filepath.Join(installer.options.Home, ".claude", "skills", "deep-rr")
	return installer.wireGlobalLink(source, target, sourceRepo, true)
}

// wireGlobalLink classifies target against source with codexgen's shared
// global-link machinery and reports/fixes it through this installer's own
// change/ok/skip transcript — the same ensureLink pattern (correct link is a
// no-op, anything else routed through installer.change so apply/dry-run
// agree), extended with the one rule ensureLink never needed: a target that
// is neither ours nor a link into the source repo is a conflict this
// installer refuses to touch.
func (installer *engine) wireGlobalLink(source, target, sourceRepoRoot string, isDir bool) error {
	kind := codexgen.GlobalLinkFile
	if isDir {
		kind = codexgen.GlobalLinkDir
	}
	state, found, err := codexgen.ClassifyGlobalLink(target, source, sourceRepoRoot, kind)
	if err != nil {
		return fmt.Errorf("inspect global link %s: %w", target, err)
	}
	if state == codexgen.GlobalLinkCorrect {
		installer.ok(target)
		return nil
	}
	message := codexgen.DescribeGlobalLinkState(state, target, source, found)
	if state == codexgen.GlobalLinkConflict {
		installer.skip(message)
		return nil
	}
	return installer.change(message, func() error {
		return codexgen.ApplyGlobalLink(target, source, state)
	})
}

func (installer *engine) reconcileCodexCommands(assets []assetFile) error {
	plan, err := installer.planCodexCommands(assets, !installer.apply)
	if err != nil {
		return err
	}
	for _, action := range plan.Actions {
		message := action.Kind + " " + action.Path
		if action.Target != "" {
			message += " -> " + action.Target
		}
		if err := installer.change(message, nil); err != nil {
			return err
		}
	}
	if blockers := codexPlanBlockers(plan.Problems); len(blockers) != 0 {
		for _, blocker := range blockers {
			installer.say("  conflict %s", blocker)
		}
		conflictErr := fmt.Errorf("reconcile Codex global commands: %s", strings.Join(blockers, "; "))
		if !installer.apply {
			installer.planErrors = append(installer.planErrors, conflictErr)
			return nil
		}
		return conflictErr
	}
	if !installer.apply {
		return nil
	}
	result, err := codexgen.RunGlobalCommands(codexgen.GlobalCommandsOptions{
		Home: installer.options.Home,
		Mode: codexgen.ModeBuild,
	})
	if err != nil {
		return fmt.Errorf("reconcile Codex global commands: %w", err)
	}
	if !result.OK {
		return fmt.Errorf("reconcile Codex global commands: %s", strings.Join(result.Problems, "; "))
	}
	installer.ok(fmt.Sprintf(
		"Codex global commands wrote=%d unchanged=%d deleted=%d",
		result.Wrote,
		result.Unchanged,
		result.Deleted,
	))
	return nil
}

func codexPlanBlockers(problems []string) []string {
	blockers := make([]string, 0)
	for _, problem := range problems {
		if strings.HasPrefix(problem, "MISSING ") || strings.HasPrefix(problem, "STALE ") || strings.HasPrefix(problem, "ORPHAN ") {
			continue
		}
		blockers = append(blockers, problem)
	}
	return blockers
}

func (installer *engine) planCodexCommands(assets []assetFile, future bool) (result codexgen.Result, returnErr error) {
	sourceHome := ""
	cleanup := func() error { return nil }
	if future && filepath.Clean(installer.options.ConfigDir) == filepath.Join(filepath.Clean(installer.options.Home), ".claude") {
		var err error
		sourceHome, cleanup, err = installer.futureCommandSource(assets)
		if err != nil {
			return codexgen.Result{}, err
		}
	}
	defer func() {
		if err := cleanup(); err != nil {
			returnErr = errors.Join(returnErr, err)
		}
	}()
	result, err := codexgen.RunGlobalCommands(codexgen.GlobalCommandsOptions{
		Home: installer.options.Home, SourceHome: sourceHome, Mode: codexgen.ModeCheck,
	})
	if err != nil {
		return codexgen.Result{}, fmt.Errorf("plan Codex global commands: %w", err)
	}
	return result, nil
}

func (installer *engine) futureCommandSource(assets []assetFile) (string, func() error, error) {
	temporary, err := os.MkdirTemp("", "pfm-install-command-plan-")
	if err != nil {
		return "", func() error { return nil }, fmt.Errorf("create Codex command plan root: %w", err)
	}
	cleanup := func() error {
		if err := os.RemoveAll(temporary); err != nil {
			return fmt.Errorf("remove Codex command plan root %s: %w", temporary, err)
		}
		return nil
	}
	fail := func(cause error) (string, func() error, error) {
		return "", func() error { return nil }, errors.Join(cause, cleanup())
	}
	source := filepath.Join(installer.options.Home, ".claude", "commands")
	target := filepath.Join(temporary, ".claude", "commands")
	if _, err := os.Lstat(source); err == nil {
		if err := copyCommandPlanTree(source, target); err != nil {
			return fail(fmt.Errorf("stage current Claude commands for preview: %w", err))
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fail(fmt.Errorf("inspect Claude commands for preview: %w", err))
	}
	for _, asset := range assets {
		commandTarget, found := installer.commandTarget(asset.path)
		if !found {
			continue
		}
		relative, err := filepath.Rel(filepath.Join(installer.options.ConfigDir, "commands"), commandTarget)
		if err != nil {
			return fail(fmt.Errorf("map planned command %s: %w", commandTarget, err))
		}
		content, err := readAsset(asset.path)
		if err != nil {
			return fail(fmt.Errorf("read planned command asset %s: %w", asset.path, err))
		}
		if err := atomicWrite(filepath.Join(target, relative), content, asset.mode); err != nil {
			return fail(fmt.Errorf("stage planned command %s: %w", relative, err))
		}
	}
	for _, retired := range []string{"bb.md", "swap.md"} {
		if err := os.RemoveAll(filepath.Join(target, retired)); err != nil {
			return fail(fmt.Errorf("retire %s from command preview: %w", retired, err))
		}
	}
	return temporary, cleanup, nil
}

// copyCommandPlanTree snapshots the current command source while omitting the
// two retired top-level commands. In particular, an old dangling /bb symlink
// must not make preview fail before the installer can report and retire it.
func copyCommandPlanTree(source, target string) error {
	if err := os.MkdirAll(target, 0o700); err != nil {
		return err
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == "bb.md" || entry.Name() == "swap.md" {
			continue
		}
		if err := copyPlanTree(filepath.Join(source, entry.Name()), filepath.Join(target, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func copyPlanTree(source, target string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		resolved, err := filepath.EvalSymlinks(source)
		if err != nil {
			return err
		}
		return copyPlanTree(resolved, target)
	}
	if info.IsDir() {
		if err := os.MkdirAll(target, 0o700); err != nil {
			return err
		}
		entries, err := os.ReadDir(source)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := copyPlanTree(filepath.Join(source, entry.Name()), filepath.Join(target, entry.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("unsupported command source mode %s", info.Mode())
	}
	content, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return atomicWrite(target, content, info.Mode().Perm())
}

func (installer *engine) uninstall(ctx context.Context) error {
	if err := installer.uninstallHarvest(); err != nil {
		return err
	}
	installer.uninstallThemes()
	assets, err := assetFiles()
	if err != nil {
		return fmt.Errorf("enumerate embedded install assets: %w", err)
	}
	if err := installer.unwireClaudeLauncher(); err != nil {
		return err
	}
	if err := installer.unwireCommands(assets); err != nil {
		return err
	}
	if err := installer.reconcileCodexCommands(nil); err != nil {
		return err
	}
	if err := installer.retireBBInstall(); err != nil {
		return err
	}
	if err := installer.retireChatCommands(); err != nil {
		return err
	}
	if schedulerIsLaunchd {
		if err := installer.unwireLaunchAgent(ctx); err != nil {
			return err
		}
	}
	managerAvailable := installer.userManagerAvailable(ctx)
	if managerAvailable && installer.apply {
		installer.runSystemctl(ctx, "disable", "--now", "pfm-name-sync.path", "pfm-name-sync.timer", mcpUnitName)
	}
	if _, err := installer.retireUnitEnablements(
		filepath.Join(installer.options.Home, ".config", "systemd", "user"),
		retiredUnitNames,
	); err != nil {
		return err
	}
	for _, enablement := range unitEnablements {
		target := filepath.Join(
			installer.options.Home,
			".config",
			"systemd",
			"user",
			enablement.wants,
			enablement.unit,
		)
		if err := installer.unlinkOne(target); err != nil {
			return err
		}
	}
	for _, name := range append(append([]string(nil), unitNames...), mcpUnitName) {
		if err := installer.unlinkOne(filepath.Join(installer.options.Home, ".config", "systemd", "user", name)); err != nil {
			return err
		}
	}
	if managerAvailable && installer.apply {
		installer.runSystemctl(ctx, "daemon-reload")
	}
	if err := installer.wireSettings(); err != nil {
		return err
	}
	if err := installer.wireCodexHooks(); err != nil {
		return err
	}
	if err := installer.wireMCP(); err != nil {
		return err
	}
	if err := installer.wireShell(true); err != nil {
		return err
	}
	if err := installer.wireVSCode(); err != nil {
		return err
	}
	if err := installer.removeManagedAssets(assets); err != nil {
		return err
	}
	if installer.apply {
		if err := installer.removeUpdateMetadata(); err != nil {
			return err
		}
	}
	return nil
}

func (installer *engine) removeUpdateMetadata() error {
	for _, path := range []string{SourceRepoPath(installer.options.Home), binaryOwnershipPath(installer.options.Home)} {
		if _, err := os.Lstat(path); errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			return err
		}
		if err := installer.change("remove "+path, func() error { return os.Remove(path) }); err != nil {
			return err
		}
	}
	if err := os.Remove(installer.managedRoot); err != nil &&
		!errors.Is(err, fs.ErrNotExist) && !errors.Is(err, syscall.ENOTEMPTY) {
		return err
	}
	return nil
}

func (installer *engine) writeUpdateMetadata() error {
	if installer.options.SourceRepo != "" {
		content, err := sourceRepoMarkerContent(installer.options.SourceRepo)
		if err != nil {
			return err
		}
		path := SourceRepoPath(installer.options.Home)
		if !sameFile(path, content, 0o600) {
			if err := installer.change("write "+path, func() error {
				return WriteSourceRepoMarker(installer.options.Home, installer.options.SourceRepo)
			}); err != nil {
				return err
			}
		} else {
			installer.ok(path)
		}
	}
	content, err := canonicalBinaryOwnershipContent(installer.options.Home)
	if err != nil {
		return err
	}
	path := binaryOwnershipPath(installer.options.Home)
	if !sameFile(path, content, 0o600) {
		if err := installer.change("write "+path, func() error {
			return RecordCanonicalBinary(installer.options.Home)
		}); err != nil {
			return err
		}
	} else {
		installer.ok(path)
	}
	return nil
}

func harvestPythonRoot(home string) string {
	return filepath.Join(home, ".local", "state", "pfm", "harvest-python")
}

func (installer *engine) harvestPlatform() harvestpy.Platform {
	platform := installer.options.HarvestPlatform
	if platform.GOOS == "" {
		platform.GOOS, platform.GOARCH = goRuntime.GOOS, goRuntime.GOARCH
	}
	return platform
}

func (installer *engine) installHarvest(ctx context.Context) error {
	if !installer.options.ProvisionHarvest {
		if installer.apply {
			installer.say("harvestpy: skipped (blocked, not attempted)")
		} else {
			installer.say("harvestpy: would skip (blocked, not attempted)")
		}
		return nil
	}
	provider := installer.options.HarvestProvisioner
	if provider == nil {
		return errors.New("harvestpy provisioning is enabled but no provisioner is configured")
	}
	platform := installer.harvestPlatform()
	plan, err := provider.Plan(platform)
	if err != nil {
		return fmt.Errorf("harvestpy install plan for %s: %w", platform, err)
	}
	installer.say(
		"harvestpy plan: platform=%s Python %s uv=%s uv_download_bytes=%d python_download_bytes=%d package_download_bytes=%d environment_bytes=%d package_status=%s offline=%t",
		plan.Platform,
		plan.PythonVersion,
		plan.UVVersion,
		plan.UVDownloadBytes,
		plan.PythonDownloadBytes,
		plan.PackageDownloadBytes,
		plan.EnvironmentBytes,
		plan.PackageDownloadStatus,
		installer.options.HarvestOffline,
	)
	if len(plan.PackageBlockers) != 0 || plan.PackageDownloadStatus == "blocked-exact-lock" {
		installer.say("harvestpy plan: BLOCKED exact lock for %s", platform)
		for _, blocker := range plan.PackageBlockers {
			installer.say("  blocker %s", blocker)
		}
		return fmt.Errorf("harvestpy target %s is blocked-exact-lock", platform)
	}
	if !installer.apply {
		installer.say("harvestpy dry-run: Plan only; writes=0 network=0 offline=%t (apply requires valid cached pins or network)", installer.options.HarvestOffline)
		return nil
	}
	root := harvestPythonRoot(installer.options.Home)
	check, checkErr := provider.Check(ctx, root, platform)
	if checkErr == nil && check.Healthy {
		installer.ok("harvestpy environment already healthy (Check fast-path; no download)")
		return nil
	}
	if checkErr != nil {
		installer.say("harvestpy Check did not establish a healthy environment; provisioning: %v", checkErr)
	} else {
		installer.say("harvestpy Check reported an unhealthy environment; provisioning")
	}
	result, provisionErr := provider.Provision(ctx, harvestpy.ProvisionOptions{
		Root:     root,
		Cache:    filepath.Join(root, "cache"),
		Platform: platform,
		Offline:  installer.options.HarvestOffline,
	})
	if provisionErr != nil {
		if errors.Is(provisionErr, harvestpy.ErrOfflineUnavailable) {
			installer.say("harvestpy offline: required pinned input is not in the verified cached inputs; prewarm the cache or rerun with network")
			return fmt.Errorf("harvestpy provisioning offline; verified cached inputs are unavailable: %w", provisionErr)
		}
		return fmt.Errorf("harvestpy provision %s: %w", platform, provisionErr)
	}
	installer.ok("harvestpy environment provisioned digest=" + result.Digest)
	return nil
}

func (installer *engine) uninstallHarvest() error {
	if !installer.options.ProvisionHarvest {
		return nil
	}
	root := harvestPythonRoot(installer.options.Home)
	info, err := os.Lstat(root)
	if errors.Is(err, fs.ErrNotExist) {
		installer.ok("managed harvestpy runtime and cache absent")
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect managed harvestpy root: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("refuse to remove managed harvestpy root that is not a directory: %s", root)
	}
	hasManaged := false
	for _, name := range []string{"env", "cache"} {
		if _, statErr := os.Lstat(filepath.Join(root, name)); statErr == nil {
			hasManaged = true
		} else if !errors.Is(statErr, fs.ErrNotExist) {
			return fmt.Errorf("inspect managed harvestpy %s: %w", name, statErr)
		}
	}
	if !hasManaged {
		installer.ok("managed harvestpy runtime and cache absent")
		return nil
	}
	return installer.change("remove managed harvestpy runtime and cache", func() error {
		for _, name := range []string{"env", "cache"} {
			if err := os.RemoveAll(filepath.Join(root, name)); err != nil {
				return fmt.Errorf("remove managed harvestpy %s: %w", name, err)
			}
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			return fmt.Errorf("inspect managed harvestpy root after removal: %w", err)
		}
		if len(entries) == 0 {
			if err := os.Remove(root); err != nil {
				return fmt.Errorf("remove empty managed harvestpy root: %w", err)
			}
		}
		return nil
	})
}

func (installer *engine) say(format string, arguments ...any) {
	if installer.outputErr != nil {
		return
	}
	_, installer.outputErr = fmt.Fprintf(installer.options.Stdout, format+"\n", arguments...)
}

func (installer *engine) change(message string, action func() error) error {
	installer.say("  change  %s", message)
	installer.report.Changed++
	if !installer.apply || action == nil {
		return nil
	}
	return action()
}

// changeDescription selects the message installer.change reports for a
// conditional rewrite: "create" when nothing existed to back up, and
// "rewrite ... (backup preserved)" only when a backup will actually be
// written. Callers must pass the SAME existed value that gates the backup —
// deriving it a second, independent way is how the message and the write it
// describes drift apart.
func changeDescription(path string, existed bool) string {
	if !existed {
		return "create " + path
	}
	return "rewrite " + path + " (backup preserved)"
}

func (installer *engine) ok(message string) {
	installer.say("  ok      %s", message)
	installer.report.OK++
}

func (installer *engine) skip(message string) {
	installer.say("  skip    %s", message)
	installer.report.Skipped++
}

func (installer *engine) stageAssets(assets []assetFile) (bool, error) {
	installer.say("embedded assets -> %s", installer.managedRoot)
	systemdChanged := false
	for _, asset := range assets {
		if mcpSchedulerAsset(asset.path) && !installer.mcpAnyEnabled() {
			continue
		}
		content, err := readAsset(asset.path)
		if err != nil {
			return false, fmt.Errorf("read embedded asset %s: %w", asset.path, err)
		}
		if asset.path == "shim/pfm.zsh" {
			content, err = renderShimAsset(content, installer.options)
		} else if asset.path == "bin/claude" {
			content, err = renderClaudeLauncherAsset(content, installer.options)
		}
		if err != nil {
			return false, fmt.Errorf("render embedded asset %s: %w", asset.path, err)
		}
		target := filepath.Join(installer.managedRoot, filepath.FromSlash(asset.path))
		if sameFile(target, content, asset.mode) {
			installer.ok(target)
			continue
		}
		if strings.HasPrefix(asset.path, "systemd/") {
			systemdChanged = true
		}
		if err := installer.change("write "+target, func() error {
			return atomicWrite(target, content, asset.mode)
		}); err != nil {
			return false, err
		}
	}
	if !installer.mcpAnyEnabled() {
		for _, relative := range []string{"systemd/pfm-mcp.service", "launchd/com.professor.pfm.mcp.plist"} {
			mcpAsset := filepath.Join(installer.managedRoot, filepath.FromSlash(relative))
			if _, err := os.Lstat(mcpAsset); err == nil {
				if err := installer.change("remove "+mcpAsset, func() error { return os.Remove(mcpAsset) }); err != nil {
					return false, err
				}
				systemdChanged = systemdChanged || strings.HasPrefix(relative, "systemd/")
			} else if !errors.Is(err, fs.ErrNotExist) {
				return false, err
			}
		}
	}
	installer.say("")
	return systemdChanged, nil
}

func (installer *engine) removeManagedAssets(assets []assetFile) error {
	installer.say("managed assets -> %s", installer.managedRoot)
	for _, asset := range assets {
		target := filepath.Join(installer.managedRoot, filepath.FromSlash(asset.path))
		if _, err := os.Lstat(target); errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			return err
		}
		if err := installer.change("remove "+target, func() error { return os.Remove(target) }); err != nil {
			return err
		}
	}
	if installer.apply {
		directories := make(map[string]bool)
		for _, asset := range assets {
			for directory := filepath.Dir(filepath.Join(installer.managedRoot, filepath.FromSlash(asset.path))); strings.HasPrefix(directory, installer.managedRoot); directory = filepath.Dir(directory) {
				directories[directory] = true
				if directory == installer.managedRoot {
					break
				}
			}
		}
		ordered := make([]string, 0, len(directories))
		for directory := range directories {
			ordered = append(ordered, directory)
		}
		sort.Slice(ordered, func(left, right int) bool {
			return strings.Count(ordered[left], string(filepath.Separator)) >
				strings.Count(ordered[right], string(filepath.Separator))
		})
		for _, directory := range ordered {
			if err := os.Remove(directory); err != nil && !errors.Is(err, fs.ErrNotExist) && !errors.Is(err, fs.ErrInvalid) {
				if !errors.Is(err, fs.ErrExist) {
					installer.skip("leave non-empty managed directory " + directory + ": " + err.Error())
				}
			}
		}
	}
	return nil
}

func (installer *engine) ensureLink(source, target string) (bool, error) {
	if current, linked := resolvedLink(target); linked && current == filepath.Clean(source) {
		installer.ok(target)
		return false, nil
	}
	info, err := os.Lstat(target)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	description := "link " + target + " -> " + source
	return true, installer.change(description, func() error {
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		if info != nil {
			if info.Mode()&os.ModeSymlink != 0 {
				if err := os.Remove(target); err != nil {
					return err
				}
			} else {
				backup := availableBackup(target, installer.stamp)
				if err := os.Rename(target, backup); err != nil {
					return fmt.Errorf("backup %s to %s: %w", target, backup, err)
				}
			}
		}
		return os.Symlink(source, target)
	})
}

func (installer *engine) unlinkOne(target string) error {
	_, linked := resolvedLink(target)
	if !linked {
		installer.skip(target + " is not an installed link")
		return nil
	}
	backup := newestBackup(target)
	if backup == "" {
		return installer.change("remove "+target, func() error { return os.Remove(target) })
	}
	return installer.change("restore "+target+" from "+filepath.Base(backup), func() error {
		if err := os.Remove(target); err != nil {
			return err
		}
		return os.Rename(backup, target)
	})
}

func (installer *engine) retire(path, reason string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("refuse to retire directory %s", path)
	}
	return installer.change("retire "+path+" ("+reason+")", func() error { return os.Remove(path) })
}

func (installer *engine) retireGlob(pattern, reason string) error {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	for _, match := range matches {
		if err := installer.retire(match, reason); err != nil {
			return err
		}
	}
	return nil
}

func (installer *engine) wireCommands(assets []assetFile) error {
	installer.say("commands -> %s", filepath.Join(installer.options.ConfigDir, "commands"))
	for _, asset := range assets {
		target, found := installer.commandTarget(asset.path)
		if !found {
			continue
		}
		source := filepath.Join(installer.managedRoot, filepath.FromSlash(asset.path))
		if _, err := installer.ensureLink(source, target); err != nil {
			return err
		}
	}
	installer.say("")
	return nil
}

func (installer *engine) unwireCommands(assets []assetFile) error {
	installer.say("commands -> %s", filepath.Join(installer.options.ConfigDir, "commands"))
	for _, asset := range assets {
		target, found := installer.commandTarget(asset.path)
		if !found {
			continue
		}
		if err := installer.unlinkOne(target); err != nil {
			return err
		}
	}
	if err := installer.retireLegacySwapCommand(); err != nil {
		return err
	}
	installer.say("")
	return nil
}

func (installer *engine) commandTarget(asset string) (string, bool) {
	commands := filepath.Join(installer.options.ConfigDir, "commands")
	switch asset {
	case "reload.command.md":
		return filepath.Join(commands, "reload.md"), true
	}
	return "", false
}

func (installer *engine) retireLegacySwapCommand() error {
	return installer.unlinkOne(filepath.Join(installer.options.ConfigDir, "commands", "swap.md"))
}

func (installer *engine) retireBBInstall() error {
	installer.say("retired /bb surfaces")
	sourceRepos, err := installer.recordedProfessorSourceRepos()
	if err != nil {
		return err
	}
	for _, link := range []struct {
		target         string
		managedSource  string
		legacyRelative string
	}{
		// frozen historical paths — pre-rename installs' /bb symlinks point here; never rename with the repo
		{
			target:         filepath.Join(installer.options.ConfigDir, "commands", "bb.md"),
			managedSource:  filepath.Join(installer.managedRoot, "bb.command.md"),
			legacyRelative: filepath.Join("blueprint", "templates", "host-swap", "bb.command.md"),
		},
		{
			target:         filepath.Join(installer.options.Home, ".agents", "skills", "bb"),
			managedSource:  filepath.Join(installer.managedRoot, "codex-skills", "bb"),
			legacyRelative: filepath.Join("blueprint", "templates", "host-swap", "codex-skills", "bb"),
		},
	} {
		current, linked := resolvedLink(link.target)
		owned := linked && current == filepath.Clean(link.managedSource)
		for _, repo := range sourceRepos {
			if linked && current == filepath.Join(repo, link.legacyRelative) {
				owned = true
				break
			}
		}
		if !owned {
			installer.skip(link.target + " is not an installed /bb link")
			continue
		}
		if err := installer.unlinkOne(link.target); err != nil {
			return err
		}
	}
	for _, relative := range []string{
		"bb.command.md",
		"codex-skills/bb/SKILL.md",
		"codex-skills/bb/agents/openai.yaml",
	} {
		if err := installer.retire(filepath.Join(installer.managedRoot, filepath.FromSlash(relative)), "retired /bb surface"); err != nil {
			return err
		}
	}
	for _, relative := range []string{"codex-skills/bb/agents", "codex-skills/bb", "codex-skills"} {
		if err := installer.retireEmptyDirectory(filepath.Join(installer.managedRoot, filepath.FromSlash(relative))); err != nil {
			return err
		}
	}
	installer.say("")
	return nil
}

// retireChatCommands retires the 19 /chat:* Claude slash commands and the
// chat.sh/history.sh compatibility scripts they shared a directory with —
// every one of them superseded by the chat MCP tools, with history.sh's
// behavior ported natively into `pfm chat history`. Same discipline as
// retireBBInstall: only a link this installer's own managed root actually
// owns gets unlinked, never a path the operator happens to have there.
func (installer *engine) retireChatCommands() error {
	installer.say("retired /chat: slash commands")
	commands := filepath.Join(installer.options.ConfigDir, "commands")
	for _, link := range []struct {
		target        string
		managedSource string
	}{
		{filepath.Join(commands, "chat", "branch.md"), filepath.Join(installer.managedRoot, "chat", "branch.command.md")},
		{filepath.Join(commands, "chat", "capture.md"), filepath.Join(installer.managedRoot, "chat", "capture.command.md")},
		{filepath.Join(commands, "chat", "find.md"), filepath.Join(installer.managedRoot, "chat", "find.command.md")},
		{filepath.Join(commands, "chat", "goal.md"), filepath.Join(installer.managedRoot, "chat", "goal.command.md")},
		{filepath.Join(commands, "chat", "inject.md"), filepath.Join(installer.managedRoot, "chat", "inject.command.md")},
		{filepath.Join(commands, "chat", "interrogate.md"), filepath.Join(installer.managedRoot, "chat", "interrogate.command.md")},
		{filepath.Join(commands, "chat", "load.md"), filepath.Join(installer.managedRoot, "chat", "load.command.md")},
		{filepath.Join(commands, "chat", "ls.md"), filepath.Join(installer.managedRoot, "chat", "ls.command.md")},
		{filepath.Join(commands, "chat", "new.md"), filepath.Join(installer.managedRoot, "chat", "new.command.md")},
		{filepath.Join(commands, "chat", "read.md"), filepath.Join(installer.managedRoot, "chat", "read.command.md")},
		{filepath.Join(commands, "chat", "save.md"), filepath.Join(installer.managedRoot, "chat", "save.command.md")},
		{filepath.Join(commands, "chat", "whoami.md"), filepath.Join(installer.managedRoot, "chat", "whoami.command.md")},
		{filepath.Join(commands, "chat", "group", "create.md"), filepath.Join(installer.managedRoot, "chat", "group", "create.command.md")},
		{filepath.Join(commands, "chat", "group", "invite.md"), filepath.Join(installer.managedRoot, "chat", "group", "invite.command.md")},
		{filepath.Join(commands, "chat", "group", "ls.md"), filepath.Join(installer.managedRoot, "chat", "group", "ls.command.md")},
		{filepath.Join(commands, "chat", "group", "read.md"), filepath.Join(installer.managedRoot, "chat", "group", "read.command.md")},
		{filepath.Join(commands, "chat", "group", "send.md"), filepath.Join(installer.managedRoot, "chat", "group", "send.command.md")},
		{filepath.Join(commands, "chat", "group", "subscribe.md"), filepath.Join(installer.managedRoot, "chat", "group", "subscribe.command.md")},
		{filepath.Join(commands, "chat", "self", "compact.md"), filepath.Join(installer.managedRoot, "chat", "self", "compact.command.md")},
		{filepath.Join(commands, "chat", "chat.sh"), filepath.Join(installer.managedRoot, "chat", "chat.sh")},
		{filepath.Join(commands, "chat", "history.sh"), filepath.Join(installer.managedRoot, "chat", "history.sh")},
	} {
		current, linked := resolvedLink(link.target)
		if !linked || current != filepath.Clean(link.managedSource) {
			installer.skip(link.target + " is not an installed /chat: link")
			continue
		}
		if err := installer.unlinkOne(link.target); err != nil {
			return err
		}
	}
	for _, relative := range []string{
		"chat/branch.command.md", "chat/capture.command.md", "chat/find.command.md",
		"chat/goal.command.md", "chat/inject.command.md", "chat/interrogate.command.md",
		"chat/load.command.md", "chat/ls.command.md", "chat/new.command.md",
		"chat/read.command.md", "chat/save.command.md", "chat/whoami.command.md",
		"chat/group/create.command.md", "chat/group/invite.command.md", "chat/group/ls.command.md",
		"chat/group/read.command.md", "chat/group/send.command.md", "chat/group/subscribe.command.md",
		"chat/self/compact.command.md", "chat/chat.sh", "chat/history.sh",
	} {
		if err := installer.retire(filepath.Join(installer.managedRoot, filepath.FromSlash(relative)), "retired /chat: slash command — superseded by the chat MCP tools"); err != nil {
			return err
		}
	}
	for _, relative := range []string{"chat/group", "chat/self", "chat"} {
		if err := installer.retireEmptyDirectory(filepath.Join(installer.managedRoot, filepath.FromSlash(relative))); err != nil {
			return err
		}
	}
	// The host side is never unconditionally empty the way managedRoot is: an
	// operator's own file, or one of the links above skipped as unowned,
	// legitimately survives here. retireEmptyDirectory's hard failure on a
	// non-empty directory is right for managedRoot's fully pfm-owned tree; it
	// would wrongly abort every future install for an operator with one
	// leftover file, so the host cleanup is best-effort instead.
	for _, relative := range []string{"chat/group", "chat/self", "chat"} {
		if err := installer.retireEmptyDirectoryTolerant(filepath.Join(commands, filepath.FromSlash(relative))); err != nil {
			return err
		}
	}
	installer.say("")
	return nil
}

func (installer *engine) recordedProfessorSourceRepos() ([]string, error) {
	repos := make([]string, 0, 2)
	seen := map[string]bool{}
	add := func(repo string) {
		repo = filepath.Clean(strings.TrimSpace(repo))
		if repo != "." && !seen[repo] {
			seen[repo] = true
			repos = append(repos, repo)
		}
	}
	add(installer.options.SourceRepo)
	marker := SourceRepoPath(installer.options.Home)
	if _, err := os.Lstat(marker); errors.Is(err, fs.ErrNotExist) {
		return repos, nil
	} else if err != nil {
		return nil, fmt.Errorf("inspect source repository marker for /bb retirement: %w", err)
	}
	repo, err := ReadSourceRepoMarker(installer.options.Home)
	if err != nil {
		return nil, fmt.Errorf("read source repository marker for /bb retirement: %w", err)
	}
	add(repo)
	return repos, nil
}

func (installer *engine) retireEmptyDirectory(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refuse to retire non-directory %s", path)
	}
	if installer.apply {
		entries, readErr := os.ReadDir(path)
		if readErr != nil {
			return readErr
		}
		if len(entries) != 0 {
			return fmt.Errorf("refuse to retire non-empty directory %s", path)
		}
	}
	return installer.change("remove empty "+path, func() error { return os.Remove(path) })
}

// retireEmptyDirectoryTolerant removes path only once it has actually turned
// out empty. Unlike retireEmptyDirectory, a directory left non-empty is a
// routine, visible skip rather than a hard failure — the same tolerant
// os.Remove-and-skip idiom removeManagedAssets uses for its own directory
// sweep, appropriate wherever content the operator (not this installer)
// controls can legitimately remain.
func (installer *engine) retireEmptyDirectoryTolerant(path string) error {
	if !installer.apply {
		return nil
	}
	if err := os.Remove(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		installer.skip("leave non-empty directory " + path + ": " + err.Error())
		return nil
	}
	installer.ok("removed empty " + path)
	return nil
}

var unitNames = []string{
	"pfm-name-sync.path",
	"pfm-name-sync.service",
	"pfm-name-sync.timer",
}

const mcpUnitName = "pfm-mcp.service"

var retiredUnitNames = []string{
	"cc-fleet-name-sync.path", "cc-fleet-name-sync.service", "cc-fleet-name-sync.timer",
	"cc-name-sync.path", "cc-name-sync.service", "cc-name-sync.timer",
}

var unitEnablements = []struct {
	unit  string
	wants string
}{
	{unit: "pfm-name-sync.path", wants: "default.target.wants"},
	{unit: "pfm-name-sync.timer", wants: "timers.target.wants"},
	{unit: mcpUnitName, wants: "default.target.wants"},
}

func (installer *engine) wireUnits(ctx context.Context) (bool, error) {
	directory := filepath.Join(installer.options.Home, ".config", "systemd", "user")
	installer.say("systemd user units -> %s", directory)
	managerAvailable := installer.userManagerAvailable(ctx)
	changed := false
	enablementChanged, err := installer.retireUnitEnablements(directory, retiredUnitNames)
	if err != nil {
		return false, err
	}
	changed = changed || enablementChanged
	for _, name := range retiredUnitNames {
		target := filepath.Join(directory, name)
		_, statErr := os.Lstat(target)
		if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
			return false, fmt.Errorf("inspect retired unit %s: %w", target, statErr)
		}
		known := statErr == nil
		if managerAvailable && !known {
			known = installer.unitKnown(ctx, name)
		}
		if !known {
			continue
		}
		if managerAvailable && installer.apply {
			installer.runSystemctl(ctx, "disable", "--now", name)
			installer.runSystemctl(ctx, "reset-failed", name)
		}
		if statErr == nil {
			if err := installer.retire(target, "superseded by pfm-name-sync"); err != nil {
				return false, err
			}
			changed = true
		}
	}
	managedNames := append([]string(nil), unitNames...)
	if installer.mcpAnyEnabled() {
		managedNames = append(managedNames, mcpUnitName)
	} else {
		mcpTarget := filepath.Join(directory, mcpUnitName)
		if _, err := os.Lstat(mcpTarget); err == nil {
			if managerAvailable && installer.apply {
				installer.runSystemctl(ctx, "disable", "--now", mcpUnitName)
			}
			if err := installer.unlinkOne(mcpTarget); err != nil {
				return false, err
			}
			changed = true
		} else if !errors.Is(err, fs.ErrNotExist) {
			return false, err
		}
	}
	for _, name := range managedNames {
		source := filepath.Join(installer.managedRoot, "systemd", name)
		target := filepath.Join(directory, name)
		linkChanged, err := installer.ensureLink(source, target)
		if err != nil {
			return false, err
		}
		changed = changed || linkChanged
	}
	for _, enablement := range unitEnablements {
		source := filepath.Join(directory, enablement.unit)
		target := filepath.Join(directory, enablement.wants, enablement.unit)
		if enablement.unit == mcpUnitName && !installer.mcpAnyEnabled() {
			info, err := os.Lstat(target)
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil {
				return false, err
			}
			if info.Mode()&os.ModeSymlink == 0 {
				installer.skip("unexpected non-symlink MCP enablement left untouched: " + target)
				continue
			}
			if err := installer.change("remove "+target, func() error { return os.Remove(target) }); err != nil {
				return false, err
			}
			changed = true
			continue
		}
		linkChanged, err := installer.ensureLink(source, target)
		if err != nil {
			return false, err
		}
		changed = changed || linkChanged
	}
	if !managerAvailable {
		installer.skip("systemd --user unavailable; units are staged and enabled for next login but not started now")
	}
	installer.say("")
	return changed, nil
}

func (installer *engine) retireUnitEnablements(directory string, names []string) (bool, error) {
	changed := false
	for _, wants := range []string{"default.target.wants", "timers.target.wants"} {
		for _, name := range names {
			target := filepath.Join(directory, wants, name)
			info, err := os.Lstat(target)
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil {
				return false, fmt.Errorf("inspect retired enablement %s: %w", target, err)
			}
			if info.Mode()&os.ModeSymlink == 0 {
				installer.skip("unexpected non-symlink retired enablement left untouched: " + target)
				continue
			}
			if err := installer.retire(target, "superseded systemd enablement"); err != nil {
				return false, err
			}
			changed = true
		}
	}
	return changed, nil
}

func (installer *engine) reloadUnits(ctx context.Context) {
	if !installer.apply || !installer.userManagerAvailable(ctx) {
		return
	}
	installer.runSystemctl(ctx, "daemon-reload")
	installer.runSystemctl(ctx, "enable", "--now", "pfm-name-sync.path", "pfm-name-sync.timer")
	if installer.mcpAnyEnabled() {
		installer.runSystemctl(ctx, "enable", "--now", mcpUnitName)
	}
}

func (installer *engine) userManagerAvailable(ctx context.Context) bool {
	return installer.options.Runner.Run(ctx, "systemctl", "--user", "show-environment") == nil
}

func (installer *engine) unitKnown(ctx context.Context, unit string) bool {
	for _, command := range []string{"is-active", "is-enabled", "is-failed"} {
		if installer.options.Runner.Run(ctx, "systemctl", "--user", command, "--quiet", unit) == nil {
			return true
		}
	}
	return false
}

func (installer *engine) runSystemctl(ctx context.Context, arguments ...string) {
	args := append([]string{"--user"}, arguments...)
	if err := installer.options.Runner.Run(ctx, "systemctl", args...); err != nil {
		installer.skip("systemctl " + strings.Join(args, " ") + " failed: " + err.Error())
	}
}

func (installer *engine) wireSettings() error {
	ownershipPath := filepath.Join(installer.managedRoot, "settings-hook-ownership.json")
	ownership, ownershipRaw, err := readSettingsHookOwnership(ownershipPath)
	if err != nil {
		return fmt.Errorf("read settings hook ownership %s: %w", ownershipPath, err)
	}
	candidates := make([]string, 0, len(installer.options.ConfigDirs)+1)
	if installer.options.ConfigDirs == nil {
		candidates = append(candidates, filepath.Join(installer.options.ConfigDir, "settings.json"))
	} else {
		for _, configDir := range installer.options.ConfigDirs {
			if strings.TrimSpace(configDir) == "" {
				continue
			}
			candidates = append(candidates, filepath.Join(configDir, "settings.json"))
		}
	}
	if len(candidates) == 0 {
		installer.skip("no Claude account config dirs configured — settings hook wiring has nothing to wire")
	}
	seen := map[string]bool{}
	seenOwnershipPaths := map[string]bool{}
	// Codex hooks share this ledger but are converged by wireCodexHooks after
	// Claude settings. They are therefore visited, just not by this loop.
	for _, codexHome := range installer.codexHomes() {
		seenOwnershipPaths[physicalSettingsPath(filepath.Join(codexHome, "hooks.json"))] = true
	}
	for _, candidate := range candidates {
		physical, err := filepath.EvalSymlinks(candidate)
		if err != nil {
			physical = candidate
		}
		physical = filepath.Clean(physical)
		if seen[physical] {
			continue
		}
		seen[physical] = true
		seenOwnershipPaths[physical] = true
		raw, err := os.ReadFile(candidate)
		if errors.Is(err, fs.ErrNotExist) {
			if installer.options.Mode == ModeUninstall {
				delete(ownership, physical)
			}
			installer.skip("no settings file at " + candidate)
			continue
		}
		if err != nil {
			return fmt.Errorf("read %s: %w", candidate, err)
		}
		updated, changed, nextOwned, err := updateSettings(
			raw,
			installer.options.Home,
			installer.options.Mode == ModeUninstall,
			ownership[physical],
		)
		if err != nil {
			if installer.options.Mode == ModeUninstall && len(ownership[physical]) > 0 {
				return fmt.Errorf("refuse to strand owned hooks in invalid settings JSON at %s: %w", candidate, err)
			}
			installer.skip("invalid settings JSON at " + candidate + ": " + err.Error())
			continue
		}
		if installer.options.Mode != ModeUninstall && hasPreservedMixedExploreDenyMatcher(updated, filepath.Join(installer.options.Home, ".local", "bin", "pfm")) {
			installer.skip("mixed PreToolUse hook entry preserved with its existing matcher at " + candidate)
		}
		if len(nextOwned) == 0 {
			delete(ownership, physical)
		} else {
			ownership[physical] = nextOwned
		}
		if !changed {
			installer.ok(candidate + " wiring")
			continue
		}
		if err := installer.change("rewrite "+candidate+" (backup preserved)", func() error {
			backup := availableBackup(candidate, installer.stamp)
			if err := copyBackup(candidate, backup); err != nil {
				return fmt.Errorf("backup %s: %w", candidate, err)
			}
			return atomicWrite(candidate, updated, 0o600)
		}); err != nil {
			return err
		}
	}
	if installer.options.Mode == ModeUninstall {
		for path := range ownership {
			if seenOwnershipPaths[path] {
				continue
			}
			if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
				delete(ownership, path)
				continue
			} else if err != nil {
				return fmt.Errorf("inspect unvisited owned settings path %s: %w", path, err)
			}
			return fmt.Errorf("refuse to strand hooks in unvisited owned settings path %s", path)
		}
	}
	return installer.writeSettingsHookOwnership(ownershipPath, ownershipRaw, ownership)
}

func (installer *engine) writeSettingsHookOwnership(
	path string,
	existing []byte,
	ownership map[string]settingsHookCounts,
) error {
	if len(ownership) == 0 {
		if len(existing) == 0 {
			installer.ok(path)
			return nil
		}
		return installer.change("remove "+path, func() error { return os.Remove(path) })
	}
	same, encoded, err := sameSettingsHookOwnership(existing, ownership)
	if err != nil {
		return err
	}
	if same && sameFile(path, encoded, 0o600) {
		installer.ok(path)
		return nil
	}
	return installer.change("write "+path, func() error {
		return atomicWrite(path, encoded, 0o600)
	})
}

func (installer *engine) wireCodexHooks() error {
	ownershipPath := filepath.Join(installer.managedRoot, "settings-hook-ownership.json")
	ownership, ownershipRaw, err := readSettingsHookOwnership(ownershipPath)
	if err != nil {
		return fmt.Errorf("read settings hook ownership %s: %w", ownershipPath, err)
	}
	seen := map[string]bool{}
	if len(installer.codexHomes()) == 0 {
		installer.skip("no Codex accounts configured — hooks.json wiring has nothing to wire")
	}
	for _, codexHome := range installer.codexHomes() {
		path := filepath.Join(codexHome, "hooks.json")
		physical := physicalSettingsPath(path)
		if seen[physical] {
			continue
		}
		seen[physical] = true
		raw, readErr := os.ReadFile(path)
		existed := true
		if errors.Is(readErr, fs.ErrNotExist) {
			existed = false
			raw = []byte("{\"hooks\":{}}\n")
			if installer.options.Mode == ModeUninstall {
				delete(ownership, physical)
				continue
			}
		} else if readErr != nil {
			return fmt.Errorf("read %s: %w", path, readErr)
		}
		updated, changed, nextOwned, updateErr := updateCodexHooks(
			raw,
			installer.options.Home,
			installer.options.Mode == ModeUninstall,
			ownership[physical],
		)
		if updateErr != nil {
			if installer.options.Mode == ModeUninstall && len(ownership[physical]) > 0 {
				return fmt.Errorf("refuse to strand owned hooks in invalid Codex hooks JSON at %s: %w", path, updateErr)
			}
			installer.skip("invalid Codex hooks JSON at " + path + ": " + updateErr.Error())
			continue
		}
		if len(nextOwned) == 0 {
			delete(ownership, physical)
		} else {
			ownership[physical] = nextOwned
		}
		if !changed {
			installer.ok(path + " wiring")
			continue
		}
		if err := installer.change(changeDescription(path, existed), func() error {
			if existed {
				backup := availableBackup(path, installer.stamp)
				if err := copyBackup(path, backup); err != nil {
					return fmt.Errorf("backup %s: %w", path, err)
				}
			}
			return atomicWrite(path, updated, 0o600)
		}); err != nil {
			return err
		}
	}
	return installer.writeSettingsHookOwnership(ownershipPath, ownershipRaw, ownership)
}

func (installer *engine) codexHomes() []string {
	if installer.options.CodexHomes == nil {
		return []string{filepath.Join(installer.options.Home, ".codex")}
	}
	return installer.options.CodexHomes
}

func (installer *engine) wireShell(uninstall bool) error {
	zshrc := filepath.Join(installer.options.Home, ".zshrc")
	shim := filepath.Join(installer.managedRoot, "shim", "pfm.zsh")
	wanted := sourceLine(shim)
	raw, err := os.ReadFile(zshrc)
	if errors.Is(err, fs.ErrNotExist) {
		raw = nil
	} else if err != nil {
		return err
	}
	if !uninstall {
		installer.reportEarlyFleetCalls(string(raw))
	}
	updated := rewriteZshrc(string(raw), wanted, uninstall)
	if string(raw) == updated {
		installer.ok(zshrc + " source line")
		return nil
	}
	description := "rewrite " + zshrc + " (backup preserved)"
	if len(raw) == 0 {
		description = "create " + zshrc
	}
	return installer.change(description, func() error {
		if len(raw) > 0 {
			backup := availableBackup(zshrc, installer.stamp)
			if err := copyBackup(zshrc, backup); err != nil {
				return err
			}
		}
		if uninstall && updated == "" {
			return os.Remove(zshrc)
		}
		return atomicWrite(zshrc, []byte(updated), 0o600)
	})
}

// reportEarlyFleetCalls names a fleet command CALLED above the source line — a
// class that cannot announce itself, because `cc` and `cx` are names the system
// already owns. It only reports: a line of the operator's own shell that is not
// ours is never rewritten from here.
func (installer *engine) reportEarlyFleetCalls(content string) {
	early := earlyFleetCalls(content)
	if len(early) == 0 {
		installer.ok("nothing above the source line calls a fleet command")
		return
	}
	installer.skip("a fleet command runs ABOVE the source line, where it does not exist yet:")
	for _, line := range early {
		installer.say("          %s", line)
	}
	installer.say("          There, `cc` is /usr/bin/cc — the C compiler — not this fleet.")
	installer.say("          Move those lines BELOW the source line. For a terminal profile that")
	installer.say("          opens a chat on launch, delete them instead and set CC_AUTO_OPEN=1 in")
	installer.say("          the profile's env: the shim runs that hook itself, last, once the")
	installer.say("          launchers are defined.")
}

func (installer *engine) migrateOldState() error {
	oldState := filepath.Join(installer.options.Home, ".local", "state", "cc-fleet")
	state := filepath.Join(installer.options.Home, ".local", "state", "pfm")
	oldInfo, oldErr := os.Stat(oldState)
	_, stateErr := os.Stat(state)
	if oldErr == nil && oldInfo.IsDir() && errors.Is(stateErr, fs.ErrNotExist) {
		return installer.change("migrate "+oldState+" -> "+state, func() error {
			if err := os.MkdirAll(filepath.Dir(state), 0o700); err != nil {
				return err
			}
			return os.Rename(oldState, state)
		})
	}
	if oldErr == nil && stateErr == nil {
		installer.skip("both old and pfm state directories exist; left both in place")
	}
	return nil
}

func (installer *engine) migrateLegacyCarrier(ctx context.Context) error {
	carrier := filepath.Join(installer.options.Home, ".claude", ".cc-ls-hidden")
	file, err := os.Open(carrier)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read retired kill carrier: %w", err)
	}
	defer file.Close()
	seen := map[string]bool{}
	var ids []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		id := strings.TrimSpace(scanner.Text())
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan retired kill carrier: %w", err)
	}
	sort.Strings(ids)
	return installer.change(fmt.Sprintf("merge %d retired carrier kill(s) into SQLite without overwriting existing rows", len(ids)), func() error {
		values := paths.Values{
			Home:     installer.options.Home,
			SharedDB: filepath.Join(installer.options.Home, ".cc", "fleet.db"),
		}
		state := shared.Open(ctx, values)
		defer state.Close()
		if err := state.Degraded(); err != nil {
			return fmt.Errorf("open shared store before carrier retirement: %w", err)
		}
		existing, err := state.KilledAt(ctx)
		if err != nil {
			return fmt.Errorf("read shared kills before carrier retirement: %w", err)
		}
		for _, id := range ids {
			if _, found := existing[id]; found {
				continue
			}
			if err := state.Kill(ctx, id, 0); err != nil {
				return fmt.Errorf("import retired carrier kill %q: %w", id, err)
			}
		}
		return nil
	})
}

func (installer *engine) retirePredecessors() error {
	config := installer.options.ConfigDir
	for _, name := range []string{
		"cc-kill.sh", "cx-kill.sh", "bb-hook.sh", "cc-archive.sh", "cc-reap.sh",
		"cc-name-sync.sh", "cx-heal.sh", "cc-portable.sh", "cc-db.sh", "cx-recover.sh",
		"cc-agent-open.sh", "cc-swap-chat.sh",
	} {
		if err := installer.retire(filepath.Join(config, "bin", name), "native pfm command"); err != nil {
			return err
		}
	}
	if err := installer.retireGlob(filepath.Join(config, "bin", "cx-recover.sh.pre-professor-*"), "retired recovery artifact"); err != nil {
		return err
	}
	for _, name := range []string{
		"statusline-command.sh",
		"statusline/segments.d/10-vertex-spend.sh",
		"statusline/segments.d/40-gpt-account.sh",
		"statusline/vertex-spend-refresh.py",
		"statusline/gpt-usage.py",
		"statusline/vertex_daily_tokens.py",
	} {
		if err := installer.retire(filepath.Join(config, filepath.FromSlash(name)), "native pfm statusline"); err != nil {
			return err
		}
	}
	for _, name := range []string{"dump.md", "chat-ops.sh", "group.sh"} {
		if err := installer.retire(filepath.Join(config, "commands", "chat", name), "native pfm chat command"); err != nil {
			return err
		}
	}
	carrier := filepath.Join(installer.options.Home, ".claude", ".cc-ls-hidden")
	for _, path := range []string{carrier, carrier + ".at", carrier + ".lock"} {
		if err := installer.retire(path, "SQLite is the sole kill store"); err != nil {
			return err
		}
	}
	return installer.retireGlob(carrier+".n.*", "retired carrier scratch")
}
