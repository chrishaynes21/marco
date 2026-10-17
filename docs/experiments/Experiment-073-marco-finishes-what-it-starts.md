---
type: experiment
status: complete
date: 2026-10-17
backend:
  - accessibility
fixture: "the owner's live desktop under standing authorization for ordinary reversible operations. File Explorer pointed at a disposable temp folder, and Notepad for the case that must fail. An isolated MARCO_HOME throughout. Every folder and file created was removed afterwards; no setting was changed."
result: all-five-bounded-workflows-completed-or-failed-honestly-with-four-defects-found-live-and-fixed-including-a-false-success-the-first-outcome-check-produced-itself
supersedes: []
affects:
  - execution
  - perception
source_paths:
  - docs/decisions/ADR-224-a-procedure-is-finished-when-the-person-has-what-they-asked-for.md
---

# Experiment 073 — Marco finishes what it starts

Five bounded workflows on real applications, run to completion or to an honest failure. The
threshold this was aimed at: not how many controls Marco resolves, but whether it finishes a
person's task.

## The five

| | asked | result | on disk |
|---|---|---|---|
| 1 | "make a new folder called Reports" | `COMPLETED: list_item "Reports" is there` (4 actions) | `Reports` |
| 2 | "rename this file to summary.txt" | `COMPLETED: …notes.txt became …summary.txt, unchanged, and nothing else moved` (3 actions) | `summary.txt` |
| 3 | "make a new folder" → *"what should it be called?"* → "Quarterly" | `COMPLETED: list_item "Quarterly" is there` (4 actions) | `Quarterly` |
| 4 | the intermediate control disappearing — task 1's step two | `verified: the menu closed, taking all 5 of its items` | — |
| 5 | "make a new folder called Ghost", in Notepad | `FAILED: nothing matching "New folder" is present` | nothing changed |

Task 1, in full, after the fixes:

```
[1/4] invoke the New menu     — verified
[2/4] choose the Folder entry — verified   (the menu closed, taking all 5 of its items)
[3/4] set the name to "Reports" — verified against the control's own value
[4/4] confirm via keyboard · enter · after 2 stronger option(s) were unavailable
      verify  the application now says "New folder" → "Reports"
COMPLETED: list_item "Reports" is there        (4 action(s) performed)
```

## Four defects, all found by running it

### One: Marco verified the mechanism it used, not the transition the press was for

The measured failure. Choosing Explorer's Folder entry created the folder, and the verifier
reported *"Folder" is no longer in the world, so its selection could not be read* — because it
asked whether the control it pressed still reported itself selected. A menu closes when you
choose from it.

Fixed by reading the menu's closing as evidence, with a second and separate piece of evidence
for whether anything appeared OUTSIDE the menu. Both are needed: Escape closes a menu too.

### Two: a procedure abandoned at a step that had nothing to do

Found by running task 2. "Rename this file to summary.txt" with the file ALREADY SELECTED
selected it again, nothing changed, and verification said *"notes.txt" reports itself selected
and nothing else changed to confirm it* — a four-step rename abandoned at step one.

The step was not unverifiable. It was unnecessary, and those are different sentences with
opposite consequences. A step whose state already holds is now counted and the procedure
continues; no record is written, because nothing was performed.

This removes the limitation [[ADR-221-a-question-the-person-already-answered]] recorded, and
removes it for the reason the record gave for leaving it: it turned out to cost a whole task.

### Three: `confirm` clicked a text field

The ladder is Invoke → click → Enter, which is right for a dialog's OK button. On an in-place
rename there is no OK button, the confirming control IS the editor, and clicking a text field
puts a caret in it. The rename was abandoned and Enter sat one rung below the click that did
nothing.

### Four: the outcome check produced a false success — and it was the new code that did it

The worst of the four, and worth recording in full because it is the same trap as the first one
in a place I had just finished congratulating myself about.

The first version of the goal-outcome check reported:

```
COMPLETED: text_field "Reports" is there
```

It was there. **Marco had typed it there.** The folder on disk was still "New folder". The
condition — something on screen called "Reports" — was satisfied by the edit box holding
Marco's own keystrokes, which is evidence about Marco and not about anybody's disk.

So a control Marco wrote to cannot be the proof that the application accepted what was
written. The program hands the outcome check every control its steps acted on, and none of them
may be the evidence. With that, the same request reports `list_item "Reports" is there` — a
list item, which is what a folder is.

**The lesson is the one this whole experiment is about.** "Verify the outcome, not the
mechanism" is easy to agree with and easy to implement wrongly, because the mechanism is
usually the nearest thing holding the right text.

## Two more things that were asked for

### A question nobody could answer

`Client.Clarify` existed and no CLI command called it. Marco could ask *"create a folder — what
should it be called?"* and there was no way to answer from a terminal: an ordinary `execute`
SUPERSEDES a pending question by design, so the answer became a request to find a control
called Quarterly. `director clarify "<answer>"` is the second half of that conversation, and
the answer completes the ORIGINAL request through the goal parser's own patterns — checked by
re-parsing, so an answer that cannot be fitted back is refused rather than reshaped.

### An email address in a status line

Measured in [[Experiment-071-marco-goes-where-you-ask-and-says-when-it-need-not]]: Marco's
ordinary verification line read *"keyboard focus moved from Chris Haynes
chris.haynes2112@gmail.com to Settings"*. Correct, and not diagnostic information.

Every label this package quotes now goes through one rule: quote what looks like a control's
NAME, describe what looks like CONTENT. An address is never repeated; a label past 48
characters is described by its length; a multi-line label is described. The role and the id
survive, which is all a diagnostic needs to be followed up.

The bound is measured rather than chosen — p50 = 9, p95 = 28, longest single-line addressable
label 47, across the 282 addressable labels of the desktop corpus. The first draft said 40 from
recollection and the package's own test caught it hiding a real Settings control.

And the rule is enforced by reading the package's own source, because a careful `labelOf`
beside twenty raw `%q`s is decoration. That audit found five more sites than I had, including
one quoting a text field's VALUE — "changed from X to Y" about a document, which is two
versions of somebody's writing in a status line.

## What still does not work

- **A rename by NAME.** "Rename notes.txt to summary.txt" fails at step three: the procedure's
  editor step acts on the editor for a BOUND object, and naming a file does not bind it. Only
  the deictic form — "rename this file" — binds, so the named form needs the binding layer to
  accept a label. Reported, not fixed.
- ~~The confirmation gate on every `confirm`.~~ **Fixed 2026-10-17** by
  [[ADR-225-a-request-authorizes-the-task-it-asked-for]]: a request authorizes the declared steps
  of the procedure it expanded into, and `make a new folder called Reports` now completes from one
  request with no prompts. `delete scratch.txt` still asks once, names the file and the effect,
  and leaves it alone when declined.
- **Paraphrase**, unchanged, and still the interpreter's job rather than a rule's.

## Task completion, counted

Five workflows, five honest outcomes: three completed with the requested state observed on the
filesystem or in a list item Marco did not write to, one verified intermediate transition, one
refusal that changed nothing. **No false successes in the final runs, and one found and fixed
along the way.** Three confirmations were answered by hand; no clarification was unnecessary;
the one question asked was the one argument missing.

## Enforced by

`internal/director/verify` holds the menu evidence and the label rule, `internal/director/goal`
the missing-argument reconstruction, `internal/director/execute` the continuation past a step
that needed nothing, and the gates named in
[[ADR-224-a-procedure-is-finished-when-the-person-has-what-they-asked-for]]. This record is the
live confirmation rather than the gate.
