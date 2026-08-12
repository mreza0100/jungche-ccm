package check

import (
	"fmt"
	"os"
	"reflect"
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
		"class\tcodex-store-only\tverified store-only thread",
		"class\tcodex-prompt-semantics\tverified prompt semantics",
		"class\tlive-prompt-drift\tverified live drift",
		"class\tdormant-account\tverified dormant account",
		"class\tcodex-legacy-source\tverified newer source schema",
		"class\tlegacy-cache-empty-cwd\tverified stale cache cwd",
		"class\tlegacy-socket-squatter\tverified parked session",
	}, "\n")))
	if err != nil {
		t.Fatal(err)
	}
	legacy := []Tuple{
		{ID: "within-120", Kind: "resume-codex", Project: "proja", Prompts: 9},
		{ID: "live", Kind: "live-claude", Project: "host-ops", Prompts: 10},
		{ID: "stale-cwd", Kind: "resume-claude", Project: "?", Prompts: 4},
		{ID: "cc-1-2-3", Kind: "live-claude", Project: "parked", Prompts: 0},
	}
	own := []Tuple{
		{ID: "within-120", Kind: "resume-codex", Project: "proja", Prompts: 8},
		{ID: "outside-120", Kind: "resume-codex", Project: "proja", Prompts: 2},
		{ID: "no-file", Kind: "resume-codex", Project: "proja", Prompts: 5},
		{ID: "live", Kind: "live-claude", Project: "host-ops", Prompts: 11},
		{ID: "account-2", Kind: "resume-claude", Project: "extra", Prompts: 3},
		{ID: "new-source", Kind: "resume-codex", Project: "extra", Prompts: 1},
		{ID: "stale-cwd", Kind: "resume-claude", Project: "projc", Prompts: 4},
	}
	differences := DiffVerified(legacy, own, rules, Verification{
		CodexCandidateIDs: map[string]struct{}{
			"within-120": {},
			"new-source": {},
		},
		CodexRolloutFileIDs: map[string]struct{}{
			"within-120":  {},
			"outside-120": {},
			"new-source":  {},
		},
		CodexLegacySource: map[string]struct{}{"new-source": {}},
		LegacyEmptyCWDIDs: map[string]struct{}{"stale-cwd": {}},
		SocketSquatters:   map[string]int{"cc-1-2-3": 1},
		DormantAccountIDs: map[string]struct{}{"account-2": {}},
	})
	if len(differences) != 11 {
		t.Fatalf("DiffVerified() returned %d differences: %#v", len(differences), differences)
	}
	classes := make(map[string]int)
	for _, difference := range differences {
		if !difference.Allowed || difference.Class == "" ||
			!strings.HasPrefix(difference.Reason, "verified ") {
			t.Fatalf("unverified semantic difference = %#v", difference)
		}
		classes[difference.Class]++
	}
	want := map[string]int{
		"codex-120-bound":        1,
		"codex-store-only":       1,
		"codex-legacy-source":    1,
		"codex-prompt-semantics": 2,
		"live-prompt-drift":      2,
		"dormant-account":        1,
		"legacy-cache-empty-cwd": 2,
		"legacy-socket-squatter": 1,
	}
	if !reflect.DeepEqual(classes, want) {
		t.Fatalf("class census = %#v, want %#v", classes, want)
	}

	// An ID still inside the newest-120 population, whose lineage owns a rollout
	// file, must not be hidden by either Codex class when it is missing entirely
	// from legacy output.
	differences = DiffVerified(
		nil,
		[]Tuple{{
			ID: "within-120", Kind: "resume-codex", Project: "proja", Prompts: 8,
		}},
		rules,
		Verification{
			CodexCandidateIDs:   map[string]struct{}{"within-120": {}},
			CodexRolloutFileIDs: map[string]struct{}{"within-120": {}},
		},
	)
	if len(differences) != 1 || differences[0].Allowed {
		t.Fatalf("within-bound missing tuple was allowlisted: %#v", differences)
	}
}

