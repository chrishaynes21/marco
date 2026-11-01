---
type: adr
status: accepted
date: 2026-11-01
decision: "ACCEPTED — A ROW THAT EDITS A PRESS OPENS WITH WHAT THE PRESS DOES, SAYS IT ONCE, AND IS ONE LINE AT AN ORDINARY WIDTH. The Architect, reading `8-rows-editor-normal.png`: *the coordinate row has no action verb, and X/Y split across lines. Keep Click · window-relative X/Y · controls readable as one row at normal width; stack the fields deliberately at narrow widths.* One appearance, three causes, and only two of them were in the product. THE MISSING VERB IN THE PICTURE WAS THE PICTURE'S. The render stub wrote a step as `click at 120, 90`, which is not a sentence `oneStep` writes; the page opens a step on a closed list of prefixes and has no `click at `, so the action column came out empty in a render and is `Left-click` on the desktop. The stub was corrected and the six states re-rendered — a picture of a page that nobody could reach is worse than no picture. BUT ONE KIND REALLY HAS NO VERB. `orchestrator.PointIn` has always offered a bare cursor move the same edit a click gets, and `oneStep` had no sentence for a `move` at all, so it fell through to its own kind name — and a point row REPLACES the value half of itself with the two boxes somebody moves the press by, so the one word it had was thrown away. A number, a badge and two fields, with nothing saying what happens at that place. `oneStep` writes `move the pointer to …` now, and the page's table opens it. The guard is not that sentence: `TestEveryKindThatPressesSomewhereHasAVerb` reads the kind names out of `macroir`'s own const block, asks `PointIn` which of them press anywhere, and requires a verb for each — so the next one of these is caught when it is added rather than when somebody photographs it. AND THE BUTTON WAS WRITTEN TWICE. [[ADR-289-the-panel-says-less-and-play-is-bigger]] cut the row's `Terms` to the button alone, which was right about the measurements and wrong about what was left: a step is written as a sentence and the row splits it into an action column and a value column, so `Left-click` was already the first thing on the row while `left` trailed the numbers at the end of it. One fact in two places, with the quiet copy in the column somebody scans. `Terms` is gone from `MovablePoint` and from `StepRow`. AND THE WIDTH WAS MEASURED, NOT REASONED ABOUT. The four columns cost about 302px before the value cell has anywhere to go; in a 1000px window the cell is 238px and the row wanted 289, so it wrapped — `x` at the end of one line with the number it names on the next. The row refuses to wrap now, the badge is one word (`WINDOW`, `SCREEN`) because two more said nothing the one does not, the boxes are 4em, and the label travels with its box at every width. That brings the row to 206px, and the narrow block that stacks it moves from 760 — where the cell is 35px and long past arguing about — to 960, which is where it measurably stops fitting."
supersedes: []
affects:
  - execution
source_paths:
  - plugins/studio/frontend/index.html
  - plugins/studio/recording.go
  - plugins/studio/stepedit.go
  - internal/orchestrator/pointedit.go
---

# The press row says what it does

## What happened

The Architect, on the rendered states that came with
[[ADR-290-a-routine-opens-where-it-is]]:

> One visible issue remains in **8-rows-editor-normal.png**: the coordinate row has no action
> verb, and X/Y split across lines. Keep **Click · window-relative X/Y · controls** readable as
> one row at normal width; stack the fields deliberately at narrow widths.

## Three causes, and one of them was not the product

### The render was lying

The scratch harness that makes those pictures stubs the backend, and its step list said

```
{at: 1, says: 'click at 120, 90', point: true, movable: true, in_window: true, x: 120, y: 90}
```

`oneStep` never writes that. A click is `left-click at 120, 90 in the window (1280×800, 96 dpi,
notepad.exe)`, and the page splits a step into an action column and a value column by stripping a
known prefix — a closed list that has `left-click at `, `right-click at ` and `middle-click at `
and no bare `click at `. So the stub produced an empty action column in a picture, and the
desktop produced `Left-click`.

The stub is corrected and all six states are re-rendered. **A picture of a page nobody can reach
is worse than no picture**: it is read as evidence, and the two true findings beside it were read
off the same frame.

### A `move` really has no verb

`orchestrator.PointIn` accepts `StepClick` **and** `StepMove`: a bare cursor move is a press-shaped
step with a position, and it has always been offered the same edit a click gets. `oneStep` had no
sentence for it, so it fell through to its own kind name, `move`.

That is tolerated for a row of text — `TestEveryStepSentenceTheBackendWritesHasAVerb` says so, and
such a step keeps its whole sentence in the value column rather than losing the front of it. It
is **not** tolerable for a press, because the step editor draws a press differently: it REPLACES
the value half with the two boxes somebody moves it by. So the only part of the sentence left on
the row is the verb, and a sentence with no verb leaves a number, a badge, two fields and nothing
saying what happens at that place.

`oneStep` writes `move the pointer to 120, 90 in the window` now. But the sentence is not the
guard — a twelfth step kind that presses somewhere is the next one of these. So the guard reads
`macroir`'s own const block for the kind names, asks `PointIn` which of them press anywhere, and
requires a verb for each.

