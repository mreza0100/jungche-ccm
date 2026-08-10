package store

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"hostops/cc-fleet/internal/paths"
)

const (
	helperProcessEnv = "CC_FLEET_STORE_HELPER"
	helperPrefixEnv  = "CC_FLEET_STORE_HELPER_PREFIX"
	helperReadyEnv   = "CC_FLEET_STORE_HELPER_READY"
	helperGateEnv    = "CC_FLEET_STORE_HELPER_GATE"
	helperWriteCount = 200
)

func TestHiddenBusyPolicyWarnsAndSucceeds(t *testing.T) {
	setStoreTestJail(t)
	blocker := openTestStore(t)
	t.Cleanup(func() { _ = blocker.Close() })

	var warnings bytes.Buffer
	writer := openTestStore(t, WithWarningWriter(&warnings))
	t.Cleanup(func() { _ = writer.Close() })
	ctx := context.Background()

	if _, err := writer.db.ExecContext(ctx, "PRAGMA busy_timeout=1"); err != nil {
		t.Fatalf("shorten busy timeout: %v", err)
	}
	if err := blocker.WithImmediateTx(ctx, func(tx *ImmediateTx) error {
		if err := tx.SetMeta(ctx, "hold_writer_lock", "yes"); err != nil {
			return err
		}
		return writer.Hide(ctx, Hidden{
			ID:       "busy-hide",
			Engine:   "cc",
			HiddenAt: 1,
		})
	}); err != nil {
		t.Fatalf("Hide() under persistent SQLITE_BUSY error = %v, want nil", err)
	}
	if got := warnings.String(); !strings.Contains(got, "WARNING:") ||
		!strings.Contains(got, "NOT saved") {
		t.Fatalf("busy warning = %q, want loud not-saved warning", got)
	}
	if _, found, err := writer.Hidden(ctx, "busy-hide"); err != nil || found {
		t.Fatalf("Hidden() after dropped busy write found = %v, error = %v; want false, nil", found, err)
	}

	if err := writer.Hide(ctx, Hidden{ID: "busy-unhide", Engine: "cx", HiddenAt: 2}); err != nil {
		t.Fatalf("seed Hide() error = %v", err)
	}
	warnings.Reset()
	if err := blocker.WithImmediateTx(ctx, func(tx *ImmediateTx) error {
		if err := tx.SetMeta(ctx, "hold_writer_lock_again", "yes"); err != nil {
			return err
		}
		return writer.Unhide(ctx, "busy-unhide")
	}); err != nil {
		t.Fatalf("Unhide() under persistent SQLITE_BUSY error = %v, want nil", err)
	}
	if got := warnings.String(); !strings.Contains(got, "WARNING:") ||
		!strings.Contains(got, "NOT saved") {
		t.Fatalf("busy warning = %q, want loud not-saved warning", got)
	}
	if _, found, err := writer.Hidden(ctx, "busy-unhide"); err != nil || !found {
		t.Fatalf("Hidden() after dropped busy unhide found = %v, error = %v; want true, nil", found, err)
	}
}

