package compose

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"hostops/pfm/internal/shared"
)

func TestBuildCosmosResolvesParticipantsAndAggregatesEdges(t *testing.T) {
	rows := []Row{
		{Kind: LiveClaude, ID: "alpha-id", Name: "Alpha", Socket: "cc-alpha", PaneID: "%1"},
		{Kind: LiveCodex, ID: "beta-id", Name: "Beta", Socket: "cx-beta", PaneID: "%2"},
		{Kind: LiveClaude, ID: "child-id", Name: "Child", Socket: "cc-child", PaneID: "%3"},
		{Kind: LiveClaude, ID: "quiet-id", Name: "Quiet", Socket: "cc-quiet", PaneID: "%4"},
	}
	events := []shared.CommsEvent{
		{AtNS: 10, Kind: shared.KindInject, SenderUUID: "alpha-id", SenderLabel: "stale alpha", Target: "Beta", ReceiverSocket: "cx-beta", ReceiverPane: "%2", Message: "first"},
		{AtNS: 20, Kind: shared.KindInject, SenderSession: "alpha-id", Target: "Beta", ReceiverSocket: "cx-beta", ReceiverPane: "%2", Message: "second"},
		{AtNS: 30, Kind: shared.KindGroup, SenderLabel: "Alpha", GroupName: "crew", Target: "B*", Members: `["Beta","Gone"]`, Message: "meeting"},
		{AtNS: 40, Kind: shared.KindSpawn, SenderSession: "beta-id", Target: "Child", ReceiverSocket: "cc-child", Message: "initial prompt"},
	}

	graph := BuildCosmos(rows, events, 100)
	if graph.Err != "" || len(graph.Warnings) != 0 {
		t.Fatalf("BuildCosmos() state = err %q warnings %v", graph.Err, graph.Warnings)
	}
	wantNodes := []CosmosNode{
		{Key: "chat:id:alpha-id", Label: "Alpha", Engine: "cc", Live: true, LastNS: 30},
		{Key: "chat:id:beta-id", Label: "Beta", Engine: "cx", Live: true, LastNS: 40},
		{Key: "chat:id:child-id", Label: "Child", Engine: "cc", Live: true, LastNS: 40},
		{Key: "group:crew", Label: "crew", Group: true, LastNS: 30},
		{Key: "chat:name:Gone", Label: "Gone", LastNS: 30},
	}
	if !reflect.DeepEqual(graph.Nodes, wantNodes) {
		t.Fatalf("nodes = %#v, want %#v", graph.Nodes, wantNodes)
	}
	wantEdges := []CosmosEdge{
		{From: "chat:id:beta-id", To: "chat:id:child-id", Kind: shared.KindSpawn, Count: 1, LastNS: 40, LastMessage: "initial prompt"},
		{From: "chat:id:alpha-id", To: "group:crew", Kind: shared.KindGroup, Count: 1, LastNS: 30, LastMessage: "meeting"},
		{From: "group:crew", To: "chat:id:beta-id", Kind: shared.KindGroup, Count: 1, LastNS: 30, LastMessage: "meeting"},
		{From: "group:crew", To: "chat:name:Gone", Kind: shared.KindGroup, Count: 1, LastNS: 30, LastMessage: "meeting"},
		{From: "chat:id:alpha-id", To: "chat:id:beta-id", Kind: shared.KindInject, Count: 2, LastNS: 20, LastMessage: "second"},
	}
	if !reflect.DeepEqual(graph.Edges, wantEdges) {
		t.Fatalf("edges = %#v, want %#v", graph.Edges, wantEdges)
	}
	for _, node := range graph.Nodes {
		if node.Label == "Quiet" {
			t.Fatal("traffic-free live row became a cosmos node")
		}
	}
}

func TestBuildCosmosMalformedGroupMembersKeepsTruthfulPartialGraph(t *testing.T) {
	graph := BuildCosmos(nil, []shared.CommsEvent{{
		ID: 7, AtNS: 50, Kind: shared.KindGroup, SenderLabel: "Alpha",
		GroupName: "crew", Members: `["Beta"`, Message: "partial",
	}}, 100)
	if len(graph.Warnings) != 1 || !strings.Contains(graph.Warnings[0], "event 7") {
		t.Fatalf("warnings = %v", graph.Warnings)
	}
	want := []CosmosEdge{{
		From: "chat:name:Alpha", To: "group:crew", Kind: shared.KindGroup,
		Count: 1, LastNS: 50, LastMessage: "partial",
	}}
	if !reflect.DeepEqual(graph.Edges, want) {
		t.Fatalf("malformed-member edges = %#v, want %#v", graph.Edges, want)
	}
}

