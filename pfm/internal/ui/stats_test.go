package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"hostops/pfm/internal/compose"
	pfmstats "hostops/pfm/internal/stats"
)

type countingStatsSampler struct {
	calls     int
	snapshots []pfmstats.Snapshot
}

func (sampler *countingStatsSampler) Sample(_ []compose.Row) (pfmstats.Snapshot, error) {
	index := sampler.calls
	sampler.calls++
	if index >= len(sampler.snapshots) {
		index = len(sampler.snapshots) - 1
	}
	return sampler.snapshots[index], nil
}

func TestStatsSamplerIsLazyAndStopsAfterLeaving(t *testing.T) {
	sampler := &countingStatsSampler{snapshots: []pfmstats.Snapshot{{}}}
	snapshot := fixtureSnapshot(120)
	snapshot.StatsSampler = sampler
	model := NewModel(snapshot)
	if sampler.calls != 0 {
		t.Fatalf("picker boot sampled stats: calls=%d", sampler.calls)
	}

	model, command := applyKey(t, model, specialKey(tea.KeyRight))
	if command == nil || sampler.calls != 0 {
		t.Fatalf("Stats focus command=%v calls=%d", command, sampler.calls)
	}
	message := command()
	if sampler.calls != 1 {
		t.Fatalf("first Stats focus sampled %d times, want 1", sampler.calls)
	}
	updated, _ := model.Update(message)
	model = updated.(Model)
	model, _ = applyKey(t, model, specialKey(tea.KeyLeft))
	updated, command = model.Update(statsTickMsg{generation: model.statsGeneration})
	model = updated.(Model)
	if command != nil || sampler.calls != 1 || model.Tab() != TabChats {
		t.Fatalf("sampling continued after leave: command=%v calls=%d tab=%d", command, sampler.calls, model.Tab())
	}
}

func TestStatsCursorFollowsSocketAcrossCPUSort(t *testing.T) {
	model := NewModel(fixtureSnapshot(120))
	model.tab = TabStats
	model.statsSort = StatsSortCPU
	model.applyStats(pfmstats.Snapshot{Chats: []pfmstats.Chat{
		{Socket: "socket-a", Name: "A", CPUPercent: 10, CPUValid: true},
		{Socket: "socket-b", Name: "B", CPUPercent: 20, CPUValid: true},
	}})
	if model.selectedStatsKey() != "socket-b" {
		t.Fatalf("initial selected stats key = %q", model.selectedStatsKey())
	}
	model.applyStats(pfmstats.Snapshot{Chats: []pfmstats.Chat{
		{Socket: "socket-a", Name: "A", CPUPercent: 30, CPUValid: true},
		{Socket: "socket-b", Name: "B", CPUPercent: 5, CPUValid: true},
	}})
	if model.selectedStatsKey() != "socket-b" || model.statsCursor != 1 {
		t.Fatalf("selected=%q cursor=%d after resort", model.selectedStatsKey(), model.statsCursor)
	}
}

func TestStatsHeaderRendersSwapSeparate(t *testing.T) {
	model := NewModel(fixtureSnapshot(120))
	model.tab = TabStats
	model.stats = pfmstats.Snapshot{Header: pfmstats.Header{
		CPUPercent: 12, CPUValid: true, PSIPercent: 34,
		RAMPercent: 56, SwapPercent: 78,
	}}
	line := ansi.Strip(model.renderStatsHeader(120))
	for _, want := range []string{"CPU 12%", "PSI 34%", "RAM 56%", "SWAP 78%"} {
		if !strings.Contains(line, want) {
			t.Fatalf("stats header %q missing %q", line, want)
		}
	}
}

func TestStatsTablesRenderLabeledColumnsAndUsage(t *testing.T) {
	model := NewModel(fixtureSnapshot(140))
	model.tab = TabStats
	model.stats = pfmstats.Snapshot{
		Ready: true,
		Chats: []pfmstats.Chat{{
			Name: "BUILD:one", Engine: "claude", CPUPercent: 12.5, CPUValid: true,
			RSSBytes: 2 << 20, RAMPercent: 3.5, TokenCount: 1_250_000,
			TokensKnown: true, TokensPerHour: 625_000, TokenRateValid: true,
		}},
		Docker: []pfmstats.Container{{
			Name: "professor-web", Image: "registry.example/professor:web",
			CPUPercent: 4.5, CPUValid: true, MemoryBytes: 128 << 20,
			LimitBytes: 512 << 20, MemoryPercent: 25,
		}},
	}

	chatPanel := ansi.Strip(model.renderStatsPanel(140, 8))
	for _, want := range []string{"NAME", "ENGINE", "CPU", "RSS", "RAM", "TOKENS", "TOK/H", "BUILD:one", "1.2M", "625K"} {
		if !strings.Contains(chatPanel, want) {
			t.Fatalf("Chats stats panel missing %q:\n%s", want, chatPanel)
		}
	}

	model.statsSubtab = StatsDocker
	dockerPanel := ansi.Strip(model.renderStatsPanel(140, 8))
	header := strings.Split(dockerPanel, "\n")[1]
	if strings.Index(header, "NAME") < 0 || strings.Index(header, "IMAGE") <= strings.Index(header, "NAME") {
		t.Fatalf("Docker header does not begin NAME then IMAGE: %q", header)
	}
	for _, want := range []string{"CPU", "MEMORY", "LIMIT", "MEM", "professor-web", "registry.example/professor:web"} {
		if !strings.Contains(dockerPanel, want) {
			t.Fatalf("Docker stats panel missing %q:\n%s", want, dockerPanel)
		}
	}
}

func TestStatsPropertiesUseSemanticColors(t *testing.T) {
	model := NewModel(fixtureSnapshot(140))
	model.tab = TabStats
	model.stats = pfmstats.Snapshot{
		Ready: true,
		Chats: []pfmstats.Chat{{
			Name: "BUILD:one", Engine: "claude", CPUPercent: 12.5, CPUValid: true,
			RSSBytes: 2 << 20, RAMPercent: 3.5, TokenCount: 1_250_000,
			TokensKnown: true, TokensPerHour: 625_000, TokenRateValid: true,
			GearCount: 2,
		}},
		Docker: []pfmstats.Container{{
			Name: "professor-web", Image: "registry.example/professor:web",
			CPUPercent: 4.5, CPUValid: true, MemoryBytes: 128 << 20,
			LimitBytes: 512 << 20, MemoryPercent: 25,
		}},
	}
	chatPanel := model.renderStatsPanel(140, 8)
	for name, sequence := range map[string]string{
		"header": "\x1b[1;38;2;34;211;238m",
		"CPU":    "\x1b[38;2;74;222;128m",
		"memory": "\x1b[38;2;96;165;250m",
		"tokens": "\x1b[38;2;250;204;21m",
		"gear":   "\x1b[38;2;251;146;60m",
	} {
		if !strings.Contains(chatPanel, sequence) {
			t.Fatalf("Chats stats panel missing %s color %q:\n%s", name, sequence, chatPanel)
		}
	}
	model.statsSubtab = StatsDocker
	dockerPanel := model.renderStatsPanel(140, 8)
	for name, sequence := range map[string]string{
		"container name":  "\x1b[38;2;94;234;212m",
		"container image": "\x1b[38;2;192;132;252m",
		"CPU":             "\x1b[38;2;74;222;128m",
		"memory":          "\x1b[38;2;96;165;250m",
	} {
		if !strings.Contains(dockerPanel, sequence) {
			t.Fatalf("Docker stats panel missing %s color %q:\n%s", name, sequence, dockerPanel)
		}
	}
}
