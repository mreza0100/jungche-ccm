package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pfmengine "hostops/pfm/internal/engine"
	"hostops/pfm/internal/paths"
	"hostops/pfm/internal/store"
)

func TestChatSaveUsesConfiguredImplicitAccountRoot(t *testing.T) {
	root := t.TempDir()
	projects := filepath.Join(root, "account", "projects")
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	slug := strings.NewReplacer("/", "-", ".", "-").Replace(cwd)
	const id = "40404040-4040-4040-8040-404040404040"
	transcriptPath := filepath.Join(projects, slug, id+".jsonl")
	if err := os.MkdirAll(filepath.Dir(transcriptPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(transcriptPath, []byte(`{"type":"user","message":{"content":"configured transcript"}}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLAUDE_CODE_SESSION_ID", id)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	target := filepath.Join(root, "saved.md")
	runtime := commandRuntime{Paths: paths.Values{Home: filepath.Join(root, "home"), Roots: map[pfmengine.ID][]string{pfmengine.Claude: {projects}}}}
	var stdout, stderr bytes.Buffer
	if code := runChatSave([]string{target}, &stdout, &stderr, runtime); code != 0 {
		t.Fatalf("save code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	saved, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(saved), "Source: "+transcriptPath) {
		t.Fatalf("saved transcript source=%q, want %q", string(saved), transcriptPath)
	}
}

func TestCurrentClaudeModelUsesConfiguredAccountRoot(t *testing.T) {
	root := t.TempDir()
	projects := filepath.Join(root, "account", "projects")
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	slug := strings.NewReplacer("/", "-", ".", "-").Replace(cwd)
	const id = "50505050-5050-4050-8050-505050505050"
	transcriptPath := filepath.Join(projects, slug, id+".jsonl")
	if err := os.MkdirAll(filepath.Dir(transcriptPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(transcriptPath, []byte(`{"type":"assistant","message":{"model":"claude-opus-5"}}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	got := currentClaudeModel(id, commandRuntime{Paths: paths.Values{
		Home:  filepath.Join(root, "home"),
		Roots: map[pfmengine.ID][]string{pfmengine.Claude: {projects}},
	}})
	if got != "opus[1m]" {
		t.Fatalf("currentClaudeModel()=%q, want opus[1m]", got)
	}
}

func TestChatLSUsesConfiguredAccountRoots(t *testing.T) {
	root := jailTest(t)
	accountRoot := filepath.Join(root, "configured-account")
	projects := filepath.Join(accountRoot, "projects")
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	slug := strings.NewReplacer("/", "-", ".", "-").Replace(cwd)
	const id = "60606060-6060-4060-8060-606060606060"
	transcriptPath := filepath.Join(projects, slug, id+".jsonl")
	if err := os.MkdirAll(filepath.Dir(transcriptPath), 0o700); err != nil {
		t.Fatal(err)
	}
	line := `{"type":"user","cwd":"` + cwd + `","message":{"content":"configured chat ls transcript"}}` + "\n"
	if err := os.WriteFile(transcriptPath, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := writeConfigFixture(t, root, `{"version":1,"accounts":[{"id":9,"configDir":"`+accountRoot+`"}]}`)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--config", configPath, "chat", "ls", "--all"}, &stdout, &stderr); code != 0 {
		t.Fatalf("chat ls code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	database, err := store.Open(store.WithWarningWriter(&bytes.Buffer{}))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	row, found, err := database.Transcript(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if !found || row.Path != transcriptPath {
		t.Fatalf("indexed transcript found=%t path=%q, want found at %q; stdout=%q stderr=%q", found, row.Path, transcriptPath, stdout.String(), stderr.String())
	}
}

func TestChatHistoryPassesConfiguredRootsToChild(t *testing.T) {
	root := t.TempDir()
	chatDir := filepath.Join(root, "chat")
	if err := os.MkdirAll(chatDir, 0o700); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(chatDir, "history.sh")
	if err := os.WriteFile(script, []byte("#!/usr/bin/env bash\nprintf '%s' \"$PFM_HISTORY_ROOTS_JSON\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PFM_HOME", root)
	runtime := commandRuntime{Paths: paths.Values{Roots: map[pfmengine.ID][]string{pfmengine.Claude: {filepath.Join(root, "account", "projects")}}}}
	var stdout, stderr bytes.Buffer
	if code := runChatSatellite("history", []string{"session"}, strings.NewReader(""), &stdout, &stderr, runtime); code != 0 {
		t.Fatalf("history code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if want := `["` + runtime.Paths.Roots[pfmengine.Claude][0] + `"]`; stdout.String() != want {
		t.Fatalf("history roots=%q, want %q", stdout.String(), want)
	}
}

func TestChatFindSearchesEveryConfiguredTranscriptRegistry(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	registry := filepath.Join(root, "registry")
	project := filepath.Join(registry, "project")
	if err := os.MkdirAll(project, 0o700); err != nil {
		t.Fatal(err)
	}
	const id = "30303030-3030-4030-8030-303030303030"
	line := `{"type":"user","timestamp":"2026-01-02T03:04:05Z","message":{"content":"a long distinctive sentence carried across the registry"}}` + "\n"
	if err := os.WriteFile(filepath.Join(project, id+".jsonl"), []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	excerpt := filepath.Join(root, "excerpt.txt")
	if err := os.WriteFile(excerpt, []byte("a long distinctive sentence carried across the registry\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PFM_HOME", home)
	t.Setenv("PFM_CLAUDE_ROOTS", registry)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"chat", "find", excerpt}, &stdout, &stderr); code != 0 {
		t.Fatalf("find code=%d stderr=%q", code, stderr.String())
	}
	if got, want := stdout.String(), id+"\t"+filepath.Join(project, id+".jsonl")+"\n"; got != want {
		t.Fatalf("stdout=%q, want %q", got, want)
	}
	if !strings.Contains(stderr.String(), "(1/1 needles hit)") {
		t.Fatalf("stderr=%q, want the hit/needle proof", stderr.String())
	}
}

func TestChatLoadEnumeratesTextAndSkipsBuildTrees(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "one.txt"), []byte("one\ntwo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ignored := filepath.Join(root, "node_modules")
	if err := os.MkdirAll(ignored, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ignored, "large.txt"), []byte("ignored\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"chat", "load", root}, &stdout, &stderr); code != 0 {
		t.Fatalf("load code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "      2  "+filepath.Join(root, "one.txt")) ||
		strings.Contains(stdout.String(), "node_modules") ||
		!strings.Contains(stdout.String(), "1 files, 2 total lines") {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestBranchCommandPlacesEveryUnsetBeforeAssignments(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(t.TempDir(), "account"))
	t.Setenv("ENABLE_PROMPT_CACHING_1H", "1")
	t.Setenv("FORCE_PROMPT_CACHING_5M", "")
	command := branchClaudeCommand("session-id", "safe name", "sonnet[1m]")
	assignment := strings.Index(command, "'CLAUDE_CONFIG_DIR=")
	lastUnset := strings.LastIndex(command, "'-u'")
	if assignment < 0 || lastUnset < 0 || lastUnset > assignment {
		t.Fatalf("env options must precede assignments: %s", command)
	}
}
