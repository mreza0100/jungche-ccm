// Package corpus enumerates and records the transcript corpus for a dreamer
// night. It owns only filesystem artifacts; model output and fleet state are
// outside this package.
package corpus

import (
	"time"

	"hostops/cc-fleet/internal/dream/artifact"
)

type WindowMode string

const (
	WindowSweepCutoff    WindowMode = "sweep-cutoff"
	WindowBootstrap      WindowMode = "bootstrap-count"
	WindowExplicitCorpus WindowMode = "corpus-file"
)

type CutoffSource string

const (
	CutoffEnumeratedAt CutoffSource = "enumerated-at"
	CutoffApplied      CutoffSource = "Applied"
	CutoffFilenameDate CutoffSource = "filename-date"
	CutoffBootstrap    CutoffSource = "bootstrap"
)

// Selection chooses one of the normal rolling window, a full-registry
// bootstrap capped to BootstrapCount, or an explicit CorpusFile.
type Selection struct {
	BootstrapCount int
	CorpusFile     string
}

// Window is the auditable selection window written to meta/window.tsv.
// CutoffTime is the value used for comparisons; CutoffExclusive preserves the
// human-readable value recorded in the artifact.
type Window struct {
	Mode               WindowMode
	BootstrapCount     int
	CorpusFile         string
	CorpusFileSHA256   string
	AgentType          string
	Lane               string
	NewestAppliedSweep string
	CutoffSource       CutoffSource
	CutoffExclusive    string
	CutoffTime         time.Time
	EnumeratedAt       time.Time
}

type GapKind string

const GapMissingTranscript GapKind = "META-PRESENT-TRANSCRIPT-MISSING"

// Gap records a matching agent metadata file whose paired transcript is not a
// regular file. That condition is evidence, not an absent selection.
type Gap struct {
	Kind       GapKind
	Meta       string
	Transcript string
}

// Candidate is a valid matching metadata/transcript pair. Bootstrap selection
// ranks candidates by MTime descending and then Meta path byte-ascending.
type Candidate struct {
	Meta       string
	Transcript string
	MTime      time.Time
}

// Result contains both the selected corpus and the evidence that explains the
// selection. CorpusFileBytes is the one and only read of an explicit corpus
// file; Write copies these exact bytes into the stage audit trail.
type Result struct {
	Paths             []string
	PathsSHA256       string
	Census            artifact.Census
	Window            Window
	Gaps              []Gap
	Selected          []Candidate
	CorpusFileBytes   []byte
	CutoffDescription string
}
