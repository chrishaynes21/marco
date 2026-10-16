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

### Coverage is an accessibility measure wearing a general name

Asked why policy refused that Stage, `Coverage` turns out to be `content/n` over roles, where
`ElementRole.Container()` lists `RoleUnknown` — so it measures **the share of reported objects
carrying a specific non-container kind**, not visual extent and not whether anything was seen. A
backgrounded Discord husk (8 panes, quality 0.90) and a 228-Prop structural Stage (quality 0.056)
both score 0.00. `Blind()` is `Actionability <= 0`, which conflates nothing-reported,
nothing-with-an-interactive-kind and nothing-addressable; and the existing distinguisher
`SeenOnlyByPixels()` counts only elements that `Affords()`, which is role-derived, so it returns
false on a structural Stage and the better sentence is unreachable. Meanwhile `uiact.Ladder` is
the component that actually knows whether an Actor path exists, and it runs AFTER the gate that
refused for lack of actionability. The resolver already states the principle policy contradicts —
*"the world being weak overall does not unfind a control we can plainly see and use"* — and checks
the same conditions only after finding nothing. See
[[Experiment-055-what-coverage-and-blind-actually-measure]].

And assessed separately: membership of a **repeated set of same-shaped rows each carrying its own
distinct text** separates every case in a five-interface corpus where `fill` and `parts` do not —
the 48% background has no siblings at all, Windows Settings yields nine 846x66 rows at a 72 px
pitch with nine distinct texts, and a paint canvas correctly yields none. It does not settle the
Explorer toolbar, which has one sibling (the title bar) where the browser's real control list has
two; pitch regularity is not usable as measured. See
[[Experiment-056-whether-a-row-is-one-of-a-set]].

### A grounded target answers the world-level question and nothing else

So the gates learned one new input. `EvaluatePlan` and `EvaluateStep` take the `Resolution` this
request produced — the resolution and not merely its target, because only a Resolution can say
whether the request resolved to ONE thing — and gates 1 and 2 are skipped when
`Resolution.GroundedIn` holds. Every clause of that predicate is a way a target pointer can exist
and mean nothing: an ambiguous status, an element this reading does not have, no rectangle to aim
at, no confidence, a stale reading. Geometry only, never permission; `Element.OperableUnder` at
grounding is still the only thing that decides whether a Prop may be operated, and policy neither
imports nor names a licence (a source guard holds that).

ONE BRANCH IS NOT CONDITIONAL: where `SeenOnlyByPixels()` holds, gate 2 refuses whether grounded
or not, because that is a claim about MECHANISM and grounding does not establish execution
capability. It reads as an asymmetry — nameless rectangles pass where detected buttons do not —
and the reason is exact: that method is a positive claim and can only be made where something
affords.

Measured on the real Stage: "click Settings" **still refuses**, at the resolver, for the reason it
always did. A request that HAD grounded a target now reaches the evidence gate instead, which asks
rather than refuses. `Coverage`, `Actionability`, `RoleUnknown` and `Targetable()` are all
unchanged. See
[[ADR-204-a-grounded-target-answers-the-world-level-question-and-nothing-else]].

### A surface hypothesis is request-scoped and refuses to guess

