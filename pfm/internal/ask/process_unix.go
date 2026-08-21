//go:build linux || darwin

package ask

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// configureBoundedCommand puts the engine and every helper it starts in one
// process group. Killing only the CLI process can leave a child holding the
// stdout/stderr pipes open past the ask timeout, so cancellation owns the
// whole group and WaitDelay bounds a pathological inherited descriptor.
func configureBoundedCommand(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error {
		if command.Process == nil {
			return os.ErrProcessDone
		}
		err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	command.WaitDelay = 500 * time.Millisecond
}
