package statusline

import (
	"bufio"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"hostops/pfm/internal/sky"
	"hostops/pfm/internal/usagehook"
)

const (
	green   = "\x1b[1;32m"
	yellow  = "\x1b[1;33m"
	red     = "\x1b[1;31m"
	cyan    = "\x1b[1;36m"
	blue    = "\x1b[1;34m"
	magenta = "\x1b[1;35m"
	dim     = "\x1b[2m"
	white   = "\x1b[1;37m"
	reset   = "\x1b[0m"
	sep     = " " + dim + "│" + reset + " "
)

type input struct {
	Model struct {
		DisplayName string `json:"display_name"`
	} `json:"model"`
	Workspace struct {
		CurrentDir  string `json:"current_dir"`
		GitWorktree string `json:"git_worktree"`
	} `json:"workspace"`
	CWD           string `json:"cwd"`
	ContextWindow struct {
		UsedPercentage    float64 `json:"used_percentage"`
		TotalInputTokens  int64   `json:"total_input_tokens"`
		TotalOutputTokens int64   `json:"total_output_tokens"`
		CurrentUsage      struct {
			CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
			CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
			InputTokens              int64 `json:"input_tokens"`
		} `json:"current_usage"`
	} `json:"context_window"`
	Cost struct {
		TotalCostUSD      float64 `json:"total_cost_usd"`
		TotalDurationMS   int64   `json:"total_duration_ms"`
		TotalLinesAdded   int64   `json:"total_lines_added"`
		TotalLinesRemoved int64   `json:"total_lines_removed"`
	} `json:"cost"`
	Vim struct {
		Mode string `json:"mode"`
	} `json:"vim"`
	Agent struct {
		Name string `json:"name"`
	} `json:"agent"`
	Worktree struct {
		Name string `json:"name"`
	} `json:"worktree"`
	RateLimits  rateLimits `json:"rate_limits"`
	OutputStyle struct {
		Name string `json:"name"`
	} `json:"output_style"`
	Effort struct {
		Level string `json:"level"`
	} `json:"effort"`
	Thinking struct {
		Enabled bool `json:"enabled"`
	} `json:"thinking"`
	SessionName    string `json:"session_name"`
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
}

type rateWindow struct {
	UsedPercentage float64 `json:"used_percentage"`
	ResetsAt       int64   `json:"resets_at"`
}

type rateLimits map[string]rateWindow

func (limits *rateLimits) UnmarshalJSON(content []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(content, &raw); err != nil {
		return err
	}
	decoded := make(rateLimits, len(raw))
	for key, value := range raw {
		var window rateWindow
		if json.Unmarshal(value, &window) == nil {
			decoded[key] = window
		}
	}
	*limits = decoded
	return nil
}

