package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"hostops/cc-fleet/internal/store"
)

func TestVersion(t *testing.T) {
	jailTest(t)

	var stdout, stderr bytes.Buffer
	if code := run([]string{"version"}, &stdout, &stderr); code != 0 {
		t.Fatalf("run(version) code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if got, want := stdout.String(), "cc-fleet dev\n"; got != want {
		t.Fatalf("run(version) stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("run(version) stderr = %q, want empty", stderr.String())
	}
}

func TestUnknownCommand(t *testing.T) {
	jailTest(t)

	var stdout, stderr bytes.Buffer
	if code := run([]string{"no-such-command"}, &stdout, &stderr); code != 2 {
		t.Fatalf("run(unknown) code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("run(unknown) stdout = %q, want empty", stdout.String())
	}
	if got := stderr.String(); !strings.Contains(got, `unknown command "no-such-command"`) {
		t.Fatalf("run(unknown) stderr = %q, want unknown-command message", got)
	}
}

func TestHideHiddenUnhideCLI(t *testing.T) {
	jailTest(t)
	const id = "99999999-9999-4999-8999-999999999999"
	database, err := store.Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := database.UpsertTranscript(
		context.Background(),
		store.Transcript{
			UUID:        id,
			Path:        "/jailed/" + id + ".jsonl",
			PromptCount: 12,
		},
	); err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if code := run([]string{"hide", id}, &stdout, &stderr); code != 0 {
		t.Fatalf("hide code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.String() != "hidden "+id+"\n" || stderr.Len() != 0 {
		t.Fatalf("hide stdout=%q stderr=%q", stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"hidden"}, &stdout, &stderr); code != 0 {
		t.Fatalf("hidden code=%d stderr=%q", code, stderr.String())
	}
	fields := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\t")
	if len(fields) != 3 || fields[0] != id || fields[1] != "cc" {
		t.Fatalf("hidden stdout=%q, want id/engine/hidden_at only", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"unhide", id}, &stdout, &stderr); code != 0 {
		t.Fatalf("unhide code=%d stderr=%q", code, stderr.String())
	}
	if stdout.String() != "unhidden "+id+"\n" {
		t.Fatalf("unhide stdout=%q", stdout.String())
	}
}

func TestHiddenPruneOrphansCLI(t *testing.T) {
	jailTest(t)
	const (
		live    = "11111111-1111-4111-8111-111111111111"
		orphan  = "22222222-2222-4222-8222-222222222222"
		orphan2 = "33333333-3333-4333-8333-333333333333"
	)
	database, err := store.Open()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := database.UpsertTranscript(ctx, store.Transcript{
		UUID: live,
		Path: "/jailed/" + live + ".jsonl",
	}); err != nil {
		t.Fatal(err)
	}
	for _, hidden := range []store.Hidden{
		{ID: live, Engine: "cc", HiddenAt: 10},
		{ID: orphan, Engine: "cc", HiddenAt: 20},
		{ID: orphan2, Engine: "cx", HiddenAt: 30},
	} {
		if err := database.Hide(ctx, hidden); err != nil {
			t.Fatal(err)
		}
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if code := run([]string{"hidden", "--prune-orphans"}, &stdout, &stderr); code != 0 {
		t.Fatalf("prune dry run code=%d stderr=%q", code, stderr.String())
	}
	// The engine column is empty for every orphan, and that IS the report: an
	// engine is derived from whichever index table claims the id, and an orphan
	// is precisely a hide no index table claims any more.
	dryRun := stdout.String()
	if !strings.Contains(dryRun, "would prune\t"+orphan+"\t\t20\n") ||
		!strings.Contains(dryRun, "would prune\t"+orphan2+"\t\t30\n") ||
		!strings.Contains(dryRun, "2 orphaned hide(s); re-run with --yes to delete\n") {
		t.Fatalf("prune dry run stdout=%q", dryRun)
	}
	if strings.Contains(dryRun, live) {
		t.Fatalf("prune dry run named the live hide: stdout=%q", dryRun)
	}

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"hidden"}, &stdout, &stderr); code != 0 {
		t.Fatalf("hidden after dry run code=%d stderr=%q", code, stderr.String())
	}
	if lines := strings.Count(stdout.String(), "\n"); lines != 3 {
		t.Fatalf("dry run deleted rows: hidden stdout=%q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"hidden", "--prune-orphans", "--yes"}, &stdout, &stderr); code != 0 {
		t.Fatalf("prune code=%d stderr=%q", code, stderr.String())
	}
	pruned := stdout.String()
	if !strings.Contains(pruned, "pruned\t"+orphan+"\t\t20\n") ||
		!strings.Contains(pruned, "pruned 2 orphaned hide(s)\n") {
		t.Fatalf("prune stdout=%q", pruned)
	}

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"hidden"}, &stdout, &stderr); code != 0 {
		t.Fatalf("hidden after prune code=%d stderr=%q", code, stderr.String())
	}
	if got := stdout.String(); !strings.HasPrefix(got, live+"\tcc\t10\n") ||
		strings.Count(got, "\n") != 1 {
		t.Fatalf("hidden after prune stdout=%q, want only the live hide", got)
	}

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"hidden", "--yes"}, &stdout, &stderr); code != 2 {
		t.Fatalf("hidden --yes without --prune-orphans code=%d, want 2", code)
	}
}

