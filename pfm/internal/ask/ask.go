// Package ask defines the content-agnostic process contract shared by
// prepared-source callers. It contains no harvesting, paging, or discovery.
package ask

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	pfmconfig "hostops/pfm/internal/config"
	"hostops/pfm/internal/deps"
	pfmengine "hostops/pfm/internal/engine"
)

const engineTimeout = 60 * time.Second

type AskInput struct {
	ContentFiles []string
	SourceLabels []string
	Prompt       string
	Engine       pfmengine.ID
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
	Usage    *TokenUsage
	Duration time.Duration
}

type Engine interface {
	Run(context.Context, AskInput) (AskResult, error)
}

// HarnessPrompt is the fixed instruction contract passed to either engine.
// Keep the wording byte-stable.
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
		var err error
		resolved.Engine, err = machine.DefaultEngine()
		if err != nil {
			return AskInput{}, err
		}
	}
	prefs := machine.Ask.PrefsFor(resolved.Engine)
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

// BinaryMissingError distinguishes a configured engine that is absent from a
// binary that resolved but crashed. Visible callers use that distinction to
// render honest absence instead of flattening every failure into "nothing".
type BinaryMissingError struct {
	Engine string
	Binary string
	Err    error
}

func (err *BinaryMissingError) Error() string {
	return fmt.Sprintf("%s binary MISSING (%s)", err.Engine, err.Binary)
}

func (err *BinaryMissingError) Unwrap() error { return err.Err }

// ResolveEngine binds one configured engine to its first roster account. The
// config roster owns both the account home and the configured binary; deps
// owns executable resolution.
func ResolveEngine(id pfmengine.ID, machine pfmconfig.Config) (Engine, error) {
	runner, err := RunnerFor(id)
	if err != nil {
		return nil, err
	}
	return runner.Resolve(machine)
}

// ResolveClaude binds the first configured Claude account to its process.
func ResolveClaude(machine pfmconfig.Config) (Engine, error) {
	if len(machine.Accounts) == 0 {
		return nil, fmt.Errorf("no Claude accounts configured")
	}
	descriptor := pfmengine.MustLookup(pfmengine.Claude)
	binary := strings.TrimSpace(machine.Claude.Binary)
	if binary == "" {
		binary = descriptor.Binary
	}
	return resolveProcess(descriptor, binary, machine.Accounts[0].ConfigDir, claudeArguments)
}

// ResolveCodex binds the first configured Codex account to its process.
func ResolveCodex(machine pfmconfig.Config) (Engine, error) {
	if len(machine.CodexAccounts) == 0 {
		return nil, fmt.Errorf("no Codex accounts configured")
	}
	descriptor := pfmengine.MustLookup(pfmengine.Codex)
	binary := strings.TrimSpace(machine.Codex.Binary)
	if binary == "" {
		binary = descriptor.Binary
	}
	return resolveProcess(descriptor, binary, machine.CodexAccounts[0].Home, codexArguments)
}

func resolveProcess(descriptor pfmengine.Descriptor, binary, home string, arguments func(AskInput) []string) (Engine, error) {
	path, err := deps.Resolve(binary)
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, &BinaryMissingError{Engine: descriptor.LongName, Binary: binary, Err: err}
		}
		return nil, fmt.Errorf("resolve %s binary %q: %w", descriptor.LongName, binary, err)
	}
	return processEngine{name: descriptor.LongName, path: path, homeVariable: descriptor.HomeEnv, home: home, argumentsFor: arguments}, nil
}

type processEngine struct {
	name         string
	path         string
	homeVariable string
	home         string
	argumentsFor func(AskInput) []string
}

