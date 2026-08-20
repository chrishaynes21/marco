---
type: decision
status: accepted
date: 2026-08-10
supersedes: []
affects:
  - perception
  - vision
  - privacy
source_paths:
  - internal/director/perception/providers/vision/targeted.go
  - internal/director/perception/observation/outcome.go
---

# ADR-037 — an opt-in is enforced on every door

`vision.Provider.Observe` opened with the rule and stated it plainly:

> It looks at NOTHING unless the request asks for vision by name. That is the trigger policy
> in one line: a caller that has not thought about whether it wants a screen capture does not
> get one.

`vision.Provider.ObserveTargeted` went straight to `p.Look`.

The collector prefers `ObserveTargeted` for any provider that implements it
([[ADR-011-provenance-is-proven-not-assumed]] is why vision implements it), so from the moment
vision became a `TargetedProvider` **every ordinary perception cycle ran a vision pass nobody
asked for** — every command, every wait poll, every diagnostic read.

## What it cost

**A screen capture per cycle**, on any Director with the vision plugin configured. That is a
privacy cost and a real one: the opt-in exists because a capture is the most invasive thing
this system does, and the rule was being enforced on the door nobody used.

**A permanent false degradation.** Before a world exists there is no focused window to resolve,
so the unrequested pass failed with *"no window to look at"* and that landed in the world's
`Degraded` list. Every live diagnosis of this project showed it. It reads as a targeting fault
and is not one — and it was the second symptom in the milestone this ADR came out of, where it
cost real time before being separated from the first.

## Decision — the guard goes on the interface, not on one implementation

Both doors check `req.Includes(SourceVision)`. Adding a capability (proving provenance) must
never widen what a provider may do.

## Decision — not-requested is its own state

`observation.StateNotRequested`. Held apart from `StateEmpty` and `StateUnavailable`, which is
the whole point: "empty" would read as *the detector examined the screen and found no
controls*, and "unavailable" would send somebody to reinstall a plugin that is working
perfectly. An outcome in this state carries no observations, no error and no reason, so nothing
appears in `Degraded` and nothing counts it as having observed the window.

## Consequences

- `cycleLooked` and any future admission rule must treat not-requested as silence rather than
  as an observation — it did not look.
- Any provider that gains `TargetedProvider` in future must carry its own trigger policy
  across. There is no shared base to inherit it from, which is a real cost of the interface
  design and is stated here rather than discovered again.

## Enforced by

- `internal/director/perception/providers/vision/optin_test.go` —
  `TestAnOrdinaryCycleDoesNotRunAVisionPass` (asserts the SCREEN WAS NOT CAPTURED),
  `TestARequestedCycleStillRunsAVisionPass`, `TestBothDoorsHonourTheSameOptIn`
- `cmd/director/structurewiring_test.go` — `TestANotRequestedProviderIsNotAnObservation`
