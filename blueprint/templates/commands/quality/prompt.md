---
name: quality:prompt
description: Use BEFORE editing any prompt file — CLAUDE.md, .claude/agents/*.md, .claude/commands/*.md, .claude/skills/*/SKILL.md, child CLAUDE.md, or /km knowledge files under {AI_PROJECT}/knowledge/. Enforces Anthropic's prompt-quality rules — cut test, ≤200-line CLAUDE.md, ≤500-line skills, positive framing, no time-sensitive narration, one canonical term, frontmatter discipline. Mandatory load for /pcm and /km.
---

# Prompt Quality

You are about to edit a prompt file that Claude Code loads at runtime (or, for `/km` knowledge files, that the {AI_SERVICE_NAME} LLM loads). Every line is paid for on every invocation. Apply the rules below at write-time.

**When to load:** `/pcm` loads this before editing any infrastructure prompt file; `/km` loads this before editing knowledge files. Also load it yourself before hand-editing any CLAUDE.md, agent, command, or skill.

## The cut test (apply to every line)

> Would removing this line cause Claude to make a mistake?

If no — delete it. Bloat dilutes the rules that matter; the model "may start forgetting earlier instructions or making more mistakes" as the file grows.

## The prompt stream — audit in context, not in isolation

A prompt rarely loads alone. In the Claude Code harness the LLM reads one concatenated context: root `CLAUDE.md`, the auto-loaded skill descriptions, the active command or agent, and every skill loaded this session — all at once. Audit a prompt against that whole stream, not just the file in front of you: a rule may already live in a co-loaded file (duplication), contradict one (conflict), or push the combined context past what the model holds well (budget). Follow the stream the target LLM actually reads, end to end, before judging any single file.

## Hard thresholds (Anthropic-published)

| File type                       | Limit                                                     | Source                           |
| ------------------------------- | --------------------------------------------------------- | -------------------------------- |
| CLAUDE.md (any)                 | ≤ 200 lines                                               | docs/claude-code/memory          |
| SKILL.md body                   | ≤ 500 lines — split via progressive disclosure above this | docs/agent-skills/best-practices |
| Skill description + when_to_use | ≤ 1,536 chars combined                                    | docs/claude-code/skills          |
| Sub-agent body                  | No formal cap; Anthropic examples are 20–35 lines         | docs/claude-code/sub-agents      |

Above threshold = split into a referenced file (one level deep, with a Table of Contents at the top if >100 lines).

## Anti-patterns — cut on sight

- **Time-sensitive narration.** "On 2026-05-19...", "after the X incident", "before August 2025". Encode the rule that resulted; the incident goes in the commit message or the relevant epic manifest, not the prompt.
- **Dates of change.** Changelog-style "changed 2026-06-07" lines or update-history dates inside a prompt or `.professor/` ledger are the same antipattern — version control already timestamps every change. State the current rule, never when it changed.
- **Restating one rule multiple ways.** Two phrasings cause Claude to "pick one arbitrarily." Pick the sharper version, delete the rest.
- **Frontmatter ↔ body duplication.** If `description:` says it, the body opening must not.
- **Voice flavor that doesn't change behavior.** Backstory, character arcs, "I built this", "the meta layer". Root CLAUDE.md owns voice; agents/skills/commands inherit it.
- **"Why this exists:" / "Why:" paragraphs that just rephrase the rule.** The rule's purpose lives in the rule's wording.
- **Negative framing where positive works.** "Use prose paragraphs" beats "don't use bullets." Reserve do NOT / NEVER for sacred ground ({SENSITIVE_DATA}, {DOMAIN_ADJ} safety, secrets).
- **Aggressive emphasis ("CRITICAL", "YOU MUST", "MANDATORY") on non-sacred rules.** {MODEL_TIER} overtriggers on it. Plain language for ordinary rules; reserve emphasis for invariants.
- **Inconsistent terminology** — mixing "endpoint / URL / route", "field / box / element", "extract / pull / get". One canonical term per concept, used everywhere.
- **Cross-references that say nothing new** ("See § X above" two paragraphs up). If the reference matters, summarize the takeaway inline.
- **Inline cross-file restatement** — child CLAUDE.md files restating workspace rules already in root CLAUDE.md. Child files keep ONLY the project-specific delta.
- **Multiple options when one default suffices.** "Use pypdf, pdfplumber, PyMuPDF, or pdf2image" → "Use pdfplumber. For OCR, use pdf2image+pytesseract."
- **Examples that don't pin down behavior.** An example earns its tokens only if the rule alone wouldn't produce the same output.
- **Vague descriptions.** "Helps with documents", "Processes data", "Does stuff with files" → no auto-invocation.
- **Deeply nested file references** (SKILL.md → reference.md → details.md). Claude `head -100`s and misses content. Keep references one level deep.
- **Inline incident logs in references / gotchas.** Once the rule is codified, the incident becomes redundant. Move it to the commit message or epic manifest.

