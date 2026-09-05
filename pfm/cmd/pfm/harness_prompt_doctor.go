package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	config "hostops/pfm/internal/config"
	pfmengine "hostops/pfm/internal/engine"
)

// printHarnessPromptDoctor re-captures the live Claude CLI's built-in system
// prompt through a localhost sink — the request dies at the listener, so no
// tokens are spent and nothing leaves the machine — and compares its sha256
// to the staged baseline `pfm install` writes under the managed root. Three
// distinct outcomes: match, DRIFT, and CHECK FAILED; a capture that failed to
// run is never rendered as either of the other two.
func printHarnessPromptDoctor(ctx context.Context, stdout io.Writer, home string, machine config.Config) int {
	baselinePath := filepath.Join(home, ".local", "share", "pfm", "install", "prompts", "harness-original.sha256")
	raw, err := os.ReadFile(baselinePath)
	if err != nil {
		fmt.Fprintf(stdout, "doctor: harness-prompt: baseline unreadable (%v) — run pfm install\n", err)
		return 1
	}
	fields := strings.Fields(string(raw))
	if len(fields) < 2 {
		fmt.Fprintf(stdout, "doctor: harness-prompt: baseline malformed at %s — run pfm install\n", baselinePath)
		return 1
	}
	decoded, decodeErr := hex.DecodeString(fields[0])
	if decodeErr != nil || len(decoded) != sha256.Size || filepath.Base(fields[1]) != fields[1] {
		fmt.Fprintln(stdout, "doctor: harness-prompt: baseline malformed — run pfm install")
		return 1
	}
	modelRaw, modelErr := os.ReadFile(filepath.Join(filepath.Dir(baselinePath), "harness-original.model"))
	baselineModel := strings.TrimSpace(string(modelRaw))
	baseline, baselineErr := os.ReadFile(filepath.Join(filepath.Dir(baselinePath), fields[1]))
	baselineSum := sha256.Sum256(baseline)
	if modelErr != nil || baselineModel == "" || baselineErr != nil || hex.EncodeToString(baselineSum[:]) != fields[0] {
		fmt.Fprintf(stdout, "doctor: harness-prompt: BASELINE UNAVAILABLE identity=%s model=%q — missing, unreadable or inconsistent baseline; run pfm install\n", fields[1], baselineModel)
		return 1
	}
	captured, captureErr := configuredHarnessCapture(ctx, machine)
	resolved, version := captured.ResolvedModel, captured.CLIVersion
	if resolved == "" {
		resolved = "unknown"
	}
	if version == "" {
		version = "unknown"
	}
	fmt.Fprintf(stdout, "doctor: harness-prompt scope=claude-sonnet-only runtime=claude-code requested=sonnet resolved=%s cli=%q baseline=%s baseline_model=%s unchecked=active-chat,fable,opus,codex\n", resolved, version, fields[1], baselineModel)
	if captureErr == nil && captured.ResolvedModel != baselineModel {
		fmt.Fprintln(stdout, "doctor: harness-prompt: BASELINE UNAVAILABLE for resolved model — model mismatch; drift unknown")
		return 1
	}
	canonicalBaseline := sha256.Sum256([]byte(normalizeHarnessPrompt(string(baseline))))
	line, warn := harnessPromptVerdict(hex.EncodeToString(canonicalBaseline[:]), fields[1], captured.Prompt, captureErr)
	fmt.Fprintln(stdout, line)
	if warn {
		return 1
	}
	return 0
}

// harnessCaptureOverride is nil in production; printHarnessPromptDoctor then
// runs the real capture below. A jail has no genuine `claude` binary to spawn
// — that is REAL-SESSION territory (TESTPLAN.md), never jailable — so the
// command-package TestMain supplies a deterministic stub here, the same
// pattern as dependencyProbeOverride and hookProbeOverride. Only the CAPTURE
// step is ever swapped; the baseline read and the verdict comparison stay
// real, so a test still exercises the actual match/DRIFT/CHECK-FAILED logic.
type harnessCapture struct {
	Prompt        string
	ResolvedModel string
	CLIVersion    string
}

var harnessCaptureOverride func(context.Context, config.Config) (harnessCapture, error)

func configuredHarnessCapture(ctx context.Context, machine config.Config) (harnessCapture, error) {
	if harnessCaptureOverride != nil {
		return harnessCaptureOverride(ctx, machine)
	}
	return captureHarnessPrompt(ctx, machine)
}

