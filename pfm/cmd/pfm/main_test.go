package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"hostops/pfm/internal/store"
	"hostops/pfm/internal/testjail"
)

func TestVersion(t *testing.T) {
	jailTest(t)

	var stdout, stderr bytes.Buffer
	if code := run([]string{"version"}, &stdout, &stderr); code != 0 {
		t.Fatalf("run(version) code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if got, want := stdout.String(), "pfm dev\n"; got != want {
		t.Fatalf("run(version) stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("run(version) stderr = %q, want empty", stderr.String())
	}
}

func TestSettledRootInterface(t *testing.T) {
	jailTest(t)

	var stdout, stderr bytes.Buffer
	if code := run([]string{"--version"}, &stdout, &stderr); code != 0 {
		t.Fatalf("run(--version) code = %d; stderr = %q", code, stderr.String())
	}
	if stdout.String() != "pfm dev\n" {
		t.Fatalf("run(--version) stdout = %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"--help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("run(--help) code = %d; stderr = %q", code, stderr.String())
	}
	help := stdout.String()
	for _, want := range []string{
		"operator commands:", "  chat", "  install", "  update", "  init", "wiring commands:",
		"  statusline", "  usage-hook",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("root help missing %q:\n%s", want, help)
		}
	}
	for _, retired := range []string{"  open ", "  headless ", "  kill ", "  unkill ", "  killed ", "  resolve ", "  bb ", "chat bb"} {
		if strings.Contains(help, retired) {
			t.Fatalf("root help still advertises %q:\n%s", retired, help)
		}
	}
}

func TestChatShimWhoamiCompatibilityRoute(t *testing.T) {
	jailTest(t)
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	t.Setenv("CODEX_THREAD_ID", "")

	var stdout, stderr bytes.Buffer
	code := runChat([]string{"whoami"}, strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("chat whoami code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if strings.Contains(stderr.String(), "unknown command") ||
		!strings.Contains(stderr.String(), "not inside tmux") {
		t.Fatalf("chat whoami did not reach the root identity command: %q", stderr.String())
	}
}

func TestLSKilledAbsorbsTheOldKilledListing(t *testing.T) {
	jailTest(t)
	const id = "88888888-8888-4888-8888-888888888888"
	database, err := store.Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Kill(context.Background(), store.Killed{
		ID: id, Engine: "cc", KilledAt: 42,
	}); err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{
		{"ls", "--killed"},
		{"ls", "--killed", "--tsv"},
		{"ls", "--tsv", "--killed"},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 0 {
			t.Fatalf("%v code=%d stderr=%q", args, code, stderr.String())
		}
		if stdout.String() != id+"\t\t42\n" {
			t.Fatalf("%v stdout=%q", args, stdout.String())
		}
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

func TestKillKilledUnkillCLI(t *testing.T) {
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
	if code := run([]string{"chat", "kill", id}, &stdout, &stderr); code != 0 {
		t.Fatalf("kill code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.String() != "killed "+id+"\n" || stderr.Len() != 0 {
		t.Fatalf("kill stdout=%q stderr=%q", stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"ls", "--killed"}, &stdout, &stderr); code != 0 {
		t.Fatalf("killed code=%d stderr=%q", code, stderr.String())
	}
	fields := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\t")
	if len(fields) != 3 || fields[0] != id || fields[1] != "cc" {
		t.Fatalf("killed stdout=%q, want id/engine/killed_at only", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"chat", "unkill", id}, &stdout, &stderr); code != 0 {
		t.Fatalf("unkill code=%d stderr=%q", code, stderr.String())
	}
	if stdout.String() != "unkilled "+id+"\n" {
		t.Fatalf("unkill stdout=%q", stdout.String())
	}
}

func TestKilledPruneOrphansCLI(t *testing.T) {
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
	for _, killed := range []store.Killed{
		{ID: live, Engine: "cc", KilledAt: 10},
		{ID: orphan, Engine: "cc", KilledAt: 20},
		{ID: orphan2, Engine: "cx", KilledAt: 30},
	} {
		if err := database.Kill(ctx, killed); err != nil {
			t.Fatal(err)
		}
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if code := run([]string{"archive", "--prune-orphans"}, &stdout, &stderr); code != 0 {
		t.Fatalf("prune dry run code=%d stderr=%q", code, stderr.String())
	}
	// The engine column is empty for every orphan, and that IS the report: an
	// engine is derived from whichever index table claims the id, and an orphan
	// is precisely a kill no index table claims any more.
	dryRun := stdout.String()
	if !strings.Contains(dryRun, "would prune\t"+orphan+"\t\t20\n") ||
		!strings.Contains(dryRun, "would prune\t"+orphan2+"\t\t30\n") ||
		!strings.Contains(dryRun, "2 orphaned kill(s); re-run with --yes to delete\n") {
		t.Fatalf("prune dry run stdout=%q", dryRun)
	}
	if strings.Contains(dryRun, live) {
		t.Fatalf("prune dry run named the live kill: stdout=%q", dryRun)
	}

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"ls", "--killed"}, &stdout, &stderr); code != 0 {
		t.Fatalf("killed after dry run code=%d stderr=%q", code, stderr.String())
	}
	if lines := strings.Count(stdout.String(), "\n"); lines != 3 {
		t.Fatalf("dry run deleted rows: killed stdout=%q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"archive", "--prune-orphans", "--yes"}, &stdout, &stderr); code != 0 {
		t.Fatalf("prune code=%d stderr=%q", code, stderr.String())
	}
	pruned := stdout.String()
	if !strings.Contains(pruned, "pruned\t"+orphan+"\t\t20\n") ||
		!strings.Contains(pruned, "pruned 2 orphaned kill(s)\n") {
		t.Fatalf("prune stdout=%q", pruned)
	}

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"ls", "--killed"}, &stdout, &stderr); code != 0 {
		t.Fatalf("killed after prune code=%d stderr=%q", code, stderr.String())
	}
	if got := stdout.String(); !strings.HasPrefix(got, live+"\tcc\t10\n") ||
		strings.Count(got, "\n") != 1 {
		t.Fatalf("killed after prune stdout=%q, want only the live kill", got)
	}

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"ls", "--killed", "--yes"}, &stdout, &stderr); code != 2 {
		t.Fatalf("killed --yes without --prune-orphans code=%d, want 2", code)
	}
}

func TestKillSelfResolveAndInternalCLI(t *testing.T) {
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
	if code := run([]string{"chat", "kill", "self"}, &stdout, &stderr); code != 0 {
		t.Fatalf("kill --self code=%d stderr=%q", code, stderr.String())
	}
	if stdout.String() != "killed "+id+"\n" {
		t.Fatalf("kill --self stdout=%q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := run(
		[]string{"chat", "resolve", "missing"},
		&stdout,
		&stderr,
	); code != codeUnknownChat {
		t.Fatalf(
			"resolve miss code=%d stdout=%q stderr=%q",
			code,
			stdout.String(),
			stderr.String(),
		)
	}
	if stdout.String() != "missing\tnot-found\n" ||
		!strings.Contains(stderr.String(), "no chat named") {
		t.Fatalf("resolve miss stdout=%q stderr=%q", stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code := run([]string{
		"internal",
		"kill-exit",
		"--engine", "invalid",
		"--id", id,
		"--path", "/jailed/transcript.jsonl",
		"--socket", "/jailed/socket",
		"--socket-name", "cc-1-1-1",
		"--pane", "%1",
	}, &stdout, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), `unknown engine "invalid"`) {
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
		!strings.Contains(stdout.String(), "touched=2") {
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
	if code := run([]string{"chat", "open", id}, &stdout, &stderr); code != 0 {
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
	if !strings.Contains(stdout.String(), "doctor: config path=") ||
		!strings.Contains(stdout.String(), "doctor: unhealthy database:") {
		t.Fatalf("doctor stdout=%q", stdout.String())
	}
}

// TestDoctorNamesAnExistingButUnwiredPrePushGate is the issue-6 regression:
// the repository shipped the hook but no diagnostic distinguished "armed"
// from "file exists and Git will never execute it".
func TestDoctorNamesAnExistingButUnwiredPrePushGate(t *testing.T) {
	root := jailTest(t)
	repository := filepath.Join(root, "repository")
	if err := os.MkdirAll(filepath.Join(repository, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repository, ".githooks"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(repository, ".git", "config"),
		[]byte("[core]\n\trepositoryformatversion = 0\n\tbare = false\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(repository, ".git", "HEAD"),
		[]byte("ref: refs/heads/main\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{"objects", filepath.Join("refs", "heads")} {
		if err := os.MkdirAll(filepath.Join(repository, ".git", directory), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(
		filepath.Join(repository, ".githooks", "pre-push"),
		[]byte("#!/bin/sh\nexit 0\n"),
		0o700,
	); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repository)

	var stdout, stderr bytes.Buffer
	if code := run([]string{"doctor"}, &stdout, &stderr); code != 1 {
		t.Fatalf("doctor code=%d, want warning exit\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
	}
	want := "doctor: pre-push gate=UNWIRED expected=.githooks actual=(unset)"
	if !strings.Contains(stdout.String(), want) {
		t.Fatalf("doctor omitted %q:\n%s", want, stdout.String())
	}

	if err := os.WriteFile(
		filepath.Join(repository, ".git", "config"),
		[]byte("[core]\n\trepositoryformatversion = 0\n\tbare = false\n\thooksPath = .githooks\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"doctor"}, &stdout, &stderr); code != 0 {
		t.Fatalf("armed doctor code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if want := "doctor: pre-push gate=armed core.hooksPath=.githooks"; !strings.Contains(stdout.String(), want) {
		t.Fatalf("armed doctor omitted %q:\n%s", want, stdout.String())
	}

	if err := os.Chmod(filepath.Join(repository, ".githooks", "pre-push"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"doctor"}, &stdout, &stderr); code != 1 {
		t.Fatalf("broken-hook doctor code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if want := "doctor: pre-push gate=BROKEN core.hooksPath=.githooks"; !strings.Contains(stdout.String(), want) {
		t.Fatalf("broken-hook doctor omitted %q:\n%s", want, stdout.String())
	}
}

func TestDoctorRecognizesThenFailedAsSatelliteMetadata(t *testing.T) {
	root := jailTest(t)
	sidDir := filepath.Join(root, "sid")
	for name, content := range map[string]string{
		"cc-1-2-3":              "/transcripts/live.jsonl",
		"cc-1-2-3.then-failed":  "prompt preserved for retry",
		"reload-cc-1-2-3.log":   "completed fleet reload",
		"reload-vsct.log":       "completed bunker reload",
		"reload-probe.log":      "not a fleet reload",
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
	if entries != 7 || invalid != 2 {
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
		{"ls", "--killed", "--all"},
		{"chat", "open"},
		{"index", "--unknown"},
		{"doctor", "extra"},
		{"revive", "extra"},
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

func jailTest(t *testing.T) string {
	t.Helper()

	root := testjail.ShortRoot(t)
	for _, directory := range []string{
		filepath.Join(root, "t"),
		filepath.Join(root, "sid"),
		filepath.Join(root, "claude"),
		filepath.Join(root, "codex"),
		filepath.Join(root, "tmux"),
		filepath.Join(root, "home"),
		filepath.Join(root, "home", ".local", "bin"),
		filepath.Join(root, "proc"),
	} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("TMUX_TMPDIR", filepath.Join(root, "t"))
	t.Setenv("PFM_DB", filepath.Join(root, "fleet.db"))
	t.Setenv("PFM_SID_DIR", filepath.Join(root, "sid"))
	t.Setenv("PFM_CLAUDE_ROOTS", filepath.Join(root, "claude"))
	t.Setenv("PFM_CODEX_ROOT", filepath.Join(root, "codex"))
	t.Setenv("PFM_TMUX_DIR", filepath.Join(root, "tmux"))
	jailedHome := filepath.Join(root, "home")
	t.Setenv("PFM_HOME", jailedHome)
	// HOME is the same concept under its other name. Pinning only
	// PFM_HOME leaves anything reading the plain variable — the test
	// itself, a subprocess, a library — writing into the operator's real
	// account, which is how fixture transcripts reached a live
	// ~/.claude/projects. The two must never be allowed to disagree.
	t.Setenv("HOME", jailedHome)
	canonical := filepath.Join(root, "home", ".local", "bin", "pfm")
	if err := os.WriteFile(canonical, []byte("jailed-pfm"), 0o700); err != nil {
		t.Fatal(err)
	}
	managedClaude := filepath.Join(root, "home", ".local", "share", "pfm", "install", "bin", "claude")
	if err := os.MkdirAll(filepath.Dir(managedClaude), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(managedClaude, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(managedClaude, filepath.Join(root, "home", ".local", "bin", "claude")); err != nil {
		t.Fatal(err)
	}
	testPath := []string{filepath.Dir(canonical)}
	for _, directory := range filepath.SplitList(os.Getenv("PATH")) {
		if _, err := os.Stat(filepath.Join(directory, "pfm")); os.IsNotExist(err) {
			testPath = append(testPath, directory)
		}
	}
	t.Setenv("PATH", strings.Join(testPath, string(os.PathListSeparator)))
	t.Setenv("PFM_PROC_ROOT", filepath.Join(root, "proc"))
	// A chat server loads the user's ~/.tmux.conf in real life; a fixture must
	// not, or the machine it runs on steers the test.
	t.Setenv("PFM_TMUX_CONF", "/dev/null")
	return root
}

func writeJailedCodexAuth(t *testing.T, root string) {
	t.Helper()
	if err := os.WriteFile(
		filepath.Join(root, "codex", "auth.json"),
		[]byte(`{"tokens":{"access_token":"fixture","account_id":"fixture"}}`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
}
