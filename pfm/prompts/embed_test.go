package prompts

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"testing"
)

func TestDreamerEmbeddedFilesMatchMovedBytes(t *testing.T) {
	want := map[string]string{
		"dreamer-distill.prompt.md": "0436dbc661b70b231025b5004f7a64c79d2f5768eb30d654975d55e3d34bb8c1",
		"dreamer-refiner.prompt.md": "a3e8a32705ce8c42e85c2c520de6e73ac79fb8f627f42f99d5fcf54bbcf8d045",
		"lanes/explorer.md":         "e1929a6f12e5df9e5d1ce5b888e6554c2ef07206dd88c339b221464dc3fa011a",
		"lanes/tracer.md":           "f7f159e0df2456d1b6ff6123f657c8db38ee3ae638bec890eb21ae0c3b6d9bd0",
	}
	for name, wantHash := range want {
		raw, err := fs.ReadFile(Dreamer(), name)
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", name, err)
		}
		digest := sha256.Sum256(raw)
		if got := hex.EncodeToString(digest[:]); got != wantHash {
			t.Fatalf("%s sha256 = %s, want moved bytes %s", name, got, wantHash)
		}
	}
}
