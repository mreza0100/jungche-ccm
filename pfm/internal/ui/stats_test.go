package ui

import (
	"errors"
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
	"hostops/pfm/internal/theme"
)

type countingStatsSampler struct {
	calls     int
	snapshots []pfmstats.Snapshot
}

type splitStatsSampler struct {
	fullCalls     int
	resourceCalls int
}

type switchingStatsSampler struct {
	fullStarted      chan struct{}
	fullRelease      chan struct{}
	resourceStarted  chan struct{}
	resourceRelease  chan struct{}
	fullSnapshot     pfmstats.Snapshot
	resourceSnapshot pfmstats.Snapshot
}

func (sampler *switchingStatsSampler) Sample(_ []compose.Row) (pfmstats.Snapshot, error) {
	select {
	case sampler.fullStarted <- struct{}{}:
	default:
	}
	<-sampler.fullRelease
	return sampler.fullSnapshot, nil
}

func (sampler *switchingStatsSampler) SampleResources(_ []compose.Row) (pfmstats.Snapshot, error) {
	select {
	case sampler.resourceStarted <- struct{}{}:
	default:
	}
	<-sampler.resourceRelease
	return sampler.resourceSnapshot, nil
}

func waitForSample(t *testing.T, started <-chan struct{}) {
	t.Helper()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("sample command did not enter its in-flight gate")
	}
}

func runStatsCommand(t *testing.T, command tea.Cmd) <-chan tea.Msg {
	t.Helper()
	if command == nil {
		t.Fatal("tab switch returned no replacement stats command")
	}
	result := make(chan tea.Msg, 1)
	go func() { result <- command() }()
	return result
}

func statsMessage(t *testing.T, messages <-chan tea.Msg) statsSampleMsg {
	t.Helper()
	select {
	case message := <-messages:
		result, ok := message.(statsSampleMsg)
		if !ok {
			t.Fatalf("stats command returned %T", message)
		}
		return result
	case <-time.After(2 * time.Second):
		t.Fatal("stats command did not complete")
		return statsSampleMsg{}
	}
}

func TestRapidLimitsToStatsDropsInFlightLimitsResult(t *testing.T) {
	oldResources := pfmstats.Snapshot{Ready: true, Chats: []pfmstats.Chat{{Socket: "old-resource"}}}
	sampler := &switchingStatsSampler{
		fullStarted:     make(chan struct{}, 1),
		fullRelease:     make(chan struct{}),
		resourceStarted: make(chan struct{}, 1),
		resourceRelease: make(chan struct{}),
		fullSnapshot:    pfmstats.Snapshot{Ready: true, Limits: []pfmstats.AccountLimits{{Account: 99}}},
		resourceSnapshot: pfmstats.Snapshot{
			Ready: true, Chats: []pfmstats.Chat{{Socket: "new-resource"}},
		},
	}
	snapshot := fixtureSnapshot(120)
	snapshot.StatsSampler = sampler
	model := NewModel(snapshot)
	model.stats = oldResources

	model, limitsCommand := applyKey(t, model, tea.KeyPressMsg(tea.Key{Code: tea.KeyTab, Mod: tea.ModShift}))
	limitsResult := runStatsCommand(t, limitsCommand)
	waitForSample(t, sampler.fullStarted)
	model, statsCommand := applyKey(t, model, tea.KeyPressMsg(tea.Key{Code: tea.KeyTab, Mod: tea.ModShift}))
	if model.Tab() != TabStats {
		t.Fatalf("rapid Limits→Stats landed on tab %d", model.Tab())
	}
	close(sampler.fullRelease)
	stale := statsMessage(t, limitsResult)
	updated, _ := model.Update(stale)
	model = updated.(Model)
	if len(model.stats.Limits) != 0 || len(model.stats.Chats) != 1 || model.stats.Chats[0].Socket != "old-resource" {
		t.Fatalf("stale Limits result overwrote Stats resources: %#v", model.stats)
	}
	resourceResult := runStatsCommand(t, statsCommand)
	close(sampler.resourceRelease)
	resourceMessage := statsMessage(t, resourceResult)
	updated, _ = model.Update(resourceMessage)
	model = updated.(Model)
	if len(model.stats.Chats) != 1 || model.stats.Chats[0].Socket != "new-resource" {
		t.Fatalf("fresh Stats resource result = %#v", model.stats)
	}
}

