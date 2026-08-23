package stats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"hostops/pfm/internal/deps"
	pfmengine "hostops/pfm/internal/engine"
	"hostops/pfm/internal/usagehook"
)

const defaultCodexUsageEndpoint = "https://chatgpt.com/backend-api/wham/usage"

type LimitAccount struct {
	ID            int
	Emoji         string
	Engine        pfmengine.ID
	Label         string
	Absent        bool
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

// defaultLimitsTTL is shared by the in-memory per-process read-through layer
// and the on-disk cache freshness check. It matches usagehook's default
// CC_USAGE_TTL cadence so the prompt hook and every open picker agree on when
// one shared provider refresh is actually due.
const defaultLimitsTTL = 3 * time.Minute

// A last-good payload remains useful through a short provider outage. This is
// the same stale horizon the prompt hook already applies; after it expires the
// sampler reports the fetch failure without presenting old quota as current.
const maxStaleLimitsAge = time.Hour

func NewLimitsSampler(accounts []LimitAccount) *LimitsSampler {
	copyAccounts := append([]LimitAccount(nil), accounts...)
	sampler := &LimitsSampler{
		Accounts:     copyAccounts,
		TTL:          defaultLimitsTTL,
		cache:        make(map[string]cachedLimits),
		ackAttempted: make(map[string]bool),
	}
	// Nil Fetch/FetchCodex selects the shared on-disk cache path. Tests may
	// override either seam directly; an override intentionally bypasses that
	// cache exactly like it bypasses the real network call.
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

// fetchClaudeCached is the default Fetch implementation: it reads and writes
// the SAME acct-<id>.json the UserPromptSubmit hook owns
// (usagehook.DefaultCacheDir/CachePath), so a second `pfm ls` opened moments
// after the first — or opened right after the hook already ran — renders
// from that file instead of paying for its own request. A record whose
// stored ConfigDir doesn't match this account's is never trusted: an empty
// ConfigDir means the hook wrote it (hook cache files carry no identity, and
// are always trusted), a mismatched one means a DIFFERENT account is
// occupying this account-number slot and the cached payload is not ours.
func (sampler *LimitsSampler) fetchClaude(
	ctx context.Context,
	account LimitAccount,
) (usagehook.Usage, time.Time, error) {
	if sampler.Fetch != nil {
		usage, err := sampler.Fetch(ctx, account)
		if err != nil {
			return usage, time.Time{}, err
		}
		return usage, sampler.now(), nil
	}
	return sampler.fetchClaudeCached(ctx, account, false)
}

// fetchClaudeAfterCredentialRefresh performs the one live retry authorized by
// a successful ACK refresh. It bypasses a credential backoff written by an
// older process/version; transient provider backoffs remain untouched on the
// normal path.
func (sampler *LimitsSampler) fetchClaudeAfterCredentialRefresh(
	ctx context.Context,
	account LimitAccount,
) (usagehook.Usage, time.Time, error) {
	if sampler.Fetch != nil {
		usage, err := sampler.Fetch(ctx, account)
		if err != nil {
			return usage, time.Time{}, err
		}
		return usage, sampler.now(), nil
	}
	return sampler.fetchClaudeCached(ctx, account, true)
}

func (sampler *LimitsSampler) fetchClaudeCached(
	ctx context.Context,
	account LimitAccount,
	bypassCredentialBackoff bool,
) (usagehook.Usage, time.Time, error) {
	now := sampler.now()
	path := usagehook.CachePath(usagehook.DefaultCacheDir(), account.ID)
	record, readErr := usagehook.ReadCacheRecord(path)
	matches := readErr == nil && (record.ConfigDir == "" || record.ConfigDir == account.ConfigDir)
	confirmedAt, confirmed := cacheConfirmedAt(record.FetchedAt, path)
	staleUsable := matches && confirmed && reusableClaudeUsage(record.Usage, now) &&
		now.Sub(confirmedAt) <= maxStaleLimitsAge
	if matches && record.Backoff != nil && now.Before(record.Backoff.RetryAfter) {
		err := errors.New(record.Backoff.Message)
		if !bypassCredentialBackoff || !needsCredentialRefresh(err) {
			if staleUsable && staleEligible(err) {
				return record.Usage, confirmedAt, err
			}
			return usagehook.Usage{}, time.Time{}, err
		}
	}
	if !bypassCredentialBackoff && matches && cacheFresh(record.FetchedAt, path, now, sampler.ttl()) {
		return record.Usage, confirmedAt, nil
	}
	previousUsage, previousFetchedAt := usagehook.Usage{}, (*time.Time)(nil)
	if matches {
		previousUsage, previousFetchedAt = record.Usage, record.FetchedAt
		if previousFetchedAt == nil && confirmed {
			stamp := confirmedAt
			previousFetchedAt = &stamp
		}
	}
	usage, err := usagehook.Fetch(ctx, usagehook.Options{
		ConfigDir: account.ConfigDir,
		Client:    sampler.client(),
		Endpoint:  sampler.Endpoint,
	})
	if err != nil {
		// Credential failures get one ACK refresh followed by an immediate live
		// retry. Recording backoff here would block that retry with the failure
		// it is specifically intended to repair.
		if needsCredentialRefresh(err) {
			return usagehook.Usage{}, time.Time{}, err
		}
		message, retryAfter := backoffFor(err, now)
		// Best-effort: a failed cache write never turns a real fetch result
		// (here, a real error) into something else. The next reader that
		// finds no usable record simply refetches — this file is a shared
		// accelerator, never the only copy of the truth.
		_ = usagehook.WriteCacheRecord(path, usagehook.CacheRecord{
			Usage: previousUsage, ConfigDir: account.ConfigDir, FetchedAt: previousFetchedAt,
			Backoff: &usagehook.CacheBackoff{Message: message, RetryAfter: retryAfter, RecordedAt: now},
		})
		var rateLimit *usagehook.RateLimitError
		if errors.As(err, &rateLimit) {
			err = errors.New(message)
		}
		if staleUsable && staleEligible(err) {
			return previousUsage, confirmedAt, err
		}
		return usagehook.Usage{}, time.Time{}, err
	}
	fetchedAt := now
	_ = usagehook.WriteCacheRecord(path, usagehook.CacheRecord{
		Usage: usage, ConfigDir: account.ConfigDir, FetchedAt: &fetchedAt,
	})
	return usage, fetchedAt, nil
}

// cacheFresh reports whether a cached payload is still inside ttl. FetchedAt
// is nil for a bare, hook-written file (usage-hook's own refresh() has never
// stamped one); freshness then falls back to the file's own mtime, which is
// exactly the signal the hook's own cacheAge() uses.
func cacheFresh(fetchedAt *time.Time, path string, now time.Time, ttl time.Duration) bool {
	confirmedAt, ok := cacheConfirmedAt(fetchedAt, path)
	return ok && now.Sub(confirmedAt) < ttl
}

func cacheConfirmedAt(fetchedAt *time.Time, path string) (time.Time, bool) {
	if fetchedAt != nil {
		return *fetchedAt, true
	}
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, false
	}
	return info.ModTime(), true
}

// backoffFor turns a fetch error into the shared cache's backoff record: a
// 429 backs off for at least ten minutes — honoring whatever Retry-After the
// server sent — and reports the SAME "limits unavailable: 429 ... — retry at
// HH:MM" message whether a caller hit the 429 directly or is replaying the
// record. Every other failure backs off for sixty seconds, just long enough
// that two pickers opened together don't both pay for the same dead
// endpoint, and keeps its original message so isCredentialRejection /
// needsCredentialRefresh still recognize a replayed failure exactly like a
// live one.
func backoffFor(err error, now time.Time) (message string, retryAfter time.Time) {
	var rateLimit *usagehook.RateLimitError
	if errors.As(err, &rateLimit) {
		wait := rateLimit.RetryAfter
		if wait < 10*time.Minute {
			wait = 10 * time.Minute
		}
		retryAfter = now.Add(wait)
		return fmt.Sprintf("limits unavailable: 429 Too Many Requests — retry at %s", retryAfter.Format("15:04")), retryAfter
	}
	return err.Error(), now.Add(time.Minute)
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
		return defaultLimitsTTL
	}
	return sampler.TTL
}

