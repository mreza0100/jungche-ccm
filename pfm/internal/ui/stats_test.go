package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"hostops/pfm/internal/compose"
	pfmconfig "hostops/pfm/internal/config"
	pfmengine "hostops/pfm/internal/engine"
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

	model, command := applyKey(t, model, specialKey(tea.KeyTab))
	if command == nil || sampler.calls != 0 {
		t.Fatalf("Stats focus command=%v calls=%d", command, sampler.calls)
	}
	message := command()
	if sampler.calls != 1 {
		t.Fatalf("first Stats focus sampled %d times, want 1", sampler.calls)
	}
	updated, _ := model.Update(message)
	model = updated.(Model)
	model, _ = applyKey(t, model, tea.KeyPressMsg(tea.Key{Code: tea.KeyTab, Mod: tea.ModShift}))
	updated, command = model.Update(statsTickMsg{generation: model.statsGeneration})
	model = updated.(Model)
	if command != nil || sampler.calls != 1 || model.Tab() != TabChats {
		t.Fatalf("sampling continued after leave: command=%v calls=%d tab=%d", command, sampler.calls, model.Tab())
	}
}

func TestLimitsTabRendersAccountWindows(t *testing.T) {
	model := NewModel(fixtureSnapshot(120))
	model.tab = TabLimits
	model.stats = pfmstats.Snapshot{Limits: []pfmstats.AccountLimits{{
		Account: 2,
		Emoji:   "🔹",
		Windows: []pfmstats.Window{{Name: "7d-fable", UsedPct: 23, ResetAt: time.Unix(1_800_000_000, 0)}},
	}}}
	plain := ansi.Strip(model.renderLimitsPanel(120, 8))
	if !strings.Contains(plain, "7d-fable") || !strings.Contains(plain, "23%") || !strings.Contains(plain, "🔹") {
		t.Fatalf("limits panel=%q", plain)
	}
}

func TestLimitsTabRendersFancyCardsAndRefreshesPastResets(t *testing.T) {
	model := NewModel(fixtureSnapshot(120))
	model.tab = TabLimits
	now := time.Unix(0, model.nowNS)
	model.stats = pfmstats.Snapshot{Limits: []pfmstats.AccountLimits{{
		Account: 1, Emoji: "🥇", Engine: pfmengine.Claude,
		Windows: []pfmstats.Window{
			{Name: "5h", UsedPct: 52, ResetAt: now.Add(90 * time.Minute)},
			{Name: "7d", UsedPct: 100, ResetAt: now.Add(-time.Minute)},
		},
	}}}
	panel := model.renderLimitsPanel(120, 10)
	plain := ansi.Strip(panel)
	for _, want := range []string{"🥇 account 1", "█", "52%", "FULL", "↻ refreshing…"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("fancy Limits panel missing %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "(now)") {
		t.Fatalf("past reset rendered as now:\n%s", plain)
	}
}

func TestLimitsTabRendersCachedWindowsUnderRefreshWarning(t *testing.T) {
	model := NewModel(fixtureSnapshot(120))
	model.tab = TabLimits
	now := time.Unix(0, model.nowNS)
	model.stats = pfmstats.Snapshot{Limits: []pfmstats.AccountLimits{{
		Account: 2, Emoji: "🥈", Engine: "claude", ConfirmedAt: now.Add(-7 * time.Minute),
		Status:  "refresh timed out; showing cached limits",
		Windows: []pfmstats.Window{{Name: "7d", UsedPct: 73, ResetAt: now.Add(4 * 24 * time.Hour)}},
	}}}
	plain := ansi.Strip(model.renderLimitsPanel(120, 9))
	for _, want := range []string{"provider confirmed 7m ago", "refresh timed out; showing cached limits", "7d", "73%"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("cached Limits card missing %q:\n%s", want, plain)
		}
	}
}

func TestLimitBarsKeepEighthCellPrecisionAndFullTail(t *testing.T) {
	_ = NewModel(fixtureSnapshot(120))
	left := ansi.Strip(limitBar(52.4, 20))
	right := ansi.Strip(limitBar(55, 20))
	if left == right || !strings.Contains(left, "▌") {
		t.Fatalf("subcell bars did not distinguish 52.4%% from 55%%: %q vs %q", left, right)
	}
	full := ansi.Strip(limitBar(100, 20))
	if !strings.Contains(full, "FULL▏") {
		t.Fatalf("full bar=%q, want FULL tail", full)
	}
}

