package inject

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type targetLock struct {
	path string
	pid  int
	now  func() time.Time
}

func acquireTargetLock(
	root, key string,
	timeout, poll, maxHold time.Duration,
) (*targetLock, error) {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("create inject lock root: %w", err)
	}
	sum := sha256.Sum256([]byte(key))
	path := filepath.Join(root, hex.EncodeToString(sum[:16])+".lock")
	pid := os.Getpid()
	deadline := time.Now().Add(timeout)
	for {
		if err := os.Mkdir(path, 0o700); err == nil {
			lock := &targetLock{path: path, pid: pid, now: time.Now}
			if err := lock.beat(); err != nil {
				_ = os.RemoveAll(path)
				return nil, err
			}
			// The settle/re-read closes chat.sh's double-steal race.
			if poll > 0 {
				time.Sleep(minDuration(poll, 50*time.Millisecond))
			}
			ownerPID, _, _ := readLockOwner(path)
			if ownerPID == pid {
				return lock, nil
			}
			continue
		} else if !errors.Is(err, fs.ErrExist) {
			return nil, fmt.Errorf("create inject target lock: %w", err)
		}

		ownerPID, epoch, ok := readLockOwner(path)
		stale := ok && (processDead(ownerPID) ||
			time.Since(time.Unix(epoch, 0)) > maxHold)
		if stale {
			_ = os.RemoveAll(path)
			continue
		}
		if !time.Now().Before(deadline) {
			return nil, fmt.Errorf("inject target lock timeout")
		}
		time.Sleep(poll)
	}
}

func (lock *targetLock) beat() error {
	ownerPID, _, ok := readLockOwner(lock.path)
	if ok && ownerPID != lock.pid {
		return nil
	}
	content := fmt.Sprintf("%d %d\n", lock.pid, lock.now().Unix())
	return os.WriteFile(filepath.Join(lock.path, "owner"), []byte(content), 0o600)
}

func (lock *targetLock) release() {
	ownerPID, _, ok := readLockOwner(lock.path)
	if ok && ownerPID == lock.pid {
		_ = os.RemoveAll(lock.path)
	}
}

func readLockOwner(path string) (pid int, epoch int64, ok bool) {
	content, err := os.ReadFile(filepath.Join(path, "owner"))
	if err != nil {
		return 0, 0, false
	}
	fields := strings.Fields(string(content))
	if len(fields) != 2 {
		return 0, 0, false
	}
	pid, err = strconv.Atoi(fields[0])
	if err != nil || pid <= 0 {
		return 0, 0, false
	}
	epoch, err = strconv.ParseInt(fields[1], 10, 64)
	return pid, epoch, err == nil
}

func processDead(pid int) bool {
	err := syscall.Kill(pid, 0)
	return errors.Is(err, syscall.ESRCH)
}

func minDuration(left, right time.Duration) time.Duration {
	if left < right {
		return left
	}
	return right
}
