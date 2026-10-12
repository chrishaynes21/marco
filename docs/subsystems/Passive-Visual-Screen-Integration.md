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
3. ~~The live cadence is not the replay's.~~ **MEASURED, and it is worse than a tuning
   problem.** [[ADR-175-no-settling-rule-survives-a-four-second-cadence]] swept every rate at
   every phase: at 3.8 seconds the current rule settles on **0 to 3** of nine screens while
   **sampling 5 to 6** of them, with a confirmation latency of 11.4 seconds at the best phase
   and nothing at all at one phase in thirty-eight. The screens were seen and the rule discarded
   them.
   Lowering the rule to one look fixes the live cadence (7 of 9 matched) and produces 113
   promotions over 900 looks at the recording's own rate, 72 of them with an unknown cause. And
   click attribution degrades faster than the screens do: 13/5/3 click/automatic/unknown at
   10fps becomes 4/2/11 at 300ms and 0/0/12 at 3.8s. **A coarse observer loses the ability to
   say why anything changed**, which is the half the product needs.
   So this is not a precondition that tuning clears. The slow cadence cannot support this model,
   and the plan below is built on that.

## The cost, measured, and what it licenses

The pixel proposer cost **39ms per frame** over the 900-frame pass — about **26 looks per second
on one core**. The expensive AI interpretation runs at ~3.8 seconds because it is expensive.

**The two cadences are tied together by the wiring and not by the cost.** That is the whole
architecture finding: a cheap local observer can run two orders of magnitude faster than the
thing it currently shares a clock with, and it does not need the AI to do its job.

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

### The screen model gets its own cadence, and it is not the AI's

This is the part the measurement changed. The original plan was to fold the arrangement in the
existing sampler's cycle. That cycle runs at the AI's rate, and at that rate this model does not
work — so the plan is now the opposite:

- a **fast local observer** folds arrangements from the window's pixels at its own interval,
  target 10fps, which the 39ms measurement says costs about 40% of one core;
- it calls the proposer and the stabiliser and **nothing else** — no accessibility, no detector,
  no fusion, no AI;
- the AI interpretation path keeps its own cadence and is unchanged;
- the two meet where they already do: the slow path's conclusions are what identify a PLACE, and
  the fast path's are what notice that the screen moved.

That division is already the architecture's stated intent — *fast local perception handles
changes; AI interprets them when needed* — and it is the first time a measurement has required
it rather than merely allowed it.

`SettleAfter` exists for this: the settle count becomes a function of the observer's own
interval, so the damping is a DURATION and a replay at 10fps and a live observer at 10fps damp
identically. It must not be derived from the AI's interval, because that is the number the
measurement rules out.

### Where the fast observer gets its pixels is an open question

It must not be a second capture path. The existing capture is `screen`-backed, occlusion-unsafe
and costs ~130ms a frame on this machine for a 1920x1080 window — which alone caps a fast
observer at about 7fps and makes the 39ms proposer the cheaper half. Whether a fast observer can
share the sampler's capture, capture at reduced resolution, or needs something else is **not
answered here and should be measured before it is designed**, offline, against a recording.

### Nothing new is captured

No frames, no extra window queries, no second detector pass. The regions it needs are the ones
the sampler already has. If this plan ever needs a capture it does not already have, it has
stopped being passive and needs its own authorisation.

## What it would be for

One thing: finding out whether the nine-screens-for-five problem is a property of the recording
or of the layer. A live session over a different application, reported and never acted on, says
that in a way no further analysis of one Rocket League recording can.

And the owner's own framing of the priority, which this plan now follows rather than resists:

> I wouldn't spend the next several iterations trying to force nine visual hypotheses into
> exactly five screens. […] Instead, prioritize the observation architecture.

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
