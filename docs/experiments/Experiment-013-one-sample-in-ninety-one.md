---
type: experiment
status: complete
result: "confirmed — a single unplaceable sample split one movement into A→unknown (destination_unresolved) and unknown→B (source_unresolved); reproduced byte-identically across two live sessions of very different length, so it is a property of the sampling"
date: 2026-08-13
affects:
  - passive-observation
  - semantic-memory
source_paths:
  - internal/director/observe/bridge.go
  - internal/director/observe/relationship.go
  - internal/director/observe/screenstate.go
  - internal/director/observe/testdata/live_settings_7.json
  - internal/director/observe/testdata/live_settings_11.json
---

# Experiment 013 — one sample in ninety-one

A cold Learn attempt on Windows Settings cleared every gate it was supposed to. It established the
START, grounded it, watched the person press a key, established the DESTINATION, recalled it after
a restart — and then recorded no relationship between the two. The one thing the session existed to
capture was the one thing missing.

## Why instrumentation came first

`RelationshipReport.SessionLocal` was a count. A missing durable edge looks identical whatever
caused it — an endpoint that would not resolve, a self-loop, an outright rejection — and the
leading hypothesis (`A → unknown → B`) was plausible enough to be dangerous. Guessing here means
changing the relationship layer on a story.

So the first change measured nothing new about the world; it made the existing refusal say which
refusal it was, in a closed vocabulary of four.

## What the two sessions say

Both captures replay as fixtures. The interesting figure is the *sample* count, because it is what
makes the shape non-obvious:

| session | samples | state_1 | state_2 | outside both | transitions |
|---|---|---|---|---|---|
| run 1 | 91 | 67 | 23 | **1** | 2 |
| run 2 | 91 | 9 | 81 | **1** | 2 |

The two visits are as different as two visits get — one lingered on the home page, the other left
almost at once — and both lost exactly one sample. Measured causes, identical in both:

```
state_1        → state_unknown     count 1, preceded {confirm: 1}    destination_unresolved
state_unknown  → state_2           count 1, unattributed 1           source_unresolved
```

That rules out the explanation anyone would reach for first. This is not somebody being slow, or
an unlucky pause; the sampler catches one frame mid-animation whatever the person does, and
`ScreenStateUnknown` cannot be an endpoint, so both halves die.

Note where the evidence sits: the `confirm` the person actually pressed is on the **entry** leg.
The exit leg is a bare arrival with nothing attributed to it. Any fix that read the exit leg would
have recovered the edge and thrown away the reason to believe in it.

## The one thing the measurement could not tell us

How long the blackout lasted. The segmenter recorded that a sample was unplaceable but not how many
in a row — which is the only figure a bound could ever be drawn against. Deriving it from
`91 − 90` worked for these two fixtures and would have been unusable in production.

`ScreenTransition.UnsettledRun` was added for it, and a mutation run immediately showed why it had
to be measured through the real tracker: deleting the counter broke nothing, because every fixture
was supplying the number itself. An unrecorded interval is now refused (`interval_unknown`) rather
than treated as a short one — a run of zero on an exit edge is missing evidence, since an exit
crossed at least one sample by construction.

## Outcome

The bridge in [[ADR-049-a-change-nobody-could-read-is-still-one-change]], bounded by
`StatePromotionCount` — segmentation's own line between a transition frame and a screen. Both
fixtures recover their route with the `confirm` intact and the edge marked as bridged.

Eight mutations, eight caught.

## Related

[[ADR-049-a-change-nobody-could-read-is-still-one-change]] ·
[[ADR-047-a-place-is-remembered-a-meaning-is-answered]] ·
[[Experiment-012-why-explorer-cannot-be-pointed-at]] · [[Passive-Observation]]
