---
type: subsystem
status: active
owners:
  - director
depends_on:
  - programs
  - semantic-actions
used_by:
  - demonstrations
  - service
updated: 2026-08-06
source_paths:
  - internal/director/actiongraph
  - internal/director/execute/replay.go
  - cmd/director/graph.go
  - cmd/director/resume.go
---

# Action Graph

The record of what the Director did, in a form that can be replayed against a world that has
since changed.

## What it stores, and what it refuses to store

It stores the **verb** and the **query**. It never stores the mechanism.

That distinction is the whole design. A replay that reproduced the mechanism would reproduce
a click at a coordinate, a pattern call on a handle, a keystroke sent to a window — all of
which are facts about the moment of recording. Storing the verb and the query means a replay
**chooses the lowering again**, against the controls that exist now.

It also carries no coordinates in a resource identity, which
`TestResourceIdentityCarriesNoCoordinates` enforces — see
[[ADR-009-window-identity-is-ephemeral]].

## Goal provenance is diagnostic only

Goal expansion writes provenance onto nodes so a person can ask where a step came from.
**Replay never reads it.** Metadata that execution consumed would be a second, invisible
input to behaviour.

## Responsibilities

- Record verb, query, status and provenance per node.
- Support replay, resume and inspection.
- Keep positional data out of identity.

## Related systems

- [[Semantic-Actions]] — supplies the verb
- [[Programs]] — supplies the sequence
- [[Demonstrations]] — recovers a goal by reading the actions
- [[Marco-Boundary]] — re-lowers on replay

## Decisions

- [[ADR-009-window-identity-is-ephemeral]]
- [[ADR-005-legal-marco-only]]
- [[ADR-007-no-progress-no-repetition]]

## Validated by

- `internal/director/actiongraph` unit tests
- `TestResourceIdentityCarriesNoCoordinates` (`internal/director/boundary_test.go`)
- `director graph`, `director history`, `director trace`

## Known gaps

- Replay confirmation policy is **not implemented** — see the Known gaps of [[Goals]].

## Milestone record

*Action graph* sections of [[director-programs]], [[director-semantic-actions]] and
[[director-demonstrations]].
