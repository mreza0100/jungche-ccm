package ui

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/sahilm/fuzzy"

	"hostops/cc-fleet/internal/action"
	"hostops/cc-fleet/internal/compose"
)

const (
	defaultWidth  = 120
	defaultHeight = 28
)

type projectGroup struct {
	name    string
	indices []int
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
	rows            []compose.Row
	search          []string
	groups          []projectGroup
	order           []int
	filtered        []int
	cursor          int
	width           int
	height          int
	nowNS           int64
	view            compose.View
	hiddenCount     int
	suppressedCount int
	refreshing      bool
	primary         int
	initialPrimary  int
	cache1H         bool
	rotation        int
	query           textinput.Model
	outcome         OutcomeKind
	outcomeRow      compose.Row
	initialHidden   map[string]bool
	hideChanges     map[string]HideChange
	// applyHide performs a ⌃X the instant it is typed. hideError holds what
	// went wrong if it could not, so a refused hide is visible instead of a
	// keystroke that appeared to do nothing.
	applyHide func(HideChange) error
	hideError string
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
		primary:         validAccount(snapshot.PrimaryAccount),
		initialPrimary:  validAccount(snapshot.PrimaryAccount),
		cache1H:         snapshot.Cache1H,
		rotation:        snapshot.Rotation,
		query:           input,
		initialHidden:   make(map[string]bool),
		hideChanges:     make(map[string]HideChange),
		applyHide:       snapshot.ApplyHide,
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

func validAccount(account int) int {
	if account < 1 || account > action.MaxAccount {
		return 1
	}
	return account
}

// Init intentionally starts no timers or I/O.
func (Model) Init() tea.Cmd {
	return nil
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
	case tea.PasteMsg:
		model.updateQuery(model.query.Value() + message.Content)
		return model, nil
	case tea.KeyMsg:
		return model.updateKey(message)
	default:
		return model, nil
	}
}

func (model Model) updateKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := message.String()
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
		model.primary = model.primary%action.MaxAccount + 1
		return model, nil
	// ⌃O, not ⌃B: the picker always runs inside tmux and C-b is tmux's PREFIX,
	// so tmux swallowed the keystroke before the picker ever saw it. Any
	// replacement must stay clear of the tmux prefix (cc-fleet.zsh:1394-1406).
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
	case "esc", "ctrl+c":
		model.outcome = OutcomeCancelled
		return model, tea.Quit
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
	// A booting row's ID is its crumbless socket, not a chat identity — unlike
	// LiveSplit's empty-ID case it would otherwise pass the check above, so it
	// needs its own guard here too (compose's applyHide carries the same
	// exclusion on the read side; neither may let a hide land on an identity
	// that stops meaning anything once the crumb appears).
	// A "_HIDE…" label is refused for a different reason than the three above:
	// the row IS hideable, but its label is what hides it, so an unhide
	// written here would delete a store row that is not holding it down and
	// the chat would stay hidden anyway. Renaming it is the unhide.
	if model.rows[index].ID == "" ||
		model.rows[index].Kind == compose.LiveSplit ||
		model.rows[index].Kind == compose.Booting ||
		model.rows[index].NameHidden {
		return
	}
	row := model.rows[index]
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
	if model.applyHide != nil {
		if err := model.applyHide(change); err != nil {
			model.hideError = fmt.Sprintf("%s: %v", change.Name, err)
			return
		}
	}
	model.hideError = ""
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
		for _, index := range group.indices {
			if model.visibleInView(model.rows[index]) {
				model.order = append(model.order, index)
			}
		}
	}
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
