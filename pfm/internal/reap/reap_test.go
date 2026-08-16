package reap

import (
	"testing"
	"time"
)

// The reaper's whole contract is a table: one socket, one verdict, and an
// action ONLY where killing is both asked for and safe. Every rule that keeps
// a chat alive is here, because each of them was paid for by a chat somebody
// lost.
func TestPlanClassifiesEverySocket(t *testing.T) {
	const busyUUID = "11111111-1111-4111-8111-111111111111"
	live := Socket{Name: "cc-100-1-1", HasServer: true, HasCrumb: true}

	cases := []struct {
		name   string
		input  Input
		want   State
		action Action
	}{
		{
			name: "attached chats are never candidates",
			input: Input{
				AgentsOK: true,
				Apply:    true,
				Sockets:  []Socket{{Name: "cc-100-1-1", HasServer: true, Attached: true, HasCrumb: true}},
			},
			want:   StateKeep,
			action: ActionNone,
		},
		{
			name: "the caller's own socket is never a candidate",
			input: Input{
				AgentsOK: true,
				Apply:    true,
				Self:     "cc-100-1-1",
				Sockets:  []Socket{live},
			},
			want:   StateSelf,
			action: ActionNone,
		},
		{
			name: "a detached teammate is headless by design",
			input: Input{
				AgentsOK: true,
				Apply:    true,
				Sockets:  []Socket{{Name: "cc-new-worker", HasServer: true}},
			},
			want:   StateMate,
			action: ActionNone,
		},
		{
			name: "a session the engine calls busy survives a sweep",
			input: Input{
				AgentsOK: true,
				Apply:    true,
				BusyIDs:  map[string]struct{}{busyUUID: {}},
				Sockets: []Socket{{
					Name:       "cc-100-1-1",
					HasServer:  true,
					HasCrumb:   true,
					CrumbUUIDs: []string{busyUUID},
				}},
			},
			want:   StateBusy,
			action: ActionNone,
		},
		{
			name: "a transcript written moments ago outranks a stale busy snapshot",
			input: Input{
				AgentsOK:   true,
				Apply:      true,
				BusyRecent: time.Minute,
				RecentIDs:  map[string]struct{}{busyUUID: {}},
				Sockets: []Socket{{
					Name:       "cc-100-1-1",
					HasServer:  true,
					HasCrumb:   true,
					CrumbUUIDs: []string{busyUUID},
				}},
			},
			want:   StateActive,
			action: ActionNone,
		},
		{
			name: "a socket hosting non-chat work is load-bearing",
			input: Input{
				AgentsOK: true,
				Apply:    true,
				Sockets: []Socket{{
					Name:      "cx-100-1-1",
					HasServer: true,
					Foreign:   []string{"node", "uv"},
				}},
			},
			want:   StateHosts,
			action: ActionNone,
		},
		{
			name: "an unattached idle chat is the reapable case",
			input: Input{
				AgentsOK: true,
				Apply:    true,
				Sockets:  []Socket{live},
			},
			want:   StateOrphan,
			action: ActionKillServer,
		},
		{
			name: "without --apply nothing is ever actioned",
			input: Input{
				AgentsOK: true,
				Sockets:  []Socket{live},
			},
			want:   StateOrphan,
			action: ActionNone,
		},
		{
			name: "a failed busy query skips every chat carrying a breadcrumb",
			input: Input{
				Apply:   true,
				Sockets: []Socket{live},
			},
			want:   StateSkip,
			action: ActionNone,
		},
		{
			name: "a claude socket with no breadcrumb is busy-unknown",
			input: Input{
				AgentsOK: true,
				Apply:    true,
				Sockets:  []Socket{{Name: "cc-100-1-1", HasServer: true}},
			},
			want:   StateSkip,
			action: ActionNone,
		},
		{
			name: "a codex socket writes no breadcrumb by design and stays reapable",
			input: Input{
				AgentsOK: true,
				Apply:    true,
				Sockets:  []Socket{{Name: "cx-100-1-1", HasServer: true}},
			},
			want:   StateOrphan,
			action: ActionKillServer,
		},
		{
			name: "an old socket file with no server is a corpse",
			input: Input{
				AgentsOK:  true,
				Apply:     true,
				DeadAfter: time.Hour,
				Sockets:   []Socket{{Name: "cc-100-1-1", Age: 3 * time.Hour}},
			},
			want:   StateDead,
			action: ActionRemoveSocketFile,
		},
		{
			name: "a young empty socket may be a server still starting",
			input: Input{
				AgentsOK:  true,
				Apply:     true,
				DeadAfter: time.Hour,
				Sockets:   []Socket{{Name: "cc-100-1-1", Age: time.Minute}},
			},
			want:   StateSkip,
			action: ActionNone,
		},
		{
			name: "a probe that could not run is not an empty answer",
			input: Input{
				AgentsOK: true,
				Apply:    true,
				Sockets: []Socket{{
					Name:        "cc-100-1-1",
					ProbeFailed: true,
					ProbeError:  "permission denied",
					Age:         72 * time.Hour,
				}},
			},
			want:   StateSkip,
			action: ActionNone,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			decisions := Plan(testCase.input)
			if len(decisions) != 1 {
				t.Fatalf("Plan() returned %d decisions, want one", len(decisions))
			}
			if decisions[0].State != testCase.want {
				t.Fatalf(
					"state = %q (%s), want %q",
					decisions[0].State,
					decisions[0].Reason,
					testCase.want,
				)
			}
			if decisions[0].Action != testCase.action {
				t.Fatalf(
					"action = %d, want %d (%s)",
					decisions[0].Action,
					testCase.action,
					decisions[0].Reason,
				)
			}
		})
	}
}

