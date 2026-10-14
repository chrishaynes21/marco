---
type: experiment
status: complete
date: 2026-10-14
backend:
  - structure
  - ocr
game: rocketleague
fixture: "rl-freeplay-1, 900 recorded frames, 300 cached readings"
result: the-two-menus-stay-apart-while-readings-arrive-within-eight-looks-and-merge-again-past-twelve
supersedes: []
affects:
  - passive-observation
  - passive-visual-screen-integration
---

# Experiment-043 — a reading that arrives late

## Question

[[Experiment-042-a-screen-remembers-what-it-said]] proved that what a screen says can keep two
visually identical menus apart, and left one thing undone and said so: the reading had to be
**look-local**, and OCR costs hundreds of milliseconds against an observation cadence of a
hundred. No live source can ever be look-local.

So the question is not how to make OCR fast. It is whether a reading that arrives several looks
after its own frame can be admitted **without corrupting screen identity** — and under what
latency it still works.

## Method

The same recording, the same 300 cached readings, the same production observer. One thing
changes: **when** each reading is delivered.

```
go run ./cmd/perceptionreplay -frames -semantic .tmp/game-capture-review/rl-freeplay-1
```

| | |
|---|---|
| pixel-only | no readings at all |
| with text, same look | through `Look.Text`, which no live source can satisfy |
| late by N looks | through `Look.Readings`, **the live path** |

A late reading is stamped with the window generation and observation epoch of the frame it came
from, and the observer judges it by the rule the delayed detections already go through
(`observe.Current`). The epoch is the load-bearing half: it moves on every screen change, so an
unchanged epoch is not "the same screen is standing" — which a screen left and returned to would
also satisfy — but **no screen change has happened since this frame**, which is a demonstrably
continuous interval.

## What happened

```
  the same 900 looks and the same 300 readings; only WHEN they arrive
                             screens settled offered admitted   stale refused
  pixel-only                       9     767       -       -       -       -
  with text, same look            13     729     900     300       0      18
  late by 1 look(s)               11     736     300     283      16      11
  late by 3 look(s)               12     723     299     260      38      16
  late by 5 look(s)               10     736     299     245      54      11
  late by 8 look(s)               10     754     298     223      75       8
  late by 12 look(s)              11     755     296     200      95       6
  late by 20 look(s)              11     746     294     157     137       5

  are the Training page and the Free Play submenu two screens
  pixel-only              cannot be asked: the fold kept no words
  with text, same look    APART: training [visual_3 visual_11], free play [visual_4]
  late by 1 look(s)       APART: training [visual_3], free play [visual_4]
  late by 3 look(s)       APART: training [visual_3], free play [visual_4]
  late by 5 look(s)       APART: training [visual_3], free play [visual_4]
  late by 8 look(s)       APART: training [visual_3], free play [visual_4]
  late by 12 look(s)      NOT FOUND: 2 read as the Training page and 0 as the submenu
  late by 20 look(s)      NOT FOUND: 2 read as the Training page and 0 as the submenu
```

**The pair stays apart while readings arrive within eight looks of their own frame.** OCR at
300–600ms on a 1920x1080 frame is three to six looks behind a 100ms cadence, so the working range
covers the real cost with margin.

## And the late path is better behaved than the synchronous one

Not the result that was expected. At five looks the live path settles **ten** screens where the
synchronous path settles thirteen, and 736 looks settle onto a screen against 729.

The reason is the admission rule. A reading whose epoch has moved is refused, and 54 of the 299
readings at five looks are exactly that — readings taken on one screen and answered on another.
Those are the readings that would have produced a spurious split, and the rule drops them for
the right reason. Lateness costs evidence and buys discipline.

## Why the merge is what makes it work

The thing that looks like a problem is the mechanism. When the Free Play submenu settled onto the
Training page's identity, **no screen change was recorded** — that is what a merge is — so the
epoch did not move. A reading of the submenu's own pixels arriving five looks later is therefore
still a reading of the screen Marco is standing on. It is admitted, it contradicts, and the merge
is refused.

A reading arriving after the screen genuinely changed is refused instead, which is the same rule
reaching the opposite answer about a different situation.

## How it fails, which is the right way

Past twelve looks the verdict is NOT FOUND rather than MERGED, and the difference matters. The
Free Play submenu is on screen for about 56 looks of 900; at twelve looks of latency its readings
arrive after the epoch has moved, so they are refused and **no screen ever accumulates its
vocabulary at all**. The fold is then the fold it was before any of this existed — nine screens
plus whatever the other readings separated, with the two menus merged.

Degrading to the previous behaviour is the correct failure. Nothing is corrupted, nothing is
mislabelled, and the counters say why: 95 refused as stale at twelve looks, 137 at twenty.

## What this does not claim

**The run has not been performed live.** The wiring exists behind `--reading` and is held by a
wiring test, and the integration is measured here against real pixels on one clock. Whether
Rocket League's OCR latency on this machine lands inside eight looks is the next thing to find
out, and the report prints `offered / admitted / refused as stale` so that one run answers it.

**The rotating-panel limit is unchanged.** [[ADR-194-a-screen-is-also-what-it-says]] names it: the
main menu's featured panel changes its words without changing its screen, text over the whole
window cannot tell that from a new screen, and telling them apart needs position. Lateness neither
helps nor hurts it.

## Related

- [[ADR-195-a-reading-that-arrives-after-the-look-it-describes]] — the rule and its admission.
- [[Experiment-042-a-screen-remembers-what-it-said]] — the offline proof this makes live.
- [[ADR-184-two-speeds-of-looking]] — the seam, and why late evidence may not reach identity.
- [[ADR-192-a-second-source-in-the-live-loop]] — the detector, whose shape this reuses exactly.