## Structural conventions by file type

### Sub-agents (`.claude/agents/*.md`)

```
---
name: kebab-case-id
description: One sentence. Includes "when to delegate" phrase.
tools: <minimal allowlist>
model: inherit | opus | sonnet | haiku
---
You are a {role}. {one-sentence scope}.

When invoked:
1. {step}
2. {step}
3. {step}

{Checklist or rubric — bulleted, short.}
{Output format — usually one paragraph or a tiny template.}
```

Frontmatter `description` is the routing weight — Claude reads it to decide auto-delegation. Use phrases like "Use proactively after X". Body is literally the system prompt; subagents see only their own prompt + env.

### Slash commands (`.claude/commands/*.md`)

```
---
name: cmd-name
description: One sentence. Action verb first.
argument-hint: [arg1] [arg2]
disable-model-invocation: true  # if has side effects
---
{Numbered procedure — or markdown body if non-procedural}
```

`$ARGUMENTS` / `$1` / `$N` substitute at invocation. Prefixing a backticked command with a bang (!\`cmd\`) injects live shell output before Claude sees the prompt.

### Skills (`.claude/skills/*/SKILL.md`)

```
---
name: lowercase-hyphenated  # ≤64 chars, no reserved words (anthropic, claude)
description: What it does AND when to use it. Highest-signal use case first. Third person. ≤1,024 chars; combined with when_to_use ≤1,536.
---
{One-line role / scope}
{Trigger conditions or "When to load"}
{Steps or rules — keep behavioral, no manifesto}
{Examples in <example> tags, 3-5 of them, relevant + diverse}
{Constraints — only the ones that aren't obvious from CLAUDE.md}
```

Skill content stays in context for the rest of the session after invocation and re-attaches after compaction. Every line is a recurring tax.

### CLAUDE.md (root + child)

Keep: bash commands Claude can't guess, code-style rules that differ from defaults, architectural decisions / invariants, non-obvious gotchas, repo etiquette / test runners.

NOT: standard language conventions, file-by-file descriptions, "write clean code" platitudes, info Claude can read from the code. Child CLAUDE.md files keep only the project-specific delta — never re-declare workspace rules already in root.

## Examples

<example name="well-shaped-subagent" source="anthropic-docs">

```
---
name: code-reviewer
description: Expert code review specialist. Proactively reviews code for quality, security, and maintainability. Use immediately after writing or modifying code.
tools: Read, Grep, Glob, Bash
model: inherit
---
You are a senior code reviewer ensuring high standards of code quality and security.

When invoked:
1. Run git diff to see recent changes
2. Focus on modified files
3. Begin review immediately

Review checklist:
- Code is clear and readable
- Proper error handling
- No exposed secrets or API keys
- Good test coverage

Provide feedback organized by priority:
- Critical issues (must fix)
- Warnings (should fix)
- Suggestions (consider improving)
```

26 lines. Role = one sentence. Procedure = 3 numbered steps. Checklist = 4 bullets. No backstory.

</example>

<example name="well-shaped-claude-md" source="anthropic-docs">

```
# Code style
- Use ES modules (import/export), not CommonJS
- Destructure imports when possible

# Workflow
- Typecheck when done with a series of changes
- Prefer single-test runs over the whole suite for speed
```

7 lines. Specific. Behavioral. Each line passes the cut test.

</example>

<example name="how-to-encode-an-incident-rule">

Wrong (in the prompt):

> The seed script once published the analysis request before registering the result waiter, so the seed hung to its full timeout. Never publish before registering again.

Right (in the prompt):

> Register the result waiter before publishing the analysis request.

The incident narration moves to the commit message / epic manifest. The rule stays sharp.

</example>

## Pre-commit self-check (run before saving any prompt file)

1. **Cut test:** Did I delete every line that wouldn't change Claude's behavior?
2. **Threshold:** Is the file under its limit (CLAUDE.md ≤200, SKILL.md ≤500)?
3. **Frontmatter discipline:** Does the body re-state what's already in `description:`? Cut.
4. **One canonical term:** Did I sweep for synonym mixing?
5. **Positive framing:** Is every "do NOT" a sacred-ground rule? If not, rewrite as a positive instruction.
6. **No time-stamps:** No "2026-XX-XX", no "after the X incident", no "we used to". Encode rules, not history.
7. **Aggressive emphasis only on invariants:** "MUST" / "NEVER" / "MANDATORY" earn their place only on sacred ground.
8. **References one level deep:** No SKILL.md → ref.md → ref2.md chains.
9. **Cross-file deduplication:** Is this rule already declared in a parent file (root CLAUDE.md, root agent)? If yes, delete here; keep only the local delta.
10. **Colleague test:** Could a colleague with no context follow this? If they'd be confused, Claude will be too.

## Where things go (anti-bloat routing)

| Content                                             | Belongs in                                            | NOT in                                            |
| --------------------------------------------------- | ----------------------------------------------------- | ------------------------------------------------- |
| Behavioral rules                                    | Prompt files (CLAUDE.md, agents, commands, skills)    | —                                                 |
| Incident narratives ("on 2026-XX-XX...")            | Commit message / epic manifest (`docs/epics/{name}/`) | Prompt files                                      |
| Architectural decisions / why-this-design           | Epic manifest or `docs/commands/{cmd}/references/`    | Prompt files (encode the rule, not the rationale) |
| Voice / character flavor                            | Root CLAUDE.md only                                   | Agents, commands, skills (they inherit)           |
| Project-specific tooling                            | Child CLAUDE.md only                                  | Per-project agents (already inherit via parent)   |
| Cross-cutting templates (report format, plan shape) | One canonical reference file                          | Duplicated per-project                            |

## Hooks vs prompts

For things that must happen every time (formatting, validation, secret-scanning), write a hook (`.claude/settings.json` PreToolUse / PostToolUse) — deterministic, cheap. Prompts are advisory; the model can drift. Don't try to enforce critical invariants by repeating "ALWAYS DO X" in the prompt.

## Iteration discipline

When a rule fails in practice:

1. Observe Claude's actual output (not how the prompt reads).
2. Diagnose: is the rule ambiguous, contradicted by another rule, or buried in noise?
3. Fix surgically — sharpen, move, scope. Adding more emphasis is rarely the answer.

If the failure is recurring and structural, consider a hook instead.

## /km knowledge files are prompts too

`/km` writes domain knowledge files under `{AI_PROJECT}/knowledge/` that get injected verbatim into the {AI_SERVICE_NAME} LLM context. Every rule above applies: cut test, cue density, one canonical term, no narration. `/km` carries additional domain-specific rules ({LLM_PROVIDER} bias control, schema fidelity, {REGULATION} compliance) — load both: quality:prompt for the structural discipline, `/km`'s Sacred Ground for the domain layer.
