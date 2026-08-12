package store

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func TestBatchUsesFiveHundredRecordTransactions(t *testing.T) {
	setStoreTestJail(t)
	store := openTestStore(t)
	t.Cleanup(func() { _ = store.Close() })
	ctx := context.Background()

	var ranges [][2]int
	if err := store.Batch(ctx, 1001, func(tx *ImmediateTx, start, end int) error {
		ranges = append(ranges, [2]int{start, end})
		for index := start; index < end; index++ {
			if err := tx.SetMeta(ctx, fmt.Sprintf("batch-%04d", index), "seen"); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("Batch() error = %v", err)
	}
	wantRanges := [][2]int{{0, 500}, {500, 1000}, {1000, 1001}}
	if !reflect.DeepEqual(ranges, wantRanges) {
		t.Fatalf("Batch() ranges = %v, want %v", ranges, wantRanges)
	}

	var count int
	if err := store.db.QueryRow(
		`SELECT count(*) FROM meta WHERE key LIKE 'batch-%'`,
	).Scan(&count); err != nil {
		t.Fatalf("count batched rows: %v", err)
	}
	if count != 1001 {
		t.Fatalf("batched row count = %d, want 1001", count)
	}
}

func TestImmediateTransactionRollsBack(t *testing.T) {
	setStoreTestJail(t)
	store := openTestStore(t)
	t.Cleanup(func() { _ = store.Close() })
	ctx := context.Background()
	sentinel := errors.New("stop")

	err := store.WithImmediateTx(ctx, func(tx *ImmediateTx) error {
		if err := tx.SetMeta(ctx, "rolled_back", "yes"); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("WithImmediateTx() error = %v, want sentinel", err)
	}
	if _, found, err := store.Meta(ctx, "rolled_back"); err != nil || found {
		t.Fatalf("Meta() after rollback found = %v, error = %v; want false, nil", found, err)
	}
}
