package compose

import (
	"testing"

	"hostops/cc-fleet/internal/gather"
	"hostops/cc-fleet/internal/store"
)

// TestLabelHiddenChatLeavesTheDefaultListing pins the rename-to-hide rule: a
// chat whose label starts with "_HIDE" is out of the default list, in the
// hidden view, and counted as hidden — with no row in the hidden table.
func TestLabelHiddenChatLeavesTheDefaultListing(t *testing.T) {
	worker := transcript(
		"labelled",
		"/accounts/1/projects/alpha/labelled.jsonl",
		"/work/alpha",
		"_HIDE headless worker",
		100,
		5,
		900,
	)
	plain := transcript(
		"plain",
		"/accounts/1/projects/alpha/plain.jsonl",
		"/work/alpha",
		"Ordinary chat",
		100,
		5,
		800,
	)
	input := Input{
		Transcripts:  []store.Transcript{worker, plain},
		AccountRoots: fixtureAccountRoots(),
		Options:      Options{View: DefaultView, PrimaryAccount: 1},
	}

	output := Compose(input)
	if _, found := rowByID(output.Rows, "labelled"); found {
		t.Fatalf("_HIDE-labelled chat listed by default: %#v", output.Rows)
	}
	if _, found := rowByID(output.Rows, "plain"); !found {
		t.Fatalf("ordinary chat missing from the default list: %#v", output.Rows)
	}
	if output.HiddenCount != 1 {
		t.Fatalf("hidden count = %d, want 1", output.HiddenCount)
	}
	if output.SuppressedCount != 0 {
		t.Fatalf("suppressed count = %d, want 0", output.SuppressedCount)
	}

	input.Options.View = HiddenView
	hidden := Compose(input)
	row, found := rowByID(hidden.Rows, "labelled")
	if !found {
		t.Fatalf("_HIDE-labelled chat missing from the hidden view: %#v", hidden.Rows)
	}
	if !row.Hidden || !row.NameHidden {
		t.Fatalf("label hide not marked on the row: %#v", row)
	}
	if _, found := rowByID(hidden.Rows, "plain"); found {
		t.Fatalf("ordinary chat leaked into the hidden view: %#v", hidden.Rows)
	}
}

// TestLabelHideCoversLiveAndCodexRowsAndFoldsCase covers the two row shapes a
// headless worker actually takes — a live Claude pane and a resumable Codex
// lineage — and the lower-case spelling.
func TestLabelHideCoversLiveAndCodexRowsAndFoldsCase(t *testing.T) {
	live := transcript(
		"live-worker",
		"/accounts/1/projects/alpha/live-worker.jsonl",
		"/work/alpha",
		"_hide live worker",
		100,
		5,
		900,
	)
	rollout := store.Rollout{
		ID:          "019f-worker",
		Path:        "/codex/rollout-2026-01-01T00-00-00-019f-worker.jsonl",
		Size:        800,
		MTimeNS:     1300,
		CWD:         "/work/alpha",
		UserThread:  true,
		FirstPrompt: "unnamed",
		PromptCount: 4,
	}
	input := Input{
		Transcripts: []store.Transcript{live},
		Rollouts:    []store.Rollout{rollout},
		CxNames:     map[string]string{"019f-worker": "_HIDE codex worker"},
		Snapshot: gather.Snapshot{
			Panes: []gather.Pane{{Socket: "cc-1-2-3", PaneID: "%1"}},
			Crumbs: []gather.Crumb{{
				Filename:       "cc-1-2-3",
				Socket:         "cc-1-2-3",
				PaneID:         "%1",
				TranscriptPath: live.Path,
			}},
		},
		AccountRoots: fixtureAccountRoots(),
		Options:      Options{View: DefaultView, PrimaryAccount: 1},
	}

	output := Compose(input)
	if row, found := rowByID(output.Rows, "live-worker"); found {
		t.Fatalf("live _hide chat listed by default: %#v", row)
	}
	if row, found := rowByID(output.Rows, "019f-worker"); found {
		t.Fatalf("Codex _HIDE chat listed by default: %#v", row)
	}
	if output.HiddenCount != 2 {
		t.Fatalf("hidden count = %d, want 2", output.HiddenCount)
	}
}

// TestSplitRowKeepsItsJoinedName guards the one exclusion: a split row's Name
// is a join of its panes' names, so a "_HIDE…" first member must not take the
// whole socket out of the list.
func TestSplitRowKeepsItsJoinedName(t *testing.T) {
	first := transcript(
		"split-a",
		"/accounts/1/projects/alpha/split-a.jsonl",
		"/work/alpha",
		"_HIDE left pane",
		100,
		5,
		900,
	)
	second := transcript(
		"split-b",
		"/accounts/1/projects/alpha/split-b.jsonl",
		"/work/alpha",
		"Right pane",
		100,
		5,
		901,
	)
	input := Input{
		Transcripts: []store.Transcript{first, second},
		Snapshot: gather.Snapshot{
			Panes: []gather.Pane{
				{Socket: "cc-9-9-9", PaneID: "%1"},
				{Socket: "cc-9-9-9", PaneID: "%2"},
			},
			Crumbs: []gather.Crumb{
				{
					Filename:       "cc-9-9-9-%1",
					Socket:         "cc-9-9-9",
					PaneID:         "%1",
					TranscriptPath: first.Path,
				},
				{
					Filename:       "cc-9-9-9-%2",
					Socket:         "cc-9-9-9",
					PaneID:         "%2",
					TranscriptPath: second.Path,
				},
			},
		},
		AccountRoots: fixtureAccountRoots(),
		Options:      Options{View: DefaultView, PrimaryAccount: 1},
	}

	output := Compose(input)
	splits := rowsByKind(output.Rows, LiveSplit)
	if len(splits) != 1 {
		t.Fatalf("split row missing from the default list: %#v", output.Rows)
	}
	if splits[0].Hidden || splits[0].NameHidden {
		t.Fatalf("split row hidden by a member's label: %#v", splits[0])
	}
}
