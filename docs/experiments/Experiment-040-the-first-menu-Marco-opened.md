---
type: experiment
status: complete
date: 2026-10-14
backend:
  - structure
game: rocketleague
fixture: "live, not a recording"
result: marco-grounded-play-hovered-it-clicked-it-and-recognised-the-screen-that-followed
supersedes: []
affects:
  - passive-observation
  - execution
---

# Experiment-040 — the first menu Marco opened

## Question

Can Marco find a control on a live game menu, press it, and recognise what it produced — without
a person answering anything during the run?

Everything before this stopped at a press that went out and changed nothing.

## Method

A bounded exploration session, authorised once at launch: ten actions, sixty seconds, Rocket
League foreground throughout, every action through legal Marco.

```
director perform-workflow --application rocketleague --workflow freeplay.workflow.json \
  --explore --from screen_a --max-actions 6 --seconds 50 --dwell 400ms --live \
  --frames hoverframes
```

## What happened

```
step 0: press control_1 on screen_a at (85,485), expecting screen_b
  target: the middle of a region at x 0.0000..0.0760, y 0.4222..0.4648
          (146x46 px at 0,456 in the window), re-acquired on this look
  authorised by the session (1 of 6)
step 0: pressed — lowered into legal Marco, compiled and run at (85,485)
  hover: the control's pixels changed by 136.7/255 over 4080 px while the pointer
         rested on it (the pointer is at 85,485)
screen_b inferred: the workflow goes screen_a -> screen_b, screen_a was established
         (confirmed, depth 0), and the screen now agrees with screen_b at 0.794
step 0: destination established — screen_b was inferred as visual_2 at depth 1
STOPPED SAFELY after 1 of 21 step(s)
```

The saved frames show it plainly: PLAY unlit before, and pale blue with "HIT THE FIELD" revealed
after. That is Rocket League's own hover state, produced by a pointer Marco moved.

**What was observed**: the control's position on this look, the pointer at the place it was sent,
136.7/255 of pixel change over the control's own rectangle, and a screen that differed afterwards.

**What was inferred**: that the new screen is `screen_b`. The workflow demonstrates
`screen_a -> screen_b` and the arrangement agreed at 0.794 — which is the workflow corroborating
its own expectation, and is reported at depth 1 and never as independent evidence.

**What was not established**: that the click caused any of it. No post-action evidence is
collected and `actions independently verified` says so.

## What stopped it

```
step 1: refused — unobservable: no proposal on this look agrees with any reading
        of this control
```

`control_2` is the 355x221 Training tile on the Play submenu. Across forty observations, nothing
the pixel proposer offered overlapped any saved reading of it. The run stopped rather than
pressing something else.

## What had been in the way

Three things, each of which had to be ruled out rather than guessed at.

**The pointer was never on the control.** `marcorunner` registers the OS host as `hosts["*"]` so
the driver's deferred guarantees cover Director programs — and one of them restores the cursor
after every run. Each Director operation is its own program, so the move was undone before the
press that depended on it. Releasing a held key is safety and still fires; putting the mouse back
is a courtesy to a person whose hand is on it, and there is no such person inside a sequence.
[[ADR-191-a-move-that-survives-until-the-press]]

**The capture could not answer where the pointer was.** GDI `BitBlt` does not composite the
cursor, so an empty rectangle in a frame is not evidence that nothing is there. `winctx.CursorPos`
answers it in one call, and the hover line carries it.

**The report said the opposite of what happened.** `the screen changed after the action: NO`
printed directly above its own destination being established, because an arrival on the very next
observation skipped the branch that recorded the change.

## Status of the claims

| claim | established by |
|---|---|
| Marco located a control on a live menu | a region re-acquired on that look, narrowed by agreement |
| the pointer reached it | `winctx.CursorPos` reporting 85,485 |
| the game reacted to the pointer | 136.7/255 over the control's own 4080 px |
| the menu changed | the destination arrived in the next observation |
| the new screen is `screen_b` | **inferred**, at 0.794, from the workflow's own expectation |
| the click caused the change | **no** — nothing collects post-action evidence |
| the rest of the route works | **no** — it stopped at step 1 |
