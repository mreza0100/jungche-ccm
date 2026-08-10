package shared

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"hostops/cc-fleet/internal/paths"
)

// The schema this package creates has to BE cc-db.sh's, column for column: the
// bash half opens the same file and its queries name these columns. A Go-shaped
// approximation would work until the first `cc-db.sh hidden-list`.
func TestSchemaMatchesCCDBShColumnForColumn(t *testing.T) {
	state, _ := openTestStore(t)
	ctx := context.Background()

	tables := queryColumn(t, state, `
SELECT name FROM sqlite_master
WHERE type='table' AND name NOT LIKE 'sqlite_%'
ORDER BY name`)
	want := []string{"chat", "children", "hidden", "meta", "swap_event"}
	if !reflect.DeepEqual(tables, want) {
		t.Fatalf("shared tables = %v, want %v", tables, want)
	}

	// cc-db.sh:75-79 — uuid-keyed, no engine, no baseline of its own.
	columns := queryColumn(t, state, "SELECT name FROM pragma_table_info('hidden')")
	if !reflect.DeepEqual(columns, []string{"uuid", "hidden_at", "at_payload"}) {
		t.Fatalf("hidden columns = %v", columns)
	}
	columns = queryColumn(t, state, "SELECT name FROM pragma_table_info('children')")
	if !reflect.DeepEqual(columns, []string{"kind", "key", "val", "created_at"}) {
		t.Fatalf("children columns = %v", columns)
	}
	columns = queryColumn(t, state, "SELECT name FROM pragma_table_info('meta')")
	if !reflect.DeepEqual(columns, []string{"key", "val", "updated_at"}) {
		t.Fatalf("meta columns = %v", columns)
	}

	// cc-db.sh:101 stamps this row on init; a database this package created
	// must look to bash exactly like one `cc-db.sh init` created.
	if version, found, err := state.Meta(ctx, "schema_version"); err != nil ||
		!found || version != "1" {
		t.Fatalf("schema_version = %q, %v, %v", version, found, err)
	}

	var journalMode string
	if err := state.db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatal(err)
	}
	if journalMode != "wal" {
		t.Fatalf("journal_mode = %q, want wal — cc-db.sh:73", journalMode)
	}
	var busyTimeout int
	if err := state.db.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		t.Fatal(err)
	}
	if busyTimeout == 0 {
		t.Fatal("busy_timeout = 0: a concurrent sqlite3 CLI writer would fail instead of queue")
	}
}

// _leg_add appends only when the file has no such line, and _leg_del rewrites
// through a scratch file (cc-db.sh:116-128). Both take the lock at
// <carrier>.lock, and neither leaves the scratch file behind.
func TestCarrierWriteThroughFollowsLegAddAndLegDel(t *testing.T) {
	state, values := openTestStore(t)
	ctx := context.Background()

	for attempt := 0; attempt < 3; attempt++ {
		if err := state.Hide(ctx, "alpha", int64(100+attempt)); err != nil {
			t.Fatalf("Hide() error = %v", err)
		}
	}
	if err := state.Hide(ctx, "beta", 200); err != nil {
		t.Fatalf("Hide() error = %v", err)
	}
	assertCarrier(t, values.HiddenCarrier, "alpha\nbeta\n")

	if err := state.Unhide(ctx, "alpha"); err != nil {
		t.Fatalf("Unhide() error = %v", err)
	}
	assertCarrier(t, values.HiddenCarrier, "beta\n")

	// Unhiding something the file never held rewrites nothing and fails
	// nothing: cc-db.sh's grep -vxF is a no-op for an absent line.
	if err := state.Unhide(ctx, "never-hidden"); err != nil {
		t.Fatalf("Unhide() of an absent id error = %v", err)
	}
	assertCarrier(t, values.HiddenCarrier, "beta\n")

	if _, err := os.Stat(values.HiddenCarrier + carrierLockSuffix); err != nil {
		t.Fatalf("no lock file at %s: %v", values.HiddenCarrier+carrierLockSuffix, err)
	}
	entries, err := os.ReadDir(filepath.Dir(values.HiddenCarrier))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), carrierTempInfix) {
			t.Fatalf("carrier rewrite left %s behind", entry.Name())
		}
	}
}

