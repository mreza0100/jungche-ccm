package professor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBaselineRoundTrip(t *testing.T) {
	root := t.TempDir()
	want := Baseline{
		Version:   BaselineVersion,
		Blueprint: BlueprintPin{Version: "0.65.0", SHA: "abc1234"},
		Files: map[string]FilePin{
			".claude/commands/dev.md": {
				Template:     "project/commands/dev.md",
				TemplateHash: "sha256:fixture",
				PinnedSHA:    "abc1234",
				PinnedAt:     "2026-09-01",
			},
		},
	}
	if err := Save(root, want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got, err := Load(root)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Version != want.Version || got.Blueprint != want.Blueprint || len(got.Files) != 1 || got.Files[".claude/commands/dev.md"] != want.Files[".claude/commands/dev.md"] {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}
}

func TestBaselineMalformedAndUnsupportedAreNamed(t *testing.T) {
	for _, test := range []struct {
		name    string
		content string
		want    string
	}{
		{name: "malformed", content: "{", want: "BASELINE-MALFORMED"},
		{name: "unsupported", content: `{"version":2,"blueprint":{},"files":{}}`, want: "BASELINE-VERSION 2: unsupported"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, ".professor", "baseline.json")
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(test.content), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := Load(root)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Load() error = %v, want %q", err, test.want)
			}
		})
	}
}
