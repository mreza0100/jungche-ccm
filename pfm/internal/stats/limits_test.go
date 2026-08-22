package stats

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pfmconfig "hostops/pfm/internal/config"
	"hostops/pfm/internal/paths"
	"hostops/pfm/internal/usagehook"
)

func TestLimitsSamplerMapsCanonicalAndScopedWindowsAndCaches(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	five, seven := 12.0, 34.0
	var calls int
	sampler := NewLimitsSampler([]LimitAccount{{ID: 2, Emoji: "🔹", ConfigDir: "config"}})
	sampler.Now = func() time.Time { return now }
	sampler.Fetch = func(context.Context, LimitAccount) (usagehook.Usage, error) {
		calls++
		return usagehook.Usage{
			FiveHour: usagehook.Window{Utilization: &five, ResetsAt: now.Add(5 * time.Hour).Format(time.RFC3339)},
			SevenDay: usagehook.Window{Utilization: &seven, ResetsAt: now.Add(7 * 24 * time.Hour).Format(time.RFC3339)},
			Limits: []usagehook.ScopedLimit{
				testScopedLimit("weekly_scoped", "Fable", 23, now.Add(6*24*time.Hour), true),
			},
		}, nil
	}
	first, warnings := sampler.Sample(context.Background())
	if len(warnings) != 0 || len(first) != 1 || calls != 1 || !first[0].ConfirmedAt.Equal(now) {
		t.Fatalf("first limits=%#v warnings=%v calls=%d", first, warnings, calls)
	}
	if len(first[0].Windows) != 3 || first[0].Windows[0].Name != "5h" || first[0].Windows[1].Name != "7d" || first[0].Windows[2].Name != "7d-fable" {
		t.Fatalf("windows=%#v", first[0].Windows)
	}
	if _, warnings = sampler.Sample(context.Background()); len(warnings) != 0 || calls != 1 {
		t.Fatalf("cache warnings=%v calls=%d", warnings, calls)
	}
	if strings.TrimSpace(first[0].Windows[0].ResetAt.Format(time.RFC3339)) == "" {
		t.Fatalf("reset=%s", first[0].Windows[0].ResetAt)
	}
}

func TestLimitsSamplerACKFallbackIsAtMostOncePerAccount(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	var fetches, acks int
	sampler := NewLimitsSampler([]LimitAccount{{ID: 7, ConfigDir: "config"}})
	sampler.Now = func() time.Time { return now }
	sampler.Fetch = func(context.Context, LimitAccount) (usagehook.Usage, error) {
		fetches++
		return usagehook.Usage{}, fmt.Errorf("401 unauthorized")
	}
	sampler.Ack = func(context.Context, LimitAccount) error {
		acks++
		return fmt.Errorf("ACK refresh failed")
	}
	_, warnings := sampler.Sample(context.Background())
	if acks != 1 || fetches != 1 || len(warnings) != 0 {
		t.Fatalf("first sample fetches=%d acks=%d warnings=%v", fetches, acks, warnings)
	}
	now = now.Add(2 * time.Minute)
	_, warnings = sampler.Sample(context.Background())
	if acks != 1 || fetches != 2 || len(warnings) != 0 {
		t.Fatalf("expired sample fetches=%d acks=%d warnings=%v", fetches, acks, warnings)
	}
}