// Render is the native high-frequency path. It performs no network work and
// never waits for a refresher: stale caches only arm detached children through
// Runtime.Spawn.
func Render(ctx context.Context, raw []byte, runtime Runtime) (string, error) {
	runtime = runtime.normalized()
	var data input
	if err := json.Unmarshal(raw, &data); err != nil {
		return "", fmt.Errorf("decode statusline input: %w", err)
	}
	if data.Model.DisplayName == "" {
		data.Model.DisplayName = "Claude"
	}
	directory := data.Workspace.CurrentDir
	if directory == "" {
		directory = data.CWD
	}
	now := runtime.now()
	writeBreadcrumb(runtime, data.TranscriptPath)

	badge, account := accountBadge(runtime)
	harvestRateLimits(runtime, now, account, data)

	modelSymbol := "●"
	switch {
	case strings.Contains(data.Model.DisplayName, "Fable"):
		modelSymbol = "✦"
	case strings.Contains(data.Model.DisplayName, "Opus"):
		modelSymbol = "◆"
	case strings.Contains(data.Model.DisplayName, "Sonnet"):
		modelSymbol = "◇"
	case strings.Contains(data.Model.DisplayName, "Haiku"):
		modelSymbol = "○"
	}
	directoryName := filepath.Base(filepath.Clean(directory))
	if directory == "" || directoryName == string(filepath.Separator) {
		directoryName = "~"
	}

	l1 := badge + cyan + modelSymbol + " " + data.Model.DisplayName + reset
	if data.SessionName != "" {
		l1 += sep + white + "🔖 " + data.SessionName + reset
	}
	if effort := effortSegment(runtime, data); effort != "" {
		l1 += sep + effort
	}
	l1 += sep + blue + directoryName + reset
	if data.Worktree.Name != "" {
		l1 += sep + magenta + "🌳 " + data.Worktree.Name + reset
	} else if data.Workspace.GitWorktree != "" {
		l1 += sep + magenta + "🌳 " + data.Workspace.GitWorktree + reset
	}
	if git := gitSegment(ctx, runtime, directory); git != "" {
		l1 += sep + git
	}
	if data.Agent.Name != "" {
		l1 += sep + magenta + "⚡" + data.Agent.Name + reset
	}
	if data.Vim.Mode != "" {
		color := green
		if data.Vim.Mode == "NORMAL" {
			color = cyan
		}
		l1 += sep + color + data.Vim.Mode + reset
	}
	claudeCount, codexCount := fleetCounts(runtime)
	l1 += sep + sky.Snapshot(claudeCount, codexCount)

	percent := int(data.ContextWindow.UsedPercentage)
	urgency := urgencyEmoji(percent)
	l2 := urgency + " " + makeBar(percent, 10) + " " + percentColor(percent) +
		strconv.Itoa(percent) + "%" + reset
	wide := runtime.Columns == 0 || runtime.Columns >= 100
	contextTokens := data.ContextWindow.CurrentUsage.CacheReadInputTokens +
		data.ContextWindow.CurrentUsage.CacheCreationInputTokens +
		data.ContextWindow.CurrentUsage.InputTokens
	if wide && contextTokens > 0 {
		l2 += sep + dim + "🧮" + formatTokens(contextTokens) + reset
	}
	// Only the width gate belongs here. Whether the transcript is readable is
	// the segment's own question to answer, and it answers it visibly.
	if wide {
		l2 += cacheWindowSegment(runtime, now, data.TranscriptPath)
	}
	totalTokens := data.ContextWindow.TotalInputTokens + data.ContextWindow.TotalOutputTokens
	if totalTokens > 0 {
		l2 += " " + dim + "(" + formatTokens(data.ContextWindow.TotalInputTokens) + "→" +
			formatTokens(data.ContextWindow.TotalOutputTokens) + ")" + reset
	}
	if data.Cost.TotalCostUSD > 0 && account != 4 {
		color := dim
		if data.Cost.TotalCostUSD >= 10 {
			color = red
		} else if data.Cost.TotalCostUSD >= 2 {
			color = yellow
		}
		l2 += sep + color + "💰" + fmt.Sprintf("$%.2f", data.Cost.TotalCostUSD) + reset
	}
	l2 += sep + dim + "⏱ " + formatDuration(data.Cost.TotalDurationMS) + reset

	l3 := vertexSegment(runtime, now)
	if account == 4 {
		gptLine, replacement := gptSegment(runtime, now, contextTokens, l2)
		if replacement != "" {
			l2 = replacement
		}
		l3 = appendSegment(l3, gptLine)
	}
	l3 = appendRateSegments(l3, now, data)

	if l3 != "" {
		return reset + l1 + "\n" + reset + l2 + "\n" + reset + l3 + "\n", nil
	}
	return reset + l1 + "\n" + reset + l2 + "\n", nil
}

