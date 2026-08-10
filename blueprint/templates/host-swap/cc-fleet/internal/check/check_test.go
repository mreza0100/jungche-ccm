package check

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"hostops/cc-fleet/internal/compose"
)

func TestParseDiffAndAllowlist(t *testing.T) {
	const output = "\x1b[32m" + legacyTupleMarker +
		"\tchat-1\tresume-claude\tproject-alpha\t7\x1b[0m\n" +
		legacyTupleMarker + "\tchat-2\tresume-codex\tproject-beta\t3\n" +
		"cc-ls: fzf not found\n"
	rows, err := ParseLegacy(output)
	if err != nil {
		t.Fatal(err)
	}
	own := []Tuple{
		{ID: "chat-1", Kind: "resume-claude", Project: "project-alpha", Prompts: 7},
		{ID: "chat-2", Kind: "resume-codex", Project: "project-beta", Prompts: 4},
	}
	legacy := ReconcileIDs(rows, own)
	differences := Diff(legacy, own, nil)
	if len(differences) != 2 {
		t.Fatalf("Diff() returned %d differences, want 2: %#v", len(differences), differences)
	}

	rules, err := ParseAllowlist(strings.NewReader(
		"either\tchat-2\tresume-codex\tproject-beta\tpartial-tail prompt count\n",
	))
	if err != nil {
		t.Fatal(err)
	}
	differences = Diff(legacy, own, rules)
	for _, difference := range differences {
		if !difference.Allowed || difference.Reason != "partial-tail prompt count" {
			t.Fatalf("allowlisted difference = %#v", difference)
		}
	}
}

func TestParseDisplayedRowsAndReconcile(t *testing.T) {
	output := strings.Join([]string{
		"● project-alpha  │ active chat                    │     7p │  123B │ now",
		"↻ project-beta   │ ⬢ codex thread                 │     3p │  456B │ 1m",
		"⚙ agents         │ teammate                       │     2p │  789B │ 2m",
	}, "\n")
	rows, err := ParseLegacy(output)
	if err != nil {
		t.Fatal(err)
	}
	own := []Tuple{
		{ID: "live-1", Kind: "live-claude", Project: "project-alpha", Prompts: 7},
		{ID: "cx-1", Kind: "resume-codex", Project: "project-beta", Prompts: 3},
		{ID: "agent-1", Kind: "agent", Project: "agents", Prompts: 2},
	}
	if differences := Diff(ReconcileIDs(rows, own), own, nil); len(differences) != 0 {
		t.Fatalf("display reconciliation differences = %#v", differences)
	}
}

func TestLiveCodexCanonicalIdentityIsSocket(t *testing.T) {
	tuples := TuplesFromRows([]compose.Row{{
		Kind:        compose.LiveCodex,
		ID:          "019f-rollout",
		Socket:      "cx-100-200-300",
		Project:     "host-ops",
		PromptCount: 12,
	}})
	if len(tuples) != 1 || tuples[0] != (Tuple{
		ID:      "cx-100-200-300",
		Kind:    "live-codex",
		Project: "host-ops",
		Prompts: 12,
	}) {
		t.Fatalf("live Codex tuple = %#v", tuples)
	}
}

