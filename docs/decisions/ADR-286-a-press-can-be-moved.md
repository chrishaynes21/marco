---
type: adr
status: accepted
date: 2026-10-31
decision: "ACCEPTED — A PRESS CAN BE MOVED, AND THE TWO NUMBERS BESIDE IT STAY TRUE. A click recorded inside a window is written down TWICE: as a point on the screen and as an offset inside that window, with ADR-250's five terms beside them, whose presence IS the opt-in for the guarded contract. Editing one of those numbers is possible only because the two were measured AT THE SAME INSTANT, so the window's origin is recoverable as `(X-RelX, Y-RelY)` and the screen point can move with the offset out of the SAME reading — nothing invented, no new measurement taken. Without that, editing either number makes the other a lie: a routine whose offset says one place and whose screen point says another presses one of them and checks the other. WHICH PAIR IS OFFERED IS THE STEP'S OWN ANSWER AND NEVER THE CALLER'S — a press measured inside a window is moved by its OFFSET, a press that was never measured against a window is moved by its screen point and does NOT acquire a window by being moved. `orchestrator.PointIn` answers for every step, so no surface works out for itself which rows have a position: a surface that decided would be free to offer a box the validator then refuses, which is a control whose refusal arrives after somebody has typed into it. Four shapes refuse, each for its own reason: a press found by its PICTURE, its COLOUR, the WORDS beside it or what Marco can SEE, because the coordinate there is a position INSIDE the thing being looked for (`AnchorClickX/Y` is exactly that) and moving it aims the picture somewhere else; a DRAG, which has two ends and one editable pair would edit half of a gesture; an INCOMPLETE MEASUREMENT, refused rather than completed, because filling a term in would be manufacturing a guarded point out of a reading nobody took and the routine would then be checked against a window this code invented; and an OFFSET THE WINDOW COULD NOT CONTAIN, which is not a place in it. EVERYTHING ELSE ON THE STEP SURVIVES — the button, the modifier, the count, the duration and every guard term — because the operation COPIES the step and sets at most four numbers; a press checked against its window before an edit is checked against it after. `EditPoint` leaves the list the length it was, so unlike `EditTyped` it collects with waits and removals in the ordinary way, and refuses only the one thing that is two answers to one question: the same step asked to be moved and removed. The displayed-list basis, the review token and the stale-source protections are the existing ones, unchanged."
supersedes: []
affects:
  - execution
source_paths:
  - internal/orchestrator/pointedit.go
  - internal/orchestrator/stepedit.go
  - plugins/studio/stepedit.go
  - plugins/studio/frontend/index.html
---

# A press can be moved

Slice 4 of [[Studio-alpha-UI-slice-2026-10-31]], and the last of it.

## What a press knows about where it is

From the owner's own routine, read off disk:

```
the p1 is a Point with X 342, Y 294, RelX 342, RelY 294, RelTo "window",
                       WinW 1920, WinH 1080, Dpi 96, Process "arkascended.exe".
```

`X, Y` is a point on the screen. `RelX, RelY` is an offset inside the window. The five terms
after them are what a run checks before it presses, and **their presence is the opt-in** for
that check — see [[ADR-251-a-press-checks-the-window-it-was-recorded-in]].

## Why editing one number is possible at all

Because the two were measured **at the same instant**. The window's origin is therefore
recoverable — `(X-RelX, Y-RelY)` — and moving the offset can move the screen point with it, out
of that same reading. Nothing is invented and no new measurement is taken.

Without that, editing either number makes the other one a lie: a routine whose offset says one
place and whose screen point says another presses one of them and checks the other.

So the **offset** is the pair offered for a measured press. It is the durable half — it still
means the same place after the window has moved — and the screen point follows it. A press that
was never measured against a window has only the screen point, and that is what is offered
instead. **It does not acquire a window by being moved.**

Which of the two is offered is never the page's choice: `orchestrator.PointIn` answers for every
step and the page draws what it is handed.

## What refuses, and why each one

| | |
| --- | --- |
| found by its **picture**, **colour**, the **words** beside it, or what Marco can **see** | the coordinate is a position INSIDE the thing being looked for — `AnchorClickX/Y` is exactly that — so moving it aims the picture somewhere else |
| a **drag** | two ends and one rectangle; one editable pair would edit half a gesture. Out of this slice by decision |
| an **incomplete measurement** | a step that says it was measured in a window and does not say which. Completing it here would manufacture a guarded point out of a reading nobody took, and the routine would be checked against a window this code invented |
| an **offset the window could not contain** | checked against the size the press was MEASURED against, which is the size the guard compares at run time |
| the position it **already has** | nothing to review, and the same class of refusal `PrepareRescope` makes about an unchanged scope |

Each refusal is shown **on the row**, beside the press it is about, outside the struck-out part so
a removal does not strike through the reason as well.

## What survives

