package dream

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// acquireRunnerLock serializes every night and apply for one organ. The lock
// file remains as organ-local evidence; releasing it only drops the advisory
// lock and closes the descriptor.
func acquireRunnerLock(organRoot string) (func() error, error) {
	path := filepath.Join(organRoot, "dreamer", "runner.lock")
	descriptor, err := syscall.Open(
		path,
		syscall.O_CREAT|syscall.O_RDWR|syscall.O_CLOEXEC|syscall.O_NOFOLLOW,
		0o600,
	)
	if err != nil {
		return nil, fmt.Errorf("open organ runner lock %s: %w", path, err)
	}
	file := os.NewFile(uintptr(descriptor), path)
	closeOnError := func(cause error) (func() error, error) {
		return nil, errors.Join(cause, file.Close())
	}
	var stat syscall.Stat_t
	if err := syscall.Fstat(descriptor, &stat); err != nil {
		return closeOnError(fmt.Errorf("inspect organ runner lock %s: %w", path, err))
	}
	if stat.Mode&syscall.S_IFMT != syscall.S_IFREG || int(stat.Uid) != os.Getuid() {
		return closeOnError(fmt.Errorf("organ runner lock is not a regular caller-owned file: %s", path))
	}
	if err := syscall.Fchmod(descriptor, 0o600); err != nil {
		return closeOnError(fmt.Errorf("set organ runner lock mode %s: %w", path, err))
	}
	if err := syscall.Flock(descriptor, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return closeOnError(fmt.Errorf("another dreamer process holds the organ runner lock: %s", path))
		}
		return closeOnError(fmt.Errorf("lock organ runner file %s: %w", path, err))
	}
	released := false
	return func() error {
		if released {
			return nil
		}
		released = true
		unlockErr := syscall.Flock(descriptor, syscall.LOCK_UN)
		closeErr := file.Close()
		if unlockErr != nil {
			unlockErr = fmt.Errorf("unlock organ runner file %s: %w", path, unlockErr)
		}
		if closeErr != nil {
			closeErr = fmt.Errorf("close organ runner file %s: %w", path, closeErr)
		}
		return errors.Join(unlockErr, closeErr)
	}, nil
}
