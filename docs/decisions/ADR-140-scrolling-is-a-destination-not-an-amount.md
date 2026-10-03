---
type: decision
status: accepted
date: 2026-10-03
affects:
  - semantic-actions
  - marco-boundary
  - execution
source_paths:
  - pkg/directorapi/semantic.go
  - internal/director/uiact/ladder.go
  - internal/uiamod/accessibility.marco
  - plugins/uia/Uia.cs
  - internal/director/verify/scroll.go
---

# ADR-140 — scrolling is a destination, not an amount

## Context

Earned, not scheduled. Using Windows Notepad as an unfamiliar application, a 70-line document
made navigation necessary and the obvious requests were impossible:

```
scroll to the top              nothing matching "top" is present in the observed window
scroll up                      I only understand click, focus, move, repeat and text editing
go to the top of the document  (the same)
```

`scroll_here` existed and is a different verb: it takes a CONTROL and brings it into whatever view
holds it. There was no way to move a view.

Directional scrolling had been considered and declined, in a comment that is still right about
what it declined:

> One rung and a refusal. Wheel notches would scroll SOMETHING by an amount nobody chose, and
> "scroll to the item" is not satisfied by "the view moved".

Both objections are about **amounts**. A destination answers them: `ScrollPattern.SetScrollPercent`
lands on a boundary, `ScrollVertical(LargeIncrement)` moves by the APPLICATION's own page, and
`VerticalScrollPercent` reports where the view ended up — so there is something to verify against.

## Decision

**One semantic verb, `scroll`, carrying a typed destination.** Four meanings and no fifth:

```
top         absolute vertical destination = top
bottom      absolute vertical destination = bottom
page_up     relative, one application-defined page
page_down   relative, one application-defined page
```

`ScrollWhere` is one enum field rather than a direction beside a destination, so "top AND one page
down" is not expressible and needs no rule forbidding it. The executor switches on the constants
and never reads a word the user typed: "top" is not a control name and "down" is not a control
name, and letting either become one is how a magic string gets into a mechanism decision.

**UIA `ScrollPattern` only.** No wheel, no PageUp/PageDown chord, no coordinate scrolling, no
scrollbar drag. Each is a different mechanism with its own targeting and its own verification
question, and a region exposing no scrolling capability is refused honestly instead.

**Legal Marco.** The Accessibility surface gained one export and the `Control` set one field
(`Where`). ADR-005 is preserved: the Director lowers to Marco source, Marco's own compiler
validates it, and UIA is reached only by the host.

```
the ctl is a Control with Element "42.7.3", Where "top", MaxNodes 1500.
do Accessibility's Scroll with ctl.
```

**Verification is semantic.** The host reporting that `SetScrollPercent` returned is DELIVERY. The
verdict compares the position the world reports, before against after:

```
top         the position IS the top boundary
bottom      the position IS the bottom boundary
page_down   the position INCREASED, or was already at the bottom
page_up     the position DECREASED, or was already at the top
```

Already-at-the-boundary succeeds and says so. Somebody at the bottom who asks to go down has what
they asked for, and `failed` is the word that would invite them to ask again. A position nobody
can read is INCONCLUSIVE and carries delivery evidence, so the retry guard will not repeat it —
repeating a delivered page scroll moves two pages.

## Grounding cost four things nobody predicted

"Scroll down" names no control, and the words in it are not labels. The query carries a capability
(`Scrollable`) and a preference (`PreferFocused`) instead of a name — and four separate gates had
to be opened, each measured shut first:

| gate | what it said |
|---|---|
| `Constrained()` | a capability query "did not describe anything specific enough to look for" |
| `Any()` | listed four affordances, not scrolling, so a scrollable container afforded nothing |
| `inertPenalty` | a list is scenery BY ROLE — you operate its items — so it fell under `MinScore` and vanished with no rejected candidate to explain it |
| focus | a FILTER refused the fixture, whose focus starts somewhere that does not scroll |

