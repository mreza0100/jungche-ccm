//go:build darwin || dragonfly || freebsd || netbsd || openbsd

package action

import "golang.org/x/sys/unix"

// See termios_linux.go for why this constant is platform-split at all. The BSD
// spelling is TIOCGETA/TIOCSETA. A GOOS with neither pair fails to build here
// with "undefined: getTermios", which is the correct outcome: the open gate puts
// a real terminal into one-key mode, and a platform whose ioctl numbers we have
// not confirmed must not be guessed at.
const (
	getTermios = unix.TIOCGETA
	setTermios = unix.TIOCSETA
)