func effortSegment(runtime Runtime, data input) string {
	if data.Effort.Level == "" {
		return ""
	}
	color, emoji := cyan, "🔆"
	switch data.Effort.Level {
	case "low":
		color, emoji = dim, "🔹"
	case "medium":
		color, emoji = green, "🔶"
	case "high":
		color, emoji = yellow, "💠"
	case "xhigh":
		color, emoji = magenta, "💎"
	case "max":
		color, emoji = red, "👑"
	}
	sessionID := runtime.getenv("CLAUDE_CODE_SESSION_ID")
	if sessionID == "" {
		sessionID = data.SessionID
	}
	if data.Effort.Level == "xhigh" && sessionID != "" {
		if _, err := os.Stat(filepath.Join(runtime.Home, ".claude", "ultracode", sessionID)); err == nil {
			return red + "🚀 ultracode" + reset
		}
	}
	if !data.Thinking.Enabled {
		return dim + "💤 " + data.Effort.Level + " (off)" + reset
	}
	return color + emoji + " " + data.Effort.Level + reset
}

func formatDuration(milliseconds int64) string {
	seconds := milliseconds / 1000
	switch {
	case seconds >= 3600:
		return fmt.Sprintf("%dh%dm", seconds/3600, seconds%3600/60)
	case seconds >= 60:
		return fmt.Sprintf("%dm%ds", seconds/60, seconds%60)
	default:
		return fmt.Sprintf("%ds", seconds)
	}
}

func urgencyEmoji(percent int) string {
	switch {
	case percent >= 95:
		return "🚨"
	case percent >= 80:
		return "🔥"
	case percent >= 50:
		return "⚡"
	default:
		return "🟢"
	}
}

func percentColor(percent int) string {
	if percent >= 80 {
		return red
	}
	if percent >= 50 {
		return yellow
	}
	return green
}

func makeBar(percent, width int) string {
	filled := percent * width / 100
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	return percentColor(percent) + strings.Repeat("▓", filled) + dim +
		strings.Repeat("░", width-filled) + reset
}

func formatTokens(tokens int64) string {
	switch {
	case tokens >= 1_000_000:
		return fmt.Sprintf("%d.%dM", tokens/1_000_000, tokens%1_000_000/100_000)
	case tokens >= 1_000:
		return fmt.Sprintf("%d.%dK", tokens/1_000, tokens%1_000/100)
	default:
		return strconv.FormatInt(tokens, 10)
	}
}

func appendSegment(line, segment string) string {
	if segment == "" {
		return line
	}
	if line == "" {
		return segment
	}
	return line + sep + segment
}

func appendRateSegments(line string, now time.Time, data input) string {
	keys := make([]string, 0, len(data.RateLimits))
	for key := range data.RateLimits {
		keys = append(keys, key)
	}
	for _, descriptor := range usagehook.DescribeWindows(keys) {
		window := data.RateLimits[descriptor.Key]
		used := int(window.UsedPercentage)
		if used <= 0 {
			continue
		}
		segment := makeBar(used, 5) + " " + percentColor(used) +
			descriptor.Label + "-used:" + strconv.Itoa(used) + "%" + reset
		remaining := window.ResetsAt - now.Unix()
		if remaining > 0 {
			if descriptor.Key == "five_hour" {
				segment += fmt.Sprintf(" %s↻%dh%dm%s", dim, remaining/3600, remaining%3600/60, reset)
			} else {
				segment += fmt.Sprintf(" %s↻%dd%dh%s", dim, remaining/86400, remaining%86400/3600, reset)
			}
		}
		line = appendSegment(line, segment)
	}
	return line
}

