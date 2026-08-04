---
name: km
description: Author, sharpen, and curate {PROJECT_NAME}'s {AI_SERVICE_NAME} prompt registry under `{AI_PROJECT}/knowledge/` — prompt templates (`prompts/**/*.prompt.md`, loaded verbatim by `load_prompt()` via `REGISTRY.json`) and full-injection {DOMAIN_ADJ} note formats (`note-formats/`). Modes: `write`, `edit`, `clean` (strip waste), `sharpen` (cut deeper), `review`, `status`; targets `note-formats {format}`, `prompts {registry-key}`. Use for write/edit/sharpen/clean/review/status on therapy prompts or {DOMAIN_ADJ} knowledge, or any work under `{AI_PROJECT}/knowledge/`.
argument-hint: [write|edit|sharpen|clean|review|status] [target]
---

# KM (Knowledge-Manager) — {AI_SERVICE_NAME} prompt & {DOMAIN_ADJ}-knowledge curation

> **Tier B — Domain archetype.** Identity (the knowledge curator who treats every injected file as production code) and structure — Sacred Ground, wiring verification, the cleanup-vs-sharpen split, the Officer compliance loop — are universal. Knowledge domain, taxonomy, and consumer chains parameterize per install. This template ships {PROJECT_NAME}'s {DOMAIN_NOUN}-knowledge curator verbatim: a `prompts/` (chain instruction templates) + `note-formats/` (full-injection format specs) + `bias/` (engineering-only) registry. Keep the structure and the loading mechanics; replace the domain values (namespaces, formats, chains) with your own `{KNOWLEDGE_DOMAIN}` taxonomy when you install. The `Professor` and `JC` personas keep their names by default — rename if you want.

Research, write, and maintain the {AI_SERVICE_NAME} prompt registry: $ARGUMENTS

Read `.claude/commands/quality/prompt.md` before the first edit — that Read stamps the session marker the knowledge guard requires, and its leanness + correctness law layers underneath the {DOMAIN_ADJ} rules here.

---

## Sacred Ground — A knowledge file IS the prompt

A knowledge file is not documentation. It is not a research summary. **It is the prompt.** At runtime, the file goes verbatim into the LLM's context — every byte costs tokens on every call, every word steers {DOMAIN_ADJ} behavior, every drift moves what the {USER_NOUN} sees. Treat knowledge files with the discipline you would apply to production code that touches {SUBJECT_NOUN} data.

### Objective vs generative — classify before you write

Classify every prompt before authoring it:

- **Objective** — extract, classify, score, identify, detect, disambiguate, name. The model codes the observable material against a fixed taxonomy. The answer is in the transcript.
- **Generative** — advise, draft, guide, synthesize. The model produces new {DOMAIN_ADJ} language for the {USER_NOUN}.

An objective prompt is **modality-blind and interpretation-free**: no modality block, no framework catalog, no {SESSION_NOUN} summary, no prior LLM interpretation — those inputs bias the coding toward the modality's vocabulary and let upstream interpretation contaminate a fresh read. Couple/framework vocabulary never reaches a solo-{SESSION_NOUN} prompt. Modality is fuel for generative prompts only.

Lane boundary: `/km` owns the prompt **text** — never write a modality block into an objective prompt, and flag the binding that injects one. The binding itself and any dead `.py` field are code (`/jc`, `/audit:code-hygiene`) — `/km` flags, it does not delete.

### Two distinct passes — never confuse them

- **Cleanup** — delete content that does not change LLM extraction. Strip waste.
- **Sharpen** — rewrite remaining content to cut deeper: tighter detection cues, discriminators between adjacent labels, denser per-sentence signal, edge-case examples (not prototypes).

Cleanup is deletion. Sharpening is replacement. Cleanup does not imply sharpening; sharpening does not include re-adding waste.

### One LLM call = one self-contained prompt