func TestOrphanedHidesListMatchesCountsAndPrunes(t *testing.T) {
	setStoreTestJail(t)
	database := openTestStore(t)
	t.Cleanup(func() { _ = database.Close() })
	ctx := context.Background()

	if err := database.UpsertTranscript(ctx, Transcript{
		UUID: "live-cc",
		Path: "/jail/live-cc.jsonl",
	}); err != nil {
		t.Fatal(err)
	}
	if err := database.UpsertRollout(ctx, Rollout{
		ID:          "live-cx",
		Path:        "/jail/live-cx.jsonl",
		LineageRoot: "lineage-root",
	}); err != nil {
		t.Fatal(err)
	}
	if err := database.UpsertCxName(ctx, CxName{
		ID:         "named-only-cx",
		ThreadName: "named",
	}); err != nil {
		t.Fatal(err)
	}
	live := []Hidden{
		{ID: "live-cc", Engine: "cc", HiddenAt: 1},
		{ID: "live-cx", Engine: "cx", HiddenAt: 2},
		{ID: "lineage-root", Engine: "cx", HiddenAt: 3},
	}
	orphaned := []Hidden{
		{ID: "dead-cc", Engine: "cc", HiddenAt: 4},
		{ID: "dead-cx", Engine: "cx", HiddenAt: 5},
		{ID: "named-only-cx", Engine: "cx", HiddenAt: 6},
	}
	for _, hidden := range append(append([]Hidden(nil), live...), orphaned...) {
		if err := database.Hide(ctx, hidden); err != nil {
			t.Fatalf("Hide(%s) error = %v", hidden.ID, err)
		}
	}

	counts, err := database.Counts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	orphans, err := database.OrphanedHides(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(orphans) != counts.OrphanedHides {
		t.Fatalf(
			"OrphanedHides() = %d rows, Counts().OrphanedHides = %d; want the same predicate",
			len(orphans),
			counts.OrphanedHides,
		)
	}
	got := make([]string, 0, len(orphans))
	for _, orphan := range orphans {
		got = append(got, orphan.ID)
	}
	want := []string{"dead-cc", "dead-cx", "named-only-cx"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("OrphanedHides() ids = %v, want %v", got, want)
	}

	deleted, err := database.DeleteOrphanedHides(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != len(want) {
		t.Fatalf("DeleteOrphanedHides() = %d, want %d", deleted, len(want))
	}
	remaining, err := database.HiddenChats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	survivors := make([]string, 0, len(remaining))
	for _, hidden := range remaining {
		survivors = append(survivors, hidden.ID)
	}
	wantSurvivors := []string{"lineage-root", "live-cc", "live-cx"}
	if !reflect.DeepEqual(survivors, wantSurvivors) {
		t.Fatalf("HiddenChats() after prune = %v, want %v", survivors, wantSurvivors)
	}

	counts, err = database.Counts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if counts.OrphanedHides != 0 {
		t.Fatalf("Counts().OrphanedHides after prune = %d, want 0", counts.OrphanedHides)
	}
	deleted, err = database.DeleteOrphanedHides(ctx)
	if err != nil || deleted != 0 {
		t.Fatalf("second DeleteOrphanedHides() = %d, %v; want 0, nil", deleted, err)
	}
}

func TestConcurrentHiddenWritesAcrossProcesses(t *testing.T) {
	dbPath := setStoreTestJail(t)
	initial := openTestStore(t)
	if err := initial.Close(); err != nil {
		t.Fatalf("initial Close() error = %v", err)
	}

	root := filepath.Dir(dbPath)
	gate := filepath.Join(root, "go")
	readyA := filepath.Join(root, "ready-a")
	readyB := filepath.Join(root, "ready-b")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	processes := []*helperCommand{
		newHelperCommand(ctx, "a", readyA, gate),
		newHelperCommand(ctx, "b", readyB, gate),
	}
	for _, process := range processes {
		if err := process.cmd.Start(); err != nil {
			t.Fatalf("start helper %s: %v", process.prefix, err)
		}
		process := process
		t.Cleanup(func() {
			if process.cmd.Process != nil {
				_ = process.cmd.Process.Kill()
			}
		})
	}

	if err := waitForPaths(ctx, readyA, readyB); err != nil {
		t.Fatalf(
			"wait for helpers: %v\n%s\n%s",
			err,
			processes[0].output.String(),
			processes[1].output.String(),
		)
	}
	if err := os.WriteFile(gate, []byte("go"), 0o600); err != nil {
		t.Fatalf("release helper gate: %v", err)
	}

	for _, process := range processes {
		if err := process.cmd.Wait(); err != nil {
			t.Fatalf("helper %s: %v\n%s", process.prefix, err, process.output.String())
		}
	}

	store := openTestStore(t)
	t.Cleanup(func() { _ = store.Close() })
	hiddenChats, err := store.HiddenChats(context.Background())
	if err != nil {
		t.Fatalf("HiddenChats() error = %v", err)
	}
	if got, want := len(hiddenChats), len(processes)*helperWriteCount; got != want {
		t.Fatalf("concurrent hidden write count = %d, want %d", got, want)
	}

	seen := make(map[string]bool, len(hiddenChats))
	for _, hidden := range hiddenChats {
		seen[hidden.ID] = true
	}
	for _, prefix := range []string{"a", "b"} {
		for index := 0; index < helperWriteCount; index++ {
			id := fmt.Sprintf("%s-%03d", prefix, index)
			if !seen[id] {
				t.Fatalf("concurrent hidden writes lost %q", id)
			}
		}
	}
}

type helperCommand struct {
	cmd    *exec.Cmd
	prefix string
	output bytes.Buffer
}

func newHelperCommand(
	ctx context.Context,
	prefix string,
	ready string,
	gate string,
) *helperCommand {
	helper := &helperCommand{prefix: prefix}
	helper.cmd = exec.CommandContext(
		ctx,
		os.Args[0],
		"-test.run=^TestHiddenWriteHelperProcess$",
	)
	helper.cmd.Env = append(
		os.Environ(),
		helperProcessEnv+"=1",
		helperPrefixEnv+"="+prefix,
		helperReadyEnv+"="+ready,
		helperGateEnv+"="+gate,
	)
	helper.cmd.Stdout = &helper.output
	helper.cmd.Stderr = &helper.output
	return helper
}

func TestHiddenWriteHelperProcess(t *testing.T) {
	if os.Getenv(helperProcessEnv) != "1" {
		t.Skip("helper process only")
	}
	if os.Getenv(paths.EnvDB) == "" {
		t.Fatal("helper process has no CC_FLEET_DB jail")
	}

	store := openTestStore(t)
	defer store.Close()
	ready := os.Getenv(helperReadyEnv)
	gate := os.Getenv(helperGateEnv)
	if err := os.WriteFile(ready, []byte("ready"), 0o600); err != nil {
		t.Fatalf("write ready marker: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := waitForPaths(ctx, gate); err != nil {
		t.Fatalf("wait for gate: %v", err)
	}

	prefix := os.Getenv(helperPrefixEnv)
	engine := "cc"
	if prefix == "b" {
		engine = "cx"
	}
	for index := 0; index < helperWriteCount; index++ {
		baseline := int64(index)
		if err := store.Hide(ctx, Hidden{
			ID:              fmt.Sprintf("%s-%03d", prefix, index),
			Engine:          engine,
			HiddenAt:        int64(index + 1),
			BaselinePrompts: &baseline,
		}); err != nil {
			t.Fatalf("Hide(%s, %d) error = %v", prefix, index, err)
		}
	}
}

func waitForPaths(ctx context.Context, paths ...string) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		allPresent := true
		for _, path := range paths {
			if _, err := os.Stat(path); err != nil {
				if !os.IsNotExist(err) {
					return err
				}
				allPresent = false
				break
			}
		}
		if allPresent {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
