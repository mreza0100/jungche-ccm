package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/sahilm/fuzzy"

	"hostops/pfm/internal/compose"
	"hostops/pfm/internal/sky"
	pfmstats "hostops/pfm/internal/stats"
)

const (
	defaultWidth         = 120
	defaultHeight        = 28
	statsRefreshInterval = 2 * time.Second
)

type projectGroup struct {
	name    string
	indices []int
}

type nameGroup struct {
	name  string
	count int
}

type orderedSearch struct {
	search []string
	order  []int
}

func (source orderedSearch) String(index int) string {
	return source.search[source.order[index]]
}

func (source orderedSearch) Len() int {
	return len(source.order)
}

// Model is a pure Bubble Tea state machine over a cached compose snapshot.
// Its commands only request Bubble Tea termination; all fleet I/O belongs to
// the Picker caller.
type Model struct {
	rows              []compose.Row
	search            []string
	groups            []projectGroup
	nameGroups        map[int]nameGroup
	order             []int
	filtered          []int
	cursor            int
	width             int
	height            int
	nowNS             int64
	view              compose.View
	hiddenCount       int
	suppressedCount   int
	refreshing        bool
	primary           int
	initialPrimary    int
	accountIDs        []int
	cache1H           bool
	rotation          int
	tab               Tab
	statsSubtab       StatsSubtab
	statsFocus        StatsFocus
	statsSort         StatsSort
	statsCursor       int
	statsDockerCursor int
	stats             pfmstats.Snapshot
	statsSampler      StatsSampler
	statsGeneration   uint64
	statsLoading      bool
	statsError        string
	skyEnabled        bool
	skyEvents         []sky.Event
	// activity is stamped on every real keystroke. The background refresh
	// stream reads it to decide whether anyone is still watching.
	activity      *ActivityClock
	query         textinput.Model
	outcome       OutcomeKind
	outcomeRow    compose.Row
	initialHidden map[string]bool
	hideChanges   map[string]HideChange
	// applyHide performs a ⌃X the instant it is typed. hideStatus is the
	// receipt: what landed, or why the keystroke was refused. A ⌃X NEVER goes
	// silent — a keystroke that appears to do nothing reads as a hide that
	// took, and the row's return on the next open reads as data loss.
	applyHide  func(HideChange) error
	hideStatus string
}

// NewModel builds the first frame entirely from cached state.
func NewModel(snapshot Snapshot) Model {
	input := textinput.New()
	input.Prompt = "find › "
	input.Placeholder = "type project or name"
	input.CharLimit = 200
	input.SetVirtualCursor(true)
	input.SetValue(snapshot.InitialQuery)
	_ = input.Focus()

	model := Model{
		rows:            append([]compose.Row(nil), snapshot.Rows...),
		width:           positiveOr(snapshot.Width, defaultWidth),
		height:          positiveOr(snapshot.Height, defaultHeight),
		nowNS:           snapshot.NowNS,
		view:            snapshot.View,
		hiddenCount:     snapshot.HiddenCount,
		suppressedCount: snapshot.SuppressedCount,
		refreshing:      snapshot.Refreshing,
		primary:         validAccount(snapshot.PrimaryAccount, snapshot.AccountIDs),
		initialPrimary:  validAccount(snapshot.PrimaryAccount, snapshot.AccountIDs),
		accountIDs:      normalizedAccountIDs(snapshot.AccountIDs),
		cache1H:         snapshot.Cache1H,
		rotation:        snapshot.Rotation,
		query:           input,
		initialHidden:   make(map[string]bool),
		hideChanges:     make(map[string]HideChange),
		applyHide:       snapshot.ApplyHide,
		statsSampler:    snapshot.StatsSampler,
		skyEnabled:      !snapshot.NoSky,
		activity:        snapshot.Activity,
	}
	for _, row := range model.rows {
		if row.ID != "" {
			model.initialHidden[row.ID] = row.Hidden
		}
	}
	model.rebuild(snapshot.InitialCursorID, 0)
	return model
}

