package artifact

import (
	"fmt"
	"sort"
	"strings"
)

// PathError keeps the exact offending artifact machine-readable while
// preserving the wrapped diagnostic text. Night failure evidence uses this
// instead of trying to recover a pathname from an error string.
type PathError struct {
	Path string
	Err  error
}

func (failure *PathError) Error() string { return failure.Err.Error() }
func (failure *PathError) Unwrap() error { return failure.Err }
func (failure *PathError) OffendingPath() string {
	return failure.Path
}

func ErrorAt(path string, err error) error {
	if err == nil {
		return nil
	}
	if path == "" {
		return fmt.Errorf("artifact path is empty: %w", err)
	}
	return &PathError{Path: path, Err: err}
}

// ParseError reports all independent problems a parser can identify safely.
// Problems are ordered deterministically so gate diagnostics remain stable.
type ParseError struct {
	Problems []string
}

func (e *ParseError) Error() string {
	return strings.Join(e.Problems, "\n")
}

func parseFailure(problems ...string) error {
	return &ParseError{Problems: append([]string(nil), problems...)}
}

func linesOf(text string) []string {
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func uniqueSorted(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		seen[value] = struct{}{}
	}
	result := make([]string, 0, len(seen))
	for value := range seen {
		result = append(result, value)
	}
	slicesSort(result)
	return result
}

// Kept local to avoid making callers depend on the representation chosen for
// deterministic artifact ordering.
func slicesSort(values []string) {
	sort.Strings(values)
}
