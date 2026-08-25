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

func TestUpdateRefusesSourceDowngrade(t *testing.T) {
	repo := newUpdateGitFixture(t)
	gitTemp(t, repo, "merge", "--ff-only", "--quiet", "v0.10.0")
	runtime := updateTestRuntime(t)

	var stdout, stderr bytes.Buffer
	if code := runUpdate([]string{"--to", "v0.9.0", "--repo", repo}, &stdout, &stderr, runtime); code == 0 {
		t.Fatalf("runUpdate() code = 0, want source-downgrade refusal; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "would downgrade source from v0.10.0") {
		t.Fatalf("runUpdate() stderr = %q, want source-downgrade diagnostic", stderr.String())
	}
}

func TestUpdateReplacesOwnedBinaryLeavesUnownedCopyAndRunsDoctor(t *testing.T) {
	repo := newUpdateGitFixture(t)
	previousBranch := updateGitBranch(t, repo)
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
	oldRollbackInstall := updateRollbackInstall
	oldRollbackDoctor := updateRollbackDoctor
	t.Cleanup(func() {
		updateBuildCandidate = oldBuild
		updateApplyInstall = oldInstall
		updateRunDoctor = oldDoctor
		updateRollbackInstall = oldRollbackInstall
		updateRollbackDoctor = oldRollbackDoctor
	})
	builds := 0
	updateBuildCandidate = func(_ context.Context, _ string, version, output string) error {
		builds++
		if version != "v0.10.0" {
			t.Fatalf("build version=%q, want selected release v0.10.0", version)
		}
		return os.WriteFile(output, []byte("new\n"), 0o755)
	}
	installCalls, doctorCalls := 0, 0
	updateApplyInstall = func(_ context.Context, candidate, workingDirectory string, _ commandRuntime, skipHarvest bool, _ io.Writer, _ io.Writer) error {
		installCalls++
		if !strings.HasSuffix(candidate, "pfm-a") {
			t.Fatalf("install candidate=%q, want first reproducible build", candidate)
		}
		if !skipHarvest {
			t.Fatal("update did not propagate --skip-harvest to install")
		}
		if workingDirectory != repo {
			t.Fatalf("candidate installer working directory=%q, want source repo %q", workingDirectory, repo)
		}
		if got := updateGitRevision(t, repo, "HEAD"); got != updateGitRevision(t, repo, "v0.10.0") {
			t.Fatalf("candidate installer saw source revision %q, want v0.10.0", got)
		}
		return nil
	}
	updateRunDoctor = func(_ context.Context, candidate string, _ commandRuntime, skipHarvest bool, _ io.Writer, _ io.Writer) error {
		doctorCalls++
		if !strings.HasSuffix(candidate, "pfm-a") {
			t.Fatalf("doctor candidate=%q, want first reproducible build", candidate)
		}
		if !skipHarvest {
			t.Fatal("update did not propagate --skip-harvest to doctor")
		}
		return nil
	}

	var stdout, stderr bytes.Buffer
	if code := runUpdate([]string{"--skip-harvest", "--repo", repo}, &stdout, &stderr, runtime); code != 0 {
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
	if got := updateGitBranch(t, repo); got != previousBranch {
		t.Fatalf("source branch after successful update = %q, want unchanged %q", got, previousBranch)
	}
}

func TestUpdateBuildsSelectedTagIntoOwnedBinaryAndSkipsHarvestProvisioning(t *testing.T) {
	jailTest(t)
	repo := newTaggedBuildFixture(t)
	runtime, err := loadCommandRuntime("")
	if err != nil {
		t.Fatal(err)
	}
	canonical := filepath.Join(runtime.Paths.Home, ".local", "bin", "pfm")
	if err := os.WriteFile(canonical, []byte("old\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := installer.RecordCanonicalBinary(runtime.Paths.Home); err != nil {
		t.Fatal(err)
	}

	previousInstaller := runInstaller
	t.Cleanup(func() { runInstaller = previousInstaller })
	var provisionHarvest bool
	runInstaller = func(_ context.Context, options installer.Options) (installer.Report, error) {
		provisionHarvest = options.ProvisionHarvest
		return installer.Report{}, nil
	}

	var stdout, stderr bytes.Buffer
	if code := runUpdate([]string{"--skip-harvest", "--to", "v0.10.0", "--repo", repo}, &stdout, &stderr, runtime); code != 0 {
		t.Fatalf("runUpdate() code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if provisionHarvest {
		t.Fatal("update propagated harvest provisioning despite --skip-harvest")
	}
	if !strings.Contains(stdout.String(), "updated v0.10.0") {
		t.Fatalf("update stdout=%q, want selected tag", stdout.String())
	}

	version := exec.Command(canonical, "version")
	output, err := version.CombinedOutput()
	if err != nil {
		t.Fatalf("updated binary version: %v output=%q", err, output)
	}
	if got := strings.TrimSpace(string(output)); got != "pfm v0.10.0" {
		t.Fatalf("updated binary version=%q, want selected tag v0.10.0", got)
	}
	if got := updateGitBranch(t, repo); got != "installed" {
		t.Fatalf("source branch after update = %q, want installed", got)
	}
	if got := updateGitRevision(t, repo, "HEAD"); got != updateGitRevision(t, repo, "v0.10.0") {
		t.Fatalf("source HEAD after update = %q, want v0.10.0", got)
	}
}

func TestUpdateRunsPostBuildActionsThroughTheSelectedCandidate(t *testing.T) {
	jailTest(t)
	repo := newTaggedBuildFixture(t)
	runtime, err := loadCommandRuntime("")
	if err != nil {
		t.Fatal(err)
	}
	canonical := filepath.Join(runtime.Paths.Home, ".local", "bin", "pfm")
	if err := os.WriteFile(canonical, []byte("old\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := installer.RecordCanonicalBinary(runtime.Paths.Home); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(t.TempDir(), "candidate-argv.log")
	t.Setenv("PFM_UPDATE_CANDIDATE_MARKER", marker)
	previousInstaller := runInstaller
	t.Cleanup(func() { runInstaller = previousInstaller })
	runInstaller = func(_ context.Context, _ installer.Options) (installer.Report, error) {
		return installer.Report{}, nil
	}

	var stdout, stderr bytes.Buffer
	if code := runUpdate([]string{"--skip-harvest", "--to", "v0.10.0", "--repo", repo}, &stdout, &stderr, runtime); code != 0 {
		t.Fatalf("runUpdate() code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	raw, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("read candidate action marker: %v", err)
	}
	if !strings.Contains(string(raw), "install") || !strings.Contains(string(raw), "doctor") {
		t.Fatalf("post-build actions ran outside selected candidate; marker=%q", raw)
	}
}

func TestUpdateRollsBackAfterStagingFailure(t *testing.T) {
	repo := newUpdateGitFixture(t)
	previousBranch := updateGitBranch(t, repo)
	previousRef := updateGitRevision(t, repo, "HEAD")
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
	oldRollbackInstall := updateRollbackInstall
	oldRollbackDoctor := updateRollbackDoctor
	t.Cleanup(func() {
		updateBuildCandidate = oldBuild
		updateApplyInstall = oldInstall
		updateRunDoctor = oldDoctor
		updateRollbackInstall = oldRollbackInstall
		updateRollbackDoctor = oldRollbackDoctor
	})
	updateBuildCandidate = func(_ context.Context, _ string, _ string, output string) error {
		return os.WriteFile(output, []byte("new\n"), 0o755)
	}
	managedMutation := filepath.Join(runtime.Paths.Home, ".local", "share", "pfm", "install", "new-asset")
	updateApplyInstall = func(context.Context, string, string, commandRuntime, bool, io.Writer, io.Writer) error {
		if err := os.MkdirAll(filepath.Dir(managedMutation), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(managedMutation, []byte("partially installed\n"), 0o600); err != nil {
			return err
		}
		return errors.New("injected install failure")
	}
	updateRunDoctor = func(context.Context, string, commandRuntime, bool, io.Writer, io.Writer) error {
		t.Fatal("doctor ran after install failure")
		return nil
	}
	updateRollbackInstall = func(_ context.Context, candidate, workingDirectory string, _ commandRuntime, _ bool, _ io.Writer, _ io.Writer) error {
		if !strings.Contains(candidate, "previous-") {
			t.Fatalf("rollback installer candidate=%q, want preserved previous binary", candidate)
		}
		if workingDirectory != repo {
			t.Fatalf("rollback installer working directory=%q, want restored source repo %q", workingDirectory, repo)
		}
		if got := updateGitRevision(t, repo, "HEAD"); got != previousRef {
			t.Fatalf("rollback installer saw source revision %q, want previous %q", got, previousRef)
		}
		return os.RemoveAll(filepath.Dir(managedMutation))
	}
	updateRollbackDoctor = func(_ context.Context, candidate string, _ commandRuntime, _ bool, _ io.Writer, _ io.Writer) error {
		if !strings.Contains(candidate, "previous-") {
			t.Fatalf("rollback doctor candidate=%q, want preserved previous binary", candidate)
		}
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
	if _, err := os.Stat(managedMutation); !os.IsNotExist(err) {
		t.Fatalf("installer mutation survived rollback: %v", err)
	}
	if got := updateGitBranch(t, repo); got != previousBranch {
		t.Fatalf("source branch after failed update = %q, want unchanged %q", got, previousBranch)
	}
	if got := updateGitRevision(t, repo, "HEAD"); got != previousRef {
		t.Fatalf("source revision after failed update = %q, want unchanged %q", got, previousRef)
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
		if err := os.WriteFile(path, []byte("maintainer-only\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	blueprint := map[string]string{
		"blueprint/CLAUDE.md":              "{{PROJECT_NAME}} contract\n",
		"blueprint/settings.json":          "{}\n",
		"blueprint/output-styles/style.md": "{{PROJECT_NAME}} style\n",
		"blueprint/commands/command.md":    "{{PROJECT_NAME}} command\n",
		"blueprint/agents/agent.md":        "{{PROJECT_NAME}} agent\n",
		"blueprint/skills/skill/SKILL.md":  "{{PROJECT_NAME}} skill\n",
	}
	for relative, content := range blueprint {
		path := filepath.Join(source, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
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
	stale := filepath.Join(target, ".claude", "commands", "stale.md")
	if err := os.MkdirAll(filepath.Dir(stale), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stale, []byte("stale\n"), 0o600); err != nil {
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
	want := map[string]string{
		"CLAUDE.md":                      "{{PROJECT_NAME}} contract\n",
		"AGENTS.md":                      "{{PROJECT_NAME}} contract\n",
		".claude/settings.json":          "{}\n",
		".claude/output-styles/style.md": "{{PROJECT_NAME}} style\n",
		".claude/commands/command.md":    "{{PROJECT_NAME}} command\n",
		".claude/agents/agent.md":        "{{PROJECT_NAME}} agent\n",
		".claude/skills/skill/SKILL.md":  "{{PROJECT_NAME}} skill\n",
	}
	for relative, expected := range want {
		got, err := os.ReadFile(filepath.Join(target, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatalf("copied %s: %v", relative, err)
		}
		if string(got) != expected {
			t.Fatalf("copied %s = %q, want adopter blueprint %q", relative, got, expected)
		}
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("--force overlaid instead of replacing stale command: %v", err)
	}
	if !strings.Contains(stdout.String(), "open Claude here and follow "+filepath.Join(source, "docs", "SETUP.md")) {
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
	if err := os.WriteFile(filepath.Join(repo, "RELEASE"), []byte("next\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitTemp(t, repo, "add", "RELEASE")
	gitTemp(t, repo, "commit", "-qm", "fixture next release")
	gitTemp(t, repo, "tag", "v0.10.0")
	remote := filepath.Join(t.TempDir(), "remote.git")
	if err := os.MkdirAll(remote, 0o700); err != nil {
		t.Fatal(err)
	}
	gitTemp(t, remote, "init", "--bare", "-q")
	gitTemp(t, repo, "remote", "add", "origin", remote)
	gitTemp(t, repo, "push", "-q", "origin", "HEAD", "--tags")
	gitTemp(t, repo, "checkout", "-qb", "installed", "v0.9.0")
	return repo
}

func newTaggedBuildFixture(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	gitTemp(t, repo, "init", "-q")
	gitTemp(t, repo, "config", "user.email", "fixture.invalid")
	gitTemp(t, repo, "config", "user.name", "fixture-identity")
	mainPath := filepath.Join(repo, "pfm", "cmd", "pfm", "main.go")
	if err := os.MkdirAll(filepath.Dir(mainPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "pfm", "go.mod"), []byte("module fixture.invalid/pfm\n\ngo 1.24\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// The fixture command only needs version output. Keep the source small and
	// deterministic so two staged update builds hash identically.
	if err := os.WriteFile(mainPath, []byte("package main\n\nimport (\n\t\"fmt\"\n\t\"os\"\n)\n\nvar version = \"dev\"\n\nfunc main() {\n\tif marker := os.Getenv(\"PFM_UPDATE_CANDIDATE_MARKER\"); marker != \"\" {\n\t\tfile, err := os.OpenFile(marker, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)\n\t\tif err != nil {\n\t\t\tpanic(err)\n\t\t}\n\t\tfmt.Fprintln(file, os.Args[1:])\n\t\t_ = file.Close()\n\t}\n\tfmt.Println(\"pfm\", version)\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitTemp(t, repo, "add", ".")
	gitTemp(t, repo, "commit", "-qm", "fixture previous release")
	gitTemp(t, repo, "tag", "v0.9.0")
	if err := os.WriteFile(filepath.Join(repo, ".e2e-current-source"), []byte("current\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitTemp(t, repo, "add", ".e2e-current-source")
	gitTemp(t, repo, "commit", "-qm", "fixture current release")
	gitTemp(t, repo, "tag", "v0.10.0")
	remote := filepath.Join(t.TempDir(), "remote.git")
	if err := os.MkdirAll(remote, 0o700); err != nil {
		t.Fatal(err)
	}
	gitTemp(t, remote, "init", "--bare", "-q")
	gitTemp(t, repo, "remote", "add", "origin", remote)
	gitTemp(t, repo, "push", "-q", "origin", "HEAD", "--tags")
	gitTemp(t, repo, "checkout", "-qb", "installed", "v0.9.0")
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

func updateGitBranch(t *testing.T, repo string) string {
	t.Helper()
	command := exec.Command("git", "symbolic-ref", "--quiet", "--short", "HEAD")
	command.Dir = repo
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("resolve fixture branch: %v\n%s", err, output)
	}
	return strings.TrimSpace(string(output))
}

func updateGitRevision(t *testing.T, repo, revision string) string {
	t.Helper()
	command := exec.Command("git", "rev-parse", "--verify", revision)
	command.Dir = repo
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("resolve fixture revision %s: %v\n%s", revision, err, output)
	}
	return strings.TrimSpace(string(output))
}
