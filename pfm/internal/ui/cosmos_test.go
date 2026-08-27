package ui

import (
	"context"
	"errors"
	"math"
	bits2 "math/bits"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"hostops/pfm/internal/compose"
	"hostops/pfm/internal/resolve"
	"hostops/pfm/internal/shared"
)

type countingCosmosSampler struct {
	events []shared.CommsEvent
	err    error
	calls  int
	since  int64
}

func (sampler *countingCosmosSampler) Sample(_ context.Context, sinceNS int64) ([]shared.CommsEvent, error) {
	sampler.calls++
	sampler.since = sinceNS
	return append([]shared.CommsEvent(nil), sampler.events...), sampler.err
}

func TestCosmosCanvasQuantizesEveryColorWithoutCursorAddressing(t *testing.T) {
	canvas := NewCanvas(2, 1)
	canvas.SetCell(0, 0, 'X', RGB{101, 102, 103}, true)
	frame := canvas.render()
	if strings.Contains(frame, "\x1b[1;1H") {
		t.Fatalf("canvas frame used cursor addressing: %q", frame)
	}
	if !strings.Contains(frame, "\x1b[1;38;2;104;104;104mX") {
		t.Fatalf("canvas color was not step-8 quantized: %q", frame)
	}
	if !strings.HasSuffix(frame, " \x1b[0m") {
		t.Fatalf("canvas did not return one complete row: %q", frame)
	}
}

func TestCosmosBrokenStatesRemainDistinct(t *testing.T) {
	snapshot := fixtureSnapshot(80)
	snapshot.NoSky = true

	empty := NewModel(snapshot)
	empty.tab = TabCosmos
	emptyText := ansi.Strip(empty.renderCosmosPanel(80, 9))
	if !strings.Contains(emptyText, "no chat traffic recorded yet") || strings.Contains(emptyText, "unreachable") {
		t.Fatalf("healthy empty cosmos was not distinct:\n%s", emptyText)
	}

	snapshot.Cosmos.Err = "sqlite: database is locked"
	broken := NewModel(snapshot)
	broken.tab = TabCosmos
	brokenText := ansi.Strip(broken.renderCosmosPanel(80, 9))
	if !strings.Contains(brokenText, "⚠ comms ledger unreachable: sqlite: database is locked") ||
		strings.Contains(brokenText, "no chat traffic recorded yet") {
		t.Fatalf("ledger failure was not distinct:\n%s", brokenText)
	}

	snapshot.Cosmos.Err = ""
	snapshot.Cosmos.Warnings = []string{compose.CosmosTruncationWarning}
	warned := NewModel(snapshot)
	warned.tab = TabCosmos
	warnedText := ansi.Strip(warned.renderCosmosPanel(80, 9))
	if !strings.Contains(warnedText, compose.CosmosTruncationWarning) || strings.Contains(warnedText, "unreachable") {
		t.Fatalf("truncated cosmos was not distinct:\n%s", warnedText)
	}
}

func TestCosmosTinyPaneFallsBackToNewestEdges(t *testing.T) {
	model := cosmosTestModel(19, true)
	plain := ansi.Strip(model.renderCosmosPanel(19, 7))
	if !strings.Contains(plain, "RR") || !strings.Contains(plain, "hello cosmos") {
		t.Fatalf("tiny pane dropped truthful edge detail:\n%s", plain)
	}
}

func TestCosmosParentlessSpawnIsNotEmptyInEitherLayout(t *testing.T) {
	snapshot := fixtureSnapshot(80)
	snapshot.NoSky = true
	snapshot.Cosmos = compose.BuildCosmos(snapshot.Rows, []shared.CommsEvent{{
		AtNS: fixtureNowNS, Kind: shared.KindSpawn, Target: "orphan",
		ReceiverSocket: "cx-orphan",
	}}, fixtureNowNS)
	if len(snapshot.Cosmos.Nodes) != 1 || len(snapshot.Cosmos.Edges) != 0 {
		t.Fatalf("parentless spawn fixture = %#v, want one node and zero edges", snapshot.Cosmos)
	}
	model := NewModel(snapshot)
	model.tab = TabCosmos

	tests := []struct {
		name          string
		width, height int
	}{
		{name: "full", width: 80, height: 9},
		{name: "compact", width: 19, height: 7},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plain := ansi.Strip(model.renderCosmosPanel(test.width, test.height))
			if strings.Contains(plain, "no chat traffic") {
				t.Fatalf("parentless spawn rendered as empty universe:\n%s", plain)
			}
		})
	}
}

