package codexgen

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

func tomlEscape(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	value = strings.ReplaceAll(value, "\r", `\r`)
	return strings.ReplaceAll(value, "\n", `\n`)
}

func tomlMultiline(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"""`, `\"\"\"`)
	if !strings.HasSuffix(value, "\n") {
		value += "\n"
	}
	return value
}

func tomlKey(key string) string {
	if regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(key) {
		return key
	}
	return `"` + tomlEscape(key) + `"`
}

func tomlString(value string) string { return `"` + tomlEscape(value) + `"` }

// validateTOML uses a complete pure-Go parser. Check mode validates every TOML
// artifact Codex can load, including hand-written keeper files; a narrow check
// for only the syntax emitted here would confuse "could not look" with clean.
func validateTOML(data string) error {
	var document map[string]any
	_, err := toml.Decode(data, &document)
	return err
}

type mcpServer struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
	URL     string            `json:"url"`
	Type    string            `json:"type"`
}

func decodeMCP(data []byte) (map[string]mcpServer, map[string][]string, error) {
	var raw struct {
		Servers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, nil, fmt.Errorf("parse .mcp.json: %w", err)
	}
	servers := make(map[string]mcpServer, len(raw.Servers))
	unmapped := make(map[string][]string)
	for name, body := range raw.Servers {
		var server mcpServer
		if err := json.Unmarshal(body, &server); err != nil {
			return nil, nil, fmt.Errorf("parse .mcp.json server %q: %w", name, err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(body, &fields); err != nil {
			return nil, nil, fmt.Errorf("parse .mcp.json server %q fields: %w", name, err)
		}
		for key := range fields {
			switch key {
			case "command", "args", "env", "url", "type":
			default:
				unmapped[name] = append(unmapped[name], key)
			}
		}
		sort.Strings(unmapped[name])
		servers[name] = server
	}
	return servers, unmapped, nil
}
