// Package usagehook implements the fail-open UserPromptSubmit usage warning.
package usagehook

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"hostops/pfm/internal/paths"
)

const defaultEndpoint = "https://api.anthropic.com/api/oauth/usage"

// Options is the complete, jail-replaceable hook environment.
type Options struct {
	Now       func() time.Time
	Home      string
	ConfigDir string
	CacheDir  string
	Warn      int
	Critical  int
	TTL       time.Duration
	Client    *http.Client
	Endpoint  string
	Log       io.Writer
}

type usage struct {
	FiveHour  usageWindow `json:"five_hour"`
	SevenDay  usageWindow `json:"seven_day"`
	SevenOpus usageWindow `json:"seven_day_opus"`
}

type usageWindow struct {
	Utilization *float64 `json:"utilization"`
	ResetsAt    string   `json:"resets_at"`
}

type credentials struct {
	OAuth struct {
		AccessToken string `json:"accessToken"`
	} `json:"claudeAiOauth"`
}

// Evaluate returns only model-facing hook text. Callers deliberately swallow
// its error and exit zero: hook infrastructure must never block a prompt.
func Evaluate(ctx context.Context, options Options) (string, error) {
	options = normalize(options)
	credentialPath := filepath.Join(options.ConfigDir, ".credentials.json")
	if _, err := os.Stat(credentialPath); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("stat usage credentials: %w", err)
	}
	account := accountNumber(options.Home, options.ConfigDir)
	if err := ensurePrivateDirectory(options.CacheDir); err != nil {
		return "", err
	}
	now := options.Now()
	cachePath := filepath.Join(options.CacheDir, fmt.Sprintf("acct-%d.json", account))
	if cacheAge(cachePath, now) >= options.TTL {
		if err := refresh(ctx, options, cachePath); err != nil {
			// A stale cache remains usable for one hour, exactly like the shell.
			fmt.Fprintf(options.Log, "pfm usage-hook: refresh failed; trying stale cache: %v\n", err)
		}
	}
	info, err := os.Stat(cachePath)
	if err != nil {
		return "", nil
	}
	if now.Sub(info.ModTime()) > time.Hour {
		return "", nil
	}
	body, err := os.ReadFile(cachePath)
	if err != nil {
		return "", err
	}
	var cached usage
	if err := json.Unmarshal(body, &cached); err != nil {
		return "", err
	}
	five := utilization(cached.FiveHour, 0)
	seven := utilization(cached.SevenDay, 0)
	opus := utilization(cached.SevenOpus, -1)
	maximum := max(five, seven, opus)
	flagPath := filepath.Join(options.CacheDir, fmt.Sprintf("warned-%d", account))
	if maximum < options.Warn {
		if _, err := os.Stat(flagPath); err == nil {
			if err := os.Remove(flagPath); err != nil {
				return "", err
			}
			return fmt.Sprintf(
				"✅ usage recovered — account %d is back to 5h %d%% · 7d %d%%. Any earlier limit warning in this conversation is STALE — ignore it and work normally.\n",
				account,
				five,
				seven,
			), nil
		}
		return "", nil
	}
	if err := os.WriteFile(flagPath, nil, 0o600); err != nil {
		return "", err
	}
	line := fmt.Sprintf(
		"5h %d%% (resets %s) · 7d %d%% (resets %s)",
		five,
		formatReset(cached.FiveHour.ResetsAt, now, "15:04"),
		seven,
		formatReset(cached.SevenDay.ResetsAt, now, "Mon 15:04"),
	)
	if opus >= 0 {
		line += fmt.Sprintf(" · 7d-opus %d%%", opus)
	}
	if maximum >= options.Critical {
		return fmt.Sprintf(
			"🔴 USAGE LIMIT IMMINENT — account %d: %s. This window is nearly exhausted: finish the in-flight step, then /swap to another account (or pause until the reset).\n",
			account,
			line,
		), nil
	}
	return fmt.Sprintf(
		"⚠ usage limit approaching — account %d: %s. Plan around it: wrap up soon or /swap to another account before the cap hits.\n",
		account,
		line,
	), nil
}

