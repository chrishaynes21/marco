---
type: subsystem
status: active
owners:
  - director
depends_on:
  - perception
  - vision
  - windows
used_by:
  - vision
  - game-packs
updated: 2026-08-12
source_paths:
  - internal/director/observe
  - internal/director/observesession
  - internal/director/learn
  - cmd/director/observecmd.go
  - cmd/director/learncmd.go
  - cmd/director/learnsessionwiring.go
  - cmd/marco/watchui.go
  - cmd/director/observelook.go
  - cmd/director/observeledger.go
  - cmd/director/observeambient.go
  - cmd/director/observewiring.go
  - cmd/director/experiment.go
---

# Passive Observation

Watching a running application for minutes at a time, sampling what is on screen so temporal
analysis has something to work with. It never acts.

## The guarantee is structural

The promise is not that this code does not click. It is that it **could not**, because
nothing it can reach knows how. A boundary test walks the package's transitive dependencies
and fails if anything capable of affecting the machine is reachable at all. See
[[ADR-010-passive-observation-cannot-execute]].

## Privacy: the classifier is two-stage, and the first stage is structural

The original rule asked what the **text** looked like — word length, letter ratio, a marker
blacklist. That cannot work, and a live Chrome session proved it: `"Chris Haynes Plus"` is
three ordinary capitalised words, indistinguishable by shape from `"Exit To Menu"`.

What separates them is what they are **attached to**. A button's name is a fact about the
interface; text inside an icon's box is a fact about the person using it.

1. **Structural eligibility** — only `button`, `menu_item`, `menu`, `tab`, `checkbox`,
   `radio` may hold plaintext. Everything else defaults to private, and the allowlist is
   closed, so a newly added role is refused until somebody decides otherwise.
2. **Shape**, as defence in depth — even a button's name is refused if it looks like a token.

Everything else keeps role + length + confidence + digest, which is all temporal analysis
ever needed. The digest is what makes "this changed" observable without "this said" being
retained.

**What it costs, stated plainly.** The detector in use emits one class that maps to
`RoleIcon`, so a game's menu labels are withheld too — the same `RESUME GAME` that reads
perfectly is stored as a digest, because nothing structural vouches for it being a button.
That is the honest trade: a classifier loose enough to keep those would also keep a friends
list. The remedy is a detector with a real class vocabulary — see [[Vision]].

**The allowlist is now one policy and it also decides what is READ.**
`directorapi.ElementRole.NameablePlaintext` since 2026-08-09
([[ADR-017-structure-earns-a-name-text-never-earns-structure]]). Four copies existed — this
classifier, the shadow diagnostic, the vision benchmark, and the scoped label reader — and the
reader's copy would have been the one that decided which regions an OCR engine is pointed at.
Nameability now gates the reading as well as the keeping, so an unsayable region costs no round
trip and its text never enters the process at all.

**Two surfaces, kept apart.** A *session-local safe label* may hold readable text for a nameable
role, because a structural role vouches for it. A *durable semantic term* is never text — it is
a member of the closed vocabulary in `observe/terms.go`, and the durable representation has
nowhere to put a string. Text read from a `text` REGION takes the stricter deal unconditionally:
it may become terms and may never be released in the clear, because nothing structural vouches
for what a text region says.

## Metrics

`internal/director/observe/metrics.go` scores semantic usefulness rather than detection
volume: stable-entity yield, anonymous ratio, structural-role coverage, safe-label
opportunity, flicker rate, transition utility, unstable-structure ratio. Named thresholds,
every failure reported rather than the first, deterministic output.

`director benchmark-vision <session>` scores a stored session with no game running.

## Presence is state-relative, and global absence is not detector failure

The correction that came out of [[Experiment-007-state-relative-tracking]], and the one thing to
understand before reading any track table.

A session is segmented into generic, session-local **screen states** (`state_1`, `state_2`, …)
from raw detection composition — role counts plus coarse normalised arrangement, computed before
any track is touched so state identity is never downstream of tracking. Every track then carries
**two** presence figures:

- **global** — `Seen / Eligible` over every valid inference in the session
- **state-local** — `Seen / Eligible` over the inferences in which its own state was active