func TestCosmosSamplerIsLazyAndRetainsGraphOnFailure(t *testing.T) {
	sampler := &countingCosmosSampler{events: []shared.CommsEvent{{
		AtNS: fixtureNowNS - int64(time.Second), Kind: shared.KindInject,
		SenderUUID: "11111111-1111-4111-8111-111111111111",
		Target:     "RR", Message: "fresh",
	}}}
	snapshot := fixtureSnapshot(120)
	snapshot.NoSky = true
	snapshot.CosmosSampler = sampler
	model := NewModel(snapshot)
	if sampler.calls != 0 {
		t.Fatalf("picker boot sampled cosmos: calls=%d", sampler.calls)
	}

	for range 3 {
		var command tea.Cmd
		model, command = applyKey(t, model, specialKey(tea.KeyTab))
		if model.tab != TabCosmos && sampler.calls != 0 {
			t.Fatalf("non-cosmos tab sampled cosmos: tab=%d calls=%d", model.tab, sampler.calls)
		}
		if model.tab == TabCosmos {
			if command == nil {
				t.Fatal("entering cosmos did not start its sampler")
			}
			message := command()
			updated, _ := model.Update(message)
			model = updated.(Model)
		}
	}
	if sampler.calls != 1 || sampler.since != fixtureNowNS-int64(compose.CosmosWindow) || len(model.cosmos.Edges) != 1 {
		t.Fatalf("cosmos sample calls=%d since=%d graph=%#v", sampler.calls, sampler.since, model.cosmos)
	}

	before := model.cosmos
	model.cosmosLoading = true
	updated, _ := model.Update(cosmosSampleMsg{
		generation: model.statsGeneration,
		err:        errors.New("read failed"),
	})
	model = updated.(Model)
	if model.cosmos.Err != "read failed" || len(model.cosmos.Nodes) != len(before.Nodes) || len(model.cosmos.Edges) != len(before.Edges) {
		t.Fatalf("failed sample did not preserve last graph: before=%#v after=%#v", before, model.cosmos)
	}
}

func TestCosmosTickOnlyRunsWhileFocusedAndSkyEnabled(t *testing.T) {
	model := cosmosTestModel(120, false)
	before := model.cosmosNowNS
	updated, command := model.Update(cosmosTickMsg{
		generation: model.cosmosTickGeneration,
		nowNS:      before + int64(125*time.Millisecond),
	})
	model = updated.(Model)
	if model.cosmosNowNS != before || command != nil {
		t.Fatalf("off-tab cosmos tick advanced clock: now=%d command=%v", model.cosmosNowNS, command)
	}

	model.tab = TabCosmos
	updated, command = model.Update(cosmosTickMsg{
		generation: model.cosmosTickGeneration,
		nowNS:      before + int64(125*time.Millisecond),
	})
	model = updated.(Model)
	if model.cosmosNowNS == before || command == nil {
		t.Fatalf("focused cosmos tick now=%d command=%v", model.cosmosNowNS, command)
	}

	disabled := cosmosTestModel(120, true)
	disabled.tab = TabCosmos
	updated, command = disabled.Update(cosmosTickMsg{
		generation: disabled.cosmosTickGeneration,
		nowNS:      disabled.cosmosNowNS + int64(125*time.Millisecond),
	})
	disabled = updated.(Model)
	if command != nil || disabled.cosmosNowNS != fixtureNowNS {
		t.Fatalf("NoSky cosmos tick advanced: now=%d command=%v", disabled.cosmosNowNS, command)
	}
}

func TestCosmosRapidAwayAndBackDropsTheOldAnimationChain(t *testing.T) {
	model := cosmosTestModel(120, false)
	model.tab = TabCosmos
	model.cosmosTickGeneration = 4
	staleGeneration := model.cosmosTickGeneration

	left, _ := model.switchTab(1)
	model = left.(Model)
	returned, command := model.switchTab(-1)
	model = returned.(Model)
	if model.tab != TabCosmos || command == nil || model.cosmosTickGeneration == staleGeneration {
		t.Fatalf("rapid return did not start a fresh cosmos chain: tab=%d generation=%d command=%v", model.tab, model.cosmosTickGeneration, command)
	}

	before := model.cosmosNowNS
	updated, staleCommand := model.Update(cosmosTickMsg{
		generation: staleGeneration,
		nowNS:      before + int64(125*time.Millisecond),
	})
	model = updated.(Model)
	if model.cosmosNowNS != before || staleCommand != nil {
		t.Fatalf("stale cosmos chain survived rapid return: now=%d command=%v", model.cosmosNowNS, staleCommand)
	}
}

