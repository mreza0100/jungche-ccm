package compose

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"hostops/pfm/internal/shared"
)

const (
	CosmosWindow            = 24 * time.Hour
	CosmosEventCap          = 5000
	CosmosTruncationWarning = "showing newest 5000 events of the window"

	// CosmosDeathBlink is how long a dead node blinks like an alarm before it
	// starts to fade. CosmosDeathFade is the fade tail that follows, after
	// which the model stops rendering it. Both live here — not in
	// internal/ui, which is where they are actually consumed — so the
	// renderer's animation timing and model.applyCosmosGraph's pruning
	// window agree on where "dead" ends: one clock, not two that can drift
	// apart. This package does not use them itself; it has no clock to
	// measure them against.
	//
	// Short on purpose: a chat that just died should read as gone quickly
	// and calmly, not strobe. A handful of fast alarm toggles says "this one
	// just stopped" without turning the whole tab into a strobe light for
	// ten seconds.
	CosmosDeathBlink = 1200 * time.Millisecond
	CosmosDeathFade  = 1500 * time.Millisecond
)

type CosmosNode struct {
	Key    string
	Label  string
	Engine string
	Live   bool
	Killed bool
	Group  bool
	LastNS int64
	// Dead marks a node whose chat no longer has a live presence in the
	// roster. Two different questions decide it, and only one of them
	// needs a clock: a matched row explicitly Killed is positive evidence,
	// true immediately, no waiting required. A node with NO matching row
	// is ambiguous on its own — a freshly spawned chat looks identical to
	// a vanished one until the row layer catches up — so BuildCosmos gives
	// it a short grace window (nowNS - LastNS) before calling a still-
	// missing row GONE rather than PENDING. A group is never Dead; it is a
	// channel, not a chat that can die.
	Dead bool
	// DiedNS, as this package sets it, is the node's own last edge activity
	// (LastNS), not a kill time — the store's kill table has no path into
	// this package (compose.Row carries only the Killed bool, not a
	// timestamp), and a chat that vanished without ever being recorded
	// killed has no timestamp at all. That is the most honest fact this
	// stateless package has; it is NOT when the chat died and this package
	// never claims otherwise. model.applyCosmosGraph (internal/ui)
	// overwrites this field with the render-detection moment — when the
	// model itself first observed the node Dead — before storing the graph
	// it actually renders from, because only a caller with memory across
	// calls can answer "when did it die".
	DiedNS int64
}

type CosmosEdge struct {
	From        string
	To          string
	Kind        string
	Count       int
	LastNS      int64
	LastMessage string
}

type CosmosGraph struct {
	Nodes    []CosmosNode
	Edges    []CosmosEdge
	Err      string
	Warnings []string
}

type cosmosBuilder struct {
	rowsByID   map[string]*Row
	rowsByPane map[string]*Row
	rowsByName map[string]*Row
	nodes      map[string]CosmosNode
	edges      map[string]CosmosEdge
	warnings   []string
	// unmatched holds every node key built with no matching row — no
	// positive evidence of death, just its absence. BuildCosmos resolves
	// these once every event has been walked and each key's final LastNS
	// is known, via the grace window below; a matched row's Dead (set in
	// cosmosRowNode from row.Killed) is never touched here.
	unmatched map[string]bool
}