Both are true and they answer different questions. A pause-menu button in a session that is half
gameplay is genuinely `bursty` globally and genuinely `persistent` within its own screen. The
live Rocket League rows read `8/16, 50%, bursty` and `8/8, 100%, persistent in state_3`
simultaneously.

**A UI structure may be conditionally persistent.** An element is judged against the
opportunities where its state was active, not against every frame — a pause-menu button being
absent during gameplay is correct evidence, not a miss. Absence counts only when the track's own
state is confidently active, a valid inference ran, and it was not detected. A different state,
an unplaceable transition, a skipped slot, a failed inference and an unproven target are all
unknown, and unknown stays unknown ([[ADR-006-unknown-is-not-false]]).

`ShadowTrack.Stable()` is the global predicate and answers **wrongly** for anything
state-dependent; `StateStable()` is the one a capability should read. States are numbered, never
named — deciding that `state_2` is "the pause menu" is interpretation and belongs to a game pack
([[Game-Packs]]).

Tracks persistent in exactly one state, cut into runs by vertical arrangement, form
**structural groups** — the first thing here that describes an interface element rather than a
box. "Exactly one" is load-bearing: an element equally reliable on two screens is ambient and
belongs to neither structure.

## The state graph has edges

A transition records what the player was observed to do before it — see [[Navigation]] for the
producer and [[ADR-013-navigation-is-meaning-not-keys]] for why it records meanings and not keys.

The wording throughout is **correlated, never caused**, and the report is arranged to keep it
that way: the dominant intent is printed with its support, competing intents survive beside it,
and changes with no navigation before them are counted rather than omitted — a transition that
usually has no input is the evidence against reading the others as caused.

Navigation is banked across slots the detector sat out, because screen evidence and input
evidence fail independently and at the real skip rate the keypress that opens a menu very often
lands in a slot the cadence gate declined.

## Related systems

- [[Vision]] — one of two structural sources; see [[ADR-036-a-screen-is-a-composition-not-a-provider]]
- [[Navigation]] — the input evidence that gives the state graph its edges
- [[Perception]] — the pipeline, and the pinning gap below
- [[Windows]] — the callback exhaustion this workload exposed

## Telling somebody what was learned

`marco observe --follow` prints a line per COMMITTED change to durable knowledge — a Place
established, a way between two screens remembered, a name settled, a destination bound to a word:

	+ learned place       Mouse
	+ learned way         Home -> Bluetooth & devices
	· saw again           Home -> Bluetooth & devices
	+ learned destination "mouse" -> Mouse

Not a log of what Marco perceived. Samples, walks, candidates and confidences are Marco LOOKING,
and a person waiting for the moment they can say "alright, do that then" would never find it under
those.

The events come from `semanticmemory.Store` after its own write succeeded. Nothing upstream
announces and nothing predicts, so a Place refused at a bound, a signature that matched a record
already held, or a file that could not be written all arrive as silence. Names are resolved when
somebody looks rather than when the write happened, because a Place is established on one pass and
named on a later one. See [[ADR-111-a-demonstration-takes-the-slot-from-watching]].

**Watching yields to a demonstration.** One observation session runs at a time, and until 38A
ambient watching simply held it: every `marco learn` came back `no_observation` — *"I couldn't
watch — I lost sight of that window"* — so the product's headline loop could not be walked from a
command line. Ambient now stands aside for a Learn, exactly as Light Mode does, and takes the slot
back afterwards.

## Decisions

- [[ADR-010-passive-observation-cannot-execute]]
- [[ADR-111-a-demonstration-takes-the-slot-from-watching]] — watching yields the one observation
  slot to a demonstration, and `marco observe --follow` reports what durable memory COMMITTED
- [[ADR-093-observe-is-attention-not-recording]] — `marco observe` is the ambient lifecycle:
  one supervisor on the one substrate, the ZERO licence so nothing it sees becomes durable,
  a transient buffer whose size tracks novelty rather than time, and no authority or desktop
  lease of any kind
- [[ADR-094-observe-gathers-evidence-learn-promotes-it]] — "learn what I just did": the trail
  gains what somebody PRESSED, an action is attributed to the screen it was performed on rather
  than to whatever is in front when it is read, and an explicit Learn promotes a selected walk
  under the same licence a live one declares. Observe still holds none
