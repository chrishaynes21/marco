---
type: experiment
status: complete
date: 2026-08-28
backend:
  - screenparser
  - production-fusion
fixture: fixtures/perception/desktop
result: screenparser-adds-nothing-to-accessible-desktop-perception
supersedes: []
source_paths:
  - cmd/director/capturedesktop.go
  - cmd/director/comparedesktop.go
  - cmd/director/redactdesktop.go
  - cmd/director/desktopcorpus_test.go
  - fixtures/perception/desktop/browser-fixture.html
---

# Experiment 016 — what ScreenParser adds to desktop perception

## Question

[[Experiment-015-screenparser-admission-measured]] asked whether ScreenParser beats the
current detector on game frames and got a clear yes. That answer does not transfer, and it
was never supposed to: a game has no accessibility tree, so on a game frame **any** detector
beats nothing.

The desktop is the opposite case. Accessibility is present, rich, labelled, and legally
attributed. So the question here is narrower and harder:

> Of the semantic items on a real desktop screen, how many does production perception miss,
> and how many of those does ScreenParser recover?

## Method

Six coherent desktop moments, each captured as **one** screenshot plus **one** production
fused world, pinned to a single window and bracketed by a re-acquisition either side so that
incoherence is recorded rather than assumed away (`director capture-desktop-sample`). Three
UI families, with two reflow pairs:

| sample | size | what it is |
|---|---|---|
| `browser-fixture-wide` / `-narrow` | 1294×703 / 720×900 | a synthetic settings-shaped page, authored for this measurement |
| `settings-mouse-wide` / `-narrow` | 1216×941 / 700×941 | one Windows Settings Place, two layouts |
| `settings-bluetooth-wide` | 1216×941 | a second Settings Place |
| `explorer-fixture-wide` | 1100×800 | Explorer over a synthetic fixture directory |

Plus a stability set: three unchanged readings each of Settings and Explorer.

`fixtures/perception/desktop/browser-fixture.html` is synthetic, network-free and contains
two items planted to be got wrong:

- `#disabled-action` — a real `<button>` that is `disabled`
- `#looks-like-a-button` — a `.badge` span styled **identically** to the buttons beside it,
  and not interactive at all

`director compare-desktop-perception` then runs ScreenParser over each frame and counts
against production's reading in the same coordinate space.

### Annotation, honestly

The roadmap asked for hand annotation of a small set. What was actually done is narrower and
should be read as such: the browser fixture's ground truth is **known by authorship** rather
than annotated by eye, and the Settings and Explorer frames were read from the screenshots.
There is no independent annotator, and n=6. Every ratio below is descriptive of this corpus
and nothing wider.

## Result

### The corpus

| sample | prod | actionable | dets | matched | det-only | prod-only |
|---|---|---|---|---|---|---|
| browser-fixture-narrow | 85 | 36 | 79 | 28 | 51 | 59 |
| browser-fixture-wide | 79 | 33 | 66 | 27 | 39 | 53 |
| explorer-fixture-wide | 140 | 82 | 94 | 31 | 63 | 115 |
| settings-bluetooth-wide | 95 | 31 | 90 | 31 | 59 | 67 |
| settings-mouse-narrow | 54 | 22 | 56 | 20 | 36 | 35 |
| settings-mouse-wide | 84 | 34 | 88 | 34 | 54 | 57 |
| **TOTAL** | **537** | **238** | **473** | **171** | **302** | **386** |

At IoU ≥ 0.5, 36% of detections land on something production already knew about.

### ADDITIVE_RECALL: the 302 "candidate additions" are not additions

The headline number looks generous to the detector — 64% of its detections have no production
element under them. It does not survive being read.

**All 302 have their centre inside an element production already perceived.** Not one
detection in the corpus points at a region production had nothing for.

That alone would be too clean to trust, so the containment was measured rather than asserted:

- median containing-element area is **9.0×** the detection
- 61 of 302 are tight (≤4×) — genuine part-of relationships: the icon in a nav row, the
  toggle in a settings row, the chevron in a combo box
- 145 are loose (>10×), where containment is weak evidence on its own

Narrowing to the fairest possible reading — a detection whose only container is an
**unlabelled, non-actionable** region, i.e. somewhere production knows merely as
undifferentiated structure — leaves **35 of 302 (12%)**, with a maximum confidence of 0.49
and most below 0.32. Reading them one by one: row icons, a breadcrumb, a scroller boundary.

And they are boxes, not meanings. The detector says `text`, not what the text says. A box
labelled `text` over a region production reads as an unlabelled `unknown` is not knowledge
Marco can act on; it is a rectangle.

