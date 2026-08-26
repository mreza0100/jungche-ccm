package codexgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGlobalAgentsCompilesInstallsAndAppliesSpawnAgentSubstitution covers the
// two-file happy path: both .md sources compile to a sibling .toml, both get
// installed into {home}/.claude/agents (raw source) and {home}/.codex/agents
// (compiled TOML), and the one Codex-specific body substitution — "Codex has
// no Agent tool" — fires exactly where the host script's docstring says it
// does.
func TestGlobalAgentsCompilesInstallsAndAppliesSpawnAgentSubstitution(t *testing.T) {
	home := t.TempDir()
	writeTestFile(t, filepath.Join(home, ".professor", "agents", "alpha.md"),
		"---\nname: alpha\ndescription: Alpha role for testing.\ntools: Read\nmodel: sonnet\n---\n\n"+
			"Delegate to children are Explore+haiku (never\nyour own type) for search fan-out.\n")
	writeTestFile(t, filepath.Join(home, ".professor", "agents", "beta.md"),
		"---\nname: beta\ndescription: Beta role for testing.\ntools: Read\nmodel: haiku\n---\n\nBeta body, unrelated.\n")

	result, err := RunGlobalAgents(GlobalAgentsOptions{Home: home})
	if err != nil {
		t.Fatalf("RunGlobalAgents: %v", err)
	}
	if len(result.Compiled) != 2 {
		t.Fatalf("compiled = %#v, want 2 entries", result.Compiled)
	}
	if len(result.Installed) != 4 {
		t.Fatalf("installed = %#v, want 4 entries (2 md + 2 toml)", result.Installed)
	}
	for _, installed := range result.Installed {
		if !installed.RegularFile {
			t.Fatalf("installed %s is not a regular file", installed.Path)
		}
	}

	alphaTOML := string(mustReadTestFile(t, filepath.Join(home, ".professor", "agents", "alpha.toml")))
	if strings.Contains(alphaTOML, "children are Explore+haiku") {
		t.Fatalf("alpha.toml: substitution did not fire:\n%s", alphaTOML)
	}
	if !strings.Contains(alphaTOML, "spawned via spawn_agent as the `explorer` role") {
		t.Fatalf("alpha.toml: substitution target text missing:\n%s", alphaTOML)
	}

	for _, path := range []string{
		filepath.Join(home, ".claude", "agents", "alpha.md"),
		filepath.Join(home, ".claude", "agents", "beta.md"),
		filepath.Join(home, ".codex", "agents", "alpha.toml"),
		filepath.Join(home, ".codex", "agents", "beta.toml"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected install at %s: %v", path, err)
		}
	}

	// The .claude install is the raw source, untouched by the Codex-only
	// substitution — Claude does have an Agent tool.
	claudeAlpha := string(mustReadTestFile(t, filepath.Join(home, ".claude", "agents", "alpha.md")))
	if !strings.Contains(claudeAlpha, "children are Explore+haiku (never\nyour own type)") {
		t.Fatalf(".claude/agents/alpha.md: raw source was mutated:\n%s", claudeAlpha)
	}
}

// TestGlobalAgentsAdversarialFixtureEmitsValidTOMLWithLiteralQuotesAndDelimiterCollision
// is the exact failure mode the host script's docstring warns about: a
// description containing a raw `"` (real examples: the trigger phrases
// "walker fast", "map it now") and a body containing a literal `"""` that
// collides with the multi-line basic-string delimiter. An emitter that drops
// either escape rule ships a TOML Codex cannot parse at startup — this test
// asserts both the exact escaped bytes AND that the result independently
// parses as TOML (via BurntSushi/toml, not our own escaping logic).
func TestGlobalAgentsAdversarialFixtureEmitsValidTOMLWithLiteralQuotesAndDelimiterCollision(t *testing.T) {
	home := t.TempDir()
	writeTestFile(t, filepath.Join(home, ".professor", "agents", "quirky.md"),
		"---\nname: quirky\ndescription: Uses \"walker fast\" and \"map it now\" verbatim.\ntools: Read\nmodel: sonnet\n---\n\n"+
			"Body has a literal triple quote \"\"\" and a backslash \\ standalone.\n")

	result, err := RunGlobalAgents(GlobalAgentsOptions{Home: home})
	if err != nil {
		t.Fatalf("RunGlobalAgents: %v", err)
	}
	if len(result.Compiled) != 1 {
		t.Fatalf("compiled = %#v, want 1 entry", result.Compiled)
	}

	got := string(mustReadTestFile(t, filepath.Join(home, ".professor", "agents", "quirky.toml")))
	want := "name = \"quirky\"\n" +
		"description = \"Uses \\\"walker fast\\\" and \\\"map it now\\\" verbatim.\"\n" +
		"developer_instructions = \"\"\"\n" +
		"Body has a literal triple quote \\\"\\\"\\\" and a backslash \\\\ standalone.\n" +
		"\"\"\"\n"
	if got != want {
		t.Fatalf("quirky.toml =\n%q\nwant\n%q", got, want)
	}
	if err := validateTOML(got); err != nil {
		t.Fatalf("emitted TOML does not parse (the exact startup failure this escaping exists to prevent): %v\n%s", err, got)
	}
}