`internal/director/surface` is the first piece of Director reasoning over the Stage rather than
consumption of it, and it is READ-ONLY: given "click Settings" it produces the identity, every
candidate surface with the evidence for and against it, a three-valued decision and a sentence,
and executes nothing. A candidate must contain the identity (by `fusion.Owners`, the shared rule),
be accounted for by something other than read text (ADR-202's own rule), locate a position in the
window (`observe.Region.Locates`), and **be the only named thing inside it** — the test that does
almost all of the work and needs no number. `fill`, `parts`, area and sibling structure are
carried as evidence and decide nothing.

Measured over five recorded interfaces: every indefensible surface refused, including the NAMED
48% background and Explorer's toolbar band. And on the case the milestone was about, the answer is
AMBIGUOUS — nothing in the evidence chooses between the word's own region and the 444x56 row
around it, and the same shape recurs on Windows Settings. Where one surface does survive it is
usually the caption, and the sentence says so rather than pretending otherwise. The one missing
piece is named exactly: **which of two nested surfaces is the control**, which no amount of looking
supplies and one demonstration does. See
[[ADR-205-a-surface-hypothesis-is-request-scoped-and-refuses-to-guess]] and
[[Experiment-057-what-earns-a-click-on-five-interfaces]].

### A demonstrated surface is one fact about one target

The answer to that question is a person's own press, and the one real discovery is that **a press
on the words settles nothing**: shown "the word, or the row?" a person clicks the word, the word is
inside the row, and the smallest-containing rule every other part of this system uses would have
answered "the word" every time while learning nothing. So a press settles it only when exactly one
surface contains it, and the question says which part of the control would tell them apart.

What is remembered is a RELATION — the identity's own region, or a containing region at a depth —
joined to a durable semantic target by its subject id, with the normalised rectangles kept as
evidence that can only CONTRADICT a later reading. No coordinate, no label, no third durable-text
exception. Three confirmations stay apart (`observed`, `attributed`, `verified`) and `observed` is
kept and refuses to aim.

Later use re-observes, re-evaluates, selects by relation, and lets the stored shape veto the
result — then hands `target.Demonstration` to the resolver, where it answers **one clause**:
`Actionability.Any()`, the affordance a proposed rectangle cannot claim for itself. ADR-202's
licence, enabledness, geometry, policy and the ladder are all still required. Identity and surface
travel as two things, because the words identify and the surface locates.

Measured on the recorded game menu: the loop closes, and the press reaches the actuator at
(320,778) — the row's own middle, where the words' is (469,775). No production path grounds a
demonstration yet: `Pipeline.Surfaces` and `Pipeline.Place` are both nil in the daemon. See
[[ADR-206-a-demonstrated-surface-is-one-fact-about-one-target]] and
[[Experiment-058-one-question-one-answer-and-the-same-request-again]].

### A question is answered by a press and confirmed by a crossing

Both halves already ran in ambient watching, so the wiring is a reader of each rather than
anything new. `navsource` admits a hand's press — refusing an injected one, one on Marco's own
surface, and one it cannot place in the window — and the session runner banks it **stamped with
the screen state it was performed on**; `ambientObserver.drain` reads that log and one call there
offers it to the open question. The ambient observer then detects a screen change, asks whether a
person or Marco caused it, and one call there offers the step as a witness. The durable place is
`observe.Place.Subject`, already resolved on every ambient look by the one resolver.

**A confirmation cannot be asserted.** `observe.SurfaceConfirmation` is a struct with no exported
field and one constructor per level — `ScreenName`'s discipline. `TransitionAttributed` is the only
door to a usable one, and it refuses a crossing with no application, one not attributed to a
person, one attributed to anything but **exactly one** human action, one with no screens either
side, one where the screen did not change, and one with no time. The zero value confirms nothing,
so a forgotten field is a refusal rather than a press, and `verified` is named with no constructor
because nothing can verify an intended outcome yet.

A question binds one request, one place, one window, one observation session and one screen state,
and lapses after ninety seconds; a later request closes it. Two presses that disagree contest each
other and leave it unanswerable — the same refusal the press rule makes of a press inside both
candidates.

Measured on a game menu and a Windows Settings page: the loop closes on both, and the four
negative crossings each keep the press as a demonstration that cannot aim. See
[[ADR-207-a-question-is-answered-by-a-press-and-confirmed-by-a-crossing]] and
[[Experiment-059-one-demonstration-from-a-person-into-durable-memory]].

`director demonstrate-surface -check` reports the preconditions for a live run and injects nothing.
Five of seven hold on the real machine, including the installed input hook and both seams. The
experiment is **blocked by a structural one**: no production observation request includes
`SourceStructure`, so the shape proposer never runs on a live Stage — and where an accessibility
tree describes the control there is nothing to ask anyway, which is the case on Windows Settings.
See [[Experiment-060-the-preconditions-for-one-live-demonstration]].

The owner chose a configuration over a redesign, so there is now ONE guarded test configuration:
sources restricted to the window system, the shape proposer and OCR, with accessibility neither
requested nor registered. It is reachable only from a throwaway MARCO_HOME, through the same
resetGuard the reset command uses, so a Director started normally cannot reach it and nil is the
production default. It changes what is OBSERVED and not what is ALLOWED — resolver, licence,
policy, ladder, Marco runtime, foreground guards and cancellation are all unchanged.
`director demonstrate-surface` drives it in four phases, three of them dry-run; the learn phase
verifies the live ambiguity exists before asking for anything and reports its absence as a result.
Six of eight preconditions hold on the real machine; the rest need a hand. See
[[ADR-208-one-guarded-test-configuration-and-no-second-one]] and
[[Experiment-061-the-live-focused-demonstration]].

**The first live run of that configuration produced no Stage at all** — an empty foreground
and zero Props on a screen full of controls. The accessibility walk is the only thing that
enumerates windows, and every pixel provider is scoped to a window, so leaving accessibility
out removed the Stage rather than thinning it. The window system now enumerates when a
request names it — `Includes`, the same opt-in the shape proposer has, so an empty request
still gets nothing and production observation is unchanged — and the harness collects two
cycles, because the first is what establishes the window the second looks inside. Measured
again: a window, a foreground application and 46 proposed rectangles with no accessibility
evidence in the cycle. See [[Experiment-062-what-a-stage-is-made-of]].

`director try-it-out` is how the harness is driven: one phase per invocation, one checkpoint
line each — READY, OBSERVING, DEMONSTRATION NEEDED, LEARNED, READY TO EXECUTE, RESULT, and
BLOCKED when a phase refused — and its own disposable store. It delegates to the same
functions `demonstrate-surface` calls and moves none of the gates: the demonstration phase
starts only when a person has said they are ready, and the one live click still needs its own
authorisation. See [[ADR-209-the-assistant-operates-the-harness]].

**The pixel-only configuration cannot close its own loop.** The first operated run reached
the surface question on a live screen — two tied defensible surfaces around "installed
apps", measured outside a fixture for the first time — and then stopped because no durable
place could be established. A screen is establishable when its signature can match again,
and it discriminates on interface terms; under pixel-only not one word was tallied against
the screen, so twelve readings of sixteen were refused as `not_discriminating`. The same
page under production defaults establishes a place immediately. So the configuration's
requirements are mutually exclusive as the system stands, and no choice of screen helps.

And the premise that led to it is false: under production observation, with every Prop from
the accessibility walk and no pixel evidence in the cycle, `click Dark` still produces two
tied surfaces on the Personalisation panel — a caption and the tile it labels, both
described by the tree, with nothing choosing between them. See
[[Experiment-063-the-configuration-that-cannot-close-its-own-loop]].

**A question nobody could answer is no longer asked.** Putting the surface question to a
live accessibility tree found every tie on eight Settings pages to be a control beside its
own wrapper — identical bounds and an identical press point, or two pixels apart — and the
press rule settles a question only when exactly one option contains the press. So a question
is offered only when some place exists that a press could use to single one out: each
surface is clipped by every other, and what survives must be at least as large in both
directions as the smallest surface already on the menu. The minimum comes from the screen
rather than from a constant, because a threshold chosen to make one application pass fails
the next one silently. The ambiguity is still reported as ambiguous; what the explanation
adds is that the demonstration cannot exist rather than that somebody failed to give one.
See [[ADR-210-a-question-nobody-could-answer-is-not-asked]].

There are now **three observation configurations**, because two of them cannot work:
production establishes a place and offers only unanswerable ties, pixel-only offers
separable surfaces and can establish no place, and the augmented one is production defaults
with the shape proposer and OCR added. The third is additive — `Include` adds opt-in sources
and removes nothing, so accessibility is collected exactly as usual and production is
unchanged because production does not ask. `Sources` stays empty on purpose: naming anything
there would switch accessibility off by implication. All three go through the one sandbox
door, and the configuration is unverified on a live screen. See
[[ADR-211-production-observation-with-the-pixels-added]].

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