func TestHideSelfResolveAndInternalCLI(t *testing.T) {
	jailTest(t)
	const id = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	database, err := store.Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := database.UpsertTranscript(
		context.Background(),
		store.Transcript{
			UUID: id,
			Path: "/jailed/" + id + ".jsonl",
		},
	); err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMUX", filepath.Join(t.TempDir(), "cc-1-1-1")+",1,0")
	t.Setenv("TMUX_PANE", "%1")
	t.Setenv("CLAUDE_CODE_SESSION_ID", id)

	var stdout, stderr bytes.Buffer
	if code := run([]string{"hide", "--self"}, &stdout, &stderr); code != 0 {
		t.Fatalf("hide --self code=%d stderr=%q", code, stderr.String())
	}
	if stdout.String() != "hidden "+id+"\n" {
		t.Fatalf("hide --self stdout=%q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := run(
		[]string{"resolve", "label", "missing"},
		&stdout,
		&stderr,
	); code != 1 {
		t.Fatalf(
			"resolve miss code=%d stdout=%q stderr=%q",
			code,
			stdout.String(),
			stderr.String(),
		)
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("resolve miss stdout=%q stderr=%q", stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code := run([]string{
		"internal",
		"hide-exit",
		"--engine", "invalid",
		"--id", id,
		"--path", "/jailed/transcript.jsonl",
		"--socket", "/jailed/socket",
		"--socket-name", "cc-1-1-1",
		"--pane", "%1",
	}, &stdout, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "unknown hide-exit engine") {
		t.Fatalf("internal code=%d stderr=%q", code, stderr.String())
	}
}

func TestWiredIndexListOpenReviveAndDoctor(t *testing.T) {
	root := jailTest(t)
	t.Setenv(codexAvailableEnv, "0")
	t.Setenv(testFreshSocketEnv, "cc-1700000000-1-1")
	project := filepath.Join(root, "work", "project")
	transcriptDir := filepath.Join(root, "claude", "project")
	if err := os.MkdirAll(project, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(transcriptDir, 0o700); err != nil {
		t.Fatal(err)
	}
	const id = "11111111-1111-4111-8111-111111111111"
	content := `{"type":"user","cwd":` +
		strconv.Quote(project) +
		`,"message":{"content":"Wired prompt"}}` + "\n"
	if err := os.WriteFile(
		filepath.Join(transcriptDir, id+".jsonl"),
		[]byte(content),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if code := run([]string{"index", "--full"}, &stdout, &stderr); code != 0 {
		t.Fatalf("index code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "files=1") ||
		!strings.Contains(stdout.String(), "full=1") ||
		!strings.Contains(stdout.String(), "touched=3") {
		t.Fatalf("index stdout=%q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"ls", "--tsv"}, &stdout, &stderr); code != 0 {
		t.Fatalf("ls code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "\t"+id+"\t") ||
		!strings.Contains(stdout.String(), "resume-claude") {
		t.Fatalf("ls stdout=%q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"open", id}, &stdout, &stderr); code != 0 {
		t.Fatalf("open code=%d stderr=%q", code, stderr.String())
	}
	if lines := strings.Count(stdout.String(), "\n"); lines != 1 {
		t.Fatalf("open emitted %d lines: %q", lines, stdout.String())
	}
	if !strings.Contains(stdout.String(), "--resume") ||
		!strings.Contains(stdout.String(), id) ||
		!strings.Contains(stdout.String(), "cc-1700000000-1-1") {
		t.Fatalf("open stdout=%q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"revive"}, &stdout, &stderr); code != 0 {
		t.Fatalf("revive code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Wired prompt") {
		t.Fatalf("revive stdout=%q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"doctor"}, &stdout, &stderr); code != 0 {
		t.Fatalf("doctor code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "doctor: clean") ||
		!strings.Contains(stdout.String(), "transcripts=1") {
		t.Fatalf("doctor stdout=%q", stdout.String())
	}
}

// TestCheckRefusesALiveCodexSocketMissingFromTheGoRows is the regression this
// checker existed to catch and did not. A legacy-only live-codex row means the
// Go side lost a live Codex chat — which is exactly what happened when the pane
// probe handed the thread resolver a BASENAME instead of a directory. The old
// legacy-dead-server class restated that symptom as its own justification and
// absolved it; there is no class for this shape any more, and there must not be.
func TestCheckRefusesALiveCodexSocketMissingFromTheGoRows(t *testing.T) {
	root := jailTest(t)
	legacy := filepath.Join(root, "legacy-live-codex.txt")
	const socket = "cx-1785120110-1697775-48706"
	if err := os.WriteFile(
		legacy,
		[]byte("__CC_FLEET_TUPLE__\t"+socket+
			"\tlive-codex\thost-ops\t0\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	// The SHIPPED allowlist, not a stand-in: the assertion is that no class this
	// tree carries can cover a lost live Codex row.
	allowlist := filepath.Join(root, "allowlist.txt")
	content, err := os.ReadFile(
		filepath.Join("..", "..", "testdata", checkAllowlistFile),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(allowlist, content, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CC_FLEET_CHECK_LEGACY_OUTPUT", legacy)
	t.Setenv(checkAllowlistEnv, allowlist)

	var stdout, stderr bytes.Buffer
	code := run([]string{"ls", "--check"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf(
			"a live Codex socket missing from the Go rows was allowlisted: %q",
			stdout.String(),
		)
	}
	if !strings.Contains(stdout.String(), "- legacy-only\t"+socket+"\tlive-codex") {
		t.Fatalf("missing live Codex row was not reported: %q", stdout.String())
	}
	for _, line := range strings.Split(stdout.String(), "\n") {
		if strings.HasPrefix(line, "~ allowed[") && strings.Contains(line, socket) {
			t.Fatalf("a class absolved the lost live Codex row: %q", line)
		}
	}
}

// TestCheckSpendsNoSquatterBudgetWithoutALiveProbe holds the other half of the
// squatter class's bound end to end: the budget comes from the live tmux probe,
// so in a jail with no server on that socket the class has nothing to spend and
// the row stays visible. The class is a count, never a shape.
func TestCheckSpendsNoSquatterBudgetWithoutALiveProbe(t *testing.T) {
	root := jailTest(t)
	legacy := filepath.Join(root, "legacy-parked.txt")
	const socket = "cc-1785850688-1928450-3031"
	if err := os.WriteFile(
		legacy,
		[]byte("__CC_FLEET_TUPLE__\t"+socket+"\tlive-claude\tprojb-be\t0\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	allowlist := filepath.Join(root, "allowlist.txt")
	if err := os.WriteFile(
		allowlist,
		[]byte("class\tlegacy-socket-squatter\tparked work, not a chat\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CC_FLEET_CHECK_LEGACY_OUTPUT", legacy)
	t.Setenv(checkAllowlistEnv, allowlist)

	var stdout, stderr bytes.Buffer
	code := run([]string{"ls", "--check"}, &stdout, &stderr)
	// The jail has no tmux server on that socket, so the probe counts no parked
	// sessions and the class has no budget to spend: the row stays visible.
	if code == 0 {
		t.Fatalf(
			"squatter class spent a budget the probe never granted: %q",
			stdout.String(),
		)
	}
	if !strings.Contains(stdout.String(), "- legacy-only\t"+socket) {
		t.Fatalf("unbudgeted squatter row was not reported: %q", stdout.String())
	}
}

func TestDoctorReportsDamagedDatabaseWithoutPanic(t *testing.T) {
	root := jailTest(t)
	dbPath := filepath.Join(root, "fleet.db")
	if err := os.WriteFile(dbPath, []byte("not a sqlite database"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"doctor"}, &stdout, &stderr); code != 1 {
		t.Fatalf("doctor code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.HasPrefix(stdout.String(), "doctor: unhealthy database:") {
		t.Fatalf("doctor stdout=%q", stdout.String())
	}
}

func TestDoctorRecognizesThenFailedAsSatelliteMetadata(t *testing.T) {
	root := jailTest(t)
	sidDir := filepath.Join(root, "sid")
	for name, content := range map[string]string{
		"cc-1-2-3":              "/transcripts/live.jsonl",
		"cc-1-2-3.then-failed":  "prompt preserved for retry",
		".open.uuid":            "lock metadata",
		"cc-1-2-3.not-metadata": "invalid",
	} {
		if err := os.WriteFile(
			filepath.Join(sidDir, name),
			[]byte(content),
			0o600,
		); err != nil {
			t.Fatal(err)
		}
	}
	entries, invalid, err := crumbHealth(sidDir)
	if err != nil {
		t.Fatal(err)
	}
	if entries != 4 || invalid != 1 {
		t.Fatalf("crumbHealth() entries=%d invalid=%d", entries, invalid)
	}
}

// The live sid directory holds two classes doctor must not call rot: crumbs
// the statusline writes for chats on servers the fleet excludes (the vsct
// bunker, the revive dashboards) and the dot-prefixed lock DIRECTORIES the
// zsh creates with mkdir to serialize opens.
func TestDoctorIgnoresBunkerCrumbsAndOpenLockDirectories(t *testing.T) {
	root := jailTest(t)
	sidDir := filepath.Join(root, "sid")
	for name, content := range map[string]string{
		"cc-1-2-3":    "/transcripts/live.jsonl",
		"vsct":        "/transcripts/bunker.jsonl",
		"vsct.%187":   "/transcripts/bunker.jsonl",
		"revive-vsct": "/transcripts/revive.jsonl",
		"rotten":      "neither a crumb nor sid metadata",
	} {
		if err := os.WriteFile(
			filepath.Join(sidDir, name),
			[]byte(content),
			0o600,
		); err != nil {
			t.Fatal(err)
		}
	}
	for _, directory := range []string{
		".open.88888888-8888-4888-8888-888888888888",
		"rotten-directory",
	} {
		if err := os.Mkdir(filepath.Join(sidDir, directory), 0o700); err != nil {
			t.Fatal(err)
		}
	}

	entries, invalid, err := crumbHealth(sidDir)
	if err != nil {
		t.Fatal(err)
	}
	if entries != 7 || invalid != 2 {
		t.Fatalf(
			"crumbHealth() entries=%d invalid=%d, want 7 and 2 (rotten + rotten-directory)",
			entries,
			invalid,
		)
	}
}

func TestUsageErrors(t *testing.T) {
	jailTest(t)
	for _, args := range [][]string{
		{"ls", "--plain", "--tsv"},
		{"ls", "--check", "--all"},
		{"ls", "--check", "id"},
		{"open"},
		{"index", "--unknown"},
		{"doctor", "extra"},
		{"revive", "extra"},
		{"legacy", "unknown"},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 2 {
			t.Fatalf("run(%q) code=%d stdout=%q stderr=%q", args, code, stdout.String(), stderr.String())
		}
		if stderr.Len() == 0 {
			t.Fatalf("run(%q) emitted no usage/error", args)
		}
	}
}

func TestActionDispatchPipeGoldenAndTTYExec(t *testing.T) {
	originalTerminal := actionOutputIsTerminal
	originalLookPath := actionLookPath
	originalExec := actionExec
	t.Cleanup(func() {
		actionOutputIsTerminal = originalTerminal
		actionLookPath = originalLookPath
		actionExec = originalExec
	})

	const attachLine = "TMUX= tmux -L 'cc-1-2-3' attach -t 'live-session'"
	var stdout bytes.Buffer
	actionOutputIsTerminal = func(io.Writer) bool { return false }
	if err := dispatchAction(&stdout, attachLine); err != nil {
		t.Fatal(err)
	}
	if got, want := stdout.String(), attachLine+"\n"; got != want {
		t.Fatalf("pipe action = %q, want byte-identical %q", got, want)
	}

	type execCall struct {
		path string
		args []string
		env  []string
	}
	var calls []execCall
	execReturned := errors.New("exec test return")
	actionOutputIsTerminal = func(io.Writer) bool { return true }
	actionLookPath = func(file string) (string, error) {
		return "/jail/bin/" + file, nil
	}
	actionExec = func(path string, args, env []string) error {
		calls = append(calls, execCall{
			path: path,
			args: append([]string(nil), args...),
			env:  append([]string(nil), env...),
		})
		return execReturned
	}

	t.Setenv("TMUX", "/tmp/driver,1,0")
	stdout.Reset()
	if err := dispatchAction(&stdout, attachLine); !errors.Is(err, execReturned) {
		t.Fatalf("terminal attach error = %v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("terminal attach printed %q", stdout.String())
	}
	if len(calls) != 1 ||
		calls[0].path != "/jail/bin/tmux" ||
		!reflect.DeepEqual(
			calls[0].args,
			[]string{
				"tmux", "-L", "cc-1-2-3",
				"attach", "-t", "live-session",
			},
		) ||
		environmentValue(calls[0].env, "TMUX") != "" {
		t.Fatalf("terminal tmux exec = %#v", calls)
	}

	const shellLine = "(cd -- '/work/project' && CC_ARM_1H=0 cc1)"
	if err := dispatchAction(&stdout, shellLine); !errors.Is(err, execReturned) {
		t.Fatalf("terminal shell error = %v", err)
	}
	if len(calls) != 2 ||
		calls[1].path != "/jail/bin/zsh" ||
		!reflect.DeepEqual(
			calls[1].args,
			[]string{"zsh", "-ic", shellLine},
		) {
		t.Fatalf("terminal zsh exec = %#v", calls)
	}
}

func TestDirectTmuxArgumentsPreserveQuotedData(t *testing.T) {
	line := `TMUX= exec tmux -L 'cc-1'"'"'quoted' new-session -c '/work/a b' 'printf "$HOME;*"!'`
	got, direct, err := directTmuxArguments(line)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"-L", "cc-1'quoted",
		"new-session", "-c", "/work/a b",
		`printf "$HOME;*"!`,
	}
	if !direct || !reflect.DeepEqual(got, want) {
		t.Fatalf("directTmuxArguments() = %q, %v; want %q, true", got, direct, want)
	}
}

func environmentValue(environment []string, key string) string {
	prefix := key + "="
	for _, entry := range environment {
		if strings.HasPrefix(entry, prefix) {
			return strings.TrimPrefix(entry, prefix)
		}
	}
	return "\x00missing"
}

func TestLSCheckParityMutationAndReadOnly(t *testing.T) {
	root := jailTest(t)
	t.Setenv(codexAvailableEnv, "0")
	project := filepath.Join(root, "work", "check-project")
	transcriptDir := filepath.Join(root, "claude", "check-project")
	if err := os.MkdirAll(project, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(transcriptDir, 0o700); err != nil {
		t.Fatal(err)
	}
	const id = "12121212-1212-4212-8212-121212121212"
	content := `{"type":"user","cwd":` + strconv.Quote(project) +
		`,"message":{"content":"Shadow parity"}}` + "\n"
	if err := os.WriteFile(
		filepath.Join(transcriptDir, id+".jsonl"),
		[]byte(content),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"index", "--full"}, &stdout, &stderr); code != 0 {
		t.Fatalf("index code=%d stderr=%q", code, stderr.String())
	}
	fixture := filepath.Join(root, "legacy.txt")
	allowlist := filepath.Join(root, "allowlist.txt")
	if err := os.WriteFile(
		fixture,
		[]byte("__CC_FLEET_TUPLE__\t"+id+
			"\tresume-claude\tcheck-project\t1\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(allowlist, []byte("# no deviations\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CC_FLEET_CHECK_LEGACY_OUTPUT", fixture)
	t.Setenv(checkAllowlistEnv, allowlist)
	before := hashJailOutsideDB(t, root)
	for iteration := 0; iteration < 20; iteration++ {
		stdout.Reset()
		stderr.Reset()
		if code := run([]string{"ls", "--check"}, &stdout, &stderr); code != 0 {
			t.Fatalf(
				"check iteration %d code=%d stdout=%q stderr=%q",
				iteration,
				code,
				stdout.String(),
				stderr.String(),
			)
		}
		if !strings.Contains(stdout.String(), "cc-fleet check: parity (1 tuples") {
			t.Fatalf("check stdout=%q", stdout.String())
		}
	}
	after := hashJailOutsideDB(t, root)
	if before != after {
		t.Fatalf("ls --check mutated jail outside database: before=%s after=%s", before, after)
	}
	t.Logf(
		"STRESS check_no_mutation iterations=20 identical=true tree_hash=%s",
		after,
	)

	if err := os.WriteFile(
		fixture,
		[]byte("__CC_FLEET_TUPLE__\t"+id+
			"\tresume-claude\tcheck-project\t2\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"ls", "--check"}, &stdout, &stderr); code != 1 {
		t.Fatalf("mutated check code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if strings.Count(stdout.String(), "\n") != 3 ||
		!strings.Contains(stdout.String(), "2 difference(s)") {
		t.Fatalf("mutated check stdout=%q", stdout.String())
	}

	if err := os.WriteFile(
		allowlist,
		[]byte("either\t"+id+
			"\tresume-claude\tcheck-project\tpartial-tail prompt count\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"ls", "--check"}, &stdout, &stderr); code != 0 {
		t.Fatalf("allowlisted check code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if strings.Count(stdout.String(), "~ allowed") != 2 ||
		!strings.Contains(stdout.String(), "partial-tail prompt count") {
		t.Fatalf("allowlisted check stdout=%q", stdout.String())
	}
}

func hashJailOutsideDB(t *testing.T, root string) string {
	t.Helper()
	digest := sha256.New()
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		if strings.HasPrefix(filepath.Base(path), "fleet.db") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if _, err := io.WriteString(
			digest,
			relative+"\x00"+info.Mode().String()+"\x00",
		); err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			_, err = io.WriteString(digest, target)
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = digest.Write(content)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(digest.Sum(nil))
}

func jailTest(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	for _, directory := range []string{
		filepath.Join(root, "t"),
		filepath.Join(root, "sid"),
		filepath.Join(root, "claude"),
		filepath.Join(root, "codex"),
		filepath.Join(root, "tmux"),
		filepath.Join(root, "home"),
		filepath.Join(root, "proc"),
	} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("TMUX_TMPDIR", filepath.Join(root, "t"))
	t.Setenv("CC_FLEET_DB", filepath.Join(root, "fleet.db"))
	t.Setenv("CC_FLEET_SID_DIR", filepath.Join(root, "sid"))
	t.Setenv("CC_FLEET_CLAUDE_ROOTS", filepath.Join(root, "claude"))
	t.Setenv("CC_FLEET_CODEX_ROOT", filepath.Join(root, "codex"))
	t.Setenv("CC_FLEET_TMUX_DIR", filepath.Join(root, "tmux"))
	t.Setenv("CC_FLEET_HOME", filepath.Join(root, "home"))
	t.Setenv("CC_FLEET_PROC_ROOT", filepath.Join(root, "proc"))
	// A chat server loads the user's ~/.tmux.conf in real life; a fixture must
	// not, or the machine it runs on steers the test.
	t.Setenv("CC_FLEET_TMUX_CONF", "/dev/null")
	return root
}
