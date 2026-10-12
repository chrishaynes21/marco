---
type: decision
status: accepted
date: 2026-10-12
affects:
  - passive-observation
  - perception
  - navigation
source_paths:
  - internal/director/observe/arrangement.go
  - cmd/perceptionreplay/screens.go
  - cmd/perceptionreplay/frames.go
---

# ADR-171 — an arrangement is an observation and a screen is an interpretation

## Context

[[ADR-170-what-an-unnamed-arrangement-would-have-segmented]] measured, on 900 frames of a real
mouse demonstration, that unnamed geometry can see seven distinguishable screens where Marco's
own model saw one — and that it changes **150 times between adjacent frames**. One spurious
transition every six frames is not a weaker route; it is a transition storm, and a transition
storm looks like evidence.

The owner read that result and drew the distinction the rest of this work needs:

> I'd introduce a distinction between three kinds of identity.
>
> 1. **Visual arrangement.** A frame-local observation of regions and their spatial
>    relationships. It can change with animations, hover effects, and detector noise.
> 2. **Session-local screen hypothesis.** A temporally stabilized interpretation that multiple
>    frames represent the same interface state. *This is what I'd build next.*
> 3. **Durable graph Place.** A persistent identity backed by stronger evidence, suitable for
>    later route planning and verification.
>
> Raw rectangle arrangements should not automatically become durable Places.

## Decision

`internal/director/observe/arrangement.go` builds the middle layer. An arrangement is an
observation; a screen is an interpretation; the gap between them is **time**.

### A change has to hold before it is a screen

Three consecutive looks — 300ms at the rate the recording ran at. Each look of a run must agree
with the arrangement that **started** the run rather than merely differ from the standing
screen, because three unrelated frames of noise are not one new screen and noise arrives in runs.

`VisualSettleLooks = 3` is deliberately **not** `StatePromotionCount = 2`. That number governs
how often an ambiguous role composition must recur on a 3.8-second discovery cadence; this
governs consecutive frames at a video rate. Sharing a constant between two clocks would be a
coincidence pretending to be a rule.

### Coming back is the same screen

A promotion searches the screens already settled before minting a new one. This is what makes a
route possible at all: walking back out of a submenu must arrive at the screen it left, or every
demonstration is a chain of new places and no repetition is visible.

A screen's stored arrangement **does not drift**. A representative that moved with every matching
look would let a screen walk away from itself over a long session — the same slow-drift failure a
tracked list has when its identity is recomputed from its current extent.

### A blank look is a missing observation, not a screen

The owner's rule, and it had a measured victim: the raw clustering made a screen out of two
frames holding **zero** regions, because `signatureSimilarity` answers 1 for two empty vectors —
correct for a sparse game screen described by roles, wrong for "the proposer found nothing".
That would be a transition into an empty place and out again, twice per animation.

So `ArrangementSimilarity` answers **0** when either side is unobserved, which is the one place
this departs from the segmenter's own arithmetic, and a blank look leaves the standing claim
alone. If the absence persists past `VisualUnobservableLooks`, the claim is given up and the
screen becomes **unknown** — and that is not a transition either, because an unknown destination
is not a place. The two failures are not symmetric: calling a fade a new screen manufactures
evidence; holding a stale claim through one only delays a correction.

### It must not become durable

A `ScreenStateID` is session-local too, but a screen state is a composition of roles a provider
vouched for. A visual screen is a count of rectangles in nine boxes — a far weaker thing to make
permanent. Type safety cannot hold this, since Go converts between string types on request, so
the guarantee is that these names appear nowhere inside `internal/` but the file that defines
them. An offline replay under `cmd/` may use them freely. The day the live path may is the day
somebody deletes a line of `TestAVisualScreenCannotReachDurableMemory` on purpose.

## Measured outcome

The same 900 frames, stabilised:

| | raw (ADR-170) | stabilised |
|---|---|---|
| distinct screens | 7 | **9** |
| changes | **150** between adjacent frames | **22** promotions, 14 distinct pairs |
| blank frames | their own screen | 2 looks, no screen, no transition |
| presses followed by a different screen | 13 of 17 | 10 of 17 |

