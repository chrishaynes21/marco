---
type: subsystem
status: active
owners:
  - director
depends_on: []
used_by:
  - fusion
  - programs
  - passive-observation
updated: 2026-08-12
source_paths:
  - internal/director/perception
  - internal/director/world
  - pkg/directorapi/observation.go
---

# Perception

Perception turns the desktop into **evidence**. It owns the observation graph, the provider
contract, provenance, and the cycle that drives a round of sampling.

It does not decide anything. What the Director believes is [[Fusion]]'s output, and what it
does about that belief belongs to [[Programs]].

## Responsibilities

- Define what an observation is, and keep that definition inside perception.
- Drive a sampling cycle across providers.
- Record provenance and confidence on every observation.
- Hand the result to fusion, and to nothing else.

## What it may not do

- Establish belief. See [[ADR-001-observations-vs-belief]].
- Reach into the reasoning layers.
- Let a consumer know which source produced something — a source is an implementation
  detail, and code that special-cases one is code that will break when it is replaced.

## Sources

| source | may establish | note |
|---|---|---|
| accessibility (UIA) | structure, identity, capabilities | the application's own assertion |
| OCR | visible text | attaches to structure, never creates it |
| [[Vision]] | proposed structure, regions | never actionability |
| visual state | that something changed | not what it is |

Governed by [[ADR-003-evidence-authority-by-source]].

## Related systems

- [[Fusion]] — consumes everything here
- [[Vision]] — a provider
- [[Windows]] — decides what may be captured at all
- [[Passive-Observation]] — a long-running consumer with no execution path

## Decisions

- [[ADR-001-observations-vs-belief]]
- [[ADR-002-fusion-owns-belief]]
- [[ADR-003-evidence-authority-by-source]]
- [[ADR-008-no-stale-window-geometry]]
- [[ADR-011-provenance-is-proven-not-assumed]]
- [[ADR-101-visual-presence-is-not-legal-actionability]]
- [[ADR-102-a-detector-earns-its-place-where-perception-fails]]
- [[ADR-103-acquisition-success-is-not-semantic-completeness]]

## Validated by

- `internal/director/perception_boundary_test.go` — `TestOnlyPerceptionKnowsWhatAnObservationIs`,
  `TestPerceptionCannotReachIntoTheReasoningLayers`, `TestNothingOutsidePerceptionKnowsThatOCRExists`
- `internal/director/perception/observation/outcome_test.go` — the guard's rules, one per test
- `internal/director/perception/providers/collector_provenance_test.go` — every provider that
  runs produces an outcome, and the in-flight replacement never reaches the admitted set

## Seeing this live

`marco director perception` reports which providers contributed to the last cycle and what
fusion made of it, and flags any provider that contributed **nothing** — which is the exact
shape of the gap below. The overlay renders the same thing as a live panel (type
`director`). Both read the service's own history, so neither perturbs what it describes.
See [[Service]].

## Is the reading good enough?

A separate question from "what did we see", and it has its own owner: `observe.ReachOfState`
judges how far into a window an observation got — on arrangement rather than richness — and
`observe.SufficiencyOf` names the answer as `sufficient`, `incomplete` or `unobservable` with
a reason a person can read.

Four facts stay apart, and three of them used to be reported as the second:

	I do not know this Place.                    healthy reading, memory has no match
	Accessibility did not show me the Place.     the frame arrived and the page did not
	Accessibility showed me the Place, slowly.   a rich reading that cost 1.5 seconds
	There was nothing to read.                   the sensors did not report

Not evidence of incompleteness: element count, acquisition time, the application's name,
whether memory recognises the Place, or whether any optional sensor exists. Validated against
the seven real desktop readings in `fixtures/perception/desktop/corpus` and, live, on a
Settings Place that reflowed from 111 elements to 57 and an Explorer tree of 294 elements that
took 1.6 seconds to read — all sufficient. See
[[ADR-103-acquisition-success-is-not-semantic-completeness]].

`director assess-desktop-sample` prints it, with acquisition cost beside the verdict and
deliberately not part of it.

## Known gaps

- ~~Vision runs an unrequested pass on every ordinary cycle.~~ **Fixed 2026-08-10.** The
  opt-in was enforced on `Observe` and not on `ObserveTargeted`, which is the door the
  collector prefers — so every command captured the screen, and every live diagnosis carried a
  false `vision: no window to look at` degradation. See
  [[ADR-037-opt-in-is-enforced-on-every-door]].