- [[ADR-095-repeated-observation-may-become-knowledge]] -- what ambient watching learns is a
  SEMANTIC GRAPH EDGE, not a demonstration, under a policy separate from watching and off by
  default: one clean traversal is already knowledge, repetition is strength on the edge rather
  than a gate in front of it, a candidate ledger that is not knowledge, no contradiction, the
  same admission boundary an explicit Learn uses, and no name invented for anything. Edges
  learned in different sessions compose into routes nobody demonstrated as a whole
- [[ADR-096-observe-and-learn-are-two-doors-into-one-graph]] -- and explicit Learn writes the
  SAME graph: one store, one identity test, one planner. What differs is the licence and the
  intent, never the knowledge. Edges taught through one door compose with edges taught through
  the other, in either order, with no duplicate topology
- [[ADR-003-evidence-authority-by-source]]
- [[ADR-012-presence-is-state-relative]]
- [[ADR-013-navigation-is-meaning-not-keys]]
- [[ADR-039-a-surface-and-a-place-inside-it]]
- [[ADR-040-a-few-scales-were-not-better-than-one]]
- [[ADR-041-a-screen-is-not-its-dominant-group]]
- [[ADR-043-teaching-is-two-passes-not-a-new-capture]]
- [[ADR-044-a-teach-attempt-is-one-episode]]
- [[ADR-047-a-place-is-remembered-a-meaning-is-answered]]
- [[ADR-063-a-pass-remembers-every-place-it-settled-on]]
- [[ADR-064-the-order-of-a-walk-is-evidence]]
- [[ADR-065-operating-marco-is-not-demonstrating-to-it]]

## Related experiments

- [[Experiment-002-dnfc-observation-baseline]]
- [[Experiment-006-button-track-fragmentation]]
- [[Experiment-007-state-relative-tracking]]
- [[Experiment-011-two-level-identity-against-real-software]]
- [[Experiment-012-why-explorer-cannot-be-pointed-at]]
- [[Experiment-013-one-sample-in-ninety-one]]

## Validated by

- `TestTheObservationPackageCannotAct`, `TestTheObservationPackageStaysGameAgnostic`
  (`internal/director/observe/boundary_test.go`)
- `TestTheRunnerCannotAct` (`internal/director/observesession/boundary_test.go`)
- `cmd/director/observeguard_test.go`
- `Runtime.Handle` refuses while a session is active (mutation guard)
- `TestTheProductionSessionPathSegmentsScreenStates`, `TestScreenStatesReachNothingAuthoritative`
  (`internal/director/observesession/statewiring_test.go`) — state segmentation is reachable
  from the production path and reaches nothing authoritative
- `internal/director/observe/screenstate_test.go` — the state-relative eligibility suite

## Known gaps

- ~~A targeted session is vision-only by construction.~~ **Corrected 2026-08-06**: the
  sampler simply never set `observation.Request.Window`. Accessibility now reaches a
  targeted session — see [[director-accessibility-targeting]].
- Durable storage, fixture export and event streaming are **not built**, deliberately:
  nothing should become durable while the classifier was wrong.

## Milestone record

Recorded in `HANDOFF.md` rather than a dedicated milestone document.

## What a screen requires

Four separate things, and keeping them separate is the point:

| | requires |
|---|---|
| **existence** | a validated window, and one observed composition — a bounded set of (role, window-relative region) pairs from any admissible source |
| **place** | the surface it belongs to, and whether a part of that surface now holds a kind of thing it did not ([[ADR-039-a-surface-and-a-place-inside-it]]) |
| **recognition** | a durable structure signature matching a remembered subject, with a discriminator ([[ADR-016-cross-session-identity-is-structural-and-conservative]]) |
| **naming** | a word a person typed ([[ADR-031-the-user-names-the-stage]]) |
| **interpretation** | a hypothesis with its contradictions ([[ADR-014-hypotheses-are-evidence-not-identity]]) |

An unfamiliar screen exists without being recognised. A recognised screen exists without a
name. **No screen requires OCR text to exist** — terms are credited to a screen that already
exists, never the other way round ([[ADR-017-structure-earns-a-name-text-never-earns-structure]]).

