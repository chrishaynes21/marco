---
type: experiment
status: complete
date: 2026-08-07
backend:
  - screenparser
game: rocket-league
fixture: fixtures/vision/v2/rocketleague
result: promote-to-shadow
supersedes: []
source_paths:
  - internal/director/visionbench/temporaltruth.go
  - internal/director/visionbench/truthmetrics.go
  - internal/director/visionbench/temporal_test.go
  - cmd/director/benchsplit_test.go
  - cmd/director/transitionaudit_test.go
---

# Experiment 005 — a transition is not a weak form of persistence

## The defect

Temporal truth had one rule: a track counted only if it appeared in at least half of its
sequence. Correct for a scene that sits still, meaningless for one that changes — and corpus v2
contains both.

Measured on the real corpus, the two transition sequences are near mirror images that landed on
opposite sides of the threshold:

```
pause-open    menu present in frames 3,4,5 of 6   3/6 >= 3   persistent    83%
pause-close   menu present in frames 0,1   of 6   2/6 <  3   none          0%
```

`pause-close` reported 0% not because anything was wrong with the detector but because it had
zero qualifying tracks, and `0/0` was rendered as zero. That number was then read for two
milestones as evidence of stale menu persistence. **The cause was `METRIC_DEFINITION`.**

## The new rule

| | Question | Scoring |
|---|---|---|
| `static` | does this element persist | unchanged: majority presence, then binary found-in-majority |
| `appearance` | did it start being found when it arrived, and not before | frame ratios |
| `disappearance` | did it stop being found when it left | frame ratios |

Per transitioning identity, over the frames of its own sequence:

```
recall     = frames it was there AND found        / frames it was there
precision  = frames it was there AND found        / (that + frames it was claimed while absent)
```

Neither refers to sequence length, which is what makes an appearance and its mirrored
disappearance score identically and a perfect detector score perfectly wherever the boundary
falls. An identity present in *every* frame of a transition sequence is not transitioning and
keeps the static rule — a HUD element does not change kind because a menu opened over it.

Mode and boundary are **declared** per sequence in `sequence.json`, never inferred. Inferring
from annotations would stop the corpus being able to disagree with itself; inferring from the
sequence name would put game knowledge in the scorer. `CheckAnnotations` then verifies the
declaration against the frames, so the declaration is load-bearing rather than decorative.

## Aggregation

Macro-average over sequences: track → sequence → corpus, unweighted at both levels. Chosen
because micro-averaging weights by frame and track count, and a nine-frame static sequence with
six identities would outvote two six-frame transitions — the transitions being the more
interesting evidence, not the less. It also keeps units safe: static tracks are counted per
track and transition tracks per frame, and both normalise to 0..1 before anything is summed.

Sequences offering no temporal opportunity are **excluded** from the mean, not scored zero.
Rendering "no opportunity" as 0% is the original defect in its purest form.

## Result — frozen held-out, nothing about the model changed

Structural, nameable and OCR are byte-identical to the pre-change run, which is the check that
the temporal repair did not leak:

```
                          BEFORE            AFTER
detections / TP / FP      74 / 21 / 9       74 / 21 / 9      unchanged
structural   P/R          70% / 52%         70% / 52%        unchanged
nameable     P/R          67% / 80%         67% / 80%        unchanged
OCR-region   P/R          67% / 80%         67% / 80%        unchanged
temporal     P/R          50% / 71%         46% / 46%        recomputed
ScoreV2                   60.4              57.0
```

Per sequence:

```
freeplay-camera-motion  static             T-PREC   0%  T-REC   0%
pause-close             disappearance@2    T-PREC  50%  T-REC  83%   10/12 on-time, 10 mistimed
pause-open              appearance@3       T-PREC  88%  T-REC  56%   10/18 on-time,  0 mistimed
```

`pause-close` recall moves 0% → 83%: the sequence is now measurable. `pause-open` recall moves
83% → 56% because per-track recall is now graded — partial tracking no longer earns full credit
from a majority vote.

## What the audit then found

The aggregate said `pause-close` earned ten mistimed frames. A percentage cannot distinguish a
one-frame release lag from sustained staleness, so `TestTransitionAuditPerFrame` printed the
per-frame distribution:

```
pause-close — disappearance at frame 2
  frame 0  PRESENT   7 detections   5/6 identities found
  frame 1  PRESENT   7 detections   5/6 identities found
  frame 2  absent    9 detections   0/6
  frame 3  absent    1 detection    0/6
  frame 4  absent    7 detections   5/6 identities found     <-- menu is BACK
  frame 5  absent    7 detections   5/6 identities found     <-- menu is BACK
```

