package dream

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateAnchorsIsTextualAndIdempotent(t *testing.T) {
	repository := hookRepository(t)
	organRoot := filepath.Join(repository, ".professor", "stm")
	legacyRow := "- `anchor.txt:2-3` — `git log -1`: `0123456789ab` (2026-08-01); blob `abcdef012345`"
	legacy := migrationMap(legacyRow, "- `stable.txt` — tree `123456789abc`")
	mapPath := filepath.Join(organRoot, "maps", "subject.md")
	writeHookFile(t, mapPath, legacy)

	result, err := MigrateAnchors(organRoot)
	if err != nil {
		t.Fatal(err)
	}
	if result.Maps != 1 || result.Rewritten != 1 || result.Rows != 1 {
		t.Fatalf("migration result = %+v", result)
	}
	if len(result.Files) != 1 || result.Files[0].MapPath != "maps/subject.md" ||
		result.Files[0].Outcome != MigrationRewritten || result.Files[0].Rows != 1 {
		t.Fatalf("migration file outcomes = %+v", result.Files)
	}
	raw, err := os.ReadFile(mapPath)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(legacy, legacyRow, "- `anchor.txt:2-3` — blob `abcdef012345`", 1)
	if string(raw) != want {
		t.Fatalf("migrated map = %q, want %q", raw, want)
	}
	second, err := MigrateAnchors(organRoot)
	if err != nil || second.Maps != 1 || second.Rewritten != 0 || second.Rows != 0 ||
		len(second.Files) != 1 || second.Files[0].Outcome != MigrationUnchanged {
		t.Fatalf("second migration = %+v, %v", second, err)
	}
}

