// Package config owns pfm's versioned machine configuration.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const Version = 2

type Source string

const (
	SourceDefault Source = "default"
	SourceFile    Source = "file"
)

const (
	PermissionBypass = "bypass"
	PermissionPrompt = "prompted"
)

// Account is one Claude account. ProjectDir is the transcript root used by
// fleet discovery; ConfigDir is the directory handed to Claude at launch.
// Implicit is true only for the default account whose current launch shape
// deliberately leaves CLAUDE_CONFIG_DIR unset.
type Account struct {
	ID         int
	ConfigDir  string
	ProjectDir string
	Implicit   bool
	Emoji      string
	Claude     *ClaudePrefs
	Codex      *CodexPrefs
}

type ClaudePrefs struct {
	PermissionMode string
	Binary         string
}

// Claude is retained as an alias for callers that used the v1 public shape.
type Claude = ClaudePrefs

type CodexPrefs struct {
	Yolo   bool
	Binary string
}

// Codex is retained as an alias for callers that used the v1 public shape.
type Codex = CodexPrefs

type MCPServer struct {
	Enabled bool
}

type MCPHTTP struct {
	Port int
}

type MCPConfig struct {
	Servers   map[string]MCPServer
	HTTP      MCPHTTP
	AuthToken string
}

type EnginePrefs struct {
	Model  string
	Effort string
}

type AskConfig struct {
	Engine string
	Codex  EnginePrefs
	Claude EnginePrefs
}

// Config is the fully materialized configuration. Sources records whether
// each effective value came from the machine file or from a default.
type Config struct {
	Version    int
	Theme      string
	Accounts   []Account
	Claude     Claude
	Codex      Codex
	MCPServers map[string]MCPServer
	MCP        MCPConfig
	Ask        AskConfig

	Path    string
	Exists  bool
	Sources map[string]Source
}

func productionMCPServers() map[string]MCPServer {
	return map[string]MCPServer{
		"chat":      {Enabled: false},
		"harvester": {Enabled: false},
	}
}

type rawConfig struct {
	Version  *int          `json:"version"`
	Theme    *string       `json:"theme,omitempty"`
	Accounts *[]rawAccount `json:"accounts,omitempty"`
	Claude   *rawClaude    `json:"claude,omitempty"`
	Codex    *rawCodex     `json:"codex,omitempty"`
	MCP      *rawMCP       `json:"mcp,omitempty"`
	Ask      *rawAsk       `json:"ask,omitempty"`
}

type rawAccount struct {
	ID        int        `json:"id"`
	ConfigDir string     `json:"configDir"`
	Emoji     string     `json:"emoji,omitempty"`
	Claude    *rawClaude `json:"claude,omitempty"`
	Codex     *rawCodex  `json:"codex,omitempty"`
}

type rawClaude struct {
	PermissionMode *string `json:"permissionMode,omitempty"`
	Binary         *string `json:"binary,omitempty"`
}

type rawCodex struct {
	Yolo   *bool   `json:"yolo,omitempty"`
	Binary *string `json:"binary,omitempty"`
}

type rawMCP struct {
	Servers   map[string]rawMCPServer `json:"servers,omitempty"`
	HTTP      *rawMCPHTTP             `json:"http,omitempty"`
	AuthToken *string                 `json:"authToken,omitempty"`
}

type rawMCPServer struct {
	Enabled *bool `json:"enabled,omitempty"`
}

type rawMCPHTTP struct {
	Port int `json:"port"`
}

type rawAsk struct {
	Engine *string    `json:"engine,omitempty"`
	Codex  *rawEngine `json:"codex,omitempty"`
	Claude *rawEngine `json:"claude,omitempty"`
}

type rawEngine struct {
	Model  *string `json:"model,omitempty"`
	Effort *string `json:"effort,omitempty"`
}

// ResolvePath applies pfm's XDG rule: only an absolute XDG_CONFIG_HOME wins.
func ResolvePath(home string) string {
	root := os.Getenv("XDG_CONFIG_HOME")
	if !filepath.IsAbs(root) {
		root = filepath.Join(home, ".config")
	}
	return filepath.Join(filepath.Clean(root), "pfm", "config.json")
}

