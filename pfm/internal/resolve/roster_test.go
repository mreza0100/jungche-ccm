package resolve

import (
	"strings"
	"testing"
)

func TestResolveRosterNamePrefersTheUniqueLiveRow(t *testing.T) {
	live := RosterCandidate{Name: "seat", ID: "thread", Socket: "cx-1-2-3", Live: true}
	got, found, err := ResolveRosterName([]RosterCandidate{
		{Name: "seat", ID: "thread"}, live,
	}, "seat")
	if err != nil || !found || got != live {
		t.Fatalf("ResolveRosterName()=(%+v,%t,%v), want live row", got, found, err)
	}
}

func TestResolveRosterNameAmbiguityNamesStableAddresses(t *testing.T) {
	_, found, err := ResolveRosterName([]RosterCandidate{
		{Name: "seat", ID: "one", Socket: "cx-1", Pane: "%1", Live: true},
		{Name: "seat", ID: "two", Socket: "cx-2", Pane: "%2", Live: true},
	}, "seat")
	if found || err == nil {
		t.Fatalf("ResolveRosterName() found=%t err=%v, want ambiguity", found, err)
	}
	for _, want := range []string{"thread id", "one", "two", "cx-1", "cx-2"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("ambiguity=%q, want %q", err, want)
		}
	}
}
