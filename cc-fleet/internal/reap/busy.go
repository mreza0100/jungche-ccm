package reap

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"hostops/cc-fleet/internal/paths"
)

// BusyProbe reports the sessions the engine itself considers busy.
//
// It returns an ERROR rather than an empty set when it cannot answer: a
// deliberately detached chat grinding a long task is indistinguishable from an
// idle orphan without this, so an unanswerable probe has to fail closed rather
// than read as "nobody is working."
type BusyProbe interface {
	BusySessions(ctx context.Context) (map[string]struct{}, error)
}

// ClaudeAgents asks each installed account's `claude agents --json` which
// sessions are busy.
type ClaudeAgents struct {
	Binary     string
	ConfigDirs []string
	Timeout    time.Duration
}

// NewClaudeAgents wires the probe to the accounts on this machine.
func NewClaudeAgents(resolved paths.Values) ClaudeAgents {
	binary, err := exec.LookPath("claude")
	if err != nil {
		binary = filepath.Join(resolved.Home, ".local", "bin", "claude")
	}
	directories := make([]string, 0, len(resolved.ClaudeRoots))
	for _, root := range resolved.ClaudeRoots {
		directory := filepath.Dir(root)
		if info, err := os.Stat(directory); err == nil && info.IsDir() {
			directories = append(directories, directory)
		}
	}
	return ClaudeAgents{
		Binary:     binary,
		ConfigDirs: directories,
		Timeout:    20 * time.Second,
	}
}

type agentRow struct {
	SessionID string `json:"sessionId"`
	Status    string `json:"status"`
}

// BusySessions queries every account. Each account is asked separately so one
// account's error output cannot poison another's parse, and ANY failure fails
// the whole probe: a partial busy set is a busy set with holes in it, and the
// holes are exactly the chats a sweep would kill.
func (agents ClaudeAgents) BusySessions(
	ctx context.Context,
) (map[string]struct{}, error) {
	if len(agents.ConfigDirs) == 0 {
		return nil, fmt.Errorf("no Claude account directories to query")
	}
	busy := make(map[string]struct{})
	for _, directory := range agents.ConfigDirs {
		queryCtx, cancel := context.WithTimeout(ctx, agents.timeout())
		command := exec.CommandContext(queryCtx, agents.Binary, "agents", "--json")
		command.Env = append(os.Environ(), "CLAUDE_CONFIG_DIR="+directory)
		output, err := command.Output()
		cancel()
		if err != nil {
			return nil, fmt.Errorf(
				"query busy agents for %s: %w",
				directory,
				err,
			)
		}
		var rows []agentRow
		if err := json.Unmarshal(output, &rows); err != nil {
			return nil, fmt.Errorf(
				"parse busy agents for %s: %w",
				directory,
				err,
			)
		}
		for _, row := range rows {
			if row.Status == "busy" && row.SessionID != "" {
				busy[row.SessionID] = struct{}{}
			}
		}
	}
	return busy, nil
}

func (agents ClaudeAgents) timeout() time.Duration {
	if agents.Timeout <= 0 {
		return 20 * time.Second
	}
	return agents.Timeout
}
