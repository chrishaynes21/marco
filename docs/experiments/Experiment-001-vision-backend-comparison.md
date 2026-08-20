---
type: experiment
status: complete
date: 2026-08-05
backend:
  - classical-cv
  - icon-detect
  - grounding-dino
game: rocket-league
fixture: fixtures/vision/rocketleague
result: continue-tuning
supersedes: []
source_paths:
  - internal/director/visionbench
  - plugins/vision-groundingdino
---

# Experiment 001 — vision backend comparison

Three detectors over a **frozen**, privacy-reviewed six-frame Rocket League corpus, in
identical order, under production-equivalent acceptance.

## Result

```
METRIC                    classical-cv   icon_detect  grounding-dino
detections                          92             8              13
accepted                            92             5               8
structural coverage               100%          100%             20%
anonymous ratio                   100%          100%            100%
temporal persistence               48%           22%             23%
false structure (lower)            67%          100%            100%
median latency                      ~0s         109ms          9.594s
acceptance floors            0.35/0.50     0.35/0.50   0.30 (calibrated)
SCORE                             61.9          53.4            20.2
```

**Verdict: continue tuning, do not promote.** Grounding DINO lost almost entirely on
latency — 9.6s per frame against a 500ms sampling budget, scoring zero on that dimension.
`torch` here is **CPU-only on a machine with an RTX 5070**, so this measures an unaccelerated
install rather than the model.

## Two of these numbers do not measure what they appear to

**Anonymous ratio is 100% for all three.** Naming is worth 20 of 100 points and discriminates
nothing. That is not a tie — it is a blind spot, sitting exactly on the blocking constraint.

**Structural coverage is near-saturated and partly false.** `icon_detect` scores 100% because
`icon` counts as structural in `vision.Class` — but `icon` is **not** in the privacy plaintext
allowlist. The incumbent scores full marks on structure while being incapable, in principle,
of ever producing a readable label.

The dimensions that genuinely separated the three were latency, false structure and
persistence. **The most important axis was invisible.**

## The scores above are no longer produced by the current scoring — 2026-08-06

The blind spot named in this section has since been given a metric and a weight:
`nameable-role coverage`, worth **10 points taken out of structural usefulness** (30 → 20).
See [[Roadmap]] item 1 and [[Vision]].

So the SCORE row here is under the **old** weights and a rerun does not reproduce it. Rerun on
the same frozen fixture, 2026-08-06:

```
METRIC                    classical-cv  grounding-dino   icon_detect
nameable-role coverage             40%             20%     could not run
structural coverage               100%             20%     could not run
SCORE (was)                  55.9 (61.9)     20.2 (20.2)   could not run
```

**Classical CV loses 6.0 points** — it is 100% structural but only 40% nameable, so six of the
ten reallocated points do not come back. It does emit `button`: 9 of its 92 accepted boxes
carry a nameable role, which is the honest version of the "already emits a nameable role"
claim in [[Roadmap]] item 4.

**Grounding DINO is unchanged at 20.2**, and the coincidence is exact rather than lucky: its
structural and nameable coverage are both 20%, so 30×0.20 and 20×0.20 + 10×0.20 are the same
six points. A backend whose structure is *entirely* nameable is held harmless by this
reweighting, which is the intended property.

**`icon_detect` could not be measured.** All six frames failed with *"Vision has no model
loaded (set `$MARCO_VISION_MODEL`, build with `-tags onnxvision`)"* — an environment gap, not a
result. Its predicted 0% nameable still follows from its emitting only `icon`, which is not in
the allowlist, but **that number remains a prediction**; the 53.4 above is the last real
measurement of it and was taken under the old weights.

## Three defects found while running it

1. **`temporal persistence 102%`** — the occurrences-vs-frames bug, previously fixed in the
   observe metrics and reintroduced here. Several boxes of one class landing in one coarse
   ninth share an identity, which is intended, and each was counted. Now counts frames.
2. **The adapter and the acceptance filter spoke different vocabularies.** `NormaliseLabel`
   mapped onto Director roles (`pane`, `menu_item`); the filter speaks `vision.Class`
   (`panel`, `menu`). Twelve of thirteen detections were discarded as unknown and the report
   blamed the model. Retargeted onto the detector contract's own vocabulary.
3. **Not a bug, but it read like one** — with production floors Grounding DINO lost 12 of 13
   to confidence alone, returning 0.32 for a correct text region. Confidence scales are not
   comparable between models, so a backend may now declare its own acceptance floors and the
   report states which were used. Accepted rose from 1 to 8.

## Method notes

- **Frozen fixtures only.** Two backends measured against two different moments are not
  comparable, and the difference would look exactly like a difference between models.
- **Acceptance mirrors production** — same floors, same structural bar, same class
  vocabulary. A backend whose boxes are all rejected downstream scores as not having helped.
- **Closed versioned vocabulary** (`game-ui-v1`), with its digest in every report.
- **Documented weights** *as this experiment ran*: structural 30, temporal 25, naming 20,
  trust 15, latency 10. Detection count carries **weight zero**. Since 2026-08-06 the split is
  structural 20, **nameable 10**, the rest unchanged — see the note above.
- **Shadow-only enforced by test** — `cmd/director/shadow_test.go` fails if a runtime
  composition file mentions the challenger.

## Mutation testing found the tests too weak

Of three mutations the spec asks for, only *reward raw volume* initially failed.
*Ignore anonymous detections* was not caught, because the noisy backend already lost on
structure. *Randomise frame order* was not caught, because every fixture frame was identical.
Both tests were rewritten; all three now fail the suite.

A first mutation attempt also did not compile, which made the count meaningless until one was
written that builds — **a mutation that fails to compile proves nothing.**

## What this implies

The open question is not "is there a better model" but **"is the problem the model or its
vocabulary?"** Nothing here distinguishes those. See
[[director-vision-backend-decision]] for the ranking that follows, and [[Roadmap]] for what
is queued.

## Not done

The DNFC fixture corpus, region-classification mode, scoped-OCR impact measurement, the
statistical-variance report, and memory measurement. Definition-of-Done items 1–4, 6–9, 11–14
are met; 5 is partial (Rocket League only); 10 is open.

## Related

- [[Vision]], [[Fusion]], [[Passive-Observation]]
- [[ADR-004-vision-cannot-establish-actionability]], [[ADR-003-evidence-authority-by-source]]
- [[Experiment-002-dnfc-observation-baseline]]
