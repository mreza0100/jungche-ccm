package ui

import (
	"fmt"
	"math"
	"strings"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"hostops/pfm/internal/compose"
	"hostops/pfm/internal/sky"
)

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#ffffff")).
			Background(lipgloss.Color("#5f3dc4"))
	groupStyleA = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#5eead4"))
	groupStyleB = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7dd3fc"))
	borderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#64748b"))
	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#ffffff")).
			Background(lipgloss.Color("#334155"))
	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#94a3b8"))
	codexStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#e879f9"))
	agentStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f0abfc"))
	warnStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#fde047"))
	labelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#67e8f9"))
)

// View renders only the visible viewport, so frame cost is independent of the
// total fleet size.
func (model Model) View() tea.View {
	view := tea.NewView(model.render())
	view.AltScreen = true
	view.MouseMode = tea.MouseModeNone
	view.WindowTitle = "pfm"
	return view
}

func (model Model) render() string {
	width := maxInt(40, model.width)
	height := maxInt(12, model.height)
	header := model.renderHeader(width)
	query := model.renderQuery(width)
	footer := model.renderFooter(width)
	bodyHeight := maxInt(4, height-6)
	body := model.renderListPanel(width, bodyHeight)
	if model.tab == TabStats {
		body = model.renderStatsPanel(width, bodyHeight)
	}
	return strings.Join([]string{header, query, body, footer}, "\n")
}

func (model Model) renderTabs(width int) string {
	chat := " Chats "
	stats := " Stats "
	if model.tab == TabChats {
		chat = selectedStyle.Render(chat)
	} else {
		stats = selectedStyle.Render(stats)
	}
	return fillLine(" tabs  "+chat+" "+stats+"   ←/→", width)
}

func (model Model) renderHeader(width int) string {
	contentWidth := width
	if model.skyEnabled {
		contentWidth = maxInt(1, width-18)
	}
	cache := "🪫 5m"
	if model.cache1H {
		cache = "⚡ 1h"
	}
	refresh := ""
	if model.refreshing {
		refresh = " · ⟳ refreshing"
	}
	text := fmt.Sprintf(
		" pfm  %s account %d · %s · %d rows · %d hidden · %d empty%s",
		accountMedal(model.primary),
		model.primary,
		cache,
		len(model.rows),
		model.hiddenCount,
		model.suppressedCount,
		refresh,
	)
	lines := []string{
		headerStyle.Render(fillLine(text, contentWidth)),
		model.renderTabs(contentWidth),
	}
	if model.tab == TabStats {
		lines = append(lines, model.renderStatsHeader(contentWidth))
	} else {
		lines = append(lines, dimStyle.Render(fillLine(
			" Chats · fuzzy search and all existing chat controls",
			contentWidth,
		)))
	}
	if !model.skyEnabled {
		return strings.Join(lines, "\n")
	}
	claude, codex := liveEngineCounts(model.rows)
	widget := sky.Frame(sky.Options{
		ClaudeCount: claude,
		CodexCount:  codex,
		Width:       18,
		Height:      3,
		TimeNS:      model.nowNS,
		Events:      model.skyEvents,
		Colorize:    true,
	})
	for index := range lines {
		lines[index] = fillLine(lines[index], contentWidth) + widget[index]
	}
	return strings.Join(lines, "\n")
}

func (model Model) renderQuery(width int) string {
	if model.tab == TabStats {
		return model.renderStatsSubtabs(width)
	}
	input := model.query.View()
	status := fmt.Sprintf(
		"%d/%d visible · project rotation %d",
		len(model.filtered),
		len(model.order),
		model.rotation,
	)
	// ⌃X takes the status line either way: the receipt when it landed, the
	// reason when it was refused. It acts immediately, so its outcome has to
	// be as immediate — and as visible — as the keystroke.
	if model.hideStatus != "" {
		status = model.hideStatus
	}
	available := maxInt(8, width-lipgloss.Width(status)-2)
	input = ansi.Truncate(input, available, "…")
	line := input + strings.Repeat(
		" ",
		maxInt(1, width-lipgloss.Width(input)-lipgloss.Width(status)),
	) + dimStyle.Render(status)
	return fillLine(line, width)
}

