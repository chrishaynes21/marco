---
type: adr
status: accepted
date: 2026-11-04
decision: "ACCEPTED — A CLICK CAN BECOME A DRAG, AND A DRAG'S TWO ENDS CAN MOVE, THROUGH THE SHARED PREVIEW/SAVE PATH, AND A CHECKED PRESS STAYS CHECKED. `orchestrator.EditToDrag` makes a click a drag in the click's own basis, and `EditDrag` moves both ends of a drag (`DragIn`, a reading of its own, so moving a press never moves one end). Both copy the step: the button and every term of its window check are carried. Both ends must be inside the measured window, and the screen points move with the offsets out of the press's own reading. The recorded path between the old ends is dropped. Held keys around the press are separate steps and stay where they are. `WhyNotADrag` is the one rule for the offer and the refusal: a click found by its picture, missing a term of its window, naming a window it was never measured in, or pressed twice is not made a drag, never silently turned into one on the screen. The drag lives in the stored account, so the next edit and a rebuild under today's rules keep it. Studio puts the control centre's `Drag` on a click row and shows a drag's two ends as boxes in its basis."
supersedes: []
affects:
  - interface
source_paths:
  - internal/orchestrator/pointedit.go
  - internal/orchestrator/stepedit.go
  - plugins/studio/stepedit.go
  - plugins/studio/recording.go
  - plugins/studio/frontend/index.html
---

# Drags can be edited

## The decision

- **A click becomes a drag in its own basis** (`EditToDrag`, start `X`,`Y`, end `X2`,`Y2`).
  - A click measured inside its window becomes a drag inside that window. Its offsets are the
    drag's ends, both inside the measured size, and its program, size and scale are carried.
    The screen points follow from the origin the click was measured at, so no new measurement
    is taken and none is invented.
  - An unchecked click becomes a drag on the screen, and the review says so.
- **A drag's two ends move together** (`EditDrag`), in the basis the drag was measured in.
  `PointIn` still refuses a drag, so a press move can never shift one end.
- **What is carried:** the button, every term of the window check, and the held keys around the
  press. Those are separate keydown/keyup steps, which no drag edit touches.
- **What is dropped:** the recorded path between the old ends. It describes a gesture that no
  longer exists, and `codegen` does not render it.
- **A checked press never becomes an unchecked one.** `WhyNotADrag` is the single rule. The
  operation refuses with it, and Studio offers the conversion exactly where it says nothing.
  Refused:
  - a click found by its picture;
  - a click missing a term of its window;
  - a click that names a window it was never measured in (`15c1bfc`, below);
  - a double click;
  - a drag that does not move.

## A gap found and closed

`83a8f4a` let a click that names a window but has no measured offset become a drag on the
screen. To `PointIn` that click is a screen point with the window's terms beside it. Moving that
point is fine, because it is still the same incomplete press, but a drag is a new gesture. A
Studio test that looked for the offer on such a click found it. `15c1bfc` made `WhyNotADrag`
refuse it and made it the one rule.

## Verified (Windows only)

- **Orchestrator:**
  - a checked right-click became a checked right-button drag in the same window, with
    offsets, screen points and measurement as expected;
  - an unchecked click became a screen drag;
  - each refusal;
  - a drag's ends moving together with its path dropped;
  - moving a press never moving one end;
  - conflicts in either order.

  Through save and the production driver, a checked click made a drag runs as a drag checked
  against `arkascended`. The next edit offers its ends in the window and moves them, a rebuild
  under today's rules keeps the checked drag, and a shift held around the click is held around
  the drag.
- **Studio:** through the bindings, a fully measured click is made a checked drag; the next
  listing offers its two ends in the window and moves them; a click with no measured offset is
  not offered.
- **Rendered:**
  - a drag's four boxes start at its ends;
  - an immovable drag says why;
  - the `drag to` boxes start where the click is;
  - the summary names each drag's basis;
  - Review sends `todrag` and `drag` with the right numbers, and never a move as well;
  - keeping the click removes the end boxes.
- **Mutants:** 16, all killed. Nine in the orchestrator, plus the rule mutant for `15c1bfc`, and
  seven in Studio. One Studio mutant (a click made a drag also sent as a move) first survived,
  because the test never moved the click's start. The test now moves it.

## Not established

- Not seen on the desktop.

## Enforced by

- `internal/orchestrator` `TestACheckedClickBecomesACheckedDragInTheSameWindow`,
  `TestAnUncheckedClickBecomesADragOnTheScreen`, `TestAClickThatCannotStayCheckedIsNotMadeADrag`,
  `TestADragsEndsMoveTogetherAndOnlyTogether`,
  `TestADragAndAnotherChangeToOneStepAreRefusedInEitherOrder`,
  `TestAClickMadeADragIsACheckedDragThroughEverythingAfter`, `TestAKeyHeldAroundAClickIsHeldAroundItsDrag`.
- `plugins/studio` `TestACheckedClickIsMadeACheckedDragThroughTheStepPanel`,
  `TestAClickThatCannotStayCheckedIsNotOfferedAsADrag`,
  `TestTheStepPanelOffersADragsEndsAndMakesAClickADrag`.
