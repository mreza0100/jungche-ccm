package compose

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"hostops/pfm/internal/resolve"
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
		{Key: "chat:id:alpha-id", Label: resolve.Named("Alpha"), Engine: "cc", Live: true, LastNS: 30},
		{Key: "chat:id:beta-id", Label: resolve.Named("Beta"), Engine: "cx", Live: true, LastNS: 40},
		{Key: "chat:id:child-id", Label: resolve.Named("Child"), Engine: "cc", Live: true, LastNS: 40},
		{Key: "group:crew", Label: resolve.Named("crew"), Group: true, LastNS: 30},
		{Key: "chat:name:Gone", Label: resolve.Named("Gone"), LastNS: 30},
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
		if nodeLabel(node) == "Quiet" {
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
		Key: "chat:session:cx-orphan", Label: resolve.Named("Orphan"), LastNS: 60,
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
		if node.Live && nodeLabel(node) == "ghost" {
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

// nodeLabel reads a node's human-facing label as text. It exists so the
// identity assertions below read the same before and after DisplayName
// stopped being a bare string.
func nodeLabel(node CosmosNode) string { return node.Label.String() }

// nodeWithKey finds one node by key, so a failure names the whole graph
// rather than panicking on an index.
func nodeWithKey(graph CosmosGraph, key string) (CosmosNode, bool) {
	for _, node := range graph.Nodes {
		if node.Key == key {
			return node, true
		}
	}
	return CosmosNode{}, false
}

// TestBuildCosmosResolvesReceiverAddressedByARawSessionID is the regression
// for the ghost the operator watched appear and vanish in the cosmos tab: a
// live, named chat received an MCP message and the graph minted a SECOND
// node labelled with that chat's raw tmux session id, which then aged out of
// the no-row grace window and was ruled GONE.
//
// Two independent misses produced it, and this event carries both. The
// receiver's recorded socket is a full PATH ("/tmp/tmux-1000/cc-…") while a
// live row's Socket is the BARE name tmux is addressed by (-L), so the pane
// index could never match; and the Target is the raw session id rather than
// the label, because the reply footer told the sender to address the chat
// that way, so the name index could never match either.
func TestBuildCosmosResolvesReceiverAddressedByARawSessionID(t *testing.T) {
	const session = "cc-1787705979-3980493-30867"
	rows := []Row{
		{Kind: LiveClaude, ID: "p-do-id", Name: "P:DO", Socket: session, PaneID: "%0"},
		{Kind: LiveClaude, ID: "sender-id", Name: "Sender", Socket: "cc-1787705979-3980493-1", PaneID: "%0"},
	}
	events := []shared.CommsEvent{{
		AtNS: 10, Kind: shared.KindInject,
		SenderUUID: "sender-id", SenderSession: "cc-1787705979-3980493-1", SenderLabel: "Sender",
		Target:         session,
		ReceiverSocket: "/tmp/tmux-1000/" + session, ReceiverPane: "%0",
		Message: "hello",
	}}

	graph := BuildCosmos(rows, events, 100)
	if len(graph.Nodes) != 2 {
		t.Fatalf("a message between two known chats minted a node: got %d nodes, want 2\nnodes = %#v", len(graph.Nodes), graph.Nodes)
	}
	node, found := nodeWithKey(graph, "chat:id:p-do-id")
	if !found {
		t.Fatalf("receiver did not resolve to its existing row\nnodes = %#v", graph.Nodes)
	}
	if label := nodeLabel(node); label != "P:DO" {
		t.Fatalf("receiver label = %q, want the chat's own label %q", label, "P:DO")
	}
	if !node.Live || node.Engine != "cc" {
		t.Fatalf("receiver node lost its row's liveness/engine: %#v", node)
	}
}

// TestBuildCosmosResolvesSenderByItsTmuxSessionName covers the wild shape an
// inject-as-SENDER writes: sender_session only, no uuid and no label (a chat
// whose 🔖 statusline had not rendered when it stated its identity). The
// session name is the one identity that event carries, so it has to resolve.
func TestBuildCosmosResolvesSenderByItsTmuxSessionName(t *testing.T) {
	const session = "cc-1787705979-3980493-30867"
	rows := []Row{{Kind: LiveClaude, ID: "p-do-id", Name: "P:DO", Socket: session, PaneID: "%0"}}
	events := []shared.CommsEvent{{
		AtNS: 10, Kind: shared.KindInject, SenderSession: session,
		Target: "Somebody", Message: "hi",
	}}

	graph := BuildCosmos(rows, events, 100)
	node, found := nodeWithKey(graph, "chat:id:p-do-id")
	if !found {
		t.Fatalf("sender did not resolve to its existing row by session name\nnodes = %#v", graph.Nodes)
	}
	if label := nodeLabel(node); label != "P:DO" {
		t.Fatalf("sender label = %q, want %q", label, "P:DO")
	}
}

// TestBuildCosmosResolvesSpawnReceiverRecordedWithNoPane covers the third
// wild shape: a spawn writes the receiver's FULL socket path and no pane id
// at all (spawn.Result carries none), and its Target is the label the chat
// was born with — which chat_name may since have replaced. Socket identity
// is the only thing that survives a rename, so it has to resolve.
func TestBuildCosmosResolvesSpawnReceiverRecordedWithNoPane(t *testing.T) {
	const session = "cc-1787705979-3980493-30867"
	rows := []Row{
		{Kind: LiveClaude, ID: "parent-id", Name: "Parent", Socket: "cc-1787705979-3980493-1", PaneID: "%0"},
		{Kind: LiveClaude, ID: "child-id", Name: "P:DO", Socket: session, PaneID: "%0"},
	}
	events := []shared.CommsEvent{{
		AtNS: 10, Kind: shared.KindSpawn,
		SenderSession:  "cc-1787705979-3980493-1",
		Target:         "born-as-this",
		ReceiverSocket: "/tmp/tmux-1000/" + session,
		ReceiverPane:   "",
		Message:        "born",
	}}

	graph := BuildCosmos(rows, events, 100)
	if len(graph.Nodes) != 2 {
		t.Fatalf("spawn between two known chats minted a node: got %d, want 2\nnodes = %#v", len(graph.Nodes), graph.Nodes)
	}
	child, found := nodeWithKey(graph, "chat:id:child-id")
	if !found {
		t.Fatalf("spawn receiver did not resolve to its existing row\nnodes = %#v", graph.Nodes)
	}
	if label := nodeLabel(child); label != "P:DO" {
		t.Fatalf("spawn receiver label = %q, want the chat's CURRENT label %q", label, "P:DO")
	}
	if _, found := nodeWithKey(graph, "chat:id:parent-id"); !found {
		t.Fatalf("spawn parent did not resolve to its existing row\nnodes = %#v", graph.Nodes)
	}
}

// TestBuildCosmosUnresolvedIdentityNeverRendersAsAName pins the honesty rule
// the ghost broke: an identity that matched no row must render as something
// a human reads as "this did not resolve". A raw session id in the name
// position is indistinguishable from a chat actually called that — the whole
// reason the original defect went unnoticed until a node blinked out.
func TestBuildCosmosUnresolvedIdentityNeverRendersAsAName(t *testing.T) {
	const sender = "cc-1787705979-3980493-30867"
	const target = "cc-1787705979-3980493-999"
	graph := BuildCosmos(nil, []shared.CommsEvent{{
		AtNS: 10, Kind: shared.KindInject, SenderSession: sender,
		Target: target, Message: "neither end has a row",
	}}, 100)
	if len(graph.Nodes) != 2 {
		t.Fatalf("nodes = %#v, want one per unresolved end", graph.Nodes)
	}
	for _, node := range graph.Nodes {
		label := nodeLabel(node)
		if label == sender || label == target {
			t.Fatalf("unresolved identity rendered as a plausible chat name: %q", label)
		}
		if !strings.Contains(label, "unresolved") {
			t.Fatalf("unresolved node label %q does not say it failed to resolve", label)
		}
	}
}

// TestBuildCosmosSpawnResolvesASplitRowThatCarriesNoPaneID answers the one
// question e94c34b left open. That fix made spawn record the receiver's FULL
// socket path so it matched what inject already wrote; the worry was that a
// spawn edge which used to match by pane no longer would.
//
// It is true, and for exactly one row class. A spawn records no pane at all
// (spawn.Result carries none), and every row built from a tmux pane carries
// tmux's own "%N" — so paneKey(socket, "") could never equal those, before or
// after e94c34b. The exception is compose.splitRow, the ONLY row constructor
// that sets Socket and leaves PaneID empty: pre-fix, paneKey(bare-socket, "")
// matched it exactly; post-fix, paneKey(full-path, "") did not. So that one
// class did regress, silently, into a ghost node.
//
// Resolution by session name repairs it for both recorded shapes at once,
// which is what this pins.
func TestBuildCosmosSpawnResolvesASplitRowThatCarriesNoPaneID(t *testing.T) {
	const session = "cc-1787705979-3980493-30867"
	// A split row as compose.splitRow builds one: a socket, no pane id, and
	// no transcript id either.
	rows := []Row{{Kind: LiveSplit, Name: "alpha+beta", Socket: session, SplitCount: 2}}
	for _, recorded := range []struct {
		name   string
		socket string
	}{
		{"post-e94c34b full socket path", "/tmp/tmux-1000/" + session},
		{"pre-e94c34b bare session name", session},
	} {
		t.Run(recorded.name, func(t *testing.T) {
			graph := BuildCosmos(rows, []shared.CommsEvent{{
				AtNS: 10, Kind: shared.KindSpawn, SenderLabel: "Parent",
				Target: "child-at-birth", ReceiverSocket: recorded.socket,
				ReceiverPane: "", Message: "born",
			}}, 100)
			node, found := nodeWithKey(graph, "chat:pane:"+session+"\t")
			if !found {
				t.Fatalf("the split row was not resolved; the spawn minted a ghost instead\nnodes = %#v", graph.Nodes)
			}
			if label := nodeLabel(node); label != "alpha+beta" {
				t.Fatalf("split node label = %q, want the row's own name", label)
			}
			if !node.Live {
				t.Fatalf("resolved split node lost its liveness: %#v", node)
			}
		})
	}
}
