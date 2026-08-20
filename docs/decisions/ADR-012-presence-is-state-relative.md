---
type: decision
status: accepted
date: 2026-08-08
supersedes: []
affects:
  - passive-observation
  - perception
  - capabilities
---

# Presence is measured against the screen state an element can exist in

## Context

Shadow tracking measured a track's presence as `Seen / Eligible`, where `Eligible` advanced once
per valid inference for every live track. One denominator, one session, every element.

That denominator is wrong for anything state-dependent, and Rocket League made it visible. A
live 60-second session ran 20 valid ScreenParser inferences: eight while the pause menu was
open, nine of sparse gameplay, three unplaceable. The four menu rows were detected in **8 of the
8 menu inferences** — every single time the screen they live on was on screen. They reported:

```
seen 8 / eligible 16   presence 0.50   shape bursty
```

Read as a capability signal, that says "this control is present half the time", which is a
statement about reliability. It is not what happened. The rows never once failed to appear. The
eight inferences they "missed" were inferences of a different screen, in which their absence was
not merely acceptable but required.

Two prior investigations had already cleared every mechanical explanation, which is what made
the arithmetic the remaining candidate. [[Experiment-006-button-track-fragmentation]] showed on
the frozen corpus that the tracker does not fragment stable rows — 9/9 detection, consecutive
IoU 1.00, one track per row, unchanged under threshold and reference-policy sweeps.
[[Experiment-007-state-relative-tracking]] then measured the live session directly: consecutive
IoU 0.947–1.000, production and `shadowreplay` assigning all 77 detections identically, all 11
button-track mints legitimate. Classification was `LIVE_INPUT_DISTRIBUTION`: nothing was broken,
the session simply spent half its time on a different screen.

## Decision

**A track's presence is measured against the inferences in which its own screen state was
active.** A session is segmented into generic, session-local screen states from raw detection
composition, and every track carries per-state `Seen` / `Eligible` alongside the global pair.

The global figures are **retained unchanged**. They answer a different and still-real question —
"how much of this session was this element on screen" — and a menu control genuinely is bursty
across a session that is half gameplay. Both are reported; neither is allowed to stand in for
the other.

Absence counts against a track only when its state is confidently active AND a valid inference
ran AND the track was not detected. A different state active, an unplaceable transition, a
skipped slot, a failed inference or an unproven target are all **unknown**, and unknown stays
unknown — the same rule [[ADR-006-unknown-is-not-false]] already applies to evidence, extended
to the denominator.

### State identity is generic and non-circular

States are `state_1`, `state_2`. Not "pause menu". Naming is interpretation and belongs to a
capability pack; building it in here would put an unfalsifiable guess underneath a measurement.

A state's signature is role composition plus coarse normalised arrangement, computed from **raw
detections only**, before any track is read or written. Signing a state with the track IDs
present would be circular — tracks would define the state that defines their own eligibility,
and a track could never be found absent in the state it was minting.

### Two thresholds, because similarity alone cannot decide

An unmatched inference is a **transition** when a known state already explains its structure
(containment ≥ 0.75) and a **new screen** when it introduces structure no known state has. A
half-drawn menu and a structurally different dialog both score ~0.3 similarity against the menu;
they differ in kind, not degree. Containment is asymmetric on purpose: it asks only whether the
frame contains anything new.

An ambiguous composition is held rather than judged, and **recurrence promotes it**. A sparse
screen is contained in every rich one, so a session that opens on a menu would otherwise read
gameplay as a permanent transition. The first sighting is spent as unknown; that is the honest
price of not guessing.

## Consequences

The four live Rocket League menu rows convert from `8/16, 50%, bursty` to `8/8, 100%,
persistent in state_3`, across three separate menu episodes, with global figures unchanged.

Temporal shape now has two levels. `ShadowTrack.Shape` is global; `TrackState.Shape` is
state-local. `StateStable()` is the capability-relevant predicate and `Stable()` is not — the
latter answers wrongly for anything state-dependent, which was the conceptual error.

Co-occurrence grouping becomes meaningful for the first time and is now derived: tracks
persistent in exactly one state, cut into runs by vertical arrangement. "Persistent in exactly
one state" is load-bearing — an element equally reliable on both screens is ambient and belongs
to neither structure.

All of it remains shadow-only. State IDs, state history and track-state association reach
nothing in World State, fusion, planning, policy or execution.

## Enforced by

- `TestGameplayInferencesDoNotErodeAMenuButton` — THE regression. Reverting eligibility to
  "every valid inference" fails it.
- `TestARealDetectorMissInsideTheStateStillCounts` — a favourable denominator does not launder a
  genuine miss; 3 of 5 stays 0.60.
- `TestAnUnplaceableTransitionCountsAgainstNobody` — unknown stays unknown.
- `TestAScreenThatReturnsResumesItsOwnStateIdentity` — recurrence, or the model says nothing.
- `TestStructurallyDifferentPanelsStayDifferentStates` — composition, not "are there buttons".
- `TestStateIdentityDoesNotDependOnTracking` — the non-circularity guard.
- `TestAGameplayElementIsPersistentInTheGameplayState` — the inverse case; menus are not
  privileged.
- `TestNonEvidenceSlotsDoNotSegmentAnything` — a cadence decision is not a screen.
- `TestTheProductionSessionPathSegmentsScreenStates` — runner-level wiring; deleting the
  segmentation call from `ShadowTotals.Add` fails it.
- `TestScreenStatesReachNothingAuthoritative` — the mutation guard.
- `TestTheMirrorReproducesProductionTracking` — global semantics are unchanged, so the
  `shadowreplay` mirror still describes the shipping tracker.
