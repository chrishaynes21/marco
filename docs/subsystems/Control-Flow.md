---
type: subsystem
status: active
owners:
  - director
depends_on:
  - fusion
used_by:
  - programs
  - goals
  - collections
updated: 2026-10-08
source_paths:
  - internal/director/wait
  - internal/director/search
  - internal/director/execute
---

# Control Flow

Waits, conditions and cancellation — how the Director decides *when* to proceed and how it
stops.

## Waits are conditions, not sleeps

A sleep encodes a guess about how long something takes on this machine today. A condition
asks the world a question and takes evidence for an answer. Waits are step **preconditions**
throughout the Director; the remaining sleeps are enumerated in [[director-waits]].

## Three-valued, always

- **Satisfied** — the condition holds.
- **Unsatisfied** — the Director looked, and it does not.
- **Unknown** — the Director could not look.

Unknown is never converted to Unsatisfied. See [[ADR-006-unknown-is-not-false]]. Verification
**defers** instead of retrying when it cannot establish an answer.

## Searching is this loop with a move in it

A wait looks at the same place repeatedly and waits for the place to change. A **search** replaces
the pause with a bounded change to what can be seen, and is otherwise the same machinery: the
same conditions, the same three values, the same Unknown gate, the same hard bound.

It adds one answer a wait has no need for. `exhausted` means every place the pattern could reach
was looked at **and read**, and is the only thing that licenses saying a control is not there —
a single failed look only says "not in this reading". A place that could not be read makes the
answer `unreadable` instead, because that place is one the thing could have been in.

The moves a search may make are a **closed set**, because it makes them without being asked:
today, a semantic scroll and nothing else. The view goes back when nothing was found.

See [[ADR-143-a-search-is-a-wait-with-a-move-in-it]].

## Cancellation

Two modes, audited. Cancellation is not timeout and timeout is not interruption — the
distinctions and where sequences stop are in [[director-cancellation]].

**Held-input safety** is part of this: a cancelled run releases held keys and restores the
clipboard, so an interrupted Director cannot leave a key stuck down or a clipboard borrowed.

## Responsibilities

- Evaluate conditions against evidence, with an explicit Unknown gate.
- Drive the wait loop, and report why it is still waiting.
- Stop sequences at well-defined boundaries.
- Guarantee held-input and clipboard release on every exit path.

## What it may not do

- Compile the wait engine into Marco. It stays on the Go side deliberately — the reasoning is
  in [[director-waits]].

## Related systems

- [[Programs]] — waits gate steps
- [[Collections]] — the pause and the lock
- [[Editing]] — borrows the clipboard, never takes it
- [[Perception]] — what a search looks THROUGH; it observes by the same path a command does

## Decisions

- [[ADR-143-a-search-is-a-wait-with-a-move-in-it]]
- [[ADR-006-unknown-is-not-false]]
- [[ADR-007-no-progress-no-repetition]]

## Validated by

- `internal/director/wait/*` and `internal/director/search` unit tests, all non-actuating
- `director wait`, `director trace`

## Known gaps

See the *Known gaps* section of [[director-cancellation]] and *Remaining sleeps* in
[[director-waits]].

## Milestone record

[[director-waits]] and [[director-cancellation]].
