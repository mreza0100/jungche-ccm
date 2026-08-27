package main

import (
	"fmt"
	"testing"
)

// staticLineage answers from a fixed id→root table. A missing id answers with
// itself (a thread that is its own root), which is what the production
// resolver does; only brokenLineage returns "" for "the rollouts could not be
// read at all".
func staticLineage(roots map[string]string) func(string) string {
	return func(id string) string {
		if id == "" {
			return ""
		}
		if root, found := roots[id]; found {
			return root
		}
		return id
	}
}

func brokenLineage(string) string { return "" }

// nothingRetired is the ordinary case: the kill table read fine and holds
// nothing relevant.
func nothingRetired(string) (bool, bool) { return false, true }

// retiredThreads answers from a fixed set, and unknownRetirement is the store
// outage — the answer that must never let a binding be dropped.
func retiredThreads(ids ...string) codexThreadRetired {
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return func(id string) (bool, bool) { return set[id], true }
}

func unknownRetirement(string) (bool, bool) { return false, false }

func onePaneAction(t *testing.T, actions []codexPaneAction) codexPaneAction {
	t.Helper()
	if len(actions) != 1 {
		t.Fatalf("decideCodexPanes returned %d actions, want 1", len(actions))
	}
	return actions[0]
}

// The decision table. Every row is one live pane and the single ruling it must
// produce — the branch coverage that the tmux-backed jail tests cannot afford
// to enumerate one real server at a time.
func TestDecideCodexPaneRulings(t *testing.T) {
	const (
		bound   = "11111111-1111-4111-8111-111111111111"
		fresh   = "22222222-2222-4222-8222-222222222222"
		sibling = "33333333-3333-4333-8333-333333333333"
	)
	for _, test := range []struct {
		name        string
		observation codexPaneObservation
		names       map[string]string
		lineage     func(string) string
		retired     codexThreadRetired
		wantBind    string
		wantKill    string
		wantSkip    string
		wantLoud    bool
	}{
		{
			name:        "bare id on an unbound pane seeds without retiring anything",
			observation: codexPaneObservation{Socket: "cx-a", PaneID: "%0", ThreadID: fresh},
			wantBind:    fresh,
		},
		{
			name:        "bare id equal to the binding is a no-op",
			observation: codexPaneObservation{Socket: "cx-a", PaneID: "%0", ThreadID: bound, Bound: bound},
		},
		{
			name:        "a bare id that replaced another lineage IS the clear",
			observation: codexPaneObservation{Socket: "cx-a", PaneID: "%0", ThreadID: fresh, Bound: bound},
			wantBind:    fresh,
			wantKill:    bound,
		},
		{
			name:        "a resume in the same lineage is never a clear",
			observation: codexPaneObservation{Socket: "cx-a", PaneID: "%0", ThreadID: fresh, Bound: bound},
			lineage:     staticLineage(map[string]string{fresh: bound}),
			wantBind:    fresh,
			wantSkip:    codexPaneSameLineage,
		},
		{
			name:        "an unreadable lineage advances the binding but never kills",
			observation: codexPaneObservation{Socket: "cx-a", PaneID: "%0", ThreadID: fresh, Bound: bound},
			lineage:     brokenLineage,
			wantBind:    fresh,
			wantSkip:    codexPaneLineageUnknown,
			wantLoud:    true,
		},
		{
			name:        "a failed capture is loud and touches nothing",
			observation: codexPaneObservation{Socket: "cx-a", PaneID: "%0", Failed: true},
			wantSkip:    codexPaneCaptureFailed,
			wantLoud:    true,
		},
		{
			name:        "a name that confirms the binding is silent",
			observation: codexPaneObservation{Socket: "cx-a", PaneID: "%0", Name: "GW", Bound: bound},
			names:       map[string]string{bound: "GW"},
		},
		{
			name:        "a name resolving to a DIFFERENT thread never moves the binding",
			observation: codexPaneObservation{Socket: "cx-a", PaneID: "%0", Name: "GW", Bound: fresh},
			names:       map[string]string{bound: "GW"},
			wantSkip:    codexPaneNameCannotMove,
		},
		{
			name:        "a unique name seeds an unbound pane",
			observation: codexPaneObservation{Socket: "cx-a", PaneID: "%0", Name: "GW"},
			names:       map[string]string{bound: "GW"},
			wantBind:    bound,
		},
		{
			name:        "a name nothing indexes is recorded but not shouted",
			observation: codexPaneObservation{Socket: "cx-a", PaneID: "%0", Name: "FIX_HAND"},
			wantSkip:    codexPaneNameUnknown,
		},
		{
			name:        "an ambiguous name seeds nothing",
			observation: codexPaneObservation{Socket: "cx-a", PaneID: "%0", Name: "GW"},
			names:       map[string]string{bound: "GW", sibling: "GW"},
			wantSkip:    codexPaneNameAmbiguous,
		},
		{
			name:        "a screen naming no thread at all is reported, not assumed idle",
			observation: codexPaneObservation{Socket: "cx-a", PaneID: "%0"},
			wantSkip:    codexPaneNoThreadNamed,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			lineage := test.lineage
			if lineage == nil {
				lineage = staticLineage(nil)
			}
			retired := test.retired
			if retired == nil {
				retired = nothingRetired
			}
			action := onePaneAction(t, decideCodexPanes(
				[]codexPaneObservation{test.observation}, test.names, lineage, retired,
			))
			if action.Bind != test.wantBind {
				t.Errorf("Bind = %q, want %q", action.Bind, test.wantBind)
			}
			if action.ClearKill != test.wantKill {
				t.Errorf("ClearKill = %q, want %q", action.ClearKill, test.wantKill)
			}
			if action.Skip != test.wantSkip {
				t.Errorf("Skip = %q, want %q", action.Skip, test.wantSkip)
			}
			if action.Loud != test.wantLoud {
				t.Errorf("Loud = %v, want %v", action.Loud, test.wantLoud)
			}
		})
	}
}

