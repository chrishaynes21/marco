---
type: subsystem
status: active
owners:
  - director
depends_on:
  - action-graph
  - goals
used_by:
  - goals
updated: 2026-08-06
source_paths:
  - internal/director/demo
  - cmd/director/democmd.go
---

# Demonstrations

Recording what somebody did, and extracting a **semantic procedure** from it — the second way
a procedure comes to exist, beside the hand-written ones in [[Goals]].

## Recording adds no observation path

The recorder reads the action graph. It does not open a new way of watching the screen, which
means demonstration recording inherits every constraint perception already has rather than
introducing a parallel set.

## What a step keeps, and what it cannot

A recorded step keeps the verb and the query. It cannot keep a coordinate, a handle or a
window generation, for the reasons in [[ADR-009-window-identity-is-ephemeral]].

Goal recovery then reads the actions and proposes what the demonstration was *for*.

## What becomes a parameter

The pieces that varied, or that are obviously an argument — a filename, a target. This is
proposed and shown, never inferred silently.

## Approval is a step nothing skips

A learned procedure is an **ordinary procedure**: same type, same validator, same safety
declaration, same downstream path. It does not get a privileged route into execution because
a person demonstrated it.

## Goal-centric since 2026-08-17

A demonstration is EVIDENCE, never the capability. `A → B → C` decomposes into one
candidate per grown edge; the leg arriving where the person stopped is the one the Learn tail
(`learnTail`) carries forward, and the destination becomes a durable goal in the person's own
words ([[ADR-056-a-goal-is-a-destination-not-a-route]]). Attributed input survives every failure
of interpretation ([[ADR-057-attributed-input-survives-interpretation]]), and a click that
resolved to a named control is as learnable as a keypress
([[ADR-058-a-demonstrated-target-may-keep-its-name]]) — it rehearses as an invocation of
that control, though a saved play still cannot say it (`cannot_say_pointer`, follow-on).

## Responsibilities

- Record from the action graph.
- Recover a goal from a sequence.
- Propose parameters.
- Refuse what cannot be represented.
- Require approval before a procedure becomes usable.

## Related systems

- [[Action-Graph]] — the source
- [[Goals]] — where a learned procedure lands
- [[Programs]] — unchanged by any of this

## Decisions

- [[ADR-009-window-identity-is-ephemeral]]
- [[ADR-005-legal-marco-only]]
- [[ADR-053-a-search-box-on-the-last-page-is-not-a-step]]
- [[ADR-054-the-one-shot-candidate-belongs-to-the-session]]
- [[ADR-056-a-goal-is-a-destination-not-a-route]]
- [[ADR-057-attributed-input-survives-interpretation]]
- [[ADR-065-operating-marco-is-not-demonstrating-to-it]]
- [[ADR-066-stop-is-a-product-event]]
- [[ADR-058-a-demonstrated-target-may-keep-its-name]]
- [[ADR-074-one-demonstration-every-leg-reviewed]] — every leg of one demonstration is reviewed,
- [[ADR-075-a-learn-episode-outlives-its-sessions]] — session boundaries are not Audience workflow
- [[ADR-077-consent-is-the-audiences-authority-is-marcos]] — a yes may never be reported as a
  decline; an answer.s application comes from the question
  boundaries; the newest Runner is no authority on an older answer
  in walk order, and an absent question is not an answer

## Validated by

- `internal/director/demo` unit tests (`demo.go`, `record.go`, `validate.go`)
- `director demo` commands; explainability output

## Known gaps

See the *Known gaps* section of [[director-demonstrations]].

## Milestone record

[[director-demonstrations]].
