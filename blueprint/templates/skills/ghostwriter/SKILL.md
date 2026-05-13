---
name: ghostwriter
version: "1.0.0"
repo: "https://github.com/mreza0100/ghost-writer"
description: Use when the user wants to extract a reusable writing-style profile from a corpus, generate text in a specific person's style, audit or update an existing voice profile, or humanize AI-sounding text via the bundled human profile. Trigger on phrases like "match my writing style", "write like this", "make it sound like me", "voice profile", "voice DNA", "audit/update my style profile", or when the user pastes a substantial sample and asks for new text in the same voice. Do not use for generic copyediting, grammar cleanup, or broad tone shifts; this skill is for evidence-grounded reproduction of a writer's mechanical fingerprint, cognitive moves, rhetorical structure, and vocabulary.
---

# Ghostwriter

Capture the mechanical fingerprint of how someone writes — vocabulary, sentence rhythm, punctuation density, formatting quirks — from a corpus, generate new text that reproduces those patterns, and maintain the profile as the writer's habits drift.

The corpus is the **source of truth**. Every rule comes from the corpus, not from priors about what good writing looks like.

## Bundled references

Read these when the situation calls for them. Don't try to keep them all in working memory.

- `references/llm-isms.md` — 29-pattern catalog of LLM-tells. Used during Mode A (corpus sanity-check), Mode A.5 (canonical definition of the `LLM_ISM` tag), and Mode B (pre-delivery self-check).
- `references/extraction-checklist.md` — corpus preparation rules and the 8-dimension mechanical extraction grid. Read before starting Mode A.
- `references/cognitive-moves.md` — extraction prompts and categories for the idea-level layer (framing, reasoning, concretization, rejections, conclusion shape, audience assumptions, argument shape). Read before extracting any non-trivial profile; read again when generating in Mode B.
- `references/rhetorical-structure.md` — the essay-arc layer that sits between cognitive moves (idea-level) and the mechanical/vocabulary layers (word/sentence-level): opening shape, full argument arc, scale-shifts, example-texture mix, reference horizon, self-reference patterns, term-coining propensity, meta-commentary, negation-as-thesis, definition-by-compression, aphorism placement, footnote habit. The layer that captures *what shape the piece takes* — easy to miss and central to writers whose recognizability is largely macro-scale.
- `references/vocabulary-fingerprint.md` — the lexical layer: verb preferences, hedge vocabulary, intensifiers, synonym binaries, casualism markers, sentence-final and topic-shift vocabulary. Read during Mode A vocabulary extraction; read again during Mode B drafting. This is where classical stylometry's identifying signal lives.

## What this skill is, and isn't

This skill captures four layers from a corpus and reproduces them in new text:

1. **The mechanical fingerprint** — pet phrases, sentence shapes, punctuation rates, formatting quirks. Observable, quotable, density-tracked.
2. **The cognitive moves** — the repeatable operations the writer performs on an idea before assembling words: how they frame problems, how they test claims, where they concretize, what they refuse, how they shape conclusions.
3. **The rhetorical structure** — the essay-scale shape: opening pattern, full argument arc, scale-shifts, example-texture mix, reference horizon, self-reference patterns, term-coining, meta-commentary, negation-as-thesis, aphorism placement, footnote habit. This is the layer that captures what shape the piece takes as it unfolds — the macro layer above sentences and below ideas.
4. **The vocabulary fingerprint** — the specific words the writer reaches for when alternatives exist: verb preferences, hedges, intensifiers, synonym binaries, casualisms.

All four layers must be evidence-grounded. Every rule requires at least two quoted instances from the corpus.

This skill is *not* a persona-direction skill. It does not capture worldview, opinions, values, beliefs, or vibe descriptors ("warm", "snarky", "earnest"). Those are downstream of cognitive moves and they read fake when ported into a different context. The distinction matters: a cognitive move is "reflexively asks 'compared to what?' before accepting a comparison" — observable in two quoted moments. A vibe descriptor is "is skeptical" — a label, not a move. The move is in scope; the label is not.

This split — mechanics + moves but no persona — is a deliberate position. McFarland's voice plugin argues that pure mechanics produce monotonous output and prefers persona; Dumont's argues for pure mechanics. The cognitive-moves layer is the bridge: the upstream layer that shapes word assembly, while staying as auditable as the mechanics. If output still feels off after both layers are dialed in, the gap is genuinely in persona, and the user should pair this skill with a separate persona prompt.