func (model Model) renderStatsSubtabs(width int) string {
	chats := " Chats "
	docker := " Docker "
	if model.statsSubtab == StatsChats {
		chats = selectedStyle.Render(chats)
	} else {
		docker = selectedStyle.Render(docker)
	}
	prefix := " subtabs  "
	if model.statsFocus == StatsFocusSubtab {
		prefix = "›subtabs  "
	}
	return fillLine(prefix+chats+" "+docker+"   ←/→", width)
}

func (model Model) renderStatsHeader(width int) string {
	value := func(percent float64, valid bool) string {
		if !valid {
			return "…"
		}
		return fmt.Sprintf("%.0f%%", percent)
	}
	header := model.stats.Header
	line := fmt.Sprintf(
		" CPU %s · PSI %.0f%% · RAM %.0f%% · SWAP %.0f%%",
		value(header.CPUPercent, header.CPUValid),
		header.PSIPercent,
		header.RAMPercent,
		header.SwapPercent,
	)
	if model.statsError != "" {
		line += " · " + warnStyle.Render("sample failed: "+model.statsError)
	} else {
		if len(model.stats.Warnings) > 0 {
			warning := model.stats.Warnings[0]
			if len(model.stats.Warnings) > 1 {
				warning += fmt.Sprintf(" (+%d)", len(model.stats.Warnings)-1)
			}
			line += " · " + warnStyle.Render("warning: "+warning)
		}
		if model.statsLoading {
			line += " · " + dimStyle.Render("sampling…")
		}
	}
	return fillLine(line, width)
}

func (model Model) renderFooter(width int) string {
	if model.tab == TabStats {
		first := " ↑↓ focus/rows  ←→ focused tab  c CPU sort  m RAM sort"
		second := " esc cancel · live samples every 2s only while Stats is focused"
		return dimStyle.Render(fillLine(first, width)) + "\n" +
			dimStyle.Render(fillLine(second, width))
	}
	first := " ↑↓ move  enter open  esc cancel  type to fuzzy-find"
	second := " ⌃T reload  ⌃R projects  ⌃X hide  ⌃E 1h  ⌃S account  ⌃O reboot"
	if width < 96 {
		first = " ↑↓ move · enter open · esc cancel · type find"
		second = " ⌃T reload · ⌃R rotate · ⌃X hide · ⌃E 1h · ⌃S acct · ⌃O reboot"
	}
	return dimStyle.Render(fillLine(first, width)) + "\n" +
		dimStyle.Render(fillLine(second, width))
}