This is what lets learning say `unknown screen A → action → unknown screen B` long before
semantic memory knows what either is.

## Where the composition comes from

`observe.StructuralView`, chosen by `StructureOf`: the fused authoritative world first, the
structural detector where fusion saw no structure at all. Provenance travels with it, because
*observed and empty* is a screen and *unobserved* is not. See
[[ADR-036-a-screen-is-a-composition-not-a-provider]].

Until 2026-08-10 the only source was the detector, which is opt-in — so the default Director
had no screen model for any application. The `Related systems` line below recorded that as a
property of the session rather than as the blocker it was.

## Two levels of identity

A composition is compared twice, and the second comparison runs only when the first said *same
surface*:

| | question | measured as |
|---|---|---|
| **surface** | is this the same application surface | weighted Jaccard over the whole signature, against a running mean |
| **state** | is this the same place inside it | mass on `role@col,row` keys that **arrived or left**, per region of the coarse grid |

One comparison could answer only one of them, and the one it answered was the one that cannot
see a local change: a live session watching somebody open a second view twice recorded a single
motionless screen. The second measures **composition, not amount**, because amount ranks the
cases the wrong way round — a list loading another page changes more structure than a panel of a
different kind, and means nothing.

A state carries `Surface`; a state with none is its own surface. `Surface`, `LocalFrom` and
`LocalCell` are all session-local — a counter, a counter and a grid coordinate — and nothing
durable is keyed on them.

### What survives a restart

Durable identity is **per local state, namespaced by application**: `stateFingerprint` reads one
`ScreenState`, and `SurfaceID` reaches nothing durable. Two places inside one application become
two remembered subjects, two names, two learned-play endpoints and two ends of a directed edge —
proved through the production paths rather than by construction.

The seam: the durable signature carries role composition, the local comparison carries
`role@col,row`. Two places differing only in **where** the same structure sits are two states in
a session and one subject in the store.

Local structural comparison is **bounded on purpose**. A change too small or too sparse to have
a composition does not define a new place; it may still be evidence, and it may still be a
property of the place it happened in. The bound is `MinLocalCellStructures`, and a measurement
across a whole corpus established that it — not spatial resolution — is what keeps a caret, a
pointer or one changed control from becoming somewhere new
([[ADR-040-a-few-scales-were-not-better-than-one]]).

**Recorded limit.** An overlay of the same kind of structure over the same kind of content is
invisible to both comparisons. At (role, coarse-cell) resolution it is the same observation as
more of that content arriving.

This is an **information** limit, not a resolution one, and that distinction was measured rather
than assumed: a bounded multi-scale field was built, given four overlapping scales and two
alignments, and saw exactly what the single grid saw. What is missing is a dimension the admitted
structural vocabulary does not carry — plausibly containment, layering, or the chronology of
things appearing and disappearing. Those are hypotheses about the gap, not planned work, and
nothing in Director implements or awaits them. See
[[ADR-039-a-surface-and-a-place-inside-it]] and [[ADR-040-a-few-scales-were-not-better-than-one]].

## Provider independence, and the escalation this leaves open

All four combinations produce the honest answer, and none is privileged by construction:

| accessibility | detector | screens from |
|---|---|---|
| useful | absent | the fused world |
| useful | present | the fused world; the detector is compared, never merged |
| unavailable | useful | the detector |
| unavailable | absent | none, and `StructureWhy` says which silence it was |

Because the choice is one pure function over one sample, a future perception policy —
cheap evidence, then stronger evidence only where the cheap answer was uncertain — has a
single place to express itself. Nothing in the current path assumes a fixed provider set, a
fixed order, or that every source runs on every sample: the detector's cadence gate already
declines most slots and the screen model is built anyway.

## Capture first, interpret second

`ShadowTotals.InputLog` is the bounded, ordered record of every admitted input event in a
session, banked before ANY gate — the structural return, the quiet expiry, the state change
that consumes the attribution buffer. It claims nothing about meaning; it is the record
that the person acted, with the context known at the time, and no failure of interpretation
can erase it. [[ADR-057-attributed-input-survives-interpretation]].

Session transitions carry `TargetedSequence` — the ordered run plus what each event was
aimed at, when the evidence identified it ([[ADR-058-a-demonstrated-target-may-keep-its-name]]).
The durable topology folds these down to plain `NavSequence`, structurally: a label read
off a screen cannot outlive the session by riding an edge.

