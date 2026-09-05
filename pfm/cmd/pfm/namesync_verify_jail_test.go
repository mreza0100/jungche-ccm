package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"hostops/pfm/internal/gather"
	"hostops/pfm/internal/paths"
)

// name-sync reports what it ACHIEVED, not what it attempted. The fixture puts
// one window in each state on a probe server: one whose name matches what was
// asked for, one a second writer took back, and one whose server is gone.
func TestNameSyncCountsOnlyVerifiedWindowsAsConverged(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	base := filepath.Join(os.TempDir(), "tmux-"+strconv.Itoa(os.Getuid()))
	if err := os.MkdirAll(base, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := os.MkdirTemp(base, "probe-pfm-verify-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Errorf("remove probe jail: %v", err)
		}
	})

	const socket = "probe-verify"
	socketPath := filepath.Join(root, socket)
	start := exec.Command(
		"tmux", "-S", socketPath, "-f", "/dev/null",
		"new-session", "-d", "-s", socket, "-n", "CONVERGED", "sleep 120",
	)
	if output, err := start.CombinedOutput(); err != nil {
		t.Fatalf("start probe server: %v: %s", err, output)
	}
	t.Cleanup(func() {
		_ = exec.Command("tmux", "-S", socketPath, "kill-server").Run()
	})
	if output, err := exec.Command(
		"tmux", "-S", socketPath, "new-window", "-d", "-n", "TAKEN-BACK", "sleep 120",
	).CombinedOutput(); err != nil {
		t.Fatalf("create second window: %v: %s", err, output)
	}
	windowIDs := strings.Fields(readProbe(t, socketPath, "list-windows", "-F", "#{window_id}"))
	if len(windowIDs) != 2 {
		t.Fatalf("probe server has windows %v, want two", windowIDs)
	}

	runtime := commandRuntime{Paths: paths.Values{TmuxDir: root}}
	var stderr bytes.Buffer
	converged, unverified := verifyRenames(context.Background(), runtime, []gather.WindowRename{
		{Socket: socket, WindowID: windowIDs[0], TargetName: "CONVERGED"},
		{Socket: socket, WindowID: windowIDs[1], TargetName: "WANTED"},
		{Socket: "probe-verify-gone", WindowID: "@0", TargetName: "WANTED"},
	}, &stderr)

	if converged != 1 {
		t.Fatalf("converged = %d, want 1 — only the window whose name reads back counts", converged)
	}
	if unverified != 2 {
		t.Fatalf("unverified = %d, want 2", unverified)
	}
	report := stderr.String()
	if !strings.Contains(report, `wanted "WANTED", reads "TAKEN-BACK" after rename`) {
		t.Fatalf("report does not name the value read back:\n%s", report)
	}
	if !strings.Contains(report, "could not be read back after rename") {
		t.Fatalf("report does not name the unreadable window:\n%s", report)
	}
}

func readProbe(t *testing.T, socketPath string, arguments ...string) string {
	t.Helper()
	command := exec.Command("tmux", append([]string{"-S", socketPath}, arguments...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("tmux %s: %v: %s", strings.Join(arguments, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}
