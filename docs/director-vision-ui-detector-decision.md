---
type: decision-record
status: accepted
date: 2026-08-07
affects:
  - vision
  - visionbench
source_paths:
  - plugins/vision/backend_onnx.go
  - internal/director/visionbench
---

# Choosing one UI-trained detector challenger

A survey, not an integration. No weights were downloaded, no runtime installed, no adapter
written. The output is a decision about what to download and why.

## The problem Corpus V2 established

Two detectors have now failed on real full-resolution game frames, for two different reasons,
and both reasons point at the same missing property.

**Classical CV: 0% structural recall** ([[Experiment-004-vision-corpus-v2]]). It finds
fragments of UI boundaries rather than UI elements — a 400x283 pause panel becomes about
twelve edge slivers, best IoU 0.065. Its output has no element-level granularity.

**The shipping detector emits one class, `icon`**, which is not in the privacy plaintext
allowlist ([[Experiment-001-vision-backend-comparison]]). It therefore cannot produce a
nameable role however well it localises.

So the question this challenger has to answer is narrow and specific:

> Can a model trained on user interfaces produce **element-level boxes with a rich enough
> role vocabulary to be nameable**, on game frames?

Nameable precision and recall have been 0% for four milestones. That is the number to move.

## Candidates

| | ScreenParser | GroundCUA `uitag` | OmniParser `icon_detect_v3` | Salesforce GPA-GUI |
|---|---|---|---|---|
| weight licence | **Apache-2.0** | **MIT** | **MIT** | MIT (claimed) |
| architecture | YOLO11-Large | YOLO11s | YOLOv9 | Ultralytics YOLO |
| classes | **55** | 9 | interactive-element (no published ontology) | not enumerated |
| weights | `best.pt`, **153 MB** | `yolo-ui.pt`, 18 MB | not surveyed in detail | `model.pt` |
| input | 1280px, full frame | 640px **tiled**, 20% overlap | — | — |
| ONNX shipped | **no** | **no** | **no** | **no** |
| training | ScreenParse v2, 1.45M screenshots | GroundCUA, 55K screenshots / 3.56M annotations | — | finetuned from OmniParser |
| reported | — | mAP@0.5 0.792; 83.6% P / 94.0% R on its own benchmark | — | — |

## Licensing — code, weights, and the toolchain are three different questions

The correction that prompted this survey is real: **permissively licensed UI detectors now
exist.** ScreenParser publishes Apache-2.0 weights with a 55-class ontology, and GroundCUA's
listing is MIT. There is no need to accept AGPL merely to run the experiment.

But a model card's licence field covers the **weights**, and three of the four candidates are
**Ultralytics YOLO11** architectures whose reference implementation is AGPL-3.0. That matters
in a specific and limited way:

| what | ScreenParser | GroundCUA | OmniParser v3 |
|---|---|---|---|
| weights | Apache-2.0 | MIT | MIT |
| inference *via the `ultralytics` package* | AGPL code path | AGPL code path | **not required** |
| inference *via ONNX Runtime* | no AGPL code | no AGPL code | no AGPL code |

OmniParser is explicit, and worth quoting because it settles the historical question this
project had carried forward:

> "`icon_detect_v3` is based on the MIT-licensed YOLOv9 implementation. Earlier
> Ultralytics-based icon detectors retain their original AGPL license."

So the old AGPL conclusion applies to `icon_detect` v1/v2 and **not** to v3. That assumption
should not have been carried forward unchecked, and is corrected here.

**Salesforce GPA-GUI is the murkiest and is excluded on that basis**: MIT is claimed while the
card also states it "is a YOLO model trained with the Ultralytics framework" and is "finetuned
from the OmniParser ecosystem", without enumerating classes or addressing upstream terms. A
candidate whose ontology is unpublished and whose licence chain is unstated cannot be assessed,
let alone defended.

**Assessment, stated with the uncertainty it actually has:**

