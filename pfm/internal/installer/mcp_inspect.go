package installer

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	pfmengine "hostops/pfm/internal/engine"
)

const (
	MCPClientAbsent              = "absent"
	MCPClientPFM                 = "pfm"
	MCPClientLegacyStandalone    = "legacy-standalone"
	MCPClientForeignRegistration = "foreign-registration"
	MCPClientUnreadable          = "unreadable"
)

// MCPClientCutover is one consumer's visible Harvester route. Unreadable is a
// first-class state so doctor cannot mistake a failed inspection for cutover.
type MCPClientCutover struct {
	Client string
	State  string
	Error  error
}

type mcpClientRegistration struct {
	Type    string            `json:"type" toml:"type"`
	Command string            `json:"command" toml:"command"`
	Args    []string          `json:"args" toml:"args"`
	URL     string            `json:"url" toml:"url"`
	Headers map[string]string `json:"headers" toml:"headers"`
	Env     map[string]string `json:"env" toml:"env"`
}

// InspectHarvesterClientCutover inspects both supported client config files.
// It never mutates a foreign registration; doctor turns non-PFM states into an
// actionable warning for the operator completing the standalone migration.
func InspectHarvesterClientCutover(home string, port int) []MCPClientCutover {
	return []MCPClientCutover{
		inspectClaudeHarvester(filepath.Join(home, ".mcp.json"), port),
		inspectCodexHarvester(filepath.Join(home, ".codex", "config.toml"), port),
	}
}

func inspectClaudeHarvester(path string, port int) MCPClientCutover {
	report := MCPClientCutover{Client: pfmengine.MustLookup(pfmengine.Claude).LongName, State: MCPClientAbsent}
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return report
	}
	if err != nil {
		report.State, report.Error = MCPClientUnreadable, fmt.Errorf("read %s: %w", path, err)
		return report
	}
	var document struct {
		Servers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		report.State, report.Error = MCPClientUnreadable, fmt.Errorf("parse %s: %w", path, err)
		return report
	}
	encoded, present := document.Servers["harvester"]
	if !present {
		return report
	}
	var registration mcpClientRegistration
	if err := json.Unmarshal(encoded, &registration); err != nil {
		report.State, report.Error = MCPClientUnreadable, fmt.Errorf("parse %s harvester registration: %w", path, err)
		return report
	}
	report.State = classifyHarvesterRegistration(registration, port)
	return report
}

func inspectCodexHarvester(path string, port int) MCPClientCutover {
	report := MCPClientCutover{Client: pfmengine.MustLookup(pfmengine.Codex).LongName, State: MCPClientAbsent}
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return report
	}
	if err != nil {
		report.State, report.Error = MCPClientUnreadable, fmt.Errorf("read %s: %w", path, err)
		return report
	}
	var document struct {
		Servers map[string]mcpClientRegistration `toml:"mcp_servers"`
	}
	if _, err := toml.Decode(string(raw), &document); err != nil {
		report.State, report.Error = MCPClientUnreadable, fmt.Errorf("parse %s: %w", path, err)
		return report
	}
	registration, present := document.Servers["harvester"]
	if !present {
		return report
	}
	report.State = classifyHarvesterRegistration(registration, port)
	return report
}

func classifyHarvesterRegistration(registration mcpClientRegistration, port int) string {
	wantedURL := fmt.Sprintf("http://127.0.0.1:%d/mcp/harvester", port)
	typeName := strings.ToLower(strings.TrimSpace(registration.Type))
	command := strings.TrimSpace(registration.Command)
	if command != "" {
		command = filepath.Base(command)
	}
	command = strings.ToLower(command)
	noExtras := len(registration.Headers) == 0 && len(registration.Env) == 0
	if registration.URL == wantedURL && (typeName == "" || typeName == "http") && command == "" && len(registration.Args) == 0 && noExtras {
		return MCPClientPFM
	}
	if command == "pfm" && registration.URL == "" && noExtras && containsArgumentSequence(registration.Args, "mcp", "harvester", "serve") {
		return MCPClientPFM
	}
	joined := strings.ToLower(strings.Join(registration.Args, " "))
	if strings.Contains(command, "harvest") || strings.Contains(joined, "harvest") || command == "uv" {
		return MCPClientLegacyStandalone
	}
	return MCPClientForeignRegistration
}

func containsArgumentSequence(args []string, sequence ...string) bool {
	if len(sequence) == 0 || len(args) < len(sequence) {
		return false
	}
	for start := 0; start <= len(args)-len(sequence); start++ {
		matched := true
		for offset, wanted := range sequence {
			if !strings.EqualFold(strings.TrimSpace(args[start+offset]), wanted) {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}