// The live-fleet defect, replayed as the timeline that produced it. Two panes
// carried the display name ENGINE_BUILDER; one thread carried it in cx_names;
// both panes ended up bound to that one thread, and a /clear had already
// retired it. `pfm chat resolve ENGINE_BUILDER` then answered with the corpse.
//
// A name may seed at most ONE pane. The second pane keeps nothing it did not
// earn, and says so out loud.
func TestDecideCodexPanesNeverSeedsTwoPanesOntoOneThread(t *testing.T) {
	const shared = "01a02dca-c83c-7871-bdf1-461c75441c77"
	actions := decideCodexPanes(
		[]codexPaneObservation{
			{Socket: "cx-first", PaneID: "%0", Name: "ENGINE_BUILDER"},
			{Socket: "cx-second", PaneID: "%0", Name: "ENGINE_BUILDER"},
		},
		map[string]string{shared: "ENGINE_BUILDER"},
		staticLineage(nil), nothingRetired,
	)
	if len(actions) != 2 {
		t.Fatalf("got %d actions, want 2", len(actions))
	}
	bindings := 0
	for _, action := range actions {
		if action.Bind == shared {
			bindings++
		}
	}
	if bindings != 1 {
		t.Fatalf("%d panes were bound to one thread, want exactly 1", bindings)
	}
}

