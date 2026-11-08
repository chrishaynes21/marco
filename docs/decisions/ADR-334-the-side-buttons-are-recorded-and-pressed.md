---
type: adr
status: accepted
date: 2026-11-08
decision: "ACCEPTED — THE MOUSE'S SIDE BUTTONS ARE RECORDED AND PRESSED, AS `back` AND `forward`. The recorder's mouse hook read only left, right and middle, so a press of a side button (WM_XBUTTONDOWN/UP) was never seen. A demonstrated ctrl+shift+forward recorded nothing at all, because the modifiers had no click to hold across and were dropped as having modified nothing. The hook now names XBUTTON1 `back` and XBUTTON2 `forward` from the high word of mouseData, and does not record a value Windows does not document. The host presses them as XDOWN/XUP with that button in mouseData, on a click at a point, a click at the cursor, and a drag's press and release alike. One list of buttons (`macroir.MouseButtons`) is what the host and Studio's added-click check read; Director, which may not import it, keeps a copy that a test holds equal. Studio's step list says `back-click at …` / `forward-click at …`, and its added-click chooser offers them. A drag naming a button nothing can press is now refused, as a click already was, instead of being dragged as left."
supersedes: []
affects:
  - execution
  - interface
source_paths:
  - internal/macroir/buttons.go
  - internal/recorder/recorder_windows.go
  - internal/recorder/syscall_windows.go
  - internal/oshost/backend_windows.go
  - internal/oshost/oshost.go
  - internal/orchestrator/pointedit.go
  - plugins/studio/frontend/index.html
---

# The side buttons are recorded and pressed

Builds on [[ADR-260-a-press-is-a-button-and-a-place]].

## Why

The owner: "it doesn't have targets for all click options, things gamers use a lot. Recording
should record the full range of inputs that can be done. For example I just tried recording shift +
ctrl + forward on the mouse and it didn't pick up a single one".

## Decision

- **Named `back` and `forward`**, which is what the buttons say on most mice and what Windows
  documents XBUTTON1 and XBUTTON2 as. A route says `Button "forward"` beside the place, the way
  ADR-260 put `Button "right"`. A Point with no button is still a left press, so no saved file
  changes.
- **Recorded through the same press path** (`recordClickDown`/`recordClickUp`) as the other three:
  the same window attribution, the same gate, and the same template capture.
- **Pressed with the button in mouseData.** `mouseButtonFlags` returns the data with the flags, so
  no caller can send XDOWN without saying which side button.
- **One list.** `macroir.MouseButtons` replaces the three hand-written `left/right/middle` checks
  in the host, Director's operation validation and Studio's step editor.
- **A drag's button is checked.** `doDrag` used to pass an unknown name to a table that answered
  left.

## Not established

- Not tried on a desktop: the recorder was driven with the structures Windows passes and the host
  with a captured SendInput, as for every other button.
- Other inputs gamers use are separate changes: the wheel, F13–F24, and numpad keys kept distinct
  from the top row.

## Enforced by

- `internal/recorder`: `TestTheSideButtonsAreRecordedByName`.
- `internal/simplify`: `TestCtrlShiftForwardIsHeldAroundAForwardClick`.
- `internal/oshost`: `TestTheSideButtonsArePressedAsSideButtons`, `TestADemonstratedNonLeftPressArrivesAsThatButtonWhereTheWindowIsNow` (back, forward), `TestADragWithAButtonNothingCanPressSendsNothing`.
- `internal/orchestrator`: `TestAnAddedClickCanUseEveryButtonMarcoPresses`.
- `internal/macroir`: `TestDirectorKnowsTheButtonsTheHostPresses` (Director keeps its own copy of the list, because it may not import macroir).
- `plugins/studio`: `TestARightOrMiddlePressIsSavedAndTheReviewSaysWhichButton` (back, forward), `TestEveryStepSentenceTheBackendWritesHasAVerb`.
