package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"hostops/pfm/internal/headless"
)

func TestChatNameConvergesTheWindowInlineOnAProbeSocket(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	base := "/tmp/tmux-1000"
	if err := os.MkdirAll(base, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := os.MkdirTemp(base, "probe-pfm-name-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Errorf("remove probe jail: %v", err)
		}
	})

	deadRuntime := filepath.Join(root, "dead-runtime")
	if err := os.MkdirAll(deadRuntime, 0o700); err != nil {
		t.Fatal(err)
	}
	proof := exec.Command("systemctl", "--user", "show-environment")
	proof.Env = append(withoutEnv(os.Environ(), "DBUS_SESSION_BUS_ADDRESS", "XDG_RUNTIME_DIR"),
		"XDG_RUNTIME_DIR="+deadRuntime)
	if err := proof.Run(); err == nil {
		t.Fatal("probe jail can reach the user systemd bus")
	}

	socketPath := filepath.Join(root, "probe-name.sock")
	session := "probe-name"
	start := exec.Command(
		"tmux", "-S", socketPath, "-f", "/dev/null",
		"new-session", "-d", "-s", session, "-n", "before", "sleep 120",
	)
	if output, err := start.CombinedOutput(); err != nil {
		t.Fatalf("start probe server: %v: %s", err, output)
	}
	t.Cleanup(func() {
		_ = exec.Command("tmux", "-S", socketPath, "kill-server").Run()
	})

	delivered := false
	deliver := func(
		_ context.Context,
		chat headless.Chat,
		name string,
	) (int, string, error) {
		delivered = chat.Socket == socketPath && name == "after"
		return 0, "", nil
	}
	var stderr bytes.Buffer
	code := applyChatName(context.Background(), headless.Chat{
		ID:      "probe-id",
		Name:    "before",
		Socket:  socketPath,
		Session: session,
		Live:    true,
	}, "after", deliver, &stderr)
	if code != 0 || stderr.Len() != 0 || !delivered {
		t.Fatalf("chat name rc=%d delivered=%t stderr=%q", code, delivered, stderr.String())
	}
	output, err := exec.Command(
		"tmux", "-S", socketPath,
		"display-message", "-p", "-t", session, "#{window_name}",
	).Output()
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(output)); got != "after" {
		t.Fatalf("window name=%q, want after", got)
	}
}

func withoutEnv(environment []string, keys ...string) []string {
	filtered := make([]string, 0, len(environment))
	for _, entry := range environment {
		keep := true
		for _, key := range keys {
			if strings.HasPrefix(entry, key+"=") {
				keep = false
				break
			}
		}
		if keep {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}
