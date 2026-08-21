package stats

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"hostops/pfm/internal/usagehook"
)

type LimitAccount struct {
	ID             int
	Emoji          string
	Engine         string
	Label          string
	SkipReason     string
	ConfigDir      string
	ClaudeBinary   string
	CodexCachePath string
}

type LimitsSampler struct {
	Accounts []LimitAccount
	Now      func() time.Time
	TTL      time.Duration
	Client   *http.Client
	Endpoint string
	Fetch    func(context.Context, LimitAccount) (usagehook.Usage, error)
	Ack      func(context.Context, LimitAccount) error

	mu           sync.Mutex
	cache        map[string]cachedLimits
	ackAttempted map[string]bool
}

type cachedLimits struct {
	limits   AccountLimits
	warnings []string
	when     time.Time
}

func NewLimitsSampler(accounts []LimitAccount) *LimitsSampler {
	copyAccounts := append([]LimitAccount(nil), accounts...)
	sampler := &LimitsSampler{
		Accounts:     copyAccounts,
		TTL:          time.Minute,
		cache:        make(map[string]cachedLimits),
		ackAttempted: make(map[string]bool),
	}
	sampler.Fetch = func(ctx context.Context, account LimitAccount) (usagehook.Usage, error) {
		return usagehook.Fetch(ctx, usagehook.Options{
			ConfigDir: account.ConfigDir,
			Client:    sampler.client(),
			Endpoint:  sampler.Endpoint,
		})
	}
	sampler.Ack = defaultAck
	return sampler
}

func (sampler *LimitsSampler) client() *http.Client {
	if sampler.Client != nil {
		return sampler.Client
	}
	return &http.Client{Timeout: 6 * time.Second}
}

func (sampler *LimitsSampler) now() time.Time {
	if sampler.Now != nil {
		return sampler.Now()
	}
	return time.Now()
}

func (sampler *LimitsSampler) Sample(ctx context.Context) ([]AccountLimits, []string) {
	if ctx == nil {
		ctx = context.Background()
	}
	now := sampler.now()
	limits := make([]AccountLimits, 0, len(sampler.Accounts))
	warnings := make([]string, 0)
	for _, account := range sampler.Accounts {
		key := account.cacheKey()
		cached, found := sampler.cached(key, now)
		if !found {
			cached = sampler.refresh(ctx, account, key, now)
		}
		limits = append(limits, cached.limits)
		warnings = append(warnings, cached.warnings...)
	}
	return limits, warnings
}

func (sampler *LimitsSampler) cached(key string, now time.Time) (cachedLimits, bool) {
	sampler.mu.Lock()
	defer sampler.mu.Unlock()
	entry, ok := sampler.cache[key]
	if !ok || now.Sub(entry.when) >= sampler.ttl() {
		return cachedLimits{}, false
	}
	return entry, true
}

func (sampler *LimitsSampler) ttl() time.Duration {
	if sampler.TTL <= 0 {
		return time.Minute
	}
	return sampler.TTL
}

func (sampler *LimitsSampler) refresh(ctx context.Context, account LimitAccount, key string, now time.Time) cachedLimits {
	engine := account.Engine
	if engine == "" {
		engine = "claude"
	}
	label := account.Label
	if label == "" && engine == "claude" {
		label = fmt.Sprintf("account %d", account.ID)
	}
	entry := cachedLimits{limits: AccountLimits{
		Account: account.ID, Emoji: account.Emoji, Engine: engine, Label: label,
	}, when: now}
	if account.SkipReason != "" {
		entry.limits.Status = fmt.Sprintf("skipped %s: %s", label, account.SkipReason)
		sampler.store(key, entry)
		return entry
	}
	if account.CodexCachePath != "" {
		var err error
		entry.limits.Windows, err = readCodexLimits(account.CodexCachePath)
		if err != nil {
			entry.limits.Status = err.Error()
			entry.warnings = append(entry.warnings, fmt.Sprintf("%s limits unavailable: %v", label, err))
		} else if len(entry.limits.Windows) == 0 {
			entry.limits.Status = "Codex cache carried no rate-limit windows"
			entry.warnings = append(entry.warnings, entry.limits.Status)
		}
		sampler.store(key, entry)
		return entry
	}
	usage, err := sampler.Fetch(ctx, account)
	if err != nil && needsCredentialRefresh(err) {
		if sampler.tryAck(ctx, account) == nil {
			usage, err = sampler.Fetch(ctx, account)
		}
	}
	if err != nil {
		if isCredentialRejection(err) {
			entry.limits.Status = fmt.Sprintf("skipped %s: credentials rejected", label)
		} else {
			entry.limits.Status = fmt.Sprintf("account %d limits unavailable: %v", account.ID, err)
			entry.warnings = append(entry.warnings, entry.limits.Status)
		}
	} else {
		entry.limits.Windows = usageWindows(usage)
		if len(entry.limits.Windows) == 0 {
			entry.limits.Status = fmt.Sprintf("account %d limits unavailable: empty usage response", account.ID)
			entry.warnings = append(entry.warnings, entry.limits.Status)
		}
	}
	sampler.store(key, entry)
	return entry
}

