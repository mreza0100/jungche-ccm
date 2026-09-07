package ui

import (
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/colorprofile"
)

// interactiveColorProfile keeps the picker colourful even when a parent shell
// exports a non-interactive color preference. It reads the real process
// environment; see interactiveColorProfileFrom for the hermetic decision
// logic and its tests.
func interactiveColorProfile(output io.Writer) colorprofile.Profile {
	return interactiveColorProfileFrom(output, os.Environ())
}

// interactiveColorProfileFrom decides the picker's color profile from an
// explicit environ slice, reading nothing ambient. It draws one line:
// capability vs preference.
//
//   - A shell PREFERENCE (NO_COLOR, CLICOLOR=0) is overridden by design — the
//     picker stays colorful even when a parent shell opted a pipeline out of
//     color, so those two entries are stripped before detection runs.
//   - A terminal CAPABILITY statement (TERM unset, or TERM=dumb) is never
//     overridden: it is not a preference, it is the terminal telling the OS
//     it cannot render SGR escapes, and writing them anyway corrupts the
//     screen. When the environ says that, this returns colorprofile.Detect's
//     honest answer with no ANSI floor applied.
func interactiveColorProfileFrom(output io.Writer, environ []string) colorprofile.Profile {
	stripped := make([]string, 0, len(environ))
	for _, entry := range environ {
		key, value, _ := strings.Cut(entry, "=")
		if key == "NO_COLOR" || (key == "CLICOLOR" && value == "0") {
			continue
		}
		stripped = append(stripped, entry)
	}

	profile := colorprofile.Detect(output, stripped)
	if declaresNoColorCapability(environ) {
		return profile
	}
	if profile < colorprofile.ANSI {
		return colorprofile.ANSI
	}
	return profile
}

// declaresNoColorCapability reports whether TERM is unset or "dumb" in the
// given environ — the terminal's own capability statement, distinct from a
// shell preference like NO_COLOR or CLICOLOR=0.
func declaresNoColorCapability(environ []string) bool {
	for _, entry := range environ {
		key, value, ok := strings.Cut(entry, "=")
		if key != "TERM" {
			continue
		}
		if !ok {
			return true
		}
		return value == "" || value == "dumb"
	}
	return true
}
