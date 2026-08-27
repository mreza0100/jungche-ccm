package compose

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"hostops/pfm/internal/shared"
)

const (
	CosmosWindow            = 24 * time.Hour
	CosmosEventCap          = 5000
	CosmosTruncationWarning = "showing newest 5000 events of the window"
)

type CosmosNode struct {
	Key    string
	Label  string
	Engine string
	Live   bool
	Killed bool
	Group  bool
	LastNS int64
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
}

// BuildCosmos derives the complete presentation graph from fleet rows and a
// caller-bounded event window. It performs no I/O and reads no clock.
func BuildCosmos(rows []Row, events []shared.CommsEvent, nowNS int64) CosmosGraph {
	_ = nowNS
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

	graph := CosmosGraph{Warnings: builder.warnings}
	graph.Nodes = make([]CosmosNode, 0, len(builder.nodes))
	for _, node := range builder.nodes {
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

func (builder *cosmosBuilder) senderNode(event shared.CommsEvent) CosmosNode {
	for _, id := range []string{event.SenderUUID, event.SenderSession} {
		if row := builder.rowsByID[id]; row != nil {
			return cosmosRowNode(*row)
		}
	}
	if row := builder.rowsByName[event.SenderLabel]; row != nil {
		return cosmosRowNode(*row)
	}
	key := ""
	switch {
	case event.SenderUUID != "":
		key = "chat:id:" + event.SenderUUID
	case event.SenderSession != "":
		key = "chat:id:" + event.SenderSession
	case event.SenderLabel != "":
		key = "chat:name:" + event.SenderLabel
	}
	label := event.SenderLabel
	if label == "" {
		label = firstNonEmpty(event.SenderSession, event.SenderUUID)
	}
	return CosmosNode{Key: key, Label: label}
}

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
		return CosmosNode{
			Key:   "chat:pane:" + paneKey(event.ReceiverSocket, event.ReceiverPane),
			Label: firstNonEmpty(event.Target, event.ReceiverSocket),
		}
	}
	return builder.namedNode(event.Target)
}

func (builder *cosmosBuilder) namedNode(name string) CosmosNode {
	if row := builder.rowsByName[name]; row != nil {
		return cosmosRowNode(*row)
	}
	if name == "" {
		return CosmosNode{}
	}
	return CosmosNode{Key: "chat:name:" + name, Label: name}
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
	}
}

func paneKey(socket, pane string) string { return socket + "\t" + pane }
func groupKey(name string) string        { return "group:" + name }
