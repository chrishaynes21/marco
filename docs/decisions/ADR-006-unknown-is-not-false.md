---
type: decision
status: accepted
date: 2026-08-06
supersedes: []
affects:
  - control-flow
  - waits
  - verification
  - safety
---

# Unknown is not false

## Context

A wait asks a question about the world: is the Save dialog open? There are three possible
answers, and a boolean can hold two of them.

- **Satisfied** — it is open.
- **Unsatisfied** — the Director looked, and it is not open.
- **Unknown** — the Director could not look.

Collapsing Unknown into false is the natural implementation and it is dangerous in a specific
way: "I could not observe the confirmation dialog" becomes "there is no confirmation dialog",
and the next step proceeds as though something had been checked when nothing was.

The failure is always in the unsafe direction, because Unknown arises exactly when something
has gone wrong — a provider is down, the window vanished, the tree would not walk.

## Decision

**Condition evaluation is three-valued.** `Unknown` is a distinct state and is never
converted to `Unsatisfied`.

A wait that cannot observe does not silently pass and does not silently fail. It reports that
it could not look, with the evidence for why.

## Consequences

- Waits defer rather than retry when verification cannot be established, which is slower and
  correct.
- Every condition must be able to say "I have no way to evaluate this" — a caller with no
  region watcher gets `Unknown` for a region condition rather than a wrong answer.
- A world with no elements in it is not a world where nothing matched. It is a world that
  could not be read.
- Diagnostics have to distinguish the two states everywhere they surface, which is more
  reporting surface than a boolean would need.

## Enforced by

- **implementation** — `internal/director/wait/evaluation/evaluation.go`, where `Unknown` is
  a named state documented as "NEVER a synonym for false"
- **implementation** — `internal/director/wait/conditions/conditions.go`, whose evaluators
  return an explicit Unknown gate rather than a bare boolean
- **implementation** — `internal/director/wait/conditions/region.go` — an unevaluable
  condition returns Unknown
- **milestone record** — the *Unsatisfied is not Unknown* section of [[director-waits]]

## Related

- [[Control-Flow]], [[Programs]]
- [[ADR-007-no-progress-no-repetition]] — the same caution applied to retrying