func TestCodexStoreOnlyClassRequiresAnAbsentRolloutFile(t *testing.T) {
	rules, err := ParseAllowlist(strings.NewReader(strings.Join([]string{
		"class\tcodex-store-only\tverified store-only thread",
		"class\tcodex-120-bound\tverified bound",
	}, "\n")))
	if err != nil {
		t.Fatal(err)
	}
	roots := map[string]string{"child": "root", "root": "root"}
	rowsWithoutFile := DiffVerified(
		nil,
		[]Tuple{{ID: "child", Kind: "resume-codex", Project: "web", Prompts: 3}},
		rules,
		Verification{
			CodexCandidateIDs:   map[string]struct{}{"child": {}},
			CodexRolloutFileIDs: map[string]struct{}{},
			CodexLineageRoots:   roots,
		},
	)
	if len(rowsWithoutFile) != 1 ||
		rowsWithoutFile[0].Class != "codex-store-only" {
		t.Fatalf("store-only difference = %#v", rowsWithoutFile)
	}

	// A file ANYWHERE in the lineage disqualifies the class: the legacy scan
	// walks files, so such a lineage is reachable and must be judged by the
	// newest-N bound instead.
	siblingHasFile := DiffVerified(
		nil,
		[]Tuple{{ID: "child", Kind: "resume-codex", Project: "web", Prompts: 3}},
		rules,
		Verification{
			CodexCandidateIDs:   map[string]struct{}{"child": {}},
			CodexRolloutFileIDs: map[string]struct{}{"root": {}},
			CodexLineageRoots:   roots,
		},
	)
	if len(siblingHasFile) != 1 || siblingHasFile[0].Allowed {
		t.Fatalf("lineage with a rollout file was allowlisted: %#v", siblingHasFile)
	}

	// The one-row-per-lineage bound applies to the store-only class too.
	duplicated := DiffVerified(
		nil,
		[]Tuple{
			{ID: "child", Kind: "resume-codex", Project: "web", Prompts: 3},
			{ID: "root", Kind: "resume-codex", Project: "web", Prompts: 4},
		},
		rules,
		Verification{
			CodexRolloutFileIDs: map[string]struct{}{},
			CodexLineageRoots:   roots,
		},
	)
	if len(duplicated) != 2 {
		t.Fatalf("duplicated store-only lineage = %#v", duplicated)
	}
	for _, difference := range duplicated {
		if difference.Allowed {
			t.Fatalf("duplicated store-only lineage was allowlisted: %#v", duplicated)
		}
	}
}

func TestAgentClassNeedsAnAbsentTranscriptFile(t *testing.T) {
	rules, err := ParseAllowlist(strings.NewReader(
		"class\tagent-without-transcript\tverified newborn agent\n",
	))
	if err != nil {
		t.Fatal(err)
	}
	own := []Tuple{
		{ID: "newborn", Kind: "agent", Project: "?", Prompts: 0},
		{ID: "flushed", Kind: "agent", Project: "host-ops", Prompts: 3},
	}
	differences := DiffVerified(nil, own, rules, Verification{
		ClaudeTranscriptIDs: map[string]struct{}{"flushed": {}},
	})
	if len(differences) != 2 {
		t.Fatalf("DiffVerified() returned %d differences: %#v", len(differences), differences)
	}
	for _, difference := range differences {
		newborn := difference.Tuple.ID == "newborn"
		if difference.Allowed != newborn {
			t.Fatalf("agent-without-transcript verdict for %#v", difference)
		}
	}

	// Two rows for ONE newborn agent is a duplication regression: the class
	// covers the first and the second stays visible.
	duplicated := DiffVerified(
		nil,
		[]Tuple{
			{ID: "newborn", Kind: "agent", Project: "?", Prompts: 0},
			{ID: "newborn", Kind: "agent", Project: "host-ops", Prompts: 0},
		},
		rules,
		Verification{ClaudeTranscriptIDs: map[string]struct{}{}},
	)
	unallowed := 0
	for _, difference := range duplicated {
		if !difference.Allowed {
			unallowed++
		}
	}
	if unallowed != 1 {
		t.Fatalf("duplicated newborn agent rows unallowed = %d: %#v",
			unallowed, duplicated)
	}

	// A legacy-only agent row is the opposite fault — Go LOST a row — and has
	// no class at all.
	lost := DiffVerified(
		[]Tuple{{ID: "newborn", Kind: "agent", Project: "?", Prompts: 0}},
		nil,
		rules,
		Verification{ClaudeTranscriptIDs: map[string]struct{}{}},
	)
	if len(lost) != 1 || lost[0].Allowed {
		t.Fatalf("a legacy-only agent row was allowlisted: %#v", lost)
	}
}

