package store

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// THE DRIFT REGRESSION. Three hand re-bridges in one night is what this test
// exists to prevent: the Go binary and the zsh fleet kept separate hide lists,
// so a hide taken in one was invisible to the other until somebody copied it
// across by hand.
//
// Both directions, with NO sync step anywhere in between:
//   - a hide written the way cc-db.sh writes one — a sqlite3 CLI INSERT into
//     ~/.cc/fleet.db plus the carrier append — is in the Go listing on the very
//     next read, through an ALREADY-OPEN store;
//   - a hide taken by the Go binary is in the carrier file the moment the call
//     returns, which is the file the picker reads on its next run.
func TestSharedHidesNeedNoBridgeInEitherDirection(t *testing.T) {
	sqlite3 := lookSQLite3(t)
	setStoreTestJail(t)
	database := openTestStore(t)
	t.Cleanup(func() { _ = database.Close() })
	ctx := context.Background()

	for _, transcript := range []Transcript{
		{UUID: "zsh-hid", Path: "/jail/zsh-hid.jsonl", Size: 10, MTimeNS: 2, PromptCount: 1},
		{UUID: "go-hid", Path: "/jail/go-hid.jsonl", Size: 10, MTimeNS: 1, PromptCount: 1},
	} {
		if err := database.UpsertTranscript(ctx, transcript); err != nil {
			t.Fatal(err)
		}
	}

	// ZSH → GO. cc-db.sh's cmd_hidden_add, transcribed (cc-db.sh:129-138): the
	// upsert, then _leg_add's append. The Go store is open the whole time and is
	// never told anything happened.
	runSQLite3(t, sqlite3, database.SharedPath(), `
INSERT INTO hidden(uuid,hidden_at,at_payload) VALUES('zsh-hid',1700000000,NULL)
ON CONFLICT(uuid) DO UPDATE SET
  hidden_at=1700000000,
  at_payload=COALESCE(excluded.at_payload, hidden.at_payload);`)

	hidden, err := database.HiddenChats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(hidden) != 1 || hidden[0].ID != "zsh-hid" ||
		hidden[0].Engine != ClaudeEngine || hidden[0].HiddenAt != 1700000000 {
		t.Fatalf("HiddenChats() after a CLI hide = %#v, want the zsh hide", hidden)
	}
	transcripts, _, counts, err := database.DefaultCandidates(ctx, 30, 15)
	if err != nil {
		t.Fatal(err)
	}
	for _, transcript := range transcripts {
		if transcript.UUID == "zsh-hid" {
			t.Fatal("the cached first frame still lists a chat the zsh half hid")
		}
	}
	if counts.Hidden != 1 {
		t.Fatalf("cached hidden count = %d, want 1", counts.Hidden)
	}

	// GO → ZSH, the carrier half: a line the picker reads on its next run.
	if err := database.Hide(ctx, Hidden{ID: "go-hid", HiddenAt: 1700000001}); err != nil {
		t.Fatal(err)
	}
	assertCarrierHas(t, database.CarrierPath(), "go-hid")

	// GO → ZSH, the database half: exactly what `cc-db.sh hidden-list` runs
	// (cc-db.sh:107), from a process that has never heard of this Go store.
	listed := runSQLite3(
		t,
		sqlite3,
		database.SharedPath(),
		"SELECT uuid FROM hidden ORDER BY hidden_at DESC;",
	)
	if listed != "go-hid\nzsh-hid\n" {
		t.Fatalf("cc-db.sh hidden-list = %q, want both hides", listed)
	}

	// And an unhide clears both halves, because half a removal is drift too.
	if err := database.Unhide(ctx, "go-hid"); err != nil {
		t.Fatal(err)
	}
	if listed = runSQLite3(
		t,
		sqlite3,
		database.SharedPath(),
		"SELECT uuid FROM hidden ORDER BY uuid;",
	); listed != "zsh-hid\n" {
		t.Fatalf("hidden rows after unhide = %q", listed)
	}
	carrier, err := os.ReadFile(database.CarrierPath())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(carrier), "go-hid") {
		t.Fatalf("carrier still carries an unhidden chat: %q", carrier)
	}
}

