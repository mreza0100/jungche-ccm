package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"hostops/pfm/internal/reload"
	"hostops/pfm/internal/resolve"
)

type reloadTargetTmux struct {
	panes []reload.Pane
}

func (tmux reloadTargetTmux) ListPanes(context.Context, string) ([]reload.Pane, error) {
	return tmux.panes, nil
}
func (reloadTargetTmux) SetRemain(context.Context, string, string, bool) error { return nil }
func (reloadTargetTmux) PaneInMode(context.Context, string, string) (bool, error) {
	return false, nil
}
func (reloadTargetTmux) CancelMode(context.Context, string, string) error { return nil }
func (reloadTargetTmux) Capture(context.Context, string, string) (string, error) {
	return "", nil
}
func (reloadTargetTmux) SendKey(context.Context, string, string, string) error { return nil }
func (reloadTargetTmux) SendLiteral(context.Context, string, string, string) error {
	return nil
}
func (reloadTargetTmux) Respawn(context.Context, string, string, string, string) error {
	return nil
}
func (reloadTargetTmux) Display(context.Context, string, string, string) error { return nil }

func TestReloadUsageIsCanonicalAndSwapIsNotMentioned(t *testing.T) {
	if !strings.Contains(reloadUsage, "usage: pfm chat reload") {
		t.Fatalf("reload usage=%q", reloadUsage)
	}
	if strings.Contains(reloadUsage, "chat swap") {
		t.Fatalf("legacy swap leaked into canonical usage: %q", reloadUsage)
	}
}

func TestReloadAcceptsBareSameAccountRequest(t *testing.T) {
	if err := validateReloadArgs(nil); err != nil {
		t.Fatalf("bare reload rejected: %v", err)
	}
}

func TestReloadTargetAcceptsRecoveredCodexSeatWithoutAmbientTmux(t *testing.T) {
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")
	identity := resolve.Identity{
		Session:    "cx-1800000000-1-1",
		SocketPath: "/jail/tmux/cx-1800000000-1-1",
		SocketName: "cx-1800000000-1-1",
		Engine:     "codex",
		ID:         "11111111-1111-4111-8111-111111111111",
		Source:     "codex-thread",
		Recovered:  true,
	}
	panes := []reload.Pane{{ID: "%0", PID: 42, CurrentPath: "/worktree"}}
	var stderr bytes.Buffer

	socket, pane, state, code := reloadTargetFromIdentity(
		context.Background(), identity, reloadTargetTmux{panes: panes}, &stderr,
	)
	if code != 0 || socket != identity.SocketPath || pane != "%0" || state.PID != 42 {
		t.Fatalf(
			"reload target=(%q,%q,%+v,%d), want recovered socket's only pane; stderr=%q",
			socket, pane, state, code, stderr.String(),
		)
	}
}
