---
type: decision
status: accepted
date: 2026-10-08
affects:
  - request-decomposition
  - text-editing
source_paths:
  - internal/director/program/decompose.go
  - internal/director/edit/planner/planner.go
---

# ADR-142 — the splitter keeps no vocabulary of its own

## Context

One task — *"add a short grocery list, then change milk to oat milk"* — found two defects in the
clause splitter, and neither was about the thing being built. Both are the same shape: a fact
about the person's words, held in a second place that drifted away from the first.

## Defect one: forty words maintained by hand

`Split` decides whether a clause is a new instruction or the tail of the previous one's text.
That matters because "and" is not always a separator: *"type hello and goodbye"* is one
instruction whose TEXT contains a conjunction, and a clause that does not begin like an
instruction is rejoined to the one before it.

The test was a closed list of forty verbs, written here. "Change" became a legal way to ask for
a replacement ([[ADR-141-finding-text-by-what-it-says]]) and this list did not learn it, so
", then change milk to oat milk" was not an instruction — it was the tail of the clause before
it, and the clause before it was an **append**. The words "then change milk to oat milk" went
into the document.

The list's own comment had worked out the shape of this failure and misjudged it:

> harmless, because resolution then fails, but it tells the user the wrong thing about why

That holds for a click, whose target then does not resolve. It does not hold for an edit, where
the swallowed clause is not a target but TEXT, and the step succeeds.

**Decision: the editing vocabulary is derived, not copied.** `planner.Verbs()` reports every
first word that package reads, built from the same literals `Parse` consults, and `decompose`
takes the union with its own list. A new editing phrasing teaches the splitter by existing.
One list, one fact.

The splitter's hand list stays — for the verbs that are not edits (click, focus, move) and for
its deliberate entries naming instructions the Director does not implement, which exist so a
clause beginning with one becomes its own step and gets rejected by name instead of quietly
rejoining.

It could not simply ask the planner "is this a phrase you read": "goodbye" is the tail of "type
hello and goodbye", and a parser permissive enough to be useful would read it as a target. The
question it needs is narrower and is what `Verbs` answers — does the clause BEGIN with a word
that starts an edit.

## Defect two: the rejoin rewrote the person's own words

Fragments were rejoined with a hard-coded `" and "`, whatever had been cut. So the commas in a
list came back as conjunctions:

```
"add milk, eggs and bread"  →  "add milk and eggs and bread"
```

Nothing failed. No step refused, the append verified against the text it was handed, and the
document held a plausible grocery list that was not the one anybody wrote.

**This is the quietest class of defect this project has.** There is no outcome word for it: the
request was understood, the action was performed, the verdict was accurate about what it
checked, and the result is only wrong when read against the request character for character. It
is invisible to every oracle that asks "did something happen".

**Decision: the separator is part of the person's text until something proves it was a
conjunction.** Each fragment carries the separator cut from in front of it, verbatim, and gets
it back if it turns out to be a tail rather than a new clause.

The comma forms — `", then "`, `", and then "` — joined the separator list for the same reason:
without them the comma in "bread, then change…" is not part of the cut, so it stays on the end
of the first clause and is typed into the document.

## Enforced by

- `TestAClauseBeginningWithAnEditVerbIsItsOwnStep` — the measured request, split in two.
- `TestTheSplitterKnowsEveryEditingVerb` — walks `planner.Verbs()` rather than naming words, so
  the next phrasing cannot arrive unannounced. Naming "change" would have closed one hole and
  left the next one open.
- `TestVerbsListsEveryWordThePlannerClaims`, `TestVerbsDeclaresNothingItDoesNotRead` — the
  derived list is complete and claims nothing it cannot read. Tested in the planner package,
  because the splitter's own list still covers several of these words and a test over there
  would pass whether or not `Verbs` is complete.
- `TestRejoiningRestoresTheSeparatorItCutOn`, `TestACommaBeforeThenIsPartOfTheCut` — every
  assertion is that characters the person typed come back unchanged.

Mutation gate: the union, the verbatim rejoin, the comma separators, `wholePhraseVerbs` and the
replace forms' contribution to `Verbs` were each removed; all five killed a test. The first run
of `wholePhraseVerbs` **survived** — every word in it was already in the splitter's hand list,
so it was doing no work and the test that looked at it passed by another route. It was given a
claim of its own (`Verbs` lists every word the planner reads, checked against the phrases
`Parse` accepts) rather than deleted, because a vocabulary that is nearly complete is worse than
no vocabulary: the caller stops checking.
