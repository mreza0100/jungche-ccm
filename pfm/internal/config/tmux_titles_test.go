package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadTmuxConfig(t *testing.T, body string) (Config, error) {
	t.Helper()
	home := filepath.Join(t.TempDir(), "home")
	path := filepath.Join(t.TempDir(), "config.json")
	content := `{"version": 2` + body + `}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return Load(path, home, nil)
}

// A host that owns its own outer-pty title must not have it seized by an
// upgrade, and a host that never heard of the key must not lose the title it
// already has. Both are the same assertion: the default is today's behaviour.
func TestTmuxTitlesDefaultsToPfmOwningTheTitle(t *testing.T) {
	got, err := loadTmuxConfig(t, "")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Tmux.Titles.Enabled {
		t.Fatalf("Tmux.Titles = %+v, want enabled by default", got.Tmux.Titles)
	}
	if source := got.Source("tmux.titles.enabled"); source != SourceDefault {
		t.Fatalf("unset tmux.titles.enabled reports source %q, want default", source)
	}
	options := got.Tmux.Titles.Options()
	want := [][]string{
		{"set-option", "-g", "set-titles", "on"},
		{"set-option", "-g", "set-titles-string", TmuxTitlesString},
	}
	if len(options) != len(want) {
		t.Fatalf("Options() = %v, want %v", options, want)
	}
	for index := range want {
		if strings.Join(options[index], " ") != strings.Join(want[index], " ") {
			t.Fatalf("Options()[%d] = %v, want %v", index, options[index], want[index])
		}
	}
}

func TestTmuxTitlesDisabledSetsNeitherOption(t *testing.T) {
	got, err := loadTmuxConfig(t, `, "tmux": {"titles": {"enabled": false}}`)
	if err != nil {
		t.Fatal(err)
	}
	if got.Tmux.Titles.Enabled {
		t.Fatalf("Tmux.Titles = %+v, want disabled", got.Tmux.Titles)
	}
	if source := got.Source("tmux.titles.enabled"); source != SourceFile {
		t.Fatalf("tmux.titles.enabled reports source %q, want file", source)
	}
	if options := got.Tmux.Titles.Options(); len(options) != 0 {
		t.Fatalf("a disabled policy planned %v, want no tmux options at all", options)
	}
}

// A tmux client built without a machine config — a test fixture, an internal
// caller — keeps pfm's default rather than silently handing the terminal title
// to the host.
func TestTmuxTitlesOrDefaultTreatsNilAsPfmOwned(t *testing.T) {
	if !TmuxTitlesOrDefault(nil).Enabled {
		t.Fatal("a nil tmux.titles policy resolved to host-owned, want pfm-owned")
	}
	disabled := TmuxTitles{Enabled: false}
	if TmuxTitlesOrDefault(&disabled).Enabled {
		t.Fatal("an explicit disabled policy resolved to pfm-owned")
	}
}

func TestTmuxTitlesRoundTripsThroughMarshal(t *testing.T) {
	got, err := loadTmuxConfig(t, `, "tmux": {"titles": {"enabled": false}}`)
	if err != nil {
		t.Fatal(err)
	}
	content, err := Marshal(got, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), `"titles"`) || !strings.Contains(string(content), `"enabled": false`) {
		t.Fatalf("Marshal dropped tmux.titles.enabled:\n%s", content)
	}
}

func TestTmuxRejectsUnknownKeys(t *testing.T) {
	if _, err := loadTmuxConfig(t, `, "tmux": {"title": {"enabled": false}}`); err == nil {
		t.Fatal("a misspelled tmux key was accepted")
	}
}
