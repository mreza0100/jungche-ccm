package ui

import (
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/colorprofile"
)

// interactiveColorProfile keeps the picker colourful even when a parent shell
// exports a non-interactive color preference. Detection still chooses the
// terminal's advertised profile, including ANSI256 and truecolor.
func interactiveColorProfile(output io.Writer) colorprofile.Profile {
	environ := make([]string, 0, len(os.Environ()))
	for _, entry := range os.Environ() {
		key, value, _ := strings.Cut(entry, "=")
		if key == "NO_COLOR" || (key == "CLICOLOR" && value == "0") {
			continue
		}
		environ = append(environ, entry)
	}

	profile := colorprofile.Detect(output, environ)
	if profile < colorprofile.ANSI {
		return colorprofile.ANSI
	}
	return profile
}
