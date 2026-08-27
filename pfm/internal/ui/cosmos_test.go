package ui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"hostops/pfm/internal/compose"
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
	child := compose.CosmosNode{Key: "chat:name:child", Label: "child"}
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
