package ask

import (
	"context"
	"errors"
	"testing"

	pfmconfig "hostops/pfm/internal/config"
)

type fakeAskAdapter interface {
	Prepare() (AskInput, Evidence)
	WantSpanKind() string
}

// These adapters stand in for the next-wave preparation layers. They carry
// no transcript or harvester domain types: both feed only the shared ask
// contract and preserve provenance in its generic fields.
type fakeTranscriptAdapter struct{}

func (fakeTranscriptAdapter) Prepare() (AskInput, Evidence) {
	return AskInput{
			ContentFiles: []string{"prepared-transcript.md"},
			SourceLabels: []string{"session fixture#turns 1-14"},
			Prompt:       "find the visible answer",
		}, Evidence{
			File:  "prepared-transcript.md",
			Label: "session fixture#turns 1-14",
			Span:  SourceSpan{Kind: "turns", Start: 1, End: 14},
			Quote: "visible answer",
		}
}

func (fakeTranscriptAdapter) WantSpanKind() string { return "turns" }

type fakeHarvesterAdapter struct{}

func (fakeHarvesterAdapter) Prepare() (AskInput, Evidence) {
	return AskInput{
			ContentFiles: []string{"prepared-source.md"},
			SourceLabels: []string{"https://fixture.invalid/source"},
			Prompt:       "find the source claim",
		}, Evidence{
			File:  "prepared-source.md",
			Label: "https://fixture.invalid/source",
			Span:  SourceSpan{Kind: "lines", Start: 4, End: 9},
			Quote: "source claim",
		}
}

func (fakeHarvesterAdapter) WantSpanKind() string { return "lines" }

func TestResolveInputUsesExplicitValuesBeforeConfig(t *testing.T) {
	cfg := pfmconfig.Config{Ask: pfmconfig.AskConfig{
		Engine: "claude",
		Codex:  pfmconfig.EnginePrefs{Model: "codex-default", Effort: "medium"},
		Claude: pfmconfig.EnginePrefs{Model: "claude-default", Effort: "low"},
	}}
	resolved, err := ResolveInput(AskInput{
		ContentFiles: []string{"prepared.md"}, SourceLabels: []string{"fixture"}, Prompt: "answer",
		Engine: "codex", Model: "custom-model",
	}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Engine != "codex" || resolved.Model != "custom-model" || resolved.Effort != "medium" {
		t.Fatalf("resolved = %+v", resolved)
	}
	resolved, err = ResolveInput(AskInput{ContentFiles: []string{"prepared.md"}, Prompt: "answer"}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Engine != "claude" || resolved.Model != "claude-default" || resolved.Effort != "low" {
		t.Fatalf("config resolution = %+v", resolved)
	}
}

func TestBothStubEnginesResolveAndReturnSentinel(t *testing.T) {
	for _, name := range []string{"codex", "claude"} {
		engine, err := ResolveEngine(name)
		if err != nil {
			t.Fatalf("ResolveEngine(%q): %v", name, err)
		}
		_, err = engine.Run(context.Background(), AskInput{})
		if !errors.Is(err, ErrNotImplemented) {
			t.Fatalf("%s Run error=%v, want ErrNotImplemented", name, err)
		}
	}
}

func TestEvidenceStaysContentAgnosticForTranscriptAndHarvesterAdapters(t *testing.T) {
	adapters := map[string]fakeAskAdapter{
		"transcript": fakeTranscriptAdapter{},
		"harvester":  fakeHarvesterAdapter{},
	}
	for name, adapter := range adapters {
		input, evidence := adapter.Prepare()
		resolved, err := ResolveInput(input, pfmconfig.Config{})
		if err != nil {
			t.Fatalf("%s adapter ResolveInput(): %v", name, err)
		}
		if _, err := BuildPrompt(resolved); err != nil {
			t.Fatalf("%s adapter BuildPrompt(): %v", name, err)
		}
		engine, err := ResolveEngine(resolved.Engine)
		if err != nil {
			t.Fatalf("%s adapter ResolveEngine(): %v", name, err)
		}
		if _, err := engine.Run(context.Background(), resolved); !errors.Is(err, ErrNotImplemented) {
			t.Fatalf("%s adapter engine error=%v, want ErrNotImplemented", name, err)
		}
		if evidence.File != resolved.ContentFiles[0] || evidence.Label != resolved.SourceLabels[0] {
			t.Fatalf("%s adapter evidence=%+v does not feed resolved input=%+v", name, evidence, resolved)
		}
		if evidence.Span.Kind != adapter.WantSpanKind() {
			t.Fatalf("%s adapter span kind=%q, want %q", name, evidence.Span.Kind, adapter.WantSpanKind())
		}
	}
}
