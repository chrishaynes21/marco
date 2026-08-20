---
type: subsystem
status: active
owners:
  - director
depends_on:
  - perception
used_by:
  - programs
  - goals
  - collections
  - control-flow
updated: 2026-08-06
source_paths:
  - internal/director/perception/fusion
  - internal/director/world
---

# Fusion

Fusion is the **only** place evidence becomes belief. Every observation from every provider
reaches world state through it or does not reach world state at all.

## Responsibilities

- Merge observations into elements, by identity and by cluster.
- Combine confidence across corroborating sources.
- Attach text to structure — conservatively, and only where the source was entitled to
  establish that structure.
- **Compare intent against evidence**, and refuse target-scoped evidence that cannot prove it
  describes the live window generation — [[ADR-011-provenance-is-proven-not-assumed]].
- Produce a fusion report that accounts for what it did and why.

## The target provenance guard

Perception records two things about a targeted cycle and does not reconcile them: what the
Director *intended* to observe, and what each provider can *prove* it observed. Fusion is
where they meet, for the same reason everything else does — it is the only place evidence
becomes belief, so it is the only place a refusal is total.

The comparison lives on `ProviderOutcome.TargetProven` (one canonical implementation, so a
second opinion cannot drift from it) and is applied at exactly one call site,
`cycle.Admitted()` in `engine.Fuse`. Refused evidence does not vanish: it degrades the world
and is named in `Report.Provenance`, because a silently empty world reads as "it is not
there" rather than "I could not attribute what I saw".

## What it may not do

- Promote standalone text to an element. Text that matched no structure stays in the
  observation graph as evidence.
- Let a detector's class become actionability. See
  [[ADR-004-vision-cannot-establish-actionability]].

## Why the report is complete

Because nothing bypasses fusion, a fusion report that does not mention something is
**proof** that it did not become belief. That property is worth the bottleneck.

## Related systems

- [[Perception]] — produces what this consumes
- [[Vision]] — a source whose claims are deliberately weighted down
- [[Programs]] — reads the belief, never the evidence

## Decisions

- [[ADR-002-fusion-owns-belief]]
- [[ADR-001-observations-vs-belief]]
- [[ADR-003-evidence-authority-by-source]]
- [[ADR-011-provenance-is-proven-not-assumed]]

## Validated by

- `internal/director/perception/fusion/engine_test.go`, `identity_test.go`, `cluster_test.go`,
  `text_test.go`, `vision_fusion_test.go`, `visual_test.go`, `explain_test.go`
- `internal/director/perception/fusion/provenance_test.go` — the guard at the boundary where
  it decides something: stale evidence never becomes belief, matching evidence is believed
  normally, an untargeted cycle is untouched
- `TestPerceptionCannotReachIntoTheReasoningLayers`

## Known gaps

- Fusion is a bottleneck by construction, and the cost on a busy screen is real — see the
  *Cost* section of [[director-perception]].
- ~~Combined perception has never been measured~~ — the accessibility provider is pinned as
  of [[director-accessibility-targeting]], so the blocker is gone; the **measurement itself
  has still not been taken**, and every fusion number on record remains single-source.
- **The guard's cost is unmeasured.** It adds a platform re-read per targeted provider per
  cycle. Expected to be small against a tree walk and never checked.

## Milestone record

[[director-perception]] — *Merge criteria*, *Fusion report*, *Conservative fusion*.