// harnessBuildStamp is the CLI build stamp inside the billing-header system
// block. Every Claude Code release changes it, so both the live capture and
// the stored baseline are masked before hashing — DRIFT means prose drift.
var harnessBuildStamp = regexp.MustCompile(`cc_version=[^; ]*;`)

// normalizeHarnessPrompt excludes only CLI identity metadata. Claude may omit
// these two Environment lines even with dynamic sections excluded. Instructions
// in that section, and matching text elsewhere, remain part of the drift hash.
func normalizeHarnessPrompt(prompt string) string {
	lines := strings.Split(harnessBuildStamp.ReplaceAllLiteralString(prompt, "cc_version=*;"), "\n")
	kept := lines[:0]
	inEnvironment := false
	for _, line := range lines {
		if strings.HasPrefix(line, "# ") || line == "=== SYSTEM BLOCK ===" {
			inEnvironment = line == "# Environment"
		}
		if inEnvironment && (strings.HasPrefix(line, " - You are powered by the model named ") || strings.HasPrefix(line, " - Assistant knowledge cutoff is ")) {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// harnessPromptVerdict is the pure comparator: baseline hash + name, the
// captured prompt, and the capture error map to exactly one doctor line.
func harnessPromptVerdict(baselineSHA, baselineName, captured string, captureErr error) (string, bool) {
	if captureErr != nil {
		return fmt.Sprintf("doctor: harness-prompt: CHECK FAILED to run (%v) — drift unknown", captureErr), true
	}
	sum := sha256.Sum256([]byte(normalizeHarnessPrompt(captured)))
	live := hex.EncodeToString(sum[:])
	if live == baselineSHA {
		return "doctor: harness-prompt: matches baseline " + baselineName, false
	}
	return fmt.Sprintf("doctor: harness-prompt: DRIFT live=%s baseline=%s (%s) — checked Sonnet prompt differs; recapture, review, re-pin", live[:16], baselineSHA[:16], baselineName), true
}

// captureHarnessPrompt DELIBERATELY BYPASSES action.ClaudeSpawn, the one spawn
// door. Every other Claude launch in this binary goes through it; this one
// must not, because the door's job is to APPLY the configured prompt policy
// and this capture exists to observe the CLI's PRODUCTION prompt with no
// policy applied at all — it pins CLAUDE_CODE_SIMPLE_SYSTEM_PROMPT=0, a dummy
// endpoint and dummy credentials, none of which the door would ever emit. A
// capture routed through the door would hash whatever the fleet configured and
// report "no drift" forever.
//
// It spawns `claude -p` pointed at an ephemeral localhost listener that
// records the request body and refuses it with a non-retryable 400 (a 500
// would put the CLI into its retry loop).
func captureHarnessPrompt(ctx context.Context, machine config.Config) (harnessCapture, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return harnessCapture{}, fmt.Errorf("open capture sink: %w", err)
	}
	bodies := make(chan []byte, 1)
	server := &http.Server{Handler: harnessSinkHandler(bodies)}
	go func() { _ = server.Serve(listener) }()
	defer func() { _ = server.Close() }()

	binary := machine.Claude.Binary
	if binary == "" {
		binary = pfmengine.MustLookup(pfmengine.Claude).Binary
	}
	versionCtx, versionCancel := context.WithTimeout(ctx, 5*time.Second)
	versionCmd := exec.CommandContext(versionCtx, binary, "--version")
	versionCmd.WaitDelay = 500 * time.Millisecond
	versionCmd.Env = harnessCaptureEnv(os.Environ(), "http://"+listener.Addr().String())
	versionRaw, versionErr := versionCmd.Output()
	versionCancel()
	if versionErr != nil {
		return harnessCapture{}, fmt.Errorf("read Claude CLI version: %w", versionErr)
	}
	runCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	command := exec.CommandContext(runCtx, binary,
		"-p", "x", "--model", "sonnet", "--output-format", "json",
		"--strict-mcp-config", "--mcp-config", `{"mcpServers":{}}`,
		"--max-turns", "1", "--exclude-dynamic-system-prompt-sections",
	)
	command.Env = harnessCaptureEnv(os.Environ(), "http://"+listener.Addr().String())
	// The CLI exits nonzero by design — the sink refused its request; the
	// capture, not the exit code, is the result. The grace window covers the
	// handler goroutine still finishing its send after Run returns; a ctx case
	// is deliberately absent — a ready body racing an expired ctx in one
	// select would drop real captures at random.
	_ = command.Run()
	select {
	case body := <-bodies:
		captured, err := decodeHarnessCapture(body)
		captured.CLIVersion = strings.TrimSpace(string(versionRaw))
		return captured, err
	case <-time.After(2 * time.Second):
		return harnessCapture{CLIVersion: strings.TrimSpace(string(versionRaw))}, errors.New("no API request reached the capture sink")
	}
}

// harnessSinkHandler refuses every request with the non-retryable 400 and
// forwards the FIRST messages-call body only — the CLI can post telemetry or
// preflight calls to the base URL before the real /v1/messages request, and
// an empty or non-messages body must not win the capture.
func harnessSinkHandler(bodies chan<- []byte) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		if strings.HasSuffix(request.URL.Path, "/messages") {
			select {
			case bodies <- body:
			default:
			}
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte(`{"type":"error","error":{"type":"invalid_request_error","message":"captured by pfm doctor"}}`))
	}
}