### And the button was written twice

[[ADR-289-the-panel-says-less-and-play-is-bigger]] cut the row's `Terms` from
`left · of 1280×800, 96 dpi, notepad.exe` to `left`, on the reasoning that the measurements are
the same promise on every row and belong in Help while the button is what the press *does*.

The first half was right. The second half missed that **the button was never missing**: a step is
written as one sentence, the row splits it, and `Left-click` was already at the front of the row
in the column a person scans — while `left` sat after the numbers in a muted machine face at the
end of it. One fact in two places, and the copy that was hard to see is the one that survived.

`Terms` is gone from `orchestrator.MovablePoint` and from `StepRow`. What replaces the assertion
is narrower and truer: the sentence OPENS with what the press does.

## The width

Measured at ten of them, because none of it can be read off the markup.

| the row | |
| --- | --- |
| number | 28px |
| action | 7.5em — the longest verb the page knows is `Middle-click` |
| value | whatever is left |
| controls | `Set` and `Remove` |

The fixed columns and their gaps cost about **302px** before the value cell has anywhere to go,
and every hundred pixels off the window takes ninety-one off that cell. In a 1000px window the
cell is **238px**; the row wanted **289** and was allowed to wrap, so `x` sat at the end of one
line with the number it names on the next — two controls called nothing, under a badge saying
which basis they are in.

Four changes, and then the row needs **206**:

- **It refuses to wrap.** Stacking becomes a decision taken in one place at one width, instead of
  whatever the window happens to be.
- **The basis is one word.** `IN THE WINDOW` was 117px of a 238px cell — half the room the numbers
  needed, spent on two words that say nothing `WINDOW` does not. The full phrase is still what
  each box is NAMED (`x of the press at step 2, inside the window`), which is what a screen reader
  says and what a badge could not carry anyway.
- **The boxes are 4em**, which holds a four-digit screen coordinate.
- **A label travels with its box.** `x` and the field it names are one unit at every width, so
  whatever the row does, it never does *that*.

And the narrow block that stacks the editor moves from **760 to 960**. 760 was never the width
this row stops fitting at — at 760 the cell is 35 pixels and long past arguing about. Below 960
the row stacks on purpose: number and verb on the first line, the basis and both pairs on the
next, the controls below them.

## What this does not do

- It does not change what a press is, what it is checked against, or what a move to it keeps. The
  five guard terms still travel through an edit and the test that reads the saved file still says
  so.
- It does not change which pair a row offers. `in_window` is still the backend's answer.
- It does not change any refusal. A press found by its picture still cannot be moved and still
  says why on its row.
- It adds one sentence to the backend's vocabulary and removes one field from two structs.

## Enforced by

- `plugins/studio` `TestEveryKindThatPressesSomewhereHasAVerb` — the kinds come out of
  `internal/macroir/step.go`, `orchestrator.PointIn` says which of them press anywhere, and each
  of those must open with a verb the page's own table knows. It is the half
  TestEveryStepSentenceTheBackendWritesHasAVerb deliberately does not cover: that one lets a kind
  with no sentence render whole, which a point row cannot do.
- `plugins/studio` `TestAPressRowIsOneLineAtAnOrdinaryWidthAndStacksWhole` — **rendered in
  headless Edge at two stated widths**. At 1100 the row draws four columns, its parts share one
  line, nothing leans out of the cell, the boxes are still at least 44px, and what the press does
  is to the left of where it presses. At 460 it has stacked, and both labels are still on their
  own box's line.
- `plugins/studio` `TestTheEditorOffersThePairAPressWasMeasuredIn` — the action column reads
  `Left-click`, neither the button nor the window is written again beside the numbers, and a point
  row whose sentence the verb table cannot open still shows that sentence rather than losing it.
- `plugins/studio` `TestAMeasuredPressCanBeMovedAndKeepsWhatItIsCheckedAgainst` — the row's
  sentence opens with `left-click at `, Help still carries the promise about what a move keeps,
  and the saved file still carries every guard term.
- `plugins/studio` `TestTheStepRowsAreCompactAndTheWarningStaysReadable` — at 1100 the four
  columns are still four columns, which is what the width above is measured against.

Seven mutations, seven killed: the move sentence removed from `oneStep`, the page's
`move the pointer to ` prefix removed, the action column's fallback removed, the badge put back
to its full phrase, the boxes put back to the width they had, the label-and-box unit undone, and
the narrow block put back at 760.

One of them failed to kill on the first run and the test was wrong, not the rule: `fits` compared
the row's own `scrollWidth` against its own `clientWidth`. An inline-flex box sizes to its own
content, so those are equal however far it leans over the column beside it — the assertion could
not fail. It compares against the grid cell now, which is the thing it has to fit inside.

## See also

- [[ADR-289-the-panel-says-less-and-play-is-bigger]] — which took the measurements off this row
  and left the button on it
- [[ADR-290-a-routine-opens-where-it-is]] — the accordion these states are renders of
- [[ADR-286-a-press-can-be-moved]] — the edit the row offers
- [[Studio-alpha-UI-slice-2026-10-31]] — the slice record
