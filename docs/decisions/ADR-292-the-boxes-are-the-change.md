---
type: adr
status: accepted
date: 2026-11-02
decision: "ACCEPTED — IN THE STEP EDITOR, A BOX IS THE CHANGE. There is no per-row Set. A wait or a press position typed into its row is collected when Review is pressed, and a value somebody typed and never confirmed can no longer be silently left out — which is what the Set button made possible: `reviewStepEdit` sent only what Set had pushed, so a number typed and not Set vanished on the way to the review, which then showed the other changes, or none. An entry that is not a value is NEVER SENT AND NEVER RESET: it stays in its box exactly as typed, its reason goes on its row, the cursor goes to it, and nothing reaches the backend until it is corrected. The fields are text with a numeric keyboard rather than `type=number`, because a number field reports an entry it cannot parse as the empty string and loses it on the next redraw. What was typed is held per step and survives every redraw of the list. A row that differs from the routine is marked and has Reset. A review made before a later edit is WITHDRAWN and its token handed back, because a review on screen describing an edit nobody is asking for any more, with a live Save beside it, is the stale-confirmation case this panel already refuses elsewhere. Declaring a typed run WAITS for other pending changes instead of clearing them. A name typed beside `Ask me for it` and not asked for is a row error, not ignored. Leaving the routine counts the typing as work. Unchanged: the basis, the token, one operation per step, and the re-list after a stale-basis refusal."
supersedes: []
affects:
  - execution
source_paths:
  - plugins/studio/frontend/index.html
---

# The boxes are the change

## What happened

[[Studio-coherence-audit-2026-11-02]] finding 6, from reading the page:
`askStepChange` was reached only from a row's Set button, and `reviewStepEdit` sent
`stepChanges`. A wait or a coordinate typed into its box and not Set was not in that list, and
Review said nothing about it. The Architect's ruling:

> Remove per-row Set and collect changed fields when Review is pressed. Preserve invalid input
> with a row-level error; don't reset it or silently omit it. Keep existing basis and confirmation
> protections.

## What changed

| | before | now |
| --- | --- | --- |
| a wait or point typed and not confirmed | not sent, not mentioned | collected at Review |
| `abc` or `12.5` in a box | `type=number` reports `''`; Set sent `0` | kept as typed; row error; nothing sent; focused |
| the list redrawn (a removal toggled elsewhere) | the box rebuilt from the change list | the box rebuilt from what was typed |
| editing after Review | the old review stays, Save live | the review is withdrawn, its token cancelled |
| declaring a typed run with other changes pending | the other changes cleared | waits: *Review or reset the other changes first* |
| a name typed beside *Ask me for it*, not pressed | ignored | row error at Review |
| leaving the routine | counted only Set changes | counts every changed box, valid or not |

Removals and *Ask me for it* are still presses. They are a decision about a whole step, and the
second one replaces all of that step's text, so it stays an explicit act.

**The basis and token protections are untouched.** A stale-basis refusal still lists the routine
again with nothing asked. Row numbers read off a list that has been replaced would be somebody's
intention applied to another step, and that is the one reset this ADR keeps.

## Enforced by

- `plugins/studio` `TestTypedStepValuesAreReviewedWithoutASetAndAreNeverDropped` covers:
  - no Set on any row;
  - a typed wait reaching `ReviewStepEdit`;
  - `abc` not sent, and kept as typed with its reason on its row and the cursor in it;
  - the typing and the reason surviving a redraw;
  - a later edit withdrawing the review and cancelling its token;
  - leaving the routine naming the typing.

  Two mutations were run and both failed it: not collecting the wait, and rebuilding a box from
  the original value instead of the draft.
- `plugins/studio` `TestThePanelDeclaresATypedRunOnItsOwn`: with a wait typed, the declaration
  sends nothing and keeps the typing, and after Reset it goes alone with its basis.
