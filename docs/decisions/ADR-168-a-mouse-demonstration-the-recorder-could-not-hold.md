---
type: decision
status: accepted
date: 2026-10-12
affects:
  - passive-observation
  - navigation
  - perception
source_paths:
  - cmd/director/capturegamesession.go
  - cmd/perceptionreplay/frames.go
  - internal/platform/navsource/navsource.go
---

# ADR-168 — a mouse demonstration the recorder could not hold

## Context

The owner changed the product direction for the first usable version, and then told me to check
one thing before recording anything:

> For the initial Marco MVP, prioritize visual click-target navigation over inferred controller
> or directional menu navigation. Marco should identify visible interactive targets, learn which
> clicked targets produce which screen transitions, and later resolve and click those targets
> using current observations.
>
> During the authorized supervised recording, capture mouse movement and click events with
> timestamps, coordinates relative to the captured window, window/foreground provenance, and the
> before/after visual evidence. **Verify that the existing recorder actually supports mouse input
> capture before starting; add the minimum required passive instrumentation if it does not.**
>
> Do not treat recorded click coordinates as permanent execution targets.

It does not support it. This records what was missing, what was added, and what the measurements
say — which is less than the direction needs.

## The defect, in three parts

### 1. Every mouse press was discarded, whole

`navsource` places a press by normalising it against the watched window's rectangle, and refuses
any press it cannot place: `windowBounds.fresh` is false until somebody pushes a rectangle in.
`director capture-game-session` opened a navigation subscription and **never pushed one**. So
every mouse press in every session this command has ever recorded was counted
`unplaceable_pointer` and dropped — not stripped of its position, dropped entirely — and the
recording reported `0 inputs`, which is what a session nobody played also reports.

The identical defect was found and fixed once already, at the live sampler. Its comment in
`cmd/director/observewiring.go` says what it cost:

> Vision is off by default […] the producer therefore never received a frame,
> `windowBounds.fresh` was false forever, and every pointer press on a default Director was
> refused as `unplaceable_pointer` — counted, and gone. A live Learn against Windows Settings
> reported exactly that: `received=2 classified=0 unplaceable_pointer=2`, for the two clicks the
> person made.

Two sites, one rule. This one stayed broken because every session so far was played on a
controller, where placement is not needed at all.

### 2. The position was dropped on the way to disk

`observe.InputEvent` has carried `Where` since the mouse half existed. The writer took `Intent`
and `AtMS` off it and left the rest, so a press that *had* survived placement would have reached
disk as `point` with no position — a record that somebody clicked and no record of what they
clicked, which is the entire content of a mouse demonstration.

The same field was dropped again on the read side: `cmd/perceptionreplay`'s `inputRecord` also
took `Intent` and `AtMS` only, and handed production an `InputEvent` with a zero position.

### 3. An input that said Marco made it

The tail that drains the controller after the last frame wrote `gameInput{Intent, AtMS}` — no
device, and `observed` false by omission. In this format `observed` is the field that keeps a
demonstration distinguishable from an actuation *in the record itself*, so those events stated
that Marco produced them. Nothing in this command can inject.

## Decision

### The rectangle pushed is the CAPTURE'S, not the reference's

The live path pushes the validated window reference's bounds, which is right there: everything
it correlates is scoped to that reference. Here the thing a press will be compared against is
geometry read from *these pixels*, and the pixels are of `img.Bounds`, which can differ from the
window rectangle by a frame or a shadow. A press expressed as a fraction of one rectangle and a
row expressed as a fraction of the other do not meet, and the error is a few percent of a
screen — enough to pick the neighbouring row, which is exactly the mistake that must not look
like a correct answer.

It is stamped with `img.CapturedAt` for the same reason the live path stamps `now`: a frame newer
than the press says nothing about the press, and `windowBounds.fresh` refuses one. Capture costs
about 130ms here, so a press made *during* the grab is still placeable against it.

### The position is demonstration evidence and never an execution target

Written into the type that carries it, because the distinction is the whole product decision:

> A route that clicked this position again would be replaying a coordinate, and a menu that
> moved — a different resolution, a patch, a scrollbar — would take it somewhere else with no way
> to notice. What the position is FOR is asking which row of a list the person pressed, against
> the geometry perception found in that same frame.

