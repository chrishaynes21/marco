---
type: decision
status: accepted
date: 2026-08-20
supersedes: []
affects:
  - invocation
  - service
  - learned-plays
  - visibility
source_paths:
  - internal/director/service/perform.go
  - internal/director/service/server.go
  - cmd/director/perform.go
---

# ADR-085 — a performance is a registry command

## PERFORM was the one mutating request that was not one

`PERFORM` arrives on `ObserveQuery`, beside `Point`, `Reach`, `Learn` and the rest. Every one of
those neighbours is a **read**. Perform is the single field there that drives real input — and it
was routed exactly like its read-only neighbours: straight into `Runtime.Observation`, with no
registry entry, no cancellable context, and no lifetime the service knew about.

What that cost, measured:

- `director status` said **nothing was running** while a play was typing and clicking.
- `director stop` answered **"nothing is running"**, because `Registry.Cancel` had no active
  command to find. `CANCEL_ACTIVE` could not reach a performance at all.
- a second mutating request was **accepted concurrently**, so two things drove one desktop.
- `rehearse.Live.Perform` checks `ctx.Err()` before EVERY step and has a `CancelledAttempt`
  terminal and a `RefusalCancelled` refusal ready. **All of it was dead code**: the only context
  ever handed in was `context.Background()`.

That last one is the shape [[Wiring-Tests]] describes — a complete, correct, well-tested
cancellation mechanism that nothing could reach. Every gate stayed green.

## The decision

**A performance enters the SAME command registry an executed phrase does, at the same layer.**

`Server.performGoal` calls `s.registry.Begin(s.ctx, requestID, phrase)`, hands the returned context
to `Performer.PerformGoal(ctx, q)`, and calls `s.registry.Finish(...)` with the state the walk
ended in. There is **no second registry and no second lifecycle** — this is `server.go`'s
`execute` pair, spelled once more for the one other request that can move the desktop.

Three properties follow, and they are the reasons:

- **It becomes visible.** `director status` names the phrase the Audience asked with, and
  `PerformView.Command` carries the id back.
- **It becomes refusable.** A concurrent mutating request is **refused, not queued** — two things
  driving one desktop is exactly what the mutating slot exists to prevent, and a performance that
  waited its turn would start against a screen the other command had already changed. It comes back
  as `Refusal: "busy"` on the normal response shape, so a client renders it rather than failing to
  decode it.
- **It becomes stoppable.** `CANCEL_ACTIVE` finds it, the context reaches the walker, and the walker
  stops at the next step boundary. **This is what makes `stop` work on a running Play** — the
  intake's control arm ([[ADR-083-one-invocation-intake]]) reaches the active execution authority,
  and before this there was nothing there to reach.

**`Performer` is a separate interface, asserted for rather than required.** A Director that only
observes is a legitimate Director — the CLI's own stub is one — and widening `Runtime` would have
made every implementer claim an ability to drive input in order to keep compiling. Asked for by
name, a Director either offers it or honestly does not (`Refusal: "no_performer"`).

**And nothing below invents its own context.** A `context.Background()` anywhere under a
performance is a branch of the walk that cannot be stopped, which is the defect this fixed;
`cmd/director/perform.go` has none left.

**Cancelled is its own command state.** "You stopped it" and "it failed" are different facts about
the same half-finished walk, and a history that rendered them alike would tell somebody their play
is broken when they are the one who stopped it. The word `cancelled` is borrowed from the walker's
own vocabulary rather than minted at the door, so the two cannot drift.

**Arrival, not completion, is what counts as done.** A walk whose every step verified and which
ended somewhere else records as `unverified`, never `completed` —
[[ADR-070-one-production-body-and-the-caller-brings-the-verification]].

## Considered and rejected

- **Leave PERFORM on the observation path and give it its own cancel channel.** A second
  cancellation mechanism beside `Registry.Cancel`, which the product already has three intakes for
  ([[34F-legacy-marco-product-audit]] §14). The whole direction of travel is one stop, not another.
