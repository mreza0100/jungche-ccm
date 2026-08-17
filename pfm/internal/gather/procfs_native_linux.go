//go:build linux

package gather

// nativeProcFS returns the kernel's own process table. On Linux that is /proc,
// which is also what the jail fixtures imitate, so this is the same reader the
// tests drive with PFM_PROC_ROOT pointed elsewhere.
func nativeProcFS() ProcFS { return RealProcFS{} }
