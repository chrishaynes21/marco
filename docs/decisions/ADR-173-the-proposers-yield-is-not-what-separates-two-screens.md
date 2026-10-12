---
type: decision
status: accepted
date: 2026-10-12
affects:
  - passive-observation
  - perception
source_paths:
  - internal/director/observe/arrangement.go
  - cmd/perceptionreplay/screens.go
---

# ADR-173 — the proposer's yield is not what separates two screens

## Context

[[ADR-170-what-an-unnamed-arrangement-would-have-segmented]] raised a doubt about its own
result, and the owner turned it into a question worth answering:

> I'd test whether arrangement similarity remains stable when the detector produces different
> numbers of regions on otherwise similar frames. But I wouldn't normalize away every count
> difference. Sometimes a genuine menu transition really does add or remove controls. The
> objective is to recognize persistent spatial structure while retaining meaningful differences.

The specific suspicion was concrete: on the stabilised recording `visual_1` settled at **236**
regions and `visual_6` at **400**, and if those are one screen at two proposal yields then the
arrangement cue is separating screens for the wrong reason.

## Decision

Measure it, change nothing.

`ArrangementShapeSimilarity` compares two arrangements by SHAPE: each cell's count divided by
that arrangement's own total before the same weighted Jaccard the stabiliser uses. Two readings
of one screen at different yields come out alike; a menu that genuinely gained a column does
not, because the distribution moved rather than the density.

`ArrangementSimilarity` — the rule the stabiliser actually applies — is untouched and still
counts. The replay reports, for every pair of settled screens, the similarity by count beside the
similarity by shape, and names the pairs that would be one screen under the second and are two
under the first.

## Measured outcome

```
highest shape-similarity between any two settled screens: 0.695 (threshold 0.55)
5 pair(s) would be ONE screen by shape and are two by count:
    visual_1   / visual_9    count 0.504  shape 0.695
    visual_3   / visual_4    count 0.540  shape 0.602
    visual_3   / visual_6    count 0.247  shape 0.642
    visual_4   / visual_6    count 0.188  shape 0.592
    visual_5   / visual_8    count 0.515  shape 0.550
```

**The suspicion is refuted.** `visual_1` / `visual_6` is not in the list: they differ by shape as
well as by count, so the 236-against-400 gap is not what is separating them. They are genuinely
different distributions of rectangles, and the cue is separating them for the right reason.

**And normalising would be actively harmful.** It would merge `visual_3` with `visual_4` — which
are two consecutive steps of the demonstrated route, joined by a transition observed in both
demonstrations and by an attributed click
([[ADR-172-a-click-transition-is-a-source-a-target-and-a-destination]]). A normalisation that
collapsed a confirmed route step would answer the yield question by destroying the thing the
question was asked in service of.

So the owner's caution was the right call and the measurement says so: *I wouldn't normalize away
every count difference.*

## Consequences

- **Count normalisation is rejected on measurement**, not on taste. The numbers are in the tree
  and the next person does not have to re-run them.
- The proposer's varying yield remains a real property of the pixels path and is **not** the
  mechanism separating these screens. Whatever is wrong with nine screens for four or five real
  ones, this is not it.
- `ArrangementShapeSimilarity` stays exported as a measurement. Nothing in the stabiliser calls
  it, and a caller that used it to decide identity would be applying a rule this decision
  rejected.
- The five pairs above are the closest any two settled screens come to each other by shape, and
  the highest is 0.695. A future attempt at this has a baseline to beat.

## Mutation gate

4 mutants, all 4 killed.

## Enforced by

- `internal/director/observe` — `TestShapeIgnoresHowManyRectanglesThereWere` (both halves: the
  yield is ignored AND a real change of layout survives it),
  `TestASettledScreensArrangementCanBeReadBack`.
- `cmd/perceptionreplay` — `TestAnL0RecordingReplaysThroughPerception` (that the lookup the
  measurement needs is reached; without it the comparison compares nothing against nothing and
  reports the same conclusion for the wrong reason).