func TestRapidStatsToLimitsDropsInFlightResourceResult(t *testing.T) {
	oldLimits := pfmstats.Snapshot{Ready: true, Limits: []pfmstats.AccountLimits{{Account: 7}}}
	sampler := &switchingStatsSampler{
		fullStarted:     make(chan struct{}, 1),
		fullRelease:     make(chan struct{}),
		resourceStarted: make(chan struct{}, 1),
		resourceRelease: make(chan struct{}),
		fullSnapshot:    pfmstats.Snapshot{Ready: true, Limits: []pfmstats.AccountLimits{{Account: 8}}},
		resourceSnapshot: pfmstats.Snapshot{
			Ready: true, Chats: []pfmstats.Chat{{Socket: "stale-resource"}},
		},
	}
	snapshot := fixtureSnapshot(120)
	snapshot.StatsSampler = sampler
	model := NewModel(snapshot)
	model.stats = oldLimits

	model, resourceCommand := applyKey(t, model, specialKey(tea.KeyTab))
	resourceResult := runStatsCommand(t, resourceCommand)
	waitForSample(t, sampler.resourceStarted)
	model, limitsCommand := applyKey(t, model, specialKey(tea.KeyTab))
	if model.Tab() != TabLimits {
		t.Fatalf("rapid Stats→Limits landed on tab %d", model.Tab())
	}
	close(sampler.resourceRelease)
	stale := statsMessage(t, resourceResult)
	updated, _ := model.Update(stale)
	model = updated.(Model)
	if len(model.stats.Limits) != 1 || model.stats.Limits[0].Account != 7 || len(model.stats.Chats) != 0 {
		t.Fatalf("stale Stats result overwrote Limits: %#v", model.stats)
	}
	fullResult := runStatsCommand(t, limitsCommand)
	close(sampler.fullRelease)
	fullMessage := statsMessage(t, fullResult)
	updated, _ = model.Update(fullMessage)
	model = updated.(Model)
	if len(model.stats.Limits) != 1 || model.stats.Limits[0].Account != 8 {
		t.Fatalf("fresh Limits result = %#v", model.stats)
	}
}

func (sampler *splitStatsSampler) Sample(_ []compose.Row) (pfmstats.Snapshot, error) {
	sampler.fullCalls++
	return pfmstats.Snapshot{}, errors.New("limits fetch blocked the resource sample")
}

func (sampler *splitStatsSampler) SampleResources(_ []compose.Row) (pfmstats.Snapshot, error) {
	sampler.resourceCalls++
	return pfmstats.Snapshot{Ready: true}, nil
}