func TestLimitsSamplerTurnsPersistentCredentialRejectionIntoNamedSkip(t *testing.T) {
	home := "/home/test"
	configDir := pfmconfig.DefaultAccountDir(home, 3)
	label := pfmconfig.DisplayAccountDir(home, 3, configDir)
	var fetches, acks int
	sampler := NewLimitsSampler([]LimitAccount{{
		ID: 3, Engine: "claude", Label: label, ConfigDir: configDir,
	}})
	sampler.Fetch = func(context.Context, LimitAccount) (usagehook.Usage, error) {
		fetches++
		return usagehook.Usage{}, fmt.Errorf("usage endpoint returned 403 Forbidden")
	}
	sampler.Ack = func(context.Context, LimitAccount) error {
		acks++
		return nil
	}
	limits, warnings := sampler.Sample(context.Background())
	if fetches != 2 || acks != 1 {
		t.Fatalf("fetches=%d acks=%d, want credential refresh followed by one retry", fetches, acks)
	}
	if len(warnings) != 0 {
		t.Fatalf("credential rejection leaked as warnings: %v", warnings)
	}
	if len(limits) != 1 || limits[0].Status != "skipped "+label+": credentials rejected" {
		t.Fatalf("limits=%#v, want named stale-account skip", limits)
	}
}

func TestUsageWindowsDropsPastScopedFableAndExplainsMissingReset(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	used := 0.0
	windows := usageWindows(
		usagehook.Usage{
			FiveHour: usagehook.Window{Utilization: &used},
			Limits: []usagehook.ScopedLimit{
				testScopedLimit("weekly_scoped", "Fable", 99, now.Add(-time.Minute), true),
			},
		},
		now,
	)
	if len(windows) != 1 {
		t.Fatalf("windows=%#v, want only the 5h window", windows)
	}
	if windows[0].Name != "5h" || windows[0].ResetNote != "reset unavailable" {
		t.Fatalf("5h window=%#v", windows[0])
	}
}

func TestLimitsSamplerIgnoresStaleCodexCacheForLiveFetch(t *testing.T) {
	root := t.TempDir()
	authPath := writeCodexAuth(t, root, "fixture-access", "fixture-account")
	staleCachePath := filepath.Join(root, "stale-codex-cache.json")
	if err := os.WriteFile(staleCachePath, []byte(`{
		"primary":{"usedPercent":99,"windowDurationMins":300,"resetsAt":1800000000}
	}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var liveFetches int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		liveFetches++
		if request.Method != http.MethodGet {
			t.Errorf("method=%s, want GET", request.Method)
		}
		if got := request.Header.Get("Authorization"); got != "Bearer fixture-access" {
			t.Errorf("Authorization=%q", got)
		}
		if got := request.Header.Get("ChatGPT-Account-ID"); got != "fixture-account" {
			t.Errorf("ChatGPT-Account-ID=%q", got)
		}
		if got := request.Header.Get("Cache-Control"); got != "no-cache" {
			t.Errorf("Cache-Control=%q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"plan_type":"pro",
			"rate_limit":{
				"primary_window":{
					"used_percent":57,
					"limit_window_seconds":604800,
					"reset_at":1785902971
				},
				"secondary_window":null
			}
		}`)
	}))
	defer server.Close()
	sampler := NewLimitsSampler([]LimitAccount{{
		Engine: "codex", Label: "Codex", CodexAuthPath: authPath,
	}})
	sampler.CodexEndpoint = server.URL
	sampler.CodexClient = server.Client()
	limits, warnings := sampler.Sample(context.Background())
	if len(warnings) != 0 || len(limits) != 1 || limits[0].Status != "" || len(limits[0].Windows) != 1 {
		t.Fatalf("limits=%#v warnings=%v", limits, warnings)
	}
	window := limits[0].Windows[0]
	if liveFetches != 1 || limits[0].Plan != "pro" || limits[0].ConfirmedAt.IsZero() || window.Name != "7d" || window.UsedPct != 57 || window.ResetAt.Unix() != 1785902971 {
		t.Fatalf("liveFetches=%d limits=%#v, want the live weekly fixture and no stale-cache read", liveFetches, limits)
	}
}

func TestCodexWindowNameUsesServerSeconds(t *testing.T) {
	for _, test := range []struct {
		seconds int64
		want    string
	}{
		{seconds: 18_000, want: "5h"},
		{seconds: 604_800, want: "7d"},
		{seconds: 259_200, want: "3d"},
		{seconds: 7_200, want: "2h"},
		{seconds: 5_400, want: "90m"},
		{seconds: 45, want: "45s"},
	} {
		if got := codexWindowName(test.seconds); got != test.want {
			t.Errorf("codexWindowName(%d)=%q, want %q", test.seconds, got, test.want)
		}
	}
}

