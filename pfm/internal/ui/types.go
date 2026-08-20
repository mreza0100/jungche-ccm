package ui

import (
	"context"
	"io"

	"hostops/pfm/internal/compose"
	pfmstats "hostops/pfm/internal/stats"
)

// Tab is one top-level picker body. Chats preserves the original picker and
// remains the default; Stats is sampled only while it is focused.
type Tab uint8

const (
	TabChats Tab = iota
	TabStats
	tabCount
)

type StatsSubtab uint8

const (
	StatsChats StatsSubtab = iota
	StatsDocker
	statsSubtabCount
)

type StatsFocus uint8

const (
	StatsFocusTop StatsFocus = iota
	StatsFocusSubtab
	StatsFocusContent
)

type StatsSort uint8

const (
	StatsSortCPU StatsSort = iota
	StatsSortRAM
)

// StatsSampler is injected so model tests can prove that boot and the Chats
// tab perform zero sampling. The real implementation reads proc/cgroup for
// resource counters and caches one identity lookup per new Docker cgroup ID.
type StatsSampler interface {
	Sample([]compose.Row) (pfmstats.Snapshot, error)
}

// Picker is the common boundary used by the interactive, plain, and TSV
// frontends. Interactive effects are described by Outcome; implementations do
// not mutate fleet state themselves.
type Picker interface {
	Pick(ctx context.Context, snapshot Snapshot) (Outcome, error)
}

// Snapshot is all state needed for a frame. Rendering never probes the
// filesystem, processes, tmux, or the clock.
type Snapshot struct {
	Rows            []compose.Row
	View            compose.View
	HiddenCount     int
	SuppressedCount int
	Refreshing      bool
	PrimaryAccount  int
	AccountIDs      []int
	Cache1H         bool
	Rotation        int
	NowNS           int64
	Width           int
	Height          int
	InitialQuery    string
	InitialCursorID string
	// ApplyHide performs a ⌃X the instant it is typed — the store write, and
	// the kill when the row is live. It is deliberately NOT deferred to quit:
	// the only exits that ever applied a batched hide were the ones that
	// launched something (Enter, ⌃T, ⌃O), so closing the picker the natural
	// way threw every mark away. Nil leaves the picker read-only (the plain
	// twin, tests).
	ApplyHide    func(HideChange) error
	StatsSampler StatsSampler
	NoSky        bool
	// Activity is the presence clock the background refresh reads to pick its
	// cadence. Nil for every non-interactive picker, which reads as always
	// active. Set once when the model is built; refresh snapshots leave it
	// alone, so a rebuild from the stream can never blind the loop to the user.
	Activity *ActivityClock
}

// OutcomeKind identifies why an interactive picker stopped.
type OutcomeKind uint8

const (
	OutcomeNone OutcomeKind = iota
	OutcomeSelected
	OutcomeReload
	OutcomeReboot
	OutcomeCancelled
)

// HideChange is one hidden-state change, applied the moment it is typed.
type HideChange struct {
	ID     string
	Engine string
	Hidden bool
	// Socket, Live and Name carry what hiding a RUNNING chat needs beyond the
	// store write: hiding it ends it, and only the picker knows the row was
	// live, which server it owns, and what to call it in the receipt.
	Socket string
	Live   bool
	Name   string
}

// Outcome contains the selected action plus every in-memory modifier. The
// caller owns persistence and action execution after the TUI has restored the
// terminal.
type Outcome struct {
	Kind           OutcomeKind
	Row            compose.Row
	PrimaryAccount int
	Cache1H        bool
	Rotation       int
	Query          string
	HideChanges    []HideChange
}

// RefreshMsg replaces the cached compose snapshot without doing I/O in Model.
type RefreshMsg struct {
	Snapshot Snapshot
}

// PlainPicker renders a human-readable noninteractive twin.
type PlainPicker struct {
	Writer io.Writer
}

// TSVPicker renders the stable machine-readable twin.
type TSVPicker struct {
	Writer io.Writer
}

// ReadWriteCloser is the narrow terminal handle BubblePicker needs.
type ReadWriteCloser interface {
	io.Reader
	io.Writer
	io.Closer
}

// BubblePicker runs the fancy picker exclusively on /dev/tty. OpenTTY is
// injectable for scratch-terminal tests; Updates may deliver fresh snapshots.
type BubblePicker struct {
	OpenTTY func() (ReadWriteCloser, error)
	Updates <-chan Snapshot
}
