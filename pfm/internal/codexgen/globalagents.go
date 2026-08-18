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
}

// GlobalAgentCompiled is one TOML this run wrote directly into the source
// directory, alongside its .md — the same layout the incumbent host script
// used, so the compiled artifact stays next to the source that produced it.
type GlobalAgentCompiled struct {
	Path string
	Size int64
}

// GlobalAgentInstalled is one file copied into a live registry directory.
// RegularFile is false only when the copy landed on something other than a
// plain file — the one shape Codex/Claude registries refuse to load.
type GlobalAgentInstalled struct {
	Path        string
	RegularFile bool
}

// GlobalAgentsResult reports every artifact this run touched, in the same
// two phases the host script performed: every compiled TOML, then every
// installed file (sources into ~/.claude/agents, TOMLs into
// ~/.codex/agents).
type GlobalAgentsResult struct {
	Compiled  []GlobalAgentCompiled
	Installed []GlobalAgentInstalled
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

	written := make([]string, 0, len(sources))
	for _, src := range sources {
		out, err := emitGlobalAgentTOML(src)
		if err != nil {
			return GlobalAgentsResult{}, err
		}
		written = append(written, out)
	}

	result := GlobalAgentsResult{}
	for _, out := range written {
		data, err := os.ReadFile(out)
		if err != nil {
			return GlobalAgentsResult{}, fmt.Errorf("read %s: %w", out, err)
		}
		if parseErr := validateTOML(string(data)); parseErr != nil {
			return GlobalAgentsResult{}, fmt.Errorf("%s: does not parse: %w", out, parseErr)
		}
		info, statErr := os.Stat(out)
		if statErr != nil {
			return GlobalAgentsResult{}, statErr
		}
		result.Compiled = append(result.Compiled, GlobalAgentCompiled{Path: out, Size: info.Size()})
	}

	claudeDest := filepath.Join(home, ".claude", "agents")
	for _, src := range sources {
		installed, err := installGlobalAgentFile(src, claudeDest)
		if err != nil {
			return GlobalAgentsResult{}, err
		}
		result.Installed = append(result.Installed, installed)
	}

	tomls, err := globSorted(filepath.Join(agentsDir, "*.toml"))
	if err != nil {
		return GlobalAgentsResult{}, fmt.Errorf("glob %s: %w", agentsDir, err)
	}
	codexDest := filepath.Join(home, ".codex", "agents")
	for _, src := range tomls {
		installed, err := installGlobalAgentFile(src, codexDest)
		if err != nil {
			return GlobalAgentsResult{}, err
		}
		result.Installed = append(result.Installed, installed)
	}

	return result, nil
}

// emitGlobalAgentTOML reads one Claude agent .md, and writes its compiled
// TOML twin next to it. It returns the path written.
func emitGlobalAgentTOML(mdPath string) (string, error) {
	raw, err := os.ReadFile(mdPath)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", mdPath, err)
	}
	match := globalAgentFrontmatter.FindStringSubmatch(string(raw))
	if match == nil {
		return "", fmt.Errorf("%s: no frontmatter", mdPath)
	}
	frontmatter, body := match[1], strings.TrimSpace(match[2])

	nameMatch := globalAgentNameField.FindStringSubmatch(frontmatter)
	descMatch := globalAgentDescField.FindStringSubmatch(frontmatter)
	if nameMatch == nil || descMatch == nil {
		return "", fmt.Errorf("%s: frontmatter needs both name: and description:", mdPath)
	}
	name := strings.TrimSpace(nameMatch[1])
	description := strings.TrimSpace(descMatch[1])

	body = strings.ReplaceAll(body, globalAgentBodyOld, globalAgentBodyNew)

	content := "name = \"" + globalAgentEscape(name) + "\"\n" +
		"description = \"" + globalAgentEscape(description) + "\"\n" +
		"developer_instructions = \"\"\"\n" + globalAgentEscapeMultiline(body) + "\n\"\"\"\n"

	out := filepath.Join(filepath.Dir(mdPath), name+".toml")
	if err := os.WriteFile(out, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", out, err)
	}
	return out, nil
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

// installGlobalAgentFile copies src as a real file into destDir, replacing
// any symlink that sits at the destination first. A symlink also loads —
// verified by the host script's own A/B test — but a copy is used anyway so
// the registry holds no dependency on the source directory's path.
func installGlobalAgentFile(src, destDir string) (GlobalAgentInstalled, error) {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return GlobalAgentInstalled{}, fmt.Errorf("mkdir %s: %w", destDir, err)
	}
	dest := filepath.Join(destDir, filepath.Base(src))
	if info, err := os.Lstat(dest); err == nil && info.Mode()&os.ModeSymlink != 0 {
		if err := os.Remove(dest); err != nil {
			return GlobalAgentInstalled{}, fmt.Errorf("remove symlink %s: %w", dest, err)
		}
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return GlobalAgentInstalled{}, fmt.Errorf("read %s: %w", src, err)
	}
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return GlobalAgentInstalled{}, fmt.Errorf("write %s: %w", dest, err)
	}
	info, err := os.Lstat(dest)
	regular := err == nil && info.Mode().IsRegular()
	return GlobalAgentInstalled{Path: dest, RegularFile: regular}, nil
}

func globSorted(pattern string) ([]string, error) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	return matches, nil
}
