---
type: subsystem
status: active
owners:
  - director
depends_on:
  - programs
  - semantic-actions
used_by:
  - service
updated: 2026-08-06
source_paths:
  - internal/director/goal
  - cmd/director/goalcmd.go
---

# Goals

The layer above [[Programs]]: the user describes an **outcome** and the Director produces the
program. 15 goals expand through **18 hand-written typed procedures** into ordinary
`program.Program`s.

Goals: `rename`, `create_folder`, `save`, `save_as`, `print`, `delete`,
`close_without_saving`, `duplicate`, `download`, `move`, `copy`, `paste`, `open_file`,
`open_settings`, `create_tab`.

## Procedures are hand-written, never generated

A procedure is typed Go, reviewed like any other code. Nothing downstream changed when this
landed — variables, collections, clarification, replay and verification all keep working
untouched — because the output is an ordinary program validated by the ordinary validator.

## Responsibilities

- Select the most-specific procedure for a request and application.
- Declare safety **before** expansion: mutations, destructive/external/irreversible,
  confirmation required.
- Refuse missing requirements as a typed question before anything runs.
- Express waits as step **preconditions**, never sleeps.

## Application overrides

Selected most-specific-first. `explorer rename` reaches Rename from the context menu;
`vscode rename symbol` is a **different operation** that shares the word and demands
confirmation. Ambiguity is detected deterministically rather than resolved by registration
order.

## Hardening

- Goal-level confirmation is enforced between expansion and step 1; accepted, rejected and
  unavailable are distinguished, and unavailable never means yes.
- Goal provenance is persisted on action-graph nodes as **diagnostic metadata that replay
  never reads**.
- Procedure labels are semantic ControlRoles with localized alias tables, and destructive
  choices require an exact match.
- Best-effort steps skip only on a demonstrably absent target.

## Related systems

- [[Programs]] — the output
- [[Semantic-Actions]] — the verbs a procedure emits
- [[Demonstrations]] — the other way a procedure comes to exist

## Decisions

- [[ADR-005-legal-marco-only]]
- [[ADR-007-no-progress-no-repetition]]

## Validated by

- `internal/director/goal` unit tests against fixtures built from a real captured tree
- `goal.NewValidatedRegistry` — registry validation enforced at service startup and in every
  CLI command
- `director goals`, `director procedures [name]`, `director explain goal "<request>" [--app X]`,
  `director goal --dry-run "<request>"`

## Known gaps

- **None of this is live-tested.** Every claim is against fixtures, not a running
  application. The live rename has never completed.
- The editor's `ValuePattern` reported *Unsupported* from PowerShell against the real
  control while the cached walk reported `value`. The edit ladder's fallback has never run
  against it, and is the likeliest thing to break on the next live attempt.
- `editorClasses` knows one application and one property.
- ~~**Safe bindings** are implemented and unit-tested but **not wired into the execution
  path**.~~ **Wrong, corrected 2026-08-06.** `internal/director/binding` is production-active:
  bind at goal expansion, re-resolve per step before the policy gate, re-resolve again on
  replay. Proven live through `director execute --dry-run`, not by reading imports. The claim
  had been stale for some time and nothing failed when it went stale — see [[Wiring-Tests]].
- No live scenario has ever run; they now skip with precise unmet prerequisites.
- Still missing: action-level confirmation for non-goal actions, replay confirmation policy,
  and the live harness itself.

## Milestone record

[[director-goals]] — procedures, overrides, preconditions, safety, hardening, and the
per-claim status table.
