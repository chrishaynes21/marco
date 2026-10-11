---
type: experiment
status: complete
date: 2026-10-10
backend:
  - structure
game: trackmania
fixture: ".tmp/game-capture-review/tm-menu-steps-sanitized"
result: pixels-read-the-menu-the-detector-missed-and-cadence-is-the-binding-constraint
supersedes: []
source_paths:
  - internal/director/perception/structure/structure.go
  - cmd/perceptionreplay/frames.go
  - internal/director/perception/selection/group.go
---

# Experiment 026 — the same menu, seen from pixels

## The question

[[Experiment-025-the-menu-was-the-wrong-screen]] failed its acceptance test: four minutes at a
real Trackmania pause menu, 92 directional presses, and no selection ever read. The diagnosis
was that recall upstream had starved everything downstream.

So: **does a model-free structure proposer, on the same game, read the menu the detector
missed?** And no live session — the question is answered by reprocessing pixels already on disk.

## Method

**This is a replay, not a live run.** It measures how well Marco interprets a recording. It
cannot establish that Marco would interact successfully with the live application.

`director capture-game-session` had already recorded what was needed, before any of this work:
864 sanitised 1920×1080 frames of a real Trackmania pause menu being navigated, 31 controller
intents on the same measured clock, the window generation per frame, and a model-assisted
annotation of which entry was selected in each.

`perceptionreplay -frames` reprocesses those pixels through `structure.Propose` and the
production `observe.InterfaceWatch`, measuring shading **from the frame** rather than from
whatever a previous detector recorded. Nothing about the annotation reaches the perception under
test; the labels are applied strictly afterwards, to a belief formed without them.

**The labels are not independent ground truth.** The annotation file says so of itself:
*"MODEL-ASSISTED VISUAL ANNOTATION … they must NOT be presented as independently validated
ground truth."* Every number below carries that caveat, and the replay prints it.

## The result

Against Experiment-025's live run on the same game (run 0002, replayed at L1):

| | old detector, live | new pipeline, same game's pixels |
| --- | --- | --- |
| grouping produced a usable stack | 42 of 107 looks (39%) | **864 of 864 (100%)** |
| a proposal covered the selected row | — | **829 of 829 labelled frames (100%)** |
| a list was identified | never | **after 28 looks** |
| frames naming an entry | 0 | **148** |
| named row matched the label | — | **147 of 148** |

Two candidates qualified, so the final belief is `ambiguous` — the designed abstention, not a
failure to see. The competitor is a **real** second list: `y 0.155–0.413, x 0.070 w 0.567`, which
the recording's own sanitisation notes identify as the track-info panel. A tracker that picked
one would be guessing.

## The finding that matters for production: cadence is the binding constraint

The same corpus, reprocessed at different intervals:

| interval between looks | identified after | frames naming an entry | matched |
| --- | --- | --- | --- |
| 66ms (the recording's own rate) | 28 | 148 | 147 |
| ~660ms | 51 | 27 | 26 |
| ~1.3s | 29 | 11 | 10 |
| **~2.3s — the measured live cadence** | **never** | **0** | — |

At the live rate a 57-second session yields 25 looks, and the rule needs 10 **judged** frames
after baselines exist. The perception is not the limit at that point; the number of looks is.

And the cost: `structure.Propose` is **37ms** per 1920×1080 frame. The ONNX inference that sets
the live cadence has a **976ms median** (run 0002). The stage that made the cadence 2.3s — and
therefore made selection tracking impossible — is about 26× more expensive than the model-free
cue that outperforms it on this corpus.

That reframes [[ADR-152-slow-discovery-fast-tracking]]: the slow-discovery half was slow because
of the model, and discovery from pixels is nearly as cheap as the fast path was supposed to be.

## An alternative tried and rejected

A **human-scale floor** on proposals, discarding anything shorter than 1% of frame height. It
was built for a real defect: a 69×29-pixel patch of rendered scene, in which four "rows" of
about seven pixels flickered one at a time, satisfied every requirement for a navigated list —
54 exclusive frames, 3 transfers, 3 holders — and qualified beside the menu.

The floor removed it and improved proposals per frame by 18%, from 204.7 to 167.6. It also took
frames naming an entry from **148 to 12** and produced a **third** qualifying list, because
removing the small proposals let a much larger stack form across most of the screen.

A component metric improved and the pipeline regressed. It is off by default, with the
measurement in the code.

## What still fails

- **Precision: 204.7 proposals per frame** on a rendered game scene, against 71–100% of
  proposals landing on known structure in drawn fixtures with flat grounds. Every false
  rectangle is a possible list member on every frame.
- **Two real lists cannot be told apart by vision alone.** The next step is evidence Marco
  already records and does not use: the list whose mark moves when a direction is pressed is the
  one being navigated. The input log sits in the same recording on the same clock.
- **Nothing is wired into production.** The engine has no pixels at sample time. Every number
  here is a replay number.
- **The drawn-fixture gap is unchanged**: a ground-changing highlight on a translucent panel
  loses the marked row and its neighbours (2 of 5, against 5 of 5 unmarked).

## Next

1. A frame path into the engine, so a session can use this at all. The cadence finding says to
   do it as a replacement for the inference on the discovery path rather than beside it.
2. Disambiguate competing lists by correlating the mark's movement with observed input.
3. Then a live run, which is the only thing that can establish any of this in production.
