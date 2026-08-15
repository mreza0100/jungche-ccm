package seat

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

// ProcessGroupJail is the identity of the process group tmux created for one
// pane. RootStartTicks prevents a recycled PID from turning a delayed cleanup
// into a signal aimed at an unrelated process.
type ProcessGroupJail struct {
	RootPID        int
	RootStartTicks uint64
	GroupID        int
	SessionID      int
}

func (jail ProcessGroupJail) valid() bool {
	return jail.RootPID > 1 && jail.RootStartTicks > 0 &&
		jail.GroupID == jail.RootPID && jail.SessionID > 0
}

// ProcessJailer captures and terminates the operating-system containment
// boundary around a pane. It is separate from tmux cleanup: killing a server
// is not proof that a signal-ignoring descendant exited.
type ProcessJailer interface {
	Capture(context.Context, int) (ProcessGroupJail, error)
	Kill(context.Context, ProcessGroupJail) error
}

// ProcJailer is the Linux production implementation. Root is injected for
// deterministic identity tests; live runners receive paths.Resolve().ProcRoot.
type ProcJailer struct {
	Root string
}

func (jailer ProcJailer) Capture(
	ctx context.Context,
	rootPID int,
) (ProcessGroupJail, error) {
	if err := ctx.Err(); err != nil {
		return ProcessGroupJail{}, err
	}
	root, err := jailer.procRoot()
	if err != nil {
		return ProcessGroupJail{}, err
	}
	record, _, err := readProcessIdentity(root, rootPID, false)
	if err != nil {
		return ProcessGroupJail{}, fmt.Errorf("capture pane process group: %w", err)
	}
	jail := ProcessGroupJail{
		RootPID:        record.PID,
		RootStartTicks: record.StartTicks,
		GroupID:        record.ProcessGroupID,
		SessionID:      record.SessionID,
	}
	if !jail.valid() {
		return ProcessGroupJail{}, fmt.Errorf(
			"pane root %d is not a dedicated process-group leader (group=%d session=%d)",
			record.PID,
			record.ProcessGroupID,
			record.SessionID,
		)
	}
	return jail, nil
}

func (jailer ProcJailer) Kill(
	ctx context.Context,
	jail ProcessGroupJail,
) error {
	if !jail.valid() {
		return errors.New("refuse to kill an unproven pane process group")
	}
	root, err := jailer.procRoot()
	if err != nil {
		return err
	}
	live, err := inspectJailedGroup(ctx, root, jail)
	if err != nil {
		return err
	}
	if !live {
		return nil
	}
	if err := syscall.Kill(-jail.GroupID, syscall.SIGKILL); err != nil &&
		!errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("signal pane process group %d: %w", jail.GroupID, err)
	}

	// SIGKILL cannot be ignored, but wait until no non-zombie member remains
	// so returning from cleanup is itself the proof that descendants stopped.
	for {
		live, err = inspectJailedGroup(ctx, root, jail)
		if err != nil {
			return err
		}
		if !live {
			return nil
		}
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("wait for pane process group %d to exit: %w", jail.GroupID, ctx.Err())
		case <-timer.C:
		}
	}
}

func (jailer ProcJailer) procRoot() (string, error) {
	root := filepath.Clean(jailer.Root)
	if jailer.Root == "" || !filepath.IsAbs(root) {
		return "", fmt.Errorf("proc root must be an absolute path, got %q", jailer.Root)
	}
	return root, nil
}

// inspectJailedGroup proves the group still belongs to the captured pane
// before it can be signaled. A missing leader is allowed only while surviving
// members still carry the captured session; that is the stubborn-descendant
// case the jail exists to clean.
func inspectJailedGroup(
	ctx context.Context,
	root string,
	jail ProcessGroupJail,
) (bool, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return false, fmt.Errorf("enumerate proc for pane process group: %w", err)
	}
	foundRoot := false
	live := false
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}
		record, state, err := readProcessIdentity(root, pid, false)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return false, fmt.Errorf("inspect process %d during pane cleanup: %w", pid, err)
		}
		if pid == jail.RootPID {
			foundRoot = true
			if record.StartTicks != jail.RootStartTicks ||
				record.ProcessGroupID != jail.GroupID ||
				record.SessionID != jail.SessionID {
				return false, fmt.Errorf("pane root pid %d changed before cleanup", jail.RootPID)
			}
		}
		if record.ProcessGroupID != jail.GroupID {
			continue
		}
		if record.SessionID != jail.SessionID {
			return false, fmt.Errorf(
				"process group %d was reused by session %d before cleanup",
				jail.GroupID,
				record.SessionID,
			)
		}
		if state != "Z" && state != "X" {
			live = true
		}
	}
	if foundRoot || live {
		return live, nil
	}
	return false, nil
}
