---
type: experiment
status: complete
date: 2026-10-18
backend:
  - structure
game: trackmania
fixture: "`tm-menu-steps-sanitized` (864 frames, 31 observed inputs, OCR on every look — 10m22s) and `ark-menu-1` (300 frames, 4 observed inputs), both the owner's own captures, derived with `perceptionreplay -frames -fixture`."
result: "A derived workflow passed all four outcome terms — positive, persistent, independent, stationary — on two looks of a screen the fold measures as ONE state visited once, and said that pressing SETTINGS produces twelve words. Nobody pressed SETTINGS. No perception term can catch that, because the missing fact is attribution, not pixels: the recording's own observed input timeline says whether a press was seen inside the control between the two looks. Ark's says yes at 0.414,0.261 and 24892ms; Trackmania's holds 31 inputs and not one press. Trackmania is now refused."
supersedes: []
affects:
  - perception
  - learning
source_paths:
  - cmd/perceptionreplay/fixture.go
  - cmd/perceptionreplay/fixture_test.go
  - internal/director/observe/outcome.go
  - docs/experiments/Experiment-087-the-panel-changed-and-the-verifier-could-say-so.md
---

# Experiment 089 — four terms passed, and nobody pressed the button

## The question

The pixel execution path has never run, and running it needs an artifact. The owner moved the
live target to Trackmania and set the standard for what a test has to show:

> the test must confirm an actual input was emitted and the expected result was independently
> observed. A successful plan, matched control, or command exit code isn't enough.

Trackmania has two existing captures and both are keyboard-only, so neither can yield a
demonstrated click. `perceptionreplay -fixture` exists for exactly that gap: it derives a
one-step workflow from two captures by measurement, needing no demonstration. So — can it
derive a Trackmania-scoped artifact?

## What was measured

OCR over all 864 frames took 10m22s and produced a word cache for every look. The semantic fold
over that cache is unambiguous:

```
pixel-only    looks  with text
visual_1        862  visual_1 x862

visits per screen -- a screen entered, left and returned to must rejoin
visual_1        862       1   visual_1        862       1
```

One screen, 862 looks, one visit, under both the pixel fold and the richer text fold. The
capture is the in-race pause menu and its 36 annotated intervals are all the same screen with
only the highlight bar moving: `resume`, `change_opponents`, `add_favourite`, `scoreboard`,
`view_records`, `settings`, `report`, `ubisoft_connect`, `EXIT`. `SETTINGS` is read on 635 of
757 looks, so a control can be grounded; nothing in the recording ever opens anything.

The derivation should therefore have refused. It did not:

```
control       8.4%x2.0% at 0.202,0.707, holding exactly "SETTINGS"
outcome       12 word(s): 000 978 author back disliketrack est gold liketrack
              nafeneoe resume select siver
```

## Three things were wrong, and only the third mattered

**The independence term was missing.** The builder hand-rolled it and let through the control's
own label: on Ark it produced 37 words where the live correlator produced 36, the extra being
`graphics` — which the tab says and which the panel it opens also says, in *Advanced Graphics*
and *Graphics Presets*. One word of thirty-seven changed no verdict, and an expectation resting
by one word on the evidence the action site was approved from is still the wrong shape.

**The persistence term could not even be asked.** The builder compared ONE look per side. A word
read on a single look cannot be distinguished from a word that stood, so a half-drawn panel and
an OCR misread both read as a destination. Fixed by measuring over windows of
`OutcomeSettleLooks` readings and calling `observe.OutcomeBetween` and the newly exported
`observe.Independent` — the same functions the live correlator and the live verifier use.

**And then all four terms passed anyway.** Re-measured through the production rule, Trackmania
still produced twelve words, with **0 transient** and **0 of 1 windows moved**. They were not
flicker. They were real, persistent, independent words on a still screen — because a row whose
text stops being inverted when the highlight bar moves off it becomes readable, and stays
readable. Positive, persistent, independent, stationary, and false.

## The term that was actually missing

No perception term can fix this, because the missing fact is not about pixels. An outcome is
only an outcome OF something, and the difference between two looks of one recording may be
attributed to a control only if the recording saw somebody press it.

Which the recording knows. `capture-game-session` writes an observed input timeline beside the
frames, and the owner's rule for what a demonstrated coordinate is good for applies exactly:

> A demonstrated coordinate is authoritative evidence of where the person acted. It must not
> become an unconditional coordinate replay instruction.

So it is used as evidence of causation and never as a replay target. Measured:

| capture | presses | inside the control, between the looks | verdict |
|---|---|---|---|
| `ark-menu-1` | 3 `point` | `0.414,0.261` at 24892ms, in the GRAPHICS rectangle `0.410,0.241`+`0.072x0.028`, between look 245 (24501ms) and look 262 (26200ms) | derived, 36 words |
| `tm-menu-steps-sanitized` | 0 `point` of 31 inputs | none, and none possible | REFUSED |

Two shapes carry different evidence and must not be allowed to look alike, so there are two
modes. One recording has a timeline spanning both looks and the question has a measured answer.
Two separate recordings share no timeline and the question cannot be asked at all — which is
the controlled test menu's shape, since it serves its before and after states from a URL so
that nobody has to click to capture either one. There the link is somebody's word, and
`-fixture-asserted` is required and written onto the step, because a derivation whose causal
claim rests on an assertion must say whose, in the artifact, where whoever reads the verdict
will see it.

## What this costs the live plan

The Trackmania artifact does not exist and cannot be derived from either existing capture. The
three evidence levels are unchanged but their contents move:

| level | surface | what it rests on |
|---|---|---|
| controlled | the disposable HTML menu | two captures, an asserted link, and an independent witness in the window title |
| assembled | `ark-menu-1` | measured geometry, measured expectation, and an OBSERVED press inside the control |
| live | whatever is in front | the live correlator, a person's hand, and the guarded executor |

The Ark artifact is the stronger of the two offline ones and was mislabelled: it is not
"constructed, no person demonstrated it" but *assembled offline from a recording in which a
person did press that control*. The report and the step now say which of the two it is.

## Enforced by

`cmd/perceptionreplay` `TestAnOutcomeNeedsAPressSeenInsideTheControl`,
`TestAPressInsideTheControlBetweenTheLooksIsTheAttribution`,
`TestAPressOutsideTheWindowOrOutsideTheControlDoesNotAttribute`,
`TestTwoSeparateRecordingsMustSayWhoAssertsTheLink`,
`TestADerivedOutcomeKeepsOnlyTheWordsThatSTOOD`,
`TestADerivedOutcomeDoesNotRestOnTheControlsOwnWords`,
`TestADerivedOutcomeIsRefusedOnAScreenThatMovesByItself`,
`TestAnAssembledFixtureCarriesWhatWasMeasuredAndNothingElse` and
`TestAnAssembledFixtureRefusesWhatItCannotShow`.

Twenty-one mutations, all killed: every outcome term, every attribution bound, the control
search, the scope, and the one call site each gate hangs from.

## See also

- [[Experiment-087-the-panel-changed-and-the-verifier-could-say-so]] — the four terms
- [[Experiment-086-the-tab-changed-the-panel-and-not-the-screen]] — why a local outcome at all
- [[ADR-237-an-outcome-is-only-an-outcome-of-something]]