// The same collision from the other direction: a pane ALREADY bound to the
// thread, and a second pane whose only claim is the shared display name. The
// incumbent keeps it; the newcomer is refused, loudly, because a fleet in this
// state is mis-following something and silence is how it stayed that way.
func TestDecideCodexPanesRefusesToSeedOntoAClaimedThread(t *testing.T) {
	const shared = "01a02dca-c83c-7871-bdf1-461c75441c77"
	actions := decideCodexPanes(
		[]codexPaneObservation{
			{Socket: "cx-incumbent", PaneID: "%0", Name: "ENGINE_BUILDER", Bound: shared},
			{Socket: "cx-newcomer", PaneID: "%0", Name: "ENGINE_BUILDER"},
		},
		map[string]string{shared: "ENGINE_BUILDER"},
		staticLineage(nil), nothingRetired,
	)
	if actions[0].Bind != "" || actions[0].Skip != "" {
		t.Fatalf("incumbent was disturbed: %+v", actions[0])
	}
	if actions[1].Bind != "" {
		t.Fatalf("newcomer stole the thread: bind=%q", actions[1].Bind)
	}
	if actions[1].Skip != codexPaneNameTaken || !actions[1].Loud {
		t.Fatalf("newcomer refusal = (%q, loud=%v), want (%q, loud=true)",
			actions[1].Skip, actions[1].Loud, codexPaneNameTaken)
	}
}

// The whole /clear timeline in order, which is where the old design came
// apart. The pane clears, pfm retires the old thread and renames the new one,
// and for one or more passes the status line shows a NAME that cx_names still
// maps only to the thread that just died. Every pass in that window must leave
// the binding where it is; nothing may be killed twice; and once the index
// catches up the answer must not change.
func TestDecideCodexPanesClearTimelineNeverWalksBackwards(t *testing.T) {
	const (
		before = "01a03e02-50ac-7582-9202-2e626f203944"
		after  = "01a03ea6-8276-7141-b1ff-1a813901371a"
		chat   = "W5_TESTER"
	)
	socket, pane := "cx-1787757492-3196324-4837", "%0"
	names := map[string]string{before: chat}
	binding := before
	kills := make([]string, 0, 2)

	step := func(label string, observed codexPaneObservation) codexPaneAction {
		t.Helper()
		observed.Socket, observed.PaneID, observed.Bound = socket, pane, binding
		action := onePaneAction(t, decideCodexPanes(
			[]codexPaneObservation{observed}, names, staticLineage(nil), nothingRetired,
		))
		if action.Bind != "" {
			binding = action.Bind
		}
		if action.ClearKill != "" {
			kills = append(kills, action.ClearKill)
		}
		t.Logf("%s: bind=%q kill=%q skip=%q", label, action.Bind, action.ClearKill, action.Skip)
		return action
	}

	// 1. Steady state: the pane shows its name, bound to the thread that owns it.
	step("steady", codexPaneObservation{Name: chat})
	if binding != before {
		t.Fatalf("steady state moved the binding to %q", binding)
	}

	// 2. /clear. The new thread is unnamed, so the pane shows a bare id.
	action := step("cleared", codexPaneObservation{ThreadID: after})
	if action.ClearKill != before || binding != after {
		t.Fatalf("clear was not detected: kill=%q binding=%q", action.ClearKill, binding)
	}

	// 3. pfm re-applies the chat name. cx_names has NOT caught up yet, so the
	//    only thread carrying this name is the one that just died.
	for pass := 1; pass <= 3; pass++ {
		action = step(fmt.Sprintf("lagging pass %d", pass), codexPaneObservation{Name: chat})
		if action.Bind != "" || action.ClearKill != "" {
			t.Fatalf("pass %d acted on a lagging name: %+v", pass, action)
		}
		if action.Skip != codexPaneNameCannotMove {
			t.Fatalf("pass %d skip = %q, want %q", pass, action.Skip, codexPaneNameCannotMove)
		}
	}

	// 4. The index catches up: both threads now carry the name.
	names[after] = chat
	step("index caught up", codexPaneObservation{Name: chat})

	if binding != after {
		t.Fatalf("binding ended on %q, want the post-clear thread %q", binding, after)
	}
	if len(kills) != 1 || kills[0] != before {
		t.Fatalf("kills = %v, want exactly [%s]", kills, before)
	}
}

