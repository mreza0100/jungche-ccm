package ask

import (
	"testing"

	pfmconfig "hostops/pfm/internal/config"
	pfmengine "hostops/pfm/internal/engine"
)

type builtinTestRunner struct{ id pfmengine.ID }

func (runner builtinTestRunner) Resolve(machine pfmconfig.Config) (Engine, error) {
	if runner.id == pfmengine.Codex {
		return ResolveCodex(machine)
	}
	return ResolveClaude(machine)
}

func init() {
	RegisterRunner(pfmengine.Claude, builtinTestRunner{id: pfmengine.Claude})
	RegisterRunner(pfmengine.Codex, builtinTestRunner{id: pfmengine.Codex})
}

func TestUnknownEngineIsANamedError(t *testing.T) {
	_, err := RunnerFor(pfmengine.ID("zz"))
	if err == nil || err.Error() != "engine zz: no ask runner registered" {
		t.Fatalf("RunnerFor(zz) error = %v", err)
	}
}
