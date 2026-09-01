package ui

import (
	"math"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"hostops/pfm/internal/compose"
	"hostops/pfm/internal/resolve"
	"hostops/pfm/internal/shared"
)

// U1: TestChronoscopeScrubCutsThePastFromTheSameEvents pins the chronoscope's
// core cut: scrubbing the playhead back builds a graph from a PREFIX of the
// same sampled timeline, never touching the live graph model.cosmos keeps.
func TestChronoscopeScrubCutsThePastFromTheSameEvents(t *testing.T) {
	model := cosmosGoldenModel(120)
	model.cosmosEvents = cosmosGoldenEvents(model.cosmosNowNS, model.rows)
	model.rebuildCosmosTimeline()

	liveNodes, liveEdges := len(model.cosmos.Nodes), len(model.cosmos.Edges)
	if liveNodes != 3 || liveEdges != 2 {
		t.Fatalf("setup: live graph = %d nodes, %d edges, want 3 and 2", liveNodes, liveEdges)
	}

	past, _ := applyKey(t, model, printableKey('['))
	if past.cosmosPast == nil {
		t.Fatal("[ did not enter replay: cosmosPast is nil")
	}
	cut := past.viewGraph()
	if len(cut.Nodes) != 2 || len(cut.Edges) != 1 {
		t.Fatalf("5-minute-back cut = %d nodes, %d edges, want 2 and 1 (the 10m-old spawn present, the 300ms-old inject absent)\n%#v", len(cut.Nodes), len(cut.Edges), cut)
	}
	// The live graph model.cosmos must never be mutated by a scrub — only
	// model.cosmosPast is a new graph.
	if len(past.cosmos.Nodes) != liveNodes || len(past.cosmos.Edges) != liveEdges {
		t.Fatalf("scrub mutated the LIVE graph: nodes=%d edges=%d, want %d and %d", len(past.cosmos.Nodes), len(past.cosmos.Edges), liveNodes, liveEdges)
	}

	live, _ := applyKey(t, past, printableKey(']'))
	if live.cosmosPast != nil {
		t.Fatalf("] landing on now did not return to live: cosmosPast=%#v", live.cosmosPast)
	}
	if live.cosmosViewNS != live.cosmosNowNS {
		t.Fatalf("cosmosViewNS = %d, want cosmosNowNS %d after returning to live", live.cosmosViewNS, live.cosmosNowNS)
	}
	restored := live.viewGraph()
	if len(restored.Nodes) != 3 || len(restored.Edges) != 2 {
		t.Fatalf("live graph after return = %d nodes, %d edges, want 3 and 2", len(restored.Nodes), len(restored.Edges))
	}
}

// U2: TestChronoscopeRefusesByNameBeforeTheFirstSample pins the refusal
// contract: a scrub key pressed before the ledger has ever been sampled
// names the reason rather than doing nothing, and the query row shows it.
func TestChronoscopeRefusesByNameBeforeTheFirstSample(t *testing.T) {
	model := cosmosGoldenModel(80)
	if model.cosmosTimeline != nil {
		t.Fatalf("setup: a fresh model already has a timeline: %#v", model.cosmosTimeline)
	}
	pressed, _ := applyKey(t, model, printableKey('['))
	if pressed.cosmosPast != nil {
		t.Fatalf("scrub before the first sample entered replay anyway: %#v", pressed.cosmosPast)
	}
	if !strings.Contains(pressed.cosmosStatus, "has not been sampled") {
		t.Fatalf("cosmosStatus = %q, want it to name the unsampled ledger", pressed.cosmosStatus)
	}
	query := ansi.Strip(pressed.renderQuery(80))
	if !strings.Contains(query, "has not been sampled") {
		t.Fatalf("query row = %q, does not carry the refusal", query)
	}
}

// U3: TestChronoscopeClampsAtTheWindowFloor pins the 24h ledger boundary: no
// amount of scrubbing back can pass CosmosWindow, and the floor names itself.
func TestChronoscopeClampsAtTheWindowFloor(t *testing.T) {
	model := cosmosGoldenModel(80)
	model.cosmosEvents = cosmosGoldenEvents(model.cosmosNowNS, model.rows)
	model.rebuildCosmosTimeline()

	for i := 0; i < 30; i++ {
		model, _ = applyKey(t, model, printableKey('{'))
	}
	floor := model.cosmosNowNS - int64(compose.CosmosWindow)
	if model.cosmosViewNS != floor {
		t.Fatalf("cosmosViewNS = %d, want the window floor %d after 30 coarse scrubs", model.cosmosViewNS, floor)
	}
	if !strings.Contains(model.cosmosStatus, "24h") {
		t.Fatalf("cosmosStatus = %q, want it to name the 24h floor", model.cosmosStatus)
	}
}