// The state a real fleet was found in, and the reason it could never get out
// of it: two panes bound to a thread a /clear had already retired. The pane
// shows a NAME from then on, and a name may not move a binding — so without an
// explicit repair the fleet follows that pane into a dead chat forever.
//
// The binding is impossible, so it is dropped. Nothing is invented in its
// place: an unbound pane is the honest answer, and the pane's own screen
// re-seats it the moment it shows a bare id.
func TestDecideCodexPanesDropsABindingOnAClearRetiredThread(t *testing.T) {
	const dead = "01a02dca-c83c-7871-bdf1-461c75441c77"
	action := onePaneAction(t, decideCodexPanes(
		[]codexPaneObservation{
			{Socket: "cx-a", PaneID: "%0", Name: "ENGINE_BUILDER", Bound: dead},
		},
		map[string]string{dead: "ENGINE_BUILDER"},
		staticLineage(nil),
		retiredThreads(dead),
	))
	if action.Bind != "" {
		t.Fatalf("a retired thread was re-seeded: %+v", action)
	}
	if !action.Forget {
		t.Fatalf("the impossible binding was left in place: %+v", action)
	}
	if !action.Loud {
		t.Fatalf("the repair was silent: %+v", action)
	}
}

// The other half of the same repair: the drop must not be undone by the seed
// path re-attaching the pane to the very thread it just let go of. That would
// churn the store once per gather pass and change nothing.
func TestDecideCodexPanesNeverSeedsOntoARetiredThread(t *testing.T) {
	const dead = "01a02dca-c83c-7871-bdf1-461c75441c77"
	action := onePaneAction(t, decideCodexPanes(
		[]codexPaneObservation{{Socket: "cx-a", PaneID: "%0", Name: "ENGINE_BUILDER"}},
		map[string]string{dead: "ENGINE_BUILDER"},
		staticLineage(nil),
		retiredThreads(dead),
	))
	if action.Bind != "" {
		t.Fatalf("seeded onto a retired thread: %+v", action)
	}
	// Quiet on purpose: this is a standing structural condition, not an
	// event, and it cannot self-heal — a Loud line here would repeat on
	// every reconcile pass forever. `pfm doctor` carries it instead.
	if action.Skip != codexPaneNameRetired || action.Loud {
		t.Fatalf("skip = (%q, loud=%v), want (%q, loud=false)", action.Skip, action.Loud, codexPaneNameRetired)
	}
}

// A kill table that could not be READ is not proof that a thread is dead. The
// repair is destructive — it erases a binding — so an unreadable table must
// leave every binding exactly where it is.
func TestDecideCodexPanesKeepsBindingsWhenRetirementIsUnknowable(t *testing.T) {
	const bound = "01a02dca-c83c-7871-bdf1-461c75441c77"
	action := onePaneAction(t, decideCodexPanes(
		[]codexPaneObservation{
			{Socket: "cx-a", PaneID: "%0", Name: "ENGINE_BUILDER", Bound: bound},
		},
		map[string]string{bound: "ENGINE_BUILDER"},
		staticLineage(nil),
		unknownRetirement,
	))
	if action.Forget {
		t.Fatalf("an unreadable kill table erased a live binding: %+v", action)
	}
	if action.Bind != "" || action.Skip != "" {
		t.Fatalf("an unreadable kill table changed the ruling: %+v", action)
	}
}

// An explicit `pfm chat kill` hides a chat that is still running in its pane.
// That binding is correct and must survive: only a /clear retirement (a
// prompt-baseline kill) proves the pane moved on.
func TestDecideCodexPanesKeepsABindingOnAnExplicitlyKilledChat(t *testing.T) {
	const bound = "01a02dca-c83c-7871-bdf1-461c75441c77"
	action := onePaneAction(t, decideCodexPanes(
		[]codexPaneObservation{
			{Socket: "cx-a", PaneID: "%0", Name: "ENGINE_BUILDER", Bound: bound},
		},
		map[string]string{bound: "ENGINE_BUILDER"},
		staticLineage(nil),
		// An explicit kill carries no baseline, so the oracle reports it as
		// NOT clear-retired.
		func(string) (bool, bool) { return false, true },
	))
	if action.Forget || action.Bind != "" || action.Skip != "" {
		t.Fatalf("an explicit kill disturbed a live binding: %+v", action)
	}
}
