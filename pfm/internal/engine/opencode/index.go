package opencode

import (
	"context"

	"hostops/pfm/internal/index"
	"hostops/pfm/internal/store"
)

type Source struct{}

func (Source) Sync(ctx context.Context, database *store.Store, roots []string, counters *index.Counters) error {
	return index.SyncOpencode(ctx, database, roots, counters)
}
