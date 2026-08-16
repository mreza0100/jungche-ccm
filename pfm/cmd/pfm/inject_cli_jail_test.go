package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const injectCLIUI = `import os, sys, tty
tty.setraw(0)
buf = bytearray()
sys.stdout.write("❯ ")
sys.stdout.flush()
while True:
    ch = os.read(0, 1)
    if not ch:
        break
    if ch == b"\x13":
        continue
    if ch in (b"\r", b"\n"):
        if buf:
            text = bytes(buf).decode("utf-8", "replace")
            sys.stdout.write("\r\nUSER:" + text + "\r\n❯ ")
            sys.stdout.flush()
            buf.clear()
        continue
    buf.extend(ch)
    sys.stdout.buffer.write(ch)
    sys.stdout.flush()
`

func TestChatInjectResolvesUnindexedLiveSessionAcrossProbeSockets(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux binary is not installed")
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 binary is not installed")
	}
	const tmuxRoot = "/tmp/tmux-1000"
	if err := os.MkdirAll(tmuxRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := os.MkdirTemp(tmuxRoot, "probe-pfm-cli-inject-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(root)
	})
	home := filepath.Join(root, "home")
	tmuxDir := filepath.Join(root, "tmux-"+strconv.Itoa(os.Getuid()))
	for _, directory := range []string{home, tmuxDir, filepath.Join(root, "sid"), filepath.Join(root, "claude"), filepath.Join(root, "codex"), filepath.Join(root, "proc")} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_TMPDIR", root)
	t.Setenv("TMPDIR", root)
	t.Setenv("PFM_HOME", home)
	t.Setenv("PFM_DB", filepath.Join(root, "fleet.db"))
	t.Setenv("PFM_SHARED_DB", filepath.Join(root, "shared.db"))
	t.Setenv("PFM_SID_DIR", filepath.Join(root, "sid"))
	t.Setenv("PFM_CLAUDE_ROOTS", filepath.Join(root, "claude"))
	t.Setenv("PFM_CODEX_ROOT", filepath.Join(root, "codex"))
	t.Setenv("PFM_TMUX_DIR", tmuxDir)
	t.Setenv("PFM_PROC_ROOT", filepath.Join(root, "proc"))
	t.Setenv("PFM_TMUX_CONF", "/dev/null")
	t.Setenv("CHAT_INJECT_POLL", "0.01")
	t.Setenv("CHAT_INJECT_ENTER_GAP", "0.01")
	t.Setenv("CHAT_INJECT_ENTER_SETTLE", "0.05")
	t.Setenv("CHAT_INJECT_PROOF_SETTLE", "0.05")
	// Deliberately do not enable PFM_TEST_PROBE_SOCKETS: the fleet/store scan
	// must not know this seat. The inject resolver independently scans every
	// socket and is the behavior under test.

	script := filepath.Join(root, "ui.py")
	if err := os.WriteFile(script, []byte(injectCLIUI), 0o700); err != nil {
		t.Fatal(err)
	}
	socket := "probe-inj-direct-sock"
	session := "probe-inj-direct-session"
	command := exec.Command("tmux", "-f", "/dev/null", "-L", socket, "new-session", "-d", "-s", session, "python3", script)
	command.Env = append(os.Environ(), "TMUX=", "TMUX_TMPDIR="+root, "HOME="+home)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("start probe tmux: %v: %s", err, output)
	}
	t.Cleanup(func() {
		kill := exec.Command("tmux", "-L", socket, "kill-server")
		kill.Env = append(os.Environ(), "TMUX=", "TMUX_TMPDIR="+root, "HOME="+home)
		_ = kill.Run()
	})

	deadline := time.Now().Add(5 * time.Second)
	for {
		capture := exec.Command("tmux", "-L", socket, "capture-pane", "-t", session, "-p", "-J")
		capture.Env = append(os.Environ(), "TMUX=", "TMUX_TMPDIR="+root, "HOME="+home)
		output, captureErr := capture.Output()
		if captureErr == nil && strings.Contains(string(output), "❯") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("probe UI did not become ready: %v", captureErr)
		}
		time.Sleep(20 * time.Millisecond)
	}

	var stdout, stderr bytes.Buffer
	message := fmt.Sprintf("direct ladder probe %d", time.Now().UnixNano())
	code := run([]string{"chat", "inject", session, message}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("inject exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "injected LIVE") {
		t.Fatalf("receipt did not prove live delivery: %q", stdout.String())
	}
	verify := exec.Command("tmux", "-L", socket, "capture-pane", "-t", session, "-p", "-J", "-S", "-")
	verify.Env = append(os.Environ(), "TMUX=", "TMUX_TMPDIR="+root, "HOME="+home)
	verified, err := verify.Output()
	if err != nil || !strings.Contains(string(verified), "USER:"+message) {
		t.Fatalf("live body missing after receipt: err=%v capture=%q", err, verified)
	}

	selectorScript := filepath.Join(root, "selector.py")
	if err := os.WriteFile(selectorScript, []byte("import time\nprint('Question\\n❯ 1. Allow\\n  2. Deny', flush=True)\ntime.sleep(30)\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	selectorSocket := "probe-inj-selector-sock"
	selectorSession := "probe-inj-selector-session"
	selectorStart := exec.Command("tmux", "-f", "/dev/null", "-L", selectorSocket, "new-session", "-d", "-s", selectorSession, "python3", selectorScript)
	selectorStart.Env = append(os.Environ(), "TMUX=", "TMUX_TMPDIR="+root, "HOME="+home)
	if output, err := selectorStart.CombinedOutput(); err != nil {
		t.Fatalf("start selector probe: %v: %s", err, output)
	}
	t.Cleanup(func() {
		kill := exec.Command("tmux", "-L", selectorSocket, "kill-server")
		kill.Env = append(os.Environ(), "TMUX=", "TMUX_TMPDIR="+root, "HOME="+home)
		_ = kill.Run()
	})
	time.Sleep(100 * time.Millisecond)
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"chat", "inject", selectorSession, "must not type"}, &stdout, &stderr)
	if code != codeUndelivered || stdout.Len() != 0 || !strings.Contains(stderr.String(), "OPEN selector") {
		t.Fatalf("selector exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	fileSocket := "probe-cx-inj-file-sock"
	fileSession := "probe-inj-file-session"
	fileStart := exec.Command("tmux", "-f", "/dev/null", "-L", fileSocket, "new-session", "-d", "-s", fileSession, "python3", script)
	fileStart.Env = append(os.Environ(), "TMUX=", "TMUX_TMPDIR="+root, "HOME="+home)
	if output, err := fileStart.CombinedOutput(); err != nil {
		t.Fatalf("start file-backed probe: %v: %s", err, output)
	}
	t.Cleanup(func() {
		kill := exec.Command("tmux", "-L", fileSocket, "kill-server")
		kill.Env = append(os.Environ(), "TMUX=", "TMUX_TMPDIR="+root, "HOME="+home)
		_ = kill.Run()
	})
	t.Setenv("PFM_TEST_PROBE_SOCKETS", "1")
	longBody := "file-start " + strings.Repeat("0123456789abcdef", 140) + " file-end"
	messagePath := filepath.Join(root, "long-message.txt")
	if err := os.WriteFile(messagePath, []byte(longBody), 0o600); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"chat", "inject", "--file", messagePath, fileSession}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "FILE-BACKED") {
		t.Fatalf("file-backed exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	fileCapture := exec.Command("tmux", "-L", fileSocket, "capture-pane", "-t", fileSession, "-p", "-J", "-S", "-")
	fileCapture.Env = append(os.Environ(), "TMUX=", "TMUX_TMPDIR="+root, "HOME="+home)
	fileOutput, err := fileCapture.Output()
	if err != nil || !strings.Contains(string(fileOutput), "file-start") || !strings.Contains(string(fileOutput), "file-end") {
		t.Fatalf("file-backed body missing: err=%v capture=%q", err, fileOutput)
	}
}

func TestChatInjectResumeLadderPathSessionAndExcerpt(t *testing.T) {
	const id = "11111111-2222-4333-8444-555555555555"
	for _, testCase := range []struct {
		name   string
		target func(string) string
	}{
		{name: "transcript path", target: func(path string) string { return path }},
		{name: "session id", target: func(string) string { return id }},
		{name: "distinctive excerpt", target: func(string) string {
			return "distinctive transcript sentence for the resume ladder"
		}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			root := t.TempDir()
			home := filepath.Join(root, "home")
			project := filepath.Join(root, "claude", "project")
			for _, directory := range []string{home, project, filepath.Join(root, "tmux"), filepath.Join(root, "codex"), filepath.Join(root, "proc")} {
				if err := os.MkdirAll(directory, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			transcript := filepath.Join(project, id+".jsonl")
			line := fmt.Sprintf(
				`{"type":"user","uuid":"aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee","sessionId":%q,"cwd":"/jail/project","version":"1","gitBranch":"","timestamp":"2026-08-16T00:00:00.000Z","message":{"role":"user","content":"distinctive transcript sentence for the resume ladder"}}`+"\n",
				id,
			)
			if err := os.WriteFile(transcript, []byte(line), 0o600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("HOME", home)
			t.Setenv("TMUX", "")
			t.Setenv("PFM_HOME", home)
			t.Setenv("PFM_DB", filepath.Join(root, "fleet.db"))
			t.Setenv("PFM_SHARED_DB", filepath.Join(root, "shared.db"))
			t.Setenv("PFM_SID_DIR", filepath.Join(root, "sid"))
			t.Setenv("PFM_CLAUDE_ROOTS", filepath.Join(root, "claude"))
			t.Setenv("PFM_CODEX_ROOT", filepath.Join(root, "codex"))
			t.Setenv("PFM_TMUX_DIR", filepath.Join(root, "tmux"))
			t.Setenv("PFM_PROC_ROOT", filepath.Join(root, "proc"))
			var stdout, stderr bytes.Buffer
			code := run(
				[]string{"chat", "inject", testCase.target(transcript), "resume ladder message"},
				&stdout,
				&stderr,
			)
			if code != 0 {
				t.Fatalf("inject exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			if !strings.Contains(stdout.String(), "RESUME") {
				t.Fatalf("receipt did not name RESUME: %q", stdout.String())
			}
			raw, err := os.ReadFile(transcript)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(raw), "resume ladder message") || strings.Count(string(raw), "\n") != 2 {
				t.Fatalf("transcript append = %q", raw)
			}
			backups, err := filepath.Glob(filepath.Join(home, ".claude-sessions", ".chat-inject-backups", "*.jsonl"))
			if err != nil || len(backups) != 1 {
				t.Fatalf("backups=%v err=%v", backups, err)
			}
		})
	}
}

func TestChatInjectResumeRefusesAProcessHeldSessionAsDead(t *testing.T) {
	const id = "99999999-2222-4333-8444-555555555555"
	root := t.TempDir()
	home := filepath.Join(root, "home")
	project := filepath.Join(root, "claude", "project")
	process := filepath.Join(root, "proc", "4242")
	for _, directory := range []string{home, project, process, filepath.Join(root, "tmux"), filepath.Join(root, "codex")} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	transcript := filepath.Join(project, id+".jsonl")
	line := fmt.Sprintf(`{"type":"user","uuid":"aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee","sessionId":%q,"message":{"role":"user","content":"held"}}`+"\n", id)
	if err := os.WriteFile(transcript, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(process, "environ"), []byte("CLAUDE_CODE_SESSION_ID="+id+"\x00"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("TMUX", "")
	t.Setenv("PFM_HOME", home)
	t.Setenv("PFM_DB", filepath.Join(root, "fleet.db"))
	t.Setenv("PFM_SHARED_DB", filepath.Join(root, "shared.db"))
	t.Setenv("PFM_SID_DIR", filepath.Join(root, "sid"))
	t.Setenv("PFM_CLAUDE_ROOTS", filepath.Join(root, "claude"))
	t.Setenv("PFM_CODEX_ROOT", filepath.Join(root, "codex"))
	t.Setenv("PFM_TMUX_DIR", filepath.Join(root, "tmux"))
	t.Setenv("PFM_PROC_ROOT", filepath.Join(root, "proc"))
	var stdout, stderr bytes.Buffer
	code := run([]string{"chat", "inject", id, "must not append"}, &stdout, &stderr)
	if code != codeDeadChat || stdout.Len() != 0 || !strings.Contains(stderr.String(), "live without a resolvable tmux pane") {
		t.Fatalf("dead guard exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	raw, err := os.ReadFile(transcript)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != line {
		t.Fatalf("dead guard mutated transcript: %q", raw)
	}
}
