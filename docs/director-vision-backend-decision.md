---
type: milestone
status: historical
---

# Which eyes next — a decision from the benchmark evidence

> **Historical record.** This describes the state of the system when it was written. It is
> kept for the reasoning, not as current truth: where it disagrees with a note in `subsystems/`
> or an ADR in `decisions/`, **they win**. See [[AI-CONTEXT]].

> **ACTED ON.** This recommendation was taken. The outcome is
> [[director-vision-ui-detector-decision]], and the vocabulary now lives in
> `plugins/vision/vocabulary.go`. Read this for the *reasoning that led to the choice*, not as an
> open question.

**Recommendation (at the time): swap in a UI-element YOLO detector with a real class vocabulary.**
One experiment, roughly one hour, answers the question everything else is waiting on.

Do not integrate Florence-2, OWLv2, Qwen2.5-VL, Molmo or SAM2 yet. Do not put Grounding
DINO on the GPU yet. The reasons are below.

---

## 1. What the benchmark actually said

Six frames, privacy-reviewed Rocket League corpus, identical order, production-equivalent
acceptance:

| metric | classical-cv | current (icon_detect) | grounding-dino |
|---|---:|---:|---:|
| detections | 92 | 8 | 13 |
| accepted | 92 | 5 | 8 |
| structural coverage | 100% | 100% | 20% |
| **anonymous ratio** | **100%** | **100%** | **100%** |
| temporal persistence | 48% | 22% | 23% |
| false structure | 67% | 100% | 100% |
| median latency | ~0ms | 109ms | 9,594ms |
| **score** | **61.9** | **53.4** | **20.2** |

### Two of these numbers are not measuring what they appear to

**Anonymous ratio is 100% for all three.** Naming is worth 20 of 100 points and it is
currently uniform, so it discriminates nothing. No backend in the comparison produces a
name. That is not a tie — it is a blind spot, and it sits exactly on Marco's blocking
constraint.

**Structural coverage is near-saturated and partly false.** `current` scores 100% because
`icon` counts as structural in `vision.Class`. But the privacy classifier's plaintext
allowlist is `button, menu_item, menu, tab, checkbox, radio` — and `icon` is not in it. So
the incumbent detector scores full marks on structure while being **incapable, in principle,
of ever producing a readable label**. It has one class and that class is disqualified.

The dimensions that genuinely separated the three were latency, false structure and
persistence. The most important axis was invisible.

### The constraint that actually governs everything

From the privacy milestone: a label may be kept in plaintext only when it is attached to a
structural control role. The chain is

```
detector emits a nameable ROLE
  → scoped OCR reads inside that region
    → privacy classifier permits plaintext
      → a request can NAME what it acts on
        → a game pack can write a rule
```

The chain breaks at step one. `icon_detect` emits exactly one class, `names={0:'icon'}`, and
`icon` is not nameable. Every downstream limitation observed across the last several
milestones — 129 anonymous entities on DNFC, "nothing stayed put long enough", `RESUME GAME`
stored as a digest — traces to that single fact.

**The open question is therefore not "is there a better model" but "is the problem the model
or its vocabulary?"** Nothing in the benchmark to date distinguishes those.

---

## 2. A licensing finding, independent of performance

The shipping detector's own metadata reads `Ultralytics YOLO11m`, `author: Ultralytics`,
`version 8.4.66`. Ultralytics YOLO is **AGPL-3.0**: using its models or fine-tuned weights
requires open-sourcing the consuming project or buying an enterprise licence.

OmniParser's own `icon_detect_v3` moved to an MIT-licensed YOLOv9 implementation, which
suggests Microsoft reached the same conclusion.

This is not a benchmark result, but it bears on the same decision: if the detector is being
replaced anyway, replacing it with something permissively licensed resolves a real exposure
at no extra cost. If Marco is ever distributed as a closed product, the current model is a
problem regardless of how well it scores.

---

## 3. The families, against Marco's actual requirements

Requirements, in the order they bind:

1. **nameable roles** — the blocking constraint above
2. **temporal stability** — a passive session needs identities that survive a frame
3. **region proposals for OCR** — scoped reading is what produces names
4. **latency ≤ ~500ms** — the sampling interval; slower means dropped samples
5. **privacy** — no free-form text may become a role
6. **plugin architecture** — must fit the existing detector contract
7. **licence** — see above

| family | nameable roles | stability | latency (this hw) | licence | integration | likely to beat current |
|---|---|---|---|---|---|---|
| **UI-element YOLO** (yolov11-ui-elements, windows-ui-locator) | **yes — that is what it is trained for** | good (single-shot, consistent) | ~100ms, same path | varies, MIT/Apache available | **near-zero: drop-in ONNX** | **high** |
| OmniParser `icon_caption` (Florence-2 base) | no — gives text, role stays `icon` | n/a | +0.6–0.8s GPU | MIT | moderate (2nd subprocess) | low *for the blocker* |
| Florence-2 (open-vocab region+caption) | partial — captions, not roles | moderate | ~1s GPU, worse CPU | MIT | moderate | medium |
| Grounding DINO | yes, via closed prompt | 23% persistence measured | **9.6s CPU** / ~0.5s GPU est. | Apache-2.0 | **done** | measured: no |
| OWLv2 | yes, via closed prompt | untested | similar class to GDINO | Apache-2.0 | moderate | duplicates GDINO |
| Qwen2.5-VL / UI-UG-7B | yes, richest understanding | untested | seconds/frame, 7B weights | varies | high | not at this latency |
| Molmo | yes (pointing) | untested | seconds/frame | Apache-2.0 | high | not at this latency |
| SAM2 | **no — masks carry no semantics** | excellent | ~100ms GPU | Apache-2.0 | moderate | wrong tool |
| classical CV | **yes — already emits `button`** | 48%, best measured | ~0ms | none | **done** | already leads |

