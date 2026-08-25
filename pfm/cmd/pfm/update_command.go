package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"hostops/pfm/internal/deps"
	"hostops/pfm/internal/installer"
)

// These seams keep update tests entirely inside their throwaway repositories;
// production uses the real build/install/doctor functions below.
var (
	updateBuildCandidate  = buildUpdateCandidate
	updateApplyInstall    = applyUpdateInstall
	updateRunDoctor       = runUpdateDoctor
	updateRollbackInstall = applyUpdateInstall
	updateRollbackDoctor  = runUpdateDoctor
)

type updateVersion struct {
	major int
	minor int
	patch int
}

func (version updateVersion) less(other updateVersion) bool {
	if version.major != other.major {
		return version.major < other.major
	}
	if version.minor != other.minor {
		return version.minor < other.minor
	}
	return version.patch < other.patch
}

func parseUpdateVersion(tag string) (updateVersion, bool) {
	parts := strings.Split(strings.TrimSpace(tag), ".")
	if len(parts) != 3 || !strings.HasPrefix(parts[0], "v") {
		return updateVersion{}, false
	}
	major, err := strconv.Atoi(strings.TrimPrefix(parts[0], "v"))
	if err != nil || major < 0 {
		return updateVersion{}, false
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil || minor < 0 {
		return updateVersion{}, false
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil || patch < 0 {
		return updateVersion{}, false
	}
	return updateVersion{major: major, minor: minor, patch: patch}, true
}

func selectHighestSemver(tags []string) (string, error) {
	ordered := append([]string(nil), tags...)
	sort.Strings(ordered)
	var selected string
	var selectedVersion updateVersion
	for _, tag := range ordered {
		version, ok := parseUpdateVersion(tag)
		if !ok {
			continue
		}
		if selected == "" || selectedVersion.less(version) {
			selected, selectedVersion = tag, version
		}
	}
	if selected == "" {
		return "", errors.New("no semantic-version tags (expected vMAJOR.MINOR.PATCH)")
	}
	return selected, nil
}

func runUpdate(args []string, stdout, stderr io.Writer, runtimes ...commandRuntime) int {
	flags := newFlagSet(
		"update",
		"usage: pfm update [--to vX.Y.Z] [--repo PATH] [--skip-harvest]",
		stderr,
	)
	target := flags.String("to", "", "target semantic-version tag")
	repoFlag := flags.String("repo", "", "source clone to update")
	skipHarvest := flags.Bool("skip-harvest", false, "leave the optional harvestpy runtime unmanaged")
	positional, code, ok := parseFlagsAnywhere(flags, args)
	if !ok {
		return code
	}
	if len(positional) != 0 {
		flags.Usage()
		return 2
	}
	runtime, err := optionalCommandRuntime(runtimes)
	if err != nil {
		fmt.Fprintf(stderr, "pfm update: config: %v\n", err)
		return 1
	}
	repo := strings.TrimSpace(*repoFlag)
	if repo == "" {
		repo, err = installer.ReadSourceRepoMarker(runtime.Paths.Home)
		if err != nil {
			fmt.Fprintf(stderr, "pfm update: %v\n", err)
			return 1
		}
	}
	repo, err = filepath.Abs(repo)
	if err != nil {
		fmt.Fprintf(stderr, "pfm update: resolve repository: %v\n", err)
		return 1
	}
	if err := updateRepository(context.Background(), repo, *target, *skipHarvest, stdout, stderr, runtime); err != nil {
		fmt.Fprintf(stderr, "pfm update: %v\n", err)
		return 1
	}
	return 0
}

func updateRepository(
	ctx context.Context,
	repo, requestedTag string,
	skipHarvest bool,
	stdout, stderr io.Writer,
	runtime commandRuntime,
) (err error) {
	previousRef, err := updateGitOutput(ctx, repo, "rev-parse", "--verify", "HEAD")
	if err != nil {
		return fmt.Errorf("resolve current revision: %w", err)
	}
	previousRef = strings.TrimSpace(previousRef)
	if _, err := updateGitOutput(ctx, repo, "symbolic-ref", "--quiet", "--short", "HEAD"); err != nil {
		return errors.New("source checkout is detached; checkout its update branch before running pfm update")
	}

	if err := updateGitRun(ctx, repo, "fetch", "--tags"); err != nil {
		return fmt.Errorf("fetch tags: %w", err)
	}
	tagOutput, err := updateGitOutput(ctx, repo, "tag", "--list")
	if err != nil {
		return fmt.Errorf("list tags: %w", err)
	}
	tags := strings.Fields(tagOutput)
	target := strings.TrimSpace(requestedTag)
	if target == "" {
		target, err = selectHighestSemver(tags)
		if err != nil {
			return fmt.Errorf("resolve latest release: %w", err)
		}
	} else if _, ok := parseUpdateVersion(target); !ok {
		return fmt.Errorf("invalid target tag %q (expected vMAJOR.MINOR.PATCH)", target)
	}
	if !containsString(tags, target) {
		return fmt.Errorf("target tag %q is not present after fetch", target)
	}
	status, err := updateGitOutput(ctx, repo, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return fmt.Errorf("inspect worktree: %w", err)
	}
	if strings.TrimSpace(status) != "" {
		return errors.New("refuse dirty worktree; commit or stash changes before update")
	}
	sourceAlreadyContainsTarget := updateGitRun(ctx, repo, "merge-base", "--is-ancestor", target, previousRef) == nil
	if !sourceAlreadyContainsTarget {
		if err := updateGitRun(ctx, repo, "merge-base", "--is-ancestor", previousRef, target); err != nil {
			return fmt.Errorf("target %s does not fast-forward the current source branch", target)
		}
	} else {
		currentTag, describeErr := updateGitOutput(ctx, repo, "describe", "--tags", "--abbrev=0", previousRef)
		currentVersion, currentOK := parseUpdateVersion(strings.TrimSpace(currentTag))
		targetVersion, targetOK := parseUpdateVersion(target)
		if describeErr == nil && currentOK && targetOK && targetVersion.less(currentVersion) {
			return fmt.Errorf("target %s would downgrade source from %s", target, strings.TrimSpace(currentTag))
		}
	}

	managedRoot := filepath.Dir(installer.SourceRepoPath(runtime.Paths.Home))
	stage, err := os.MkdirTemp(filepath.Dir(managedRoot), "update-")
	if err != nil {
		return fmt.Errorf("stage update beside managed root: %w", err)
	}
	worktreeAdded := false
	defer func() {
		var cleanupErr error
		if worktreeAdded {
			if removeErr := updateGitRun(ctx, repo, "worktree", "remove", "--force", filepath.Join(stage, "source")); removeErr != nil {
				cleanupErr = errors.Join(cleanupErr, fmt.Errorf("cleanup staged source worktree: %w", removeErr))
			}
		}
		if removeErr := os.RemoveAll(stage); removeErr != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("cleanup update stage %s: %w", stage, removeErr))
		}
		if cleanupErr == nil {
			return
		}
		if err != nil {
			err = errors.Join(err, cleanupErr)
			return
		}
		fmt.Fprintf(stderr, "pfm update: cleanup warning after successful update: %v\n", cleanupErr)
	}()
	stagedSource := filepath.Join(stage, "source")
	if err := updateGitRun(ctx, repo, "worktree", "add", "--detach", "--quiet", stagedSource, target); err != nil {
		return fmt.Errorf("stage source at %s: %w", target, err)
	}
	worktreeAdded = true
	candidateA := filepath.Join(stage, "pfm-a")
	candidateB := filepath.Join(stage, "pfm-b")
	if err := updateBuildCandidate(ctx, stagedSource, target, candidateA); err != nil {
		return fmt.Errorf("build candidate first pass: %w", err)
	}
	if err := updateBuildCandidate(ctx, stagedSource, target, candidateB); err != nil {
		return fmt.Errorf("build candidate second pass: %w", err)
	}
	hashA, err := fileHash(candidateA)
	if err != nil {
		return fmt.Errorf("hash first candidate: %w", err)
	}
	hashB, err := fileHash(candidateB)
	if err != nil {
		return fmt.Errorf("hash second candidate: %w", err)
	}
	if hashA != hashB {
		return fmt.Errorf("candidate hash mismatch: first=%s second=%s", hashA, hashB)
	}
	if err := updateGitRun(ctx, repo, "worktree", "remove", "--force", stagedSource); err != nil {
		return fmt.Errorf("remove staged source worktree: %w", err)
	}
	worktreeAdded = false

	ledger, err := installer.ReadBinaryOwnership(runtime.Paths.Home)
	if err != nil {
		return err
	}
	if len(ledger.Paths) == 0 {
		return errors.New("binary ownership ledger is empty; refusing to overwrite PATH copies")
	}
	replacements := make([]updateReplacement, 0, len(ledger.Paths))
	for _, targetPath := range ledger.Paths {
		if strings.TrimSpace(targetPath) == "" || !filepath.IsAbs(targetPath) {
			return fmt.Errorf("binary ownership ledger contains invalid path %q", targetPath)
		}
		backup := filepath.Join(stage, fmt.Sprintf("previous-%d", len(replacements)))
		if err := copyUpdateFile(targetPath, backup); err != nil {
			return fmt.Errorf("preserve owned binary %s: %w", targetPath, err)
		}
		replacements = append(replacements, updateReplacement{target: targetPath, backup: backup})
	}
	sourceAdvanced := false
	if !sourceAlreadyContainsTarget {
		if err := updateGitRun(ctx, repo, "merge", "--ff-only", "--quiet", target); err != nil {
			return fmt.Errorf("fast-forward source branch to %s: %w", target, err)
		}
		sourceAdvanced = true
	}
	for index := range replacements {
		if err := replaceUpdateFile(candidateA, replacements[index].target); err != nil {
			rollbackErr := rollbackUpdateState(
				ctx, repo, previousRef, sourceAdvanced, replacements, runtime, skipHarvest, stdout, stderr,
			)
			return updateFailure(fmt.Errorf("replace owned binary %s: %w", replacements[index].target, err), rollbackErr)
		}
		replacements[index].replaced = true
	}

	if err := updateApplyInstall(ctx, candidateA, repo, runtime, skipHarvest, stdout, stderr); err != nil {
		return updateFailure(
			fmt.Errorf("install --yes after staging: %w", err),
			rollbackUpdateState(ctx, repo, previousRef, sourceAdvanced, replacements, runtime, skipHarvest, stdout, stderr),
		)
	}
	if err := updateRunDoctor(ctx, candidateA, runtime, skipHarvest, stdout, stderr); err != nil {
		return updateFailure(
			fmt.Errorf("doctor after update: %w", err),
			rollbackUpdateState(ctx, repo, previousRef, sourceAdvanced, replacements, runtime, skipHarvest, stdout, stderr),
		)
	}
	fmt.Fprintf(stdout, "updated %s from %s\n", target, repo)
	return nil
}

