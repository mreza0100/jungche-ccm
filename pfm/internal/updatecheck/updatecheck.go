// Package updatecheck maintains the silent, next-invocation Professor release
// notice consumed by the interactive fleet picker.
package updatecheck

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const lockStaleAfter = 2 * time.Minute

// Notice is one successful release lookup. Current is rewritten to the
// invoking binary's version when Read returns it; the cached value is retained
// only as useful diagnostic provenance.
type Notice struct {
	Current    string    `json:"current"`
	Latest     string    `json:"latest"`
	ReleaseURL string    `json:"release_url"`
	CheckedAt  time.Time `json:"checked_at"`
}

type semanticVersion struct {
	major int
	minor int
	patch int
}

// Read returns a notice only when the last successful lookup found a release
// newer than the binary asking. Missing cache state is an ordinary first run;
// malformed state is named to callers, which may deliberately keep the picker
// silent and let the detached checker repair it.
func Read(path, current string) (Notice, bool, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Notice{}, false, nil
	}
	if err != nil {
		return Notice{}, false, fmt.Errorf("read update cache: %w", err)
	}
	var notice Notice
	if err := json.Unmarshal(raw, &notice); err != nil {
		return Notice{}, false, fmt.Errorf("decode update cache: %w", err)
	}
	installed, installedOK := parseVersion(current)
	latest, latestOK := parseReleaseVersion(notice.Latest)
	if !installedOK || !latestOK || !newer(latest, installed) {
		return Notice{}, false, nil
	}
	notice.Current = normalizeVersion(current)
	return notice, true, nil
}

// Check performs one bounded latest-release lookup and atomically replaces the
// cache only after a complete, valid response. A failed lookup leaves the last
// successful notice intact, so temporary network failures cannot make an
// already-known update disappear.
func Check(ctx context.Context, path, current, latestURL string, client *http.Client) error {
	release, err := acquire(path + ".lock")
	if err != nil {
		return err
	}
	if release == nil {
		return nil
	}
	defer release()

	if _, ok := parseVersion(current); !ok {
		return fmt.Errorf("current version %q is not vMAJOR.MINOR.PATCH", current)
	}
	if client == nil {
		client = &http.Client{Timeout: 12 * time.Second}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodHead, latestURL, nil)
	if err != nil {
		return fmt.Errorf("build latest-release request: %w", err)
	}
	request.Header.Set("User-Agent", "pfm-update-check/"+normalizeVersion(current))
	noFollow := *client
	noFollow.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	response, err := noFollow.Do(request)
	if err != nil {
		return fmt.Errorf("request latest Professor release: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 300 || response.StatusCode >= 400 {
		return fmt.Errorf("latest Professor release returned %s", response.Status)
	}
	location := strings.TrimSpace(response.Header.Get("Location"))
	if location == "" {
		return errors.New("latest Professor release redirect omitted Location")
	}
	resolved, err := request.URL.Parse(location)
	if err != nil {
		return fmt.Errorf("parse latest Professor release redirect: %w", err)
	}
	latest := pathVersion(resolved)
	if _, ok := parseReleaseVersion(latest); !ok {
		return fmt.Errorf("latest Professor release redirect %q has no vMAJOR.MINOR.PATCH tag", location)
	}
	notice := Notice{
		Current:    normalizeVersion(current),
		Latest:     latest,
		ReleaseURL: resolved.String(),
		CheckedAt:  time.Now().UTC(),
	}
	if err := writeAtomic(path, notice); err != nil {
		return fmt.Errorf("write update cache: %w", err)
	}
	return nil
}

func acquire(path string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create update cache directory: %w", err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			if closeErr := file.Close(); closeErr != nil {
				_ = os.Remove(path)
				return nil, fmt.Errorf("close update lock: %w", closeErr)
			}
			return func() { _ = os.Remove(path) }, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return nil, fmt.Errorf("create update lock: %w", err)
		}
		info, statErr := os.Stat(path)
		if statErr != nil {
			if errors.Is(statErr, fs.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("inspect update lock: %w", statErr)
		}
		if time.Since(info.ModTime()) <= lockStaleAfter {
			return nil, nil
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("remove stale update lock: %w", err)
		}
	}
	return nil, nil
}

func writeAtomic(path string, notice Notice) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".update-check-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(notice); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func pathVersion(location *url.URL) string {
	parts := strings.Split(strings.Trim(location.Path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return normalizeVersion(parts[len(parts)-1])
}

func normalizeVersion(value string) string {
	value = strings.TrimSpace(value)
	if value != "" && value[0] != 'v' {
		value = "v" + value
	}
	return value
}

func parseVersion(value string) (semanticVersion, bool) {
	value = strings.TrimPrefix(normalizeVersion(value), "v")
	if separator := strings.IndexAny(value, "-+"); separator >= 0 {
		if separator == 0 || separator == len(value)-1 {
			return semanticVersion{}, false
		}
		value = value[:separator]
	}
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return semanticVersion{}, false
	}
	numbers := make([]int, 3)
	for index, part := range parts {
		if part == "" {
			return semanticVersion{}, false
		}
		number, err := strconv.Atoi(part)
		if err != nil || number < 0 || strconv.Itoa(number) != part {
			return semanticVersion{}, false
		}
		numbers[index] = number
	}
	return semanticVersion{major: numbers[0], minor: numbers[1], patch: numbers[2]}, true
}

func parseReleaseVersion(value string) (semanticVersion, bool) {
	normalized := strings.TrimPrefix(normalizeVersion(value), "v")
	if strings.ContainsAny(normalized, "-+") {
		return semanticVersion{}, false
	}
	return parseVersion(normalized)
}

func newer(candidate, current semanticVersion) bool {
	if candidate.major != current.major {
		return candidate.major > current.major
	}
	if candidate.minor != current.minor {
		return candidate.minor > current.minor
	}
	return candidate.patch > current.patch
}
