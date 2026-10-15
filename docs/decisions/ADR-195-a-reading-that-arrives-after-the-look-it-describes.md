---
type: decision
status: accepted
date: 2026-10-14
deciders: director
affects:
  - passive-observation
  - passive-visual-screen-integration
source_paths:
  - internal/director/observe/screenreading.go
  - internal/director/fastobserve/fastobserve.go
  - cmd/director/readingeyes.go
  - cmd/director/performworkflow.go
---

# ADR-195 — a reading that arrives after the look it describes

## Context

[[ADR-194-a-screen-is-also-what-it-says]] let a screen's identity consult what was read on a
look, and required the reading to be **look-local**: `fastobserve.Look.Text` is what a source read
of THOSE pixels and the type cannot carry anything else. That rule exists because the comment
beside the fold already refused to feed it delayed detections — a channel arriving ten looks late
and intermittently would make the signature of one screen depend on whether inference had landed
yet.

OCR is that channel. It costs 300–600ms on a 1920x1080 frame against an observation cadence of
100ms, so **no live source can ever satisfy `Look.Text`**. ADR-194 said so and stopped there.

The question is not how to make OCR fast. It is whether a late reading can be admitted without
corrupting screen identity.

## Decision

A late reading may inform the interpretation of the observation it describes, and may never be
treated as fresh evidence about a newer one.

`observe.ScreenReading` carries the same three stamps `observe.Candidate` does — the look, the
window generation, the observation epoch — and is judged by the same rule, through one shared
function:

```go
func Current(wasGeneration, wasEpoch, generation, epoch uint64) bool
```

`Admissible` for a detection and `ScreenReading.Admissible` both call it. Two copies would
eventually disagree, and the shape of that disagreement is an integration that refuses stale
rectangles while quietly accepting stale words.

### The epoch is the load-bearing half, and `admitSlow` already argued it

An epoch moves on every screen change. So an unchanged epoch is **not** "the same screen is
standing" — a screen left and returned to would satisfy that, and anything could have changed on
the way back — but "no screen change has happened since this frame", which is a demonstrably
continuous interval. A reading from inside such an interval is a reading of the screen standing
now, however many looks ago the pixels were captured.

Three rules, the same three and in the same order as for detections: the look it arrives on must
itself be evidence (pixels obtained, target window in front, something proposed); the window
generation must be the one standing now; the observation generation must be the one standing now.

### Why the merge is what makes this work

Counter-intuitive and it is the mechanism. When the Free Play submenu settled onto the Training
page's identity, **no screen change was recorded** — that is what a merge is — so the epoch did
not move. A reading of the submenu's own pixels arriving five looks later is still a reading of
the screen Marco is standing on: admitted, contradicting, and the merge refused.

A reading arriving after the screen genuinely changed is refused. Same rule, opposite answer,
different situation.

### What an admitted reading may do

One thing: **refuse to believe the standing screen is the screen the arrangement says.** It never
mints a screen, never moves the standing claim, and never reaches a stored arrangement. The
semantic half is `VisualScreens.Read`, and it is the same half the synchronous path does — the
reading is paired with the previous admitted reading OF THE SAME SCREEN, only what both said
counts, that agreement is folded into the screen's vocabulary, and a contradiction is raised
against the SCREEN so it outlives this call.

That last part is what makes a late reading able to do anything at all: ADR-194 already had to
carry a refusal across the looks between readings, because an unread look settles onto the
standing screen and would clear the pending run on its way past.

A reading that **agrees** withdraws a contradiction an earlier one raised, so the evidence can
correct itself before the run it started ever settles. Two readings that straddle a screen change
are not paired, because their agreement would be whatever junk they happen to share.

### Nothing here reaches backwards

A refusal changes which screen later looks are folded onto. It does not revisit a press, an
authorisation, or an arrival already established — none of those is stored in the screen model,
and every one of them was decided against the observation it happened in. A stale reading is
refused outright rather than filed against the screen it came from: filing it would be
defensible and is not free, the screen it described has gone, and nothing downstream is asking.