`movedPoint` **copies the step** and sets at most four numbers. The button, the modifier, the
count, the duration and every one of the five guard terms travel on the copy — which is why it is
written as a copy rather than as a fresh `macroir.Step`, and why there is no second place that
decides which fields a move may touch.

A press that was checked against its window before an edit is checked against it after.

## How it collects

`EditPoint` leaves the list the length it was, so unlike `EditTyped` it collects with waits,
removals and argument declarations in the ordinary way — every other position in the same review
still means what it meant.

It refuses exactly one pairing: the same step asked to be **moved and removed**, which is two
answers to one question. And a step asked to be moved to two places, which is the same thing
twice.

Carried forward unchanged: the displayed-list basis (`StepEditing.Basis`, sent back with the
changes), the review token, and the stale-source refusal — a move is counted against the list it
was read off, like everything else in this panel.

## What this does not do

- It does not move a drag.
- It does not offer the screen point of a press that was measured in a window. One editable pair,
  and the derived one moves with it.
- It does not add, remove or alter any guard term.
- It no longer prints the window's measurements beside every press.
  [[ADR-289-the-panel-says-less-and-play-is-bigger]] took them off the rows: the terms were put
  there so that nobody had to find out afterwards whether moving a press also changed what it
  was checked against, and that is the same promise on every row, so it is made once in Help's
  `measured` section instead of three measurements deep down a list. The row kept the BUTTON,
  and [[ADR-291-the-press-row-says-what-it-does]] took that off too — not because it was the
  wrong thing to say but because it was already being said: a step is written as one sentence
  and the row splits it into an action column and a value column, so `Left-click` was at the
  front of the row while `left` trailed the numbers at the end of it. **Nothing about what is
  preserved changed**, and the test below still reads it off the saved file.
- It does not touch the recording review's list. A draft is not saved yet and its coordinates are
  still what was demonstrated.

## Enforced by

- `internal/orchestrator` `TestMovingAPressInsideItsWindowMovesTheScreenPointWithIt` — the
  fixture's window origin is **1100, 1000**, not zero, so an arithmetic mistake cannot pass by
  making the two pairs the same number.
- `internal/orchestrator` `TestMovingAPressKeepsTheButtonTheModifiersAndEveryGuardTerm` —
  eleven fields compared one at a time.
- `internal/orchestrator` `TestMovingAnUnmeasuredPressMovesTheScreenPointAndInventsNoWindow` —
  and asserts that none of `WinRel`, `RelTo`, `WinW`, `WinH`, `Dpi`, `Process`, `RelX`, `RelY`
  was given a value.
- `internal/orchestrator` `TestAPressFoundByItsPictureCannotBeMoved` — all four ways of being
  found by looking, each with its own reason.
- `internal/orchestrator` `TestADragIsNotMovedByThisAndSaysSo`,
  `TestAnIncompleteMeasurementIsRefusedAndNotCompleted` (five terms, one at a time),
  `TestAnOffsetOutsideTheWindowIsRefused` (and the edges are inside it),
  `TestMovingWhatCannotBeMovedSaysWhichOfTheThreeItIs`.
- `internal/orchestrator` `TestAMoveCollectsWithOtherChangesAndRefusesARemovalOfTheSameStep`.
- `plugins/studio` `TestAMeasuredPressCanBeMovedAndKeepsWhatItIsCheckedAgainst` — against a real
  library: the row offers the offset, the review shows BOTH numbers moved before anything is
  written, the saved file still carries all five terms, and the routine can be edited again. It
  asserted the window terms beside the numbers as well; since
  [[ADR-289-the-panel-says-less-and-play-is-bigger]] it asserted the row shows the button and
  NOT the measurements, and since [[ADR-291-the-press-row-says-what-it-does]] it asserts the
  narrower thing the button being in the action column leaves behind — that the sentence OPENS
  with what the press does. Help carrying the promise is still asserted here.
- `plugins/studio` `TestAPressThatCannotBeMovedSaysSoOnItsRow` and
  `TestAMoveCountedAgainstAListThatHasMovedIsRefused`.
- `plugins/studio` `TestTheEditorOffersThePairAPressWasMeasuredIn` — **rendered in headless
  Edge**: each press shows which pair it is offering, the measured one offers its OFFSET, the
  action column says what the press does and nothing repeats it beside the numbers, a press
  found by its picture has no boxes and says why,
  a step that presses nowhere has neither, every box is labelled and names its step and its
  basis, the asked line says where the press would be AND in which basis, and the pair reaches
  the backend against the row it was read off.

## See also

- [[Studio-alpha-UI-slice-2026-10-31]]
- [[ADR-251-a-press-checks-the-window-it-was-recorded-in]]
- [[ADR-273-a-routine-writes-down-what-it-does]] — the step account this edits through
- [[ADR-284-the-step-editor-is-a-list-of-rows]] — the row this draws into
