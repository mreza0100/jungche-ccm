package usagehook

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

func TestUsageParsesFableAndRetainsUnknownWindows(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "usage-fable.json"))
	if err != nil {
		t.Fatal(err)
	}
	var parsed Usage
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("decode usage fixture: %v", err)
	}
	if parsed.SevenFable.Utilization == nil || int(*parsed.SevenFable.Utilization) != 23 {
		t.Fatalf("fable window = %#v", parsed.SevenFable)
	}
	unknown, ok := parsed.Extra["seven_day_sonnet"]
	if !ok || unknown.Utilization == nil || int(*unknown.Utilization) != 17 {
		t.Fatalf("unknown windows = %#v", parsed.Extra)
	}
}

func TestNamedWindowsUsesOneCanonicalMappingAndLabelsUnknownKeys(t *testing.T) {
	var parsed Usage
	content := []byte(`{
		"seven_day":{"utilization":11,"resets_at":"2026-08-28T00:00:00Z"},
		"seven_day_opus":{"utilization":22,"resets_at":"2026-08-28T00:00:00Z"},
		"seven_day_fable":{"utilization":0,"resets_at":""},
		"seven_day_nimbus_quill":{"utilization":33,"resets_at":"2026-08-28T00:00:00Z"}
	}`)
	if err := json.Unmarshal(content, &parsed); err != nil {
		t.Fatal(err)
	}
	windows := parsed.NamedWindows()
	want := []struct {
		key, label string
		known      bool
	}{
		{key: "seven_day", label: "7d", known: true},
		{key: "seven_day_opus", label: "7d-opus", known: true},
		{key: "seven_day_fable", label: "7d-fable", known: true},
		{key: "seven_day_nimbus_quill", label: "unknown[seven_day_nimbus_quill]", known: false},
	}
	if len(windows) != len(want) {
		t.Fatalf("NamedWindows() = %#v, want %d windows", windows, len(want))
	}
	for index, expected := range want {
		if windows[index].Key != expected.key || windows[index].Label != expected.label || windows[index].Known != expected.known {
			t.Fatalf("NamedWindows()[%d] = %#v, want %#v", index, windows[index], expected)
		}
	}
}

func TestWarnRecoverQuietTransition(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, ".claude")
	cacheDir := filepath.Join(root, "cache")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(configDir, ".credentials.json"),
		[]byte(`{"claudeAiOauth":{"accessToken":"fixture-token"}}`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	options := Options{
		Now:       func() time.Time { return time.Unix(1_786_838_400, 0) },
		Home:      root,
		ConfigDir: configDir,
		CacheDir:  cacheDir,
		Warn:      80,
		Critical:  95,
		TTL:       24 * time.Hour,
	}
	seed := func(five, seven int) {
		t.Helper()
		if err := os.MkdirAll(cacheDir, 0o700); err != nil {
			t.Fatal(err)
		}
		body := []byte(`{"five_hour":{"utilization":` + itoa(five) +
			`,"resets_at":"2030-01-01T10:00:00Z"},` +
			`"seven_day":{"utilization":` + itoa(seven) +
			`,"resets_at":"2030-01-03T08:00:00Z"},` +
			`"seven_day_opus":{"utilization":null}}`)
		if err := os.WriteFile(filepath.Join(cacheDir, "acct-1.json"), body, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	seed(20, 96)
	warn, err := Evaluate(context.Background(), options)
	if err != nil || !strings.Contains(warn, "USAGE LIMIT IMMINENT") {
		t.Fatalf("warn=%q err=%v", warn, err)
	}
	seed(13, 3)
	recovered, err := Evaluate(context.Background(), options)
	if err != nil || !strings.Contains(recovered, "usage recovered") ||
		!strings.Contains(recovered, "STALE") {
		t.Fatalf("recovered=%q err=%v", recovered, err)
	}
	quiet, err := Evaluate(context.Background(), options)
	if err != nil || quiet != "" {
		t.Fatalf("quiet=%q err=%v", quiet, err)
	}
	seed(96, 20)
	rewarn, err := Evaluate(context.Background(), options)
	if err != nil || !strings.Contains(rewarn, "USAGE LIMIT IMMINENT") {
		t.Fatalf("rewarn=%q err=%v", rewarn, err)
	}
	seed(5, 5)
	secondRecovery, err := Evaluate(context.Background(), options)
	if err != nil || !strings.Contains(secondRecovery, "usage recovered") {
		t.Fatalf("second recovery=%q err=%v", secondRecovery, err)
	}
	if err := os.Remove(filepath.Join(cacheDir, "warned-1")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	seed(10, 10)
	coldHealthy, err := Evaluate(context.Background(), options)
	if err != nil || coldHealthy != "" {
		t.Fatalf("cold healthy=%q err=%v", coldHealthy, err)
	}
}

func TestRefreshUsesHeaderAndAcceptsOnlyAUsagePayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer fixture-token" {
			t.Errorf("authorization header = %q", request.Header.Get("Authorization"))
		}
		if request.Header.Get("anthropic-beta") != "oauth-2025-04-20" {
			t.Errorf("beta header = %q", request.Header.Get("anthropic-beta"))
		}
		_, _ = io.WriteString(writer, `{
			"five_hour":{"utilization":81.9,"resets_at":"2030-01-01T10:00:00Z"},
			"seven_day":{"utilization":12,"resets_at":"2030-01-03T08:00:00Z"},
			"seven_day_opus":{"utilization":null}
		}`)
	}))
	defer server.Close()

	root := t.TempDir()
	configDir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(configDir, ".credentials.json"),
		[]byte(`{"claudeAiOauth":{"accessToken":"fixture-token"}}`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	message, err := Evaluate(context.Background(), Options{
		Now:       time.Now,
		Home:      root,
		ConfigDir: configDir,
		CacheDir:  filepath.Join(root, "cache"),
		Warn:      80,
		Critical:  95,
		TTL:       time.Minute,
		Client:    server.Client(),
		Endpoint:  server.URL,
	})
	if err != nil || !strings.Contains(message, "usage limit approaching") ||
		!strings.Contains(message, "5h 81%") {
		t.Fatalf("message=%q err=%v", message, err)
	}
}

func TestMissingCredentialsAndPoisonPayloadFailOpen(t *testing.T) {
	root := t.TempDir()
	message, err := Evaluate(context.Background(), Options{
		Home: root, ConfigDir: filepath.Join(root, ".claude"), CacheDir: filepath.Join(root, "cache"),
	})
	if err != nil || message != "" {
		t.Fatalf("missing credentials: message=%q err=%v", message, err)
	}

	configDir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, ".credentials.json"), []byte(`{"claudeAiOauth":{"accessToken":"fixture"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, `{"error":"unauthorized"}`)
	}))
	defer server.Close()
	message, err = Evaluate(context.Background(), Options{
		Home: root, ConfigDir: configDir, CacheDir: filepath.Join(root, "cache"),
		Client: server.Client(), Endpoint: server.URL,
	})
	if err != nil || message != "" {
		t.Fatalf("poison payload: message=%q err=%v", message, err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "cache", "acct-1.json")); !os.IsNotExist(statErr) {
		t.Fatalf("invalid payload replaced cache: %v", statErr)
	}
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := make([]byte, 0, 4)
	for value > 0 {
		digits = append(digits, byte('0'+value%10))
		value /= 10
	}
	for left, right := 0, len(digits)-1; left < right; left, right = left+1, right-1 {
		digits[left], digits[right] = digits[right], digits[left]
	}
	return string(digits)
}
