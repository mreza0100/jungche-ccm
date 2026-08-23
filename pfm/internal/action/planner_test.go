package action

import (
	"testing"

	pfmengine "hostops/pfm/internal/engine"
)

type builtinTestPlanner struct{ id pfmengine.ID }

func (planner builtinTestPlanner) Plan(request HeadlessRequest) (HeadlessPlan, error) {
	if planner.id == pfmengine.Codex {
		return PlanCodex(request)
	}
	return PlanClaude(request)
}

func init() {
	RegisterPlanner(pfmengine.Claude, builtinTestPlanner{id: pfmengine.Claude})
	RegisterPlanner(pfmengine.Codex, builtinTestPlanner{id: pfmengine.Codex})
}

func TestUnknownEngineIsANamedError(t *testing.T) {
	_, err := PlannerFor(pfmengine.ID("zz"))
	if err == nil || err.Error() != "engine zz: no headless planner registered" {
		t.Fatalf("PlannerFor(zz) error = %v", err)
	}
}