// U4: TestChronoscopePlayAdvancesAtSixtyTimesAndSnapsToLive pins playback:
// one real second of tick advances the playhead sixty ledger-seconds, and
// reaching now snaps cleanly back to live. --no-sky refuses play by name.
func TestChronoscopePlayAdvancesAtSixtyTimesAndSnapsToLive(t *testing.T) {
	model := cosmosSkyGoldenModel(80)
	model.cosmosEvents = cosmosGoldenEvents(model.cosmosNowNS, model.rows)
	model.rebuildCosmosTimeline()

	playing, _ := applyKey(t, model, printableKey(' '))
	if playing.cosmosPast == nil || !playing.cosmosPlaying {
		t.Fatalf("space did not start playback: past=%#v playing=%v", playing.cosmosPast, playing.cosmosPlaying)
	}
	wantStart := playing.cosmosNowNS - int64(time.Hour)
	if playing.cosmosViewNS != wantStart {
		t.Fatalf("space did not seat the playhead an hour back: got %d, want %d", playing.cosmosViewNS, wantStart)
	}

	before := playing.cosmosViewNS
	updated, cmd := playing.Update(cosmosTickMsg{
		generation: playing.cosmosTickGeneration,
		nowNS:      playing.cosmosNowNS + int64(time.Second),
	})
	if cmd == nil {
		t.Fatal("a live cosmos tick returned no follow-up command")
	}
	ticked := updated.(Model)
	advanced := ticked.cosmosViewNS - before
	if math.Abs(float64(advanced)-float64(60*time.Second)) > float64(2*time.Millisecond) {
		t.Fatalf("one real second advanced the playhead by %s, want ~60s", time.Duration(advanced))
	}

	// Keep ticking, real time advancing 1s each frame, until the playhead
	// (advancing 60 ledger-seconds per tick) must have crossed "now".
	now := ticked.cosmosNowNS
	model = ticked
	for i := 0; i < 65; i++ {
		now += int64(time.Second)
		updated, _ := model.Update(cosmosTickMsg{generation: model.cosmosTickGeneration, nowNS: now})
		model = updated.(Model)
	}
	if model.cosmosPast != nil || model.cosmosPlaying {
		t.Fatalf("playback did not snap back to live after crossing now: past=%#v playing=%v", model.cosmosPast, model.cosmosPlaying)
	}

	noSky := cosmosGoldenModel(80) // NoSky (skyEnabled=false)
	noSky.cosmosEvents = cosmosGoldenEvents(noSky.cosmosNowNS, noSky.rows)
	noSky.rebuildCosmosTimeline()
	refused, _ := applyKey(t, noSky, printableKey(' '))
	if refused.cosmosPast != nil || refused.cosmosPlaying {
		t.Fatalf("--no-sky started playback anyway: past=%#v playing=%v", refused.cosmosPast, refused.cosmosPlaying)
	}
	if !strings.Contains(refused.cosmosStatus, "--no-sky") {
		t.Fatalf("cosmosStatus = %q, want it to name --no-sky", refused.cosmosStatus)
	}
}

// U5: TestChronoscopeReplayRendersADeadChatAsAGhostNotAnAlarm pins replay
// death honesty (cosmos.go's drawCosmosUniverse Dead branch, and
// applyCosmosGraph's doc comment): a row Killed right now, with ledger
// activity in the past, is dropped from the LIVE graph (never witnessed
// alive by this model, so applyCosmosGraph drops it as unwitnessed) but
// still renders — dim and static, never the alarm blink — when the
// chronoscope is scrubbed back to when it was talking.
func TestChronoscopeReplayRendersADeadChatAsAGhostNotAnAlarm(t *testing.T) {
	snapshot := fixtureSnapshot(120)
	snapshot.NoSky = false // the sky must be ON: the ghost rule must hold even then
	rows := append([]compose.Row(nil), snapshot.Rows...)
	rows[0].Killed = true
	rows[0].Name = "Ghosty"
	snapshot.Rows = rows

	oldEvent := shared.CommsEvent{
		AtNS: fixtureNowNS - int64(10*time.Minute), Kind: shared.KindInject,
		SenderUUID: rows[0].ID, Target: rows[1].Name, ReceiverSocket: rows[1].Socket,
		Message: "dying words",
	}
	snapshot.Cosmos = compose.BuildCosmos(snapshot.Rows, []shared.CommsEvent{oldEvent}, fixtureNowNS)

	model := NewModel(snapshot)
	model.tab = TabCosmos
	deadKey := "chat:id:" + rows[0].ID

	for _, node := range model.cosmos.Nodes {
		if node.Key == deadKey {
			t.Fatalf("the live graph kept a node this model never witnessed alive: %#v", node)
		}
	}

	model.cosmosEvents = []shared.CommsEvent{oldEvent}
	model.rebuildCosmosTimeline()
	replay, _ := applyKey(t, model, printableKey('['))
	if replay.cosmosPast == nil {
		t.Fatal("[ did not enter replay")
	}
	// The live graph had only RR seated (Ghosty was dropped as unwitnessed);
	// entering replay adds Ghosty as a brand-new seat at its own target while
	// RR's pre-existing seat only starts EASING toward its own new target —
	// by design, seats never teleport. Settle both here so the render below
	// checks the settled picture a viewer actually sees, not the one
	// coincidental transient frame where an old and a new seat overlap.
	for _, seat := range replay.cosmosSeats {
		seat.Angle = seat.Target
		seat.Ring = seat.RingTarget
	}
	deadNode, found := cosmosNodeByKey(replay.viewGraph(), deadKey)
	if !found {
		t.Fatalf("the replayed past dropped the node entirely: %#v", replay.viewGraph().Nodes)
	}
	if !deadNode.Dead {
		t.Fatalf("replayed node should be Dead (its row is Killed right now): %#v", deadNode)
	}

	if !strings.Contains(ansi.Strip(replay.View().Content), "Ghosty") {
		t.Fatal("the ghost's label never reached the rendered frame")
	}

	canvas := NewCanvas(114, 22)
	now := time.Unix(0, replay.cosmosNowNS)
	view := time.Unix(0, replay.cosmosViewNS)
	graph := replay.viewGraph()
	replay.drawCosmosUniverse(canvas, graph, now, view)
	nodes := cosmosNodeMap(graph.Nodes)
	frame := cosmosLayout(canvas, replay.cosmosSeats, nodes, graph.Edges, now, replay.skyEnabled, replay.classicSky, replay.cosmosFocus)
	point, ok := frame.points[deadKey]
	if !ok {
		t.Fatal("the layout seated no point for the dead node")
	}
	colX, colY := int(point.x)/2, int(point.y)/4
	if colX < 0 || colX >= canvas.Cols || colY < 0 || colY >= canvas.Rows {
		t.Fatalf("the ghost's glyph landed off-canvas at (%d,%d)", colX, colY)
	}
	got := canvas.cells[colY*canvas.Cols+colX].fg
	want := scaleRGB(cosmosNodeColor(deadNode), 0.35)
	if got != want {
		t.Fatalf("replayed dead glyph color = %#v, want the static ghost color %#v (not the alarm blink)", got, want)
	}
}

