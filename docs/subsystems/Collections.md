---
type: subsystem
status: active
owners:
  - director
depends_on:
  - programs
  - fusion
used_by:
  - goals
updated: 2026-08-06
source_paths:
  - internal/director/collections
  - internal/director/execute/iterate.go
---

# Collections

"Close every Notepad window." A collection is a **bounded semantic query, re-run every
iteration** — never a captured list of ids or handles.

## Why a query and not a list

A captured list is a snapshot of a world that is about to change, and it changes *because of
what the loop is doing*. Closing the first window renumbers the rest; a list of handles
becomes a list of stale handles halfway through. Re-running the query means the loop always
acts on what is there now.

## Responsibilities

- Bound the query by construction, not by an iteration cap alone.
- Make ordering explicit.
- Identify members **semantically**, not positionally.
- Verify before advancing — see [[ADR-007-no-progress-no-repetition]].
- Treat empty as observable: "there were none" is a result, not a failure to look.
- Protect against ordinal drift across a clarification pause.

## Bulk is a second policy gate

A closed allowlist: `focus` and `activate` may proceed in bulk. **A bulk click always asks.**
The gate is separate from the per-action policy because "do this once" and "do this to
everything I can see" are different requests, and consent to the first is not consent to the
second.

## Related systems

- [[Programs]] — the loop belongs to the Director, not to the target application
- [[Semantic-Actions]] — the verb applied per member
- [[Control-Flow]] — the lock and the pause

## Decisions

- [[ADR-007-no-progress-no-repetition]]
- [[ADR-006-unknown-is-not-false]]

## Validated by

- `internal/director/collections` unit tests
- `cmd/director/lockrule_test.go` — the control-plane lock
- `director collections`, `director explain collection`

## Known gaps

See the *Known gaps* section of [[director-collections]].

## Milestone record

[[director-collections]] — membership identity, drift across a clarification pause, lifetime,
the control-plane lock.