func (model Model) renderStatsPanel(width, height int) string {
	innerWidth := maxInt(1, width-2)
	innerHeight := maxInt(1, height-2)
	lines := make([]string, 0, innerHeight)
	if model.statsSubtab == StatsChats {
		nameWidth := minInt(32, maxInt(8, innerWidth-60))
		header := fmt.Sprintf(
			"  %-*s %-7s %7s %8s %6s %9s %9s %5s",
			nameWidth, "NAME", "ENGINE", "CPU%", "RSS", "RAM%", "TOKENS", "TOK/H", "GEAR",
		)
		lines = append(lines, dimStyle.Render(fillLine(header, innerWidth)))
		for index, chat := range model.stats.Chats {
			if len(lines) >= innerHeight {
				break
			}
			cpu := "…"
			if chat.CPUValid {
				cpu = fmt.Sprintf("%.1f%%", chat.CPUPercent)
			}
			gear := "-"
			if chat.GearCount > 0 {
				gear = fmt.Sprintf("⚙%d", chat.GearCount)
			}
			tokens := "…"
			if chat.TokensKnown {
				tokens = formatTokens(chat.TokenCount)
			}
			tokensPerHour := "…"
			if chat.TokenRateValid {
				tokensPerHour = formatTokenRate(chat.TokensPerHour)
			}
			line := fmt.Sprintf(
				"  %-*s %-7s %7s %8s %5.1f%% %9s %9s %5s",
				nameWidth, clipRunes(cleanField(chat.Name), nameWidth), chat.Engine, cpu,
				formatSize(int64(chat.RSSBytes)), chat.RAMPercent,
				tokens, tokensPerHour, gear,
			)
			line = fillLine(line, innerWidth)
			if model.statsFocus == StatsFocusContent && index == model.statsCursor {
				line = selectedStyle.Render("›" + ansi.Truncate(line[1:], maxInt(0, innerWidth-1), ""))
			}
			lines = append(lines, line)
		}
	} else {
		available := maxInt(16, innerWidth-36)
		nameWidth := minInt(24, maxInt(8, available/3))
		imageWidth := maxInt(8, available-nameWidth)
		header := fmt.Sprintf(
			"  %-*s %-*s %7s %8s %8s %6s",
			nameWidth, "NAME", imageWidth, "IMAGE", "CPU%", "MEMORY", "LIMIT", "MEM%",
		)
		lines = append(lines, dimStyle.Render(fillLine(header, innerWidth)))
		for index, container := range model.stats.Docker {
			if len(lines) >= innerHeight {
				break
			}
			cpu := "…"
			if container.CPUValid {
				cpu = fmt.Sprintf("%.1f%%", container.CPUPercent)
			}
			limit := "max"
			if container.LimitBytes > 0 {
				limit = formatSize(int64(container.LimitBytes))
			}
			line := fillLine(fmt.Sprintf(
				"  %-*s %-*s %7s %8s %8s %5.1f%%",
				nameWidth, clipRunes(cleanField(container.Name), nameWidth),
				imageWidth, clipRunes(cleanField(container.Image), imageWidth), cpu,
				formatSize(int64(container.MemoryBytes)), limit,
				container.MemoryPercent,
			), innerWidth)
			if model.statsFocus == StatsFocusContent && index == model.statsDockerCursor {
				line = selectedStyle.Render("›" + ansi.Truncate(line[1:], maxInt(0, innerWidth-1), ""))
			}
			lines = append(lines, line)
		}
	}
	if len(lines) == 1 && len(lines) < innerHeight {
		message := "  waiting for first sample…"
		if model.stats.Ready {
			message = "  no live rows"
		}
		lines = append(lines, dimStyle.Render(fillLine(message, innerWidth)))
	}
	for len(lines) < innerHeight {
		lines = append(lines, strings.Repeat(" ", innerWidth))
	}
	return framePanel(" stats ", lines, width)
}

func (model Model) renderListPanel(width, height int) string {
	title := fmt.Sprintf(" fleet %d ", len(model.filtered))
	innerWidth := maxInt(1, width-2)
	innerHeight := maxInt(1, height-2)
	lines := make([]string, 0, innerHeight)
	if len(model.filtered) == 0 {
		lines = append(lines, dimStyle.Render(fillLine("  no matches", innerWidth)))
	} else {
		start := maxInt(0, model.cursor-innerHeight/2)
		previousProject := ""
		previousNameGroup := ""
		for position := start; position < len(model.filtered) &&
			len(lines) < innerHeight; position++ {
			row := model.rows[model.filtered[position]]
			project := cleanField(row.Project)
			if project == "" {
				project = "?"
			}
			if project != previousProject {
				if len(lines)+1 >= innerHeight && position == model.cursor {
					// The selected row always wins the final viewport line.
				} else {
					style := groupStyleA
					if model.projectOrdinal(project)%2 == 1 {
						style = groupStyleB
					}
					group := "╭─ " + clipRunes(project, maxInt(1, innerWidth-5))
					lines = append(
						lines,
						style.Render(fillLine(group, innerWidth)),
					)
				}
				previousProject = project
				previousNameGroup = ""
			}
			if len(lines) >= innerHeight {
				break
			}
			nameGroup, grouped := model.nameGroups[model.filtered[position]]
			if grouped && nameGroup.name != previousNameGroup && len(lines) < innerHeight {
				lines = append(lines, labelStyle.Render(fillLine(
					"│  "+nameGroup.name+fmt.Sprintf(" (%d)", nameGroup.count),
					innerWidth,
				)))
			}
			if grouped {
				previousNameGroup = nameGroup.name
			} else {
				previousNameGroup = ""
			}
			if len(lines) >= innerHeight {
				break
			}
			lines = append(
				lines,
				model.renderGroupedRow(row, position == model.cursor, innerWidth, grouped),
			)
		}
	}
	for len(lines) < innerHeight {
		lines = append(lines, strings.Repeat(" ", innerWidth))
	}
	return framePanel(title, lines, width)
}