func TestCacheEmptyCWDClassNeedsTheStaleCacheRow(t *testing.T) {
	rules, err := ParseAllowlist(strings.NewReader(
		"class\tlegacy-cache-empty-cwd\tverified stale cache cwd\n",
	))
	if err != nil {
		t.Fatal(err)
	}
	legacy := []Tuple{
		{ID: "stale", Kind: "resume-claude", Project: "?", Prompts: 6},
		{ID: "fresh", Kind: "resume-claude", Project: "?", Prompts: 6},
	}
	own := []Tuple{
		{ID: "stale", Kind: "resume-claude", Project: "projc", Prompts: 6},
		{ID: "fresh", Kind: "resume-claude", Project: "projc", Prompts: 6},
	}
	differences := DiffVerified(legacy, own, rules, Verification{
		LegacyEmptyCWDIDs: map[string]struct{}{"stale": {}},
	})
	if len(differences) != 4 {
		t.Fatalf("DiffVerified() returned %d differences: %#v", len(differences), differences)
	}
	for _, difference := range differences {
		allowed := difference.Tuple.ID == "stale"
		if difference.Allowed != allowed {
			t.Fatalf("cache-empty-cwd verdict for %#v", difference)
		}
	}

	// A prompt-count disagreement is a different fault and must not ride along:
	// the pair no longer matches on count, so neither side is absolved.
	unpaired := DiffVerified(
		[]Tuple{{ID: "stale", Kind: "resume-claude", Project: "?", Prompts: 6}},
		[]Tuple{{ID: "stale", Kind: "resume-claude", Project: "projc", Prompts: 7}},
		rules,
		Verification{LegacyEmptyCWDIDs: map[string]struct{}{"stale": {}}},
	)
	if len(unpaired) != 2 {
		t.Fatalf("unpaired differences = %#v", unpaired)
	}
	for _, difference := range unpaired {
		if difference.Allowed {
			t.Fatalf("count mismatch rode the cache class: %#v", unpaired)
		}
	}
}