// The effective hidden set is the union, and it never prunes: an id the file
// carries but the database has not seen is hidden, and reading it must not be
// what deletes it.
func TestHiddenAtUnionsDatabaseAndCarrier(t *testing.T) {
	state, values := openTestStore(t)
	ctx := context.Background()

	if err := state.Hide(ctx, "in-both", 111); err != nil {
		t.Fatal(err)
	}
	appendLine(t, values.HiddenCarrier, "file-only")
	if _, err := state.db.ExecContext(
		ctx,
		"INSERT INTO hidden(uuid,hidden_at,at_payload) VALUES('db-only',222,NULL)",
	); err != nil {
		t.Fatal(err)
	}

	hidden, err := state.HiddenAt(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int64{"in-both": 111, "file-only": 0, "db-only": 222}
	if !reflect.DeepEqual(hidden, want) {
		t.Fatalf("HiddenAt() = %v, want %v", hidden, want)
	}

	// The read changed nothing.
	if hidden, err = state.HiddenAt(ctx); err != nil ||
		!reflect.DeepEqual(hidden, want) {
		t.Fatalf("second HiddenAt() = %v, %v", hidden, err)
	}
	assertCarrier(t, values.HiddenCarrier, "in-both\nfile-only\n")
}

// A database that cannot be opened costs the fleet its durability, never its
// hides: the carrier file alone still answers (cc-db.sh:17-18).
func TestDegradedStoreKeepsHidingThroughTheCarrier(t *testing.T) {
	root := t.TempDir()
	blocker := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	values := paths.Values{
		SharedDB:      filepath.Join(blocker, "fleet.db"),
		HiddenCarrier: filepath.Join(root, "home", ".claude", ".cc-ls-hidden"),
		Home:          filepath.Join(root, "home"),
	}
	ctx := context.Background()
	state := Open(ctx, values)
	t.Cleanup(func() { _ = state.Close() })

	if state.Degraded() == nil {
		t.Fatal("Degraded() = nil, want the reason the database is unusable")
	}
	if err := state.Hide(ctx, "still-hidden", 7); err != nil {
		t.Fatalf("degraded Hide() error = %v", err)
	}
	hidden, err := state.HiddenAt(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := hidden["still-hidden"]; !found {
		t.Fatalf("degraded HiddenAt() = %v, want the hide the carrier holds", hidden)
	}
	if err := state.Unhide(ctx, "still-hidden"); err != nil {
		t.Fatalf("degraded Unhide() error = %v", err)
	}
	assertCarrier(t, values.HiddenCarrier, "")
}

// child-add / child-list / child-clear, in the shapes chat.sh writes them:
// a bare socket for a detached teammate (chat.sh:1362) and "<socket>\t<pane>"
// for one sharing this chat's server (chat.sh:435).
func TestChildrenRoundTripInCCDBShShapes(t *testing.T) {
	state, _ := openTestStore(t)
	ctx := context.Background()

	if err := state.AddChild(ctx, KindNew, "chat-1", "cc-new-worker", 10); err != nil {
		t.Fatal(err)
	}
	if err := state.AddChild(ctx, KindPane, "chat-1", "cc-1-1-1\t%3", 11); err != nil {
		t.Fatal(err)
	}
	// cc-db.sh:281 inserts OR IGNORE, so a repeat registration is one row.
	if err := state.AddChild(ctx, KindNew, "chat-1", "cc-new-worker", 12); err != nil {
		t.Fatal(err)
	}

	detached, fromTable, err := state.Children(ctx, KindNew, "chat-1")
	if err != nil || !fromTable {
		t.Fatalf("Children(new) fromTable=%v err=%v", fromTable, err)
	}
	if !reflect.DeepEqual(detached, []string{"cc-new-worker"}) {
		t.Fatalf("Children(new) = %v", detached)
	}
	panes, _, err := state.Children(ctx, KindPane, "chat-1")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(panes, []string{"cc-1-1-1\t%3"}) {
		t.Fatalf("Children(pane) = %v", panes)
	}

	if err := state.ClearChildren(ctx, KindNew, "chat-1"); err != nil {
		t.Fatal(err)
	}
	detached, _, err = state.Children(ctx, KindNew, "chat-1")
	if err != nil || len(detached) != 0 {
		t.Fatalf("Children(new) after clear = %v, %v", detached, err)
	}
	if panes, _, err = state.Children(ctx, KindPane, "chat-1"); err != nil ||
		len(panes) != 1 {
		t.Fatalf("clearing one kind cleared the other: %v, %v", panes, err)
	}
}

// primary-get reads the database first and the ~/.claude-primary mirror only
// when the database has no row (cc-db.sh:262-267).
func TestPrimaryAccountPrefersTheDatabaseOverTheMirror(t *testing.T) {
	state, values := openTestStore(t)
	ctx := context.Background()

	if account, found := PrimaryAccount(ctx, values); found {
		t.Fatalf("PrimaryAccount() with nothing recorded = %d, %v", account, found)
	}

	mirror := filepath.Join(values.Home, ".claude-primary")
	if err := os.MkdirAll(filepath.Dir(mirror), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mirror, []byte("1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if account, found := PrimaryAccount(ctx, values); !found || account != 1 {
		t.Fatalf("PrimaryAccount() from the mirror = %d, %v", account, found)
	}

	if err := state.SetMeta(ctx, PrimaryAccountKey, "2", 99); err != nil {
		t.Fatal(err)
	}
	if account, found := PrimaryAccount(ctx, values); !found || account != 2 {
		t.Fatalf(
			"PrimaryAccount() = %d, %v; want 2 — the database outranks a stale mirror",
			account,
			found,
		)
	}
}

// The reader must not be what creates the state store: a missing database is a
// missing database, not an empty one.
func TestPrimaryAccountNeverCreatesTheDatabase(t *testing.T) {
	root := t.TempDir()
	values := paths.Values{
		SharedDB:      filepath.Join(root, "state", "fleet.db"),
		HiddenCarrier: filepath.Join(root, "home", ".claude", ".cc-ls-hidden"),
		Home:          filepath.Join(root, "home"),
	}
	if account, found := PrimaryAccount(context.Background(), values); found {
		t.Fatalf("PrimaryAccount() = %d, %v, want not found", account, found)
	}
	if _, err := os.Stat(values.SharedDB); !os.IsNotExist(err) {
		t.Fatalf("reading the primary account created %s: %v", values.SharedDB, err)
	}
}

func openTestStore(t *testing.T) (*Store, paths.Values) {
	t.Helper()

	root := t.TempDir()
	values := paths.Values{
		SharedDB:      filepath.Join(root, "cc", "fleet.db"),
		HiddenCarrier: filepath.Join(root, "home", ".claude", ".cc-ls-hidden"),
		Home:          filepath.Join(root, "home"),
	}
	state := Open(context.Background(), values)
	if err := state.Degraded(); err != nil {
		t.Fatalf("Open() degraded: %v", err)
	}
	t.Cleanup(func() { _ = state.Close() })
	return state, values
}

func queryColumn(t *testing.T, state *Store, query string) []string {
	t.Helper()

	rows, err := state.db.Query(query)
	if err != nil {
		t.Fatalf("query %q: %v", query, err)
	}
	defer rows.Close()
	var values []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			t.Fatal(err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return values
}

func assertCarrier(t *testing.T, carrier, want string) {
	t.Helper()

	content, err := os.ReadFile(carrier)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read carrier: %v", err)
	}
	if string(content) != want {
		t.Fatalf("carrier = %q, want %q", content, want)
	}
}

func appendLine(t *testing.T, path, line string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(line + "\n"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