func positiveOr(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func validAccount(account int, rosters ...[]int) int {
	ids := []int{1, 2, 3}
	if len(rosters) != 0 {
		ids = normalizedAccountIDs(rosters[0])
	}
	for _, id := range ids {
		if account == id {
			return account
		}
	}
	return ids[0]
}

func normalizedAccountIDs(values []int) []int {
	if len(values) == 0 {
		return []int{1, 2, 3}
	}
	result := make([]int, 0, len(values))
	seen := make(map[int]bool, len(values))
	for _, value := range values {
		if value > 0 && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	if len(result) == 0 {
		return []int{1, 2, 3}
	}
	return result
}

// Init starts only the pure animation clock. Stats remains fully lazy: its
// first sampling command is created only by a key that focuses the Stats tab.
func (model Model) Init() tea.Cmd {
	if !model.skyEnabled {
		return nil
	}
	return skyTickCmd()
}

// Update applies one message without touching the outside world.
func (model Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		model.width = positiveOr(message.Width, model.width)
		model.height = positiveOr(message.Height, model.height)
		model.query.SetWidth(maxInt(8, model.width/2))
		return model, nil
	case RefreshMsg:
		model.applyRefresh(message.Snapshot)
		return model, nil
	case statsSampleMsg:
		if model.tab != TabStats || message.generation != model.statsGeneration {
			return model, nil
		}
		model.statsLoading = false
		if message.err != nil {
			model.statsError = message.err.Error()
		} else {
			model.statsError = ""
			model.applyStats(message.snapshot)
		}
		return model, statsWaitCmd(message.generation)
	case statsTickMsg:
		if model.tab != TabStats || message.generation != model.statsGeneration {
			return model, nil
		}
		return model, model.startStatsSample()
	case skyTickMsg:
		if !model.skyEnabled {
			return model, nil
		}
		model.nowNS = message.nowNS
		kept := model.skyEvents[:0]
		for _, event := range model.skyEvents {
			if !event.Expired(model.nowNS) {
				kept = append(kept, event)
			}
		}
		model.skyEvents = kept
		return model, skyTickCmd()
	case tea.PasteMsg:
		model.activity.Stamp(time.Now())
		model.updateQuery(model.query.Value() + message.Content)
		return model, nil
	case tea.KeyMsg:
		model.activity.Stamp(time.Now())
		return model.updateKey(message)
	default:
		return model, nil
	}
}

func (model Model) updateKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := message.String()
	if key == "esc" || key == "ctrl+c" {
		model.outcome = OutcomeCancelled
		return model, tea.Quit
	}
	switch key {
	case "left":
		return model.navigateHorizontal(-1)
	case "right":
		return model.navigateHorizontal(1)
	}
	if model.tab != TabChats {
		return model.updateStatsKey(key)
	}
	switch key {
	case "ctrl+t":
		model.outcome = OutcomeReload
		return model, tea.Quit
	case "ctrl+r":
		follow := model.selectedKey()
		if count := len(model.groups); count > 0 {
			model.rotation = (model.rotation + 1) % count
		}
		model.rebuild(follow, model.cursor)
		return model, nil
	case "ctrl+x":
		model.toggleHidden()
		return model, nil
	case "ctrl+e":
		model.cache1H = !model.cache1H
		return model, nil
	case "ctrl+s":
		for index, account := range model.accountIDs {
			if account == model.primary {
				model.primary = model.accountIDs[(index+1)%len(model.accountIDs)]
				break
			}
		}
		return model, nil
	// ⌃O, not ⌃B: the picker always runs inside tmux and C-b is tmux's PREFIX,
	// so tmux swallowed the keystroke before the picker ever saw it. Any
	// replacement must stay clear of the tmux prefix.
	case "ctrl+o":
		if row, ok := model.selectedRow(); ok && isLive(row.Kind) {
			model.outcome = OutcomeReboot
			model.outcomeRow = row
			return model, tea.Quit
		}
		return model, nil
	case "enter":
		if row, ok := model.selectedRow(); ok {
			model.outcome = OutcomeSelected
			model.outcomeRow = row
			return model, tea.Quit
		}
		return model, nil
	case "up", "ctrl+p":
		if model.cursor > 0 {
			model.cursor--
		}
		return model, nil
	case "down", "ctrl+n":
		if model.cursor+1 < len(model.filtered) {
			model.cursor++
		}
		return model, nil
	case "pgup":
		model.cursor = maxInt(0, model.cursor-model.pageRows())
		return model, nil
	case "pgdown":
		model.cursor = minInt(
			maxInt(0, len(model.filtered)-1),
			model.cursor+model.pageRows(),
		)
		return model, nil
	case "home":
		model.cursor = 0
		return model, nil
	case "end":
		model.cursor = maxInt(0, len(model.filtered)-1)
		return model, nil
	}

	switch key {
	case "backspace", "ctrl+h":
		runes := []rune(model.query.Value())
		if len(runes) != 0 {
			model.updateQuery(string(runes[:len(runes)-1]))
		}
	case "ctrl+w":
		model.updateQuery(trimLastWord(model.query.Value()))
	case "ctrl+u":
		model.updateQuery("")
	default:
		if text := message.Key().Text; text != "" {
			model.updateQuery(model.query.Value() + text)
		}
	}
	return model, nil
}

