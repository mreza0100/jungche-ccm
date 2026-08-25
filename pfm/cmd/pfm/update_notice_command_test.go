package main

import (
	"strings"
	"testing"

	"hostops/pfm/internal/compose"
)

func TestProfessorUpdatePromptExplainsThenAsksBeforeUpdating(t *testing.T) {
	prompt := professorUpdatePrompt(compose.Row{ID: "pfm-update-v0.61.2"})
	overview := strings.Index(prompt, "present a concise overview")
	approval := strings.Index(prompt, "Ask the user for explicit approval")
	update := strings.Index(prompt, "pfm update --to v0.61.2")
	if overview < 0 || approval < overview || update < approval {
		t.Fatalf("update prompt order is not overview → approval → update: %q", prompt)
	}
	for _, want := range []string{"pfm doctor", "Do not push, tag, publish, release", "Professor v0.61.2"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("update prompt %q lacks %q", prompt, want)
		}
	}
}
