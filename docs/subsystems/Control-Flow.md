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
updated: 2026-08-06
source_paths:
  - internal/director/wait
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

## Decisions

- [[ADR-006-unknown-is-not-false]]
- [[ADR-007-no-progress-no-repetition]]

## Validated by

- `internal/director/wait/*` unit tests
- `director wait`, `director trace`

## Known gaps

See the *Known gaps* section of [[director-cancellation]] and *Remaining sleeps* in
[[director-waits]].

## Milestone record

[[director-waits]] and [[director-cancellation]].
