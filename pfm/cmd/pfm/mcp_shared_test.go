package main

import (
	"testing"

	"hostops/pfm/internal/compose"
)

func TestMCPListExcludesEveryFreshAndBootingRow(t *testing.T) {
	for _, kind := range []compose.Kind{
		compose.NewClaude,
		compose.NewCodex,
		compose.NewOpencode,
		compose.Booting,
	} {
		if !excludedFromMCPList(kind) {
			t.Errorf("MCP list admitted non-resumable row kind %s", kind)
		}
	}
	for _, kind := range []compose.Kind{
		compose.LiveClaude,
		compose.LiveCodex,
		compose.ResumeClaude,
		compose.ResumeCodex,
		compose.ResumeOpencode,
	} {
		if excludedFromMCPList(kind) {
			t.Errorf("MCP list excluded resumable/live row kind %s", kind)
		}
	}
}
