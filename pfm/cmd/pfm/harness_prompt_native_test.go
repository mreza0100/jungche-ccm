package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"hostops/pfm/internal/config"
)

// Explicit REAL-SESSION opt-in: a loopback sink rejects the request before any
// inference. Ordinary jailed tests never need a native CLI or account.
func TestHarnessPromptNativeRepeatedCapture(t *testing.T) {
	binary := os.Getenv("PFM_TEST_CLAUDE_NATIVE")
	if binary == "" {
		t.Skip("set PFM_TEST_CLAUDE_NATIVE for localhost-only real CLI capture")
	}
	machine := config.Config{}
	machine.Claude.Binary = binary
	var previous harnessCapture
	for attempt := 0; attempt < 3; attempt++ {
		captured, err := captureHarnessPrompt(context.Background(), machine)
		if err != nil {
			t.Fatal(err)
		}
		if captured.ResolvedModel == "" || captured.CLIVersion == "" {
			t.Fatalf("capture lacks identity: %+v", captured)
		}
		if attempt > 0 && (captured.ResolvedModel != previous.ResolvedModel || normalizeHarnessPrompt(captured.Prompt) != normalizeHarnessPrompt(previous.Prompt)) {
			t.Fatal("repeated native captures changed beyond normalized metadata")
		}
		sum := sha256.Sum256([]byte(normalizeHarnessPrompt(captured.Prompt)))
		t.Logf("capture=%d requested=sonnet resolved=%s cli=%s normalized_sha256=%s", attempt+1, captured.ResolvedModel, captured.CLIVersion, hex.EncodeToString(sum[:]))
		if dir := os.Getenv("PFM_HARNESS_CAPTURE_DIR"); dir != "" {
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(captured)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "sonnet-capture.json"), raw, 0600); err != nil {
				t.Fatal(err)
			}
		}
		previous = captured
	}
}
