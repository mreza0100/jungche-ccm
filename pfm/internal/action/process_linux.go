//go:build linux

package action

import (
	"context"
	"errors"
)

// nativeProcesses is unreachable on Linux, where /proc always exists. It stays
// defined so process.go needs no build tag of its own.
func nativeProcesses(context.Context) ([]Process, error) {
	return nil, errors.New("read process table: /proc is unavailable")
}
