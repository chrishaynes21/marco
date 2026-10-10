---
type: decision
status: accepted
date: 2026-10-10
affects:
  - passive-observation
  - learned-plays
source_paths:
  - internal/director/learn/control.go
  - internal/director/learn/learn.go
  - cmd/director/learnsessionwiring.go
---

# ADR-155 — a learn session has one owner, and control is a message to it

## Context

`go test -race ./cmd/director` reported ten data races across five tests on 2026-10-09, found
while race-checking the new capture goroutine of
[[ADR-152-slow-discovery-fast-tracking]] and unrelated to it. Every one of the ten was
`learn.Coordinator.Cancel` or `learn.Coordinator.Finish` against `learn.Coordinator.Advance`.

`learn.Coordinator` had no mutex anywhere on the type. `learnSession.start` runs `Advance` on its
own goroutine, in a loop, each call of which may block for the length of a perception pass —
forty-five seconds by `DefaultBounds`. Meanwhile the person may say they are done
(`learnSession.finish`) or abandon the attempt (`learnSession.stop`), and a surface may draw a
panel (`Session`), all from other goroutines. `cancelled` and `finished` were plain bools, and the
whole `Session` struct — four growing lists among them — was read by a surface while a step
appended to it.

The field comment claimed a guarantee the language does not give:

> *cancelled is set by Cancel and checked at every phase boundary, so a cancel that lands mid-pass
> takes effect the moment the pass returns rather than being overwritten by it.*

An unguarded read is not a stale read. There is no happens-before edge between the write and the
read, so the compiler may hoist the load out of the loop and the processor may never see the
store. **`director learn --cancel` was not guaranteed to stop a demonstration.** The ordinary
suite was green and always had been: this is invisible without `-race`.

Dates: the fields are from 2026-08-19, seven weeks before the finding.

## Decision

### The problem is ownership, not shared mutable state

Exactly one goroutine at a time has any business writing a Coordinator's fields. The other two
want to **say** something to it and to **read** what it last concluded. So those are three
different things with three different mechanisms, rather than one lock over everything:

- A **step** owns `Coordinator.s` and everything beside it, start to finish. Nothing else reads
  or writes those fields while it runs — which is why the thirty-odd phase handlers needed no
  locking and were left untouched.
- **Control** goes through `control`: a small box with its own mutex holding the two decisions a
  person can make, two flags saying whether each has been carried into the session yet, and a
  published snapshot.
- **Observation** is `Session()`, which returns that snapshot.

### A lock held across `Advance` would be the wrong fix

It is the obvious one and it is worse than the bug. That lock is held across a perception pass and
across every callback out to `Passes`, `Tail`, `Grounding` and `Memory`. `Cancel` would then block
for up to the length of a watch, waiting for the very pass it is trying to stop, and a surface
drawing a panel would block with it.

**Cancellation that waits on the thing being cancelled is not cancellation.** `control.mu` is
never held across a pass, a callback, or a durable write — only across flag arithmetic and one
struct copy, which is the smallest critical section that can publish and hand back ownership
atomically.

### A control call that arrives mid-step leaves a message; one that arrives with nobody there acts

This is the phase-boundary behaviour the old comment described and did not have.

`Cancel` and `Finish` check `stepping` under `control.mu`. If a step owns the state they record
the decision and return; the step applies it at its boundary, in `release`. If nothing owns the
state they take ownership themselves and apply it — because a session nobody is advancing any
more would otherwise never apply its own cancellation, and `learnSession`'s driver exits on
`ctx.Done()` without another `Advance`.

`release` re-checks after each application and publishes under the same lock it gives ownership
back in. Both halves matter: without the re-check, a decision that landed while the first was
being applied is recorded by nobody; without the single critical section, a cancel that arrives
between the publish and the handover is deferred to a step that has already finished.

### What a caller sees the instant `Cancel` returns is `Cancelled`

The authoritative transition — the account line and the withdrawal of a pending learning request
— happens on whichever goroutine owns the state. But the **published phase** is set to `Cancelled`
immediately, so `learnSession.stop` still returns a cancelled session to the person who asked and
nobody reading in between is told the demonstration is still running.

### Terminal states are consistent and idempotent, and a save beats a cancel

`applyCancel` returns early on a settled phase. Three consequences, all wanted:

