package stats

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pfmconfig "hostops/pfm/internal/config"
	"hostops/pfm/internal/usagehook"
)

func TestLimitsSamplerMapsTypedAndGenericWindowsAndCaches(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	used := 23.0
	var calls int
	sampler := NewLimitsSampler([]LimitAccount{{ID: 2, Emoji: "🔹", ConfigDir: "config"}})
	sampler.Now = func() time.Time { return now }
	sampler.Fetch = func(context.Context, LimitAccount) (usagehook.Usage, error) {
		calls++
		return usagehook.Usage{
			SevenFable: usagehook.Window{Utilization: &used, ResetsAt: now.Add(24 * time.Hour).Format(time.RFC3339)},
			Extra: map[string]usagehook.Window{
				"seven_day_sonnet": {Utilization: &used, ResetsAt: now.Add(48 * time.Hour).Format(time.RFC3339)},
			},
		}, nil
	}
	first, warnings := sampler.Sample(context.Background())
	if len(warnings) != 0 || len(first) != 1 || calls != 1 {
		t.Fatalf("first limits=%#v warnings=%v calls=%d", first, warnings, calls)
	}
	if len(first[0].Windows) != 2 || first[0].Windows[0].Name != "7d-fable" {
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

func TestUsageWindowsNamesUnknownAndExplainsMissingReset(t *testing.T) {
	used := 0.0
	windows := usageWindows(usagehook.Usage{
		SevenFable: usagehook.Window{Utilization: &used},
		Extra: map[string]usagehook.Window{
			"seven_day_nimbus_quill": {Utilization: &used},
		},
	})
	if len(windows) != 2 {
		t.Fatalf("windows=%#v, want Fable and unknown", windows)
	}
	if windows[0].Name != "7d-fable" || windows[0].ResetNote != "reset unavailable" {
		t.Fatalf("Fable window=%#v", windows[0])
	}
	if windows[1].Name != "unknown[seven_day_nimbus_quill]" || windows[1].ResetNote != "reset unavailable" {
		t.Fatalf("unknown window=%#v", windows[1])
	}
}

func TestLimitsSamplerKeepsCodexCacheReadFailureAsVisibleRow(t *testing.T) {
	sampler := NewLimitsSampler([]LimitAccount{{
		Engine: "codex", Label: "Codex", CodexCachePath: filepath.Join(t.TempDir(), "missing.json"),
	}})
	sampler.Fetch = func(context.Context, LimitAccount) (usagehook.Usage, error) {
		t.Fatal("Codex cache failure fell through to the Claude usage endpoint")
		return usagehook.Usage{}, nil
	}
	limits, warnings := sampler.Sample(context.Background())
	if len(limits) != 1 || limits[0].Label != "Codex" || !strings.Contains(limits[0].Status, "read Codex cache") {
		t.Fatalf("limits=%#v, want visible Codex read failure", limits)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "read Codex cache") {
		t.Fatalf("warnings=%v, want Codex read diagnostic", warnings)
	}
}

func TestLimitsSamplerRendersCodexCacheWindows(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "codex.json")
	if err := os.WriteFile(cachePath, []byte(`{
		"primary":{"usedPercent":32,"windowDurationMins":300,"resetsAt":1800000000},
		"secondary":{"usedPercent":71,"windowDurationMins":10080,"resetsAt":1800086400}
	}`), 0o600); err != nil {
		t.Fatal(err)
	}
	sampler := NewLimitsSampler([]LimitAccount{{
		Engine: "codex", Label: "Codex", CodexCachePath: cachePath,
	}})
	limits, warnings := sampler.Sample(context.Background())
	if len(warnings) != 0 || len(limits) != 1 || limits[0].Status != "" || len(limits[0].Windows) != 2 {
		t.Fatalf("limits=%#v warnings=%v", limits, warnings)
	}
	if limits[0].Windows[0].Name != "codex-5h" || limits[0].Windows[1].Name != "codex-7d" {
		t.Fatalf("Codex windows=%#v", limits[0].Windows)
	}
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