- ~~The accessibility provider is not pinned to a session's window.~~ **Fixed 2026-08-06.**
  The targeting mechanism existed at every layer; `liveSampler.request()` simply never set
  `observation.Request.Window`. A re-run against VS Code went from 1 fused element to **353**
  (279 stable), with the terminal foregrounded throughout. See
  [[director-accessibility-targeting]], and the superseded finding in
  [[Experiment-002-dnfc-observation-baseline]].
- ~~**Observations carry no window generation.**~~ **Fixed 2026-08-06.** A provider now
  reports a `ProviderOutcome` carrying what it was asked to observe and what it can *prove*
  it observed, established after collection by re-reading the platform. Fusion compares the
  two and admits nothing target-scoped that cannot prove itself. See
  [[ADR-011-provenance-is-proven-not-assumed]].
- **Accessibility ownership is never re-validated after a snapshot.** The capture path has
  before/after validation ([[ADR-008-no-stale-window-geometry]]); the accessibility path
  trusts the handle it was given.
- ~~**Provider outcomes are collapsed.**~~ **Fixed 2026-08-06** as part of the same change.
  `ProviderState` distinguishes contributed / empty / unobservable / unavailable /
  target_changed / provenance_mismatch / failed / timed_out, and a truncated walk carries its
  shortfall separately as `Incomplete` — real evidence that still cannot support a negative
  conclusion.
- **The provenance guard has never run against a live desktop.** Every test above uses a fake
  platform, deliberately — a window replaced mid-walk cannot be staged on demand. What is
  unproven is the live path: whether a real UIA bridge reports `ObservedWindow` faithfully
  when its window dies underneath it. The Rocket League close/relaunch in [[Windows]] is the
  natural way to find out.

## Milestone record

[[director-perception]] — the observation graph, the fusion engine, OCR, and visual state,
with merge criteria and cost.

## Known live-geometry defect: members measured outside their own window

**Status: OPEN.** Not a presentation defect, and deliberately not fixed inside Roadmap 34.

Some inferred members are measured outside the current window frame. Normalising such an element
against the frame produces a region outside `0..1` — usually a negative `Y`.

Measured live, 2026-08-12, watching Discord: every structural group in the session carried a
negative envelope, `y` between `-0.18` and `-0.37`, against a frame of `1936x1048+-1928+-8`. The
elements sit a few hundred pixels above the window they were attributed to. The same session
watching VS Code, on the same monitor with the same frame, produced entirely well-formed regions.

This is the same family as the durable-envelope problem recorded in [[Semantic-Memory]] — the
Explorer record whose stored envelope sits mostly above its window and can therefore never be
recognised again. Both are measurement or attribution faults; neither is about drawing.

### What the grounding layer does about it, and why that is all it should do

Fail closed, at two levels:

```
a member cannot be placed reliably     → omit that member
every member cannot be placed reliably → refuse to point, and say so
```

The refusal a person reads is *"I know which structure I'm asking about, but I can't work out
reliably where it is on this display, so I'd rather not point at the wrong thing."* That is the
correct outcome: a highlight that is nearly right is read as exactly right, so an approximate box
is worse than none.

Explicitly NOT done, and not to be done later as a convenience:

- `pkg/referent` is not changed. Its all-or-nothing refusal is right from where it stands — one
  stray rectangle among many is indistinguishable from a window that moved mid-measurement.
- Off-frame geometry is not clamped into range. The resulting rectangle would be inside the window
  and would describe nothing that was measured.
- Boxes are not shifted to make them visible.

The per-member omission lives in `observe.regionsOf`, one layer above `pkg/referent`, because that
layer knows which member each region belongs to and that they came from a single inference — so it
can tell "this one element is unplaceable" from "this whole batch is stale", which `Map` cannot.

### Enforced by

- `TestAMemberOutsideItsOwnWindowDoesNotSilenceTheRest`
- `TestASubjectEntirelyOutsideItsWindowSaysTheCoordinatesAreUnreliable`
- `TestARegionOutsideItsWindowIsRefusedNotClamped` (`pkg/referent`)

### Next

Its own perception-correctness milestone, together with the negative durable envelopes. The
question to answer first is whether the element bounds or the window frame is wrong — the two
produce identical symptoms and need different fixes.