func TestMultiWindowCodexNormalizesToServerProjectSet(t *testing.T) {
	own := TuplesFromRows([]compose.Row{
		{
			Kind:        compose.LiveCodex,
			ID:          "rollout-a",
			Socket:      "cx-1-2-3",
			Project:     "projb-fe",
			PromptCount: 17,
		},
		{
			Kind:        compose.LiveCodex,
			ID:          "rollout-b",
			Socket:      "cx-1-2-3",
			Project:     "proja",
			PromptCount: 23,
		},
		{
			Kind:        compose.LiveCodex,
			ID:          "rollout-c",
			Socket:      "cx-1-2-3",
			Project:     "projb-be",
			PromptCount: 31,
		},
	})
	legacy := []Tuple{
		{ID: "cx-1-2-3", Kind: "live-codex", Project: "proja", Prompts: 17},
		{ID: "cx-1-2-3", Kind: "live-codex", Project: "projb-be", Prompts: 17},
		{ID: "cx-1-2-3", Kind: "live-codex", Project: "projb-fe", Prompts: 17},
	}
	own = NormalizeLiveCodex(own)
	legacy = NormalizeLiveCodex(legacy)
	if differences := Diff(legacy, own, nil); len(differences) != 0 {
		t.Fatalf("multi-window server differences = %#v; own=%#v legacy=%#v",
			differences, own, legacy)
	}
	if len(own) != 1 ||
		own[0].Project != "proja,projb-be,projb-fe" ||
		own[0].Prompts != 0 {
		t.Fatalf("normalized multi-window tuple = %#v", own)
	}
}

func TestAllowlistRejectsBroadOrUnexplainedSyntax(t *testing.T) {
	for _, input := range []string{
		"go-only\tid\tkind\tproject\n",
		"go-only\tid\tkind\tproject\t\n",
		"unknown\tid\tkind\tproject\treason\n",
		"go-only\t[\tkind\tproject\treason\n",
		"class\tunknown-class\treason\n",
	} {
		if _, err := ParseAllowlist(strings.NewReader(input)); err == nil {
			t.Fatalf("ParseAllowlist(%q) unexpectedly succeeded", input)
		}
	}
}

func TestVerifiedAllowlistClasses(t *testing.T) {
	rules, err := ParseAllowlist(strings.NewReader(strings.Join([]string{
		"class\tcodex-120-bound\tverified bound",
		"class\tcodex-prompt-semantics\tverified prompt semantics",
		"class\tlive-prompt-drift\tverified live drift",
		"class\tdormant-account\tverified dormant account",
		"class\tlegacy-dead-server\tverified missing codex process",
		"class\tcodex-legacy-source\tverified newer source schema",
		"class\tlegacy-nonfleet-multiwindow\tverified non-fleet multiwindow",
	}, "\n")))
	if err != nil {
		t.Fatal(err)
	}
	legacy := []Tuple{
		{ID: "within-120", Kind: "resume-codex", Project: "proja", Prompts: 9},
		{ID: "live", Kind: "live-claude", Project: "host-ops", Prompts: 10},
		{ID: "cx-dead", Kind: "live-codex", Project: "host-ops", Prompts: 0},
	}
	own := []Tuple{
		{ID: "within-120", Kind: "resume-codex", Project: "proja", Prompts: 8},
		{ID: "outside-120", Kind: "resume-codex", Project: "proja", Prompts: 2},
		{ID: "live", Kind: "live-claude", Project: "host-ops", Prompts: 11},
		{ID: "account-2", Kind: "resume-claude", Project: "extra", Prompts: 3},
		{ID: "new-source", Kind: "resume-codex", Project: "extra", Prompts: 1},
	}
	differences := DiffVerified(legacy, own, rules, Verification{
		CodexCandidateIDs: map[string]struct{}{
			"within-120": {},
			"new-source": {},
		},
		CodexLegacySource: map[string]struct{}{"new-source": {}},
		DormantAccountIDs: map[string]struct{}{"account-2": {}},
		LiveCodexSockets:  map[string]struct{}{},
	})
	if len(differences) != 8 {
		t.Fatalf("DiffVerified() returned %d differences: %#v", len(differences), differences)
	}
	for _, difference := range differences {
		if !difference.Allowed || difference.Class == "" ||
			!strings.HasPrefix(difference.Reason, "verified ") {
			t.Fatalf("unverified semantic difference = %#v", difference)
		}
	}
	differences = DiffVerified(
		[]Tuple{{
			ID: "cx-live", Kind: "live-codex", Project: "host-ops",
		}},
		nil,
		rules,
		Verification{LiveCodexSockets: map[string]struct{}{"cx-live": {}}},
	)
	if len(differences) != 1 || differences[0].Allowed {
		t.Fatalf("living Codex socket was allowlisted as dead: %#v", differences)
	}

	// An ID still inside the newest-120 population must not be hidden by the
	// bound class when it is missing entirely from legacy output.
	differences = DiffVerified(
		nil,
		[]Tuple{{
			ID: "within-120", Kind: "resume-codex", Project: "proja", Prompts: 8,
		}},
		rules,
		Verification{CodexCandidateIDs: map[string]struct{}{"within-120": {}}},
	)
	if len(differences) != 1 || differences[0].Allowed {
		t.Fatalf("within-bound missing tuple was allowlisted: %#v", differences)
	}
}

