//go:build linux || darwin

package harvestpy

import (
	"errors"
	"os/exec"
	"syscall"
)

// configureBrowserCommand puts the browser worker and everything it spawns
// (patchright's node driver, Chrome) in one process group. Killing only the
// direct python child on cancellation would leave Chrome reparented and alive
// — one leaked Chrome per cancelled fetch.
func configureBrowserCommand(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killBrowserProcessGroup SIGKILLs the worker's whole process group; ESRCH
// means the group is already gone.
func killBrowserProcessGroup(pid int) error {
	err := syscall.Kill(-pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}