func (model Model) navigateHorizontal(direction int) (tea.Model, tea.Cmd) {
	if model.tab == TabStats && model.statsFocus == StatsFocusSubtab {
		model.statsSubtab = StatsSubtab((int(model.statsSubtab) + int(statsSubtabCount) + direction) % int(statsSubtabCount))
		return model, nil
	}
	if model.tab == TabStats && model.statsFocus == StatsFocusContent {
		return model, nil
	}
	previous := model.tab
	model.tab = Tab((int(model.tab) + int(tabCount) + direction) % int(tabCount))
	if previous != TabStats && model.tab == TabStats {
		model.statsFocus = StatsFocusTop
		return model, model.startStatsSample()
	}
	return model, nil
}

func (model Model) updateStatsKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "down", "ctrl+n":
		switch model.statsFocus {
		case StatsFocusTop:
			model.statsFocus = StatsFocusSubtab
		case StatsFocusSubtab:
			model.statsFocus = StatsFocusContent
		case StatsFocusContent:
			if model.statsSubtab == StatsChats && model.statsCursor+1 < len(model.stats.Chats) {
				model.statsCursor++
			} else if model.statsSubtab == StatsDocker && model.statsDockerCursor+1 < len(model.stats.Docker) {
				model.statsDockerCursor++
			}
		}
	case "up", "ctrl+p":
		switch model.statsFocus {
		case StatsFocusSubtab:
			model.statsFocus = StatsFocusTop
		case StatsFocusContent:
			if model.statsSubtab == StatsChats && model.statsCursor > 0 {
				model.statsCursor--
			} else if model.statsSubtab == StatsDocker && model.statsDockerCursor > 0 {
				model.statsDockerCursor--
			} else {
				model.statsFocus = StatsFocusSubtab
			}
		}
	case "c":
		model.statsSort = StatsSortCPU
		model.sortStats(model.selectedStatsKey())
	case "m":
		model.statsSort = StatsSortRAM
		model.sortStats(model.selectedStatsKey())
	}
	return model, nil
}

type statsSampleMsg struct {
	generation uint64
	snapshot   pfmstats.Snapshot
	err        error
}

type statsTickMsg struct{ generation uint64 }

type skyTickMsg struct{ nowNS int64 }

func skyTickCmd() tea.Cmd {
	return tea.Tick(125*time.Millisecond, func(now time.Time) tea.Msg {
		return skyTickMsg{nowNS: now.UnixNano()}
	})
}

func liveSockets(rows []compose.Row) map[string]bool {
	sockets := make(map[string]bool)
	for _, row := range rows {
		if row.Socket != "" && isLiveNameGroupRow(row.Kind) {
			sockets[row.Socket] = true
		}
	}
	return sockets
}

