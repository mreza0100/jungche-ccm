package ui

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"hostops/cc-fleet/internal/action"
	"hostops/cc-fleet/internal/compose"
)

func TestModelKeysRotationHideModifiersReloadAndCancel(t *testing.T) {
	snapshot := fixtureSnapshot(120)
	snapshot.InitialCursorID = snapshot.Rows[1].ID
	model := NewModel(snapshot)
	follow := model.SelectedKey()
	model, command := applyKey(t, model, controlKey('r'))
	if command != nil || model.SelectedKey() != follow || model.Rotation() != 1 {
		t.Fatalf(
			"rotation command=%v selected=%q rotation=%d",
			command,
			model.SelectedKey(),
			model.Rotation(),
		)
	}

	beforeRows := append([]compose.Row(nil), snapshot.Rows...)
	model, _ = applyKey(t, model, controlKey('x'))
	result := model.Result()
	if model.SelectedKey() != follow ||
		len(result.HideChanges) != 1 ||
		!result.HideChanges[0].Hidden ||
		result.HideChanges[0].Engine != "cx" {
		t.Fatalf("hide selected=%q result=%#v", model.SelectedKey(), result)
	}
	if !reflect.DeepEqual(snapshot.Rows, beforeRows) {
		t.Fatal("pure model mutated input snapshot")
	}
	// A second ⌃X is an UNHIDE, applied like the first one. The receipt keeps
	// the row's final state rather than cancelling out to nothing: both writes
	// really happened.
	model, _ = applyKey(t, model, controlKey('x'))
	undo := model.Result()
	if len(undo.HideChanges) != 1 || undo.HideChanges[0].Hidden {
		t.Fatalf("second ⌃X did not record an unhide: %#v", undo)
	}

	model, _ = applyKey(t, model, controlKey('e'))
	if model.Cache1H() {
		t.Fatal("ctrl+e did not toggle 1h off")
	}
	// ⌃S cycles the roster and wraps: the fixture starts on 2, the last account,
	// so the very next press must come back to 1. A cycle hardcoded wider than
	// the roster is what froze the account picker on an account nothing accepts.
	for step := 0; step < action.MaxAccount+1; step++ {
		before := model.PrimaryAccount()
		model, _ = applyKey(t, model, controlKey('s'))
		want := before%action.MaxAccount + 1
		if model.PrimaryAccount() != want {
			t.Fatalf("account=%d want=%d", model.PrimaryAccount(), want)
		}
	}

	reload, command := applyKey(t, model, controlKey('t'))
	if command == nil || reload.Result().Kind != OutcomeReload {
		t.Fatalf("reload command=%v result=%#v", command, reload.Result())
	}
	cancelled, command := applyKey(t, model, specialKey(tea.KeyEscape))
	if command == nil || cancelled.Result().Kind != OutcomeCancelled {
		t.Fatalf("cancel command=%v result=%#v", command, cancelled.Result())
	}
}

// TestCancelKeepsAppliedHidesAndDropsPrimaryChange pins the split that ⌃X
// acting immediately creates: Esc still abandons a ⌃S account switch, which is
// only ever a pending intent, but it can no longer take back a hide. Batching
// the hide until quit meant the only exits that applied one were the ones that
// launched something, so closing the list the natural way silently threw every
// ⌃X away.
func TestCancelKeepsAppliedHidesAndDropsPrimaryChange(t *testing.T) {
	snapshot := fixtureSnapshot(120)
	snapshot.PrimaryAccount = 1
	var applied []HideChange
	snapshot.ApplyHide = func(change HideChange) error {
		applied = append(applied, change)
		return nil
	}
	model := NewModel(snapshot)

	model, _ = applyKey(t, model, controlKey('x'))
	if len(applied) != 1 || !applied[0].Hidden {
		t.Fatalf("⌃X did not apply on the keypress: %#v", applied)
	}
	model, _ = applyKey(t, model, controlKey('s'))
	if model.PrimaryAccount() == 1 {
		t.Fatal("setup: ⌃S did not change the account")
	}

	cancelled, command := applyKey(t, model, specialKey(tea.KeyEscape))
	if command == nil {
		t.Fatal("cancel returned no quit command")
	}
	result := cancelled.Result()
	if result.Kind != OutcomeCancelled ||
		len(result.HideChanges) != 1 ||
		!result.HideChanges[0].Hidden ||
		result.PrimaryAccount != 1 {
		t.Fatalf(
			"cancel result=%#v, want Cancelled/hide kept/account reverted to 1",
			result,
		)
	}
	if len(applied) != 1 {
		t.Fatalf("Esc re-applied or reverted the hide: %#v", applied)
	}
}