- **Queue a performance behind the active command instead of refusing it.** It would start against
  a screen the earlier command had already changed, and every reading it took would be about a
  world it did not plan against. Refusing is the honest answer, and it is the one the mutating slot
  was built to give.
- **Widen `Runtime` with `PerformGoal`.** Every Director implementation — including test stubs and
  the observe-only CLI Director — would then claim the ability to drive real input to keep
  compiling. A capability that everybody must declare is not a capability.
- **Report a busy or cancelled performance as a transport error.** Both clients (`director perform`
  and `marco do`'s bridge) already decode a `PerformView` on `ResponsePerception`. Busy and
  cancelled are things to RENDER, not things that break decoding; the refusal vocabulary is where
  the difference belongs.
- **Record a cancelled performance as failed.** It is the one state the person themselves caused.

## Consequences, including the costs

- **A performance can now be refused for being second**, where before it simply ran. Somebody who
  presses a hotkey while a Director command is in flight gets a refusal instead of two things
  happening at once — which is correct, and which is a behaviour change a person will notice.
- **The registry's mutating slot is now shared by two very different durations.** A phrase is
  seconds; a learned Play walking several edges with verification between them can be much longer.
  Nothing bounds how long the slot is held, so a long performance blocks every other mutating
  request for its whole length.
- **`s.ctx` is the parent**, so a service shutdown cancels a performance mid-walk. That is the
  intended reading of shutdown, and it means a half-finished walk can leave the desktop changed —
  the same fact `stopped()` reports as "you stopped it after N of M steps".
- **The performance's own refusals and the registry's states are two vocabularies that must map.**
  `performState` is that mapping, and it is a place two accounts of one run can drift; it is pinned
  by test rather than by types because `internal/director/service` must not import the walker.
- **This is Phase 3's foundation, not Phase 3.** A locally-run (non-learned) Play still stops by a
  different route — the panic-stop hook and the overlay's child kill. One stop across all of them
  is still owed ([[34F-legacy-marco-product-audit]] §14).

## Enforced by

- `internal/director/service` — `TestStoppingAPerformanceReportsItAsCancelled`: deleting the
  Begin/Finish pair, or passing `context.Background()` again, fails it.
- `internal/director/service` — `TestAPerformanceIsVisibleToStatusAndRefusesAConcurrentCommand`:
  both halves of "it is a command" — status can see it, and a second mutating request is refused.
- `internal/director/service` — `TestACancelledPerformanceIsRecordedAsCancelled`: the state mapping
  itself, so a stopped walk is not filed as a failure.
- `internal/director/service` — `TestADirectorThatCannotPerformRefusesInsteadOfObserving`: the
  `Performer` assertion refuses honestly rather than falling through to the observation path.
- `cmd/director` — `TestNothingInAPerformanceInventsItsOwnContext`: the mutation gate for the
  original defect — any `context.Background()` reintroduced under a performance fails it.
- `cmd/director` — `TestStoppingBetweenEdgesEndsTheWalk` and
  `TestAStoppedPerformanceNeverForegroundsAnything`: the context actually reaches the walker, and a
  cancelled performance does not go on to move a window.
- `cmd/director` — `TestTheCancelledWordIsTheWalkersWord`: the literal at the door is held against
  the walker's constants.

## Related

[[Invocation]] · [[Service]] · [[Learned-Plays]] · [[Visibility]] · [[Wiring-Tests]] ·
[[ADR-083-one-invocation-intake]] · [[ADR-084-a-plays-identity-is-its-subject]] ·
[[ADR-078-a-learned-play-is-performed-by-the-director]] ·
[[ADR-070-one-production-body-and-the-caller-brings-the-verification]] ·
[[ADR-066-stop-is-a-product-event]] · [[34F-legacy-marco-product-audit]]
