package recovery

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunRebuildsMessagesAndCompactionMemoryFromRollout(t *testing.T) {
	root := t.TempDir()
	threadID := "11111111-1111-4111-8111-111111111111"
	rollout := filepath.Join(root, "rollout-"+threadID+".jsonl")
	large := strings.Repeat("x", 70*1024)
	content := strings.Join([]string{
		`{"type":"response_item","timestamp":"2026-08-17T10:00:00Z","payload":{"type":"message","role":"user","content":[{"text":"hello"}]}}`,
		`{"type":"response_item","timestamp":"2026-08-17T10:01:00Z","payload":{"type":"message","role":"assistant","content":[{"text":"` + large + `"}]}}`,
		`{"type":"compacted","timestamp":"2026-08-17T10:02:00Z","payload":{"replacement_history":[{"role":"assistant","content":[{"text":"carried forward"}]},{"role":"user","content":[{"text":"# AGENTS.md instructions are boilerplate"}]}]}}`,
		"not json",
	}, "\n") + "\n"
	if err := os.WriteFile(rollout, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := Run(context.Background(), root, rollout)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.ThreadID != threadID || result.Messages != 2 || result.Carried != 1 || result.Malformed != 1 {
		t.Fatalf("Run() result = %+v", result)
	}
	out := filepath.Join(root, "recovered-"+threadID)
	for _, name := range []string{"transcript.md", "compaction-memory.md", "brief.md"} {
		path := filepath.Join(out, name)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("missing recovered %s: %v", name, err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("recovered %s mode = %o, want 0600", name, info.Mode().Perm())
		}
	}
	transcript, err := os.ReadFile(filepath.Join(out, "transcript.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(transcript), "hello") || !strings.Contains(string(transcript), large) {
		t.Fatal("transcript omitted a parsed message")
	}
	memory, err := os.ReadFile(filepath.Join(out, "compaction-memory.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(memory), "carried forward") || strings.Contains(string(memory), "AGENTS.md") {
		t.Fatalf("compaction memory filtering regressed:\n%s", memory)
	}
}

func TestRunChecksContextBeforeLocatingRollout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Run(ctx, t.TempDir(), "missing-thread")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}
}
