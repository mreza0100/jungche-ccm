package reap

import (
	pfmengine "hostops/pfm/internal/engine"
	"hostops/pfm/internal/gather"
)

type reapTestMatcher struct{ id pfmengine.ID }

func (matcher reapTestMatcher) IsCommand(argv []string, binaries ...string) bool {
	if matcher.id == pfmengine.Codex {
		return gather.IsCodexCommand(argv, binaries...)
	}
	return gather.IsClaudeCommand(argv, binaries...)
}

func init() {
	gather.RegisterMatcher(pfmengine.Claude, reapTestMatcher{id: pfmengine.Claude})
	gather.RegisterMatcher(pfmengine.Codex, reapTestMatcher{id: pfmengine.Codex})
}
