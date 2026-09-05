package stats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	pfmconfig "hostops/pfm/internal/config"
	pfmengine "hostops/pfm/internal/engine"
	"hostops/pfm/internal/paths"
	"hostops/pfm/internal/usagehook"
)

func TestLimitsSamplerUsesIdentityMatchedStatuslineQuotaWithoutCredentialFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv(paths.EnvHome, home)
	configDir := filepath.Join(home, ".cc", "2")
	realConfigDir := filepath.Join(home, ".claude2")
	if err := os.MkdirAll(filepath.Dir(configDir), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(realConfigDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realConfigDir, configDir); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_800_000_000, 0)
	rateDir := filepath.Join(home, "tmp", "cc-rate-limits")
	writeStatuslineQuotaFixture(t, rateDir, realConfigDir, now)

	sampler := NewLimitsSampler([]LimitAccount{{
		ID: 2, Emoji: "🥈", Engine: pfmengine.Claude, Label: "account 2", ConfigDir: configDir,
	}})
	sampler.Now = func() time.Time { return now }
	var acks int
	sampler.Ack = func(context.Context, LimitAccount) error {
		acks++
		return fmt.Errorf("credential refresh must not run")
	}
	limits, warnings := sampler.Sample(context.Background())
	if acks != 0 || len(warnings) != 0 || len(limits) != 1 || len(limits[0].Windows) != 2 {
		t.Fatalf("acks=%d warnings=%v limits=%#v, want two statusline-confirmed windows", acks, warnings, limits)
	}
	if limits[0].Windows[0].UsedPct != 31 || limits[0].Windows[1].UsedPct != 47 ||
		!limits[0].ConfirmedAt.Equal(now) {
		t.Fatalf("statusline quota provenance = %#v, want 31/47 confirmed at %s", limits[0], now)
	}
}

func TestLimitsSamplerRejectsStatuslineQuotaFromPreviousAccountIdentity(t *testing.T) {
	home := t.TempDir()
	t.Setenv(paths.EnvHome, home)
	// A path may legitimately contain HTTP-looking digits. Credential
	// classification must inspect the error semantics, never arbitrary path
	// substrings such as this directory name.
	currentConfig := filepath.Join(home, "403", ".cc", "2")
	if err := os.MkdirAll(currentConfig, 0o700); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_800_000_000, 0)
	writeStatuslineQuotaFixture(t, filepath.Join(home, "tmp", "cc-rate-limits"), filepath.Join(home, ".old-account-2"), now)

	sampler := NewLimitsSampler([]LimitAccount{{ID: 2, Engine: pfmengine.Claude, Label: "account 2", ConfigDir: currentConfig}})
	sampler.Now = func() time.Time { return now }
	var acks int
	sampler.Ack = func(context.Context, LimitAccount) error {
		acks++
		return nil
	}
	limits, warnings := sampler.Sample(context.Background())
	if acks != 0 || len(warnings) != 1 || len(limits) != 1 || len(limits[0].Windows) != 0 ||
		!strings.Contains(limits[0].Status, ".credentials.json") {
		t.Fatalf("acks=%d warnings=%v limits=%#v, want old identity refused and missing credentials surfaced", acks, warnings, limits)
	}
}

