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

// carrierLockSuffix and carrierTempInfix name the lock and atomic-rewrite
// scratch file beside the carrier.
const (
	carrierLockSuffix = ".lock"
	carrierTempInfix  = ".n."
)

// withCarrierLock runs fn holding an exclusive flock on <carrier>.lock. A
// platform without flock still records the hide instead of failing closed.
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

// carrierAdd appends an id under the lock unless the file already holds that
// exact line.
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

// carrierDelete treats a missing file as success and atomically replaces the
// original, so a reader never sees a truncated hide list.
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
// same scratch-then-rename sequence as carrierDelete.
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

// readCarrier returns the file's ids in file order with blank lines dropped.
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