func TestCosmosSeatSurvivesGraphRefreshAndSpawnStartsAtParent(t *testing.T) {
	model := cosmosTestModel(120, true)
	parent := "chat:id:11111111-1111-4111-8111-111111111111"
	model.cosmosSeats[parent].Angle = 0.42
	model.cosmosSeats[parent].Target = 0.42
	child := compose.CosmosNode{Key: "chat:name:child", Label: resolve.Named("child")}
	model.cosmos.Nodes = append(model.cosmos.Nodes, child)
	model.cosmos.Edges = append(model.cosmos.Edges, compose.CosmosEdge{
		From: parent, To: child.Key, Kind: shared.KindSpawn, LastNS: fixtureNowNS,
	})
	model.mergeCosmosSeats()
	if model.cosmosSeats[parent].Angle != 0.42 {
		t.Fatalf("parent seat reset across merge: %#v", model.cosmosSeats[parent])
	}
	if model.cosmosSeats[child.Key].Angle != 0.42 {
		t.Fatalf("spawn child was not born at parent angle: %#v", model.cosmosSeats[child.Key])
	}
}

// The planetary law: every moon of one parent shares ONE orbit — the same
// aspect-corrected distance from the parent, evenly spaced from its siblings,
// the ring widening with the brood. --no-sky holds the system perfectly
// still; the sky rotates it slowly around the parent.
func TestCosmosMoonsShareOneEvenlySpacedOrbit(t *testing.T) {
	model := cosmosTestModel(120, true)
	parent := "chat:id:11111111-1111-4111-8111-111111111111"
	parentHome := ""
	for _, node := range model.cosmos.Nodes {
		if node.Key == parent {
			parentHome = node.Home
		}
	}
	moonA := compose.CosmosNode{Key: "chat:name:moon-a", Label: resolve.Named("moon-a"), Home: parentHome}
	moonB := compose.CosmosNode{Key: "chat:name:moon-b", Label: resolve.Named("moon-b"), Home: parentHome}
	model.cosmos.Nodes = append(model.cosmos.Nodes, moonA, moonB)
	model.cosmos.Edges = append(model.cosmos.Edges,
		compose.CosmosEdge{From: parent, To: moonA.Key, Kind: shared.KindSpawn, LastNS: fixtureNowNS},
		compose.CosmosEdge{From: parent, To: moonB.Key, Kind: shared.KindSpawn, LastNS: fixtureNowNS},
	)
	model.mergeCosmosSeats()
	canvas := NewCanvas(118, 24)
	nodes := cosmosNodeMap(model.cosmos.Nodes)
	now := time.Unix(0, fixtureNowNS)
	points, _, _, _, _ := cosmosLayout(canvas, model.cosmosSeats, nodes, model.cosmos.Edges, now, false)

	anchor := points[parent]
	orbitOf := func(p cosmosPoint) float64 {
		return math.Hypot(p.x-anchor.x, p.y-anchor.y)
	}
	wantOrbit := 5 + 2.5*2
	if got := orbitOf(points[moonA.Key]); math.Abs(got-wantOrbit) > 0.001 {
		t.Fatalf("moon A orbit = %v, want %v", got, wantOrbit)
	}
	if got := orbitOf(points[moonB.Key]); math.Abs(got-wantOrbit) > 0.001 {
		t.Fatalf("moon B orbit = %v, want %v", got, wantOrbit)
	}
	// Two moons evenly spaced means directly opposite: their midpoint is the
	// parent itself.
	midX := (points[moonA.Key].x + points[moonB.Key].x) / 2
	midY := (points[moonA.Key].y + points[moonB.Key].y) / 2
	if math.Abs(midX-anchor.x) > 0.001 || math.Abs(midY-anchor.y) > 0.001 {
		t.Fatalf("moons are not evenly spaced around the parent: mid=(%v,%v) parent=(%v,%v)", midX, midY, anchor.x, anchor.y)
	}

	// --no-sky is a still frame: a later clock renders the identical system.
	later, _, _, _, _ := cosmosLayout(canvas, model.cosmosSeats, nodes, model.cosmos.Edges, now.Add(5*time.Second), false)
	if later[moonA.Key] != points[moonA.Key] {
		t.Fatalf("no-sky moon moved: %#v vs %#v", later[moonA.Key], points[moonA.Key])
	}

	// The sky orbits the moon around its parent — position changes, the
	// orbit distance does not.
	skyLater, _, _, _, _ := cosmosLayout(canvas, model.cosmosSeats, nodes, model.cosmos.Edges, now.Add(5*time.Second), true)
	if skyLater[moonA.Key] == points[moonA.Key] {
		t.Fatalf("sky moon did not orbit")
	}
	skyAnchor := skyLater[parent]
	if got := math.Hypot(skyLater[moonA.Key].x-skyAnchor.x, skyLater[moonA.Key].y-skyAnchor.y); math.Abs(got-wantOrbit) > 0.001 {
		t.Fatalf("orbiting moon left its orbit: %v, want %v", got, wantOrbit)
	}
}

