package main

import (
	"testing"

	"hostops/pfm/internal/compose"
)

// TestMCPListExcludesOnlyThePickerPlaceholderRows is the F6-reworked half of
// the original TestMCPListExcludesEveryFreshAndBootingRow: excludedFromMCPList
// still drops the picker's "start a new chat" placeholder rows (New*), but no
// longer drops compose.Booting — a booting row is a real chat with a live
// process and a live tmux pane, not a placeholder, and mcp_shared.go's List
// closure now admits it with state "booting" instead (see
// TestMCPListAdmitsBootingRow in mcp_list_filter_jail_test.go for the
// end-to-end proof of that state string).
func TestMCPListExcludesOnlyThePickerPlaceholderRows(t *testing.T) {
	for _, kind := range []compose.Kind{
		compose.NewClaude,
		compose.NewCodex,
		compose.NewOpencode,
	} {
		if !excludedFromMCPList(kind) {
			t.Errorf("MCP list admitted placeholder row kind %s", kind)
		}
	}
	for _, kind := range []compose.Kind{
		compose.Booting,
		compose.LiveClaude,
		compose.LiveCodex,
		compose.ResumeClaude,
		compose.ResumeCodex,
		compose.ResumeOpencode,
	} {
		if excludedFromMCPList(kind) {
			t.Errorf("MCP list excluded real row kind %s", kind)
		}
	}
}
