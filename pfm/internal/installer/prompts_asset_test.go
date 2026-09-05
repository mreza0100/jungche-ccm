package installer

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// The shipped template and the embedded installer asset are the same file by
// contract; this is the no-drift gate between them.
func TestProfessorPromptAssetMatchesShippedTemplate(t *testing.T) {

	for _, pair := range [][2]string{{"professor-prompt.md", "professor.md"}, {"codex-appendix.md", "codex-appendix.md"}, {"harness-original.model", "harness-original.model"}} {
		embedded, err := readAsset("prompts/" + pair[0])
		if err != nil {
			t.Fatal(err)
		}
		template, err := os.ReadFile(filepath.Join("..", "..", "..", "templates", "prompts", pair[1]))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(embedded, template) {
			t.Fatalf("embedded %s differs from template %s", pair[0], pair[1])
		}
	}
}

func TestHarnessBaselineAssetPairIsCoherent(t *testing.T) {
	pin, err := readAsset("prompts/harness-original.sha256")
	if err != nil {
		t.Fatalf("read baseline pin: %v", err)
	}
	fields := bytes.Fields(pin)
	if len(fields) != 2 {
		t.Fatalf("baseline pin = %q, want '<sha256>  <filename>'", pin)
	}
	if _, err := readAsset("prompts/" + string(fields[1])); err != nil {
		t.Fatalf("baseline pin names %q but that asset is not embedded: %v", fields[1], err)
	}
}
