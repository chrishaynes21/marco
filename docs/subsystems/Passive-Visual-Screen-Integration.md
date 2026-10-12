---
type: subsystem
status: proposed
owners:
  - director
depends_on:
  - passive-observation
  - perception
used_by: []
updated: 2026-10-12
source_paths:
  - internal/director/observe/arrangement.go
  - internal/director/observe/clicktarget.go
  - cmd/director/observewiring.go
---

# Passive visual screen integration — a plan, not a wiring

**Status: PROPOSED. Nothing in this note is built.** The owner asked for it as the fourth item of
a brief whose first three are done:

> Once the offline representation is stable, connect session-local screen hypotheses to
> Director's live observation path **in passive mode only**. Do not enable execution or durable
> Place promotion.

This is that plan, with the conditions it should not be started under stated first.

## What exists, and where it runs

| | |
|---|---|
| `observe.VisualScreens` | stabilises arrangements into session-local screens. [[ADR-171-an-arrangement-is-an-observation-and-a-screen-is-an-interpretation]] |
| `observe.ClickTargets` | correlates presses with changes, three causes. [[ADR-172-a-click-transition-is-a-source-a-target-and-a-destination]], [[ADR-174-three-kinds-of-transition-and-what-a-brief-screen-was]] |
| both of them | called **only** from `cmd/perceptionreplay`, over a recording |

Nothing in `internal/director/observesession` or `cmd/director` touches either. That is the
boundary this plan would cross, and it is the only one it may.

## The three preconditions

Measured on `rl-freeplay-1`, and none of them is met yet.

1. **The screen count is unexplained.** Nine settled screens for the four or five a person would
   name. The proposal-yield hypothesis is measured and refuted
   ([[ADR-173-the-proposers-yield-is-not-what-separates-two-screens]]), and nothing replaces it.
   Wiring a layer that over-segments by a factor of two into the live path would put that factor
   into every live session's diagnostics before anybody knows what causes it.
2. **One recording.** Every number rests on 900 frames of one game recorded once. A second
   recording of a different application is worth more than any amount of further analysis of this
   one.
3. **The live cadence is not the replay's.** The stabiliser counts LOOKS, and
   `VisualSettleLooks = 3` was chosen against a 10fps recording — 300ms. The live observation
   path samples on a ~3.8-second discovery cadence
   ([[Experiment-008-unknown-game-discovery]]), where three looks is **eleven seconds**. On that
   clock the damping would discard every real navigation. This is the precondition that matters
   most and the one that is easiest to miss, because the code would run and produce plausible
   nonsense.

## The shape it should take

### It observes and it is asked; it never pushes

The same shape as the interface belief: folded on the sampler's own goroutine, exposed as a
snapshot somebody can ask for, and reaching nothing. Specifically:

- fold a look's regions in `shadowSampleFor`'s existing path, beside where the interface watch is
  already folded, from the SAME regions — not a second capture and not a second normalisation;
- expose `Session().VisualScreen` and the step evidence on the session snapshot, as diagnostics;
- do not let it reach `EstablishPlace`, `StructureSignature`, the relationship layer, or any
  hypothesis that crosses the durable boundary.

The last point is already enforced by `TestAVisualScreenCannotReachDurableMemory`, which holds
the visual names to four files inside `internal/`. **Wiring this means editing that test**, and
the edit should add exactly one file — the composition root — and nothing else. If the diff wants
more, the design is wrong.

### The settle rule has to take the cadence, not assume it

`VisualSettleLooks` becomes a parameter of the session rather than a constant, derived from the
sampler's interval: the damping is about a duration (a change that survives ~300ms of looking)
and looks are how that duration is counted. A replay at 10fps and a live session at 3.8s must
damp the same amount of TIME, or the measurement and production are two different things —
which is exactly the drift this repository's worst defects are made of.

That is a change to `arrangement.go` and it should be made and measured **offline, against the
existing recording at several strides**, before anything live sees it.

### Nothing new is captured

No frames, no extra window queries, no second detector pass. The regions it needs are the ones
the sampler already has. If this plan ever needs a capture it does not already have, it has
stopped being passive and needs its own authorisation.

## What it would be for

One thing: finding out whether the nine-screens-for-five problem is a property of the recording
or of the layer. A live session over a different application, reported and never acted on, says
that in a way no further analysis of one Rocket League recording can.

## What it must not do

- No durable Place from an arrangement. Unchanged, and the reason is unchanged: a screen state is
  a composition of roles a provider vouched for and this is a count of rectangles in nine boxes.
- No execution of any kind. A click-caused step is evidence that a press was followed by a
  change; it is not a plan, and the target would have to be re-grounded on the screen in front of
  Marco before anything could act on it.
- No new live recording and no actuation without separate authorisation.

## Related

- [[Passive-Observation]] — where all of this lives and what it may not reach
- [[Experiment-027-what-did-you-click-and-where-did-it-take-you]] — the recording every number
  comes from
