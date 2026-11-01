---
type: adr
status: accepted
date: 2026-11-01
decision: "ACCEPTED — A ROUTINE IS A ROW THAT OPENS WHERE IT IS, PLAY NEVER LEAVES THAT ROW, AND ONLY THE DISCLOSURE TOGGLES. The separate selected-routine card is gone: choosing a routine used to draw a panel BELOW the list, so the thing being read and the thing it was about were a scroll apart and a list of eleven routines pushed the panel off the screen. Each row now carries a chevron and opens in place. ONE PANEL, CARRIED, AND NEVER REBUILT — `routineChosen` is moved into whichever row is open and parked in an empty `#routinePark` when none is, which answers three questions with one mechanism: unsaved inputs survive, because moving a node keeps the text in its boxes, the name half-typed into the rename field, which source panes are open and the step changes collected against a list, where a panel rendered per row would lose all of it on every poll; ids stay unique, because every function in this file reaches these elements by name and a panel per routine is eleven `routinePlayBtn`s; and ONE panel IS 'open one routine at a time', since there is no second place for a second one to be. THE DISCLOSURE IS THE ONLY CONTROL THAT TOGGLES, and that is structural rather than a `stopPropagation` in each control: with the panel INSIDE the row, every press in it — Rename, a step's Remove, a character typed into an argument box — would otherwise have bubbled to the row and closed the thing being used, and no control added later has to remember to stop anything. It is a real `<button>` with `aria-expanded` and `aria-controls`, so Enter and Space work and what it does is announced. PLAY IS ON THE ROW, OPEN OR SHUT, ONCE PER ROUTINE. A shut row's Play is live whenever the routine can run at all, because pressing it OPENS the row and puts the cursor in whatever it still needs — the window chooser or the first unfilled value — so the press always does something; the open row's Play keeps the rule it had, dead until the window and the values are there, because those are on screen in front of the person. A routine that cannot run has a dark Play with the first sentence of its refusal beside it. LEAVING IS THE ONLY THING THAT CAN LOSE WORK, SO IT IS THE ONLY THING THAT ASKS, and it asks by refusing once and naming what would go in the person's own terms — the arguments they typed into, the count of changes they collected — never 'unsaved changes'. A value a REQUEST proposed is not work in progress: nobody typed it and asking again produces it again, so it is excluded by VALUE, which keeps a proposal somebody has since corrected. No new backend call, no owner-library change, no Studio rebuild."
supersedes: []
affects:
  - execution
source_paths:
  - plugins/studio/frontend/index.html
---

# A routine opens where it is

## What happened

The Architect, on the panel that [[ADR-289-the-panel-says-less-and-play-is-bigger]] had just
trimmed:

> Replace the separate selected-routine card with an inline accordion. Put a chevron beside each
> routine name; clicking the row or chevron expands it, clicking again collapses it. Open one
> routine at a time.
>
> Keep Rename beside the expanded routine's name, with Play and its required inputs immediately
> below. Editing, aliases and maintenance stay within that routine's expanded area.
>
> Preserve unsaved inputs when clicking Rename, Play or other controls—those clicks must not
> toggle the row. If switching routines would discard pending edits, make that explicit.
>
> Use an accessible button with `aria-expanded` and keyboard support. Render collapsed, expanded
> and long-editor states at narrow and normal widths.
>
> Reuse existing backend calls. No new behavior, owner-library changes or Studio rebuild in this
> slice.

and, on the button:

> Yes—keep **Play on the collapsed row**, with the chevron and routine name on the left, Play on
> the right. [...] Clicking Play must not also toggle the row. Keep a single primary Play button
> per routine—don't duplicate it inside the expanded section. All existing confirmations and
> guards remain.

## One panel, carried

The obvious implementation is a panel per row. It is the wrong one, and the reason is the
Architect's own requirement: *preserve unsaved inputs*.

`routineChosen` is **moved** — into `#routineHere-<key>` inside whichever row is open, and into
an empty `#routinePark` when none is. It is the same elements every time, and moving a node keeps
what rebuilding it throws away:

| | |
| --- | --- |
| the text in the argument boxes | a value somebody is part way through typing |
| the rename field | a name somebody is part way through typing |
| which `<details>` are open | the two source panes of a review |
| the step changes collected | positions read off the list on screen |

Two more things fall out of it. **Ids stay unique**: every function in this file reaches these
elements by name, and a panel per routine is eleven `routinePlayBtn`s. And **one panel IS "open
one routine at a time"** — there is no second place for a second one to be, so the two cannot
disagree.

