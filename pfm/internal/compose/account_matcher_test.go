package compose

import (
	"os"
	"path/filepath"
	"testing"

	"hostops/pfm/internal/store"
)

func TestAccountAttributionUsesThirdAliasAndLongestNestedRoot(t *testing.T) {
	root := t.TempDir()
	physical := filepath.Join(root, "physical")
	projects := filepath.Join(physical, "projects")
	nested := filepath.Join(projects, "nested")
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	aliasOne := filepath.Join(root, "alias-one")
	aliasTwo := filepath.Join(root, "alias-two")
	aliasThree := filepath.Join(root, "alias-three")
	for _, alias := range []string{aliasOne, aliasTwo, aliasThree} {
		if err := os.Symlink(physical, alias); err != nil {
			t.Fatal(err)
		}
	}
	transcriptPath := filepath.Join(aliasThree, "projects", "nested", "claude.jsonl")
	rolloutPath := filepath.Join(aliasThree, "projects", "nested", "rollout-codex.jsonl")
	output := Compose(Input{
		Transcripts: []store.Transcript{{
			UUID: "claude-alias", Path: transcriptPath, CWD: "/work/claude",
			FirstPrompt: "Claude alias", PromptCount: 1, Size: 10, MTimeNS: 1,
		}},
		Rollouts: []store.Rollout{{
			ID: "codex-alias", Path: rolloutPath, CWD: "/work/codex",
			FirstPrompt: "Codex alias", PromptCount: 1, Size: 10, MTimeNS: 2,
			UserThread: true,
		}},
		AccountRoots: []AccountRoot{
			{Account: 11, Path: filepath.Join(aliasOne, "projects")},
			{Account: 12, Path: filepath.Join(aliasOne, "projects", "nested")},
		},
		CodexRoots: []AccountRoot{
			{Account: 21, Path: filepath.Join(aliasOne, "projects")},
			{Account: 22, Path: filepath.Join(aliasOne, "projects", "nested")},
		},
		Options: Options{View: AllView},
	})
	claude, found := rowByID(output.Rows, "claude-alias")
	if !found || claude.Account != 12 {
		t.Fatalf("third-alias Claude row = %#v, want nested account 12", claude)
	}
	codex, found := rowByID(output.Rows, "codex-alias")
	if !found || codex.Account != 22 {
		t.Fatalf("third-alias Codex row = %#v, want nested account 22", codex)
	}
}
