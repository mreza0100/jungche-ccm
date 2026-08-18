package codexgen

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	overrideVersion = 1

	modeReplaceSection = "replace-section"
	modeReplaceExact   = "replace-exact"
	modeDelete         = "delete"
	modeInsertAfter    = "insert-after"
)

// OverrideStatus is the check/build-visible result for one override file.
// Status is deliberately a small vocabulary: an override is either applied,
// missing its exact anchor, or pinned to a source region that has drifted.
type OverrideStatus struct {
	File       string
	Source     string
	Mode       string
	Anchor     string
	Status     string
	MatchCount int
	SourceHash string
}

// Override is the validated, project-local form of one override file.  Exactly
// one of HeadingPath and Anchor must be present.
type Override struct {
	Version     int
	Source      string
	Mode        string
	HeadingPath []string
	Anchor      string
	Content     string
	SourceHash  string
}

type overrideFile struct {
	Version     *int     `json:"version"`
	Source      string   `json:"source"`
	Mode        string   `json:"mode"`
	HeadingPath []string `json:"headingPath"`
	Anchor      string   `json:"anchor"`
	Content     *string  `json:"content"`
	SourceHash  string   `json:"sourceHash"`
}

// ApplyOverrides applies every sorted JSON override for sourceName.  The
// source is kept as bytes (rather than parsed/re-serialized Markdown), so an
// override never introduces unrelated formatting churn.  A missing or
// ambiguous anchor and a stale pin both return statuses and an error: callers
// must not be able to mistake an unapplied override for a clean build.
func ApplyOverrides(root, sourceName, source string, cfg Config) (string, []OverrideStatus, error) {
	return applyOverrides(root, sourceName, source, cfg)
}

func applyOverrides(root, sourceName, source string, cfg Config) (string, []OverrideStatus, error) {
	if err := validateConfig(cfg); err != nil {
		return source, nil, fmt.Errorf("apply overrides: %w", err)
	}
	dir := filepath.Join(root, filepath.FromSlash(cfg.OverridesDir))
	paths, err := sortedOverrideFiles(dir)
	if err != nil {
		return source, nil, err
	}
	current := source
	statuses := make([]OverrideStatus, 0, len(paths))
	normalSource := filepath.ToSlash(filepath.Clean(sourceName))
	for _, path := range paths {
		override, err := readOverride(path)
		if err != nil {
			return current, statuses, err
		}
		if override.Source != normalSource {
			continue
		}

		status := OverrideStatus{
			File:       path,
			Source:     override.Source,
			Mode:       override.Mode,
			Anchor:     overrideAnchorDescription(override),
			Status:     "anchor-missing",
			SourceHash: override.SourceHash,
		}
		start, end, matchCount := locateOverrideRegion(current, override)
		status.MatchCount = matchCount
		if matchCount != 1 {
			statuses = append(statuses, status)
			if matchCount == 0 {
				return current, statuses, fmt.Errorf("override anchor-missing: %s: %s", path, status.Anchor)
			}
			return current, statuses, fmt.Errorf("override anchor-missing: %s: %s matched %d times (want exactly one)", path, status.Anchor, matchCount)
		}

		region := current[start:end]
		if override.SourceHash != "" && !sourceHashMatches(override.SourceHash, []byte(region)) {
			status.Status = "stale-pin"
			statuses = append(statuses, status)
			return current, statuses, fmt.Errorf("override stale — re-review: %s: %s", path, status.Anchor)
		}

		current = applyOverrideEdit(current, start, end, override)
		status.Status = "applied"
		statuses = append(statuses, status)
	}
	return current, statuses, nil
}

