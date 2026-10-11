---
type: decision
status: accepted
date: 2026-10-09
affects:
  - passive-observation
  - vision
source_paths:
  - internal/director/perception/providers/vision/shape.go
  - internal/director/observe/shape.go
  - internal/director/observe/interface.go
  - cmd/director/shadowwiring.go
  - cmd/director/shadowsample.go
---

# ADR-154 — geometry is evidence even when the words are not

## Context

[[Experiment-024-watching-a-player-navigate-a-real-menu]] put a controller in a player's hands
in front of a real Trackmania menu. Every press was observed. The menu never was.

Across the 150 seconds of navigation the provider admitted **nothing on 56 of 72 looks and a
single button on the other 16**, so the session could not name a list and never reported a
selected entry. Two readings taken immediately afterwards say why:

```
considered 77   accepted 1   rejected 76
   nothing readable inside 72
text 70   button 3   image 2   radio 1
```

The detector finds about seventy boxes a frame on that game's screens. Nearly all come back
classed `text`, and a `text` box whose words nobody could read was discarded, on this
reasoning, written in `vision/provider.go`:

> There is no evidence here to report: "an image is at these coordinates" is not something
> anything downstream can use.

**That was true when it was written and stopped being true when `selection.Measure` landed.**
The selection reader takes a rectangle and returns how it is painted. It never needs the words.
So the rows were being found, classified, and thrown away at the one gate they could not pass —
readability — before the measurement that could have used them ever ran.

## Decision

### The geometry is kept, as a different kind of thing

`vision.Shape` is a rectangle, the detector's confidence in it, how it is shaded, and whether
anybody got words out of it. There is no field for a role, a label, a name or a native id, and
that absence is the safety argument: **a shape cannot carry what would let a consumer mistake it
for a control.** `TestAKeptShapeCarriesNothingThatCouldNameAControl` asserts the absence over
the type rather than over an instance.

Shapes travel on the provider's own `Diagnostics`, which is the channel the label budget
already travels on, and **never in `ProviderOutcome.Observations`**. Fusion reads observations.
There is no path from here into belief, World State, or actionability
([[ADR-004-vision-cannot-establish-actionability]]).

### On the experiment's instance only

`KeepUnreadableShapes` is false by default and set in `newShadowProvider` and nowhere else. The
authoritative provider is a different object with different thresholds and the flag unset, so
its text acceptance and everything fusion does with it are exactly as they were — which is the
condition this repair was authorised under. The experiment's instance is additionally wrapped in
a `shadow.Provider`, whose evidence the collector routes to `Cycle.Shadow`, a field
`Admitted()` never reads.

### Kept whatever happens to the words

A shape is recorded for every `text` detection that cleared the geometry and confidence gates,
including the ones that were read. The channel must not appear and disappear with the label
budget: on the measured session that budget bought a reading on roughly **one sample in six**,
and geometry that existed only on those samples would be a cadence rather than a signal.
`Shape.Read` carries the difference.

`image` is excluded, deliberately. A `text` box is somewhere something wrote something, so a
row of them at uniform spacing is an argument about arrangement; an `image` box has no rows in
it by construction, and measuring one would invite exactly the inference this ADR refuses.

### A rectangle is not a menu entry, and the report never lets that be forgotten

The warning attached to the authorisation, in the words it was given in:

> a region being detected is not proof that it is a menu entry … preserving 70 unreadable text
> regions could increase ambiguity rather than solve it.

So the two are counted apart, everywhere, forever:

- `ShadowTotals.Shapes` and `ShapesShaded` are their own numbers. `Detections` still means what
  it meant, and a session whose every box was nameless reports `detections 0 total`.
- `InterfaceBelief.FromShapes` says how many of the belief's entries are rectangles nobody could
  name. Equal to `Entries` means nothing in the list has a name, and the report prints
  *"ALL of them"*.
- The belief's `Because` sentence says it too, because a reader told "entry 3 of 9 is selected"
  who has to go hunting through a payload to discover that none of the nine has a name has been
  misled by omission.

### Shapes do not reach screen-state segmentation

The sharpest form of the same warning. A screen state is a composition of named roles. Letting
seventy nameless rectangles into it would make every frame a new composition, destroy screen
identity, and produce a transition per look — which is precisely the failure
[[ADR-151-a-screen-that-changed-for-one-frame-did-not-change]] exists to stop, arriving by a
different door. `TestUnnamedGeometryDoesNotChangeTheScreenComposition` compares a session with
shapes against the same session without them and requires the same states and the same
crossings.

What shapes DO reach is the grouping inside `InterfaceWatch`, on equal terms with named regions
— because that layer's questions are "are these arranged like a list" and "is one of them
painted unlike its neighbours", and neither has ever needed a word. Equal terms in the
question; not equal standing in the answer.

### An unmeasured rectangle is not a dark row

`ShadowShape.Shaded()` gates every use. A rectangle with no measurement is dropped exactly as
an unshaded region already was: "nobody measured this" and "this is the darkest thing on screen"
are different answers, and defaulting the first to zero makes them the same one.

## The OCR provenance failures are a different defect

