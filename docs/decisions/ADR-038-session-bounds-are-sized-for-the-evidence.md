---
type: decision
status: accepted
date: 2026-08-11
supersedes: []
affects:
  - passive-observation
  - semantic-memory
  - hypotheses
source_paths:
  - internal/director/observe/shadowtrack.go
  - internal/director/observe/screenstate.go
---

# ADR-038 — a session bound must be sized for the evidence, not for the detector

`MaxActiveTracks = 128` had no measured rationale, only a purpose: "bound one session's memory".

A fused accessibility tree reports 52 stable structures for a Chrome window, 128 for VS Code
and 128 for File Explorer. So after
[[ADR-036-a-screen-is-a-composition-not-a-provider]] made screens buildable from accessibility,
the bound was **exactly one realistic screen**.

## What that cost

Total, and silent. A deterministic two-screen session evicted **1,460 tracks** and produced
structure for one screen out of two. The second screen a session visited could begin no tracks,
which means:

```
no tracks → no structural group → no hypothesis → no durable subject
          → no relationship endpoint → no relationship, ever
```

Every session on real software could observe a transition and none could remember one. Nothing
errored; `Evicted` counted it and nothing read the counter.

## Decision — size the bound against the evidence, and say so

1024 active, 2048 retired: roughly eight realistic screens. A `ShadowTrack` is on the order of
600 bytes once its per-state evidence is counted, so this is under two megabytes — against an
observation history that already retains five cycles of a 685-element accessibility tree with
labels, sources and geometry. **The bound is still a bound; it is now above the size of the
thing it is bounding rather than below it.**

Cost measured, not assumed: `assign` is O(regions × tracks), and at 716 tracks one inference
takes **328 µs** against a 176–298 ms perception cycle sampled every 500 ms–1 s.
`BenchmarkTrackAssignmentAtAccessibilityScale`.

## Decision — the same class of defect is now checked rather than reasoned about

Two other constants were audited against accessibility-scale evidence in the same pass, and both
turned out to be scale-dependent in the same way. They are **recorded and not changed**, because
resolving them needs a measurement of real software that nobody has taken:

- `StateMatchSimilarity = 0.55` was measured on a detector whose frames of the same screen score
  0.71–0.79. At accessibility scale a real interface change scores 0.693 and harmless churn
  scores 0.882. One global constant cannot accept the first band and reject the second — see
  `TestOneGlobalSimilarityThresholdCannotServeBothSources`.
- `StructureSignature.Discriminating()` counts read text or an envelope. A whole screen has no
  envelope, so a screen with no scoped reading can never be remembered — however distinctive its
  twelve-role, 128-structure histogram is. See
  `TestAScreenWithNoDiscriminatorCannotBeRemembered`.

Loosening either on synthetic evidence would be exactly what
[[ADR-016-cross-session-identity-is-structural-and-conservative]] warns about.

## Consequences

- A session can now hold structure for several screens, which is what a relationship needs.
- Eviction is still possible and still counted. It is now reported to a person as well —
  a bound that truncates silently is what made this take a milestone to find.
- Any future bound in this subsystem has to state the size of the evidence it is bounding.
  "Bound one session's memory" is a purpose, not a number.

## Enforced by

- `internal/director/observe/trackcost_test.go` —
  `TestBothScreensOfASessionEarnTheirOwnStructure`,
  `BenchmarkTrackAssignmentAtAccessibilityScale`
- `internal/director/observe/shadowtrack_test.go` — `TestTrackingIsBounded`, now sized against
  the constants rather than against a literal
- `internal/director/observe/screenfixture/threshold_test.go` —
  `TestWhatTheIdentityThresholdCanSeeAtAccessibilityScale`,
  `TestOneGlobalSimilarityThresholdCannotServeBothSources`
- `internal/director/observesession/substratewiring_test.go` —
  `TestAScreenWithNoDiscriminatorCannotBeRemembered`