func writeStatuslineQuotaFixture(t *testing.T, rateDir, configDir string, now time.Time) {
	t.Helper()
	if err := os.MkdirAll(rateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	snapshot, err := json.Marshal(map[string]any{
		"acct":                2,
		"config_dir":          configDir,
		"five_hour_used":      31,
		"seven_day_used":      47,
		"five_hour_resets_at": now.Add(4 * time.Hour).Unix(),
		"seven_day_resets_at": now.Add(6 * 24 * time.Hour).Unix(),
		"ts":                  now.Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rateDir, "acct-2.session.json"), append(snapshot, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLimitsSamplerMapsCanonicalAndScopedWindowsAndCaches(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	five, seven := 12.0, 34.0
	var calls int
	sampler := NewLimitsSampler([]LimitAccount{{ID: 2, Emoji: "🔹", Engine: pfmengine.Claude, ConfigDir: "config"}})
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
	sampler := NewLimitsSampler([]LimitAccount{{ID: 7, Engine: pfmengine.Claude, ConfigDir: "config"}})
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
	now = now.Add(defaultLimitsTTL + time.Minute)
	_, warnings = sampler.Sample(context.Background())
	if acks != 1 || fetches != 2 || len(warnings) != 0 {
		t.Fatalf("expired sample fetches=%d acks=%d warnings=%v", fetches, acks, warnings)
	}
}

func TestDefaultLimitsTTLMatchesSharedUsageCacheCadence(t *testing.T) {
	if got := NewLimitsSampler(nil).ttl(); got != 3*time.Minute {
		t.Fatalf("default Limits TTL=%s, want the shared usage cache's 3m cadence", got)
	}
}

func TestLimitsSamplerTurnsPersistentCredentialRejectionIntoNamedSkip(t *testing.T) {
	home := "/home/test"
	configDir := pfmconfig.DefaultAccountDir(home, 3)
	label := pfmconfig.DisplayAccountDir(home, 3, configDir)
	var fetches, acks int
	sampler := NewLimitsSampler([]LimitAccount{{
		ID: 3, Engine: pfmengine.Claude, Label: label, ConfigDir: configDir,
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

func TestLimitsSamplerCredentialRefreshCanRetryBeforeBackoff(t *testing.T) {
	home := t.TempDir()
	t.Setenv(paths.EnvHome, home)
	configDir := filepath.Join(home, ".cc", "6")
	writeFixtureCredentials(t, configDir)

	now := time.Unix(1_800_000_000, 0)
	var hits, acks int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		if hits == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, usageJSONBody(8, 19, now))
	}))
	defer server.Close()

	sampler := NewLimitsSampler([]LimitAccount{{ID: 6, Engine: pfmengine.Claude, Label: "account 6", ConfigDir: configDir}})
	sampler.Now = func() time.Time { return now }
	sampler.Endpoint = server.URL
	sampler.Ack = func(context.Context, LimitAccount) error {
		acks++
		return nil
	}
	limits, warnings := sampler.Sample(context.Background())
	if hits != 2 || acks != 1 || len(warnings) != 0 || len(limits) != 1 || len(limits[0].Windows) != 2 {
		t.Fatalf("credential refresh retry was blocked: hits=%d acks=%d warnings=%v limits=%#v", hits, acks, warnings, limits)
	}
}

func TestStaleStatusClassifiesTimeout(t *testing.T) {
	err := fmt.Errorf("fetch usage endpoint: %w", context.DeadlineExceeded)
	if got := staleStatus(err); got != "refresh timed out; showing cached limits" {
		t.Fatalf("staleStatus(timeout)=%q", got)
	}
}

func TestLocalCredentialFileErrorsDoNotTriggerLiveAckRefresh(t *testing.T) {
	for _, message := range []string{
		"stat usage credentials: permission denied",
		"read usage credentials: input/output error",
		"decode usage credentials: invalid character",
	} {
		if needsCredentialRefresh(errors.New(message)) {
			t.Fatalf("local I/O error routed to live credential refresh: %q", message)
		}
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
		Engine: pfmengine.Codex, Label: "Codex", CodexAuthPath: authPath,
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

func TestCodexWindowsIncludesEveryAppServerBucket(t *testing.T) {
	usage := codexUsage{
		PlanType: "pro",
		RateLimit: &codexRateLimitBucket{
			LimitID: "codex",
			PrimaryWindow: &codexRateLimitWindow{
				UsedPercent: 31, LimitWindowSeconds: 604_800, ResetAt: 1_787_821_172,
			},
		},
		RateLimitsByLimitID: map[string]codexRateLimitBucket{
			"codex": {
				LimitID: "codex",
				PrimaryWindow: &codexRateLimitWindow{
					UsedPercent: 31, LimitWindowSeconds: 604_800, ResetAt: 1_787_821_172,
				},
			},
			"codex_bengalfox": {
				LimitID: "codex_bengalfox", LimitName: "GPT-5.3-Codex-Spark",
				PrimaryWindow: &codexRateLimitWindow{
					UsedPercent: 5, LimitWindowSeconds: 18_000, ResetAt: 1_787_530_676,
				},
				SecondaryWindow: &codexRateLimitWindow{
					UsedPercent: 8, LimitWindowSeconds: 604_800, ResetAt: 1_788_117_476,
				},
			},
		},
	}
	windows := codexWindows(usage)
	if len(windows) != 3 {
		t.Fatalf("windows=%#v, want base weekly plus both Spark windows", windows)
	}
	for index, want := range []string{"7d", "5h-spark", "7d-spark"} {
		if windows[index].Name != want {
			t.Fatalf("window %d name=%q, want %q (all=%#v)", index, windows[index].Name, want, windows)
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
		Engine: pfmengine.Codex, Label: "Codex", CodexAuthPath: authPath,
	}})
	sampler.CodexEndpoint = endpoint
	sampler.CodexClient = client
	return sampler.Sample(context.Background())
}

func TestLimitsSamplerKeepsRealFetchFailureVisible(t *testing.T) {
	sampler := NewLimitsSampler([]LimitAccount{{ID: 2, Engine: pfmengine.Claude, Label: "account 2"}})
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

func TestLimitsSamplerServesLastGoodClaudeCacheDuringSharedBackoff(t *testing.T) {
	home := t.TempDir()
	t.Setenv(paths.EnvHome, home)
	configDir := filepath.Join(home, ".cc", "8")
	writeFixtureCredentials(t, configDir)

	now := time.Unix(1_800_000_000, 0)
	failed := false
	var hits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		if failed {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, usageJSONBody(17, 29, now))
	}))
	defer server.Close()

	account := LimitAccount{ID: 8, Engine: pfmengine.Claude, Label: "account 8", ConfigDir: configDir}
	samplerA := NewLimitsSampler([]LimitAccount{account})
	samplerA.Now = func() time.Time { return now }
	samplerA.Endpoint = server.URL
	first, warnings := samplerA.Sample(context.Background())
	if hits != 1 || len(warnings) != 0 || len(first) != 1 || !first[0].ConfirmedAt.Equal(now) {
		t.Fatalf("initial sample: hits=%d warnings=%v limits=%#v", hits, warnings, first)
	}

	now = now.Add(30 * time.Second)
	samplerB := NewLimitsSampler([]LimitAccount{account})
	samplerB.Now = func() time.Time { return now }
	samplerB.Endpoint = server.URL
	cached, warnings := samplerB.Sample(context.Background())
	if hits != 1 || len(warnings) != 0 || len(cached) != 1 || !cached[0].ConfirmedAt.Equal(now.Add(-30*time.Second)) {
		t.Fatalf("fresh shared-cache sample invented confirmation time: hits=%d warnings=%v limits=%#v", hits, warnings, cached)
	}

	failed = true
	now = now.Add(defaultLimitsTTL)
	stale, warnings := samplerA.Sample(context.Background())
	if hits != 2 || len(stale) != 1 || len(stale[0].Windows) != 2 || stale[0].Status != "provider temporarily unavailable; showing cached limits" {
		t.Fatalf("failed refresh discarded last good payload: hits=%d warnings=%v limits=%#v", hits, warnings, stale)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "503 Service Unavailable") || !stale[0].ConfirmedAt.Equal(now.Add(-defaultLimitsTTL-30*time.Second)) {
		t.Fatalf("failed refresh lost diagnostic/provenance: warnings=%v limits=%#v", warnings, stale)
	}

	now = now.Add(30 * time.Second)
	samplerC := NewLimitsSampler([]LimitAccount{account})
	samplerC.Now = func() time.Time { return now }
	samplerC.Endpoint = server.URL
	backedOff, warnings := samplerC.Sample(context.Background())
	if hits != 2 || len(backedOff) != 1 || len(backedOff[0].Windows) != 2 || backedOff[0].Status != stale[0].Status {
		t.Fatalf("shared backoff refetched or lost cache: hits=%d warnings=%v limits=%#v", hits, warnings, backedOff)
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

	account := LimitAccount{ID: 1, Engine: pfmengine.Claude, Label: "account 1", ConfigDir: configDir}

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
	sampler := NewLimitsSampler([]LimitAccount{{ID: 4, Engine: pfmengine.Claude, ConfigDir: configDir}})
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

	account := LimitAccount{ID: 5, Engine: pfmengine.Claude, Label: "account 5", ConfigDir: configDir}

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

	sampler := NewLimitsSampler([]LimitAccount{{ID: 1, Engine: pfmengine.Claude, Label: "account 1", ConfigDir: configDir}})
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

func TestLimitsSamplerLiveRefreshDoesNotBlockOtherAccounts(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan int, 2)
	completed := make(chan int, 2)
	var calls atomic.Int32

	sampler := NewLimitsSampler([]LimitAccount{
		{ID: 1, Engine: pfmengine.Claude, Label: "account 1"},
		{ID: 2, Engine: pfmengine.Claude, Label: "account 2"},
	})
	sampler.Now = func() time.Time { return now }
	sampler.Fetch = func(ctx context.Context, account LimitAccount) (usagehook.Usage, error) {
		calls.Add(1)
		started <- account.ID
		if account.ID == 1 {
			<-ctx.Done()
			completed <- account.ID
			return usagehook.Usage{}, ctx.Err()
		}
		completed <- account.ID
		return liveClaudeUsage(now, 22), nil
	}

	type sampleResult struct {
		limits   []AccountLimits
		warnings []string
	}
	result := make(chan sampleResult, 1)
	go func() {
		limits, warnings := sampler.SampleLive(ctx)
		result <- sampleResult{limits, warnings}
	}()
	var initial sampleResult
	select {
	case initial = <-result:
	case <-time.After(time.Second):
		t.Fatal("live sampling blocked behind a provider")
	}
	limits, warnings := initial.limits, initial.warnings
	if len(limits) != 2 || len(warnings) != 0 {
		t.Fatalf("initial live sample limits=%#v warnings=%v, want two immediate cards", limits, warnings)
	}
	seen := map[int]bool{}
	for range 2 {
		seen[waitForLimitSignal(t, started)] = true
	}
	if !seen[1] || !seen[2] {
		t.Fatalf("refreshes started for accounts=%v, want both blocked account 1 and healthy account 2", seen)
	}
	if got := waitForLimitSignal(t, completed); got != 2 {
		t.Fatalf("completed refresh account=%d, want fast account 2", got)
	}
	waitForCachedWindow(t, sampler, 2, 22)

	limits, warnings = sampler.SampleLive(ctx)
	if len(warnings) != 0 || len(limits) != 2 || len(limits[1].Windows) != 2 || limits[1].Windows[0].UsedPct != 22 {
		t.Fatalf("fast account was not usable while account 1 was blocked: limits=%#v warnings=%v", limits, warnings)
	}
	if !strings.Contains(limits[0].Status, "refreshing") {
		t.Fatalf("blocked account status=%q, want explicit refreshing state", limits[0].Status)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("live refresh calls=%d, want one worker per account", got)
	}
	cancel()
	if got := waitForLimitSignal(t, completed); got != 1 {
		t.Fatalf("canceled refresh account=%d, want account 1", got)
	}
}

func TestLimitsSamplerLiveRefreshUsesSingleFlightPerAccount(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	ctx := context.Background()
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	completed := make(chan struct{}, 1)
	var calls atomic.Int32

	sampler := NewLimitsSampler([]LimitAccount{{ID: 3, Engine: pfmengine.Claude, Label: "account 3"}})
	sampler.Now = func() time.Time { return now }
	sampler.Fetch = func(ctx context.Context, _ LimitAccount) (usagehook.Usage, error) {
		calls.Add(1)
		started <- struct{}{}
		select {
		case <-release:
			completed <- struct{}{}
			return liveClaudeUsage(now, 33), nil
		case <-ctx.Done():
			return usagehook.Usage{}, ctx.Err()
		}
	}

	sampler.SampleLive(ctx)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("first live refresh did not start")
	}
	sampler.SampleLive(ctx)
	if got := calls.Load(); got != 1 {
		t.Fatalf("duplicate live workers=%d, want single-flight account refresh", got)
	}
	close(release)
	select {
	case <-completed:
	case <-time.After(time.Second):
		t.Fatal("single live refresh did not complete")
	}
}

func TestLimitsSamplerLiveTTLDoesNotExtendProviderConfirmationAge(t *testing.T) {
	var clock atomic.Int64
	start := time.Unix(1_800_000_000, 0)
	clock.Store(start.UnixNano())
	started := make(chan int, 2)
	release := make(chan struct{})
	var calls atomic.Int32

	sampler := NewLimitsSampler([]LimitAccount{{ID: 4, Engine: pfmengine.Claude, Label: "account 4"}})
	sampler.TTL = LiveLimitsTTL
	sampler.Now = func() time.Time { return time.Unix(0, clock.Load()) }
	sampler.Fetch = func(_ context.Context, _ LimitAccount) (usagehook.Usage, error) {
		call := int(calls.Add(1))
		started <- call
		<-release
		return liveClaudeUsage(time.Unix(0, clock.Load()), float64(call*10)), nil
	}

	sampler.SampleLive(context.Background())
	if got := waitForLimitSignal(t, started); got != 1 {
		t.Fatalf("first refresh call=%d, want 1", got)
	}
	close(release)
	waitForCachedWindow(t, sampler, 4, 10)

	clock.Store(start.Add(4 * time.Second).UnixNano())
	limits, _ := sampler.SampleLive(context.Background())
	if got := calls.Load(); got != 1 || len(limits[0].Windows) != 2 {
		t.Fatalf("fresh 4s read-through calls=%d limits=%#v, want cache without refresh", got, limits)
	}

	clock.Store(start.Add(6 * time.Second).UnixNano())
	limits, _ = sampler.SampleLive(context.Background())
	if len(limits) != 1 || len(limits[0].Windows) != 2 || limits[0].Windows[0].UsedPct != 10 {
		t.Fatalf("stale live card=%#v, want last-good windows during refresh", limits)
	}
	if got := waitForLimitSignal(t, started); got != 2 {
		t.Fatalf("expired refresh call=%d, want second provider call after 5s TTL", got)
	}
	waitForCachedWindow(t, sampler, 4, 20)
}

func TestLimitsSamplerLiveRecoversFutureAndEmptyCacheEntries(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	t.Run("future", func(t *testing.T) {
		started := make(chan struct{}, 1)
		release := make(chan struct{})
		sampler := NewLimitsSampler([]LimitAccount{{ID: 5, Engine: pfmengine.Claude, Label: "account 5"}})
		sampler.Now = func() time.Time { return now }
		sampler.Fetch = func(_ context.Context, _ LimitAccount) (usagehook.Usage, error) {
			started <- struct{}{}
			<-release
			return liveClaudeUsage(now, 55), nil
		}
		key := sampler.Accounts[0].cacheKey()
		sampler.mu.Lock()
		sampler.cache[key] = cachedLimits{
			limits: AccountLimits{Account: 5, Engine: pfmengine.Claude, Windows: []Window{{Name: "5h", UsedPct: 99}}},
			when:   now.Add(time.Minute),
		}
		sampler.mu.Unlock()

		limits, _ := sampler.SampleLive(context.Background())
		if len(limits) != 1 || len(limits[0].Windows) != 1 || limits[0].Windows[0].UsedPct != 99 {
			t.Fatalf("future cache card=%#v, want visible last-good payload while refreshing", limits)
		}
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("future cache did not trigger recovery refresh")
		}
		close(release)
		waitForCachedWindow(t, sampler, 5, 55)
	})

	t.Run("empty", func(t *testing.T) {
		var calls atomic.Int32
		var clock atomic.Int64
		clock.Store(now.UnixNano())
		sampler := NewLimitsSampler([]LimitAccount{{ID: 6, Engine: pfmengine.Claude, Label: "account 6"}})
		sampler.TTL = LiveLimitsTTL
		sampler.Now = func() time.Time { return time.Unix(0, clock.Load()) }
		sampler.Fetch = func(_ context.Context, _ LimitAccount) (usagehook.Usage, error) {
			if calls.Add(1) == 1 {
				return usagehook.Usage{}, nil
			}
			return liveClaudeUsage(now, 66), nil
		}

		limits, _ := sampler.SampleLive(context.Background())
		if len(limits) != 1 || limits[0].Status != "refreshing limits…" {
			t.Fatalf("empty cold card=%#v, want explicit refreshing state", limits)
		}
		waitForCachedStatus(t, sampler, 6, "empty usage response")
		clock.Store(now.Add(LiveLimitsTTL).UnixNano())
		limits, _ = sampler.SampleLive(context.Background())
		if !strings.Contains(limits[0].Status, "empty usage response") {
			t.Fatalf("empty cached status=%q, want failed attempt retained while retrying", limits[0].Status)
		}
		waitForCachedWindow(t, sampler, 6, 66)
		if got := calls.Load(); got != 2 {
			t.Fatalf("empty-cache recovery calls=%d, want initial empty response plus retry", got)
		}
	})
}

func TestLimitsSamplerCanceledLiveWorkerDoesNotBackoffAndCanRetry(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan int, 2)
	completed := make(chan struct{}, 1)
	var calls atomic.Int32

	sampler := NewLimitsSampler([]LimitAccount{{ID: 7, Engine: pfmengine.Claude, Label: "account 7"}})
	sampler.Now = func() time.Time { return now }
	sampler.Fetch = func(ctx context.Context, _ LimitAccount) (usagehook.Usage, error) {
		call := int(calls.Add(1))
		started <- call
		if call == 1 {
			<-ctx.Done()
			completed <- struct{}{}
			return usagehook.Usage{}, ctx.Err()
		}
		completed <- struct{}{}
		return liveClaudeUsage(now, 77), nil
	}

	sampler.SampleLive(ctx)
	if got := waitForLimitSignal(t, started); got != 1 {
		t.Fatalf("first worker=%d, want 1", got)
	}
	cancel()
	select {
	case <-completed:
	case <-time.After(time.Second):
		t.Fatal("canceled worker did not stop")
	}
	waitForNoCachedEntry(t, sampler, 7)

	sampler.SampleLive(context.Background())
	if got := waitForLimitSignal(t, started); got != 2 {
		t.Fatalf("retry worker=%d, want a new worker after cancellation", got)
	}
	waitForCachedWindow(t, sampler, 7, 77)
	if got := calls.Load(); got != 2 {
		t.Fatalf("canceled worker calls=%d, want exactly one retry and no backoff suppression", got)
	}
}

func TestLimitsSamplerLiveReturnsIndependentCachedValues(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	sampler := NewLimitsSampler([]LimitAccount{{ID: 8, Engine: pfmengine.Claude, Label: "account 8"}})
	sampler.Now = func() time.Time { return now }
	key := sampler.Accounts[0].cacheKey()
	sampler.mu.Lock()
	sampler.cache[key] = cachedLimits{
		limits:   AccountLimits{Account: 8, Engine: pfmengine.Claude, Windows: []Window{{Name: "5h", UsedPct: 18}}},
		warnings: []string{"cached warning"}, when: now,
	}
	sampler.mu.Unlock()

	limits, warnings := sampler.SampleLive(context.Background())
	if len(limits) != 1 || len(limits[0].Windows) != 1 || len(warnings) != 1 {
		t.Fatalf("cached sample limits=%#v warnings=%v", limits, warnings)
	}
	limits[0].Windows[0].UsedPct = 99
	limitsAgain, warningsAgain := sampler.SampleLive(context.Background())
	if limitsAgain[0].Windows[0].UsedPct != 18 || len(warningsAgain) != 1 || warningsAgain[0] != "cached warning" {
		t.Fatalf("cached values aliased caller memory: limits=%#v warnings=%v", limitsAgain, warningsAgain)
	}
}

func TestLimitsSamplerLiveKeepsRefreshingAcrossHours(t *testing.T) {
	var clock atomic.Int64
	start := time.Unix(1_800_000_000, 0)
	clock.Store(start.UnixNano())
	var calls atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sampler := NewLimitsSampler([]LimitAccount{{ID: 23, Engine: pfmengine.Claude, Label: "monitor"}})
	sampler.TTL = LiveLimitsTTL
	sampler.Now = func() time.Time { return time.Unix(0, clock.Load()) }
	sampler.Fetch = func(context.Context, LimitAccount) (usagehook.Usage, error) {
		return liveClaudeUsage(sampler.Now(), float64(calls.Add(1)%100)), nil
	}
	// Model two hours of uninterrupted six-second refresh cycles. The real
	// UI's two-second result poll checks the five-second TTL at this cadence.
	for tick := 0; tick <= 1200; tick++ {
		clock.Store(start.Add(time.Duration(tick) * 6 * time.Second).UnixNano())
		sampler.SampleLive(ctx)
		waitForCachedWindow(t, sampler, 23, float64((tick+1)%100))
		if got := calls.Load(); got != int32(tick+1) {
			t.Fatalf("six-second cycle %d: provider calls=%d, want %d", tick, got, tick+1)
		}
	}
}

func TestLimitsSamplerStaleRateLimitStatusPreservesRetryTime(t *testing.T) {
	home := t.TempDir()
	t.Setenv(paths.EnvHome, home)
	now := time.Unix(1_800_000_000, 0)
	account := LimitAccount{ID: 9, Engine: pfmengine.Claude, Label: "account 9", ConfigDir: filepath.Join(home, "claude")}
	confirmedAt := now.Add(-10 * time.Minute)
	usage := liveClaudeUsage(confirmedAt, 49)
	retryMessage := "limits unavailable: 429 Too Many Requests — retry at 15:04"
	if err := usagehook.WriteCacheRecord(usagehook.CachePath(usagehook.DefaultCacheDir(), account.ID), usagehook.CacheRecord{
		Usage: usage, ConfigDir: account.ConfigDir, FetchedAt: &confirmedAt,
		Backoff: &usagehook.CacheBackoff{Message: retryMessage, RetryAfter: now.Add(10 * time.Minute), RecordedAt: now},
	}); err != nil {
		t.Fatal(err)
	}

	sampler := NewLimitsSampler([]LimitAccount{account})
	sampler.Now = func() time.Time { return now }
	limits, warnings := sampler.Sample(context.Background())
	if len(limits) != 1 || len(limits[0].Windows) != 2 || limits[0].Status != "provider rate-limited; retry at 15:04; showing cached limits" {
		t.Fatalf("rate-limited stale card=%#v, want retry time and cached windows", limits)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "retry at 15:04") {
		t.Fatalf("rate-limit warnings=%v, want retry time preserved", warnings)
	}
}

func TestLimitsSamplerSuccessfulFetchReportsClaudeCacheWriteFailure(t *testing.T) {
	home := t.TempDir()
	t.Setenv(paths.EnvHome, home)
	if err := os.WriteFile(filepath.Join(home, "tmp"), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	configDir := filepath.Join(home, "claude")
	writeFixtureCredentials(t, configDir)
	now := time.Unix(1_800_000_000, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, usageJSONBody(51, 61, now))
	}))
	defer server.Close()

	sampler := NewLimitsSampler([]LimitAccount{{ID: 10, Engine: pfmengine.Claude, Label: "account 10", ConfigDir: configDir}})
	sampler.Now = func() time.Time { return now }
	sampler.Endpoint = server.URL
	sampler.Client = server.Client()
	limits, warnings := sampler.Sample(context.Background())
	if len(limits) != 1 || len(limits[0].Windows) != 2 || !strings.HasPrefix(limits[0].Status, "write Claude limits cache:") {
		t.Fatalf("successful provider fetch hid cache write error: limits=%#v", limits)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "write Claude limits cache:") {
		t.Fatalf("cache write warnings=%v, want concrete write failure", warnings)
	}
}

