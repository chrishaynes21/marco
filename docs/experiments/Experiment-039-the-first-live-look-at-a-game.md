---
type: experiment
status: complete
date: 2026-10-13
backend:
  - structure
game: rocketleague
fixture: "live, not a recording"
result: the-path-holds-at-10fps-live-and-similarity-ranks-the-right-screen-first-without-establishing-it
supersedes: []
affects:
  - passive-observation
---

# Experiment-039 — the first live look at a game

## Question

Two things had never been done, and neither could be done offline:

1. Does the two-speed perception path hold its cadence against a live game window?
2. Does a saved workflow's screen evidence mean anything when matched against the screen actually
   in front of somebody, rather than against the recording it was learned from?

Everything up to [[ADR-188-performing-a-saved-workflow]] was measured by replaying a recording
through the production path. A recording is the same pixels every time.

## Method

Rocket League, open, at the main menu, in the foreground. Two commands, back to back, neither
able to send input and neither answered by anybody:

```
director observe-screen   --application rocketleague --seconds 15 --fps 10
director perform-workflow --application rocketleague --workflow freeplay.workflow.json \
                          --seconds 25 --fps 10 --patience 60      # DRY, stdin closed
```

The second stops at its first question by construction — nobody was answering, and a stand-in
answering on a person's behalf is the one thing this subsystem is arranged to prevent. What it
prints *before* stopping is the measurement: the ranked agreement of all nine saved screens
against the screen standing now.

## What happened

### The cadence holds, with room

```
150 look(s): 150 published, 0 dropped, 150 consumed
  not evidence: 0 with no pixels, 0 behind another window, 0 window change(s)
  acquisition:  median 19.59ms   p95 21.58ms   max 30.29ms
  proposer   :  median 34.33ms   p95 44.49ms   max 52.57ms
  whole look :  median 55.17ms   p95 66.76ms   max 76.05ms
  fold latency: median  0.00ms   p95  0.51ms   max  0.61ms
```

55ms of a 100ms budget at the median, 67ms at p95, nothing dropped across 150 consecutive looks,
and every look was evidence. [[ADR-177-acquisition-is-a-frame-interval-not-a-cost]] predicted
about 50ms for a full-resolution look on this machine and that is what it costs against a real
game.

### Similarity proposed the right screen

```
  Which saved screen are you looking at?
    screen_a   arrangement agrees 0.619
    screen_i   arrangement agrees 0.408
    screen_f   arrangement agrees 0.373
    screen_h   arrangement agrees 0.308
    screen_c   arrangement agrees 0.284
    screen_g   arrangement agrees 0.251
    screen_d   arrangement agrees 0.234
    screen_b   arrangement agrees 0.225
    screen_e   arrangement agrees 0.178
```

`screen_a` first, by 0.21 over the runner-up. The right answer was established independently,
twice: the step graph makes `screen_a` the hub — eight steps leave it, six return to it — and
frame 0000 of the recording is the main menu, with `control_1`'s box sitting on **PLAY**.

This is a measurement of a proposal and not of an establishment. The run stopped with *"nobody
could say which saved screen visual_1 is"*, which is correct: a ranking cannot say whether the
right answer is in the list, and that remains the thing a person adds.

### A stationary menu is not a stationary screen

The finding nobody was looking for. Fifteen seconds on the main menu with nothing touched:

```
2 session-local screen(s) settled (never durable):
  visual_1     63 look(s), settled at 3,  240 region(s) over 9 cell(s)
  visual_2     72 look(s), settled at 31, 351 region(s) over 9 cell(s)
2 distinct transition(s):  visual_1 -> visual_2 x1,  visual_2 -> visual_1 x1
changes by cause: 0 click, 0 automatic, 2 unknown
```

Two settled identities, a transition each way, cause `unknown` for both — nothing was pressed, so
nothing explains them. The difference is large: 240 proposed regions against 351.

**Recorded, not acted on.** One run of fifteen seconds is not grounds for changing a settle rule
or a binding threshold, and the honest summary is that the main menu animates enough to re-settle
and nobody has measured how often. What it predicts, and what the next supervised run should
watch for, is whether these transitions cost extra confirmations or invalidate an authorisation
mid-question — an authorisation carries the observation it was granted in, so a re-settle between
the question and the press is exactly the case the re-observation in
[[ADR-189-answering-without-taking-the-foreground]] refuses.

## What it cost

The run also found something the code could not have found, and it is in
[[ADR-189-answering-without-taking-the-foreground]]: a person cannot answer a terminal prompt
without taking the foreground, which is the precondition of the press the prompt is about.

## The live run that followed

One real left click, authorised by F7 with Rocket League in the foreground throughout, at the
middle of the PLAY button. **The menu did not change.**

The point was right — confirmed by the person watching. What was missing was time: the host sends
the move, the press and the release in one `SendInput` batch, and the menu had not yet noticed
the pointer was there. See [[ADR-190-the-pointer-arrives-before-the-button-goes-down]].

Three defects in how the run described itself were found in the same log, and all three are
fixed: it said it had "stopped before the first step" after performing an action; it said
"nothing was executed" beside a line reporting real input sent; and it counted one question
answered when two had been. A run that misdescribes itself is the failure this subsystem exists
to prevent, so they are held by tests rather than noted.

## Status of the claims

| claim | established by |
|---|---|
| the perception path runs live at 10fps with headroom | 150 looks, 0 dropped, p95 67ms |
| a saved arrangement ranks the right live screen first | 0.619 against 0.408 |
| a ranking establishes a screen | **no** — and it never will |
| the menu is one screen while nobody touches it | **no** — two settled, one run |
| anything was pressed | dry run: **no**. The live run that followed: one real click |
| the click activated the control | **no** — the point was right and the timing was not |
