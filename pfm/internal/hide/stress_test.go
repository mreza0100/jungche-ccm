package hide

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hostops/pfm/internal/store"
)

const (
	hideStressHelperEnv = "PFM_HIDE_STRESS_HELPER"
	hideStressIndexEnv  = "PFM_HIDE_STRESS_INDEX"
	hideStressReadyEnv  = "PFM_HIDE_STRESS_READY"
	hideStressGateEnv   = "PFM_HIDE_STRESS_GATE"
	hideStressProcesses = 20
	hideStressIDs       = 5
)

func TestStressTwentyHideUnhideProcesses(t *testing.T) {
	jail := newHideJail(t)
	database := jail.open(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err := database.Batch(
		ctx,
		hideStressProcesses*hideStressIDs*2,
		func(tx *store.ImmediateTx, start, end int) error {
			for index := start; index < end; index++ {
				process := index / (hideStressIDs * 2)
				offset := (index / 2) % hideStressIDs
				kind := "keep"
				if index%2 == 1 {
					kind = "drop"
				}
				id := fmt.Sprintf("%s-%02d-%02d", kind, process, offset)
				if err := tx.UpsertTranscript(ctx, store.Transcript{
					UUID: id,
					Path: "/jailed/" + id + ".jsonl",
				}); err != nil {
					return err
				}
			}
			return nil
		},
	); err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	gate := filepath.Join(jail.root, "hide-stress-go")
	processes := make([]*hideStressProcess, 0, hideStressProcesses)
	readyPaths := make([]string, 0, hideStressProcesses)
	for index := 0; index < hideStressProcesses; index++ {
		ready := filepath.Join(jail.root, fmt.Sprintf("ready-%02d", index))
		command := exec.CommandContext(
			ctx,
			os.Args[0],
			"-test.run=^TestHideStressProcessHelper$",
		)
		command.Env = append(
			os.Environ(),
			hideStressHelperEnv+"=1",
			fmt.Sprintf("%s=%d", hideStressIndexEnv, index),
			hideStressReadyEnv+"="+ready,
			hideStressGateEnv+"="+gate,
		)
		process := &hideStressProcess{command: command}
		command.Stdout = &process.output
		command.Stderr = &process.output
		if err := command.Start(); err != nil {
			t.Fatalf("start helper %d: %v", index, err)
		}
		processes = append(processes, process)
		readyPaths = append(readyPaths, ready)
	}
	t.Cleanup(func() {
		for _, process := range processes {
			if process.command.Process != nil {
				_ = process.command.Process.Kill()
			}
		}
	})
	if err := waitHideStressPaths(ctx, readyPaths); err != nil {
		t.Fatalf("wait for helpers: %v\n%s", err, hideStressOutputs(processes))
	}
	started := time.Now()
	if err := os.WriteFile(gate, []byte("go"), 0o600); err != nil {
		t.Fatal(err)
	}
	warnings := 0
	for index, process := range processes {
		if err := process.command.Wait(); err != nil {
			t.Fatalf("helper %d: %v\n%s", index, err, process.output.String())
		}
		warnings += strings.Count(process.output.String(), "WARNING:")
	}
	elapsed := time.Since(started)

	database = jail.open(t)
	defer database.Close()
	hidden, err := database.HiddenChats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := hideStressProcesses * hideStressIDs
	if len(hidden) != want {
		t.Fatalf(
			"hidden count=%d, want=%d; warnings=%d",
			len(hidden),
			want,
			warnings,
		)
	}
	for _, row := range hidden {
		if !strings.HasPrefix(row.ID, "keep-") {
			t.Fatalf("unhide lost for %#v", row)
		}
	}
	if warnings != 0 {
		t.Fatalf(
			"contention dropped accepted writes: warnings=%d, want 0",
			warnings,
		)
	}
	t.Logf(
		"STRESS hide/unhide processes=%d operations=%d final=%d warnings=%d elapsed=%s",
		hideStressProcesses,
		hideStressProcesses*hideStressIDs*3,
		len(hidden),
		warnings,
		elapsed,
	)
}

func TestHideStressProcessHelper(t *testing.T) {
	if os.Getenv(hideStressHelperEnv) != "1" {
		return
	}
	index := 0
	if _, err := fmt.Sscanf(os.Getenv(hideStressIndexEnv), "%d", &index); err != nil {
		t.Fatal(err)
	}
	database, err := store.Open(store.WithWarningWriter(os.Stdout))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	manager, err := New(database, Dependencies{})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		os.Getenv(hideStressReadyEnv),
		[]byte("ready"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for {
		if _, err := os.Stat(os.Getenv(hideStressGateEnv)); err == nil {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(5 * time.Millisecond):
		}
	}
	for offset := 0; offset < hideStressIDs; offset++ {
		keep := fmt.Sprintf("keep-%02d-%02d", index, offset)
		drop := fmt.Sprintf("drop-%02d-%02d", index, offset)
		if _, err := manager.Hide(ctx, Request{ID: keep}); err != nil {
			t.Fatal(err)
		}
		if _, err := manager.Hide(ctx, Request{ID: drop}); err != nil {
			t.Fatal(err)
		}
		if err := manager.Unhide(ctx, drop); err != nil {
			t.Fatal(err)
		}
	}
}

type hideStressProcess struct {
	command *exec.Cmd
	output  bytes.Buffer
}

func waitHideStressPaths(ctx context.Context, paths []string) error {
	pending := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		pending[path] = struct{}{}
	}
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for len(pending) != 0 {
		for path := range pending {
			if _, err := os.Stat(path); err == nil {
				delete(pending, path)
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
	return nil
}

func hideStressOutputs(processes []*hideStressProcess) string {
	var output strings.Builder
	for index, process := range processes {
		fmt.Fprintf(&output, "\nhelper %d:\n%s", index, process.output.String())
	}
	return output.String()
}
