package theme

import (
	"reflect"
	"testing"

	pfmengine "hostops/pfm/internal/engine"
	"hostops/pfm/internal/naming"
)

func TestLoadEmbeddedPalettes(t *testing.T) {
	defaultPalette := Load("default")
	if defaultPalette.EngineRow[pfmengine.Codex] != "#e879f9" || defaultPalette.AgentRow != "#fb923c" || defaultPalette.StatsEngine[pfmengine.Claude] != "#5eead4" || defaultPalette.StatsCPU != "#4ade80" {
		t.Fatalf("default palette = %#v", defaultPalette)
	}
	if tokyo := Load("tokyo-night"); reflect.DeepEqual(tokyo, defaultPalette) {
		t.Fatal("tokyo-night palette unexpectedly equals default")
	}
}

func TestEveryEngineHasThemeColours(t *testing.T) {
	for _, paletteName := range []string{"default", "tokyo-night"} {
		palette := Load(paletteName)
		for _, id := range pfmengine.All() {
			if palette.EngineRow[id] == "" {
				t.Errorf("%s engine %s has no row colour", paletteName, id)
			}
			if palette.StatsEngine[id] == "" {
				t.Errorf("%s engine %s has no stats colour", paletteName, id)
			}
		}
	}
}

func TestUnknownPaletteFallsBackToDefault(t *testing.T) {
	if got, want := Load("not-a-palette"), Load("default"); !reflect.DeepEqual(got, want) {
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
