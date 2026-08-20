---
type: experiment
status: complete
date: 2026-08-06
backend:
  - classical-cv
game: rocket-league
fixture: fixtures/vision/rocketleague
result: ceiling-reached
supersedes: []
source_paths:
  - internal/director/visionbench/classical.go
  - internal/director/visionbench/classical_features.go
  - internal/director/visionbench/classical_classify.go
---

# Experiment 003 — how far geometry alone can go

The question: before buying Marco better eyes, how much interface structure can a
deterministic, zero-dependency rectangle detector extract reliably?

**Answer: it reaches a hard ceiling, and the ceiling is not a missing heuristic.** The
detector was restructured, given deterministic features, ablated rule by rule, measured
against declared ground truth, and shadowed over a real 1920x1080 screen. What that produced
is a precise account of why geometry cannot finish this job — and a finding that the
reference corpus cannot be used to tune it.

## Baseline

```
detections 92   accepted 92   structural 100%   nameable 40%
false structure 67%   persistence 48%   median latency ~0   score 55.9
```

Stored at `docs/experiments/data/classical-baseline.json`, not overwritten.

## Failure taxonomy

The manifest declares which frames contain no interface, which makes false positives
countable without inference. Of 92 detections, **65 (71%) land on frames marked
`contains_no_ui_structure`** — corroborating the 67% temporal proxy from two directions.

| class | count | note |
|---|---:|---|
| texture rectangle | ~65 | near-minimum-size uniform colour runs in arena scenery |
| nested duplicate | **0** | the grid scan consumes cells; it cannot emit nested boxes |
| scene-sized region | **0** on corpus | but 1 per frame on a real screen — see below |
| fragmented control | few | left-edge sidebar fragments in the pause frames |

Two of the milestone's suggested categories measured **zero**. Nested suppression was
therefore never built: it would have been decorative logic that never fires.

## What was built, measured, and removed

Scored by declared ground truth — a detection on a no-interface frame is a false positive:

| configuration | false positives | true structure kept |
|---|---:|---:|
| baseline | 65 | 27 |
| **size filter only** | **16** | **27** |
| size + alignment | 12 | 23 |
| size + border evidence | 2 | 3 |
| all three | 6 | 12 |

- **Border continuity** was expected to carry the work: a control is bounded on four sides,
  texture is bounded where the scan stopped. Measured, the populations do not separate — the
  corpus's two genuine menu buttons score **0.27 and 0.54**, kept panels range 0.15–0.75.
  It costs 24 of 27 true detections to remove 14 false ones. **Removed.**
- **Alignment as a rejection** trades 4 false positives for 4 true ones. A shuffle, not an
  improvement. **Removed as a rejection, retained as a feature** for a future classifier.
- **Scene-sized rejection** never fired on the corpus and was removed — then **reinstated**
  when a synthetic full-frame test showed a blank 640x480 frame produces exactly one
  candidate covering 100% of itself. Every corpus frame is a *crop*, and a crop of arena
  never contains the uniform full-frame background a real screen has.

## The finding: this corpus cannot tune a normalised size rule

The size filter looked like the whole win — until it was shadowed over a real screen.

| `minNormSide` | live 1920x1080 kept | corpus false positives |
|---|---:|---:|
| 0.045 | **4 of 83** | 16 (71% → 37%) |
| 0.030 | 7 of 83 | — |
| 0.020 | 28 of 83 | 65 (no improvement) |
| 0.015 | 42 of 83 | 65 (no improvement) |

0.045 is an 86x49px floor at 1080p. A real button is ~30px tall (0.028), so that value
rejects essentially every control on a real screen. **It scores well on the corpus only
because the corpus is crops**: a 24px region is 10% of a 240px crop and 2% of a real screen.

The two demand opposite values. The shipped threshold is **0.015** — safe for a 16px checkbox
at 1080p — and the corpus consequently shows **no improvement**. That is the honest outcome.

## The ceiling

At any threshold safe for real controls, a 24x24 patch of arena texture survives, because a
24x24 uniform square **is** a checkbox and **is** a patch of texture. Rectangle geometry has
no further information to appeal to. Recorded as
`TestSpeckledTextureIsIndistinguishableFromControls`, which asserts the specks are kept — so
a future over-fit cannot quietly reintroduce itself.

## Live shadow, real screen

Benchmark-only by construction (`cmd/director/shadow_test.go`), so it was run over a frame
captured outside the Director:

```
baseline   83 candidates, 83 kept   → 73 panel, 9 button, 1 bar
tuned      83 candidates, 42 kept   → 32 panel, 9 button, 1 bar   (41 refused: too_small)
```

Output halved while **every button and bar the baseline found was preserved**. That is a
genuine precision gain, visible only on real evidence — the corpus cannot show it.

## Metrics, before and after, identical evidence

| | baseline | shipped |
|---|---:|---:|
| detections (corpus) | 92 | 92 |
| nameable coverage | 40% | 37.5% |
| false structure | 67% | 69% |
| persistence | 48% | 46% |
| score | 55.9 | ~54.6 |
| **live screen kept** | **83 of 83** | **42 of 83** |
| median latency | ~0 | ~0 |

The corpus numbers are flat by design. The improvement is real and is not in that column.

## A benchmark defect this exposed

The score is **anti-correlated with precision** on this corpus. Temporal metrics are worth 40
of 100 points, and the corpus is six heterogeneous crops rather than a sequence, so
"persisted across frames" mostly measures whether two unrelated rectangles fell in the same
coarse ninth. Emitting more junk raises persistence and therefore the score: at
`minNormSide` 0.045 the ground-truth false-positive rate halves while the score **drops**
55.2 → 50.9.

Until the corpus is a real full-resolution sequence, the score cannot rank detectors on
precision — which is what a detector comparison is for.

## Not done

DNFC fixtures (no privacy-safe extraction path exists yet), grid and meter detection,
temporal post-filtering, OCR-opportunity measurement. All are blocked on the same thing: a
corpus that can measure them.

## Related

- [[Vision]], [[Roadmap]], [[Experiment-001-vision-backend-comparison]]
- [[ADR-004-vision-cannot-establish-actionability]]
