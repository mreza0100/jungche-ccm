// Package ask defines the content-agnostic contract shared by prepared-source
// callers. It intentionally contains no harvesting, paging, discovery, or
// model process invocation.
package ask

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	pfmconfig "hostops/pfm/internal/config"
)

var ErrNotImplemented = errors.New("ask engine run not implemented")

type AskInput struct {
	ContentFiles []string
	SourceLabels []string
	Prompt       string
	Engine       string
	Model        string
	Effort       string
}

type SourceSpan struct {
	Kind       string
	Start, End int
}

type Evidence struct {
	File  string
	Label string
	Span  SourceSpan
	Quote string
}

type FileStatus struct {
	File   string
	Status string
	Note   string
}

type TokenUsage struct {
	Input       int
	CachedInput int
	Output      int
}

type AskResult struct {
	Answer   string
	Evidence []Evidence
	PerFile  []FileStatus
	Usage    TokenUsage
	Duration time.Duration
}

type Engine interface {
	Run(context.Context, AskInput) (AskResult, error)
}

// HarnessPrompt is the fixed instruction contract passed to either engine by
// the next-wave runner. Keep the wording byte-stable.
const HarnessPrompt = `Read the prepared content files listed below. Work ONLY from them; no network access, no other files.
{numbered file list: "N. <file path> — source: <source label>"}
TASK: {user prompt}
Rules: if a file is truncated or unusable, say so explicitly for that file instead of guessing.
After your answer, append a section titled exactly "EVIDENCE" listing one line per load-bearing claim:
  [file N] <location: line range, turn number, or chunk id> — "<short verbatim quote>"`

// ResolveInput materializes engine, model, and effort from config. Explicit
// non-empty fields always win; source labels default to the prepared paths so
// every file remains traceable without a domain-specific field.
func ResolveInput(input AskInput, machine pfmconfig.Config) (AskInput, error) {
	if len(input.ContentFiles) == 0 {
		return AskInput{}, fmt.Errorf("content files must not be empty")
	}
	if strings.TrimSpace(input.Prompt) == "" {
		return AskInput{}, fmt.Errorf("prompt must not be empty")
	}
	for index, file := range input.ContentFiles {
		if strings.TrimSpace(file) == "" {
			return AskInput{}, fmt.Errorf("content file %d is empty", index+1)
		}
	}
	if len(input.SourceLabels) != 0 && len(input.SourceLabels) != len(input.ContentFiles) {
		return AskInput{}, fmt.Errorf("source labels length %d does not match content files length %d", len(input.SourceLabels), len(input.ContentFiles))
	}
	resolved := input
	if resolved.Engine == "" {
		resolved.Engine = machine.Ask.Engine
	}
	if resolved.Engine == "" {
		resolved.Engine = "codex"
	}
	if resolved.Engine != "codex" && resolved.Engine != "claude" {
		return AskInput{}, fmt.Errorf("unknown ask engine %q", resolved.Engine)
	}
	prefs := machine.Ask.Codex
	if resolved.Engine == "claude" {
		prefs = machine.Ask.Claude
	}
	if resolved.Model == "" {
		resolved.Model = prefs.Model
	}
	if resolved.Effort == "" {
		resolved.Effort = prefs.Effort
	}
	if len(resolved.SourceLabels) == 0 {
		resolved.SourceLabels = append([]string(nil), resolved.ContentFiles...)
	}
	return resolved, nil
}

// BuildPrompt renders the fixed harness prompt around already-prepared files.
func BuildPrompt(input AskInput) (string, error) {
	if len(input.ContentFiles) == 0 {
		return "", fmt.Errorf("content files must not be empty")
	}
	if strings.TrimSpace(input.Prompt) == "" {
		return "", fmt.Errorf("prompt must not be empty")
	}
	if len(input.SourceLabels) != len(input.ContentFiles) {
		return "", fmt.Errorf("source labels length %d does not match content files length %d", len(input.SourceLabels), len(input.ContentFiles))
	}
	var builder strings.Builder
	builder.WriteString("Read the prepared content files listed below. Work ONLY from them; no network access, no other files.\n")
	for index, file := range input.ContentFiles {
		fmt.Fprintf(&builder, "%d. %s — source: %s\n", index+1, file, input.SourceLabels[index])
	}
	fmt.Fprintf(&builder, "TASK: %s\n", input.Prompt)
	builder.WriteString("Rules: if a file is truncated or unusable, say so explicitly for that file instead of guessing.\n")
	builder.WriteString("After your answer, append a section titled exactly \"EVIDENCE\" listing one line per load-bearing claim:\n")
	builder.WriteString("  [file N] <location: line range, turn number, or chunk id> — \"<short verbatim quote>\"")
	return builder.String(), nil
}

// ResolveEngine returns one of the two named stub engines. The registry is
// deliberately closed until a next-wave runner supplies process semantics.
func ResolveEngine(name string) (Engine, error) {
	switch name {
	case "codex":
		return codexEngine{}, nil
	case "claude":
		return claudeEngine{}, nil
	default:
		return nil, fmt.Errorf("unknown ask engine %q", name)
	}
}

type codexEngine struct{}

func (codexEngine) Run(context.Context, AskInput) (AskResult, error) {
	return AskResult{}, ErrNotImplemented
}

type claudeEngine struct{}

func (claudeEngine) Run(context.Context, AskInput) (AskResult, error) {
	return AskResult{}, ErrNotImplemented
}
