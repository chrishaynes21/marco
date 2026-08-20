---
type: decision
status: accepted
date: 2026-08-20
supersedes: []
affects:
  - invocation
  - cancellation
  - learned-plays
  - overlay
source_paths:
  - internal/stopsignal/stopsignal.go
  - cmd/marco/panicstop.go
  - cmd/marco/intake.go
  - cmd/marco/stop.go
  - plugins/overlay/acts.go
  - internal/director/service/stop.go
---

# ADR-087 — one stop, and it crosses a process boundary

## The Audience has one word and it reached about a third of what was running

[[ADR-083-one-invocation-intake]] made `stop` a control phrase from every entrance, and
[[ADR-085-a-performance-is-a-registry-command]] gave the Director's own performances something for
`CANCEL_ACTIVE` to find. Both were true and neither was enough, because the thing a person is most
often trying to stop is a Play running in a **different process** from the one they said it to.

`marco do` performs an authored or recorded Play inside a short-lived child. The overlay starts one;
so does the control centre; so does a hotkey; so does a terminal. The stopper and the stopped rarely
share a process, and the engine could not name a sibling. So `runControl` had exactly one arm —
`director stop` — and a Play typing into a text box could not be stopped by saying "stop".

Underneath that, three separate faults, each of which alone would have been enough:

1. **`MARCO_NO_PANIC_STOP` answered a question it was never asked.** It exists so a front end that
   already owns a low-level keyboard hook does not get a second, competing `WH_KEYBOARD_LL` in its
   child — the hook invariants in `CLAUDE.md` are about exactly that thread. But the one `if` that
   implemented it returned `fn(context.Background())`, and nothing cancels a Background. So every
   Play the overlay spawned — which is most Plays anybody runs — was **uncancellable from the
   instant it started**.
2. **The overlay's cancel was a kill.** With nothing to cancel, `cancelRun` called `Process.Kill`.
   `TerminateProcess` runs no deferred function: `ReleaseHeld` and `CursorSnapshot` never fired, so
   a key the Play was holding stayed held after the person stopped it.
3. **`finally` did not run on cancellation anyway.** See
   [[ADR-088-cleanup-runs-when-the-audience-stops]]. Even a graceful cancel would have skipped the
   cleanup that releases the key.

And two things the stop word could not reach at all: a **Learn rehearsal**, which drives real input
and sat outside the command registry with a `context.Background()` of its own, and a **Learn
episode**, which `Registry.Begin` is never called for — so "stop" during a demonstration answered
*"nothing is running"* while the demonstration continued.

## The decision

**One stop, raised in one place, received by everything — and the rendezvous is the store, not a
process handle.**

`internal/stopsignal` keeps a monotonically increasing **generation** in a file under the
`$MARCO_HOME` every part of Marco already shares. Raising a stop writes a larger number. A running
Play reads the number when it starts and cancels itself when it sees a larger one.

```
Audience stop, from ANY entrance
  → invoke.Decide ⇒ KindControl → runControl, TWO arms, BOTH always run:
      (a) stopsignal.Raise   → every local Play cancels → runtime cancelTree
                             → `finally` runs → held input released
      (b) director stop      → CANCEL_ACTIVE → a registry command
                                             → a pending question
                                             → a Learn REHEARSAL   (a performance)
                                             → a Learn EPISODE     (a session, ADR-066 Cancel)
```

**A file, and a generation, because three constraints leave one honest answer.** The engine has no
external dependencies and no service dependency — `marco do` must work with the Director switched
off, and that rules out the Director as broker. The stopper usually has no handle on the stopped, so
process handles are the wrong currency and the shared store is the right one. And it must not be a
kill, because stopping has to arrive as a **cancellation** for the runtime to unwind through
`finally`.

It holds a NUMBER rather than a flag because a flag needs clearing, and whoever cleared it would race
the next Play into starting. A generation has no such moment: a stale file reads as "already
accounted for" and nothing has to tidy it up.

**`MARCO_NO_PANIC_STOP` now means only what it always meant.** It suppresses the hooks. Every run —
hooks or not, real input or dryrun — is watched. That single change is what makes the
overlay-spawned Play, previously the largest uncancellable class in the product, stoppable.

**The overlay asks before it kills.** `stopRunningPlays` raises the generation first and
synchronously, then arms a per-child timeout that terminates only what is still alive after
`stopGrace`. The kill survives as a last resort for a child that ignores the signal; it is no longer
the mechanism.

**A rehearsal is a performance; an episode is a session.** They get different mechanisms on purpose.
The rehearsal joins the command registry beside `PERFORM`, exactly as
[[ADR-085-a-performance-is-a-registry-command]] describes. The episode does not — it is reached by
routing `CANCEL_ACTIVE` through the **same surface operation** the existing cancel verb already
calls, so [[ADR-066-stop-is-a-product-event]]'s `Cancel` keeps exactly one
implementation.

