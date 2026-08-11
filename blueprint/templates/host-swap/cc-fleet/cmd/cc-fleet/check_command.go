package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	fleetcheck "hostops/cc-fleet/internal/check"
	"hostops/cc-fleet/internal/compose"
	"hostops/cc-fleet/internal/gather"
	"hostops/cc-fleet/internal/paths"
	"hostops/cc-fleet/internal/store"
)

const (
	checkAllowlistEnv  = "CC_FLEET_CHECK_ALLOWLIST"
	checkAllowlistFile = "check-allowlist.txt"
)

func runLSCheck(ctx context.Context, stdout, stderr io.Writer) int {
	resolved, err := paths.Resolve()
	if err != nil {
		fmt.Fprintf(stderr, "cc-fleet ls --check: %v\n", err)
		return 1
	}
	legacyOutput, err := fleetcheck.LegacyOutput(ctx, resolved)
	if err != nil {
		fmt.Fprintf(stderr, "cc-fleet ls --check: %v\n", err)
		return 1
	}
	database, err := store.Open(store.WithWarningWriter(stderr))
	if err != nil {
		fmt.Fprintf(stderr, "cc-fleet ls --check: %v\n", err)
		return 1
	}
	defer database.Close()
	scan, err := scanFleet(ctx, database, scanRequest{
		View:     compose.AllView,
		ReadOnly: true,
	}, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "cc-fleet ls --check: %v\n", err)
		return 1
	}
	own := fleetcheck.NormalizeLiveCodex(
		fleetcheck.TuplesFromRows(scan.Output.Rows),
	)
	legacyRows, err := fleetcheck.ParseLegacy(legacyOutput)
	if err != nil {
		fmt.Fprintf(stderr, "cc-fleet ls --check: %v\n", err)
		return 1
	}
	legacy := fleetcheck.NormalizeLiveCodex(
		fleetcheck.ReconcileIDs(legacyRows, own),
	)

	allowlistPath, err := resolveCheckAllowlist(executableDir(), workingDir())
	if err != nil {
		fmt.Fprintf(stderr, "cc-fleet ls --check: %v\n", err)
		return 1
	}
	allowlist, err := loadCheckAllowlist(allowlistPath)
	if err != nil {
		fmt.Fprintf(stderr, "cc-fleet ls --check: %v\n", err)
		return 1
	}
	codexCandidates, err := fleetcheck.CodexCandidateIDs(resolved.CodexRoot, 120)
	if err != nil {
		fmt.Fprintf(stderr, "cc-fleet ls --check: verify Codex candidate bound: %v\n", err)
		return 1
	}
	codexLegacySource, err := fleetcheck.CodexLegacyUnrecognizedIDs(
		resolved.CodexRoot,
		120,
	)
	if err != nil {
		fmt.Fprintf(stderr, "cc-fleet ls --check: verify Codex source schema: %v\n", err)
		return 1
	}
	codexRolloutFiles, err := fleetcheck.CodexRolloutFileIDs(resolved.CodexRoot)
	if err != nil {
		fmt.Fprintf(stderr, "cc-fleet ls --check: verify Codex rollout files: %v\n", err)
		return 1
	}
	legacyEmptyCWD, err := fleetcheck.LegacyCacheEmptyCWDIDs(
		ctx,
		resolved.SharedDB,
	)
	if err != nil {
		fmt.Fprintf(stderr, "cc-fleet ls --check: verify legacy name cache: %v\n", err)
		return 1
	}
	claudeTranscripts, err := fleetcheck.ClaudeTranscriptIDs(resolved.ClaudeRoots)
	if err != nil {
		fmt.Fprintf(stderr, "cc-fleet ls --check: verify Claude store: %v\n", err)
		return 1
	}
	codexMachineSpawned, err := fleetcheck.CodexMachineSpawnedIDs(ctx, resolved.CodexRoot)
	if err != nil {
		fmt.Fprintf(
			stderr,
			"cc-fleet ls --check: verify Codex machine-spawned threads: %v\n",
			err,
		)
		return 1
	}
	rollouts, err := database.Rollouts(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "cc-fleet ls --check: verify Codex lineages: %v\n", err)
		return 1
	}
	_, codexLineageRoots := store.ResolveCodexLineages(rollouts)
	differences := fleetcheck.DiffVerified(
		legacy,
		own,
		allowlist,
		fleetcheck.Verification{
			CodexCandidateIDs:   codexCandidates,
			CodexRolloutFileIDs: codexRolloutFiles,
			CodexLegacySource:   codexLegacySource,
			CodexLineageRoots:   codexLineageRoots,
			LegacyEmptyCWDIDs:   legacyEmptyCWD,
			SocketSquatters:     socketSquatters(scan.Live.Panes),
			ClaudeTranscriptIDs: claudeTranscripts,
			DormantAccountIDs: dormantAccountIDs(
				scan.Output.Rows,
				accountRoots(resolved.ClaudeRoots),
			),
			CodexMachineSpawnedIDs: codexMachineSpawned,
		},
	)
	unallowed := 0
	allowed := 0
	for _, difference := range differences {
		prefix := "+"
		if difference.Direction == fleetcheck.LegacyOnly {
			prefix = "-"
		}
		if difference.Allowed {
			allowed++
			class := difference.Class
			if class == "" {
				class = "tuple"
			}
			fmt.Fprintf(
				stdout,
				"~ allowed[%s] %s\t%s\t%s\t%s\t%dp\t%s\n",
				class,
				difference.Direction,
				difference.Tuple.ID,
				difference.Tuple.Kind,
				difference.Tuple.Project,
				difference.Tuple.Prompts,
				difference.Reason,
			)
			continue
		}
		unallowed++
		fmt.Fprintf(
			stdout,
			"%s %s\t%s\t%s\t%s\t%dp\n",
			prefix,
			difference.Direction,
			difference.Tuple.ID,
			difference.Tuple.Kind,
			difference.Tuple.Project,
			difference.Tuple.Prompts,
		)
	}
	if unallowed != 0 {
		fmt.Fprintf(
			stdout,
			"cc-fleet check: %d difference(s), %d allowlisted\n",
			unallowed,
			allowed,
		)
		return 1
	}
	fmt.Fprintf(
		stdout,
		"cc-fleet check: parity (%d tuples, %d allowlisted deviation(s))\n",
		len(own),
		allowed,
	)
	return 0
}

