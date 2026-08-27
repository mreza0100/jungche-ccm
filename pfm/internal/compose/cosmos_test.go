package compose

import (
	"reflect"
	"strings"
	"testing"

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
		Key: "chat:pane:cx-orphan\t", Label: "Orphan", LastNS: 60,
	}}
	if !reflect.DeepEqual(graph.Nodes, want) {
		t.Fatalf("parentless nodes = %#v, want %#v", graph.Nodes, want)
	}
}