// sortedOverrideFiles walks the configured directory in lexical path order.
// WalkDir is deterministic for each directory, and the final sort also makes
// the ordering explicit if the filesystem implementation changes.
func sortedOverrideFiles(dir string) ([]string, error) {
	if _, err := os.Stat(dir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("inspect Codex override directory %s: %w", dir, err)
	}
	paths := make([]string, 0)
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk Codex override directory %s: %w", path, walkErr)
		}
		if entry.IsDir() {
			return nil
		}
		if !strings.HasSuffix(entry.Name(), ".json") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("stat Codex override %s: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

func readOverride(path string) (Override, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Override{}, fmt.Errorf("read Codex override %s: %w", path, err)
	}
	var file overrideFile
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&file); err != nil {
		return Override{}, fmt.Errorf("decode Codex override %s: %w", path, err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return Override{}, fmt.Errorf("decode Codex override %s: multiple JSON values", path)
		}
		return Override{}, fmt.Errorf("decode Codex override %s: trailing data: %w", path, err)
	}
	if file.Version == nil {
		return Override{}, fmt.Errorf("Codex override %s: version is required", path)
	}
	if *file.Version != overrideVersion {
		return Override{}, fmt.Errorf("Codex override %s: unsupported version %d (want %d)", path, *file.Version, overrideVersion)
	}
	if strings.TrimSpace(file.Source) == "" {
		return Override{}, fmt.Errorf("Codex override %s: source is required", path)
	}
	normalSource, err := normalizeOverrideSource(file.Source)
	if err != nil {
		return Override{}, fmt.Errorf("Codex override %s: %w", path, err)
	}
	if !validOverrideMode(file.Mode) {
		return Override{}, fmt.Errorf("Codex override %s: invalid mode %q", path, file.Mode)
	}
	if (len(file.HeadingPath) == 0) == (strings.TrimSpace(file.Anchor) == "") {
		return Override{}, fmt.Errorf("Codex override %s: set exactly one of headingPath or anchor", path)
	}
	for index, heading := range file.HeadingPath {
		if normalizeHeadingTitle(heading) == "" {
			return Override{}, fmt.Errorf("Codex override %s: headingPath[%d] is empty", path, index)
		}
	}
	if file.Content == nil && file.Mode != modeDelete {
		return Override{}, fmt.Errorf("Codex override %s: content is required for mode %s", path, file.Mode)
	}
	content := ""
	if file.Content != nil {
		content = *file.Content
	}
	if file.SourceHash != "" {
		if _, err := parseSHA256Pin(file.SourceHash); err != nil {
			return Override{}, fmt.Errorf("Codex override %s: %w", path, err)
		}
	}
	return Override{
		Version:     *file.Version,
		Source:      normalSource,
		Mode:        file.Mode,
		HeadingPath: normalizedHeadingPath(file.HeadingPath),
		Anchor:      file.Anchor,
		Content:     content,
		SourceHash:  file.SourceHash,
	}, nil
}

func normalizeOverrideSource(source string) (string, error) {
	if filepath.IsAbs(source) {
		return "", errors.New("source must be repository-relative")
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(source)))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.TrimSpace(source) == "" {
		return "", fmt.Errorf("source %q must be repository-relative", source)
	}
	return clean, nil
}

func validOverrideMode(mode string) bool {
	switch mode {
	case modeReplaceSection, modeReplaceExact, modeDelete, modeInsertAfter:
		return true
	default:
		return false
	}
}

func overrideAnchorDescription(override Override) string {
	if len(override.HeadingPath) != 0 {
		return "headingPath " + strings.Join(override.HeadingPath, " > ")
	}
	return "anchor " + fmt.Sprintf("%q", override.Anchor)
}

func sourceHashMatches(pin string, region []byte) bool {
	want, err := parseSHA256Pin(pin)
	if err != nil {
		return false
	}
	got := sha256.Sum256(region)
	return bytes.Equal(got[:], want[:])
}

func parseSHA256Pin(pin string) ([32]byte, error) {
	var zero [32]byte
	value := strings.TrimSpace(pin)
	if strings.HasPrefix(strings.ToLower(value), "sha256:") {
		value = value[len("sha256:"):]
	}
	if len(value) != sha256.Size*2 {
		return zero, fmt.Errorf("sourceHash must be sha256:<64 hex characters>")
	}
	decoded, err := hex.DecodeString(value)
	if err != nil {
		return zero, fmt.Errorf("sourceHash must be sha256:<64 hex characters>: %w", err)
	}
	copy(zero[:], decoded)
	return zero, nil
}

