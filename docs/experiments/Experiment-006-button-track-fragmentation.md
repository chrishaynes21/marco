---
type: experiment
status: complete
date: 2026-08-07
backend:
  - screenparser
game: rocket-league
fixture: fixtures/vision/v2/rocketleague
result: detector-recall-limited-on-transitions
supersedes: []
source_paths:
  - internal/director/shadowreplay/replay.go
  - internal/director/shadowreplay/anchors.go
  - internal/director/shadowreplay/replay_test.go
  - cmd/director/shadowreplaycmd.go
  - internal/director/observe/shadowtrack.go
---

# Experiment 006 — the tracker was not the problem

## Question

A live shadow session produced roughly **1.08 button detections per track** — near-total
fragmentation — while icons in the same session, through the same matcher, at the same
cadence, produced 4.5. Why does a visually stable pause-menu button fail to become one
stable session-local track?

Six candidate causes, one of which had to be shown to dominate: `CADENCE_LIMITED`,
`DETECTOR_RECALL_LIMITED`, `GEOMETRY_MATCH_LIMITED`, `ASSIGNMENT_LIMITED`,
`REFERENCE_DRIFT_LIMITED`, `STABILITY_THRESHOLD_LIMITED`.

## Method

Rocket League was unavailable for a live capture, so the diagnosis ran against the frozen
corpus instead — which turned out to be the better instrument. A live trace cannot count the
inferences where a button was **on screen and not detected**, because nothing live knows where
the button was. The corpus carries a per-identity box on every annotated frame, so a miss is
countable.

`internal/director/shadowreplay` replays recorded detections through a mirror of the
production matcher. The mirror exists because the diagnosis needs per-detection assignment,
the losing candidates, and counterfactual policies — none of which `observe.ShadowTracker`
offers, and adding them would have meant editing production tracking during the milestone
meant to measure it. `TestTheMirrorReproducesProductionTracking` is what makes the mirror
evidence rather than a second opinion: six behavioural scenarios, identical tracks, seen
counts, eligibility and mean IoU.

What the corpus cannot measure is cadence — its frames are consecutive captures, not 2s
samples. The asymmetry runs the useful way: geometry that fails on dense frames fails harder
when sampled sparsely, so a failure found here is real. Only a clean result needs live
confirmation, which is exactly what happened.

## Result

On `pause-stable` — the menu genuinely open and unchanging for nine frames:

| element | eligible | detected | matched | fragmented | missed | tracks | recall | consec IoU | ref IoU |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| menu_resume | 9 | 9 | 9 | 0 | 0 | 1 | 1.00 | 1.00 | 1.00 |
| menu_change_mode | 9 | 9 | 9 | 0 | 0 | 1 | 1.00 | 1.00 | 1.00 |
| menu_settings | 9 | 9 | 9 | 0 | 0 | 1 | 1.00 | 1.00 | 0.97 |
| menu_exit | 9 | 9 | 9 | 0 | 0 | 1 | 1.00 | 1.00 | 1.00 |

**45 button detections, 5 tracks, 9.0 detections per track.** Zero fragmentation events across
all three pause sequences, at every threshold from 0.20 to 0.50, under every reference policy.
All four rows satisfy `Stable()`: persistent shape, presence ratio 1.00, mean IoU ≥ 0.97.

The only failures are detector misses on **transition frames**: `pause-open` index 3 (menu
fading in) returns zero regions; `pause-close` index 3 returns zero and indices 4–5 return two
of four rows. Recall on fully-rendered menu frames is 1.00; recall across transition frames
falls to 0.40.

Four causes are therefore **ruled out** for stable menu content:

- `GEOMETRY_MATCH_LIMITED` — consecutive IoU is 1.00; no consecutive pair falls under 0.30.
- `ASSIGNMENT_LIMITED` — the four rows are full-width and disjoint. The fifth detection per
  frame is a 0.011×0.019 sub-box inside the resume row, IoU ≈ 0.03, far too small to compete.
- `REFERENCE_DRIFT_LIMITED` — see below.
- `STABILITY_THRESHOLD_LIMITED` — the rows clear the current criteria outright.

## The refuted hypothesis, and why it is still worth keeping

Reading `observe.ShadowTrack.record` shows it updates every statistic **except** `Reference`,
so production matches every later detection against the first box it ever saw.
`TestFrozenReferenceGeometryFragmentsASteadilyMovingControl` demonstrates on production code
that a control drifting a third of its height per inference fragments even though its
consecutive overlap never once drops below the threshold.

That mechanism is real, and it is now pinned by a test. It is simply **not what is happening
here**: Rocket League's pause rows are pixel-stationary, `x` and `w` identical to three
decimals across all nine frames, so the frozen reference never has anything to drift from.
This is also why fixed HUD icons never exposed it.

Recording a refuted hypothesis rather than deleting it: the next detector with an animated
menu will hit this, and the measurement that would distinguish it — previous-detection IoU
versus reference IoU, both carried on every fragmentation event — now exists.

## What is still open

Offline and live disagree by a factor of eight (9.0 versus 1.08 detections per track), and the
Run B session data is gone — observation sessions live in memory only and the service
restarted. The gap cannot be closed from the corpus.

Gameplay content offline reproduces the *shape* of the live ratio: `freeplay-camera-motion`
yields 3 transient icon detections across 10 frames and 3 tracks — a ratio of 1.0, the
signature of unrelated one-off boxes rather than of a recurring structure. `freeplay-static`
yields nothing at all in 8 frames.

So the live session's fragmentation is not the tracker mishandling stable buttons; the
tracker handles those perfectly. The remaining question is what the live path fed it, and the
one measurement that answers it is per-inference normalised button geometry from a live
pause-menu capture — the trace machinery for which already exists behind
`$MARCO_SHADOW_TRACE`.

## Enforced by

- `TestTheMirrorReproducesProductionTracking` — the replay describes the shipping tracker.
- `TestFrozenReferenceGeometryFragmentsASteadilyMovingControl` — the drift mechanism is real.
- `TestAStationaryControlIsUnaffectedByTheReferencePolicy` — and does not fire on still ones.
- `TestALowerThresholdMergesAdjacentRows` — loosening the threshold has a measured cost.
- `TestAMissingInferenceIsNotADetectorMiss` — a skipped slot stays unknown.
- `TestARealDetectorMissIsCounted` — recall is measured against visibility.
- `TestFragmentationRecordsPreviousAndReferenceOverlap` — every fragmentation event carries
  the two numbers that classify it.

## Corpus

All 35 declared frames across the five sequences were present and analysed. The loader reports
a declared frame with no image on disk rather than silently analysing fewer frames than it
claims; nothing triggered it. Frames 029, 030, 036 and 037 legitimately appear in two
sequences each, which is how the overlapping pause windows were cut.
