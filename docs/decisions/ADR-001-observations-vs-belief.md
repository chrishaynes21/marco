---
type: decision
status: accepted
date: 2026-08-06
supersedes: []
affects:
  - perception
  - fusion
  - world-state
---

# Observations are evidence; World State is belief

## Context

A provider reports what it saw. An accessibility walk returns a tree, an OCR pass returns
words, a detector returns boxes. Each of those is a claim made by one source at one moment,
under its own failure modes.

The tempting shortcut is to let a provider write directly into the model the planner reads.
It is one less type and one less stage. It also destroys the ability to answer "why does the
Director think there is a Save button there", because by the time anything is wrong the
evidence that produced the belief is gone.

## Decision

**An observation is evidence and never a fact.** It carries its provenance and its
confidence, and it is immutable. The World State is a separate thing: the Director's
*belief*, assembled from evidence, and the only thing the reasoning layers read.

No provider writes to world state. No planner reads an observation.

## Consequences

- Every belief can be traced to the evidence that produced it, which is what makes
  `director explain` and the fusion report possible at all.
- Disagreement between sources is representable rather than a race. Two providers may claim
  different things about the same region; that is data, not a bug.
- There is a cost: two type families and a conversion stage that a simpler design would not
  need.
- Confidence is a first-class property of evidence, not something recomputed later from
  appearance.

## Enforced by

- **implementation** — `internal/director/perception/observation`, `internal/director/world`
- **boundary test** — `TestOnlyPerceptionKnowsWhatAnObservationIs`
  (`internal/director/perception_boundary_test.go`) fails if an observation type is
  referenced outside perception
- **boundary test** — `TestPerceptionCannotReachIntoTheReasoningLayers` (same file)

## Related

- [[ADR-002-fusion-owns-belief]] — the conversion itself
- [[ADR-003-evidence-authority-by-source]] — what each source is permitted to establish
- [[Perception]], [[Fusion]]