func TestLimitsSamplerKeepsCodexAuthAndPayloadFailuresVisible(t *testing.T) {
	t.Run("missing sign-in", func(t *testing.T) {
		assertCodexStatus(t, filepath.Join(t.TempDir(), "missing.json"), "", nil, "no local Codex sign-in")
	})
	t.Run("incomplete session", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "auth.json")
		if err := os.WriteFile(path, []byte(`{"tokens":{"access_token":"fixture-access"}}`), 0o600); err != nil {
			t.Fatal(err)
		}
		assertCodexStatus(t, path, "", nil, "Codex session incomplete")
	})
	for _, code := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(fmt.Sprintf("HTTP %d", code), func(t *testing.T) {
			root := t.TempDir()
			path := writeCodexAuth(t, root, "fixture-access", "fixture-account")
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(code)
			}))
			defer server.Close()
			assertCodexStatus(t, path, server.URL, server.Client(), fmt.Sprintf("Codex credential rejected (HTTP %d)", code))
		})
	}
	t.Run("unreadable payload", func(t *testing.T) {
		root := t.TempDir()
		path := writeCodexAuth(t, root, "fixture-access", "fixture-account")
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprint(w, `{"rate_limit":`)
		}))
		defer server.Close()
		assertCodexStatus(t, path, server.URL, server.Client(), "Codex payload unreadable")
	})
	t.Run("network failure", func(t *testing.T) {
		root := t.TempDir()
		path := writeCodexAuth(t, root, "fixture-access", "fixture-account")
		server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		endpoint := server.URL
		client := server.Client()
		server.Close()
		assertCodexStatusPrefix(t, path, endpoint, client, "Codex fetch failed: ")
	})
}

func TestDefaultCodexClientUsesTwentySecondTimeout(t *testing.T) {
	sampler := NewLimitsSampler(nil)
	if got := sampler.codexClient().Timeout; got != 20*time.Second {
		t.Fatalf("Codex client timeout=%s, want 20s", got)
	}
}

func testScopedLimit(kind, displayName string, percent float64, resetAt time.Time, active bool) usagehook.ScopedLimit {
	limit := usagehook.ScopedLimit{
		Kind: kind, Percent: &percent, ResetsAt: resetAt.Format(time.RFC3339), IsActive: active,
	}
	limit.Scope.Model.DisplayName = displayName
	return limit
}

func writeCodexAuth(t *testing.T, root, accessToken, accountID string) string {
	t.Helper()
	path := filepath.Join(root, "auth.json")
	body := fmt.Sprintf(`{"tokens":{"access_token":%q,"account_id":%q}}`, accessToken, accountID)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertCodexStatus(t *testing.T, authPath, endpoint string, client *http.Client, want string) {
	t.Helper()
	limits, warnings := sampleCodexStatus(authPath, endpoint, client)
	if len(limits) != 1 || limits[0].Status != want {
		t.Fatalf("limits=%#v, want status %q", limits, want)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], want) {
		t.Fatalf("warnings=%v, want visible %q diagnostic", warnings, want)
	}
}

func assertCodexStatusPrefix(t *testing.T, authPath, endpoint string, client *http.Client, want string) {
	t.Helper()
	limits, warnings := sampleCodexStatus(authPath, endpoint, client)
	if len(limits) != 1 || !strings.HasPrefix(limits[0].Status, want) {
		t.Fatalf("limits=%#v, want status prefix %q", limits, want)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], want) {
		t.Fatalf("warnings=%v, want visible %q diagnostic", warnings, want)
	}
}