type markdownHeading struct {
	Start int
	Level int
	Title string
	Path  []string
}

var markdownHeadingRE = regexp.MustCompile(`^ {0,3}(#{1,6})[ \t]+(.+?)[ \t]*$`)

func locateOverrideRegion(source string, override Override) (start, end, count int) {
	if len(override.HeadingPath) == 0 {
		from := 0
		for {
			index := strings.Index(source[from:], override.Anchor)
			if index < 0 {
				break
			}
			index += from
			count++
			if count == 1 {
				start, end = index, index+len(override.Anchor)
			}
			// Advance one byte so overlapping literal matches are counted too;
			// exactly-one means exactly one occurrence, not one non-overlapping
			// occurrence.
			from = index + 1
			if len(override.Anchor) == 0 {
				break
			}
		}
		return start, end, count
	}

	headings := parseMarkdownHeadings(source)
	for _, heading := range headings {
		if !headingPathMatches(heading.Path, override.HeadingPath) {
			continue
		}
		count++
		if count != 1 {
			continue
		}
		start = heading.Start
		end = len(source)
		for _, next := range headings {
			if next.Start > heading.Start && next.Level <= heading.Level {
				end = next.Start
				break
			}
		}
		end = trimSectionEnd(source, start, end)
	}
	return start, end, count
}

func normalizedHeadingPath(path []string) []string {
	result := make([]string, len(path))
	for index, heading := range path {
		result[index] = normalizeHeadingTitle(heading)
	}
	return result
}

func normalizeHeadingTitle(heading string) string {
	heading = strings.TrimSpace(heading)
	heading = strings.TrimLeft(heading, "#")
	return strings.TrimSpace(heading)
}

func headingPathMatches(actual, wanted []string) bool {
	if len(wanted) > len(actual) {
		return false
	}
	return equalStrings(actual[len(actual)-len(wanted):], wanted)
}

func parseMarkdownHeadings(source string) []markdownHeading {
	lines := strings.SplitAfter(source, "\n")
	headings := make([]markdownHeading, 0)
	stack := make([]markdownHeading, 0, 6)
	offset := 0
	inFence := false
	var fence string
	for _, lineWithNL := range lines {
		line := strings.TrimSuffix(strings.TrimSuffix(lineWithNL, "\n"), "\r")
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			mark := trimmed[:3]
			if !inFence {
				inFence, fence = true, mark
			} else if mark == fence {
				inFence = false
			}
			offset += len(lineWithNL)
			continue
		}
		if !inFence {
			if match := markdownHeadingRE.FindStringSubmatch(line); match != nil {
				title := strings.TrimSpace(strings.TrimRight(match[2], "#"))
				level := len(match[1])
				for len(stack) > 0 && stack[len(stack)-1].Level >= level {
					stack = stack[:len(stack)-1]
				}
				path := make([]string, 0, len(stack)+1)
				for _, parent := range stack {
					path = append(path, parent.Title)
				}
				path = append(path, title)
				heading := markdownHeading{Start: offset, Level: level, Title: title, Path: path}
				headings = append(headings, heading)
				stack = append(stack, heading)
			}
		}
		offset += len(lineWithNL)
	}
	return headings
}

func trimSectionEnd(source string, start, end int) int {
	if end <= start {
		return end
	}
	region := source[start:end]
	trimmed := strings.TrimRight(region, "\r\n")
	if len(trimmed) == len(region) {
		return end
	}
	lineEnding := "\n"
	if strings.HasSuffix(region[:len(region)-len(strings.TrimRight(region, "\r\n"))], "\r\n") {
		lineEnding = "\r\n"
	}
	return start + len(trimmed) + len(lineEnding)
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func applyOverrideEdit(source string, start, end int, override Override) string {
	content := override.Content
	switch override.Mode {
	case modeReplaceSection, modeReplaceExact:
		return source[:start] + content + source[end:]
	case modeDelete:
		return source[:start] + source[end:]
	case modeInsertAfter:
		return source[:end] + content + source[end:]
	default:
		// readOverride validates this before it reaches the edit.  Keeping a
		// total function here makes a future caller unable to lose source bytes.
		return source
	}
}
