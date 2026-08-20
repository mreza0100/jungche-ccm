package reload

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"hostops/pfm/internal/action"
)

func TestRunRespawnsWithConfiguredClaudePolicy(t *testing.T) {
	tmux := &fakeReloadTmux{}
	configDir := filepath.Join(t.TempDir(), "account 42")
	customBinary := "/opt/tools/claude enterprise"
	_, err := Run(
		context.Background(),
		Request{
			SocketPath:        "/tmp/tmux-1000/configured-reload",
			Pane:              "%7",
			SessionID:         "11111111-1111-4111-8111-111111111111",
			CWD:               "/jail/project",
			Account:           42,
			AccountIDs:        []int{42},
			AccountConfigDir:  configDir,
			ClaudeBinary:      customBinary,
			PromptPermissions: true,
			Cache1H:           true,
		},
		Options{SIDDir: t.TempDir(), Delay: -1, Poll: -1, ExitTries: 2},
		tmux,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for _, want := range []string{
		"CLAUDE_CONFIG_DIR=" + action.Quote(configDir),
		"ENABLE_PROMPT_CACHING_1H=1",
		action.Quote(customBinary),
	} {
		if !strings.Contains(tmux.respawn, want) {
			t.Fatalf("respawn command %q lacks configured policy %q", tmux.respawn, want)
		}
	}
	if strings.Contains(tmux.respawn, "skip-permissions") {
		t.Fatalf("prompt permission policy still armed bypass flags: %q", tmux.respawn)
	}
}