func sampleCodexStatus(authPath, endpoint string, client *http.Client) ([]AccountLimits, []string) {
	sampler := NewLimitsSampler([]LimitAccount{{
		Engine: "codex", Label: "Codex", CodexAuthPath: authPath,
	}})
	sampler.CodexEndpoint = endpoint
	sampler.CodexClient = client
	return sampler.Sample(context.Background())
}

func TestLimitsSamplerKeepsRealFetchFailureVisible(t *testing.T) {
	sampler := NewLimitsSampler([]LimitAccount{{ID: 2, Engine: "claude", Label: "account 2"}})
	sampler.Fetch = func(context.Context, LimitAccount) (usagehook.Usage, error) {
		return usagehook.Usage{}, fmt.Errorf("network route unavailable")
	}
	limits, warnings := sampler.Sample(context.Background())
	if len(limits) != 1 || !strings.Contains(limits[0].Status, "network route unavailable") {
		t.Fatalf("limits=%#v, want visible real-account error", limits)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "network route unavailable") {
		t.Fatalf("warnings=%v, want real-account diagnostic", warnings)
	}
}

// --- shared on-disk cache regression coverage -------------------------------
//
// Every picker process constructs its own LimitsSampler (cmd/pfm/commands.go,
// `statsSampler.Limits = pfmstats.NewLimitsSampler(...)` runs once per `pfm ls`
// invocation). LimitsSampler.cache today (limits.go:47-48) is an unexported,
// in-process map — nothing on disk backs it — so two samplers standing in for
// two concurrently open pickers each pay their own TTL and their own fetch,
// exactly like two independent processes would on a real host. The tests below
// construct a fresh LimitsSampler per "process" (never reusing a Go value) and
// assert on observable network-call counts, matching the existing package's
// httptest-server style (TestLimitsSamplerIgnoresStaleCodexCacheForLiveFetch
// above). None of them add a field to LimitsSampler or usagehook.Options —
// they drive the contract entirely through what already compiles today, so a
// failure below is a real, running assertion mismatch, never a build error.

// writeFixtureCredentials plants an invented, secret-free OAuth credential so
// usagehook.Fetch / usagehook.Evaluate treat configDir as a signed-in account.
func writeFixtureCredentials(t *testing.T, configDir string) {
	t.Helper()
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	body := `{"claudeAiOauth":{"accessToken":"fixture-oauth-token-not-a-real-secret"}}`
	if err := os.WriteFile(filepath.Join(configDir, ".credentials.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// usageJSONBody renders a minimal, valid usagehook.Usage payload — the exact
// wire shape both usagehook.Fetch and usagehook.Evaluate decode.
func usageJSONBody(five, seven float64, now time.Time) string {
	return fmt.Sprintf(
		`{"five_hour":{"utilization":%v,"resets_at":%q},"seven_day":{"utilization":%v,"resets_at":%q}}`,
		five, now.Add(5*time.Hour).Format(time.RFC3339),
		seven, now.Add(7*24*time.Hour).Format(time.RFC3339),
	)
}

// TestLimitsSamplerSharesFetchAcrossProcessesWithinTTL pins the shared-cache
// contract: a second `pfm ls` opened moments after the first must reuse
// whatever the first one just fetched instead of making its own request.
//
// Fails at HEAD: LimitsSampler has no on-disk cache at all (limits.go:47-48
// `cache map[string]cachedLimits` lives only on the struct value), so sampler
// B — a brand-new value, exactly like a second process — starts with an empty
// cache and fetches again. hits ends at 2, not 1.
func TestLimitsSamplerSharesFetchAcrossProcessesWithinTTL(t *testing.T) {
	configDir := t.TempDir()
	writeFixtureCredentials(t, configDir)
	five, seven := 11.0, 22.0
	now := time.Now()
	var hits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, usageJSONBody(five, seven, now))
	}))
	defer server.Close()

	account := LimitAccount{ID: 1, Engine: "claude", Label: "account 1", ConfigDir: configDir}

	samplerA := NewLimitsSampler([]LimitAccount{account})
	samplerA.Endpoint = server.URL
	limitsA, warningsA := samplerA.Sample(context.Background())
	if hits != 1 || len(warningsA) != 0 || len(limitsA) != 1 || len(limitsA[0].Windows) != 2 {
		t.Fatalf("sampler A (first process): hits=%d warnings=%v limits=%#v, want one clean fetch", hits, warningsA, limitsA)
	}

	// A fresh LimitsSampler value, same account, same endpoint — a second
	// picker process opened a moment later against the same host state.
	samplerB := NewLimitsSampler([]LimitAccount{account})
	samplerB.Endpoint = server.URL
	limitsB, warningsB := samplerB.Sample(context.Background())
	if hits != 1 {
		t.Fatalf("sampler B (second process) fetched independently: hits=%d, want 1 (shared cache hit, 0 new fetches)", hits)
	}
	if len(warningsB) != 0 || len(limitsB) != 1 || len(limitsB[0].Windows) != len(limitsA[0].Windows) {
		t.Fatalf("sampler B limits=%#v warnings=%v, want the same windows sampler A already fetched", limitsB, warningsB)
	}
}

