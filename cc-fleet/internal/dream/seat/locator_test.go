package seat

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hostops/cc-fleet/internal/paths"
)

func TestFilesystemLocatorUsesSnapshotCWDAndExactThreadName(t *testing.T) {
	root := t.TempDir()
	locator := FilesystemRolloutLocator{CodexRoot: root}
	writeRollout(t, root, "old", "old-id", "/stage", "old answer")
	snapshot, err := locator.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}

	wantedPath := writeRollout(t, root, "wanted", "seat-id", "/organ/stage", "final answer")
	writeRollout(t, root, "wrong-cwd", "other-id", "/somewhere/else", "other answer")
	writeSessionIndex(t, root,
		`{"id":"seat-id","thread_name":"dream-distill"}`,
		`{"id":"other-id","thread_name":"dream-distill"}`,
	)
	chat, found, err := locator.Locate(context.Background(), snapshot, RolloutMatch{
		Name:    "dream-distill",
		CWD:     "/organ/stage",
		Socket:  "dream-distill-socket",
		Session: "dream-distill-socket",
	})
	if err != nil {
		t.Fatalf("Locate() error = %v", err)
	}
	if !found {
		t.Fatal("Locate() did not find the independently matched rollout")
	}
	if chat.Path != wantedPath || chat.ID != "seat-id" || chat.Name != "dream-distill" ||
		chat.CWD != "/organ/stage" || chat.Socket != "dream-distill-socket" {
		t.Fatalf("chat = %#v", chat)
	}
}

func TestFilesystemLocatorFailsOnAmbiguousNewRollouts(t *testing.T) {
	root := t.TempDir()
	locator := FilesystemRolloutLocator{CodexRoot: root}
	snapshot, err := locator.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	writeRollout(t, root, "one", "one-id", "/organ/stage", "one")
	writeRollout(t, root, "two", "two-id", "/organ/stage", "two")
	writeSessionIndex(t, root,
		`{"id":"one-id","thread_name":"same-name"}`,
		`{"id":"two-id","thread_name":"same-name"}`,
	)
	_, found, err := locator.Locate(context.Background(), snapshot, RolloutMatch{
		Name: "same-name", CWD: "/organ/stage", Socket: "socket", Session: "socket",
	})
	if err == nil || !strings.Contains(err.Error(), "ambiguous rollout") {
		t.Fatalf("Locate() error = %v, want ambiguity", err)
	}
	if found {
		t.Fatal("ambiguous rollout was returned as found")
	}
}

func TestFilesystemLocatorWaitsForTheExactName(t *testing.T) {
	root := t.TempDir()
	locator := FilesystemRolloutLocator{CodexRoot: root}
	snapshot, err := locator.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	writeRollout(t, root, "new", "seat-id", "/organ/stage", "answer")
	writeSessionIndex(t, root, `{"id":"seat-id","thread_name":"somebody-else"}`)
	_, found, err := locator.Locate(context.Background(), snapshot, RolloutMatch{
		Name: "wanted", CWD: "/organ/stage", Socket: "socket", Session: "socket",
	})
	if err != nil {
		t.Fatalf("Locate() error = %v", err)
	}
	if found {
		t.Fatal("CWD-only match bypassed the exact thread-name check")
	}
}

func TestFilesystemLocatorTreatsOnlyUnterminatedNewRecordsAsPending(t *testing.T) {
	root := t.TempDir()
	locator := FilesystemRolloutLocator{CodexRoot: root}
	snapshot, err := locator.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	path := filepath.Join(root, "sessions", "2026", "08", "13", "rollout-pending.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"type":"session_meta"`), 0o600); err != nil {
		t.Fatal(err)
	}
	writeSessionIndex(t, root, `{"id":"pending-id","thread_name":"seat"}`)
	if _, found, err := locator.Locate(context.Background(), snapshot, RolloutMatch{
		Name: "seat", CWD: "/stage", Socket: "socket", Session: "socket",
	}); err != nil || found {
		t.Fatalf("partial first record = found %t, error %v; want pending", found, err)
	}
	if err := os.WriteFile(path, []byte(sessionMeta("pending-id", "/stage")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	chat, found, err := locator.Locate(context.Background(), snapshot, RolloutMatch{
		Name: "seat", CWD: "/stage", Socket: "socket", Session: "socket",
	})
	if err != nil || !found || chat.ID != "pending-id" {
		t.Fatalf("completed first record = %#v, found %t, error %v", chat, found, err)
	}
}

func TestDefaultLocatorNeverOpensFleetDatabase(t *testing.T) {
	root := t.TempDir()
	database := filepath.Join(root, "must-not-exist", "fleet.db")
	t.Setenv(paths.EnvCodexRoot, filepath.Join(root, "codex"))
	t.Setenv(paths.EnvDB, database)
	locator, err := NewFilesystemRolloutLocator()
	if err != nil {
		t.Fatalf("NewFilesystemRolloutLocator() error = %v", err)
	}
	if _, err := locator.Snapshot(context.Background()); err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if _, err := os.Stat(database); !os.IsNotExist(err) {
		t.Fatalf("fleet database was touched: stat error = %v", err)
	}
}

func TestFilesystemLocatorFailsClosedOnMalformedNewMetadataAndIndex(t *testing.T) {
	for _, test := range []struct {
		name       string
		rollout    string
		indexLines []string
	}{
		{name: "metadata", rollout: `{"type":"not-session-meta"}`, indexLines: []string{`{"id":"id","thread_name":"seat"}`}},
		{name: "index", rollout: sessionMeta("id", "/stage"), indexLines: []string{`not-json`}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			locator := FilesystemRolloutLocator{CodexRoot: root}
			snapshot, err := locator.Snapshot(context.Background())
			if err != nil {
				t.Fatalf("Snapshot() error = %v", err)
			}
			path := filepath.Join(root, "sessions", "2026", "08", "13", "rollout-bad.jsonl")
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(test.rollout+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			writeSessionIndex(t, root, test.indexLines...)
			if _, _, err := locator.Locate(context.Background(), snapshot, RolloutMatch{
				Name: "seat", CWD: "/stage", Socket: "socket", Session: "socket",
			}); err == nil {
				t.Fatal("malformed rollout discovery rendered as absence")
			}
		})
	}
}

func writeRollout(t *testing.T, root, suffix, id, cwd, answer string) string {
	t.Helper()
	path := filepath.Join(root, "sessions", "2026", "08", "13", "rollout-"+suffix+".jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	content := sessionMeta(id, cwd) + "\n" + codexAssistant(answer) + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func sessionMeta(id, cwd string) string {
	return fmt.Sprintf(`{"type":"session_meta","payload":{"id":%q,"cwd":%q}}`, id, cwd)
}

func codexAssistant(text string) string {
	return fmt.Sprintf(
		`{"type":"event_msg","payload":{"type":"agent_message","message":%q,"model":%q}}`,
		text,
		SeatModel,
	)
}

func codexUser(text string) string {
	return fmt.Sprintf(
		`{"type":"event_msg","payload":{"type":"user_message","message":%q,"model":%q}}`,
		text,
		SeatModel,
	)
}

func writeSessionIndex(t *testing.T, root string, lines ...string) {
	t.Helper()
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	content := strings.Join(lines, "\n")
	if content != "" {
		content += "\n"
	}
	if err := os.WriteFile(filepath.Join(root, "session_index.jsonl"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
