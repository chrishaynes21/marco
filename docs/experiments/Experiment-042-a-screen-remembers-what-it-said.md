---
type: experiment
status: complete
date: 2026-10-14
backend:
  - structure
  - ocr
game: rocketleague
fixture: "rl-freeplay-1, 900 recorded frames"
result: the-two-menus-that-merged-are-now-two-screens-and-the-main-menu-is-still-one
supersedes: []
affects:
  - passive-observation
  - passive-visual-screen-integration
---

# Experiment-042 — a screen remembers what it said

## Question

[[Experiment-041-the-control-Marco-found-for-itself]] navigated Rocket League to Free Play with
three real clicks and could not establish that it had arrived. The diagnosis ruled out the
obvious answers: `screen_d` agreed with the live destination at **0.750** and ranked first. The
observer had already merged the Training page and the Free Play submenu into ONE live identity,
so the destination matcher was never offered a distinct screen to recognise.

**Premature identity fusion, not a destination threshold.** So: does what a screen SAID separate
two screens the arrangement merged — without fragmenting the screens that were right?

## Method

Fold the same recording twice, through the same production observer, and change one thing.

```
go run ./cmd/perceptionreplay -frames -semantic .tmp/game-capture-review/rl-freeplay-1
```

| | |
|---|---|
| pixel-only | what production did before this |
| with text | the same 900 looks, each carrying what OCR read on its own pixels |

OCR runs once over the frames and is cached beside the recording, because the reading costs
minutes and the fold costs seconds. 300 readings of 900 frames — one in three, which is about
the density a live loop could ever afford.

Nothing else moved. The same proposer, the same `VisualScreenMatch` of 0.55, the same
`VisualSettleLooks` of 3, no new model.

## What happened

```
  the same 900 looks, the same proposer, one thing changed
                                     pixel-only  with text
  screens settled                             9         13
  promotions                                 22         39
  looks that settled onto a screen          767        729
  looks read                                  0        300
  merges text refused                       n/a         18
  reuses text refused                       n/a         12
```

**Every pair the ARRANGEMENT would have merged, and what the words said about it:**

```
                                         arrangement   kept   novel
  main menu / map select                       0.686   0.00    1.00   refused
  main menu / garage                           0.651   0.21    0.73   refused
  Free Play submenu / custom training          0.617   0.00    1.00   refused
  Training page / Free Play submenu            0.559   0.20    0.80   refused   <- THE PAIR
  main menu / main menu, news panel rotated    0.946   0.43    0.37   allowed
```

**The Training page and the Free Play submenu are now two screens.** Their arrangements agree at
**0.559** — above the 0.55 at which a look joins a known screen, so the arrangement alone merges
them, which is exactly what it did during the live run. Their words agree at 0.111: the Training
page's stable vocabulary is `aerial create goalie party striker` and the submenu's is
`create free join online play`. One shared word out of five each.

Three other separations fell out of the same rule, and all three are real screens the pixel-only
fold had absorbed: the **map select**, the **garage** (`blueprints car customize inventory manage
trade`), and the **custom training panel**.

## And the screens that were right are still right

The test that matters more than the separation. Reuse is what makes a route possible: walking
back out of a submenu has to arrive at the screen it left, or every demonstration is a chain of
new places and no repetition is visible.

```
  visits per screen -- a screen entered, left and returned to must rejoin
  pixel-only    looks  visits   with text     looks  visits
  visual_1        449       7   visual_1        259      11   the main menu, ONE identity
  visual_6        181       2   visual_7        130       2   in-game
  visual_3         64       2   visual_3         46       2   the Training page
  visual_4         56       2   visual_4         53       3   the Free Play submenu

  pixel-only screens entered more than once       7 of 9
  with-text screens entered more than once        9 of 13
```

The main menu is **one** identity across 259 looks and eleven separate visits. 729 of the looks
that settled before still settle onto a screen, against 767.

## What it cost to get there, which is the part worth keeping

Four attempts, and each failure was a real defect rather than a tuning step.

**One symmetric similarity cannot do this job.** Two readings of the same screen at different
OCR yields agree at 0.300; the two screens that must separate agree at 0.231. No threshold sits
between those. What separates them is *which side* the missing words are on — a growing screen
still says everything it said, and a partial reading says nothing new — so the rule is two
asymmetric fractions, `kept` and `novel`, and both must fail before anything is refused.

**A refusal has to outlive the look that made it.** Text is sparse. An unread look agrees with
the standing screen by arrangement, settles onto it, and clears the pending run on its way past
— so at one reading in three the refusal fired on every reading and no run ever reached the three
looks it needed. The same screen was refused a thousand times and never separated. The refusal is
carried by the screen now, and an unread look does not overturn it.

**A single reading cannot be the look's side.** Every reading of a dark game menu carries its own
garbage, and garbage is by definition words the screen never said. Taking one reading as the
look's side split one main menu into **eight** identities over 449 looks. Both sides are
agreements now: the screen's across its looks, the look's across the last two readings.

**And a vocabulary must stop growing.** Keeping every word ever folded in meant a screen seen
sixty times accumulated every misreading that ever happened twice in a row — a hundred and more
words, of which perhaps forty were ever on screen. `kept` then falls steadily towards nothing for
a screen that never changed, and the main menu flapped between three identities 55 times. A word
belongs to a screen when it is there on at least half the looks.

Two more, smaller: nothing under three characters is a word — two readings of one menu disagreed
about forty tokens like `ae an bs ee fe om on oo pe ro si we ww` while agreeing about every label
— and the first look of a new screen must not teach its words to the old one.

## The limit, named

**The main menu's featured panel rotates.** Its arrangement is 0.946 against itself and its words
change, and at `TextKept = 0.50` that split the menu in two and then flapped between the halves.
It is allowed at 0.35 because the four real separations keep at most 0.21 and this pair keeps
0.43 — a gap the numbers sit inside rather than on the edge of.

That is the failure the owner predicted of this whole idea, and it is worth being precise about
what survived it: a rotating panel is tolerated because it is *content inside one screen*, and
nothing here can see that. Text over the whole window cannot tell "the labels changed" from "a
panel's content changed". Doing that needs position, and position is not in this change.

## Honest accounting of the 13

Of the four extra screens, three are real screens the pixel-only fold had merged (garage, map
select, custom training panel) and one is the Free Play submenu this experiment is about. The
remaining difference is the music overlay appearing as two identities (`after night` and
`after fire listen night silly sily`), which is the rotating-content limit again at a smaller
scale.

No screen that recurred in the pixel-only fold stopped recurring.

## What this does not claim

Nothing about the live loop. OCR costs hundreds of milliseconds against a 100ms cadence, and
`fastobserve.Look.Text` is **look-local by construction** — the type cannot carry a reading of
some other look, because an identity that depended on arrival timing would be the defect
[[ADR-184-two-speeds-of-looking]] built `admitSlow` to avoid. A live source fast enough to answer
within a look is the next question and is not this one.

Nothing about recognising a saved screen either. This is about not fusing two screens during
observation. The destination matcher now has words saved beside each arrangement — see
`workflowstore.Screen.Text` — and whether they help it is its own measurement.

## Related

- [[ADR-194-a-screen-is-also-what-it-says]] — the rule and every number in it.
- [[Experiment-041-the-control-Marco-found-for-itself]] — the live run and the diagnosis.
- [[Experiment-039-the-first-live-look-at-a-game]] — a stationary menu settling two screens, which
  is the same measure being too brittle in the other direction.
