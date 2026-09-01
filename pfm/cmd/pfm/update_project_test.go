package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"hostops/pfm/internal/paths"
	"hostops/pfm/internal/professor"
)

func TestUpdateCheckReportsEveryProjectStatusAndIsSideEffectFree(t *testing.T) {
	fixture := newProjectUpdateFixture(t)
	before := projectTreeDigest(t, fixture.project)
	var stdout, stderr bytes.Buffer
	code := runUpdate([]string{"check", "--root", fixture.project}, &stdout, &stderr, fixture.runtime)
	if code != 3 {
		t.Fatalf("runUpdate(check) code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	for _, want := range []string{
		"current       1",
		"UPDATED       2",
		"NEW           1",
		"GONE-UPSTREAM 1",
		"LOCAL-DELETED 1",
		"review: git -C " + fixture.store,
		"REVIEW REQUIRED — 5 items",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("check output missing %q:\n%s", want, stdout.String())
		}
	}
	if got := projectTreeDigest(t, fixture.project); got != before {
		t.Fatalf("check mutated project tree: before=%s after=%s", before, got)
	}
	if strings.Contains(stdout.String(), "UNKNOWN") {
		t.Fatalf("check invented an unknown status:\n%s", stdout.String())
	}
	t.Logf("fixture pfm update check output:\n%s", stdout.String())
}

