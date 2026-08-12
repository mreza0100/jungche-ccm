package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"hostops/cc-fleet/internal/compose"
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
	view.WindowTitle = "cc-fleet"
	return view
}

func (model Model) render() string {
	width := maxInt(40, model.width)
	height := maxInt(12, model.height)
	header := model.renderHeader(width)
	query := model.renderQuery(width)
	footer := model.renderFooter(width)
	bodyHeight := maxInt(6, height-4)

	previewWidth := 40
	if width < 100 {
		previewWidth = 30
	}
	if previewWidth > width/2 {
		previewWidth = maxInt(20, width/2)
	}
	listWidth := maxInt(18, width-previewWidth-1)
	previewWidth = width - listWidth - 1

	list := model.renderListPanel(listWidth, bodyHeight)
	preview := model.renderPreviewPanel(previewWidth, bodyHeight)
	body := joinColumns(list, preview, " ")
	return strings.Join([]string{header, query, body, footer}, "\n")
}

func (model Model) renderHeader(width int) string {
	cache := "🪫 5m"
	if model.cache1H {
		cache = "⚡ 1h"
	}
	refresh := ""
	if model.refreshing {
		refresh = " · ⟳ refreshing"
	}
	text := fmt.Sprintf(
		" cc-fleet  %s account %d · %s · %d rows · %d hidden · %d empty%s",
		accountMedal(model.primary),
		model.primary,
		cache,
		len(model.rows),
		model.hiddenCount,
		model.suppressedCount,
		refresh,
	)
	return headerStyle.Render(fillLine(text, width))
}

func (model Model) renderQuery(width int) string {
	input := model.query.View()
	status := fmt.Sprintf(
		"%d/%d visible · project rotation %d",
		len(model.filtered),
		len(model.order),
		model.rotation,
	)
	available := maxInt(8, width-lipgloss.Width(status)-2)
	input = ansi.Truncate(input, available, "…")
	line := input + strings.Repeat(
		" ",
		maxInt(1, width-lipgloss.Width(input)-lipgloss.Width(status)),
	) + dimStyle.Render(status)
	return fillLine(line, width)
}

func (model Model) renderFooter(width int) string {
	first := " ↑↓ move  enter open  esc cancel  type to fuzzy-find"
	second := " ⌃T reload  ⌃R projects  ⌃X hide  ⌃E 1h  ⌃S account  ⌃O reboot"
	if width < 96 {
		first = " ↑↓ move · enter open · esc cancel · type find"
		second = " ⌃T reload · ⌃R rotate · ⌃X hide · ⌃E 1h · ⌃S acct · ⌃O reboot"
	}
	return dimStyle.Render(fillLine(first, width)) + "\n" +
		dimStyle.Render(fillLine(second, width))
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
			}
			if len(lines) >= innerHeight {
				break
			}
			lines = append(
				lines,
				model.renderRow(row, position == model.cursor, innerWidth),
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
	pointer := "│ "
	if selected {
		pointer = "› "
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

func (model Model) renderPreviewPanel(width, height int) string {
	innerWidth := maxInt(1, width-2)
	innerHeight := maxInt(1, height-2)
	lines := make([]string, 0, innerHeight)
	row, ok := model.selectedRow()
	if !ok {
		lines = append(lines, dimStyle.Render("no highlighted row"))
	} else {
		lines = append(lines,
			previewField("name", clipRunes(cleanField(row.Name), 30), innerWidth),
			previewField("cwd", cleanField(row.CWD), innerWidth),
			previewField("account", previewAccount(row), innerWidth),
			previewField("size", formatSize(row.Size), innerWidth),
			previewField("age", formatAge(row, model.nowNS), innerWidth),
			previewField(
				"prompts",
				strconv.FormatInt(row.PromptCount, 10),
				innerWidth,
			),
			"",
			labelStyle.Render("last prompt"),
			ansi.Truncate(
				cleanField(row.LastPrompt),
				innerWidth,
				"…",
			),
		)
		if row.LastPrompt == "" {
			lines[len(lines)-1] = dimStyle.Render("(none cached)")
		}
	}
	if len(lines) > innerHeight {
		lines = lines[:innerHeight]
	}
	for len(lines) < innerHeight {
		lines = append(lines, "")
	}
	for index := range lines {
		lines[index] = fillLine(lines[index], innerWidth)
	}
	return framePanel(" preview ", lines, width)
}

func previewField(label, value string, width int) string {
	prefix := labelStyle.Render(label + " ")
	return prefix + ansi.Truncate(
		value,
		maxInt(1, width-lipgloss.Width(prefix)),
		"…",
	)
}

func previewAccount(row compose.Row) string {
	if len(row.Accounts) != 0 {
		values := make([]string, 0, len(row.Accounts))
		for _, account := range row.Accounts {
			values = append(values, accountMedal(account)+" "+strconv.Itoa(account))
		}
		return strings.Join(values, " · ")
	}
	if row.Account == 0 {
		return "—"
	}
	return accountMedal(row.Account) + " " + strconv.Itoa(row.Account)
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

func joinColumns(left, right, separator string) string {
	leftLines := strings.Split(left, "\n")
	rightLines := strings.Split(right, "\n")
	height := maxInt(len(leftLines), len(rightLines))
	leftWidth := 0
	for _, line := range leftLines {
		leftWidth = maxInt(leftWidth, lipgloss.Width(line))
	}
	lines := make([]string, height)
	for index := range height {
		var leftLine, rightLine string
		if index < len(leftLines) {
			leftLine = leftLines[index]
		}
		if index < len(rightLines) {
			rightLine = rightLines[index]
		}
		lines[index] = fillLine(leftLine, leftWidth) + separator + rightLine
	}
	return strings.Join(lines, "\n")
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
