package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"hostops/pfm/internal/installer"
	"hostops/pfm/internal/paths"
)

func TestSelectHighestSemverUsesParsedComponents(t *testing.T) {
	got, err := selectHighestSemver([]string{"v0.9.0", "v0.10.0", "v0.10.0-rc1", "notes"})
	if err != nil {
		t.Fatalf("selectHighestSemver() error = %v", err)
	}
	if got != "v0.10.0" {
		t.Fatalf("selectHighestSemver() = %q, want v0.10.0", got)
	}
}

func TestUpdateRefusesDirtyWorktree(t *testing.T) {
	repo := newUpdateGitFixture(t)
	if err := os.WriteFile(filepath.Join(repo, "dirty.txt"), []byte("dirty\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	runtime := updateTestRuntime(t)
	var stdout, stderr bytes.Buffer
	if code := runUpdate([]string{"--repo", repo}, &stdout, &stderr, runtime); code == 0 {
		t.Fatalf("runUpdate() code = 0, want dirty-worktree refusal; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "dirty worktree") {
		t.Fatalf("runUpdate() stderr = %q, want dirty-worktree diagnostic", stderr.String())
	}
}

func TestUpdateReplacesOwnedBinaryLeavesUnownedCopyAndRunsDoctor(t *testing.T) {
	repo := newUpdateGitFixture(t)
	runtime := updateTestRuntime(t)
	canonical := filepath.Join(runtime.Paths.Home, ".local", "bin", "pfm")
	unowned := filepath.Join(t.TempDir(), "pfm")
	if err := os.MkdirAll(filepath.Dir(canonical), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(canonical, []byte("old\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(unowned, []byte("unowned\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := installer.RecordCanonicalBinary(runtime.Paths.Home); err != nil {
		t.Fatal(err)
	}

	oldBuild := updateBuildCandidate
	oldInstall := updateApplyInstall
	oldDoctor := updateRunDoctor
	t.Cleanup(func() {
		updateBuildCandidate = oldBuild
		updateApplyInstall = oldInstall
		updateRunDoctor = oldDoctor
	})
	builds := 0
	updateBuildCandidate = func(_ context.Context, _ string, output string) error {
		builds++
		return os.WriteFile(output, []byte("new\n"), 0o755)
	}
	installCalls, doctorCalls := 0, 0
	updateApplyInstall = func(context.Context, commandRuntime, io.Writer, io.Writer) error {
		installCalls++
		return nil
	}
	updateRunDoctor = func(context.Context, commandRuntime, io.Writer, io.Writer) error {
		doctorCalls++
		return nil
	}

	var stdout, stderr bytes.Buffer
	if code := runUpdate([]string{"--repo", repo}, &stdout, &stderr, runtime); code != 0 {
		t.Fatalf("runUpdate() code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if builds != 2 {
		t.Fatalf("build calls=%d, want 2", builds)
	}
	if installCalls != 1 || doctorCalls != 1 {
		t.Fatalf("install calls=%d doctor calls=%d, want 1/1", installCalls, doctorCalls)
	}
	if got, err := os.ReadFile(canonical); err != nil || string(got) != "new\n" {
		t.Fatalf("canonical binary=%q err=%v, want new", got, err)
	}
	if got, err := os.ReadFile(unowned); err != nil || string(got) != "unowned\n" {
		t.Fatalf("unowned binary=%q err=%v, want unchanged", got, err)
	}
}

func TestUpdateRollsBackAfterStagingFailure(t *testing.T) {
	repo := newUpdateGitFixture(t)
	runtime := updateTestRuntime(t)
	canonical := filepath.Join(runtime.Paths.Home, ".local", "bin", "pfm")
	if err := os.MkdirAll(filepath.Dir(canonical), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(canonical, []byte("old\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := installer.RecordCanonicalBinary(runtime.Paths.Home); err != nil {
		t.Fatal(err)
	}

	oldBuild := updateBuildCandidate
	oldInstall := updateApplyInstall
	oldDoctor := updateRunDoctor
	t.Cleanup(func() {
		updateBuildCandidate = oldBuild
		updateApplyInstall = oldInstall
		updateRunDoctor = oldDoctor
	})
	updateBuildCandidate = func(_ context.Context, _ string, output string) error {
		return os.WriteFile(output, []byte("new\n"), 0o755)
	}
	updateApplyInstall = func(context.Context, commandRuntime, io.Writer, io.Writer) error {
		return errors.New("injected install failure")
	}
	updateRunDoctor = func(context.Context, commandRuntime, io.Writer, io.Writer) error {
		t.Fatal("doctor ran after install failure")
		return nil
	}

	var stdout, stderr bytes.Buffer
	if code := runUpdate([]string{"--repo", repo}, &stdout, &stderr, runtime); code == 0 {
		t.Fatalf("runUpdate() code=0, want failure; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if got, err := os.ReadFile(canonical); err != nil || string(got) != "old\n" {
		t.Fatalf("canonical after rollback=%q err=%v, want old", got, err)
	}
	if !strings.Contains(stderr.String(), "rolled back") {
		t.Fatalf("rollback diagnostic=%q", stderr.String())
	}
}

func TestInitCopiesRecordedBlueprintAndHonorsForce(t *testing.T) {
	source := t.TempDir()
	home := t.TempDir()
	for _, relative := range []string{
		"CLAUDE.md",
		"AGENTS.md",
		".claude/settings.json",
		".claude/output-styles/style.md",
		".claude/commands/command.md",
		".claude/agents/agent.md",
		".claude/skills/skill/SKILL.md",
	} {
		path := filepath.Join(source, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("{{PLACEHOLDER}}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := installer.WriteSourceRepoMarker(home, source); err != nil {
		t.Fatal(err)
	}
	target := t.TempDir()
	if err := os.Mkdir(filepath.Join(target, ".claude"), 0o700); err != nil {
		t.Fatal(err)
	}
	runtime := commandRuntime{Paths: paths.Values{Home: home}}
	var stdout, stderr bytes.Buffer
	if code := runInit([]string{target}, &stdout, &stderr, runtime); code == 0 {
		t.Fatalf("runInit() code=0, want existing-.claude refusal; stderr=%q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "--force") {
		t.Fatalf("runInit() refusal=%q", stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := runInit([]string{"--force", target}, &stdout, &stderr, runtime); code != 0 {
		t.Fatalf("runInit(--force) code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	for _, relative := range []string{
		"CLAUDE.md",
		"AGENTS.md",
		".claude/settings.json",
		".claude/output-styles/style.md",
		".claude/commands/command.md",
		".claude/agents/agent.md",
		".claude/skills/skill/SKILL.md",
	} {
		got, err := os.ReadFile(filepath.Join(target, relative))
		if err != nil {
			t.Fatalf("copied %s: %v", relative, err)
		}
		if string(got) != "{{PLACEHOLDER}}\n" {
			t.Fatalf("copied %s = %q, placeholders changed", relative, got)
		}
	}
	if !strings.Contains(stdout.String(), "open Claude here and follow docs/SETUP.md") {
		t.Fatalf("init handoff=%q", stdout.String())
	}
}

func updateTestRuntime(t *testing.T) commandRuntime {
	t.Helper()
	home := t.TempDir()
	t.Setenv(paths.EnvHome, home)
	return commandRuntime{Paths: paths.Values{Home: home}}
}

func newUpdateGitFixture(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	gitTemp(t, repo, "init", "-q")
	gitTemp(t, repo, "config", "user.email", "fixture.invalid")
	gitTemp(t, repo, "config", "user.name", "fixture-identity")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitTemp(t, repo, "add", "README.md")
	gitTemp(t, repo, "commit", "-qm", "fixture")
	gitTemp(t, repo, "tag", "v0.9.0")
	gitTemp(t, repo, "tag", "v0.10.0")
	remote := filepath.Join(t.TempDir(), "remote.git")
	if err := os.MkdirAll(remote, 0o700); err != nil {
		t.Fatal(err)
	}
	gitTemp(t, remote, "init", "--bare", "-q")
	gitTemp(t, repo, "remote", "add", "origin", remote)
	gitTemp(t, repo, "push", "-q", "origin", "HEAD", "--tags")
	return repo
}

func gitTemp(t *testing.T, repo string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = repo
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}