**ADDITIVE_RECALL on this corpus is effectively zero.** There is no set of human-visible
semantic items that production misses and ScreenParser recovers.

### The most confident unique finding is a false control

The highest-confidence detector-only box in the entire corpus — **0.63** in the narrow frame,
0.54 in the wide — is `#looks-like-a-button`. The `.badge` span. The trap.

ScreenParser calls it a `button`. Production calls it `text`, `actionable=false`.

Production also reads the other trap correctly: `#disabled-action` comes back
`role=button, enabled=false, actionable=false, affords=false`. Disabled state is not visible
in pixels in any reliable way, and no detector class encodes it.

So on the one item in this corpus where visual and semantic truth disagree, the detector is
confidently wrong and production is right. This is the concrete case
[[ADR-101-visual-presence-is-not-legal-actionability]] was written against, now measured
rather than reasoned about.

### The performance gate, taken

37A and 37B both listed production perception timings and both skipped them. Measured here:

| | Collect | Fuse | ScreenParser detect |
|---|---|---|---|
| Settings (102 elements) | 104–120ms | **0–1ms** | 645–872ms |
| Explorer (140 elements) | 1489–1515ms | **0–1ms** | 650–1379ms |

**Fusion is free.** The entire cost of production perception is the accessibility walk, and
on Explorer's tree that walk is ~1.5s — an order of magnitude above everything else, and the
real performance problem in the stack. It is not fusion, and it is not the detector.

On Settings, a ScreenParser pass costs roughly **7× the whole production perception** and
adds nothing.

### Stability

Zero jitter, both layers, three unchanged readings each:

- production elements: Settings 102/102/102, Explorer 140/140/140
- detections: Settings 87/87/87, Explorer 77/77/77

Neither layer wobbles on an unchanged screen, so stability does not discriminate between them.

### Reflow — evidence for 35D

The Settings pair is one Place at two widths: **84 elements wide, 54 narrow**. 36% of the
reading disappears on resize. The browser pair holds structure but not visibility: 90 elements
both ways, 79 visible wide against 85 narrow.

So element count is not a Place identity signal, and a narrow layout is not a partial reading
of the wide one — it is a different reading of the same Place.

## What this does not say

- It does not say ScreenParser is bad. [[Experiment-015-screenparser-admission-measured]]
  stands: on game frames, where there is no accessibility tree, it is the strongest detector
  measured.
- It does not generalise past six frames of three applications, all with healthy
  accessibility. The interesting case — an application whose accessibility is **absent or
  broken** — is not in this corpus, because the brief forbade breaking accessibility to
  manufacture one.
- It does not measure OCR. A detector that read the text inside its boxes would be answering
  a different question, and might answer it well.

## Decision

**37C_DECISION: SCREENPARSER_STRONG_DEGRADED_REPAIR_CANDIDATE.**

Both halves are needed, and neither alone is honest:

- On **accessible** desktop UI, ScreenParser is `NOT_USEFUL_ENOUGH`. It adds no actionable
  semantic item, costs several times the perception it would supplement, and its most
  confident unique claim in this corpus is a control that does not exist.
- Where accessibility is **absent**, Experiment 015 already showed it is strong.

Those are the same finding seen from two sides: ScreenParser's value is a function of what
accessibility is not providing. That is a repair signal, not a perception layer.

So it stays shadow-only, and the next question is not "admit it" but "detect degradation" —
which is [[ADR-102-a-detector-earns-its-place-where-perception-fails]].

## Enforced by

- `cmd/director` `TestTheCommittedCorpusCarriesNoPersonalInformation` — the committed corpus
  carries no email-shaped string
- `cmd/director` `TestEveryCommittedSampleIsOneMoment` — every sample is coherent and shows
  its gap
- `cmd/director` `TestNoCommittedElementIsOutsideTheFrame` — nothing is scored that was never
  drawn
- `cmd/director` `TestFusionIsTheOnlyDoorFromSensorsToBelief` — the corpus capture is named as
  the fifth `Fuse` site, deliberately

## Privacy

Captured from a real desktop, so the corpus was redacted before it was committed
(`director redact-desktop-sample`, geometric — see `redactdesktop.go` for why it is not
name-based):

- two Settings frames carried an account header (name and email) — blacked out, 4 labels
  cleared each
- the Explorer frame showed the user's home-folder tree — blacked out, 24 labels cleared per
  sample
- 79 elements across the corpus sat wholly **outside** the frame, including a nav pane at
  y = −1.95 of the window height; dropped and counted, which is a privacy fix and a
  measurement fix at once

The judgement calls are recorded in each sample's `redactions` array rather than left
implicit.