// Defaults returns today's effective behavior over the supplied discovery
// roots. The roots are preserved byte-for-byte as discovery inputs; only the
// corresponding launch directory is derived.
func Defaults(home string, projectRoots []string) Config {
	return defaultsWithMCPServers(home, projectRoots, productionMCPServers())
}

func defaultsWithMCPServers(
	home string,
	projectRoots []string,
	registered map[string]MCPServer,
) Config {
	accounts := make([]Account, 0, len(projectRoots))
	for index, projectRoot := range projectRoots {
		configDir := filepath.Dir(projectRoot)
		accounts = append(accounts, Account{
			ID:         index + 1,
			ConfigDir:  filepath.Clean(configDir),
			ProjectDir: filepath.Clean(projectRoot),
			Implicit:   index == 0,
			Emoji:      defaultEmoji(index + 1),
		})
	}
	if len(accounts) == 0 {
		for account := 1; account <= 3; account++ {
			configDir := filepath.Join(home, ".cc", fmt.Sprint(account))
			accounts = append(accounts, Account{
				ID:         account,
				ConfigDir:  configDir,
				ProjectDir: filepath.Join(configDir, "projects"),
				Implicit:   account == 1,
				Emoji:      defaultEmoji(account),
			})
		}
	}
	sources := map[string]Source{
		"version":               SourceDefault,
		"theme":                 SourceDefault,
		"accounts":              SourceDefault,
		"claude.permissionMode": SourceDefault,
		"claude.binary":         SourceDefault,
		"codex.yolo":            SourceDefault,
		"codex.binary":          SourceDefault,
		"mcp.http.port":         SourceDefault,
		"ask.engine":            SourceDefault,
		"ask.codex.model":       SourceDefault,
		"ask.codex.effort":      SourceDefault,
		"ask.claude.model":      SourceDefault,
		"ask.claude.effort":     SourceDefault,
	}
	servers := make(map[string]MCPServer, len(registered))
	for name, server := range registered {
		servers[name] = server
		sources["mcp.servers."+name+".enabled"] = SourceDefault
	}
	return Config{
		Version:    Version,
		Theme:      "default",
		Accounts:   accounts,
		Claude:     Claude{PermissionMode: PermissionBypass, Binary: "claude"},
		Codex:      Codex{Yolo: true, Binary: "codex"},
		MCPServers: servers,
		MCP:        MCPConfig{Servers: cloneMCPServers(servers), HTTP: MCPHTTP{Port: 8377}},
		Ask: AskConfig{
			Engine: "codex",
			Codex:  EnginePrefs{Model: "gpt-5.6-luna", Effort: "low"},
			Claude: EnginePrefs{Model: "claude-haiku-4-5", Effort: "low"},
		},
		Sources: sources,
	}
}

func cloneMCPServers(values map[string]MCPServer) map[string]MCPServer {
	cloned := make(map[string]MCPServer, len(values))
	for name, server := range values {
		cloned[name] = server
	}
	return cloned
}

func defaultEmoji(id int) string {
	switch id {
	case 1:
		return "🥇"
	case 2:
		return "🥈"
	case 3:
		return "🥉"
	case 4:
		return "🍀"
	default:
		return "·"
	}
}

// Load reads a machine config over defaults. An absent file returns defaults;
// every present-file error is returned with the file path attached.
func Load(path, home string, projectRoots []string) (Config, error) {
	return loadWithMCPServers(path, home, projectRoots, productionMCPServers())
}