// U6: TestChronoscopeEmptyPastNamesTheMomentNotTheLedger pins the three
// distinct broken/empty states a replay can be in, so an operator never
// mistakes one for another.
func TestChronoscopeEmptyPastNamesTheMomentNotTheLedger(t *testing.T) {
	t.Run("scrubbed before the first event names the moment", func(t *testing.T) {
		model := cosmosGoldenModel(80)
		model.cosmosEvents = []shared.CommsEvent{}
		model.rebuildCosmosTimeline()
		pressed, _ := applyKey(t, model, printableKey('['))
		text := ansi.Strip(pressed.renderCosmosPanel(80, 9))
		at := time.Unix(0, pressed.cosmosViewNS).Format("15:04:05")
		if !strings.Contains(text, "no chat traffic recorded before "+at) {
			t.Fatalf("panel = %q, want it to name the moment %q", text, at)
		}
		if strings.Contains(text, "no chat traffic recorded yet") {
			t.Fatal("empty replay read as the never-sampled-ledger message")
		}
		if strings.Contains(text, "unreachable") {
			t.Fatal("a healthy empty replay claimed the ledger was unreachable")
		}
	})

	t.Run("ledger health is a now-fact and rides over the replay", func(t *testing.T) {
		model := cosmosGoldenModel(80)
		model.cosmosEvents = []shared.CommsEvent{}
		model.rebuildCosmosTimeline()
		pressed, _ := applyKey(t, model, printableKey('['))
		pressed.cosmos.Err = "sqlite: database is locked"
		text := ansi.Strip(pressed.renderCosmosPanel(80, 9))
		if !strings.Contains(text, "⚠ comms ledger unreachable: sqlite: database is locked") {
			t.Fatalf("panel = %q, want the unreachable banner even while replaying", text)
		}
		if strings.Contains(text, "no chat traffic recorded before") {
			t.Fatal("the unreachable banner did not take priority over the empty-past message")
		}
	})

	t.Run("a truncation warning survives the cut into the replay graph", func(t *testing.T) {
		model := cosmosGoldenModel(80)
		model.cosmosEvents = cosmosGoldenEvents(model.cosmosNowNS, model.rows)
		model.rebuildCosmosTimeline()
		model.cosmos.Warnings = append(model.cosmos.Warnings, compose.CosmosTruncationWarning)
		pressed, _ := applyKey(t, model, printableKey('['))
		if pressed.cosmosPast == nil {
			t.Fatal("[ did not enter replay")
		}
		found := false
		for _, warning := range pressed.cosmosPast.Warnings {
			if warning == compose.CosmosTruncationWarning {
				found = true
			}
		}
		if !found {
			t.Fatalf("replay graph did not inherit the live truncation warning: %#v", pressed.cosmosPast.Warnings)
		}
	})
}