**And "stop" is bound to Cancel, never Finish.** Stop is the abort word: it is what a person says
when something is going wrong. Finishing a demonstration is a positive act and keeps its own
affordance and its own honest name. Two controls labelled "stop" doing opposite things is what this
resolves — `director learn --cancel` and `--finish` say what they do, and `--stop` survives as an
alias for `--cancel` so no script breaks.

## Considered and rejected

- **Register running Plays with the Director.** It is the obvious rendezvous and it is the wrong one:
  `marco do` must work offline with no service running, and a stop that worked only when a daemon
  happened to be alive would be a stop a person could not trust.
- **Enumerate sibling processes and signal them.** Platform-specific, and it would put process
  enumeration in an engine whose whole discipline is the standard library and one host boundary.
  It also answers a harder question than the one being asked: not "which processes exist" but
  "which of them are mine, against this store".
- **Keep the kill and make it tidier.** No amount of tidying makes `TerminateProcess` run a deferred
  function. The held key is the whole point.
- **A flag file instead of a generation.** Needs clearing; clearing races the next Play. Described
  above.
- **Put the Learn episode into `service.Registry`.** It would give `CANCEL_ACTIVE` something to find
  in one line — and it would be a **second implementation of ADR-066's Cancel**, sitting beside the
  surface verb's, free to drift. An episode is not a command; it outlives its sessions
  ([[ADR-075-a-learn-episode-outlives-its-sessions]]).
- **Make a global "stop" FINISH a demonstration.** It is the reading that loses a person's work:
  somebody who says "stop" because a demonstration is going wrong would have the half-demonstration
  durably saved. ADR-066 exists because these are different sentences.
- **Poll faster than 100ms.** The interval is chosen against the thing being interrupted, not against
  a benchmark. 100ms is far below the moment a person starts wondering whether stop worked, and the
  cost is one stat of a twenty-byte file — off any hook thread, which is the constraint that actually
  binds here.

## Consequences, including the costs

- **A stop is a BROADCAST.** It stops every Play running against that store, not the one the person
  was looking at. For "stop" that is the right reading — but it means two unrelated Plays run
  concurrently cannot be stopped separately, and nothing warns that a second one was also ended.
- **There is now a file in the user's store** that nothing cleans up. It is twenty bytes, it is
  plainly named, and deleting it costs nothing — but it is one more thing in a directory people look
  at, and the store still has no note explaining what lives there.
- **Every run pays a polling goroutine**, including a dryrun and including a one-line Play that
  finishes in a millisecond. Deliberate: a stop that behaved differently while somebody was trying a
  Play out than when they ran it for real is a stop they would not come to trust.
- **The overlay's grace period is a guess about a person.** `stopGrace` sits above one poll interval
  and below the runtime's cleanup budget. Too short and it is a kill again; too long and stop feels
  broken. It is one named constant with its derivation written beside it, and it will need revisiting
  the first time a real Play holds something slow.
- **`unavailable` had to be redefined for control.** The common case is a Director that is not
  running and a local Play that was stopped, and reporting that as "nothing was delivered" would be
  false. `unavailable` is now reserved for both arms failing.
- **A stop cannot be undone and does not report who heard it.** A Play that had already finished is
  not an error to have asked to stop; broadcasts do not get receipts. So the product cannot say "I
  stopped three things", and it does not try to.

## Enforced by

- `internal/stopsignal` — `TestAStopRaisedAfterAPlayStartedIsHeard` and
  `TestAStaleStopDoesNotStopTheNextPlay`: the mechanism, and the reason it is a generation.
- `cmd/marco` — `TestOneStopReachesALocalPlayAndTheDirector`: deleting either arm of `runControl`
  fails it. `TestASpawnedPlayIsCancellableWithoutHooks`: restoring the `context.Background()` early
  return fails it. `TestAStopWithNoDirectorRunningIsNotUnavailable`: the common case reports
  `cancelled`. `TestMarcoStopEntersTheOneIntake`: the verb goes through `runInvocation` like
  everything else, rather than growing a fourth intake.
- `plugins/overlay` — `TestAStopAsksTheStoreBeforeItKillsAnything`,
  `TestAStopReachesAPlayTheOverlayNeverSpawned`, `TestAChildThatIgnoresTheStopIsTerminatedAfterTheGrace`.
- `cmd/director` — `TestStoppingReachesALiveRehearsal`,
  `TestNothingThatCanReachTheWalkerInventsItsOwnContext` (a tree walk, not a named file — the
  previous guard was hard-coded to `perform.go`, which is how the identical defect survived one file
  over), `TestStopOnTheCommandLineStillMeansCancel`, `TestCancelAndFinishStayApart`.
- `internal/director/service` — `TestStoppingDuringADemonstrationCancelsTheEpisode`.

## Related

- [[ADR-088-cleanup-runs-when-the-audience-stops]] — the other half; a stop that skipped `finally`
  would have released nothing
- [[ADR-083-one-invocation-intake]] — where the control phrase is recognised
- [[ADR-085-a-performance-is-a-registry-command]] — the Director-side half, and its own closing note
  that one stop was still owed
- [[ADR-066-stop-is-a-product-event]] — untouched, and the reason "stop"
  means only one of them
