package installer

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// Shipped templates and embedded assets are the same source by contract.
func TestProfessorPromptAssetMatchesShippedTemplate(t *testing.T) {
	for _, pair := range [][2]string{{"professor-prompt.md", "professor.md"}, {"codex-appendix.md", "codex-appendix.md"}} {
		assertPromptAssetPair(t, pair[0], pair[1])
	}
}

func assertPromptAssetPair(t *testing.T, asset, name string) []byte {
	t.Helper()
	embedded, err := readAsset("prompts/" + asset)
	if err != nil {
		t.Fatal(err)
	}
	template, err := os.ReadFile(filepath.Join("..", "..", "..", "templates", "prompts", name))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(embedded, template) {
		t.Fatalf("embedded %s differs from template %s", asset, name)
	}
	return embedded
}

func TestHarnessBaselineAssetPairIsCoherent(t *testing.T) {
	for _, stem := range []string{"harness-original", "harness-opus"} {
		t.Run(stem, func(t *testing.T) {
			pin := assertPromptAssetPair(t, stem+".sha256", stem+".sha256")
			fields := bytes.Fields(pin)
			if len(fields) != 2 {
				t.Fatalf("malformed baseline pin: %q", pin)
			}
			name := string(fields[1])
			prompt := assertPromptAssetPair(t, name, name)
			sum := sha256.Sum256(prompt)
			if hex.EncodeToString(sum[:]) != string(fields[0]) {
				t.Fatal("baseline body does not match pinned hash")
			}
			model := assertPromptAssetPair(t, stem+".model", stem+".model")
			if len(bytes.TrimSpace(model)) == 0 {
				t.Fatal("baseline model provenance missing")
			}
		})
	}
}
