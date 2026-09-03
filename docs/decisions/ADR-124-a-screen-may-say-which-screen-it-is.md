---
type: decision
status: accepted
date: 2026-09-03
affects:
  - passive-observation
source_paths:
  - internal/director/observe/screenstate.go
---

# ADR-124 — a screen may say which screen it is

## Context

A clean, correctly-armed dogfood walked `Home → Bluetooth & devices → Mouse → System → Home` at
normal speed. Two Places came out of it, one of them unnamed. The instrument built to find out why
reported:

```
applicationframehost state_1  ×11 read
    agreeing 1 of 11, visits 1, settled false, read over 24263ms
```

**Eleven readings over twenty-four seconds, entered once.** The whole four-page walk was absorbed
into ONE session state whose shape differed on every reading, so it never settled and none of the
four pages became a Place. Settlement was doing its job; it was being asked about a state that
should never have been one state.

The control, on the same application sitting idle: `agreeing 15 of 15, 1 distinct shape, settles`.

### One rule, two failures

Both directions reproduce from `ScreenSegmenter.Observe` alone:

```
4 Settings-shaped pages × 2 readings          →  1 state     false collapse
1 page whose content churns kind × 8 readings →  3 states    false split
```

The second is the live `MARCO ×3` defect: three durable Chrome Places sharing one uninformative
name, carrying fifty-five affordances between them.

`localChange` asks `replacedMass`, which counts **structures whose KIND arrived or left**. Its own
note states the intent — *"a list loading another page changes forty structures and remains
entirely itself"* — and that is exactly right for a list and exactly wrong for a page navigation,
because a page navigation IS a list loading another page. Structure cannot tell *"the content of
this page changed"* from *"this is a different page"*: at the level of kinds and cells they are
identical.

### The evidence was already at the decision point

`sem SemanticEvidence` is a parameter of `Observe`. It carries the admitted destination claim —
what the screen says it is, from its own selected navigation, through `AdmittedPlaceName` — and it
was read only AFTER the state had been chosen, to be tallied against it. Every reading of that walk
carried the right word and none of it reached the decision.

## Decision

**Transient segmentation may use current semantic evidence to decide whether the screen changed,
without that evidence becoming part of durable Place identity.**

That sentence is the layer this system was missing, between raw frame composition and permanent
identity, and it rests on an asymmetry of consequence:

| | wrong answer costs |
|---|---|
| session segmentation | one session's grouping, discarded when watching stops |
| durable Place identity | a permanent memory that outlives every session that could contradict it |

Identity stays conservative because a false merge poisons memory. Continuity only has to answer
*"did we likely cross a boundary just now"*, and can afford richer current-only evidence.

### Both directions, because either alone is half a fix

The claim participates in **choosing** the state, not only in vetoing one:

- A state whose most-tallied word equals the claim is preferred as the match, and the reading folds
  into it. Content churning kind is what a page does; it is not the page becoming another page.
- Otherwise, if the structurally-best state's **settled** word disagrees with the claim, that is a
  boundary. No amount of structural similarity outweighs the screen saying it is somewhere else.
- Otherwise the existing structural rule decides, unchanged.

The asymmetry between *most-tallied* and *settled* is deliberate. Agreeing makes FEWER states and
can only fold a reading into a screen that already exists, so one agreeing word is safe.
Disagreeing MINTS, so it needs the settled word — a transition frame carries the name of the page
being LEFT, and one sighting of it must not become a screen.

### What a first attempt got wrong

Forcing a boundary whenever the claim disagreed with the *structural* best match kept minting a new
state on every visit to a page whose composition matched a different one — the false split,
reintroduced by the fix for the false collapse:

```
state_unknown, state_2, state_unknown, state_3      four readings of one screen
```

Found by this change's own fixture before it ever ran live. It is why the claim chooses rather than
merely vetoes.

## Consequences

- `NewScreenSignature` is untouched, so what a Place is remembered BY is unchanged.
- Two screens the segmenter now separates may still be ONE durable Place when their composition and
  terms match. That is correct for now and it is the invariant, not an oversight: memory says
  `candidate`, never `different`, and refuses to mint a second record out of a word.
- Interfaces that make no destination claim — a game, a canvas, an editor pane — are segmented
  exactly as before. Absence is a fallback, never a boundary.
- Whether the `MARCO ×3` identity defect survives this is now an open measurement. Some of what
  looked like a durable identity problem may have been garbage entering the durable layer because
  segmentation handed it three bogus transient states.

## Enforced by

- `internal/director/observe` `TestFourPagesWithFourDestinationsAreFourStates`
- `internal/director/observe` `TestOneDestinationSurvivesItsContentChurning`
- `internal/director/observe` `TestTwoIdenticalCompositionsWithDifferentDestinationsAreTwoScreens`
- `internal/director/observe` `TestWithNoDestinationClaimStructureStillDecides`
- `internal/director/observe` `TestADestinationSeenOnceDoesNotDecideAnything`
- `internal/director/observe` `TestADestinationGoingQuietIsNotABoundary`
- `internal/director/observe` `TestADestinationArrivingLateDecidesFromThereOn`
- `internal/director/observe` `TestReflowUnderOneDestinationIsOneScreen`
- `internal/director/observe` `TestSeparatingTwoScreensDoesNotSeparateWhatTheyAreRememberedBy`

## Related

- [[ADR-076-a-place-may-say-what-it-appears-to-be-called]]
- [[ADR-123-a-control-you-can-see-is-worth-knowing-about]]
- [[Experiment-022-the-first-dogfood]]
