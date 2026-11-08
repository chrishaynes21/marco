---
type: adr
status: accepted
date: 2026-11-08
decision: "ACCEPTED — THE WHEEL IS RECORDED, SAVED AND TURNED, CHECKED LIKE A PRESS. The recorder read no wheel at all. Its mouse hook now reads WM_MOUSEWHEEL and WM_MOUSEHWHEEL as EvScroll (appended after the markers), with the signed delta, whether it was horizontal, and the point and window measurements a press takes, since what scrolls is whatever is under the pointer. `simplify` turns a run of notches into one StepScroll: the same direction, at the same place, each within 400ms of the last, with nothing else in between. It carries a Wheel (`up`, `down`, `left`, `right`), a Count of notches (WHEEL_DELTA rounded, at least one) and the window measurement. A modifier held across a turn is a hold, as across a click (ctrl+wheel). The generator writes it as one guarded sentence, `do OS's Scroll with pN.`, on a Point carrying `Scroll \"down\", Notches 3` beside the place, and it is a link in a guarded chain, so a scroll refused or not delivered stops the routine. The OS act exports Scroll; the host resolves and checks the point exactly as a click's (clickTarget), reads the stop before the check and before the turn, refuses a direction it does not know, and keeps a turn that was not delivered in the account. Windows sends one WHEEL_DELTA a notch (HWHEEL for left and right), the first in the same SendInput call as the move to the point, then 15ms apart. Studio says `scroll down 3 notches at …`, counts a scroll as a step that sends input, and names a scroll's missing measurement at review."
supersedes: []
affects:
  - execution
  - interface
source_paths:
  - internal/recorder/recorder.go
  - internal/recorder/recorder_windows.go
  - internal/simplify/simplify.go
  - internal/macroir/step.go
  - internal/codegen/codegen.go
  - internal/osmod/os.marco
  - internal/oshost/oshost.go
  - internal/oshost/backend_windows.go
  - internal/orchestrator/draft.go
  - internal/orchestrator/pointedit.go
  - plugins/studio/recording.go
  - plugins/studio/frontend/index.html
  - cmd/marco/edit.go
---

# The wheel is recorded and turned

Follows [[ADR-334-the-side-buttons-are-recorded-and-pressed]] and
[[ADR-335-the-keys-games-bind-are-recorded-apart]], from the same request: "Recording should record
the full range of inputs that can be done".

## Decision

- **A turn is checked like a press.** Where the pointer is decides what scrolls, so a turn that
  ignored the window it was recorded in would scroll whatever is there now. It uses the same Point,
  the same measurement and the same `clickTarget`, so the guarded contract, refusals and the account
  all apply unchanged.
- **One step per turn, not per notch.** Three notches a few tens of milliseconds apart are one
  movement of the hand. A pause, a change of direction or anything else in between starts a new
  step and keeps the wait.
- **Notches are sent one at a time.** A game that reads one notch per frame would take three sent at
  once as one.
- **A host-module change, not a Core one.** `Scroll` is a capability of the OS act and two fields of
  its Point, as `Button` was (ADR-260). Every route saved before this reads and runs as it did.
- **Its position can be edited** in Studio like a click's (`PointIn`), and it can be the basis for
  an added click (`ClickBases`).

## Not established

- Not tried on a desktop or in a game: the hook was driven with the structures Windows passes, the
  backend with a captured SendInput, and the run end to end through the fake desktop.
- Whether 15ms between notches suits every game. It is a guess at a person's speed, not a
  measurement.
- Fine-grained (high-resolution) wheels are rounded to whole notches.

## Enforced by

- `internal/recorder`: `TestTheWheelIsRecordedWhereItTurned`.
- `internal/simplify`: `TestARunOfNotchesIsOneScroll`, `TestCtrlHeldAcrossTheWheelIsAHold`.
- `internal/oshost`: `TestADemonstratedScrollTurnsWhereTheWindowIsNow`, `TestAScrollIntoAWindowOfAnotherSizeTurnsNothing`, `TestAScrollTheMachineWouldNotTakeStopsTheRoutine`, `TestAScrollThatDoesNotSayWhichWayTurnsNothing`, `TestTheWheelTurnsTheWayItWasRecorded`.
- `internal/orchestrator`: `TestAScrollIsAPressableStep`.
- `internal/spectest`: `TestTheMatrixCoversEveryStepKind` (the `scroll` and `scroll-in-a-window` shapes).
- `plugins/studio`: `TestEveryStepSentenceTheBackendWritesHasAVerb`, `TestAnUnmeasuredScrollIsSaidAtReview`.
- `cmd/marco`: `TestTheControlCentreNamesAScroll`.