func gitSegment(ctx context.Context, runtime Runtime, directory string) string {
	if directory == "" {
		return ""
	}
	if err := os.MkdirAll(runtime.CacheDir, 0o700); err != nil {
		return ""
	}
	digest := md5.Sum([]byte(directory))
	cachePath := filepath.Join(runtime.CacheDir, "cc-sl-"+hex.EncodeToString(digest[:]))
	stale := true
	if info, err := os.Stat(cachePath); err == nil {
		stale = runtime.now().Sub(info.ModTime()) > 5*time.Second
	}
	if stale {
		commandContext, cancel := context.WithTimeout(ctx, 750*time.Millisecond)
		branch, err := runtime.Command.Output(
			commandContext,
			"git",
			"-C", directory, "--no-optional-locks", "symbolic-ref", "--short", "HEAD",
		)
		cancel()
		content := ""
		if err == nil && strings.TrimSpace(string(branch)) != "" {
			staged := gitDiffCount(ctx, runtime, directory, true)
			modified := gitDiffCount(ctx, runtime, directory, false)
			content = fmt.Sprintf("%s|%d|%d", strings.TrimSpace(string(branch)), staged, modified)
		}
		_ = atomicWrite(cachePath, []byte(content), 0o600)
	}
	body, err := os.ReadFile(cachePath)
	if err != nil || len(body) == 0 {
		return ""
	}
	fields := strings.Split(strings.TrimSpace(string(body)), "|")
	if len(fields) != 3 || fields[0] == "" {
		return ""
	}
	staged, _ := strconv.Atoi(fields[1])
	modified, _ := strconv.Atoi(fields[2])
	color, suffix := green, ""
	if staged > 0 {
		suffix += fmt.Sprintf(" %s+%d", green, staged)
		color = yellow
	}
	if modified > 0 {
		suffix += fmt.Sprintf(" %s~%d", yellow, modified)
		color = yellow
	}
	return color + "🌿 " + fields[0] + suffix + reset
}

func gitDiffCount(ctx context.Context, runtime Runtime, directory string, staged bool) int {
	args := []string{"-C", directory, "--no-optional-locks", "diff"}
	if staged {
		args = append(args, "--cached")
	}
	args = append(args, "--numstat")
	commandContext, cancel := context.WithTimeout(ctx, 750*time.Millisecond)
	defer cancel()
	output, err := runtime.Command.Output(commandContext, "git", args...)
	if err != nil {
		return 0
	}
	return len(strings.FieldsFunc(strings.TrimSpace(string(output)), func(r rune) bool { return r == '\n' }))
}

func accountBadge(runtime Runtime) (string, int) {
	if strings.TrimSpace(runtime.ConfigDir) == "" {
		return "", 0
	}
	configDir := canonicalRuntimePath(runtime.ConfigDir)
	account := 0
	for directory, id := range runtime.AccountDirs {
		if configDir == canonicalRuntimePath(directory) {
			account = id
			break
		}
	}
	if account == 0 {
		if len(runtime.AccountDirs) == 0 {
			if legacy, err := strconv.Atoi(filepath.Base(configDir)); err == nil && legacy > 0 {
				return accountBadgeForID(runtime, legacy)
			}
		}
		return "", 0
	}
	return accountBadgeForID(runtime, account)
}

func canonicalRuntimePath(path string) string {
	cleaned := filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(cleaned); err == nil {
		return filepath.Clean(resolved)
	}
	return cleaned
}

func accountBadgeForID(runtime Runtime, account int) (string, int) {
	if emoji := runtime.AccountEmojis[account]; emoji != "" && emoji != "·" {
		return emoji + " ", account
	}
	switch account {
	case 1:
		return "🥇 ", 1
	case 2:
		return "🥈 ", 2
	case 3:
		return "🥉 ", 3
	case 4:
		return "🍀 ", 4
	default:
		return "", 0
	}
}

