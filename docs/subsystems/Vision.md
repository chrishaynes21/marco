---
type: subsystem
status: blocked
owners:
  - director
depends_on:
  - perception
used_by:
  - fusion
  - passive-observation
  - game-packs
updated: 2026-08-09
source_paths:
  - internal/director/perception/providers/vision
  - internal/platform/visionclient
  - internal/director/visionbench
  - plugins/vision
  - pkg/directorapi/nameability.go
---

# Vision

Vision is an evidence source in the [[Perception]] pipeline: a learned detector that
proposes structure where accessibility has nothing to say — a game HUD, a canvas, a
custom-drawn control.

Status is **blocked**, and the blocker is the model's class vocabulary rather than the
wiring. See [[Roadmap]].

## Responsibilities

- Produce structural detections with confidence.
- Propose regions for scoped OCR, and read inside the ones whose role may be named.
- Preserve provenance and confidence.
- Never establish actionability directly.

## What it may not do

- Establish actionability — [[ADR-004-vision-cannot-establish-actionability]].
- Send coordinates back up from the plugin. The image goes down; the plugin returns boxes in
  its own frame, and placement happens on the Go side.
- Run without being asked. Vision is opt-in, always, and scoped READING is opt-in per cycle
  on top of that — `SourceOCR` in the request is the permission.
- Create structure from text. A word read inside a region never produces a control, a role or
  a detection; see [[ADR-017-structure-earns-a-name-text-never-earns-structure]].

## Scope of the blocking constraint — narrowed 2026-08-06

The constraint below was treated as the blocker on naming *in general*. It is not.
Accessibility supplies nameable structural roles wherever an application exposes a tree: a
targeted VS Code session produces readable `button` and `tab` labels with no vision involved
at all. What follows binds on surfaces where **accessibility genuinely does not exist** — a
game HUD, a canvas, a custom-drawn control. See [[director-accessibility-targeting]] and the
superseded finding in [[Experiment-002-dnfc-observation-baseline]].

## The blocking constraint

A label may be kept in plaintext only when it is attached to a **structural control role**,
and the closed allowlist is `button, menu_item, menu, tab, checkbox, radio`. The chain is:

```
detector emits a nameable ROLE
  → scoped OCR reads inside that region
    → privacy classifier permits plaintext
      → closed-vocabulary terms reach state, hypothesis and durable identity
        → a request can NAME what it acts on
          → a game pack can write a rule
```

**Everything after step one is built and measured as of 2026-08-09**
([[Experiment-010-vision-structure-as-a-semantic-path]],
[[ADR-017-structure-earns-a-name-text-never-earns-structure]]). A detector emitting `button` on
a surface with no accessibility tree now produces interface terms and a durable signature, and
two structurally identical settings screens are separated by them. The allowlist is one policy,
`directorapi.ElementRole.NameablePlaintext`, and it gates what is **read** as well as what is
kept — so an unsayable region costs no OCR round trip and is counted as `LabelsUnsayable`.

The chain still breaks at step one, on the shipping model, which emits exactly
one class, `icon`, which is **not** in the allowlist. It therefore scores 100% structural
coverage in the benchmark while being incapable, in principle, of ever producing a readable
label.

Every downstream limitation observed across several milestones traces to that single fact.

Since 2026-08-06 the benchmark **measures** this rather than obscuring it. `visionbench`
reports **nameable-role coverage** — the share of tracked things whose class maps into the
allowlist above — on the same denominator as structural coverage, so the two are read
together. Only four of the eleven vision classes survive the mapping (`button`, `menu`,
`checkbox`, `radio`); `icon`, `slot`, `panel`, `bar` and `field` are structural and unsayable,
and `text` and `image` are not structural at all. It carries **10 of the 100 score points**,
taken out of structural usefulness (30 → 20) rather than added beside it, because structure
that cannot be named was being paid for twice.

## Admission — and why the detector is still in the shadows

[[ADR-101-visual-presence-is-not-legal-actionability]] recorded the decision:
`SCREENPARSER_DECISION = REMAIN_SHADOW_ONLY`. It first recorded that no ONNX Runtime was
installed to measure with, which was **wrong** — 1.28 was in `tools/onnxruntime/` inside this
repository, and the search that concluded otherwise had looked everywhere except the tree it was
standing in. The measurement was taken and the ADR corrected.

[[Experiment-016-desktop-perception-corpus]] then asked the question admission actually turns on,
over six coherent desktop moments: what does the detector ADD to perception that already works?
Nothing measurable. All 302 of its unmatched detections fall inside elements production already
perceived; the fairest additive reading leaves 12%, and those are boxes without meanings. Its
single most confident unique detection in the corpus is a `<span>` styled to look like a button
and wired to nothing — production reads it as `text`.

[[ADR-102-a-detector-earns-its-place-where-perception-fails]] draws the general rule out of that:
a detector is admitted by the ABSENCE of better evidence, never by its own quality. Experiment 015
and Experiment 016 ran the same model and disagreed completely, because a game has no
accessibility tree and a Settings page has a good one. So ScreenParser's value is a function of
what the other sensors are not providing, and the next question is degradation detection rather
than admission.

It also closed the hole admission would have fallen into: `Targetable()` derived capability from
role alone, so a detector rectangle classified `button` read as legally targetable with nothing
having claimed a mechanism to press it. Affordance and capability are now separate questions, and
the second is decided from provenance.

