---
type: adr
status: accepted
date: 2026-10-19
decision: "ACCEPTED — seven conditions found by review of the supervised-run lifecycle are closed, under three rules. (1) A STOP IS ONLY A STOP AT THE LAST MOMENT IT COULD MATTER: the stop key is read once more immediately before input is sent, it is bridged onto the executor's own context, it is honoured before a runner exists, and a consent question may not consume it. (2) AUTHORISATION IS NOT EVIDENCE: the screen a click is aimed at is re-measured after a person authorises, not re-read from the look that produced the question. (3) A RECORD IS THE ATTEMPT IT NAMES: a run id carries randomness, its record is written with O_EXCL, its frames live under its own id, and its one-word outcome is read from how far the workflow got as well as from each action's verdict — so an incomplete run is not `verified` and a window holding partial positive evidence still expires at its deadline."
supersedes: []
affects:
  - execution
  - safety
source_paths:
  - cmd/director/performworkflow.go
  - cmd/director/consentdesk.go
  - cmd/director/runrecord.go
  - internal/director/workflowrun/workflowrun.go
  - internal/director/workflowrun/verify.go
---

# ADR-240 — a stop is only a stop at the last moment it could matter

## The gap

The supervised-run lifecycle landed in `cec983d`, `7ae9dd1` and `22fd169`: an explicit Start, a
countdown with no input during it, a visible Stop, a readable final result, and a run reference
tied to the evidence. All of it was tested and all of the tests passed.

A review sweep of the same code then traced seven conditions none of those tests described. Six
of them are not about whether a feature works — they are about **when** it works, and the
answer in each case was "not at the moment that matters":

1. **Screen validation used stale evidence after authorisation.** The grounder measured the
   target against the look that had produced the consent question. A person takes a second or
   two to press F7; the check answered for the screen as it was *before* they did.
2. **F9 could arrive after the final stop check and still permit a click.** The stop flag was
   read, then the executor was called, and between those two statements nothing was watching.
3. **F9 during startup could dereference a nil runner.** The stop path reached for a runner the
   run had not constructed yet.
4. **A partially completed run could be recorded as `verified`.** The classifier returned
   `verified` on any positive verdict count, so a workflow whose first step verified and whose
   second refused was, as a whole, a success.
5. **Opening a consent question could discard F9.** The desk drained pending keys before posing
   its question and threw away whatever it found, including a stop.
6. **Partial positive evidence bypassed the verification deadline.** A window holding one
   qualifying reading waited for more, unconditionally — so `--verify-for` existed and could
   not expire.
7. **Evidence could be overwritten or attributed to a different run.** Two attempts in one
   second shared an id; `os.WriteFile` truncated; `--frames` wrote to one directory for every
   run.

## The decision

Three rules, each of which generalises past the condition that found it.

### A stop is only a stop at the last moment it could matter

A stop that is read early and acted on late is not a stop, it is a prediction. So the flag is
read again immediately before `exec.Do`, and — because a single executor call can itself take
long enough for a person to change their mind — `stopping(ctx)` bridges the flag onto the
context the executor already watches. `stopper atomic.Pointer[workflowrun.Runner]` lets the key
handler record a stop before any runner exists and have the run honour it when one does. And
the consent desk's drain loop now inspects what it drains: a `ConsentStop` found there calls
the stop, instead of being counted and dropped.

### Authorisation is not evidence

A person's approval is permission to act on the world as it is *now*. `freshLook` takes a real
look, fed through the same observer, so the inputs `worldMoved` reasons about describe the
present rather than the moment the question was asked. This costs one capture per authorised
action and buys the only guarantee that mattered: a click lands on the control a person said
yes to.

### A record is the attempt it names

A reference whose single job is retrieval must retrieve. Four hex characters of system
randomness make an id unique within a second; `O_EXCL` turns a collision into a refusal
instead of a silent substitution; frames go under `<root>/<run-id>/`. And the one-word outcome
is a property of the run, not of its best action: `rep.Progress.Complete()` gates `verified`,
`rep.Pressed > decided` catches an action nobody ever answered for, and a verification window
with partial positive evidence expires `unconfirmed` at its deadline like any other.

## What this does not do

Two limitations were traced in the same sweep and are **not** closed here, because both are
larger than a lifecycle fix:

- **Control-centre Stop** (`cmd/marco/watchui.go:426`) reaches service cancellation without the
  standalone-play cancellation broadcast, so Stop there does not reach a standalone play.
- **Inter-step verification** (`internal/director/workflowrun/workflowrun.go`) can advance a
  different-screen step on destination arrival while a previous step's outcome verification is
  still pending.

Both are named so the next session does not rediscover them.

## Enforced by

- `cmd/director/freshconfirm_test.go` — `TestTheTargetIsCheckedAgainstAFreshLook`: a target
  that moved between the question and the approval is refused.
- `cmd/director/consentstop_test.go` — `TestADrainedStopIsHonoured`: a stop queued before a
  consent question ends the run.
- `cmd/director/runlifecycle_test.go` — stop injected from the hover and from the confirm
  callback; stop before a runner exists.
- `cmd/director/runrecord_test.go` — an incomplete run with one verified action is not
  `verified`; a pressed action with no verdict is `unconfirmed`.
- `cmd/director/runevidence_test.go` — two runs in one second get two records; a second write
  to one id is refused; frames land under the run id.
- `internal/director/workflowrun/deadlinehold_test.go` —
  `TestOneQualifyingReadingDoesNotWaitForever`: one qualifying reading and then silence expires
  `unconfirmed` with no cursor advance.
- `internal/director/workflowrun/laststop_test.go` — a stop arriving after the last check sends
  no input.

## Links

- [[ADR-235-three-records-and-only-one-of-them-may-act]]
- [[ADR-238-a-verdict-needs-both-sides-and-a-window-of-its-own]]
- [[ADR-239-running-what-you-demonstrated-by-the-name-you-gave-it]]
- [[Experiment-093-seven-conditions-a-green-suite-did-not-describe]]