// TestLimitsSamplerRespectsShortTTLWithinAndAcrossWindow is not a regression
// test — the single-sampler in-memory TTL already works today. It pins the
// TTL boundary itself (300ms: 0/100/200ms stay cached, 400ms refetches) so a
// shared-cache rewrite of refresh()/cached() cannot silently widen or drop
// the existing per-sampler freshness window while fixing the cross-process
// gap above.
func TestLimitsSamplerRespectsShortTTLWithinAndAcrossWindow(t *testing.T) {
	configDir := t.TempDir()
	writeFixtureCredentials(t, configDir)
	var hits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, usageJSONBody(10, 20, time.Now()))
	}))
	defer server.Close()

	now := time.Unix(1_800_000_000, 0)
	sampler := NewLimitsSampler([]LimitAccount{{ID: 4, Engine: "claude", ConfigDir: configDir}})
	sampler.Endpoint = server.URL
	sampler.TTL = 300 * time.Millisecond
	sampler.Now = func() time.Time { return now }

	if _, warnings := sampler.Sample(context.Background()); len(warnings) != 0 || hits != 1 {
		t.Fatalf("t=0: hits=%d warnings=%v, want 1 clean fetch", hits, warnings)
	}
	now = now.Add(100 * time.Millisecond)
	sampler.Sample(context.Background())
	now = now.Add(100 * time.Millisecond) // t=200ms
	sampler.Sample(context.Background())
	if hits != 1 {
		t.Fatalf("hits=%d at t=200ms, want 1 (still inside the 300ms TTL)", hits)
	}
	now = now.Add(200 * time.Millisecond) // t=400ms
	sampler.Sample(context.Background())
	if hits != 2 {
		t.Fatalf("hits=%d at t=400ms, want 2 (TTL expired, one refetch)", hits)
	}
}

// TestLimitsSamplerBacksOff429AcrossProcessesForAtLeastTenMinutes pins the
// 429-backoff half of the shared-cache contract: a rate-limited response must
// be recorded where every sampler can see it, and a fresh sampler standing in
// for a second process must not repeat the request that just got rate-limited.
//
// Fails at HEAD for the same reason as the fetch-sharing test above: sampler
// B starts with an empty in-memory cache and has no shared record of A's 429,
// so it fetches again — hits ends at 2, not 1.
func TestLimitsSamplerBacksOff429AcrossProcessesForAtLeastTenMinutes(t *testing.T) {
	configDir := t.TempDir()
	writeFixtureCredentials(t, configDir)
	var hits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	account := LimitAccount{ID: 5, Engine: "claude", Label: "account 5", ConfigDir: configDir}

	samplerA := NewLimitsSampler([]LimitAccount{account})
	samplerA.Endpoint = server.URL
	limitsA, _ := samplerA.Sample(context.Background())
	if hits != 1 || len(limitsA) != 1 || !strings.Contains(limitsA[0].Status, "429") {
		t.Fatalf("sampler A: hits=%d limits=%#v, want one recorded 429", hits, limitsA)
	}

	samplerB := NewLimitsSampler([]LimitAccount{account})
	samplerB.Endpoint = server.URL
	limitsB, _ := samplerB.Sample(context.Background())
	if hits != 1 {
		t.Fatalf("sampler B retried during the shared 429 backoff window: hits=%d, want 1 (no new request)", hits)
	}
	if len(limitsB) != 1 || !strings.Contains(limitsB[0].Status, "429") {
		t.Fatalf("sampler B limits=%#v, want the shared 429 status surfaced without a fetch", limitsB)
	}
}