The cost is one rule: `drawRoutines` must carry the panel OUT before `list.innerHTML = ''` and
back in after. That is stated at both ends, because the failure is silent and total.

## Only the disclosure toggles

The row used to carry the handler. With the panel inside the row, that is not a small problem:
every press in the panel would bubble to the row and close the thing being used.

The answer is structural and not a `stopPropagation` in each control. The disclosure is a
`<button>` that is a SIBLING of the panel, so a press in the panel never reaches it, and no
control added later has to remember to stop anything. It carries `aria-expanded` and
`aria-controls`; being a real button is what gives it Enter, Space and a focus ring for free.

## Play, on the row, once

Two different rules for one button, and the difference is what a press can usefully do.

| | enabled when | a press |
| --- | --- | --- |
| **a shut row** | the routine can run at all | opens it, and puts the cursor in what it still needs |
| **the open row** | …and a window is chosen and the values are given | runs it |

A shut row knows neither the window list nor what the routine asks for — both are read when it
opens — so the press sets `wantsPlay` and the answer is taken up in `paintRoutinePlay` as soon as
**both** reads land. Asking earlier would focus the first field of a routine that turns out to
need nothing, or start a run with no window chosen.

The open row keeps the rule it had — *dead rather than live-and-then-refused* — because there the
window and the text are on screen, and a press that could only answer "fill that in" would be
answering about something the person is looking at.

A routine that cannot run has a dark Play and **the first sentence of its refusal beside it**.
The whole refusal is in the panel, under the name, where a second short copy would be the same
sentence twice.

## Leaving says what it would throw away

Leaving is the only thing here that can lose work, so it is the only thing that asks.

It asks by **refusing once**. Not a `confirm()`: a modal is a dialog this window does not
otherwise own, it cannot be read by a test that renders the page, and headless browsers answer it
themselves. The first press names what would go and does nothing; the same press again goes.

And it names it in the person's own terms — *the text you typed for "item"*, *the 3 changes you
collected to its steps* — never "unsaved changes", which is a sentence that could be about
anything and is how a warning stops being read.

**A value a request proposed is not work in progress.** Nobody typed it, and asking again
produces it again, so stopping to warn about it would be this window asking somebody to confirm
its own work. The exclusion is by VALUE and not by name, which keeps a proposal somebody has
since corrected: that one is theirs.

## Three defects found on the way

None of them is about the accordion. All three were covered by one accident.

**A draw did not record what it drew.** Every caller that changed the selection set
`shownRoutines = ''` to force a redraw, and nothing set it afterwards — so the next poll compared
the real list against `""` and drew everything again, 2.5 seconds later. Harmless while a row was
a row. With the panel inside the row it moves the panel, takes focus out of whatever is being
typed in, and — the way it was found — **wipes the warning that leaving would lose somebody's
work, about two seconds after they read it**. The signature is one function now and the draw is
what records it.

**Forgetting a proposal left it on screen.** `forgetProposals` cleared `argValues` and did not
repaint, so the box kept the proposal's text under a clause that had correctly gone: two halves
of one fact disagreeing in front of somebody. It passed because the stray redraw above repainted
it. A rule nothing obeys and an accident that covers for it are indistinguishable from working.

**A half-typed name followed the panel.** The rename form stayed open across a change of routine
with the LAST one's name in it — an offer to call *save the file* *cloud pull stone*, which is a
different routine that already exists. Every other panel here already states this: `reviewing`,
`moving`, `choices`, `stepList` and `argNeeds` are all dropped when the routine they belong to is
not the one on screen. This one was not.

## And a fourth, in the stylesheet

The narrow rules for the step editor were written in the same `@media` block as everything else
and **did nothing**. A media query is not more specific than anything: `.steprows .sr.four`
inside one and the same selector outside it weigh the same, so the one written LATER wins — and
the four-column rule is ninety lines below. The block fired, the row layout in it took effect,
and the grid columns silently did not. They are at the end of the stylesheet now, with the reason.

Which exposed a fifth: **the harness was rendering at 756px**, four pixels under the breakpoint.
"An ordinary width" was whatever the browser happened to pick, so the claim about four columns
was being made about a narrow window and the width people actually use was covered by nothing.
`renderForCountersAt` states the width, and the tests that are about one now say which.

## Narrow

The rail is 214px and `main` pads by up to 56px a side, so at 460 the column a routine is read in
was 182px wide. That is not a tight layout — the disclosure shrank to **zero** and the row had a
name on it that could not be seen.