// U7: TestNavigatorSelectionCyclesAndReconciles pins ↑/↓ selection order,
// wraparound, the reticle's HUD card, and the reconcile-not-retarget rule:
// a refresh that drops the selected node clears the reticle instead of
// jumping it to whatever remains.
func TestNavigatorSelectionCyclesAndReconciles(t *testing.T) {
	model := cosmosTestModel(120, true)
	model.tab = TabCosmos
	nodes := model.viewGraph().Nodes
	if len(nodes) != 2 {
		t.Fatalf("setup: want exactly 2 nodes, got %d: %#v", len(nodes), nodes)
	}

	down1, _ := applyKey(t, model, specialKey(tea.KeyDown))
	if down1.cosmosSelected != nodes[0].Key {
		t.Fatalf("first ↓ selected %q, want the first visible node %q", down1.cosmosSelected, nodes[0].Key)
	}
	down2, _ := applyKey(t, down1, specialKey(tea.KeyDown))
	if down2.cosmosSelected != nodes[1].Key {
		t.Fatalf("second ↓ selected %q, want the second node %q", down2.cosmosSelected, nodes[1].Key)
	}
	down3, _ := applyKey(t, down2, specialKey(tea.KeyDown))
	if down3.cosmosSelected != nodes[0].Key {
		t.Fatalf("third ↓ did not wrap: got %q, want %q", down3.cosmosSelected, nodes[0].Key)
	}

	fresh := cosmosTestModel(120, true)
	fresh.tab = TabCosmos
	up, _ := applyKey(t, fresh, specialKey(tea.KeyUp))
	if up.cosmosSelected != nodes[len(nodes)-1].Key {
		t.Fatalf("↑ from no selection = %q, want the last node %q", up.cosmosSelected, nodes[len(nodes)-1].Key)
	}

	hud := down2.cosmosSelectionHUD()
	if !strings.Contains(hud, "RR") || !strings.Contains(hud, "codex") || !strings.Contains(hud, "alpha") ||
		!strings.Contains(hud, "in 1") || !strings.Contains(hud, "out 0") {
		t.Fatalf("cosmosSelectionHUD() = %q, missing label/engine/home/in/out", hud)
	}

	newSnapshot := fixtureSnapshot(120)
	newSnapshot.Cosmos = compose.BuildCosmos(newSnapshot.Rows, []shared.CommsEvent{{
		AtNS: fixtureNowNS, Kind: shared.KindInject,
		SenderUUID: newSnapshot.Rows[1].ID, Target: newSnapshot.Rows[4].Name, ReceiverSocket: "",
		Message: "unrelated traffic",
	}}, fixtureNowNS)
	updated, _ := down1.Update(RefreshMsg{Snapshot: newSnapshot})
	refreshed := updated.(Model)
	if refreshed.cosmosSelected != "" {
		t.Fatalf("a refresh that dropped the selected node left cosmosSelected = %q, want it cleared", refreshed.cosmosSelected)
	}
	stillThere := false
	for _, node := range refreshed.cosmos.Nodes {
		if node.Key == nodes[1].Key {
			stillThere = true
		}
	}
	if !stillThere {
		t.Fatal("setup: the OTHER node should have survived the refresh, proving this was a reconcile, not a wipe")
	}
}

