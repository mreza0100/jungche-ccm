package config

import (
	"strings"
	"testing"
	"time"
)

func TestNameSyncIntervalDefaultsToFifteenMinutes(t *testing.T) {
	got, err := loadTmuxConfig(t, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.NameSync.Interval != 15*time.Minute {
		t.Fatalf("NameSync.Interval = %s, want 15m", got.NameSync.Interval)
	}
	if source := got.Source("nameSync.interval"); source != SourceDefault {
		t.Fatalf("unset nameSync.interval reports source %q, want default", source)
	}
}

func TestNameSyncIntervalFromFile(t *testing.T) {
	got, err := loadTmuxConfig(t, `, "nameSync": {"interval": "2m30s"}`)
	if err != nil {
		t.Fatal(err)
	}
	if got.NameSync.Interval != 150*time.Second {
		t.Fatalf("NameSync.Interval = %s, want 2m30s", got.NameSync.Interval)
	}
	if source := got.Source("nameSync.interval"); source != SourceFile {
		t.Fatalf("nameSync.interval reports source %q, want file", source)
	}
}

// The floor is refused loudly rather than clamped: the value is rendered into
// a scheduler at install time and never read again, so a silently substituted
// one would be invisible for the life of the install.
func TestNameSyncIntervalRejectsUnusableValues(t *testing.T) {
	for _, test := range []struct{ body, want string }{
		{`, "nameSync": {"interval": "30s"}`, "at least 1m0s"},
		{`, "nameSync": {"interval": "0s"}`, "at least 1m0s"},
		{`, "nameSync": {"interval": "-5m"}`, "at least 1m0s"},
		{`, "nameSync": {"interval": "15"}`, "must be a Go duration"},
		{`, "nameSync": {"interval": "quarter hour"}`, "must be a Go duration"},
	} {
		_, err := loadTmuxConfig(t, test.body)
		if err == nil {
			t.Fatalf("%s was accepted", test.body)
		}
		if !strings.Contains(err.Error(), test.want) {
			t.Fatalf("%s reported %v, want a message containing %q", test.body, err, test.want)
		}
	}
}

func TestNameSyncIntervalRoundTripsThroughMarshal(t *testing.T) {
	got, err := loadTmuxConfig(t, `, "nameSync": {"interval": "5m"}`)
	if err != nil {
		t.Fatal(err)
	}
	content, err := Marshal(got, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), `"interval": "5m0s"`) {
		t.Fatalf("Marshal dropped nameSync.interval:\n%s", content)
	}
}
