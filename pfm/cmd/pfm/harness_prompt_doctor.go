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
	captured, captureErr := captureHarnessPrompt(ctx, machine)
	line, warn := harnessPromptVerdict(fields[0], fields[1], captured, captureErr)
	fmt.Fprintln(stdout, line)
	if warn {
		return 1
	}
	return 0
}

// harnessPromptVerdict is the pure comparator: baseline hash + name, the
// captured prompt, and the capture error map to exactly one doctor line.
func harnessPromptVerdict(baselineSHA, baselineName, captured string, captureErr error) (string, bool) {
	if captureErr != nil {
		return fmt.Sprintf("doctor: harness-prompt: CHECK FAILED to run (%v) — drift unknown", captureErr), true
	}
	sum := sha256.Sum256([]byte(captured))
	live := hex.EncodeToString(sum[:])
	if live == baselineSHA {
		return "doctor: harness-prompt: matches baseline " + baselineName, false
	}
	return fmt.Sprintf("doctor: harness-prompt: DRIFT live=%s baseline=%s (%s) — Claude Code changed its system prompt; recapture, review, re-pin", live[:16], baselineSHA[:16], baselineName), true
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
func captureHarnessPrompt(ctx context.Context, machine config.Config) (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("open capture sink: %w", err)
	}
	bodies := make(chan []byte, 1)
	server := &http.Server{Handler: harnessSinkHandler(bodies)}
	go func() { _ = server.Serve(listener) }()
	defer func() { _ = server.Close() }()

	binary := machine.Claude.Binary
	if binary == "" {
		binary = pfmengine.MustLookup(pfmengine.Claude).Binary
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
		return joinSystemBlocks(body)
	case <-time.After(2 * time.Second):
		return "", errors.New("no API request reached the capture sink")
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
	return strings.Join(texts, "\n\n=== SYSTEM BLOCK ===\n\n") + "\n", nil
}
