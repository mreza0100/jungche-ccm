package mcpserv

import (
	"slices"
	"testing"
)

func TestChatMCPAdvertisesEveryNativeWorkflowTool(t *testing.T) {
	tools := ToolNames()
	for _, name := range []string{
		"chat_branch",
		"chat_goal",
		"chat_load",
	} {
		if !slices.Contains(tools, name) {
			t.Errorf("chat MCP tool roster lacks %q", name)
		}
	}
}

// TestChatMCPRosterNeverAdvertisesRetiredGroupTools asserts the retired
// chat-group tools' ABSENCE explicitly. A shrunken expected-list count alone
// is a coincidence detector — it would also pass if a later change renamed
// or dropped an unrelated tool by mistake. Naming every retired tool here
// means the roster can only pass by genuinely never serving one of them.
func TestChatMCPRosterNeverAdvertisesRetiredGroupTools(t *testing.T) {
	tools := ToolNames()
	for _, name := range []string{
		"chat_group_create",
		"chat_group_subscribe",
		"chat_group_invite",
		"chat_group_send",
		"chat_group_read",
		"chat_group_ls",
	} {
		if slices.Contains(tools, name) {
			t.Errorf("chat MCP tool roster still advertises retired tool %q", name)
		}
	}
}