## The transition substrate

```
composition ─▶ ScreenSegmenter.Observe        screenstate.go:479
                 │  similarity >= 0.55        ─▶ SAME screen        (:501)
                 │  coverage   >= 0.75        ─▶ held, promoted on recurrence (:505)
                 │  otherwise                 ─▶ mint a NEW screen  (:524)
                 ▼
               note(from, to, inputs)          transition + attribution (:667)
                 │  inputs drained per inference in ShadowTracker.Observe
                 ▼
               ShadowTotals.Transitions        session-local, numbered
                 ▼
               RelationshipsFrom               relationship.go:298 — BOTH endpoints must
                 │                             resolve to remembered subjects, or the edge
                 │                             stays session-local
                 ▼
               Memory.RememberRelationships    durable, directed, bounded
                 ▼
               AssessLearning                  closed refusals; never a cause
```

Attribution is **banked before** the structural gate and **drained on every inference**, so
navigation survives a slot nothing looked at and cannot be attached to a change several samples
later. A transition exists whether or not anything was seen before it; `Unattributed` is the
control evidence that keeps the correlated ones readable as correlations.

### Why an edge stayed session-local

`RelationshipReport` names it, in a **closed** vocabulary — `source_unresolved` ·
`destination_unresolved` · `both_unresolved` · `same_subject` — recorded per refused transition.
Added because a missing edge looks identical whatever caused it, and the first real Learn failure
on Windows Settings was diagnosed at exactly this boundary rather than guessed at.

### One unreadable sample no longer loses the route

Live Settings recorded a single movement as two halves across a sample it could not place:

```
state_1        → state_unknown    preceded {confirm: 1}     destination_unresolved
state_unknown  → state_2          unattributed 1            source_unresolved
```

`bridgeUnsettled` (`bridge.go`) recovers the adjacency A→B from that, and asserts nothing about
what the unplaced sample was. Bounded by `Crossing.Run < StatePromotionCount` — segmentation's own
line between a transition frame and a screen, not a number invented for the occasion — plus
`Continuity.Unbroken()`, two resolving endpoints, and `from != to`. Nine refusals in the
`BridgeRefusal` vocabulary say why not.
[[ADR-049-a-change-nobody-could-read-is-still-one-change]].

### …and neither does a walk with several of them

It recovered exactly **one** interval per session until 2026-08-17, because `Transitions` is keyed
by `(From, To)` and therefore cannot say what followed what. A two-step demonstration crosses two
transition frames, which aggregates to two entries into `state_unknown` and two exits out of it —
`A→C, B→B` fits the same counts as `A→B, B→C` — so it refused `ambiguous_interval` and lost BOTH
adjacencies. At ordinary human speed every step crosses a frame, so that was the common case, not
an edge case.

`ShadowTotals.Crossings` is the session's **walk**: every change, in order, written at the one
call site (`note`) every change already passes through. It is thin on purpose — no navigation, no
counts, no interpretation, all of which stay on the aggregate. `unsettledIntervals` reads it, so
each excursion has one entry and one exit by construction, and `A → ? → C → ? → B` now yields
`A→C` and `C→B` rather than a refusal — C stays in the middle. A truncated walk
(`EvictedCrossings > 0`) is refused outright rather than paired across the gap.
[[ADR-064-the-order-of-a-walk-is-evidence]].

## What a screen must have to be REMEMBERED

Existence is cheap; durability is not, and the requirement chain is worth reading in one place:

```
structure → track (exclusive to one screen, presence ≥ 0.80)
          → structural group (an even vertical run)
          → hypothesis about the screen
          → a person answers it            ← OR: an explicit `learn "…"` pass ends here
          → a DISCRIMINATING signature (read terms, or an envelope)
          → durable subject
          → and only then can it be one end of a relationship
```

Every link is a real refusal with a reason, and four of them were invisible until this was
traced: the track bound (fixed, [[ADR-038-session-bounds-are-sized-for-the-evidence]]), the
group's vertical-run requirement, the discriminator rule — and *"a person answers it"*, which was
the only door in — so an application nobody had happened to answer a question about was an
application nobody could Learn anything in. A licensed Learn pass is the second door, and
it persists identity only:
[[ADR-047-a-place-is-remembered-a-meaning-is-answered]].

