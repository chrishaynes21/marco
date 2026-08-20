---
type: decision
status: accepted
date: 2026-08-13
supersedes: []
affects:
  - learned-plays
source_paths:
  - internal/director/teach/teach.go
  - internal/director/rehearse/live.go
---

# ADR-055 — an authorised rehearsal waits for its start

*Recorded retrospectively: the decision was made and implemented on 2026-08-13 and the code
has referenced this number since; the note was never written. Content reconstructed from
`notReadyYet`, the `WaitingForStart` phase and `patient_test.go`.*

## Context

A rehearsal that fired once on the instant of approval and gave up made a person
choreograph their way back to a screen at a second they could not predict — test
scaffolding, not a product. The demonstration capture has been patient since it was
written: armed, and waiting until the person is standing on the start.

## Decision

A pre-input refusal that is about the WORLD — the source unobservable, unrecognised,
ambiguous, mismatched, the target lost or moved, and (since [[ADR-060-input-has-no-address]])
the window not being in front — sends the coordinator to `WaitingForStart`, and the same
call retries. Every member of the patient set shares two properties: nothing was emitted,
and the grant was not claimed — `BeginAttempt` compares scope before it spends. The bound
is the grant's own expiry, never a timer of Teach's own.

Everything else stays terminal: `no_actuator` is a fact about the build, and a spent,
revoked, expired or mismatched grant is the authority saying no.

## Enforced by

- `TestArrivingAtTheStartLaterRunsItExactlyOnce`,
  `TestVisitingOtherPlacesDoesNotConsumeTheGrant`,
  `TestARefusalWaitingCannotFixStillRefuses`,
  `TestBeingOnAKnownButWrongScreenWaits`, `TestWaitingNeverWatchesAnything`
  (`internal/director/teach/patient_test.go`)

## Related

[[ADR-023-rehearsal-is-attempt-scoped-authority]] · [[ADR-060-input-has-no-address]] ·
[[Learned-Plays]]
