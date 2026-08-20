// Package stats samples host, live-chat process-tree, and Docker cgroup
// pressure for the picker's lazy Stats tab. Resource counters never call a
// daemon; Docker names and images are resolved once per newly observed ID.
package stats

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"hostops/pfm/internal/compose"
)

type Header struct {
	CPUPercent  float64
	CPUValid    bool
	PSIPercent  float64
	RAMPercent  float64
	SwapPercent float64
	MemoryBytes uint64
}

type Chat struct {
	Socket          string
	Name            string
	Engine          string
	CPUPercent      float64
	CPUValid        bool
	RSSBytes        uint64
	RAMPercent      float64
	GearCount       int
	TokenCount      int64
	TokensKnown     bool
	TokensPerMinute float64
	TokenRateValid  bool
}

type Container struct {
	ID            string
	Name          string
	Image         string
	CPUPercent    float64
	CPUValid      bool
	MemoryBytes   uint64
	LimitBytes    uint64
	MemoryPercent float64
}

type Window struct {
	Name    string
	UsedPct int
	ResetAt time.Time
}

type AccountLimits struct {
	Account int
	Emoji   string
	Windows []Window
}

type Snapshot struct {
	Header     Header
	Chats      []Chat
	Docker     []Container
	Limits     []AccountLimits
	Ready      bool
	Warnings   []string
	SampleTime int64
}

type processSample struct {
	parent  int
	jiffies uint64
	rss     uint64
	command string
}

type dockerSample struct {
	usageMicros uint64
}

type rawSample struct {
	timeNS      int64
	systemTotal uint64
	systemIdle  uint64
	processes   map[int]processSample
	docker      map[string]dockerSample
}

// Sampler keeps only the preceding raw counters needed for instantaneous
// deltas. It is safe for Bubble Tea commands to call serially or concurrently.
type Sampler struct {
	ProcRoot      string
	CgroupRoot    string
	CPUCount      int
	Clock         func() int64
	DockerInspect func(string) (name, image string, err error)
	Limits        *LimitsSampler

	mu               sync.Mutex
	previous         *rawSample
	tokenMu          sync.Mutex
	tokenCache       map[string]*tokenCacheEntry
	tokenRates       map[string]*tokenRateState
	tokenGeneration  uint64
	dockerMu         sync.Mutex
	dockerIdentities map[string]dockerIdentity
}

func (sampler *Sampler) Sample(rows []compose.Row) (Snapshot, error) {
	now := time.Now().UnixNano()
	if sampler.Clock != nil {
		now = sampler.Clock()
	}
	procRoot := sampler.ProcRoot
	if procRoot == "" {
		procRoot = "/proc"
	}
	cgroupRoot := sampler.CgroupRoot
	if cgroupRoot == "" {
		cgroupRoot = "/sys/fs/cgroup"
	}
	cpuCount := sampler.CPUCount
	if cpuCount <= 0 {
		cpuCount = runtime.NumCPU()
	}

	total, idle, err := readSystemCPU(procRoot)
	if err != nil {
		return Snapshot{}, err
	}
	header, err := readHeader(procRoot)
	if err != nil {
		return Snapshot{}, err
	}
	processes, warnings, err := readProcesses(procRoot)
	if err != nil {
		return Snapshot{}, err
	}
	docker, dockerRaw, dockerWarnings, err := readDocker(cgroupRoot)
	if err != nil {
		return Snapshot{}, err
	}
	dockerWarnings = append(dockerWarnings, sampler.resolveDockerIdentities(docker)...)
	sort.Slice(docker, func(i, j int) bool {
		if docker[i].Name != docker[j].Name {
			return docker[i].Name < docker[j].Name
		}
		return docker[i].ID < docker[j].ID
	})
	warnings = append(warnings, dockerWarnings...)
	current := &rawSample{
		timeNS: now, systemTotal: total, systemIdle: idle,
		processes: processes, docker: dockerRaw,
	}

	sampler.mu.Lock()
	previous := sampler.previous
	sampler.previous = current
	sampler.mu.Unlock()

	if previous != nil && total > previous.systemTotal {
		deltaTotal := total - previous.systemTotal
		deltaIdle := uint64(0)
		if idle > previous.systemIdle {
			deltaIdle = idle - previous.systemIdle
		}
		if deltaIdle > deltaTotal {
			deltaIdle = deltaTotal
		}
		header.CPUPercent = percent(deltaTotal-deltaIdle, deltaTotal)
		header.CPUValid = true
	}
	chats := chatTrees(rows, processes, previous, total, cpuCount, header.MemoryBytes)
	tokenWarnings := sampler.attachTokenUsage(rows, chats, now)
	warnings = append(warnings, tokenWarnings...)
	var limits []AccountLimits
	if sampler.Limits != nil {
		var limitWarnings []string
		limits, limitWarnings = sampler.Limits.Sample(context.Background())
		warnings = append(warnings, limitWarnings...)
	}
	if previous != nil && now > previous.timeNS {
		for index := range docker {
			prior, found := previous.docker[docker[index].ID]
			currentDocker := dockerRaw[docker[index].ID]
			if !found || currentDocker.usageMicros < prior.usageMicros {
				continue
			}
			deltaMicros := currentDocker.usageMicros - prior.usageMicros
			elapsedMicros := uint64(now-previous.timeNS) / 1_000
			if elapsedMicros == 0 {
				continue
			}
			docker[index].CPUPercent = percent(deltaMicros, elapsedMicros)
			docker[index].CPUValid = true
		}
	}
	return Snapshot{
		Header: header, Chats: chats, Docker: docker,
		Ready: previous != nil, Warnings: warnings, Limits: limits, SampleTime: now,
	}, nil
}

