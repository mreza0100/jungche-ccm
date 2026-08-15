// Package artifact defines the dreamer's file formats and validates them at
// their trust boundary. It deliberately performs no filesystem or Git I/O;
// gates consume these typed values and do live checks separately.
package artifact

type CoverageStatus string

const (
	CoverageRead CoverageStatus = "READ"
	CoverageSkip CoverageStatus = "SKIP"
)

type CoverageLine struct {
	Index  int
	Status CoverageStatus
	Reason string
}

type ConductKind string

const (
	ConductTechnique ConductKind = "technique"
	ConductPrior     ConductKind = "prior"
	ConductBaseline  ConductKind = "baseline"
)

type ConductLine struct {
	Kind   ConductKind
	Slug   string
	Reason string
}

type Coverage struct {
	Lines   []CoverageLine
	Conduct []ConductLine
}

type GitObjectType string

const (
	GitBlob GitObjectType = "blob"
	GitTree GitObjectType = "tree"
)

type AnchorRow struct {
	DisplayPath string
	LookupPath  string
	ObjectType  GitObjectType
	Hash        string
}

type Provenance struct {
	Date      string
	SessionID string
}

// Map is the syntax-validated structure of a canonical map. Its anchors have
// not yet been compared with a live Git tree; that belongs to the anchor gate.
type Map struct {
	Title string
	// Lesson is the one-line rule the lane surface floats in an agent's context
	// beside the map pointer. Empty for maps written before the section existed;
	// RenderSurfaces falls back to Title and counts the gap so a missing lesson
	// is reported rather than silently absent.
	Lesson          string
	Question        string
	Answer          string
	DerivationTrail string
	Provenance      Provenance
	Anchors         []AnchorRow
}

type SeatVerdict string

const (
	VerdictConfirm SeatVerdict = "CONFIRM"
	VerdictAmend   SeatVerdict = "AMEND"
	VerdictRefute  SeatVerdict = "REFUTE"
)

type Verdict struct {
	Kind     SeatVerdict
	MapPath  string
	Evidence string
}

type NormalizedVerdictKind string

const (
	NormalizedConfirm NormalizedVerdictKind = "CONFIRM"
	NormalizedAmend   NormalizedVerdictKind = "AMEND"
	NormalizedRefute  NormalizedVerdictKind = "REFUTE"
	NormalizedUnruled NormalizedVerdictKind = "UNRULED"
)

type NormalizedVerdict struct {
	Kind     NormalizedVerdictKind
	MapPath  string
	Evidence string
}

type Census struct {
	WindowMetaCount                  int
	AgentMetaCount                   int
	PairedTranscriptCount            int
	SelectedPairedTranscriptCount    int
	OmittedPairedTranscriptCount     int
	CoverageGapCount                 int
	ExcludedOtherAgentOrInvalidCount int
	InvalidMetaCount                 int
}

type LaneMembership map[string]string

type HoldState string

const (
	HoldReady         HoldState = "READY"
	HoldZeroSurvivors HoldState = "ZERO-SURVIVORS"
	HoldZeroYield     HoldState = "ZERO-YIELD"
)

type LaneProfile struct {
	AgentType string
	Lane      string
	Path      string
	Body      string
}

type StageLayout struct {
	Root               string
	Maps               string
	Meta               string
	Paths              string
	Pin                string
	Coverage           string
	Verdicts           string
	NormalizedVerdicts string
	StructuredLog      string
	HumanLog           string
}

type RepoContext struct {
	RepoRoot string
	Organ    string
	Registry string
}

type LaneContext struct {
	AgentType string
	Lane      string
}
