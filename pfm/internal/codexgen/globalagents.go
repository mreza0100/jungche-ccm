package codexgen

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// GlobalAgentsOptions selects the host HOME whose global Codex agents get
// (re)compiled and installed. The source directory is always
// {Home}/.professor/agents; installs land at {Home}/.claude/agents (the raw
// .md, Claude reads it directly) and {Home}/.codex/agents (the compiled
// .toml, Codex reads it directly).
type GlobalAgentsOptions struct {
	Home string
	Mode Mode
}

// GlobalAgentCompiled is one desired TOML beside its source .md. Build writes
// changed bytes; check reports the same desired artifact without writing it.
type GlobalAgentCompiled struct {
	Path string
	Size int64
}

// GlobalAgentInstalled is one desired file in a live registry directory.
// Build leaves it as a regular file; check reports that same final shape.
type GlobalAgentInstalled struct {
	Path        string
	RegularFile bool
}

// GlobalAgentsResult reports every desired artifact in the same two phases
// the host script performed. Actions narrows that roster to paths build would
// actually replace.
type GlobalAgentsResult struct {
	Compiled  []GlobalAgentCompiled
	Installed []GlobalAgentInstalled
	Actions   []GlobalAgentAction
}

// GlobalAgentAction is one exact path a build would replace. Check mode emits
// the same action list without touching disk, which lets the host installer
// present its complete plan before any earlier install mutation can land.
type GlobalAgentAction struct {
	Kind string
	Path string
}

type globalAgentOutput struct {
	path      string
	content   []byte
	compiled  bool
	installed bool
}

// Codex has no Agent tool — the lead spawns children through spawn_agent
// instead. This is the one Codex-specific substitution the host script
// applies to every agent body; it is a literal, not a general transform.
const (
	globalAgentBodyOld = "children are Explore+haiku (never\nyour own type)"
	globalAgentBodyNew = "children are spawned via spawn_agent as the `explorer` role (never your own type)"
)

// globalAgentFrontmatter requires the entire file to be exactly
// "---\n<frontmatter>\n---\n<body>" with nothing before it and the body
// running to end of file — the same shape build-global-agents.py required
// (\A...\Z, DOTALL). A file that doesn't fit this shape (extra leading
// content, no closing "---" line) is not a global agent source.
var (
	globalAgentFrontmatter = regexp.MustCompile(`(?s)\A---\n(.*?)\n---\n(.*)\z`)
	globalAgentNameField   = regexp.MustCompile(`(?m)^name:\s*(.+)$`)
	globalAgentDescField   = regexp.MustCompile(`(?m)^description:\s*(.+)$`)
)