func readSystemCPU(root string) (total, idle uint64, err error) {
	content, err := os.ReadFile(filepath.Join(root, "stat"))
	if err != nil {
		return 0, 0, fmt.Errorf("read host CPU counters: %w", err)
	}
	line, _, _ := strings.Cut(string(content), "\n")
	fields := strings.Fields(line)
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, 0, fmt.Errorf("read host CPU counters: malformed first line")
	}
	values := make([]uint64, 0, len(fields)-1)
	for _, field := range fields[1:] {
		value, parseErr := strconv.ParseUint(field, 10, 64)
		if parseErr != nil {
			return 0, 0, fmt.Errorf("parse host CPU counter %q: %w", field, parseErr)
		}
		total += value
		values = append(values, value)
	}
	idle = values[3]
	if len(values) > 4 {
		idle += values[4]
	}
	return total, idle, nil
}

func readHeader(root string) (Header, error) {
	content, err := os.ReadFile(filepath.Join(root, "meminfo"))
	if err != nil {
		return Header{}, fmt.Errorf("read host memory counters: %w", err)
	}
	values := make(map[string]uint64)
	for _, line := range strings.Split(string(content), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		value, parseErr := strconv.ParseUint(fields[1], 10, 64)
		if parseErr == nil {
			values[key] = value * 1024
		}
	}
	total := values["MemTotal"]
	available := values["MemAvailable"]
	swapTotal := values["SwapTotal"]
	swapFree := values["SwapFree"]
	header := Header{MemoryBytes: total}
	if total > 0 && available <= total {
		header.RAMPercent = percent(total-available, total)
	}
	if swapTotal > 0 && swapFree <= swapTotal {
		header.SwapPercent = percent(swapTotal-swapFree, swapTotal)
	}
	pressure, err := os.ReadFile(filepath.Join(root, "pressure", "cpu"))
	if err != nil {
		return Header{}, fmt.Errorf("read CPU pressure: %w", err)
	}
	for _, field := range strings.Fields(string(pressure)) {
		if strings.HasPrefix(field, "avg10=") {
			header.PSIPercent, err = strconv.ParseFloat(strings.TrimPrefix(field, "avg10="), 64)
			if err != nil {
				return Header{}, fmt.Errorf("parse CPU pressure avg10: %w", err)
			}
			break
		}
	}
	return header, nil
}

