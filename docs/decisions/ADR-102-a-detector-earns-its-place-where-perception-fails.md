---
type: decision
status: accepted
date: 2026-08-28
supersedes: []
affects:
  - perception
  - fusion
  - vision
source_paths:
  - cmd/director/comparedesktop.go
  - cmd/director/capturedesktop.go
  - pkg/directorapi/actionability.go
---

# ADR-102 — a detector earns its place where perception fails, not where it works

## Context

[[ADR-101-visual-presence-is-not-legal-actionability]] built the firewall: a detection
describes pixels, and pixels do not confer the right to act. That settled what a detection
may **become**. It did not settle when a detector should run at all.

The open question was admission. ScreenParser had won a benchmark
([[Experiment-015-screenparser-admission-measured]]) and the obvious next move was to admit it
to desktop perception and see what it added.

[[Experiment-016-desktop-perception-corpus]] measured that instead of assuming it. Over six
coherent desktop moments — 537 production elements, 473 detections:

- 64% of detections had no production element under them at IoU, and **all 302 of those** had
  their centre inside an element production already perceived
- the fairest additive reading — a detection whose only container is an unlabelled,
  non-actionable region — leaves 12%, at confidences topping out at 0.49, and they are boxes
  without meanings
- the single most confident unique detection in the corpus, at 0.63, is a `<span>` styled to
  look like a button and wired to nothing. Production reads it as `text`. The detector reads
  it as a control
- production reads a disabled button as disabled; no detector class encodes that
- fusion costs 0–1ms; a ScreenParser pass costs 645–1379ms

## Decision

**A detector is admitted by the ABSENCE of better evidence, never by its own quality.**

Concretely:

1. ScreenParser remains shadow-only on the desktop. Its measured additive contribution where
   accessibility is healthy is zero, and its cost is several times the perception it would
   supplement.

2. The question that governs admission is not "is this detector good?" but "is the evidence
   that would otherwise answer this missing?" A detector's value is a function of what the
   other sensors are not providing, so it is not a property of the detector and cannot be
   established by benchmarking the detector alone.

3. A benchmark win does not license admission. Experiment 015 and Experiment 016 used the same
   model and disagreed completely, because they asked about different surfaces. Any future
   result of the form "detector X scores well on corpus Y" answers nothing about admission
   until corpus Y is shown to resemble the surface where X would run.

4. Where a detector does run, [[ADR-101-visual-presence-is-not-legal-actionability]] still
   holds: it contributes description, never the right to act.

## Consequences

- The next perception question is **degradation detection** — recognising that accessibility
  has failed on a given window — not detector admission. Without that, there is no trigger
  that could ever admit a detector, and the strongest measured result we have
  (Experiment 015, on a surface with no accessibility at all) stays unreachable in production.
- The real performance problem in perception is now named and is not the detector: the
  accessibility walk costs ~1.5s on an Explorer tree, against 0–1ms for fusion.
- The corpus is committed, so a future detector can be measured against the same moments
  rather than a fresh capture that cannot be compared to this one.
- This forecloses a plausible and wrong move: admitting ScreenParser to desktop perception
  "since it's already built and it's good". It is good. It is good at a job the desktop does
  not currently have.

## Enforced by

- `cmd/director` `TestTheCommittedCorpusCarriesNoPersonalInformation`,
  `TestEveryCommittedSampleIsOneMoment` and `TestNoCommittedElementIsOutsideTheFrame` — the
  evidence this decision rests on stays intact, coherent, and scoreable
- `cmd/director` `TestFusionIsTheOnlyDoorFromSensorsToBelief` — the corpus capture reads
  production's own pipeline and is named as doing so, so the measurement cannot drift into
  being about a private second reading
- `pkg/directorapi` `TestVisualPresenceIsNotLegalActionability` — the firewall that makes shadow-only
  a property of the code rather than of the current configuration