## Learn: the same machinery, on purpose

Until now Marco had to **notice** a habit before it could learn one — a route needed independent
corroboration across sessions before the invitation policy would let it ask. `director learn
"open downloads"` reverses the timing without touching the evidence model.

```
director learn "open downloads" --window-id window_37

  Hold still a moment while I learn where we're starting.
  Okay — go ahead and show me.
  I'd like to see that once more, from where we started.
  I think I understand. Want me to try it once?
```

The coordinator (`internal/director/learn`) owns no evidence, no identity, no judgement and no
authority. Each line above is a phase; each phase waits on an object that already owns the fact —
`observe.PlaceNow` for where we are, the durable topology for what changed, the ordinary armed
`Capture` for the demonstration, `AssessCandidate` for whether it holds up.

The demonstration capture is a **confirmation** mechanism and cannot discover a destination, so
Learn runs two passes and arms the capture between them through the ordinary pending learning
request — see [[ADR-043-teaching-is-two-passes-not-a-new-capture]] for why that beats giving the
capture an open destination.

**Learn grants nothing.** It cannot press a key, and permission to rehearse is still a separate
yes through the ledger. `ready_to_rehearse` and `naming` are *waiting* phases: the coordinator
polls the question that already exists and prints the command that answers it.

The tail follows the same lifecycle to its end — the authorised rehearsal, the lowering judgement,
the naming questions the judgement demands, the one persistence path — and only a saved file lets
anything say `I learned "open downloads"`. `Session.Learned` reads the artifact, never the phase.

A Learn attempt is **one episode**: its passes fold their evidence but claim one independent
sighting between them, so asking for it explicitly cannot manufacture the corroboration the
invitation policy exists to require. See [[ADR-044-a-teach-attempt-is-one-episode]].

A Learn pass may also **establish a place** — make the screen the user is standing on durably
recognisable, persisting no judgement about what it is. That is the other half of the same fact:
`learn "…"` is an explicit human semantic event, and both consequences travel together on
`observesession.Episode`. Without it, Learn refused at its first step against any application
nobody had happened to answer a question about, and *which* question Marco raised was never the
user's to choose. At most one place per pass, only where it could ever be matched again, and
`Result.Places` says which of the closed reasons applied when none was.
See [[ADR-047-a-place-is-remembered-a-meaning-is-answered]].

## Where a person sees it

Ambient watching is background behaviour, so its state is a first-class answer rather than a
diagnostic — a mode nobody can see the state of is the shape of surveillance whatever its
intentions. `AmbientView` carries it, and the control centre's **Here** panel renders it beside
what Marco can currently see: whether Marco is watching, whether what it watches may become
**durable memory** (a separate agreement, and a separate switch — see
[[ADR-095-repeated-observation-may-become-knowledge]]), and what its store has actually committed.

The surface may ask and may not decide. Four verbs, all attention or permission; an unrecognised
one is a read rather than the nearest match. It writes nothing to semantic memory, and the learning
events it draws are the store's own after its own write committed, already worded — so it cannot
name a Place even by accident. See
[[ADR-112-the-loop-belongs-where-a-person-is-already-looking]] and
[[ADR-111-a-demonstration-takes-the-slot-from-watching]].

### Three product states, and learning is inside watching

What a person operates is not the two permissions. It is three states — **not watching**,
**watching**, **watching and learning** — with one status line each and a sentence saying what
Marco is doing. `LEARN ⊂ WATCH`: asking Marco to learn starts watching with it, stopping watching
ends learning, and stopping learning leaves attention alone. There is no user-facing
"learning but not watching"; the Director makes it unreachable and the strip reports the
combination as inconsistent rather than dressing it up as a mode somebody chose. See
[[ADR-113-learning-is-inside-watching]].

The two panels below the strip answer two different questions, and saying which is what teaches
the distinction that explaining Observe and Learn never did:

```
CURRENT       what Marco sees now
JUST LEARNED  what Marco wrote down
```