// TestLimitsSamplerReadsCachePayloadTheHookWroteWithoutFetching pins the other
// direction of the same shared cache: the UserPromptSubmit hook
// (usagehook.Evaluate, hook.go:182-215) already writes a shared, on-disk
// acct-<id>.json through its own refresh() (hook.go:280-284 for the CacheDir
// default, keyed off PFM_HOME exactly like every other jail override in this
// package — see pfm/CLAUDE.md § Environment Variables). Once LimitsSampler
// reads that same file/location, a picker opened right after a prompt already
// ran must render instantly from the hook's cache instead of firing its own
// request.
//
// Fails at HEAD: LimitsSampler never looks at CacheDir/PFM_HOME or any
// acct-<id>.json at all — refresh() (limits.go:151) goes straight to
// sampler.Fetch — so it fetches from the network regardless of what the hook
// already wrote. samplerHits ends at 1, not 0.
func TestLimitsSamplerReadsCachePayloadTheHookWroteWithoutFetching(t *testing.T) {
	home := t.TempDir()
	t.Setenv(paths.EnvHome, home)
	configDir := filepath.Join(home, ".claude")
	writeFixtureCredentials(t, configDir)

	five, seven := 41.0, 62.0
	now := time.Now()

	var hookHits int
	hookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hookHits++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, usageJSONBody(five, seven, now))
	}))
	defer hookServer.Close()

	// Plant the shared cache file the way a running host already does today:
	// through the hook's own writer (usagehook.Evaluate -> refresh ->
	// atomicWrite, hook.go:280-284/300-304), before any LimitsSampler runs.
	if _, err := usagehook.Evaluate(context.Background(), usagehook.Options{
		Home: home, ConfigDir: configDir, Endpoint: hookServer.URL, Client: hookServer.Client(),
	}); err != nil {
		t.Fatalf("plant hook cache via usagehook.Evaluate: %v", err)
	}
	if hookHits != 1 {
		t.Fatalf("hook fixture setup hits=%d, want exactly 1 (the cache-planting fetch)", hookHits)
	}

	var samplerHits int
	limitsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		samplerHits++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, usageJSONBody(five, seven, now))
	}))
	defer limitsServer.Close()

	sampler := NewLimitsSampler([]LimitAccount{{ID: 1, Engine: "claude", Label: "account 1", ConfigDir: configDir}})
	sampler.Endpoint = limitsServer.URL
	limits, warnings := sampler.Sample(context.Background())
	if samplerHits != 0 {
		t.Fatalf("sampler fetched instead of reading the hook's cache: samplerHits=%d, want 0", samplerHits)
	}
	if len(warnings) != 0 || len(limits) != 1 || len(limits[0].Windows) != 2 {
		t.Fatalf("limits=%#v warnings=%v, want the hook-planted windows with no warnings", limits, warnings)
	}
	if limits[0].Windows[0].UsedPct != five || limits[0].Windows[1].UsedPct != seven {
		t.Fatalf("windows=%#v, want five=%v seven=%v read from the planted cache", limits[0].Windows, five, seven)
	}
}
