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
- [[ADR-104-perception-is-a-budget-not-a-habit]]
- [[ADR-105-repair-buys-knowledge-not-permission]]
- [[ADR-106-a-place-is-not-how-long-you-looked-at-it]]
- [[ADR-107-a-sensor-appearing-is-not-the-screen-changing]]

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

## What a reading costs, and how often one is taken

One `Snapshot` is a single bulk `CacheRequest` over the whole subtree — every property of every
node in one cross-process call. The bridge keeps no state between calls, subscribes to no UI
Automation events, and has no cheap targeted read (`Locate` exists for actuation and searches
the same tree). Measured over repeated readings of unchanged windows:

	File Explorer   298 elements   1674ms mean
	Settings        155 elements     70ms mean

Exactly one walk per `Collect`. So cost is entirely a question of how often one is asked for,
and the answer is the attention the ambient supervisor already keeps: one second while things
are changing, relaxing to eight while they are not, governing both the gap between sessions and
the cadence inside one. Nothing is cached — every sample is a full fresh walk — there are simply
fewer of them on a quiet desktop. See [[ADR-104-perception-is-a-budget-not-a-habit]].

`director walk-audit` counts and times them; `--pixels` adds the visual pass an ordinary sample
used to ask for unconditionally.

The visual detector and OCR are AUTHORITATIVE providers in the same collector — no ShadowOnly
marker, so their evidence reaches fusion and therefore reaches SufficiencyOf. They are opt-in
per request, and the request is made when `observe.EscalationOf` says more evidence is worth
buying. Measured with the detector configured:

	Settings   66ms / 155 elements  →   940ms / 176 elements   sufficient either way
	Explorer  1567ms / 298 elements →  2460ms / 307 elements   sufficient either way

So a healthy reading buys nothing, and repair can improve what Marco KNOWS without improving
what Marco MAY DO. See [[ADR-105-repair-buys-knowledge-not-permission]].

### A judgement must not measure the session

`observe.ReachOfState` decides whether a large space in a window is vacant by the SHARE of the
reading's structures that lie inside it. Both halves of that ratio have to be counted over the
same reading, and one of them was not: the denominator was every structure ever seen in the
screen state, which grows with every sample.

One session over one Settings page nobody touched, at 900ms:

	 14 samples    466 ever-seen    142 present    recognised
	 27 samples    817 ever-seen    142 present    recognised
	 40 samples   1024 ever-seen     88 present    UNREADABLE
	183 samples   1024 ever-seen     88 present    UNREADABLE

Eighty-seven structures sat inside the content region throughout. Recognition therefore stopped
working part way through every long look, and said so as a fact about the page rather than about
the reading. `tracksInState` now returns the structures that are still there. See
[[ADR-106-a-place-is-not-how-long-you-looked-at-it]].

### The escalation gate runs inside the cycle

`liveSampler.Sample` holds `Runtime.mu` for the whole collect-and-fuse, and both consumers of
`moreEvidenceIsWorthBuying` are called from inside it — the sampler's own request as an argument
to `Collect`, and the shadow provider's gate deeper still. A gate that reaches back for that lock
deadlocks the session on its second sample, which is exactly what happened; the first sample
survives because nothing has settled yet, so it reads as a slow start rather than a hang.

Measured: 14 samples in 12 seconds with the gate bypassed, one sample and then silence with it.
Anything the gate needs to remember has its own mutex.

### A sensor appearing is not the screen changing

The composition a screen is identified by is what a source that can describe STRUCTURE said the
screen is made of. A detector's own word is corroboration, not composition — unless pixels are
all Marco has, which is the case the detector exists for.

`directorapi.KindEvidence` carries who accounted for what a thing IS, which is a different
question from `Provenance.OnlyDescribesPixels` — that one asks whether anything but a camera
reported the thing is THERE. Three answers, because a detection nothing structural saw and an
element accessibility saw without being able to name are not the same:

	described     a structural source said what this is       counted as it is
	pixel_named   something structural saw it and could not   counted as `unknown`
	              name it; a detector did
	pixel_only    pixels are the whole account                not counted, unless they are
	                                                          the whole reading

Classified in `buildSample` beside the chrome classifier — the last point where the evidence and
its provenance are both in scope — and read at `NewScreenSignature`, which both the segmenter and
`stateFingerprint` go through. A label, not a removal: every structure is still tracked, still
counted toward whether the window was read, and still reported.

Without it, one cold Learn with the detector configured left two durable Places for one Settings
page nobody touched. See [[ADR-107-a-sensor-appearing-is-not-the-screen-changing]].

### And a camera still cannot say how to work a control — unless somebody asks

