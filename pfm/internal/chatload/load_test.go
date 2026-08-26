package chatload

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadReturnsEveryTextFileAndNoBuildTree(t *testing.T) {
	root := t.TempDir()
	for path, content := range map[string]string{
		"a.txt":                    "a\nb\n",
		"nested/b.txt":             "c\n",
		"node_modules/ignored.txt": "ignored\n",
	} {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	result, err := Load([]string{root}, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 2 || result.TotalLines != 3 {
		t.Fatalf("load result = %+v", result)
	}
	for _, file := range result.Files {
		if strings.Contains(file.Path, "node_modules") || file.Text == "" {
			t.Fatalf("unexpected loaded file = %+v", file)
		}
	}
}

func TestLoadFailsWholeSetAtByteCeiling(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.txt")
	if err := os.WriteFile(path, []byte("complete body\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := Load([]string{path}, 4)
	if err == nil || len(result.Files) != 0 || !strings.Contains(err.Error(), "exceeds max_bytes") {
		t.Fatalf("Load() result=%+v err=%v", result, err)
	}
}
