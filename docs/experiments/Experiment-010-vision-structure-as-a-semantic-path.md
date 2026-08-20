---
type: experiment
status: complete
date: 2026-08-09
backend:
  - screenparser
  - tesseract
game: none — synthetic fixture plus one real captured panel
fixture: accessibility-poor synthetic screen; fixtures/vision/rocketleague/05-pause-panel.png
result: vision-semantic-path-proven
supersedes: []
source_paths:
  - internal/director/perception/providers/vision/label.go
  - internal/director/perception/providers/vision/vision.go
  - pkg/directorapi/nameability.go
  - cmd/director/shadowsample.go
  - cmd/director/shadowwiring.go
---

# Experiment 010 — giving vision-derived structure somewhere safe for a word to belong

## Question

[[Experiment-009-ocr-as-a-semantic-discriminator]] ended with `OCR_DISCRIMINATOR_INSUFFICIENT`:
OCR works, and every term Marco has ever classified came from accessibility. The failure was
upstream of OCR — text became a label only where accessibility had already supplied a
structurally real, nameable element.

The detector supplies exactly that missing half on exactly the surfaces accessibility cannot
see. Does connecting the two produce interface terms without accessibility, and are those terms
the discriminator [[ADR-016-cross-session-identity-is-structural-and-conservative]] needs?

## The seam, before anything was written

Traced end to end. Three links were missing and one had drifted:

- **The experiment had no reader.** `newShadowVision` built a `vision.Provider` and never set
  `Reader`, so scoped OCR never ran on a ScreenParser detection at all. The mechanism existed,
  fully tested, wired only into the authoritative provider.
- **The shadow representation had nowhere to put a word.** `ShadowRegion` is role, geometry,
  confidence and nameable — no label, no term.
- **Semantic classification read the authoritative side only.**
  `SemanticEvidenceFrom(sample.Entities)` runs over *fused* entities, and shadow evidence never
  reaches fusion by construction. Nothing shadow found could ever be classified.
- **Nameability was four allowlists.** The privacy classifier, the shadow diagnostic and the
  benchmark each kept their own copy; two carried a written rationale for being copies.

The reader itself needed nothing: `vision.LabelReader` and the crop-scale-read composition were
already correct and already measured. This milestone connected them and decided who may be named.

## Method

Deterministic first, as this repository's live-play rule requires. One synthetic
accessibility-poor fixture — a heading, four rendered controls, an icon, a panel — run through
the production composition: fake capture → registered detector → `newShadowProvider` → real
collector → real sampler → `ShadowSample` → `ShadowTotals` → hypotheses → `StructureSignature`.
Accessibility contributes nothing: the fusion engine in the fixture returns an empty world, so
`sample.Entities` is empty and any term must have come from the detector's own boxes.

Cost was measured against a **real** captured panel rather than the synthetic one.

## Result — the gain

| | structure | nameable | terms | observed |
|---|---|---|---|---|
| before (no reader, same fixture, same path) | 6 | 4 | none | **false** — unknown, not empty |
| after | 6 | 4 | `audio, back, controls, settings` | true |

Structure is **unchanged**. That is the invariant, not a coincidence: text enriches structure
and never creates or destroys it, and the test asserts the two counts are equal.

Where the budget went, on that pass:

| unsayable | attempted | control labels read | screen texts read | ambiguous | budget-skipped |
|---|---|---|---|---|---|
| 2 | 5 | 4 | 1 | 0 | 0 |

The icon and the panel cost **nothing**. Under the previous arrangement they would each have
cost a round trip and then been withheld by the classifier.

## Result — is it the discriminator?

Two settings screens with identical composition — `button:4, icon:1`, four members — and
different words:

| | verdict |
|---|---|
| structure only (terms stripped) | `candidate` |
| structure + vision-derived terms | **`different`** |

That is the false-merge case ADR-016 refuses to resolve on structure and has never had a way to
separate. It is now separated, on a surface with no accessibility tree.

| case | verdict | terms |
|---|---|---|
| A — same subject, two observations | `same` | `audio, back, settings` |
| B — similar structure, different words | `different` | `audio, back, settings` vs `back, display, settings` |
| C — one control **permanently** unreadable | `different` | `audio, back, settings` vs `audio, settings` |
| C' — one control unreadable on **1 pass in 3** | `same` | identical |

C and C' are the measurement that decides whether the exact-set rule is now the bottleneck, and
they answer it: **it is not**. The per-state term ratio absorbs intermittent evidence, which is
what it exists for. A term lost on *every* reading is a real difference in what the screen says,
and calling it one is correct rather than brittle.

**The matcher was not changed.** Measuring it and then adjusting it in the same milestone would
have been measuring the adjustment.

## Result — cost

Measured through the production reader over `fixtures/vision/rocketleague/05-pause-panel.png`
(400×270, real capture, tesseract v5.4.0, 3× upscale):

| | time | read |
|---|---|---|
| whole panel, 3× (1200×810) | 521ms | **0 spans** |
| scoped region (1200×141), median of 5 | **129ms** | see below |
| scoped, p95 / max | 136ms / 136ms | |
| five regions, total | 614ms | |

The whole-panel read returning **nothing** reproduces the original finding that made scoped
reading necessary, on a different fixture and two milestones later.

The regions here are arbitrary 47px bands rather than detected boxes, and the readings show it:
`EXIT TO MAIN MENU` came back exactly, while a band straddling two buttons produced
`US Gem EVE CETTIN(SS`. Both filters behaved — the second is refused by shape. This is a second
piece of evidence for the same rule: **the structure is what makes the reading trustworthy**, and
a crop that does not correspond to a control produces confident nonsense.

Budget arithmetic, from that number: 6 control labels + 2 screen texts = 8 readings ≈ **1.0s**,
on top of a ~0.9s ScreenParser inference, against a 2s cadence. A label pass will therefore
occasionally cost one skipped shadow slot. Skips were already counted; the label budget is now
counted beside them, per inference and per session.

## What is still not proven

- **No live run, and it is not currently possible on this machine.** `plugins/vision/models/`
  holds `icon_detect.onnx` and nothing else — there is no `screenparser-1280.onnx` and
  `$MARCO_SCREENPARSER_MODEL` is unset, so `newShadowVision` refuses before it starts. A live
  session on the shipping detector would produce zero terms, correctly and uninformatively,
  because every one of its detections is an `icon`. Nothing was asked of the user.
- **The shipping detector still cannot benefit.** One class, `icon`, unsayable — so on that
  model this path produces nothing. The blocker recorded in [[Vision]] is unchanged and is now
  *measured* by `LabelsUnsayable` rather than inferred.
- **Cross-session recognition on live data** remains unmeasured, for the reason
  Experiment-009 gave: an idle window never recurs.

## Conclusion

**`VISION_SEMANTIC_PATH_PROVEN`.** ScreenParser-derived structure becomes semantic without
accessibility; OCR cannot synthesize structure; the privacy and provenance boundaries hold; closed
terms reach state, hypothesis and durable signature; the gain is measured; and the
similar-structure discriminator case works.

## Enforced by

See the full list in
[[ADR-017-structure-earns-a-name-text-never-earns-structure]]. Eight mutations were verified to
fail a production-path test: deleting the shadow reader, removing the nameability gate on
reading, letting a read text region become an element, widening the role allowlist, collapsing
unknown into empty, bypassing the provenance gate, dropping terms on the way into the durable
signature, and disconnecting the association call while leaving every helper intact.
