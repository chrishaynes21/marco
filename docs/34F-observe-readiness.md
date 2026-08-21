---
type: reference
status: active
updated: 2026-08-20
source_paths:
  - internal/director/observe
  - internal/director/observesession
  - internal/director/semanticmemory
  - internal/director/perception
  - cmd/director/observeregistry.go
  - cmd/director/observewiring.go
  - cmd/director/sightplace.go
  - cmd/director/learnsessionwiring.go
  - cmd/director/pointing.go
  - cmd/director/playbill.go
  - cmd/marco/learnui.go
---

# 34F Observe-readiness — what OBSERVE needs, and what already works

The next roadmap is **OBSERVE**: Marco continuously understands what is happening, with **LEARN**
as an explicit attention-and-admission licence layered on top of it. The closeout requirement for
34F is narrow and testable:

> Roadmap 35A must be able to introduce `marco observe` **without faking a Learn episode** and
> **without building a second perception system**.

This note traces the actual dependency graph and answers that question. Nothing here is
implemented. **Observe does not exist**; `director light` is the closest thing to it and it is an
instrument, not a loop.

Companions: [[34F-duplication-matrix]] §3 (there is no `Stage` type, and that is the consolidation
this needs most) and [[Passive-Observation]] (the subsystem as built).

---

## 1. The headline, stated plainly

**The stack is far less Learn-welded than it looks, and the one weld that exists is one boolean in
one line.** Light Mode already runs a passive observation session with a **zero `Episode`** — it
emits no input, answers no question and establishes no place — and place recognition already works
inside it.

`observesession.Episode` has three fields (`SameEpisode`, `EstablishPlaces`, `PermissionExpected`).
Verified against the current tree: **exactly one production site ever sets `EstablishPlaces` true**
— `cmd/director/learnsessionwiring.go:89`, inside `learnPasses.episode()`, whose own doc says
*"THE one place `EstablishPlaces` is ever set true"*. Every other session start passes the zero
value.

That single boolean then travels to two places, and from there **grants at least four distinct
licences**:

```
learnsessionwiring.go:89   Episode{EstablishPlaces: true}
        |
        +-- Config.Episode ---------> observesession/runner.go:1067   may this session make a
        |                                                             PLACE'S IDENTITY DURABLE?
        |
        +-- Config.Episode ---------> observesession/runner.go:1221   may a watched demonstration
        |                                                             create a CANDIDATE (and the
        |                                                             TARGETS it names)?
        |
        +-- observeregistry.go:143    ls.demonstration = episode.EstablishPlaces
                    |
                    +--------------> observe/sample.go:197            may a CLICKABLE control's
                    |                                                 NAME ride a semantic target?
                    |
                    +--------------> observe/sample.go:594            may Marco read what the place
                                                                      APPEARS TO BE CALLED?
```

**Splitting that boolean is a prerequisite for 35A, and it is stated here as a prerequisite rather
than as a nice-to-have**, because an ambient Observe will predictably need one of these licences
without the others. The failure mode is easy to write down in advance:

> *"Ambient observation should be able to name places, so set `ls.demonstration = true` for Light
> Mode."*

That one edit also widens **target-label admission across every clickable control on every screen
the person visits, for as long as the session runs.** `AdmittedTargetLabel`'s own doc is careful
that the widening is sound **because** the person explicitly asked and **because** it reaches only
the one control their own input landed on. Neither of those holds for a sweep. The recommendation
is small and it must land **before** anybody makes Light Mode smarter, not after: split the
sampler's `demonstration` field into two named licences — one for *place naming*, one for
*target-label widening*.

There is also a **second, undocumented place-establishment licence** that is not the Episode at
all: `cmd/director/sightplace.go rememberHere`, the Audience typing a name into the HERE panel
during Light Mode, which calls `store.EstablishPlace` directly and bypasses the runner. It is
defensible and arguably *stronger* than Learn's (a person typed a name), but
[[ADR-047-a-place-is-remembered-a-meaning-is-answered]]'s **Enforced by** section names only the
session path. The comment saying `EstablishPlaces` is set in one place is true about the *flag*
and misleading about the *capability*.

---

## 2. The dependency graph, mechanism by mechanism

Classification: **ALWAYS** = should work with no licence · **LEARN** = should require an explicit
Learn or an equivalent human semantic event · **CONFIG** = should be configurable · **POLICY** =
a 35A decision, not a fact.

