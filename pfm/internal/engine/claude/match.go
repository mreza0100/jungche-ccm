package claude

import (
	"path/filepath"
	"strings"

	pfmengine "hostops/pfm/internal/engine"
)

type Matcher struct{}

func (Matcher) IsCommand(argv []string, binaries ...string) bool {
	if len(argv) == 0 {
		return false
	}
	descriptor := pfmengine.MustLookup(pfmengine.Claude)
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
	// Version-managed native executables are named after their version.
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