// socketSquatters counts the sessions on each tmux socket whose name is not the
// socket's own. Every fleet chat is launched with `new-session -s "$sock"`, so
// a differently named session is other work parked on that server — the bound
// for the legacy-socket-squatter class, and never a chat.
func socketSquatters(panes []gather.Pane) map[string]int {
	sessions := make(map[string]map[string]struct{})
	for _, pane := range panes {
		if pane.SessionName == "" || pane.SessionName == pane.Socket {
			continue
		}
		names := sessions[pane.Socket]
		if names == nil {
			names = make(map[string]struct{})
			sessions[pane.Socket] = names
		}
		names[pane.SessionName] = struct{}{}
	}
	result := make(map[string]int, len(sessions))
	for socket, names := range sessions {
		result[socket] = len(names)
	}
	return result
}

// dormantAccountIDs names the Claude rows the legacy picker cannot reach
// because they live under an account root it never walks. The shadow harness
// points the reference script at account 1's root alone, so "account > 1" is
// only a real absence when that account's root CANONICALISES somewhere else:
// on this fleet ~/.cc/2/projects and ~/.cc/3/projects are symlinks back to
// account 1's store, and treating a row there as unreachable would hide a chat
// the legacy picker does list.
func dormantAccountIDs(
	rows []compose.Row,
	roots []compose.AccountRoot,
) map[string]struct{} {
	walked := ""
	for _, root := range roots {
		if root.Account == 1 {
			walked = root.Path
			break
		}
	}
	separate := make(map[int]struct{}, len(roots))
	for _, root := range roots {
		if root.Account > 1 && root.Path != walked {
			separate[root.Account] = struct{}{}
		}
	}
	result := make(map[string]struct{})
	for _, row := range rows {
		if row.Kind != compose.ResumeClaude {
			continue
		}
		if _, unreachable := separate[row.Account]; unreachable {
			result[row.ID] = struct{}{}
		}
	}
	return result
}

// resolveCheckAllowlist locates the shadow-check allowlist without knowing
// where the tree lives: the environment override, then testdata/ beside the
// running binary, then testdata/ under the working directory. The tree moves;
// neither the binary nor the working directory lies about where it went.
func resolveCheckAllowlist(executable, working string) (string, error) {
	if path := os.Getenv(checkAllowlistEnv); path != "" {
		return path, nil
	}
	candidates := make([]string, 0, 2)
	for _, directory := range []string{executable, working} {
		if directory == "" {
			continue
		}
		candidates = append(
			candidates,
			filepath.Join(directory, "testdata", checkAllowlistFile),
		)
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf(
		"locate check allowlist: set %s, or keep testdata/%s beside the binary "+
			"or under the working directory (tried %s)",
		checkAllowlistEnv,
		checkAllowlistFile,
		strings.Join(candidates, ", "),
	)
}

func executableDir() string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Dir(executable)
}

func workingDir() string {
	directory, err := os.Getwd()
	if err != nil {
		return ""
	}
	return directory
}

func loadCheckAllowlist(path string) ([]fleetcheck.AllowRule, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open check allowlist %q: %w", path, err)
	}
	defer file.Close()
	rules, err := fleetcheck.ParseAllowlist(file)
	if err != nil {
		return nil, fmt.Errorf("parse check allowlist %q: %w", path, err)
	}
	return rules, nil
}