Voice doesn't operate alone, either. In a content-creator setting, it's one axis of a triple: voice (how it sounds and thinks) + audience (who it's for) + business context (what it's selling or building toward). This skill owns voice. If the user needs the other two, point them at separate ICP and business-profile work.

## Modes

Pick the mode based on what the user asked for. Modes can chain in one response.

- **Mode A — Extract.** Corpus → structured profile (saved as markdown).
- **Mode A.5 — Calibrate.** Profile → calibration samples → tagged feedback → revised profile.
- **Mode B — Generate.** Profile (or sample + prompt) → new text in that style + audit note.
- **Mode C — Audit.** Existing profile + N recent pieces → drift report (4 buckets: strong / thin / missing / fix).
- **Mode D — Update.** Existing profile + audit findings or feedback → revised profile + changelog.

If the user gives a sample and asks for new text in one go, do A then B. If they bring a profile that's been around for weeks and want it tuned to recent work, do C then D. **Profiles compound with iteration** — every audit-update cycle makes the profile sharper. Treat it as a living document, not a one-shot artifact.

## Built-in profiles

One profile ships with the skill. It lives at `profiles/human.md` and is the default fallback.

- **`human`** — the **negative profile**. Instead of capturing one writer's fingerprint, it bans the full 29-pattern LLM-ism catalog (see `references/llm-isms.md`) and sets human-typical density ranges (burstiness σ ≥ 7, em-dash ≤ 3/1000w, no rule-of-three lists, etc.). Use it when:
  - The user asks to "humanize" some AI-sounding text — no specific writer involved, just remove the tells.
  - The user wants generic-but-human writing and hasn't profiled anyone yet.
  - A specific person's profile is too thin to use confidently — fall back to `human` and prompt for more samples.

This is a different *kind* of profile from a person profile — it has no pet phrases, no fingerprint to reproduce, no signature moves. The whole point is the absence of identifiable patterns plus the LLM-tell ban list. Treat it as the floor: any person profile should at least clear what `human` clears, and add a fingerprint on top.

Don't run Mode A.5 (calibration) or Mode C (audit) on `human` — there's no source corpus to drift from. Mode B is the only mode that meaningfully applies.

If the user asks to write something and doesn't specify a profile, default to `human` rather than asking. Quietly mention which profile was used in the "Rules applied" note so they can swap if they wanted a specific person.

## Core principles

### The corpus is the source of truth

Every rule must be grounded in evidence. Attach a short quoted example AND a frequency to each rule — not "uses em-dashes" but "em-dash ~3 per 1000 words; e.g., 'and then — without warning — it stopped'". If you can't find a quote that demonstrates the rule, the rule isn't really there. Drop it. **Under-claiming beats over-claiming.**

The alternative — a confident-sounding profile of generic "good writing" rules — is the most common failure mode. It will make generated text sound like default-Claude prose with a costume on, not the actual person.

### Density, not presence

For every recurring quirk, capture **the rate**. "Em-dashes ~3/1000w", not "uses em-dashes". "Sentence fragments ~1 in 12 sentences", not "uses fragments". When generating, match the *rate* the writer actually uses — caricature is the failure mode of presence rules.

### VOICE vs PLATFORM vs BORDERLINE

Before recording any rule, classify it. Mark the bucket in the profile.

- **VOICE** — a personal pattern that travels with the writer across formats. ("Starts paragraphs with 'so'.", "ends emails with 'cheers,'.")
- **PLATFORM** — a convention of the medium the corpus came from, not the writer. (Slack: short single-sentence messages; LinkedIn: line breaks every sentence; academic: passive voice and hedging.)
- **BORDERLINE** — could be either; flag and ask the user, or note as low-confidence.

If you encode platform conventions as personal voice, the imitation will read fine in the original medium and feel wrong everywhere else. With a single-medium corpus, default ambiguous patterns to PLATFORM unless the pattern is unusual enough that it's clearly the writer's own.

### What NOT to capture

