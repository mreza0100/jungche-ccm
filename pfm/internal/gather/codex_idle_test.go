package gather

import (
	"testing"
)

func TestRefreshCodexHeldRolloutsDropsStaleClaimsAndTracksRotation(t *testing.T) {
	const oldPath = "/jail/codex/sessions/rollout-old.jsonl"
	const newPath = "/jail/codex/sessions/rollout-new.jsonl"
	previous := []LiveCodex{{PID: 400, RolloutPath: oldPath, ThreadID: "old", RolloutHeld: true}, {PID: 999, RolloutPath: oldPath, RolloutHeld: true}}
	for _, test := range []struct {
		name  string
		links []FDLink
		want  string
	}{
		{"closed", nil, ""},
		{"rotated", []FDLink{{FD: 4, Target: newPath}}, newPath},
		{"foreign", []FDLink{{FD: 4, Target: "/jail/foreign/sessions/rollout-other.jsonl"}}, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			proc := &fakeProcFS{processes: map[int]fakeProcess{400: {cmdline: []string{"codex"}, fdLinks: test.links}}}
			got, err := RefreshCodexHeldRollouts(proc, previous, []string{"/jail/codex"})
			if err != nil || len(got) != 1 {
				t.Fatalf("refresh=%#v err=%v", got, err)
			}
			if got[0].RolloutPath != test.want || got[0].RolloutHeld != (test.want != "") || got[0].ThreadID != CodexRolloutID(test.want) {
				t.Fatalf("stale or incorrect claim: %#v", got[0])
			}
			if previous[0].RolloutPath != oldPath || !previous[0].RolloutHeld {
				t.Fatal("mutated old snapshot")
			}
		})
	}
}