func readProcesses(root string) (map[int]processSample, []string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, nil, fmt.Errorf("read process root: %w", err)
	}
	processes := make(map[int]processSample)
	var warnings []string
	for _, entry := range entries {
		pid, parseErr := strconv.Atoi(entry.Name())
		if parseErr != nil || pid <= 0 || !entry.IsDir() {
			continue
		}
		content, readErr := os.ReadFile(filepath.Join(root, entry.Name(), "stat"))
		if readErr != nil {
			if !os.IsNotExist(readErr) {
				warnings = append(warnings, fmt.Sprintf("read process %d stat: %v", pid, readErr))
			}
			continue
		}
		process, parseErr := parseProcessStat(pid, content)
		if parseErr != nil {
			warnings = append(warnings, parseErr.Error())
			continue
		}
		if statm, readErr := os.ReadFile(filepath.Join(root, entry.Name(), "statm")); readErr == nil {
			fields := strings.Fields(string(statm))
			if len(fields) >= 2 {
				pages, parseErr := strconv.ParseUint(fields[1], 10, 64)
				if parseErr == nil {
					process.rss = pages * uint64(os.Getpagesize())
				}
			}
		} else if !os.IsNotExist(readErr) {
			warnings = append(warnings, fmt.Sprintf("read process %d memory: %v", pid, readErr))
		}
		if command, readErr := os.ReadFile(filepath.Join(root, entry.Name(), "cmdline")); readErr == nil {
			process.command, _, _ = strings.Cut(string(command), "\x00")
		} else if !os.IsNotExist(readErr) {
			warnings = append(warnings, fmt.Sprintf("read process %d command: %v", pid, readErr))
		}
		processes[pid] = process
	}
	return processes, warnings, nil
}

func parseProcessStat(pid int, content []byte) (processSample, error) {
	closeParen := strings.LastIndex(string(content), ") ")
	if closeParen < 0 {
		return processSample{}, fmt.Errorf("parse process %d stat: missing command terminator", pid)
	}
	fields := strings.Fields(string(content[closeParen+2:]))
	if len(fields) <= 12 {
		return processSample{}, fmt.Errorf("parse process %d stat: short record", pid)
	}
	parent, err := strconv.Atoi(fields[1])
	if err != nil {
		return processSample{}, fmt.Errorf("parse process %d parent: %w", pid, err)
	}
	user, err := strconv.ParseUint(fields[11], 10, 64)
	if err != nil {
		return processSample{}, fmt.Errorf("parse process %d user ticks: %w", pid, err)
	}
	system, err := strconv.ParseUint(fields[12], 10, 64)
	if err != nil {
		return processSample{}, fmt.Errorf("parse process %d system ticks: %w", pid, err)
	}
	return processSample{parent: parent, jiffies: user + system}, nil
}

type chatTree struct {
	chat  Chat
	roots []int
}

func chatTrees(rows []compose.Row, current map[int]processSample, previous *rawSample, systemTotal uint64, cpuCount int, memoryTotal uint64) []Chat {
	bySocket := make(map[string]*chatTree)
	for _, row := range rows {
		if row.Socket == "" || !liveKind(row.Kind) {
			continue
		}
		tree := bySocket[row.Socket]
		if tree == nil {
			tree = &chatTree{chat: Chat{Socket: row.Socket, Name: row.Name, Engine: engineName(row.Kind)}}
			bySocket[row.Socket] = tree
		}
		tree.roots = append(tree.roots, row.PanePIDs...)
	}
	children := make(map[int][]int)
	for pid, process := range current {
		children[process.parent] = append(children[process.parent], pid)
	}
	chats := make([]Chat, 0, len(bySocket))
	for _, tree := range bySocket {
		// tmux is the direct parent of each pane root and belongs to the socket
		// tree too. Include it once per socket so the Stats row represents the
		// server, the engine pane, and every descendant—not merely the pane's
		// child subtree.
		roots := append([]int(nil), tree.roots...)
		for _, root := range tree.roots {
			process, found := current[root]
			if !found {
				continue
			}
			parent, found := current[process.parent]
			if found && tmuxCommand(parent.command) {
				roots = append(roots, process.parent)
			}
		}
		tree.roots = roots
		visited := make(map[int]bool)
		stack := append([]int(nil), tree.roots...)
		rootSet := make(map[int]bool, len(tree.roots))
		for _, root := range tree.roots {
			rootSet[root] = true
		}
		var currentTicks, priorTicks uint64
		for len(stack) > 0 {
			pid := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if visited[pid] {
				continue
			}
			visited[pid] = true
			process, found := current[pid]
			if !found {
				continue
			}
			currentTicks += process.jiffies
			tree.chat.RSSBytes += process.rss
			if previous != nil {
				if prior, found := previous.processes[pid]; found {
					priorTicks += prior.jiffies
				}
			}
			if !rootSet[pid] && !engineCommand(process.command) && !shellCommand(process.command) {
				tree.chat.GearCount++
			}
			stack = append(stack, children[pid]...)
		}
		if memoryTotal > 0 {
			tree.chat.RAMPercent = percent(tree.chat.RSSBytes, memoryTotal)
		}
		if previous != nil && systemTotal > previous.systemTotal && currentTicks >= priorTicks {
			tree.chat.CPUPercent = percent(currentTicks-priorTicks, systemTotal-previous.systemTotal) * float64(cpuCount)
			tree.chat.CPUValid = true
		}
		chats = append(chats, tree.chat)
	}
	sort.Slice(chats, func(i, j int) bool { return chats[i].Socket < chats[j].Socket })
	return chats
}