// U8: TestNavigatorEnterOpensTheSelectedRow pins `enter` as the Chats tab's
// own door reached from the sky: it opens a live row, refuses a ghost and a
// row the fleet no longer lists by name, and refuses silence when nothing
// is selected at all.
func TestNavigatorEnterOpensTheSelectedRow(t *testing.T) {
	model := cosmosTestModel(120, true)
	model.tab = TabCosmos
	var rrKey, rrRowID string
	for _, node := range model.viewGraph().Nodes {
		if node.RowID != "" {
			rrKey, rrRowID = node.Key, node.RowID
		}
	}
	if rrKey == "" {
		t.Fatal("setup: no node in the fixture graph carries a RowID")
	}
	model.cosmosSelected = rrKey

	opened, cmd := applyKey(t, model, specialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("enter on a live node returned no command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("enter on a live node did not quit: cmd() = %#v", cmd())
	}
	result := opened.Result()
	if result.Kind != OutcomeSelected || result.Row.ID != rrRowID {
		t.Fatalf("Result() = %#v, want OutcomeSelected with Row.ID %q", result, rrRowID)
	}

	ghostModel := cosmosTestModel(120, true)
	ghostModel.tab = TabCosmos
	ghostModel.cosmos.Nodes = append(ghostModel.cosmos.Nodes, compose.CosmosNode{
		Key: "chat:name:ghost", Label: resolve.Named("ghost"),
	})
	ghostModel.cosmosSelected = "chat:name:ghost"
	refusedGhost, ghostCmd := applyKey(t, ghostModel, specialKey(tea.KeyEnter))
	if ghostCmd != nil {
		t.Fatal("enter on a ghost node returned a quit command")
	}
	if !strings.Contains(refusedGhost.cosmosStatus, "ghost") {
		t.Fatalf("cosmosStatus = %q, want it to name the ghost", refusedGhost.cosmosStatus)
	}
	if refusedGhost.outcome == OutcomeSelected {
		t.Fatal("enter on a ghost set OutcomeSelected anyway")
	}

	noneModel := cosmosTestModel(120, true)
	noneModel.tab = TabCosmos
	refusedNone, noneCmd := applyKey(t, noneModel, specialKey(tea.KeyEnter))
	if noneCmd != nil {
		t.Fatal("enter with nothing selected returned a quit command")
	}
	if !strings.Contains(refusedNone.cosmosStatus, "nothing selected") {
		t.Fatalf("cosmosStatus = %q, want it to name that nothing is selected", refusedNone.cosmosStatus)
	}
}

// U9: TestNavigatorSpotlightDimsEveryOtherEdge pins the reticle's spotlight:
// selecting a node keeps ITS edges at full strength and dims every other
// edge to 0.45×, and draws a halo of light around the selected body.
func TestNavigatorSpotlightDimsEveryOtherEdge(t *testing.T) {
	snapshot := fixtureSnapshot(120)
	snapshot.NoSky = true // isolate the plain rail Bezier from comet/wind/twinkle noise
	rows := snapshot.Rows
	touching := shared.CommsEvent{
		AtNS: fixtureNowNS - int64(time.Minute), Kind: shared.KindInject,
		SenderUUID: rows[0].ID, Target: rows[1].Name, ReceiverSocket: rows[1].Socket,
		Message: "touches the selection",
	}
	unrelated := shared.CommsEvent{
		AtNS: fixtureNowNS - int64(time.Minute), Kind: shared.KindInject,
		SenderUUID: rows[4].ID, Target: rows[5].Name,
		Message: "does not touch the selection",
	}
	snapshot.Cosmos = compose.BuildCosmos(rows, []shared.CommsEvent{touching, unrelated}, fixtureNowNS)
	model := NewModel(snapshot)
	model.tab = TabCosmos
	selectedKey := "chat:id:" + rows[0].ID

	graph := model.cosmos
	nodes := cosmosNodeMap(graph.Nodes)
	now := time.Unix(0, model.cosmosNowNS)
	frame := cosmosLayout(NewCanvas(114, 22), model.cosmosSeats, nodes, graph.Edges, now, false, false, "")

	var touchingEdge, unrelatedEdge compose.CosmosEdge
	for _, edge := range graph.Edges {
		if edge.From == selectedKey || edge.To == selectedKey {
			touchingEdge = edge
		} else {
			unrelatedEdge = edge
		}
	}
	if touchingEdge.Kind == "" || unrelatedEdge.Kind == "" {
		t.Fatalf("setup: expected one touching and one unrelated edge: %#v", graph.Edges)
	}

	midpoint := func(edge compose.CosmosEdge) (int, int) {
		fp, tp := frame.points[edge.From], frame.points[edge.To]
		x0, y0, cpx, cpy, x1, y1 := cosmosEdgeRail(fp, tp, frame.cx, frame.cy)
		mx, my := BezierPoint(x0, y0, cpx, cpy, x1, y1, 0.5)
		return int(mx), int(my)
	}
	touchIdx := func() int { x, y := midpoint(touchingEdge); return (y/4)*114 + x/2 }()
	unrelatedIdx := func() int { x, y := midpoint(unrelatedEdge); return (y/4)*114 + x/2 }()

	baseline := NewCanvas(114, 22)
	model.drawCosmosUniverse(baseline, graph, now, now)

	selected := model
	selected.cosmosSelected = selectedKey
	spotlighted := NewCanvas(114, 22)
	selected.drawCosmosUniverse(spotlighted, graph, now, now)

	if spotlighted.dotLuma[touchIdx] != baseline.dotLuma[touchIdx] {
		t.Fatalf("the selected node's OWN edge changed brightness when selected: before=%v after=%v",
			baseline.dotLuma[touchIdx], spotlighted.dotLuma[touchIdx])
	}
	if spotlighted.dotLuma[unrelatedIdx] >= baseline.dotLuma[unrelatedIdx] {
		t.Fatalf("an edge NOT touching the selection was not dimmed: before=%v after=%v",
			baseline.dotLuma[unrelatedIdx], spotlighted.dotLuma[unrelatedIdx])
	}

	point := frame.points[selectedKey]
	haloHits := 0
	for index := 0; index < 12; index++ {
		angle := 2 * math.Pi * float64(index) / 12
		px := int(point.x + 2.2*math.Cos(angle)*1.6)
		py := int(point.y + 2.2*math.Sin(angle))
		if px < 0 || py < 0 || px >= spotlighted.PW() || py >= spotlighted.PH() {
			continue
		}
		idx := (py/4)*spotlighted.Cols + px/2
		if spotlighted.dots[idx] != 0 {
			haloHits++
		}
	}
	if haloHits < 6 {
		t.Fatalf("halo dots around the selected node = %d/12 hit, want most of the ring lit", haloHits)
	}
}

// U10: TestSystemFocusCyclesAndPushesOtherStarsOffFrame pins `s`: the
// focused star takes the galactic centre, every OTHER system's chats move
// off-canvas (never simply vanish), and the subheader counts what is drawn.
func TestSystemFocusCyclesAndPushesOtherStarsOffFrame(t *testing.T) {
	snapshot := fixtureSnapshot(120)
	rows := snapshot.Rows
	intraAlpha := shared.CommsEvent{
		AtNS: fixtureNowNS, Kind: shared.KindInject,
		SenderUUID: rows[0].ID, Target: rows[1].Name, ReceiverSocket: rows[1].Socket,
		Message: "inside alpha",
	}
	crossToBeta := shared.CommsEvent{
		AtNS: fixtureNowNS, Kind: shared.KindSpawn,
		SenderUUID: rows[0].ID, Target: rows[4].Name,
		Message: "leaves for beta",
	}
	snapshot.Cosmos = compose.BuildCosmos(rows, []shared.CommsEvent{intraAlpha, crossToBeta}, fixtureNowNS)
	model := NewModel(snapshot)
	model.tab = TabCosmos

	focused, _ := applyKey(t, model, printableKey('s'))
	if focused.cosmosFocus != "alpha" {
		t.Fatalf("first s focused %q, want the first sorted home %q", focused.cosmosFocus, "alpha")
	}

	canvas := NewCanvas(114, 22)
	now := time.Unix(0, focused.cosmosNowNS)
	graph := focused.viewGraph()
	nodes := cosmosNodeMap(graph.Nodes)
	frame := cosmosLayout(canvas, focused.cosmosSeats, nodes, graph.Edges, now, focused.skyEnabled, focused.classicSky, focused.cosmosFocus)
	if frame.starPoints["alpha"] != (cosmosPoint{x: frame.cx, y: frame.cy}) {
		t.Fatalf("focused star = %#v, want the galactic centre (%v,%v)", frame.starPoints["alpha"], frame.cx, frame.cy)
	}
	for _, node := range graph.Nodes {
		if node.Home != "beta" {
			continue
		}
		if !frame.hidden[node.Key] {
			t.Fatalf("node %q of the unfocused system beta was not marked hidden", node.Key)
		}
		point := frame.points[node.Key]
		if point.x >= 0 && point.x < float64(canvas.PW()) && point.y >= 0 && point.y < float64(canvas.PH()) {
			t.Fatalf("hidden node %q still landed inside the frame: %#v", node.Key, point)
		}
	}

	want := " cosmos · ⌖ alpha · 2 chats · 1 edges · 1 leave the system · last 24h"
	if got := focused.renderCosmosSubheader(); got != want {
		t.Fatalf("subheader = %q, want %q", got, want)
	}

	toBeta, _ := applyKey(t, focused, printableKey('s'))
	if toBeta.cosmosFocus != "beta" {
		t.Fatalf("second s = %q, want the next sorted home %q", toBeta.cosmosFocus, "beta")
	}
	toWhole, _ := applyKey(t, toBeta, printableKey('s'))
	if toWhole.cosmosFocus != "" {
		t.Fatalf("s past the last home = %q, want the whole sky \"\"", toWhole.cosmosFocus)
	}
}

// U11: TestKeplerRingsSplitACrowdedSystemByActivity pins the ring split: a
// system past cosmosRingCap planets splits by activity, the busiest six
// take the inner track, physics slows the outer ring, a ring CHANGE eases
// rather than teleports, and the inner ring's revolution accumulates faster.
func TestKeplerRingsSplitACrowdedSystemByActivity(t *testing.T) {
	model := cosmosTestModel(120, true)
	nodes := make([]compose.CosmosNode, 9)
	for i := range nodes {
		name := "crowd" + string(rune('0'+i))
		nodes[i] = compose.CosmosNode{
			Key:    "chat:name:" + name,
			Label:  resolve.Named(name),
			Home:   "crowded",
			LastNS: int64(8 - i), // crowd0 newest ... crowd8 oldest
		}
	}
	model.cosmos.Nodes = nodes
	model.cosmos.Edges = nil
	model.mergeCosmosSeats()

	innerFactor := cosmosRingFactor(0, 2)
	outerFactor := cosmosRingFactor(1, 2)
	for i, node := range nodes {
		seat := model.cosmosSeats[node.Key]
		if seat == nil {
			t.Fatalf("no seat for %q", node.Key)
		}
		want := innerFactor
		if i >= 6 {
			want = outerFactor
		}
		if seat.RingTarget != want {
			t.Fatalf("node %d (LastNS=%d) RingTarget = %v, want %v", i, node.LastNS, seat.RingTarget, want)
		}
	}
	if cosmosRingSpeed(innerFactor) <= cosmosRingSpeed(outerFactor) {
		t.Fatalf("inner ring speed %v should exceed outer ring speed %v (Kepler III)", cosmosRingSpeed(innerFactor), cosmosRingSpeed(outerFactor))
	}

	canvas := NewCanvas(114, 22)
	now := time.Unix(0, model.cosmosNowNS)
	frame := cosmosLayout(canvas, model.cosmosSeats, cosmosNodeMap(nodes), nil, now, false, false, "")
	rings := frame.rings["crowded"]
	if len(rings) != 2 || rings[0] != innerFactor || rings[1] != outerFactor {
		t.Fatalf("rings[\"crowded\"] = %v, want [%v %v] sorted ascending", rings, innerFactor, outerFactor)
	}

	// Orbit accumulates faster on the inner ring: advance both an inner and
	// an outer seat by the same dt and compare how far each revolved.
	inner, outer := model.cosmosSeats[nodes[0].Key], model.cosmosSeats[nodes[8].Key]
	innerBefore, outerBefore := inner.Orbit, outer.Orbit
	model.advanceCosmos(model.cosmosNowNS + int64(time.Second))
	if delta := inner.Orbit - innerBefore; delta <= outer.Orbit-outerBefore {
		t.Fatalf("inner ring orbit delta %v did not exceed outer ring orbit delta %v", delta, outer.Orbit-outerBefore)
	}

	// A promotion to a new ring eases (Ring moves toward RingTarget) instead
	// of teleporting: demote the hottest ring back to the outer track and
	// re-merge, then check the physical Ring lags RingTarget right after.
	hotKey := nodes[0].Key
	promoted := model.cosmosSeats[hotKey]
	if promoted.Ring != promoted.RingTarget {
		t.Fatalf("setup: a freshly created seat should start settled: %#v", promoted)
	}
	nodes[0].LastNS = -100 // now the quietest: demoted to the outer ring
	model.cosmos.Nodes = nodes
	model.mergeCosmosSeats()
	reseated := model.cosmosSeats[hotKey]
	if reseated.RingTarget != outerFactor {
		t.Fatalf("setup: demoted node RingTarget = %v, want %v", reseated.RingTarget, outerFactor)
	}
	if reseated.Ring == reseated.RingTarget {
		t.Fatalf("a ring change snapped instantly instead of easing: Ring already equals RingTarget (%v)", reseated.Ring)
	}
	model.advanceCosmos(model.cosmosNowNS + int64(200*time.Millisecond))
	if reseated.Ring == innerFactor || reseated.Ring == outerFactor {
		t.Fatalf("after a small tick the seat already reached an endpoint (%v) instead of easing partway between %v and %v", reseated.Ring, innerFactor, outerFactor)
	}
}

// U12: TestLabelNudgeStepsAroundTextAndNeverLeavesTheCanvas pins
// Canvas.TextFree's collision table and the label nudge it drives: a
// colliding label steps to another row, and a label pinned at row 0 can
// never nudge to the nonexistent row -1 — it accepts the overlap instead of
// vanishing.
func TestLabelNudgeStepsAroundTextAndNeverLeavesTheCanvas(t *testing.T) {
	t.Run("TextFree table", func(t *testing.T) {
		canvas := NewCanvas(10, 5)
		canvas.Text(2, 2, "AB", RGB{1, 1, 1}, false)
		canvas.Dot(0, 0, RGB{1, 1, 1}) // braille-only cell in row 0

		tests := []struct {
			name    string
			x, y, n int
			want    bool
		}{
			{"a free row is free", 5, 2, 2, true},
			{"an occupied row is not free", 2, 2, 2, false},
			{"a braille-only cell counts as free", 0, 0, 1, true},
			{"a row below the canvas counts as free", 0, 99, 1, true},
			{"a row above the canvas counts as free", 0, -1, 1, true},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				if got := canvas.TextFree(test.x, test.y, test.n); got != test.want {
					t.Fatalf("TextFree(%d,%d,%d) = %v, want %v", test.x, test.y, test.n, got, test.want)
				}
			})
		}
	})

	t.Run("colliding labels land on different rows", func(t *testing.T) {
		model := cosmosTestModel(60, true)
		model.classicSky = true
		model.cosmos.Nodes = []compose.CosmosNode{
			{Key: "chat:name:AAAAAAAAAAAAAA", Label: resolve.Named("AAAAAAAAAAAAAA")},
			{Key: "chat:name:BBBBBBBBBBBBBB", Label: resolve.Named("BBBBBBBBBBBBBB")},
		}
		model.cosmos.Edges = nil
		model.mergeCosmosSeats()
		for _, seat := range model.cosmosSeats {
			seat.Angle, seat.Target = 0, 0 // force both nodes onto the exact same point
		}

		canvas := NewCanvas(60, 20)
		now := time.Unix(0, model.cosmosNowNS)
		model.drawCosmosUniverse(canvas, model.cosmos, now, now)
		lines := strings.Split(ansi.Strip(canvas.render()), "\n")

		lineOf := func(needle string) int {
			for index, line := range lines {
				if strings.Contains(line, needle) {
					return index
				}
			}
			return -1
		}
		rowA, rowB := lineOf("AAAA"), lineOf("BBBB")
		if rowA < 0 || rowB < 0 {
			t.Fatalf("one of the colliding labels never rendered at all: rowA=%d rowB=%d\n%s", rowA, rowB, strings.Join(lines, "\n"))
		}
		if rowA == rowB {
			t.Fatalf("colliding labels landed on the SAME row %d instead of nudging apart", rowA)
		}
	})

	t.Run("a label pinned at row 0 accepts the overlap instead of vanishing", func(t *testing.T) {
		model := cosmosTestModel(60, true)
		model.classicSky = true
		model.cosmos.Nodes = []compose.CosmosNode{
			{Key: "chat:name:AAAA", Label: resolve.Named("AAAA")},
			{Key: "chat:name:BBBB", Label: resolve.Named("BBBB")},
			{Key: "chat:name:CCCC", Label: resolve.Named("CCCC")},
		}
		model.cosmos.Edges = nil
		model.mergeCosmosSeats()
		for _, seat := range model.cosmosSeats {
			seat.Angle, seat.Target = -math.Pi/2, -math.Pi/2 // top of the ellipse
		}

		canvas := NewCanvas(60, 7) // short enough that the top seat lands on row 0
		now := time.Unix(0, model.cosmosNowNS)
		model.drawCosmosUniverse(canvas, model.cosmos, now, now)
		frame := cosmosLayout(canvas, model.cosmosSeats, cosmosNodeMap(model.cosmos.Nodes), nil, now, false, true, "")
		point := frame.points["chat:name:AAAA"]
		colY := int(point.y) / 4
		if colY != 0 {
			t.Skipf("setup: forced point landed on row %d, not row 0 — geometry drifted, adjust the canvas height", colY)
		}
		lines := strings.Split(ansi.Strip(canvas.render()), "\n")
		found := false
		for _, line := range lines {
			if strings.Contains(line, "CCCC") {
				found = true
			}
		}
		if !found {
			t.Fatalf("the third colliding label at row 0 vanished instead of accepting the overlap:\n%s", strings.Join(lines, "\n"))
		}
	})
}