Staleness decays. This does not. Inspecting the frames directly:
`rocketleague-pause-cycle-041.png` (index 5) shows the pause menu at **full opacity** —
`PAUSED`, `RESUME GAME`, `CHANGE MODE/MATCH`, `SETTINGS`, `EXIT TO MAIN MENU` — and its ground
truth declares zero interface regions. Index 3 shows the menu mid-transition and still legible,
also annotated as empty.

**`pause-close` is not a disappearance.** The capture is a pause *cycle*, and the menu reopens
within the six frames the sequence covers. Cause: `GROUND_TRUTH`. The detector's ten "mistimed"
claims are the detector being right and the corpus being wrong.

Left unrepaired in this milestone deliberately: the brief froze truth regions so that
structural and nameable could be proved unchanged, and repairing the annotation moves those
numbers too. It is the single next task.

## Does ScreenParser retire menu UI correctly?

On the evidence that is valid — `pause-open`, 0 mistimed across 18 opportunities — it never
claims a menu before it exists. The one sequence that would answer the closing half is
mis-annotated, so **the question is not yet answered**, and the old "stale persistence"
conclusion is withdrawn rather than replaced.

## Runtime

Median 735 ms, p95 797–820 ms, CPU, 1280 px, 22 held-out frames. Plugin working set: 7.8 MB
before the runtime loads, ~155 MB after session creation, rising to a 1.26 GB steady state
during inference.

```
200ms   no      500ms   no      1s   marginal      2s   comfortable
```

Memory, not latency, is now the binding constraint for running this beside the game.

## Enforced by

- `TestMirroredTransitionsScoreIdentically` — mirrored appearance/disappearance score alike
- `TestBoundaryPlacementDoesNotChangeAPerfectScore` — perfect at every boundary position
- `TestOldStaticRuleSplitTheRealCorpusShape` — reproduces the original defect
- `TestLateAppearanceCostsRecall`, `TestStaleDisappearanceCostsPrecision`,
  `TestPrematureDisappearanceCostsRecall`, `TestEarlyFalseAppearance`
- `TestStaticPersistenceSurvives`, `TestPersistentIdentityInsideATransitionSequence`
- `TestAggregateDoesNotWeightBySequenceLength`, `TestNoTemporalOpportunityIsExcludedNotZero`
- `TestSequenceModeChangesTheResult`, `TestTransitionBoundaryIsLoadBearing`,
  `TestCorpusMirrorSequencesScoreAlike`
- `TestHeldOutExcludesCalibrationSequences`, `TestFrameIdentityIsSequenceScoped`,
  `TestSplitDeclarationIsLoadBearing`

## Related

- [[Experiment-004-vision-corpus-v2]], [[Experiment-003-classical-cv-tuning]]
- [[Wiring-Tests]], [[Vision-Corpus-Workflow]], [[Roadmap]]

---

# Addendum — the corpus repair, and the promotion decision

The milestone above located a `GROUND_TRUTH` defect and deliberately left it. This addendum
repairs it, re-runs the frozen benchmark, and settles the question.

## What pause-close actually contains

Read from the pixels, not the sequence name and not the detector:

| idx | frame | menu | what is visible |
|---|---|---|---|
| 0 | …-036 | **yes**, full opacity | PAUSED, RESUME GAME (highlighted), CHANGE MODE/MATCH, SETTINGS, EXIT TO MAIN MENU, panel |
| 1 | …-037 | **yes**, full opacity | same six |
| 2 | …-038 | no | gameplay; white transition flash; free-play control legend top-right |
| 3 | …-039 | **yes**, attenuated | menu behind a portal effect; panel chrome faint, all four items legible |
| 4 | …-040 | **yes**, full opacity | same six, over a portal backdrop |
| 5 | …-041 | **yes**, full opacity | same six |

The menu is absent on exactly **one** frame. The sequence is a *recurrence*, not a
disappearance, and the previous declaration asserted "no interface" across four frames of which
three plainly contain a menu.

## The representation: intervals, not modes

`mode` + `boundary` could describe one change. Rather than add a fourth mode, the model was
replaced with the simpler general one — per identity, the spans it is present in, with the
complement asserted absent:

```json
{ "identity": "menu_resume", "present": [[0, 1], [3, 5]] }
```