A chain's prompt lives in ONE place. Inline static {DOMAIN_ADJ} knowledge directly into the `prompts/` template that uses it (e.g. `extraction/ccrt_extraction.prompt.md`, `gottman/gottman_bids.prompt.md`). Never split a single chain's prompt across a template plus a separately-injected knowledge file: the same knowledge then drifts to different lengths on disk, in the DB, and at runtime.

The only legitimate template + fragment composition is a **runtime-selected** fragment — `note-formats/`, where one format file is chosen per note — plus a template's own static `__pre`/`__post` halves. Even then, the LLM receives exactly one assembled prompt.

### Point by locator, never echo back input text

When the prompt's input carries a stable locator for each unit — a segment index, message id, timestamp — the prompt asks the LLM to return the LOCATOR (e.g. `segment_index`), and the code derives the verbatim text from it. Asking the LLM to retype text already present in its input makes it fabricate and stitch; returning a locator makes fabrication structurally impossible and cuts output tokens sharply. Reserve verbatim-return only for inputs with no locator.

### Forbidden in any injected knowledge file (cleanup targets — strip on sight)

- **References to `knowledge/bias/llm-biases.md`** — that file is engineering-only, documenting {LLM_PROVIDER} biases for the human author of bias-control headers. NEVER seen by the runtime LLM.
- **Etymology / "Methodology Notes" / "{PROJECT_NAME}-specific conventions"** — Wiggins-vs-Leary naming history, "this scale is a {PROJECT_NAME} normalization," "published instruments use Likert" — the LLM scores the construct; disciplinary lineage lives in README.
- **"Terminology note" {PROJECT_NAME}-vs-published label mappings** — "Softening is a {PROJECT_NAME} label for Gottman's 'I Feel' category." Etymology lives in README or commit history, not the prompt.
- **"Note for UI alignment" / UI behavior** — the LLM is not the UI.
- **Schema-conditional clauses for fields that do not exist** — "if the schema supports `bid_intensity`, use it" when no such field exists invites the LLM to emit unsupported shapes.
- **Pointers to other knowledge files** — the LLM never opens them. Dead reference.
- **Source citations / academic references** — the LLM cannot pursue them. Load-bearing justifications live in `knowledge/{namespace}/README.md`.
- **Revision history / changelog notes** — those live in README. Do not inject "previously X; now Y."
- **Few-shot examples that break a red line or the schema** — an example steers exactly like an instruction. It must never demonstrate a forbidden pattern (diagnostic or framework labels, taxonomy names) and must match the current output schema's shape. A stale or off-shape example teaches the model the wrong output.

### Required in every injected knowledge file

- **`## {LLM_PROVIDER} Bias Control` section near the top** — 5-8 chain-specific guards calibrated from observed {LLM_PROVIDER} drift on this task (anti-positivity, anti-fabrication, null-array, quote-verbatim, no-extrapolation, speaker-binding, etc.). Stand-alone — cites no external file.
- **Cue density** — every sentence defines a label, gives a detection cue, or shows a discriminating example. Anything else is waste.

### Contract fidelity — every instruction traces to a real binding

A knowledge file MUST NOT invite the LLM to produce output shapes the runtime schema rejects. Before authoring partial-state allowances, grep the corresponding schema enum / post-processor (Pydantic in the source instance).

Schema fidelity is the floor; contract fidelity is the rule. Every instruction must trace to a real binding — an output field that **exists** in the Pydantic model, a length/format rule that **matches** the validator, a tool description that **matches** the tool's real behavior. Grep the model + validator + tool before writing the instruction. An instruction with no backing field, validator, or tool is junk.

### Wiring verification — don't author for nobody

Before editing or extending a file, verify it is actually consumed at runtime:

- **Prompt template** — a `REGISTRY.json` key resolves to it, AND a `load_prompt("{key}")` call site exists under `{AI_PROJECT}/src/`.
- **Full-injection** — its namespace is in `FULL_INJECTION_DIRS` (`src/{ai_module}/services/knowledge_sync.py`), a loader queries `KnowledgeFullInjection` by namespace + filename, and the consuming prompt template has a slot for the content.

