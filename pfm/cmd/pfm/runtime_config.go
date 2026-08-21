package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	pfmconfig "hostops/pfm/internal/config"
	"hostops/pfm/internal/paths"
)

// commandRuntime is loaded exactly once by run and then passed to the command
// branches that consume machine policy. It is immutable for that invocation.
type commandRuntime struct {
	Config      pfmconfig.Config
	Paths       paths.Values
	ConfigError error
}

func optionalCommandRuntime(runtimes []commandRuntime) (commandRuntime, error) {
	if len(runtimes) != 0 {
		return runtimes[0], nil
	}
	return loadCommandRuntime("")
}

func loadCommandRuntime(configPath string) (commandRuntime, error) {
	resolved, err := paths.Resolve()
	if err != nil {
		return commandRuntime{}, fmt.Errorf("resolve paths: %w", err)
	}
	effective, err := pfmconfig.Load(
		configPath,
		resolved.Home,
		resolved.ClaudeRoots,
		resolved.CodexRoot,
	)
	if err != nil {
		return commandRuntime{}, err
	}
	resolved.ClaudeRoots = effective.ProjectRoots()
	return commandRuntime{Config: effective, Paths: resolved}, nil
}

func loadDiagnosticRuntime(configPath string) (commandRuntime, error) {
	resolved, err := paths.Resolve()
	if err != nil {
		return commandRuntime{}, fmt.Errorf("resolve paths: %w", err)
	}
	effective, configErr := pfmconfig.Load(configPath, resolved.Home, resolved.ClaudeRoots, resolved.CodexRoot)
	if configErr == nil {
		resolved.ClaudeRoots = effective.ProjectRoots()
		return commandRuntime{Config: effective, Paths: resolved}, nil
	}
	// Diagnostics must remain usable on a broken config. They operate on
	// defaults, but carry the original error to the visible command surface.
	path := configPath
	if path == "" {
		path = pfmconfig.ResolvePath(resolved.Home)
	}
	effective = pfmconfig.Defaults(resolved.Home, resolved.ClaudeRoots, resolved.CodexRoot)
	effective.Path = path
	effective.Exists = true
	resolved.ClaudeRoots = effective.ProjectRoots()
	return commandRuntime{Config: effective, Paths: resolved, ConfigError: configErr}, nil
}

// splitGlobalConfig accepts the global flag only before the command. This is
// deliberate: internal subcommands already own flags named --config, and a
// global parser must never steal their arguments.
func splitGlobalConfig(args []string) (string, []string, error) {
	path := ""
	for len(args) > 0 {
		switch {
		case args[0] == "--config":
			if path != "" {
				return "", nil, errors.New("--config may be specified only once")
			}
			if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
				return "", nil, errors.New("--config requires a path")
			}
			path = args[1]
			args = args[2:]
		case strings.HasPrefix(args[0], "--config="):
			if path != "" {
				return "", nil, errors.New("--config may be specified only once")
			}
			path = strings.TrimPrefix(args[0], "--config=")
			if strings.TrimSpace(path) == "" {
				return "", nil, errors.New("--config requires a path")
			}
			args = args[1:]
		default:
			if path != "" && !filepath.IsAbs(path) {
				absolute, err := filepath.Abs(path)
				if err != nil {
					return "", nil, fmt.Errorf("resolve --config path %q: %w", path, err)
				}
				path = absolute
			}
			return path, args, nil
		}
	}
	return path, args, nil
}