// Age is a corpse test, never a staleness test: a chat quiet for a month is
// still a chat, and the only thing its socket's age may decide is whether an
// EMPTY socket is dead or still starting.
func TestPlanNeverReapsFromAgeAlone(t *testing.T) {
	decisions := Plan(Input{
		AgentsOK:  true,
		Apply:     true,
		DeadAfter: time.Hour,
		Sockets: []Socket{{
			Name:      "cc-100-1-1",
			Age:       30 * 24 * time.Hour,
			HasServer: true,
			Attached:  true,
			HasCrumb:  true,
		}},
	})
	if decisions[0].State != StateKeep || decisions[0].Action != ActionNone {
		t.Fatalf(
			"a month-old ATTACHED chat was classified %q/%d",
			decisions[0].State,
			decisions[0].Action,
		)
	}
}

// The dry run must be a true preview: the shell original reported fail-closed
// skips as plain orphans, so its preview promised kills the reap never made.
func TestDryRunPreviewsExactlyWhatApplyWouldDo(t *testing.T) {
	sockets := []Socket{
		{Name: "cc-100-1-1", HasServer: true},
		{Name: "cc-200-1-1", HasServer: true, HasCrumb: true},
		{Name: "cx-300-1-1", HasServer: true, Foreign: []string{"vite"}},
	}
	preview := Plan(Input{AgentsOK: true, Sockets: sockets})
	applied := Plan(Input{AgentsOK: true, Apply: true, Sockets: sockets})
	if len(preview) != len(applied) {
		t.Fatalf("preview has %d rows, apply has %d", len(preview), len(applied))
	}
	for index := range preview {
		if preview[index].State != applied[index].State {
			t.Fatalf(
				"%s previewed as %q but applies as %q",
				preview[index].Socket,
				preview[index].State,
				applied[index].State,
			)
		}
		if preview[index].Action != ActionNone {
			t.Fatalf("dry run planned action %d on %s",
				preview[index].Action, preview[index].Socket)
		}
	}
}

// The bunker socket is shared: one idle session is killed by NAME, never by
// killing the server every other terminal is living on.
func TestPlanSweepsBunkerSessionsIndividually(t *testing.T) {
	decisions := Plan(Input{
		AgentsOK:    true,
		Apply:       true,
		VSCTMaxIdle: 7 * 24 * time.Hour,
		VSCT: []VSCTSession{
			{Name: "projc-1", Attached: true, Idle: 30 * 24 * time.Hour},
			{Name: "proja-2", Idle: time.Hour},
			{Name: "old-3", Idle: 30 * 24 * time.Hour},
		},
	})
	want := map[string]struct {
		state  State
		action Action
	}{
		"vsct:projc-1": {StateKeep, ActionNone},
		"vsct:proja-2": {StateKeep, ActionNone},
		"vsct:old-3":     {StateOrphan, ActionKillSession},
	}
	for _, decision := range decisions {
		expected, found := want[decision.Socket]
		if !found {
			t.Fatalf("unexpected row %q", decision.Socket)
		}
		if decision.State != expected.state || decision.Action != expected.action {
			t.Fatalf(
				"%s = %q/%d, want %q/%d",
				decision.Socket,
				decision.State,
				decision.Action,
				expected.state,
				expected.action,
			)
		}
	}
}