| mechanism | requires Learn today | decided at | should be |
|---|---|---|---|
| **Accessibility sampling** | **No** | providers wired unconditionally in `cmd/director/runtime.go`; collected in `observewiring.go` | **ALWAYS** — already does |
| **OCR sampling** (scoped label reading) | **No** | budget only — `observesession/runner.go shouldReadLabels`, `LabelEvery` / `MaxLabelPasses` | **CONFIG** — it is, by budget, not by licence |
| **Vision / shadow detector** | **No** | opt-in by bridge availability, `cmd/director/runtime.go` | **CONFIG** — already is |
| **Fusion sampling** | **No** | `engine.Fuse(cycle)` in `observewiring.go`; the engine is built unconditionally | **ALWAYS** — already does |
| **Place recognition** (`PlaceNow`) | **No** | `internal/director/observe/place.go:58` — a pure projection over `ShadowTotals` and a narrow `Recogniser` | **ALWAYS** — already does |
| **Current Place** (the surface answer) | **No** | `cmd/director/playbill.go`, `sightplace.go withPlace`, `pointing.go placeNow` | **ALWAYS** — already does, and `sightplace.go` says so in as many words: *place recognition is not a property of Learn* |
| **Place *establishment*** (identity persisted) | **YES** | `observesession/runner.go:1067` (`if !cfg.EstablishPlaces`) — **plus** the second, non-session licence at `sightplace.go rememberHere` | **LEARN**, or an equivalent human semantic event — which is exactly what the second licence is |
| **Target recognition** (recall of a durable target) | **No** | `observe/recall.go` → `compareTargets`; surfaced at `cmd/director/pointing.go targetsHere` | **ALWAYS** — already does |
| **Target *creation*** (`RememberTarget`) | **Partly** — two licensed doors, neither ambient | `runner.go:1221`'s `watchedDemonstration` (Learn-gated) and `finishCapture` (gated on an **approved** `LearningRequest`) | **LEARN** — a target's *label* is the person's data |
| **Transition / crossing detection** | **No** | a fresh `observe.NewAnalyzer` per session; totals accumulate in `stats.Shadow` on every sample | **ALWAYS** — already does |
| **Evidence accumulation → durable relationships** | **No licence check**, but a hard *precondition* | `runner.go` calls `rememberRelationships` unconditionally; `observe/relationship.go:466` requires **both** endpoints to `Recall` as `Established` | **ALWAYS** — it does, but it is a **no-op** until places exist. See Q3 |
| **Semantic naming inference** (what the place appears to be called) | **YES — hard** | `observe/sample.go:594`: `if !demonstration { return "" }` | **POLICY** — the single most Learn-welded mechanism in the stack |
| **Target-label admission, widened** | **YES — partially** | `observe/sample.go:197`: `if !eligible(role) && !(demonstration && role.Clickable())`. Outside Learn only plaintext-nameable roles carry text | **LEARN** — the widening is a privacy decision, argued at the definition |
| **Persistence / admission of semantic knowledge** | **No** | `runner.go` `memory.Remember` on a person's answer, and on a revision. Neither reads `Episode` | **ALWAYS** — the licence is a **human answer**, not a Learn session |
| **Candidate creation** | **Two doors, both licensed** | `runner.go:1221` (`if !ep.EstablishPlaces`) — Learn only; and `armCapture`, which needs a **pending `LearningRequest`**, i.e. a person answered "yes, learn this" to an invitation | **LEARN** — the second door is the passive-invitation form of the same consent |
| **Candidate promotion** (offer to learn a habit) | **No** | `runner.go reviewLearning` → `observe.ReviewRelationships`; thresholds `MinSessions: 2, MinObservations: 3` | **ALWAYS** — this *is* the ambient path, and it is already built |
| **Rehearsal question** | **No** to raise; a soft weld on budget | `reviewRehearsal` runs on every session; `PermissionExpected` only **exempts** the demonstrated route from the one-question budget | **ALWAYS** to raise; the exemption is **LEARN**. Correct as built |
| **Rehearsal execution** | **No** | a grant plus `Runtime.Rehearse`; the grant exists only after a person's yes (`authorizeRehearsal`) | **LEARN**-equivalent consent — and consent is not the same thing as Learn |
| **Play creation** (lower → compile → save → register) | **No** | `cmd/director/learnedplay.go LearnedPlay`, reachable from `director learned --save --register` **with no session at all** | **ALWAYS** — already does |

### What is NOT welded that a reader would expect to be

