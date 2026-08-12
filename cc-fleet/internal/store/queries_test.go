package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPlainQueries(t *testing.T) {
	setStoreTestJail(t)
	store := openTestStore(t)
	t.Cleanup(func() { _ = store.Close() })
	ctx := context.Background()

	transcript := Transcript{
		UUID:         "cc-1",
		Path:         "/fixtures/cc-1.jsonl",
		Size:         100,
		MTimeNS:      200,
		ParsedOffset: 90,
		CWD:          "/work/one",
		CustomTitle:  "custom",
		AITitle:      "ai",
		FirstPrompt:  "first",
		LastPrompt:   "last",
		PromptCount:  3,
		IsBG:         true,
	}
	if err := store.UpsertTranscript(ctx, transcript); err != nil {
		t.Fatalf("UpsertTranscript() error = %v", err)
	}
	gotTranscript, found, err := store.Transcript(ctx, transcript.UUID)
	if err != nil {
		t.Fatalf("Transcript() error = %v", err)
	}
	if !found || !reflect.DeepEqual(gotTranscript, transcript) {
		t.Fatalf("Transcript() = %#v, %v, want %#v, true", gotTranscript, found, transcript)
	}
	transcript.LastPrompt = "new last"
	transcript.PromptCount++
	if err := store.UpsertTranscript(ctx, transcript); err != nil {
		t.Fatalf("second UpsertTranscript() error = %v", err)
	}
	transcripts, err := store.Transcripts(ctx)
	if err != nil {
		t.Fatalf("Transcripts() error = %v", err)
	}
	if !reflect.DeepEqual(transcripts, []Transcript{transcript}) {
		t.Fatalf("Transcripts() = %#v, want %#v", transcripts, []Transcript{transcript})
	}

	rollout := Rollout{
		ID:           "cx-1",
		Path:         "/fixtures/cx-1.jsonl",
		Size:         300,
		MTimeNS:      400,
		ParsedOffset: 280,
		CWD:          "/work/two",
		UserThread:   true,
		SessionID:    "session",
		ParentThread: "parent",
		LineageRoot:  "session",
		FirstPrompt:  "hello",
		PromptCount:  5,
	}
	if err := store.UpsertRollout(ctx, rollout); err != nil {
		t.Fatalf("UpsertRollout() error = %v", err)
	}
	gotRollout, found, err := store.Rollout(ctx, rollout.ID)
	if err != nil {
		t.Fatalf("Rollout() error = %v", err)
	}
	if !found || !reflect.DeepEqual(gotRollout, rollout) {
		t.Fatalf("Rollout() = %#v, %v, want %#v, true", gotRollout, found, rollout)
	}
	rollouts, err := store.Rollouts(ctx)
	if err != nil {
		t.Fatalf("Rollouts() error = %v", err)
	}
	if !reflect.DeepEqual(rollouts, []Rollout{rollout}) {
		t.Fatalf("Rollouts() = %#v, want %#v", rollouts, []Rollout{rollout})
	}

	if err := store.ReplaceCxNames(ctx, []CxName{
		{ID: "cx-1", ThreadName: "one"},
		{ID: "cx-2", ThreadName: "two"},
	}); err != nil {
		t.Fatalf("ReplaceCxNames() error = %v", err)
	}
	if err := store.ReplaceCxNames(ctx, []CxName{
		{ID: "cx-2", ThreadName: "renamed"},
	}); err != nil {
		t.Fatalf("second ReplaceCxNames() error = %v", err)
	}
	names, err := store.CxNames(ctx)
	if err != nil {
		t.Fatalf("CxNames() error = %v", err)
	}
	if !reflect.DeepEqual(names, map[string]string{"cx-2": "renamed"}) {
		t.Fatalf("CxNames() = %#v, want cx-2 only", names)
	}

	if err := store.SetMeta(ctx, "cx_index_size", "10"); err != nil {
		t.Fatalf("SetMeta() error = %v", err)
	}
	value, found, err := store.Meta(ctx, "cx_index_size")
	if err != nil {
		t.Fatalf("Meta() error = %v", err)
	}
	if !found || value != "10" {
		t.Fatalf("Meta() = %q, %v, want 10, true", value, found)
	}
	if err := store.DeleteMeta(ctx, "cx_index_size"); err != nil {
		t.Fatalf("DeleteMeta() error = %v", err)
	}
	if _, found, err := store.Meta(ctx, "cx_index_size"); err != nil || found {
		t.Fatalf("Meta() after delete found = %v, error = %v; want false, nil", found, err)
	}

	// A caller may still hand over a baseline; the shared schema has no place
	// to put one, so it reads back nil and the hide is permanent regardless.
	baseline := int64(7)
	hidden := Hidden{
		ID:              "cc-1",
		Engine:          "cc",
		HiddenAt:        1234,
		BaselinePrompts: &baseline,
	}
	if err := store.Hide(ctx, hidden); err != nil {
		t.Fatalf("Hide() error = %v", err)
	}
	gotHidden, found, err := store.Hidden(ctx, hidden.ID)
	if err != nil {
		t.Fatalf("Hidden() error = %v", err)
	}
	wantHidden := Hidden{ID: "cc-1", Engine: "cc", HiddenAt: 1234}
	if !found || !reflect.DeepEqual(gotHidden, wantHidden) {
		t.Fatalf("Hidden() = %#v, %v, want %#v, true", gotHidden, found, wantHidden)
	}
	if err := store.Hide(ctx, Hidden{ID: "cx-1", Engine: "cx", HiddenAt: 2345}); err != nil {
		t.Fatalf("Hide() with lazy baseline error = %v", err)
	}
	hiddenChats, err := store.HiddenChats(ctx)
	if err != nil {
		t.Fatalf("HiddenChats() error = %v", err)
	}
	if len(hiddenChats) != 2 || hiddenChats[1].BaselinePrompts != nil {
		t.Fatalf("HiddenChats() = %#v, want two rows and a nil lazy baseline", hiddenChats)
	}
	if err := store.Unhide(ctx, hidden.ID); err != nil {
		t.Fatalf("Unhide() error = %v", err)
	}
	if _, found, err := store.Hidden(ctx, hidden.ID); err != nil || found {
		t.Fatalf("Hidden() after unhide found = %v, error = %v; want false, nil", found, err)
	}

	if err := store.DeleteTranscript(ctx, transcript.UUID); err != nil {
		t.Fatalf("DeleteTranscript() error = %v", err)
	}
	if _, found, err := store.Transcript(ctx, transcript.UUID); err != nil || found {
		t.Fatalf("Transcript() after delete found = %v, error = %v; want false, nil", found, err)
	}
	if err := store.DeleteRollout(ctx, rollout.ID); err != nil {
		t.Fatalf("DeleteRollout() error = %v", err)
	}
	if _, found, err := store.Rollout(ctx, rollout.ID); err != nil || found {
		t.Fatalf("Rollout() after delete found = %v, error = %v; want false, nil", found, err)
	}
}

