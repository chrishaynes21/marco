---
type: decision
status: accepted
date: 2026-08-06
supersedes: []
affects:
  - execution
  - collections
  - programs
  - safety
---

# No verified progress means no repetition

## Context

An action did not visibly work. Retrying is the reflex, and it is right about a minority of
cases — a transient miss, a control that was not ready yet.

For the rest it is actively harmful. The most common reason an action appears not to have
worked is that it *did* work and verification could not see it. The second most common is
that it genuinely does not work, which repeating will not fix and which repeating makes
worse. An edit that could not be confirmed, applied twice, has silently doubled.

The same failure appears in a collection loop: without a progress test, an iteration that
cannot advance re-acts on the same member forever.

## Decision

**Repetition requires verified progress.** An action is repeated only when the Director has
positive evidence that the previous attempt did not take effect — never merely because it
could not confirm that it did.

Progress is *classified*, not assumed. An iteration that cannot demonstrate progress on the
current member stops rather than advancing or retrying.

## Consequences

- The Director will sometimes stop when a retry would have worked, and will say why.
- Idempotence is never assumed on the Director's behalf. Some steps are safe to repeat, but
  that is a property of a specific step, declared, not a default.
- Collection loops terminate by construction rather than by an iteration cap alone.
- This interacts with [[ADR-006-unknown-is-not-false]]: "could not verify" is Unknown, and
  Unknown does not authorise a repeat.

## Enforced by

- **implementation** — `internal/director/execute/pipeline.go`: *"an edit that could not be
  confirmed is not repeated, because repeating it may apply it twice"*
- **implementation** — `internal/director/execute/iterate.go`: *"Iteration stopped: no
  verified progress for the current member."*
- **implementation** — `internal/director/execute/edit.go` — the multi-step double-apply case
- **milestone record** — *Progress is classified, not assumed* in [[director-collections]];
  *Verification* in [[director-programs]]

## Related

- [[ADR-006-unknown-is-not-false]]
- [[Collections]], [[Programs]], [[Editing]]
