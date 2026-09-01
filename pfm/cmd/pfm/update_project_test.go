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
		"ignored       0",
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

func TestUpdateAdoptPinsExistingInstall(t *testing.T) {
	store := newScaffoldStoreFixture(t)
	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, ".professor"), 0o700); err != nil {
		t.Fatal(err)
	}
	manifest := fmt.Sprintf("{\"interview\":{\"blueprint_clone_path\":%q}}\n", store)
	if err := os.WriteFile(filepath.Join(project, ".professor", "manifest.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	writeProjectFixtureFile(t, project, "CLAUDE.md", "# local\n")
	writeProjectFixtureFile(t, project, ".claude/commands/dev.md", "---\nname: dev\n---\nlocal\n")
	home := t.TempDir()
	runtime := commandRuntime{Paths: paths.Values{Home: home}}
	headSHA := projectGitShortSHA(t, store)

	storeStruct, err := professor.ResolveStore(project, home)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := planInitCopies(storeStruct)
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if code := runUpdate([]string{"adopt", "--root", project}, &stdout, &stderr, runtime); code != 0 {
		t.Fatalf("adopt code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "adopted 2 file(s)") {
		t.Fatalf("adopt stdout=%q", stdout.String())
	}
	wantAbsent := len(plan) - 2
	if !strings.Contains(stdout.String(), fmt.Sprintf("absent        %d", wantAbsent)) {
		t.Fatalf("adopt absent count stdout=%q, want absent %d (plan size %d)", stdout.String(), wantAbsent, len(plan))
	}

	baseline, err := professor.Load(project)
	if err != nil {
		t.Fatal(err)
	}
	if len(baseline.Files) != 2 {
		t.Fatalf("pinned files=%#v, want exactly 2", baseline.Files)
	}
	if baseline.Blueprint.SHA != headSHA {
		t.Fatalf("Blueprint.SHA=%q, want store HEAD %q", baseline.Blueprint.SHA, headSHA)
	}

	stdout.Reset()
	stderr.Reset()
	if code := runUpdate([]string{"adopt", "--root", project}, &stdout, &stderr, runtime); code != 0 {
		t.Fatalf("second adopt code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "adopted 0 file(s)") || !strings.Contains(stdout.String(), "kept          2") {
		t.Fatalf("second adopt stdout=%q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	// The fixture's raw template tree (walked by check's NEW pass) also
	// carries templates/project/agents/per-project/developer.md — a file
	// planInitCopies deliberately skips, so it is never in plan and is
	// never absent, but check still counts it NEW: NEW = plan size - 2 + 1.
	if code := runUpdate([]string{"check", "--root", project}, &stdout, &stderr, runtime); code != 3 {
		t.Fatalf("check code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	wantNew := len(plan) - 2 + 1
	for _, want := range []string{"current       2", fmt.Sprintf("NEW           %d", wantNew)} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("check output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestUpdateAdoptAtPinsAgainstBlueprintRefAndValidatesIt(t *testing.T) {
	store, oldSHA, headSHA := newAdoptAtStoreFixture(t)
	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, ".professor"), 0o700); err != nil {
		t.Fatal(err)
	}
	manifest := fmt.Sprintf("{\"interview\":{\"blueprint_clone_path\":%q}}\n", store)
	if err := os.WriteFile(filepath.Join(project, ".professor", "manifest.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	writeProjectFixtureFile(t, project, ".claude/commands/dev.md", "local dev\n")
	writeProjectFixtureFile(t, project, ".claude/commands/later.md", "local later\n")
	home := t.TempDir()
	runtime := commandRuntime{Paths: paths.Values{Home: home}}

	var stdout, stderr bytes.Buffer
	if code := runUpdate([]string{"adopt", "--root", project, "--at", oldSHA}, &stdout, &stderr, runtime); code != 0 {
		t.Fatalf("adopt --at code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "absent-at-ref 1") {
		t.Fatalf("adopt --at stdout missing absent-at-ref: %q", stdout.String())
	}
	baseline, err := professor.Load(project)
	if err != nil {
		t.Fatal(err)
	}
	devPin, pinned := baseline.Files[".claude/commands/dev.md"]
	if !pinned {
		t.Fatalf("dev.md was not pinned: %#v", baseline.Files)
	}
	wantHash := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("old\n")))
	if devPin.TemplateHash != wantHash {
		t.Fatalf("dev.md TemplateHash=%q, want %q", devPin.TemplateHash, wantHash)
	}
	if devPin.PinnedSHA != oldSHA {
		t.Fatalf("dev.md PinnedSHA=%q, want %q", devPin.PinnedSHA, oldSHA)
	}
	if baseline.Blueprint.SHA != oldSHA {
		t.Fatalf("Blueprint.SHA=%q, want %q", baseline.Blueprint.SHA, oldSHA)
	}
	if _, pinned := baseline.Files[".claude/commands/later.md"]; pinned {
		t.Fatal("later.md was pinned even though it did not exist at --at ref")
	}

	stdout.Reset()
	stderr.Reset()
	if code := runUpdate([]string{"check", "--root", project}, &stdout, &stderr, runtime); code != 3 {
		t.Fatalf("check code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), ".claude/commands/dev.md   project/commands/dev.md  pinned @"+oldSHA) ||
		!strings.Contains(stdout.String(), fmt.Sprintf("review: git -C %s diff %s..%s", store, oldSHA, headSHA)) {
		t.Fatalf("check did not report dev.md UPDATED with the expected review line:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "project/commands/later.md — adopt:") {
		t.Fatalf("check did not report later.md NEW:\n%s", stdout.String())
	}

	noGitStore := t.TempDir()
	for relative, content := range map[string]string{
		"VERSION":                                                  "0.65.0\n",
		"templates/project/CLAUDE.md":                              "# x\n",
		"templates/project/settings.json":                          "{}\n",
		"templates/project/commands/dev.md":                        "old\n",
		"templates/project/agents/gitter.md":                       "body\n",
		"templates/project/scripts/dev.sh":                         "#!/usr/bin/env bash\n",
		"templates/project/skills/legal/SKILL.md":                  "body\n",
		"templates/project/workflows/audit.js":                     "export default {};\n",
		"templates/project/codex/config.toml":                      "model=\"x\"\n",
		"templates/project/docs-commands/jc/references/jc-core.md": "# JC\n",
		"templates/project/docs-agents/_index.md":                  "# Agents\n",
	} {
		writeProjectFixtureFile(t, noGitStore, relative, content)
	}
	noGitProject := t.TempDir()
	if err := os.MkdirAll(filepath.Join(noGitProject, ".professor"), 0o700); err != nil {
		t.Fatal(err)
	}
	noGitManifest := fmt.Sprintf("{\"interview\":{\"blueprint_clone_path\":%q}}\n", noGitStore)
	if err := os.WriteFile(filepath.Join(noGitProject, ".professor", "manifest.json"), []byte(noGitManifest), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := runUpdate([]string{"adopt", "--root", noGitProject, "--at", "HEAD"}, &stdout, &stderr, runtime); code != 1 || !strings.Contains(stderr.String(), "--at requires a git blueprint clone") {
		t.Fatalf("no-.git adopt --at code=%d stderr=%q", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := runUpdate([]string{"adopt", "--root", project, "--at", "nosuchref"}, &stdout, &stderr, runtime); code != 1 || !strings.Contains(stderr.String(), "resolve --at nosuchref") {
		t.Fatalf("bad --at ref code=%d stderr=%q", code, stderr.String())
	}
}

// TestUpdateIgnoreManagesBaselineIgnored is a REGRESSION test for the NEW
// walk skipping ignored templates: watched failing against a build where
// buildProjectReport's ignored-skip was neutralized (every ignored template
// still surfaced NEW) — see the RED-first record in the qa report.
func TestUpdateIgnoreManagesBaselineIgnored(t *testing.T) {
	fixture := newProjectUpdateFixture(t)
	var stdout, stderr bytes.Buffer
	if code := runUpdate([]string{"ignore", "project/new.md", "--root", fixture.project}, &stdout, &stderr, fixture.runtime); code != 0 {
		t.Fatalf("ignore new.md code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "ignored 1 template(s)") {
		t.Fatalf("ignore stdout=%q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := runUpdate([]string{"check", "--root", fixture.project}, &stdout, &stderr, fixture.runtime); code != 3 {
		t.Fatalf("check after ignore code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	for _, want := range []string{"NEW           0", "ignored       1", "REVIEW REQUIRED — 4 items"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("check after ignore missing %q:\n%s", want, stdout.String())
		}
	}

	stdout.Reset()
	stderr.Reset()
	if code := runUpdate([]string{"check", "--json", "--root", fixture.project}, &stdout, &stderr, fixture.runtime); code != 3 {
		t.Fatalf("check --json after ignore code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	var object map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &object); err != nil {
		t.Fatalf("json output is not one object: %v\n%s", err, stdout.String())
	}
	ignoredList, ok := object["ignored"].([]any)
	if !ok || len(ignoredList) != 1 || ignoredList[0] != "project/new.md" {
		t.Fatalf("json ignored=%#v, want [\"project/new.md\"]", object["ignored"])
	}

	stdout.Reset()
	stderr.Reset()
	if code := runUpdate([]string{"ignore", "project/current.md", "--root", fixture.project}, &stdout, &stderr, fixture.runtime); code != 1 || !strings.Contains(stderr.String(), "is pinned by .claude/current.md") {
		t.Fatalf("ignore pinned template code=%d stderr=%q", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := runUpdate([]string{"ignore", "project/nope.md", "--root", fixture.project}, &stdout, &stderr, fixture.runtime); code != 1 || !strings.Contains(stderr.String(), "does not exist upstream") {
		t.Fatalf("ignore missing template code=%d stderr=%q", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := runUpdate([]string{"ignore", "--undo", "project/new.md", "--root", fixture.project}, &stdout, &stderr, fixture.runtime); code != 0 || !strings.Contains(stdout.String(), "un-ignored 1 template(s)") {
		t.Fatalf("undo ignore code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := runUpdate([]string{"check", "--root", fixture.project}, &stdout, &stderr, fixture.runtime); code != 3 || !strings.Contains(stdout.String(), "NEW           1") {
		t.Fatalf("check after undo code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := runUpdate([]string{"ignore", "project/new.md", "--root", fixture.project}, &stdout, &stderr, fixture.runtime); code != 0 {
		t.Fatalf("re-ignore code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	writeProjectFixtureFile(t, fixture.project, ".claude/new.md", "adapted locally\n")
	stdout.Reset()
	stderr.Reset()
	if code := runUpdate([]string{"pin", "--template", "project/new.md", ".claude/new.md", "--root", fixture.project}, &stdout, &stderr, fixture.runtime); code != 0 {
		t.Fatalf("pin adopted ignored template code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	baseline, err := professor.Load(fixture.project)
	if err != nil {
		t.Fatal(err)
	}
	if len(baseline.Ignored) != 0 {
		t.Fatalf("baseline.Ignored=%#v after pinning a formerly ignored template, want empty", baseline.Ignored)
	}
}

// TestUpdateCheckMissingBaselineNamesAdoptCommand is a REGRESSION test for
// the "no baseline" message text: watched failing against the old text
// ".professor/baseline.json not found" (no "pfm update adopt" guidance) —
// see the RED-first record in the qa report.
func TestUpdateCheckMissingBaselineNamesAdoptCommand(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	runtime := commandRuntime{Paths: paths.Values{Home: home}}
	var stdout, stderr bytes.Buffer
	if code := runUpdate([]string{"check", "--root", dir}, &stdout, &stderr, runtime); code != 1 {
		t.Fatalf("missing baseline check code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "pfm update adopt pins an existing install") {
		t.Fatalf("missing baseline stdout=%q, want it to name pfm update adopt", stdout.String())
	}
}

// newAdoptAtStoreFixture builds a full planInitCopies-compatible blueprint
// store (every initTemplatePaths source must exist or planInitCopies fails)
// with two commits: old has templates/project/commands/dev.md = "old\n";
// new changes dev.md to "new\n" and adds templates/project/commands/later.md.
func newAdoptAtStoreFixture(t *testing.T) (store, oldSHA, headSHA string) {
	t.Helper()
	root := t.TempDir()
	for relative, content := range map[string]string{
		"VERSION":                                                  "0.65.0\n",
		"templates/project/CLAUDE.md":                              "# {TOKEN} contract\n",
		"templates/project/settings.json":                          "{}\n",
		"templates/project/commands/dev.md":                        "old\n",
		"templates/project/agents/gitter.md":                       "---\nname: gitter\n---\nbody\n",
		"templates/project/scripts/dev.sh":                         "#!/usr/bin/env bash\nset -euo pipefail\n",
		"templates/project/skills/legal/SKILL.md":                  "---\nname: legal\n---\nbody\n",
		"templates/project/workflows/audit.js":                     "export default {};\n",
		"templates/project/codex/config.toml":                      "model = \"{TOKEN}\"\n",
		"templates/project/docs-commands/jc/references/jc-core.md": "# JC\n",
		"templates/project/docs-agents/_index.md":                  "# Agents\n",
	} {
		writeProjectFixtureFile(t, root, relative, content)
	}
	gitTemp(t, root, "init", "-q")
	gitTemp(t, root, "config", "user.email", "fixture.invalid")
	gitTemp(t, root, "config", "user.name", "fixture-identity")
	gitTemp(t, root, "add", ".")
	gitTemp(t, root, "commit", "-qm", "old templates")
	oldSHA = projectGitShortSHA(t, root)

	writeProjectFixtureFile(t, root, "templates/project/commands/dev.md", "new\n")
	writeProjectFixtureFile(t, root, "templates/project/commands/later.md", "later\n")
	gitTemp(t, root, "add", "-A")
	gitTemp(t, root, "commit", "-qm", "new templates")
	headSHA = projectGitShortSHA(t, root)
	return root, oldSHA, headSHA
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
