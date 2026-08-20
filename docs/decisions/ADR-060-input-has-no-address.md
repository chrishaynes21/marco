---
type: decision
status: accepted
date: 2026-08-17
supersedes: []
affects:
  - learned-plays
source_paths:
  - internal/director/rehearse/live.go
  - internal/director/rehearse/rehearse.go
  - internal/director/learn/learn.go
  - cmd/director/rehearserun.go
---

# ADR-060 — input has no address

Emitted input goes to whatever window leads the desktop, and nothing on the rehearsal path
targeted a window. The live shape: the person answers "yes, try it" in a terminal, the
terminal holds the foreground, and the rehearsal fires its keys into the terminal — then
honestly reports that the watched application never moved. It reads exactly like "the
rehearsal never fired", and it cost a live evening.

(The same evening exposed the second half: every live rehearsal that DID land was
classified against the empty application, because the step record never carried its scope —
`StepRecord.Application`/`Source` are now set and mutation-gated by
`TestALiveStepIsClassifiedAgainstItsOwnApplication`.)

## The decision

A REAL attempt asks the platform whether the watched window is in front, and refuses
`window_not_in_front` when it is not — **before the grant is claimed**, so nothing is
spent, and re-checked before every step, so a window that falls behind mid-route stops the
attempt where it stands rather than typing into what took its place.

Upstream, `window_not_in_front` joins the patient set
([[ADR-055-an-authorised-rehearsal-waits-for-its-start]]): the window comes forward the
moment the person clicks back into it, and waiting spends nothing. The patient loop is also
paced now (`teachStartPoll`) — each retry is a full perception pass, and a flat-out loop was
a busy-wait wearing patience's clothes.

A yes that creates no authority is also no longer silent: the runner records the closed
reason (`AuthorizationRefusal`), the tail carries it up (`teach.GrantDiagnoser`), and the
eventual timeout blames the failed authorization rather than the person.

## Enforced by

- `TestARealAttemptRefusesWhileTheWindowIsBehind`,
  `TestAWindowFallingBehindMidRouteSendsNoFurtherInput`,
  `TestALiveStepIsClassifiedAgainstItsOwnApplication`
  (`internal/director/rehearse/route_test.go`)
- `TestAWindowBehindTheForegroundWaits` (`internal/director/learn/patient_test.go`)
- `TestAYesThatCreatedNoAuthorityIsSaidOutLoud` (`internal/director/learn/tail_test.go`)

## Related

[[ADR-023-rehearsal-is-attempt-scoped-authority]] ·
[[ADR-055-an-authorised-rehearsal-waits-for-its-start]] · [[Learned-Plays]]