// BuildCosmos derives the complete presentation graph from fleet rows and a
// caller-bounded event window. It performs no I/O and remembers nothing
// between calls, but it DOES read nowNS, for exactly one question it can
// answer truthfully: whether a no-row-matched node's absence has had long
// enough to mean the row layer will never catch up (see the grace-window
// resolution below). That is a different question from "when did this
// chat die" — this package still refuses that one, because LastNS cannot
// answer it and DiedNS below stays honestly labeled as last edge activity,
// never a kill time. Deciding how long a WITNESSED death stays visible
// once animating is likewise still not this function's job:
// model.applyCosmosGraph (internal/ui) owns that, keyed on the
// render-detection moment only a caller with memory across calls can know.
func BuildCosmos(rows []Row, events []shared.CommsEvent, nowNS int64) CosmosGraph {
	builder := cosmosBuilder{
		rowsByID:   make(map[string]*Row),
		rowsByPane: make(map[string]*Row),
		rowsByName: make(map[string]*Row),
		nodes:      make(map[string]CosmosNode),
		edges:      make(map[string]CosmosEdge),
	}
	for index := range rows {
		row := &rows[index]
		if row.ID != "" {
			if _, exists := builder.rowsByID[row.ID]; !exists {
				builder.rowsByID[row.ID] = row
			}
		}
		if row.Socket != "" {
			key := paneKey(row.Socket, row.PaneID)
			if _, exists := builder.rowsByPane[key]; !exists {
				builder.rowsByPane[key] = row
			}
		}
		if row.Name != "" {
			if _, exists := builder.rowsByName[row.Name]; !exists {
				builder.rowsByName[row.Name] = row
			}
		}
	}

	for _, event := range events {
		switch event.Kind {
		case shared.KindInject:
			from := builder.senderNode(event)
			to := builder.receiverNode(event)
			builder.touch(from, event.AtNS)
			builder.touch(to, event.AtNS)
			builder.addEdge(from.Key, to.Key, event)
		case shared.KindGroup:
			from := builder.senderNode(event)
			group := CosmosNode{Key: groupKey(event.GroupName), Label: event.GroupName, Group: true}
			builder.touch(from, event.AtNS)
			builder.touch(group, event.AtNS)
			builder.addEdge(from.Key, group.Key, event)
			var members []string
			if err := json.Unmarshal([]byte(event.Members), &members); err != nil {
				builder.warnings = append(builder.warnings, fmt.Sprintf(
					"group comms event %d has malformed members: %v", event.ID, err,
				))
				continue
			}
			for _, member := range members {
				node := builder.namedNode(member)
				builder.touch(node, event.AtNS)
				builder.addEdge(group.Key, node.Key, event)
			}
		case shared.KindSpawn:
			child := builder.receiverNode(event)
			builder.touch(child, event.AtNS)
			if event.SenderSession == "" && event.SenderUUID == "" && event.SenderLabel == "" {
				continue
			}
			parent := builder.senderNode(event)
			builder.touch(parent, event.AtNS)
			builder.addEdge(parent.Key, child.Key, event)
		default:
			builder.warnings = append(builder.warnings, fmt.Sprintf(
				"comms event %d has unknown kind %q", event.ID, event.Kind,
			))
		}
	}

	// The row layer's own indexing latency is bounded and short, unlike a
	// chat's idle time — reusing CosmosDeathBlink+CosmosDeathFade here is
	// not a coincidence of convenience, it is the same "how long does the
	// app wait before treating an absence as final" question this package
	// already has one answer to, so a second, independently-tuned knob
	// would just be two numbers that can drift apart for no reason.
	rowGrace := int64(CosmosDeathBlink + CosmosDeathFade)
	graph := CosmosGraph{Warnings: builder.warnings}
	graph.Nodes = make([]CosmosNode, 0, len(builder.nodes))
	for key, node := range builder.nodes {
		if builder.unmatched[key] {
			// No row matched. On its own that is ambiguous — a freshly
			// spawned chat looks identical to a vanished one until the row
			// layer catches up — so give it rowGrace before calling a
			// still-missing row GONE rather than PENDING. This is the one
			// nowNS-dependent verdict this stateless package makes, and it
			// answers "has enough time passed for absence to mean gone",
			// never "when did it die".
			node.Dead = nowNS-node.LastNS > rowGrace
		}
		if node.Dead {
			// The only death time this package can honestly report is the
			// node's own last edge activity: compose.Row carries a Killed
			// bool but no kill timestamp, and a chat that vanished without
			// ever being recorded killed has no timestamp at all.
			// Fabricating a more precise one would be a lie the graph
			// cannot back up — and this package has no memory of its own
			// prior calls to do better, so it does not try. The node stays
			// in the graph regardless of age; model.applyCosmosGraph
			// decides how long to keep RENDERING it.
			node.DiedNS = node.LastNS
		}
		graph.Nodes = append(graph.Nodes, node)
	}
	sort.Slice(graph.Nodes, func(left, right int) bool {
		leftLabel := strings.ToLower(graph.Nodes[left].Label)
		rightLabel := strings.ToLower(graph.Nodes[right].Label)
		if leftLabel != rightLabel {
			return leftLabel < rightLabel
		}
		if graph.Nodes[left].Label != graph.Nodes[right].Label {
			return graph.Nodes[left].Label < graph.Nodes[right].Label
		}
		return graph.Nodes[left].Key < graph.Nodes[right].Key
	})
	graph.Edges = make([]CosmosEdge, 0, len(builder.edges))
	for _, edge := range builder.edges {
		graph.Edges = append(graph.Edges, edge)
	}
	sort.Slice(graph.Edges, func(left, right int) bool {
		if graph.Edges[left].LastNS != graph.Edges[right].LastNS {
			return graph.Edges[left].LastNS > graph.Edges[right].LastNS
		}
		if graph.Edges[left].From != graph.Edges[right].From {
			return graph.Edges[left].From < graph.Edges[right].From
		}
		if graph.Edges[left].To != graph.Edges[right].To {
			return graph.Edges[left].To < graph.Edges[right].To
		}
		return graph.Edges[left].Kind < graph.Edges[right].Kind
	})
	return graph
}