func TestBuildCosmosParentlessSpawnCreatesOnlyTheChildNode(t *testing.T) {
	graph := BuildCosmos(nil, []shared.CommsEvent{{
		AtNS: 60, Kind: shared.KindSpawn, Target: "Orphan",
		ReceiverSocket: "cx-orphan", Message: "born",
	}}, 100)
	if len(graph.Edges) != 0 {
		t.Fatalf("parentless spawn edges = %#v", graph.Edges)
	}
	want := []CosmosNode{{
		Key: "chat:session:cx-orphan", Label: "Orphan", LastNS: 60,
	}}
	if !reflect.DeepEqual(graph.Nodes, want) {
		t.Fatalf("parentless nodes = %#v, want %#v", graph.Nodes, want)
	}
}

func TestBuildCosmosTimestampTieKeepsNewestFirstMessage(t *testing.T) {
	graph := BuildCosmos(nil, []shared.CommsEvent{
		{ID: 2, AtNS: 70, Kind: shared.KindInject, SenderLabel: "Alpha", Target: "Beta", Message: "newest id"},
		{ID: 1, AtNS: 70, Kind: shared.KindInject, SenderLabel: "Alpha", Target: "Beta", Message: "older id"},
	}, 100)
	want := []CosmosEdge{{
		From: "chat:name:Alpha", To: "chat:name:Beta", Kind: shared.KindInject,
		Count: 2, LastNS: 70, LastMessage: "newest id",
	}}
	if !reflect.DeepEqual(graph.Edges, want) {
		t.Fatalf("tie edges = %#v, want %#v", graph.Edges, want)
	}
}

func TestBuildCosmosUnknownKindWarnsInsteadOfVanishingSilently(t *testing.T) {
	graph := BuildCosmos(nil, []shared.CommsEvent{{ID: 9, AtNS: 80, Kind: "reload"}}, 100)
	if len(graph.Nodes) != 0 || len(graph.Edges) != 0 || len(graph.Warnings) != 1 ||
		!strings.Contains(graph.Warnings[0], `event 9`) ||
		!strings.Contains(graph.Warnings[0], `unknown kind "reload"`) {
		t.Fatalf("unknown-kind graph = %#v", graph)
	}
}