// Residence wins over lineage: a chat spawned into ANOTHER star's system is
// a planet of the star it lives under, never a moon of its parent — the
// spawn edge alone carries the bloodline between the systems.
func TestCosmosCrossStarSpawnIsAPlanetNotAMoon(t *testing.T) {
	nodes := map[string]compose.CosmosNode{
		"chat:name:parent": {Key: "chat:name:parent", Home: ".professor"},
		"chat:name:child":  {Key: "chat:name:child", Home: "intuita"},
		"chat:name:local":  {Key: "chat:name:local", Home: ".professor"},
	}
	edges := []compose.CosmosEdge{
		{From: "chat:name:parent", To: "chat:name:child", Kind: shared.KindSpawn},
		{From: "chat:name:parent", To: "chat:name:local", Kind: shared.KindSpawn},
	}
	parents, children := cosmosOrbits(edges, nodes)
	if parents["chat:name:child"] != "" {
		t.Fatalf("cross-star spawn became a moon of %q, want a free planet of its own star", parents["chat:name:child"])
	}
	if parents["chat:name:local"] != "chat:name:parent" || len(children["chat:name:parent"]) != 1 {
		t.Fatalf("same-star spawn should still be a moon: parents=%v children=%v", parents, children)
	}
}

// Every distinct Home is a star: one project alone holds the galactic
// centre, several share the ring — and every star seats at a distinct point.
func TestCosmosEveryHomeIsAStar(t *testing.T) {
	single := map[string]compose.CosmosNode{
		"chat:name:a": {Key: "chat:name:a", Home: ".professor"},
	}
	order, points := cosmosStarPoints(single, 100, 50, 60, 30)
	if len(order) != 1 || points[".professor"] != (cosmosPoint{x: 100, y: 50}) {
		t.Fatalf("single star should hold the centre: order=%v points=%v", order, points)
	}
	multi := map[string]compose.CosmosNode{
		"chat:name:a": {Key: "chat:name:a", Home: ".professor"},
		"chat:name:b": {Key: "chat:name:b", Home: "intuita"},
		"chat:name:c": {Key: "chat:name:c", Home: "/tmp"},
		"chat:name:g": {Key: "chat:name:g", Group: true},
	}
	order, points = cosmosStarPoints(multi, 100, 50, 60, 30)
	if len(order) != 3 {
		t.Fatalf("want three stars (groups are not stars), got %v", order)
	}
	distinct := map[cosmosPoint]bool{}
	for _, home := range order {
		distinct[points[home]] = true
	}
	if len(distinct) != 3 {
		t.Fatalf("stars share a seat: %v", points)
	}
}

func TestClipCosmosLabelTruncatesVisiblyWithinAvailableSpace(t *testing.T) {
	tests := []struct {
		name      string
		label     string
		rightward bool
		colX      int
		cols      int
		want      string
	}{
		{name: "fits rightward, unchanged", label: "RR", rightward: true, colX: 5, cols: 80, want: "RR"},
		{name: "clips rightward with a visible ellipsis", label: "123456789", rightward: true, colX: 73, cols: 80, want: "1234…"},
		{name: "clips leftward with a visible ellipsis", label: "123456789", rightward: false, colX: 6, cols: 80, want: "1234…"},
		{name: "no space left produces empty, not the raw label", label: "cc-1787827912-1607460-33758", rightward: true, colX: 79, cols: 80, want: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := clipCosmosLabel(test.label, test.rightward, test.colX, test.cols)
			if got != test.want {
				t.Fatalf("clipCosmosLabel(%q, rightward=%v, colX=%d, cols=%d) = %q, want %q",
					test.label, test.rightward, test.colX, test.cols, got, test.want)
			}
		})
	}
}