func liveEngineCounts(rows []compose.Row) (claude, codex int) {
	for _, row := range rows {
		switch row.Kind {
		case compose.LiveCodex:
			codex++
		case compose.LiveSplit:
			claude += maxInt(1, row.SplitCount)
		case compose.LiveClaude, compose.Agent, compose.Booting:
			claude++
		}
	}
	return claude, codex
}

func (model *Model) startStatsSample() tea.Cmd {
	if model.statsSampler == nil || model.statsLoading {
		return nil
	}
	model.statsGeneration++
	generation := model.statsGeneration
	model.statsLoading = true
	rows := append([]compose.Row(nil), model.rows...)
	sampler := model.statsSampler
	return func() tea.Msg {
		snapshot, err := sampler.Sample(rows)
		return statsSampleMsg{generation: generation, snapshot: snapshot, err: err}
	}
}

func statsWaitCmd(generation uint64) tea.Cmd {
	return tea.Tick(statsRefreshInterval, func(time.Time) tea.Msg {
		return statsTickMsg{generation: generation}
	})
}

func (model *Model) applyStats(snapshot pfmstats.Snapshot) {
	follow := model.selectedStatsKey()
	model.stats = snapshot
	model.sortStats(follow)
}

func (model *Model) sortStats(follow string) {
	sort.SliceStable(model.stats.Chats, func(left, right int) bool {
		if model.statsSort == StatsSortRAM {
			if model.stats.Chats[left].RSSBytes != model.stats.Chats[right].RSSBytes {
				return model.stats.Chats[left].RSSBytes > model.stats.Chats[right].RSSBytes
			}
		} else if model.stats.Chats[left].CPUPercent != model.stats.Chats[right].CPUPercent {
			return model.stats.Chats[left].CPUPercent > model.stats.Chats[right].CPUPercent
		}
		return model.stats.Chats[left].Socket < model.stats.Chats[right].Socket
	})
	if len(model.stats.Chats) == 0 {
		model.statsCursor = 0
		return
	}
	if follow != "" {
		for index := range model.stats.Chats {
			if model.stats.Chats[index].Socket == follow {
				model.statsCursor = index
				return
			}
		}
	}
	model.statsCursor = minInt(model.statsCursor, len(model.stats.Chats)-1)
}

func (model Model) selectedStatsKey() string {
	if model.statsCursor < 0 || model.statsCursor >= len(model.stats.Chats) {
		return ""
	}
	return model.stats.Chats[model.statsCursor].Socket
}

func (model *Model) updateQuery(value string) {
	follow := model.selectedKey()
	if runes := []rune(value); len(runes) > model.query.CharLimit &&
		model.query.CharLimit > 0 {
		value = string(runes[:model.query.CharLimit])
	}
	model.query.SetValue(value)
	model.refilter(follow, 0)
}

func trimLastWord(value string) string {
	value = strings.TrimRight(value, " \t")
	index := strings.LastIndexAny(value, " \t")
	if index < 0 {
		return ""
	}
	return value[:index+1]
}

func (model *Model) applyRefresh(snapshot Snapshot) {
	follow := model.selectedKey()
	fallback := model.cursor
	if model.skyEnabled {
		before := liveSockets(model.rows)
		after := liveSockets(snapshot.Rows)
		eventTime := snapshot.NowNS
		if eventTime == 0 {
			eventTime = model.nowNS
		}
		for socket := range after {
			if !before[socket] {
				model.skyEvents = append(model.skyEvents, sky.Event{
					Kind: sky.EventBirth, StartNS: eventTime,
				})
			}
		}
		for socket := range before {
			if !after[socket] {
				model.skyEvents = append(model.skyEvents, sky.Event{
					Kind: sky.EventDeath, StartNS: eventTime,
				})
			}
		}
	}
	model.rows = append(model.rows[:0], snapshot.Rows...)
	model.nowNS = snapshot.NowNS
	model.view = snapshot.View
	model.hiddenCount = snapshot.HiddenCount
	model.suppressedCount = snapshot.SuppressedCount
	model.refreshing = snapshot.Refreshing
	for index := range model.rows {
		id := model.rows[index].ID
		if id != "" {
			if _, found := model.initialHidden[id]; !found {
				model.initialHidden[id] = model.rows[index].Hidden
			}
		}
		if change, ok := model.hideChanges[id]; ok {
			if model.rows[index].Hidden != change.Hidden {
				model.adjustHiddenCount(change.Hidden)
			}
			model.rows[index].Hidden = change.Hidden
		}
	}
	model.rebuild(follow, fallback)
}