func TestSocketSquatterClassIsCappedByTheLiveSessionCount(t *testing.T) {
	rules, err := ParseAllowlist(strings.NewReader(
		"class\tlegacy-socket-squatter\tverified parked session\n",
	))
	if err != nil {
		t.Fatal(err)
	}
	// Three sessions are parked on a dead chat's surviving server, each in its
	// own directory, none of them named after the socket.
	legacy := []Tuple{
		{ID: "cc-1-2-3", Kind: "live-claude", Project: "projb-be", Prompts: 0},
		{ID: "cc-1-2-3", Kind: "live-claude", Project: "projb-fe", Prompts: 0},
		{ID: "cc-1-2-3", Kind: "live-claude", Project: "projb-cortex", Prompts: 0},
	}
	differences := DiffVerified(legacy, nil, rules, Verification{
		SocketSquatters: map[string]int{"cc-1-2-3": 3},
	})
	if len(differences) != 3 {
		t.Fatalf("DiffVerified() returned %d differences: %#v", len(differences), differences)
	}
	for _, difference := range differences {
		if !difference.Allowed ||
			difference.Class != "legacy-socket-squatter" ||
			difference.Reason != "verified parked session" {
			t.Fatalf("socket squatter difference = %#v", difference)
		}
	}

	// One row beyond the live session count is a row the probe cannot account
	// for, and the class must not cover it.
	overBudget := DiffVerified(legacy, nil, rules, Verification{
		SocketSquatters: map[string]int{"cc-1-2-3": 2},
	})
	unallowed := 0
	for _, difference := range overBudget {
		if !difference.Allowed {
			unallowed++
		}
	}
	if unallowed != 1 {
		t.Fatalf("over-budget squatter rows unallowed = %d: %#v", unallowed, overBudget)
	}

	for name, verification := range map[string]Verification{
		"no parked sessions": {SocketSquatters: map[string]int{}},
		"another socket": {
			SocketSquatters: map[string]int{"cc-9-9-9": 3},
		},
	} {
		t.Run(name, func(t *testing.T) {
			for _, difference := range DiffVerified(legacy, nil, rules, verification) {
				if difference.Allowed {
					t.Fatalf("unverified difference was allowlisted: %#v", difference)
				}
			}
		})
	}

	// A chat that HAS spoken is never a squatter, whatever the socket hosts.
	prompted := DiffVerified(
		[]Tuple{{
			ID: "cc-1-2-3", Kind: "live-claude", Project: "projb-be", Prompts: 4,
		}},
		nil,
		rules,
		Verification{SocketSquatters: map[string]int{"cc-1-2-3": 3}},
	)
	if len(prompted) != 1 || prompted[0].Allowed {
		t.Fatalf("prompted live row was allowlisted as a squatter: %#v", prompted)
	}
}

