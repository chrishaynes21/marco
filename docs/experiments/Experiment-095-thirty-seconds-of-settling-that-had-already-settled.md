---
type: experiment
status: conclusive
date: 2026-10-21
backend:
  - ocr
  - structure
fixture: "One live recording through Studio: `enter freeplay` on `rocketleague`, 60s ceiling, previews on, five presses. The recorder's own `learnstate` stream (`studio-learn-465302490.jsonl`, 25 events over 65s) and the artifact it saved (`sequences/enter_freeplay.json`). The owner's hands, their desktop, the game running."
result: "The first press captured 7.8s after the click, across two restarts of its outcome window; the stream's reading-by-reading ticks are 480ms apart, so OCR answered a little over twice a second beside a running game. The second press was clicked at 10:16:59.46 and 10:17:02.15 is the last thing the recorder ever said about it; its window can have taken at most until 10:17:05 to fill, so at least 26 of the following 29.5s were spent showing `Checking what changed — wait for the result to settle` about a reading already taken. The measurement was over; the recorder had no way to say so. `LocalOutcome.Verified` is false both while the readings are arriving and after they have arrived and said no, and nothing distinguished the two."
supersedes: []
affects:
  - learning
  - perception
source_paths:
  - internal/director/observe/outcome.go
  - internal/director/learnstate/learnstate.go
  - cmd/director/demonstratevisual.go
---

# Experiment-095 — thirty seconds of settling that had already settled

## The question, in the owner's words

> Okay I tried recording enter freeplay, the screenshots are there which is nice, but it got
> stuck on settling for like 30s and I just had to finish the recording rather than waiting.
> Settling shouldn't take 5 seconds right?

Two questions, and the recording answers both. The second one is yes, roughly: **four seconds is
the design.** The first one is not about settling at all.

## What the stream says

The recorder emits a snapshot on every change, so the timing of its own lines is the measurement.

| at | phase | what the recorder said |
| --- | --- | --- |
| 10:16:48.78 | `checking` | press 1 — nothing was read on one side of the press |
| 10:16:51.36 → 52.26 | `checking` | 1, 2, 3 readings of the 5 persistence needs |
| 10:16:52.78 | `checking` | **1** — the window restarted |
| 10:16:53.26 → 54.18 | `checking` | 2, 3, 4 |
| 10:16:54.66 | `checking` | **1** — restarted again |
| 10:16:55.07 → 56.06 | `checking` | 2, 3, 4 |
| 10:16:56.55 | `captured` | 7.8s after the click |
| 10:16:59.46 | `checking` | press 2 — nothing was read on one side yet |
| 10:17:02.15 | `checking` | only a few words appeared and stayed |
| *(nothing for 29.5s)* | | |
| 10:17:31.67 | `checking` | press 3 |

Two numbers come straight off it.

**The reading cadence is 480ms.** Each of those ticks is one OCR result folded into the watch, so
the gaps — 0.48, 0.49, 0.52, 0.48, 0.41, 0.51, 0.48, 0.41, 0.51, 0.48 — are what OCR costs on a
1920x1080 frame beside a running game. [[Experiment-043-a-reading-that-arrives-late]] put it at 300-600ms on an idle machine;
this is the same number with a game competing for it.

**So the floor is a little over three seconds.** `OutcomeSkipReadings` is two and
`OutcomeSettleLooks` is five, which is seven readings at 480ms. Press 1 took 7.8 because Rocket
League's menu redrew inside the window twice and the persistence term correctly started it again.

## And the 29.5 seconds were not that

Press 2 was clicked at 10:16:59.46 and the recorder’s last word about it is at 10:17:02.15. The
stream then says nothing for 29.5 seconds, which is the writer refusing to repeat itself: the
phase, the counts and the reason were all identical, so nothing the recorder reports changed in
that time.

The window itself can have taken a little longer than that line, because the reason *only a few
words appeared and stayed* does not mention how many readings have arrived — so it could have
gone on filling silently. Not much longer: two readings skipped, five restarts at the allowance’s
limit and five readings to fill is twelve at 480ms, which lands by **10:17:05 at the latest**. So
at least 26 of the 29.5 seconds were spent waiting for a reading that had already been taken.

The cause is one sentence. `LocalOutcome.Verified` is false in two situations with opposite
remedies:

| | what to do |
| --- | --- |
| the five readings are still arriving | wait — which is most of a second after every click, and is the ordinary case |
| the five readings arrived and few of what they said stood | nothing; the watch takes no more, so the verdict cannot improve |

The recorder read `Verified` alone, so it said `checking` for both and the card said *wait for the
result to settle*. See [[ADR-245-one-task-at-a-time]] for `LocalOutcome.Final` and
`learnstate.Unchecked`, which separate them.

## And the answer was a legitimate no

Rocket League's main menu is the screen [[Experiment-087-the-panel-changed-and-the-verifier-could-say-so]]
measured as the one the outcome rule does NOT work on: a rotating news panel, animated playlists
and live counters, so words change by themselves and few of them stand. 11 of its 17 presses
admitted false outcomes under every variant of the terms. Nothing here is a fault to repair — the
terms are refusing exactly what they were measured to refuse. What was wrong is that a recorder
holding that answer asked for patience.

## What the recording did produce

Four of five presses became steps, one with a verified outcome:

| step | from → to | cause | outcome |
| --- | --- | --- | --- |
| 0 | `screen_a` → `screen_b` | unknown | — |
| 1 | `screen_b` → `screen_c` | click, `control_1` | 7 words |
| 2 | `screen_c` → `screen_d` | click, `control_2` | — |
| 3 | `screen_d` → `screen_e` | click, `control_3` | — |
| 4 | `screen_e` → `screen_b` | click, `control_4` | — |
| 5 | `screen_b` → `screen_f` | automatic | — |

Two things in that table are open and are NOT what this record closes.

- **`control_2`, `control_3` and `control_4` are full-window rectangles** — 0,0 to 1,0.96. The
  press landed inside nothing smaller than the whole window, so the smallest containing region
  IS the window. That is [[Experiment-079-the-first-human-game-menu-demonstration]]'s finding
  unchanged, and it is about the structural proposer on this game's menus rather than about any
  of the above.
- **`screen_b` is reached twice and left twice, by two different causes.** Step 4 arrives at it
  by a click and step 5 leaves it `automatic`, which is a menu that moved on by itself. A routine
  with a loop in it is not something the cursor can perform.

Both are recorded here and neither is diagnosed.
