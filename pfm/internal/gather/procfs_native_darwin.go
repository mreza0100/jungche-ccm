//go:build darwin

package gather

// nativeProcFS returns the kernel's own process table. macOS has no /proc at
// all, so this is sysctl — see procfs_darwin.go for what that can and cannot
// answer.
func nativeProcFS() ProcFS { return NewDarwinProcFS() }