---

## 4. Ranking by expected information per engineering hour

**1. UI-element YOLO detector — ~1 hour, resolves the central question.**

The plugin already reads a model's embedded `names` map (added after the incident where a
one-class model was mislabelled "button" and announced 56 desktop icons as controls). A
different YOLO UI model is therefore a *file swap*: same ONNX runtime, same bridge, same
acceptance thresholds, same fixture, same benchmark command. Latency stays ~100ms.

It tests the hypothesis directly. If a detector trained on `button / field / checkbox /
menu / tab` produces nameable roles at comparable latency, then the problem was the
vocabulary and the whole downstream chain unblocks at once — scoped OCR gets regions worth
reading, the privacy allowlist starts permitting labels, and the observe metrics stop
reading 100% anonymous. If it does *not*, that is equally informative: it means UI detection
on game interfaces is genuinely hard and the heavier families become worth their cost.

Either outcome changes what is built next. Nothing else on the list has that property.

**2. Improve classical CV — ~4–8 hours, incremental, already the leader.**

It won at 61.9, runs in ~0ms, has no licence and no dependency, and already emits `button` —
a nameable role. Its weaknesses are measured: 67% false structure and roles assigned by shape
heuristic rather than evidence. Edge-based refinement, connected components instead of a grid
scan, and rejecting regions that fail to persist would all help.

It is second rather than first because it is improving the thing that already works, which
teaches less than finding out whether the thing that does not work is fixable. It is also the
correct **fallback**: if the UI detector experiment fails, this becomes the primary path, and
it is the only candidate with no external dependency at all.

**3. Grounding DINO on GPU — ~2 hours, narrow question.**

`torch` here is CPU-only on a machine with an RTX 5070, so the 9.6s that sank it is an
artefact of the install. Installing CUDA torch would likely move it by an order of magnitude.

But even discounting latency entirely, it scored 20% structural coverage and 100% false
structure on this fixture — it found 13 boxes across six frames, barely two per frame. Fixing
latency would raise its score without making it useful. The question "is GDINO fast enough"
is worth answering *after* something makes it look worth accelerating.

**4. OmniParser `icon_caption` — deferred, and for a subtle reason.**

This looks like the obvious move: it is the other half of the model already running, it is
MIT, and it describes what elements *do*. But it does not solve the blocker. A caption gives
a *label*; the privacy allowlist gates on *role*. Captioning an `icon` yields a withheld
label attached to a still-unnameable role. It becomes valuable **after** a detector produces
nameable roles, not before.

**5. Florence-2 standalone, OWLv2 — wait.** OWLv2 duplicates a question already answered by
Grounding DINO. Florence-2 is worth revisiting once roles are solved, as a naming layer.

**6. Qwen2.5-VL, Molmo, SAM2 — wait, or never.** The first two cannot meet a 500ms sampling
budget on any hardware likely to be present, and passive observation samples continuously.
SAM2 produces excellent masks with no semantics; it cannot supply a role, which is the one
thing needed.

---

## 5. The recommendation

**Implement one UI-element detector as the next challenger.** Prefer a permissively licensed
YOLO-family model exporting to ONNX with a genuine class vocabulary, so it drops into the
existing plugin unchanged.

Success is measurable on the corpus that already exists:

| what to watch | current | success looks like |
|---|---:|---|
| nameable-role coverage | **0%** | > 25% |
| anonymous ratio | 100% | < 60% |
| structural persistence | 22% | > 50% |
| false structure | 100% | < 50% |
| median latency | 109ms | ≤ ~200ms |

One command produces the comparison:

```
director benchmark-vision --fixture fixtures/vision/rocketleague
```

### One benchmark change to make first

Add a **nameable-role coverage** metric — the share of detections whose role appears in the
privacy plaintext allowlist — and give it weight. The current `structural coverage` says 100%
for a detector that can never name anything, which is how the blind spot survived three
milestones. Without this metric the experiment above cannot be scored properly.

That is perhaps twenty minutes of work and it is a prerequisite, not a nicety.

---

## 6. If the experiment fails

If a purpose-built UI detector also produces mostly anonymous, non-persistent boxes on game
interfaces, the honest conclusion is that **game UI is not desktop UI** — these models are
trained on web and office screenshots, and a stylised game HUD is out of distribution for all
of them. In that case the ranking inverts: classical CV becomes the primary path, because
geometry does not care what a widget was trained on, and the effort moves to making its role
assignment evidence-based rather than heuristic.

That would also be a real finding, arrived at cheaply.

---

## Sources

- [microsoft/OmniParser](https://github.com/microsoft/OmniParser) — model composition
- [Icon Detection and Captioning Models — DeepWiki](https://deepwiki.com/microsoft/OmniParser/2.2-icon-detection-and-captioning-models)
- [microsoft/OmniParser-v2.0 — Hugging Face](https://huggingface.co/microsoft/OmniParser-v2.0)
- [OmniParser V2 — Microsoft Foundry Labs](https://labs.ai.azure.com/innovations/omniparserv2/) — latency figures
- [Ultralytics License](https://www.ultralytics.com/license) — AGPL-3.0 terms
- [YOLOv8 Model License — Roboflow](https://roboflow.com/model-licenses/yolov8)
- [macpaw-research/yolov11l-ui-elements-detection](https://huggingface.co/macpaw-research/yolov11l-ui-elements-detection)
- [IndextDataLab/windows-ui-locator](https://huggingface.co/IndextDataLab/windows-ui-locator)
- [neovateai/UI-UG-7B](https://huggingface.co/neovateai/UI-UG-7B) — Qwen2.5-VL-based UI model
