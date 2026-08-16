package stats

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"hostops/pfm/internal/compose"
)

func uintString(value uint64) string { return strconv.FormatUint(value, 10) }

func TestSamplerComputesInstantaneousTreeCPUFromTwoProcSamples(t *testing.T) {
	root := t.TempDir()
	proc := filepath.Join(root, "proc")
	cgroup := filepath.Join(root, "cgroup")
	if err := os.MkdirAll(filepath.Join(proc, "pressure"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cgroup, 0o700); err != nil {
		t.Fatal(err)
	}
	writeStatsFixture(t, proc, 1000, 100)
	sampler := &Sampler{ProcRoot: proc, CgroupRoot: cgroup, CPUCount: 1, Clock: func() int64 { return 1 }}
	rows := []compose.Row{{
		Kind: compose.LiveClaude, Socket: "probe-chat", Name: "worker", PanePIDs: []int{100},
	}}
	first, err := sampler.Sample(rows)
	if err != nil {
		t.Fatal(err)
	}
	if first.Header.CPUValid || len(first.Chats) != 1 || first.Chats[0].CPUValid {
		t.Fatalf("first delta sample = %#v", first)
	}
	writeStatsFixture(t, proc, 1200, 130)
	sampler.Clock = func() int64 { return 2_000_000_001 }
	second, err := sampler.Sample(rows)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Chats[0].CPUValid || second.Chats[0].CPUPercent < 14.9 || second.Chats[0].CPUPercent > 15.1 {
		t.Fatalf("instantaneous chat CPU = %#v, want 15%%", second.Chats[0])
	}
	if second.Chats[0].RSSBytes == 0 || second.Header.RAMPercent != 50 || second.Header.SwapPercent != 25 {
		t.Fatalf("memory sample = %#v", second)
	}
}

func TestChatTreeIncludesTmuxServerAndMarksForeignDescendants(t *testing.T) {
	current := map[int]processSample{
		50:  {parent: 1, jiffies: 10, rss: 10, command: "/usr/bin/tmux"},
		100: {parent: 50, jiffies: 20, rss: 20, command: "/opt/bin/claude"},
		101: {parent: 100, jiffies: 30, rss: 30, command: "/usr/bin/node"},
		102: {parent: 100, jiffies: 40, rss: 40, command: "/work/dev-server"},
	}
	rows := []compose.Row{{
		Kind: compose.LiveClaude, Socket: "probe-tree", Name: "worker", PanePIDs: []int{100},
	}}
	chats := chatTrees(rows, current, nil, 0, 1, 1000)
	if len(chats) != 1 || chats[0].RSSBytes != 100 || chats[0].GearCount != 2 {
		t.Fatalf("socket tree = %#v, want tmux+engine+children RSS and two foreign descendants", chats)
	}
}

func TestDockerCgroupSamplingNeedsNoDaemon(t *testing.T) {
	root := t.TempDir()
	proc := filepath.Join(root, "proc")
	cgroup := filepath.Join(root, "cgroup")
	container := filepath.Join(cgroup, "system.slice", "docker-1234567890abcdef.scope")
	for _, directory := range []string{filepath.Join(proc, "pressure"), container} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	writeStatsFixture(t, proc, 1000, 100)
	writeDockerFixture(t, container, 1_000, 200, 1000)
	now := int64(1)
	sampler := &Sampler{ProcRoot: proc, CgroupRoot: cgroup, CPUCount: 1, Clock: func() int64 { return now }}
	first, err := sampler.Sample(nil)
	if err != nil || len(first.Docker) != 1 || first.Docker[0].CPUValid {
		t.Fatalf("first Docker sample=%#v err=%v", first.Docker, err)
	}
	now = 1_000_000_001
	writeStatsFixture(t, proc, 1100, 100)
	writeDockerFixture(t, container, 1_500, 250, 1000)
	second, err := sampler.Sample(nil)
	if err != nil || len(second.Docker) != 1 {
		t.Fatalf("second Docker sample=%#v err=%v", second.Docker, err)
	}
	got := second.Docker[0]
	if got.Name != "1234567890ab" || !got.CPUValid || got.CPUPercent != 0.05 ||
		got.MemoryBytes != 250 || got.MemoryPercent != 25 {
		t.Fatalf("Docker cgroup sample = %#v", got)
	}
}

func TestSamplerCountsClaudeAndCodexLifetimeTokensIncrementally(t *testing.T) {
	root := t.TempDir()
	proc := filepath.Join(root, "proc")
	cgroup := filepath.Join(root, "cgroup")
	if err := os.MkdirAll(filepath.Join(proc, "pressure"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cgroup, 0o700); err != nil {
		t.Fatal(err)
	}
	writeStatsFixture(t, proc, 1000, 100)

	claudePath := filepath.Join(root, "claude.jsonl")
	codexPath := filepath.Join(root, "codex.jsonl")
	claude := strings.Join([]string{
		`{"timestamp":"2026-08-16T10:00:00Z","type":"user","message":{"role":"user"}}`,
		`{"timestamp":"2026-08-16T11:00:00Z","type":"assistant","message":{"usage":{"input_tokens":100,"cache_read_input_tokens":200,"cache_creation_input_tokens":300,"output_tokens":400}}}`,
	}, "\n") + "\n"
	codex := strings.Join([]string{
		`{"timestamp":"2026-08-16T10:30:00Z","type":"session_meta","payload":{}}`,
		`{"timestamp":"2026-08-16T11:30:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"total_tokens":9000}}}}`,
	}, "\n") + "\n"
	if err := os.WriteFile(claudePath, []byte(claude), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(codexPath, []byte(codex), 0o600); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC).UnixNano()
	sampler := &Sampler{
		ProcRoot: proc, CgroupRoot: cgroup, CPUCount: 1,
		Clock: func() int64 { return now },
	}
	rows := []compose.Row{
		{Kind: compose.LiveClaude, Socket: "claude-socket", Name: "claude", Path: claudePath, PanePIDs: []int{100}},
		{Kind: compose.LiveCodex, Socket: "codex-socket", Name: "codex", Path: codexPath, PanePIDs: []int{100}},
	}
	first, err := sampler.Sample(rows)
	if err != nil {
		t.Fatal(err)
	}
	chats := chatsBySocket(first.Chats)
	if got := chats["claude-socket"]; !got.TokensKnown || got.TokenCount != 1000 ||
		!got.TokenRateValid || got.TokensPerHour != 500 {
		t.Fatalf("Claude token accounting = %#v, want 1000 total and 500/h", got)
	}
	if got := chats["codex-socket"]; !got.TokensKnown || got.TokenCount != 9000 ||
		!got.TokenRateValid || got.TokensPerHour != 6000 {
		t.Fatalf("Codex token accounting = %#v, want 9000 total and 6000/h", got)
	}

	appendFile(t, claudePath,
		`{"timestamp":"2026-08-16T12:00:00Z","type":"assistant","message":{"usage":{"input_tokens":10,"cache_read_input_tokens":20,"cache_creation_input_tokens":30,"output_tokens":40}}}`+"\n")
	appendFile(t, codexPath,
		`{"timestamp":"2026-08-16T12:00:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"total_tokens":12000}}}}`+"\n")
	now = time.Date(2026, 8, 16, 13, 0, 0, 0, time.UTC).UnixNano()
	second, err := sampler.Sample(rows)
	if err != nil {
		t.Fatal(err)
	}
	chats = chatsBySocket(second.Chats)
	if got := chats["claude-socket"]; got.TokenCount != 1100 || got.TokensPerHour < 366.6 || got.TokensPerHour > 366.7 {
		t.Fatalf("incremental Claude token accounting = %#v, want 1100 total and 366.7/h", got)
	}
	if got := chats["codex-socket"]; got.TokenCount != 12000 || got.TokensPerHour != 4800 {
		t.Fatalf("incremental Codex token accounting = %#v, want latest 12000 total and 4800/h", got)
	}
}

func TestDockerIdentityIsResolvedOncePerCgroupID(t *testing.T) {
	root := t.TempDir()
	proc := filepath.Join(root, "proc")
	cgroup := filepath.Join(root, "cgroup")
	id := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	container := filepath.Join(cgroup, "system.slice", "docker-"+id+".scope")
	for _, directory := range []string{filepath.Join(proc, "pressure"), container} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	writeStatsFixture(t, proc, 1000, 100)
	writeDockerFixture(t, container, 1_000, 200, 1000)
	inspectCalls := 0
	sampler := &Sampler{
		ProcRoot: proc, CgroupRoot: cgroup, CPUCount: 1,
		DockerInspect: func(gotID string) (string, string, error) {
			inspectCalls++
			if gotID != id {
				t.Fatalf("inspected container %q, want full cgroup ID %q", gotID, id)
			}
			return "professor-web", "registry.example/professor:web", nil
		},
	}
	for sample := 0; sample < 2; sample++ {
		snapshot, err := sampler.Sample(nil)
		if err != nil || len(snapshot.Docker) != 1 {
			t.Fatalf("Docker sample %d=%#v err=%v", sample, snapshot.Docker, err)
		}
		got := snapshot.Docker[0]
		if got.ID != id || got.Name != "professor-web" || got.Image != "registry.example/professor:web" {
			t.Fatalf("Docker identity = %#v", got)
		}
	}
	if inspectCalls != 1 {
		t.Fatalf("Docker identity lookup calls=%d, want one cached lookup", inspectCalls)
	}
}

func chatsBySocket(chats []Chat) map[string]Chat {
	result := make(map[string]Chat, len(chats))
	for _, chat := range chats {
		result[chat.Socket] = chat
	}
	return result
}

func appendFile(t *testing.T, path, content string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(content); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeDockerFixture(t *testing.T, directory string, usage, memory, limit uint64) {
	t.Helper()
	files := map[string]string{
		"cpu.stat":       "usage_usec " + uintString(usage) + "\n",
		"memory.current": uintString(memory) + "\n",
		"memory.max":     uintString(limit) + "\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func writeStatsFixture(t *testing.T, proc string, systemTicks, processTicks uint64) {
	t.Helper()
	files := map[string]string{
		"stat":         "cpu " + uintString(systemTicks/2) + " 0 0 " + uintString(systemTicks/2) + " 0 0 0 0\n",
		"meminfo":      "MemTotal: 1000 kB\nMemAvailable: 500 kB\nSwapTotal: 400 kB\nSwapFree: 300 kB\n",
		"pressure/cpu": "some avg10=7.25 avg60=0.00 avg300=0.00 total=1\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(proc, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	directory := filepath.Join(proc, "100")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	fields := []string{"S", "1", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", uintString(processTicks), "0", "0", "0", "0", "0", "0", "0", "1"}
	if err := os.WriteFile(filepath.Join(directory, "stat"), []byte("100 (pane) "+strings.Join(fields, " ")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "statm"), []byte("10 5\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "cmdline"), []byte("bash\x00"), 0o600); err != nil {
		t.Fatal(err)
	}
}
