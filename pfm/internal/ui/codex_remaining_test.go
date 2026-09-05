package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	pfmengine "hostops/pfm/internal/engine"
	pfmstats "hostops/pfm/internal/stats"
)

func TestCodexLimitsShowRemainingWeeklyQuota(t *testing.T) {
	for _, width := range []int{38, 78, 118} {
		for _, used := range []float64{0, 61, 100} {
			t.Run(fmt.Sprintf("width%d_used%.0f", width, used), func(t *testing.T) {
				model := NewModel(fixtureSnapshot(120))
				model.stats = pfmstats.Snapshot{Limits: []pfmstats.AccountLimits{{
					Engine: pfmengine.Codex, Label: "Codex 1",
					Windows: []pfmstats.Window{{Name: "7d", UsedPct: used}},
				}}}
				plain := ansi.Strip(strings.Join(model.renderLimitCards(width), "\n"))
				want := fmt.Sprintf("%.0f%% left", 100-used)
				if !strings.Contains(plain, want) || !strings.Contains(plain, "7d") || strings.Contains(plain, "5h") || strings.Contains(plain, "% used") {
					t.Fatalf("weekly Codex quota must show %s, not used or an invented 5h window:\n%s", want, plain)
				}
				if used == 100 && (strings.Contains(plain, "█") || strings.Contains(plain, "FULL")) {
					t.Fatalf("exhausted remaining quota has a filled bar:\n%s", plain)
				}
				if used == 0 && !strings.Contains(plain, "█") {
					t.Fatalf("untouched remaining quota has an empty bar:\n%s", plain)
				}
			})
		}
	}
}
