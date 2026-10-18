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
  - internal/director/binding/named.go
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

## Naming a thing is saying which thing

Measured live, on the same file in the same window: "rename this file to summary.txt" completed
and "rename notes.txt to summary.txt" failed at step three with *this step acts on the editor for
an object nothing bound*. The procedure's editor step acts on the editor for a BOUND object —
which is why it works at all, because a details view has an Edit control per column per row.
Pointing bound the file; naming it did not.

An object the person NAMED now binds as readily as one they pointed at, through one binder and
one assembly site. A name is stronger evidence than a gesture, not weaker: "this file" means
whatever they were looking at, and "notes.txt" is a name they chose. So the clauses are the same
shape — one match, several is an ambiguity, none is a refusal — and what differs is how the
object is found.

Two things it had to get right. A named binding is exempt from revalidation's focused-or-selected
requirement, because that rule is about ATTENTION and nobody's attention identified a name. And a
name can identify an object before anything can classify it — a kind comes from a backing path,
and a file manager reports the path for its SELECTION, so an unselected row cannot be called a
file and selecting it is step one of the procedure that wants it. The resolution is an ordering:
identify by name now, carry `KindUnconfirmed`, and check the kind at revalidation, after the
harmless select and before the rename. A folder called "notes.txt" is refused there.

See [[ADR-226-naming-a-thing-is-saying-which-thing]].

## A step may name its own control

A directive aims at a control three ways, in this order: by ROLE, which resolves through the
role's whole alias table and is therefore the localisable form; by LABEL, which resolves through
the one name the control bore when it was demonstrated; and, for the object the procedure is
ABOUT, by a bound identity. The second of those is new, and it exists because a demonstration of
three Settings navigations extracted as *open the focused control* twice — a navigation item is
not one of the semantic control roles, so a constant step was keeping nothing at all about what
it had been aimed at.

`Directive.TargetLabel` is separate from `Directive.Target` because the claims differ: a step
carrying a `Target` is BOUND to the procedure's object, and routing an incidental label through
it aimed every step of a learned navigation at the bound file. A label-aimed step is tied to the
language it was demonstrated in, which is what the evidence supports and also the one targeting
form a pixel-only surface can fill.

See [[ADR-227-a-step-may-name-its-own-control]] and
[[Experiment-074-two-halves-of-teach-and-replay]], which measures the whole teach-and-replay
workflow and reports that naming and reachability are the halves that do not exist.

## Decisions

- [[ADR-227-a-step-may-name-its-own-control]]
- [[ADR-226-naming-a-thing-is-saying-which-thing]]
- [[ADR-005-legal-marco-only]]
- [[ADR-007-no-progress-no-repetition]]

## Validated by

- `internal/director/goal` unit tests against fixtures built from a real captured tree
- `goal.NewValidatedRegistry` — registry validation enforced at service startup and in every
  CLI command
- `director goals`, `director procedures [name]`, `director explain goal "<request>" [--app X]`,
  `director goal --dry-run "<request>"`

## Known gaps

- **`goal.OpenFile` has no phrasing.** Nothing in the pattern table produces it, so
  `genericOpenFile` has never been reachable from a request — and neither is a learned procedure
  approved to override it. `director goal "open gaming"` answers *Not a goal*. Adding a bare
  `open` prefix would reroute every *"open the file menu"* away from `uiact`, because `IsGoal` is
  consulted first, so this is a routing decision rather than a missing line. Measured in
  [[Experiment-074-two-halves-of-teach-and-replay]].
- **A demonstration cannot be given a name.** `demo` reads one as a better way to achieve a goal
  Director already has a word for, generalizing the first step into the procedure's parameter and
  freezing the rest, so there is nothing for a person's name to name. Same experiment.
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
