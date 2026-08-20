package stats

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

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
	if acks != 1 || fetches != 1 || len(warnings) != 1 || !strings.Contains(warnings[0], "limits unavailable") {
		t.Fatalf("first sample fetches=%d acks=%d warnings=%v", fetches, acks, warnings)
	}
	now = now.Add(2 * time.Minute)
	_, warnings = sampler.Sample(context.Background())
	if acks != 1 || fetches != 2 || len(warnings) != 1 || !strings.Contains(warnings[0], "limits unavailable") {
		t.Fatalf("expired sample fetches=%d acks=%d warnings=%v", fetches, acks, warnings)
	}
}
