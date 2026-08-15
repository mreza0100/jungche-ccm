package store

import (
	"context"
	"testing"
)

// TestLabelHiddenCandidatesNeverSpendAFrameSlot is the cached first frame's
// half of the rename-to-hide rule. compose would drop these rows anyway, so
// the point of the SQL/lineage predicates is the WINDOW: a fleet of headless
// "_HIDE…" workers must not push real chats out of the limited candidate set
// the frame is built from, and the counts must call them hidden, not
// suppressed.
func TestLabelHiddenCandidatesNeverSpendAFrameSlot(t *testing.T) {
	setStoreTestJail(t)
	database := openTestStore(t)
	defer database.Close()
	ctx := context.Background()

	// The two workers are NEWER than the real chat, so a window of one returns
	// the real chat only if they never entered it.
	for _, transcript := range []Transcript{
		{
			UUID: "worker-upper", Path: "/cc/worker-upper.jsonl", Size: 100,
			MTimeNS: 300, CWD: "/work/a", CustomTitle: "_HIDE worker 1",
			FirstPrompt: "go", PromptCount: 2,
		},
		{
			UUID: "worker-lower", Path: "/cc/worker-lower.jsonl", Size: 100,
			MTimeNS: 200, CWD: "/work/a", FirstPrompt: "_hide worker 2",
			PromptCount: 2,
		},
		{
			UUID: "real", Path: "/cc/real.jsonl", Size: 100,
			MTimeNS: 100, CWD: "/work/a", CustomTitle: "Real chat",
			FirstPrompt: "hello", PromptCount: 2,
		},
	} {
		if err := database.UpsertTranscript(ctx, transcript); err != nil {
			t.Fatal(err)
		}
	}

	transcripts, _, counts, err := database.DefaultCandidates(ctx, 1, 15)
	if err != nil {
		t.Fatal(err)
	}
	if len(transcripts) != 1 || transcripts[0].UUID != "real" {
		t.Fatalf("cached candidates = %#v, want only the real chat", transcripts)
	}
	if counts.Hidden != 2 {
		t.Fatalf("hidden count = %d, want 2", counts.Hidden)
	}
	if counts.Suppressed != 0 {
		t.Fatalf("suppressed count = %d, want 0", counts.Suppressed)
	}
}

// TestLabelHiddenCodexLineageLeavesTheCachedFrame covers the Codex half, whose
// name comes from cx_names through the lineage walk rather than from a column.
func TestLabelHiddenCodexLineageLeavesTheCachedFrame(t *testing.T) {
	setStoreTestJail(t)
	database := openTestStore(t)
	defer database.Close()
	ctx := context.Background()

	for _, rollout := range []Rollout{
		{
			ID: "cx-worker", Path: "/cx/worker.jsonl", Size: 100,
			MTimeNS: 300, CWD: "/work/a", UserThread: true,
			FirstPrompt: "go", PromptCount: 2,
		},
		{
			ID: "cx-real", Path: "/cx/real.jsonl", Size: 100,
			MTimeNS: 200, CWD: "/work/a", UserThread: true,
			FirstPrompt: "hello", PromptCount: 2,
		},
	} {
		if err := database.UpsertRollout(ctx, rollout); err != nil {
			t.Fatal(err)
		}
	}
	if err := database.UpsertCxName(ctx, CxName{
		ID:         "cx-worker",
		ThreadName: "_HIDE codex worker",
		Source:     CxNameSourceStore,
	}); err != nil {
		t.Fatal(err)
	}

	_, rollouts, counts, err := database.DefaultCandidates(ctx, 30, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(rollouts) != 1 || rollouts[0].ID != "cx-real" {
		t.Fatalf("cached rollouts = %#v, want only the real thread", rollouts)
	}
	if counts.Hidden != 1 {
		t.Fatalf("hidden count = %d, want 1", counts.Hidden)
	}
}