func TestNonFleetMultiWindowClassRequiresExcludedSocketShape(t *testing.T) {
	rules, err := ParseAllowlist(strings.NewReader(
		"class\tlegacy-nonfleet-multiwindow\tverified non-fleet multiwindow\n",
	))
	if err != nil {
		t.Fatal(err)
	}
	legacy := []Tuple{
		{
			ID:      "projc-dev",
			Kind:    "live-claude",
			Project: "backend",
			Prompts: 0,
		},
		{
			ID:      "projc-dev",
			Kind:    "live-claude",
			Project: "frontend",
			Prompts: 0,
		},
	}
	differences := DiffVerified(
		legacy,
		nil,
		rules,
		Verification{},
	)
	if len(differences) != 2 {
		t.Fatalf("DiffVerified() returned %d differences: %#v", len(differences), differences)
	}
	for _, difference := range differences {
		if !difference.Allowed ||
			difference.Class != "legacy-nonfleet-multiwindow" ||
			difference.Reason != "verified non-fleet multiwindow" {
			t.Fatalf("non-fleet multiwindow difference = %#v", difference)
		}
	}

	for name, input := range map[string][]Tuple{
		"fleet socket": {
			{
				ID:      "cc-1-2-3",
				Kind:    "live-claude",
				Project: "backend",
				Prompts: 0,
			},
			{
				ID:      "cc-1-2-3",
				Kind:    "live-claude",
				Project: "frontend",
				Prompts: 0,
			},
		},
		"single window": {legacy[0]},
	} {
		t.Run(name, func(t *testing.T) {
			differences := DiffVerified(input, nil, rules, Verification{})
			for _, difference := range differences {
				if difference.Allowed {
					t.Fatalf("unverified difference was allowlisted: %#v", difference)
				}
			}
		})
	}

	differences = DiffVerified(
		legacy,
		[]Tuple{legacy[0]},
		rules,
		Verification{},
	)
	for _, difference := range differences {
		if difference.Allowed {
			t.Fatalf("partially live server difference was allowlisted: %#v", difference)
		}
	}
}

func TestCodexBoundClassRejectsDuplicateRowsInOneLineage(t *testing.T) {
	rules, err := ParseAllowlist(strings.NewReader(
		"class\tcodex-120-bound\tverified one-per-lineage bound\n",
	))
	if err != nil {
		t.Fatal(err)
	}
	roots := map[string]string{
		"root":    "root",
		"child-a": "root",
		"child-b": "root",
	}
	single := DiffVerified(
		nil,
		[]Tuple{{
			ID: "child-a", Kind: "resume-codex", Project: "web", Prompts: 10,
		}},
		rules,
		Verification{
			CodexCandidateIDs: map[string]struct{}{},
			CodexLineageRoots: roots,
		},
	)
	if len(single) != 1 ||
		!single[0].Allowed ||
		single[0].Class != "codex-120-bound" {
		t.Fatalf("single outside-bound lineage difference = %#v", single)
	}

	duplicated := DiffVerified(
		nil,
		[]Tuple{
			{
				ID: "child-a", Kind: "resume-codex",
				Project: "web", Prompts: 10,
			},
			{
				ID: "child-b", Kind: "resume-codex",
				Project: "web", Prompts: 10,
			},
		},
		rules,
		Verification{
			CodexCandidateIDs: map[string]struct{}{},
			CodexLineageRoots: roots,
		},
	)
	if len(duplicated) != 2 {
		t.Fatalf("duplicated lineage differences = %#v", duplicated)
	}
	for _, difference := range duplicated {
		if difference.Allowed {
			t.Fatalf("duplicated lineage was allowlisted: %#v", duplicated)
		}
	}
}