func TestLimitsTabNarrowRowsDropResetAndNeverWrap(t *testing.T) {
	model := NewModel(fixtureSnapshot(50))
	model.tab = TabLimits
	now := time.Unix(0, model.nowNS)
	model.stats = pfmstats.Snapshot{Limits: []pfmstats.AccountLimits{{
		Account: 1, Emoji: "🥇", Engine: pfmengine.Claude,
		Windows: []pfmstats.Window{{Name: "7d-fable", UsedPct: 52.4, ResetAt: now.Add(14 * time.Minute)}},
	}}}
	panel := model.renderLimitsPanel(50, 8)
	if strings.Contains(ansi.Strip(panel), "↻") {
		t.Fatalf("narrow Limits panel kept reset column:\n%s", ansi.Strip(panel))
	}
	for index, line := range strings.Split(panel, "\n") {
		if got := lipgloss.Width(line); got != 50 {
			t.Fatalf("narrow Limits line %d width=%d, want 50: %q", index, got, line)
		}
	}
}

func TestLimitsTabOmitsProviderMaxTotals(t *testing.T) {
	model := NewModel(fixtureSnapshot(120))
	model.tab = TabLimits
	model.stats = pfmstats.Snapshot{Limits: []pfmstats.AccountLimits{
		{Account: 1, Engine: pfmengine.Claude, Windows: []pfmstats.Window{{Name: "5h", UsedPct: 52}}},
		{Account: 2, Engine: pfmengine.Claude, Windows: []pfmstats.Window{{Name: "5h", UsedPct: 75}}},
	}}
	plain := ansi.Strip(model.renderLimitsPanel(120, 10))
	if strings.Contains(plain, "Σ") {
		t.Fatalf("provider max-totals line still rendered:\n%s", plain)
	}
	for _, want := range []string{"5h", "52%", "75%"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("Limits panel missing per-account window %q:\n%s", want, plain)
		}
	}
}

func TestLimitsTabRendersEngineAbsencesAsTwoDimLines(t *testing.T) {
	model := NewModel(fixtureSnapshot(120))
	model.tab = TabLimits
	model.stats = pfmstats.Snapshot{Limits: []pfmstats.AccountLimits{
		{Engine: pfmengine.Claude, Label: "no Claude accounts configured", Status: "no Claude accounts configured", Absent: true},
		{Engine: pfmengine.Codex, Label: "no Codex accounts configured", Status: "no Codex accounts configured", Absent: true},
	}}
	plain := ansi.Strip(model.renderLimitsPanel(120, 8))
	for _, want := range []string{"no Claude accounts configured", "no Codex accounts configured"} {
		if strings.Count(plain, want) != 1 {
			t.Fatalf("Limits panel did not render exactly one plain absence line for %q:\n%s", want, plain)
		}
	}
	for _, reject := range []string{"provider confirmation unavailable", "⚠ no Claude", "⚠ no Codex"} {
		if strings.Contains(plain, reject) {
			t.Fatalf("Limits absence rendered as a card/error %q:\n%s", reject, plain)
		}
	}
}

