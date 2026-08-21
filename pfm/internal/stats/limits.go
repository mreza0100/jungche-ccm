package stats

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"hostops/pfm/internal/usagehook"
)

const defaultCodexUsageEndpoint = "https://chatgpt.com/backend-api/wham/usage"

type LimitAccount struct {
	ID            int
	Emoji         string
	Engine        string
	Label         string
	SkipReason    string
	ConfigDir     string
	ClaudeBinary  string
	CodexAuthPath string
}

type LimitsSampler struct {
	Accounts      []LimitAccount
	Now           func() time.Time
	TTL           time.Duration
	Client        *http.Client
	Endpoint      string
	CodexClient   *http.Client
	CodexEndpoint string
	Fetch         func(context.Context, LimitAccount) (usagehook.Usage, error)
	FetchCodex    func(context.Context, LimitAccount) (codexUsage, error)
	Ack           func(context.Context, LimitAccount) error

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
	sampler.FetchCodex = sampler.fetchCodex
	sampler.Ack = defaultAck
	return sampler
}

func (sampler *LimitsSampler) client() *http.Client {
	if sampler.Client != nil {
		return sampler.Client
	}
	return &http.Client{Timeout: 6 * time.Second}
}

func (sampler *LimitsSampler) codexClient() *http.Client {
	if sampler.CodexClient != nil {
		return sampler.CodexClient
	}
	return &http.Client{
		Timeout: 20 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
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
	if engine == "codex" {
		usage, err := sampler.FetchCodex(ctx, account)
		if err != nil {
			entry.limits.Status = err.Error()
			entry.warnings = append(entry.warnings, fmt.Sprintf("%s limits unavailable: %v", label, err))
		} else {
			entry.limits.Plan = usage.PlanType
			entry.limits.ConfirmedAt = now
			entry.limits.Windows = codexWindows(usage)
		}
		if err == nil && len(entry.limits.Windows) == 0 {
			entry.limits.Status = "Codex payload unreadable"
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
		entry.limits.ConfirmedAt = now
		entry.limits.Windows = usageWindows(usage, now)
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
	return fmt.Sprintf("%s:%d:%s:%s", account.Engine, account.ID, account.ConfigDir, account.CodexAuthPath)
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

func usageWindows(usage usagehook.Usage, now time.Time) []Window {
	named := usage.NamedWindowsAt(now)
	windows := make([]Window, 0, len(named))
	for _, entry := range named {
		source := entry.Window
		if source.Utilization == nil {
			continue
		}
		resetAt, resetNote := parseReset(source.ResetsAt)
		windows = append(windows, Window{
			Name: entry.Label, UsedPct: *source.Utilization, ResetAt: resetAt, ResetNote: resetNote,
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

type codexCredentialEnvelope struct {
	Tokens struct {
		AccessToken string `json:"access_token"`
		AccountID   string `json:"account_id"`
	} `json:"tokens"`
}

type codexUsage struct {
	PlanType  string `json:"plan_type"`
	RateLimit *struct {
		PrimaryWindow   *codexRateLimitWindow `json:"primary_window"`
		SecondaryWindow *codexRateLimitWindow `json:"secondary_window"`
	} `json:"rate_limit"`
}

type codexRateLimitWindow struct {
	UsedPercent        float64 `json:"used_percent"`
	ResetAt            int64   `json:"reset_at"`
	LimitWindowSeconds int64   `json:"limit_window_seconds"`
}

func loadCodexCredentials(path string) (accessToken, accountID string, err error) {
	if strings.TrimSpace(path) == "" {
		return "", "", fmt.Errorf("no local Codex sign-in")
	}
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", fmt.Errorf("no local Codex sign-in")
		}
		return "", "", fmt.Errorf("Codex fetch failed: read local sign-in: %w", err)
	}
	var envelope codexCredentialEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return "", "", fmt.Errorf("Codex session incomplete")
	}
	accessToken = strings.TrimSpace(envelope.Tokens.AccessToken)
	accountID = strings.TrimSpace(envelope.Tokens.AccountID)
	if accessToken == "" || accountID == "" {
		return "", "", fmt.Errorf("Codex session incomplete")
	}
	return accessToken, accountID, nil
}

func (sampler *LimitsSampler) fetchCodex(ctx context.Context, account LimitAccount) (codexUsage, error) {
	accessToken, accountID, err := loadCodexCredentials(account.CodexAuthPath)
	if err != nil {
		return codexUsage{}, err
	}
	endpoint := sampler.CodexEndpoint
	if endpoint == "" {
		endpoint = defaultCodexUsageEndpoint
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return codexUsage{}, fmt.Errorf("Codex fetch failed: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("ChatGPT-Account-ID", accountID)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Cache-Control", "no-cache")
	request.Header.Set("Pragma", "no-cache")

	response, err := sampler.codexClient().Do(request)
	if err != nil {
		return codexUsage{}, fmt.Errorf("Codex fetch failed: %v", err)
	}
	body, readErr := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	closeErr := response.Body.Close()
	if readErr != nil {
		return codexUsage{}, fmt.Errorf("Codex fetch failed: %v", readErr)
	}
	if closeErr != nil {
		return codexUsage{}, fmt.Errorf("Codex fetch failed: %v", closeErr)
	}
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return codexUsage{}, fmt.Errorf("Codex credential rejected (HTTP %d)", response.StatusCode)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return codexUsage{}, fmt.Errorf("Codex fetch failed: HTTP %d", response.StatusCode)
	}

	var usage codexUsage
	if err := json.Unmarshal(body, &usage); err != nil || usage.RateLimit == nil || usage.RateLimit.PrimaryWindow == nil {
		return codexUsage{}, fmt.Errorf("Codex payload unreadable")
	}
	return usage, nil
}

func codexWindows(usage codexUsage) []Window {
	windows := make([]Window, 0, 2)
	for _, entry := range []*codexRateLimitWindow{usage.RateLimit.PrimaryWindow, usage.RateLimit.SecondaryWindow} {
		if entry == nil {
			continue
		}
		window := Window{
			Name:    codexWindowName(entry.LimitWindowSeconds),
			UsedPct: entry.UsedPercent, ResetAt: time.Unix(entry.ResetAt, 0),
		}
		if entry.ResetAt <= 0 {
			window.ResetAt = time.Time{}
			window.ResetNote = "reset unavailable"
		}
		windows = append(windows, window)
	}
	return windows
}

func codexWindowName(seconds int64) string {
	switch seconds {
	case 18_000:
		return "5h"
	case 604_800:
		return "7d"
	}
	if seconds > 0 && seconds%86_400 == 0 {
		return fmt.Sprintf("%dd", seconds/86_400)
	}
	if seconds > 0 && seconds%3_600 == 0 {
		return fmt.Sprintf("%dh", seconds/3_600)
	}
	if seconds > 0 && seconds%60 == 0 {
		return fmt.Sprintf("%dm", seconds/60)
	}
	return fmt.Sprintf("%ds", seconds)
}