func loadWithMCPServers(
	path, home string,
	projectRoots []string,
	registered map[string]MCPServer,
) (Config, error) {
	result := defaultsWithMCPServers(home, projectRoots, registered)
	if path == "" {
		path = ResolvePath(home)
	} else if !filepath.IsAbs(path) {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return Config{}, fmt.Errorf("resolve config path %q: %w", path, err)
		}
		path = absolute
	}
	result.Path = filepath.Clean(path)

	content, err := os.ReadFile(result.Path)
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", result.Path, err)
	}
	result.Exists = true

	var raw rawConfig
	if err := decodeStrict(content, &raw); err != nil {
		return Config{}, configJSONError(result.Path, err, int64(len(content)))
	}
	if raw.Version == nil {
		return Config{}, fmt.Errorf("config %s: required key %q is missing", result.Path, "version")
	}
	if *raw.Version != 1 && *raw.Version != Version {
		return Config{}, fmt.Errorf("config %s: version must be 1 or %d, got %d", result.Path, Version, *raw.Version)
	}
	result.Sources["version"] = SourceFile
	// Version 1 is accepted as a compatibility input, but callers always see
	// the current materialized schema version.
	result.Version = Version
	if raw.Theme != nil {
		if strings.TrimSpace(*raw.Theme) == "" {
			return Config{}, fmt.Errorf("config %s: theme must be non-empty", result.Path)
		}
		result.Theme = *raw.Theme
		result.Sources["theme"] = SourceFile
	}

	if raw.Accounts != nil {
		accounts, err := validateAccounts(*raw.Accounts, home)
		if err != nil {
			return Config{}, fmt.Errorf("config %s: accounts: %w", result.Path, err)
		}
		result.Accounts = accounts
		result.Sources["accounts"] = SourceFile
		for index, value := range *raw.Accounts {
			if value.Emoji != "" {
				result.Accounts[index].Emoji = value.Emoji
				result.Sources[fmt.Sprintf("accounts[%d].emoji", index)] = SourceFile
			}
			if value.Claude != nil {
				prefs, err := decodeClaudePrefs(*value.Claude, result.Path, "accounts", index)
				if err != nil {
					return Config{}, err
				}
				result.Accounts[index].Claude = &prefs
			}
			if value.Codex != nil {
				prefs, err := decodeCodexPrefs(*value.Codex, result.Path, "accounts", index)
				if err != nil {
					return Config{}, err
				}
				result.Accounts[index].Codex = &prefs
			}
		}
	}
	if raw.Claude != nil {
		prefs, err := decodeClaudePrefs(*raw.Claude, result.Path, "claude", -1)
		if err != nil {
			return Config{}, err
		}
		if raw.Claude.PermissionMode != nil {
			result.Sources["claude.permissionMode"] = SourceFile
		}
		if raw.Claude.Binary != nil {
			result.Sources["claude.binary"] = SourceFile
		}
		if raw.Claude.PermissionMode != nil {
			result.Claude.PermissionMode = prefs.PermissionMode
		}
		if raw.Claude.Binary != nil {
			result.Claude.Binary = prefs.Binary
		}
	}
	if raw.Codex != nil {
		prefs, err := decodeCodexPrefs(*raw.Codex, result.Path, "codex", -1)
		if err != nil {
			return Config{}, err
		}
		if raw.Codex.Yolo != nil {
			result.Codex.Yolo = prefs.Yolo
			result.Sources["codex.yolo"] = SourceFile
		}
		if raw.Codex.Binary != nil {
			result.Codex.Binary = prefs.Binary
			result.Sources["codex.binary"] = SourceFile
		}
	}
	if raw.MCP != nil {
		for name, server := range raw.MCP.Servers {
			if _, known := registered[name]; !known {
				return Config{}, fmt.Errorf("config %s: unknown key %q", result.Path, "mcp.servers."+name)
			}
			if server.Enabled == nil {
				return Config{}, fmt.Errorf("config %s: required key %q is missing", result.Path, "mcp.servers."+name+".enabled")
			}
			result.MCPServers[name] = MCPServer{Enabled: *server.Enabled}
			result.MCP.Servers[name] = MCPServer{Enabled: *server.Enabled}
			result.Sources["mcp.servers."+name+".enabled"] = SourceFile
		}
		if raw.MCP.HTTP != nil {
			if raw.MCP.HTTP.Port < 1 || raw.MCP.HTTP.Port > 65535 {
				return Config{}, fmt.Errorf("config %s: mcp.http.port must be between 1 and 65535", result.Path)
			}
			result.MCP.HTTP.Port = raw.MCP.HTTP.Port
			result.Sources["mcp.http.port"] = SourceFile
		}
		if raw.MCP.AuthToken != nil {
			result.MCP.AuthToken = *raw.MCP.AuthToken
			result.Sources["mcp.authToken"] = SourceFile
		}
	}
	if raw.Ask != nil {
		if raw.Ask.Engine != nil {
			if *raw.Ask.Engine != "codex" && *raw.Ask.Engine != "claude" {
				return Config{}, fmt.Errorf("config %s: ask.engine must be %q or %q, got %q", result.Path, "codex", "claude", *raw.Ask.Engine)
			}
			result.Ask.Engine = *raw.Ask.Engine
			result.Sources["ask.engine"] = SourceFile
		}
		if raw.Ask.Codex != nil {
			applyEnginePrefs(&result.Ask.Codex, *raw.Ask.Codex)
			if raw.Ask.Codex.Model != nil {
				result.Sources["ask.codex.model"] = SourceFile
			}
			if raw.Ask.Codex.Effort != nil {
				result.Sources["ask.codex.effort"] = SourceFile
			}
		}
		if raw.Ask.Claude != nil {
			applyEnginePrefs(&result.Ask.Claude, *raw.Ask.Claude)
			if raw.Ask.Claude.Model != nil {
				result.Sources["ask.claude.model"] = SourceFile
			}
			if raw.Ask.Claude.Effort != nil {
				result.Sources["ask.claude.effort"] = SourceFile
			}
		}
	}
	result.MCPServers = cloneMCPServers(result.MCP.Servers)
	return result, nil
}