This is the same line `observe.SemanticTarget` already draws one layer up, and that type's own
comment states it better: *a click at (0.42, 0.55) is evidence that the person clicked somewhere;
a click on the list item called "Mouse" is evidence of what they MEANT.*

### The recording says which silence it is

`session.json` now carries a `pointer` note, and the three outcomes are three different problems:
presses recorded; presses **refused** for want of a placeable frame, which is a Marco defect and
must never read as a quiet session; and nothing at all, which is a fact about the person, who may
have used a controller.

### The replay counts what a press landed on, and does not decide what it was

`scorePresses` tallies, per frame: presses seen, presses inside at least one region the proposer
found, and presses inside more than one. A press inside a proposed region can be re-grounded
later by finding that region again; a press on bare background can only ever be replayed as a
coordinate. The live pointer path resolves a press to the smallest containing control and that
rule lives there — a press under several regions is counted as *needing* that choice rather than
having it made twice in two places.

Read from the events handed to the slot rather than from the records they were parsed from,
because the slot is what production receives: a press whose position failed to reach the event is
a press the interface watch could not correlate against anything either.

## Measured outcome

Nothing about the Trackmania corpus changes: it was recorded on a controller, has no pointer
presses, and the three new tallies are silent on it. The perception numbers at strides 1, 3, 10
and 20 are unchanged from [[ADR-167-a-slow-gate-and-two-rules-that-measured-worse]], and the
corpus gate still passes.

What is measured is the path itself, on drawn fixtures and through the real producer:

| claim | how it is measured |
|---|---|
| a press with no window frame is refused | the synthetic pointer source, in the state this command shipped in: `ignored[unplaceable_pointer] = 1`, nothing on the subscription |
| the same press with a frame is recorded at its position | window (100,200) 400x300, press at (300,350) → `where = 0.5, 0.5` |
| the position survives the recording and reaches perception | an L0 recording written in the writer's layout, replayed: two presses, one on a drawn row and one on background, scored 1 on a region |
| a press on nothing is not scored as a press on something | the same fixture's background press |

## Consequences

- **The mouse half of a recording exists now, and has never been exercised live.** Every number
  above is a drawn fixture or a synthetic producer. No session has been recorded with it.
- **A click is not yet associated with a named target in a game.** `SemanticTarget` resolution
  reads an offered control index, which the live path fills from accessibility. A game has no
  accessibility tree, so in Rocket League the honest answer will be `no controls were on offer`,
  and the grounding has to come from the geometry the replay finds in the same frame. That join
  is counted here and is not yet made.
- **Screen segmentation still cannot distinguish two game screens**, which is the half the new
  direction needs most: `StructuralView.Regions` comes from named detections and the structure
  proposer names nothing, so on the pixels discovery path the screen model is one vacuous state.
  "Which clicked target produced which screen transition" cannot be answered while every screen
  is the same state. Unchanged by this decision, recorded in Passive-Observation's known gaps,
  and the next thing in the way.
- Nothing here injects, clicks, or replays anything. The recorder still cannot actuate and the
  controller reader still does not bind the call that could.

## Mutation gate

21 mutants, all 21 killed. One guard was deleted rather than excused: `shapesUnder` tested a
region for zero area, and for zero width `at.X >= r.X+r.Width` is the negation of the test beside
it — so the half-open comparison already excluded every area-less region and the check was dead
code. The property is asserted in the test, where it belongs.

## Enforced by

- `cmd/director` — `TestAGameRecordingGivesThePointerAFrame`,
  `TestTheMouseNoteTellsAQuietSessionFromADiscardedOne`,
  `TestEveryRecordedInputSaysItWasObserved`, `TestTheSessionSaysWhatTheMouseHalfIsWorth`.
- `cmd/perceptionreplay` — `TestTheInputRecordReadsTheWritersTags`,
  `TestAPointerPressReachesPerceptionWithItsPosition`, `TestEachPressIsCountedForWhatItLandedOn`,
  `TestAPressInsideTwoRegionsIsCountedAsAmbiguous`.
