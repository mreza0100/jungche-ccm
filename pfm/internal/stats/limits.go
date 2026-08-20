package stats

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"hostops/pfm/internal/usagehook"
)

type LimitAccount struct {
	ID             int
	Emoji          string
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
	cache        map[int]cachedLimits
	ackAttempted map[int]bool
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
		cache:        make(map[int]cachedLimits),
		ackAttempted: make(map[int]bool),
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
		cached, found := sampler.cached(account.ID, now)
		if !found {
			cached = sampler.refresh(ctx, account, now)
		}
		limits = append(limits, cached.limits)
		warnings = append(warnings, cached.warnings...)
	}
	return limits, warnings
}

func (sampler *LimitsSampler) cached(account int, now time.Time) (cachedLimits, bool) {
	sampler.mu.Lock()
	defer sampler.mu.Unlock()
	entry, ok := sampler.cache[account]
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

func (sampler *LimitsSampler) refresh(ctx context.Context, account LimitAccount, now time.Time) cachedLimits {
	entry := cachedLimits{limits: AccountLimits{Account: account.ID, Emoji: account.Emoji}, when: now}
	if account.CodexCachePath != "" {
		entry.limits.Windows, entry.warnings = readCodexLimits(account.CodexCachePath, account.ID)
		if len(entry.limits.Windows) != 0 {
			sampler.store(account.ID, entry)
			return entry
		}
	}
	usage, err := sampler.Fetch(ctx, account)
	if err != nil && needsCredentialRefresh(err) {
		if sampler.tryAck(ctx, account) == nil {
			usage, err = sampler.Fetch(ctx, account)
		}
	}
	if err != nil {
		entry.warnings = append(entry.warnings, fmt.Sprintf("account %d limits unavailable: %v", account.ID, err))
	} else {
		entry.limits.Windows = usageWindows(usage)
		if len(entry.limits.Windows) == 0 {
			entry.warnings = append(entry.warnings, fmt.Sprintf("account %d limits unavailable: empty usage response", account.ID))
		}
	}
	sampler.store(account.ID, entry)
	return entry
}

func (sampler *LimitsSampler) store(account int, entry cachedLimits) {
	sampler.mu.Lock()
	sampler.cache[account] = entry
	sampler.mu.Unlock()
}

func (sampler *LimitsSampler) tryAck(ctx context.Context, account LimitAccount) error {
	sampler.mu.Lock()
	if sampler.ackAttempted[account.ID] {
		sampler.mu.Unlock()
		return fmt.Errorf("credential refresh already attempted for account %d", account.ID)
	}
	sampler.ackAttempted[account.ID] = true
	sampler.mu.Unlock()
	if sampler.Ack == nil {
		return fmt.Errorf("credential refresh unavailable for account %d", account.ID)
	}
	return sampler.Ack(ctx, account)
}

func needsCredentialRefresh(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, marker := range []string{"credential", "access token", "401", "unauthorized", "forbidden"} {
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
	windows := make([]Window, 0, 4+len(usage.Extra))
	appendWindow := func(name string, source usagehook.Window) {
		if source.Utilization == nil {
			return
		}
		windows = append(windows, Window{
			Name: name, UsedPct: int(*source.Utilization), ResetAt: parseReset(source.ResetsAt),
		})
	}
	appendWindow("5h", usage.FiveHour)
	appendWindow("7d", usage.SevenDay)
	appendWindow("7d-opus", usage.SevenOpus)
	appendWindow("7d-fable", usage.SevenFable)
	extra := make([]string, 0, len(usage.Extra))
	for name := range usage.Extra {
		extra = append(extra, name)
	}
	sort.Strings(extra)
	for _, name := range extra {
		appendWindow(displayWindowName(name), usage.Extra[name])
	}
	return windows
}

func displayWindowName(name string) string {
	name = strings.TrimPrefix(name, "seven_day_")
	name = strings.ReplaceAll(name, "_", "-")
	if name == "" {
		return "7d"
	}
	return "7d-" + name
}

func parseReset(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
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

func readCodexLimits(path string, account int) ([]Window, []string) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, []string{fmt.Sprintf("account %d limits unavailable: read Codex cache: %v", account, err)}
	}
	var cache codexUsageCache
	if err := json.Unmarshal(body, &cache); err != nil {
		return nil, []string{fmt.Sprintf("account %d limits unavailable: decode Codex cache: %v", account, err)}
	}
	windows := make([]Window, 0, 2)
	for _, entry := range []*codexWindow{cache.Primary, cache.Secondary} {
		if entry == nil {
			continue
		}
		windows = append(windows, Window{
			Name:    fmt.Sprintf("codex-%s", durationLabel(entry.WindowDurationMins)),
			UsedPct: int(entry.UsedPercent), ResetAt: time.Unix(entry.ResetsAt, 0),
		})
	}
	return windows, nil
}

func durationLabel(minutes int64) string {
	if minutes >= 24*60 {
		return fmt.Sprintf("%dd", minutes/(24*60))
	}
	return fmt.Sprintf("%dh", minutes/60)
}