- **Opinions, beliefs, values.** "Believes in transparency." "Pro-open-source." These are downstream of cognitive moves and read fake when ported.
- **Subject matter / topics.** A corpus about cooking → "writes about food" is not a style rule.
- **Voice / tone / personality descriptors** ("warm", "snarky", "earnest", "thoughtful", "contrarian"). Subjective labels. The cognitive-moves layer captures the *moves* that produce these impressions, with quotes; the labels themselves are out.
- **Coarse cognitive archetypes** ("thinks like an engineer", "approaches things like a journalist"). Too broad to be useful; quote specific moves instead.
- **Generic good-writing or good-thinking virtues** ("uses active voice", "considers counterarguments"). Only include if the corpus shows them as *distinctively* present, with numbers or two quoted instances.
- **Platform conventions** confused as personal voice.
- **LLM-isms in the corpus.** If the corpus has clusters of patterns from `references/llm-isms.md`, verify with the user before encoding any of them.

The line between an in-scope cognitive move and an out-of-scope vibe descriptor is the evidence rule: a move is something you can quote two distinct instances of. A vibe descriptor is something you'd have to *argue* the writer demonstrates. If you're arguing, it's not in scope.

---

## Mode A: Extract a style profile

### Step 1: Vet the corpus

Read `references/extraction-checklist.md` for the full corpus rules. The short version:

- **10+ documents, 2+ formats** is the floor. Single-format corpora encode the format's conventions as voice.
- **No AI-generated content.** AI in the corpus poisons extraction.
- **Recency:** prefer the last 2 years.
- **Length variety:** include short and long pieces.
- **Single author:** all samples written by the person being profiled.

If the corpus fails any rule, say so explicitly in the profile's *Confidence notes*. A tentative profile from a thin corpus is honest; a confident one is a lie.

### Step 2: Read along the 8 mechanical dimensions

Run through the dimensions in `references/extraction-checklist.md`: sentence patterns, opening patterns, vocabulary fingerprint, structural patterns, tone markers, formatting habits, language-specific patterns, LLM-ism scan. Treat each independently — patterns in one don't predict patterns in another.

### Step 2b: Read along the cognitive-moves dimensions

Run through the categories in `references/cognitive-moves.md`: framing moves, reasoning moves, concretization tendencies, reflexive rejections, conclusion shape, audience assumptions, argument shape. Use the extraction prompts at the end of that file — they surface moves that a one-pass mechanical read misses.

Apply the same evidence discipline as the mechanical layer: a cognitive move is only a rule if you can quote two distinct moments from the corpus where the writer demonstrably uses it. If you'd have to argue the move is present, it isn't a rule.

### Step 2c: Extract the rhetorical structure

Run through the 12 categories in `references/rhetorical-structure.md`: opening shape, argument arc, scale-shifts, example-texture mix, reference horizon, self-reference pattern, term-coining propensity, meta-commentary, negation-as-thesis, definition-by-compression, aphorism placement, footnote habit.

Use the 12 extraction prompts at the end of that file. Two are especially high-leverage:

- **Sample 10+ piece openings and classify the dominant shape** (paradox / anecdote / question / scene-setting / direct thesis / historical / negation). Most writers have one dominant opening shape across 60%+ of their pieces.
- **Trace the full structural arc of 5 representative pieces.** The arc is one of the most identifying patterns in the corpus and is invisible at the sentence level.

This layer is the most-often-missed layer in style imitation. For thinner corpora (<5 pieces or <3000 words), use the abridged extraction in that file.

### Step 2d: Extract the vocabulary fingerprint

Run through the 12 categories in `references/vocabulary-fingerprint.md`: top content lexicon, function-word patterns, verb preferences, hedge vocabulary, intensifiers, synonym binaries, casualism markers, profanity, sentence-final vocabulary, topic-shift vocabulary, question vocabulary, and lexical banned-by-omission.

The single highest-leverage section is **6.6 Synonym binaries** — when the writer had a choice between two common alternatives, which side did they pick consistently? Aim for 8-15 binaries from a substantive corpus.

For thinner corpora (<5000 words OR <10 pieces), do the abridged extraction described in that file.

### Step 3: Compute the quantitative layer

Numbers Claude can compute directly:

