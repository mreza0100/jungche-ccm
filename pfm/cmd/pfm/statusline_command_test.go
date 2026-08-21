package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"hostops/pfm/internal/statusline"
)

func TestStatuslineCommandRendersFromJailedInput(t *testing.T) {
	jailTest(t)
	root := t.TempDir()
	cacheDir := filepath.Join(root, "tmp")
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "cc-vertex-spend"), []byte("1.23|4.56|0"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PFM_HOME", root)
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(root, ".cc", "1"))
	t.Setenv("PFM_TMUX_DIR", filepath.Join(root, "tmux"))
	t.Setenv("PFM_SID_DIR", filepath.Join(root, "sid"))

	var stdout, stderr bytes.Buffer
	code := runStatusline(
		nil,
		strings.NewReader(`{"model":{"display_name":"Opus 4"}}`),
		&stdout,
		&stderr,
	)
	if code != 0 || !strings.Contains(stdout.String(), "◆ Opus 4") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestDetachedRefreshCLIPathsWriteCachesInTwinHome(t *testing.T) {
	jailTest(t)
	root := t.TempDir()
	cacheDir := filepath.Join(root, "tmp")
	t.Setenv("PFM_HOME", root)

	monitoring, err := os.ReadFile(filepath.Join("..", "..", "internal", "statusline", "testdata", "monitoring.json"))
	if err != nil {
		t.Fatal(err)
	}
	billing, err := os.ReadFile(filepath.Join("..", "..", "internal", "statusline", "testdata", "billing.json"))
	if err != nil {
		t.Fatal(err)
	}
	storage, err := os.ReadFile(filepath.Join("..", "..", "internal", "statusline", "testdata", "storage.json"))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case strings.Contains(request.URL.Path, "/timeSeries"):
			_, _ = writer.Write(monitoring)
		case strings.Contains(request.URL.Path, "/skus"):
			_, _ = writer.Write(billing)
		case strings.Contains(request.URL.Path, "/cachedContents"):
			_, _ = writer.Write(storage)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	originalVertex := statuslineVertexOptions
	originalGPT := statuslineGPTOptions
	t.Cleanup(func() {
		statuslineVertexOptions = originalVertex
		statuslineGPTOptions = originalGPT
	})
	statuslineVertexOptions = func(context.Context, io.Writer) (statusline.VertexOptions, error) {
		return statusline.VertexOptions{
			Now:               func() time.Time { return time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC) },
			Project:           "sample-project",
			Locations:         []string{"europe-west4"},
			AccessToken:       func(context.Context) (string, error) { return "fixture-token", nil },
			Client:            server.Client(),
			MonitoringBaseURL: server.URL,
			BillingBaseURL:    server.URL,
			AIBaseURL:         server.URL,
		}, nil
	}
	statuslineGPTOptions = func() statusline.GPTOptions {
		return statusline.GPTOptions{
			Now: func() time.Time { return time.Unix(1_786_838_400, 0) },
			ReadRateLimits: func(context.Context) ([]byte, error) {
				return os.ReadFile(filepath.Join("..", "..", "internal", "statusline", "testdata", "gpt-app-server.jsonl"))
			},
		}
	}

	for _, flag := range []string{"--refresh-vertex", "--refresh-gpt"} {
		var stdout, stderr bytes.Buffer
		if code := runStatusline([]string{flag}, strings.NewReader(""), &stdout, &stderr); code != 0 {
			t.Fatalf("%s code=%d stdout=%q stderr=%q", flag, code, stdout.String(), stderr.String())
		}
	}
	for _, path := range []string{
		filepath.Join(cacheDir, "cc-vertex-spend"),
		filepath.Join(cacheDir, "cc-gpt-usage-"+strconv.Itoa(os.Getuid())+".json"),
	} {
		if info, err := os.Stat(path); err != nil || info.Size() == 0 {
			t.Fatalf("native refresher did not write %s: info=%v err=%v", path, info, err)
		}
	}
}

func TestStatuslineAndUsageHookCommandsFailOpen(t *testing.T) {
	jailTest(t)
	root := t.TempDir()
	var stdout, stderr bytes.Buffer
	if code := runStatusline(nil, strings.NewReader("{"), &stdout, &stderr); code != 0 ||
		!strings.Contains(stderr.String(), "fail-open") {
		t.Fatalf("malformed statusline: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	configDir := filepath.Join(root, ".claude")
	target := filepath.Join(root, "squatted-cache")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, ".credentials.json"), []byte(`{"claudeAiOauth":{"accessToken":"fixture"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cacheLink := filepath.Join(root, "tmp", "cc-usage-"+strconv.Itoa(os.Getuid()))
	if err := os.MkdirAll(filepath.Dir(cacheLink), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, cacheLink); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PFM_HOME", root)
	t.Setenv("CLAUDE_CONFIG_DIR", configDir)
	stdout.Reset()
	stderr.Reset()
	if code := runUsageHook(nil, &stdout, &stderr); code != 0 || stdout.Len() != 0 ||
		!strings.Contains(stderr.String(), "fail-open") {
		t.Fatalf("usage hook: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestCodexSeatUsageHookNeverTouchesClaudeCredentials(t *testing.T) {
	jailTest(t)
	root := t.TempDir()
	claudeConfig := filepath.Join(root, "claude-must-not-be-read")
	if err := os.WriteFile(claudeConfig, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_HOME", filepath.Join(root, ".codex-2"))
	t.Setenv("CODEX_THREAD_ID", "thread-2")
	t.Setenv("CLAUDE_CONFIG_DIR", claudeConfig)
	var stdout, stderr bytes.Buffer
	if code := runUsageHookWithRuntime(nil, &stdout, &stderr, commandRuntime{}); code != 0 || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("Codex usage hook touched Claude state: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}