## Consequences

Measured in [[Experiment-043-a-reading-that-arrives-late]] on the existing recording — the same
900 looks, the same 300 readings, only their arrival changed:

```
                           screens settled offered admitted   stale refused   the pair
  pixel-only                     9     767       -       -       -       -   cannot ask
  with text, same look          13     729     900     300       0      18   APART
  late by 3 looks               12     723     299     260      38      16   APART
  late by 5 looks               10     736     299     245      54      11   APART
  late by 8 looks               10     754     298     223      75       8   APART
  late by 12 looks              11     755     296     200      95       6   merged
```

- **The pair stays apart while readings arrive within eight looks**, and OCR's real cost is three
  to six.
- **The late path is better behaved than the synchronous one.** At five looks it settles ten
  screens where the synchronous path settles thirteen, because the 54 readings taken on one screen
  and answered on another are refused — and those are exactly the ones that would have produced a
  spurious split. Lateness costs evidence and buys discipline.
- **It fails by degrading.** Past twelve looks the submenu's readings all arrive stale, no screen
  accumulates its vocabulary, and the fold is the one it was before this existed. Nothing is
  corrupted or mislabelled, and the counters say why.

`--reading` wires it live. The report prints `offered / admitted / refused as stale (window N,
screen moved on M) / unusable look K` and whether any reading ever contradicted anything, because
a source that read three hundred frames and admitted none is a run where OCR cost minutes and
changed no decision — and it must not read like a run where it worked.

**And the real clock did not fit.** Timed on the same frames through this plugin and this
tesseract, a full-resolution reading is 847ms at the median -- nine to ten looks behind a
window of eight. So `readingEyes` reads a HALVED frame: 516ms, six looks, with the
discrimination intact at kept 0.21 and novel 0.70 against thresholds of 0.35 and 0.55. A
third is faster still and loses it, because tesseract starts inventing words the two screens
then share. The factor is local to this source, which has a deadline; `director ocr` reads a
frame for a person to look at and does not.

**Not yet performed live.** The configuration is now inside the measured window on this
machine's own clock, and the report prints offered, admitted and refused so one run says
whether that holds against a real game.

## Enforced by

`internal/director/observe` — `TestALateReadingIsJudgedByTheSameRuleAsALateDetection`,
`TestALateReadingSeparatesTheScreenItsOwnFrameWasMergedInto`,
`TestAReadingWithNoScreenStandingConcludesNothing`,
`TestTheFirstReadingOfAScreenConcludesNothing`,
`TestALateReadingThatAgreesWithdrawsTheRefusal`,
`TestTwoReadingsThatStraddleAScreenChangeAreNotPaired`,
`TestAReadingThatFailedIsNotASilentScreen`,
`TestWithNoReadingsAtAllTheFoldIsUnchanged`,
`TestAReadingCarriesItsProvenance`.

`internal/director/fastobserve` — `TestALateReadingReachesTheScreenModelAndSeparatesTwoMenus`,
`TestAReadingWhoseContextMovedIsRefusedAndCounted`,
`TestAReadingOnAnUnusableLookIsRefused`,
`TestARunWithNoReadingsIsTheRunItAlwaysWas`,
`TestAFailedReadingIsCountedApartFromASilentScreen`.

`cmd/director` — `TestPerformWorkflowInstallsItsGates`, which names both halves of the wiring:
offering without draining reads frames and throws every answer away, and a frame offered without
the epoch would be admitted forever because a zero epoch matches whatever the observer started at.

## Related

- [[ADR-194-a-screen-is-also-what-it-says]] — the rule this makes reachable live.
- [[ADR-184-two-speeds-of-looking]] — the seam, and why late evidence may not reach identity.
- [[ADR-192-a-second-source-in-the-live-loop]] — the detector, whose shape this reuses exactly.
- [[Experiment-043-a-reading-that-arrives-late]] — the latency measurement.
