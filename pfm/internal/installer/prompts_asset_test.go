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
	embedded, err := readAsset("prompts/professor-prompt.md")
	if err != nil {
		t.Fatalf("read embedded professor prompt: %v", err)
	}
	template, err := os.ReadFile(filepath.Join("..", "..", "..", "templates", "prompts", "professor.md"))
	if err != nil {
		t.Fatalf("read shipped template: %v", err)
	}
	if !bytes.Equal(embedded, template) {
		t.Fatal("embedded prompts/professor-prompt.md and templates/prompts/professor.md differ — edit one source and copy byte-exact")
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
