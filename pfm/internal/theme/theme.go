// Package theme contains pfm's embedded visual palettes.
package theme

import (
	"fmt"
	"os"
)

// Palette is the color vocabulary shared by the picker and status panels.
// The extra fields preserve the non-row literals that were already part of
// the default renderer, so a palette change cannot leave a stray hardcoded
// color behind.
type Palette struct {
	CodexRow    string
	ClaudeRow   string
	AgentRow    string
	StatsClaude string
	StatsCPU    string
	StatsRAM    string
	Accent      string
	Muted       string
	Warn        string

	Header      string
	GroupA      string
	GroupB      string
	Border      string
	Selected    string
	Dim         string
	StatsHeader string
	StatsMemory string
	StatsToken  string
	StatsGear   string
	StatsImage  string
	Label       string
}

var defaultPalette = Palette{
	CodexRow:    "#e879f9",
	ClaudeRow:   "#5eead4",
	AgentRow:    "#fb923c",
	StatsClaude: "#5eead4",
	StatsCPU:    "#4ade80",
	StatsRAM:    "#60a5fa",
	Accent:      "#22d3ee",
	Muted:       "#94a3b8",
	Warn:        "#fde047",
	Header:      "#ffffff",
	GroupA:      "#5eead4",
	GroupB:      "#7dd3fc",
	Border:      "#64748b",
	Selected:    "#334155",
	Dim:         "#94a3b8",
	StatsHeader: "#22d3ee",
	StatsMemory: "#60a5fa",
	StatsToken:  "#facc15",
	StatsGear:   "#fb923c",
	StatsImage:  "#c084fc",
	Label:       "#67e8f9",
}

var tokyoNightPalette = Palette{
	CodexRow:    "#bb9af7",
	ClaudeRow:   "#73daca",
	AgentRow:    "#ff9e64",
	StatsClaude: "#73daca",
	StatsCPU:    "#9ece6a",
	StatsRAM:    "#7aa2f7",
	Accent:      "#7dcfff",
	Muted:       "#565f89",
	Warn:        "#e0af68",
	Header:      "#c0caf5",
	GroupA:      "#73daca",
	GroupB:      "#7dcfff",
	Border:      "#3b4261",
	Selected:    "#292e42",
	Dim:         "#565f89",
	StatsHeader: "#7dcfff",
	StatsMemory: "#7aa2f7",
	StatsToken:  "#e0af68",
	StatsGear:   "#ff9e64",
	StatsImage:  "#bb9af7",
	Label:       "#2ac3de",
}

// Load returns an embedded palette. Unknown names intentionally degrade to
// the default while making the repair visible to the operator.
func Load(name string) Palette {
	switch name {
	case "", "default":
		return defaultPalette
	case "tokyo-night":
		return tokyoNightPalette
	default:
		fmt.Fprintf(os.Stderr, "pfm: unknown theme %q; using default\n", name)
		return defaultPalette
	}
}