func (model *Model) toggleHidden() {
	if len(model.filtered) == 0 {
		return
	}
	fallback := model.cursor
	index := model.filtered[model.cursor]
	row := model.rows[index]
	// Refusals are SPOKEN, never silent: a swallowed ⌃X looks identical to a
	// hide that landed, and the row's return on the next open then reads as
	// the store losing the order.
	// A booting row's ID is its crumbless socket, not a chat identity — a hide
	// keyed by it stops meaning anything once the crumb appears (compose's
	// applyHide carries the same exclusion on the read side).
	// A "_HIDE…" label is refused for a different reason: the row IS hideable,
	// but its label is what hides it, so an unhide written here would delete a
	// store row that is not holding it down and the chat would stay hidden
	// anyway. Renaming it is the unhide.
	switch {
	case row.Kind == compose.Booting:
		model.hideStatus = "⌃X refused — " + row.Name +
			" is still booting (no identity yet); retry once it settles"
		return
	case row.Kind == compose.LiveSplit:
		model.hideStatus = "⌃X refused — split live window; hide its chats individually"
		return
	case row.NameHidden:
		model.hideStatus = "⌃X refused — hidden by its _HIDE label; rename it to unhide"
		return
	case row.ID == "":
		model.hideStatus = "⌃X refused — " + row.Name + " is not a chat yet"
		return
	}
	change := HideChange{
		ID:     row.ID,
		Engine: rowEngine(row.Kind),
		Hidden: !row.Hidden,
		Socket: row.Socket,
		Live:   isLive(row.Kind),
		Name:   row.Name,
	}
	// ⌃X lands NOW — the store write, and the kill when the row is live. It
	// used to be held until the picker quit, which meant only the exits that
	// launched something ever applied it: closing the list the natural way
	// discarded every mark that had just been typed.
	// A nil applier means nothing can persist this keystroke: refuse rather
	// than paint a cosmetic hide the next open would take back.
	if model.applyHide == nil {
		model.hideStatus = "⌃X refused — this picker cannot write hides"
		return
	}
	if err := model.applyHide(change); err != nil {
		model.hideStatus = fmt.Sprintf("⌃X failed — %s: %v", change.Name, err)
		return
	}
	switch {
	case change.Hidden && change.Live:
		model.hideStatus = "ended + hidden — " + change.Name
	case change.Hidden:
		model.hideStatus = "hidden — " + change.Name
	default:
		model.hideStatus = "unhidden — " + change.Name
	}
	follow := rowKey(row)
	model.rows[index].Hidden = change.Hidden
	model.adjustHiddenCount(change.Hidden)
	model.hideChanges[change.ID] = change
	// The chat was running and has just been ended: demote the row rather than
	// leave it claiming a server that no longer exists.
	if change.Hidden && change.Live {
		model.rows[index] = demoteToResumable(model.rows[index])
	}
	model.rebuild(follow, fallback)
}

// demoteToResumable rewrites a row whose server has just been killed, mirroring
// what a reboot does to one: the chat is still resumable from its transcript,
// but every handle to the dead server is gone.
func demoteToResumable(row compose.Row) compose.Row {
	if row.Kind == compose.LiveCodex {
		row.Kind = compose.ResumeCodex
	} else {
		row.Kind = compose.ResumeClaude
	}
	row.Socket = ""
	row.PaneID = ""
	row.SessionName = ""
	row.WindowName = ""
	row.Attached = false
	return row
}

func (model *Model) adjustHiddenCount(hidden bool) {
	if hidden {
		model.hiddenCount++
	} else if model.hiddenCount > 0 {
		model.hiddenCount--
	}
}