## Related systems

- [[Fusion]] — weights these claims down deliberately
- [[Perception]] — the pipeline this sits in
- [[Passive-Observation]] — no longer vision-only: accessibility reaches a targeted session
  since [[director-accessibility-targeting]]
- [[Game-Packs]] — the consumer that needs nameable roles

## Decisions

- [[ADR-004-vision-cannot-establish-actionability]]
- [[ADR-017-structure-earns-a-name-text-never-earns-structure]] — structure earns a name; text
  never earns structure. The role/nameability/OCR-eligibility table lives there.
- [[ADR-003-evidence-authority-by-source]]

## Related experiments

- [[Experiment-001-vision-backend-comparison]] — three detectors, frozen corpus
- [[Experiment-002-dnfc-observation-baseline]] — what one class produces over three minutes
- [[Experiment-003-classical-cv-tuning]] — how far geometry alone goes, and where it stops
- [[Experiment-010-vision-structure-as-a-semantic-path]] — vision structure becoming semantic
  without accessibility, and what it cost
- [[director-vision-backend-decision]] — the recommendation that came out of both

## The corpus is now the blocker

Established by [[Experiment-003-classical-cv-tuning]]: the reference corpus is six
heterogeneous **crops**, not an ordered full-resolution sequence, and two consequences follow
that affect every detector rather than any one of them.

Temporal metrics carry 40 of the score's 100 points and the corpus cannot support them, so
the score is **anti-correlated with precision** — a detector that filters nothing scores
best. And a normalised size rule needs opposite thresholds for a crop and a real screen, so
nothing scale-relative can be tuned on it.

Neither is a classical-CV problem. A learned challenger would be ranked by the same broken
score. See [[Roadmap]] item 4a.

**The measurement is fixed as of 2026-08-07** ([[Experiment-004-vision-corpus-v2]]): declared
ground truth with negative regions, paired precision/recall metrics including **temporal
precision**, and `ScoreV2`, which reports *unavailable* rather than zero on a corpus that
cannot support it. The legacy corpus is marked and every report says it cannot calibrate.

**Temporal semantics were split in two on 2026-08-07**
([[Experiment-005-transition-temporal-metrics]]). Persistence is the right question only for a
scene that sits still; a sequence that changes must be asked whether the detector started, or
stopped, finding an element at the right moment. Every sequence now declares its mode —
`static`, `appearance`, `disappearance` — plus a transition boundary, in `sequence.json`, and
the declaration is checked against the annotations rather than trusted. The governing rule:

> A transition is not a weak form of persistence. Measure appearance as appearance and
> disappearance as disappearance.

The same milestone found that `pause-close` was mis-annotated — the pause menu reopens inside
the sequence, at full opacity, on frames declared to hold no interface. **Repaired**: the
sequence now declares `[[0,1],[3,5]]` for its six menu identities. What had been read as
detector staleness for two milestones was a ground-truth defect.

**Tracking was cleared of the button-fragmentation failure on 2026-08-07**
([[Experiment-006-button-track-fragmentation]]). Replaying real ScreenParser detections over
the pause corpus through a proven mirror of the production matcher gives **45 button
detections, 5 tracks, zero fragmentation events** — at every threshold from 0.20 to 0.50 and
under every reference policy. Recall on fully-rendered menu frames is 1.00 and falls to 0.40
across fade-in/fade-out transitions. The tracker handles stable menu content correctly; what
it is fed live is the open question. The governing rule:

> Do not optimize sampling because tracking looks bad. Do not loosen tracking because sampling
> looks sparse. Use the trace to prove which layer is losing identity.

What remains is the evidence. Producing it is [[Vision-Corpus-Workflow]], and its one
irreducibly human step is deciding that a picture of the user's screen may be committed.

## Validated by

- `cmd/director/vision_e2e_test.go` — detect → place → filter → fuse → world, fake detector
- `internal/platform/visionclient/visionclient_test.go` — encode → call → decode, real PNG
  and real JSON against a fake host
- `internal/director/perception/providers/vision/vision_test.go` — class/role mapping
- `cmd/director/shadow_test.go` — a benchmark-only challenger cannot reach runtime composition
- `internal/director/visionbench/visionbench_test.go`
  `TestStructurePerfectionDoesNotHideAnUnnameableVocabulary` — two backends at 100% structural
  coverage, one speaking only unsayable roles, must not score the same

## Known gaps

1. **No model has ever run end to end.** Every test uses a fake detector returning scripted
   detections. What has never run is the join: a real capture, a real subprocess, a real
   model, a real screen.
2. **The shipping model is AGPL-3.0.** Its metadata reads `Ultralytics YOLO11m`. Using its
   weights requires open-sourcing the consuming project or buying a licence.
3. ~~**The benchmark cannot score the thing that matters.**~~ Closed 2026-08-06 —
   nameable-role coverage is measured and weighted, see above. On the frozen fixture classical
   CV reads **40%** and Grounding DINO **20%**. The shipping detector's predicted **0%** is
   still unconfirmed: it needs `-tags onnxvision` and `$MARCO_VISION_MODEL`, and without them
   all six frames fail rather than scoring zero. See
   [[Experiment-001-vision-backend-comparison]].

## Milestone record

[[director-vision]] — where it sits, what gets refused, grids, scoped reading, the frame log.
