package shared

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

// carrierLockSuffix and carrierTempPrefix reproduce cc-db.sh's own filenames.
// The lock is `9>"$LEG_HID.lock"` (cc-db.sh:120, :127) and the rewrite scratch
// file is `"$LEG_HID.n.$$"` (cc-db.sh:125) — same directory, same shape, so a
// half-finished Go rewrite and a half-finished bash rewrite are the same
// leftover to a human reading the directory.
const (
	carrierLockSuffix = ".lock"
	carrierTempInfix  = ".n."
)

// withCarrierLock runs fn holding an exclusive flock on <carrier>.lock, the
// same descriptor cc-db.sh's _leg_add/_leg_del take (cc-db.sh:118, :124).
//
// A failure to lock is not a failure to write: cc-db.sh writes `flock 9 ||
// true` because flock is absent on macOS and a lock that cannot be taken must
// never stop a hide from being recorded. This mirrors that exactly.
func withCarrierLock(carrier string, fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(carrier), 0o700); err != nil {
		return fmt.Errorf("create carrier directory: %w", err)
	}
	lock, err := os.OpenFile(
		carrier+carrierLockSuffix,
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC,
		0o600,
	)
	if err != nil {
		return fn()
	}
	defer lock.Close()
	if flockErr := syscall.Flock(
		int(lock.Fd()),
		syscall.LOCK_EX,
	); flockErr == nil {
		defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	}
	return fn()
}

// carrierAdd is _leg_add (cc-db.sh:116-121): under the lock, append the id
// unless the file already holds that exact line. `grep -qxF` is a whole-line
// fixed-string match, so the Go comparison is on the trimmed line, not a
// substring.
func carrierAdd(carrier, id string) error {
	if id == "" {
		return nil
	}
	return withCarrierLock(carrier, func() error {
		present, err := carrierHas(carrier, id)
		if err != nil {
			return err
		}
		if present {
			return nil
		}
		file, err := os.OpenFile(
			carrier,
			os.O_WRONLY|os.O_CREATE|os.O_APPEND,
			0o600,
		)
		if err != nil {
			return fmt.Errorf("append to carrier file: %w", err)
		}
		if _, err := file.WriteString(id + "\n"); err != nil {
			_ = file.Close()
			return fmt.Errorf("append to carrier file: %w", err)
		}
		return file.Close()
	})
}

// carrierDelete is _leg_del (cc-db.sh:122-128): a missing file is success, and
// the rewrite goes to <carrier>.n.<pid> and is renamed over the original, so a
// reader never sees a truncated hide list.
func carrierDelete(carrier, id string) error {
	if id == "" {
		return nil
	}
	if _, err := os.Stat(carrier); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return withCarrierLock(carrier, func() error {
		ids, err := readCarrier(carrier)
		if err != nil {
			return err
		}
		kept := make([]string, 0, len(ids))
		for _, existing := range ids {
			if existing != id {
				kept = append(kept, existing)
			}
		}
		if len(kept) == len(ids) {
			return nil
		}
		return rewriteCarrier(carrier, kept)
	})
}

// carrierRewrite replaces the whole file with ids, under the same lock and the
// same scratch-then-rename dance _leg_del uses. `legacy export` is its only
// caller: no ordinary hide or unhide rewrites the carrier wholesale, because a
// whole-file rewrite is exactly the read-modify-write that lost hides before
// the database existed (cc-db.sh:4-8).
func carrierRewrite(carrier string, ids []string) error {
	return withCarrierLock(carrier, func() error {
		return rewriteCarrier(carrier, ids)
	})
}

func rewriteCarrier(carrier string, ids []string) error {
	if err := os.MkdirAll(filepath.Dir(carrier), 0o700); err != nil {
		return fmt.Errorf("create carrier directory: %w", err)
	}
	temp := carrier + carrierTempInfix + strconv.Itoa(os.Getpid())
	file, err := os.OpenFile(
		temp,
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC,
		0o600,
	)
	if err != nil {
		return fmt.Errorf("create carrier rewrite file: %w", err)
	}
	writer := bufio.NewWriter(file)
	for _, id := range ids {
		if _, err := writer.WriteString(id + "\n"); err != nil {
			_ = file.Close()
			_ = os.Remove(temp)
			return fmt.Errorf("write carrier rewrite file: %w", err)
		}
	}
	if err := writer.Flush(); err != nil {
		_ = file.Close()
		_ = os.Remove(temp)
		return fmt.Errorf("write carrier rewrite file: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(temp)
		return fmt.Errorf("write carrier rewrite file: %w", err)
	}
	if err := os.Rename(temp, carrier); err != nil {
		_ = os.Remove(temp)
		return fmt.Errorf("replace carrier file: %w", err)
	}
	return nil
}

func carrierHas(carrier, id string) (bool, error) {
	ids, err := readCarrier(carrier)
	if err != nil {
		return false, err
	}
	for _, existing := range ids {
		if existing == id {
			return true, nil
		}
	}
	return false, nil
}

// readCarrier returns the file's ids in file order, blank lines dropped —
// `grep .` in cc-db.sh's fallback listing (cc-db.sh:108).
func readCarrier(carrier string) ([]string, error) {
	file, err := os.Open(carrier)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read carrier file: %w", err)
	}
	defer file.Close()

	var ids []string
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		id := scanner.Text()
		if id == "" {
			continue
		}
		ids = append(ids, id)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read carrier file: %w", err)
	}
	return ids, nil
}