func TestCosmosDeathVisualBlinksThenFadesThenGoesQuiet(t *testing.T) {
	base := RGB{56, 189, 248} // Codex's own hue
	onColor, onBold := cosmosDeathVisual(base, 0)
	if onColor != base || !onBold {
		t.Fatalf("blink-on at deathAge=0 = %#v bold=%v, want full base color and bold", onColor, onBold)
	}
	offColor, offBold := cosmosDeathVisual(base, 200*time.Millisecond)
	if offColor == base || offBold {
		t.Fatalf("blink-off at 200ms = %#v bold=%v, want dimmed and not bold", offColor, offBold)
	}
	if onColor == offColor {
		t.Fatal("blink phases render identically: it cannot read as an alarm")
	}
	// Engine identity must survive the off phase too — a dimmed dot is
	// still Codex-blue, never grey or tinted toward a fixed alarm color.
	if offColor.B == 0 || offColor.G <= offColor.R {
		t.Fatalf("blink-off color = %#v lost the base hue's blue-dominant shape", offColor)
	}
	fadingColor, fadeBold := cosmosDeathVisual(base, compose.CosmosDeathBlink+compose.CosmosDeathFade/2)
	if fadeBold {
		t.Fatal("fade phase reported bold: an alarm that is fading is not blinking")
	}
	if fadingColor == (RGB{}) {
		t.Fatal("fade midpoint is already fully black")
	}
	goneColor, _ := cosmosDeathVisual(base, compose.CosmosDeathBlink+compose.CosmosDeathFade)
	if goneColor != (RGB{}) {
		t.Fatalf("fade end = %#v, want fully faded to black", goneColor)
	}
}

func cosmosTestModel(width int, noSky bool) Model {
	snapshot := fixtureSnapshot(width)
	snapshot.NoSky = noSky
	snapshot.Cosmos = compose.BuildCosmos(snapshot.Rows, []shared.CommsEvent{{
		AtNS:       fixtureNowNS - int64(300*time.Millisecond),
		Kind:       shared.KindInject,
		SenderUUID: "11111111-1111-4111-8111-111111111111",
		Target:     "RR",
		Message:    "hello cosmos",
	}}, fixtureNowNS)
	return NewModel(snapshot)
}

// TestApplyCosmosGraphKeysDeathToDetectionNotLastMessage is the regression
// for the DiedNS question: compose.BuildCosmos has no memory across calls,
// so its own DiedNS is only ever last edge activity — a chat idle for a
// long stretch before it is killed would already read as long past
// CosmosDeathBlink+CosmosDeathFade on the very first frame that notices
// it, and never animate at all. model.applyCosmosGraph is what fixes that:
// it stamps the death moment from ITS OWN clock the instant it first
// observes a transition from alive to Dead, independent of how old the
// node's last message was.
func TestApplyCosmosGraphKeysDeathToDetectionNotLastMessage(t *testing.T) {
	model := NewModel(fixtureSnapshot(80))
	model.cosmosNowNS = fixtureNowNS
	staleActivity := fixtureNowNS - int64(2*time.Hour)

	// First refresh: alive, idle for two hours already. Idle is not dead.
	model.applyCosmosGraph(compose.CosmosGraph{
		Nodes: []compose.CosmosNode{{Key: "chat:id:X", Label: resolve.Named("X"), LastNS: staleActivity}},
	})
	if len(model.cosmos.Nodes) != 1 || model.cosmos.Nodes[0].Dead {
		t.Fatalf("setup: node should be alive: %#v", model.cosmos.Nodes)
	}

	// Second refresh, the model's own clock 500ms later: the SAME chat is
	// now Dead. Its last message never moved (killing a chat writes no
	// ledger event) — LastNS/compose's own DiedNS are still two hours
	// stale. This render is the first time the model has EVER seen it
	// dead, and that detection moment — not the two-hour-old activity —
	// must be what gets stamped and what keeps the node alive on canvas.
	detectionNS := fixtureNowNS + int64(500*time.Millisecond)
	model.cosmosNowNS = detectionNS
	model.applyCosmosGraph(compose.CosmosGraph{
		Nodes: []compose.CosmosNode{{
			Key: "chat:id:X", Label: resolve.Named("X"), Dead: true,
			LastNS: staleActivity, DiedNS: staleActivity,
		}},
	})
	if len(model.cosmos.Nodes) != 1 {
		t.Fatalf("node vanished on the very frame its death was first observed: %#v", model.cosmos.Nodes)
	}
	got := model.cosmos.Nodes[0]
	if !got.Dead {
		t.Fatalf("node lost its Dead flag: %#v", got)
	}
	if got.DiedNS != detectionNS {
		t.Fatalf("DiedNS = %d, want the detection moment %d (last-message activity was %d)", got.DiedNS, detectionNS, staleActivity)
	}

	// A third refresh just past CosmosDeathBlink+CosmosDeathFade AFTER the
	// detection moment must finally drop it — the window is measured from
	// detection, not from staleActivity (which is already ancient).
	model.cosmosNowNS = detectionNS + int64(compose.CosmosDeathBlink+compose.CosmosDeathFade) + int64(time.Millisecond)
	model.applyCosmosGraph(compose.CosmosGraph{
		Nodes: []compose.CosmosNode{{
			Key: "chat:id:X", Label: resolve.Named("X"), Dead: true,
			LastNS: staleActivity, DiedNS: staleActivity,
		}},
	})
	if len(model.cosmos.Nodes) != 0 {
		t.Fatalf("node survived past its own blink+fade window measured from detection: %#v", model.cosmos.Nodes)
	}
}