// TestGlobalAgentEscapeMirrorsBuildCodexMJS pins the basic-string escape to
// build-codex.mjs:151 exactly: backslash doubled, then a raw quote escaped.
// Order matters — escaping the quote first would double-escape the
// backslashes it just inserted.
func TestGlobalAgentEscapeMirrorsBuildCodexMJS(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"raw quote pair", `Uses "walker fast" and "map it now".`, `Uses \"walker fast\" and \"map it now\".`},
		{"backslash before quote", `back\slash "then" quote`, `back\\slash \"then\" quote`},
		{"plain text unchanged", `no special characters here`, `no special characters here`},
	}
	for _, c := range cases {
		if got := globalAgentEscape(c.in); got != c.want {
			t.Errorf("%s: globalAgentEscape(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

// TestGlobalAgentEscapeMultilineNeutralisesTripleQuoteCollision pins the
// multi-line basic-string escape to build-codex.mjs:153 exactly: backslash
// doubled, then a literal `"""` neutralised so it can never be mistaken for
// the string's own closing delimiter.
func TestGlobalAgentEscapeMultilineNeutralisesTripleQuoteCollision(t *testing.T) {
	in := "line one\nhas a literal \"\"\" inside\nand a backslash \\ alone"
	want := "line one\nhas a literal \\\"\\\"\\\" inside\nand a backslash \\\\ alone"
	if got := globalAgentEscapeMultiline(in); got != want {
		t.Fatalf("globalAgentEscapeMultiline(%q) = %q, want %q", in, got, want)
	}
}

func TestGlobalAgentsMissingFrontmatterFieldIsAHardError(t *testing.T) {
	home := t.TempDir()
	writeTestFile(t, filepath.Join(home, ".professor", "agents", "broken.md"),
		"---\nname: broken\n---\n\nno description field.\n")

	_, err := RunGlobalAgents(GlobalAgentsOptions{Home: home})
	if err == nil || !strings.Contains(err.Error(), "needs both name: and description:") {
		t.Fatalf("RunGlobalAgents: got %v, want a frontmatter error", err)
	}
}

func TestGlobalAgentsNoSourcesIsAHardError(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".professor", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := RunGlobalAgents(GlobalAgentsOptions{Home: home})
	if err == nil || !strings.Contains(err.Error(), "no agent .md files in") {
		t.Fatalf("RunGlobalAgents: got %v, want a no-sources error", err)
	}
}

func TestGlobalAgentsCheckReportsActualInstalledFileShape(t *testing.T) {
	home := t.TempDir()
	writeTestFile(t, filepath.Join(home, ".professor", "agents", "alpha.md"),
		"---\nname: alpha\ndescription: Alpha role for testing.\n---\n\nbody\n")
	result, err := RunGlobalAgents(GlobalAgentsOptions{Home: home, Mode: ModeCheck})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Installed) != 2 {
		t.Fatalf("installed rows=%#v, want two desired targets", result.Installed)
	}
	for _, installed := range result.Installed {
		if installed.RegularFile {
			t.Fatalf("check mode fabricated absent target as a regular file: %#v", installed)
		}
	}
}

// TestGlobalAgentsInstallReplacesASymlinkWithARegularFile matches the host
// script's own documented behavior: a symlink also loads, but install always
// leaves a real file behind so the registry holds no dependency on the
// source directory's path.
func TestGlobalAgentsInstallReplacesASymlinkWithARegularFile(t *testing.T) {
	home := t.TempDir()
	writeTestFile(t, filepath.Join(home, ".professor", "agents", "alpha.md"),
		"---\nname: alpha\ndescription: Alpha role for testing.\n---\n\nbody\n")

	claudeDest := filepath.Join(home, ".claude", "agents")
	if err := os.MkdirAll(claudeDest, 0o755); err != nil {
		t.Fatal(err)
	}
	elsewhere := filepath.Join(home, "elsewhere.md")
	writeTestFile(t, elsewhere, "stale symlink target\n")
	if err := os.Symlink(elsewhere, filepath.Join(claudeDest, "alpha.md")); err != nil {
		t.Fatal(err)
	}

	if _, err := RunGlobalAgents(GlobalAgentsOptions{Home: home}); err != nil {
		t.Fatalf("RunGlobalAgents: %v", err)
	}

	info, err := os.Lstat(filepath.Join(claudeDest, "alpha.md"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("alpha.md is still a symlink after install")
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("alpha.md is not a regular file after install: mode=%v", info.Mode())
	}
}
