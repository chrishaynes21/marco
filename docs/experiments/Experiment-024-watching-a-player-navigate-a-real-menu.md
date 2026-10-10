---
type: experiment
status: complete
date: 2026-10-09
backend:
  - screenparser
game: trackmania
fixture: "%LOCALAPPDATA%\\marco-liveiface\\runs"
result: the-detector-discards-the-geometry-the-selection-reader-needs
supersedes: []
source_paths:
  - watch-interface.cmd
  - internal/director/perception/providers/vision/provider.go
  - internal/director/observe/screenstate.go
  - internal/platform/padsource
---

# Experiment 024 — watching a player navigate a real menu

## The question

The acceptance test, stated by the person who set it:

> You navigate a real menu with a controller, and Marco reports the same sequence of
> selections and changes without inventing extra transitions.

Four questions behind it:

1. Does the Director actually see the controller presses?
2. Does it identify the menu and its selected entry?
3. Does it tell genuine selection changes from detector flicker?
4. Does its interface belief go stale, or disappear, when the menu closes?

## Method

`watch-interface.cmd trackmania`, 240s, a six-phase protocol the script narrates on a clock and
records: hold still, move slowly, move fast, reverse, wrap, close the menu. Navigation by Xbox
controller. Nothing injected — the Windows backend of `padsource` binds `XInputGetState` and not
`XInputSetState`, so there is no setter to call.

Recorded: `slots.jsonl` (every slot, inputs device-stamped, session-relative times),
`ticks.jsonl` (the session's whole answer every ~2.1s), `phases.txt` (each phase's wall clock
and the tick it began after), `final.json`, `capture-cost.txt`.

`$MARCO_SHADOW_UNGATED=1`, `$MARCO_SHADOW_CADENCE=1s`, isolated `$MARCO_HOME`.

## The result: the acceptance test failed

Marco did not identify the menu and never reported a selected entry.

```
interface   status: unknown   selected: -1   reading: insufficient_evidence
because:    "no list has been seen handing a mark between its members
             (3 candidates watched over 37 frames)"
identified_after: -1
```

It abstained with a reason, which is the correct behaviour and the wrong outcome.

## What did work

**Controller observation.** 36 controller events over the run, every one carrying the device it
came from; 30,652 polls behind them at 125Hz. All three devices reported `watched: true,
present: true`. 94 of 94 inferences valid, **0 provenance failures, 0 uncomparable, one window
generation** throughout.

**Attribution.** The transition table carries the navigation that preceded each change:

```
state_4 → state_6   count 4   preceded {down: 3, up: 2}
   sequences: [down,down] [down,down,down,up,up,up,down,down] [up,up] [down×5]
state_4 → state_1   count 1   preceded {back: 1}
state_1 → state_4   count 1   preceded {point: 1}
```

**Candidate identity.** `joined: 83` at mean IoU **0.994**, `separated: 10`. The matcher is not
the weak link.

**Flicker rejection.** 21 proposed, **10 confirmed**, 11 rejected, 3 dropouts, **1** rejection
with a press before it. Against the previous run's 40 transitions in 55 inferences, the
transition rate fell about sevenfold.

## What the ten confirmed changes actually were

```
phase 2 (slow,    5 presses)   state_4 → state_6 → state_4
phase 3 (fast,   12 presses)   state_4 → state_6 → state_4
phase 4 (reverse, 5 presses)   state_4 → state_6 → state_4
phase 5 (wrap,    7 presses)   state_4 → state_6 → state_4
```

Four phases, four different input patterns, the same output. `state_4` is composition `{}` —
nothing. `state_6` is `button=1`. A one-button blip lasting ~7s, recurring every ~28s.

**The confirmed transitions track the detector, not the player.**

## Why: the detector's geometry never arrives

The whole run as the detector saw it:

