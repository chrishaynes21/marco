---
type: adr
status: accepted
date: 2026-10-18
decision: "ACCEPTED — a workflow derived from captures rather than from a demonstration must say what links its two states to its control, and may not infer that link from the difference between them. Within ONE recording the link is measured: the observed input timeline must hold a press inside the control's rectangle between the two looks, or the derivation is refused. Across TWO recordings there is no shared timeline, the question cannot be asked, and an explicit assertion is required and written onto the step. The four perception terms — positive, persistent, independent, stationary — are necessary and are not sufficient; and there is now exactly one implementation of each, used by the live correlator, the live verifier and the offline builder alike."
supersedes: []
affects:
  - perception
  - learning
  - execution
source_paths:
  - cmd/perceptionreplay/fixture.go
  - internal/director/observe/outcome.go
  - docs/experiments/Experiment-089-four-terms-passed-and-nobody-pressed-the-button.md
---

# ADR-237 — an outcome is only an outcome of something

## The defect

`perceptionreplay -fixture` derives a one-step workflow from two captures of an application so
that the pixel execution path can be tested without a demonstration. Pointed at the owner's
Trackmania recording, it produced a workflow saying that pressing SETTINGS yields twelve
words — `resume`, `author`, `gold`, `select`, `back` and eight others.

Nobody pressed SETTINGS. The recording is keyboard-only: 31 observed inputs, not one of them a
press. The two looks were the same pause menu with the highlight bar on a different row, and a
row whose text stops being inverted becomes readable.

All four of the outcome terms passed. The twelve words were positive, they stood on every one of
five readings, none came from inside the control, and the before window held still — 0
transient, 0 of 1 windows moved. [[Experiment-089-four-terms-passed-and-nobody-pressed-the-button]]
has the measurements.

## Why no perception term fixes it

Because the missing fact is not about pixels. Every term in
[[Experiment-087-the-panel-changed-and-the-verifier-could-say-so]] asks whether a difference
between two states is a STANDING, independent, positive difference. None of them asks whether
the control had anything to do with it, and on a screen where selection changes what OCR can
read, the answer can be no while every term says yes.

Tightening a term would not have helped and would have cost the real outcomes: the Ark
expectation this same rule derives — 36 words, the GRAPHICS panel — is a genuine destination and
would fail any threshold raised high enough to exclude twelve persistent words.

## The decision

An outcome is only an outcome OF something, and a derivation must say what.

**Within one recording the link is measured.** `capture-game-session` writes an observed input
timeline beside the frames. A derivation from two looks of one recording requires a `point`
input inside the control's own rectangle, with its timestamp between the two looks' timestamps.
Ark's capture carries `{"intent":"point","at_ms":24892,"where":{"x":0.414,"y":0.261}}`, inside
the GRAPHICS rectangle at `0.410,0.241` and between look 245 at 24501ms and look 262 at
26200ms. Trackmania's carries no press at all and is refused, naming the counts.

This is the owner's own rule about demonstrated coordinates, applied in the direction it
permits:

> A demonstrated coordinate is authoritative evidence of where the person acted. It must not
> become an unconditional coordinate replay instruction.

The press is read as evidence of causation. It never becomes the replay target: the step still
carries the control's measured rectangle, resolved against the live screen, and the saved
sentence says so out loud.

**Across two recordings the link is asserted, explicitly.** Two captures share no timeline and
the question cannot be asked. The controlled test menu is deliberately this shape — it serves
its before and after states from a URL so that nobody has to click to capture either one — so
the link there is somebody's word. `-fixture-asserted` is then REQUIRED, and what it says is
written onto the step beside the expectation. A causal claim resting on an assertion must name
whose assertion, in the artifact, where whoever reads the verdict will see it. Silence is
refused rather than taken as consent.

**And the artifact stops overclaiming in the other direction too.** The Ark fixture was labelled
"CONSTRUCTED, no person demonstrated this". That was wrong in the other direction: a person DID press that
control and the recorder saw it. The step now says ASSEMBLED — not acquired by
the live correlator — and names which of the two links it rests on.

## And one rule, not two copies of it

The builder had hand-rolled the independence and persistence terms rather than calling the
production ones, and both hand-rolled versions were wrong: 37 words against the correlator's 36
on the same capture, and a single look per side where persistence needs a window. The
independence term is now exported as `observe.Independent`, the builder measures over windows
of `observe.OutcomeSettleLooks` readings, and the live correlator, the live verifier and the
offline builder call the same three functions.

Two paths that disagree about what an outcome IS will eventually disagree about one that
matters.

## What this costs

The Trackmania workflow does not exist and cannot be derived from either Trackmania capture.
That is the correct outcome and it is a real cost: the live pixel test's real-game level still
rests on Ark, and a Trackmania-scoped artifact needs a recording in which somebody presses
something.

## Enforced by

`cmd/perceptionreplay` `TestAnOutcomeNeedsAPressSeenInsideTheControl`,
`TestAPressInsideTheControlBetweenTheLooksIsTheAttribution`,
`TestAPressOutsideTheWindowOrOutsideTheControlDoesNotAttribute`,
`TestTwoSeparateRecordingsMustSayWhoAssertsTheLink` and
`TestAnAssembledFixtureRefusesWhatItCannotShow`, with
`TestADerivedOutcomeKeepsOnlyTheWordsThatSTOOD` and
`TestADerivedOutcomeDoesNotRestOnTheControlsOwnWords` holding the two terms that were
hand-rolled wrong.

## See also

- [[Experiment-089-four-terms-passed-and-nobody-pressed-the-button]]
- [[Experiment-087-the-panel-changed-and-the-verifier-could-say-so]]
- [[ADR-236-a-workflow-belongs-to-the-application-it-was-shown-in]]
- [[ADR-235-three-records-and-only-one-of-them-may-act]]
