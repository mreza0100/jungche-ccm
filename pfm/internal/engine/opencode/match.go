package opencode

import (
	pfmengine "hostops/pfm/internal/engine"
	"hostops/pfm/internal/engine/matchutil"
)

type Matcher struct{}

func (Matcher) IsCommand(argv []string, binaries ...string) bool {
	return matchutil.Command(pfmengine.Opencode, argv, false, binaries...)
}