// TestApplyCosmosGraphDropsEdgesWithTheirPrunedNode is the edge/node
// consistency invariant, now enforced here since compose.BuildCosmos no
// longer prunes anything by time: once a node is dropped past its own
// blink+fade window, any edge still naming it must be dropped in the same
// pass, or the cosmos header's "N edges" (ui/render.go, len(model.cosmos.
// Edges) raw) would count an edge the canvas has no seat left to draw.
func TestApplyCosmosGraphDropsEdgesWithTheirPrunedNode(t *testing.T) {
	model := NewModel(fixtureSnapshot(80))
	model.cosmosNowNS = fixtureNowNS
	model.applyCosmosGraph(compose.CosmosGraph{
		Nodes: []compose.CosmosNode{{Key: "chat:id:X", Label: resolve.Named("X")}, {Key: "chat:id:Y", Label: resolve.Named("Y")}},
	})

	model.cosmosNowNS = fixtureNowNS + int64(time.Second)
	model.applyCosmosGraph(compose.CosmosGraph{
		Nodes: []compose.CosmosNode{
			{Key: "chat:id:X", Label: resolve.Named("X"), Dead: true, LastNS: model.cosmosNowNS},
			{Key: "chat:id:Y", Label: resolve.Named("Y")},
		},
		Edges: []compose.CosmosEdge{{From: "chat:id:X", To: "chat:id:Y", Kind: shared.KindInject}},
	})
	if len(model.cosmos.Edges) != 1 {
		t.Fatalf("edge missing right after detection, while X is still animating: %#v", model.cosmos)
	}

	model.cosmosNowNS += int64(compose.CosmosDeathBlink+compose.CosmosDeathFade) + int64(time.Millisecond)
	model.applyCosmosGraph(compose.CosmosGraph{
		Nodes: []compose.CosmosNode{
			{Key: "chat:id:X", Label: resolve.Named("X"), Dead: true, LastNS: fixtureNowNS + int64(time.Second)},
			{Key: "chat:id:Y", Label: resolve.Named("Y")},
		},
		Edges: []compose.CosmosEdge{{From: "chat:id:X", To: "chat:id:Y", Kind: shared.KindInject}},
	})
	for _, node := range model.cosmos.Nodes {
		if node.Key == "chat:id:X" {
			t.Fatalf("X should have been dropped past its own blink+fade window: %#v", model.cosmos.Nodes)
		}
	}
	if len(model.cosmos.Edges) != 0 {
		t.Fatalf("edge to a dropped node survived: %#v", model.cosmos.Edges)
	}
	if len(model.cosmos.Nodes) != 1 || model.cosmos.Nodes[0].Key != "chat:id:Y" {
		t.Fatalf("Y should have survived untouched: %#v", model.cosmos.Nodes)
	}
}

