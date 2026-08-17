//go:build linux

package action

import "golang.org/x/sys/unix"

// The ioctl request that reads and writes a terminal's line settings is spelled
// differently per kernel — it encodes the direction and the size of the struct
// it carries, so the NUMBER is not portable even though the struct is. Linux
// calls them TCGETS/TCSETS; the BSDs (macOS included) call them
// TIOCGETA/TIOCSETA. Naming them once here is what lets gate.go stay one file.
const (
	getTermios = unix.TCGETS
	setTermios = unix.TCSETS
)
