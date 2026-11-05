---
type: adr
status: accepted
date: 2026-11-04
decision: "ACCEPTED — A STEP CAN BE ADDED, THROUGH THE SHARED PREVIEW/SAVE PATH. `orchestrator.EditInsert` adds a wait, a key press or typed text in the GAP after an original position (-1 before the first). It is collected with every other operation against the list that was shown, and the new list is built once. So an addition shares a review with a removal, a changed wait or a changed key, and nothing is renumbered under the person; what goes after a removed step goes where that step was. Nothing may be added before the step that brings the window forward. What is added obeys the rules a change of its kind obeys. No click yet: a press at a point needs the window measurements its check is made of. Studio puts the control centre's `+` on every step row; it opens a dashed row under that row, read when Review is pressed like every other box. A change refused for something asked keeps what was typed (ADR-311)."
supersedes: []
affects:
  - interface
source_paths:
  - internal/orchestrator/stepedit.go
  - plugins/studio/stepedit.go
  - plugins/studio/frontend/index.html
---

# A step can be added

## Why

Inserting steps is the control centre's second editor capability Studio lacked
([[ADR-310-studio-wears-the-control-centres-look]]). The control centre inserts source lines and
saves them directly. Here, an addition is an operation on Studio's step-edit contract, so it is
reviewed before anything is written, saved with the stored step account, and editable afterwards
like any other step.

## The decision

- **An addition names a gap, not a step.** `At` is the original position it goes after; -1
  means before the first step.
  - Every operation in one review refers to the list the person was looking at. The new list is
    built once, by walking the original steps and placing each addition in its gap.
  - That is why an addition can share a review with other changes, where declaring a run typed
    text (`EditTyped`) cannot: that one merges a range and moves every later position.
  - Several additions in one gap keep the order they were asked in. What is added after a step
    being removed goes where that step was.
- **Nothing goes before the window comes forward.** A routine that brings its window forward
  first presses nothing until it has. A step put in front of that would press into whatever
  window was in front.
- **The same rules as a change of that kind:**
  - a wait is a non-negative whole number of milliseconds;
  - a key is one Marco can press (`macroir.IsChord`);
  - text is fixed text with no `{{`/`}}`.

  A step cannot be refused as an edit and then accepted as an addition.
- **No click yet.** A guarded press is checked against the window it was recorded in, using the
  offset, size and scale measured at that moment. A hand-made press has none of those, so what
  it would be (unguarded, or measured against a window chosen now) is a decision of its own.
- **Studio** puts the control centre's `+` on every step row.
  - It opens a dashed row under that row with what kind of step, its one value, and Remove.
  - Like every other box, it is read when Review is pressed.
  - An empty value, or a wait that is not a number, is a correction on that row and is never
    sent. The panel names it as "the step added after step N".
  - A refusal of something asked keeps the added rows and what was typed in them.

## Verified (Windows only)

- **Orchestrator:**
  - each kind lands in its gap;
  - five changes in one review, one removal, one changed wait and three additions, land on the
    positions the person saw;
  - the window-forward rule, including a routine that brings nothing forward;
  - the shared rules.

  End to end, two additions after the window comes forward are what the saved routine asks
  for, in order. The added key is then changed by a later edit.
- **Studio:** a key and text are added through the bindings and are in the file; the panel
  lists two more steps afterwards. A step before the window comes forward is refused as a
  correction, with nothing written.
- **Rendered:**
  - every row offers `+`;
  - the new row appears under its row and gets the cursor;
  - an empty added text is a correction naming its row, and nothing is sent;
  - three additions are sent in order with their gaps and values;
  - a correction keeps them;
  - a removed addition is gone.
- **Mutants:** 11, all killed. They covered: Studio dropping the kind; the page never sending an
  added key, never drawing added rows, sending an empty text, adding after the wrong row, or
  discarding additions on a correction; what goes after a removed step, or at the start, being
  lost; a step allowed before the window comes forward; any key accepted; braces accepted.
- **Changed guard:** `TestTheStepPanelOffersTwoOperationsAndNoOthers` claimed the panel offered
  no adding and no editing of a position or a key. It now checks only what is still true: no
  reordering.

## Not established

- **Not seen on the desktop.** Rendered in headless Edge with stub bindings, and run against a
  recording host.
- Clicks, repeats and drags are later slices.

## Enforced by

- `internal/orchestrator` `TestAStepIsAddedInTheGapAfterAPosition`,
  `TestAnAdditionSharesAReviewWithOtherChanges`, `TestNothingIsAddedBeforeTheWindowComesForward`,
  `TestAnAdditionIsRefusedWhenAChangeOfItsKindWouldBe`, `TestAnAddedStepIsWhatTheRoutineDoes`.
- `plugins/studio` `TestAStepIsAddedThroughTheStepPanel`,
  `TestAStepBeforeTheWindowComesForwardIsRefusedByTheStepPanel`,
  `TestAStepIsAddedOnThePageAndReviewSendsIt`.
