---
type: adr
status: accepted
date: 2026-10-17
decision: an object the person NAMED binds as readily as one they pointed at, through one binder and one assembly site — resolved by looking for the object that bears the name, identified thereafter by its backing path or the source's own id rather than by the name, and exempt from the focused-or-selected requirement that exists for deixis; a name whose object cannot yet be classified binds with its kind UNCONFIRMED and that kind is checked at revalidation, before anything is changed
supersedes: []
affects:
  - execution
source_paths:
  - internal/director/binding/named.go
  - internal/director/binding/resolve.go
  - internal/director/goal/expand.go
  - internal/director/execute/binding.go
---

# ADR-226 — naming a thing is saying which thing

## What was measured

Live, on the same file in the same Explorer window:

```
rename this file to summary.txt    →  COMPLETED, summary.txt on disk
rename notes.txt to summary.txt    →  FAILED: step 3 of 4 (set the name to
                                      "summary.txt") did not complete: this step acts
                                      on the editor for an object nothing bound, so
                                      there is nothing to look for an editor of
```

The procedure's third step acts on the editor for a BOUND object, and that is the whole reason
it works: a details view has an Edit control per column per row, and typing into the wrong one
changes a caption and renames nothing. Pointing bound the file. Naming it did not, because
`binding.Resolve` is entirely deictic — it reads focus and selection and never looks at the
phrase except to quote it in a refusal.

## Why a name is not weaker evidence than pointing

It is stronger. "This file" means whatever the person was looking at, and the entire deictic
path exists to pin that down before their attention moves. "notes.txt" is a name they chose,
and the only question is which observed object bears it.

So the clauses are the same shape and the same vocabulary: the kind must satisfy what the
procedure declared, exactly one object must match, several is an ambiguity the person can
settle, none is a refusal. What changes is how the primary object is found — and `bindingFor`
is now one assembly site shared by both paths, because the identity evidence a binding records
must not differ by how the object was chosen.

## Three things it had to get right

### The details view's Name cell is never the answer

A details view has a `text_field` bearing the same text as its row. A text field classifies as
a text selection — positively not a file — so it is excluded by the kind check rather than by a
rule about Explorer.

### A named binding is exempt from the focused-or-selected requirement

Revalidation's third rule refuses when the object is found but no longer focused or selected,
because *"binding to the newly focused object is exactly what a careless implementation does,
and it is how 'rename this file' renames whatever the user happened to click"*.

That rule is about ATTENTION, and nobody's attention identified "notes.txt". A named binding is
re-checked by identity — the backing path first, then the source's own id — and that
requirement is dropped for it and kept for everything else. `Binding.ByName` records which,
so revalidation does not have to guess from the evidence list.

### A name can identify an object before anything can classify it

The hard part, and it is a chicken and egg. A kind comes from a backing PATH, and a file
manager reports the path for its SELECTION. So an unselected item called "notes.txt" cannot be
classified as a file at all — and selecting it is step one of the procedure that wants it.

The resolution is an ORDERING rather than a relaxation:

- **Identify by name now.** An item whose kind is INDEFINITE — no path, so nothing has said
  what it stands for — is eligible, and the binding carries `KindUnconfirmed`. An item whose
  kind is definite and WRONG is not eligible.
- **Confirm the kind before anything is changed.** An unconfirmed binding may not take
  revalidation's unchanged-world short-circuit, so it is re-identified and classified before
  the step that would act on it. A folder called "notes.txt" is refused there — after the
  harmless `select`, before the rename.

What that leaves: an unselected named row's only identity is the source's own id, because *a
label alone is not identity* — two files can share a name in different folders. If the tree is
rebuilt between expansion and the first step, re-identification fails and the request refuses.
Refusing is the safe direction and the rule that makes it refuse is the one worth keeping.

## Where the binding was being thrown away

Worth recording, because the first version of this change did not work and the live run said
so with exactly the message it had before. Three sites asked `d.TargetDeictic`:

1. `bindDirective`, which decides whether to bind at all — changed first, alone, and the
   expansion loop went on gating on deixis, so the binding was made and discarded.
2. The expansion loop, twice: whether to bind, and whether to hand the binding to the step.
3. `reference`, where a named target built a literal reference carrying no binding id.

One predicate now answers all of them — `bindsTheirObject` — because the sites have to agree.
`ex.Deictic` stays deictic-only: a planning-only expansion of a POINTED goal is unrunnable by
construction, and a named one is perfectly runnable from its label.

## What it does, live

```
rename notes.txt to summary.txt
  [1/4] select "notes.txt"            — verified
  [2/4] invoke the rename command     — verified
  [3/4] set the name to "summary.txt" — verified against the control's own value
  [4/4] confirm the name              — verified
  COMPLETED: 4 of 4 steps verified    (4 action(s) performed)
```

`summary.txt` on disk, `other.txt` untouched, and no confirmation prompts — the request
authorized its own steps ([[ADR-225-a-request-authorizes-the-task-it-asked-for]]).

## The limitation this leaves, stated plainly

**A named rename is verified by its steps and not against the filesystem.** The deictic version
ends with *"…notes.txt became …summary.txt, unchanged, and nothing else moved"* — a correlation
against the real file, read from the backing path. `prepareRenameCheck` needs that path BEFORE
the program runs, to digest the file's contents while it still exists under its old name, and a
named binding has no path until step one selects it.

So the strongest verification Marco has is unavailable on the named path, and the cost is
precise: a named rename that renamed the wrong file would be reported as done. Closing it means
capturing the check between step one and the rename step, which is a hook into the program loop
that this change did not earn.

## Enforced by

`internal/director/binding` `TestANamedObjectBindsByItsName`,
`TestANameThatFindsNothingOrSeveralThingsRefuses`,
`TestANamedBindingSurvivesTheSelectionMovingAway` (and the deictic binding still refused in the
same world, so the rule is narrowed rather than removed),
`TestAnUnconfirmedKindIsCheckedAtRevalidation` (a folder refused, a file accepted, and an
unconfirmed binding unable to skip the check).