func fleetCounts(runtime Runtime) (int, int) {
	allowProbe := runtime.getenv("PFM_TEST_PROBE_SOCKETS") == "1"
	if body, err := os.ReadFile(filepath.Join(runtime.ProcRoot, "net", "unix")); err == nil {
		claude, codex := 0, 0
		seen := map[string]bool{}
		for _, line := range strings.Split(string(body), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 8 {
				continue
			}
			path := fields[len(fields)-1]
			if filepath.Clean(filepath.Dir(path)) != filepath.Clean(runtime.TmuxDir) {
				continue
			}
			name := filepath.Base(path)
			if seen[name] {
				continue
			}
			seen[name] = true
			switch {
			case strings.HasPrefix(name, "cc-"):
				claude++
			case strings.HasPrefix(name, "cx-"):
				codex++
			case allowProbe && strings.HasPrefix(name, "probe-cc-"):
				claude++
			case allowProbe && strings.HasPrefix(name, "probe-cx-"):
				codex++
			}
		}
		return claude, codex
	}
	entries, err := os.ReadDir(runtime.TmuxDir)
	if err != nil {
		return 0, 0
	}
	claude, codex := 0, 0
	for _, entry := range entries {
		name := entry.Name()
		switch {
		case strings.HasPrefix(name, "cc-"):
			claude++
		case strings.HasPrefix(name, "cx-"):
			codex++
		case allowProbe && strings.HasPrefix(name, "probe-cc-"):
			claude++
		case allowProbe && strings.HasPrefix(name, "probe-cx-"):
			codex++
		}
	}
	return claude, codex
}

func writeBreadcrumb(runtime Runtime, transcriptPath string) {
	tmux := runtime.getenv("TMUX")
	if tmux == "" || transcriptPath == "" {
		return
	}
	socket := strings.SplitN(tmux, ",", 2)[0]
	socket = filepath.Base(socket)
	if socket == "." || socket == "" {
		return
	}
	if err := os.MkdirAll(runtime.SIDDir, 0o700); err != nil {
		return
	}
	_ = os.Chmod(runtime.SIDDir, 0o700)
	_ = atomicWrite(filepath.Join(runtime.SIDDir, socket), []byte(transcriptPath), 0o600)
	if pane := runtime.getenv("TMUX_PANE"); pane != "" {
		_ = atomicWrite(filepath.Join(runtime.SIDDir, socket+"."+pane), []byte(transcriptPath), 0o600)
	}
}

func harvestRateLimits(runtime Runtime, now time.Time, account int, data input) {
	fiveWindow := data.RateLimits["five_hour"]
	sevenWindow := data.RateLimits["seven_day"]
	five := int(fiveWindow.UsedPercentage)
	seven := int(sevenWindow.UsedPercentage)
	if (five <= 0 && seven <= 0) || fiveWindow.ResetsAt <= now.Unix() {
		return
	}
	if err := os.MkdirAll(runtime.RateLimitDir, 0o700); err != nil {
		return
	}
	sessionID := data.SessionID
	if sessionID == "" {
		sessionID = "anon"
	}
	body, err := json.Marshal(map[string]int64{
		"acct":                int64(account),
		"five_hour_used":      int64(five),
		"seven_day_used":      int64(seven),
		"five_hour_resets_at": fiveWindow.ResetsAt,
		"seven_day_resets_at": sevenWindow.ResetsAt,
		"ts":                  now.Unix(),
	})
	if err == nil {
		body = append(body, '\n')
		_ = atomicWrite(
			filepath.Join(runtime.RateLimitDir, fmt.Sprintf("acct-%d.%s.json", account, sessionID)),
			body,
			0o600,
		)
	}
	entries, _ := os.ReadDir(runtime.RateLimitDir)
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), fmt.Sprintf("acct-%d.", account)) ||
			!strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr == nil && now.Sub(info.ModTime()) > time.Hour {
			_ = os.Remove(filepath.Join(runtime.RateLimitDir, entry.Name()))
		}
	}
}

