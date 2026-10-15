---
type: experiment
status: complete
date: 2026-10-14
backend:
  - structure
  - ocr
game: rocketleague
fixture: "the rl-freeplay-1 recording, folded 222 times through the production observer, and the whole 900-look recording through cmd/perceptionreplay -frames -semantic"
result: one-reading-may-refuse-a-merge-and-it-separates-6-of-222-more-folds-with-no-fragmentation-while-the-live-four-look-dwell-is-unchanged-because-a-page-passed-through-is-never-established
supersedes: []
affects:
  - passive-observation
---

# Experiment-047 — what one reading is allowed to do

## The question

[[Experiment-046-the-destination-it-reached-and-did-not-recognise]] found the pair: Rocket
League's Play page and its Training page agree at mean 0.592 by arrangement against the 0.55 that
merges them, where the main menu and the Play page never reach 0.55 at all. Their words do not
agree — `match offline play private` against `aerial goalie striker` — and
[[ADR-194-a-screen-is-also-what-it-says]] already permits words to refuse a merge.

The owner's decision: **one fresh reading may provisionally refuse a merge, and may not establish
a permanent identity.** Three safeguards named — the existing screen must have established
semantic evidence, the new reading must contain enough meaningful text, and the disagreement must
be substantial — with the instruction to measure it against the recorded folds, to check the
rotating main-menu panel specifically, and to **stop rather than add more heuristics** if it does
not improve without fragmenting revisited screens.

The rule is [[ADR-199-one-reading-may-refuse-a-merge-and-may-not-mint-a-screen]]. This is what it
measured.

## The folds, and why these folds

The 22 folds of Experiment-046 varied one thing: which look became the Play page's signature. That
is one of four things that decide whether the navigation is visible, and the other three are what
the live run differed from the recording by. So the grid:

```
  p       which recorded frame of the Play page the fold starts on   78, 81 ... 108
  dwell   how many looks the Play page is held for                   4, 6, 10, 18, 34
  every   a reading on every Nth frame                               3, 6
  delay   how many looks after its own frame a reading arrives       3, 5, 8
```

222 folds. Each one is `main menu 0..27` + `play p..p+dwell` + `training 114..141`, spliced from
the recording's own frames and fed through `fastobserve` with readings delivered on the **live
late path** — each stamped with the window generation and observation epoch of the look it
describes, exactly as a live OCR source stamps them. The pages are the ones verified by content in
Experiment-046, not by frame index.

The axis that matters is `dwell`. The live run held the Play page for about four looks; the
recording holds it for thirty-four. **That difference is the experiment.**

## Before

```
  every  delay  dwell  separated      menu split   play said training's words
  3      3/5/8  4      6 of 11        0 of 11      5 of 11
  3      3/5/8  6      5 of 10        0 of 10      5 of 10
  3      3/5/8  10     8 of 9         0 of 9       1 of 9
  3      3/5/8  18     6 of 6         0 of 6       0 of 6
  6      3/5/8  4      6 of 11        0 of 11      5 of 11
  6      3/5/8  6      5 of 10        0 of 10      5 of 10
  6      3/5/8  10     5 of 9         0 of 9       4 of 9

  TOTAL separated 147 of 222; main menu split in 0; play said training's words in 75
```

Two things to read off it. The failure is **reproduced exactly**: in the merged folds the Play
page's identity is read as `aerial goalie striker`, which is the string Experiment-046's live run
printed for `visual_2` over 103 looks. And `text refused a merge` is **0 in all 222 folds** — not
once overruled, never asked.

The delay column makes no difference anywhere, which is worth saying because it was the first
hypothesis about the live run.

## After

```
  every  delay  dwell  separated      menu split   play said training's words
  6      3/5/8  10     7 of 9   (+2)  0 of 9       2 of 9   (-2)
  everything else unchanged

  TOTAL separated 153 of 222 (+6); main menu split in 0; training's words in 69 (-6)
  27 folds now refuse a merge; all 27 refusals rested on one reading; every one was correct
```

Six folds changed, all in the same cell, and nothing regressed. The other 21 refusals were raised
on folds where the arrangement went on to separate the pages anyway, two looks later — the words
got there first and the outcome was already right.

**At a four-look dwell the outcome is unchanged**, and that is the finding.

## Why the live condition does not move

Traced through the fold, with the readings the recording actually carries:

```
  look 28..31   the Play page           settles as visual_2 at look 30
  look 31+d     a reading of the Play page arrives
                -> REFUSED AS STALE. The epoch moved when visual_2 was promoted, and
                   ADR-195 is right to refuse it: it was read before the change.
  look 34+      a reading of the Play page taken after the promotion arrives
                -> admitted, and NOT FOLDED. A screen's vocabulary is built from
                   AGREEMENTS between consecutive readings of one screen, and this is
                   the first reading of this screen. There is nothing to pair it with.
  look 37+      the first reading of the TRAINING page arrives
                -> paired with the Play reading above, because both were taken while
                   visual_2 was standing. Their agreement is `create party` -- the party
                   panel, which is on both pages. Two words, fewer than the three it
                   takes to be a claim, and it is folded into visual_2.
  look 40+      the second Training reading
                -> paired with the first: `aerial create goalie party striker tutorial`,
                   folded. visual_2 now has two readings and says the Training page's
                   words, so a Training look agrees with it at kept 1.00.
```