func liveClaudeUsage(now time.Time, used float64) usagehook.Usage {
	return usagehook.Usage{
		FiveHour: usagehook.Window{Utilization: &used, ResetsAt: now.Add(5 * time.Hour).Format(time.RFC3339)},
		SevenDay: usagehook.Window{Utilization: &used, ResetsAt: now.Add(7 * 24 * time.Hour).Format(time.RFC3339)},
	}
}

func waitForLimitSignal[T any](t *testing.T, signals <-chan T) T {
	t.Helper()
	select {
	case signal := <-signals:
		return signal
	case <-time.After(time.Second):
		var zero T
		t.Fatal("timed out waiting for live limits worker")
		return zero
	}
}

func waitForCachedWindow(t *testing.T, sampler *LimitsSampler, accountID int, used float64) {
	t.Helper()
	key := limitTestAccountKey(t, sampler, accountID)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		sampler.mu.Lock()
		entry, found := sampler.cache[key]
		_, running := sampler.flights[key]
		sampler.mu.Unlock()
		if !running && found && len(entry.limits.Windows) > 0 && entry.limits.Windows[0].UsedPct == used {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("account %d cache never reached %.0f%%", accountID, used)
}

func waitForCachedStatus(t *testing.T, sampler *LimitsSampler, accountID int, fragment string) {
	t.Helper()
	key := limitTestAccountKey(t, sampler, accountID)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		sampler.mu.Lock()
		entry, found := sampler.cache[key]
		_, running := sampler.flights[key]
		sampler.mu.Unlock()
		if !running && found && strings.Contains(entry.limits.Status, fragment) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("account %d cache status never contained %q", accountID, fragment)
}

func waitForNoCachedEntry(t *testing.T, sampler *LimitsSampler, accountID int) {
	t.Helper()
	key := limitTestAccountKey(t, sampler, accountID)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		sampler.mu.Lock()
		_, found := sampler.cache[key]
		_, running := sampler.flights[key]
		sampler.mu.Unlock()
		if !found && !running {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("account %d retained a canceled worker cache entry", accountID)
}

func limitTestAccountKey(t *testing.T, sampler *LimitsSampler, accountID int) string {
	t.Helper()
	for _, account := range sampler.Accounts {
		if account.ID == accountID {
			return account.cacheKey()
		}
	}
	t.Fatalf("fixture has no account %d", accountID)
	return ""
}