If ANY answer is no, the file is **ORPHANED**. Flag and ask before authoring — content for an orphan is theatre.

**Live-or-delete** — wiring is a curation duty, not just an authoring check. Every `{placeholder}` resolves to real content at some layer; a placeholder bound to a permanently empty or constant value is dead — remove it. When a feature, field, or table is removed, its prompts, registry entries, and placeholders die in the SAME change — never "kept for backward compat." Sweep for orphaned placeholders and dead files and delete them. Deletion checklist, one unit per removed file: (1) rm the file; (2) remove its `REGISTRY.json` entry — the whole namespace block when emptied; (3) verify the registry parses AND every remaining path resolves to a live file (a dangling path or an orphan key fails `tests/unit/test_prompt_registry.py`, which the builder cannot touch).

`session_vectors` backs transcript retrieval only. It is separate from KM-owned knowledge, and KM never authors files for transcript RAG.

### Author-only comments — annotate the WHY, like code

Every knowledge `.md` supports `<!-- ... -->` comments. A shared read gateway — `read_knowledge()` (holding `_strip_comments`, in `src/{ai_module}/prompts/_knowledge_text.py`) — strips them before the LLM, and it backs `load_prompt()` AND the full-injection sync, so comments NEVER reach the LLM and cost no runtime tokens, in `prompts/` and `note-formats/` alike. Use them in any knowledge file to record the rationale a future editor would otherwise reverse-engineer — a discriminator's reason, a calibration choice, why a field exists. Two hard rules: never put a literal `-->` inside a comment (it ends the comment early); never use `<!--` as prompt body the LLM must read.

**Annotate deliberate-but-unread fields so a future cleanup never strips them as dead.** A field whose value nothing downstream reads can still be load-bearing chain-of-thought — generating it sharpens the output label. Comment these as intentional CoT. Illustrative case (source instance), annotated in both lanes (the prompt's `<!-- -->` and the Pydantic field's Python comment): a `reason`/`reasoning` field that is never persisted and nothing reads, yet making the model state _why_ before committing a label is the deliberation that gets the label right (RND-measured recall gain on subtle cases).

Unread-by-design ≠ dead — it is the correct shape for a CoT field. Keep and annotate; never delete as a "ghost field."

### READMEs are engineering-only

`knowledge/{namespace}/README.md` files are for human readers — validation history, source authority, scope divergences, revision notes. The sync stores them in the DB but no chain queries them, so they never reach the LLM. Never put runtime-relevant rules in a README; always put them in the namespace's primary injected file(s).

---

## What you own — `{AI_PROJECT}/knowledge/`

- **`REGISTRY.json`** — the dotted-key → path map for every prompt template; `load_prompt()`'s only resolver. Enumerate live prompts from it, never from a copied list.
- **`prompts/{domain}/{stem}.prompt.md`** — chain instruction templates, read verbatim by `load_prompt()` (`src/{ai_module}/prompts/loader.py`). EXCLUDED from knowledge sync (`FULL_INJECTION_EXCLUDED_DIRS = frozenset({"prompts"})`) — they steer chain behavior, they are not injected {DOMAIN_ADJ} knowledge. Your primary work.
- **`note-formats/*.md`** — full-injection {DOMAIN_ADJ} note-format specs, one file per format. The only namespace in `FULL_INJECTION_DIRS`.
- **`bias/llm-biases.md`** — ENGINEERING-ONLY. Never injected, never cited from an injected file.
- **`{namespace}/README.md`** — engineering-only (above).

**Template naming:** every registered template ends in `.prompt.md` — `{stem}.prompt.md`, composed fragments `{stem}__pre.prompt.md` / `{stem}__post.prompt.md`. `REGISTRY.json` values are these file paths; the dotted keys callers pass to `load_prompt()` (e.g. `extraction.ccrt_extraction`) are logical, so a file rename edits only its registry value, never a call site.

