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
		"chat_group_create",
		"chat_group_invite",
		"chat_group_ls",
		"chat_group_read",
		"chat_group_send",
		"chat_group_subscribe",
		"chat_load",
	} {
		if !slices.Contains(tools, name) {
			t.Errorf("chat MCP tool roster lacks %q", name)
		}
	}
}