func TestLimitsTabConsolidatesSkipsDropsUnknownsAndKeepsErrors(t *testing.T) {
	home := "/home/test"
	label1 := pfmconfig.DisplayAccountDir(home, 1, pfmconfig.DefaultAccountDir(home, 1))
	label3 := pfmconfig.DisplayAccountDir(home, 3, pfmconfig.DefaultAccountDir(home, 3))
	label4 := pfmconfig.DisplayAccountDir(home, 4, pfmconfig.DefaultAccountDir(home, 4))
	emoji1 := pfmconfig.Defaults(home, []string{pfmconfig.DefaultAccountProjectDir(home, 1)}).EmojiFor(1)
	model := NewModel(fixtureSnapshot(140))
	model.tab = TabLimits
	model.stats = pfmstats.Snapshot{Limits: []pfmstats.AccountLimits{
		{
			Account: 1, Emoji: emoji1, Engine: pfmengine.Claude, Label: label1,
			Windows: []pfmstats.Window{
				{Name: "7d-fable", UsedPct: 0, ResetNote: "reset unavailable"},
				{Name: "unknown[seven_day_nimbus_quill]", UsedPct: 0, ResetNote: "reset unavailable"},
			},
		},
		{Account: 3, Engine: pfmengine.Claude, Label: label3, Status: "skipped " + label3 + ": credentials rejected"},
		{Account: 4, Engine: pfmengine.Claude, Label: label4, Status: "skipped " + label4 + ": no valid credentials"},
		{Engine: pfmengine.Codex, Label: "Codex", Status: "Codex credential rejected (HTTP 401)"},
	}}
	plain := ansi.Strip(model.renderLimitsPanel(140, 18))
	for _, want := range []string{
		"7d-fable", "0%", "↻ reset unavailable",
		"skipped " + label3 + ": credentials rejected",
		"skipped " + label4 + ": no valid credentials",
		"Codex", "Codex credential rejected (HTTP 401)",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("limits panel missing %q:\n%s", want, plain)
		}
	}
	if got := strings.Count(plain, "skipped "); got != 2 {
		t.Fatalf("Limits footer carried %d skip notes, want both notes on one footer line:\n%s", got, plain)
	}
	for _, line := range strings.Split(plain, "\n") {
		if strings.Contains(line, "skipped ") && strings.Count(line, "skipped ") != 2 {
			t.Fatalf("skip notes were not consolidated:\n%s", plain)
		}
	}
	for _, reject := range []string{"(?)", "403 Forbidden", "unknown["} {
		if strings.Contains(plain, reject) {
			t.Fatalf("limits panel contains unexplained/retired output %q:\n%s", reject, plain)
		}
	}
}

func TestUsageSparkScalesToTheBusiestSample(t *testing.T) {
	cases := []struct {
		name   string
		deltas []int64
		want   string
	}{
		{"no history yet", nil, "…"},
		{"all idle", []int64{0, 0, 0}, "▁▁▁"},
		{"burst after idle", []int64{0, 100}, "▁█"},
		{"mixed", []int64{50, 100, 25}, "▅█▃"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := usageSpark(testCase.deltas); got != testCase.want {
				t.Fatalf("usageSpark(%v) = %q, want %q", testCase.deltas, got, testCase.want)
			}
		})
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
			TokensKnown: true, TokensPerMinute: 625_000, TokenRateValid: true,
		}},
		Docker: []pfmstats.Container{{
			Name: "professor-web", Image: "registry.example/professor:web",
			CPUPercent: 4.5, CPUValid: true, MemoryBytes: 128 << 20,
			LimitBytes: 512 << 20, MemoryPercent: 25,
		}},
	}

	chatPanel := ansi.Strip(model.renderStatsPanel(140, 8))
	for _, want := range []string{"NAME", "ENGINE", "CPU", "RSS", "RAM", "TOKENS", "TOK/MIN", "BUILD:one", "1.2M", "625K"} {
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

func TestStatsChatLiveTokenRateRendersUnknownActiveAndIdle(t *testing.T) {
	model := NewModel(fixtureSnapshot(120))
	model.tab = TabStats
	model.stats = pfmstats.Snapshot{Ready: true, Chats: []pfmstats.Chat{
		{Name: "UNKNOWN", Engine: "claude", CPUValid: true, TokenCount: 100, TokensKnown: true},
		{Name: "ACTIVE", Engine: "codex", CPUValid: true, TokenCount: 200, TokensKnown: true, TokensPerMinute: 125, TokenRateValid: true},
		{Name: "IDLE", Engine: "claude", CPUValid: true, TokenCount: 300, TokensKnown: true, TokensPerMinute: 0, TokenRateValid: true},
	}}
	panel := ansi.Strip(model.renderStatsPanel(120, 10))
	for name, want := range map[string]string{"UNKNOWN": "…", "ACTIVE": "125", "IDLE": "–"} {
		var line string
		for _, candidate := range strings.Split(panel, "\n") {
			if strings.Contains(candidate, name) {
				line = candidate
				break
			}
		}
		if line == "" || !strings.Contains(line, want) {
			t.Fatalf("Stats row %s = %q, want live rate %q\n%s", name, line, want, panel)
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
			TokensKnown: true, TokensPerMinute: 625_000, TokenRateValid: true,
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