| slots | seconds | admitted detections |
| --- | --- | --- |
| 1–10 | 0–43 | `button=16..21, icon=7, menu=1` — the pause menu |
| 11–82 | 43–194 | **0 on 56 looks, one button on 16** ← every press happened here |
| 83–94 | 195–245 | `button=3..4, menu=1, radio=1` — a dialog |

Two readings taken immediately afterwards, game in the foreground, nothing covering it:

```
considered 77   accepted 1   rejected 76
   nothing readable inside 72
text 70   button 3   image 2   radio 1
```

The detector finds about seventy boxes on this game's screens. Nearly all are classed `text`,
and a `text` box the detector could not read itself is discarded at
`vision/provider.go` — `RejectedUnreadable` — on this reasoning:

> *There is no evidence here to report: "an image is at these coordinates" is not something
> anything downstream can use.*

True when written. **Untrue since `selection.Measure` landed**: the selection reader takes a
rectangle and returns a shade, and never needs the words. The rows are found, classified, and
thrown away before shade matters.

Separately, and not the same defect: `27 of 94 samples refused evidence (2317 observations
quarantined)`, refused provider `ocr`, **with an empty reason string**. Words were being
produced and failing provenance. Text is lost twice over, by two different mechanisms.

## Timing, separated

| | |
| --- | --- |
| capture, 1920×1080 | **24ms** in-run, 32–39ms after |
| detect (ScreenParser) | **1.05–1.25s** |
| construct | 0–1ms |
| selection read | not separable live — it runs inside the provider per detection; bounded at 597µs |

Capture is **~2%** of a look. [[ADR-152-slow-discovery-fast-tracking]]'s missing number, and
more favourable than it assumed: at 24ms full-window the envelope is needed to honour
`MaxRegionArea`, not to afford tracking.

Session median inference was **1042ms**, p95 2027ms, max 3095ms — not the 1972ms published in
[[ADR-148-a-claim-of-sufficiency-is-not-a-claim-about-a-game]]. That figure was a full menu; ~1.0s
is a sparse screen.

## The cadence, which is the other bottleneck

**94 samples in 244s = 2.6s each**, against a configured 500ms interval, `samples_late: 360`.
The shadow inference blocks the observation cycle.

The fast burst was **12 presses in 3.35s** (gaps 144–744ms). All twelve landed inside
inferences **39 and 40**. Two looks for twelve presses.

So the two-look confirmation window is not what suppresses fast navigation; a one-look gate
would have had the same two readings. The gate's own error rate was one refusal-with-input in
94 looks. **Do not tune the window on this evidence** — tune the cadence.

## One real defect in the gate

A change was **confirmed into `state_unknown`** during the hold-still phase. A detector blackout
lasting two looks becomes a semantic transition. A one-frame dropout is carried as uncertainty;
a two-second one is not. The decision is recorded as ADR-153 when the repair lands.

## Two flaws in the protocol, not in Marco

**The hold-still phase was the only time the menu was legible.** Slots 1–10 are the one stretch
with a readable twenty-button menu, and the instruction then was to touch nothing. `group_1`
exists with 24 members, 16 buttons and `episodes: 0` because there was a list and nobody moved
the mark on it. The next run must navigate *that* menu.

**And it is the cleanest flicker reading in the run.** Between 27s and 80s the screen did not
change, and the detector reported `29 → 0 → 13 → 13 → 18 → 0 → 0 → 1 → 1 → 1 → 0…`. The gate
refused five of those wobbles and confirmed one.

## What this experiment decided

1. Preserve unreadable text geometry for shadow analysis, with provenance, and keep it
   distinguishable from named structure — ADR-154.
2. Separate slow discovery from fast tracking, off the cycle that ScreenParser blocks —
   [[ADR-152-slow-discovery-fast-tracking]].
3. A blackout is uncertainty or expiry, never a confirmed destination — ADR-153.

Those three ADRs are written with the repair, each naming the test that holds it. This record
is the evidence they rest on and deliberately predates them.

Not decided, and still requiring live validation: whether a tracked selection follows a real
player's navigation. Nothing in this run observed a selection at all.
