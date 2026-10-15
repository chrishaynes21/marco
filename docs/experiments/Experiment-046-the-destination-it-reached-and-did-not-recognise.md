---
type: experiment
status: complete
date: 2026-10-14
backend:
  - structure
  - ocr
  - vision
game: rocketleague
fixture: "live Rocket League, signed-in main menu, 1920x1080, --explore --learned --reading; and the rl-freeplay-1 recording"
result: two-correct-presses-reached-the-training-page-and-the-standing-screen-never-changed-cause-not-established-because-the-run-kept-no-record-of-what-its-observer-saw
supersedes: []
affects:
  - passive-observation
  - execution
---

# Experiment-046 — the destination it reached, and did not recognise

## What the run did

[[ADR-198-a-move-that-reported-completed-and-did-not-happen]] was in, and the pointer answered
for the first time since [[Experiment-041-the-control-Marco-found-for-itself]].

```
  step 0: press control_1 on screen_a at (109,491)
    target: 34x22 px at (92,480), geometrically re-acquired
    pointer: the operating system puts the pointer at (109,491), where it was sent
    hover:   the control's pixels changed by 161.3/255 over 748 px
  step 0: destination established — screen_b inferred as visual_2, arrangement agreed 0.662

  step 1: press control_2 on screen_b at (577,640)
    target: 354x222 px at (400,529), geometrically re-acquired
    pointer: the operating system puts the pointer at (577,640), where it was sent
    hover:   changed by 5.2/255 over 78234 px, below the 8 that counts
  step 1: refused — screen_c not established within 60 observation(s); the screen
          standing is visual_2

  STOPPED SAFELY after 1 of 21 step(s). 2 of 6 actions. 115 of 117 looks usable.
  words: offered 20, admitted 17, refused as stale 3, unusable look 0
         no reading ever contradicted the standing screen
```

A passive capture taken the moment it stopped shows **FREE PLAY, CUSTOM TRAINING, TUTORIAL,
AERIAL, STRIKER, GOALIE** under a TRAINING watermark. Both presses landed. Marco walked Main
Menu → Play → Training and **did not notice it had arrived**: `visual_2` stood from step 0's
arrival, through step 1's press, and for the sixty observations after it.

This is the product metric reading **goal reached 1, goal recognised 0**.

## What was ruled out, and how

### The look rate is not the cause

The live run took 117 looks in ~75s — about 640ms a look, six times slower than the recording,
because each look captures, proposes and probes. The recorded Play → Training transition was
replayed through the production fold at 100ms, 300ms and 600ms a look:

```
  one look every 6 frame(s) (600ms)
    frame 90   play    visual_2   <- SCREEN CHANGED
    frame 114  train   visual_2
    frame 120  train   visual_2
    frame 126  train   visual_3   <- SCREEN CHANGED
    visual_2 says: match offline play private
    visual_3 says: aerial back field goalie hit invited players practice solo striker
```

**It separates at every rate**, within two looks of the page appearing, and `text refused a
merge` is not what does it — the arrangements alone are 0.391 apart, well under the 0.55 that
would merge them. The text veto is not even reached.

### The learned detector is not the cause

`fastobserve` builds the arrangement from `regions`, never from `cands`, and says why: a channel
that arrives ten looks late and intermittently would make a screen's signature depend on whether
inference had landed. The detector answering 9 of 117 looks cannot move screen identity, by
construction. It costs candidates for targeting, and nothing here.

### The capture is not the cause

The one live Rocket League frame taken immediately after the run proposes **76 regions**, inside
the recording's range for that page (58–97). The live capture path produces a healthy
arrangement.

### A measurement that proved nothing, recorded because it cost an hour

A 40-frame burst "of the Training page" captured **Trackmania** — the user had moved on to
another game, and `--application` returns the covering window's pixels under the target's name.
Its 51-to-400 region counts and 11-of-39 sub-threshold pairs are a racing game in motion, not a
menu. That window capture is not occlusion-safe is a known trap here, and it caught nobody
twice; the lesson that generalises is that a capture named after a window should be **looked at**
before it is measured.

## The cause is not established, and the reason is the finding

Nothing reproducible explains why `visual_2` held. The recording does not reproduce it at any
cadence, the detector cannot have done it, and the capture is sound.

**The run keeps no record of what its observer believed.** Its report could say how many looks
were usable, how many readings were admitted and that none contradicted the standing screen —
and every one of those numbers is equally consistent with:

- an arrangement that never differed from the Play page's at all;
- one that differed on some looks and never held for the three it takes to settle, each
  near-miss resetting the pending run;
- a standing screen whose vocabulary had learned the Training page's words, after which a
  Training look agrees with it and the text veto correctly declines to fire.

The third is a real path through the rule and worth stating precisely, because it is the one the
report's own wording hides. A screen's vocabulary needs `TextStableLooks` readings before it may
contradict anything. The Play page was left within a few looks of being reached, so it may never
have had two; a screen that cannot contradict still ACCUMULATES, and what it accumulates next are
agreements between consecutive Training readings. `no reading ever contradicted the standing
screen` is then not evidence that the pages read alike — it is what a correctly-working veto
prints when the screen it is defending has quietly become the other page.

**Which of the three it was cannot be decided from this run's output.** That is the defect being
fixed here.

## The correction

The report now prints what the observer believed: the screens in the order they were settled on,
what each was READ to say, and the stabiliser's own counters.

```
  the screens this run saw                   2 screen(s) over 117 look(s)
      visual_1        12 look(s)   club garage play profile quit settings
      visual_2       103 look(s)   aerial goalie striker
      in this order  visual_1 -> visual_2
      and the fold   settled 98, unsettled 14, warming 3, text refused a merge 0 time(s)
```

Three facts, none of them new work — `Walk`, `Seen`, `SaidBy` and `Stability` are all already
computed and exported. A walk that does not move across a navigation is the first hypothesis; a
vocabulary holding words that belong to the next page is the third, visible on sight; and
`unsettled` separates "never differed" from "differed and never held", which is the pair this
report could not tell apart.

**No threshold was changed and no rule was altered.** The cause is not known, and a correction
aimed at a cause that is not known would be a guess with a test written to protect it.

## What this does not claim

**That the instrumentation is the fix.** It is what makes the next run able to answer the
question in one pass instead of none, which is the only honest step available from here.

**That the three candidates are the only ones.** They are the three the current output cannot
distinguish.

## Related

- [[Experiment-045-the-pointer-that-would-not-leave]] — the run before this, and the gate that
  let this one press at all.
- [[ADR-194-a-screen-is-also-what-it-says]] — the veto, its asymmetry, and the vocabulary rule.
- [[ADR-195-a-reading-that-arrives-after-the-look-it-describes]] — why readings are sparse.
- [[Experiment-041-the-control-Marco-found-for-itself]] — the last run that reached Free Play,
  and did not recognise it either.
