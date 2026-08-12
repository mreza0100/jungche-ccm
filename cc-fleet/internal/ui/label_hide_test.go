package ui

import (
	"testing"

	"hostops/cc-fleet/internal/compose"
)

// TestHideKeyRefusesALabelHiddenRow pins the one place the two hide kinds must
// not be confused: a "_HIDE…" chat is held out of the list by its LABEL, so an
// unhide written from the picker would delete a store row that is not holding
// it down and the chat would stay hidden anyway. The key does nothing instead.
func TestHideKeyRefusesALabelHiddenRow(t *testing.T) {
	snapshot := Snapshot{
		View:           compose.HiddenView,
		HiddenCount:    1,
		PrimaryAccount: 1,
		NowNS:          fixtureNowNS,
		Width:          120,
		Height:         17,
		Rows: []compose.Row{{
			Kind:        compose.ResumeClaude,
			ID:          "33333333-3333-4333-8333-333333333333",
			Name:        "_HIDE headless worker",
			Project:     "alpha",
			CWD:         "/work/alpha",
			Size:        1024,
			PromptCount: 4,
			ActivityNS:  fixtureNowNS,
			Account:     1,
			Hidden:      true,
			NameHidden:  true,
		}},
	}

	model := NewModel(snapshot)
	model, _ = applyKey(t, model, controlKey('x'))
	result := model.Result()
	if len(result.HideChanges) != 0 {
		t.Fatalf("hide key wrote a change for a label-hidden row: %#v", result.HideChanges)
	}
	if !model.rows[0].Hidden || !model.rows[0].NameHidden {
		t.Fatalf("label-hidden row was flipped in the model: %#v", model.rows[0])
	}
}
