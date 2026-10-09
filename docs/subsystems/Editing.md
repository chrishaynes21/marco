---
type: subsystem
status: active
owners:
  - director
depends_on:
  - semantic-actions
  - control-flow
used_by:
  - goals
  - programs
updated: 2026-10-08
source_paths:
  - internal/director/edit
  - internal/director/execute/edit.go
  - cmd/director/editcmd.go
---

# Editing

Semantic text entry: "focus the search box then type Director and press enter".

## Why this is not "type into the box"

Typing is a sequence of keystrokes sent to whatever has focus. That is not an edit — it is a
guess that focus is where it was a moment ago, that the field is empty or that appending is
correct, and that every character arrived. Each of those has been wrong in practice.

## The strategy ladder

Set the value through the accessibility pattern where the control supports it; fall back to
select-all-then-type; refuse rather than append blindly. As with [[Semantic-Actions]], the
rung is chosen from the control's **real** capabilities and every rejected rung is recorded.

## The clipboard is borrowed, never taken

Paste-based strategies save and restore the clipboard, on every exit path including
cancellation. A person's clipboard is theirs; an automation tool that eats it is a tool
people stop trusting. See [[Control-Flow]].

## Finding text by what it says

`ReplaceText{Old, New}` searches the value the editor has already read. It needs no new
perception — the editor reads a control's contents before writing them, so finding "milk" is a
string search over a value in hand. Found once, it is replaced; found nowhere or more than once,
it refuses and says which. See [[ADR-141-finding-text-by-what-it-says]].

This is the only operation here whose result is **predictable in advance**, which makes it the
only one verified against a computed string rather than a weaker property of one. Everything
else depends on a caret, a selection or an application's undo history, none of which the World
Model observes.

## Verification, and why a failed edit is not retried

An edit that could not be confirmed is **not repeated**, because repeating it may apply it
twice. This is [[ADR-007-no-progress-no-repetition]] in its most concrete form.

Two measured failures shape the arms. A `replace_selection` that grew the value by exactly the
length of the new text replaced **nothing** — the selection was empty and the text was inserted
at the caret — and a `set_text` whose control exposes no value pattern reads back empty whatever
it holds, which is inconclusive rather than failed. [[ADR-006-unknown-is-not-false]].

## Responsibilities

- Establish focus deliberately.
- Choose an entry strategy from real capabilities.
- Wait on conditions rather than sleeping.
- Verify the resulting value.
- Restore borrowed state.

## Related systems

- [[Semantic-Actions]] — the same ladder pattern
- [[Goals]] — `rename` and friends depend on this
- [[Control-Flow]] — waits and clipboard restoration
- [[Marco-Boundary]] — `ClipboardGet`/`ClipboardSet` were declared for this

## Decisions

- [[ADR-141-finding-text-by-what-it-says]]
- [[ADR-007-no-progress-no-repetition]]
- [[ADR-006-unknown-is-not-false]]
- [[ADR-005-legal-marco-only]]

## Validated by

- `internal/director/edit` unit tests
- `internal/director/execute/edit.go` double-apply guard
- `director edit` diagnostics

## Known gaps

- The editor's `ValuePattern` reported *Unsupported* from PowerShell against the real
  Explorer control while the cached walk reported `value`. **The select-all-then-type
  fallback has never run against it**, and it is the likeliest thing to be wrong on the next
  live attempt.
- A commit's final value is not recorded — the editor is gone by the time it is looked at, so
  the snapshot's `FinalValue` is filled by the edit step rather than the commit.
- See *Known limitations* in [[director-editing]].

## Milestone record

[[director-editing]] — the operations, the ladder, focus, verification, phrases understood.
