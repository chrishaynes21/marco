---
type: decision
status: accepted
date: 2026-08-11
supersedes: []
affects:
  - passive-observation
  - navigation
  - visibility
  - semantic-memory
source_paths:
  - internal/director/observe/screenstate.go
  - internal/director/observe/screenfixture/surface.go
  - cmd/director/playbill.go
  - pkg/playbill/playbill.go
---

# ADR-039 — a surface and a place inside it

A live session against ordinary software, sampled 141 times over a run that included idling,
scrolling and opening a second view twice, reported:

```
observation    1 screen    0 transitions
identity       141 same (weakest 0.759, mean 0.928)   0 other
```

The person driving it had gone somewhere and come back, twice, and Director recorded a single
motionless screen. Nothing above it — recognition, relationships, learning, rehearsal, plays —
can work on a world with one room in it.

## The invariant that was failing

Stated without naming any product:

> A surface whose persistent parts stay put while its state-bearing part is replaced is
> **the same surface** and **not the same place inside it**. Director could represent only one of
> those, and the one it represented was the one that cannot see a local change.

Screen identity was a single whole-surface comparison — a weighted Jaccard over a role histogram
and a coarse cell histogram, against a running mean. One number, one verdict, and therefore one
of two wrong answers available: "nothing happened" or "you are somewhere unrelated".

## Why no threshold fixes it

The two populations are not close. They are **inverted**.

A whole-surface comparison sums over everything, so a change confined to a minority region
contributes only its own fraction, while an application redrawing a large fraction of itself
without going anywhere contributes more. Measured on a generic surface with persistent chrome
around a smaller content region:

| what happened | whole-surface | within-a-part |
|---|---:|---:|
| a viewport scrolls | 1.000 | 1.000 |
| a tree gains a few nodes | 0.989 | 1.000 |
| a tree loses a few nodes | 0.983 | 1.000 |
| a sidebar appears | 0.947 | 0.862 |
| the content region is replaced | 0.714 | 0.000 |

The whole-surface margin between the harmless and the meaningful is **0.036**. Ordinary activity
in the live session moved that same figure by **0.241**. There is no value of the threshold that
separates them, which is why lowering it was refused: it would have bought one application's one
interaction at the cost of every application minting a place whenever somebody scrolled.

## The decision

**Screen identity becomes two levels.**

- The **surface** is what the whole-surface comparison already decided. Unchanged, including its
  threshold. It answers "is this the same application surface".
- A **state** is a place inside a surface. A second comparison, run only when the first said
  "same surface", answers "is this the same place inside it".

A state carries `Surface`, and a state with none is its own — so the relation costs nothing on a
surface that only ever had one place. `SurfaceID` is a session counter, like a state id, and
nothing durable is keyed on it.

## What the second comparison measures

**Composition, not amount**, per region of the coarse grid.

A feature key is `role@col,row`. Mass sits on a key that **arrived** or **left** — a part of the
surface now holds a kind of thing it did not, or a kind that was there is gone. More of what was
already there is not replacement however much of it there is.

This is load-bearing and was found by a failing test, not by reasoning: a list loading another
page changes forty structures and remains entirely itself, while a panel of a different kind
changes fifteen and means somewhere else. Anything sized on the amount ranks those the wrong way
round — the same inversion as above, one level down.

## One constant

`MinLocalCellStructures = 8`. How much of the surface must be made of something it was not.

It is **not calibrated against any application**. It is what the sentence has to mean: "a part of
this surface now holds something it did not" is a claim about a region with parts, and below
about eight of them it is arithmetic about two or three boxes. Mutation pins it from both sides —
at 3, ordinary churn in a rich tree becomes a place; at 12, a small window's replaced panel goes
unseen.

Two other constants were **removed** during this work rather than kept:

- a survival ratio beside it, once the whole suite proved indifferent to its value. Once
  "replaced" means composition, a ratio adds nothing but a knob somebody would later tune.
- a self-scaling cell floor (the mean cell mass). Mutation found nothing depending on it, and
  what it actually suppressed was a small change in a quiet region of a **large** surface — a
  dialog opening over the empty corner of a big window. Scaling the bar with the surface makes a
  window's own size an argument against noticing things inside it. Resolution is a property of a
  region, not of its neighbours.

## What a reader is told

Watch: *"Part of the screen changed, in the same application."* — a different sentence from *"The
screen changed"*, and it never appears when the session left the surface.

Diagnostics carries both comparisons. "One screen, no transitions" is consistent with an
application that never moved **and** with a comparison that could not see it move, and only the
within-a-screen figures tell those apart.

## What this deliberately does not do

- No application-specific condition, allowlist or name appears anywhere in it.
- The whole-surface threshold is unchanged.
- Nothing durable gained a field. `Surface`, `LocalFrom` and `LocalCell` are session-local, and
  they are a counter, a counter and a grid coordinate.

