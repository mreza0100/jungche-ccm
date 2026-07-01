---
name: quality:prompt
description: Use BEFORE editing any prompt file — CLAUDE.md, .claude/agents/*.md, .claude/commands/*.md, .claude/skills/*/SKILL.md, child CLAUDE.md, or /km knowledge files under {AI_PROJECT}/knowledge/. Enforces Anthropic's prompt-quality rules — cut test, ≤200-line CLAUDE.md, ≤500-line skills, positive framing, no time-sensitive narration, one canonical term, frontmatter discipline. Mandatory first step before any prompt-file edit, whether hand-edited or routed through /pcm or /km.
---

# Prompt Quality

You are about to edit a prompt file that Claude Code loads at runtime (or, for `/km` knowledge files, that the {AI_SERVICE_NAME} LLM loads). Every line is paid for on every invocation. Apply the rules below at write-time.

**When to load:** `/pcm` loads this before editing any infrastructure prompt file; `/km` loads this before editing knowledge files. Also load it yourself before hand-editing any CLAUDE.md, agent, command, or skill.

**Scope boundary:** serving both consumers, this skill carries runtime-agnostic prompt LAW only. The Claude-Code file-shape skeletons (agent/command/skill frontmatter + templates + "well-shaped" examples) live in `/pcm § Authoring conventions`; harness runtime mechanics (spawn economics, registries, orchestrator/spawn-brief design) live in `/pcm § System Wiring`.

## Cut mode — `quality:prompt cut <file>`

Rewrite the target leaner in place: read it, apply every rule below, cut hard. Preserve every distinct behavioral rule, threshold, and behavior-pinning example; cut scaffolding, never substance. Never weaken a sacred-ground rule ({SENSITIVE_DATA}, {DOMAIN_ADJ} safety, secrets) to save tokens. Report each cut in one line.

## The cut test (apply to every line)

> Would removing this line cause Claude to make a mistake?

If no — delete it. Bloat dilutes the rules that matter; the model "may start forgetting earlier instructions or making more mistakes" as the file grows.

## Compact aggressively (the layer below the cut test)

The cut test deletes lines that change nothing; this compacts the survivors. Run both passes, repeat until neither fires:

- **Merge.** Rules covering overlapping ground collapse into one that covers both.
- **One word for two.** Where one precise word carries a phrase, use it. Recurse clause by clause until removing any word costs meaning.

Before → after, at the aggression this expects:

- `stopping to ask is the only failure.` → `only failure = stop/ask`
- `resolve every ambiguity and blocker yourself and carry the work to completion` → `resolve ambiguity/blocker by yourself & get to completion`
- `**Reuse before you write** — grep for an existing function/type/util and import it before adding one. Never keep a near-copy in sync; extract and call.` → `Reuse code - grep for existing code(function/type/util) RE-USE, NO duplication`

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

1. **Time-sensitive narration.** "On 2026-05-19...", "after the X incident", "before August 2025". Encode the rule that resulted; the incident goes in the commit message or the relevant epic manifest, not the prompt.
2. **Dates of change.** Changelog-style "changed 2026-06-07" lines or update-history dates inside a prompt or `.professor/` ledger are the same antipattern — version control already timestamps every change. State the current rule, never when it changed.
3. **Restating one rule — reworded OR repeated across sections (NO DUPLICATION).** Two phrasings of one rule, or the same rule echoed in a non-negotiable, a routing-table cell, and a process bullet (e.g. three copies of "route framework changes through `/pcm`"), make Claude pick one arbitrarily and rot out of sync. State each rule ONCE in its canonical home. Before adding a rule, grep the whole file for its key noun; if it already lives somewhere, sharpen that one and stop.
4. **Frontmatter ↔ body duplication.** If `description:` says it, the body opening must not.
5. **Voice flavor that doesn't change behavior.** Backstory, character arcs, "I built this", "the meta layer". Voice lives in `.claude/output-styles/` — the session persona as the active output style (main-loop only; subagents never receive it), command personas as overlay files read at invocation. CLAUDE.md and every agent/skill/command carry zero voice.
6. **"Why this exists:" / "Why:" paragraphs that just rephrase the rule.** The rule's purpose lives in the rule's wording.
7. **Negative framing where positive works.** "Use prose paragraphs" beats "don't use bullets." Reserve do NOT / NEVER for sacred ground ({SENSITIVE_DATA}, {DOMAIN_ADJ} safety, secrets).
8. **Aggressive emphasis ("CRITICAL", "YOU MUST", "MANDATORY") on non-sacred rules.** {MODEL_TIER} overtriggers on it. Plain language for ordinary rules; reserve emphasis for invariants.
9. **Inconsistent terminology** — mixing "endpoint / URL / route", "field / box / element", "extract / pull / get". One canonical term per concept, used everywhere.
10. **Cross-references that say nothing new** ("See § X above" two paragraphs up). If the reference matters, summarize the takeaway inline.
11. **Inline cross-file restatement** — child CLAUDE.md files restating workspace rules already in root CLAUDE.md. Child files keep ONLY the project-specific delta.
12. **Multiple options when one default suffices.** "Use pypdf, pdfplumber, PyMuPDF, or pdf2image" → "Use pdfplumber. For OCR, use pdf2image+pytesseract."
13. **Examples that don't pin down behavior.** An example earns its tokens only if the rule alone wouldn't produce the same output.
14. **Vague descriptions.** "Helps with documents", "Processes data", "Does stuff with files" → no auto-invocation.
15. **Deeply nested file references** (SKILL.md → reference.md → details.md). Claude `head -100`s and misses content. Keep references one level deep.
16. **Inline incident logs in references / gotchas.** Once the rule is codified, the incident becomes redundant. Move it to the commit message or epic manifest.
17. **Cross-document contract restatement.** Copying SQL/SDL/contract bodies between pipeline or reference docs — cite doc + section instead ("cite, don't restate").
18. **Token-heavy formatting.** HTML tags (`<example>`, `<div>`), XML-style wrappers, drawn ASCII boxes, decorative dividers — they cost tokens markdown gives free. Use `## Example` over `<example name=…>`, a fenced block over a drawn box, a single `—` over a rule of dashes. Keep the structure, drop the scaffolding.
19. **List-item definitions read `- term: gloss`** — a plain term, a colon, one tight gloss. The smell is a bold term, an em-dash, and clauses chained with `;` / `—` into a run-on. `- High: the default — step up only for a genuinely hard problem` beats `- **High** — the level you reach for in nearly all work; balances depth against cost; step up only when the task truly demands it`.

