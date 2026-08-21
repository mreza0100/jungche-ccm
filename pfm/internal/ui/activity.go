package ui

import (
	"sync/atomic"
	"time"
)

// ActivityClock is the picker's presence signal: the moment a human last
// touched the keyboard. The background fleet refresh reads it to choose its
// own cadence, so a picker nobody is driving stops paying for scans nobody
// reads.
//
// It exists because the refresh loop had no way to ask the question at all. A
// full pass costs roughly 2.6 CPU-seconds here — one tmux fork+exec per live
// socket plus a whole store read — and it fired every 4 seconds for the life
// of the process. An abandoned picker in a detached pane therefore held ~50%
// of a core indefinitely while rendering to nobody.
//
// The zero value is not usable; the nil pointer deliberately is. Every
// non-interactive caller (plain, tsv, the one-shot commands, the existing
// stream tests) passes nil and reads as permanently active, which is the
// pre-fix cadence and the safe direction to fail: a missing presence signal
// slows nothing down, it only declines to speed anything up.
type ActivityClock struct {
	lastNS atomic.Int64
}

// NewActivityClock starts a clock already stamped: typing `pfm ls` IS the
// first interaction, so the picker opens at full cadence rather than climbing
// out of a backoff it never earned.
func NewActivityClock(now time.Time) *ActivityClock {
	clock := &ActivityClock{}
	clock.Stamp(now)
	return clock
}

// Stamp records an interaction. Safe on a nil clock.
func (clock *ActivityClock) Stamp(now time.Time) {
	if clock == nil {
		return
	}
	clock.lastNS.Store(now.UnixNano())
}

// StampNS returns the raw stamp of the last interaction, or 0 for a nil or
// never-stamped clock. The refresh stream compares it for CHANGE rather than
// reading an elapsed duration: equality needs no wall clock, so a machine
// whose time steps backwards cannot fake an interaction or kill one.
func (clock *ActivityClock) StampNS() int64 {
	if clock == nil {
		return 0
	}
	return clock.lastNS.Load()
}
