package headless

import (
	"hostops/pfm/internal/ask"
	pfmconfig "hostops/pfm/internal/config"
	pfmengine "hostops/pfm/internal/engine"
)

type headlessTestRunner struct{ id pfmengine.ID }

func (runner headlessTestRunner) Resolve(machine pfmconfig.Config) (ask.Engine, error) {
	if runner.id == pfmengine.Codex {
		return ask.ResolveCodex(machine)
	}
	return ask.ResolveClaude(machine)
}

func init() {
	ask.RegisterRunner(pfmengine.Claude, headlessTestRunner{id: pfmengine.Claude})
	ask.RegisterRunner(pfmengine.Codex, headlessTestRunner{id: pfmengine.Codex})
}