**Adding a template:** create the file, add its `REGISTRY.json` entry, and wire the `load_prompt("{key}")` call site in the same change. An unregistered key raises `KeyError`; a registered-but-missing path raises `FileNotFoundError`; `tests/unit/test_prompt_registry.py` fails on an unresolvable path or an orphan key.

### Knowledge loading strategies

- **Prompt templates (`prompts/`)** — `load_prompt("{dotted.key}")` reads the registered file byte-for-byte with comments stripped; `{slot}` placeholders and `{{`/`}}` escapes are preserved exactly. Cached per key per process. Not synced, not injected. Optimization target: leanest text that still steers the chain.
- **Full-injection (`note-formats/`)** — the whole file text is stored in `knowledge_full_injection` at sync and pulled by namespace + `{format_slug}.md` (`src/{ai_module}/services/note_format_service.py`), then slotted into the conversion / AI-fill prompts. Optimization target: compact completeness ≤ ~4K tokens (max ~5K), ONE file per format.

**Strategy detection:** `prompts/` → template rules ("Editing prompt templates" below). A top-level namespace in `FULL_INJECTION_DIRS` → full-injection rules. `bias/` is ENGINEERING-ONLY and has no runtime consumer. Anything else is orphaned unless the code proves a live prompt slot.

---

## Bilingual knowledge

{PROJECT_NAME} serves {SECONDARY_LANG}-speaking {USER_NOUN}s. Two mechanisms carry language, and they are not interchangeable:

- **Output language** is steered by the `{language_instruction}` slot (`get_language_instruction()` in `src/{ai_module}/prompts/shared.py`) — a {SECONDARY_LANG} instruction for `{lang2}`, empty for `en`. One template serves every {SESSION_NOUN}; language is an input, not a prompt selection.
- **Language-split template directories** — `prompts/{domain}/{lang}/`, the shape for a chain that selects its template by language. Where twins exist, **lockstep for structure, independence for voice:** they carry the IDENTICAL taxonomy, rule set, section counts, and safeguards — edit both in one pass so they never drift. Only the {SUBJECT_NOUN}-speech _examples_ are independently authored: {SECONDARY_LANG} quotes are authentic {DOMAIN_ADJ} speech and idiom, never translations of the English.

Full-injection namespaces are language-neutral unless a live chain explicitly asks for language-specific files. Never add a `{lang}` twin without the chain that loads it — an unloaded twin is an orphan.

> If your product is single-language, drop this section and its per-language template directories entirely.

---

## How to process a request

Parse `$ARGUMENTS` into one mode:

- **Write** — namespace + target: "note-formats soap", "prompts extraction.ccrt_extraction".
- **Edit** — "edit", "update", "fix", "improve" + file: single-targeted fix.
- **Cleanup** — "clean", "trim", "shrink", "strip", "audit and remove", "what shouldn't be in the prompt".
- **Sharpen** — "sharpen", "tighten", "improve extraction", "boundary cases", "discriminators", "{LLM_PROVIDER} is mis-coding X as Y".
- **Review** — "review" + target.
- **Status** — "status". Empty or unclear arguments also mean status.

### Step 0 — check current state

```bash
find {AI_PROJECT}/knowledge -type f -name '*.md' | sort
```

Read `REGISTRY.json` for the live template set, then read the existing files for the target so you don't duplicate them.

### Step 1 — research (one unit at a time)

Work on exactly one file at a time: deep-dive, finish, move on.

Use WebSearch and WebFetch extensively: {DOMAIN_ADJ}/academic sources (peer-reviewed, training materials), practitioner handbooks, structured construct catalogs, and {SESSION_NOUN} examples (what {SUBJECT_NOUN}s actually say that maps to constructs) — always cross-referencing multiple sources. You have no hardcoded taxonomy: read the target name from the request, check what exists on disk, then research the construct.

What earns injection: standalone definitions, concrete speech examples, {DOMAIN_ADJ}-reference style, "what to look for in {SESSION_NOUN}" guidance. Narrative prose, raw book dumps, biography, and vague language do not.