| | benchmark use, locally | redistribution in a product |
|---|---|---|
| ScreenParser | allowed | **unclear** — Apache weights, AGPL training/export toolchain |
| GroundCUA | allowed | **unclear** — same shape |
| OmniParser v3 | allowed | likely allowed |
| Salesforce | unclear | unclear |

Whether an ONNX graph exported *by* AGPL tooling is a derivative work of that tooling is a
genuinely contested question, and this note does not pretend to settle it. What is safe to say
is that **local benchmark use is fine for all of them**, and that distribution needs a real
answer before any of this ships. That distinction is recorded here rather than buried in a
Known Gaps line.

## Runtime — the finding that changes the plan

**No surveyed candidate ships an ONNX artifact.** Every one publishes PyTorch `.pt` weights
only.

The intended route was:

```
Go plugin → ONNX Runtime → model        (no Python at runtime)
```

That route still works, but it now has a one-time step in front of it:

```
Python + ultralytics → export .onnx     (once, offline)
```

This machine has **no Python at all**, and the milestone brief said not to install it. The
constraint was written about the *runtime*, and the distinction is worth making explicitly:

- **development dependency** — Python + ultralytics, used once to produce an `.onnx` file, on
  any machine, never shipped;
- **runtime dependency** — ONNX Runtime DLL + the `.onnx` file + the existing Go plugin.

Only the second is a permanent cost to this project. But the first is not free either: it is a
multi-GB install for one export, and `best.pt` is a **153 MB pickle with 36 pickle imports**,
which executes code on load. Neither is a reason to stop; both are reasons to decide
deliberately rather than by momentum.

## Vocabulary — where ScreenParser separates from the field

Marco's nameable allowlist is `button, menu_item, menu, tab, checkbox, radio`. Mapping each
candidate's published ontology onto it:

| model class | Marco class | nameable | OCR-eligible | confidence |
|---|---|---|---|---|
| **ScreenParser** | | | | |
| Button, Utility Button | button | **yes** | yes | high |
| Menu, ContextMenu, DockMenu, EditMenu, PopUp Menu | menu | **yes** | yes | high |
| Tab, Tab Bar | tab | **yes** | yes | high |
| Checkbox | checkbox | **yes** | yes | high |
| Radiobox | radio | **yes** | yes | high |
| List Item | menu_item | **yes** | yes | medium |
| Text Input, Search Field, Search Bar | field | no | yes | high |
| Progress bar, Slider | bar | no | no | high |
| Window, Side Bar, Toolbar, Alert | panel | no | limited | high |
| App Icon, File Icon, Logo, Avatar | icon | no | no | high |
| Text, Heading, Code snippet | text | no | yes | high |
| **GroundCUA** | | | | |
| Button | button | **yes** | yes | high |
| Menu | menu | **yes** | yes | high |
| Input_Elements | field | no | yes | medium |
| Navigation, Sidebar | panel | no | limited | low |
| Information_Display, Visual_Elements | unknown | no | no | **very low** |
| Others, Unknown | unknown | no | no | n/a |

ScreenParser maps onto **six** of Marco's nameable roles. GroundCUA reaches **two**, and three
of its nine classes (`Others`, `Unknown`, `Visual_Elements`) carry no usable semantics at all.
OmniParser v3 publishes no ontology — it detects interactive elements generically, which is
structurally the same limitation as the shipping detector's single `icon` class.

**Unknown stays unknown.** No ambiguous class is mapped to a nameable role to improve a score;
that is precisely the inflation `NameablePrecision` was added to catch.

## Game-generalization — the honest caveat for all of them

None of these were trained on games. ScreenParse v2, GroundCUA and OmniParser are desktop, web
and mobile interfaces. A game HUD is drawn, stylised, translucent, and sits over moving
scenery — none of which appears in a screenshot corpus of Chrome and VS Code.

This is the same trap [[Experiment-001-vision-backend-comparison]] fell into, and its
conclusion stands as the null hypothesis: **game UI may simply not be desktop UI.** The
benchmark exists to find that out, and either answer is informative. What has changed is that
the benchmark can now tell the difference — declared negative scenery, temporal precision, and
nameable precision are exactly the instruments for it.