## Teaching by example — when a stated rule keeps leaking

When the model keeps violating a rule the prompt already states, that is confusion about where the rule applies, not disobedience — sharpening the wording or piling on emphasis only adds noise. Reach for a **contrastive example**: show the tempting WRONG answer and the trap that produces it, then the correct one (✗→✓), drawn from a real failure. **Counterweight** every "avoid X" example with an "X is correct here" example — a contrastive example against a frequent label otherwise teaches the model to avoid that label everywhere, suppressing its legitimate uses.

## Example — encoding an incident rule

Wrong (in the prompt):

> The seed script once published the analysis request before registering the result waiter, so the seed hung to its full timeout. Never publish before registering again.

Right (in the prompt):

> Register the result waiter before publishing the analysis request.

The incident narration moves to the commit message / epic manifest. The rule stays sharp.

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
11. **No duplication:** Did I grep this file for the rule's key noun and confirm it appears in exactly ONE section? And did I leave skill/command rosters out (Claude Code self-indexes them)?

## Where things go (anti-bloat routing)

| Content                                             | Belongs in                                            | NOT in                                            |
| --------------------------------------------------- | ----------------------------------------------------- | ------------------------------------------------- |
| Behavioral rules                                    | Prompt files (CLAUDE.md, agents, commands, skills)    | —                                                 |
| Incident narratives ("on 2026-XX-XX...")            | Commit message / epic manifest (`docs/epics/{name}/`) | Prompt files                                      |
| Architectural decisions / why-this-design           | Epic manifest or `docs/commands/{cmd}/references/`    | Prompt files (encode the rule, not the rationale) |
| Voice / character flavor                            | `.claude/output-styles/` (session style + overlays)   | CLAUDE.md, agents, skills, commands (zero voice)  |
| Project-specific tooling                            | Child CLAUDE.md only                                  | Per-project agents (already inherit via parent)   |
| Cross-cutting templates (report format, plan shape) | One canonical reference file                          | Duplicated per-project                            |

## Hooks vs prompts

For things that must happen every time (formatting, validation, secret-scanning), write a hook (`.claude/settings.json` PreToolUse / PostToolUse) — deterministic, cheap. Prompts are advisory; the model can drift. Don't try to enforce critical invariants by repeating "ALWAYS DO X" in the prompt. Once a hook owns an invariant, delete the prompt rule that restated it — keeping both is duplication against a deterministic mechanism.

## Iteration discipline

When a rule fails in practice:

1. Observe Claude's actual output (not how the prompt reads).
2. Diagnose: is the rule ambiguous, contradicted by another rule, or buried in noise?
3. Fix surgically — sharpen, move, scope. Adding more emphasis is rarely the answer.

If the failure is recurring and structural, consider a hook instead.

## /km knowledge files are prompts too

`/km` writes domain knowledge files under `{AI_PROJECT}/knowledge/` that get injected verbatim into the {AI_SERVICE_NAME} LLM context. Every rule above applies: cut test, cue density, one canonical term, no narration. `/km` carries additional domain-specific rules ({LLM_PROVIDER} bias control, schema fidelity, {REGULATION} compliance) — load both: this command (`/quality:prompt`) for the prompt-quality discipline, `/km`'s Sacred Ground for the domain layer.
