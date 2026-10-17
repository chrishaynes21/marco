---
type: adr
status: accepted
date: 2026-10-17
decision: an intermediate step is judged by the transition it was for and a procedure by the OUTCOME the person asked for — a menu closing is evidence a choice landed, a step whose state already holds is counted and the procedure continues, and completion is claimed only when a goal's declared outcome is observed on a control Marco did not itself write to
supersedes: []
affects:
  - execution
  - perception
source_paths:
  - internal/director/verify/verify.go
  - internal/director/verify/semantic.go
  - internal/director/verify/outcome.go
  - internal/director/execute/program.go
  - internal/director/goal/goal.go
  - internal/director/goal/library.go
  - internal/director/uiact/ladder.go
---

# ADR-224 — a procedure is finished when the person has what they asked for

## What was measured

A live Explorer window on a disposable folder. "Make a new folder called Reports":

```
[1/4] invoke the New menu    — verified   (12 menu items appeared)
[2/4] choose the Folder entry — unverified
    ! verify inconclusive: "Folder" is no longer in the world, so its selection
      could not be read
UNVERIFIED: step 2 of 4 could not be verified
```

**The folder was created.** Both steps did the right thing, the menu closed as menus do, and
the verifier went looking for the chosen item's selection state. Marco was verifying the
MECHANISM it used rather than the transition the press was for, and that stopped a procedure
two steps from finishing.

Three further defects came out of fixing it, each found by running the thing again:

- A procedure whose first step had NOTHING TO DO was abandoned at step one.
- `confirm` lowered to a CLICK on a text field, which does not commit a rename.
- And the first version of the outcome check reported **`COMPLETED: text_field "Reports" is
  there`** while the folder on disk was still "New folder" — satisfied by the edit box holding
  Marco's own keystrokes.

## The decision, in four parts

### One: a menu closing is evidence, and is not success on its own

`menuClosedTaking` reports that the menu a control belonged to was dismissed, taking the
control with it. Three things must hold together, because each alone is ambiguous: the control
was a menu member, several visible menu items went with it, and none stayed — a submenu
replacing its parent leaves items visible and is not a menu closing.

That is evidence the press landed ON THE MENU and nothing more. A menu dismissed by Escape
satisfies all three. So a second, separate piece of evidence asks whether anything appeared
OUTSIDE the menu — Explorer's new folder arrives as a fresh list item with an editor in it —
and the existing weighting decides. Delivery alone is 0.4 and never sufficient; delivery plus
advancement concludes.

When the menu closed and nothing else changed, the verdict is not success and the sentence says
what was seen: *"the menu closed, which is what choosing an item does, but nothing else changed
to show the application acted on it"*. "Nothing could be read" sends somebody looking for a
broken control.

### Two: a step that needed nothing is counted, and the procedure continues

`ResultNothingToDo` is now a case in the program loop: the step is counted, the context
advances, the account says "nothing needed doing", and no record is written — because nothing
was performed, and inventing an action to describe is how an account stops being worth reading.

This replaces the limitation [[ADR-221-a-question-the-person-already-answered]] recorded. A
precondition already met means the procedure should carry on, and the live run showed what the
alternative cost: "rename this file to summary.txt" with the file already selected selected it
again, nothing changed, and a four-step rename was abandoned at step one.

The flag that remains decides one narrower thing: after an action has been PERFORMED — a retry,
a replay — the question is whether it landed, which is verification's job.

### Three: completion is the person's outcome, not Marco's step count

A `goal.Procedure` may declare `Outcome`: what the request was FOR, in the condition vocabulary
`directorapi.Plan.SuccessConditions` has always described as *"must ALL hold after the plan for
it to count as done"* — a sentence that was true of nothing, because nothing set those
conditions and nothing read them. `verify.CheckOutcome` reads them.

Both create-folder procedures declare the same one, because the outcome belongs to the GOAL and
the procedures are two ways of reaching it.

Three rules make it safe:

- **A condition that cannot be evaluated is NOT satisfied.** It is reported as unchecked, and
  an unchecked outcome is not a completed request. Treating "I do not know how to look at that"
  as a pass would make every future condition type a silent success.
- **The match is exact on the normalised text.** "Reports" must not be satisfied by "Reports
  (1)" or "Quarterly Reports Draft", which are different folders.
- **A control Marco wrote to cannot be the evidence.** This is the one that was measured as a
  false success, and it is the same trap as the menu one tier up: the edit box said "Reports"
  because Marco typed it there. The program hands `CheckOutcome` every control its steps acted
  on, and none of them may be the proof.

A procedure that declares no outcome is judged exactly as before. Adding the field must not
make every existing procedure assert something nobody wrote down.

### Four: clicking a text field confirms nothing

The `confirm` ladder is Invoke → click → Enter → refuse, which is right for a dialog's OK
button. On an in-place rename there is no OK button: the confirming control resolves to the
editor, Invoke is unavailable, and the ladder clicked the text field. Clicking a text field
puts a caret in it. The rename was abandoned and Enter sat one rung below the click that did
nothing.

So the click rung declines the one control for which clicking is not a confirmation. Narrow on
purpose: a fact about text fields, not about Explorer, and nothing else in the ladder moves.

## What this did on a real desktop

```
make a new folder called Reports
  [1/4] invoke the New menu     — verified
  [2/4] choose the Folder entry — verified   (the menu closed, taking all 5 of its items)
  [3/4] set the name to "Reports" — verified
  [4/4] confirm via keyboard · enter · after 2 stronger option(s) were unavailable
        verify  the application now says "New folder" → "Reports"
  COMPLETED: list_item "Reports" is there        (4 action(s) performed)
```

And on disk: `Reports`. See
[[Experiment-073-marco-finishes-what-it-starts]].

## Enforced by

- `internal/director/verify` `TestChoosingAMenuItemIsVerifiedByTheMenuClosing`,
  `TestAMenuThatMerelyClosedIsNotASuccess`,
  `TestAVanishedControlThatWasNotInAMenuIsStillInconclusive`,
  `TestASubmenuReplacingItsParentIsNotAMenuClosing`.
- `internal/director/execute` `TestAProcedureContinuesPastAStepThatNeededNothing`,
  `TestRenameModeActionTargetsTheBoundFile`.
- The false success is held by `verify.CheckOutcome`'s exclusion and by
  `TestARequestForAPageThatIsOpenPerformsNothingAndSaysSo`'s sibling checks on the record,
  plan and policy being absent.
