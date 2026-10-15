---
type: experiment
status: complete
date: 2026-10-14
backend:
  - structure
  - ocr
  - vision
game: rocketleague
fixture: "live Rocket League, main menu, 1920x1080, --explore --learned --reading"
result: the-menu-had-moved-and-re-acquisition-cannot-tell-a-button-from-the-pitch-behind-it-which-text-now-refuses
supersedes: []
affects:
  - passive-observation
  - execution
---

# Experiment-044 — two live runs that pressed nothing, and pressed grass

## Question

[[Experiment-043-a-reading-that-arrives-late]] put the semantic observer inside its measured
latency window and said what was left: whether Rocket League's OCR on this machine lands inside
eight looks, and whether Marco reaches Free Play and **knows** it reached Free Play.

Four questions were set for the run: does live OCR arrive quickly enough to separate Training
from Free Play; does Marco navigate Main Menu → Play → Training → Free Play; does it recognise
`screen_d` on arrival; and if not, is the failure OCR freshness, screen settling, or destination
matching.

**None of them got asked.** The run never left the main menu, and what stopped it was none of
the three.

## Run one — nothing was performed

```
  step 0: refused -- refused for 60 observation(s): unobservable: no proposal on this look
          agrees with any reading of this control, though a region that contained the
          original press is still here

  observations Marco could use          61 of 126
  the learned detector                  offered 126, answered 8, dropped while busy 116
  the words on each look                offered 126, read 16, dropped while busy 108
      offered 16, admitted 6, refused as stale 2, unusable look 8
  real input sent                       NO
```

Two separate things, and the smaller one first: **63 of 126 looks were not evidence at all**,
because the game was not the window in front. The run was launched three seconds after the user
typed, and the terminal still had the foreground. That cost half the perception and eight of the
sixteen readings; it did not cause the failure, because the other 61 looks refused too.

### What the capture showed

Comparing the recording's main menu against the live one, frame against frame:

| | recorded | live |
|---|---|---|
| items | PLAY, **ITEM SHOP**, **ROCKET PASS**, GARAGE, CLUB, PROFILE, SETTINGS, QUIT | PLAY, GARAGE, CLUB, PROFILE, SETTINGS, QUIT |
| NEWS / WEEKLY CHALLENGES | present | **absent** |
| PLAY | y 473..505 | y 565..605 |
| QUIT | y ~822 | y ~822 |

**The list is bottom-anchored.** Signed out, it loses two entries and both side panels, and the
whole block slides down 95px. `control_1` was recorded at y 465..512; that rectangle now covers
pitch.

### And the repair could not be asked

`control_1` is not a band — it locates perfectly well. It is simply not where it was. So the
refusal was `StepControlUnresolved`, and the regrounder is asked only on `StepControlUnlocated`.
Sixty identical refusals with the repair for exactly this sitting unused beside them.

That is [[ADR-196-a-control-that-moved-is-not-a-control-that-was-covered]], and the rule it adds
is that the **patience window** is what separates a control that was covered — which comes back
within it — from one that moved, which never does.

## Run two — one action, into the grass

With that rule in, and the game in front for 81 of 83 looks:

```
  step 0: press control_1 on screen_a at (146,490), expecting screen_b
    target: a region at (100,478) 128x24 px, re-acquired on this look
  step 0: pressed
    hover: the control's pixels changed by 0.0/255 over 2208 px, below the 8 that
           counts as a reaction
  step 0: refused -- destination screen_b not established within 60 observation(s)

  the words on each look   offered 10, admitted 8, refused as stale 2, unusable look 0
  real input sent          YES          the screen changed after the action   NO
```

**The new rule never fired, because the step was never refused.** A proposer boxed a 128x24 patch
of pitch at (100,478), that box and the saved reading each contain the other's centre, and
re-acquisition reported MATCHED. Marco moved the pointer there, rested 400ms, measured that
nothing under it changed, and pressed anyway — 75px above the PLAY button, which was still
sitting where the first run's capture found it.

## The finding

```go
// sameControl reports whether any rectangle in one set is the same rectangle as any in the other.
// "The same rectangle" is mutual centre containment: each contains the other's centre.
```

**Re-acquisition consults no pixels and no text.** `grep -n "Appearance\|pixels\|similar"
internal/director/observe/reacquire.go` returns nothing. So *"re-acquired on this look"* means
only **a proposer drew a box in the same place as the recorded one** — and the two runs differ
only in whether a proposer happened to do that over a patch of grass.

The consequence is sharper than a missed navigation:

- A wrong match is **indistinguishable from a right one** at the moment of pressing.
- A wrong match reaches the press **ahead of** the repair that would have been right. The
  regrounder grounds `control_1` by the word PLAY, read live, and geometry answers first.
- The recorded coordinates are therefore acting as a replay target whenever any source boxes
  something near them, which is the thing
  [[ADR-182-two-controls-that-could-never-be-found-again]] and the Learn licence exist to prevent.

**One measurement did say so, and nothing used it.** The hover probe reported `0.0/255 over
2208 px`. On the same button in [[Experiment-041-the-control-Marco-found-for-itself]] it reported
`136.7/255`. Marco took fresh evidence that the thing it was about to press was not a control,
printed it, and pressed.

## What this does not claim

**That the hover reading is the fix.** A control that does not highlight is an ordinary control,
and `TestAControlThatDidNotReactIsStillPressedAndTheReportSaysSo` holds that on purpose. Two
samples do not make 0.0/255 mean "not a control", and inventing a threshold here is the failure
mode this subsystem has a measured history of.

**That the four questions were answered.** They were not reached. What the run does say about the
first of them is that OCR landed inside the window on real game frames: of 10 readings offered,
**8 admitted and 2 refused as stale**, with none lost to an unusable look. The halved frame from
[[ADR-195-a-reading-that-arrives-after-the-look-it-describes]] holds up live — on a screen that
never changed, so it was never asked to discriminate anything.

## And then the answer, on the same menu

One frame of that signed-out menu, through the production detector and the production OCR
at the production scale, against the region the live run actually matched:

```
  the detector proposed 7 region(s); OCR read 35 word(s) at half size
  * "PLAY"        x  60.. 367  y  564.. 608    the only candidate named in the task
    "GARAGE"      x  59.. 368  y  610.. 654
    "CLUB"        x  59.. 368  y  658.. 703
    "PROFILE "    x  59.. 367  y  706.. 750
    "SETTINGS"    x  59.. 367  y  752.. 797

  the region the live run pressed:  x 100.. 227  y 478.. 501
    verdict: contradicted by a better-named candidate
    instead: "PLAY" at x 60..367 y 564..608
```

**The press that was sent would not have been sent.** The matched region reads nothing, the
PLAY row 86px below it reads PLAY, and exactly one candidate on the whole screen is worded
in "enter free play" -- so the repair that follows the refusal has an unambiguous answer and
grounds the row the person would have clicked.

The rule is [[ADR-197-a-geometric-match-nominates-and-does-not-identify]]. It is the
asymmetry from [[ADR-194-a-screen-is-also-what-it-says]] one level down: text may REFUSE a
match and may never propose one, and the contradiction is a competing candidate rather than
a silence -- because an icon, a label drawn outside its clickable rectangle and an OCR that
missed all look identical from here, and refusing on any of them would make every unlabelled
control in every application unpressable on a bad frame.

**Not performed live.** The verification above is offline, on real pixels from the menu that
produced the defect.

## The decision this produced

What corroborates a geometric match before it becomes a press. The candidate that fits what is
already built is the asymmetry
[[ADR-194-a-screen-is-also-what-it-says]] established one level up: **text may refuse a match and
may never propose one.** A control matched to a region where nothing is read, while the word the
workflow's own name points at is plainly on screen elsewhere, is a match to refuse — and the
refusal reaches the regrounder, which grounds it by that word.

It needed the look's words on the readiness path, which they did not reach. That is a
trust-boundary change — it decides what licenses a press — so it was put to the person who owns
that boundary rather than taken in passing, and it is
[[ADR-197-a-geometric-match-nominates-and-does-not-identify]].

The bound that came back with the approval is the one that makes it safe: **absence of text must
not invalidate a match.** A control may be an icon, its label may be drawn outside its clickable
rectangle, or OCR may have missed it, and those are indistinguishable from here. The meaningful
contradiction is a competing, positively identified candidate elsewhere — nothing less.

## Related

- [[ADR-196-a-control-that-moved-is-not-a-control-that-was-covered]] — the first rule this produced.
- [[ADR-197-a-geometric-match-nominates-and-does-not-identify]] — the second, and the one that
  refuses the press.
- [[ADR-193-a-control-re-grounded-from-the-workflows-own-name]] — the repair, and its bounds.
- [[ADR-190-the-pointer-arrives-before-the-button-goes-down]] — the hover probe that said no.
- [[Experiment-041-the-control-Marco-found-for-itself]] — the same button, three clicks, 136.7/255.
- [[Experiment-043-a-reading-that-arrives-late]] — the latency this was to confirm live.
