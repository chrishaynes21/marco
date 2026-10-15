---
type: experiment
status: complete
date: 2026-10-14
backend:
  - structure
  - ocr
  - vision
game: rocketleague
fixture: "live Rocket League, signed-in main menu, 1920x1080, --explore --learned --reading"
result: the-target-was-right-and-the-pointer-never-went-there-an-open-defect-that-outlived-the-process
supersedes: []
affects:
  - execution
  - passive-observation
---

# Experiment-045 — the pointer that would not leave

## Question

[[ADR-197-a-geometric-match-nominates-and-does-not-identify]] was verified offline against the
frame that produced [[Experiment-044-two-live-runs-that-pressed-nothing-and-pressed-grass]]. This
run was to ask it live, on the signed-in menu: does Marco reach Free Play, and does it know it
reached Free Play.

**It did not get to ask.** One action was sent, nothing moved, and the run ended at step 0. What
it found instead is a defect in the half of the system that was not under test.

## What the run did

The game was checked passively before launch — signed in, the 8-item menu, NEWS panel up, PLAY at
the height the demonstration recorded it. Rocket League was the window in front.

```
  step 0: press control_1 on screen_a at (84,485), expecting screen_b
    target: the middle of a region at x 0.0000..0.0740, y 0.4222..0.4648
            (142x46 px at 0,456 in the window), geometrically re-acquired
    authorised by the session (1 of 6)
  step 0: pressed
    hover: the control's pixels changed by 0.0/255 over 3944 px, below the 8 that
           counts as a reaction (the pointer is at 390,937)
  step 0: refused — the action was performed and screen_b was not established
          within 60 observation(s)

  observations Marco could use    64 of 66
  the learned detector            offered 66, answered 3, dropped while busy 61
  the words on each look          offered 66, read 6, dropped while busy 58
      offered 6, admitted 4, refused as stale 2, unusable look 0
  real input sent                 YES
  the screen changed after it     NO — the application did not visibly respond
```

**The target was right.** That region is the one
[[Experiment-041-the-control-Marco-found-for-itself]] pressed successfully: `explore6.log` aimed
at (85,485) inside a 146x46 box at the same origin, and the button lit up 136.7/255. Nothing was
wrong with perception, selection or the artifact on this run.

## The defect

**The pointer was at (390,937). It was sent to (84,485).**

The move reported `StatusCompleted` — a failed move refuses the step with *"the pointer could not
be moved to the control"*, and it did not. Four hundred milliseconds later the operating system
said the cursor was 306px right and 452px down from where legal Marco had put it, and the press
that followed was aimed at a control the pointer had never been on.

The probe exists for exactly this. It asks the OS where the cursor is rather than trusting the
frame, because *"GDI BitBlt does not composite the cursor, so an empty rectangle in a frame is not
evidence that nothing is there"* — and the sentence it is there to make possible is **the pointer
is somewhere else**, not **the control ignored the pointer**. It said the first one.

### And it outlived the process

The user reported, after the run: **the pointer kept returning to the position it held when the
run started**, and the machine had to be restarted to clear it.

Sampled minutes after the run, with the run long over:

| | |
|---|---|
| `director` / `vision` / `marco` / `tesseract` | **none running** |
| cursor | **(390,937)** — unmoved, the same coordinates the probe reported |
| mouse buttons held | none |
| `GetClipCursor` | `-1920..1920 × 0..1080` — the whole virtual desktop, **not clipped** |

So whatever held the pointer was still holding it after every Marco process was gone, and no
Marco process was there to be stopped.

## What this rules out

- **Not a coordinate bug.** `normAbsolute` maps through the virtual desktop and handles a negative
  origin, which this machine has — there is a second monitor at x -1920..0. A dropped origin would
  have put the pointer on the left-hand screen, not 306px right on the primary.
- **Not `SetCursorPos` being ignored by a game reading raw input.** The backend sends a real
  injected absolute `MOUSEEVENTF_MOVE` through `SendInput`, and only falls back to `SetCursorPos`
  if that is rejected.
- **Not the cursor courtesy, on paper.** `Host.CursorSnapshot` is the one mechanism in the tree
  that reproduces the reported symptom exactly — it records the cursor and puts it back when a
  route's program ends. `actionHost` in `internal/platform/marcorunner` deliberately withholds it
  from Director actions, and the comment that withholds it cites **this same 0.0/255 Rocket League
  measurement** from an earlier run as the reason. A Director operation is one step of a sequence,
  not one errand, so restoring the pointer after the move would undo the move before the press.
  That reasoning is right and the code does what it says. It is named here because it is the
  nearest mechanism to the symptom, and because its withholding is the first thing to re-verify —
  not because it is established as the cause.

## What this does not claim

**That Marco caused it.** The only sample taken during the run was after the move, so there is no
measurement of where the cursor was before Marco touched it. It may have been pinned already — the
readiness capture a minute earlier showed ITEM SHOP highlighted, which is the row *below* PLAY and
not the row either coordinate points at. A game, an overlay, a controller with a drifting stick
mapped to the pointer, or a driver can all pin a cursor, and none of them was sampled.

**That the earlier 0.0/255 readings were this.** `live-run-045.log` reported the pointer at
(146,490), which *is* where that run aimed — there the pointer arrived and the pixels under it
genuinely did not change, because they were grass. The two look alike in the log and are different
findings, which is precisely why the probe prints the position.

## The other finding, which was free

**ADR-197 was never asked anything.** The step was pressed on the first or second observation, and
the first OCR reading had not arrived — 6 readings were read across 66 looks, and 4 admitted. With
no words on the look, `ControlIdentityOf` returns `unexamined`, which permits the press by design:
missing evidence is not contradictory evidence.

That bound is right and is not up for revision. But it means **the identity gate is vacuous on the
first look of a run**, which is the look a workflow's first step is always pressed on. A gate that
cannot fire when it is needed most is worth a decision of its own: either the first action of a run
waits for one admitted reading, or the report says plainly that the press went out unexamined,
rather than listing it beside the corroborated ones as *geometrically re-acquired*.

## The hold

**No further live run until the pointer is understood.** A bounded experiment that leaves somebody
rebooting their machine has exceeded its bound, whatever moved the cursor. The next live session
needs, before anything is sent: the cursor position sampled and reported *before* the first move,
the same sample after it, and the run stopped if they disagree — because a move that reports
completed and did not happen is currently indistinguishable from a control that did not react.

## Related

- [[Experiment-041-the-control-Marco-found-for-itself]] — the same region, pressed, 136.7/255.
- [[Experiment-044-two-live-runs-that-pressed-nothing-and-pressed-grass]] — the run this was to
  close, and the pointer reading that was right there.
- [[ADR-190-the-pointer-arrives-before-the-button-goes-down]] — the probe, and why it asks the OS.
- [[ADR-197-a-geometric-match-nominates-and-does-not-identify]] — asked nothing, for a reason worth
  a decision.