func (model Model) projectOrdinal(project string) int {
	for index, group := range model.groups {
		if group.name == project {
			return index
		}
	}
	return 0
}

func (model Model) renderRow(
	row compose.Row,
	selected bool,
	width int,
) string {
	return model.renderGroupedRow(row, selected, width, false)
}

func (model Model) renderGroupedRow(
	row compose.Row,
	selected bool,
	width int,
	grouped bool,
) string {
	pointer := "│ "
	if selected {
		pointer = "› "
	}
	if grouped {
		pointer += "  "
	}
	name := cleanField(row.Name)
	if name == "" {
		name = "(unnamed)"
	}
	name = fixedDisplayColumn(name, 30)
	marker := rowMarker(row.Kind)
	badges := rowBadges(row)
	badges = fixedDisplayColumn(badges, 20)
	prompts := fmt.Sprintf("%dp", row.PromptCount)
	size := formatSize(row.Size)
	left := pointer + marker + " " + name + " " + badges + " " +
		fmt.Sprintf("%4s %6s", prompts, size)
	age := formatAge(row, model.nowNS)
	leftWidth := maxInt(1, width-lipgloss.Width(age)-1)
	left = ansi.Truncate(left, leftWidth, "…")
	line := left + strings.Repeat(
		" ",
		maxInt(1, width-lipgloss.Width(left)-lipgloss.Width(age)),
	) + age
	line = fillLine(line, width)
	if selected {
		return selectedStyle.Render(line)
	}
	switch row.Kind {
	case compose.LiveCodex, compose.ResumeCodex, compose.NewCodex:
		return codexStyle.Render(line)
	case compose.Agent:
		return agentStyle.Render(line)
	default:
		return line
	}
}

func framePanel(title string, lines []string, width int) string {
	innerWidth := maxInt(1, width-2)
	topLabel := "─" + title
	top := "╭" + ansi.Truncate(topLabel, innerWidth, "…")
	top += strings.Repeat("─", maxInt(0, width-lipgloss.Width(top)-1)) + "╮"
	bottom := "╰" + strings.Repeat("─", innerWidth) + "╯"
	framed := make([]string, 0, len(lines)+2)
	framed = append(framed, borderStyle.Render(top))
	for _, line := range lines {
		framed = append(
			framed,
			borderStyle.Render("│")+
				fillLine(line, innerWidth)+
				borderStyle.Render("│"),
		)
	}
	framed = append(framed, borderStyle.Render(bottom))
	return strings.Join(framed, "\n")
}

func rowMarker(kind compose.Kind) string {
	switch kind {
	case compose.LiveClaude, compose.LiveCodex, compose.LiveSplit:
		return "●"
	case compose.Booting:
		return "◐"
	case compose.Agent:
		return "⚙"
	case compose.ResumeClaude, compose.ResumeCodex:
		return "↻"
	case compose.NewClaude, compose.NewCodex:
		return "✦"
	default:
		return "·"
	}
}

