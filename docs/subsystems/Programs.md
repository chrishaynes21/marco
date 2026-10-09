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
updated: 2026-10-08
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

## Splitting is a claim about somebody's words

"And" is not always a separator. *"Type hello and goodbye"* is one instruction whose TEXT
contains a conjunction, so a clause that does not begin like an instruction is rejoined to the
one before it — and a cut inside a quoted run is refused outright.

Two facts the splitter must not hold a private copy of, both found by one task:

- **Which words begin an instruction.** It kept forty of them by hand, and the list did not
  learn "change" when the editor did; the clause became the tail of an **append** and went into
  the document as text. The editing vocabulary is now derived from
  [[Editing]]'s own literals.
- **What was cut.** Fragments were rejoined with a hard-coded `" and "`, so "add milk, eggs and
  bread" came back as "add milk and eggs and bread". Nothing failed and the document held a
  list nobody wrote. The separator travels with the fragment and is restored verbatim.

See [[ADR-142-the-splitter-keeps-no-vocabulary-of-its-own]].

## A step can wait for its target to exist, and now go and find it

A step is resolved against the reading taken for THAT step, which is what makes "open File, then
click Save" possible — Save does not exist until File has been clicked. A target that is not in
that reading used to end the program.

It no longer has to. A resolution of `absent` earns a bounded search
([[ADR-143-a-search-is-a-wait-with-a-move-in-it]]), and a step that finds its target carries on
from the reading it was found in. That is the whole of the continuation: one request, and nobody
asked to insert "scroll down" between two clauses they did not think of as separate.

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

- [[ADR-143-a-search-is-a-wait-with-a-move-in-it]]
- [[ADR-142-the-splitter-keeps-no-vocabulary-of-its-own]]
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