The last was settled by two surfaces disagreeing. Notepad's editor both scrolls and holds focus;
the WinForms fixture has one scrollable list and focus elsewhere. Filtering on focus refused the
second; ignoring focus would pick arbitrarily among the first's three scrollable regions.
Preferring it answers both, and leaves a genuine tie — several regions, none focused — to the
resolver's ambiguity path, which asks.

The `inertPenalty` exemption is narrow: it applies only when the query ASKED for the capability the
element has. A scrollable pane that merely matches somebody's words is still scenery for a click.

## One verb withheld from the model

The interpreter answers `{kind, target, ordinal, deictic}`. `scroll` needs a destination and there
is nowhere to put one, so the moment it joined the vocabulary the model began choosing it for
anything starting with the word — and every such reading arrived unperformable:

```
"scroll the Save button into view"  →  scroll, no destination  →  refused
```

Before `scroll` existed the model read that phrase as `scroll_here` with the target "Save", which
is right. One unofferable verb cost a performable one. `SemanticActionKind.Interpretable()` now
withholds it: **offer the interpreter a verb only if the answer it may give can express that verb
completely.** Widening the protocol with a destination field was the alternative and was not
taken — the deterministic table covers the phrasings people use, and growing a wire contract so a
3B model can guess among four direction words is cost without a demonstrated benefit.

## What this cost two existing tests

Both used "scroll down" as their stand-in for "unsupported", and the capability landing
invalidated their EXAMPLE rather than their claim. They now use a bare pointer move, which is
unsupported as a program step by decision rather than by omission — an example chosen from the
not-yet-built pile expires the moment somebody builds it, which is how both broke on one commit.

A desktop acceptance case asserting `scroll down` could not be performed was deleted. Its comment
had said "it does NOT add a verb to make itself pass", and that held: the verb arrived by
measurement, and the case became a test asserting the absence of a feature that exists.

## Measured limitation, recorded and not fixed

A second scroll in one session sometimes refuses with *"the target could not be found again: 2
controls match that about equally well"*. It comes from the executor's RE-resolution: the plan
carries the element id and the query, `resolveRef` prefers the id, and when the id stops resolving
it falls back to the query — which matches more than one thing in the fused world, even though the
accessibility host reports exactly one scrollable region before and after. Diagnosed to that
boundary and no further. Intermittent; the acceptance case records it and still asserts the part
that must not bend — the refusal is not a false success, and the view did not move the wrong way.

## Enforced by

- `TestEveryOperationLowersToLegalMarco` — all four destinations compile through Marco's real
  compiler. Removing the export from `accessibility.marco` kills it with a compiler error.
- `TestScrollVerificationAsksWhereTheViewEndedUp` — twelve cases over the four meanings.
- `TestAnAlreadyAtTheBoundaryScrollSaysSo`, `TestAnUnreadablePositionVerifiesNothingAndIsNotRetried`
  — the boundary case explains itself; an unreadable position is inconclusive and carries delivery
  evidence.
- `TestAScrollFindsTheOnlyScrollableRegion`, `TestAScrollPrefersTheRegionYouAreIn`,
  `TestSeveralScrollableRegionsAndNoneFocusedIsNotGuessed`,
  `TestANonScrollableControlIsNotAScrollTarget`,
  `TestTheScrollCapabilityIsNotTheScrollItemCapability` — the grounding, including the one-letter
  trap between `scroll` and `scrollitem` (2 elements against 346 on one snapshot).
- `TestTheInterpreterIsOnlyOfferedVerbsItCanExpress` — the withheld verb, and that nothing else
  was withheld with it.
- `TestDesktopScrolling` — the real desktop, two channels: the accessibility scroll percentage and
  the ListBox's own `TopIndex`, written by the fixture because WinForms raises no event when a list
  scrolls and a state file written only on other controls' events reported the view where it used
  to be.