func cacheWindowSegment(runtime Runtime, now time.Time, transcriptPath string) string {
	ttl := time.Hour
	label := "1h"
	if runtime.getenv("FORCE_PROMPT_CACHING_5M") == "1" {
		ttl = 5 * time.Minute
		label = "5m"
	}
	// A transcript we could not read is NOT a chat without a cache window, and
	// the two must never share a rendering. Returning "" here made the segment
	// disappear, which is indistinguishable from a statusline that has no cache
	// timer at all — so the one state worth shouting about, a chat running with
	// transcript saving off, arrived as silence. That chat cannot be resumed and
	// its window cannot be measured; the statusline is where the user finds out.
	//
	// "!" is deliberately not "?": "?" means the transcript WAS read and simply
	// carries no user turn to anchor on, which is a fact about the chat. "!" is
	// a fact about us — we could not look.
	if transcriptPath == "" {
		return sep + red + "💾" + label + "!" + reset
	}
	info, err := os.Stat(transcriptPath)
	if err != nil || info.IsDir() {
		return sep + red + "💾" + label + "!" + reset
	}
	cachePath := filepath.Join(
		runtime.CacheDir,
		"cc-sl-anchor-"+strings.TrimSuffix(filepath.Base(transcriptPath), ".jsonl"),
	)
	key := fmt.Sprintf("%d:%d", info.ModTime().Unix(), info.Size())
	cachedKey, anchor := readAnchorCache(cachePath)
	if cachedKey != key {
		anchor = newestUserTimestamp(transcriptPath)
		encoded := "-"
		if !anchor.IsZero() {
			encoded = strconv.FormatInt(anchor.Unix(), 10)
		}
		_ = atomicWrite(cachePath, []byte(key+" "+encoded), 0o600)
	}
	if anchor.IsZero() {
		return sep + yellow + "💾" + label + "?" + reset
	}
	remaining := ttl - now.Sub(anchor)
	if remaining > 0 {
		return sep + green + "💾" + label + "✓" + formatCacheTime(remaining, false) + reset
	}
	return sep + red + "💾" + label + "✗" + formatCacheTime(-remaining, true) + reset
}

func readAnchorCache(path string) (string, time.Time) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", time.Time{}
	}
	fields := strings.Fields(string(body))
	if len(fields) != 2 || fields[1] == "-" {
		if len(fields) == 2 {
			return fields[0], time.Time{}
		}
		return "", time.Time{}
	}
	epoch, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return "", time.Time{}
	}
	return fields[0], time.Unix(epoch, 0)
}

func newestUserTimestamp(path string) time.Time {
	for _, size := range []int64{65_536, 1_048_576} {
		body, err := readTail(path, size)
		if err != nil {
			continue
		}
		var newest time.Time
		scanner := bufio.NewScanner(strings.NewReader(string(body)))
		scanner.Buffer(make([]byte, 64*1024), int(size)+1)
		for scanner.Scan() {
			var record struct {
				Type      string `json:"type"`
				Sidechain bool   `json:"isSidechain"`
				Timestamp string `json:"timestamp"`
			}
			if json.Unmarshal(scanner.Bytes(), &record) != nil ||
				record.Type != "user" || record.Sidechain || record.Timestamp == "" {
				continue
			}
			parsed, parseErr := time.Parse(time.RFC3339Nano, record.Timestamp)
			if parseErr == nil && parsed.After(newest) {
				newest = parsed
			}
		}
		if !newest.IsZero() {
			return newest
		}
	}
	return time.Time{}
}

func readTail(path string, size int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	start := info.Size() - size
	if start < 0 {
		start = 0
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return nil, err
	}
	return io.ReadAll(file)
}

func formatCacheTime(duration time.Duration, expired bool) string {
	seconds := int64(duration / time.Second)
	if seconds < 0 {
		seconds = -seconds
	}
	if seconds >= 3600 {
		if expired {
			return fmt.Sprintf("%dh:%dm", seconds/3600, seconds%3600/60)
		}
		return fmt.Sprintf("%dh:%dm:%ds", seconds/3600, seconds%3600/60, seconds%60)
	}
	if seconds >= 60 {
		return fmt.Sprintf("%dm:%ds", seconds/60, seconds%60)
	}
	return fmt.Sprintf("%ds", seconds)
}