func TestStatsTabSamplesResourcesWithoutWaitingForLimits(t *testing.T) {
	sampler := &splitStatsSampler{}
	snapshot := fixtureSnapshot(120)
	snapshot.StatsSampler = sampler
	model := NewModel(snapshot)

	model, command := applyKey(t, model, specialKey(tea.KeyTab))
	if command == nil {
		t.Fatal("entering Stats returned no sampling command")
	}
	message, ok := command().(statsSampleMsg)
	if !ok {
		t.Fatalf("Stats command returned %T", message)
	}
	if message.err != nil || sampler.resourceCalls != 1 || sampler.fullCalls != 0 {
		t.Fatalf(
			"Stats sample err=%v resource=%d full=%d; resource sampling must not wait for limits",
			message.err,
			sampler.resourceCalls,
			sampler.fullCalls,
		)
	}
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

func TestLimitsTabScrollsToAccountsBelowTheViewport(t *testing.T) {
	model := NewModel(fixtureSnapshot(80))
	model.tab = TabLimits
	model.height = 12
	model.stats = pfmstats.Snapshot{Limits: []pfmstats.AccountLimits{
		{Account: 1, Engine: pfmengine.Claude, Label: "Claude 1", Windows: []pfmstats.Window{{Name: "5h", UsedPct: 1}}},
		{Account: 2, Engine: pfmengine.Claude, Label: "Claude 2", Windows: []pfmstats.Window{{Name: "7d", UsedPct: 2}}},
		{Account: 1, Engine: pfmengine.Codex, Label: "Codex 1", Windows: []pfmstats.Window{{Name: "5h-spark", UsedPct: 3}}},
	}}
	before := ansi.Strip(model.renderLimitsPanel(80, 6))
	if strings.Contains(before, "Codex 1") {
		t.Fatalf("setup did not place Codex below the viewport:\n%s", before)
	}
	model, command := applyKey(t, model, specialKey(tea.KeyEnd))
	if command != nil || model.limitsOffset == 0 {
		t.Fatalf("End did not scroll Limits: command=%v offset=%d", command, model.limitsOffset)
	}
	after := ansi.Strip(model.renderLimitsPanel(80, 6))
	if !strings.Contains(after, "Codex 1") || !strings.Contains(after, "5h-spark") {
		t.Fatalf("Codex remained unreachable after scrolling:\n%s", after)
	}
}

func TestLimitsTabScrollsAcrossClaudeCodexAndOpenCodeCardsAfterResize(t *testing.T) {
	model := NewModel(fixtureSnapshot(50))
	model.tab = TabLimits
	model.height = 12
	model.stats = pfmstats.Snapshot{Limits: []pfmstats.AccountLimits{
		{Account: 1, Engine: pfmengine.Claude, Label: "Claude 1", Windows: []pfmstats.Window{{Name: "5h", UsedPct: 1}}},
		{Account: 2, Engine: pfmengine.Claude, Label: "Claude 2", Windows: []pfmstats.Window{{Name: "7d", UsedPct: 2}}},
		{Account: 1, Engine: pfmengine.Codex, Label: "Codex 1", Windows: []pfmstats.Window{{Name: "5h", UsedPct: 3}}},
		{Account: 2, Engine: pfmengine.Codex, Label: "Codex 2", Windows: []pfmstats.Window{{Name: "7d", UsedPct: 4}}},
		{Account: 1, Engine: pfmengine.Opencode, Label: "OpenCode 1", Windows: []pfmstats.Window{{Name: "5h", UsedPct: 5}}},
		{Account: 2, Engine: pfmengine.Opencode, Label: "OpenCode 2", Windows: []pfmstats.Window{{Name: "7d", UsedPct: 6}}},
	}}
	if before := ansi.Strip(model.renderLimitsPanel(50, 6)); strings.Contains(before, "OpenCode 2") {
		t.Fatalf("tiny viewport unexpectedly showed the final card:\n%s", before)
	}
	model, command := applyKey(t, model, specialKey(tea.KeyEnd))
	if command != nil || model.limitsOffset == 0 {
		t.Fatalf("End did not reach the bottom of the tiny Limits viewport: command=%v offset=%d", command, model.limitsOffset)
	}
	bottom := ansi.Strip(model.renderLimitsPanel(50, 6))
	if !strings.Contains(bottom, "OpenCode") || !strings.Contains(bottom, " 6%") || !strings.Contains(bottom, "7d") {
		t.Fatalf("bottom of Limits cards was unreachable:\n%s", bottom)
	}

	model.height = 30
	model, command = applyKey(t, model, specialKey(tea.KeyHome))
	if command != nil || model.limitsOffset != 0 {
		t.Fatalf("Home after resize did not reset Limits offset: command=%v offset=%d", command, model.limitsOffset)
	}
	wide := ansi.Strip(model.renderLimitsPanel(80, 30))
	for _, label := range []string{"Claude", "Codex", "OpenCode", " 1%", " 4%", " 6%"} {
		if !strings.Contains(wide, label) {
			t.Fatalf("resized Limits viewport lost %q:\n%s", label, wide)
		}
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

func TestLimitPercentageUsesStableHighContrastForeground(t *testing.T) {
	palette := theme.Load("default")
	configureStyles(palette)
	t.Cleanup(func() { configureStyles(theme.Load("default")) })
	line := renderLimitWindow(time.Unix(0, 0), pfmstats.Window{Name: "5h", UsedPct: 4}, 80)
	want := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(palette.Header)).
		Render("  4%")
	if !strings.Contains(line, want) {
		t.Fatalf("percentage does not use stable high-contrast foreground:\n%q\nwant fragment %q", line, want)
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

func TestStatsOpencodeEngineUsesItsOwnColor(t *testing.T) {
	model := NewModel(fixtureSnapshot(120))
	model.tab = TabStats
	model.stats = pfmstats.Snapshot{Ready: true, Chats: []pfmstats.Chat{{
		Name: "OPEN", Engine: pfmengine.MustLookup(pfmengine.Opencode).LongName,
		CPUValid: true,
	}}}
	panel := model.renderStatsPanel(120, 8)
	want := statsEngineStyles[pfmengine.Opencode].Render("opencode")
	if !strings.Contains(panel, want) {
		t.Fatalf("OpenCode stats row lacks its engine color %q:\n%s", want, panel)
	}
}
