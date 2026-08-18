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
	compactPattern    = regexp.MustCompile(`^[[:space:]]*/compact([[:space:]]|$)`)
	queueProofPattern = regexp.MustCompile(`(?i)press up to edit queued messages|queued messages?|pending messages?|message (will be|was) (queued|submitted)|submitted after (the )?next tool call`)
)

// isCompactCommand mirrors chat.sh's `grep -qE '^[[:space:]]*/compact([[:space:]]|$)'`
// test, used both for the primary message and for every then steer.
func isCompactCommand(message string) bool {
	return compactPattern.MatchString(message)
}

func isHarnessCommand(message string) bool {
	return strings.HasPrefix(strings.TrimLeftFunc(message, unicode.IsSpace), "/")
}

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
	rest := strings.TrimSpace(line)
	rest = strings.TrimPrefix(rest, "❯")
	rest = strings.TrimPrefix(rest, "›")
	rest = strings.TrimSpace(rest)
	rest = strings.ReplaceAll(rest, "Press up to edit queued messages", "")
	for _, character := range rest {
		// POSIX [[:graph:]] is printable ASCII excluding space. Go's
		// unicode.IsGraphic includes ASCII space, which turns format-only
		// placeholders into apparent drafts.
		if character >= 0x21 && character <= 0x7e {
			return true
		}
	}
	return false
}

func hasPastePlaceholder(value string) bool {
	lower := strings.ToLower(value)
	return strings.Contains(lower, "[pasted text") ||
		strings.Contains(lower, "[pasted content")
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

// deliveryProven distinguishes a cleared composer from evidence that the turn
// moved into the transcript/engine. A busy delivery needs queue-specific
// evidence; the spinner was already present before we typed and proves only
// the older turn.
func deliveryProven(before, after, message string, queued, fileBacked bool) bool {
	pastComposer := withoutLastComposerLine(after)
	if messageVisible(pastComposer, message) ||
		(fileBacked && hasPastePlaceholder(pastComposer)) {
		return true
	}
	if queued {
		return queueProofPattern.MatchString(after) &&
			!queueProofPattern.MatchString(before)
	}
	return !IsBusy(before) && IsBusy(after)
}

func proofExpectation(queued bool) string {
	if queued {
		return "queue indicator"
	}
	return "engine processing state"
}

func withoutLastComposerLine(capture string) string {
	composer := lastComposerLine(capture)
	if composer == "" {
		return capture
	}
	lines := strings.Split(capture, "\n")
	for index := len(lines) - 1; index >= 0; index-- {
		if lines[index] == composer {
			lines = append(lines[:index], lines[index+1:]...)
			break
		}
	}
	return strings.Join(lines, "\n")
}

func messageVisible(capture, message string) bool {
	capture = normalizeSpace(capture)
	message = normalizeSpace(message)
	if capture == "" || message == "" {
		return false
	}
	if len([]rune(message)) <= 80 {
		return strings.Contains(capture, message)
	}
	return strings.Contains(capture, headRunes(message, 40)) ||
		strings.Contains(capture, tailRunes(message, 40))
}