func (sampler *LimitsSampler) refresh(ctx context.Context, account LimitAccount, key string, now time.Time) cachedLimits {
	engine := account.Engine
	label := account.Label
	if label == "" && engine != "" {
		label = fmt.Sprintf("%s account %d", pfmengine.MustLookup(engine).Short, account.ID)
	}
	entry := cachedLimits{limits: AccountLimits{
		Account: account.ID, Emoji: account.Emoji, Engine: engine, Label: label, Absent: account.Absent,
	}, when: now}
	source, err := UsageSourceFor(engine)
	if err != nil {
		entry.limits.Status = err.Error()
		entry.warnings = append(entry.warnings, err.Error())
		sampler.store(key, entry)
		return entry
	}
	if account.Absent {
		entry.limits.Status = label
		sampler.store(key, entry)
		return entry
	}
	if account.SkipReason != "" {
		entry.limits.Status = fmt.Sprintf("skipped %s: %s", label, account.SkipReason)
		sampler.store(key, entry)
		return entry
	}
	fetched, fetchErr := source.Fetch(withLimitsSampler(ctx, sampler), account)
	applyFetchedLimits(&entry.limits, fetched)
	if fetchErr != nil {
		if entry.limits.Status == "" {
			entry.limits.Status = fetchErr.Error()
		}
		entry.warnings = append(entry.warnings, fetchErr.Error())
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

func staleEligible(err error) bool {
	if err == nil || needsCredentialRefresh(err) {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, marker := range []string{"sign-in", "session incomplete"} {
		if strings.Contains(message, marker) {
			return false
		}
	}
	return true
}

func staleStatus(err error) string {
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "429") || strings.Contains(message, "too many requests") {
		return "provider rate-limited; showing cached limits"
	}
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(message, "deadline exceeded") ||
		strings.Contains(message, "timeout") || strings.Contains(message, "timed out") {
		return "refresh timed out; showing cached limits"
	}
	for _, marker := range []string{
		"500 internal server error", "502 bad gateway", "503 service unavailable", "504 gateway timeout",
	} {
		if strings.Contains(message, marker) {
			return "provider temporarily unavailable; showing cached limits"
		}
	}
	return "refresh failed; showing cached limits"
}

func reusableClaudeUsage(usage usagehook.Usage, now time.Time) bool {
	return len(usageWindows(usage, now)) > 0
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
		binary = pfmengine.MustLookup(pfmengine.Claude).Binary
	}
	command := exec.CommandContext(ctx, deps.Executable(binary), "-p", "ACK", "--model", "claude-haiku-4-5", "--max-turns", "1")
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
	if response.StatusCode == http.StatusTooManyRequests {
		return codexUsage{}, &usagehook.RateLimitError{
			RetryAfter: usagehook.ParseRetryAfter(response.Header.Get("Retry-After"), sampler.now()),
		}
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

// codexCacheRecord is codex's on-disk shape of the shared usage cache —
// codex-<id>.json alongside the hook's own acct-<id>.json, same directory,
// same rules. There is no hook writer for codex to stay compatible with (the
// UserPromptSubmit hook never fetches Codex usage), so unlike CacheRecord's
// ConfigDir this always carries the CodexAuthPath that produced it.
type codexCacheRecord struct {
	codexUsage
	CodexAuthPath string                  `json:"codex_auth_path,omitempty"`
	FetchedAt     *time.Time              `json:"fetched_at,omitempty"`
	Backoff       *usagehook.CacheBackoff `json:"backoff,omitempty"`
}

func codexCachePath(cacheDir string, account int) string {
	return filepath.Join(cacheDir, fmt.Sprintf("codex-%d.json", account))
}

func readCodexCacheRecord(path string) (codexCacheRecord, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return codexCacheRecord{}, err
	}
	var record codexCacheRecord
	if err := json.Unmarshal(body, &record); err != nil {
		return codexCacheRecord{}, fmt.Errorf("decode codex usage cache %s: %w", path, err)
	}
	return record, nil
}

func writeCodexCacheRecord(path string, record codexCacheRecord) error {
	if err := usagehook.EnsurePrivateDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	body, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode codex usage cache %s: %w", path, err)
	}
	return usagehook.AtomicWrite(path, body, 0o600)
}

func (sampler *LimitsSampler) fetchCodexForAccount(
	ctx context.Context,
	account LimitAccount,
) (codexUsage, time.Time, error) {
	if sampler.FetchCodex != nil {
		usage, err := sampler.FetchCodex(ctx, account)
		if err != nil {
			return usage, time.Time{}, err
		}
		return usage, sampler.now(), nil
	}
	return sampler.fetchCodexCached(ctx, account)
}

// fetchCodexCached is codex's twin of fetchClaudeCached. A record whose stored
// CodexAuthPath doesn't match this account's is never trusted: it belongs to a
// different Codex sign-in that previously occupied this account-number slot.
func (sampler *LimitsSampler) fetchCodexCached(
	ctx context.Context,
	account LimitAccount,
) (codexUsage, time.Time, error) {
	now := sampler.now()
	path := codexCachePath(usagehook.DefaultCacheDir(), account.ID)
	record, readErr := readCodexCacheRecord(path)
	matches := readErr == nil && record.CodexAuthPath == account.CodexAuthPath
	confirmedAt, confirmed := cacheConfirmedAt(record.FetchedAt, path)
	staleUsable := matches && confirmed && len(codexWindows(record.codexUsage)) > 0 &&
		now.Sub(confirmedAt) <= maxStaleLimitsAge
	if matches && record.Backoff != nil && now.Before(record.Backoff.RetryAfter) {
		err := errors.New(record.Backoff.Message)
		if staleUsable && staleEligible(err) {
			return record.codexUsage, confirmedAt, err
		}
		return codexUsage{}, time.Time{}, err
	}
	if matches && cacheFresh(record.FetchedAt, path, now, sampler.ttl()) {
		return record.codexUsage, confirmedAt, nil
	}
	previousUsage, previousFetchedAt := codexUsage{}, (*time.Time)(nil)
	if matches {
		previousUsage, previousFetchedAt = record.codexUsage, record.FetchedAt
		if previousFetchedAt == nil && confirmed {
			stamp := confirmedAt
			previousFetchedAt = &stamp
		}
	}
	usage, err := sampler.fetchCodex(ctx, account)
	if err != nil {
		message, retryAfter := backoffFor(err, now)
		// Best-effort — see fetchClaudeCached's identical comment.
		_ = writeCodexCacheRecord(path, codexCacheRecord{
			codexUsage: previousUsage, CodexAuthPath: account.CodexAuthPath, FetchedAt: previousFetchedAt,
			Backoff: &usagehook.CacheBackoff{Message: message, RetryAfter: retryAfter, RecordedAt: now},
		})
		var rateLimit *usagehook.RateLimitError
		if errors.As(err, &rateLimit) {
			err = errors.New(message)
		}
		if staleUsable && staleEligible(err) {
			return previousUsage, confirmedAt, err
		}
		return codexUsage{}, time.Time{}, err
	}
	fetchedAt := now
	_ = writeCodexCacheRecord(path, codexCacheRecord{
		codexUsage: usage, CodexAuthPath: account.CodexAuthPath, FetchedAt: &fetchedAt,
	})
	return usage, fetchedAt, nil
}

func codexWindows(usage codexUsage) []Window {
	if usage.RateLimit == nil {
		return nil
	}
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
