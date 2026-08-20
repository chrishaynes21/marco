---
type: decision
status: accepted
date: 2026-08-10
supersedes: []
affects:
  - passive-observation
  - perception
  - semantic-memory
  - vision
source_paths:
  - internal/director/observe/structure.go
  - internal/director/observe/shadow.go
  - internal/director/observe/shadowtrack.go
  - cmd/director/observesnapshot.go
---

# ADR-036 — a screen is a composition, not a provider

Watch's first live diagnosis of unfamiliar software said this:

```
accessibility  687 obs
fusion         687 obs -> 685 elements
observation    0 screens   0 transitions
```

Six hundred and eighty-five fused elements, and no screen. Therefore no recognition, no
relationships, no learning, no rehearsal and no plays — against Chrome, against VS Code, and
against every other ordinary application.

## What was actually happening

Screen segmentation read `Sample.Shadow.Regions` and nothing else. That is the **experimental
structural detector's** output, and the detector is opt-in behind `$MARCO_SHADOW_VISION`
because it costs ~1.25 GB resident and ~0.9s an inference.

So the default Director — the one everybody runs — had no screen model at all, and the entire
learning stack above it was reachable only through an experiment. `Sample.Entities` carried
the same window-relative normalised geometry and the same role vocabulary the segmenter wants,
and there was no path from it.

The repository had half-noticed. [[Passive-Observation]] listed *"Vision — currently the only
source reaching a targeted session"* under Related systems, as a fact about the session rather
than as the blocker it was.

## Decision 1 — a screen is segmented from a COMPOSITION, whatever described it

`ScreenSignature` reads role and window-relative region. That is the whole contract; it does
not want labels, text, identities, absolute coordinates or a provider's name — and a
composition is the same composition whichever admissible source described it.

`observe.StructuralView` is that composition plus its provenance, and `StructureOf` chooses it.

## Decision 2 — the authoritative world first, the experiment second

FUSED FIRST. Every provider that proved its target contributed to it, fusion merged the ones
describing the same thing, and the privacy classifier has already run. A screen is then a fact
about what the Director **believes** is on screen.

THE DETECTOR SECOND, and only where fusion saw no structure at all — the surface it exists
for, and the case [[ADR-017-structure-earns-a-name-text-never-earns-structure]] was written
about.

**Alternatives, not addends.** The detector is deliberately outside fusion so it cannot
influence belief, and a screen identity is belief-adjacent: it decides what a track is
measured against, what a relationship connects, and what a play says it begins on. Merging
unfused detector boxes into an authoritative composition would let the experiment move all of
that, and would double-count every control both sources found. Corroboration between them is
measured separately, by `ShadowComparison`, which is where a comparison belongs.

## Decision 3 — observed-and-empty is a screen; unobserved is not

The gate was `Ran && TargetProven && Unavailable == ""` on the shadow sample. It is now
`StructuralView.Observed()`, which is the same rule expressed where provenance is decided —
so a second structural source cannot arrive without one.

An empty composition from a source that LOOKED is the sparse screen an application can
legitimately present: real, recurring, minted. An empty composition from a source that did not
look is silence, and minting a state from silence would give every track a screen to be absent
from ([[ADR-006-unknown-is-not-false]]).

## Decision 4 — the screen model is published even when the experiment sat out

Found by mutation. `ShadowTotals.add` copied the tracker's conclusions out *after* the
`!s.Ran` early return, so a Director with both sources — where the detector's cadence gate
declines most slots — held a screen model and reported none. Every test had either a detector
on every slot or no detector at all; production has neither shape.

## Consequences

- Screen existence no longer depends on an opt-in experiment. Provider independence is real in
  all four combinations, and none of them is privileged by construction.
- Screen identity is now derived from post-fusion belief, which means it inherits fusion's
  provenance guard for free — and inherits fusion's mistakes too.
- Discrimination at accessibility scale was **measured**, not assumed: two 128-element screens
  separate below ~63% shared composition and merge above ~75%. That boundary came from this
  milestone; the thresholds themselves came from a four-box detector trace and have not been
  retuned.

## Enforced by

- `internal/director/observesession/structurewiring_test.go` —
  `TestAnAccessibleApplicationProducesScreens` (the regression),
  `TestASkippedDetectorSlotStillSegmentsTheFusedComposition`,
  `TestASessionThatObservedNothingMintsNoScreen`,
  `TestASourceThatLookedAndFoundNothingStillMintsAScreen`,
  `TestDetectorAndAccessibilityStructureSegmentAlike`
- `cmd/director/structurewiring_test.go` — `TestFusedStructureReachesTheScreenModel`,
  `TestAnUnprovenCycleProducesNoComposition`, `TestAnUnfamiliarScreenExistsWithoutRecognitionNameOrText`
- `internal/director/observe/structure_test.go` —
  `TestTheExperimentCannotDisplaceTheAuthoritativeComposition`, `TestTheDetectorGateIsUnchanged`
- `internal/director/observe/screenscale_test.go` —
  `TestTwoRichScreensWithDifferentCompositionsStaySeparate`,
  `TestTheDiscriminationBoundaryAtAccessibilityScale`