func atomicWrite(path string, body []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".pfm-statusline-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(body); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func fileAge(path string, now time.Time) time.Duration {
	info, err := os.Stat(path)
	if err != nil {
		return 100 * 365 * 24 * time.Hour
	}
	return now.Sub(info.ModTime())
}

func armRefresh(runtime Runtime, kind RefreshKind, cachePath string, ttl time.Duration) {
	if runtime.Spawn == nil || fileAge(cachePath, runtime.now()) <= ttl {
		return
	}
	lockPath := strings.TrimSuffix(cachePath, ".json") + ".lock"
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o700); err != nil {
		return
	}
	if kind == RefreshKindVertex && fileAge(lockPath, runtime.now()) > 5*time.Minute {
		_ = os.Remove(lockPath)
	}
	lock, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return
	}
	_ = lock.Close()
	if err := runtime.Spawn(kind); err != nil {
		_ = os.Remove(lockPath)
	}
}

func vertexSegment(runtime Runtime, now time.Time) string {
	cachePath := filepath.Join(runtime.CacheDir, "cc-vertex-spend")
	armRefresh(runtime, RefreshKindVertex, cachePath, 10*time.Minute)
	body, err := os.ReadFile(cachePath)
	if err != nil {
		return ""
	}
	fields := strings.Split(strings.TrimSpace(string(body)), "|")
	if len(fields) < 2 || fields[0] == "" {
		return ""
	}
	segment := cyan + "☁️ Vertex" + reset + " " + white + "~€" + fields[0] +
		" today" + reset + " " + dim + "·" + reset + " " + white + "~€" + fields[1] + " 7d" + reset
	if fileAge(cachePath, now) > time.Hour {
		segment += " " + dim + "(stale)" + reset
	}
	return segment
}

type gptUsageCache struct {
	Primary   *gptWindow `json:"primary"`
	Secondary *gptWindow `json:"secondary"`
	PlanType  string     `json:"planType"`
}

type gptWindow struct {
	UsedPercent        float64 `json:"usedPercent"`
	WindowDurationMins int64   `json:"windowDurationMins"`
	ResetsAt           int64   `json:"resetsAt"`
}

func gptSegment(runtime Runtime, now time.Time, contextTokens int64, currentL2 string) (string, string) {
	model := runtime.getenv("ANTHROPIC_MODEL")
	if model == "" {
		model = "gpt-5.6-sol"
	}
	model = strings.TrimSuffix(model, "[1m]")
	segment := green + "🍀 " + model + reset
	procTCP, err := os.ReadFile(filepath.Join(runtime.ProcRoot, "net", "tcp"))
	if err == nil && strings.Contains(strings.ToUpper(string(procTCP)), ":494D ") {
		segment += sep + dim + "⇅ proxy" + reset
	} else {
		segment += sep + red + "⇅ proxy DOWN" + reset
	}
	requests, authReject := gptRequestCount(runtime, now)
	if requests > 0 {
		segment += sep + dim + "↻ " + strconv.Itoa(requests) + " today" + reset
	}
	if authReject {
		segment += sep + red + "⚠ auth-reject streak — WS upgrade refused; CCP_CODEX_TRANSPORT=http" + reset
	}
	usagePath := filepath.Join(runtime.CacheDir, fmt.Sprintf("cc-gpt-usage-%d.json", runtime.UID))
	armRefresh(runtime, RefreshKindGPT, usagePath, 5*time.Minute)
	usageBody, usageErr := os.ReadFile(usagePath)
	var usage gptUsageCache
	if usageErr == nil && json.Unmarshal(usageBody, &usage) == nil {
		for _, window := range []*gptWindow{usage.Primary, usage.Secondary} {
			if window == nil || window.UsedPercent < 0 {
				continue
			}
			percent := int(window.UsedPercent)
			segment += sep + makeBar(percent, 5) + " " + percentColor(percent) +
				windowLabel(window.WindowDurationMins) + "-used:" + strconv.Itoa(percent) + "%" + reset +
				resetCountdown(now, window.ResetsAt)
		}
		if usage.PlanType != "" {
			segment += sep + dim + "ChatGPT " + usage.PlanType + reset
		}
	} else {
		segment += sep + dim + "ChatGPT subscription" + reset
	}

	replacement := ""
	window := int64(0)
	baseModel := strings.TrimSuffix(model, "-fast")
	if strings.HasPrefix(baseModel, "gpt-5.6-") {
		window = 272_000
	} else {
		window, _ = strconv.ParseInt(runtime.getenv("CLAUDE_CODE_AUTO_COMPACT_WINDOW"), 10, 64)
	}
	if window > 0 && contextTokens > 0 {
		percent := int(contextTokens * 100 / window)
		if percent > 100 {
			percent = 100
		}
		rest := ""
		if index := strings.Index(currentL2, sep); index >= 0 {
			rest = currentL2[index:]
		}
		replacement = urgencyEmoji(percent) + " " + makeBar(percent, 10) + " " +
			percentColor(percent) + strconv.Itoa(percent) + "%" + reset + " " + dim +
			"of " + formatTokens(window) + reset + rest
	}
	return segment, replacement
}

