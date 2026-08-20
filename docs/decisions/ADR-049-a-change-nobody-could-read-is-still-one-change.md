---
type: decision
status: accepted
date: 2026-08-13
supersedes: []
affects:
  - passive-observation
  - semantic-memory
  - demonstrations
source_paths:
  - internal/director/observe/bridge.go
  - internal/director/observe/relationship.go
  - internal/director/observe/screenstate.go
  - internal/director/observesession/runner.go
---

# ADR-049 — a change nobody could read is still one change

A live Learn attempt on Windows Settings established both endpoints, watched the person press a
key, and then failed to record that the two were connected. The route the whole session existed to
capture did not exist at the end of it.

## What was measured

Instrumentation came first, deliberately: `RelationshipReport` gained a **closed**
`SessionLocalCause` vocabulary — `source_unresolved` · `destination_unresolved` ·
`both_unresolved` · `same_subject` — so the refusal could be read at the exact boundary rather than
guessed at from a missing edge.

Two captured Settings sessions, replayed as fixtures, give the identical shape:

```
state_1        → state_unknown    count 1, preceded {confirm: 1}     destination_unresolved
state_unknown  → state_2          count 1, unattributed 1            source_unresolved
```

One sample of ninety-one fell outside both recognised screens. `state_unknown` is not a screen: it
produces no hypothesis and can never be a durable subject, so **both** halves of one movement
resolved to nothing. Reproduced on a nine-observation visit and on a sixty-seven-observation one,
so it is a property of the sampling and not of how long anybody lingered.

Fixtures: `internal/director/observe/testdata/live_settings_7.json`, `live_settings_11.json`.

## The decision

When a session crosses an unplaceable sample, the relationship layer may recover **one** adjacency
A→B — and nothing else.

It concludes exactly one thing: *a change from A to B was observed*. It does not decide what the
unreadable sample was. Nothing mints a state, names one, gives one identity, or lets
`ScreenStateUnknown` become a subject. The unplaced sample stays unplaced.

Five conditions, all read from evidence the session already keeps:

1. **Exactly one entry and one exit.** More than one and which pairs with which is not something
   aggregated counts can say — and one of the paths may have run through a screen that *is*
   recognisable.
2. **The interval was recorded, and shorter than `StatePromotionCount`.**
3. **The session held one uninterrupted view of one window** — `Continuity.Unbroken()`, from the
   generation count and target-loss count the runner already tracks.
4. **Both endpoints resolve to durable subjects.** Recognisability is not relaxed.
5. **The endpoints differ.** A self-loop says nothing.

Evidence comes from the **entry** leg, and that is the point: the navigation the person performed
is recorded against the change *into* the unplaced sample, because that is the observation that
first saw the screen move. In every case measured the exit leg carried no intent at all.
`Observations` is the smaller of the two counts — a traversal is evidence of a *completed* change
only when both halves were seen — and `Evidence.Bridged` records the claim so it stays visible
wherever the edge is later read.

### Why the bound is StatePromotionCount

Because segmentation already draws exactly this line:

> One sighting is a transition frame; a second is a screen.

An unsettled run shorter than `StatePromotionCount` is, by the architecture's own definition, a
transition frame — so bridging it asserts nothing segmentation does not already assert. A run long
enough to have been promoted might have contained a screen that simply never recurred, and skipping
it would hide a real place. That is the difference between recovering a lost adjacency and
inventing one.

No new magic number was chosen. That was a requirement, and it turned out the architecture already
had the notion.

### What had to be added to measure it

`ScreenTransition.UnsettledRun` — how many consecutive samples the segmenter could not place before
the one it could. A run of zero on an exit edge is **missing evidence**, not a short gap: an exit
crossed at least one sample by construction. It is refused as `interval_unknown`, which keeps the
bound meaningful — without the length there is nothing to bound.

## What it must never do, and does not

- Assign identity to the unknown sample.
- Change `ScreenStateUnknown`, screen identity, or `Recall`.
- Weaken endpoint recognisability.
- Bridge across a window replacement or a target loss. No bound on *length* would make that honest;
  an unsettled interval that overlaps one may be hiding an entire application restart.

## Considered and rejected

- **Treat the unknown sample as a continuation of A.** It is the smallest change and it is a lie
  about what was seen; every consumer of screen identity would inherit it.
- **Lower the promotion threshold so the sample becomes a screen.** Fixes the symptom by making
  Marco believe in more screens, and would mint a durable subject per animation frame.
- **Widen the transition window in the relationship layer generally.** Reaches the same edge
  without proving continuity, and would silently connect two places across an arbitrary gap.

## Enforced by

- `internal/director/observe/bridge_test.go` — the seven controls, each a distinct refusal:
  `TestLeavingAndComingBackDoesNotManufactureASelfRelationship`,
  `TestABrokenObservationIsNotBridged`,
  `TestAnIntervalLongEnoughToHaveBeenAScreenIsNotBridged`,
  `TestAnUnrecordedIntervalIsNotBridged`,
  `TestAnIntermediateRecognisableScreenIsNotSkipped`,
  `TestALeadingUnknownDoesNotInventASource`,
  `TestATrailingUnknownDoesNotInventADestination`,
  `TestTheBridgeDoesNotRelaxEndpointRecognisability`, plus
  `TestADirectChangeIsUnchangedByBridging` and `TestOneUnplaceableSampleDoesNotLoseTheAdjacency`.
- `internal/director/observe/liveevidence_test.go` —
  `TestTheLiveSettingsTransitionsNameTheirCauses` pins the two measured causes against the captured
  sessions, and `TestTheLiveSettingsRouteIsRecoveredByBridging` requires the real route back.
- `internal/director/observe/unsettledrun_test.go` — the counter is driven through the **real**
  `ShadowTracker`, because a mutation run found that deleting `g.unsettled++` broke nothing: every
  bridge fixture was supplying the number itself. `TestTheSegmenterRecordsHowLongItCouldNotPlaceTheScreen`,
  `TestALongerBlackoutIsRecordedAsLonger`, `TestTheUnsettledRunResetsWhenAScreenIsPlacedAgain`.

Eight mutations applied, eight caught, every file restored byte-identically.

## Related

[[ADR-035-uncertainty-survives-the-screen]] ·
[[ADR-041-a-screen-is-not-its-dominant-group]] ·
[[ADR-047-a-place-is-remembered-a-meaning-is-answered]] ·
[[Passive-Observation]] · [[Semantic-Memory]]
