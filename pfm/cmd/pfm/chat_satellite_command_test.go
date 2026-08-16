package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