type updateReplacement struct {
	target   string
	backup   string
	replaced bool
}

func updateFailure(primary, rollbackErr error) error {
	if rollbackErr != nil {
		return fmt.Errorf("%w; rollback residue: %v; manually repair the reported update-owned state", primary, rollbackErr)
	}
	return fmt.Errorf("%w; rolled back update-owned changes", primary)
}

func rollbackUpdateReplacements(replacements []updateReplacement, stderr io.Writer) error {
	var rollbackErr error
	for index := len(replacements) - 1; index >= 0; index-- {
		replacement := replacements[index]
		if !replacement.replaced {
			continue
		}
		if err := replaceUpdateFile(replacement.backup, replacement.target); err != nil {
			rollbackErr = errors.Join(rollbackErr, fmt.Errorf("%s: %w", replacement.target, err))
			continue
		}
		fmt.Fprintf(stderr, "pfm update: rolled back %s\n", replacement.target)
	}
	return rollbackErr
}

// rollbackUpdateState first restores every owned binary, then uses the prior
// binary's embedded installer to converge all installer-owned host wiring back
// to the previous release. A clean doctor is part of rollback proof; without
// it, updateFailure reports residue instead of claiming a safe rollback.
func rollbackUpdateState(
	ctx context.Context,
	repo, previousRef string,
	sourceAdvanced bool,
	replacements []updateReplacement,
	runtime commandRuntime,
	skipHarvest bool,
	stdout, stderr io.Writer,
) error {
	rollbackErr := rollbackUpdateReplacements(replacements, stderr)
	if sourceAdvanced {
		if err := updateGitRun(ctx, repo, "reset", "--keep", previousRef); err != nil {
			return errors.Join(rollbackErr, fmt.Errorf("restore source revision %s: %w", previousRef, err))
		}
		fmt.Fprintf(stderr, "pfm update: rolled back source to %s\n", previousRef)
	}
	if len(replacements) == 0 {
		return errors.Join(rollbackErr, errors.New("no previous binary is available to restore installer state"))
	}
	previousBinary := replacements[0].backup
	if err := updateRollbackInstall(ctx, previousBinary, repo, runtime, skipHarvest, stdout, stderr); err != nil {
		return errors.Join(rollbackErr, fmt.Errorf("reapply previous installer state: %w", err))
	}
	if err := updateRollbackDoctor(ctx, previousBinary, runtime, skipHarvest, stdout, stderr); err != nil {
		return errors.Join(rollbackErr, fmt.Errorf("doctor after rollback: %w", err))
	}
	return rollbackErr
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func updateGitRun(ctx context.Context, repo string, args ...string) error {
	command := exec.CommandContext(ctx, deps.Executable("git"), args...)
	command.Dir = repo
	command.Env = os.Environ()
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Run(); err != nil {
		return fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(output.String()))
	}
	return nil
}

