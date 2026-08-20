package main

import (
	"strings"
	"testing"
)

func TestReloadUsageIsCanonicalAndSwapIsNotMentioned(t *testing.T) {
	if !strings.Contains(reloadUsage, "usage: pfm chat reload") {
		t.Fatalf("reload usage=%q", reloadUsage)
	}
	if strings.Contains(reloadUsage, "chat swap") {
		t.Fatalf("legacy swap leaked into canonical usage: %q", reloadUsage)
	}
}
