---
type: experiment
status: complete
date: 2026-08-09
backend:
  - tesseract
  - accessibility
game: none — desktop applications only
fixture: live capture, three 45s sessions, no user interaction
result: ocr-discriminator-insufficient
supersedes: []
source_paths:
  - internal/director/observe/terms.go
  - internal/director/observe/recall.go
  - internal/director/perception/providers/ocr
  - cmd/director/observewiring.go
---

# Experiment 009 — OCR contributed nothing, and two defects were found on the way

## Question

Cross-session semantic memory refuses to recognise a subject without a discriminator, and
interface terms are the only one most screens have. OCR was assumed to be the missing source.
Does it actually supply terms that structure alone cannot?

## Method

Tesseract was **already installed** at `C:\Program Files\Tesseract-OCR\tesseract.exe` (v5.4.0,
winget, `UB-Mannheim.TesseractOCR`) and simply not discoverable: not on `PATH`, and
`$MARCO_TESSERACT` unset. No installation was performed. The plugin discovers it by `PATH` or
that variable; absence is a fully supported state.

Three 45-second observation sessions against already-open desktop windows, **no user
interaction and no game**. ScreenParser in shadow throughout. The decisive one is an A/B on the
same application with OCR enabled and then disabled.

## Two defects found by auditing the path, not by a failing test

**Terms could never qualify in a live session.** Scoped OCR runs on a label pass — `sequence == 1
|| sequence%6 == 0` — so roughly one inference in six. The per-state term ratio divided by *every*
inference against a 0.50 threshold, so a term present on **every** reading scored ≈0.17 and was
always discarded. The semantic discriminator was structurally unreachable in production. Every
test that appeared to exercise it set the evidence on every sample directly.

**Unavailability was indistinguishable from absence.** `SemanticEvidence` had no way to say "could
not look". In `CompareStructure`, a remembered subject with terms compared against a session with
none produced `MatchDifferent` — so OCR being unavailable made Marco conclude the screen *differs*.

Both were fixed before measuring, because measuring through them would have measured the defects.
`SemanticEvidence.Observed` records that perception had text to classify; `ScreenState.TermObservations`
is the ratio denominator; `Fingerprint.TermsKnown` and `StructureSignature.TermsKnown` carry
UNKNOWN into matching, where unknown terms are no longer evidence in either direction.

## Result — OCR contributed zero terms

| session | terms | term observations | provenance |
|---|---|---|---|
| VS Code, OCR **on** | `back, notifications, search` | 13 / 19 samples | 4 samples refused, **1691 observations quarantined (ocr)** |
| VS Code, OCR **off** | `back, notifications, search` — **identical** | 16 / 24 samples | clean; every provider proved its target |
| Steam (roles, no accessible names) | **none**, `terms_known: false` | 0 / 9 samples | 2 refused, 784 quarantined (ocr) |

The A/B is the measurement that matters: **the same three terms with and without OCR**. Every term
came from accessibility. Enabling OCR did not add one, reduced the number of usable readings
(13 vs 16), and introduced provenance refusals.

Steam is the case where OCR was the *only* possible source — accessibility exposed 12 `button` and
51 `menu` roles with no readable names. OCR ran, accepted text, and still produced no term.

## Why, architecturally

OCR text becomes a label only through fusion's `TextFilledMissingLabel` rule — "the element is
structurally real but had no label; the contained text names it" — and the label is released in
plaintext only for the structural allowlist (`button`, `menu_item`, `menu`, `tab`, `checkbox`,
`radio`). So a term requires **accessibility to supply the structure and role** *and* text to land
on that element geometrically.

Where accessibility already supplies names, OCR is redundant. Where it supplies roles without
names — the case OCR exists for — the text did not attach, and a large share of it was quarantined
by the target-provenance guard before it could.

## Measured cost

`director ocr` on a real 1920×1080 window: capture 28ms, **recognise 731ms**, total 760ms.
ScreenParser measured 625ms median (p95 667ms) in the same sessions. Both at a 2s cadence is
roughly 1.4s of the 2s budget, and the OCR session recorded **11 late samples of 9 taken** against
**0 late of 24** without it.

## Conclusion

**`OCR_DISCRIMINATOR_INSUFFICIENT`.** Not because OCR fails to run — it runs, through production,
with its output correctly classified and its raw text correctly discarded — but because in both
applications measured it supplied no interface term that accessibility had not already supplied,
and none at all where accessibility supplied nothing.

The matcher was **not** loosened. The conservative rules stand: a subject with no discriminator
stays `candidate` and inherits nothing.

## What remains unmeasured

Cross-session recognition on live data. No real session produced a `supported` hypothesis, because
an idle window never *recurs* — recurrence needs a screen to appear, go away and come back, which
needs interaction. This was not requested of the user.

## Enforced by

`TestTheSamplerAsksForOCROnALabelPass`, `TestLabelsBecomeTermsAndReachTheDurableSignature`,
`TestRawLabelTextDiesAtTheClassificationBoundary`,
`TestASampleWithNoReadableLabelsReportsUnknownNotEmpty`. Four mutations verified: deleting the OCR
request, deleting the classification call, dropping terms from identity, and emptying classified
terms each fail a production-path test.
