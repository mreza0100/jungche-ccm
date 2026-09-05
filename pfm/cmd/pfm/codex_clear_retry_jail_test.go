package main

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"hostops/pfm/internal/gather"
	fleetindex "hostops/pfm/internal/index"
	"hostops/pfm/internal/kill"
	"hostops/pfm/internal/store"
	"hostops/pfm/internal/ui"
)

func TestCodexClearRefreshesBaselineAndRetainsFailedRetirement(t *testing.T) {
	for _, scenario := range []string{"unindexed", "missing-rollout", "write-failed", "stale-baseline"} {
		t.Run(scenario, func(t *testing.T) {
			root := jailTest(t)
			ctx := context.Background()
			tmuxTmpDir := filepath.Join(root, "tmuxtmp")
			const socket = "cx-1800000901-1-1"
			const oldID = "11111111-1111-4111-8111-111111111111"
			const newID = "22222222-2222-4222-8222-222222222222"
			startCodexStatusPane(t, tmuxTmpDir, socket, "  "+newID+` · /work/example · Full Access\n`)
			resolved := jailPaths(t)
			resolved.TmuxDir = filepath.Join(tmuxTmpDir, "tmux-"+strconv.Itoa(os.Getuid()))
			database, err := store.Open()
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			manager, err := kill.New(database, kill.Dependencies{})
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := manager.AdvanceCodexPane(ctx, socket, "%0", oldID); err != nil {
				t.Fatal(err)
			}
			var rolloutPath string
			if scenario != "unindexed" {
				rolloutPath = codexJailRollout(t, database, root, oldID, 1)
			}
			if scenario == "missing-rollout" {
				if err := os.Remove(rolloutPath); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "stale-baseline" {
				appendCodexClearPrompt(t, rolloutPath)
			}
			var faultDB *sql.DB
			if scenario == "write-failed" {
				faultDB, err = sql.Open("sqlite", database.SharedPath())
				if err != nil {
					t.Fatal(err)
				}
				defer faultDB.Close()
				if _, err := faultDB.Exec(`CREATE TRIGGER reject_clear BEFORE INSERT ON hidden BEGIN SELECT RAISE(FAIL, 'clear write fault'); END`); err != nil {
					t.Fatal(err)
				}
			}
			var stderr bytes.Buffer
			reconcile := func() {
				reconcileCodexPanes(ctx, database, gather.Snapshot{Panes: []gather.Pane{codexPane(socket, "%0")}}, commandRuntime{Paths: resolved}, printWarn(&stderr))
			}
			reconcile()
			if scenario != "stale-baseline" {
				bound, found, err := manager.CodexPaneBinding(ctx, socket, "%0")
				if err != nil || !found || bound != oldID {
					t.Fatalf("failed retirement lost retry: binding=%q found=%v err=%v warnings=%s", bound, found, err, stderr.String())
				}
				if stderr.Len() == 0 {
					t.Fatal("failed retirement was silent")
				}
				if faultDB != nil {
					if _, err := faultDB.Exec(`DROP TRIGGER reject_clear`); err != nil {
						t.Fatal(err)
					}
				}
				rolloutPath = codexJailRollout(t, database, root, oldID, 1)
				reconcile()
			}
			indexer, err := fleetindex.NewWithPaths(database, resolved)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := indexer.Run(ctx, fleetindex.Options{}); err != nil {
				t.Fatal(err)
			}
			lineage, found, err := database.CodexLineage(ctx, oldID)
			if err != nil || !found {
				t.Fatalf("lineage missing: %v", err)
			}
			killed, found, err := database.Killed(ctx, oldID)
			if err != nil || !found || killed.BaselinePrompts == nil || *killed.BaselinePrompts != lineage.PromptCount {
				t.Fatalf("index catch-up undid clear: kill=%#v prompts=%d found=%v err=%v warnings=%s", killed, lineage.PromptCount, found, err, stderr.String())
			}
			bound, found, err := manager.CodexPaneBinding(ctx, socket, "%0")
			if err != nil || !found || bound != newID {
				t.Fatalf("successful retirement did not advance: %q %v %v", bound, found, err)
			}
			// An actual later prompt still lifts the temporary clear hide.
			appendCodexClearPrompt(t, rolloutPath)
			if _, err := indexer.Run(ctx, fleetindex.Options{}); err != nil {
				t.Fatal(err)
			}
			if _, found, err := database.Killed(ctx, oldID); err != nil || found {
				t.Fatalf("genuine resumed prompt did not lift clear hide: found=%v err=%v", found, err)
			}
		})
	}
}

func TestParkedPickerStillObservesCodexClear(t *testing.T) {
	runParkedCodexClear(t, false)
}

func TestParkedPickerRetriesRefreshAfterCodexClear(t *testing.T) {
	runParkedCodexClear(t, true)
}

type clearRetryIndexRunner struct{ calls int }

func (runner *clearRetryIndexRunner) Run(context.Context, fleetindex.Options) (fleetindex.Counters, error) {
	runner.calls++
	if runner.calls == 3 {
		return fleetindex.Counters{}, fmt.Errorf("injected post-clear index failure")
	}
	return fleetindex.Counters{}, nil
}

func runParkedCodexClear(t *testing.T, failRefresh bool) {
	t.Helper()
	database, manager, socket, oldID, currentID := codexRegatherJailFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, _, err := manager.AdvanceCodexPane(ctx, socket, "%0", currentID); err != nil {
		t.Fatal(err)
	}
	resolved := jailPaths(t)
	updates := make(chan ui.Snapshot, 8)
	var stderr bytes.Buffer
	dependencies := refreshDependencies{activity: ui.NewActivityClock(time.Now())}
	if failRefresh {
		dependencies.newIndexer = func(*store.Store) (indexRunner, error) { return &clearRetryIndexRunner{}, nil }
	}
	go streamFleetRefreshesWith(ctx, database, scanRequest{}, printWarn(&stderr), &stderr, updates, dependencies)
	defer func() {
		cancel()
		for range updates {
		}
	}()
	for completed := 0; completed < 2; {
		select {
		case snapshot, ok := <-updates:
			if !ok {
				t.Fatal("stream closed before park")
			}
			if !snapshot.Refreshing {
				completed++
			}
		case <-time.After(15 * time.Second):
			t.Fatal("stream did not reach park")
		}
	}
	current, found, err := database.Rollout(ctx, currentID)
	if err != nil || !found {
		t.Fatalf("current rollout missing: %v", err)
	}
	appendCodexClearPrompt(t, current.Path)
	// Use the fixture's other indexed thread as the new bare identity. This
	// exercises real tmux capture without touching the picker's activity clock.
	command := exec.Command("tmux", "-L", socket, "send-keys", "-t", "%0", "-l", "\n  "+oldID+" · /work/example · Full Access")
	command.Env = append(os.Environ(), "TMUX=", "TMUX_TMPDIR="+filepath.Dir(resolved.TmuxDir))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("change fixture identity: %v: %s", err, output)
	}
	deadline := time.After(10 * time.Second)
	for {
		select {
		case snapshot, ok := <-updates:
			if !ok {
				t.Fatal("stream closed after clear")
			}
			if snapshot.Refreshing {
				continue
			}
			for _, row := range snapshot.Rows {
				if row.ID == currentID {
					t.Fatalf("cleared chat remained visible: %#v", row)
				}
			}
			assertNoStaleCodexRow(t, snapshot.Rows, currentID, oldID, "parked clear")
			return
		case <-deadline:
			t.Fatal("parked picker missed Codex /clear for 10 seconds")
		}
	}
}

func appendCodexClearPrompt(t *testing.T, path string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := file.WriteString(`{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"next"}]}}` + "\n")
	closeErr := file.Close()
	if writeErr != nil {
		t.Fatal(writeErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
}