- Three cancels are one decision and one account line.
- A cancel on an already-refused or already-complete attempt changes nothing.
- A cancel that loses the race to a step which **wrote the play** leaves the play alone, and in
  particular does not withdraw the learning request from under it. The old eager `Cancel` *did*
  withdraw it in that case, then let the step's terminal phase stand — a declined request beside
  a saved play. That is fixed here as a side effect of putting the transition in one place.

### `Session()` returns a snapshot, not a view

The same rule and the same bug as `observe.ScreenSegmenter.States`
([[ADR-153-a-blackout-is-not-a-destination]]'s companion commit): the value travels to another
goroutine while the step goes on appending to `Account` and `Diagnostics` and writing `Edges[i]`
in place. `Session.clone` copies the four growing lists, and each note's `Facts`, into fresh
backing arrays.

The pointer fields are **shared on purpose**, and that is a claim about them: each is assigned
once from a value the step has finished with — `Attempt` from a local, `Demonstration` and
`Assessment` from a pass result, `Saved` from the write that created it — and nothing in the
package writes through one afterwards.

### A step that cannot claim the state waits rather than spins

`claim` waits on a condition variable while another goroutine owns the state, instead of
returning false. The alternative is a driver loop calling a no-op `Advance` as fast as it can
until a control call lets go. The wait is bounded by what control calls do; a step never waits on
a pass, because only the driver takes steps.

**This one is a cost argument and no test holds the line on it.** Mutation testing says a `claim`
that returns false instead of waiting passes every test here, because the spin resolves in
microseconds and the attempt still settles. Said out loud rather than enforced, because an
enforcement claim nobody can fail is worse than none.

## What is NOT decided

**No Learn behaviour changed.** Same phases, same bounds, same refusals, same account, same
driver loop in `learnSession`. The two observable differences are both consequences of the rule
above rather than new policy: a cancel applied at the boundary now writes the same account line
the eager path wrote, and a cancel that lost to a save no longer withdraws that save's request.

**The rest of `cmd/director` is not audited for races.** This fixes the ten reports that existed.
`go test -race` is still not part of the documented gate — `go build ./... && go test ./...` is —
and making it one is a separate decision with its own cost, because the race build is roughly
four times slower on this package.

## Enforced by

- `TestACancelThatLandsMidPassIsNotLost` — the pass is held open on a channel, the cancel lands
  inside it, and the attempt ends once with the line that says why and without establishing a
  start for a session nobody is learning.
- `TestCancellationDoesNotWaitForThePassItIsCancelling` — the liveness claim that rules out the
  one-big-mutex fix, for `Cancel` and for `Session`.
- `TestFinishDuringAPassEndsTheCaptureAtOnceAndIsRecordedOnce` — the capture ends before the step
  boundary, and the account line lands exactly once however many goroutines could have written
  it.
- `TestRepeatedCancellationIsOneDecision` — idempotence, plus: a cancelled attempt runs no
  further passes.
- `TestASecondCancelDuringAPassDoesNotSwallowTheFirst` — the case the settled check alone misses,
  and the reason `Cancel`'s own guard exists. Found by mutation testing rather than by reasoning:
  with a step in flight the first cancel publishes `Cancelled` before applying it, so a second
  one falling through to the settled branch would mark the decision applied, the step would skip
  it, and the cancel would be lost exactly the way the race lost it.
- `TestCancelLeavesASettledAttemptAlone` and `TestACancelNeverUnsettlesAnAttemptThatFinished` —
  the two terminal-state rules, including that nothing is withdrawn from under a saved play.
- `TestAPublishedSessionIsASnapshotNotAView` — each list checked by writing through the copy,
  which is deterministic where append-and-hope is not.
- `TestTheNameAPlayWillHaveIsPublishedBeforeAnybodyDemonstrates` — the regression this change
  introduced and the snapshot made possible: `Session()` stopped returning the live struct, and
  `New` publishes before `WithPlayName` runs, so `learnSessionView.WillBeCalled` was silently
  empty on every start response. Nothing in the existing suite noticed.
- `TestControlAndAdvanceDoNotRaceEachOther` — everything at once, which is what the five
  `cmd/director` tests were doing by accident. Under `-race` it is the regression test for the
  ten reports.

Eight mutants of the above, one per rule, were run and all eight died.

## Related

- [[ADR-048-learn-teach-and-do-are-three-different-sentences]] — what `Finish` and `Cancel` mean
  as instructions, and why they are not each other.
- [[ADR-051-one-demonstration-and-an-attempt]] — the lifecycle these two calls interrupt.
- [[Passive-Observation]] — where the gap was recorded before it was fixed.
