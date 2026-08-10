package check

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCodexCandidateIDsReproducesNewestFileBound(t *testing.T) {
	root := t.TempDir()
	sessions := filepath.Join(root, "sessions", "2026", "01", "01")
	if err := os.MkdirAll(sessions, 0o700); err != nil {
		t.Fatal(err)
	}
	type fixture struct {
		id    string
		age   time.Duration
		bytes string
	}
	fixtures := []fixture{
		{id: "old-user", age: 3 * time.Hour, bytes: `{"thread_source":"user"}`},
		{id: "new-subagent", age: time.Hour, bytes: `{"thread_source":"subagent"}`},
		{id: "new-user", age: 2 * time.Hour, bytes: `{"thread_source":"user"}`},
		{id: "new-schema", age: 30 * time.Minute, bytes: `{"source":"vscode"}`},
	}
	now := time.Now()
	for _, fixture := range fixtures {
		path := filepath.Join(
			sessions,
			"rollout-2026-01-01T00-00-00-"+fixture.id+".jsonl",
		)
		if err := os.WriteFile(path, []byte(fixture.bytes+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		mtime := now.Add(-fixture.age)
		if err := os.Chtimes(path, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	ids, err := CodexCandidateIDs(root, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 3 {
		t.Fatalf("candidate IDs = %#v", ids)
	}
	if _, found := ids["new-subagent"]; !found {
		t.Fatal("subagent file was incorrectly removed before the legacy bound")
	}
	if _, found := ids["new-user"]; !found {
		t.Fatal("new user file missing")
	}
	if _, found := ids["old-user"]; found {
		t.Fatal("old file survived newest-two bound")
	}
	unrecognized, err := CodexLegacyUnrecognizedIDs(root, 3)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := unrecognized["new-schema"]; !found {
		t.Fatal("new interactive source schema was not verified as legacy-unrecognized")
	}
	if _, found := unrecognized["new-user"]; found {
		t.Fatal("legacy-recognized user source was marked unrecognized")
	}
}