func normalize(options Options) Options {
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.Home == "" {
		options.Home = os.Getenv(paths.EnvHome)
		if options.Home == "" {
			options.Home, _ = os.UserHomeDir()
		}
	}
	if options.ConfigDir == "" {
		options.ConfigDir = os.Getenv("CLAUDE_CONFIG_DIR")
		if options.ConfigDir == "" {
			options.ConfigDir = filepath.Join(options.Home, ".claude")
		}
	}
	if options.CacheDir == "" {
		if jailHome := os.Getenv(paths.EnvHome); jailHome != "" {
			options.CacheDir = filepath.Join(jailHome, "tmp", "cc-usage-"+strconv.Itoa(os.Getuid()))
		} else {
			options.CacheDir = filepath.Join(os.TempDir(), "cc-usage-"+strconv.Itoa(os.Getuid()))
		}
	}
	if options.Warn <= 0 {
		options.Warn = envInt("CC_USAGE_WARN", 80)
	}
	if options.Critical <= 0 {
		options.Critical = envInt("CC_USAGE_CRIT", 95)
	}
	if options.TTL <= 0 {
		options.TTL = time.Duration(envInt("CC_USAGE_TTL", 180)) * time.Second
	}
	if options.Client == nil {
		options.Client = &http.Client{Timeout: 6 * time.Second}
	}
	if options.Endpoint == "" {
		options.Endpoint = defaultEndpoint
	}
	if options.Log == nil {
		options.Log = io.Discard
	}
	return options
}

func accountNumber(home, configDir string) int {
	physical, err := filepath.EvalSymlinks(configDir)
	if err != nil {
		physical = configDir
	}
	switch filepath.Clean(physical) {
	case filepath.Join(home, ".claude"):
		return 1
	case filepath.Join(home, ".claude3"):
		return 2
	case filepath.Join(home, ".cc", "3"):
		return 3
	}
	value, err := strconv.Atoi(filepath.Base(physical))
	if err == nil && value > 0 {
		return value
	}
	return 1
}

func ensurePrivateDirectory(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("usage cache is not a real directory")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Getuid() {
		return fmt.Errorf("usage cache is not owned by this uid")
	}
	return os.Chmod(path, 0o700)
}

func refresh(ctx context.Context, options Options, cachePath string) error {
	credentialPath := filepath.Join(options.ConfigDir, ".credentials.json")
	body, err := os.ReadFile(credentialPath)
	if err != nil {
		return err
	}
	var credential credentials
	if err := json.Unmarshal(body, &credential); err != nil {
		return err
	}
	if credential.OAuth.AccessToken == "" {
		return fmt.Errorf("usage credential contains no access token")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, options.Endpoint, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+credential.OAuth.AccessToken)
	request.Header.Set("anthropic-beta", "oauth-2025-04-20")
	response, err := options.Client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("usage endpoint returned %s", response.Status)
	}
	fresh, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return err
	}
	var decoded usage
	if err := json.Unmarshal(fresh, &decoded); err != nil {
		return err
	}
	if decoded.FiveHour.Utilization == nil {
		return fmt.Errorf("usage response omitted five_hour utilization")
	}
	return atomicWrite(cachePath, fresh, 0o600)
}

func atomicWrite(path string, body []byte, mode os.FileMode) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".usage-*")
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

func utilization(window usageWindow, fallback int) int {
	if window.Utilization == nil {
		return fallback
	}
	return int(*window.Utilization)
}

func cacheAge(path string, now time.Time) time.Duration {
	info, err := os.Stat(path)
	if err != nil {
		return 100 * 365 * 24 * time.Hour
	}
	return now.Sub(info.ModTime())
}

func formatReset(raw string, now time.Time, layout string) string {
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return "?"
	}
	return parsed.In(now.Location()).Format(layout)
}

func envInt(name string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(name))
	if err != nil {
		return fallback
	}
	return value
}

func max(values ...int) int {
	maximum := values[0]
	for _, value := range values[1:] {
		if value > maximum {
			maximum = value
		}
	}
	return maximum
}