**A screen that is passed through can never have established what it says.** Its first reading is
discarded for having nothing to agree with, the reading before that is discarded for being older
than the change, and the next pair available is two readings of the page that replaced it. By the
time `stable()` will answer at all, what it answers is the next page's vocabulary.

So the first of the three safeguards — the existing screen must have established semantic
evidence — is the one that decides this case, and no relaxation of the look's side can reach it.

## The rotating panel, which is what this had to not break

The harness never splits the main menu (0 of 222), but it only ever shows it one panel. The
rotation is in the full recording, so the check is the production replay over all 900 looks, at
every latency, before and after:

```
                       screens        refusals       training/freeplay apart
  late by 1 look(s)    11 -> 13       11 -> 40       yes -> yes
  late by 3 look(s)    12 -> 10       16 -> 25       yes -> yes
  late by 5 look(s)    10 -> 11       11 -> 19       yes -> yes
  late by 8 look(s)    10 -> 10        8 -> 10       yes -> yes
  late by 12 look(s)   11 -> 10        6 ->  6       NO  -> yes
  late by 20 look(s)   11 -> 10        5 ->  6       no  -> no
```

- **The main menu keeps ONE identity across its 449 looks**, before and after, with its featured
  panel rotating through two different sets of words inside it. Its vocabulary is the same
  seventeen words either way.
- **Revisits survive.** Nine of eleven screens are entered more than once after, against nine of
  ten before, and seven of nine in the pixel-only fold.
- The Training page and the Free Play submenu — the pair ADR-194 exists for — are told apart at
  one more latency than before.
- The quit-confirmation dialog is two identities before and after. It is separated at 0.454 by
  **arrangement**, not by text.

## And the variant the owner did not authorise, measured

The blocker is the screen's side, which the decision deliberately left alone. So the adjacent
change was written, measured, and reverted, the same way
[[Experiment-046-the-destination-it-reached-and-did-not-recognise]]'s learning guard was: a
screen's FIRST reading is folded into it even though nothing pairs with it, and one reading is
enough for `stable()` to answer.

```
  every  delay  dwell  separated      play said training's words
  3      3/5/8  4      11 of 11       0 of 11
  3      3/5/8  6      10 of 10       0 of 10
  6      3/5/8  4      9 of 11        3 of 11
  6      3/5/8  6      8 of 10        3 of 10

  TOTAL separated 210 of 222; play said training's words in 16
```

**It separates the pair at the live dwell**, and the whole-recording fold says what it costs:

```
  the main menu, 449 pixel-only looks, distributed by the richer fold
    the rule     visual_1 x272   visual_10 x149 (the garage)   + two dialog fragments
    the variant  visual_1 x252   visual_10 x120 (the garage)   visual_11 x77
                 visual_11, over 93 looks and FOUR visits, says:
                   2v2 and challenges chat club competitive create double doubles earn
                   get goals match matches online party play profile quick settings
                   silly take the use view
```

`visual_11` is the main menu with its other panel. **The variant mints a second identity for one
screen that never changed, and returns to it four times** — which is ADR-194's measured failure,
in the same place, for the same reason: *"every look at that menu matched whichever of the two
vocabularies happened to be closer"*. It also loses the Training/Free Play separation at twelve
and twenty looks of delay, which the authorised rule gains at twelve.

That is the trade, and it is a product question rather than an engineering one: **separating two
pages a workflow passes through, against one durable identity per screen.** Both numbers are
above. The decision is not taken here.

## Also observed, and not fixed

An agreement between two readings that share NO words is `read` and `silent` — a reading with no
words, rather than no reading. `noteText` folds it, so it increments a screen's reading count
without adding anything, and `TextCommon` needs a word in half the readings. A screen alternating
between readings that agree and readings that do not therefore **dilutes its own vocabulary to
nothing**: measured in `TestOneOddReadingCostsAScreenNothingItKeeps`, the screen's five words drop
out after three such cycles and it stops being able to refuse anything at all.

It is recorded rather than fixed because it is the same shape as the guard reverted in
Experiment-046 — a change to what a screen learns, where the measurement went the other way — and
nothing here depends on it.

## What this does not claim

**That the destination problem is solved.** Six folds of 222, and the live four-look dwell is
unchanged. What the run can now do is refuse a merge on the first reading of the page it arrived
on, where before it needed two, and the report says when a decision rested on one reading.

**That 222 folds are the population.** They are one recorded navigation, spliced four ways. The
live run is the population and there has been one of it.

## Related

- [[ADR-199-one-reading-may-refuse-a-merge-and-may-not-mint-a-screen]] — the rule, its four
  conditions, and what a refusal still may not do.
- [[ADR-194-a-screen-is-also-what-it-says]] — the two agreements this relaxes one of.
- [[ADR-195-a-reading-that-arrives-after-the-look-it-describes]] — the stale-epoch rule, which is
  half of why a page passed through is never established.
- [[Experiment-042-a-screen-remembers-what-it-said]] — where `TextKept`, `TextNovel` and
  `TextCommon` came from, and the menu that flapped.