- The **whole sampling pipeline** — accessibility, window system, OCR, vision, fusion — is
  constructed once in `cmd/director/runtime.go` and knows nothing about Learn.
- **Light Mode already exists and is precisely "ambient observation, no acquisition."**
  `sightplace.go watchHere` starts the ordinary passive session with a zero Episode, and its doc
  says it *"emits no input, answers no question and establishes no place."*
- `Runtime.withPlace` is called on the idle path as well as the Learn path, pinned by
  `TestHereIsPresentWithoutALearningSession`.
- **Durable relationship writing happens on every session end regardless of licence.** Only
  `SameEpisode` is stamped from the Episode.

---

## 3. The three gating questions

### Q1 — Can `PlaceNow` operate with no active Learn episode?

**YES. Proven, and already running in production.** The Watch button in the control centre's Learn
panel sends `ObserveLearn{Watch:true}` → `Runtime.watchHere` → `StartObservation` with a **zero
Episode** → the runner's sample loop → `observe.PlaceNow`, and the answer surfaces through
`playbill.go` → `withPlace`, rendered by `director light` and the control centre's HERE panel.

`PlaceNow` takes shadow totals, an application name, a `Recogniser` and thresholds. It has **no
parameter through which a Learn session could travel**, by design — the narrow `Recogniser`
interface exists precisely so that "where are we" cannot become one of several answers.
**Not a 35A blocker.**

**The caveat that is a blocker, and it is a different one.** `cmd/director/perform.go placeNowIn`
returns empty unless `r.observations.ActiveID() != ""` — a Place can only be recognised **while an
observation session is running** — and `freshPlace` works around that by *starting a session* and
polling. So "where am I" is not a standing fact today; it is a side effect of opening an episode.
That is a **session-liveness** dependency, not a Learn dependency, and inverting it is the actual
shape of 35A.

### Q2 — Can Target recognition operate with no active Learn episode?

**Recognition: YES. Creation: NO — and correctly so.**

Recognition is a pure comparison against the store (`observe.Recall` → `ExplainStructure` →
`compareTargets`). The production surface takes shadow totals from the active session **or the most
recently finished one**. No Learn anywhere on that path.

Creation has exactly two production call sites, both inside `Runner.rememberTargets`: one inside
`watchedDemonstration` (Learn-gated) and one inside `finishCapture` (reachable only after a person
approved a `LearningRequest` — consent-gated, not Learn-gated). And the *label* a target carries is
separately gated: under a passive session only plaintext-nameable roles carry text at all.

**This is the right shape and it should stay.** An ambient Observe that minted a target from every
clickable control it saw would be building a durable index of the person's files, friends and
messages. **Not a 35A blocker; a 35A design constraint to preserve.**

### Q3 — Can transition evidence be produced without inventing a Learn session?

**YES structurally. NO practically, on a cold install — and that is the one genuine 35A gate.**

Structurally, `Runner.Run` calls `rememberRelationships` unconditionally, and
`observe.RelationshipsFrom` takes no licence argument at all. The only Episode influence is
`SameEpisode` being stamped onto each observation. **No fake Learn session is required.**

Practically, `observe/relationship.go:466` resolves each endpoint through `Recall` and refuses
unless the verdict is `Established`:

```go
if !rec.Verdict.Established() || rec.Subject.ID == "" {
```

A durable edge therefore needs **both** endpoints to already be established places — and places
become durable through exactly two doors: the runner's `EstablishPlace` behind the Learn licence,
and the Audience typing a name into the HERE panel.

So on a fresh `$MARCO_HOME`, an ambient session accumulates transitions, generates hypotheses,
reports both endpoints unresolved, and **writes nothing**. The ambient learning loop that
`reviewLearning` implements — *seen twice across sessions, three observations, offer to learn it* —
**cannot start itself.** Nothing is Established on a fresh install, and nothing ambient may
Establish.

**This is the cold-start bootstrap problem, and it is a POLICY / ADR decision for 35A, not a 34F
fix.** The smallest possible change — letting an ambient session establish places — is precisely
what `Episode.EstablishPlaces` and [[ADR-047-a-place-is-remembered-a-meaning-is-answered]] exist to
prevent, because it makes durable storage grow with **observation** rather than with **human
semantic events**. Three options exist and 35A must choose one deliberately, with an ADR:

1. keep `rememberHere` as the only ambient bootstrap and make it far more prominent — today it is
   one button in one panel;
2. add a **third**, bounded ambient establishment licence, with its own storage bound and its own
   ADR;