func liveKind(kind compose.Kind) bool {
	return kind == compose.LiveClaude || kind == compose.LiveCodex ||
		kind == compose.LiveSplit || kind == compose.Agent || kind == compose.Booting
}

func engineName(kind compose.Kind) string {
	if kind == compose.LiveCodex {
		return "codex"
	}
	if kind == compose.LiveSplit {
		return "mixed"
	}
	return "claude"
}

func engineCommand(command string) bool {
	name := strings.ToLower(filepath.Base(command))
	return strings.Contains(name, "claude") || strings.Contains(name, "codex")
}

func shellCommand(command string) bool {
	switch strings.ToLower(filepath.Base(command)) {
	case "bash", "zsh", "sh", "fish", "tmux":
		return true
	default:
		return false
	}
}

func tmuxCommand(command string) bool {
	return strings.HasPrefix(strings.ToLower(filepath.Base(command)), "tmux")
}

func readDocker(root string) ([]Container, map[string]dockerSample, []string, error) {
	containers := make([]Container, 0)
	raw := make(map[string]dockerSample)
	var warnings []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			warnings = append(warnings, fmt.Sprintf("walk Docker cgroup %s: %v", path, walkErr))
			return nil
		}
		if !entry.IsDir() || !dockerCgroup(path) {
			return nil
		}
		id := dockerID(filepath.Base(path))
		name := shortDockerID(id)
		usage, err := readKeyUint(filepath.Join(path, "cpu.stat"), "usage_usec")
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("read Docker CPU %s: %v", name, err))
			return filepath.SkipDir
		}
		memory, err := readUintFile(filepath.Join(path, "memory.current"))
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("read Docker memory %s: %v", name, err))
			return filepath.SkipDir
		}
		limitContent, err := os.ReadFile(filepath.Join(path, "memory.max"))
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("read Docker limit %s: %v", name, err))
			return filepath.SkipDir
		}
		limit := uint64(0)
		if strings.TrimSpace(string(limitContent)) != "max" {
			limit, err = strconv.ParseUint(strings.TrimSpace(string(limitContent)), 10, 64)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("parse Docker limit %s: %v", name, err))
				return filepath.SkipDir
			}
		}
		container := Container{ID: id, Name: name, Image: "?", MemoryBytes: memory, LimitBytes: limit}
		if limit > 0 {
			container.MemoryPercent = percent(memory, limit)
		}
		containers = append(containers, container)
		raw[id] = dockerSample{usageMicros: usage}
		return filepath.SkipDir
	})
	if err != nil {
		return nil, nil, warnings, fmt.Errorf("walk Docker cgroups: %w", err)
	}
	sort.Slice(containers, func(i, j int) bool { return containers[i].Name < containers[j].Name })
	return containers, raw, warnings, nil
}

func dockerCgroup(path string) bool {
	base := filepath.Base(path)
	parent := filepath.Base(filepath.Dir(path))
	return (strings.HasPrefix(base, "docker-") && strings.HasSuffix(base, ".scope")) ||
		(parent == "docker" && len(base) >= 12)
}

func dockerID(base string) string {
	return strings.TrimSuffix(strings.TrimPrefix(base, "docker-"), ".scope")
}

func shortDockerID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func readKeyUint(path, key string) (uint64, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(content), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == key {
			return strconv.ParseUint(fields[1], 10, 64)
		}
	}
	return 0, fmt.Errorf("key %s is absent", key)
}

func readUintFile(path string) (uint64, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(strings.TrimSpace(string(content)), 10, 64)
}

func percent(used, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return float64(used) / float64(total) * 100
}
