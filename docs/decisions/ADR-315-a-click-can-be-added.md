---
type: adr
status: accepted
date: 2026-11-04
decision: "ACCEPTED — A CLICK CAN BE ADDED, IN AN EXPLICITLY CHOSEN BASIS, THROUGH THE SHARED PREVIEW/SAVE PATH. The owner's policy. WINDOW-RELATIVE: the person chooses one of the windows the routine's own presses were measured in full (`orchestrator.ClickBases`). The click reuses that measurement's program, size, scale and recorded origin and nothing else (no locator), and its offset is validated against that size. That is an existing window measurement supporting a newly authored action, not a claim that the click was recorded. With no measured basis, window-relative insertion is unavailable and the panel says so. SCREEN: allowed only in a routine that checks nothing (`ScreenClicksAllowed`, asked of `codegen.GuardsAnything`), with the basis stated, and never a fallback in a checked routine. The demonstration is unchanged, the authored click is in the stored account, and the review names the action and its basis."
supersedes: []
affects:
  - interface
  - execution
source_paths:
  - internal/orchestrator/pointedit.go
  - internal/orchestrator/stepedit.go
  - internal/codegen/codegen.go
  - plugins/studio/stepedit.go
  - plugins/studio/frontend/index.html
---

# A click can be added

## The policy, as approved

- **Window-relative click.** The person explicitly chooses an existing measured-window basis
  from the routine. Its program, size, scale and recorded origin are reused, and the person
  enters the new relative X/Y. These are existing window measurements supporting a newly
  authored action, not a claim that the click was recorded.
- **Bounds.** The coordinates are validated against that window's size. No anchor is copied and
  no measurement is invented.
- **No basis.** If no measured basis exists, the panel explains that window-relative insertion
  is unavailable.
- **Screen-coordinate click.** Allowed explicitly in an unguarded routine, with the coordinate
  basis clear. It is never used silently as a fallback inside a guarded routine.
- **The record.** The original demonstration is unchanged, the authored step is persisted in the
  account, and the review shows the new action and its coordinate basis.

## How it is built

- **`ClickBases`** lists the distinct windows the routine's presses were measured in full:
  - clicks, drags and pointer moves with every term of their window present;
  - not a press found by a picture or by words, because its offset is a place inside what it
    looks for;
  - two presses in the same window at the same origin count as one basis.

  Each basis is labelled by the step it was measured at: *the window step 7 was measured in
  (notepad.exe, 1280×720, 96 dpi)*.
- **`EditInsert` kind `click`** takes `Button` (left by default; right and middle allowed),
  `X`/`Y`, and either `BasisAt` (one of the bases) or `Screen`.
  - **A window click** copies `RelTo`, `Process`, `WinW`, `WinH` and `Dpi` from the basis. Its
    screen point is the basis's origin plus the offset, and the offset must lie inside the
    basis's size.
  - **A screen click** is refused with that reason in a routine that checks anything. The check
    is the generator's own (`codegen.GuardsAnything`, exported for it), so the editor and the
    generator cannot disagree about which routines are checked.
- **Studio's step list carries:**
  - `ClickBases`, each with its label and size;
  - `ScreenClicks`;
  - `ClickSays`, the sentence when window clicks are not possible: only screen points in an
    unchecked routine with no measured press, or no click at all in a checked routine with
    none.
- **The page:** the added-step row offers `click` only where one can be added. The basis is a
  named choice and never a default into a different kind of place, with the button chooser and
  two offset boxes whose hint is the window's range. The summary names the basis: *a left-click
  at 640, 360 in the window of step 7 is added after step 7*.

## Verified (Windows only)

- **Orchestrator:**
  - a right-click added inside a basis carries exactly its measurement, its own offset, a
    screen point from the basis's origin, and no locator;
  - refused when outside the window, when the basis is not a measured press, when there is no
    measured window, when the only press was found by its words, or for an unknown button;
  - a screen click accepted in an unchecked routine and refused in a checked one;
  - bases deduplicated by window and origin.

  Through save and the production driver, a click added inside the window of one of the
  routine's own presses is performed checked against `arkascended`:
  - the demonstration is byte-for-byte unchanged;
  - the next edit sees the click;
  - a rebuild under today's rules keeps it;
  - a screen click in an unchecked routine is performed at its point.
- **Studio:**
  - through the bindings, a measured routine offers its window and no screen point, saves a
    window click whose review names its basis, and refuses a screen click as a correction;
  - an unchecked routine offers a screen point and says it has no window;
  - a checked routine with no fully measured press offers neither, and says so.
- **Rendered:** what each kind of routine is offered, the summary, and exactly what Review sends.
- **Mutants:** 15, all killed. Eight in the orchestrator:
  - a screen click in a checked routine;
  - outside the window;
  - a worded press as a basis;
  - a window offered twice;
  - the screen point not from the origin;
  - the scale not carried;
  - any step as a basis;
  - any button.

  Seven in Studio:
  - the basis dropped;
  - screen clicks offered to a checked routine;
  - no reason given;
  - the page offering a screen point everywhere;
  - `click` offered where none can be added;
  - a screen click sent as a window one;
  - the summary dropping the basis.
- **Changed guard:** `TestAnAdditionIsRefusedWhenAChangeOfItsKindWouldBe` held that a click
  cannot be added. It now holds that an unknown kind cannot be added.

## Not established

- **Not seen on the desktop.** No added click has been pressed on a real window, only on a
  recording host.

## Enforced by

- `internal/orchestrator` `TestAClickIsAddedInsideAWindowTheRoutineWasMeasuredIn`,
  `TestAWindowClickIsRefusedWhereItsBasisDoesNotHoldIt`,
  `TestAScreenClickIsAddedOnlyToARoutineThatChecksNothing`,
  `TestTheBasesAreTheDistinctWindowsTheRoutineWasMeasuredIn`,
  `TestAClickAddedInsideAWindowIsWhatTheRoutineDoes`,
  `TestAScreenClickAddedToAnUncheckedRoutineIsWhatItDoes`.
- `plugins/studio` `TestAClickIsAddedInsideTheRoutinesWindowThroughTheStepPanel`,
  `TestAScreenClickIsAddedToAnUncheckedRoutineThroughTheStepPanel`,
  `TestACheckedRoutineWithNoMeasuredWindowSaysNoClickCanBeAdded`,
  `TestAClickAddedInsideAWindowIsSentWithItsBasis`,
  `TestAScreenClickIsOfferedOnlyWhereTheRoutineChecksNothing`,
  `TestNoClickIsOfferedWhereNoneCanBeAdded`.
