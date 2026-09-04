package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"hostops/pfm/internal/config"
	"hostops/pfm/internal/deps"
	"hostops/pfm/internal/gather"
	"hostops/pfm/internal/paths"
)

// The two states a live tmux server can be in for the tmux.titles concept.
// They are stated as values, not inferred at each print site, so the check and
// the config key describe the same two things in the same words.
const (
	titlesPfmOwned  = "pfm-owned"
	titlesHostOwned = "host-owned"
)

// printTmuxTitlesDoctor REPORTS which side owns the outer terminal's title on
// every live socket. It never fails and never modifies.
//
// pfm takes over a host-level surface here: `set-titles on` plus its own
// set-titles-string. A host that emits its own OSC title on the outer pty
// before tmux starts keeps it only while `set-titles off` stands, so a
// framework that flips it must at minimum record that it did — otherwise the
// next reader cannot tell "pfm owns this" from "the host owns this and pfm has
// not stomped it yet", and reads a deliberate setting as drift. That
// misreading is exactly how eight live servers had their tab badges destroyed
// by a well-meaning fix.
//
// It is INFO, never a warning: both states are legitimate. The config key says
// which one pfm intends; these lines say which one each server is actually in.
func printTmuxTitlesDoctor(
	ctx context.Context,
	stdout io.Writer,
	resolved paths.Values,
	machine config.Config,
) {
	intended := titlesHostOwned
	if machine.Tmux.Titles.Enabled {
		intended = titlesPfmOwned
	}
	fmt.Fprintf(
		stdout,
		"doctor: tmux titles policy=%s (config tmux.titles.enabled=%t %s)\n",
		intended, machine.Tmux.Titles.Enabled, machine.Source("tmux.titles.enabled"),
	)

	client := gather.CommandTmux{TmuxTmpDir: filepath.Dir(resolved.TmuxDir)}
	probe, err := gather.ProbeTmuxReadOnly(ctx, resolved.TmuxDir, client, time.Now())
	if err != nil {
		fmt.Fprintf(stdout, "doctor: tmux titles sockets=unprobed error=%v\n", err)
		return
	}
	seen := make(map[string]bool, len(probe.Panes))
	sockets := make([]string, 0, len(probe.Panes))
	for _, pane := range probe.Panes {
		if seen[pane.Socket] {
			continue
		}
		seen[pane.Socket] = true
		sockets = append(sockets, pane.Socket)
	}
	sort.Strings(sockets)
	if len(sockets) == 0 {
		fmt.Fprintln(stdout, "doctor: tmux titles sockets=none live")
		return
	}
	for _, socket := range sockets {
		state, detail := readTmuxTitlesState(ctx, resolved, socket)
		fmt.Fprintf(stdout, "doctor: tmux titles %s=%s (%s)\n", socket, state, detail)
	}
}

// readTmuxTitlesState asks one live server for its set-titles value. It is
// read-only by construction: show-options is the only tmux verb it runs, and
// a socket that will not answer is reported unknown rather than assumed.
func readTmuxTitlesState(
	ctx context.Context,
	resolved paths.Values,
	socket string,
) (state, detail string) {
	commandContext, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	command := exec.CommandContext(
		commandContext,
		deps.Executable("tmux"),
		"-S", filepath.Join(resolved.TmuxDir, socket),
		"show-options", "-g", "set-titles",
	)
	command.Env = append(os.Environ(), "TMUX=")
	output, err := command.Output()
	if err != nil {
		return "unknown", fmt.Sprintf("show-options failed: %v", err)
	}
	value := strings.TrimSpace(string(output))
	// tmux prints the option name with its value, and omits the whole line when
	// the option sits at its default — an absent line is tmux's default, off.
	if value == "" {
		value = "set-titles off"
	}
	if strings.HasSuffix(value, " on") {
		return titlesPfmOwned, value
	}
	return titlesHostOwned, value
}