`HerePlace.Perceived` is what the screen in front says it is called — the settled reading, through
`observe.SettledPlaceNameFor`, written nowhere. So watching alone can say *this screen says it is
"Mouse"* while committing nothing, and the difference between seeing and remembering is visible
rather than documented.

## Naming a Place Marco already knows

A Place is established the first time Marco can recognise it; a name settles by RECURRENCE. The
two almost never coincide, so **naming is its own sweep** rather than something that happens while
establishing — `observe.PlaceNamesToRecord`, over every state the session has seen, whether or not
any of them is new.

Ambient watching runs that sweep on every reading when learning is on, through `promotion.call`,
the one door a semantic name reaches the store by. It needs no walk, no edge, no goal, no explicit
Learn and no second Place: `PlaceNamesToRecord` skips a state memory cannot recall and
`ObserveSemanticName` refuses a subject the store does not hold, so enrichment can never fork an
identity.

Before 38A.1 there was no such sweep, and the only naming write ambient could reach lived inside
`promotion.establish` — once, at Place creation, and only when an EDGE was promoted. The first
dogfood measured the result: forty-five durable Settings places, none named, while perception could
name them the whole time. The rule was right and nothing on the path a person uses called it.

## What ambient learning may keep, and what it produces

Two permissions travel with Watch & Learn, and only one of them used to arrive.

**Promotion** is asked in `ambient.Judge`: may this candidate become durable knowledge.
**The control's name** is asked much earlier, in `liveSampler.mayNameTargets`, at the push that
offers the window's controls to the navigation producer — and a candidate whose control has no
name is refused by rule (`Act.Representable` is false, and the verdict is
`Never / control_not_named`) whatever the promotion policy says.

`NameablePlaintext` is `{button, menu_item, menu, tab, checkbox, radio}`. Anything that navigates
by `list_item`, `tree_item`, `link` or `row` — Windows Settings, most file managers, most
sidebars — needs the licence, and ambient sessions declare none. So ambient learning could not
promote anything in those interfaces at all until `mayNameTargets` learned to read the ambient
learning switch as the second door to a permission `ambientPromotionLicence` had always declared.
Read live, per cycle, because somebody presses the button mid-session. See
[[ADR-114-watching-and-learning-may-keep-the-name-of-what-you-clicked]].

Watching alone still keeps no control name, and the shape filter is unconditional either way.

### Topology is learned; a goal is meant

Ambient promotion writes places and the ways between them. It writes **no goal** — `admitWatched`
is explicit about it — so a perfect traversal still leaves nothing a person can ask for by name.
The canonical way to close that gap is the retrospective Learn, `ObserveLearn{Recent: true, Name}`,
which promotes the walk, keeps the demonstration and records the goal in one act.

```
known Place  →  known way between screens  →  a name you gave it  →  Try it
```

### And learning says what it made of what you did

`AmbientEvidence` renders every candidate with the policy's own verdict and its own sentence, and
the control centre's Here panel draws it. Learned, seen again, or refused with the reason —
never silence, because a mode indicator with no observable result is a light rather than a
product. One rendering rule is enforced rather than assumed: a promoted candidate reports
`never / already_known`, so a surface reading the verdict alone would draw a refusal beside the one
relationship Marco actually learned.

## One experiment at a time

Marco holds at most one proposal: a promoted candidate edge it has never walked itself, chosen
deterministically and stated as a claim — from HERE, doing THIS, you arrive THERE — with a reason
made of the record's own fields. `Runtime.Experiment` is the read; it chooses nothing durable and
moves nothing, because observation permission is not actuation permission and neither is a page
refresh.

Running it is `Runtime.TestEdge`, and it is a projection of the performance path rather than a
second executor: `bringForward`, `freshLook`, `observe.PlanToGoal`, `performPlan`,
`confirmArrival`, the command registry, the per-step authority and the actuation lease, all
unchanged. What is new is that the SOURCE is required before the action — an experiment run from
the wrong screen tests nothing and presses a control nobody chose — and that the desktop is given
back afterwards, on every path out including the ones where Marco gave up.

Positioning uses the canonical planner with the canonical eligibility and does not explore. An
experiment whose source nothing connects to refuses, and says what it needed and where it was.

See [[ADR-115-one-experiment-and-the-desktop-given-back]].
