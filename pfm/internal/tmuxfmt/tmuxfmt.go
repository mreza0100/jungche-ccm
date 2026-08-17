// Package tmuxfmt parses the output of a tmux -F format string.
//
// It exists because the answer is version-dependent and was written five times.
// Every probe in this repository asks tmux for fields joined by the ASCII unit
// separator (0x1F) — a byte no pane title, path or window name can contain, so
// it cannot be confused for data. What comes BACK is not the same on every
// tmux: older builds render the non-printable separator as the four printable
// characters \037, while tmux 3.6 emits the raw byte. Ubuntu ships the former
// and macOS Homebrew the latter, so a parser that knows only one spelling reads
// a whole record as a single field and every live chat silently loses its row.
//
// Splitting on both is not a guess: 0x1F is unreachable from real pane data,
// and a literal backslash-0-3-7 in a title would have been treated as a
// separator by the previous parser too.
package tmuxfmt

import "strings"

// Separator is the ASCII unit separator a format string joins fields with.
const Separator = "\x1f"

// Escaped is how a tmux that refuses to emit control bytes spells Separator.
const Escaped = `\037`

// Join builds a format string from tmux field specifiers.
func Join(fields ...string) string {
	return strings.Join(fields, Separator)
}

// SplitN splits one output record into at most count fields, accepting either
// spelling of the separator.
func SplitN(line string, count int) []string {
	return strings.SplitN(strings.ReplaceAll(line, Escaped, Separator), Separator, count)
}