One structural point in ScreenParser's favour: it runs **full-frame at 1280px**, which is much
closer to the 1920x1080 evidence in Corpus V2 than 640px tiles. GroundCUA's tiled inference is
a real integration cost — tiling, 20% overlap, and cross-tile NMS all have to live in the
plugin — and tiles also destroy the whole-screen context that distinguishes a HUD from scenery.

## Ranking

Weights are for *selection*, not for benchmarking.

| | vocab 30% | ONNX/runtime 25% | licence 20% | game fit 15% | size/latency 10% | total |
|---|---:|---:|---:|---:|---:|---:|
| **ScreenParser** | **9** | 6 | 7 | 7 | 4 | **7.05** |
| GroundCUA | 4 | 5 | 7 | 6 | 9 | 5.65 |
| OmniParser v3 | 2 | 7 | 9 | 5 | 8 | 5.25 |
| Salesforce | 2 | 5 | 3 | 5 | 6 | 3.80 |

## Decision

```
SELECTED CHALLENGER
docling-project/ScreenParser  (YOLO11-Large, Apache-2.0 weights, 55 UI classes)
```

**Why this one.** It is the only candidate whose ontology can move the metric that has been
zero for four milestones. Six of Marco's nameable roles map onto its classes with high
confidence; nothing else surveyed reaches more than two. Its weights are Apache-2.0, the
cleanest licence among the ontology-rich options, and it runs full-frame at a resolution close
to the corpus.

**Why not OmniParser v3.** Its licence is the cleanest of all and its runtime is the lightest —
but it detects interactive elements without a published role vocabulary, which is
architecturally the same thing the shipping detector already does. Benchmarking it would
re-run an experiment whose answer is on file. Lowest information gain of the three.

**Why not GroundCUA.** Attractive on cost — 18 MB, MIT, strong reported numbers — but two
nameable roles and three semantically empty classes, plus tiled inference that both complicates
the plugin and discards the whole-frame context needed to separate HUD from scenery. It is the
right *second* experiment if ScreenParser's vocabulary transfers but its latency does not.

**Why not Salesforce.** Unpublished ontology and an unstated licence chain. Not assessable.

**Licence status**: benchmark use allowed (Apache-2.0 weights). Redistribution **unclear**,
pending a real answer on ONNX export by AGPL tooling. Do not treat this as cleared for
shipping.

**Expected ONNX path**: standard Ultralytics export (`YOLO('best.pt').export(format='onnx')`),
one-time, offline. Static 1280x1280 input, standard YOLO detection output, NMS outside the
model. No custom operators expected.

**Expected download**: 153 MB weights, plus an ONNX Runtime DLL (~200 MB) for the Go plugin.

**Expected integration delta** against `plugins/vision/backend_onnx.go`: input resolution 1280
rather than the current default; a 55-entry class map replacing the embedded `names` read; NMS
and confidence thresholds calibrated per Part 7 on a held-out split. No architectural change —
this is what the plugin seam was built for.

## Next command

**Not** an integration. One decision remains, and it is the user's, because it is the only part
of this that installs something:

> ScreenParser ships `.pt` only. Producing the `.onnx` needs a one-time Python + ultralytics
> install (development dependency, never shipped). Approve that, or supply an `.onnx` exported
> elsewhere.

Once resolved, the next engineering task is bounded and singular: export ScreenParser to ONNX,
register it as a benchmark backend behind the existing plugin seam, calibrate its confidence
threshold on `freeplay-static` + part of `pause-stable`, and evaluate on the held-out
sequences. Shadow-only; the boundary test in `cmd/director/shadow_test.go` stays.

## Related

- [[Experiment-004-vision-corpus-v2]] — the corpus and the 0% result that motivates this
- [[Experiment-001-vision-backend-comparison]] — why a one-class detector cannot be nameable
- [[Vision]], [[Roadmap]]

