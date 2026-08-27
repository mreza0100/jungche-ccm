package installer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	themeManifestRelative = "blueprint/themes/sources.json"
	themeOwnershipName    = "theme-ownership.json"
	themeOwnerToken       = "{GH_USER}"
	maxThemeDownloadBytes = 10 << 20
)

type themeManifest struct {
	Comment       string                 `json:"_comment,omitempty"`
	SourceFetched map[string]themeSource `json:"source_fetched"`
}

type themeSource struct {
	Repo     string `json:"repo"`
	Raw      string `json:"raw"`
	Target   string `json:"target"`
	Activate string `json:"activate"`
	Requires string `json:"requires"`
}

type themeOwnershipRecord struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

func (installer *engine) installThemes(ctx context.Context) {
	if !installer.options.InstallThemes {
		return
	}
	sources, err := loadThemeSources(ctx, installer.options)
	if err != nil {
		installer.skip("themes NOT installed: load " + themeManifestRelative + ": " + err.Error())
		return
	}
	ownershipPath := filepath.Join(installer.managedRoot, themeOwnershipName)
	ownership, err := readThemeOwnership(ownershipPath)
	if err != nil {
		installer.skip("themes NOT installed: read ownership " + ownershipPath + ": " + err.Error())
		return
	}

	for _, name := range sortedThemeNames(sources) {
		source := sources[name]
		target, targetErr := themeTarget(installer.options.Home, source.Target)
		if targetErr != nil {
			installer.skip("theme " + name + " manifest target invalid: " + targetErr.Error())
			continue
		}
		existing, exists, readErr := readOptionalRegularFile(target)
		if readErr != nil {
			installer.skip("theme " + name + " inspect failed: " + readErr.Error())
			continue
		}
		record, owned := ownership[name]
		if owned {
			if filepath.Clean(record.Path) != filepath.Clean(target) {
				installer.skip(fmt.Sprintf("theme %s ownership target drift: ledger=%s manifest=%s", name, record.Path, target))
				continue
			}
			if exists && contentSHA256(existing) != record.SHA256 {
				installer.skip("theme " + name + " locally modified; left in place at " + target)
				continue
			}
		} else if exists {
			installer.skip("theme " + name + " already exists but is not installer-owned; left in place at " + target)
			continue
		}

		if !installer.apply {
			if owned && exists {
				installer.ok("theme " + name + " currently installed; apply checks its source for updates")
			} else {
				if changeErr := installer.change("fetch theme "+name+" -> "+target, nil); changeErr != nil {
					installer.skip("theme " + name + " preview failed: " + changeErr.Error())
				}
			}
			continue
		}

		content, fetchErr := fetchTheme(ctx, installer.options.ThemeHTTPClient, source.Raw)
		if fetchErr != nil {
			installer.skip("theme " + name + " fetch failed: " + fetchErr.Error())
			continue
		}
		if !json.Valid(content) {
			installer.skip("theme " + name + " fetch failed: response is not valid JSON")
			continue
		}
		digest := contentSHA256(content)
		if exists && bytes.Equal(existing, content) && owned && record.SHA256 == digest {
			installer.ok("theme " + name + " unchanged at " + target)
			continue
		}

		next := cloneThemeOwnership(ownership)
		next[name] = themeOwnershipRecord{Path: target, SHA256: digest}
		if writeErr := atomicWrite(target, content, 0o644); writeErr != nil {
			installer.skip("theme " + name + " install failed: write " + target + ": " + writeErr.Error())
			continue
		}
		if ledgerErr := writeThemeOwnership(ownershipPath, next); ledgerErr != nil {
			rollbackErr := rollbackTheme(target, existing, exists)
			message := "theme " + name + " install failed: record ownership: " + ledgerErr.Error()
			if rollbackErr != nil {
				message += "; rollback failed: " + rollbackErr.Error()
			}
			installer.skip(message)
			continue
		}
		ownership = next
		if changeErr := installer.change("write theme "+name+" -> "+target, nil); changeErr != nil {
			installer.skip("theme " + name + " report failed after install: " + changeErr.Error())
		}
	}
}