func gptRequestCount(runtime Runtime, now time.Time) (int, bool) {
	cachePath := filepath.Join(runtime.CacheDir, "cc-sl-gptreq")
	if fileAge(cachePath, now) > 30*time.Second {
		logPath := filepath.Join(runtime.Home, ".local", "state", "claude-code-proxy", "proxy.log")
		file, err := os.Open(logPath)
		if err == nil {
			defer file.Close()
			today := now.UTC().Format("2006-01-02")
			count := 0
			last := make([]int, 0, 3)
			scanner := bufio.NewScanner(file)
			scanner.Buffer(make([]byte, 64*1024), 1024*1024)
			for scanner.Scan() {
				var row struct {
					Message string `json:"msg"`
					Time    string `json:"t"`
					Status  int    `json:"status"`
				}
				if json.Unmarshal(scanner.Bytes(), &row) != nil {
					continue
				}
				if row.Message != "request_completed" {
					continue
				}
				if strings.HasPrefix(row.Time, today) {
					count++
				}
				last = append(last, row.Status)
				if len(last) > 3 {
					last = last[len(last)-3:]
				}
			}
			reject := len(last) == 3
			for _, status := range last {
				reject = reject && (status == 401 || status == 403)
			}
			_ = atomicWrite(cachePath, []byte(fmt.Sprintf("%d\t%d\n", count, boolInt(reject))), 0o600)
		}
	}
	body, err := os.ReadFile(cachePath)
	if err != nil {
		return 0, false
	}
	fields := strings.Fields(string(body))
	if len(fields) != 2 {
		return 0, false
	}
	count, _ := strconv.Atoi(fields[0])
	reject, _ := strconv.Atoi(fields[1])
	return count, reject == 1
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func windowLabel(minutes int64) string {
	switch {
	case minutes >= 1440:
		return fmt.Sprintf("%dd", minutes/1440)
	case minutes >= 60:
		return fmt.Sprintf("%dh", minutes/60)
	default:
		return fmt.Sprintf("%dm", minutes)
	}
}

func resetCountdown(now time.Time, resetsAt int64) string {
	remaining := resetsAt - now.Unix()
	if remaining <= 0 {
		return ""
	}
	if remaining >= 86400 {
		return fmt.Sprintf(" %s↻%dd%dh%s", dim, remaining/86400, remaining%86400/3600, reset)
	}
	return fmt.Sprintf(" %s↻%dh%dm%s", dim, remaining/3600, remaining%3600/60, reset)
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