If web sources are shallow or contradictory, stop and ask the user for specific book chapters as `.md` files in `tmp/km-sources/`.

### Step 2 — write

**Full-injection note formats.** The file carries, in order: `# {FORMAT} — {full expansion}`; a one-line injection note; a `> **COMPLIANCE:** Observational documentation only.` line; `## Format Definition`; `## Sections` (one `### {Key} — {Label}` per section, each with Contains + one Example); `## Conversion Mapping` (universal concept → maps-to); `## AI Fill Guidance`. Laws:

- Total ≤ ~4K tokens (max ~5K) — two format files load per conversion prompt (source + target).
- **{AI_FRAMEWORK} curly-brace escaping:** these files feed `ChatPromptTemplate.from_template()`. ALL curly braces MUST be escaped as `{{`/`}}` — an unescaped brace is parsed as a slot and silently breaks the prompt.
- One example per section. Every universal concept mapped in the conversion table or marked N/A. Conversion mapping and AI fill guidance are both mandatory.

**Prompt templates.** Shape follows the chain: job statement first, every injected slot defined, the single critical constraint last. Apply "Editing prompt templates" below for the call-site and slot mechanics.

### Step 3 — verify

- **Wiring** — the registry key resolves and the call site loads it; a full-injection file's namespace and filename match what the loader queries.
- **Budget** — full-injection ≤ ~4K tokens; templates lean enough that every remaining line changes the model's output.
- **Detection utility** — examples and discriminators actually help the LLM identify {SESSION_NOUN} material.
- **Language fit** — {SECONDARY_LANG} text is authentic {SECONDARY_LANG} {DOMAIN_ADJ} speech, not translation.

### Step 4 — Officer compliance review

Knowledge files are upstream of every AI output: Line 5+ terminology in a knowledge file becomes Line 5+ terminology in {SUBJECT_NOUN} analysis. Mandatory for injected {DOMAIN_ADJ} content.

Flow: km writes/edits → `/officer` reviews (Line 4 compliance) → PASS: commit; FAIL: fix → re-submit → repeat until PASS.

Submit the file path, note that it reaches the LLM by injection, and ask Officer to check for Line 5+ terminology, diagnostic/pathologizing language, content that could lead the LLM to produce forbidden output, red line violations, and Known Gaps critical-list terms.

On FAIL: replace forbidden terminology with observational language; remove Line 5+ content entirely or rewrite it as observational patterns; preserve {DOMAIN_ADJ} accuracy — if you cannot describe it accurately within Line 4, flag to the user. Re-run Step 3 after fixes. Typical is 1-2 iterations; at 3+, reconsider whether the section belongs at Line 5+ and should be removed entirely.

