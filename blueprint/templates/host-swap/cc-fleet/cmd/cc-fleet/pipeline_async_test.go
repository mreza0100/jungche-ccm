package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	fleetindex "hostops/cc-fleet/internal/index"
	"hostops/cc-fleet/internal/store"
	"hostops/cc-fleet/internal/ui"
)

type slowIndexRunner struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
	mutex   sync.Mutex
	options []fleetindex.Options
}

type immediateIndexRunner struct{}

func (immediateIndexRunner) Run(
	context.Context,
	fleetindex.Options,
) (fleetindex.Counters, error) {
	return fleetindex.Counters{}, nil
}

func (runner *slowIndexRunner) Run(
	ctx context.Context,
	options fleetindex.Options,
) (fleetindex.Counters, error) {
	runner.mutex.Lock()
	runner.options = append(runner.options, options)
	runner.mutex.Unlock()
	blocked := false
	runner.once.Do(func() {
		close(runner.started)
		blocked = true
	})
	if blocked {
		select {
		case <-runner.release:
		case <-ctx.Done():
			return fleetindex.Counters{}, ctx.Err()
		}
	}
	return fleetindex.Counters{}, nil
}

func TestCachedFirstPaintWhileIndexRefreshIsSlow(t *testing.T) {
	jailTest(t)
	t.Setenv(codexAvailableEnv, "0")
	database, err := store.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	const rows = 5_000
	if err := database.Batch(
		context.Background(),
		rows,
		func(tx *store.ImmediateTx, start, end int) error {
			for index := start; index < end; index++ {
				if err := tx.UpsertTranscript(context.Background(), store.Transcript{
					UUID:        fmt.Sprintf("cached-%08d", index),
					Path:        fmt.Sprintf("/jail/cached-%08d.jsonl", index),
					Size:        1024,
					MTimeNS:     int64(index + 1),
					CWD:         cwd,
					FirstPrompt: fmt.Sprintf("cached row %08d", index),
					PromptCount: 1,
				}); err != nil {
					return err
				}
			}
			return nil
		},
	); err != nil {
		t.Fatal(err)
	}

	strict := os.Getenv("CC_FLEET_STRESS_STRICT") == "1"
	limit := 2 * time.Second
	if strict {
		limit = 100 * time.Millisecond
	}
	request := scanRequest{Cache1H: true}
	started := time.Now()
	cached, err := scanFleetCached(context.Background(), database, request)
	if err != nil {
		t.Fatal(err)
	}
	if len(cached.Snapshot.Rows) != 31 ||
		cached.Snapshot.HiddenCount != 0 ||
		cached.Snapshot.SuppressedCount != rows-30 {
		t.Fatalf(
			"cached snapshot rows/hidden/empty=%d/%d/%d, want 31/0/%d",
			len(cached.Snapshot.Rows),
			cached.Snapshot.HiddenCount,
			cached.Snapshot.SuppressedCount,
			rows-30,
		)
	}
	scanReady := time.Since(started)
	modelStarted := time.Now()
	model := ui.NewModel(cached.Snapshot)
	modelReady := time.Since(modelStarted)
	viewStarted := time.Now()
	if view := model.View(); view.Content == "" {
		t.Fatal("cached first frame is empty")
	}
	viewReady := time.Since(viewStarted)
	firstPaint := time.Since(started)
	if firstPaint >= limit {
		t.Fatalf(
			"cached 5k first paint=%s (scan=%s model=%s view=%s), want <%s (strict=%t)",
			firstPaint,
			scanReady,
			modelReady,
			viewReady,
			limit,
			strict,
		)
	}

	runner := &slowIndexRunner{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	updates := make(chan ui.Snapshot, 1)
	refreshContext, cancel := context.WithCancel(context.Background())
	before := runtime.NumGoroutine()
	var stderr bytes.Buffer
	go streamFleetRefreshesWith(
		refreshContext,
		database,
		request,
		&stderr,
		updates,
		refreshDependencies{
			newIndexer: func(*store.Store) (indexRunner, error) {
				return runner, nil
			},
		},
	)
	gathered, ok := <-updates
	if !ok {
		t.Fatalf("refresh ended before gather: %s", stderr.String())
	}
	select {
	case <-runner.started:
	case <-time.After(2 * time.Second):
		t.Fatal("slow project index did not start asynchronously")
	}
	follow := model.SelectedKey()
	updated, command := model.Update(ui.RefreshMsg{Snapshot: gathered})
	if command != nil {
		t.Fatal("gather refresh returned a command")
	}
	model = updated.(ui.Model)
	if model.SelectedKey() != follow {
		t.Fatalf("gather refresh moved cursor from %q to %q", follow, model.SelectedKey())
	}
	close(runner.release)
	for snapshot := range updates {
		updated, _ = model.Update(ui.RefreshMsg{Snapshot: snapshot})
		model = updated.(ui.Model)
		if model.SelectedKey() != follow {
			t.Fatalf("index refresh moved cursor from %q to %q", follow, model.SelectedKey())
		}
	}
	cancel()
	runner.mutex.Lock()
	options := append([]fleetindex.Options(nil), runner.options...)
	runner.mutex.Unlock()
	if len(options) != 2 ||
		!options[0].PriorityOnly ||
		options[0].PriorityCWD != cwd ||
		options[1].PriorityOnly ||
		options[1].PriorityCWD != cwd {
		t.Fatalf("async index stages = %#v", options)
	}
	runtime.GC()
	after := runtime.NumGoroutine()
	if after > before+1 {
		t.Fatalf("async refresh leaked goroutines: before=%d after=%d", before, after)
	}
	t.Logf(
		"STRESS cached_rows=%d first_paint=%s scan=%s model=%s view=%s slow_index_async=true refreshes=3 cursor_follow=true goroutines=%d/%d strict=%t",
		rows,
		firstPaint,
		scanReady,
		modelReady,
		viewReady,
		before,
		after,
		strict,
	)
}

func TestAsyncCallerRefreshStormPreservesCursorAndGoroutines(t *testing.T) {
	jailTest(t)
	database, err := store.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	const rows = 256
	if err := database.Batch(
		context.Background(),
		rows,
		func(tx *store.ImmediateTx, start, end int) error {
			for index := start; index < end; index++ {
				if err := tx.UpsertTranscript(context.Background(), store.Transcript{
					UUID:        fmt.Sprintf("storm-%08d", index),
					Path:        fmt.Sprintf("/jail/storm-%08d.jsonl", index),
					Size:        1024,
					MTimeNS:     int64(index + 1),
					CWD:         cwd,
					FirstPrompt: fmt.Sprintf("storm row %08d", index),
					PromptCount: 1,
				}); err != nil {
					return err
				}
			}
			return nil
		},
	); err != nil {
		t.Fatal(err)
	}
	request := scanRequest{}
	cached, err := scanFleetCached(context.Background(), database, request)
	if err != nil {
		t.Fatal(err)
	}
	cached.Snapshot.InitialCursorID = cached.Snapshot.Rows[len(cached.Snapshot.Rows)/2].ID
	model := ui.NewModel(cached.Snapshot)
	follow := model.SelectedKey()
	before := runtime.NumGoroutine()
	const storms = 100
	for storm := 0; storm < storms; storm++ {
		updates := make(chan ui.Snapshot, 1)
		go streamFleetRefreshesWith(
			context.Background(),
			database,
			request,
			&bytes.Buffer{},
			updates,
			refreshDependencies{
				newIndexer: func(*store.Store) (indexRunner, error) {
					return immediateIndexRunner{}, nil
				},
			},
		)
		for snapshot := range updates {
			updated, command := model.Update(ui.RefreshMsg{Snapshot: snapshot})
			if command != nil {
				t.Fatalf("storm %d refresh returned command", storm)
			}
			model = updated.(ui.Model)
			if model.SelectedKey() != follow {
				t.Fatalf(
					"storm %d moved cursor from %q to %q",
					storm,
					follow,
					model.SelectedKey(),
				)
			}
		}
	}
	runtime.GC()
	after := runtime.NumGoroutine()
	if after > before+1 {
		t.Fatalf("refresh storm leaked goroutines: before=%d after=%d", before, after)
	}
	t.Logf(
		"STRESS async_caller_storms=%d messages=%d cursor_follow=true goroutines=%d/%d",
		storms,
		storms*3,
		before,
		after,
	)
}
