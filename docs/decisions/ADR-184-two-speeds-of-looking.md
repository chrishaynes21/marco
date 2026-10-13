---
type: decision
status: accepted
date: 2026-10-13
affects:
  - passive-observation
  - perception
  - navigation
---

# ADR-184 — two speeds of looking

## Status: accepted and built, with one clause replaced before it was

The owner authorised the integration after reviewing
[[Experiment-030-asking-the-learned-detector-for-the-tile]], and rejected one clause of what is
written below:

> I'd approve implementing ADR-184's source-aware candidate bridge, with one important
> condition: the fast observer must never wait for ONNX.
>
> The proposed lifetime rule says a detection remains admissible while its screen remains
> settled. That's a useful starting point, but screen identity alone may be insufficient to
> validate a delayed detection. […] A return to the same arrangement should not automatically
> authorize reuse of an old rectangle.

That objection is correct and is the single most important thing on this page, so section 3
below has been rewritten rather than annotated — see **the observation generation**.
[[Experiment-031-the-workflow-gets-nine-steps-in]] is what the built version measured:
**the completed workflow prefix went from 1 of 21 to 9 of 21, with zero false matches and no
change in fold latency.**

## Context

### The measurement that sets the shape

| | per 1920x1080 frame |
|---|---|
| a whole fast look (capture, propose, screens, clicks) | 55-65ms |
| `structure.Propose` alone | ~37ms |
| ScreenParser inference | **569-746ms** measured in Experiment-030; 976ms median recorded on a Trackmania corpus |

The detector is 10 to 16 times a whole fast look. It cannot run in the loop, and no tuning changes
that by an order of magnitude. The owner reached the same conclusion from the architecture:

> The fast observer shouldn't have to run ONNX at 10 fps. It should recognize changes cheaply and
> request richer perception when needed.

### The thing that is already true and was not obvious

Marco has **two perception stacks**, and they do not touch.

```
THE DIRECTOR CYCLE                          THE FAST OBSERVER
capture                                     capture
  providers (accessibility, OCR,              structure.Propose  -> []observe.Region
             vision/ScreenParser,             observe.VisualScreens  (arrangement, settle)
             visualstate)                     observe.ClickTargets   (press -> target)
  fusion.Engine  -> directorapi.Element       workflow.Draft
  planner, policy, verification               ADR-181/182/183
```

The left stack is mature and **already has every discipline this ADR would otherwise have to
invent**: a source ladder (`directorapi.SourceRank`), a hard rule that vision and OCR can never
make a thing operable (`directorapi.ActuatingSource`), evidence that is never destroyed, conflicts
recorded rather than resolved, and `fusion/visual.go`'s rule in its own words — *pixels alone must
not create an actionable control*. It already runs the ScreenParser detector, at a frozen
calibration, with a shadow path. `providers/vision/structureshapes.go` already runs **both** cues
over one capture and already chose pixels as the default because of the latency above.

The right stack is where the product lives — demonstrate once, remember, reproduce — and it has
none of that, because it has never had more than one source. Its entire currency is
`fastobserve.Look.Regions []observe.Region`: a bare rectangle with no idea where it came from.

So the gap Experiment-028 found is not a missing capability. It is a missing **wire**, between a
stack that can see the tile and a stack that needs to.

## The minimal interface

Four parts. Three are small; the second is the only real design.

### 1. A rectangle that knows where it came from

The one type change the whole thing rests on. Today a proposal is an anonymous rectangle, so there
is nowhere to record that one of them came from a model — and a provenance that has nowhere to live
becomes a provenance nobody can check.

```go
// A proposal and the evidence for it. The rectangle is unchanged; what is added is
// an answer to "who says so".
type Candidate struct {
    Region Region
    // From is the ladder's own vocabulary, not a new one: SourceVision or the
    // proposer's own source. Carried so a press can be explained after the fact.
    From   directorapi.ObservationSource
    // Class is the detector's word for this — "image", "button", "menu". CARRIED,
    // NEVER READ by anything that decides. See the boundary rule below.
    Class  string
    // Score is the model's own confidence, or 0 for a pixel proposal, which has none.
    Score  float64
    // Look is the look this was computed FROM, which is not the look it arrives on.
    Look   int
}
```

`Look.Regions` becomes `Look.Candidates`, and `ClickTargets.Press` keeps taking the whole set and
keeps choosing the smallest containing the press. **The ranking rule does not change.** That is the
point of Experiment-030's geometry: the tile is 0.93x its true area and less than half the stadium
slab's, so simply being in the set is enough for it to win, and the slab never has to be suppressed.

### 2. A slow source that cannot stall the fast one

Not a second provider called per look — a request and a later answer.

```go
type SlowSource interface {
    // Request asks for richer perception of a look. Non-blocking, and free to drop:
    // a source already working on one must not queue another.
    Request(look int, frame image.Image)
    // Take returns whatever has arrived since the last call, or nil. Called on the
    // fast path, where it must cost approximately nothing.
    Take() []Candidate
}
```

**When to request** is the design question, and the fast observer already computes the answer for
another reason: a screen *settles* after `VisualSettleLooks` (3) consistent looks. That is the
cheap change-detection the owner described, and it already exists. Request on settle; do not
request per look.

The arithmetic works out, and it is why this shape is viable at all:

```
screen settles        ~3 looks   (~180ms)
inference returns    ~10 looks later (~600ms)
```

A detection therefore always arrives **about ten looks after the frame it describes**, and that is
the fact rule 3 exists to handle.

### 3. A detection is evidence about an OBSERVATION, not about a screen

This section said "screen" and was wrong, and the way it was wrong is worth keeping on the page
because it is the error this whole design is most likely to make again.

A screen can be left and returned to:

