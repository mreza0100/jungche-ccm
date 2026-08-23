package index

import (
	"context"
	"testing"

	pfmengine "hostops/pfm/internal/engine"
	"hostops/pfm/internal/store"
)

type builtinTestSource struct{ id pfmengine.ID }

func (source builtinTestSource) Sync(ctx context.Context, database *store.Store, roots []string, counters *Counters) error {
	switch source.id {
	case pfmengine.Claude:
		return SyncClaude(ctx, database, roots, counters)
	case pfmengine.Codex:
		return SyncCodex(ctx, database, roots, counters)
	case pfmengine.Opencode:
		return SyncOpencode(ctx, database, roots, counters)
	default:
		return nil
	}
}

func init() {
	RegisterSource(pfmengine.Claude, builtinTestSource{id: pfmengine.Claude})
	RegisterSource(pfmengine.Codex, builtinTestSource{id: pfmengine.Codex})
	RegisterSource(pfmengine.Opencode, builtinTestSource{id: pfmengine.Opencode})
}

func TestUnknownEngineIsANamedError(t *testing.T) {
	_, err := SourceFor(pfmengine.ID("zz"))
	if err == nil || err.Error() != "engine zz: no index source registered" {
		t.Fatalf("SourceFor(zz) error = %v", err)
	}
}
