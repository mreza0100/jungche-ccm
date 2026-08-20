package theme

import (
	"testing"

	"hostops/pfm/internal/naming"
)

func TestLoadEmbeddedPalettes(t *testing.T) {
	defaultPalette := Load("default")
	if defaultPalette.CodexRow != "#e879f9" || defaultPalette.AgentRow != "#fb923c" || defaultPalette.StatsClaude != "#5eead4" || defaultPalette.StatsCPU != "#4ade80" {
		t.Fatalf("default palette = %#v", defaultPalette)
	}
	if tokyo := Load("tokyo-night"); tokyo == defaultPalette {
		t.Fatal("tokyo-night palette unexpectedly equals default")
	}
}

func TestUnknownPaletteFallsBackToDefault(t *testing.T) {
	if got, want := Load("not-a-palette"), Load("default"); got != want {
		t.Fatalf("unknown palette = %#v, want default %#v", got, want)
	}
}

func TestConfiguredEmojiAndLegacyMedalsAreRecognized(t *testing.T) {
	if !naming.ContainsMedalFor("🔖 custom", []string{"custom"}) {
		t.Fatal("configured emoji was not recognized")
	}
	if !naming.ContainsMedalFor("🔖 🥉 legacy", nil) {
		t.Fatal("legacy medal was not recognized")
	}
}
