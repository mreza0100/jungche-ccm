package installer

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

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
}

func Run(ctx context.Context, options Options) (Report, error) {
	options, err := normalize(options)
	if err != nil {
		return Report{}, fmt.Errorf("resolve installer options: %w", err)
	}
	if options.Mode > ModeUninstall {
		return Report{}, fmt.Errorf("unknown installer mode %d", options.Mode)
	}
	if err := options.Runner.Run(ctx, "systemctl", "--user", "status"); err == nil {
		return Report{}, ErrReachableUserBus
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

	if options.Mode == ModeUninstall {
		err = installer.uninstall(ctx)
	} else {
		err = installer.install(ctx)
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

func (installer *engine) install(ctx context.Context) error {
	assets, err := assetFiles()
	if err != nil {
		return fmt.Errorf("enumerate embedded install assets: %w", err)
	}
	systemdAssetChanged, err := installer.stageAssets(assets)
	if err != nil {
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
	if err := installer.wireCommands(assets); err != nil {
		return err
	}
	unitChanged, err := installer.wireUnits(ctx)
	if err != nil {
		return err
	}
	if systemdAssetChanged || unitChanged {
		installer.reloadUnits(ctx)
	}
	if err := installer.wireSettings(); err != nil {
		return err
	}
	if err := installer.wireCodexHooks(); err != nil {
		return err
	}
	return installer.wireShell(false)
}

func (installer *engine) uninstall(ctx context.Context) error {
	assets, err := assetFiles()
	if err != nil {
		return fmt.Errorf("enumerate embedded install assets: %w", err)
	}
	if err := installer.unwireCommands(assets); err != nil {
		return err
	}
	if err := installer.retireBBInstall(); err != nil {
		return err
	}
	managerAvailable := installer.userManagerAvailable(ctx)
	if managerAvailable && installer.apply {
		installer.runSystemctl(ctx, "disable", "--now", "pfm-name-sync.path", "pfm-name-sync.timer")
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
	for _, name := range unitNames {
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
	if err := installer.wireShell(true); err != nil {
		return err
	}
	return installer.removeManagedAssets(assets)
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
		content, err := readAsset(asset.path)
		if err != nil {
			return false, fmt.Errorf("read embedded asset %s: %w", asset.path, err)
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
	installer.say("")
	return nil
}

func (installer *engine) commandTarget(asset string) (string, bool) {
	commands := filepath.Join(installer.options.ConfigDir, "commands")
	switch asset {
	case "swap.command.md":
		return filepath.Join(commands, "swap.md"), true
	case "chat/chat.sh", "chat/history.sh":
		return filepath.Join(commands, filepath.FromSlash(asset)), true
	}
	if strings.HasPrefix(asset, "chat/") && strings.HasSuffix(asset, ".command.md") {
		relative := strings.TrimSuffix(asset, ".command.md") + ".md"
		return filepath.Join(commands, filepath.FromSlash(relative)), true
	}
	return "", false
}

func (installer *engine) retireBBInstall() error {
	installer.say("retired /bb surfaces")
	for _, link := range []struct{ target, source string }{
		{
			target: filepath.Join(installer.options.ConfigDir, "commands", "bb.md"),
			source: filepath.Join(installer.managedRoot, "bb.command.md"),
		},
		{
			target: filepath.Join(installer.options.Home, ".agents", "skills", "bb"),
			source: filepath.Join(installer.managedRoot, "codex-skills", "bb"),
		},
	} {
		current, linked := resolvedLink(link.target)
		if !linked || current != filepath.Clean(link.source) {
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

var unitNames = []string{
	"pfm-name-sync.path",
	"pfm-name-sync.service",
	"pfm-name-sync.timer",
}

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
	for _, name := range unitNames {
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
		linkChanged, err := installer.ensureLink(source, target)
		if err != nil {
			return false, err
		}
		changed = changed || linkChanged
	}
	if !managerAvailable {
		installer.skip("systemd --user unavailable; units are staged but not enabled")
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
	candidates := []string{filepath.Join(installer.options.ConfigDir, "settings.json")}
	if installer.options.ConfigDir == filepath.Join(installer.options.Home, ".claude") {
		for account := 1; account <= 4; account++ {
			candidate := filepath.Join(installer.options.Home, ".cc", fmt.Sprint(account), "settings.json")
			if _, err := os.Stat(candidate); err == nil {
				candidates = append(candidates, candidate)
			}
		}
	}
	seen := map[string]bool{}
	for index, candidate := range candidates {
		physical, err := filepath.EvalSymlinks(candidate)
		if err != nil {
			physical = candidate
		}
		physical = filepath.Clean(physical)
		if seen[physical] {
			continue
		}
		seen[physical] = true
		raw, err := os.ReadFile(candidate)
		if errors.Is(err, fs.ErrNotExist) {
			installer.skip("no settings file at " + candidate)
			continue
		}
		if err != nil {
			return fmt.Errorf("read %s: %w", candidate, err)
		}
		updated, changed, err := updateSettings(
			raw,
			installer.options.Home,
			index == 0,
			installer.options.Mode == ModeUninstall,
		)
		if err != nil {
			installer.skip("invalid settings JSON at " + candidate + ": " + err.Error())
			continue
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
	return nil
}

func (installer *engine) wireCodexHooks() error {
	path := filepath.Join(installer.options.Home, ".codex", "hooks.json")
	raw, err := os.ReadFile(path)
	existed := true
	if errors.Is(err, fs.ErrNotExist) {
		existed = false
		raw = []byte("{\"hooks\":{}}\n")
	} else if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	updated, changed, err := updateCodexHooks(
		raw,
		installer.options.Home,
		installer.options.Mode == ModeUninstall,
	)
	if err != nil {
		installer.skip("invalid Codex hooks JSON at " + path + ": " + err.Error())
		return nil
	}
	if !changed {
		installer.ok(path + " wiring")
		return nil
	}
	return installer.change("rewrite "+path+" (backup preserved)", func() error {
		if existed {
			backup := availableBackup(path, installer.stamp)
			if err := copyBackup(path, backup); err != nil {
				return fmt.Errorf("backup %s: %w", path, err)
			}
		}
		return atomicWrite(path, updated, 0o600)
	})
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
	updated := rewriteZshrc(string(raw), wanted, uninstall)
	if string(raw) == updated {
		installer.ok(zshrc + " source line")
		return nil
	}
	return installer.change("rewrite "+zshrc+" (backup preserved)", func() error {
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
		return fmt.Errorf("read retired hide carrier: %w", err)
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
		return fmt.Errorf("scan retired hide carrier: %w", err)
	}
	sort.Strings(ids)
	return installer.change(fmt.Sprintf("merge %d retired carrier hide(s) into SQLite without overwriting existing rows", len(ids)), func() error {
		values := paths.Values{
			Home:     installer.options.Home,
			SharedDB: filepath.Join(installer.options.Home, ".cc", "fleet.db"),
		}
		state := shared.Open(ctx, values)
		defer state.Close()
		if err := state.Degraded(); err != nil {
			return fmt.Errorf("open shared store before carrier retirement: %w", err)
		}
		existing, err := state.HiddenAt(ctx)
		if err != nil {
			return fmt.Errorf("read shared hides before carrier retirement: %w", err)
		}
		for _, id := range ids {
			if _, found := existing[id]; found {
				continue
			}
			if err := state.Hide(ctx, id, 0); err != nil {
				return fmt.Errorf("import retired carrier hide %q: %w", id, err)
			}
		}
		return nil
	})
}

func (installer *engine) retirePredecessors() error {
	config := installer.options.ConfigDir
	for _, name := range []string{
		"cc-hide.sh", "cx-hide.sh", "bb-hook.sh", "cc-archive.sh", "cc-reap.sh",
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
		if err := installer.retire(path, "SQLite is the sole hide store"); err != nil {
			return err
		}
	}
	return installer.retireGlob(carrier+".n.*", "retired carrier scratch")
}
