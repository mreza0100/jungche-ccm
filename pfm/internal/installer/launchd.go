package installer

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// launchdLabel is the job's name, and the handle launchctl addresses it by.
const launchdLabel = "com.professor.pfm.name-sync"

const mcpLaunchdLabel = "com.professor.pfm.mcp"

const launchdAsset = "launchd/" + launchdLabel + ".plist"

const mcpLaunchdAsset = "launchd/" + mcpLaunchdLabel + ".plist"

// launchAgentPath returns where macOS expects a per-user agent to live.
func (installer *engine) launchAgentPath() string {
	return filepath.Join(
		installer.options.Home, "Library", "LaunchAgents", launchdLabel+".plist",
	)
}

func (installer *engine) mcpLaunchAgentPath() string {
	return filepath.Join(
		installer.options.Home, "Library", "LaunchAgents", mcpLaunchdLabel+".plist",
	)
}

// wireLaunchAgent installs the macOS half of the name-sync scheduler.
//
// The plist is written as a REAL FILE, not a symlink into the managed asset
// root the way every other installed asset is. launchd refuses to load an agent
// through a symlink, and the failure is silent — the job simply never runs — so
// the one place the managed-asset pattern is broken is the one place breaking
// it is required.
func (installer *engine) wireLaunchAgent(ctx context.Context) error {
	path := installer.launchAgentPath()
	installer.say("launchd user agent -> %s", path)

	template, err := readAsset(launchdAsset)
	if err != nil {
		return fmt.Errorf("read embedded launch agent: %w", err)
	}
	// launchd has no %h, so the home is substituted here rather than expanded
	// at load time. The poll comes from the machine config's nameSync.interval
	// through the same renderer the systemd timer uses, so the two schedulers
	// cannot drift.
	wanted, err := renderNameSyncLaunchAgent([]byte(strings.ReplaceAll(
		string(template), "__PFM_HOME__", installer.options.Home,
	)), installer.options)
	if err != nil {
		return fmt.Errorf("render launch agent: %w", err)
	}
	installer.say("  %s", nameSyncScheduleSummary(installer.options))

	if sameFile(path, wanted, 0o644) {
		installer.ok(path)
	} else {
		if err := installer.change("write "+path, func() error {
			if _, statErr := os.Lstat(path); statErr == nil {
				backup := availableBackup(path, installer.stamp)
				if err := copyBackup(path, backup); err != nil {
					return err
				}
			}
			return atomicWrite(path, wanted, 0o644)
		}); err != nil {
			return err
		}
	}
	if !installer.apply {
		installer.say("")
		return nil
	}
	return installer.reloadLaunchAgent(ctx, path)
}

func (installer *engine) wireMCPLaunchAgent(ctx context.Context) error {
	path := installer.mcpLaunchAgentPath()
	if !installer.mcpAnyEnabled() {
		if _, err := os.Lstat(path); errors.Is(err, fs.ErrNotExist) {
			return nil
		} else if err != nil {
			return err
		}
		if installer.apply {
			domain := "gui/" + strconv.Itoa(os.Getuid())
			_ = installer.options.Runner.Run(ctx, "launchctl", "bootout", domain+"/"+mcpLaunchdLabel)
		}
		return installer.change("remove "+path, func() error { return os.Remove(path) })
	}
	template, err := readAsset(mcpLaunchdAsset)
	if err != nil {
		return fmt.Errorf("read embedded MCP launch agent: %w", err)
	}
	wanted := []byte(strings.ReplaceAll(string(template), "__PFM_HOME__", installer.options.Home))
	if !sameFile(path, wanted, 0o644) {
		if err := installer.change("write "+path, func() error {
			if _, statErr := os.Lstat(path); statErr == nil {
				if err := copyBackup(path, availableBackup(path, installer.stamp)); err != nil {
					return err
				}
			}
			return atomicWrite(path, wanted, 0o644)
		}); err != nil {
			return err
		}
	} else {
		installer.ok(path)
	}
	if !installer.apply {
		return nil
	}
	return installer.reloadLaunchAgentWithLabel(ctx, path, mcpLaunchdLabel)
}

// reloadLaunchAgent re-registers the job so an edited plist takes effect.
//
// bootout before bootstrap is deliberate and its failure is ignored: launchd
// rejects bootstrapping a label it already knows, and on a first install there
// is nothing to boot out. Only the bootstrap verdict is reported.
func (installer *engine) reloadLaunchAgent(ctx context.Context, path string) error {
	return installer.reloadLaunchAgentWithLabel(ctx, path, launchdLabel)
}

func (installer *engine) reloadLaunchAgentWithLabel(ctx context.Context, path, label string) error {
	domain := "gui/" + strconv.Itoa(os.Getuid())
	_ = installer.options.Runner.Run(ctx, "launchctl", "bootout", domain+"/"+label)
	if err := installer.options.Runner.Run(ctx, "launchctl", "bootstrap", domain, path); err != nil {
		installer.skip("launchctl bootstrap failed; the agent is installed but not loaded: " + err.Error())
		installer.say("")
		return nil
	}
	installer.ok("launchctl bootstrap " + label)
	installer.say("")
	return nil
}

// unwireLaunchAgent removes the agent and unloads it.
func (installer *engine) unwireLaunchAgent(ctx context.Context) error {
	path := installer.launchAgentPath()
	if _, err := os.Lstat(path); errors.Is(err, fs.ErrNotExist) {
		installer.skip("no launch agent at " + path)
		return installer.unwireMCPLaunchAgent(ctx)
	}
	if installer.apply {
		domain := "gui/" + strconv.Itoa(os.Getuid())
		_ = installer.options.Runner.Run(ctx, "launchctl", "bootout", domain+"/"+launchdLabel)
	}
	if err := installer.change("remove "+path, func() error { return os.Remove(path) }); err != nil {
		return err
	}
	return installer.unwireMCPLaunchAgent(ctx)
}

func (installer *engine) unwireMCPLaunchAgent(ctx context.Context) error {
	path := installer.mcpLaunchAgentPath()
	if _, err := os.Lstat(path); errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	if installer.apply {
		domain := "gui/" + strconv.Itoa(os.Getuid())
		_ = installer.options.Runner.Run(ctx, "launchctl", "bootout", domain+"/"+mcpLaunchdLabel)
	}
	return installer.change("remove "+path, func() error { return os.Remove(path) })
}

// launchAgentRunning reports whether the name-sync job is executing right now,
// and whether the question could be asked at all.
//
// "state = not running" contains "running", so the state line is compared whole
// rather than searched — a substring match here would refuse every install on a
// perfectly idle agent.
func launchAgentRunning(ctx context.Context, runner CommandRunner) (running bool, probed bool) {
	reader, ok := runner.(OutputRunner)
	if !ok {
		return false, false
	}
	output, err := reader.Output(
		ctx, "launchctl", "print", "gui/"+strconv.Itoa(os.Getuid())+"/"+launchdLabel,
	)
	if err != nil {
		// A label launchd does not know is not an error to report: nothing is
		// installed yet, so nothing can be mid-execution.
		return false, true
	}
	for _, line := range strings.Split(string(output), "\n") {
		if strings.TrimSpace(line) == "state = running" {
			return true, true
		}
	}
	return false, true
}
