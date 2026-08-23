package compose

import (
	"testing"

	"hostops/pfm/internal/store"
)

func ocFixture() []store.OcSession {
	return []store.OcSession{
		{ID: "ses_live", Title: "live one", Directory: "/work/a", ProjectDir: "/work/a", TimeUpdatedMS: 9_000},
		{ID: "ses_titled", Title: "titled", Directory: "/work/b", ProjectDir: "/work/b", FirstPrompt: "fallback text", TimeUpdatedMS: 8_000},
		{ID: "ses_untitled", Title: "", Directory: "/work/c", ProjectDir: "/work/c", FirstPrompt: "untitled prompt body", TimeUpdatedMS: 7_000},
		{ID: "ses_child", ParentID: "ses_titled", Title: "child", Directory: "/work/b", ProjectDir: "/work/b", TimeUpdatedMS: 6_000},
		{ID: "ses_archived", Title: "archived", Directory: "/work/d", ProjectDir: "/work/d", TimeUpdatedMS: 5_000, TimeArchivedMS: 4_000},
	}
}

func TestOpencodeSessionsBecomeResumeRows(t *testing.T) {
	output := Compose(Input{
		OcSessions: ocFixture(),
		Options:    Options{View: AllView},
	})
	rows := make(map[string]Row)
	for _, row := range output.Rows {
		if row.Kind == ResumeOpencode {
			rows[row.ID] = row
		}
	}
	if len(rows) != 3 {
		t.Fatalf("resume-opencode rows = %d (%v), want 3", len(rows), rows)
	}
	live := rows["ses_live"]
	if live.CWD != "/work/a" || live.Name != "live one" {
		t.Errorf("ses_live = %+v", live)
	}
	if got := rows["ses_live"].ActivityNS; got != 9_000*1_000_000 {
		t.Errorf("ActivityNS = %d, want ms→ns converted", got)
	}
	if got := rows["ses_untitled"]; got.Name == "" {
		t.Error("untitled session lost its first-prompt fallback name")
	}
	if _, found := rows["ses_child"]; found {
		t.Error("subagent child earned a row")
	}
	if _, found := rows["ses_archived"]; found {
		t.Error("archived session earned a row")
	}
	for id, row := range rows {
		if EngineForKind(row.Kind) != "ox" {
			t.Errorf("%s: engine = %q, want ox", id, EngineForKind(row.Kind))
		}
	}
}

func TestOpencodeKilledSessionsAreOmittedAndCounted(t *testing.T) {
	killedAt := int64(12345)
	output := Compose(Input{
		OcSessions: []store.OcSession{
			{ID: "ses_dead", Title: "dead", Directory: "/work/x", ProjectDir: "/work/x", TimeUpdatedMS: 1},
			{ID: "ses_alive", Title: "alive", Directory: "/work/y", ProjectDir: "/work/y", TimeUpdatedMS: 2},
		},
		Killed:  []store.Killed{{ID: "ses_dead", KilledAt: killedAt}},
		Options: Options{View: AllView},
	})
	for _, row := range output.Rows {
		if row.ID == "ses_dead" && !row.Killed {
			t.Fatalf("killed oc session rendered as alive: %+v", row)
		}
	}
	if output.KilledCount != 1 {
		t.Fatalf("KilledCount = %d, want 1", output.KilledCount)
	}
}