// TestHideAppliesImmediatelyAndEndsALiveChat is the whole point of the change:
// the keypress writes the hide AND kills the chat's server, and the row stops
// claiming a server that is gone.
func TestHideAppliesImmediatelyAndEndsALiveChat(t *testing.T) {
	snapshot := fixtureSnapshot(120)
	var live compose.Row
	for _, row := range snapshot.Rows {
		if row.Kind == compose.LiveClaude && row.Socket != "" {
			live = row
			break
		}
	}
	if live.ID == "" {
		t.Skip("fixture has no live claude row with a socket")
	}
	snapshot.Rows = []compose.Row{live}
	snapshot.InitialCursorID = live.ID
	var applied []HideChange
	snapshot.ApplyHide = func(change HideChange) error {
		applied = append(applied, change)
		return nil
	}
	model := NewModel(snapshot)

	model, _ = applyKey(t, model, controlKey('x'))
	if len(applied) != 1 {
		t.Fatalf("⌃X did not apply on the keypress: %#v", applied)
	}
	if !applied[0].Live || applied[0].Socket != live.Socket {
		t.Fatalf("the kill had nothing to aim at: %#v", applied[0])
	}
	if got := model.rows[0]; got.Kind != compose.ResumeClaude ||
		got.Socket != "" || got.PaneID != "" {
		t.Fatalf("killed row still claims a live server: %#v", got)
	}
}

// TestHideThatCannotLandLeavesTheRowAlone: a ⌃X that fails must not pretend it
// worked. The row keeps its state and the failure takes the status line.
func TestHideThatCannotLandLeavesTheRowAlone(t *testing.T) {
	snapshot := fixtureSnapshot(120)
	snapshot.ApplyHide = func(HideChange) error {
		return errors.New("store is locked")
	}
	model := NewModel(snapshot)
	before := model.rows[model.filtered[model.cursor]].Hidden

	model, _ = applyKey(t, model, controlKey('x'))
	if model.rows[model.filtered[model.cursor]].Hidden != before {
		t.Fatal("a failed ⌃X flipped the row anyway")
	}
	if len(model.Result().HideChanges) != 0 {
		t.Fatalf("a failed ⌃X was recorded: %#v", model.Result().HideChanges)
	}
	if !strings.Contains(model.View().Content, "store is locked") {
		t.Fatal("a failed ⌃X was invisible to the user")
	}
}

func TestEnterOutcomeEveryKindAndLiveReboot(t *testing.T) {
	for _, row := range fixtureSnapshot(120).Rows {
		snapshot := Snapshot{
			Rows:           []compose.Row{row},
			View:           compose.AllView,
			PrimaryAccount: action.MaxAccount,
			Cache1H:        true,
			NowNS:          fixtureNowNS,
		}
		model := NewModel(snapshot)
		selected, command := applyKey(t, model, specialKey(tea.KeyEnter))
		result := selected.Result()
		if command == nil ||
			result.Kind != OutcomeSelected ||
			rowKey(result.Row) != rowKey(row) ||
			result.PrimaryAccount != action.MaxAccount ||
			!result.Cache1H {
			t.Fatalf("%s enter command=%v result=%#v", row.Kind, command, result)
		}

		// Reboot is ⌃O, never ⌃B: the picker always runs inside tmux, and C-b is
		// tmux's prefix — it never reaches the picker (cc-fleet.zsh:1394-1400).
		reboot, rebootCommand := applyKey(t, model, controlKey('o'))
		if isLive(row.Kind) {
			if rebootCommand == nil ||
				reboot.Result().Kind != OutcomeReboot ||
				rowKey(reboot.Result().Row) != rowKey(row) {
				t.Fatalf("%s reboot=%#v cmd=%v", row.Kind, reboot.Result(), rebootCommand)
			}
		} else if rebootCommand != nil || reboot.Result().Kind != OutcomeNone {
			t.Fatalf("%s unexpectedly rebooted: %#v", row.Kind, reboot.Result())
		}

		stale, staleCommand := applyKey(t, model, controlKey('b'))
		if staleCommand != nil || stale.Result().Kind != OutcomeNone {
			t.Fatalf("%s rebooted on the tmux prefix key: %#v", row.Kind, stale.Result())
		}
	}
}