func decodeStrict(content []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func configJSONError(path string, err error, contentSize ...int64) error {
	var syntax *json.SyntaxError
	if errors.As(err, &syntax) {
		return fmt.Errorf("parse config %s at byte %d: %w", path, syntax.Offset, err)
	}
	var kind *json.UnmarshalTypeError
	if errors.As(err, &kind) {
		return fmt.Errorf("parse config %s at byte %d: %w", path, kind.Offset, err)
	}
	if errors.Is(err, io.ErrUnexpectedEOF) && len(contentSize) > 0 {
		return fmt.Errorf("parse config %s at byte %d: %w", path, contentSize[0], err)
	}
	return fmt.Errorf("parse config %s: %w", path, err)
}

func decodeClaudePrefs(raw rawClaude, path, scope string, index int) (ClaudePrefs, error) {
	prefs := ClaudePrefs{}
	if raw.PermissionMode != nil {
		mode := *raw.PermissionMode
		// v1 used "prompt". Keep accepting it as an input alias while
		// materializing the v2 canonical value.
		if mode == "prompt" {
			mode = PermissionPrompt
		}
		if mode != PermissionBypass && mode != PermissionPrompt {
			return ClaudePrefs{}, fmt.Errorf("config %s: %s.permissionMode must be %q or %q, got %q", path, configScope(scope, index), PermissionBypass, PermissionPrompt, *raw.PermissionMode)
		}
		prefs.PermissionMode = mode
	}
	if raw.Binary != nil {
		if strings.TrimSpace(*raw.Binary) == "" || strings.ContainsRune(*raw.Binary, '\x00') {
			return ClaudePrefs{}, fmt.Errorf("config %s: %s.binary must be a non-empty command", path, configScope(scope, index))
		}
		prefs.Binary = *raw.Binary
	}
	return prefs, nil
}

func decodeCodexPrefs(raw rawCodex, path, scope string, index int) (CodexPrefs, error) {
	prefs := CodexPrefs{}
	if raw.Yolo != nil {
		prefs.Yolo = *raw.Yolo
	}
	if raw.Binary != nil {
		if strings.TrimSpace(*raw.Binary) == "" || strings.ContainsRune(*raw.Binary, '\x00') {
			return CodexPrefs{}, fmt.Errorf("config %s: %s.binary must be a non-empty command", path, configScope(scope, index))
		}
		prefs.Binary = *raw.Binary
	}
	return prefs, nil
}

func configScope(scope string, index int) string {
	if index >= 0 {
		return fmt.Sprintf("%s[%d]", scope, index)
	}
	return scope
}

func applyEnginePrefs(target *EnginePrefs, raw rawEngine) {
	if raw.Model != nil {
		target.Model = *raw.Model
	}
	if raw.Effort != nil {
		target.Effort = *raw.Effort
	}
}

func validateAccounts(values []rawAccount, home string) ([]Account, error) {
	if len(values) == 0 {
		return nil, errors.New("roster must contain at least one account")
	}
	seen := make(map[int]bool, len(values))
	accounts := make([]Account, 0, len(values))
	for index, value := range values {
		if value.ID < 1 {
			return nil, fmt.Errorf("entry %d id must be positive", index+1)
		}
		if seen[value.ID] {
			return nil, fmt.Errorf("duplicate id %d", value.ID)
		}
		seen[value.ID] = true
		configDir, err := expandHomePath(value.ConfigDir, home)
		if err != nil {
			return nil, fmt.Errorf("entry %d configDir: %w", index+1, err)
		}
		accounts = append(accounts, Account{
			ID:         value.ID,
			ConfigDir:  configDir,
			ProjectDir: filepath.Join(configDir, "projects"),
			Emoji:      defaultEmoji(value.ID),
		})
	}
	return accounts, nil
}

func expandHomePath(value, home string) (string, error) {
	if strings.ContainsRune(value, '\x00') {
		return "", errors.New("must not contain NUL")
	}
	switch {
	case value == "~":
		value = home
	case strings.HasPrefix(value, "~/"):
		value = filepath.Join(home, strings.TrimPrefix(value, "~/"))
	case value == "$HOME":
		value = home
	case strings.HasPrefix(value, "$HOME/"):
		value = filepath.Join(home, strings.TrimPrefix(value, "$HOME/"))
	}
	if !filepath.IsAbs(value) {
		return "", fmt.Errorf("must be absolute or start with ~/ or $HOME/, got %q", value)
	}
	return filepath.Clean(value), nil
}

func (config Config) Source(key string) Source {
	if source, found := config.Sources[key]; found {
		return source
	}
	return SourceDefault
}

func (config Config) Account(id int) (Account, bool) {
	return config.AccountByID(id)
}

// AccountByID returns the configured account with the requested id.
func (config Config) AccountByID(id int) (Account, bool) {
	for _, account := range config.Accounts {
		if account.ID == id {
			return account, true
		}
	}
	return Account{}, false
}

// EffectiveClaude resolves an account override over the top-level Claude
// posture and binary. Unknown accounts receive the top-level posture.
func (config Config) EffectiveClaude(id int) ClaudePrefs {
	result := config.Claude
	if account, ok := config.AccountByID(id); ok && account.Claude != nil {
		if account.Claude.PermissionMode != "" {
			result.PermissionMode = account.Claude.PermissionMode
		}
		if account.Claude.Binary != "" {
			result.Binary = account.Claude.Binary
		}
	}
	return result
}

// EffectiveCodex resolves an account override over the top-level Codex
// posture and binary. Unknown accounts receive the top-level posture.
func (config Config) EffectiveCodex(id int) CodexPrefs {
	result := config.Codex
	if account, ok := config.AccountByID(id); ok && account.Codex != nil {
		if account.Codex.Yolo {
			result.Yolo = true
		} else if account.Codex != nil {
			// A false override is meaningful, so use the pointer's presence.
			result.Yolo = account.Codex.Yolo
		}
		if account.Codex.Binary != "" {
			result.Binary = account.Codex.Binary
		}
	}
	return result
}

// EmojiFor returns the configured account badge or the honest unknown marker.
func (config Config) EmojiFor(id int) string {
	if account, ok := config.AccountByID(id); ok && account.Emoji != "" {
		return account.Emoji
	}
	return "·"
}

func (config Config) AccountIDs() []int {
	ids := make([]int, 0, len(config.Accounts))
	for _, account := range config.Accounts {
		ids = append(ids, account.ID)
	}
	return ids
}

func (config Config) ProjectRoots() []string {
	roots := make([]string, 0, len(config.Accounts))
	for _, account := range config.Accounts {
		roots = append(roots, account.ProjectDir)
	}
	return roots
}

func RegisteredMCPServers() []string {
	registered := productionMCPServers()
	names := make([]string, 0, len(registered))
	for name := range registered {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// SetMCPServer atomically changes one registered server in the machine file.
// Repeating an already-effective setting is a no-op.
func SetMCPServer(config Config, name string, enabled bool) (bool, error) {
	server, registered := config.MCPServers[name]
	if !registered {
		return false, fmt.Errorf("unknown MCP server %q", name)
	}
	if server.Enabled == enabled {
		return false, nil
	}

	top := make(map[string]json.RawMessage)
	if config.Exists {
		content, err := os.ReadFile(config.Path)
		if err != nil {
			return false, fmt.Errorf("read config %s for update: %w", config.Path, err)
		}
		if err := json.Unmarshal(content, &top); err != nil {
			return false, configJSONError(config.Path, err)
		}
	}
	version, _ := json.Marshal(Version)
	top["version"] = version

	mcpObject := make(map[string]json.RawMessage)
	if content := top["mcp"]; len(content) != 0 {
		if err := json.Unmarshal(content, &mcpObject); err != nil {
			return false, fmt.Errorf("decode config %s mcp for update: %w", config.Path, err)
		}
	}
	servers := make(map[string]json.RawMessage)
	if content := mcpObject["servers"]; len(content) != 0 {
		if err := json.Unmarshal(content, &servers); err != nil {
			return false, fmt.Errorf("decode config %s mcp.servers for update: %w", config.Path, err)
		}
	}
	serverObject := map[string]bool{"enabled": enabled}
	serverContent, _ := json.Marshal(serverObject)
	servers[name] = serverContent
	serversContent, _ := json.Marshal(servers)
	mcpObject["servers"] = serversContent
	mcpContent, _ := json.Marshal(mcpObject)
	top["mcp"] = mcpContent

	content, err := json.MarshalIndent(top, "", "  ")
	if err != nil {
		return false, fmt.Errorf("encode config %s: %w", config.Path, err)
	}
	content = append(content, '\n')
	if err := writeAtomic(config.Path, content); err != nil {
		return false, err
	}
	return true, nil
}

// SetMCPAuthToken atomically records the installer-owned daemon credential in
// the machine config while preserving every unrelated field.
func SetMCPAuthToken(config Config, token string) (bool, error) {
	if config.Path == "" {
		return false, errors.New("MCP auth token update has no config path")
	}
	if config.MCP.AuthToken == token && config.Exists {
		return false, nil
	}
	top := make(map[string]json.RawMessage)
	if config.Exists {
		content, err := os.ReadFile(config.Path)
		if err != nil {
			return false, fmt.Errorf("read config %s for MCP auth update: %w", config.Path, err)
		}
		if err := json.Unmarshal(content, &top); err != nil {
			return false, configJSONError(config.Path, err)
		}
	}
	version, _ := json.Marshal(Version)
	top["version"] = version
	mcpObject := make(map[string]json.RawMessage)
	if content := top["mcp"]; len(content) != 0 {
		if err := json.Unmarshal(content, &mcpObject); err != nil {
			return false, fmt.Errorf("decode config %s mcp for auth update: %w", config.Path, err)
		}
	}
	authContent, _ := json.Marshal(token)
	mcpObject["authToken"] = authContent
	mcpContent, _ := json.Marshal(mcpObject)
	top["mcp"] = mcpContent
	content, err := json.MarshalIndent(top, "", "  ")
	if err != nil {
		return false, fmt.Errorf("encode config %s: %w", config.Path, err)
	}
	if err := writeAtomic(config.Path, append(content, '\n')); err != nil {
		return false, err
	}
	return true, nil
}

func writeAtomic(path string, content []byte) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create config directory %s: %w", directory, err)
	}
	file, err := os.CreateTemp(directory, ".config.json.tmp-*")
	if err != nil {
		return fmt.Errorf("create config scratch beside %s: %w", path, err)
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return fmt.Errorf("secure config scratch %s: %w", temporary, err)
	}
	if _, err := file.Write(content); err != nil {
		_ = file.Close()
		return fmt.Errorf("write config scratch %s: %w", temporary, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close config scratch %s: %w", temporary, err)
	}
	if err := os.Rename(temporary, path); err != nil {
		return fmt.Errorf("install config %s: %w", path, err)
	}
	return nil
}

// MarshalDefault returns the strict, comment-free JSON used by `pfm config
// init`. It deliberately emits resolved defaults so the file is useful as a
// documented starting point while the loader remains backward compatible.
func MarshalDefault(home string, projectRoots []string) ([]byte, error) {
	return Marshal(Defaults(home, projectRoots), false)
}

// WriteDefault installs the default machine file atomically. Existing files
// are protected unless force is explicitly requested.
func WriteDefault(path, home string, projectRoots []string, force bool) error {
	if path == "" {
		path = ResolvePath(home)
	}
	if _, err := os.Stat(path); err == nil && !force {
		return fmt.Errorf("config %s already exists; use --force to overwrite", path)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect config %s: %w", path, err)
	}
	content, err := MarshalDefault(home, projectRoots)
	if err != nil {
		return fmt.Errorf("encode defaults: %w", err)
	}
	return writeAtomic(path, content)
}

// Marshal encodes a resolved config as strict JSON. The redaction option is
// intended for human-facing output only.
func Marshal(config Config, redact bool) ([]byte, error) {
	accounts := make([]map[string]any, 0, len(config.Accounts))
	for _, account := range config.Accounts {
		value := map[string]any{
			"id":        account.ID,
			"configDir": displayPath(account.ConfigDir, configHome(config)),
			"emoji":     account.Emoji,
		}
		if account.Claude != nil {
			value["claude"] = map[string]any{
				"permissionMode": account.Claude.PermissionMode,
				"binary":         account.Claude.Binary,
			}
		}
		if account.Codex != nil {
			value["codex"] = map[string]any{
				"yolo":   account.Codex.Yolo,
				"binary": account.Codex.Binary,
			}
		}
		accounts = append(accounts, value)
	}
	servers := make(map[string]any, len(config.MCP.Servers))
	for name, server := range config.MCP.Servers {
		servers[name] = map[string]any{"enabled": server.Enabled}
	}
	value := map[string]any{
		"version":  config.Version,
		"theme":    config.Theme,
		"accounts": accounts,
		"claude": map[string]any{
			"permissionMode": config.Claude.PermissionMode,
			"binary":         config.Claude.Binary,
		},
		"codex": map[string]any{
			"yolo":   config.Codex.Yolo,
			"binary": config.Codex.Binary,
		},
		"mcp": map[string]any{
			"servers": servers,
			"http":    map[string]any{"port": config.MCP.HTTP.Port},
		},
		"ask": map[string]any{
			"engine": config.Ask.Engine,
			"codex":  map[string]any{"model": config.Ask.Codex.Model, "effort": config.Ask.Codex.Effort},
			"claude": map[string]any{"model": config.Ask.Claude.Model, "effort": config.Ask.Claude.Effort},
		},
	}
	if config.MCP.AuthToken != "" {
		value["mcp"].(map[string]any)["authToken"] = config.MCP.AuthToken
	}
	content, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	if redact {
		return RedactSecrets(append(content, '\n')), nil
	}
	return append(content, '\n'), nil
}

func configHome(config Config) string {
	if len(config.Accounts) == 0 {
		return ""
	}
	// Config paths are already expanded at load time. The serialized default
	// uses absolute paths; this helper exists to keep the conversion explicit
	// and avoid guessing a user's home from an arbitrary account roster.
	return ""
}

func displayPath(value, home string) string {
	if home != "" && (value == home || strings.HasPrefix(value, home+string(filepath.Separator))) {
		return "~" + strings.TrimPrefix(value, home)
	}
	return value
}

// RedactSecrets preserves JSON shape while replacing secret-looking object
// fields. It is intentionally generic so future credentials are safe by
// default without another display-path audit.
func RedactSecrets(content []byte) []byte {
	var value any
	if err := json.Unmarshal(content, &value); err != nil {
		return content
	}
	redactJSON(value)
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return content
	}
	return encoded.Bytes()
}

func redactJSON(value any) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			lower := strings.ToLower(key)
			if strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "credential") || strings.Contains(lower, "password") {
				typed[key] = "<redacted>"
				continue
			}
			redactJSON(child)
		}
	case []any:
		for _, child := range typed {
			redactJSON(child)
		}
	}
}