## The durable half (added 2026-08-11)

The split was proved through the session and out to what a person reads before anything checked
the half everything else depends on: a name, a start guard, a destination check and a
relationship endpoint all resolve through `SignatureOf(hypothesis)` into the semantic store.

**Nothing needed changing.** Every durable path was already per-state — `stateFingerprint` reads
one `ScreenState`, and the store is namespaced by **application**. `SurfaceID` reaches nothing
durable and must not: it is a session counter. So the enclosing context that survives a restart
is the application, and the meaning inside it is the state. What was missing was proof, and the
proof is now the load-bearing kind:

- two states of one surface, produced by a real session, become **two durable subjects** with
  different ids;
- a learned play guarded on one of them **sends nothing** when the other is in front — same
  application, same window, same chrome;
- a play whose destination is the other one **fails** when the application never left the first,
  with all its effects already sent;
- a habit between them becomes a **directed durable edge** rather than a loop, accumulates
  across sessions onto one edge, and two destinations inside one surface stay two edges.

### The seam

The durable signature carries role **composition** — which roles, in what numbers, plus the read
terms. The local comparison carries `role@col,row`. So:

| how two places differ | session | durable store |
|---|---|---|
| what the region is made of | two states | two subjects |
| only where the same structure sits | two states | **one subject** |

Measured, not argued (`TestTwoPlacesDifferingOnlyInArrangement`). The missing dimension is coarse
occupancy **in the durable signature** — the information is already observed, and nothing about
perception is failing. It was not added, because durable identity that reads a 3×3 grid makes a
panel crossing a cell boundary under resize into a different remembered screen, and geometry
invariance is a guarantee this system already holds. Closing it needs a signature that is
tolerant of the boundary, which is a decision with its own evidence to gather.

## Recorded limit

**An overlay of the same kind of structure over the same kind of content is invisible**, at
whole-surface (0.963) and within-a-part (1.000) alike. Its structure spreads across the coarse
grid into cells other structure already occupies, and at (role, coarse-cell) resolution a panel
of the same kind of thing over the same kind of thing is *literally the same observation* as more
of that thing arriving.

This is an information limit of the signature, not a threshold that wants moving: a bar loose
enough to catch it also catches a list getting longer. Closing it means the signature carrying
something it does not — a decision about the perception vocabulary, not about this comparison.

## Enforced by

- `internal/director/observe/screenfixture/local_test.go` —
  `TestTheLocalComparisonSeesWhatTheGlobalOneCannot` (the counterexample, and that the local
  margin exceeds the global one), `TestSurfaceSizeDoesNotDecideWhetherAChangeCounts` (the bar is
  absolute), `TestWhatTheLocalComparisonStillCannotSee` (the recorded limit).
- `internal/director/observesession/surfacewiring_test.go` —
  `TestASurfaceStaysItselfWhileItsStateChanges` (both truths at once, through a real session),
  `TestOrdinaryUseOfASurfaceProducesNoState`, `TestARegionGrowingByMoreOfTheSameIsNotANewPlace`,
  `TestACompositionSeenTwiceBecomesAState`, `TestBothStatesOfASurfaceMayBeUnknown`,
  `TestMovingOrResizingTheWindowPreservesTheState`,
  `TestTheSurfaceRelationCarriesNothingObservable`.
- `cmd/director/surfacewatch_test.go` — `TestWatchSaysAChangeStayedInsideOneApplication`,
  `TestWatchDoesNotClaimOneApplicationWhenTheWorldChanged`,
  `TestDiagnosticsCarriesTheWithinScreenComparison`.
- `internal/director/observe/localcost_test.go` — the cost, bounded by the signature.
- `internal/director/observesession/localsubjectwiring_test.go` —
  `TestTwoPlacesInOneSurfaceBecomeTwoDurableSubjects` (the durable half),
  `TestTwoPlacesDifferingOnlyInArrangement` (the seam, measured).
- `internal/orchestrator/localstateguard_test.go` —
  `TestALearnedPlayRefusesFromAnotherPlaceInTheSameSurface` (the start guard),
  `TestAPlayThatNeverLeftItsPlaceFails` (the destination check), with their controls.
- `internal/director/observesession/localrelationshipwiring_test.go` — durable edges,
  recurrence, and two destinations inside one surface staying two edges.
- `internal/director/observesession/localtaxonomywiring_test.go` — the modal/overlay/menu
  classification, provider independence, and unobserved-is-not-empty.

## Related

[[ADR-036-a-screen-is-a-composition-not-a-provider]] ·
[[ADR-035-uncertainty-survives-the-screen]] ·
[[ADR-038-session-bounds-are-sized-for-the-evidence]] ·
[[Passive-Observation]] · [[Visibility]]