// senderNode resolves the row this event's sender still lives in. Row
// lookup priority is unchanged from before: id, then session, then label.
// Only the fallback below — reached once none of those match, meaning the
// chat has no surviving row — changed shape.
func (builder *cosmosBuilder) senderNode(event shared.CommsEvent) CosmosNode {
	for _, id := range []string{event.SenderUUID, event.SenderSession} {
		if row := builder.rowsByID[id]; row != nil {
			return cosmosRowNode(*row)
		}
	}
	if row := builder.rowsByName[event.SenderLabel]; row != nil {
		return cosmosRowNode(*row)
	}
	// No row survives. The tmux session name is the one identity both a
	// sender and a receiver independently carry for the SAME chat (see
	// receiverNode below), so it is preferred over a UUID or a label — an
	// id-only or label-only fallback cannot be correlated with anything the
	// other side of a conversation reports.
	if event.SenderSession != "" {
		return builder.chatSessionNode(event.SenderSession, firstNonEmpty(event.SenderLabel, event.SenderSession))
	}
	key := ""
	switch {
	case event.SenderUUID != "":
		key = "chat:id:" + event.SenderUUID
	case event.SenderLabel != "":
		key = "chat:name:" + event.SenderLabel
	}
	label := event.SenderLabel
	if label == "" {
		label = event.SenderUUID
	}
	return builder.fallbackNode(key, label)
}

// receiverNode resolves the row this event's receiver still lives in. Row
// lookup priority is unchanged: the exact (socket, pane) pair, then the
// target name. Only the fallback below changed shape.
func (builder *cosmosBuilder) receiverNode(event shared.CommsEvent) CosmosNode {
	if event.ReceiverSocket != "" {
		if row := builder.rowsByPane[paneKey(event.ReceiverSocket, event.ReceiverPane)]; row != nil {
			return cosmosRowNode(*row)
		}
	}
	if row := builder.rowsByName[event.Target]; row != nil {
		return cosmosRowNode(*row)
	}
	if event.ReceiverSocket != "" {
		// filepath.Base of a full socket PATH ("/tmp/tmux-1000/cc-…") and of
		// a bare session NAME ("cc-…", the pre-fix writer's shape) both land
		// on the same tmux session name — the one datum senderNode's
		// fallback above can also produce from sender_session, so a dead
		// chat's sent and received edges converge on ONE ghost, not two.
		session := filepath.Base(event.ReceiverSocket)
		return builder.chatSessionNode(session, firstNonEmpty(event.Target, event.ReceiverSocket))
	}
	return builder.namedNode(event.Target)
}

// chatSessionNode is the canonical no-row-matched identity for a chat: both
// halves of the ledger can derive the same tmux session name independently,
// so it is the only fallback key space that keeps a dead chat as one node.
func (builder *cosmosBuilder) chatSessionNode(session, label string) CosmosNode {
	if session == "" {
		return CosmosNode{}
	}
	return builder.fallbackNode("chat:session:"+session, label)
}

func (builder *cosmosBuilder) namedNode(name string) CosmosNode {
	if row := builder.rowsByName[name]; row != nil {
		return cosmosRowNode(*row)
	}
	if name == "" {
		return CosmosNode{}
	}
	return builder.fallbackNode("chat:name:"+name, name)
}

// fallbackNode builds a no-row-matched node identity and marks it for the
// grace-window resolution BuildCosmos performs once every event has been
// walked. It carries no Dead verdict here — the ABSENCE of a row is not
// the same evidence as row.Killed, and whether a still-missing row means
// PENDING or GONE cannot be decided until this key's final LastNS (every
// touch this graph will ever see for it) is known.
func (builder *cosmosBuilder) fallbackNode(key, label string) CosmosNode {
	if key == "" {
		return CosmosNode{}
	}
	if builder.unmatched == nil {
		builder.unmatched = make(map[string]bool)
	}
	builder.unmatched[key] = true
	return CosmosNode{Key: key, Label: label}
}

func (builder *cosmosBuilder) touch(node CosmosNode, atNS int64) {
	if node.Key == "" {
		return
	}
	current, exists := builder.nodes[node.Key]
	if !exists {
		current = node
	}
	if atNS > current.LastNS {
		current.LastNS = atNS
	}
	builder.nodes[node.Key] = current
}

func (builder *cosmosBuilder) addEdge(from, to string, event shared.CommsEvent) {
	if from == "" || to == "" {
		return
	}
	key := from + "\x00" + to + "\x00" + event.Kind
	edge := builder.edges[key]
	if edge.Count == 0 {
		edge = CosmosEdge{
			From: from, To: to, Kind: event.Kind,
			LastNS: event.AtNS, LastMessage: event.Message,
		}
	} else if event.AtNS > edge.LastNS {
		edge.LastNS = event.AtNS
		edge.LastMessage = event.Message
	}
	edge.Count++
	builder.edges[key] = edge
}

func cosmosRowNode(row Row) CosmosNode {
	key := ""
	switch {
	case row.ID != "":
		key = "chat:id:" + row.ID
	case row.Socket != "":
		key = "chat:pane:" + paneKey(row.Socket, row.PaneID)
	case row.Name != "":
		key = "chat:name:" + row.Name
	}
	return CosmosNode{
		Key: key, Label: firstNonEmpty(row.Name, row.ID, row.Socket),
		Engine: string(EngineForKind(row.Kind)), Live: row.Socket != "", Killed: row.Killed,
		Dead: row.Killed,
	}
}

func paneKey(socket, pane string) string { return socket + "\t" + pane }
func groupKey(name string) string        { return "group:" + name }