func isCredentialRejection(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, marker := range []string{"401", "403", "unauthorized", "forbidden", "access token rejected"} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func (sampler *LimitsSampler) store(key string, entry cachedLimits) {
	sampler.mu.Lock()
	sampler.cache[key] = entry
	sampler.mu.Unlock()
}

func (sampler *LimitsSampler) tryAck(ctx context.Context, account LimitAccount) error {
	key := account.cacheKey()
	sampler.mu.Lock()
	if sampler.ackAttempted[key] {
		sampler.mu.Unlock()
		return fmt.Errorf("credential refresh already attempted for account %d", account.ID)
	}
	sampler.ackAttempted[key] = true
	sampler.mu.Unlock()
	if sampler.Ack == nil {
		return fmt.Errorf("credential refresh unavailable for account %d", account.ID)
	}
	return sampler.Ack(ctx, account)
}

func (account LimitAccount) cacheKey() string {
	return fmt.Sprintf("%s:%d:%s:%s", account.Engine, account.ID, account.ConfigDir, account.CodexCachePath)
}

func needsCredentialRefresh(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, marker := range []string{"credential", "access token", "401", "403", "unauthorized", "forbidden"} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func defaultAck(ctx context.Context, account LimitAccount) error {
	binary := account.ClaudeBinary
	if binary == "" {
		binary = "claude"
	}
	command := exec.CommandContext(ctx, binary, "-p", "ACK", "--model", "claude-haiku-4-5", "--max-turns", "1")
	command.Env = append(os.Environ(), "CLAUDE_CONFIG_DIR="+account.ConfigDir)
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("refresh account %d OAuth token: %w (%s)", account.ID, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func usageWindows(usage usagehook.Usage) []Window {
	named := usage.NamedWindows()
	windows := make([]Window, 0, len(named))
	for _, entry := range named {
		source := entry.Window
		if source.Utilization == nil {
			continue
		}
		resetAt, resetNote := parseReset(source.ResetsAt)
		windows = append(windows, Window{
			Name: entry.Label, UsedPct: int(*source.Utilization), ResetAt: resetAt, ResetNote: resetNote,
		})
	}
	return windows
}

func parseReset(value string) (time.Time, string) {
	if value == "" {
		return time.Time{}, "reset unavailable"
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, "invalid reset timestamp"
	}
	return parsed, ""
}

type codexUsageCache struct {
	Primary   *codexWindow `json:"primary"`
	Secondary *codexWindow `json:"secondary"`
}

type codexWindow struct {
	UsedPercent        float64 `json:"usedPercent"`
	WindowDurationMins int64   `json:"windowDurationMins"`
	ResetsAt           int64   `json:"resetsAt"`
}

func readCodexLimits(path string) ([]Window, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read Codex cache: %w", err)
	}
	var cache codexUsageCache
	if err := json.Unmarshal(body, &cache); err != nil {
		return nil, fmt.Errorf("decode Codex cache: %w", err)
	}
	windows := make([]Window, 0, 2)
	for _, entry := range []*codexWindow{cache.Primary, cache.Secondary} {
		if entry == nil {
			continue
		}
		window := Window{
			Name:    fmt.Sprintf("codex-%s", durationLabel(entry.WindowDurationMins)),
			UsedPct: int(entry.UsedPercent), ResetAt: time.Unix(entry.ResetsAt, 0),
		}
		if entry.ResetsAt <= 0 {
			window.ResetAt = time.Time{}
			window.ResetNote = "reset unavailable"
		}
		windows = append(windows, window)
	}
	return windows, nil
}

func durationLabel(minutes int64) string {
	if minutes >= 24*60 {
		return fmt.Sprintf("%dd", minutes/(24*60))
	}
	return fmt.Sprintf("%dh", minutes/60)
}
