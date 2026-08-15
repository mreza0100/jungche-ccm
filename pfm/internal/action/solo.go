package action

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"hostops/pfm/internal/gather"
)

// Solo closes every competing tmux host and stray Claude process for a
// transcript while preserving the requested live socket and its ttys.
func (executor *Executor) Solo(
	ctx context.Context,
	id, keepSocket string,
	liveAgent bool,
) error {
	if id == "" {
		return nil
	}
	if err := os.MkdirAll(executor.sidDir, 0o700); err != nil {
		return fmt.Errorf("create solo lock directory: %w", err)
	}
	sum := sha256.Sum256([]byte(id))
	lockPath := filepath.Join(
		executor.sidDir,
		fmt.Sprintf(".solo-%x.lock", sum[:12]),
	)
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("open solo lock: %w", err)
	}
	defer lockFile.Close()
	if err := flockContext(ctx, lockFile); err != nil {
		return err
	}
	defer syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)

	entries, err := os.ReadDir(executor.sidDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read solo crumbs: %w", err)
	}
	paneCrumbs := make(map[string]int)
	for _, entry := range entries {
		socket, paneID, ok := gather.ParseCrumbName(entry.Name())
		if ok && paneID != "" {
			paneCrumbs[socket]++
		}
	}
	for _, entry := range entries {
		socket, paneID, ok := gather.ParseCrumbName(entry.Name())
		if !ok || !strings.HasPrefix(socket, "cc-") ||
			(keepSocket != "" && socket == keepSocket) {
			continue
		}
		crumbPath := filepath.Join(executor.sidDir, entry.Name())
		content, err := os.ReadFile(crumbPath)
		if err != nil {
			continue
		}
		if !strings.HasSuffix(
			strings.TrimRight(string(content), "\r\n"),
			"/"+id+".jsonl",
		) {
			continue
		}
		panes, err := executor.tmux.ListPanes(ctx, socket)
		if err != nil || len(panes) == 0 {
			if err := removeFile(crumbPath); err != nil {
				return err
			}
			continue
		}
		if paneID != "" {
			if paneExists(panes, paneID) {
				fmt.Fprintf(
					executor.stderr,
					"cc: solo — closing duplicate of this chat on %s %s\n",
					socket,
					paneID,
				)
				_ = executor.tmux.KillPane(ctx, socket, paneID)
			}
		} else if paneCrumbs[socket] == 0 && len(panes) == 1 {
			fmt.Fprintf(
				executor.stderr,
				"cc: solo — closing duplicate of this chat on %s\n",
				socket,
			)
			_ = executor.tmux.KillServer(ctx, socket)
		}
		if err := removeFile(crumbPath); err != nil {
			return err
		}
	}
	if liveAgent || executor.processes == nil {
		return nil
	}
	keepTTYs := make(map[string]struct{})
	if keepSocket != "" {
		if panes, err := executor.tmux.ListPanes(ctx, keepSocket); err == nil {
			for _, pane := range panes {
				keepTTYs[strings.TrimPrefix(pane.TTY, "/dev/")] = struct{}{}
			}
		}
	}
	processes, err := executor.processes.Processes(ctx)
	if err != nil {
		return fmt.Errorf("scan solo processes: %w", err)
	}
	for _, process := range processes {
		if !isStrayClaude(process, id) {
			continue
		}
		if _, keep := keepTTYs[strings.TrimPrefix(process.TTY, "/dev/")]; keep {
			continue
		}
		if err := executor.processes.Terminate(process.PID); err == nil {
			fmt.Fprintf(
				executor.stderr,
				"cc: solo — killed stray claude %d holding %s\n",
				process.PID,
				id,
			)
		}
	}
	return nil
}

func flockContext(ctx context.Context, file *os.File) error {
	for {
		err := syscall.Flock(
			int(file.Fd()),
			syscall.LOCK_EX|syscall.LOCK_NB,
		)
		if err == nil {
			return nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			return fmt.Errorf("lock solo operation: %w", err)
		}
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func removeFile(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove solo crumb %q: %w", path, err)
	}
	return nil
}

func paneExists(panes []Pane, paneID string) bool {
	for _, pane := range panes {
		if pane.PaneID == paneID {
			return true
		}
	}
	return false
}

func isStrayClaude(process Process, id string) bool {
	if len(process.Argv) == 0 {
		return false
	}
	for _, argument := range process.Argv {
		if argument == "--bg-pty-host" {
			return false
		}
	}
	executable := process.Argv[0]
	if filepath.Base(executable) != "claude" &&
		!strings.Contains(executable, "/claude/versions/") {
		return false
	}
	for _, argument := range process.Argv {
		if strings.Contains(argument, id) {
			return true
		}
	}
	return false
}
