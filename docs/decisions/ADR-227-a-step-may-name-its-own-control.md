---
type: adr
status: accepted
date: 2026-10-17
decision: a demonstrated step keeps the LABEL of the control it acted on, as a directive field of its own — separate from the object the procedure is about, so that the procedure's binding is not attached to a control it merely passes through; a role still outranks a label wherever one was recorded, and a label-aimed step is language-bound by construction rather than by oversight
supersedes: []
affects:
  - execution
source_paths:
  - internal/director/demo/extract.go
  - internal/director/demo/procedure.go
  - internal/director/goal/goal.go
  - internal/director/goal/expand.go
---

# ADR-227 — a step may name its own control

## What was measured

Three navigations in Settings, recorded as one demonstration and all three verified —
`"go to bluetooth settings"`, `"go to accounts"`, `"go to gaming"`, each `COMPLETED`. The
extraction of them:

```
1 open the object the user points at
2 open the focused control
3 open the focused control
```

The first step became the procedure's parameter. The other two lost what they were aimed at
**entirely** — and the proposal's own words hid how badly. *"open the focused control"* is what
`Describe` prints for a step with nothing to describe, and a step with nothing to aim at does not
reach a focused control: `SemanticOpen` needs a target, so the procedure the person was being
invited to approve **could not expand at all**. It would have refused at step 2 with *open needs
a control to act on*, having already performed step 1.

That is the mutation run's finding rather than a live one, and it is why the test deletes the
branch rather than asserting the text: a proposal that reads plausibly and cannot run is worse
than one that reads wrong.

The cause is one line and not a subtle one: `CandidateStep` carried `Role` and no label, and a
settings navigation item is not one of the semantic control roles — so a constant step kept
nothing at all about what it had been aimed at.

## Why a label is allowed to be kept here

This package's refusal is specific and already written down: *nothing here captures pixels,
handles, runtime ids, element ids, OCR text or plaintext secrets, and the tests assert that over
the serialised form*. A label is on the other side of that line. It is what the control said it
was, which is semantics — the same evidence `Target.Role` is, read less precisely.

**A role still outranks it.** A role resolves through its whole alias table, so "the control that
discards changes" finds *Nicht speichern* as readily as *Don't Save*; a label resolves through
the one name it bore on the machine where it was demonstrated. The ordering is therefore role
first, label second, nothing third — and third is what a navigation sequence used to get.

## The defect the fix's own test found

The first version put the label in `Directive.Target`, which was wrong, and the test said so
before any live run did: all three steps of the learned procedure came out aimed at `Alpha.txt`,
the file the fixture's binder had bound.

`Directive.Target`'s own doc says why: *"A USER-supplied name — a file, a folder — rather than a
UI label."* A directive carrying a `Target` is **bound** — that is the whole reason
[[ADR-226-naming-a-thing-is-saying-which-thing]] works, because a later step can then ask for
"the editor for the thing I bound". Feeding it the label of a control the procedure merely passes
through attached the procedure's object to every one of them.

So the label gets a field of its own, `Directive.TargetLabel`, and its branch in `reference`
builds a literal query and **no binding and no expected kind**. The two are different claims:

| | what it says | bound |
|---|---|---|
| `Target` | the object this procedure is ABOUT | yes |
| `TargetLabel` | a control this step named on the way | no |

## The cost, stated rather than hidden

A label-aimed step **is tied to English**, or to whatever language the demonstration was given
in. That is not an oversight to fix later; it is what the evidence supports. Nothing named that
navigation item by meaning, so a label is the most the recording holds, and the alternative on
offer was a step aimed at nothing.

It is also the field a pixel-only surface can fill. A game menu yields no role and no element id,
and what OCR reads off it is a label — so the one targeting form that survives the loss of the
accessibility tree is the one this ADR adds.

## Enforced by

`internal/director/demo` `TestANavigationSequenceKeepsWhatEachStepOpened` (the proposal says
which destinations, and no step of the expanded program is aimed at nothing) and
`TestARecordedRoleOutranksTheLabelItBore` (the rename command resolves through its role's alias
table, not through the single label it bore). `TestNothingMechanicalSurvivesExtraction` continues
to hold the line over the serialised form.