// TestBootingPaneClassPairsTheStructuralKindSplit is the checker half of the
// booting-row fix, covered by a unit test rather than a live repro (no
// booting-shaped chat was live during this run): a crumbless-live pane
// structurally produces one go-only "booting" tuple (Go's new Kind, keyed on
// the socket) and one legacy-only "live-claude" tuple that ReconcileIDs could
// never resolve to a real id, because no Go tuple carries kind "live-claude"
// for that chat anymore. Reasoned from check.go's own tuple construction
// (TuplesFromRows keys every row on row.Kind.String(); ReconcileIDs matches
// candidates by kind) and verified here exactly as the live checker would
// verify it — against BootingProjects, a count taken from Go's own compose
// pass, never from the diff's shape.
func TestBootingPaneClassPairsTheStructuralKindSplit(t *testing.T) {
	rules, err := ParseAllowlist(strings.NewReader(
		"class\tbooting-pane\tverified crumbless live pane\n",
	))
	if err != nil {
		t.Fatal(err)
	}
	legacy := []Tuple{
		{ID: "unresolved-display-row-1", Kind: "live-claude", Project: "projb-fe", Prompts: 0},
	}
	own := []Tuple{
		{ID: "cc-new-CC_FLEET_1", Kind: "booting", Project: "projb-fe", Prompts: 0},
	}
	differences := DiffVerified(legacy, own, rules, Verification{
		BootingProjects: map[string]int{"projb-fe": 1},
	})
	if len(differences) != 2 {
		t.Fatalf("DiffVerified() returned %d differences: %#v", len(differences), differences)
	}
	for _, difference := range differences {
		if !difference.Allowed || difference.Class != "booting-pane" {
			t.Fatalf("booting-pane verdict for %#v", difference)
		}
	}

	// A second, unrelated go-only booting row beyond the verified per-project
	// budget is a duplication regression and must stay visible.
	overBudget := DiffVerified(
		legacy,
		append(append([]Tuple(nil), own...), Tuple{
			ID: "cc-new-CC_FLEET_2", Kind: "booting", Project: "projb-fe", Prompts: 0,
		}),
		rules,
		Verification{BootingProjects: map[string]int{"projb-fe": 1}},
	)
	unallowed := 0
	for _, difference := range overBudget {
		if !difference.Allowed {
			unallowed++
		}
	}
	if unallowed != 1 {
		t.Fatalf("over-budget booting rows unallowed = %d: %#v", unallowed, overBudget)
	}

	// A legacy-only live-claude row that DID resolve to a real id is a
	// genuine miss, not the reconciliation sentinel, and must never ride
	// along even when a booting row exists in the same project.
	resolved := DiffVerified(
		[]Tuple{{ID: "real-id", Kind: "live-claude", Project: "projb-fe", Prompts: 0}},
		own,
		rules,
		Verification{BootingProjects: map[string]int{"projb-fe": 1}},
	)
	for _, difference := range resolved {
		if difference.Tuple.ID == "real-id" && difference.Allowed {
			t.Fatalf("resolved legacy row was absolved as booting: %#v", resolved)
		}
	}

	// Without the class enabled, the split stays a plain unallowlisted
	// difference on both sides.
	bare := DiffVerified(legacy, own, nil, Verification{
		BootingProjects: map[string]int{"projb-fe": 1},
	})
	for _, difference := range bare {
		if difference.Allowed {
			t.Fatalf("booting-pane absolved without the class rule: %#v", bare)
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

func TestCodexMachineSpawnedClassNeedsTheStatePredicate(t *testing.T) {
	rules, err := ParseAllowlist(strings.NewReader(
		"class\tcodex-machine-spawned\tverified machine-spawned thread\n",
	))
	if err != nil {
		t.Fatal(err)
	}

	// A legacy-only resume-codex row whose thread the state store classifies
	// as machine-spawned is absolved.
	verified := DiffVerified(
		[]Tuple{{
			ID: "worker", Kind: "resume-codex", Project: "proja", Prompts: 1,
		}},
		nil,
		rules,
		Verification{CodexMachineSpawnedIDs: map[string]struct{}{"worker": {}}},
	)
	if len(verified) != 1 ||
		!verified[0].Allowed ||
		verified[0].Class != "codex-machine-spawned" {
		t.Fatalf("machine-spawned legacy-only row = %#v", verified)
	}

	// The bound stays honest: a legacy-only row whose thread the state store
	// does NOT classify as machine-spawned (or that the store simply never
	// mentions) fails the check exactly as any other real regression would.
	unverified := DiffVerified(
		[]Tuple{{
			ID: "renamed", Kind: "resume-codex", Project: "proja", Prompts: 1,
		}},
		nil,
		rules,
		Verification{CodexMachineSpawnedIDs: map[string]struct{}{"worker": {}}},
	)
	if len(unverified) != 1 || unverified[0].Allowed {
		t.Fatalf("non-machine-spawned legacy-only row was allowlisted: %#v", unverified)
	}

	// The predicate is normalized through the lineage root, same as every
	// other Codex class: a legacy-only row named by a MEMBER id still matches
	// a machine-spawned root.
	rooted := DiffVerified(
		[]Tuple{{
			ID: "child", Kind: "resume-codex", Project: "proja", Prompts: 1,
		}},
		nil,
		rules,
		Verification{
			CodexMachineSpawnedIDs: map[string]struct{}{"root": {}},
			CodexLineageRoots:      map[string]string{"child": "root", "root": "root"},
		},
	)
	if len(rooted) != 1 || !rooted[0].Allowed {
		t.Fatalf("lineage-rooted machine-spawned row = %#v", rooted)
	}

	// The class is legacy-only by construction: a GO-only row can never ride
	// it, whatever the state store says about the thread — is_bg keeps a
	// background thread out of the DEFAULT listing, never out of --check's
	// own AllView scan, so a Go-only surplus here is a real regression.
	goOnly := DiffVerified(
		nil,
		[]Tuple{{
			ID: "worker", Kind: "resume-codex", Project: "proja", Prompts: 1,
		}},
		rules,
		Verification{CodexMachineSpawnedIDs: map[string]struct{}{"worker": {}}},
	)
	if len(goOnly) != 1 || goOnly[0].Allowed {
		t.Fatalf("go-only row rode the legacy-only machine-spawned class: %#v", goOnly)
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