func updateGitOutput(ctx context.Context, repo string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, deps.Executable("git"), args...)
	command.Dir = repo
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func buildUpdateCandidate(ctx context.Context, repo, version, output string) error {
	moduleRoot := repo
	if _, err := os.Stat(filepath.Join(repo, "pfm", "go.mod")); err == nil {
		moduleRoot = filepath.Join(repo, "pfm")
	}
	command := exec.CommandContext(
		ctx,
		deps.Executable("go"),
		"-C", moduleRoot,
		"build", "-trimpath", "-ldflags", "-X main.version="+version,
		"-o", output,
		"./cmd/pfm",
	)
	command.Env = envWithEmptyGOFLAGS()
	command.Dir = repo
	outputBytes, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go build: %w: %s", err, strings.TrimSpace(string(outputBytes)))
	}
	return nil
}

func envWithEmptyGOFLAGS() []string {
	env := make([]string, 0, len(os.Environ())+1)
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "GOFLAGS=") {
			continue
		}
		env = append(env, entry)
	}
	return append(env, "GOFLAGS=")
}

func fileHash(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(raw)), nil
}

func copyUpdateFile(source, target string) error {
	raw, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	return writeUpdateFile(target, raw, info.Mode().Perm())
}

func replaceUpdateFile(source, target string) error {
	raw, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	return writeUpdateFile(target, raw, info.Mode().Perm())
}