func (engine processEngine) Run(parent context.Context, input AskInput) (AskResult, error) {
	prompt, err := BuildPrompt(input)
	if err != nil {
		return AskResult{}, err
	}
	ctx, cancel := context.WithTimeout(parent, engineTimeout)
	defer cancel()
	args := engine.argumentsFor(input)
	command := exec.CommandContext(ctx, engine.path, args...)
	configureBoundedCommand(command)
	command.Env = replaceEnvironment(os.Environ(), engine.homeVariable, engine.home)
	command.Stdin = strings.NewReader(prompt)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	started := time.Now()
	runErr := command.Run()
	duration := time.Since(started)
	if runErr != nil {
		switch {
		case errors.Is(ctx.Err(), context.DeadlineExceeded):
			return AskResult{}, fmt.Errorf("%s ask timed out: %w", engine.name, context.DeadlineExceeded)
		case ctx.Err() != nil:
			return AskResult{}, fmt.Errorf("%s ask canceled: %w", engine.name, ctx.Err())
		default:
			return AskResult{}, fmt.Errorf("%s ask failed: %w; stderr tail %q", engine.name, runErr, boundedTail(stderr.String(), 1024))
		}
	}
	answer, usage := extractUsage(stdout.String(), stderr.String())
	if answer == "" {
		return AskResult{}, fmt.Errorf("%s ask returned an empty answer", engine.name)
	}
	return AskResult{Answer: answer, Usage: usage, Duration: duration}, nil
}

func claudeArguments(input AskInput) []string {
	model := strings.TrimSpace(input.Model)
	effort := strings.ToLower(strings.TrimSpace(input.Effort))
	args := []string{"-p"}
	if model != "" {
		args = append(args, "--model", model)
	}
	if effort != "" {
		args = append(args, "--effort", effort)
	}
	return append(args, "--output-format", "text")
}

func codexArguments(input AskInput) []string {
	model := strings.TrimSpace(input.Model)
	effort := strings.ToLower(strings.TrimSpace(input.Effort))
	args := []string{"exec"}
	if model != "" {
		args = append(args, "--model", model)
	}
	if effort != "" {
		args = append(args, "-c", `model_reasoning_effort="`+effort+`"`)
	}
	return append(args, "--ephemeral", "--skip-git-repo-check", "--color", "never", "-")
}

func replaceEnvironment(environment []string, name, value string) []string {
	prefix := name + "="
	result := make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		if !strings.HasPrefix(entry, prefix) {
			result = append(result, entry)
		}
	}
	return append(result, prefix+value)
}

var usageField = regexp.MustCompile(`(?i)\b(cached_input_tokens|input_tokens|output_tokens)\b\s*[:=]\s*([0-9]+)`)

func extractUsage(stdout, stderr string) (string, *TokenUsage) {
	var usage *TokenUsage
	kept := make([]string, 0)
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		if parsed, ok := parseUsage(line); ok {
			usage = mergeUsage(usage, parsed)
			continue
		}
		kept = append(kept, line)
	}
	for _, line := range strings.Split(strings.TrimSpace(stderr), "\n") {
		if parsed, ok := parseUsage(line); ok {
			usage = mergeUsage(usage, parsed)
		}
	}
	return strings.TrimSpace(strings.Join(kept, "\n")), usage
}

func parseUsage(line string) (TokenUsage, bool) {
	normalized := strings.NewReplacer(`"`, "", `'`, "").Replace(line)
	matches := usageField.FindAllStringSubmatch(normalized, -1)
	if len(matches) == 0 {
		return TokenUsage{}, false
	}
	var usage TokenUsage
	for _, match := range matches {
		value, err := strconv.Atoi(match[2])
		if err != nil {
			continue
		}
		switch strings.ToLower(match[1]) {
		case "input_tokens":
			usage.Input = value
		case "cached_input_tokens":
			usage.CachedInput = value
		case "output_tokens":
			usage.Output = value
		}
	}
	return usage, true
}

func mergeUsage(current *TokenUsage, next TokenUsage) *TokenUsage {
	if current == nil {
		current = &TokenUsage{}
	}
	if next.Input != 0 {
		current.Input = next.Input
	}
	if next.CachedInput != 0 {
		current.CachedInput = next.CachedInput
	}
	if next.Output != 0 {
		current.Output = next.Output
	}
	return current
}

func boundedTail(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[len(value)-limit:]
}
