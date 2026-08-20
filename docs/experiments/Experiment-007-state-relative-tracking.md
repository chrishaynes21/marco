---
type: experiment
status: complete
date: 2026-08-08
backend:
  - screenparser
game: rocket-league
fixture: live capture, 60s, privacy-safe trace
result: live-input-distribution
supersedes: []
source_paths:
  - internal/director/observe/screenstate.go
  - internal/director/observe/structuralgroup.go
  - internal/director/observe/shadowtrack.go
  - internal/director/observe/screenstate_test.go
  - internal/director/observesession/statewiring_test.go
  - cmd/director/shadowtracereport.go
---

# Experiment 007 — the denominator was the bug

## Question

[[Experiment-006-button-track-fragmentation]] closed with one thing unresolved: a live session
had produced roughly 26 button detections and ~24 button tracks, while the frozen corpus
produced one stable track per menu row. The corpus could not explain the gap, and the session
data was gone. Why did live and offline disagree by a factor of eight?

## Method

Rocket League became available. One 60-second observation at a 2s interval, with the user
playing and opening the pause menu several times, captured through the already-wired
`$MARCO_SHADOW_TRACE`. Nothing was retuned first: model, cadence, confidence 0.15, NMS 0.45,
match IoU 0.30, reference policy and stability thresholds all at their frozen values.

One repair was needed to collect anything. The service loaded `plugins/vision/onnxruntime.dll`
(ORT 1.26) while the plugin binding `yalue/onnxruntime_go v1.32.0` requires API 28, and startup
logged `The requested API version [28] is not available`. `$MARCO_ONNXRUNTIME` was repointed at
`tools/onnxruntime/onnxruntime-win-x64-1.28.0`. See "Packaging debt" below.

## Result — the tracker was fine, and so was everything else

Session integrity: completed 60s/60s, 31 slots → 20 valid, 11 skipped, 0 failed, 0 unproven,
one window generation, 0 regions outside `[0,1]`. 77 detections (56 button, 21 icon). Cadence
from recorded timestamps: median gap 3505 ms, max 4383 ms — inference latency ~850 ms against a
2s gate, so 11 slots were declined.

Four recurring full-width rows at x≈0.414, w≈0.172:

| row y | eligible | detected | misses | matched | fragmented | tracks | consec IoU | ref IoU |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| 0.437 | 8 | 8 | 0 | 8 | 0 | 1 | 0.992 (min 0.947) | 0.992 (min 0.947) |
| 0.480 | 8 | 8 | 0 | 8 | 0 | 1 | 0.999 (min 0.997) | 0.999 (min 0.997) |
| 0.520 | 8 | 8 | 0 | 8 | 0 | 1 | 0.992 (min 0.948) | 0.991 (min 0.951) |
| 0.562 | 8 | 8 | 0 | 8 | 0 | 1 | 1.000 | 1.000 |

Production and `shadowreplay` assigned **all 77 detections identically** — same 15 tracks, same
IDs, same per-track counts. All 11 button-track mints were legitimate (6 `FIRST_OF_ROLE` on the
first menu frame, 5 `NO_COMPATIBLE_TRACK` in genuinely new places). Buttons tracked at 5.09
detections per track against icons at 5.25, with buttons showing *better* geometry (0.992–1.000
versus 0.826–0.920).

**Classification: `LIVE_INPUT_DISTRIBUTION`.** Cadence is a controlled variable — same binary,
same gate, same latency as the earlier session — so it cannot explain a ratio that moved from
1.08 to 5.09. The old session's detections were singletons from gameplay and transition frames,
which the tracker correctly minted separately; icons scored 4.5 there because icons are HUD and
persist, while buttons exist only while a menu is open.

## What was actually wrong

Nothing mechanical. The four rows reported `seen 8 / eligible 16, presence 0.50, bursty` — a
reliability claim about controls that never once failed to appear. Eight of those sixteen
"opportunities" were inferences of gameplay, where a pause-menu button's absence is correct.
See [[ADR-012-presence-is-state-relative]] for the decision this produced.

## Replaying the same trace through state-relative tracking

The captured trace, fed back through `observe.ShadowTotals.Add` — the production analyzer, not a
second opinion:

| state | inferences | episodes | composition |
|---|---:|---:|---|
| state_1 | 9 | 3 | *(no structural evidence — gameplay)* |
| state_3 | 8 | 3 | button×7 icon×2 |
| state_4 | 2 | 1 | button×2 icon×1 |
| state_2 | 1 | 1 | icon×1 |

All 20 valid inferences were placed; none was lost to `state_unknown`. Transitions:
`state_3→state_1 ×2`, `state_1→state_3 ×1`, `state_3→state_4 ×1`, `state_4→state_3 ×1`,
`state_1→state_2 ×1`, `state_2→state_3 ×1`.

The four menu rows:

| track | global | global shape | state-local | state-local shape |
|---|---|---|---|---|
| shadow_2 | 8/16 (50%) | bursty | **8/8 (100%)** | **persistent** in state_3 |
| shadow_3 | 8/16 (50%) | bursty | **8/8 (100%)** | **persistent** in state_3 |
| shadow_4 | 8/16 (50%) | bursty | **8/8 (100%)** | **persistent** in state_3 |
| shadow_8 | 8/16 (50%) | bursty | **8/8 (100%)** | **persistent** in state_3 |

Two structural groups fell out of state_3. `group_1` is the menu column — 5 tracks, envelope
x 0.414 w 0.173 y 0.437 h 0.160, spacing 0.031, uniformity 0.66, **5 of 5 nameable**, recurring
over 3 episodes. The uniformity is depressed by `shadow_6`, a genuine 0.011×0.019 sub-detection
inside the top row; that is correct reporting, not a defect. `group_2` is a two-track corner
pair at y 0.867–0.894, correctly split out by arrangement rather than merged into the column.

## Two design corrections found by testing

Both were caught by tests that failed, and both are recorded because the failure was the
informative part.

**Similarity cannot separate a partial rendering from a different screen.** A half-drawn menu
and a dialog with a checkbox both score ~0.3 against the menu. Any single threshold either turns
every transition frame into a junk one-frame state or swallows genuinely new screens. They
differ in kind: a partial rendering introduces nothing new. Containment, asymmetric, decides it.

**Containment alone is order-dependent.** A sparse screen is contained in every rich one, so a
session opening on a menu reads gameplay as a permanent transition — which is exactly what
`TestTheProductionSessionPathSegmentsScreenStates` reported. Recurrence resolves it: an
ambiguous composition is held and promoted on its second sighting, at the cost of the first.

## Packaging debt

`plugins/vision/onnxruntime.dll` is ORT 1.26 while the binding requires API 28. It is **not
checked in** — `.gitignore` excludes `*.dll` — so it is an accidental stale local artifact, not
product runtime and not a packaging dependency. It matters because the binding's default is to
load `onnxruntime.dll` from the executable's directory, so a plugin run without an explicit
`$MARCO_ONNXRUNTIME` finds the wrong one. Not fixed here, deliberately: nothing checked in is
broken, and the fix belongs with whatever provisions the runtime.

## Enforced by

See [[ADR-012-presence-is-state-relative]] for the full list. The two that matter most:
`TestGameplayInferencesDoNotErodeAMenuButton` (the semantic regression) and
`TestTheProductionSessionPathSegmentsScreenStates` (the wiring, at the runner).