func (installer *engine) uninstallThemes() {
	if !installer.options.InstallThemes {
		return
	}
	ownershipPath := filepath.Join(installer.managedRoot, themeOwnershipName)
	ownership, err := readThemeOwnership(ownershipPath)
	if err != nil {
		installer.skip("themes NOT removed: read ownership " + ownershipPath + ": " + err.Error())
		return
	}
	if len(ownership) == 0 {
		installer.ok("theme ownership ledger absent; no themes removed")
		return
	}

	for _, name := range sortedThemeOwnershipNames(ownership) {
		record := ownership[name]
		content, exists, readErr := readOptionalRegularFile(record.Path)
		if readErr != nil {
			installer.skip("theme " + name + " uninstall inspect failed: " + readErr.Error())
			continue
		}
		if exists && contentSHA256(content) != record.SHA256 {
			installer.skip("theme " + name + " locally modified; left in place and retained recovery ownership at " + record.Path)
			continue
		}
		if !installer.apply {
			if changeErr := installer.change("remove theme "+name+" "+record.Path, nil); changeErr != nil {
				installer.skip("theme " + name + " uninstall preview failed: " + changeErr.Error())
			}
			continue
		}

		next := cloneThemeOwnership(ownership)
		delete(next, name)
		if exists {
			if removeErr := os.Remove(record.Path); removeErr != nil {
				installer.skip("theme " + name + " uninstall failed: remove " + record.Path + ": " + removeErr.Error())
				continue
			}
		}
		if ledgerErr := writeThemeOwnership(ownershipPath, next); ledgerErr != nil {
			var rollbackErr error
			if exists {
				rollbackErr = atomicWrite(record.Path, content, 0o644)
			}
			message := "theme " + name + " uninstall failed: update ownership: " + ledgerErr.Error()
			if rollbackErr != nil {
				message += "; rollback failed: " + rollbackErr.Error()
			}
			installer.skip(message)
			continue
		}
		ownership = next
		if changeErr := installer.change("remove theme "+name+" "+record.Path, nil); changeErr != nil {
			installer.skip("theme " + name + " uninstall report failed: " + changeErr.Error())
		}
	}
	if installer.apply {
		themesDir := filepath.Join(installer.options.Home, ".claude", "themes")
		if removeErr := os.Remove(themesDir); removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) && !errors.Is(removeErr, fs.ErrExist) {
			installer.skip("leave theme directory " + themesDir + ": " + removeErr.Error())
		}
	}
}

func loadThemeSources(ctx context.Context, options Options) (map[string]themeSource, error) {
	var content []byte
	var origin string
	var err error
	if strings.TrimSpace(options.SourceRepo) != "" {
		origin = filepath.Join(options.SourceRepo, filepath.FromSlash(themeManifestRelative))
		content, err = os.ReadFile(origin)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return nil, fmt.Errorf("read local manifest %s: %w", origin, err)
			}
			localErr := err
			origin = strings.TrimSpace(options.ThemeManifestURL)
			if origin == "" {
				return nil, fmt.Errorf("read local manifest: %w; no release manifest URL is configured", localErr)
			}
			content, err = fetchTheme(ctx, options.ThemeHTTPClient, origin)
			if err != nil {
				return nil, fmt.Errorf("local theme manifest unavailable: %v; fetch release manifest %s: %w", localErr, origin, err)
			}
		}
	} else {
		origin = strings.TrimSpace(options.ThemeManifestURL)
		if origin == "" {
			return nil, errors.New("no source repository or release manifest URL is configured")
		}
		content, err = fetchTheme(ctx, options.ThemeHTTPClient, origin)
		if err != nil {
			return nil, fmt.Errorf("fetch release manifest %s: %w", origin, err)
		}
	}
	if bytes.Contains(content, []byte(themeOwnerToken)) {
		owner, ownerErr := themeManifestOwner(options)
		if ownerErr != nil {
			return nil, fmt.Errorf("resolve registered placeholder %s: %w", themeOwnerToken, ownerErr)
		}
		content = bytes.ReplaceAll(content, []byte(themeOwnerToken), []byte(owner))
	}
	var manifest themeManifest
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("decode %s: %w", origin, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, fmt.Errorf("decode %s: multiple JSON values", origin)
		}
		return nil, fmt.Errorf("decode %s trailing content: %w", origin, err)
	}
	if len(manifest.SourceFetched) == 0 {
		return nil, fmt.Errorf("manifest %s has no source_fetched themes", origin)
	}
	for name, source := range manifest.SourceFetched {
		if strings.TrimSpace(name) == "" || strings.TrimSpace(source.Raw) == "" || strings.TrimSpace(source.Target) == "" {
			return nil, fmt.Errorf("manifest %s theme %q is missing name, raw, or target", origin, name)
		}
		if err := validateThemeURL(source.Raw); err != nil {
			return nil, fmt.Errorf("manifest %s theme %q raw URL: %w", origin, name, err)
		}
	}
	return manifest.SourceFetched, nil
}

func themeManifestOwner(options Options) (string, error) {
	if strings.TrimSpace(options.SourceRepo) != "" {
		manifestPath := filepath.Join(options.SourceRepo, ".professor", "manifest.json")
		content, err := os.ReadFile(manifestPath)
		if err == nil {
			var manifest struct {
				InstalledFrom struct {
					Repo string `json:"repo"`
				} `json:"installed_from"`
			}
			if decodeErr := json.Unmarshal(content, &manifest); decodeErr != nil {
				return "", fmt.Errorf("decode %s: %w", manifestPath, decodeErr)
			}
			if owner := repositoryOwner(manifest.InstalledFrom.Repo); owner != "" {
				return owner, nil
			}
			return "", fmt.Errorf("%s installed_from.repo does not name owner/repo", manifestPath)
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("read %s: %w", manifestPath, err)
		}
	}
	parsed, err := url.Parse(strings.TrimSpace(options.ThemeManifestURL))
	if err != nil {
		return "", fmt.Errorf("parse release manifest URL: %w", err)
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) < 2 || strings.TrimSpace(segments[0]) == "" {
		return "", fmt.Errorf("release manifest URL %q does not name an owner/repository", options.ThemeManifestURL)
	}
	return segments[0], nil
}

