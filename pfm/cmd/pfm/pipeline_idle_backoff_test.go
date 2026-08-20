package main

import (
	"bytes"
	"context"
	"testing"
	"time"

	"hostops/pfm/internal/store"
	"hostops/pfm/internal/ui"
)

func TestRefreshCadenceGrowsWhileUntouched(t *testing.T) {
	clock := ui.NewActivityClock(time.Now())
	cadence := newRefreshCadence(clock)

	if cadence.interval != fleetRefreshInterval {
		t.Fatalf(
			"opening interval = %s, want %s",
			cadence.interval, fleetRefreshInterval,
		)
	}

	want := []time.Duration{
		5500 * time.Millisecond,
		6050 * time.Millisecond,
		6655 * time.Millisecond,
	}
	for step, expected := range want {
		got := cadence.next()
		if got != expected {
			t.Fatalf("interval after %d untouched passes = %s, want %s",
				step+1, got, expected)
		}
	}
}

func TestRefreshCadenceResetsOnInteraction(t *testing.T) {
	clock := ui.NewActivityClock(time.Now())
	cadence := newRefreshCadence(clock)
	for range 20 {
		cadence.next()
	}
	if cadence.interval <= fleetRefreshInterval {
		t.Fatalf(
			"interval after 20 untouched passes = %s, want > %s",
			cadence.interval, fleetRefreshInterval,
		)
	}

	clock.Stamp(time.Now().Add(time.Second))
	if got := cadence.next(); got != fleetRefreshInterval {
		t.Fatalf(
			"interval after a keystroke = %s, want %s: a picker under an "+
				"active user must snap back to full cadence",
			got, fleetRefreshInterval,
		)
	}
}

func TestRefreshCadenceCapsAndNilClockNeverBacksOff(t *testing.T) {
	cadence := newRefreshCadence(ui.NewActivityClock(time.Now()))
	for range 500 {
		if got := cadence.next(); got > fleetRefreshMaxInterval {
			t.Fatalf("interval %s exceeded the cap %s",
				got, fleetRefreshMaxInterval)
		}
	}
	if cadence.interval != fleetRefreshMaxInterval {
		t.Fatalf("interval after 500 passes = %s, want the cap %s",
			cadence.interval, fleetRefreshMaxInterval)
	}

	// A nil clock is every non-interactive caller: no presence signal must
	// ever be READ as "nobody is there".
	nilCadence := newRefreshCadence(nil)
	for range 50 {
		if got := nilCadence.next(); got != fleetRefreshInterval {
			t.Fatalf("nil-clock interval = %s, want a steady %s",
				got, fleetRefreshInterval)
		}
	}
}

// TestPickerRefreshStreamStretchesWhileUntouched pins the fix in the LIVE
// loop, not just in the cadence arithmetic. A picker used to refresh on a
// fixed ticker for its whole life, and one pass costs more than the interval
// did — a tmux fork+exec per live socket plus a full store read — so a picker
// left in a pane nobody watched ground half a CPU core indefinitely.
//
// The unit tests above prove next() computes the right numbers. Only this one
// proves the loop actually ASKS it: a stream still wired to a constant would
// pass every test above and keep burning the core.
func TestPickerRefreshStreamStretchesWhileUntouched(t *testing.T) {
	jailTest(t)
	t.Setenv(codexAvailableEnv, "0")

	gaps := refreshGaps(t, 4)
	first, last := gaps[0], gaps[len(gaps)-1]
	if last <= first {
		t.Fatalf(
			"refresh gaps did not stretch: first %s, last %s (all %v) — the "+
				"loop is still refreshing on a constant interval",
			first, last, gaps,
		)
	}
	// 5s → 6.05s by the third gap is a 21% stretch; require well over half of
	// it so scheduler noise cannot pass a loop that never backs off at all.
	if ratio := float64(last) / float64(first); ratio < 1.10 {
		t.Fatalf(
			"refresh gap ratio = %.2f (first %s, last %s, all %v), want >= 1.10",
			ratio, first, last, gaps,
		)
	}
}

// refreshGaps runs one untouched refresh stream and returns the intervals
// between the first count completed passes. It fails the test outright if the
// stream dies or starves, so a returned slice always means "we watched real
// passes", never "we saw nothing and called it a result".
func refreshGaps(t *testing.T, count int) []time.Duration {
	t.Helper()
	database, err := store.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	updates := make(chan ui.Snapshot, 1)
	var stderr bytes.Buffer
	runner := &immediateIndexRunner{}
	// Stamped once at birth and never again: this picker is opened and then
	// abandoned, which is exactly the shape that burned the core.
	go streamFleetRefreshesWith(
		ctx,
		database,
		scanRequest{},
		printWarn(&stderr),
		&stderr,
		updates,
		refreshDependencies{
			newIndexer: func(*store.Store) (indexRunner, error) {
				return runner, nil
			},
			activity: ui.NewActivityClock(time.Now()),
		},
	)

	starve := time.After(3 * time.Minute)
	completed := make([]time.Time, 0, count+1)
	for len(completed) < count+1 {
		select {
		case snapshot, ok := <-updates:
			if !ok {
				t.Fatalf(
					"refresh stream closed after %d of %d passes: %s",
					len(completed), count+1, stderr.String(),
				)
			}
			if !snapshot.Refreshing {
				completed = append(completed, time.Now())
			}
		case <-starve:
			t.Fatalf(
				"refresh stream produced %d of %d passes in 3m: %s",
				len(completed), count+1, stderr.String(),
			)
		}
	}
	cancel()
	for range updates {
	}

	gaps := make([]time.Duration, 0, count)
	for index := 1; index < len(completed); index++ {
		gaps = append(gaps, completed[index].Sub(completed[index-1]))
	}
	return gaps
}