func writeUpdateFile(path string, raw []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".pfm-update-")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode.Perm()); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(raw); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func applyUpdateInstall(ctx context.Context, candidate, repo string, runtime commandRuntime, skipHarvest bool, stdout, stderr io.Writer) error {
	args := []string{"--yes"}
	if skipHarvest {
		args = append(args, "--skip-harvest")
	}
	return runUpdateCandidateCommand(ctx, candidate, runtime, repo, stdout, stderr, "install", args...)
}

func runUpdateDoctor(ctx context.Context, candidate string, runtime commandRuntime, skipHarvest bool, stdout, stderr io.Writer) error {
	var args []string
	if skipHarvest {
		args = []string{"--skip-harvest"}
	}
	// Doctor normally inspects the current repository's publication hook as
	// well as host health. An update is validating the newly installed host
	// binary, not whichever source checkout invoked it, so run from a fresh
	// non-repository directory and keep an unwired maintainer checkout from
	// rolling back an otherwise healthy update.
	doctorDirectory, err := os.MkdirTemp("", "pfm-update-doctor-")
	if err != nil {
		return fmt.Errorf("create isolated doctor directory: %w", err)
	}
	defer func() {
		if cleanupErr := os.RemoveAll(doctorDirectory); cleanupErr != nil {
			fmt.Fprintf(stderr, "pfm update: cleanup isolated doctor directory %s: %v\n", doctorDirectory, cleanupErr)
		}
	}()
	return runUpdateCandidateCommand(ctx, candidate, runtime, doctorDirectory, stdout, stderr, "doctor", args...)
}

func runUpdateCandidateCommand(
	ctx context.Context,
	candidate string,
	runtime commandRuntime,
	workingDirectory string,
	stdout, stderr io.Writer,
	commandName string,
	commandArgs ...string,
) error {
	args := make([]string, 0, len(commandArgs)+3)
	if runtime.Config.Path != "" {
		args = append(args, "--config", runtime.Config.Path)
	}
	args = append(args, commandName)
	args = append(args, commandArgs...)
	command := exec.CommandContext(ctx, candidate, args...)
	command.Dir = workingDirectory
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("target candidate %s: %w", commandName, err)
	}
	return nil
}
