package harvest

import "testing"

// mustNew constructs a Harvester or fails the test with New's own error.
// Tests that exercise cache-dir resolution itself call New directly.
func mustNew(t testing.TB, options Options) *Harvester {
	t.Helper()
	h, err := New(options)
	if err != nil {
		t.Fatalf("harvest.New: %v", err)
	}
	return h
}
