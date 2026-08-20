---
type: subsystem
status: active
owners:
  - director
depends_on:
  - fusion
  - control-flow
  - marco-boundary
used_by:
  - goals
  - collections
  - demonstrations
updated: 2026-08-06
source_paths:
  - internal/director/program
  - internal/director/execute
  - internal/director/plan
  - internal/director/verify
---

# Programs

A program is an ordered sequence of semantic steps, validated as a whole and executed one at
a time with verification between them.

Not a batch. A batch commits to a plan made against a world that has since changed; a program
re-observes between steps, which is what makes the difference between "click Save then click
Yes" working and it clicking whatever now occupies the coordinates Yes used to.

## Responsibilities

- Decompose a request into steps — by **splitting**, never by inferring an unstated step.
- Validate the whole request before performing any of it.
- Execute sequentially, verifying each step before the next.
- Maintain program context across steps.
- Refuse cleanly when busy.

## What it may not do

- Infer a step nobody asked for.
- Proceed past a step it could not verify — see [[ADR-007-no-progress-no-repetition]].
- Reach the desktop by any route but lowering — see [[ADR-005-legal-marco-only]].

## Related systems

- [[Goals]] — the layer above, which produces programs
- [[Semantic-Actions]] — what an individual step means
- [[Control-Flow]] — waits as step preconditions, and cancellation
- [[Action-Graph]] — what execution records
- [[Marco-Boundary]] — how a step reaches the desktop

## Decisions

- [[ADR-005-legal-marco-only]]
- [[ADR-006-unknown-is-not-false]]
- [[ADR-007-no-progress-no-repetition]]

## Validated by

- `internal/director/program`, `internal/director/execute` and `internal/director/verify`
  unit tests
- `cmd/director/dryrun_test.go`, `cmd/director/runtime_test.go`

## Known gaps

See the *Known gaps* section of [[director-programs]].

## Milestone record

[[director-programs]] — the model, decomposition, whole-request validation, verification,
cancellation, program context.