func (model *Model) rebuild(follow string, fallback int) {
	model.search = make([]string, len(model.rows))
	byProject := make(map[string]int)
	model.groups = model.groups[:0]
	for index, row := range model.rows {
		project := cleanField(row.Project)
		if project == "" {
			project = "?"
		}
		model.search[index] = strings.ToLower(
			project + " " + cleanField(row.Name),
		)
		groupIndex, found := byProject[project]
		if !found {
			groupIndex = len(model.groups)
			byProject[project] = groupIndex
			model.groups = append(model.groups, projectGroup{name: project})
		}
		model.groups[groupIndex].indices = append(
			model.groups[groupIndex].indices,
			index,
		)
	}
	model.rebuildOrder()
	model.refilter(follow, fallback)
}

func (model *Model) rebuildOrder() {
	model.order = model.order[:0]
	model.nameGroups = make(map[int]nameGroup)
	if len(model.groups) == 0 {
		model.rotation = 0
		return
	}
	model.rotation %= len(model.groups)
	if model.rotation < 0 {
		model.rotation += len(model.groups)
	}
	for offset := range model.groups {
		group := model.groups[(model.rotation+offset)%len(model.groups)]
		members := make(map[string][]int)
		for _, index := range group.indices {
			row := model.rows[index]
			if !model.visibleInView(row) || !isLiveNameGroupRow(row.Kind) {
				continue
			}
			if prefix, ok := nameGroupPrefix(row.Name); ok {
				members[prefix] = append(members[prefix], index)
			}
		}
		emitted := make(map[string]bool)
		for _, index := range group.indices {
			if !model.visibleInView(model.rows[index]) {
				continue
			}
			prefix, grouped := nameGroupPrefix(model.rows[index].Name)
			grouped = grouped && len(members[prefix]) >= 2
			if !grouped {
				model.order = append(model.order, index)
				continue
			}
			if emitted[prefix] {
				continue
			}
			emitted[prefix] = true
			for _, member := range members[prefix] {
				model.nameGroups[member] = nameGroup{name: prefix, count: len(members[prefix])}
				model.order = append(model.order, member)
			}
		}
	}
}

func nameGroupPrefix(name string) (string, bool) {
	prefix, _, found := strings.Cut(cleanField(name), ":")
	prefix = strings.TrimSpace(prefix)
	return prefix, found && prefix != ""
}

func isLiveNameGroupRow(kind compose.Kind) bool {
	return isLive(kind) || kind == compose.Agent || kind == compose.Booting
}

func (model *Model) refilter(follow string, fallback int) {
	query := strings.TrimSpace(model.query.Value())
	model.filtered = model.filtered[:0]
	if query == "" {
		model.filtered = append(model.filtered, model.order...)
	} else {
		query = strings.ToLower(query)
		matched := make([]bool, len(model.rows))
		fuzzyOrder := make([]int, 0, len(model.order))
		for _, rowIndex := range model.order {
			candidate := model.search[rowIndex]
			if strings.Contains(candidate, query) {
				matched[rowIndex] = true
			} else if runeSubsequence(query, candidate) {
				fuzzyOrder = append(fuzzyOrder, rowIndex)
			}
		}
		if len(fuzzyOrder) != 0 {
			source := orderedSearch{
				search: model.search,
				order:  fuzzyOrder,
			}
			for _, match := range fuzzy.FindFromNoSort(query, source) {
				matched[fuzzyOrder[match.Index]] = true
			}
		}
		for _, rowIndex := range model.order {
			if matched[rowIndex] {
				model.filtered = append(model.filtered, rowIndex)
			}
		}
	}
	if len(model.filtered) == 0 {
		model.cursor = 0
		return
	}
	if follow != "" {
		for cursor, index := range model.filtered {
			if rowKey(model.rows[index]) == follow {
				model.cursor = cursor
				return
			}
		}
	}
	model.cursor = minInt(maxInt(fallback, 0), len(model.filtered)-1)
}

func runeSubsequence(pattern, candidate string) bool {
	if pattern == "" {
		return true
	}
	patternRunes := []rune(pattern)
	position := 0
	for _, candidateRune := range candidate {
		if candidateRune == patternRunes[position] {
			position++
			if position == len(patternRunes) {
				return true
			}
		}
	}
	return false
}

