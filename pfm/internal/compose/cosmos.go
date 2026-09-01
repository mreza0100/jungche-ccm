package compose

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"hostops/pfm/internal/resolve"
	"hostops/pfm/internal/shared"
)

const (
	CosmosWindow            = 24 * time.Hour
	CosmosEventCap          = 5000
	CosmosTruncationWarning = "showing newest 5000 events of the window"

	// CosmosTrafficWindow is the lookback a star's temperature is measured
	// over: the events that touched a system inside this window, counted at
	// whatever clock BuildCosmos is handed, so a replayed past renders the
	// heat it had then.
	CosmosTrafficWindow = time.Hour

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
	Key string
	// Label is what a human reads. Its type is resolve.DisplayName, not a
	// string, and that is the point: outside internal/resolve there is no
	// literal that produces one, so a tmux session name or a socket path
	// cannot be assigned here by accident. It arrives from exactly one
	// place — the answer resolve.Directory gave for this node's identity.
	Label  resolve.DisplayName
	Engine string
	// Home is the star this chat belongs to — its Row.Project, the same
	// project identity the picker groups by, so the cosmos and the list can
	// never disagree about where a chat lives. Empty for a ghost with no
	// matching row: its home is genuinely unknown, and the renderer seats it
	// under the shared unknown star rather than guessing.
	Home string
	// RowID is the fleet row this node resolved to — the join key the picker
	// uses to open a chat from the sky (rowKey prefers ID). Empty for a
	// ghost.
	RowID  string
	Live   bool
	Killed bool
	LastNS int64
	// Dead marks a node whose chat no longer has a live presence in the
	// roster. Two different questions decide it, and only one of them
	// needs a clock: a matched row explicitly Killed is positive evidence,
	// true immediately, no waiting required. A node with NO matching row
	// is ambiguous on its own — a freshly spawned chat looks identical to
	// a vanished one until the row layer catches up — so BuildCosmos gives
	// it a short grace window (nowNS - LastNS) before calling a still-
	// missing row GONE rather than PENDING.
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

// CosmosStar is one project's sun as the ledger sees it. TrafficHour is how
// many events touched the system inside CosmosTrafficWindow — an event
// counts ONCE per distinct Home among its endpoints, so an intra-system
// message warms one star and a cross-system message warms both. Population
// is not here on purpose: the renderer counts the nodes it actually draws.
type CosmosStar struct {
	Home        string
	TrafficHour int
	LastNS      int64
}

type CosmosGraph struct {
	Nodes    []CosmosNode
	Edges    []CosmosEdge
	Stars    map[string]CosmosStar
	Err      string
	Warnings []string
}

type cosmosBuilder struct {
	// directory is the ONE index every identity in this graph resolves
	// through. Both halves of a conversation ask it the same question in
	// the same way; the three hand-rolled maps that used to sit here were
	// two divergent ladders, and the asymmetry between them was the root
	// of the ghost-node family.
	directory *resolve.Directory[Row]
	nodes     map[string]CosmosNode
	edges     map[string]CosmosEdge
	warnings  []string
	// unmatched holds every node key built with no matching row — no
	// positive evidence of death, just its absence. BuildCosmos resolves
	// these once every event has been walked and each key's final LastNS
	// is known, via the grace window below; a resolved row's Dead (set in
	// chatNode from row.Killed) is never touched here.
	unmatched map[string]bool
	// nowNS is the clock BuildCosmos was handed, threaded onto the builder
	// so warm can answer "did this touch fall inside CosmosTrafficWindow"
	// without every call site re-deriving it.
	nowNS int64
	// stars accumulates one CosmosStar per distinct Home touched by warm.
	// BuildCosmos folds it against the node set at the end so a home with
	// zero traffic still gets a cold star, and a home no node belongs to
	// never gets one it was not asked for.
	stars map[string]CosmosStar
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
		directory: resolve.NewDirectory(rows, rowAddress),
		nodes:     make(map[string]CosmosNode),
		edges:     make(map[string]CosmosEdge),
		nowNS:     nowNS,
	}

	unsigned := 0
	for _, event := range events {
		switch event.Kind {
		case shared.KindInject:
			if event.SenderSession == "" && event.SenderUUID == "" && event.SenderLabel == "" {
				unsigned++
			}
			from := builder.chatNode(senderAddress(event))
			to := builder.chatNode(receiverAddress(event))
			builder.touch(from, event.AtNS)
			builder.touch(to, event.AtNS)
			builder.addEdge(from.Key, to.Key, event)
			builder.warm(event.AtNS, from, to)
		case shared.KindSpawn:
			child := builder.chatNode(receiverAddress(event))
			builder.touch(child, event.AtNS)
			if event.SenderSession == "" && event.SenderUUID == "" && event.SenderLabel == "" {
				unsigned++
				builder.warm(event.AtNS, child)
				continue
			}
			parent := builder.chatNode(senderAddress(event))
			builder.touch(parent, event.AtNS)
			builder.addEdge(parent.Key, child.Key, event)
			builder.warm(event.AtNS, child, parent)
		case "group":
			// Retired chat-group events: a historical fleet.db row of this
			// kind is known history, not an unknown one — skip it silently,
			// with no node, no edge, and no warning.
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
	if unsigned > 0 {
		// Error is never absence: an unsigned event has no sender node, so
		// its line and its lineage CANNOT be drawn — and a universe missing
		// lines must say why, not render a quieter sky and let the gap read
		// as "no traffic".
		builder.warnings = append(builder.warnings, fmt.Sprintf(
			"%d events carry no sender identity — their lines cannot be drawn", unsigned,
		))
	}
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
	// Stars gets one entry per distinct Home among the nodes just built —
	// zero traffic included, a cold star is still a star — and only for
	// those homes: a home warm() touched but that ended up with no
	// surviving node (there is none such today, but the rule is the graph's
	// node set decides, not the builder's touch history) does not get one.
	homes := make(map[string]bool, len(graph.Nodes))
	for _, node := range graph.Nodes {
		homes[node.Home] = true
	}
	graph.Stars = make(map[string]CosmosStar, len(homes))
	for home := range homes {
		star := builder.stars[home]
		star.Home = home
		graph.Stars[home] = star
	}
	sort.Slice(graph.Nodes, func(left, right int) bool {
		leftLabel := graph.Nodes[left].Label.String()
		rightLabel := graph.Nodes[right].Label.String()
		if folded := strings.ToLower(leftLabel); folded != strings.ToLower(rightLabel) {
			return folded < strings.ToLower(rightLabel)
		}
		if leftLabel != rightLabel {
			return leftLabel < rightLabel
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

// rowAddress projects one fleet row into the shared identity address space.
// It is the ONLY place this package states which of a Row's fields are
// identities, and Row.Socket is deliberately handed over as Session: a live
// row carries the BARE socket name tmux is addressed by (-L), while the
// comms ledger records full -S paths, and resolve.Address normalises both to
// the same tmux session name. Keying one side bare and the other by path is
// what made the pane index unmatchable.
func rowAddress(row Row) resolve.Address {
	return resolve.Address{
		ID:      row.ID,
		Session: row.Socket,
		Pane:    row.PaneID,
		Label:   row.Name,
	}
}

// senderAddress and receiverAddress are the ledger's two identity
// projections. Every column lands in the field whose flavour it actually
// has — except Target, which is whatever a caller typed and therefore goes
// into Text, the untyped slot resolution sweeps across every namespace. That
// is not a hedge: the reply footer used to hand callers a raw session id to
// address a chat by, so the target column genuinely holds session names,
// labels and stale labels alike.
func senderAddress(event shared.CommsEvent) resolve.Address {
	return resolve.Address{
		ID:      event.SenderUUID,
		Session: event.SenderSession,
		Label:   event.SenderLabel,
	}
}

func receiverAddress(event shared.CommsEvent) resolve.Address {
	return resolve.Address{
		Session: event.ReceiverSocket,
		Pane:    event.ReceiverPane,
		Text:    event.Target,
	}
}

// chatNode is the ONE path from any identity a recorded event carries to the
// node it belongs to. Both halves of a conversation come through here, which
// is the whole fix: a sender and a receiver describing the SAME chat used to
// walk two different ladders and land on two different nodes.
//
// A node whose identity resolved to no row is still a node — the graph must
// show that somebody talked — but it is marked for the grace-window
// resolution BuildCosmos performs once every event has been walked. It
// carries no Dead verdict here: the ABSENCE of a row is not the same
// evidence as row.Killed, and whether a still-missing row means PENDING or
// GONE cannot be decided until this key's final LastNS is known.
func (builder *cosmosBuilder) chatNode(query resolve.Address) CosmosNode {
	answer := builder.directory.Lookup(query)
	key := cosmosKey(answer)
	if key == "" {
		return CosmosNode{}
	}
	node := CosmosNode{Key: key, Label: answer.Display}
	if answer.Found {
		node.Engine = string(EngineForKind(answer.Chat.Kind))
		node.Home = answer.Chat.Project
		node.RowID = answer.Chat.ID
		node.Live = answer.Chat.Socket != ""
		node.Killed = answer.Chat.Killed
		node.Dead = answer.Chat.Killed
		return node
	}
	if builder.unmatched == nil {
		builder.unmatched = make(map[string]bool)
	}
	builder.unmatched[key] = true
	return node
}

// cosmosKey is the graph's node identity, and it keys on machine addresses,
// never on a label: chat_name renames a chat at will and a freed label is
// reused by a later one, so a label-keyed node would retroactively rewrite
// who said what in an append-only ledger.
//
// The unresolved branch prefers the SESSION over anything else for one
// specific reason: it is the only identity both halves of a conversation
// carry independently for the same chat — a sender states its own
// sender_session, a receiver is recorded by socket — so a chat that has
// vanished from the roster converges on ONE ghost instead of one per side.
func cosmosKey(answer resolve.Answer[Row]) string {
	address := answer.Address
	name := firstNonEmpty(address.Label, address.Text)
	if answer.Found {
		switch {
		case address.ID != "":
			return "chat:id:" + address.ID
		case address.Session != "":
			return "chat:pane:" + address.Session + "\t" + address.Pane
		case name != "":
			return "chat:name:" + name
		}
		return ""
	}
	switch {
	case address.Session != "":
		return "chat:session:" + address.Session
	case address.ID != "":
		return "chat:id:" + address.ID
	case name != "":
		return "chat:name:" + name
	}
	return ""
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

// warm folds one event's timestamp into every distinct Home among the given
// endpoint nodes — an intra-system event (both endpoints share a Home) warms
// that one star once, never twice, and a cross-system event warms both. A
// node with no Key (an unresolved lookup) never reaches a star: it carries
// no identity to warm anything with. TrafficHour only increments inside
// CosmosTrafficWindow of nowNS, but LastNS always advances, so a star this
// package has never seen fresh still remembers when it last was.
func (builder *cosmosBuilder) warm(atNS int64, nodes ...CosmosNode) {
	if builder.stars == nil {
		builder.stars = make(map[string]CosmosStar)
	}
	seen := make(map[string]bool, len(nodes))
	for _, node := range nodes {
		if node.Key == "" || seen[node.Home] {
			continue
		}
		seen[node.Home] = true
		star := builder.stars[node.Home]
		star.Home = node.Home
		if atNS > builder.nowNS-int64(CosmosTrafficWindow) {
			star.TrafficHour++
		}
		if atNS > star.LastNS {
			star.LastNS = atNS
		}
		builder.stars[node.Home] = star
	}
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