Under 760px the rail narrows to 132px, the gutter goes to 16px, the row's controls wrap under the
name rather than taking width from it, and the step editor lays its four parts out as three
lines — number and verb, then what the step does, then its controls — so the value keeps a whole
line to wrap in and the buttons keep their labels.

| | |
| --- | --- |
| ![collapsed, normal](images/studio-ui/8-rows-collapsed-normal.png) | ![collapsed, narrow](images/studio-ui/8-rows-collapsed-narrow.png) |
| ![expanded, normal](images/studio-ui/8-rows-expanded-normal.png) | ![expanded, narrow](images/studio-ui/8-rows-expanded-narrow.png) |
| ![editor, normal](images/studio-ui/8-rows-editor-normal.png) | ![editor, narrow](images/studio-ui/8-rows-editor-narrow.png) |

## What this does not do

- No new backend call. Every binding this uses was already exported and already called.
- It does not change what Play does, what it confirms, or any guard before a press.
- It does not change what a rename, a move, an edit or a rebuild preserve or refuse.
- It does not touch the owner's library, and Studio was not rebuilt.

## Enforced by

Every claim is **rendered in headless Edge**, because all of them are about where things are and
what a press does. The panel's position is a fact about the running page and not about the
markup; whether a press on Rename closes the row it is in depends on which element the handler is
attached to; and whether Play is bigger than the controls beside it is computed by the cascade. A
markup test would pass with the panel nailed to the bottom of the page.

- `plugins/studio` `TestARoutineOpensWhereItIsAndPlayStaysOnItsRow` — the list opens with every
  row shut and the panel parked; the disclosure is a button that names what it controls; opening
  one puts the panel inside that row; Rename and a control inside the panel do NOT collapse it;
  opening a second closes the first and takes the panel; pressing the open one collapses it; a
  shut row's Play opens it and asks for the value it needs without running, and runs the one that
  needs nothing; three Play buttons for three routines with none inside the panel; and the
  routine that cannot run has a dark Play with its reason beside it.
- `plugins/studio` `TestLeavingARoutineSaysWhatItWouldDiscardAndKeepsItUntilAgreed` — the first
  press warns and stays, naming the argument rather than saying "unsaved"; the warning, the text
  and the open routine all survive a poll; the second press leaves.
- `plugins/studio` `TestAValueARequestProposedIsNotSomethingSomebodyTyped` — a request proposes a
  value into the box and opening another routine does not stop to ask about it.
- `plugins/studio` `TestPlayIsTheBiggestThingOnARowAndTheRowReadsLeftToRight` — **at a stated
  width**: chevron, name, then Rename and Play to the right of both, on one line, with Play
  taller and in bigger lettering than the control beside it.
- `plugins/studio` `TestTheRowsHoldTogetherInANarrowWindow` — at 460px: no sideways scroll, the
  name keeps the line, and the step editor opens inside the row with the value having the line
  rather than a ribbon between the verb and the buttons.
- `plugins/studio` `TestTheStepRowsAreCompactAndTheWarningStaysReadable` — now **at a stated
  width too**, and asserts the grid really is four columns there.
- `plugins/studio` `TestTheJourneyPutsTheQuestionFirstAndPlayUnderWhatIsSelected` — the panel's
  markup holds no `routinePlayBtn`, so the duplicate the ruling names cannot be reintroduced
  quietly.

Twenty-four mutations, twenty-four killed — among them: the disclosure stops saying whether it is
open, the disclosure is not a button, the row carries the handler again, the panel is not carried
out before the list is emptied, the panel is not carried back in, pressing the open routine opens
it again, leaving never asks, typed text does not count, the warning says "unsaved", a draw no
longer records what it drew, Play on a shut row does not open it, a deferred Play is never taken
up, Play runs whatever it is waiting for, the panel grows a second Play, a shut row loses its
Play, the row cannot wrap, the rail keeps its width, the editor keeps four columns when narrow,
the narrow rules go back where they lose the cascade, Play is as big as everything else, Rename
moves to the right of Play, a forgotten proposal stays in the box, a proposal counts as typing,
and a half-typed name follows the panel.

## See also

- [[ADR-289-the-panel-says-less-and-play-is-bigger]] — the trim this rebuilds on, and where Play
  got its size
- [[ADR-285-a-routine-can-be-called-something-else]] — the rename whose button sits on the row
- [[ADR-284-the-step-editor-is-a-list-of-rows]] — the four columns, and why they are fixed
- [[Studio-alpha-UI-slice-2026-10-31]] — the slice this follows
- [[Studio-journey-discoverability-2026-10-31]]
