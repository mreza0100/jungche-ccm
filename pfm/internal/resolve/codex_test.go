package resolve

import (
	"strings"
	"testing"
)

func codexCandidates() []CodexThread {
	return []CodexThread{
		{
			ID:          "born-earlier",
			CWD:         "/work/project",
			CreatedAt:   1000,
			RolloutPath: "/codex/sessions/rollout-born-earlier.jsonl",
		},
		{
			ID:          "born-nearest",
			CWD:         "/work/project",
			CreatedAt:   1090,
			RolloutPath: "/codex/sessions/rollout-born-nearest.jsonl",
		},
		{
			ID:        "other-directory",
			CWD:       "/work/elsewhere",
			CreatedAt: 1100,
		},
		{
			ID:        "outside-window",
			CWD:       "/work/project",
			CreatedAt: 1221,
		},
	}
}

func TestCodexThreadIDPrefersTheExportedIdentity(t *testing.T) {
	// The birth match would pick born-nearest; the exported identity outranks
	// it, and it is honored even when the state store has not recorded it yet.
	thread, err := CodexThreadID(
		"born-earlier",
		"/work/project",
		1100,
		codexCandidates(),
	)
	if err != nil {
		t.Fatalf("CodexThreadID(exported) error = %v", err)
	}
	if thread.ID != "born-earlier" ||
		thread.RolloutPath != "/codex/sessions/rollout-born-earlier.jsonl" {
		t.Fatalf("CodexThreadID(exported) = %#v", thread)
	}

	unknown, err := CodexThreadID(
		"not-in-the-store",
		"/work/project",
		1100,
		codexCandidates(),
	)
	if err != nil {
		t.Fatalf("CodexThreadID(unknown export) error = %v", err)
	}
	if unknown.ID != "not-in-the-store" || unknown.RolloutPath != "" {
		t.Fatalf("CodexThreadID(unknown export) = %#v", unknown)
	}
}

func TestCodexThreadIDFallsBackToDirectoryAndBirth(t *testing.T) {
	thread, err := CodexThreadID("", "/work/project/", 1100, codexCandidates())
	if err != nil {
		t.Fatalf("CodexThreadID(birth match) error = %v", err)
	}
	if thread.ID != "born-nearest" {
		t.Fatalf("CodexThreadID(birth match) = %#v, want the nearest birth", thread)
	}

	edge, err := CodexThreadID("", "/work/project", 1341, []CodexThread{
		{ID: "exactly-at-the-bound", CWD: "/work/project", CreatedAt: 1221},
	})
	if err != nil {
		t.Fatalf("CodexThreadID(window edge) error = %v", err)
	}
	if edge.ID != "exactly-at-the-bound" {
		t.Fatalf("CodexThreadID(window edge) = %#v, want the %ds bound inclusive",
			edge, CodexBirthWindowSeconds)
	}
}

func TestCodexThreadIDReportsAMissInsteadOfGuessing(t *testing.T) {
	// 121 seconds past born-nearest and 221 past born-earlier: nothing is
	// close enough, and the other candidate is in a different directory.
	if thread, err := CodexThreadID(
		"",
		"/work/project",
		1221,
		codexCandidates()[:3],
	); err == nil {
		t.Fatalf("CodexThreadID(no match) = %#v, want an error", thread)
	} else if !strings.Contains(err.Error(), "/work/project") ||
		!strings.Contains(err.Error(), "120s") {
		t.Fatalf("CodexThreadID(no match) error = %v, want the directory and window", err)
	}

	if _, err := CodexThreadID("", "", 1100, codexCandidates()); err == nil ||
		!strings.Contains(err.Error(), CodexThreadEnv) {
		t.Fatalf("CodexThreadID(no directory) error = %v, want a clean refusal", err)
	}
	if _, err := CodexThreadID("", "/work/project", 0, codexCandidates()); err == nil {
		t.Fatal("CodexThreadID(no birth time) = nil error, want a clean refusal")
	}
}