// TestBuildCosmosDeadChatConvergesSpawnAndInjectOnOneGhostNode is the
// regression for the three-nodes-per-chat bug: a spawn event recorded the
// receiver's tmux session as a BARE NAME (run_command.go:152's pre-fix
// shape) while an inject event for the very same chat, once it has no
// surviving row to resolve against (it died), states its own identity as
// that SAME bare session name via sender_session. Before the fix these two
// facts lived in two different fallback key spaces (chat:pane:<bare-name>\t
// vs chat:id:<session>) and produced two ghosts for one chat — plus a third
// whenever a live-shaped full socket path was also seen. This asserts they
// converge on exactly one.
func TestBuildCosmosDeadChatConvergesSpawnAndInjectOnOneGhostNode(t *testing.T) {
	const deadSession = "cc-1787827285-1466858-781"
	events := []shared.CommsEvent{
		{
			AtNS: 10, Kind: shared.KindSpawn, SenderSession: "parent-session",
			Target: "ghost", ReceiverSocket: deadSession, // bare session name: the pre-fix writer's shape
			Message: "spawned",
		},
		{
			AtNS: 20, Kind: shared.KindInject, SenderSession: deadSession,
			Target: "someone-else", ReceiverSocket: "/tmp/tmux-1000/other-session", ReceiverPane: "%0",
			Message: "the dead chat's last reply",
		},
	}
	// No rows at all: the chat has fully vanished from the roster, exactly
	// the state the operator described after killing ten test chats.
	graph := BuildCosmos(nil, events, 100)
	var matches []CosmosNode
	for _, node := range graph.Nodes {
		if strings.Contains(node.Key, deadSession) {
			matches = append(matches, node)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("dead chat %q produced %d node(s), want exactly 1\nmatches = %#v\nfull graph.Nodes = %#v",
			deadSession, len(matches), matches, graph.Nodes)
	}
}

// TestBuildCosmosLiveChatStillYieldsOneNodeAcrossSpawnAndInject is the
// non-regression half: a chat with a surviving row must still resolve to
// exactly one node through both a spawn and an inject event, so the
// fallback fix above cannot be mistaken for a change to the live path.
func TestBuildCosmosLiveChatStillYieldsOneNodeAcrossSpawnAndInject(t *testing.T) {
	rows := []Row{
		{Kind: LiveClaude, ID: "ghost-id", Name: "ghost", Socket: "/tmp/tmux-1000/cc-1787827285-1466858-781", PaneID: "%0"},
	}
	events := []shared.CommsEvent{
		{
			AtNS: 10, Kind: shared.KindSpawn, SenderSession: "parent-session",
			Target: "ghost", ReceiverSocket: "/tmp/tmux-1000/cc-1787827285-1466858-781", ReceiverPane: "%0",
			Message: "spawned",
		},
		{
			AtNS: 20, Kind: shared.KindInject, SenderUUID: "ghost-id",
			Target: "someone-else", ReceiverSocket: "/tmp/tmux-1000/other-session", ReceiverPane: "%0",
			Message: "reply",
		},
	}
	graph := BuildCosmos(rows, events, 100)
	count := 0
	for _, node := range graph.Nodes {
		if node.Live && node.Label == "ghost" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("live chat produced %d live node(s) labeled ghost, want 1\ngraph.Nodes = %#v", count, graph.Nodes)
	}
}

// TestBuildCosmosDoesNotPruneDeadNodesByTime documents the current division
// of labor, corrected from an earlier version of this package that pruned a
// dead node here once nowNS - DiedNS exceeded the blink+fade window:
// BuildCosmos has no memory across calls, so DiedNS (last edge activity) is
// the only fact it can honestly report, and that fact cannot answer "when
// did it die" — a chat killed after a long idle period would already be
// past any window BuildCosmos could check on the very first call that
// notices it, pruning it before anything downstream ever had a chance to
// animate it. Deciding how long a dead node stays visible after it is first
// OBSERVED dead is therefore not this package's job: model.applyCosmosGraph
// (internal/ui) owns it, using a detection timestamp only a caller with
// memory between renders can produce. A dead node, and every edge naming
// it, survive here regardless of age.
func TestBuildCosmosDoesNotPruneDeadNodesByTime(t *testing.T) {
	events := []shared.CommsEvent{
		{
			AtNS: 0, Kind: shared.KindInject, SenderLabel: "Ghost",
			Target: "Someone", Message: "long gone",
		},
	}
	// Ten hours past a zero-timestamp event would have been many orders of
	// magnitude past CosmosDeathBlink+CosmosDeathFade under the old
	// nowNS-based prune. It changes nothing here now.
	graph := BuildCosmos(nil, events, int64(10*time.Hour))
	if len(graph.Nodes) != 2 || len(graph.Edges) != 1 {
		t.Fatalf("BuildCosmos dropped a dead node or its edge by age: nodes=%#v edges=%#v", graph.Nodes, graph.Edges)
	}
	for _, node := range graph.Nodes {
		if !node.Dead {
			t.Fatalf("node %#v should be Dead: no row survives for either fallback identity", node)
		}
		if node.DiedNS != 0 {
			t.Fatalf("node %#v DiedNS should be 0 (its only touch was AtNS 0), not a fabricated later time", node)
		}
	}
}

// TestBuildCosmosNoRowMatchIsPendingWithinGraceThenDead pins the boundary
// of the grace window itself: a node with no matching row is PENDING
// (Dead false) while nowNS - LastNS is within CosmosDeathBlink+
// CosmosDeathFade — long enough for the row layer to index a freshly
// spawned chat — and only becomes Dead once that grace has elapsed. A
// matched row's Dead (row.Killed) is positive evidence and does not wait;
// this boundary only applies to the ambiguous no-row-match case.
func TestBuildCosmosNoRowMatchIsPendingWithinGraceThenDead(t *testing.T) {
	events := []shared.CommsEvent{{
		AtNS: 0, Kind: shared.KindInject, SenderLabel: "Newborn",
		Target: "Someone", Message: "hi",
	}}
	grace := int64(CosmosDeathBlink + CosmosDeathFade)

	pending := BuildCosmos(nil, events, grace)
	pendingFound := false
	for _, node := range pending.Nodes {
		if node.Key != "chat:name:Newborn" {
			continue
		}
		pendingFound = true
		if node.Dead {
			t.Fatalf("node exactly at the grace boundary should still be PENDING: %#v", node)
		}
	}
	if !pendingFound {
		t.Fatal("pending node missing from the graph entirely")
	}

	gone := BuildCosmos(nil, events, grace+1)
	goneFound := false
	for _, node := range gone.Nodes {
		if node.Key != "chat:name:Newborn" {
			continue
		}
		goneFound = true
		if !node.Dead {
			t.Fatalf("node one tick past the grace window should be Dead: %#v", node)
		}
	}
	if !goneFound {
		t.Fatal("dead node missing from the graph entirely")
	}
}