func TestMigrateAnchorsDoesNotMistakeProseForALegacyAnchor(t *testing.T) {
	repository := hookRepository(t)
	organRoot := filepath.Join(repository, ".professor", "stm")
	firstPath := filepath.Join(organRoot, "maps", "a.md")
	secondPath := filepath.Join(organRoot, "maps", "b.md")
	first := migrationMap(
		"- `anchor.txt` — `git log -1`: `0123456789ab` (2026-08-01); blob `abcdef012345`",
		"- `stable.txt` — tree `123456789abc`",
	)
	writeHookFile(t, firstPath, first)
	second := strings.Replace(
		migrationMap("- `anchor.txt` — blob `abcdef012345`", "- `stable.txt` — tree `123456789abc`"),
		"Trail body.",
		"Trail body mentions git log -1 and is not an anchor.",
		1,
	)
	writeHookFile(t, secondPath, second)

	result, err := MigrateAnchors(organRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 2 || result.Files[0].Outcome != MigrationRewritten ||
		result.Files[1].Outcome != MigrationUnchanged {
		t.Fatalf("migration outcomes = %+v", result.Files)
	}
	raw, err := os.ReadFile(firstPath)
	wantFirst := strings.Replace(
		first,
		"- `anchor.txt` — `git log -1`: `0123456789ab` (2026-08-01); blob `abcdef012345`",
		"- `anchor.txt` — blob `abcdef012345`",
		1,
	)
	if err != nil || string(raw) != wantFirst {
		t.Fatalf("first map migration = %q, %v; want %q", raw, err, wantFirst)
	}
	unchanged, err := os.ReadFile(secondPath)
	if err != nil || string(unchanged) != second {
		t.Fatalf("prose-only map changed: %q, %v", unchanged, err)
	}
}

func TestMigrateAnchorsRejectsUnrecognizedLegacyRowBeforeWriting(t *testing.T) {
	repository := hookRepository(t)
	organRoot := filepath.Join(repository, ".professor", "stm")
	path := filepath.Join(organRoot, "maps", "subject.md")
	// The 11-digit object hash prevents the legacy translator from matching;
	// ParseMap must still reject the retired row rather than silently skip it.
	raw := migrationMap(
		"- `anchor.txt` — `git log -1`: `0123456789ab` (2026-08-01); blob `abcdef01234`",
		"- `stable.txt` — tree `123456789abc`",
	)
	writeHookFile(t, path, raw)
	result, err := MigrateAnchors(organRoot)
	if err == nil || !strings.Contains(err.Error(), "maps/subject.md") ||
		!strings.Contains(err.Error(), "REJECTED") {
		t.Fatalf("MigrateAnchors() error = %v", err)
	}
	if len(result.Files) != 1 || result.Files[0].Outcome != MigrationRejected {
		t.Fatalf("rejected migration outcomes = %+v", result.Files)
	}
	unchanged, readErr := os.ReadFile(path)
	if readErr != nil || string(unchanged) != raw {
		t.Fatalf("unrecognized legacy map changed: %q, %v", unchanged, readErr)
	}
}

func TestMigrateAnchorsRejectsMalformedProducedMapBeforeAnyWrite(t *testing.T) {
	repository := hookRepository(t)
	organRoot := filepath.Join(repository, ".professor", "stm")
	firstPath := filepath.Join(organRoot, "maps", "a.md")
	secondPath := filepath.Join(organRoot, "maps", "b.md")
	first := migrationMap(
		"- `anchor.txt` — `git log -1`: `0123456789ab` (2026-08-01); blob `abcdef012345`",
		"- `stable.txt` — tree `123456789abc`",
	)
	second := migrationMap(
		"- `../secret.txt` — `git log -1`: `0123456789ab` (2026-08-01); blob `abcdef012345`",
		"- `stable.txt` — tree `123456789abc`",
	)
	writeHookFile(t, firstPath, first)
	writeHookFile(t, secondPath, second)

	result, err := MigrateAnchors(organRoot)
	if err == nil || !strings.Contains(err.Error(), "maps/b.md") ||
		!strings.Contains(err.Error(), "unsafe anchor path") {
		t.Fatalf("MigrateAnchors() error = %v", err)
	}
	if result.Rewritten != 0 || result.Rows != 0 || len(result.Files) != 2 ||
		result.Files[0].Outcome != MigrationNotWritten ||
		result.Files[1].Outcome != MigrationRejected {
		t.Fatalf("failed migration result = %+v", result)
	}
	for path, want := range map[string]string{firstPath: first, secondPath: second} {
		raw, readErr := os.ReadFile(path)
		if readErr != nil || string(raw) != want {
			t.Fatalf("map changed before grammar failure %s: %q, %v", path, raw, readErr)
		}
	}
}

func TestMigrateAnchorsUsesCanonicalFilenameGrammarAndReportsSkips(t *testing.T) {
	repository := hookRepository(t)
	organRoot := filepath.Join(repository, ".professor", "stm")
	invalid := filepath.Join(organRoot, "maps", "Bad--Map.md")
	raw := migrationMap(
		"- `anchor.txt` — `git log -1`: `0123456789ab` (2026-08-01); blob `abcdef012345`",
		"- `stable.txt` — tree `123456789abc`",
	)
	writeHookFile(t, invalid, raw)

	result, err := MigrateAnchors(organRoot)
	if err != nil {
		t.Fatal(err)
	}
	if result.Maps != 0 || result.Rewritten != 0 || len(result.Files) != 1 ||
		result.Files[0].MapPath != "maps/Bad--Map.md" ||
		result.Files[0].Outcome != MigrationSkipped ||
		result.Files[0].Reason != "invalid map filename" {
		t.Fatalf("skip result = %+v", result)
	}
	unchanged, readErr := os.ReadFile(invalid)
	if readErr != nil || string(unchanged) != raw {
		t.Fatalf("invalid-name map changed: %q, %v", unchanged, readErr)
	}
}

func TestRenderMigrationResultNamesEveryFileOutcome(t *testing.T) {
	got := RenderMigrationResult(MigrationResult{
		Organ: "/repo/.professor/stm", Maps: 2, Rewritten: 1, Rows: 3,
		Files: []MigrationFileResult{
			{MapPath: "maps/a.md", Outcome: MigrationRewritten, Rows: 3},
			{MapPath: "maps/Bad.md", Outcome: MigrationSkipped, Reason: "invalid map filename"},
			{MapPath: "maps/b.md", Outcome: MigrationUnchanged},
		},
	})
	want := "MIGRATE FILE path=maps/a.md outcome=REWRITTEN rows=3\n" +
		"MIGRATE FILE path=maps/Bad.md outcome=SKIPPED rows=0 reason=\"invalid map filename\"\n" +
		"MIGRATE FILE path=maps/b.md outcome=UNCHANGED rows=0\n" +
		"MIGRATE PASS organ=/repo/.professor/stm maps=2 rewritten=1 rows=3\n"
	if got != want {
		t.Fatalf("migration rendering = %q, want %q", got, want)
	}
}

func TestMigrationRollbackOutcomesDescribeRestoredAndUnrestoredFiles(t *testing.T) {
	result := MigrationResult{
		Rewritten: 2,
		Rows:      3,
		Files: []MigrationFileResult{
			{MapPath: "maps/a.md", Outcome: MigrationRewritten, Rows: 1},
			{MapPath: "maps/b.md", Outcome: MigrationRewritten, Rows: 2},
		},
	}
	replaced := []migrationRewrite{
		{path: "/maps/a.md", before: []byte("a"), translated: 1, outcome: 0},
		{path: "/maps/b.md", before: []byte("b"), translated: 2, outcome: 1},
	}
	var order []string
	rollbackErrors := rollbackMigration(&result, replaced, func(path string, _ []byte) error {
		order = append(order, path)
		if strings.HasSuffix(path, "a.md") {
			return errors.New("restore denied")
		}
		return nil
	})
	if strings.Join(order, ",") != "/maps/b.md,/maps/a.md" {
		t.Fatalf("rollback order = %q", order)
	}
	if len(rollbackErrors) != 1 || result.Rewritten != 1 || result.Rows != 1 ||
		result.Files[0].Outcome != MigrationRewritten ||
		!strings.Contains(result.Files[0].Reason, "rollback failed") ||
		result.Files[1].Outcome != MigrationNotWritten ||
		!strings.Contains(result.Files[1].Reason, "rolled back") {
		t.Fatalf("rollback result=%+v errors=%v", result, rollbackErrors)
	}
}

func migrationMap(anchors ...string) string {
	return "# Subject\n\n" +
		"## Question\n\nQuestion?\n\n" +
		"## Answer\n\nAnswer.\n\n" +
		"## Derivation trail\n\nTrail body.\n\n" +
		"Provenance: 2026-08-01 · sid 01234567\n\n" +
		"## Anchors\n\n" + strings.Join(anchors, "\n") + "\n"
}
