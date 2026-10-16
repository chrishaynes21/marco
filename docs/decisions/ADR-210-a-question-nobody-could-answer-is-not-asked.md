---
type: decision
status: accepted
date: 2026-10-16
deciders: director, owner
affects:
  - perception
  - execution
source_paths:
  - internal/director/surface/question.go
  - internal/director/surface/surface.go
---

# ADR-210 — a question nobody could answer is not asked

## Context

[[Experiment-063-the-configuration-that-cannot-close-its-own-loop]] put the surface question to a
live accessibility tree for the first time. On eight pages of Windows Settings, every tie it found
was between a control and its own wrapper:

```
  e49 {908 54 98 38} press {957 73}   the words' own region   DEFENSIBLE
  e50 {908 54 98 38} press {957 73}   a surface around them   DEFENSIBLE
```

Identical bounds, identical press point. On the Personalisation panel, three options of which two
differed by two pixels. The evaluator is right that it cannot choose — but
[[ADR-206-a-demonstrated-surface-is-one-fact-about-one-target]]'s press rule settles a question
only when exactly one option contains the press, so **no press could ever answer these**. Marco
would ask somebody to click a place that does not exist, and their correct press would come back
as `undiscriminating`, which reads as their mistake.

The owner's decision:

> Marco should never ask someone to distinguish two regions if no possible click could distinguish
> them. However, the test should be slightly stronger than checking whether the rectangles are
> identical. It should determine whether there is an actual usable point inside exactly one
> candidate. … Use existing geometry and pointer-safety rules where possible. Don't introduce
> arbitrary thresholds to make Settings pass.

## Decision

**A question is offered only when some place exists that a press could use to single out exactly
one option.** `Hypothesis.Question()` refuses otherwise, and the evaluator says why.

### What counts as a usable place

The first attempt asked whether any option's own middle fell inside exactly one option, on the
reasoning that those are the only points this system would ever press itself. A fixture refuted
it: two peer surfaces overlapping in the middle are told apart by a press near either outer edge,
and neither middle is in exactly one of them. **A person presses where they like**, so the
question is whether a usable place exists, not whether Marco would have nominated it.

So each surface is clipped by every other, and what survives is the part of it the others leave
clear.

### The minimum comes from the screen, not from a constant

The only pointer-safety rule in force anywhere in this tree is `ClickableByBounds`:
`!Bounds.Empty() && Visible && !Offscreen` — a non-empty rectangle is somewhere real to aim.
That alone admits the two-pixel margin the owner named as unusable.
`structure.Options.MinHeightFraction` documents a human-scale rule at 0.010 of frame height, but
it defaults to zero and is set nowhere, so leaning on it would have been pretending.

**The clear area must therefore be at least as large, in both directions, as the smallest surface
this question is already willing to press.** If a 24×24 icon is a legitimate target on this
screen, a 24×24 clear area is equally pointable; if the clear area cannot hold the smallest thing
on the menu, it is not a target. No absolute size appears anywhere, which is the property that
matters — a threshold chosen to make one application pass is a threshold that fails the next one
silently.

### Conservative, deliberately

The clip is greedy: each surface keeps the largest of the four rectangles that survive each cut.
A full plane sweep would find areas this misses. Refusing a question that was barely answerable
costs an instruction; asking one that was not costs somebody's trust in the request.

### And nobody is blamed

The ambiguity is still real and still reported as ambiguous — nothing chose, and saying otherwise
would be worse than saying nothing. What the explanation adds is why no demonstration can settle
it:

> No press could tell them apart: the part of each surface the others leave clear is smaller than
> the smallest surface here, so there is nowhere anybody could click that would single one out.
> That is a demonstration which cannot exist rather than one somebody failed to give, and the
> answer is better evidence about the surfaces, not a clearer instruction.

## Consequences

**The admission rule and the press rule are now one rule.** `covers` is half-open and shared, so
the gate that decides whether to ask and the judgement of a real press cannot disagree about what
being inside means.

**Fewer questions, all of them answerable.** On Windows Settings under production observation,
that is every tie on eight pages: the feature is not exercisable there without pixel evidence
beside the tree, which is what [[ADR-211-production-observation-with-the-pixels-added]] is for.

**A refused surface is never on the menu.** Already true, now held: admitting the full-window
container the live Stage also reported would let a demonstration settle on the whole window, which
is the press ADR-205's band rule exists to refuse — and it would look like a successful answer.

## Enforced by

`internal/director/surface` — `TestAPressCouldTellThemApart`,
`TestAQuestionNoPressCouldAnswerIsNotAsked`,
`TestAnUnanswerableAmbiguitySaysWhyRatherThanAsking`, `TestContainmentIsHalfOpen`. Every rectangle
in those fixtures is measured, from a live screen or a recorded one. 14 of 14 mutants killed.

## Related

- [[ADR-206-a-demonstrated-surface-is-one-fact-about-one-target]] — the press rule this completes.
- [[ADR-205-a-surface-hypothesis-is-request-scoped-and-refuses-to-guess]] — why a point is never invented or nudged.
- [[Experiment-063-the-configuration-that-cannot-close-its-own-loop]] — where the unanswerable
  questions were measured.
