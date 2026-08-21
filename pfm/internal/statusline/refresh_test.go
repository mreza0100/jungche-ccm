package statusline

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestVertexRefresherWritesSpendAndPriceCachesFromRecordedAPIs(t *testing.T) {
	monitoring := mustFixture(t, "monitoring.json")
	billing := mustFixture(t, "billing.json")
	storage := mustFixture(t, "storage.json")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer fixture-token" {
			t.Errorf("authorization header = %q", request.Header.Get("Authorization"))
		}
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

	root := t.TempDir()
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	cachePath := filepath.Join(root, "cc-vertex-spend")
	pricePath := filepath.Join(root, "cc-vertex-prices-eur.json")
	if err := os.WriteFile(cachePath+".lock", nil, 0o600); err != nil {
		t.Fatal(err)
	}
	err := RefreshVertex(context.Background(), VertexOptions{
		Now:               func() time.Time { return now },
		Project:           "sample-project",
		Locations:         []string{"europe-west4"},
		CachePath:         cachePath,
		PriceCachePath:    pricePath,
		Client:            server.Client(),
		AccessToken:       func(context.Context) (string, error) { return "fixture-token", nil },
		MonitoringBaseURL: server.URL,
		BillingBaseURL:    server.URL,
		AIBaseURL:         server.URL,
		Log:               io.Discard,
	})
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(body)); got != "2.25|2.25|1786881600" {
		t.Fatalf("spend cache = %q", got)
	}
	if _, err := os.Stat(cachePath + ".lock"); !os.IsNotExist(err) {
		t.Fatalf("Vertex refresh lock survived completion: %v", err)
	}
	var prices map[string][2]float64
	priceBody, err := os.ReadFile(pricePath)
	if err != nil || json.Unmarshal(priceBody, &prices) != nil {
		t.Fatalf("price cache unreadable: err=%v body=%q", err, priceBody)
	}
	if prices["gemini-2.5-flash"] != [2]float64{0.263, 2.194} {
		t.Fatalf("live prices = %#v", prices)
	}
}

func TestGPTRefresherWritesRecordedRateLimitsAtomically(t *testing.T) {
	root := t.TempDir()
	cachePath := filepath.Join(root, "gpt.json")
	if err := os.WriteFile(strings.TrimSuffix(cachePath, ".json")+".lock", nil, 0o600); err != nil {
		t.Fatal(err)
	}
	fixture := mustFixture(t, "gpt-app-server.jsonl")
	err := RefreshGPT(context.Background(), GPTOptions{
		CachePath: cachePath,
		ReadRateLimits: func(context.Context) ([]byte, error) {
			return fixture, nil
		},
		Now: func() time.Time { return time.Unix(1_786_838_400, 0) },
	})
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	var cached map[string]any
	if err := json.Unmarshal(body, &cached); err != nil {
		t.Fatal(err)
	}
	if cached["planType"] != "plus" || int(cached["ts"].(float64)) != 1_786_838_400 {
		t.Fatalf("gpt cache = %#v", cached)
	}
	if _, err := os.Stat(strings.TrimSuffix(cachePath, ".json") + ".lock"); !os.IsNotExist(err) {
		t.Fatalf("GPT refresh lock survived completion: %v", err)
	}
}

func TestRenderSchedulesStaleRefreshersWithoutWaiting(t *testing.T) {
	root := t.TempDir()
	spawned := make(chan RefreshKind, 2)
	started := time.Now()
	_, err := Render(context.Background(), []byte(`{"model":{"display_name":"Claude"}}`), Runtime{
		Now:          func() time.Time { return time.Unix(1_786_838_400, 0) },
		Home:         root,
		ConfigDir:    filepath.Join(root, ".cc", "4"),
		CacheDir:     filepath.Join(root, "cache"),
		RateLimitDir: filepath.Join(root, "rates"),
		TmuxDir:      filepath.Join(root, "tmux"),
		ProcRoot:     filepath.Join(root, "proc"),
		UID:          1000,
		Engine:       "codex",
		Env:          map[string]string{},
		Command:      quietRunner{},
		Spawn: func(kind RefreshKind) error {
			spawned <- kind
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
		t.Fatalf("render blocked for %v", elapsed)
	}
	seen := map[RefreshKind]bool{}
	for index := 0; index < 2; index++ {
		select {
		case kind := <-spawned:
			seen[kind] = true
		default:
			t.Fatalf("only %d refreshers spawned: %#v", index, seen)
		}
	}
	if !seen[RefreshKindVertex] || !seen[RefreshKindGPT] {
		t.Fatalf("spawned = %#v", seen)
	}
	_, err = Render(context.Background(), []byte(`{"model":{"display_name":"Claude"}}`), Runtime{
		Now:          func() time.Time { return time.Unix(1_786_838_400, 0) },
		Home:         root,
		ConfigDir:    filepath.Join(root, ".cc", "4"),
		CacheDir:     filepath.Join(root, "cache"),
		RateLimitDir: filepath.Join(root, "rates"),
		TmuxDir:      filepath.Join(root, "tmux"),
		ProcRoot:     filepath.Join(root, "proc"),
		UID:          1000,
		Engine:       "codex",
		Env:          map[string]string{},
		Command:      quietRunner{},
		Spawn: func(kind RefreshKind) error {
			spawned <- kind
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case kind := <-spawned:
		t.Fatalf("lock failed to suppress duplicate %s refresher", kind)
	default:
	}
}

func mustFixture(t *testing.T, name string) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return body
}
