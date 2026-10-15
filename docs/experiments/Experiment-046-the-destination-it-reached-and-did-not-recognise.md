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
result: the-play-page-and-the-training-page-overlap-at-0-592-against-a-0-55-merge-threshold-so-whether-the-navigation-is-seen-depends-on-which-look-became-the-signature
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

## And then the captures, which found it

A passive recording of the live Training page -- 400 frames, 40 seconds, no input injected and
none observed -- settles into **one** screen: 67 to 154 regions a look, median 76, **2 of 99
adjacent pairs** below the merge threshold, 96 settled and 1 unsettled. The live page is not
noisy and is not the problem.

The problem is the pair. Every page measured against every other, through the production
measure:

```
  median pairwise arrangement similarity (merge at 0.55)
    rec-playpage    rec-training     mean 0.592  (0.350..0.786)   <- WOULD MERGE
    rec-playpage    live-training    mean 0.590  (0.342..0.800)   <- WOULD MERGE
    rec-training    live-training    mean 0.813  (0.667..0.958)
    rec-mainmenu    rec-playpage     mean 0.345  (0.199..0.527)
```

**The Play page and the Training page are, on average, similar enough to merge.** The main menu
and the Play page are not close (0.345, never reaching 0.55 on any pair). These two are: a
single look of one against a single look of the other lands anywhere from 0.350 to 0.786, with
the threshold at 0.55 straddled by the distribution.

A screen's signature is ONE arrangement -- whichever look settled it. So whether the navigation
is seen depends on which look that was. Folding the same recorded navigation twenty-two times,
varying only where the Play page starts:

```
  play from 87    training got its own id    visual_2 says: match offline play private
  play from 96    training got its own id    visual_2 says: create match offline party play private
  play from 99    training got its own id    visual_2 says: aerial create goalie party striker
  play from 99/8  MERGED                     visual_2 says: create party play
  play from 102   the Play page merged into the MAIN MENU
```

One fold in twenty-two merges the Training page into the Play page outright. Another ends with
the Play page's identity holding the Training page's words. Three more never give the Play page
an identity at all -- it merges into the main menu. **Nothing the session controls decides
which.** The live run landed on the merging side; the recording, replayed at every cadence, had
landed on the other.

## A correction that was written, measured, and reverted

The fold teaches the standing screen the words of looks that arrive during the three it takes a
new arrangement to settle. A guard was written for it: a reading that materially disagrees with
what a screen has already been read to say is not folded into it, compared against the raw
vocabulary rather than the stable one, on the argument that refusing to LEARN is weaker than
refusing a merge and so needs less corroboration.

**Measured over the same twenty-two folds, it changed one and made it worse** -- the fold at
`play from 99` went from a separated pair with a corrupted vocabulary to an outright merge. It
was reverted.

Recorded because the first measurement of it said the opposite. An earlier harness sampled the
Play page on frames that carry no OCR at all, so half its folds had an UNREAD screen acquiring
the next page's words -- which looks exactly like contamination in the output and is a different
thing. Sampling the pages where they were actually read, contamination appears once in
twenty-two. The lesson is the same one the Trackmania capture taught three hours earlier: check
what the fixture contains before measuring it.

## What this does not claim

**That the instrumentation is the fix.** It is what makes the next run able to answer the
question in one pass instead of none, which is the only honest step available from here.

**That the three candidates are the only ones.** They are the three the current output cannot
distinguish.

**That the cause is now repaired.** It is located, not fixed. Two pages sit inside the
arrangement measure's own resolution, and every obvious response to that is one this project has
refused on purpose: lowering the merge threshold splits screens that are the same, raising it
merges more, and a second measure chosen because it separates THIS pair is a Rocket League
heuristic wearing a general name. The one evidence that does separate them cleanly -- `match
offline play private` against `aerial goalie striker` -- is already in the system and is already
allowed to refuse a merge; what it is not allowed to do is refuse one on a screen nobody has
read twice, and the Play page is a screen a workflow passes through in four looks.

Whether that bound should move is a question about what licenses a screen identity, which is
not a decision to take while holding a measurement.

## Related

- [[Experiment-045-the-pointer-that-would-not-leave]] — the run before this, and the gate that
  let this one press at all.
- [[ADR-194-a-screen-is-also-what-it-says]] — the veto, its asymmetry, and the vocabulary rule.
- [[ADR-195-a-reading-that-arrives-after-the-look-it-describes]] — why readings are sparse.
- [[Experiment-041-the-control-Marco-found-for-itself]] — the last run that reached Free Play,
  and did not recognise it either.
