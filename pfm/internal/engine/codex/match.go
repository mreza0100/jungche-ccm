package codex

import (
	"path/filepath"

	pfmengine "hostops/pfm/internal/engine"
)

type Matcher struct{}

func (Matcher) IsCommand(argv []string, binaries ...string) bool {
	if len(argv) == 0 {
		return false
	}
	name := filepath.Base(filepath.ToSlash(argv[0]))
	if name == pfmengine.MustLookup(pfmengine.Codex).Binary {
		return true
	}
	for _, binary := range binaries {
		if binary != "" && name == filepath.Base(binary) {
			return true
		}
	}
	return false
}