// U13: TestCosmosLegendNamesOnlyWhatTheSkyDraws pins the top-right legend:
// the orbital sky names the sun glyph, the classic sky (no suns) does not,
// and a canvas too narrow for the legend plus its margin draws none of it.
func TestCosmosLegendNamesOnlyWhatTheSkyDraws(t *testing.T) {
	orbital := ansi.Strip(cosmosGoldenModel(80).renderCosmosPanel(80, 9))
	if !strings.Contains(orbital, "✹ project") {
		t.Fatalf("orbital legend missing the sun glyph:\n%s", orbital)
	}
	classic := ansi.Strip(cosmosClassicGoldenModel(80).renderCosmosPanel(80, 9))
	if strings.Contains(classic, "✹ project") {
		t.Fatalf("classic legend still names the sun glyph:\n%s", classic)
	}
	if !strings.Contains(classic, "◆ opencode") {
		t.Fatalf("classic legend dropped the engine glyphs entirely:\n%s", classic)
	}
	narrow := ansi.Strip(cosmosGoldenModel(80).renderCosmosPanel(60, 9))
	if strings.Contains(narrow, "claude  ▲ codex") {
		t.Fatalf("a canvas too narrow for the legend drew it anyway:\n%s", narrow)
	}
}

// U14: TestCosmosKeysNeverGoSilent pins the no-silent-keystroke contract for
// the cosmos tab's own keys: on an empty sky, every refusal names itself on
// cosmosStatus, and the two unconditional keys (o, n) still change state.
func TestCosmosKeysNeverGoSilent(t *testing.T) {
	refusals := []string{"up", "down", "enter", "s", "[", "]", "{", "}", "space"}
	for _, key := range refusals {
		t.Run(key, func(t *testing.T) {
			model := NewModel(fixtureSnapshot(80))
			model.tab = TabCosmos
			pressed, _ := applyKey(t, model, specialOrPrintable(key))
			if pressed.cosmosStatus == "" {
				t.Fatalf("key %q on an empty sky went silent: no cosmosStatus set", key)
			}
		})
	}

	t.Run("o", func(t *testing.T) {
		model := NewModel(fixtureSnapshot(80))
		model.tab = TabCosmos
		pressed, _ := applyKey(t, model, printableKey('o'))
		if pressed.classicSky == model.classicSky {
			t.Fatal("o on an empty sky did not toggle classicSky")
		}
	})

	t.Run("n", func(t *testing.T) {
		model := NewModel(fixtureSnapshot(80))
		model.tab = TabCosmos
		model.cosmosPast = &compose.CosmosGraph{}
		model.cosmosViewNS = model.cosmosNowNS - int64(5*time.Minute)
		pressed, _ := applyKey(t, model, printableKey('n'))
		if pressed.cosmosPast != nil {
			t.Fatal("n on a scrubbed empty sky did not return to live")
		}
	})
}

// specialOrPrintable builds the right tea.KeyMsg for a cosmos key string,
// matching how the model itself would receive it from a real keyboard.
func specialOrPrintable(key string) tea.KeyPressMsg {
	switch key {
	case "up":
		return specialKey(tea.KeyUp)
	case "down":
		return specialKey(tea.KeyDown)
	case "enter":
		return specialKey(tea.KeyEnter)
	case "space":
		return printableKey(' ')
	default:
		return printableKey(rune(key[0]))
	}
}