func TestANSIUTF8FuzzNoPhantomRows(t *testing.T) {
	var builder strings.Builder
	for index := 0; index < 1000; index++ {
		switch index % 4 {
		case 0:
			fmt.Fprintf(
				&builder,
				"\x1b[3%dm%s\tid-%04d\tresume-claude\tproj-界-%d\t%d\x1b[0m\n",
				index%8,
				legacyTupleMarker,
				index,
				index%17,
				index%31,
			)
		case 1:
			fmt.Fprintf(
				&builder,
				"\x1b]8;;https://invalid.example/%d\x1b\\%s\tid-%04d\tresume-claude\tcombining-e\u0301\t%d\x1b]8;;\x1b\\\n",
				index,
				legacyTupleMarker,
				index,
				index%31,
			)
		case 2:
			fmt.Fprintf(
				&builder,
				"\x1b[2K%s\tid-%04d\tresume-codex\temoji-🧪\t%d\n",
				legacyTupleMarker,
				index,
				index%31,
			)
		case 3:
			fmt.Fprintf(
				&builder,
				"%s\tR\tid-%04d\tid-%04d\t/work\t↻ invalid-utf8 │ na",
				legacyRowMarker,
				index,
				index,
			)
			builder.WriteByte(0xff)
			fmt.Fprintf(
				&builder,
				"me │ %dp │ 1B │ now\n",
				index%31,
			)
		}
	}
	rows, err := ParseLegacy(builder.String())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1000 {
		t.Fatalf("ParseLegacy() returned %d rows, want 1000", len(rows))
	}
	for index, row := range rows {
		if row.ID != fmt.Sprintf("id-%04d", index) {
			t.Fatalf("row %d ID = %q", index, row.ID)
		}
	}
	t.Log("STRESS ansi_utf8 cases=1000 panics=0 phantom_rows=0")
}

func TestCheckStress(t *testing.T) {
	const total = 5000
	legacy := make([]Tuple, 0, total)
	for index := 0; index < total; index++ {
		legacy = append(legacy, Tuple{
			ID:      fmt.Sprintf("id-%05d", index),
			Kind:    "resume-claude",
			Project: fmt.Sprintf("project-%03d", index%200),
			Prompts: int64(index % 47),
		})
	}
	if differences := Diff(legacy, legacy, nil); len(differences) != 0 {
		t.Fatalf("identical corpus produced %d false positives", len(differences))
	}
	own := append([]Tuple(nil), legacy[25:]...)
	for index := 0; index < 25; index++ {
		own = append(own, Tuple{
			ID:      fmt.Sprintf("go-only-%02d", index),
			Kind:    "resume-codex",
			Project: "seeded",
			Prompts: int64(index),
		})
	}
	started := time.Now()
	differences := Diff(legacy, own, nil)
	elapsed := time.Since(started)
	if len(differences) != 50 {
		t.Fatalf("seeded corpus produced %d differences, want 50", len(differences))
	}
	limit := 2 * time.Second
	if os.Getenv("CC_FLEET_STRESS_STRICT") == "1" {
		limit = time.Second
	}
	t.Logf(
		"STRESS check rows=%d seeded=50 caught=%d false_positives=0 elapsed=%s limit=%s strict=%t",
		total,
		len(differences),
		elapsed,
		limit,
		os.Getenv("CC_FLEET_STRESS_STRICT") == "1",
	)
	if elapsed >= limit {
		t.Fatalf("Diff() took %s, want <%s", elapsed, limit)
	}
}