func TestRolloutUpsertRepairsCollapsedIdentityPathConflict(t *testing.T) {
	setStoreTestJail(t)
	database := openTestStore(t)
	t.Cleanup(func() { _ = database.Close() })
	ctx := context.Background()
	path := "/codex/rollout-fork-id.jsonl"
	if err := database.UpsertRollout(ctx, Rollout{
		ID:   "parent-id",
		Path: path,
	}); err != nil {
		t.Fatal(err)
	}
	if err := database.UpsertRollout(ctx, Rollout{
		ID:   "fork-id",
		Path: path,
	}); err != nil {
		t.Fatal(err)
	}
	if _, found, err := database.Rollout(ctx, "parent-id"); err != nil || found {
		t.Fatalf("displaced parent found=%t err=%v", found, err)
	}
	repaired, found, err := database.Rollout(ctx, "fork-id")
	if err != nil || !found || repaired.Path != path {
		t.Fatalf("repaired rollout=%#v found=%t err=%v", repaired, found, err)
	}
}

func TestDefaultCandidatesAreCappedAndCountsStayHonest(t *testing.T) {
	setStoreTestJail(t)
	database := openTestStore(t)
	defer database.Close()
	ctx := context.Background()
	for index := 0; index < 35; index++ {
		if err := database.UpsertTranscript(ctx, Transcript{
			UUID:        fmt.Sprintf("cc-%02d", index),
			Path:        fmt.Sprintf("/cc/%02d.jsonl", index),
			Size:        100,
			MTimeNS:     int64(index),
			CWD:         "/work/cc",
			PromptCount: 1,
		}); err != nil {
			t.Fatal(err)
		}
	}
	for index := 0; index < 5; index++ {
		if err := database.UpsertTranscript(ctx, Transcript{
			UUID:        fmt.Sprintf("bg-%02d", index),
			Path:        fmt.Sprintf("/cc/bg-%02d.jsonl", index),
			Size:        100,
			CWD:         "/work/bg",
			PromptCount: 1,
			IsBG:        true,
		}); err != nil {
			t.Fatal(err)
		}
	}
	for index := 0; index < 2; index++ {
		if err := database.UpsertTranscript(ctx, Transcript{
			UUID: fmt.Sprintf("empty-%02d", index),
			Path: fmt.Sprintf("/cc/empty-%02d.jsonl", index),
			CWD:  "/work/empty",
		}); err != nil {
			t.Fatal(err)
		}
	}
	current := int64(2)
	if err := database.UpsertTranscript(ctx, Transcript{
		UUID: "cc-hidden", Path: "/cc/hidden.jsonl", Size: 100,
		MTimeNS: 100, CWD: "/work/cc", PromptCount: 2,
	}); err != nil {
		t.Fatal(err)
	}
	if err := database.Hide(ctx, Hidden{
		ID: "cc-hidden", Engine: "cc", BaselinePrompts: &current,
	}); err != nil {
		t.Fatal(err)
	}
	// cc-grown carries a stale baseline its prompt count already passed: the
	// retired ratchet must not bring it back.
	stale := int64(1)
	if err := database.UpsertTranscript(ctx, Transcript{
		UUID: "cc-grown", Path: "/cc/grown.jsonl", Size: 100,
		MTimeNS: 101, CWD: "/work/cc", PromptCount: 2,
	}); err != nil {
		t.Fatal(err)
	}
	if err := database.Hide(ctx, Hidden{
		ID: "cc-grown", Engine: "cc", BaselinePrompts: &stale,
	}); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 20; index++ {
		if err := database.UpsertRollout(ctx, Rollout{
			ID:          fmt.Sprintf("cx-%02d", index),
			Path:        fmt.Sprintf("/cx/%02d.jsonl", index),
			Size:        100,
			MTimeNS:     int64(index),
			CWD:         "/work/cx",
			UserThread:  true,
			PromptCount: 1,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := database.UpsertRollout(ctx, Rollout{
		ID: "cx-hidden", Path: "/cx/hidden.jsonl", Size: 100,
		MTimeNS: 100, CWD: "/work/cx", UserThread: true, PromptCount: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := database.Hide(ctx, Hidden{
		ID: "cx-hidden", Engine: "cx",
	}); err != nil {
		t.Fatal(err)
	}

	transcripts, rollouts, counts, err := database.DefaultCandidates(ctx, 30, 15)
	if err != nil {
		t.Fatal(err)
	}
	if len(transcripts) != 30 || len(rollouts) != 15 {
		t.Fatalf("candidate lengths=%d/%d, want 30/15", len(transcripts), len(rollouts))
	}
	if counts.Hidden != 3 || counts.Suppressed != 17 {
		t.Fatalf("cached counts=%+v, want hidden=3 suppressed=17", counts)
	}
	for _, transcript := range transcripts {
		if transcript.IsBG || transcript.Size <= 0 || transcript.PromptCount <= 0 ||
			transcript.UUID == "cc-hidden" || transcript.UUID == "cc-grown" {
			t.Fatalf("ineligible cached transcript: %#v", transcript)
		}
	}
}

// TestDefaultRolloutsHiddenOnAnyLineageMember is codexLineageHidden's own
// fixture, and it guards cx-hide.sh's actual write shape (cx-hide.sh:147): a
// hide keyed on a MEMBER id — never the lineage root — must still pull the
// whole conversation out of the cached first frame. "child-old" is neither
// the root nor the newest file, so a fix that only special-cases those two
// would still miss it.
func TestDefaultRolloutsHiddenOnAnyLineageMember(t *testing.T) {
	setStoreTestJail(t)
	database := openTestStore(t)
	defer database.Close()
	ctx := context.Background()

	for _, rollout := range lineageFixtureRollouts() {
		if err := database.UpsertRollout(ctx, rollout); err != nil {
			t.Fatal(err)
		}
	}
	_, rollouts, counts, err := database.DefaultCandidates(ctx, 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rollouts) != 1 || rollouts[0].LineageRoot != "root" {
		t.Fatalf("unhidden lineage = %#v, want exactly the root-lineage row", rollouts)
	}
	if counts.Hidden != 0 {
		t.Fatalf("counts before hide = %+v, want zero hidden", counts)
	}

	if err := database.Hide(ctx, Hidden{ID: "child-old", Engine: "cx"}); err != nil {
		t.Fatal(err)
	}
	_, rollouts, counts, err = database.DefaultCandidates(ctx, 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rollouts) != 0 {
		t.Fatalf(
			"a hide on a non-root, non-newest member left the lineage listed: %#v",
			rollouts,
		)
	}
	if counts.Hidden != 1 {
		t.Fatalf("counts after member hide = %+v, want hidden=1", counts)
	}

	if err := database.Unhide(ctx, "child-old"); err != nil {
		t.Fatal(err)
	}
	_, rollouts, _, err = database.DefaultCandidates(ctx, 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rollouts) != 1 || rollouts[0].LineageRoot != "root" {
		t.Fatalf("unhide did not bring the lineage back: %#v", rollouts)
	}
}

// The v3->v4 migration adds cx_names.source and cx_names.renamed_at without
// losing a pre-migration row, and running it — through the ordinary
// open-time migration gate — is a no-op whether the database starts at v3
// or is already at v4: PRAGMA user_version guards migration_v4.sql from
// ever running twice, which an ALTER TABLE ADD COLUMN cannot survive.
func TestV4MigrationAddsCxNameProvenanceIdempotently(t *testing.T) {
	dbPath := setStoreTestJail(t)
	ctx := context.Background()
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		t.Fatal(err)
	}
	database, err := sql.Open(driverName, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{schemaV1, schemaV2, schemaV3} {
		if _, err := database.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := database.ExecContext(ctx, "PRAGMA user_version=3"); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(
		ctx,
		"INSERT INTO cx_names (id, thread_name) VALUES (?, ?)",
		"pre-v4",
		"OLD NAME",
	); err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	migrated := openTestStore(t)
	// The whole chain runs from v3, not just migration_v4.sql in isolation.
	assertSchemaVersion(t, migrated, SchemaVersion)
	records, err := migrated.CxNameRecords(ctx)
	if err != nil {
		t.Fatal(err)
	}
	record, found := records["pre-v4"]
	if !found ||
		record.ThreadName != "OLD NAME" ||
		record.Source != CxNameSourceSessionIndex ||
		record.RenamedAt != 0 {
		t.Fatalf(
			"pre-v4 row after migration = %#v, found = %t, want the column defaults",
			record,
			found,
		)
	}
	if err := migrated.Close(); err != nil {
		t.Fatal(err)
	}

	// Reopening an already-v4 database is the other half of "idempotent":
	// the migration loop must not attempt migration_v4.sql again.
	reopened := openTestStore(t)
	t.Cleanup(func() { _ = reopened.Close() })
	assertSchemaVersion(t, reopened, SchemaVersion)
	records, err = reopened.CxNameRecords(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if record := records["pre-v4"]; record.ThreadName != "OLD NAME" {
		t.Fatalf("pre-v4 row after reopen = %#v", record)
	}
}