func rowBadges(row compose.Row) string {
	badges := make([]string, 0, 7)
	switch row.Kind {
	case compose.LiveCodex, compose.ResumeCodex, compose.NewCodex:
		badges = append(badges, "⬢")
	case compose.Agent:
		badges = append(badges, "⚙ agent")
	}
	if row.ServerCount > 1 {
		badges = append(
			badges,
			warnStyle.Render(fmt.Sprintf("⚠%dsrv", row.ServerCount)),
		)
	}
	if row.SplitCount > 1 {
		badges = append(badges, fmt.Sprintf("⊞%d", row.SplitCount))
	}
	if len(row.Accounts) != 0 {
		for _, account := range row.Accounts {
			badges = append(badges, accountMedal(account))
		}
	} else if row.Account != 0 {
		badges = append(badges, accountMedal(row.Account))
	}
	if row.C1H {
		badges = append(badges, "⚡")
	}
	if row.Attached {
		badges = append(badges, "⇄")
	}
	if row.Here {
		badges = append(badges, "←here")
	}
	if row.Hidden {
		badges = append(badges, dimStyle.Render("·hidden"))
	}
	return strings.Join(badges, " ")
}

func formatAge(row compose.Row, nowNS int64) string {
	age := row.AgeNS
	if nowNS > 0 && row.ActivityNS > 0 {
		age = nowNS - row.ActivityNS
	}
	if age < 0 {
		age = 0
	}
	duration := time.Duration(age)
	switch {
	case duration < time.Minute:
		return fmt.Sprintf("%ds", int64(duration/time.Second))
	case duration < time.Hour:
		return fmt.Sprintf("%dm", int64(duration/time.Minute))
	case duration < 24*time.Hour:
		return fmt.Sprintf("%dh", int64(duration/time.Hour))
	default:
		return fmt.Sprintf("%dd", int64(duration/(24*time.Hour)))
	}
}

func formatSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%dB", maxInt64(0, size))
	}
	units := []string{"K", "M", "G", "T"}
	value := float64(size)
	for _, unit := range units {
		value /= 1024
		if value < 1024 || unit == "T" {
			if value < 10 {
				return fmt.Sprintf("%.1f%s", value, unit)
			}
			return fmt.Sprintf("%.0f%s", value, unit)
		}
	}
	return "0B"
}

func formatTokens(tokens int64) string {
	if tokens < 0 {
		return "…"
	}
	return formatTokenNumber(float64(tokens))
}

func formatTokenRate(tokens float64) string {
	if tokens < 0 || math.IsNaN(tokens) || math.IsInf(tokens, 0) {
		return "…"
	}
	return formatTokenNumber(tokens)
}

func formatTokenNumber(tokens float64) string {
	if tokens < 1000 {
		return fmt.Sprintf("%.0f", tokens)
	}
	units := []string{"K", "M", "B", "T"}
	value := tokens
	for _, unit := range units {
		value /= 1000
		if value < 1000 || unit == "T" {
			if value < 10 {
				return fmt.Sprintf("%.1f%s", value, unit)
			}
			return fmt.Sprintf("%.0f%s", value, unit)
		}
	}
	return "0"
}

func maxInt64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}

func accountMedal(account int) string {
	switch account {
	case 1:
		return "🥇"
	case 2:
		return "🥈"
	case 3:
		return "🥉"
	default:
		return "·"
	}
}

func fillLine(value string, width int) string {
	value = ansi.Truncate(value, maxInt(0, width), "")
	padding := width - lipgloss.Width(value)
	if padding > 0 {
		value += strings.Repeat(" ", padding)
	}
	return value
}

func fixedDisplayColumn(value string, width int) string {
	value = ansi.Truncate(value, maxInt(0, width), "…")
	if padding := width - lipgloss.Width(value); padding > 0 {
		value += strings.Repeat(" ", padding)
	}
	return value
}

func clipRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	if limit == 1 {
		return "…"
	}
	return string(runes[:limit-1]) + "…"
}

func cleanField(value string) string {
	return strings.TrimSpace(strings.Map(func(runeValue rune) rune {
		if unicode.IsControl(runeValue) {
			return ' '
		}
		return runeValue
	}, value))
}
