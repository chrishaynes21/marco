---
type: adr
status: accepted
date: 2026-10-17
decision: a live pixel demonstration is captured by the loop `director observe-screen` already ran and written as the same named workflow artifact desktop replay uses — four calls, no new format and no new engine; and the screen a control was demonstrated on is NOT supporting context but the discriminator doing almost all the work, so per-control grounding may not be scoped by nothing
supersedes: []
affects:
  - perception
  - execution
source_paths:
  - cmd/director/visualsession.go
  - cmd/director/demonstratevisual.go
  - cmd/director/reacquirecmd.go
  - internal/director/workflowstore/workflowstore.go
  - cmd/perceptionreplay/reacquire.go
---

# ADR-230 — a demonstration from pixels, and what the screen was doing

## What was asked

> Our next milestone is transferring that same workflow artifact to a pixel-only game menu.
> Investigate the smallest way to record a person's clicks, key actions, and relevant
> before/after frames during an existing Director demonstration session.
> Make individual learned controls and transitions the basis for game-menu replay. Use
> whole-screen matching as supporting context, not proof of identity.

Two things: the capture, and the grounding hypothesis. The first is plumbing and it is done. The
second was measured and **the hypothesis is the wrong way round.**

## The capture: four calls

Everything needed existed and nothing joined it up.

```
director observe-screen    the live loop — capture, propose, fold, correlate presses
observe.ClickTargets       what was pressed, as a control rather than a coordinate
workflow.Learn             a Draft from a walk and its click evidence
workflowstore.SaveWith     the durable artifact, with each screen's arrangement
demo.Store.SaveSequence    by the name the person gave it
```

So `director demonstrate-visual "<name>" --application X` is the join. The loop moved to
`visualsession.go` and is shared; `observe-screen` keeps its own observer construction and its
own report, which is what its wiring test reads it for, and that test now reads **both** files so
the four properties it protects — bounded length, a named application, a frame before the
subscription, a consumer on the queue — are still asserted where they now live. The write happens
in the demonstrating command, in the file a reader looking for the thing that writes will open.

Nothing new is stored. A control's identity is its READINGS, as the proposer saw them, and its
containers; a press coordinate is not written, because ADR-181 keeps the correlator out of the
planner's reach and a store that wrote them back would undo that in one field.

## The grounding hypothesis, measured

[[Experiment-077-the-screen-was-not-supporting-context]] ran the same recording, in one pass,
both ways.

| | scoped by the demonstrated screen | no screen at all |
|---|---|---|
| target-looks | 1287 | 6594 |
| matched | 52.4% | 47.7% |
| ambiguous | 12.7% | 9.6% |
| unobservable | 35.0% | 42.7% |
| ground truth, second presses | 3 verified, 0 contradicted, 2 declined | identical |
| **colliding matches** | **0 over 0 looks** | **3323 over 616 looks** |
| **matched on a screen it was never demonstrated on** | — | **2470, or 78.6% of all matches** |

Dropping the screen bought **no extra match rate** — it is slightly worse — and cost 3323 pairs
of controls claiming one rectangle, plus nearly four matches in five made on a screen the control
had never been seen on. A replay looking for one control at a time, with nothing scoping it,
would have been prepared to press something that is not there on 2470 looks.

### Why, and it is not a surprise once stated

A game menu is a column of near-identical rectangles. `sameRect` is mutual centre containment —
threshold-free and robust, and deliberately so — and it cannot distinguish the third row of one
menu from the third row of another, because geometrically they are the same rectangle. The screen
was never supporting context. **It was the discriminator, and the control match was the weaker
half.**

## So the decision is the narrower one

**Per-control grounding is admitted, and it may not be scoped by nothing.** Concretely:

1. `Artifact.VisualTargets()` exists and returns targets whose `Screen` is **empty**, so a caller
   can ask the screen-independent question — and so nothing downstream can mistake the weaker
   claim for the stronger one. `Reacquisition.Screen` comes back empty too.
2. Nothing acts on it. `director reacquire "<name>"` measures and presses nothing; `Actionable()`
   still says *could*, and the authorisation is somebody else's to give.
3. The context a replay should use is **already designed and never measured**:
   [[ADR-186-a-person-can-say-where-they-are]] records the rule as *"expectation supplies
   presence; similarity supplies agreement — neither alone is admitted: without the transition
   this is Experiment-029's two wrong bindings; without the agreement it is arriving somewhere
   and calling it arriving where you meant."*

   Experiment 076 measured similarity alone and it fails. Experiment 077 measured the control
   alone and it fails worse. **The conjunction is what the design says to use and it is the one
   thing nobody has measured.** A replay at step three expects the screen step two led to — which
   comes from the saved order, not from a global classifier — and that is the next experiment
   rather than the next rule.

## What the live session proved, and the one thing it cannot

Measured today, on Settings, through the production command:

```
director demonstrate-visual "settings check" --window-title Settings --seconds 15 --dry-run
  1 screen(s) seen, 1 visited in order, 0 demonstrated control(s)
  presses: 0 of 0 classified
  nothing was demonstrated — no press landed on a settled screen, so there is no step to save
```

The loop runs, the screen settles, and a session that learned nothing says so and writes nothing.

**It cannot demonstrate to itself, and that is by design.** `navsource` discards injected presses
at the callback as not coming from a hand, so `PointerInjected` counts them and `Received` never
does. Marco driving its own clicks produces a session with zero attributed presses. A pixel
demonstration therefore needs the person's hands on the mouse — which is
[[ADR-029-resolution-is-not-permission]]'s boundary showing up as a product requirement,
and the right trade rather than a gap.

One selector lesson from the same run: `--application notepad` and `--application
applicationframehost` are both refused as ambiguous on a real desktop, correctly, so both new
commands take `--window-title` as well — exactly one of the two, because `Selector.Validate`
requires exactly one and resolving two by precedence would be this layer deciding which half of a
person's sentence to believe.

## Enforced by

`cmd/director` `TestTheScreenObserverIsReachableAndBounded` (now over both files),
`TestAPixelDemonstrationIsReachableAndWritesOnlyWhereItShould`,
`TestASavedPixelDemonstrationCarriesNoCoordinates`; `internal/director/workflowstore`
`TestVisualTargetsCarryNoScreen`, `TestVisualTargetsSkipAControlWithNoVisualEvidence`;
`cmd/perceptionreplay` `TestTheScreenFreeTallyIsReportedBesideTheScopedOne`.
