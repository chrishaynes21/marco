---
type: decision
status: accepted
date: 2026-09-07
affects:
  - passive-observation
  - vision
source_paths:
  - cmd/director/escalationwiring.go
  - cmd/director/observewiring.go
  - internal/director/perception/providers/vision/provider.go
  - cmd/director/visiondiag.go
---

# ADR-129 — a repair may read what it was bought to read

## Context

[[ADR-118-a-reading-can-be-read-and-still-say-nothing]] gave a semantically silent screen one bounded
extra pass per settled screen state. It was the right bound and it did not work, for a reason
no test could see: whether that one pass could READ was decided somewhere else entirely.

`liveSampler.request` nested the reading budget inside the escalation gate:

```go
if s.rt.moreEvidenceIsWorthBuying() {
    out = observation.WithVision(nil)
    if s.labels && req.ReadLabels {     // <- the session's LabelEvery cadence
        out = observation.WithPixels(nil)
    }
}
```

`req.ReadLabels` is the label cadence — every sixth sample, and it knows nothing about repairs.
So the single expensive token bought BECAUSE structured perception could not say which state a
screen is had roughly a `1/LabelEvery` chance of performing the reading that is the only thing
that could say. The rest of the time it was spent blind, and the screen stayed nameless until it
changed.

Both halves were individually correct, which is why the suite was green. The budget spends once
per settled screen; the request does nest pixels inside the gate. The defect lived only in the
seam, and every existing test of that gate called it DIRECTLY — the same shape as the cycle-lock
defect, where the rule every test proved was right and the place production asked it from was
never entered. See `TestTheSensorGateDoesNotReenterTheCycleLock`.

## What the measurement said

Epic Games Launcher, whose accessibility tree exposes **one** element and reports `SHALLOW` /
`BLIND` / `0 named clickable controls`:

```
14 vision passes      31 structural controls every pass, byte-identical
 4 reading passes     the same 12 labels, same boxes, same confidences
```

The detector is deterministic. The reading is deterministic. A pass carrying the reading budget
names eight controls:

```
button  "Buy Now"     menu "Store"    menu "Library"
menu    "Unreal Engine"   menu "Fab"  button "Gift"
menu    "Hogwarts Legacy"  menu "Definitely Not Fried Chicke"
```

Three hypotheses were falsified on the way and are recorded because each would have been an
expensive wrong turn:

- **the detector is unstable** — no. Ten passes, zero bounds drift, zero class variance.
- **OCR and vision must be made to coincide** — no. `labelsOver` reads inside an accepted
  control's own box on the SAME captured image. The naming path was always same-pass, and a
  cross-sample correspondence layer would have been built to solve a problem that does not
  exist, against the "a remembered Target is not a visible Target" boundary.
- **provenance is refusing the labels** — no. The quarantine is the STANDALONE OCR provider,
  which `WithPixels` also enables and which the standalone-text rule correctly refuses. It is
  not the naming path.

The World's six anonymous shapes at 12% presence were never flicker. They were the budget.

## Decision

The repair carries the reading budget. `moreEvidenceIsWorthBuying` returns an
`evidencePurchase{Buy, Read}` rather than a boolean, and `Read` is set **only** for the bounded
repair a semantically silent screen earns.

Not for ignorance. That arm — no registry, or nothing placed — is unbounded by construction: an
unplaced reading is unplaced again next sample and the one after, so a reading budget attached to
it never stops. That is the continuous OCR this whole design refuses, and it is exactly what a
repair-shaped fix turns into if the two arms are conflated.

This is not "OCR more often". It is the same one bounded purchase per settled screen state, spent
on a pass capable of answering the question it was bought for. Ordinary sampling is untouched and
still follows `LabelEvery`.

## Consequences

- A semantically silent screen gets one pass that can name its controls, once, per settled screen
  state. On an accessibility-blind application that is the difference between 31 anonymous boxes
  and `button "Buy Now"`.
- It costs the reading — measured at 1.7s over 31 controls on Epic — once per settled screen.
  Ugly, and bounded; latency is a later question.
- Ignorance still buys a look and never a reading.
- Nothing about provenance, the standalone-text rule, thresholds, Fusion, Place identity or
  durable affordances moved.

## Enforced by

- `TestTheSemanticRepairCanReadWhatItWasBoughtToRead` (`cmd/director`) — drives a real registry
  and a real session through the production `request`, and asserts its premises rather than
  assuming them.
- `TestIgnoranceBuysALookAndNeverAReading` (`cmd/director`) — every ignorant branch, because they
  are separate returns and a mutation teaching one of them to read survived a test that reached
  only another.
- `TestOneSilentScreenBuysOneRepair` and `TestTheRepairBudgetIsOnePerSettledScreen` — the bound
  is unchanged.
- `TestPinningDoesNotChangeWhichProvidersRun` — ordinary non-label sampling still buys no OCR.

## Diagnostics this rests on

`director vision --read-labels` buys the reading budget by hand, and `Candidate.Label` records
what the reading made of each box. Without them a report could say a control was accepted but
never what it turned out to say — which is the one thing a naming question needs, and the reason
this defect read for two milestones as a detector that finds shapes and no words.

Enforced by `TestTheDiagnosticSaysWhatTheReadingMadeOfEachBox`,
`TestADetectOnlyPassLeavesEveryCandidateUnread` (vision) and
`TestTheReadLabelsFlagReachesThePixelPass` (`cmd/director`).