Reported here so the two are not conflated, and **not fixed**.

The same session reported `27 of 94 samples refused evidence (2317 observations quarantined)`,
refused provider `ocr`. The cause is structural: `ocr.Provider` implements
`observation.Provider` and not `TargetedProvider`, so the collector wraps it with an unknown
observed target — "nothing is inferred on its behalf and nothing is assumed about it" — and on
a targeted cycle the guard refuses everything it produced. The words were read, and then
discarded for being unable to say which window they came from.

Making OCR a targeted provider would ADMIT those 2317 observations, which is a change to what
authoritative text evidence fusion believes, and is outside what was authorised.

What is fixed is the reporting: the refusal printed an **empty reason**, so a reader could not
tell a missing engine from an engine whose evidence did not describe the window.
`provenanceReason` composes the sentence from the two generations the guard actually compared,
and distinguishes "established no target" from "observed a different one".

## Addendum, 2026-10-10 — a session can now say where the detector's output went

Found while preparing the live run that is meant to test this decision, and it is a defect in
the decision's own checkability rather than in the repair.

This ADR rests on a number the session does not record. *"The detector finds about seventy
boxes on this game's screens, nearly all classed `text`, and a `text` box it could not read
itself is discarded"* — that came from two `director vision` readings taken by hand after
Experiment-024 finished: `considered 77, accepted 1, rejected 76`. A different sample, of a
different screen, at a different moment, under the run's central conclusion.

The session's own record carried only what survived admission. So `detections 0` on a look had
two explanations and no way to choose between them: the detector found nothing, or it found
seventy boxes and refused every one. Those call for opposite conclusions, and the next run is
specifically a test of which one is happening.

`ShadowAdmission` now rides on the shadow sample beside `ShadowLabels`, accumulates onto the
totals, and prints under the detections line:

```
  detections   0 total   0 nameable   0 unmapped
  admission    76 proposed   0 accepted   76 refused
               refused: 72 nothing readable inside   4 too uncertain
```

It carries counts the provider already computes and resets per pass; no behaviour changed, and
nothing new crosses the privacy boundary — a refusal count names no window, no label and no
text. It reaches the session the way the label budget does, through `Inner().LastDiagnostics()`
at the one call site where the provider and the sample are both in scope.

**This makes the decision's three outcomes distinguishable in one line**, which is what it
could not do before:

| what the run shows | what it means |
| --- | --- |
| `Unreadable` high, geometry kept | the repair is working |
| `Unreadable` high, geometry 0 | the repair is not reaching this path |
| `proposed` near 0 | neither — the detector saw nothing, which is a finding about the model or the capture and not about admission at all |

## Enforced by

- `TestTheSessionSaysWhereTheDetectorsOutputWent` (`cmd/director`) — the admission account
  reaches the sample through `liveSampler.Sample` AND is folded into the session through
  `ShadowTotals.Observe`, on the measured fixture of nine unreadable rows. Both halves
  mutation-checked, because a count attached to every sample that nothing accumulates is the
  shape of eight recorded cases in this repository.
- `TestTheReportSaysWhereTheDetectorsOutputWent` — it is printed, with the refusal kinds named
  rather than summed, and a session nobody asked a detector prints no refusal rate at all.
- `TestAnUnreadableTextRegionKeepsItsGeometry` — the case the repair exists for, with the
  counter asserted unchanged: what was refused as evidence has not changed at all.
- `TestTheAuthoritativeProviderStillDiscardsUnreadableText` — the condition of the
  authorisation, same detector and same frame with the flag unset.
- `TestAKeptShapeCarriesNothingThatCouldNameAControl` — over the type, not an instance.
- `TestAnImageRegionIsNotKept`, `TestGeometryThatFailedTheFloorsIsNotKept` — only valid `text`.
- `TestReadableTextKeepsItsGeometryAndSaysItWasRead` — independence from the label budget.
- `TestKeptGeometryIsBounded` — a per-cycle diagnostic must not become a payload.
- `TestTheExperimentKeepsGeometryItCannotRead` (`cmd/director`) — through `newShadowProvider`,
  with `vision.New`'s default asserted beside it.
- `TestTheSampleCarriesGeometryTheDetectorCouldNotName` — through `liveSampler.Sample`, with the
  regions required to be normalised and the lit row required to survive as shading.
- `TestAListOfUnnamedRowsCanStillBeWatched` — a menu of nothing but rectangles, watched, with
  the mark read.
- `TestUnnamedGeometryIsNotCountedAsADetection`,
  `TestUnnamedGeometryDoesNotChangeTheScreenComposition`,
  `TestAnUnmeasuredShapeIsNotADarkRow`, `TestNamedAndUnnamedGeometryFromOneLookAreOneLook`.
- `TestTheReportCountsUnnamedGeometryApart`, `TestTheReportSaysWhenAListHasNoNames`.
- `TestARefusedProviderSaysWhyItWasRefused`, `TestAProviderThatObservedAnotherWindowSaysSo`,
  `TestAProvidersOwnReasonSurvives`, `TestAProviderThatProducedNothingIsNotARefusal`
  (`observesession`) — the separate OCR finding's reporting half.