// TestBootingRowIgnoresHideAndRebootButSelectsOnEnter is the pure-model half
// of the booting-row fix: ⌃X and ⌃O must both be no-ops on a booting row —
// its "id" is a crumbless socket, not a chat identity, and stops meaning
// anything once the crumb lands — while Enter still selects it normally, the
// one live operation (attach) this Kind supports.
func TestBootingRowIgnoresHideAndRebootButSelectsOnEnter(t *testing.T) {
	row := compose.Row{
		Kind:    compose.Booting,
		ID:      "cc-new-CC_FLEET_1",
		Socket:  "cc-new-CC_FLEET_1",
		Name:    "booting…",
		Project: "booting-project",
	}
	snapshot := Snapshot{
		Rows:           []compose.Row{row},
		View:           compose.DefaultView,
		PrimaryAccount: 1,
		NowNS:          fixtureNowNS,
	}
	model := NewModel(snapshot)

	model, command := applyKey(t, model, controlKey('x'))
	if command != nil || len(model.Result().HideChanges) != 0 || model.rows[0].Hidden {
		t.Fatalf(
			"⌃X on a booting row was not a no-op: command=%v result=%#v",
			command,
			model.Result(),
		)
	}

	reboot, rebootCommand := applyKey(t, model, controlKey('o'))
	if rebootCommand != nil || reboot.Result().Kind != OutcomeNone {
		t.Fatalf("⌃O on a booting row was not a no-op: %#v", reboot.Result())
	}

	selected, selectCommand := applyKey(t, model, specialKey(tea.KeyEnter))
	result := selected.Result()
	if selectCommand == nil ||
		result.Kind != OutcomeSelected ||
		rowKey(result.Row) != rowKey(row) {
		t.Fatalf("Enter on a booting row = command=%v result=%#v", selectCommand, result)
	}
}

func TestAccountsOffTheRosterFallBackToTheFirst(t *testing.T) {
	// The roster is cc-db.sh's (1-2). A stale ~/.claude-primary naming a retired
	// account must open the picker on account 1, not on an account no launcher
	// can reach.
	for _, account := range []int{0, -1, action.MaxAccount + 1, 9} {
		snapshot := fixtureSnapshot(120)
		snapshot.PrimaryAccount = account
		if got := NewModel(snapshot).PrimaryAccount(); got != 1 {
			t.Fatalf("account %d became %d, want 1", account, got)
		}
	}
	for account := 1; account <= action.MaxAccount; account++ {
		snapshot := fixtureSnapshot(120)
		snapshot.PrimaryAccount = account
		if got := NewModel(snapshot).PrimaryAccount(); got != account {
			t.Fatalf("account %d became %d", account, got)
		}
	}
}

func TestFuzzyFilterAndRefreshCursorFollow(t *testing.T) {
	snapshot := fixtureSnapshot(120)
	snapshot.InitialCursorID = snapshot.Rows[3].ID
	model := NewModel(snapshot)
	for _, runeValue := range "needle" {
		model, _ = applyKey(t, model, printableKey(runeValue))
	}
	rows := model.VisibleRows()
	if model.Query() != "needle" ||
		len(rows) != 1 ||
		rows[0].ID != snapshot.Rows[3].ID ||
		!model.HasVisibleSelection() {
		t.Fatalf("query=%q rows=%#v cursor=%d", model.Query(), rows, model.Cursor())
	}

	refresh := snapshot
	refresh.Rows = append([]compose.Row(nil), snapshot.Rows...)
	refresh.Rows[0], refresh.Rows[3] = refresh.Rows[3], refresh.Rows[0]
	updated, command := model.Update(RefreshMsg{Snapshot: refresh})
	if command != nil {
		t.Fatalf("refresh command=%v", command)
	}
	model = updated.(Model)
	if model.SelectedKey() != snapshot.Rows[3].ID {
		t.Fatalf("refresh cursor moved to %q", model.SelectedKey())
	}

	model = NewModel(snapshot)
	for _, runeValue := range "agndl" {
		model, _ = applyKey(t, model, printableKey(runeValue))
	}
	rows = model.VisibleRows()
	if len(rows) != 1 || rows[0].ID != snapshot.Rows[3].ID {
		t.Fatalf("non-contiguous fuzzy query rows=%#v", rows)
	}
}

func TestDefaultHideRemovesRowAndKeepsValidCursor(t *testing.T) {
	snapshot := fixtureSnapshot(80)
	snapshot.View = compose.DefaultView
	snapshot.Rows = snapshot.Rows[:3]
	snapshot.InitialCursorID = snapshot.Rows[1].ID
	model := NewModel(snapshot)
	model, _ = applyKey(t, model, controlKey('x'))
	if len(model.VisibleRows()) != 2 ||
		!model.HasVisibleSelection() ||
		model.SelectedKey() == snapshot.Rows[1].ID {
		t.Fatalf(
			"rows=%d cursor=%d selected=%q",
			len(model.VisibleRows()),
			model.Cursor(),
			model.SelectedKey(),
		)
	}
}

func applyKey(
	t *testing.T,
	model Model,
	message tea.KeyMsg,
) (Model, tea.Cmd) {
	t.Helper()
	updated, command := model.Update(message)
	result, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T", updated)
	}
	return result, command
}

func controlKey(runeValue rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: runeValue, Mod: tea.ModCtrl})
}

func printableKey(runeValue rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{
		Code: runeValue,
		Text: string(runeValue),
	})
}

func specialKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code})
}