One mechanism now covers every shape (`static`, `appearance`, `disappearance`, `transient`,
`recurring`); the scorer contains no mode, no boundary and no sequence name. `Shape()` derives a
label for reports only and can never change a number.

**Why absence must be declared at all**: this corpus is deliberately partial, so "no annotation
here" cannot mean "not here" — treating it that way would charge every detector for the corpus's
incompleteness. A declared interval is the difference between *nobody wrote it down* and *it is
not there*. Only declared identities are scored for timing; the rest keep the static rule.

`CheckAnnotations` now refuses a corpus in which the two disagree, **in both directions**. The
original defect fails it loudly.

## Corrected held-out result

Frozen: same ONNX artifact (sha256 `0bf9905e…`), conf 0.15, vocabulary, floors, preprocessing,
NMS, split, ScoreV2 weights. Only `pause-close` truth changed (+18 regions).

```
                    BEFORE REPAIR      AFTER REPAIR
truth regions       40                 58
detections          74                 74            model frozen, same input
TP / FP / FN        21 / 9 / 27        35 / 9 / 27
structural  P/R     70% / 52%          80% / 53%
nameable    P/R     67% / 80%          83% / 75%
OCR-region  P/R     67% / 80%          83% / 75%
temporal    P/R     46% / 46%          62% / 41%
mistimed claims     10                 0
ScoreV2             57.0               64.7
latency             735ms / 797ms      880ms / 991ms
```

```
freeplay-camera-motion  static        14 dets   1 TP  1 FP   9 FN    50%/10%   T   0%/  0%
pause-close             recurring     38 dets  24 TP  8 FP  10 FN    75%/67%   T 100%/ 67%   20/30 on-time, 0 mistimed
pause-open              appearance    22 dets  10 TP  0 FP   8 FN   100%/56%   T  88%/ 56%   10/18 on-time, 0 mistimed
```

`pause-close` presence timeline — truth `X X . X X X`, ScreenParser `5/6 5/6 none 0/6 5/6 5/6`.

## The stale-persistence claim is withdrawn

**Zero mistimed claims across the entire held-out split.** ScreenParser never once reported a
menu identity in a frame where truth says it is absent — it released the menu on frame 2 and
reacquired it on frame 4, correctly, both times.

The ten "mistimed" claims recorded before the repair were **entirely a ground-truth error**.
The finding is closed, not re-explained. What it actually misses is narrower and real:

- `menu_exit` — the bottom item, missed in every frame it appears (4 of the 10 misses)
- every identity on frame 3, the attenuated transition frame (the other 6)

## Memory

Plugin working set, sampled at 100ms:

```
 51 MB   process start, before ONNX Runtime
206 MB   ORT environment + session created
161 MB   after load settles
427 → 716 → 1247 MB   climbing across the first inferences
1247 MB  plateau, flat for the remainder
```

Where it comes from: model weights account for ~150 MB (a 102 MB ONNX file), and the input
(1×3×1280×1280 float32 = 19.7 MB) plus output (1×59×33600 = 7.9 MB) tensors for ~28 MB. The
remaining **~1.05 GB is ORT arena for intermediate activations**, allocated on first inference
and never returned — the default arena allocator does not release.

Not a leak: first-half max 1253.5 MB against second-half max 1257.6 MB across 22 frames. The
lever for a future performance milestone is ORT arena configuration, not the model.

## Cadence

`200ms` no · `500ms` no · `1s` feasible · `2s` comfortable. Unchanged. Marco does not need
frame-rate inference; 1–2 second semantic sampling is the design point.

## Verdict: PROMOTE TO SHADOW

ScoreV2 **64.7** against Classical CV's **5.0**. Structural precision 80%, nameable precision
83%, zero mistimed temporal claims, zero false positives under camera motion in the sequence
that tests scenery, and a Go runtime with no Python in the inference path.

## Enforced by (addendum)

- `TestRecurringPresenceScoresLikeAnyOtherShape` — A→B→A perfect/held/dropped/early
- `TestRecurrenceAndAppearanceAreScoredAlike` — where a gap falls is not a detector property
- `TestDeclaredAbsenceCannotContradictAnAnnotatedFrame` — the pause-close defect, refused
- `TestDeclaredPresenceCannotExceedTheAnnotations` — the other direction
- `TestARecurringIdentityMustDeclareItsIntervals`
- `TestSpansMustBeOrderedDisjointAndInRange`, `TestShapeNamesThePattern`
