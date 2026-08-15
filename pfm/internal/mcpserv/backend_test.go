package mcpserv

import (
	"testing"

	"hostops/pfm/internal/compose"
)

// TestExcludedFromChatLSKeepsBootingOffTheMCPContract is the MCP-backend half
// of the booting-row fix: a Booting row is a real live chat, but its "id" is
// a crumbless socket with no stable identity for the MCP tool contract to
// hand a caller, so chat_ls must keep excluding it exactly as it already
// excludes the two synthetic New* actions — never surfacing, never flipping
// back on by accident for an ordinary live/resumable kind.
func TestExcludedFromChatLSKeepsBootingOffTheMCPContract(t *testing.T) {
	excluded := map[compose.Kind]bool{
		compose.NewClaude:    true,
		compose.NewCodex:     true,
		compose.Booting:      true,
		compose.LiveClaude:   false,
		compose.LiveCodex:    false,
		compose.LiveSplit:    false,
		compose.Agent:        false,
		compose.ResumeClaude: false,
		compose.ResumeCodex:  false,
	}
	for kind, want := range excluded {
		if got := excludedFromChatLS(kind); got != want {
			t.Fatalf("excludedFromChatLS(%s) = %v, want %v", kind, got, want)
		}
	}
}