// RunGlobalAgents is the Go port of the retired host script
// ~/.professor/agents/build-global-agents.py: it compiles every
// {Home}/.professor/agents/*.md into a sibling TOML, validates every
// compiled TOML parses, then installs the .md sources into
// {Home}/.claude/agents and the compiled .toml files into
// {Home}/.codex/agents.
//
// TOML escaping mirrors build-codex.mjs:151-153 exactly — see
// globalAgentEscape / globalAgentEscapeMultiline — because a raw `"` in an
// agent description (real examples: the trigger phrases "walker fast",
// "map it now") breaks the TOML parser at Codex startup and takes the whole
// role down with it.
//
// A failed compile or an unparseable TOML aborts before any install, the
// same fail-loud order the host script used: an emitter that ships an
// unparseable artifact has done nothing useful.
func RunGlobalAgents(options GlobalAgentsOptions) (GlobalAgentsResult, error) {
	home, err := resolveHome(options.Home)
	if err != nil {
		return GlobalAgentsResult{}, err
	}
	agentsDir := filepath.Join(home, ".professor", "agents")

	sources, err := globSorted(filepath.Join(agentsDir, "*.md"))
	if err != nil {
		return GlobalAgentsResult{}, fmt.Errorf("glob %s: %w", agentsDir, err)
	}
	if len(sources) == 0 {
		return GlobalAgentsResult{}, fmt.Errorf("no agent .md files in %s", agentsDir)
	}

	outputs := make([]globalAgentOutput, 0, len(sources)*3)
	for _, src := range sources {
		out, content, err := renderGlobalAgentTOML(src)
		if err != nil {
			return GlobalAgentsResult{}, err
		}
		if parseErr := validateTOML(content); parseErr != nil {
			return GlobalAgentsResult{}, fmt.Errorf("%s: does not parse: %w", out, parseErr)
		}
		raw, err := os.ReadFile(src)
		if err != nil {
			return GlobalAgentsResult{}, fmt.Errorf("read %s: %w", src, err)
		}
		outputs = append(outputs,
			globalAgentOutput{path: out, content: []byte(content), compiled: true},
			globalAgentOutput{path: filepath.Join(home, ".claude", "agents", filepath.Base(src)), content: raw, installed: true},
			globalAgentOutput{path: filepath.Join(home, ".codex", "agents", filepath.Base(out)), content: []byte(content), installed: true},
		)
	}

	result := GlobalAgentsResult{}
	for _, output := range outputs {
		if output.compiled {
			result.Compiled = append(result.Compiled, GlobalAgentCompiled{Path: output.path, Size: int64(len(output.content))})
		}
		if output.installed {
			result.Installed = append(result.Installed, GlobalAgentInstalled{Path: output.path, RegularFile: true})
		}
		same, err := sameGlobalAgentFile(output.path, output.content)
		if err != nil {
			return GlobalAgentsResult{}, err
		}
		if same {
			continue
		}
		result.Actions = append(result.Actions, GlobalAgentAction{Kind: "write", Path: output.path})
	}
	if options.Mode == ModeCheck {
		return result, nil
	}
	for _, output := range outputs {
		same, err := sameGlobalAgentFile(output.path, output.content)
		if err != nil {
			return GlobalAgentsResult{}, err
		}
		if same {
			continue
		}
		if err := writeGlobalAgentFile(output.path, output.content); err != nil {
			return GlobalAgentsResult{}, err
		}
	}

	return result, nil
}

func sameGlobalAgentFile(path string, content []byte) (bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect global agent artifact %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return false, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read global agent artifact %s: %w", path, err)
	}
	return string(raw) == string(content), nil
}

func writeGlobalAgentFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	info, err := os.Lstat(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("inspect global agent destination %s: %w", path, err)
	}
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("remove symlink %s: %w", path, err)
		}
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func renderGlobalAgentTOML(mdPath string) (string, string, error) {
	raw, err := os.ReadFile(mdPath)
	if err != nil {
		return "", "", fmt.Errorf("read %s: %w", mdPath, err)
	}
	match := globalAgentFrontmatter.FindStringSubmatch(string(raw))
	if match == nil {
		return "", "", fmt.Errorf("%s: no frontmatter", mdPath)
	}
	frontmatter, body := match[1], strings.TrimSpace(match[2])

	nameMatch := globalAgentNameField.FindStringSubmatch(frontmatter)
	descMatch := globalAgentDescField.FindStringSubmatch(frontmatter)
	if nameMatch == nil || descMatch == nil {
		return "", "", fmt.Errorf("%s: frontmatter needs both name: and description:", mdPath)
	}
	name := strings.TrimSpace(nameMatch[1])
	description := strings.TrimSpace(descMatch[1])

	body = strings.ReplaceAll(body, globalAgentBodyOld, globalAgentBodyNew)

	content := "name = \"" + globalAgentEscape(name) + "\"\n" +
		"description = \"" + globalAgentEscape(description) + "\"\n" +
		"developer_instructions = \"\"\"\n" + globalAgentEscapeMultiline(body) + "\n\"\"\"\n"

	out := filepath.Join(filepath.Dir(mdPath), name+".toml")
	return out, content, nil
}

// globalAgentEscape is for a TOML basic string — build-codex.mjs:151.
// Only a backslash and a raw quote need neutralising; name/description are
// always a single frontmatter line, so no literal newline ever reaches it.
func globalAgentEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, `"`, `\"`)
}

// globalAgentEscapeMultiline is for a TOML multi-line basic string —
// build-codex.mjs:153. Only a backslash and a literal triple-quote need
// neutralising; TOML tolerates a raw newline inside """ ... """.
func globalAgentEscapeMultiline(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, `"""`, `\"\"\"`)
}

func globSorted(pattern string) ([]string, error) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	return matches, nil
}
