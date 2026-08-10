package inject

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	claudeMenuPattern = regexp.MustCompile(`❯[[:space:]]*[0-9]+\.[[:space:]]`)
	codexMenuPattern  = regexp.MustCompile(`›[[:space:]]*[0-9]+\.[[:space:]]`)
	numberedOption    = regexp.MustCompile(`^[[:space:]]*›?[[:space:]]*[0-9]+\.[[:space:]]`)
	busyPattern       = regexp.MustCompile(`(?i)esc to interrupt|\([0-9]+s ·|· [0-9]+s|[0-9]+ tokens`)
	menuHintPattern   = regexp.MustCompile(`(?i)enter to (confirm|continue|select)|esc to (cancel|go back)`)
	ansiPattern       = regexp.MustCompile(`\x1b\[[0-9;]*m`)
)

// IsBusy mirrors chat.sh's live spinner-detail test.
func IsBusy(capture string) bool {
	return busyPattern.MatchString(capture)
}

// SelectorLine returns the selected numbered option for a real open menu.
// A lone Codex "› 1. draft" remains a legal composer line.
func SelectorLine(capture string) string {
	claude := lastLineContaining(capture, "❯")
	if claudeMenuPattern.MatchString(claude) {
		return strings.TrimSpace(claude)
	}

	codex := lastLineContaining(capture, "›")
	if !codexMenuPattern.MatchString(codex) {
		return ""
	}
	options := 0
	for _, line := range strings.Split(capture, "\n") {
		if numberedOption.MatchString(line) {
			options++
		}
	}
	if menuHintPattern.MatchString(capture) || options >= 2 {
		return strings.TrimSpace(codex)
	}
	return ""
}

func hasDraft(line string) bool {
	if line == "" {
		return false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "❯"))
	rest = strings.ReplaceAll(rest, "Press up to edit queued messages", "")
	for _, character := range rest {
		if character <= unicode.MaxASCII && unicode.IsGraphic(character) {
			return true
		}
	}
	return false
}

func isDimPlaceholder(styledLine string) bool {
	if !strings.Contains(styledLine, "\x1b[2m") {
		return false
	}
	// chat.sh treats the whole dim SGR-2 span as contextual placeholder text.
	withoutDim := styledLine
	for {
		start := strings.Index(withoutDim, "\x1b[2m")
		if start < 0 {
			break
		}
		rest := withoutDim[start+len("\x1b[2m"):]
		end := strings.Index(rest, "\x1b[")
		if end < 0 {
			withoutDim = withoutDim[:start]
			break
		}
		withoutDim = withoutDim[:start] + rest[end:]
	}
	withoutDim = ansiPattern.ReplaceAllString(withoutDim, "")
	return !hasDraft(withoutDim)
}

func lastComposerLine(capture string) string {
	if line := lastLineContaining(capture, "❯"); line != "" {
		return line
	}
	return lastLineContaining(capture, "›")
}

func lastLineContaining(capture, marker string) string {
	last := ""
	for _, line := range strings.Split(capture, "\n") {
		if strings.Contains(line, marker) {
			last = line
		}
	}
	return last
}

func lastNonEmptyLines(capture string, limit int) string {
	lines := strings.Split(capture, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRightFunc(line, unicode.IsSpace)
		if line != "" {
			filtered = append(filtered, line)
		}
	}
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}
	return strings.Join(filtered, "\n")
}