func (model Model) visibleInView(row compose.Row) bool {
	switch model.view {
	case compose.AllView:
		return true
	case compose.HiddenView:
		return row.Hidden
	default:
		return !row.Hidden
	}
}

func (model Model) pageRows() int {
	return maxInt(1, model.height-8)
}

func (model Model) selectedRow() (compose.Row, bool) {
	if model.cursor < 0 || model.cursor >= len(model.filtered) {
		return compose.Row{}, false
	}
	return model.rows[model.filtered[model.cursor]], true
}

func (model Model) selectedKey() string {
	row, ok := model.selectedRow()
	if !ok {
		return ""
	}
	return rowKey(row)
}

func rowKey(row compose.Row) string {
	if row.ID != "" {
		return row.ID
	}
	if row.Socket != "" {
		return row.Kind.String() + "\x00" + row.Socket + "\x00" + row.PaneID
	}
	return row.Kind.String() + "\x00" + row.CWD + "\x00" + row.Project
}

func rowEngine(kind compose.Kind) string {
	switch kind {
	case compose.LiveCodex, compose.ResumeCodex, compose.NewCodex:
		return "cx"
	default:
		return "cc"
	}
}

func isLive(kind compose.Kind) bool {
	return kind == compose.LiveClaude ||
		kind == compose.LiveCodex ||
		kind == compose.LiveSplit
}

// Result returns the effects accumulated by the pure model. Cancelled reverts
// PrimaryAccount to what the picker opened with, since a ⌃S account switch is
// only a pending intent until the picker exits deliberately.
//
// HideChanges survives a cancel: it is no longer a request to apply anything,
// it is the RECEIPT of what ⌃X already did while the picker was open. Esc
// closes the list; it does not resurrect a chat that was hidden and killed.
func (model Model) Result() Outcome {
	if model.outcome == OutcomeCancelled {
		return Outcome{
			Kind:           OutcomeCancelled,
			PrimaryAccount: model.initialPrimary,
			Cache1H:        model.cache1H,
			Rotation:       model.rotation,
			Query:          model.query.Value(),
			HideChanges:    model.appliedChanges(),
		}
	}
	return Outcome{
		Kind:           model.outcome,
		Row:            model.outcomeRow,
		PrimaryAccount: model.primary,
		Cache1H:        model.cache1H,
		Rotation:       model.rotation,
		Query:          model.query.Value(),
		HideChanges:    model.appliedChanges(),
	}
}

func (model Model) appliedChanges() []HideChange {
	changes := make([]HideChange, 0, len(model.hideChanges))
	for _, change := range model.hideChanges {
		changes = append(changes, change)
	}
	sort.Slice(changes, func(left, right int) bool {
		return changes[left].ID < changes[right].ID
	})
	return changes
}

// VisibleRows returns a copy for model tests and noninteractive adapters.
func (model Model) VisibleRows() []compose.Row {
	rows := make([]compose.Row, 0, len(model.filtered))
	for _, index := range model.filtered {
		rows = append(rows, model.rows[index])
	}
	return rows
}

func (model Model) Cursor() int           { return model.cursor }
func (model Model) Rotation() int         { return model.rotation }
func (model Model) Query() string         { return model.query.Value() }
func (model Model) PrimaryAccount() int   { return model.primary }
func (model Model) Cache1H() bool         { return model.cache1H }
func (model Model) Tab() Tab              { return model.tab }
func (model Model) SelectedKey() string   { return model.selectedKey() }
func (model Model) GroupCount() int       { return len(model.groups) }
func (model Model) OrderedRowCount() int  { return len(model.order) }
func (model Model) FilteredRowCount() int { return len(model.filtered) }
func (model Model) ValidUTF8Query() bool  { return utf8.ValidString(model.Query()) }
func (model Model) HasVisibleSelection() bool { // compact invariant helper
	return len(model.filtered) == 0 ||
		(model.cursor >= 0 && model.cursor < len(model.filtered))
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
