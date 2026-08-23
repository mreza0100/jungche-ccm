package compose

import (
	"strings"
	"testing"
)

func TestEngineForKindCheckedRejectsUnknown(t *testing.T) {
	_, err := EngineForKindChecked(Kind(255))
	if err == nil || !strings.Contains(err.Error(), "unknown compose kind 255") {
		t.Fatalf("EngineForKindChecked(255) error=%v", err)
	}
}

func TestEngineForKindUnknownIsExplicit(t *testing.T) {
	if got := EngineForKind(Kind(255)); got != "unknown" {
		t.Fatalf("EngineForKind(255)=%q, want explicit unknown label", got)
	}
}