```
settle on A  ->  request  ->  navigate to B  ->  return to A  ->  the answer arrives
```

Under a screen rule that answer is admitted, because the arrangement standing now IS the
arrangement it was computed on — they are the same menu. But the interface went somewhere else
in between, and anything could have changed on the way back. No arrangement comparison can
separate the two visits, because there is nothing to compare: they are the same.

So a detection carries the **observation generation** — a counter the observer moves on every
screen change and every window replacement. Returning to A costs two increments, and the old
answer is refused for the right reason: not that the screen is different, but that the
observation is. Measured, this refused **56 of 249** delivered candidates on one recording.

A candidate is admissible while its window generation AND its observation generation are both
still current. It is then **held and offered on every look** until one of them moves, at which
point the whole held set goes at once — 164 expired that way. Holding is not an optimisation:
the press that needs the TRAINING tile lands about twenty looks after the menu settled, so a
detection offered only on its arrival look would never be there when it mattered.

Never reprojected, never aged, never adjusted for a scroll.

### 4. Authority — and most of it is already written

- **A slow candidate may become a click target.** It is a proposal; the press still decides which
  proposal is the control; it wins or loses on containment like every other. Proposing is not
  deciding, which is the principle Experiment-028 was argued on.
- **A slow candidate may never become an identity.** `workflow.Support` stays two-valued —
  `SupportNone` and `SupportGeometric`, with no semantic value. A detection localises; it does not
  certify that two readings are the same control. The owner's distinction between geometric
  evidence and semantic identity survives this change untouched, and that is the test of the
  design.
- **`Class` is carried and never read.** The detector called the tile `image`, which is correct and
  is not a licence. Nothing downstream may branch on it. A boundary test of the kind
  `workflow/boundary_test.go` already performs — an import walk — can hold this by construction.
- **Reacquisition is unchanged.** `observe.Reacquire` matches a target's stored readings against
  the look's proposals. A slow candidate is simply another proposal in that set.
- **Disagreement is recorded, not resolved.** `fusion/visual.go`'s rule, in a place that has no
  fusion engine to enforce it.
- **Absent is the ordinary case.** No detector is what most machines have, and the fast path must
  behave exactly as it does today when `Take()` returns nil forever.
- **Nothing becomes durable.** A detection is session-local evidence like everything else here. It
  does not promote a Place, mint an entry identity, or touch the durable graph.

## Why not just call `fusion.Engine`

It is the right set of rules and the wrong object. `fusion` produces `directorapi.Element`s for the
planner — a heavy belief carrying role, state, actionability, native ids and cross-cycle identity.
The fast observer's currency is a rectangle on a screen, and converting between them every 60ms
would import the whole Director cycle into the loop this exists to keep cheap.

So: **borrow the vocabulary, not the engine.** `ObservationSource`, `SourceRank` and
`ActuatingSource` are already shared types in `pkg/directorapi`. Using them here means the two
stacks cannot drift into two different ideas of what a source is allowed to do, which is the defect
worth preventing.

## What this does not fix

- **Cross-session binding.** [[Experiment-029-binding-a-screen-across-two-independent-folds]] found
  5 correct, 2 confidently wrong, no separating threshold. Better candidates may change the
  arrangements being compared, but the missing piece there is an **abstention**, and nothing in this
  note supplies one. [[ADR-183-remembering-recognising-and-being-allowed-to-act]] still has an
  unready half.
- **Semantic identity.** Still nothing. A class is not a name.
- **The scrolled-list limit.** Geometry cannot see reordering, and a second source of geometry does
  not change that.
- **How much farther the 21-step workflow gets.** Unknown and unmeasurable until something is
  built. Experiment-030 answers whether the tile can be a candidate, not what happens next.

## Consequences

- **The two stacks acquire exactly one shared concept** — a sourced candidate — rather than being
  merged. That is the smallest change that lets the evidence meet.
- **One type change is load-bearing and touches many call sites.** `Look.Regions` is passed around
  the fast path. Doing it as a widening (a `Candidate` that still answers for its `Region`) keeps
  the diff honest; doing it later, after more is built on the bare rectangle, costs more.
- **A new refusal becomes expressible**: *I have no rich perception of this screen yet.* Today the
  fast observer cannot say that, because it has never had a second source to be missing.
- **Nothing is built,** and the order matters: this is the wire, and the owner has not reviewed the
  probe that justifies it.

## What the implementation added that this note did not anticipate

- **The duplicate rule nearly deleted the capability.** When both sources describe one
  rectangle one copy is kept, and the obvious choice is the pixel one so a stored reading never
  depends on inference timing. `sameRect` is loose enough that a few pixels of difference would
  make the stadium slab and the tile inside it "the same rectangle" — discarding the tile, with
  every admission counter still reporting success. The **more specific** reading survives.
- **The arrangement had to be held out explicitly.** Screen identity must be a function of the
  look alone; feeding it a channel that arrives ten looks late would make one screen's signature
  depend on whether inference had landed.
- **Reacquisition had to see the same set a press is judged against**, or Marco could learn a
  control it could never find again.

## Enforced by

[[Experiment-031-the-workflow-gets-nine-steps-in]] names the seventeen tests and the mutation
run (16 of 16, after two real survivors were found and fixed). The two this note asked for on
day one both exist: `TestNoDecisionReadsTheDetectorsClass` holds the detector's vocabulary out
of every decision, and `TestAHeldDetectionIsDroppedWhenTheScreenMovesOn` holds the staleness
rule.

Superseded for this note's own purposes: the original "nothing yet, by design." The tests arrive with the implementation. The two that should exist on day
one are named here so they are not invented afterwards to fit what was written: an **import walk**
proving no decision-making package reads `Class`, and a **staleness test** proving a candidate is
dropped when its screen goes rather than being carried onto the next one.