3. let ambient sessions establish a place only for a screen they have **re-recognised N times
   within the session** — moving the bound from *who asked* to *how often seen*.

None of these is inside 34F scope, and none of them should be chosen by an implementation detail.

---

## 4. The real 35A blockers — none of them is the Learn licence

| # | constraint | where | consequence for continuous understanding |
|---|---|---|---|
| B1 | **A session must name exactly ONE window.** A zero selector is refused | `perception/windowref/selector.go` | there is no "watch the desktop" session shape at all. `watchHere` works around it by awaiting a settled foreground window and pinning that one |
| B2 | **The session DIES when the window goes.** No substitution | `observesession/runner.go` — *"no other window was substituted"*; `ReacquireWindow` expiry | alt-tabbing away for longer than the reacquire window ends observation. Ambient use means *following* the foreground, which nothing does |
| B3 | **One session at a time**, refused rather than queued | `cmd/director/observeregistry.go` | Light Mode and Learn already collide; `yieldWatching` is hand-written arbitration for the only pair that exists. A third continuous consumer needs a real scheduler |
| B4 | **A 15-minute hard maximum** on a session | `observe/observe.go MaxDuration`; Light Mode sits exactly at it | ambient observation means re-arming every 15 minutes; nothing does that |
| B5 | **Bounded accumulation is per-session and in memory** | `MaxRetainedSessions = 10`, in memory only; `MaxSnapshots` / `MaxTransitions` caps | continuous understanding needs a decay / rollover model, not a bounded buffer discarded at ten sessions and lost on restart |
| B6 | **No CLI door to ambient observation of the foreground.** `director observe-game` requires a selector and is named for games; `director light` is a **read** that starts no session | `cmd/director/observecmd.go`, `lightcmd.go` | the only production entrance that starts foreground-following ambient observation is **the Watch button in the Learn panel**. That is a surprising home for the system's most strategically important capability |
| B7 | **Place recognition is gated on an ACTIVE session**, not on a Learn licence | `cmd/director/perform.go placeNowIn` / `freshPlace` | "where am I" is a side effect of opening an episode rather than a standing fact. Q1's caveat |
| B8 | **There is no `Stage` type.** Evidence selection in front of `PlaceNow` is decided independently by each of its callers | [[34F-duplication-matrix]] §3 | until one `Stage.Now()` owns "which evidence, which application, which window", there is no single answer to *what is Marco looking at* — which is the precondition for continuous understanding |

**These eight are the 35A work.** They live in `observesession`, `observeregistry`, `windowref` and
`cmd/director`, and **none of them requires touching the Learn licence, the privacy classifier or
the Play lifecycle.**

---

## 5. Privacy — the four tiers, and what must not be weakened

The separation between transient perception and durable memory is **real and typed**, not
conventional. It has four tiers.

