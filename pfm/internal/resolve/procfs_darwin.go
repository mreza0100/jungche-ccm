//go:build darwin

package resolve

import (
	"fmt"

	"golang.org/x/sys/unix"
)

type darwinProcFS struct{}

func newNativeProcFS(string) ProcFS { return darwinProcFS{} }

func (darwinProcFS) Environ(pid int) (map[string]string, error) {
	return nil, fmt.Errorf("process environment for %d is not available through sysctl", pid)
}

func (darwinProcFS) Stat(pid int) (ProcStat, error) {
	process, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil {
		return ProcStat{}, fmt.Errorf("read kern.proc.pid for %d: %w", pid, err)
	}
	return ProcStat{ParentPID: int(process.Eproc.Ppid)}, nil
}
