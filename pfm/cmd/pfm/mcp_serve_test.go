package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"hostops/pfm/internal/config"
	"hostops/pfm/internal/harvestmcp"
	"hostops/pfm/internal/mcpserv"
	"hostops/pfm/internal/paths"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPDaemonHandlerRequiresBearerAndReportsSurface(t *testing.T) {
	handler := newMCPDaemonHandler(mcpDaemonOptions{
		Version:   "test-version",
		Token:     "neutral-token",
		StartedAt: time.Unix(123, 0).UTC(),
		Endpoint:  "http://127.0.0.1:8377",
		Chat:      http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
		Harvester: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
	})

	unauthenticated := httptest.NewRecorder()
	handler.ServeHTTP(unauthenticated, httptest.NewRequest(http.MethodGet, "/status", nil))
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want 401", unauthenticated.Code)
	}

	request := httptest.NewRequest(http.MethodGet, "/status", nil)
	request.Header.Set("Authorization", "Bearer neutral-token")
	authenticated := httptest.NewRecorder()
	handler.ServeHTTP(authenticated, request)
	if authenticated.Code != http.StatusOK {
		t.Fatalf("authenticated status = %d, body=%s", authenticated.Code, authenticated.Body.String())
	}
	var status mcpDaemonStatus
	if err := json.Unmarshal(authenticated.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.PFMVersion != "test-version" || status.ProtocolVersion == "" || status.PID < 1 || status.Endpoint == "" {
		t.Fatalf("status = %+v", status)
	}
	if !strings.Contains(authenticated.Body.String(), "chat") || !strings.Contains(authenticated.Body.String(), "harvester") {
		t.Fatalf("status surface = %s", authenticated.Body.String())
	}
}

func TestMCPDaemonMountedServersRequireAuthAndServeTools(t *testing.T) {
	root := jailTest(t)
	resolved, err := paths.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	chat, err := mcpserv.NewConfigured("test", nil, mcpserv.Runtime{Paths: resolved})
	if err != nil {
		t.Fatal(err)
	}
	defer chat.Close()
	harvester, err := harvestmcp.NewConfigured("test", harvestmcp.Runtime{
		Home: root, CacheDir: root + "/cache",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer harvester.Close()

	handler := newMCPDaemonHandler(mcpDaemonOptions{
		Version: "test", Token: "neutral-token", StartedAt: time.Now(), Endpoint: "http://127.0.0.1:8377",
		Chat: chat.NewHTTPHandler(), Harvester: harvester.NewHTTPHandler(),
	})
	server := httptest.NewServer(handler)
	defer server.Close()
	client := &http.Client{Transport: bearerRoundTripper{base: http.DefaultTransport, token: "neutral-token"}}
	ctx := context.Background()
	chatSession, err := connectHTTPMCP(t, ctx, server.URL+"/mcp/chat", client)
	if err != nil {
		t.Fatal(err)
	}
	defer chatSession.Close()
	tools, err := chatSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, tool := range tools.Tools {
		seen[tool.Name] = true
	}
	for _, name := range []string{"chat_keys", "chat_whoami", "chat_new", "chat_reload"} {
		if !seen[name] {
			t.Fatalf("chat tools omitted %q: %v", name, seen)
		}
	}
	if _, err := chatSession.CallTool(ctx, &mcp.CallToolParams{Name: "chat_whoami"}); err != nil {
		t.Fatalf("chat_whoami: %v", err)
	}

	harvesterSession, err := connectHTTPMCP(t, ctx, server.URL+"/mcp/harvester", client)
	if err != nil {
		t.Fatal(err)
	}
	defer harvesterSession.Close()
	if _, err := harvesterSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "searchCache", Arguments: map[string]any{"pattern": "never-match"},
	}); err != nil {
		t.Fatalf("searchCache: %v", err)
	}
}

func TestMCPServeRefusesHealthySecondInstance(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	portText := listener.Addr().(*net.TCPAddr).Port
	port := strconv.Itoa(portText)
	server := &http.Server{Handler: newMCPDaemonHandler(mcpDaemonOptions{
		Version: "test", Token: "neutral-token", StartedAt: time.Unix(5, 0), Endpoint: "http://127.0.0.1:" + port,
		Chat: http.NotFoundHandler(), Harvester: http.NotFoundHandler(),
	})}
	go server.Serve(listener)
	defer server.Close()

	runtime := commandRuntime{Config: config.Defaults(t.TempDir(), nil)}
	runtime.Config.MCP.HTTP.Port = portText
	runtime.Config.MCP.AuthToken = "neutral-token"
	runtime.Config.MCPServers["chat"] = config.MCPServer{Enabled: true}
	var stdout, stderr bytes.Buffer
	if code := runMCPServe(&stdout, &stderr, runtime); code != 1 {
		t.Fatalf("second serve code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "already running (pid") || !strings.Contains(stderr.String(), "since") {
		t.Fatalf("second serve stderr=%q", stderr.String())
	}
}

type bearerRoundTripper struct {
	base  http.RoundTripper
	token string
}

func (transport bearerRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	clone.Header.Set("Authorization", "Bearer "+transport.token)
	return transport.base.RoundTrip(clone)
}

func connectHTTPMCP(t *testing.T, ctx context.Context, endpoint string, client *http.Client) (*mcp.ClientSession, error) {
	t.Helper()
	protocolClient := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	return protocolClient.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: endpoint, HTTPClient: client, MaxRetries: -1, DisableStandaloneSSE: true,
	}, nil)
}
