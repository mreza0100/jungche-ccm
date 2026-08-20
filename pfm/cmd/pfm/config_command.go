package main

import (
	"fmt"
	"io"
	"strings"

	pfmconfig "hostops/pfm/internal/config"
)

func runConfig(args []string, stdout, stderr io.Writer, runtime commandRuntime) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: pfm config init [--force] | show | validate")
		return 2
	}
	switch args[0] {
	case "init":
		return runConfigInit(args[1:], stdout, stderr, runtime)
	case "show":
		if len(args) != 1 {
			fmt.Fprintln(stderr, "usage: pfm config show")
			return 2
		}
		if runtime.ConfigError != nil {
			fmt.Fprintf(stderr, "pfm config show: configuration error: %v\n", runtime.ConfigError)
		}
		printResolvedConfig(stdout, runtime)
		return 0
	case "validate":
		if len(args) != 1 {
			fmt.Fprintln(stderr, "usage: pfm config validate")
			return 2
		}
		loaded, err := pfmconfig.Load(runtime.Config.Path, runtime.Paths.Home, runtime.Paths.ClaudeRoots)
		if err != nil {
			fmt.Fprintf(stderr, "pfm config validate: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "config valid: %s\n", loaded.Path)
		return 0
	default:
		fmt.Fprintln(stderr, "usage: pfm config init [--force] | show | validate")
		return 2
	}
}

func runConfigInit(args []string, stdout, stderr io.Writer, runtime commandRuntime) int {
	flags := newFlagSet("config init", "usage: pfm config init [--force]", stderr)
	force := flags.Bool("force", false, "overwrite an existing config")
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 {
		flags.Usage()
		return 2
	}
	if err := pfmconfig.WriteDefault(runtime.Config.Path, runtime.Paths.Home, runtime.Paths.ClaudeRoots, *force); err != nil {
		fmt.Fprintf(stderr, "pfm config init: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "config initialized: %s\n", runtime.Config.Path)
	fmt.Fprintln(stdout, "field documentation:")
	fmt.Fprintln(stdout, "  accounts: configured Claude account roster, configDir, and emoji badge")
	fmt.Fprintln(stdout, "  claude.permissionMode: bypass or prompted; account values override this default")
	fmt.Fprintln(stdout, "  codex.yolo: whether Codex launches with approval bypass; account values override this default")
	fmt.Fprintln(stdout, "  theme: embedded palette name (default or tokyo-night)")
	fmt.Fprintln(stdout, "  mcp: enabled servers and loopback HTTP port")
	fmt.Fprintln(stdout, "  ask: default one-shot engine, model, and effort")
	return 0
}

func printResolvedConfig(stdout io.Writer, runtime commandRuntime) {
	config := runtime.Config
	fmt.Fprintf(stdout, "config path=%s exists=%t\n", config.Path, config.Exists)
	fmt.Fprintf(stdout, "config version=%d (%s)\n", config.Version, config.Source("version"))
	fmt.Fprintf(stdout, "config theme=%s (%s)\n", config.Theme, config.Source("theme"))
	accounts := make([]string, 0, len(config.Accounts))
	for index, account := range config.Accounts {
		accounts = append(accounts, fmt.Sprintf("%d:%s:%s", account.ID, account.ConfigDir, config.EmojiFor(account.ID)))
		if config.Source(fmt.Sprintf("accounts[%d].emoji", index)) == pfmconfig.SourceFile {
			accounts[len(accounts)-1] += " (file)"
		} else {
			accounts[len(accounts)-1] += " (default)"
		}
	}
	fmt.Fprintf(stdout, "config accounts=%s (%s)\n", strings.Join(accounts, ","), config.Source("accounts"))
	fmt.Fprintf(stdout, "config claude.permissionMode=%s (%s)\n", config.Claude.PermissionMode, config.Source("claude.permissionMode"))
	fmt.Fprintf(stdout, "config claude.binary=%s (%s)\n", config.Claude.Binary, config.Source("claude.binary"))
	fmt.Fprintf(stdout, "config codex.yolo=%t (%s)\n", config.Codex.Yolo, config.Source("codex.yolo"))
	fmt.Fprintf(stdout, "config codex.binary=%s (%s)\n", config.Codex.Binary, config.Source("codex.binary"))
	fmt.Fprintf(stdout, "config mcp.http.port=%d (%s)\n", config.MCP.HTTP.Port, config.Source("mcp.http.port"))
	for _, name := range pfmconfig.RegisteredMCPServers() {
		key := "mcp.servers." + name + ".enabled"
		fmt.Fprintf(stdout, "config %s=%t (%s)\n", key, config.MCPServers[name].Enabled, config.Source(key))
	}
	if config.MCP.AuthToken == "" {
		fmt.Fprintf(stdout, "config mcp.authToken=<redacted> (%s)\n", config.Source("mcp.authToken"))
	} else {
		fmt.Fprintf(stdout, "config mcp.authToken=<redacted> (%s)\n", config.Source("mcp.authToken"))
	}
	fmt.Fprintf(stdout, "config ask.engine=%s (%s)\n", config.Ask.Engine, config.Source("ask.engine"))
	fmt.Fprintf(stdout, "config ask.codex.model=%s (%s)\n", config.Ask.Codex.Model, config.Source("ask.codex.model"))
	fmt.Fprintf(stdout, "config ask.codex.effort=%s (%s)\n", config.Ask.Codex.Effort, config.Source("ask.codex.effort"))
	fmt.Fprintf(stdout, "config ask.claude.model=%s (%s)\n", config.Ask.Claude.Model, config.Source("ask.claude.model"))
	fmt.Fprintf(stdout, "config ask.claude.effort=%s (%s)\n", config.Ask.Claude.Effort, config.Source("ask.claude.effort"))
}