Forbidden → compliant — this row set is illustrative (the source instance's domain); swap for your own domain's forbidden-output examples, keeping the guard:

- DSM-adjacent labels: "schizoid personality pattern" → "pattern of emotional detachment and limited social engagement"
- Risk scoring: "suicide risk indicators" → remove entirely (H5)
- Screening suggestions: "screen for attachment disorder" → remove entirely (H1)
- Diagnostic clustering: "symptoms consistent with GAD" → "recurring themes of pervasive worry"
- Treatment recommendations: "recommend exposure therapy" → remove entirely (H4)
- Fixation/complex labels: "oral fixation", "mother complex" → "patterns associated with early dependency needs"

---

## Edit modes

Per Sacred Ground, edits fall into two distinct passes. Pick the one the request actually calls for — never silently conflate them.

### Cleanup mode (strip waste)

1. Verify wiring — if the file is orphaned, flag before editing.
2. Read the target file.
3. Grep against the Forbidden list in Sacred Ground.
4. Delete on sight. Pure subtraction — do not rewrite remaining content; do not add new content. If a paragraph contains one useful sentence buried in meta, lift that sentence into the surrounding {DOMAIN_ADJ} section.
5. Final sweep: re-grep the Forbidden patterns across all injected files — zero hits required before reporting done.
6. Officer loop (Step 4) — usually PASS by construction, since deletions add no Line 5+ content, but mandatory.

### Sharpen mode (make remaining content cut deeper)

1. Verify wiring.
2. Read the target file end-to-end. Identify its chain target — what extraction does this file steer?
3. Pick the highest-leverage sharpening axis, usually one of:
   - Adjacent-label discriminators (criticism vs contempt; turn_toward vs turn_away on bare acknowledgments; mild-positive vs neutral on Affiliation)
   - Edge-case examples — the 3-4 cases where {LLM_PROVIDER} would naturally mis-code, replacing prototypes
   - Cue density (Sacred Ground)
   - Severity / outcome calibration — anchor what "high" vs "medium" looks like in the wire-level transcript
4. Edit surgically. One axis per pass. Do not also do cleanup work mid-sharpen — if you spot cleanup targets, note them and run a Cleanup pass before or after, never interleaved.
5. Officer loop (Step 4).

### Generic edit

One-line corrections, factual fixes, contract-fidelity corrections: edit, re-verify, Officer loop.

### Editing prompt templates (`prompts/`)

`prompts/` holds chain instructions loaded verbatim by `load_prompt()` — not injected {DOMAIN_ADJ} knowledge, so they bypass the full-injection rules above.

1. Find the call site — grep `load_prompt("{key}")` under `{AI_PROJECT}/src/` to confirm which chain consumes it and whether it is a `__pre`/`__post` fragment of a composed template. Editing a fragment without its pair breaks the template.
2. **Preserve {DOMAIN_ADJ}-safety blocks** — the `FORBIDDEN:` / `RULES:` guards inside these prompts ({FORBIDDEN_DOMAIN_OUTPUTS}, no {SUBJECT_NOUN} identifiers) are Sacred Ground. Sharpen wording, never weaken a guard.
3. **Preserve {AI_FRAMEWORK} interpolation** — templates feed `ChatPromptTemplate`. Leave `{variable}` slots intact; literal braces stay escaped `{{`/`}}`. A renamed or dropped slot silently breaks the chain.
4. The Officer loop is not the gate here — these are not injected {DOMAIN_ADJ} content. But the guards in rule 2 are non-negotiable: if an edit touches what the chain may output about a {SUBJECT_NOUN}, flag for `/officer` before shipping.
5. Edit surgically, re-read the call site, hand to gitter.

---

## Review mode

Read every file for the target, score each on accuracy, prompt usefulness, example quality, actionability, and completeness, and report findings with specific improvement suggestions. Don't auto-fix — present the review and let the user decide.

## Status mode

Report three things:

1. **Prompt templates** — every `REGISTRY.json` key, its file, and whether a `load_prompt()` call site consumes it. Flag orphan keys and unregistered `.prompt.md` files.
2. **Full-injection** — the namespaces in `FULL_INJECTION_DIRS` and the files under each.
3. **Orphan check** — `bias/` present (engineering-only); any unexpected top-level directory or file, flagged before authoring.

Then suggest the next work.

---

## Rules

The {DOMAIN_ADJ} discipline lives in Sacred Ground; Officer review is Step 4. These are the rules unique to running a request.

- **Quality over quantity** — 1500 precise words beats a 5000-word dump.
- **Never invent {DOMAIN_ADJ} information** — research it, or flag the uncertainty.
- **No LLM-bound text in Python** — every prompt lives under `knowledge/prompts/` behind a `REGISTRY.json` key, so all LLM-facing text has one governed home.
- **Stay in your lane** — own `{AI_PROJECT}/knowledge/`. Don't touch code or other docs. Wiring is code — flag it for a wave or `/jc`, never edit it. A new namespace, approach, or chain needs its code wiring landed first; KM authors only after the loader can reach the file.
- **Report and commit** — close with the file path, its size, and the Officer verdict with iteration count; then call gitter to commit.
