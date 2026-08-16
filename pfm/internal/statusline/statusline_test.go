package statusline

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

type quietRunner struct{}

func (quietRunner) Output(
	context.Context,
	string,
	...string,
) ([]byte, error) {
	return nil, nil
}

func TestStatuslineCapturedInputGoldens(t *testing.T) {
	for _, sample := range []struct {
		name    string
		account int
		env     map[string]string
	}{
		{name: "primary", account: 1, env: map[string]string{"PFM_TEST_PROBE_SOCKETS": "1"}},
		{name: "gpt", account: 4, env: map[string]string{
			"PFM_TEST_PROBE_SOCKETS": "1",
			"ANTHROPIC_MODEL":        "gpt-5.6-sol[1m]",
		}},
	} {
		t.Run(sample.name, func(t *testing.T) {
			root := t.TempDir()
			now := time.Unix(1_786_838_400, 0)
			cacheDir := filepath.Join(root, "cache")
			tmuxDir := filepath.Join(root, "tmux")
			procRoot := filepath.Join(root, "proc")
			for _, directory := range []string{cacheDir, tmuxDir, filepath.Join(procRoot, "net")} {
				if err := os.MkdirAll(directory, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			for _, socket := range []string{"probe-cc-one", "probe-cc-two", "probe-cx-one"} {
				if err := os.WriteFile(filepath.Join(tmuxDir, socket), nil, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			writeGoldenCache(t, filepath.Join(cacheDir, "cc-vertex-spend"), "1.23|4.56|1786838400", now)
			if sample.account == 4 {
				writeGoldenCache(t, filepath.Join(cacheDir, "cc-gpt-usage-1000.json"),
					`{"primary":{"usedPercent":32,"windowDurationMins":300,"resetsAt":1786845600},"secondary":{"usedPercent":71,"windowDurationMins":10080,"resetsAt":1787443200},"planType":"plus"}`,
					now,
				)
				writeGoldenCache(t, filepath.Join(cacheDir, "cc-sl-gptreq"), "7 0\n", now)
				if err := os.WriteFile(filepath.Join(procRoot, "net", "tcp"), []byte(" 00000000:494D "), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			raw, err := os.ReadFile(filepath.Join("testdata", "render-"+sample.name+".json"))
			if err != nil {
				t.Fatal(err)
			}
			got, err := Render(context.Background(), raw, Runtime{
				Now:          func() time.Time { return now },
				Home:         root,
				ConfigDir:    filepath.Join(root, ".cc", strconv.Itoa(sample.account)),
				CacheDir:     cacheDir,
				RateLimitDir: filepath.Join(root, "rates"),
				SIDDir:       filepath.Join(root, "sid"),
				TmuxDir:      tmuxDir,
				ProcRoot:     procRoot,
				Columns:      120,
				UID:          1000,
				Env:          sample.env,
				Command:      quietRunner{},
			})
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(filepath.Join("testdata", "render-"+sample.name+".golden"))
			if err != nil {
				t.Fatalf("golden is missing; captured output is:\n%s", strconv.Quote(got))
			}
			if quoted := strconv.Quote(got) + "\n"; quoted != string(want) {
				t.Fatalf("statusline visual drift:\n got %s\nwant %s", quoted, want)
			}
		})
	}
}

func writeGoldenCache(t *testing.T, path, body string, now time.Time) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	stamp := now.Add(-5 * time.Minute)
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
}

func TestRenderCarriesNativeIdentityMetricsAndSky(t *testing.T) {
	root := t.TempDir()
	tmuxDir := filepath.Join(root, "tmux")
	if err := os.MkdirAll(tmuxDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"probe-cc-one", "probe-cc-two", "probe-cx-one"} {
		if err := os.WriteFile(filepath.Join(tmuxDir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	input := []byte(`{
  "model":{"display_name":"Opus 4"},
  "workspace":{"current_dir":"/work/sample"},
  "context_window":{
    "used_percentage":42.9,
    "total_input_tokens":12345,
    "total_output_tokens":678,
    "current_usage":{"cache_read_input_tokens":8000,"cache_creation_input_tokens":2000,"input_tokens":345}
  },
  "cost":{"total_cost_usd":3.42,"total_duration_ms":332000},
  "effort":{"level":"high"},
  "thinking":{"enabled":true},
  "session_name":"BUILDER:1",
  "session_id":"11111111-1111-4111-8111-111111111111"
}`)
	runtime := Runtime{
		Now:          func() time.Time { return time.Unix(1_786_838_400, 0) },
		Home:         root,
		ConfigDir:    filepath.Join(root, ".cc", "1"),
		CacheDir:     filepath.Join(root, "cache"),
		RateLimitDir: filepath.Join(root, "rates"),
		SIDDir:       filepath.Join(root, "sid"),
		TmuxDir:      tmuxDir,
		ProcRoot:     filepath.Join(root, "proc"),
		Columns:      120,
		UID:          1000,
		Env:          map[string]string{"PFM_TEST_PROBE_SOCKETS": "1"},
		Command:      quietRunner{},
	}
	got, err := Render(context.Background(), input, runtime)
	if err != nil {
		t.Fatal(err)
	}
	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(got, "")
	for _, want := range []string{
		"🥇 ", "◆ Opus 4", "🔖 BUILDER:1", "💠 high", "sample",
		"42%", "🧮10.3K", "💰$3.42", "⏱ 5m32s", "·2 ·1",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("render lacks %q:\n%q", want, got)
		}
	}
}

func TestDormantFourthAccountCloverAndRealContextWindow(t *testing.T) {
	root := t.TempDir()
	input := []byte(`{
  "model":{"display_name":"Claude"},
  "workspace":{"current_dir":"/work/sample"},
  "context_window":{"used_percentage":10,"current_usage":{"input_tokens":136000}},
  "cost":{"total_cost_usd":99,"total_duration_ms":1000}
}`)
	got, err := Render(context.Background(), input, Runtime{
		Now:       func() time.Time { return time.Unix(1_786_838_400, 0) },
		Home:      root,
		ConfigDir: filepath.Join(root, ".cc", "4"),
		CacheDir:  filepath.Join(root, "cache"),
		TmuxDir:   filepath.Join(root, "tmux"),
		ProcRoot:  filepath.Join(root, "proc"),
		Columns:   120,
		UID:       1000,
		Env:       map[string]string{"ANTHROPIC_MODEL": "gpt-5.6-sol[1m]"},
		Command:   quietRunner{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "🍀 ") ||
		!strings.Contains(got, "🍀 gpt-5.6-sol") ||
		!strings.Contains(got, "50%") ||
		strings.Contains(got, "💰$99.00") {
		t.Fatalf("fourth-account rendering drifted:\n%q", got)
	}
}

func TestFleetSnapshotCountsOnlySocketsPresentInProcOnLinux(t *testing.T) {
	root := t.TempDir()
	tmuxDir := filepath.Join(root, "tmux")
	procNet := filepath.Join(root, "proc", "net")
	if err := os.MkdirAll(tmuxDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(procNet, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"probe-cc-live", "probe-cc-grave", "probe-cx-live"} {
		if err := os.WriteFile(filepath.Join(tmuxDir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	proc := "Num RefCount Protocol Flags Type St Inode Path\n" +
		"0: 2 0 00010000 0001 01 1 " + filepath.Join(tmuxDir, "probe-cc-live") + "\n" +
		"1: 2 0 00010000 0001 01 2 " + filepath.Join(tmuxDir, "probe-cx-live") + "\n"
	if err := os.WriteFile(filepath.Join(procNet, "unix"), []byte(proc), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Render(context.Background(), []byte(`{"model":{"display_name":"Claude"}}`), Runtime{
		Home: root, ConfigDir: filepath.Join(root, ".cc", "1"), CacheDir: filepath.Join(root, "cache"),
		TmuxDir: tmuxDir, ProcRoot: filepath.Join(root, "proc"), UID: 1000,
		Env: map[string]string{"PFM_TEST_PROBE_SOCKETS": "1"}, Command: quietRunner{},
	})
	if err != nil {
		t.Fatal(err)
	}
	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(got, "")
	if !strings.Contains(plain, "·1 ·1") || strings.Contains(plain, "·2 ·1") {
		t.Fatalf("graveyard socket affected live fleet count: %q", plain)
	}
}

func TestGPTAuthRejectStreakIsTheLastThreeCompletedRequests(t *testing.T) {
	root := t.TempDir()
	logDir := filepath.Join(root, ".local", "state", "claude-code-proxy")
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		t.Fatal(err)
	}
	log := strings.Join([]string{
		`{"msg":"request_completed","t":"2026-08-16T01:00:00Z","status":401}`,
		`{"msg":"upstream_diagnostic","t":"2026-08-16T02:00:00Z","status":403}`,
		`{"msg":"request_completed","t":"2026-08-16T03:00:00Z","status":401}`,
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(logDir, "proxy.log"), []byte(log), 0o600); err != nil {
		t.Fatal(err)
	}
	count, reject := gptRequestCount(Runtime{
		Now:  func() time.Time { return time.Date(2026, 8, 16, 5, 0, 0, 0, time.UTC) },
		Home: root, CacheDir: filepath.Join(root, "cache"),
	}, time.Date(2026, 8, 16, 5, 0, 0, 0, time.UTC))
	if count != 2 || reject {
		t.Fatalf("count=%d reject=%v; non-request rows must not manufacture a three-request streak", count, reject)
	}
}
