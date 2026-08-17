//go:build !darwin

package seat

import (
	"errors"
	"os"
)

// procRootUsable reports whether a real proc tree is there to read. On Linux it
// always is, outside a jail that points elsewhere.
func procRootUsable(root string) bool {
	info, err := os.Stat(root)
	return err == nil && info.IsDir()
}

// nativeProcessIdentity is unreachable where /proc exists.
func nativeProcessIdentity(int, bool) (ProcessRecord, string, error) {
	return ProcessRecord{}, "", errors.New("read process identity: /proc is unavailable")
}

// nativeProcessPIDs is unreachable where /proc exists.
func nativeProcessPIDs() ([]int, error) {
	return nil, errors.New("list processes: /proc is unavailable")
}