| tier | representation | lifetime | bounded where |
|---|---|---|---|
| **1. Inferred now** — transient perception | `observation.Cycle` → fused `WorldState` → `observe.Sample` | one sample; one copy held in process; a bounded ring for diagnostics | **never written to disk.** The window rectangle is deliberately dropped from `observe.Sample` and kept only in process |
| **2. Observed repeatedly** — session-local belief | `ShadowTotals`, analyzer findings, hypotheses, the proposal ledger | one session, in memory; the last ten results retained **in memory only** — a service restart loses them | max snapshots / transitions / hypotheses |
| **3. Candidate evidence** — durable, but not knowledge | `RememberedRelationship` (**counts only**: observations, sessions, preceded, unattributed), procedure candidates, rehearsal evidence, learning requests | durable, in `semantic-memory.json` | written once per session; sequences are **stripped to plain runs at the durable boundary, structurally** — the store's sequence type has nowhere to put a target, so a label read off a screen **cannot outlive the session by riding an edge** |
| **4. Admitted durable knowledge** | `RememberedSubject` + settled `SemanticKnowledge`, `ScreenName` (the Audience's own word), durable targets, goals, and finally a saved + registered `.marco` with its `.origin.json` | durable | the licence table below |

The typing does the work: `observe.PlaceStore` can write a place's **identity and nothing else**;
`observe.TargetStore` can write a target's identity and nothing else — *"no SemanticKnowledge here,
no relationship, no goal and no authority."* The runner is handed exactly the narrow interfaces it
needs.

The distinction is **conceptual as well as structural**: a `RememberedRelationship` carries
*observations* and *sessions* as separate integers and the policy reads them separately, because
*"a habit seen ten times in one sitting and a habit seen four times on three separate days are
different claims."*

### Where the persistence licence is enforced

| licence | enforced at |
|---|---|
| text may become a plaintext label at all — structural role eligibility, then a shape filter | `observe/sample.go` `eligible` → `Classify` → `safeLabelText` |
| a control's name may ride a semantic target | `observe/sample.go:190 AdmittedTargetLabel`, gate at `:197` |
| a screen's apparent name may be read at all | `observe/sample.go:593 AdmittedPlaceName`, gate at `:594` |
| a place's identity may become durable | `observesession/runner.go:1067` (the licence) + `semanticmemory`'s `Discriminating()` — a record that could never be matched again is refused |
| the Audience may establish a place by naming it | `cmd/director/sightplace.go rememberHere` — **the second licence, and the undocumented one** |
| a judgement about meaning may become durable | `runner.go` — **only** on a person's answer to a proposal, and on a revision. `Store.Remember` has no other production caller |
| a demonstration may become a candidate | `runner.go:1221` (Learn) or `armCapture` (an approved invitation) |
| a candidate may become authority to act | `runner.go authorizeRehearsal` — the ONE yes→grant site |
| a verified route may become a Play | `observe/lowering.go` (refuses when not verified) then `driver.CheckSource` |
| Marco may ask what a screen is called | `cmd/director/learnedplay.go` — **only** when a verified Play is blocked on it. Explicitly not a sweep |

Two further structural guards: raw human text becomes a `ScreenName` at exactly one conversion
point, and `EvidenceSource.Authoritative()` returns true **only** for `FromUser` — perception may
supply evidence and can **never** correct a person.

### What must not be weakened

1. **Durable storage grows with human semantic events, not with observation time.** Every option in
   Q3 must keep an explicit bound; option (iii) in particular must state its bound in an ADR.
2. **The label seam.** Sequences are stripped to plain runs at the durable boundary *structurally*,
   because the store's type has nowhere to put a target. Do not add somewhere to put one.
3. **The two licences that are currently one boolean must become two.** Granting ambient place
   naming must not grant ambient target harvesting. This is the single highest-risk edit in 35A and
   it is ten lines.
4. **Perception may never correct a person.** `Authoritative()` stays `FromUser`-only.
5. **`rememberHere` needs to be written down.** It is a real, defensible, second establishment
   licence with no ADR coverage, and [[ADR-047-a-place-is-remembered-a-meaning-is-answered]]'s
   *Enforced by* names only the session path.

---

## 6. The answer to the closeout question

**Can 35A introduce `marco observe` without faking a Learn episode?** — **Yes.** Every perception
mechanism it needs (sampling, fusion, place recognition, target recognition, transition detection,
relationship accumulation, candidate promotion) already runs under a zero Episode, and Light Mode
proves it in production today.

**Without a second perception system?** — **Yes, and it must.** There is exactly one sampler, one
fusion engine, one `PlaceNow`, one `Recall`. What is missing is not perception; it is a **session
shape** — foreground-following, restartable, unbounded in wall-clock, with one owner of evidence
selection in front of `PlaceNow`.

**What 35A must decide before it writes code**, each with an ADR:

1. **The cold-start bootstrap** (Q3). Nothing is Established on a fresh install and nothing ambient
   may Establish, so the ambient loop cannot start itself. Choose one of the three options above.
2. **Splitting the one boolean into named licences** — place naming versus target-label widening.
3. **Session shape** — B1 through B6: following the foreground, surviving a window's death,
   scheduling more than one consumer, re-arming past 15 minutes, and a decay model for accumulation
   that survives a restart.
4. **One `Stage.Now()`** — B8, and it is also the top of [[34F-duplication-matrix]]'s backlog.

None of these is a 34F fix. All four are the shape of 35A.

## Related

- [[34F-legacy-marco-product-audit]] · [[34F-duplication-matrix]] · [[34F-legacy-matrix]]
- [[Passive-Observation]] · [[Perception]] · [[Semantic-Memory]] · [[Demonstrations]] · [[Roadmap]]
- [[ADR-047-a-place-is-remembered-a-meaning-is-answered]] ·
  [[ADR-065-operating-marco-is-not-demonstrating-to-it]] ·
  [[ADR-068-the-theater-is-the-durable-semantic-world]] ·
  [[ADR-037-opt-in-is-enforced-on-every-door]]
