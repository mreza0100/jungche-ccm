package seat

import (
	"bufio"
	"context"
	"errors"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestProcJailerKillsARealStubbornDescendantAndItsPIDDisappears(t *testing.T) {
	command := exec.Command(
		"sh",
		"-c",
		`trap '' TERM; (trap '' TERM; while :; do sleep 30; done) & printf '%s\n' "$!"; wait`,
	)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	rootPID := command.Process.Pid
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	waited := false
	defer func() {
		// The negative target is the exact group this test created.
		_ = syscall.Kill(-rootPID, syscall.SIGKILL)
		if !waited {
			select {
			case <-done:
			case <-time.After(5 * time.Second):
			}
		}
	}()

	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatalf("read stubborn descendant pid: %v", err)
	}
	childPID, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || childPID <= 1 {
		t.Fatalf("stubborn descendant pid = %q", line)
	}

	jailer := ProcJailer{Root: "/proc"}
	jail, err := jailer.Capture(context.Background(), rootPID)
	if err != nil {
		t.Fatalf("Capture() error = %v", err)
	}
	if jail.GroupID != rootPID {
		t.Fatalf("captured group = %d, want root %d", jail.GroupID, rootPID)
	}
	if err := syscall.Kill(-jail.GroupID, syscall.SIGTERM); err != nil {
		t.Fatalf("send TERM to stubborn group: %v", err)
	}
	time.Sleep(30 * time.Millisecond)
	if err := syscall.Kill(childPID, 0); err != nil {
		t.Fatalf("descendant did not ignore TERM: %v", err)
	}

	cleanupContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := jailer.Kill(cleanupContext, jail); err != nil {
		t.Fatalf("Kill() error = %v", err)
	}
	select {
	case <-done:
		waited = true
	case <-cleanupContext.Done():
		t.Fatalf("root pid %d survived cleanup: %v", rootPID, cleanupContext.Err())
	}
	for {
		err := syscall.Kill(childPID, 0)
		if errors.Is(err, syscall.ESRCH) {
			break
		}
		if err != nil {
			t.Fatalf("probe descendant pid %d: %v", childPID, err)
		}
		select {
		case <-cleanupContext.Done():
			t.Fatalf("stubborn descendant pid %d still exists after cleanup", childPID)
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestProcJailerRefusesAReusedRootIdentity(t *testing.T) {
	jailer := ProcJailer{Root: "/proc"}
	command := exec.Command("sleep", "30")
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		_ = command.Wait()
	}()
	jail, err := jailer.Capture(context.Background(), command.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	jail.RootStartTicks++
	err = jailer.Kill(context.Background(), jail)
	if err == nil || !strings.Contains(err.Error(), "changed before cleanup") {
		t.Fatalf("Kill() error = %v, want recycled-pid refusal", err)
	}
	if signalErr := syscall.Kill(command.Process.Pid, 0); signalErr != nil {
		t.Fatalf("identity mismatch killed unrelated process: %v", signalErr)
	}
}