Of 900 looks: **767 agreed with the standing screen**, 107 differed and did not become one, 2
were taken before the first screen settled, 2 were blank, 22 were promotions. Every look is in
exactly one category, which is what makes the report arithmetic rather than a selection — and
the warm-up looks are counted apart from the flaps because charging the stabiliser for looks it
could not have interpreted would make the damping look worse than it is.

### Seven transitions happened exactly twice — once per demonstration

```
visual_1 -> visual_2   x2   at looks [80 546]
visual_2 -> visual_3   x2   at looks [114 574]
visual_3 -> visual_4   x2   at looks [168 584]
visual_4 -> visual_1   x2   at looks [180 628]
visual_1 -> visual_6   x2   at looks [220 659]
visual_6 -> visual_7   x2   at looks [311 749]
visual_7 -> visual_8   x2   at looks [341 766]
```

The first look of each pair falls in the first demonstration and the second in the second. They
chain into two walks — `1 → 2 → 3 → 4 → 1` and `1 → 6 → 7 → 8` — and `visual_1` holds 416 of the
900 looks, which is what a menu somebody keeps returning to looks like.

**This is the repeated-route correspondence the milestone asks for**, and nothing in it was
inferred from the input events: the transitions are between settled arrangements, and the
presses are correlated with them afterwards.

### What it costs, and what it still cannot say

- **107 looks differed from the standing screen and were discarded as unsettled.** That is the
  price of the damping, reported rather than hidden: if a real navigation is in there, this is
  where it was lost. Nothing yet says whether any of them was one.
- **Presses followed by a change fell from 13 to 10.** Three of the raw "changes" were flaps.
  That is the measurement getting more honest rather than worse, but it is a reduction and it
  is reported as one.
- **The settle window hides slow transitions from the press report.** The press at frame 138
  reads `visual_3 -> visual_3`, while the transition `visual_3 -> visual_4` is recorded at look
  168 — thirty frames later, outside the ten-frame window the press was asked about. So "the
  screen after a press" and "the next transition after a press" are different questions, and
  only the first is answered. The second is what an attributable click transition needs.
- **Nine screens for four or five real ones.** `visual_5` (8 looks), `visual_9` (2 looks) and
  `visual_8` (15 looks) are short-lived, and the proposal-yield question ADR-170 raised —
  `visual_1` at 236 regions against `visual_6` at 400 — is untouched.

## Consequences

- The middle layer exists and is measured. Nothing in the live observation path calls it; the
  replay does.
- **No geometry-only screen may become a durable Place**, enforced by a test that walks
  `internal/`.
- The next question is the one the press report cannot answer: attributing a press to the
  **next transition** rather than to the screen a fixed window later, which is what turns two
  screenshots separated by a mouse event into an observed click transition.

## Mutation gate

20 mutants, all 20 killed. One test was rewritten rather than excused: the transition listing's
sort by count was equivalent under its first fixture, because the most repeated pair also sorted
first by name. The fixture now walks three screens so the two orders disagree.

## Enforced by

- `internal/director/observe` — `TestAOneLookFlickerIsNotANewScreen`,
  `TestThreeDifferentFlickersAreNotOneNewScreen`,
  `TestAChangeThatHoldsIsAScreenAndComingBackIsTheSameOne`,
  `TestABlankLookIsAMissingObservationAndNotAScreen`,
  `TestWhenTheAbsencePersistsTheScreenBecomesUnknown`,
  `TestARepeatedNavigationIsCountedTwiceWithWhen`,
  `TestAnArrangementIsMadeOfRectanglesThatHaveAnArea`,
  `TestAVisualScreenCannotReachDurableMemory`,
  `TestTheDurableSignatureStillRefusesUnnamedGeometry`.
- `cmd/perceptionreplay` — `TestAnL0RecordingReplaysThroughPerception` (that the layer is
  reached from the replay at all).