- Avg sentence length and **burstiness** (= standard deviation of sentence length).
- Avg paragraph length in sentences.
- Type-token ratio (first 500 words of long-form samples).
- Em-dash, semicolon, colon, ellipsis rates per 1000 words.
- Contraction rate (actual / eligible).
- Hedge-word rate per 1000 words.
- Top 5 sentence-initial connectors with counts.
- Exclamation rate per 1000 words.

Don't fake precision. Small corpus → say so and round.

### Step 4: Classify and rate every rule

For each rule: quoted example, density figure, VOICE / PLATFORM / BORDERLINE, confidence (high / medium / tentative).

### Step 5: Save the profile

Default path: `.claude/skills/ghostwriter/profiles/<name>.md`. If not writeable, fall back to the user's outputs folder. Show the rules in chat as well.

---

## Mode A.5: Calibration round (run after first extraction)

Extraction misses things humans only notice in fresh output. Run this every time after Mode A.

1. Generate **3 short calibration samples** in the new style — one per most-likely-use format.
2. Ask the user to mark each issue with a tag: `WRONG`, `OVERSTATED`, `UNDERSTATED`, `MISSING`, `NEEDS_NUANCE`, `LLM_ISM`, `NOT_ME`.
3. Apply the tagged feedback.
4. Log the change in the changelog. Re-render calibration samples once. Stop after one revision unless the user wants another pass.

---

## Mode B: Generate (or humanize) text in the style

Mode B covers two related operations:

- **Generate** — input is a writing prompt; produce new text in the profile's style.
- **Humanize** — input is existing AI-sounding text; rewrite it through the profile (typically `human`) to remove LLM-tells.

### Choosing a profile

1. If the user names a profile, use it.
2. If the user provides a corpus and asks for new text in one go, do Mode A first (quick), then come back here.
3. If neither, default to `profiles/human.md` and note which profile was used.

### Steps

1. **Read the profile top-down.** Order matters: banned words first, cognitive moves next, rhetorical structure, then quantitative and vocabulary layers.
2. If the input is existing text (humanize), read it once for content, once for which LLM-isms it contains.
3. **Apply the cognitive moves to the topic before drafting words.**
4. Check the **priority hierarchy**: hard contextual norms > audience norms > personal style > platform conventions.
5. Draft (or rewrite). Reproduce profile rules at documented densities. Match densities; don't crank.
6. Run the **three-pass self-review** before delivering (LLM-ism scan, performative scan, moves/structure/vocabulary pass).
7. Append the **Rules applied** note (with profile name).

---

## Mode C: Audit a profile against recent writing

1. Read the existing profile.
2. Read N recent pieces (3+ ideally).
3. Re-compute the quantitative layer.
4. For each rule in the profile, check: does it still hold?
5. Look for new patterns not in the profile.
6. Produce a drift report using the **4-bucket structure**: strong / thin / missing / fix.

---

## Mode D: Update an existing profile

1. Open the profile.
2. Apply the changes — adjust densities, update or remove decayed rules, add new patterns with evidence.
3. **Append a changelog entry.**
4. Show the diff in chat.
5. Optionally re-run Mode A.5 calibration.

---

## Output format (always)

- **Mode A:** the profile (saved to file + shown in chat). Optionally followed by Mode A.5.
- **Mode A.5:** calibration samples + tag-request prompt; after feedback, revised profile + diff.
- **Mode B:** the new text + the "Rules applied" note (with three-pass self-review).
- **Mode C:** the 4-bucket drift report (saved + shown).
- **Mode D:** the diff + updated profile (saved).
- **Combined:** chain in order, separated by `---`.

## Edge cases

- **Corpus under ~150 words or single document.** Profile will be tentative. Offer to proceed with caveats, or ask for more.
- **Single-format corpus.** Mark all platform-conditioned patterns as PLATFORM.
- **Multilingual writer.** Extract per language; some patterns carry across; vocabulary mostly doesn't.
- **User asks "write more like this" mid-draft.** Treat the existing fragment as a tiny corpus; do a quick extract; continue.
- **Profile passed in but parts conflict with the new sample.** Flag the conflict; ask which to honor.
- **Sample contains AI-generated text.** Detection cues in `references/llm-isms.md`. Verify with the user before encoding any of them.

---

## Version & Updates

**Current version:** 1.0.0
**Repository:** https://github.com/mreza0100/ghost-writer

To check for updates, compare the `version` field in your installed SKILL.md frontmatter against the latest in the repository.