// harnessCaptureEnv is the fleet hygiene strip applied in-process: inherited
// session identity, endpoint and cache overrides are dropped, then the sink
// endpoint, dummy credentials and the full-prompt arm are pinned.
func harnessCaptureEnv(environ []string, sinkURL string) []string {
	stripped := map[string]bool{
		"CLAUDE_CODE_SESSION_ID": true, "CLAUDECODE": true, "CLAUDE_CODE_CHILD_SESSION": true,
		"CLAUDE_CONFIG_DIR": true, "ENABLE_PROMPT_CACHING_1H": true, "FORCE_PROMPT_CACHING_5M": true,
		"ANTHROPIC_BASE_URL": true, "ANTHROPIC_AUTH_TOKEN": true, "ANTHROPIC_API_KEY": true,
		"ANTHROPIC_MODEL": true, "ANTHROPIC_SMALL_FAST_MODEL": true,
		"CLAUDE_CODE_AUTO_COMPACT_WINDOW": true, "CLAUDE_CODE_SIMPLE_SYSTEM_PROMPT": true,
	}
	result := make([]string, 0, len(environ)+5)
	for _, entry := range environ {
		name, _, _ := strings.Cut(entry, "=")
		if stripped[name] {
			continue
		}
		result = append(result, entry)
	}
	return append(result,
		"ANTHROPIC_BASE_URL="+sinkURL,
		"ANTHROPIC_API_KEY=pfm-doctor-sink",
		"ANTHROPIC_AUTH_TOKEN=pfm-doctor-sink",
		"CLAUDE_CODE_SIMPLE_SYSTEM_PROMPT=0",
		"FORCE_PROMPT_CACHING_5M=1",
	)
}

// joinSystemBlocks renders a captured request's system prompt exactly the way
// the baseline file was produced (jq: `.system | map(.text) |
// join("\n\n=== SYSTEM BLOCK ===\n\n")` with jq -r's trailing newline) — the
// hashes only ever match if this stays byte-compatible.
func joinSystemBlocks(body []byte) (string, error) {
	var payload struct {
		System json.RawMessage `json:"system"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("parse captured request: %w", err)
	}
	if len(payload.System) == 0 {
		return "", errors.New("captured request carries no system prompt")
	}
	var plain string
	if err := json.Unmarshal(payload.System, &plain); err == nil {
		if strings.TrimSpace(plain) == "" {
			return "", errors.New("captured system prompt is empty")
		}
		return plain + "\n", nil
	}
	var blocks []struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(payload.System, &blocks); err != nil {
		return "", fmt.Errorf("parse system blocks: %w", err)
	}
	texts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		texts = append(texts, block.Text)
	}
	if strings.TrimSpace(strings.Join(texts, "")) == "" {
		return "", errors.New("captured system prompt is empty")
	}
	return strings.Join(texts, "\n\n=== SYSTEM BLOCK ===\n\n") + "\n", nil
}

func decodeHarnessCapture(body []byte) (harnessCapture, error) {
	var request struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &request); err != nil {
		return harnessCapture{}, fmt.Errorf("parse captured request: %w", err)
	}
	if strings.TrimSpace(request.Model) == "" {
		return harnessCapture{}, errors.New("captured request carries no resolved model")
	}
	prompt, err := joinSystemBlocks(body)
	return harnessCapture{Prompt: prompt, ResolvedModel: request.Model}, err
}
