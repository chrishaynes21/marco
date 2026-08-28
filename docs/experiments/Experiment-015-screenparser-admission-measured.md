---
type: experiment
status: complete
date: 2026-08-28
backend:
  - screenparser
  - classical-cv
  - grounding-dino
game: rocketleague
fixture: fixtures/vision/v2/rocketleague
result: screenparser-remains-shadow-only
supersedes: []
source_paths:
  - cmd/director/benchscreenparser.go
  - cmd/director/benchisolation_test.go
  - internal/director/perception/shadow/provider.go
---

# Experiment 015 — what ScreenParser is worth, measured

## Question

[[ADR-100-marco-sees-through-evidence]] recorded that ScreenParser is `ShadowOnly` and that its
evidence never reaches fusion, and left the obvious question open: **has it earned admission?**

[[Experiment-010-vision-structure-as-a-semantic-path]] proved the semantic PATH works — detector
structure plus scoped OCR becomes interface terms without accessibility — but could not run the
model: *"No live run, and it is not currently possible on this machine."*

That is no longer true.

## The false start, recorded because it changed the conclusion once

An audit pass concluded the measurement still could not be taken, because `$MARCO_ONNXRUNTIME`
was unset and a search found no runtime. **The search was wrong**: it looked in
`$LOCALAPPDATA`, `$ProgramFiles`, `C:\onnxruntime` and three levels of `D:\`, and the runtime is
in the repository at `tools/onnxruntime/onnxruntime-win-x64-1.28.0/lib/onnxruntime.dll`, with an
older 1.26 copy beside the plugin at `plugins/vision/onnxruntime.dll`.

A decision was committed on that reasoning before a background search finished and contradicted
it. It is recorded here rather than quietly corrected, because "the measurement is impossible" is
exactly the kind of conclusion that should be hard to reach and easy to check.

Three things were needed and all three were present:

| | |
|---|---|
| model | `tools/vision-export/weights/screenparser-1280.onnx`, 97.4 MB |
| runtime | `tools/onnxruntime/onnxruntime-win-x64-1.28.0/lib/onnxruntime.dll` |
| plugin | `go build -tags onnxvision` in `plugins/vision` |

The 1.26 copy beside the plugin fails honestly: *"The requested API version [28] is not available,
only API versions [1, 26] are supported in this build."* Two runtimes in one tree, one of them
too old, and the error names the mismatch precisely.

## Method

`director benchmark-vision --fixture fixtures/vision/v2/rocketleague`, the frozen v2 corpus:
39 frames, 5 sequences, 120 annotated truth regions. ScreenParser at its frozen settings —
1280px, conf 0.15, IoU 0.45 — through the ordinary vision plugin.

## Result — the detector

| | ScreenParser | classical CV | Grounding DINO |
|---|---|---|---|
| structural P / R | **90% / 63%** | 0% / 0% | 77% / 17% |
| nameable P / R | **81% / 88%** | 0% / 0% | — |
| detections | 145 (80 TP, 9 FP, 44 FN) | 3915 (0 TP, 1858 FP) | 43 (20 TP, 6 FP) |
| latency median / p95 | **895–932ms / 992ms–1.04s** | 1ms / 2ms | — |
| ScoreV2 | **66.8** | 5.0 | — |

By sequence:

| sequence | shape | dets | P | R |
|---|---|---|---|---|
| pause-stable | static menu | 63 | 100% | 83% |
| pause-open | appearance | 22 | 100% | 56% |
| pause-close | recurring | 38 | 75% | 67% |
| freeplay-camera-motion | in-game | 14 | 50% | 10% |
| freeplay-static | in-game | 8 | — | 0% |

**It reads menus and panels well and gameplay HUD barely at all.** That is what a UI-trained
detector should do, and it is the first time this project has had the number.

Nameable precision and recall were **0% for four milestones** — the number
[[director-vision-ui-detector-decision]] existed to move. 81% / 88% moved it.

## Result — the benchmark was comparing one model against itself

`current` and `screenparser` came back **byte-identical**: 145 detections, 80 TP, 9 FP,
ScoreV2 66.8, differing only in the latency jitter of two runs.

`newScreenParserBackend` configured the plugin with process-wide `os.Setenv`, and a bridge host
launches its child on **first use** — during the run, after every backend has been constructed.
So the baseline's plugin spawned inheriting ScreenParser's model.

This is the same defect `newShadowVision` records having been caught in the first live start:

> It was process-wide `os.Setenv` first, and that was a real defect caught in the first live
> start … The experiment had silently become the authoritative detector.

Fixed there. Not here. With per-child environment, `current` correctly reports UNAVAILABLE — no
model is configured for the shipping detector — and only the challenger produces numbers.

## Conclusion

**`SCREENPARSER_REMAINS_SHADOW_ONLY`**, on what the measurement says rather than on what could
not be measured:

- **895–932ms median** against ambient Observe's 1-second active cadence is the entire budget on
  one sensor. This is why the provider already runs on its own cadence with skip-never-queue.
- **The corpus is a game.** The loop that matters runs against desktop applications. Settings is
  menu-shaped and the menu numbers are good, so the result is encouraging — and encouraging is
  not measured.
- **63% structural recall**: 44 real regions missed and 56 detections matching nothing.
- **No degraded reading occurred**, and manufacturing one was forbidden.

## What is still not proven

- **Desktop workload.** The single most valuable next measurement, and the reason the follow-on
  recommendation is a desktop corpus rather than an admission attempt.
- **Degraded-UIA repair.** Unmeasured, as it has been since 35C.
- **The ordinary pipeline's cost.** UIA, capture, OCR and fusion timings ride on every sample as
  `Phases` and were not sampled. Two roadmaps have now skipped that gate.
- **Per-class calibration.** The corpus has 120 regions across five sequences, which is too few
  to say anything per-class about a 55-class ontology.

## Enforced by

`cmd/director` — `TestTheBenchmarkBaselineIsNotTheChallenger`, which fails if detector
configuration escapes into the process again. The measurement itself is not a test: it needs a
model, a runtime and a corpus, and it is reproduced by the command in Method.

## Related

[[ADR-101-visual-presence-is-not-legal-actionability]] ·
[[ADR-100-marco-sees-through-evidence]] ·
[[Experiment-010-vision-structure-as-a-semantic-path]] ·
[[Experiment-004-vision-corpus-v2]] ·
[[director-vision-ui-detector-decision]] · [[Vision]]
