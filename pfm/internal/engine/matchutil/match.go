// Package matchutil holds the one executable-matching implementation shared
// by engine registry matchers and legacy process scanners.
package matchutil

import (
	"path/filepath"
	"strings"

	pfmengine "hostops/pfm/internal/engine"
)

// Command reports whether argv names an engine process. VersionNamed enables
// Claude's native-install convention where argv[0] is a dotted version number.
func Command(id pfmengine.ID, argv []string, versionNamed bool, binaries ...string) bool {
	if len(argv) == 0 {
		return false
	}
	descriptor := pfmengine.MustLookup(id)
	executable := filepath.ToSlash(argv[0])
	name := filepath.Base(executable)
	if name == descriptor.Binary {
		return true
	}
	for _, hint := range descriptor.BinaryPathHints {
		if strings.Contains(executable, hint) {
			return true
		}
	}
	for _, binary := range binaries {
		if binary != "" && name == filepath.Base(binary) {
			return true
		}
	}
	if !versionNamed {
		return false
	}
	dot := strings.IndexByte(name, '.')
	if dot <= 0 {
		return false
	}
	for _, character := range name[:dot] {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}