func repositoryOwner(repository string) string {
	value := strings.TrimSpace(strings.TrimSuffix(repository, ".git"))
	if parsed, err := url.Parse(value); err == nil && parsed.Host != "" {
		value = strings.Trim(parsed.Path, "/")
	}
	parts := strings.Split(value, "/")
	if len(parts) < 2 {
		return ""
	}
	return strings.TrimSpace(parts[len(parts)-2])
}

func validateThemeURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return err
	}
	if parsed.Scheme == "https" && parsed.Host != "" {
		return nil
	}
	host := parsed.Hostname()
	if parsed.Scheme == "http" && (host == "127.0.0.1" || host == "::1" || host == "localhost") {
		return nil
	}
	return fmt.Errorf("must be HTTPS (HTTP is accepted only for loopback tests)")
}

func fetchTheme(ctx context.Context, client *http.Client, raw string) ([]byte, error) {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, fmt.Errorf("create GET %s: %w", raw, err)
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", raw, err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, drainErr := io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
		closeErr := response.Body.Close()
		if drainErr != nil {
			return nil, errors.Join(fmt.Errorf("GET %s: HTTP %s; drain response: %w", raw, response.Status, drainErr), closeErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("GET %s: HTTP %s; close response: %w", raw, response.Status, closeErr)
		}
		return nil, fmt.Errorf("GET %s: HTTP %s", raw, response.Status)
	}
	limited := io.LimitReader(response.Body, maxThemeDownloadBytes+1)
	content, err := io.ReadAll(limited)
	closeErr := response.Body.Close()
	if err != nil {
		return nil, errors.Join(fmt.Errorf("read GET %s: %w", raw, err), closeErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close GET %s response: %w", raw, closeErr)
	}
	if len(content) > maxThemeDownloadBytes {
		return nil, fmt.Errorf("GET %s exceeds %d bytes", raw, maxThemeDownloadBytes)
	}
	return content, nil
}

func themeTarget(home, target string) (string, error) {
	if !strings.HasPrefix(target, "~/") {
		return "", fmt.Errorf("target %q must start with ~/", target)
	}
	resolved := filepath.Clean(filepath.Join(home, filepath.FromSlash(strings.TrimPrefix(target, "~/"))))
	root := filepath.Join(filepath.Clean(home), ".claude", "themes")
	relative, err := filepath.Rel(root, resolved)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("target %q must name a file beneath ~/.claude/themes", target)
	}
	return resolved, nil
}

func readOptionalRegularFile(path string) ([]byte, bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("inspect %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, true, fmt.Errorf("%s is not a regular file", path)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, true, fmt.Errorf("read %s: %w", path, err)
	}
	return content, true, nil
}

func readThemeOwnership(path string) (map[string]themeOwnershipRecord, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]themeOwnershipRecord{}, nil
	}
	if err != nil {
		return nil, err
	}
	records := map[string]themeOwnershipRecord{}
	if err := json.Unmarshal(content, &records); err != nil {
		return nil, fmt.Errorf("decode JSON: %w", err)
	}
	for name, record := range records {
		if strings.TrimSpace(name) == "" || !filepath.IsAbs(record.Path) || len(record.SHA256) != sha256.Size*2 {
			return nil, fmt.Errorf("invalid ownership record %q", name)
		}
		if _, err := hex.DecodeString(record.SHA256); err != nil {
			return nil, fmt.Errorf("ownership record %q sha256: %w", name, err)
		}
	}
	return records, nil
}

func writeThemeOwnership(path string, records map[string]themeOwnershipRecord) error {
	if len(records) == 0 {
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove empty ownership ledger %s: %w", path, err)
		}
		return nil
	}
	content, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return fmt.Errorf("encode ownership: %w", err)
	}
	content = append(content, '\n')
	if err := atomicWrite(path, content, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func rollbackTheme(path string, previous []byte, existed bool) error {
	if existed {
		return atomicWrite(path, previous, 0o644)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func contentSHA256(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}

func cloneThemeOwnership(records map[string]themeOwnershipRecord) map[string]themeOwnershipRecord {
	cloned := make(map[string]themeOwnershipRecord, len(records))
	for name, record := range records {
		cloned[name] = record
	}
	return cloned
}

func sortedThemeNames(sources map[string]themeSource) []string {
	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedThemeOwnershipNames(records map[string]themeOwnershipRecord) []string {
	names := make([]string, 0, len(records))
	for name := range records {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
