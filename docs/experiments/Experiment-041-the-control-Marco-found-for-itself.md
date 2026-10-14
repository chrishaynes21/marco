---
type: experiment
status: complete
date: 2026-10-14
backend:
  - structure
  - vision
  - ocr
game: rocketleague
fixture: "live, not a recording"
result: three-real-clicks-to-free-play-with-the-last-target-recovered-from-live-evidence-and-the-arrival-unrecognised
supersedes: []
affects:
  - passive-observation
  - execution
---

# Experiment-041 — the control Marco found for itself

## Question

A saved control that locates nothing can never be performed, however long it is given. Can
Director re-establish it from the live screen — and refuse when the evidence does not support
one answer?

## Method

One bounded exploration session, authorised at launch, with both perception sources and the
repair path:

```
director perform-workflow --application rocketleague --workflow freeplay.workflow.json \
  --explore --learned --from screen_a --max-actions 6 --seconds 75 --dwell 400ms \
  --repair proposed.json --live
```

## What happened

**Main Menu → Play → Training → Free Play.** Three real clicks.

```
step 0: press control_1 on screen_a at (92,485)        [pixel proposer]
  hover: pixels changed by 144.1/255 over 4488 px
  destination established: screen_b at 0.741

step 1: press control_2 on screen_b at (577,640)       [learned detector]
  target: 354x221 px at 400,530
  destination established: screen_c at 0.590

step 2: control_3 locates nothing. Looking for it on the screen instead.
  the workflow is called "enter free play"; 65 candidate(s) carry text,
  1 of them words it uses
  re-grounded: the only control on this screen whose text ("FREE PLAY") is made of
               words from the workflow's own name ("enter free play")
  pressed at (411,482)
  hover: pixels changed by 12.5/255 over 4828 px while the pointer rested on it
  the screen changed after the action: YES
  screen_d: NOT established
```

A capture afterwards shows the Free Play submenu — `FREE PLAY` and `CREATE ONLINE FREE PLAY`.
**The navigation succeeded and Director could not establish that it had.**

## What was observed, and what was not

| claim | established by |
|---|---|
| each control's position on its own look | re-acquisition, agreement, and for step 2 a live repair |
| the pointer reached each one | `winctx.CursorPos` |
| two of three controls reacted to the pointer | 144.1, 12.5 and 5.4 of 255 over their own pixels |
| the screen changed after every action | the observation after it differed |
| the first two destinations are `screen_b`, `screen_c` | **inferred**, at 0.741 and 0.590, depth 1 and 2 |
| the third destination is `screen_d` | **no** — nothing recognised it |
| Marco reached Free Play | **yes, and Director cannot say so** |
| any click caused any of it | **no** — no post-action evidence is collected |

## The repair, and exactly how much it is worth

Three sources of evidence were available for `control_3` and only one of them is about identity.

- **Where things are**: the detector boxes the tiles; OCR reads every label at ~95% confidence.
  Localisation was never the problem.
- **Which one was meant**: the artifact calls it `control_3`, a generated name, with no label, no
  text and no container. The demonstration recorded where somebody clicked and not what they
  clicked. **The intent was never saved.**
- **The workflow's own name**: somebody typed "enter free play". The only human-supplied meaning
  in the file.

So the rule is strict and the strictness is the point: every word of a candidate's text must
appear in the workflow's name, whole words, one direction, and **exactly one** candidate may
qualify. 65 carried text; one qualified. `CUSTOM TRAINING`, `TUTORIAL`, `AERIAL`, `STRIKER` and
`GOALIE` did not.

**This is not independent confirmation.** A unique match among 65 candidates makes the selection
defensible for this experiment; it is the workflow corroborating itself, the same weakness
`Arrival` reports as depth, and it would be worth much less on a screen where two labels shared a
word with the task name. It is used for one session, written nowhere, and `--repair` emits a
proposal beside the artifact carrying the new region, the evidence, the original reading it would
replace, and whether the action changed the screen and whether its destination was recognised —
two fields, because a repair that produced an unrecognised transition is a good region and a
recognition problem.

Worth a reviewer's eye: the repair grounded the **text label** (340..482 x 466..500), not the
tile. It works because the label is inside the clickable area.

## What it cost to find out

**The target line lied.** It printed the saved full-window rectangle while pressing the correct
point: `intentOf` filled `Where` from the plannable action and a later loop overwrote it with the
matched outcome. The good answer was computed and then discarded for display only.

**`--from` is a claim nobody checks.** A run in which Rocket League was not in front for 125 of
199 observations pressed once anyway, during a window when it was, and produced a confident click
that did nothing. The claim was right that time and the foreground was wrong — but a wrong
`--from` produces exactly the same shape, and the arrangement ranking that would flag it is
computed for the asking path and never consulted for the claimed one.

## The gap worth measuring

**Goal reached: yes. Goal recognised: no.** Those are different capabilities and today they
disagree. Closing that gap matters more than any further sophistication in the harness.