Sources: [ScreenParser](https://huggingface.co/docling-project/ScreenParser),
[ScreenParser files](https://huggingface.co/docling-project/ScreenParser/tree/main),
[GroundCUA uitag](https://huggingface.co/laywens/uitag-yolo11s-ui-detect-v1),
[OmniParser](https://github.com/microsoft/OmniParser),
[Salesforce GPA-GUI-Detector](https://huggingface.co/Salesforce/GPA-GUI-Detector)

---

# Export and first measurement — 2026-08-07

## Export

Isolated venv at `tools/vision-export/.venv`, git-ignored, nothing global:

```
Python 3.12.10 (winget, user scope)   torch 2.13.0+cpu
ultralytics 8.3.253                   onnx 1.22.0     onnxruntime 1.28.0
```

Weights pinned: `docling-project/ScreenParser`, revision `f029e565f1206577402e43206454522075be3f72`,
`best.pt`, 153,259,543 bytes,
sha256 `dbcb4f583ccfdb8100a68e606525c247890a2de4c1a54b14741e0ee29ce0ab88`, Apache-2.0.

Exported `screenparser-1280.onnx` (97.4 MB, opset 17, **NMS not included** — the plugin owns
it). YOLO11l, 25.3M params, input `(1,3,1280,1280)`, output `(1,59,33600)` = 4 box coords +
55 class scores across 33600 anchors.

**ONNX ≡ PyTorch, verified before benchmarking**: 4/4 detections matched, **100% class
agreement**, median IoU **0.9921**, max confidence delta 0.081. The conversion is sound.

## Calibration — frozen before any held-out run

Split by SEQUENCE (`freeplay-static` + `pause-stable` calibrate; camera-motion, pause-open,
pause-close held out), chosen before a threshold was picked.

| conf | dets | precision | recall | nameable P | nameable R | TP | FP |
|---:|---:|---:|---:|---:|---:|---:|---:|
| **0.15** | 72 | **100%** | **73%** | 80% | **100%** | 45 | **0** |
| 0.25 | 58 | 100% | 58% | 75% | 75% | 36 | 0 |
| 0.35 | 55 | 100% | 58% | 75% | 75% | 36 | 0 |
| 0.50 | 0 | — | — | — | — | 0 | 0 |

Threshold frozen at **0.15**.

On `pause-stable` it finds exactly `4 × Button + 1 × Heading` — the four menu items and the
title — consistently across frames: **100% precision, 83% recall, 83%/83% temporal**. Against
Classical CV's 0% recall and 1921 false positives, that is a different category of result.

## The held-out run is INVALID, and the corpus is why

At the frozen threshold the held-out sequences returned ScoreV2 **4.0** (13% precision, 9%
recall). That number must not be reported as ScreenParser's performance, because two defects
in the corpus — both mine, both introduced this session — make it unmeasurable:

**1. Four frames belong to two sequences at once.** `pause-cycle-029`, `-030`, `-036`, `-037`
were copied into both `pause-stable` and the transition sequences, to give each a proper
boundary. Every lookup in the evaluator is keyed by frame BASENAME, so a duplicated name
resolves to one sequence and the other silently loses its menu frames. `pause-open` scored 0
TP on frames that plainly contain the menu.

**2. `hud_boost` is annotated on frames whose HUD was masked.** Sanitisation masked the bottom
band of *every* pause-cycle frame, including the ones that are gameplay — and those legitimately
had a boost meter there. Seven annotations across `pause-open` and `pause-close` now claim a
region sanitisation removed, which is precisely what the workflow warns against. They count as
unfound truth and depress recall.

`pause-close` shows both at once: 31 detections, 0 TP, 13 FP.

The calibration figures are unaffected — `pause-stable` and `freeplay-static` contain no
duplicated frames and no masked-away annotations — so the strong signal stands. The held-out
verdict does not.

## Verdict: CONTINUE

Not *promote*: no valid held-out measurement exists yet. Not *reject*: the calibration evidence
is the strongest any detector has produced on this corpus by a wide margin, and the two
blockers are corpus bugs with known fixes, not model limitations.

**Exactly one follow-up: repair the corpus and re-run the held-out evaluation.** De-duplicate
the boundary frames (give each sequence its own copies under distinct names, or key the
evaluator by sequence+frame), and reconcile the annotations with the sanitisation mask. Neither
is speculative work.

Also outstanding, and deliberately not started: **the Go plugin transport**. Inference currently
runs the ONNX artifact through Python. Every metric above is computed by the real Go `ScoreV2`,
but ScreenParser is not yet reachable through `director benchmark-vision`, so the wiring test
this repository has learned to demand does not exist. That remains part of promotion, not of
this measurement.

**Latency**: median 602ms, p95 643ms, CPU, 1280px. Too slow for the 200ms sampling floor;
usable at a 1s–2s cadence. No optimisation attempted.

**Licensing**: Apache-2.0 weights; Ultralytics used only as export tooling and removed from the
runtime path. Local benchmark use acceptable. Redistribution still requires a real review if
this is ever promoted — unchanged from the survey.

---

# Corpus repair and valid held-out result — 2026-08-07

## Repair

**Frame identity is now sequence-scoped.** `FrameTruth.Key()` returns `sequence/frame`, and
every lookup uses it. Sharing an image between two sequences stays legal — one picture is a
legitimate observation in two temporal contexts — while sharing an *identity* is now caught by
`DuplicateKeys`.

**Ignore regions are a first-class third category.** The privacy mask is neither interface
(sanitisation destroyed what was there) nor declared scenery (Marco painted it). A detection
lying wholly inside one is dropped before scoring, so no detector is measured against the
sanitiser's rectangle.

**Phantom annotations removed.** `hud_boost` is gone from all three pause sequences: the mask
covers y>0.80 and the meter sits at y 0.750-0.930, more than half beneath it. Truth regions
fell from 151 to 118 — a reduction that is a correction, not a loss.

Two regressions added, both mandatory, because both defects were found by a result that looked
wrong rather than by a test:
`TestTwoSequencesMaySharePicturesButNotIdentities`,
`TestAnAnnotationInsideAPrivacyMaskIsRefused`, plus
`TestADetectionInsideAPrivacyMaskIsNeitherCreditedNorPunished`.

ScoreV2 sanity re-checked after the repair: box spammer **9.8**, narrow-but-correct **49.2**.
The ranking survived the change to identity and ignore semantics.

## Calibration unchanged, threshold still frozen

The calibration sequences contained no duplicated frames and no masked-away annotations, so
calibration is bit-identical after repair and **confidence stays frozen at 0.15**. No sweep was
re-run.

## Held-out — valid this time

| | invalid run | **repaired** |
|---|---:|---:|
| structural precision | 13% | **63%** |
| structural recall | 9% | **55%** |
| nameable precision | 0% | **67%** |
| nameable recall | 0% | **80%** |
| temporal precision | 0% | **27%** |
| temporal recall | 0% | **71%** |
| ScoreV2 | 4.0 | see below |

**Nameable recall 80% on held-out evidence.** That number has been 0% for four milestones.

## By sequence — where the real behaviour is

| sequence | dets | TP | FP | precision | recall | T-prec | T-rec |
|---|---:|---:|---:|---:|---:|---:|---:|
| freeplay-camera-motion | 14 | 2 | **0** | **100%** | 20% | 0% | 0% |
| pause-open | 22 | 10 | **0** | **100%** | 56% | 33% | 83% |
| pause-close | 45 | 10 | 13 | 43% | 83% | 25% | 0% |
| *pause-stable (cal)* | 63 | 45 | 0 | 100% | 83% | 83% | 83% |

**Camera motion is the headline.** Zero false positives while the entire arena rotates. The
test that Classical CV failed catastrophically — 1921 false positives on scenery — ScreenParser
passes outright. It does not hallucinate interface in a moving game world. Its 20% recall there
reflects that the only truth in those frames is the boost meter, which it largely misses: a
stylised translucent arc is not a desktop control.

**Pause-close is the failure mode, and it is a specific one.** 13 false positives and 0%
temporal recall: ScreenParser keeps reporting menu elements after the menu has gone. Stale
persistence — exactly what temporal precision was built to catch, caught on the first valid run.

## Verdict: CONTINUE

Not *promote*: the Go runtime path is unproven, and promotion without it would repeat this
repository's oldest mistake.

Not *reject*: this is comfortably the strongest result any backend has produced here — 100%
precision on two of three held-out sequences, zero false positives under camera motion, and the
first non-zero nameable recall in the project's history.

**Exactly one follow-up: wire ScreenParser through the Go → ONNX Runtime plugin path** and
re-run the repaired held-out evaluation through it. The gate the previous brief set — non-trivial
structural recall, materially better precision than Classical, non-zero nameable recall — is met
on every count.

Not started, and deliberately: ONNX Runtime DLL, the Go adapter, Python↔Go equivalence, NMS
equivalence, the benchmark wiring test. Inference still runs through Python; every metric above
is computed by the real Go `ScoreV2`, but ScreenParser is not yet reachable from
`director benchmark-vision`. That is the whole of the next milestone.

The export venv is retained until that path is proven, per its lifecycle rule.

---

# Go → ONNX Runtime integration — 2026-08-07

## What runs

```
plugins/vision (cgo, -tags onnxvision) → onnxruntime_go v1.32.0
  → ONNX Runtime 1.28.0 → screenparser-1280.onnx
```

ONNX Runtime **1.28.0** win-x64, `onnxruntime.dll` 15,809,848 bytes,
sha256 `18370c375f07357fa5874344a9d9ac17e6b6fe1eb18b1dd209d79483b4470257`, git-ignored under
`tools/onnxruntime/`.

**1.20.1 was tried first and refused**: `onnxruntime_go` v1.32.0 requests ORT API 28 and 1.20.1
provides at most 20. The error was explicit, which is the behaviour the diagnostics rules ask
for — it did not silently return zero detections.

## The integration delta was almost nothing

The existing backend already implemented every piece ScreenParser needs, because it was built
for exactly this shape of model: dynamic runtime loading via `$MARCO_ONNXRUNTIME`, configurable
input side, letterbox with grey-114 padding, CHW float32 0..1, `[1, 4+nc, n]` channel-major
decode with `cx,cy,w,h`, class-aware greedy NMS, and reverse-letterbox mapping. Class names come
from the model's own embedded `names` metadata — all 55, with no hand-maintained list to drift.

The whole configuration is:

```
MARCO_VISION_MODEL=…/screenparser-1280.onnx
MARCO_VISION_SIZE=1280        # not the 640 default
MARCO_VISION_CONF=0.15        # the frozen threshold
MARCO_VISION_IOU=0.45
MARCO_ONNXRUNTIME=…/onnxruntime.dll
```

That the seam absorbed a 1280px 55-class YOLO11 model without a code change is the strongest
evidence yet that the plugin boundary was drawn in the right place.

## Parity — Go against the validated Python ONNX reference

Same frame, `pause-cycle-033`, conf 0.15:

| class | Go | Python |
|---|---|---|
| Button | 0.43 (796,562 330x39) | 0.45 (795,562 330x39) |
| Button | 0.43 (795,607 330x38) | 0.47 (796,606 330x38) |
| Button | 0.40 (796,518 330x38) | 0.36 (795,517 329x38) |
| Button | 0.25 (795,472 330x40) | 0.22 (794,472 329x40) |
| Button | 0.37 (877,483 21x21) | 0.37 (876,482 21x21) |
| Heading | 0.43 (790,412 349x43) | 0.38 (791,410 345x44) |
| Image | 0.39 (821,723 278x140) | 0.42 (822,724 283x139) |

**7 detections both sides, identical class set, boxes within 1-2px, confidence within ~0.05.**

The confidence spread has a known cause and it is worth naming rather than absorbing: the Go
letterbox resizes **nearest-neighbour**, Ultralytics resizes **bilinear**. Different input
pixels, same detections, slightly different scores. It is a real difference and it did not
change what was found, at this threshold, on this frame.

## Not done

**The benchmark command does not yet select ScreenParser.** `director benchmark-vision` has no
`screenparser` backend registered, so there is no wiring test and the repaired held-out
evaluation has NOT been re-run through Go. The plugin is proven at the CLI; the
registry→visionbench→ScoreV2 path is not.

That is the remaining work, and it is deliberately not claimed as finished. Given this
repository's record, an unwired backend reported as integrated would be the exact failure
[[Wiring-Tests]] exists to prevent.

Also outstanding: Go-path latency and memory, full sequence-level parity, fragmentation through
Go, and the stale-persistence diagnosis.

## Benchmark registration — 2026-08-07

`director benchmark-vision --backend=screenparser` now runs:

```
CLI → availableBackends → screenparserBackend → vision plugin (cgo, -tags onnxvision)
    → ONNX Runtime 1.28.0 → screenparser-1280.onnx → detections → visionbench
```

**14 detections crossed that path.** No Python process is involved. Registration lives in
`availableBackends` and nowhere else — the same single site the Grounding DINO challenger uses,
so `shadow_test.go` continues to guarantee it cannot reach a runtime composition.

Availability is reported as a reason, never as an empty frame: a missing model, an unset
`$MARCO_ONNXRUNTIME` and a missing plugin each produce a distinct message.

### And it immediately found the next defect

All 14 detections were **rejected by the acceptance filter**. The plugin emits ScreenParser's
native class names — `Button`, `Heading`, `Image` — and `vision.ClassOf` speaks Marco's closed
vocabulary, so every one is unknown and discarded.

This is precisely the defect [[Experiment-001-vision-backend-comparison]] recorded and named:

> "The adapter and the acceptance filter spoke different vocabularies. Twelve of thirteen
> detections were discarded as unknown and the report blamed the model."

It has happened again, in a new backend, for the same reason. The mapping exists and is already
written down — 55 ScreenParser classes onto Marco's vocabulary, used by the Python-path
evaluation that produced the valid held-out numbers — but it lives in the benchmark test rather
than at the plugin boundary where the detections are produced.

**That is the one remaining piece**, and it is small: move the existing mapping into the
ScreenParser backend so `Button → button` before the acceptance filter sees it.

Not done, and consequently unmeasured through Go: the repaired held-out ScoreV2, sequence-level
parity, latency, memory, and the temporal audit of pause-close persistence. All are blocked
behind the same one-line-shaped fix, and none of them is claimed.

## Class normalisation at the plugin boundary — 2026-08-07

`plugins/vision/vocabulary.go` maps a detector's native class word onto Marco's closed
vocabulary, applied in `labelFor` — the last point at which the model's own word exists.
Everything above the plugin now receives `button`, `text`, `image`, never `Button`, `Heading`,
`Image`.

Deliberately **generic**, with no `if model == screenparser` anywhere: UI detectors converge on
much the same words, so one table over their union serves every backend. A per-model mapper
would be a second place for the same knowledge to drift, which is the failure this file ends.
Unmapped classes keep their native word and are refused downstream — a closed vocabulary stays
closed by refusing what it does not recognise.

The benchmark-side mapping is now redundant and the plugin is the single source of truth.

### A second gate behind the first

Normalisation alone did not fix it: 14 detections, still 14 rejected. The cause was the
acceptance filter's confidence floors — production requires 0.35 (0.50 structural), and
ScreenParser's real scores run **0.22-0.47**. The four pause-menu buttons it finds perfectly
score 0.25 to 0.47.

This is the same defect Experiment-001 recorded for Grounding DINO — "0.40 for a correct
`menu` against a 0.50 structural floor, twelve of thirteen discarded, and the report blamed the
model" — and the benchmark already had the remedy: the `Calibrated` interface, which lets a
backend declare the floors it should be judged at. ScreenParser now declares **0.15**, the
threshold frozen on the calibration split before any held-out evidence was seen. Not tuning:
the same number the Python reference used.

**Result: 14 detections, 14 accepted, 0 rejected.** Median 529ms, p95 1.069s through the real
Go path on the legacy corpus.

Two gates, both invisible until the wire was actually connected, both previously recorded
against a different backend. Reachability was never the problem; representation was.