// TestApplyCosmosGraphRetiredNodeStaysGoneWhileComposeKeepsEmittingIt is the
// regression for the resurrection bug: compose.BuildCosmos never prunes by
// time, so it keeps emitting a Dead node every frame the ledger still
// carries it — including frames AFTER the node has already blinked, faded,
// and dropped past its own window. A dropped node's cosmosDiedAtNS entry
// must survive that drop so the NEXT frame still recognizes it as already
// retired (known=true, past its window) rather than mistaking it for a
// node never seen before (known=false, previousAlive=false), which lands
// in the undated branch and renders it as permanent dim-static debris —
// the exact "permanent debris" failure this whole feature exists to
// remove, arriving back through a different door one frame later.
func TestApplyCosmosGraphRetiredNodeStaysGoneWhileComposeKeepsEmittingIt(t *testing.T) {
	model := NewModel(fixtureSnapshot(80))
	model.cosmosNowNS = fixtureNowNS

	// Frame 1: alive.
	model.applyCosmosGraph(compose.CosmosGraph{
		Nodes: []compose.CosmosNode{{Key: "chat:session:ghost", Label: resolve.Named("ghost")}},
	})

	// Frame 2: dies, with a baseline (previousAlive), so it is detected and
	// stamped rather than landing in the undated path.
	model.cosmosNowNS = fixtureNowNS + int64(time.Second)
	model.applyCosmosGraph(compose.CosmosGraph{
		Nodes: []compose.CosmosNode{{
			Key: "chat:session:ghost", Label: resolve.Named("ghost"), Dead: true, LastNS: model.cosmosNowNS,
		}},
	})
	if len(model.cosmos.Nodes) != 1 {
		t.Fatalf("frame 2: node should still be present and animating: %#v", model.cosmos.Nodes)
	}

	// Frame 3: well past the blink+fade window. compose STILL emits the
	// same Dead node every frame — it does not prune by time at all.
	model.cosmosNowNS += int64(compose.CosmosDeathBlink+compose.CosmosDeathFade) + int64(time.Millisecond)
	model.applyCosmosGraph(compose.CosmosGraph{
		Nodes: []compose.CosmosNode{{
			Key: "chat:session:ghost", Label: resolve.Named("ghost"), Dead: true, LastNS: fixtureNowNS + int64(time.Second),
		}},
	})
	if len(model.cosmos.Nodes) != 0 {
		t.Fatalf("frame 3: node should have dropped past its window: %#v", model.cosmos.Nodes)
	}

	// Frame 4: one tick later, compose reports the EXACT SAME dead node
	// again (nothing about it changed — it just has not aged out of the
	// 24h ledger window yet). It must stay gone.
	model.cosmosNowNS += int64(time.Millisecond)
	model.applyCosmosGraph(compose.CosmosGraph{
		Nodes: []compose.CosmosNode{{
			Key: "chat:session:ghost", Label: resolve.Named("ghost"), Dead: true, LastNS: fixtureNowNS + int64(time.Second),
		}},
	})
	if len(model.cosmos.Nodes) != 0 {
		t.Fatalf("frame 4: RESURRECTION — a dropped dead node returned: %#v", model.cosmos.Nodes)
	}

	// And once compose finally stops emitting it at all (aged out of the
	// ledger window), the retained entry must be swept — the map does not
	// grow without bound.
	model.applyCosmosGraph(compose.CosmosGraph{})
	if len(model.cosmosDiedAtNS) != 0 {
		t.Fatalf("cosmosDiedAtNS leaked an entry after compose stopped emitting the node: %#v", model.cosmosDiedAtNS)
	}
}

// TestApplyCosmosGraphDropsAnUnwitnessedDeadNodeOnFirstSight is the
// regression for the second permanent-debris door: a chat killed BEFORE
// the picker ever opened the cosmos tab has no row (the fleet DB carries
// none for it) and no witnessed alive->dead transition — this model has
// simply never seen it before. compose has already resolved that ambiguity
// upstream (its grace window elapsed with no row), so it arrives here
// Dead. Without a witnessed transition there is no honest detection
// moment to animate from, and per applyCosmosGraph's doc comment that is
// treated as already past its window: dropped on sight, not kept as
// permanent dim-static debris.
func TestApplyCosmosGraphDropsAnUnwitnessedDeadNodeOnFirstSight(t *testing.T) {
	model := NewModel(fixtureSnapshot(80))
	model.cosmosNowNS = fixtureNowNS
	model.applyCosmosGraph(compose.CosmosGraph{
		Nodes: []compose.CosmosNode{{
			Key: "chat:session:already-dead", Label: resolve.Named("already-dead"),
			Dead: true, LastNS: fixtureNowNS - int64(time.Hour),
		}},
	})
	if len(model.cosmos.Nodes) != 0 {
		t.Fatalf("a node Dead on the very first graph this model ever saw was rendered: %#v", model.cosmos.Nodes)
	}
}

// TestApplyCosmosGraphRendersAPendingNodeInsideTheGraceWindow is the
// companion half: a node compose has NOT (yet) called Dead — inside its
// row-indexing grace window, a freshly spawned child whose row has not
// been gathered — must render normally, exactly like any other node.
func TestApplyCosmosGraphRendersAPendingNodeInsideTheGraceWindow(t *testing.T) {
	model := NewModel(fixtureSnapshot(80))
	model.cosmosNowNS = fixtureNowNS
	model.applyCosmosGraph(compose.CosmosGraph{
		Nodes: []compose.CosmosNode{{
			Key: "chat:session:newborn", Label: resolve.Named("newborn"), LastNS: fixtureNowNS,
		}},
	})
	if len(model.cosmos.Nodes) != 1 || model.cosmos.Nodes[0].Dead {
		t.Fatalf("a pending (not-yet-dead) node was not rendered normally: %#v", model.cosmos.Nodes)
	}
}