// A hide that reached only the carrier file — a run where SQLite was
// unopenable, or the zsh half's own no-database fallback (cc-db.sh:108) — is
// still a hide. The union is the read, so no repair command stands between
// that file and the listing.
func TestCarrierOnlyHideIsHiddenWithoutAnImport(t *testing.T) {
	setStoreTestJail(t)
	database := openTestStore(t)
	t.Cleanup(func() { _ = database.Close() })
	ctx := context.Background()

	if err := database.UpsertTranscript(ctx, Transcript{
		UUID:        "carried",
		Path:        "/jail/carried.jsonl",
		Size:        10,
		PromptCount: 1,
	}); err != nil {
		t.Fatal(err)
	}
	carrier := database.CarrierPath()
	if err := os.MkdirAll(filepath.Dir(carrier), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(carrier, []byte("carried\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, found, err := database.Hidden(ctx, "carried"); err != nil || !found {
		t.Fatalf("Hidden() for a carrier-only hide = %v, %v; want true, nil", found, err)
	}
	transcripts, _, _, err := database.DefaultCandidates(ctx, 30, 15)
	if err != nil {
		t.Fatal(err)
	}
	if len(transcripts) != 0 {
		t.Fatalf("cached frame = %#v, want the carrier-only hide filtered out", transcripts)
	}
}

// The one-time adoption is a UNION, in both directions, and it runs once. It
// must never be able to delete: a startup that could drop a row is a startup
// that could lose a decision.
func TestAdoptingLocalHidesUnionsOnceAndDeletesNothing(t *testing.T) {
	setStoreTestJail(t)

	first := openTestStore(t)
	ctx := context.Background()
	// A hide in the shape the retired local table held: engine column and all,
	// written straight to the private cache as an older binary would have.
	if _, err := first.db.ExecContext(ctx, `
INSERT INTO hidden(id, engine, hidden_at, baseline_prompts)
VALUES ('cache-hide', 'cc', 4242, 9)`); err != nil {
		t.Fatal(err)
	}
	// And a hide the zsh half left in the carrier file with no row anywhere.
	carrier := first.CarrierPath()
	if err := os.MkdirAll(filepath.Dir(carrier), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(carrier, []byte("carrier-hide\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := first.SetMeta(ctx, adoptedHidesMeta, ""); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second := openTestStore(t)
	hidden, err := second.HiddenChats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(hidden) != 2 ||
		hidden[0].ID != "cache-hide" || hidden[0].HiddenAt != 4242 ||
		hidden[1].ID != "carrier-hide" {
		t.Fatalf("adopted hides = %#v", hidden)
	}
	// The carrier file gained the adopted row and kept the one it had.
	assertCarrierHas(t, carrier, "cache-hide")
	assertCarrierHas(t, carrier, "carrier-hide")
	// The retired table is left populated: it is the rollback, not a leak.
	var remaining int
	if err := second.db.QueryRowContext(
		ctx,
		"SELECT count(*) FROM hidden",
	).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 1 {
		t.Fatalf("retired local hidden rows = %d, want the original left in place", remaining)
	}

	// An unhide now, and a reopen must NOT resurrect it: adoption ran once.
	if err := second.Unhide(ctx, "cache-hide"); err != nil {
		t.Fatal(err)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}

	third := openTestStore(t)
	t.Cleanup(func() { _ = third.Close() })
	hidden, err = third.HiddenChats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(hidden) != 1 || hidden[0].ID != "carrier-hide" {
		t.Fatalf("hides after reopen = %#v, want the unhide to have stuck", hidden)
	}
}

func lookSQLite3(t *testing.T) string {
	t.Helper()

	path, err := exec.LookPath("sqlite3")
	if err != nil {
		t.Skip("sqlite3 CLI is absent: the zsh half of the store cannot be exercised")
	}
	return path
}

func runSQLite3(t *testing.T, sqlite3, database, statement string) string {
	t.Helper()

	// The same invocation cc-db.sh's _q uses (cc-db.sh:57): the .timeout DOT
	// COMMAND, never PRAGMA busy_timeout, which would print a row of its own
	// ahead of the real result.
	command := exec.Command(
		sqlite3,
		"-batch",
		"-noheader",
		"-cmd",
		".timeout 5000",
		database,
		statement,
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("sqlite3 %q: %v: %s", statement, err, output)
	}
	return string(output)
}
