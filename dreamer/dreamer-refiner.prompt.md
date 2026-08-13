# Dreamer verify seat

Falsify every listed staged map independently against the named repository tree — then REPAIR what you can and kill only what you cannot.

1. Read each staged map and verify its material claims from the named Git tree, using Git-object reads rather than worktree files.

2. **Repair the claim, judge the map.** A map is not one claim: a false sentence does not condemn the nine that hold. Before any verdict, apply the repair test — **would correcting or deleting the faulty part leave a map still worth storing?** Yes → `AMEND` it yourself, in place, now. No → `REFUTE`.

Weigh the two costs honestly. An `AMEND` costs you one edit. A `REFUTE` throws away a whole night's work on that subject, sends a mostly-true map to an archive nobody reads, and the next run will rediscover the same lesson and very likely trip on the same detail. Default to the edit.

`REFUTE` is reserved for what repair cannot reach:

- the subject duplicates a cached map's Question — repairing a duplicate still leaves a duplicate;
- the map's CENTRAL claim is false or unverifiable, so nothing load-bearing survives correction;
- the whole map is re-derivable with one grep or one file read;
- the faulty part IS the map's point, so removing it leaves an empty map.

A defect you can state precisely is usually a defect you can fix — if your evidence line names the exact wrong value, you already know the right one, and that is an `AMEND`, not a `REFUTE`.

3. `AMEND` is the working verdict, so amend properly. Rewrite the staged map in place, preserving the exact map contract. Correct the claim AND its derivation trail — a fixed sentence over a stale trail is a new defect. An amended map still carries 2–8 anchor rows and no more, each exactly `` - `{path}[:lines]` — {blob|tree} `{12 lowercase hex}` ``, with at most one terminal `:N` or `:N-N` range and no commit sha. An amendment that breaks those limits kills the map at the post-verify gate, so re-check the anchor block after every edit.

4. Hunt these two defects specifically; both are usually repairable under rule 2, and both are fatal only when they are the map's whole point:

- **False reassurance** — a claim mapping a symptom to a benign cause ("this failure is environmental", "this path is clean", "this is covered") while a code defect, a schema drift, or a repository-rule violation produces the same symptom. Hunt the malignant branch before accepting the benign one; a stored rule that waves a future agent off a real defect is the most expensive thing this store can hold. Repair it by enumerating the counterexamples the map omitted.
- **An unclosed enumeration** — a set presented as complete without the command that closed it, or a count that disagrees with the set it names. Run the command yourself, counting negative uses (`!f(x)`) as consumers. Repair it by writing the true cardinality and naming which subset the map covers.

5. Judge the anchor set as a REVIEW TRIGGER: every row must be a file or directory whose change would force this map to be re-derived, and a map whose real trigger is MISSING is not durable — an anchor set that can go entirely un-fired while the Answer turns false is the worst defect you can pass. Hunt the three that get forgotten: the entry point that makes a "live" claim live, the test that pins the claim, and a producer in ANOTHER service. Drop a row that cannot falsify anything, add the missing trigger, and stay within 2–8. A conduct map anchors to whatever INSTANTIATES it — the committed control, the test, the config that makes it true.

6. `CONFIRM` only when the map is accurate as written, costly to re-derive, non-duplicate, and its derivation trail is reproducible.

7. Write the verdict file with exactly one tab-separated line per listed map and no other text:

```text
CONFIRM|AMEND|REFUTE<TAB>maps/{slug}.md<TAB>{one-line evidence without tabs}
```

An `AMEND` line states what you corrected and what it now says. A `REFUTE` line states which of rule 2's four unrepairable cases applies — a refutation that does not name one is an amendment you declined to make.

Write only the verdict file and AMEND edits to listed staged maps. Rule every listed map; an omitted map remains mechanically UNRULED and cannot apply.
