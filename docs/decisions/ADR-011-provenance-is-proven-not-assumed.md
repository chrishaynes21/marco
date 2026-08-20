---
type: decision
status: accepted
date: 2026-08-06
supersedes: []
affects:
  - perception
  - fusion
  - windows
  - safety
---

# Target provenance is proven by the provider, not assumed from the request

## Context

[[ADR-008-no-stale-window-geometry]] made capture safe at the moment of capture, and
[[director-accessibility-targeting]] made collection correct at the moment of collection.
Neither says anything about the interval in between, and evidence lives in that interval:

```
generation 7 validated → provider begins → the window is replaced
→ generation 8 is current → the provider returns generation-7 evidence
```

The request was correct when it was made. The provider answered it faithfully. The evidence
describes a window that no longer exists, and every check available at request time passes.

A cycle fuses several providers, and they do not run at the same speed. A tree walk over a
warm editor takes far longer than a monitor enumeration. So the question is not only whether
one provider is stale — it is whether accessibility, vision and OCR are describing the *same*
live window by the time their evidence is combined.

## The mechanism that does not work

The obvious design stamps each observation with the target from the request, and has fusion
compare that against the request's own expectation. It was built that way first.

Both sides are copies of one value. They match by construction. The guard passes precisely
the stale evidence it was built to reject, and does so while appearing to work — which is
worse than having no guard, because the report says the evidence was checked.

## Decision

**Intent and evidence are separate values with separate origins, and fusion compares them.**

```
ExpectedTarget   what the Director INTENDED to observe
                 built from the reference the runner validated

ObservedTarget   what the provider can PROVE its evidence describes
                 established after collection, by re-reading the platform
```

Three rules follow, and the third is the one that carries the weight:

1. A provider establishes `ObservedTarget` from its own post-collection evidence — the window
   the bridge says it walked, the window the frame was captured from — re-resolved against
   the platform as it is *now*. Never from the request.
2. Fusion admits target-scoped evidence only when the observed target is **Known** and
   **Matches** the expectation. Global evidence is exempt: monitor topology belongs to no
   window and cannot go stale when one is replaced.
3. **An unknown observed target is refused, not excused.** "I could not establish which window
   this came from" is not "it is probably fine". Treating it as agreement would let any
   provider bypass the guard by declining to answer, which is the cheapest possible bypass.

A targeted cycle that collected no outcomes at all therefore believes **nothing**. That is
the fail-safe: a path that pins a target and cannot say which provider proved what has
established nothing, and falling back to the unguarded evidence list would reopen the hole
silently, on the exact path where staleness is possible.

## Consequences

- A provider earns the right to contribute to a pinned session by proving what it saw.
  Implementing `TargetedProvider` is that proof; a plain `Provider` still works for ordinary
  command perception, where nothing is pinned and nothing has to be proven.
- Refused evidence **degrades the world**. It is not silently dropped — a confidently empty
  world reads as "the button is not there" when the truth is "I could not establish that what
  I saw was the right window", and that confusion is the reason this subsystem exists.
- Refused evidence is **retained** in its outcome for diagnostics. It may not be believed; it
  can still be described.
- Re-resolution must not repair what it checks. `Tracker.Confirm` reads the platform and
  changes nothing — no adopt, no reacquire, no epoch change. `Acquire` would notice the window
  had gone, reacquire, and report health, making the staleness disappear at the moment of
  testing.
- One resolver serves both the tree walk and the pixels, because they establish provenance by
  different routes and must not be allowed to drift into two answers.

## Enforced by

- **implementation** — `internal/director/perception/observation/outcome.go`
  (`ExpectedTarget`/`ObservedTarget`, `TargetProven`, `Usable`), `cycle.go` (`Admitted`),
  `internal/director/perception/fusion/engine.go` (the single call site),
  `cmd/director/targetprovenance.go` (the live resolver)
- **the race, end to end** — `TestAWindowReplacedMidCollectionDoesNotReachTheAdmittedSet`
  (`internal/director/perception/providers`): the window is replaced *while the walk is in
  flight*, and the evidence never reaches the admitted set
- **the trap** — `TestCopyingTheExpectationProvesNothing`,
  `TestExpectedAndObservedComeFromDifferentPaths`: an implementation that copies the
  expectation reports agreement and fails both
- **fail-safe** — `TestATargetedCycleWithNoOutcomesBelievesNothing`,
  `TestUnknownObservedTargetIsRefused`
- **not over-broad** — `TestAnUntargetedCycleIsUnaffectedByTheGuard`,
  `TestGlobalEvidenceSurvivesAGuardedCycle`,
  `TestAnUnreplacedWindowContributesThroughTheSamePath`. Every ordinary command runs the
  first of these; a guard that refused everything would satisfy the tests above and blind
  the Director.
- **the wiring** — `TestTheSamplerPinsTheGenerationAndNotOnlyTheWindow`. The previous defect
  in this subsystem was one unset field on this exact function, so the assertion exists.
- **read-only re-validation** — `TestConfirmDoesNotReacquireOrAdvanceTheGeneration`
  (`internal/director/perception/windowref`)
- **mutation-verified** — four, each restored after: replacing `cycle.Admitted()` with the
  raw observation list; replacing `Tracker.Confirm` with `Tracker.Current`; unsetting
  `Request.Target` in the sampler; zeroing the guard's effect on the report

## Related

- [[ADR-008-no-stale-window-geometry]] — safe at the moment of capture
- [[ADR-009-window-identity-is-ephemeral]] — why a generation is the durable number
- [[ADR-001-observations-vs-belief]], [[ADR-002-fusion-owns-belief]] — why the comparison
  belongs in fusion and nowhere else
- [[Perception]], [[Fusion]], [[Windows]]