func TestUpdatePinAdvancesOnlySelectedFilesAndDropClearsDeferredStates(t *testing.T) {
	fixture := newProjectUpdateFixture(t)
	var stdout, stderr bytes.Buffer
	if code := runUpdate([]string{"pin", ".claude/updated-one.md", "--root", fixture.project}, &stdout, &stderr, fixture.runtime); code != 0 {
		t.Fatalf("pin one code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	baseline, err := professor.Load(fixture.project)
	if err != nil {
		t.Fatal(err)
	}
	if baseline.Blueprint.SHA != fixture.headSHA || baseline.Files[".claude/updated-one.md"].PinnedSHA != fixture.headSHA {
		t.Fatalf("pin did not advance selected file: %#v", baseline)
	}
	if baseline.Files[".claude/updated-two.md"].PinnedSHA != fixture.oldSHA {
		t.Fatalf("pin advanced deferred file: %#v", baseline.Files[".claude/updated-two.md"])
	}
	stdout.Reset()
	stderr.Reset()
	if code := runUpdate([]string{"check", "--root", fixture.project}, &stdout, &stderr, fixture.runtime); code != 3 || !strings.Contains(stdout.String(), "UPDATED       1") || !strings.Contains(stdout.String(), ".claude/updated-two.md") {
		t.Fatalf("deferred check code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	newLocal := filepath.Join(fixture.project, ".claude", "new.md")
	if err := os.WriteFile(newLocal, []byte("adapted locally\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := runUpdate([]string{"pin", "--template", "project/new.md", ".claude/new.md", "--root", fixture.project}, &stdout, &stderr, fixture.runtime); code != 0 {
		t.Fatalf("pin new code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := runUpdate([]string{"pin", "--all", "--root", fixture.project}, &stdout, &stderr, fixture.runtime); code != 0 {
		t.Fatalf("pin all code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := runUpdate([]string{"drop", ".claude/gone.md", "--root", fixture.project}, &stdout, &stderr, fixture.runtime); code != 0 {
		t.Fatalf("drop code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	writeProjectFixtureFile(t, fixture.project, ".claude/deleted.md", "restored locally\n")
	stdout.Reset()
	stderr.Reset()
	if code := runUpdate([]string{"pin", ".claude/deleted.md", "--root", fixture.project}, &stdout, &stderr, fixture.runtime); code != 0 {
		t.Fatalf("pin restored code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := runUpdate([]string{"check", "--root", fixture.project}, &stdout, &stderr, fixture.runtime); code != 0 || !strings.HasSuffix(stdout.String(), "clean\n") {
		t.Fatalf("clean check code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestUpdateCheckJSONIsOneObjectAndUnreadableBaselineFails(t *testing.T) {
	fixture := newProjectUpdateFixture(t)
	var stdout, stderr bytes.Buffer
	if code := runUpdate([]string{"check", "--json", "--root", fixture.project}, &stdout, &stderr, fixture.runtime); code != 3 {
		t.Fatalf("json check code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	var object map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &object); err != nil {
		t.Fatalf("json output is not one object: %v\n%s", err, stdout.String())
	}
	if object["reviewRequired"] != float64(5) {
		t.Fatalf("reviewRequired=%#v", object["reviewRequired"])
	}

	if err := os.WriteFile(professor.BaselinePath(fixture.project), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := runUpdate([]string{"check", "--root", fixture.project}, &stdout, &stderr, fixture.runtime); code != 1 || !strings.Contains(stdout.String(), "FAILED — BASELINE-MALFORMED") {
		t.Fatalf("malformed check code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestProfessorDoctorProjectLine(t *testing.T) {
	fixture := newProjectUpdateFixture(t)
	var output bytes.Buffer
	warnings := printProfessorDoctor(&output, fixture.project, fixture.runtime.Paths.Home)
	if warnings != 0 || !strings.Contains(output.String(), "professor: current 1 · review-required 5") {
		t.Fatalf("doctor warnings=%d output=%q", warnings, output.String())
	}
	if err := os.WriteFile(professor.BaselinePath(fixture.project), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	output.Reset()
	warnings = printProfessorDoctor(&output, fixture.project, fixture.runtime.Paths.Home)
	if warnings != 1 || !strings.Contains(output.String(), "professor: UNREADABLE") {
		t.Fatalf("unreadable doctor warnings=%d output=%q", warnings, output.String())
	}
}

type projectUpdateFixture struct {
	project string
	store   string
	oldSHA  string
	headSHA string
	runtime commandRuntime
}

func newProjectUpdateFixture(t *testing.T) projectUpdateFixture {
	t.Helper()
	storeRoot := filepath.Join(t.TempDir(), "blueprint")
	if err := os.MkdirAll(filepath.Join(storeRoot, "templates", "project"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeProjectFixtureFile(t, storeRoot, "VERSION", "0.65.0\n")
	writeProjectFixtureFile(t, storeRoot, "templates/project/current.md", "current\n")
	writeProjectFixtureFile(t, storeRoot, "templates/project/updated-one.md", "old one\n")
	writeProjectFixtureFile(t, storeRoot, "templates/project/updated-two.md", "old two\n")
	writeProjectFixtureFile(t, storeRoot, "templates/project/deleted.md", "deleted local\n")
	writeProjectFixtureFile(t, storeRoot, "templates/project/gone.md", "gone\n")
	gitTemp(t, storeRoot, "init", "-q")
	gitTemp(t, storeRoot, "config", "user.email", "fixture.invalid")
	gitTemp(t, storeRoot, "config", "user.name", "fixture-identity")
	gitTemp(t, storeRoot, "add", ".")
	gitTemp(t, storeRoot, "commit", "-qm", "old templates")
	oldSHA := projectGitShortSHA(t, storeRoot)
	writeProjectFixtureFile(t, storeRoot, "templates/project/updated-one.md", "new one\n")
	writeProjectFixtureFile(t, storeRoot, "templates/project/updated-two.md", "new two\n")
	if err := os.Remove(filepath.Join(storeRoot, "templates", "project", "gone.md")); err != nil {
		t.Fatal(err)
	}
	writeProjectFixtureFile(t, storeRoot, "templates/project/new.md", "new upstream\n")
	gitTemp(t, storeRoot, "add", "-A")
	gitTemp(t, storeRoot, "commit", "-qm", "new templates")
	headSHA := projectGitShortSHA(t, storeRoot)

	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, ".professor"), 0o700); err != nil {
		t.Fatal(err)
	}
	manifest := fmt.Sprintf("{\"interview\":{\"blueprint_clone_path\":%q}}\n", storeRoot)
	if err := os.WriteFile(filepath.Join(project, ".professor", "manifest.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{".claude/current.md", ".claude/updated-one.md", ".claude/updated-two.md", ".claude/gone.md"} {
		writeProjectFixtureFile(t, project, relative, "local truth\n")
	}
	hash := func(template string) string {
		t.Helper()
		value, err := professor.HashTemplate(filepath.Join(storeRoot, "templates", filepath.FromSlash(template)))
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	oldHash := func(content string) string {
		return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(content)))
	}
	baseline := professor.Baseline{
		Version:   professor.BaselineVersion,
		Blueprint: professor.BlueprintPin{Version: "0.64.0", SHA: oldSHA},
		Files: map[string]professor.FilePin{
			".claude/current.md":     {Template: "project/current.md", TemplateHash: hash("project/current.md"), PinnedSHA: oldSHA, PinnedAt: "2026-08-31"},
			".claude/updated-one.md": {Template: "project/updated-one.md", TemplateHash: oldHash("old one\n"), PinnedSHA: oldSHA, PinnedAt: "2026-08-31"},
			".claude/updated-two.md": {Template: "project/updated-two.md", TemplateHash: oldHash("old two\n"), PinnedSHA: oldSHA, PinnedAt: "2026-08-31"},
			".claude/deleted.md":     {Template: "project/deleted.md", TemplateHash: hash("project/deleted.md"), PinnedSHA: oldSHA, PinnedAt: "2026-08-31"},
			".claude/gone.md":        {Template: "project/gone.md", TemplateHash: oldHash("gone\n"), PinnedSHA: oldSHA, PinnedAt: "2026-08-31"},
		},
	}
	if err := professor.Save(project, baseline); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	return projectUpdateFixture{project: project, store: storeRoot, oldSHA: oldSHA, headSHA: headSHA, runtime: commandRuntime{Paths: paths.Values{Home: home}}}
}

func writeProjectFixtureFile(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func projectGitShortSHA(t *testing.T, root string) string {
	t.Helper()
	command := exec.Command("git", "rev-parse", "--short", "HEAD")
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git rev-parse --short HEAD: %v\n%s", err, output)
	}
	return strings.TrimSpace(string(output))
}

func projectTreeDigest(t *testing.T, root string) string {
	t.Helper()
	var rows []string
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rows = append(rows, fmt.Sprintf("%s:%x", filepath.ToSlash(relative), sha256.Sum256(raw)))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sort.Strings(rows)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(rows, "\n"))))
}