// cosmosNodeByKey names the node a test means, so a miss fails with the whole
// graph instead of panicking on an index.
func cosmosNodeByKey(graph compose.CosmosGraph, key string) (compose.CosmosNode, bool) {
	for _, node := range graph.Nodes {
		if node.Key == key {
			return node, true
		}
	}
	return compose.CosmosNode{}, false
}

// TestCosmosRendersARawSessionAddressedSendOnTheExistingNode is the
// end-to-end proof for the defect the operator watched happen: a chat sent an
// MCP message to a live, named chat, and the cosmos tab grew a node labelled
// with the RECEIVER's raw tmux session id, which then blinked out 2.7s later
// when the no-row grace window expired.
//
// It asserts all three things that have to be true, not just the one that was
// broken. The send must land on the two nodes that already exist and mint
// NONE. The node must render the chat's human label. And the send visual must
// still fire — a resolution fix that quietly stopped the comet would be a
// regression the node-count assertion alone would never catch.
func TestCosmosRendersARawSessionAddressedSendOnTheExistingNode(t *testing.T) {
	const receiverSession = "cx-1799999900-1-1" // fixture row 1 ("RR")
	build := func(noSky bool) Model {
		snapshot := fixtureSnapshot(80)
		snapshot.NoSky = noSky
		snapshot.Cosmos = compose.BuildCosmos(snapshot.Rows, []shared.CommsEvent{{
			AtNS:       fixtureNowNS - int64(300*time.Millisecond),
			Kind:       shared.KindInject,
			SenderUUID: "11111111-1111-4111-8111-111111111111",
			// The pathological shape: the caller addressed the chat by
			// the raw session id the old reply footer handed it, and the
			// receiver was recorded by its full socket PATH while a live
			// row carries the bare name.
			Target:         receiverSession,
			ReceiverSocket: "/tmp/tmux-1000/" + receiverSession,
			ReceiverPane:   "%0",
			Message:        "hello cosmos",
		}}, fixtureNowNS)
		model := NewModel(snapshot)
		model.tab = TabCosmos
		return model
	}

	graph := build(true).cosmos
	if len(graph.Nodes) != 2 {
		t.Fatalf("a send between two known chats produced %d nodes, want 2: %#v", len(graph.Nodes), graph.Nodes)
	}
	receiver, found := cosmosNodeByKey(graph, "chat:id:22222222-2222-4222-8222-222222222222")
	if !found {
		t.Fatalf("the receiver did not resolve to its existing row: %#v", graph.Nodes)
	}
	if got := receiver.Label.String(); got != "RR" {
		t.Fatalf("receiver rendered as %q, want its label %q", got, "RR")
	}
	for _, node := range graph.Nodes {
		if label := node.Label.String(); strings.Contains(label, receiverSession) {
			t.Fatalf("a raw session id reached a node label: %q", label)
		}
	}

	if !strings.Contains(build(true).View().Content, "RR") {
		t.Fatalf("the rendered cosmos frame does not show the receiver's label")
	}

	// The send visual. Comparing a sky-ON frame against a sky-OFF one would
	// prove nothing — the starfield alone makes those differ whether the
	// comet flew or was never wired — so this holds the sky ON for both
	// renders and moves only the CLOCK: once while the edge is in flight,
	// once a full comet duration past it. The universe canvas is where the
	// comet paints, and its dot count is the direct evidence.
	model := build(false)
	inFlight := cosmosUniverseDots(model, time.Unix(0, fixtureNowNS))
	landed := cosmosUniverseDots(
		model,
		time.Unix(0, fixtureNowNS+int64(cosmosCometDuration(shared.KindInject))),
	)
	if inFlight <= landed {
		t.Fatalf(
			"the send visual did not fire: universe dots in flight = %d, after landing = %d",
			inFlight, landed,
		)
	}
}

// cosmosUniverseDots renders the universe canvas at one instant and counts the
// subpixels it painted. The comet, its head burst and its tail are all dots,
// so this is what "the send animated" looks like from the outside.
func cosmosUniverseDots(model Model, now time.Time) int {
	canvas := NewCanvas(80, 24)
	model.drawCosmosUniverse(canvas, now)
	painted := 0
	for _, bits := range canvas.dots {
		painted += bits2.OnesCount8(bits)
	}
	return painted
}
