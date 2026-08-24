package installer

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	sourceRepoMarkerName = "source-repo"
	binaryOwnershipName  = "binary-ownership.json"
)

// BinaryOwnership is the small ledger used by pfm update. Paths are absolute
// because update must never infer ownership from PATH order.
type BinaryOwnership struct {
	Paths []string `json:"paths"`
}

func managedRootForHome(home string) string {
	return filepath.Join(home, ".local", "share", "pfm", "install")
}

// SourceRepoPath returns the install-owned clone marker location.
func SourceRepoPath(home string) string {
	return filepath.Join(managedRootForHome(home), sourceRepoMarkerName)
}

// WriteSourceRepoMarker records exactly one normalized clone path.
func WriteSourceRepoMarker(home, repo string) error {
	content, err := sourceRepoMarkerContent(repo)
	if err != nil {
		return err
	}
	return atomicWrite(SourceRepoPath(home), content, 0o600)
}

func sourceRepoMarkerContent(repo string) ([]byte, error) {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return nil, errors.New("source repository path is empty")
	}
	abs, err := filepath.Abs(repo)
	if err != nil {
		return nil, fmt.Errorf("resolve source repository %q: %w", repo, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("inspect source repository %s: %w", abs, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("source repository %s is not a directory", abs)
	}
	return []byte(abs + "\n"), nil
}

// ReadSourceRepoMarker reads the one-line clone marker and verifies that it
// still names a directory. A missing marker is a visible update/init error.
func ReadSourceRepoMarker(home string) (string, error) {
	raw, err := os.ReadFile(SourceRepoPath(home))
	if err != nil {
		return "", fmt.Errorf("read source repository marker: %w", err)
	}
	repo := strings.TrimSpace(string(raw))
	if repo == "" || strings.ContainsAny(repo, "\r\n") {
		return "", errors.New("source repository marker is not one path")
	}
	info, err := os.Stat(repo)
	if err != nil {
		return "", fmt.Errorf("inspect recorded source repository %s: %w", repo, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("recorded source repository %s is not a directory", repo)
	}
	return repo, nil
}

func binaryOwnershipPath(home string) string {
	return filepath.Join(managedRootForHome(home), binaryOwnershipName)
}

// ReadBinaryOwnership returns the install ledger. A missing ledger means no
// binary is owned; update must not adopt an arbitrary PATH copy.
func ReadBinaryOwnership(home string) (BinaryOwnership, error) {
	raw, err := os.ReadFile(binaryOwnershipPath(home))
	if errors.Is(err, fs.ErrNotExist) {
		return BinaryOwnership{}, nil
	}
	if err != nil {
		return BinaryOwnership{}, fmt.Errorf("read binary ownership ledger: %w", err)
	}
	var ledger BinaryOwnership
	if err := json.Unmarshal(raw, &ledger); err != nil {
		return BinaryOwnership{}, fmt.Errorf("decode binary ownership ledger: %w", err)
	}
	return ledger, nil
}

// RecordCanonicalBinary records only the canonical ~/.local/bin/pfm path.
func RecordCanonicalBinary(home string) error {
	content, err := canonicalBinaryOwnershipContent(home)
	if err != nil {
		return err
	}
	if sameFile(binaryOwnershipPath(home), content, 0o600) {
		return nil
	}
	return atomicWrite(binaryOwnershipPath(home), content, 0o600)
}

func canonicalBinaryOwnershipContent(home string) ([]byte, error) {
	path := filepath.Join(home, ".local", "bin", "pfm")
	ledger, err := ReadBinaryOwnership(home)
	if err != nil {
		return nil, err
	}
	found := false
	for _, owned := range ledger.Paths {
		if filepath.Clean(owned) == filepath.Clean(path) {
			found = true
			break
		}
	}
	if !found {
		ledger.Paths = append(ledger.Paths, path)
	}
	encoded, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode binary ownership ledger: %w", err)
	}
	return append(encoded, '\n'), nil
}
