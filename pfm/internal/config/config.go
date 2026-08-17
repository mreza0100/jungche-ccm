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

const Version = 1

type Source string

const (
	SourceDefault Source = "default"
	SourceFile    Source = "file"
)

const (
	PermissionBypass = "bypass"
	PermissionPrompt = "prompt"
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
}

type Claude struct {
	PermissionMode string
	Binary         string
}

type Codex struct {
	Yolo   bool
	Binary string
}

type MCPServer struct {
	Enabled bool
}

// Config is the fully materialized configuration. Sources records whether
// each effective value came from the machine file or from a default.
type Config struct {
	Version    int
	Accounts   []Account
	Claude     Claude
	Codex      Codex
	MCPServers map[string]MCPServer

	Path    string
	Exists  bool
	Sources map[string]Source
}

func productionMCPServers() map[string]MCPServer {
	return map[string]MCPServer{"chat": {Enabled: false}}
}

type rawConfig struct {
	Version  *int          `json:"version"`
	Accounts *[]rawAccount `json:"accounts,omitempty"`
	Claude   *rawClaude    `json:"claude,omitempty"`
	Codex    *rawCodex     `json:"codex,omitempty"`
	MCP      *rawMCP       `json:"mcp,omitempty"`
}

type rawAccount struct {
	ID        int    `json:"id"`
	ConfigDir string `json:"configDir"`
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
	Servers map[string]rawMCPServer `json:"servers,omitempty"`
}

type rawMCPServer struct {
	Enabled *bool `json:"enabled,omitempty"`
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
			})
		}
	}
	sources := map[string]Source{
		"version":               SourceDefault,
		"accounts":              SourceDefault,
		"claude.permissionMode": SourceDefault,
		"claude.binary":         SourceDefault,
		"codex.yolo":            SourceDefault,
		"codex.binary":          SourceDefault,
	}
	servers := make(map[string]MCPServer, len(registered))
	for name, server := range registered {
		servers[name] = server
		sources["mcp.servers."+name+".enabled"] = SourceDefault
	}
	return Config{
		Version:    Version,
		Accounts:   accounts,
		Claude:     Claude{PermissionMode: PermissionBypass, Binary: "claude"},
		Codex:      Codex{Yolo: true, Binary: "codex"},
		MCPServers: servers,
		Sources:    sources,
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
	if *raw.Version != Version {
		return Config{}, fmt.Errorf("config %s: version must be %d, got %d", result.Path, Version, *raw.Version)
	}
	result.Sources["version"] = SourceFile

	if raw.Accounts != nil {
		accounts, err := validateAccounts(*raw.Accounts, home)
		if err != nil {
			return Config{}, fmt.Errorf("config %s: accounts: %w", result.Path, err)
		}
		result.Accounts = accounts
		result.Sources["accounts"] = SourceFile
	}
	if raw.Claude != nil {
		if raw.Claude.PermissionMode != nil {
			mode := *raw.Claude.PermissionMode
			if mode != PermissionBypass && mode != PermissionPrompt {
				return Config{}, fmt.Errorf("config %s: claude.permissionMode must be %q or %q, got %q", result.Path, PermissionBypass, PermissionPrompt, mode)
			}
			result.Claude.PermissionMode = mode
			result.Sources["claude.permissionMode"] = SourceFile
		}
		if raw.Claude.Binary != nil {
			if strings.TrimSpace(*raw.Claude.Binary) == "" || strings.ContainsRune(*raw.Claude.Binary, '\x00') {
				return Config{}, fmt.Errorf("config %s: claude.binary must be a non-empty command", result.Path)
			}
			result.Claude.Binary = *raw.Claude.Binary
			result.Sources["claude.binary"] = SourceFile
		}
	}
	if raw.Codex != nil {
		if raw.Codex.Yolo != nil {
			result.Codex.Yolo = *raw.Codex.Yolo
			result.Sources["codex.yolo"] = SourceFile
		}
		if raw.Codex.Binary != nil {
			if strings.TrimSpace(*raw.Codex.Binary) == "" || strings.ContainsRune(*raw.Codex.Binary, '\x00') {
				return Config{}, fmt.Errorf("config %s: codex.binary must be a non-empty command", result.Path)
			}
			result.Codex.Binary = *raw.Codex.Binary
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
			result.Sources["mcp.servers."+name+".enabled"] = SourceFile
		}
	}
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
	for _, account := range config.Accounts {
		if account.ID == id {
			return account, true
		}
	}
	return Account{}, false
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