`ActuatingSource` admits accessibility, a native integration, a DOM bridge and a capability pack,
because each can say how to OPERATE a thing. A detector and an OCR reader describe what the
screen looks like, so `Actionability.Actuable` is false for an element only they saw and the
resolver refuses it. That rule is unchanged and is what stops Marco operating a surface nobody
vouched for, on its own initiative, everywhere.

What it also did was make every control in an application with no accessibility tree unreachable
except by demonstration. So an explicit instruction may now license ONE action on such a Prop:
`directorapi.OperateLicence` is a fact about the REQUEST — these words, this verb — and says
nothing about which Prop is meant. Ambiguity still refuses, geometry still has to be there and
refuses at grounding, policy never sees the licence so risk rules are untouched, and read text
alone is not a clickable surface, because the rectangle of a word is a caption and not the
control it labels. It is a local value on one call: nothing stores it, nothing can reuse it, and
it mints no identity.

`Resolve` is unchanged and licenses nothing; `ResolveUnder` is the licensed sibling, so every
existing caller behaves as it did. See
[[ADR-202-an-instruction-may-license-one-action-on-a-prop-only-pixels-describe]].

And exercising it against two real applications found that the first missing connection is
upstream of all of it. `capture.WindowCapture` is one method, so a recorded frame reaches the
real OCR and ScreenParser providers with no new code — and on a Trackmania pause menu they
produce 46 observations of which **every one is text**. The detector proposes three buttons and
a menu but below the 0.15 acceptance floor, so they never become observations; its confident
detections are all `text`. Fusion therefore builds no Element, the Stage carries no Props, and
the Director correctly says nothing in the window can be operated. A perception gap, not a
grounding or authorization one, and not fixed by tuning a floor to one recording. See
[[Experiment-051-the-stage-two-real-applications-produced]].

Investigating the existing structural proposer as the missing source found the evidence is
already there and is lost later: on that same frame it produces a **444x56 menu row** for the
SETTINGS entry, the same 56 px height as every other entry. (That experiment measured the
association with `NameCandidates`, whose rule is total containment, and the OCR box overruns
the row by eight pixels. Fusion does not use `NameCandidates` — it admits text by FRACTIONAL
coverage at 0.80, so on the Stage path the association succeeds. See
[[Experiment-053-what-the-stage-can-already-say-it-is-unsure-of]].) The proposer is generic — 228 regions on a game, 40 on Explorer, 25
on Paint — and it finds the rows the detector rejects below its floor. Neither reaches fusion.
No new interface would be needed to change that, only a provider and an `ObservationSource`,
and a new source is not in `ActuatingSource`, so proposed Props would arrive non-`Actuable` and
still need a licence. See [[Experiment-052-what-the-structural-proposer-offers-the-stage]].

### A proposed rectangle is Stage evidence, and not a control

That provider exists now. `providers/structure` runs the same proposer and emits its rectangles
as element observations under `SourceStructure`, carrying the proposer's own `kind`, `fill` and
`parts` and claiming nothing else: no role, no label. The Trackmania menu goes from 46
observations and **zero Props** to 274 and **228 Props**, 13 of them named, with SETTINGS and
SCOREBOARD each fusing a structural region and its OCR reading into one Prop; Explorer gives 40
Props, 19 named. RESUME stays nameless because OCR never read it, which is the honest answer.

**Nothing became clickable.** Three rules do that and none was touched: `ActuatingSource` does
not list the source, `RoleUnknown` affords nothing so `Targetable()` is false whether or not a
licence is granted, and fusion's text rules are unchanged. The 48%-of-window background region
on that frame is not a target, and neither — this is the gap, reported rather than closed — is
the well-grounded 444x56 menu row, because the only thing actionability consults is the role and
the proposer may not assign one. See
[[ADR-203-a-proposed-rectangle-is-stage-evidence-and-not-a-control]].

Asked where that gap actually sits, the whole path from a real capture to a real decision was
measured once more. **The row is already a Prop** — `{98 750 444 56}`, unnamed, at the structural
source's confidence floor — and fusion already computes the relation: the word-shaped region and
the row contain the SETTINGS text at 0.955 and 0.950, a difference far inside `ambiguityMargin`,
and only `nestingRatio` keeps it from being called ambiguous by classifying the row an ANCESTOR.
That classification is the relation, and nothing but the winner's id survives it. **ADR-202's
licence already passes** on that Prop — sources `[structure ocr]`, so `OperableUnder` returns true
with no reason — and the single refusal at grounding is `Any()`, from the role. And **two further
gates refuse the whole world** before an action is considered: `Coverage` is 0.000 because
`RoleUnknown` is a container, so policy's first gate reports *"only 0% of this window's content was
visible"*, and `Blind()` fires on the second. `Locates()` refuses none of the oversized cases.
See [[Experiment-054-the-surface-a-named-prop-does-not-carry]].

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
