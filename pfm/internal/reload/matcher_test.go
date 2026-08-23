package reload

import (
	pfmengine "hostops/pfm/internal/engine"
	"hostops/pfm/internal/gather"
)

type reloadTestMatcher struct{ id pfmengine.ID }

func (matcher reloadTestMatcher) IsCommand(argv []string, binaries ...string) bool {
	if matcher.id == pfmengine.Codex {
		return gather.IsCodexCommand(argv, binaries...)
	}
	return gather.IsClaudeCommand(argv, binaries...)
}

func init() {
	gather.RegisterMatcher(pfmengine.Claude, reloadTestMatcher{id: pfmengine.Claude})
	gather.RegisterMatcher(pfmengine.Codex, reloadTestMatcher{id: pfmengine.Codex})
}
