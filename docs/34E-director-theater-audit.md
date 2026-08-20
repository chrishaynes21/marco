---
type: reference
status: active
updated: 2026-08-18
source_paths:
  - cmd/director
  - cmd/marco/theaterwiring.go
  - internal/platform/theaterhost
  - internal/theatermod
  - internal/director/rehearse
  - internal/director/observe/theater.go
  - internal/director/observe/place.go
---

# Roadmap 34E — Director / Theater ownership audit

Read-only. No code moved, renamed or changed.

## The finding that reframes everything

**Theater is fully built, wired into the engine, emitted by the play generator, and has never
executed a single production.**

It lives entirely on the `cmd/marco` side:

| piece | where | reached by |
|---|---|---|
| `theatermod` | `internal/theatermod` | `use theater.` in a generated play |
| `theaterhost` | `internal/platform/theaterhost` | `hosts["Theater"]` in `cmd/marco/assistant.go` |
| `AccessibilityActor` | same | cast by `Theater.Activate` |

`cmd/director` imports none of it. Director's live production path is:

```
rehearse.Live → marcoexec.Operation → runtime.Host("Accessibility")
```

Theater's path only runs when a **saved play** executes `do Theater's Activate`. No play has ever
completed the learn lifecycle, so that path has never run outside tests.

**Director is the brain; Theater is the body — and the brain grew its own hands first.**

### The proof, in one pair of functions

Both of these mean "press this control":

| | fallback chain | fixed |
|---|---|---|
| `rehearse.pressThrough` | Invoke → Select → Expand → Toggle | today |
| `theaterhost.AccessibilityActor.Perform` | Invoke only | **no** |

`Perform` still has the exact defect that killed four live runs. Every Windows Settings
navigation item is a selection item, so a saved play replayed through Theater would fail with
`unsupported: the control does not implement InvokePattern` — the bug we just spent an afternoon
finding, sitting unfixed on the path that runs saved plays.

Two implementations of one capability, drifting, and only the unused one is broken.

### The same shape, twice more

**Verification.** `theaterhost.New(nil, actors...)` — the production wiring passes a **nil
Verifier**, so Theater always reports `not_verified`. Meanwhile `rehearse.classifyOutcome` is a
real, careful verification with a closed vocabulary and a settle rule. Theater owns verification
in the ADR and has none; Director has one it cannot lend.

**Casting.** Theater has `Casting`. Director's rehearsal has no actor concept at all — it emits an
`Operation` and the host answers. There is one actor either way, so nothing has broken yet.

## "Where are we?" is answered in seven places

`observe.PlaceNow` — durable place recognition — is called directly from:

```
cmd/director/playbill.go:201        currentFrom          (the live account)
cmd/director/pointing.go:496        placeNow             (Sight's sentence)
cmd/director/pointing.go:546        placeNowSubject      (HERE's handle)
cmd/director/pointing.go:650        targetsHere          (what can be acted on)
cmd/director/reach.go:51            reach                (planning)
internal/director/observesession/runner.go:1651
internal/director/teach/teach.go:598
```

Plus `rehearse/live.go:732` calling `memory.Recall` directly for its own answer.

Nothing is *wrong* with any one of them — they all use the canonical matcher, which is why they
agree today. But "where are we" is asked eight times by five packages, and this afternoon two of
them **did** disagree: `placeNowSubject` resolved a subject while `currentFrom` said `unknown`,
because one asked the matcher and the other asked the proposal ledger. That produced "a screen
Marco has no name for" for a place it held, and offered to remember it twice.

That is the duplicated-verdict risk, already realised once.

## `director light` — mixed, and mostly Theater

Traced: `runLight` → `Client.Playbill` → `Runtime.Playbill` → sections.

| section | what it answers | owner |
|---|---|---|
| `Current` (place, recognition, screen name) | what is on Stage | **Theater** |
| `Seeing` (structure, terms, detections) | what perception produced | **Fusion** |
| `Offers` (what can be acted on) | what Actors can reach | **Theater** |
| `Thinking` (hypotheses, proposals) | what Marco is working out | Director |
| `Learning` / `Teaching` | lifecycle state | Director |
| `Question` | what is being asked of the Audience | Director |
| `Doing` | the command in flight | Director |
| `Recent` (moments) | narration | diagnostics |
| refresh cadence, rendering | presentation | UI |

`DIRECTOR_LIGHT_IS_MIXED`. The **Stage half** (Current, Seeing, Offers) is Theater's live account.
The **reasoning half** (Thinking, Learning, Teaching, Question, Doing) is genuinely Director's.

Light is not "Director with fewer sensors". Nothing in `runLight` selects sensors — it is a read of
one account at a chosen interval. **Light is a perception configuration, not a mode of Theater.**
Accessibility-only versus Accessibility+Vision+OCR is a property of how the Stage is *constructed*,
and neither Director nor the command should know which.

## Target ontology — five unrelated meanings

| type | meaning | layer |
|---|---|---|
| `observe.SemanticTarget` (role + label) | what a captured click landed on | perception evidence |
| `observe.SubjectTarget` + `TargetSignature(place,label,kind)` | a durable thing that can be acted on | **Repertoire** |
| `theaterhost.Target` (Name + Kind) | what a play asks for | intent, in the play |
| `theaterhost.Candidate` (Handle) | one live resolution of that | **live Stage** |
| `marcoexec.Operation.Element` | a runtime element id | execution handle |
| `Control.Name` | a name sent over the bridge | executor detail |

The ontology is already correct and already documented in ADR-068. What is missing is that
**Director's rehearsal path uses none of it**: `rehearse.go:477` builds an `Operation` with an
`Element` resolved by `controls.ResolveControl`, entirely outside the Target model. So durable
Targets exist, are stored, and are consulted by nothing that actually acts.

## Authority — clean, and worth protecting

`NewRehearsalGrant` is in `internal/director/observe`. Theater has no authority concept, creates
none, and consumes none — it is handed a Candidate and performs. `BeginAttempt` spends the grant
in `rehearse`, before anything reaches a host.

**The invariant already holds: Theater consumes, Director/Audience manufacture.** The migration
must not disturb this. The one risk is that moving production into Theater tempts a second
foreground/safety gate inside it; there is already one in `rehearse` (`window_not_in_front`) and it
belongs where the attempt is bounded.

## Learn — a deliberate split, currently collapsed

| responsibility | today | should be |
|---|---|---|
| "learn X", interpret as a goal | Director (`teach`) | Director |
| decide an episode begins, ask questions, take authority | Director | Director |
| watch the Stage, record transitions | `observesession` | Theater (Stage) |
| identify Targets, establish durable places | `observe` + `semanticmemory` | Theater (Repertoire) |
| derive the production, generate the play | `marcoexec/play.go` | Theater (Repertoire) |
| rehearse it | `rehearse` | Theater (Production) |
| verify it | `rehearse.classifyOutcome` | Theater (Verification) |
| decide what the result means for the goal | Director | Director |

Learn is a Director **workflow** that teaches Theater. Today Director performs the middle six rows
itself.

## Responsibility matrix

| | current owner | should own |
|---|---|---|
| Audience intent | Director | Director |
| Goal interpretation | Director (`teach`, `goal`) | Director |
| Goal planning / reach | Director (`plan`, `reach.go`) | Director |
| Current Stage | Director (`playbill.currentFrom`) | **Theater** |
| Raw observations | providers | providers |
| Fusion | `perception/fusion` | Fusion |
| Current place | Director (5 call sites) | **Theater** |
| Durable place identity | `semanticmemory` | semanticmemory (via Theater) |
| Place recognition (matcher) | `observe.CompareStructure` | **Theater** consults it |
| Durable Targets | `observe/theater.go` + store | **Theater/Repertoire** |
| Live Target resolution | `rehearse` + `theaterhost.Find` (two) | **Theater** |
| Target ambiguity | both, independently | **Theater** |
| Repertoire | `semanticmemory` | Theater over semanticmemory |
| Learn episode coordination | Director (`teach`) | Director |
| Demonstration observation | `observesession` | **Theater (Stage)** |
| Capability extraction | `observe` | **Theater (Repertoire)** |
| Candidate/rehearsal knowledge | `semanticmemory` | Repertoire |
| Rehearsal permission | `observe` grant | Director/Audience |
| Rehearsal execution | `rehearse.Live` | **Theater (Production)** |
| Actor discovery | `theaterhost` (unused) | Theater |
| Actor casting | `theaterhost` (unused) | Theater |
| Action execution | `marcoexec` via both paths | Theater → marcoexec |
| Action verification | `rehearse.classifyOutcome` | **Theater (Verification)** |
| Goal satisfaction | Director | Director |
| Grounding semantics | `observe` + `pointing` | **Theater** |
| Grounding presentation | `cmd/director` + panel | UI |
| Sight / HERE data | Director (`sightplace.go`) | **Theater**, rendered by UI |
| Questions / clarification | Director | Director |
| Authority creation | `observe` | Director/Audience |
| Authority consumption | `rehearse` | Theater |
| Semantic memory persistence | `semanticmemory` | semanticmemory |
| Generated plays | `marcoexec/play.go` | Repertoire |
| Diagnostics | everywhere | presentation |

## Call graph: "Open Mouse Settings"

**Today** — one path for rehearsal, a different one for a saved play:

```
Audience → Learn panel → Director.Learn
  Director: teach coordinator
  Director: observesession watches, establishes places, stores candidate
  Director: observe judges, raises the rehearsal question
  Audience: yes
  Director: observe mints the grant
  Director: rehearse.Live claims it, resolves the control, presses, verifies
                                            ↑ Theater not involved

(if a play were ever saved)
Audience → marco do → runtime → do Theater's Activate
  Theater: Find → cast → Perform → verify(nil) → not_verified
```

**Desired** — one production path, entered from two places:

```
Audience → Director
  Director: goal "Open Mouse Settings"
  Director → Theater.Stage()            "what is on stage?"
  Theater  → place, targets, actors, confidence, refusals
  Director: chooses the production (Repertoire consulted for what CAN be done)
  Director → Theater.Perform(production, authority)
  Theater  : resolve targets → cast actor → perform → verify
  Theater  → Production{performed, cast, verified, refusal, steps}
  Director : does that satisfy the goal? ask / continue / report
Audience
```

Rehearsal becomes `Theater.Perform` with a one-attempt authority, rather than a second engine.

## Proposed APIs

```go
// Director → Theater: what is on stage right now?
// A READ. No sampling of its own; a projection over the current world.
type Stage interface {
    Now(ctx) StageAccount
}
type StageAccount struct {
    Application string
    Place       PlaceAccount   // durable id, name, recognition verdict, why
    Targets     []Target       // what can be acted on here
    Actors      []ActorStatus  // who could act, and whether available
    Freshness   Freshness      // how recent, how degraded
    Refusals    []Refusal      // why anything above is missing
}

// Director → Theater: put this on.
type Production interface {
    Perform(ctx, Request, Authority) Report
}
type Report struct {
    Performed bool
    Cast      string
    Steps     []StepAccount   // expected vs observed, per step
    Verified  bool
    Refused   Refusal
    Stage     StageAccount    // where it left things
}
```

`Report.Stage` matters: Director's next decision needs where it ended, and returning it removes
the "now go and ask again" round trip that currently re-derives place five ways.

## Migration phases

Each keeps the suite green, preserves behaviour, is mutation-gatable, and needs no live test until
the seam is ready.

**Phase 1 — one answer to "where are we".** Introduce `Theater.Stage.Now()` as a projection over
the existing account. Repoint the five `PlaceNow` call sites in `cmd/director` at it. No behaviour
change; the matcher is untouched. Acceptance: the five surfaces return identical answers to today,
and a mutation making one disagree fails a test. *Semantic ownership change only — no package
moves.*

**Phase 2 — one press.** Give `theaterhost.AccessibilityActor.Perform` the fallback chain, and
have `rehearse.pressThrough` call the Actor rather than its own loop. Kills the live drift.
Acceptance: the existing press tests pass against the Actor. *API boundary change.*

**Phase 3 — lend Theater the verifier.** Pass Director's real verification into
`theaterhost.New` instead of nil, so Theater stops reporting `not_verified` for everything.
Acceptance: a saved play reports a real verdict in a dry run. *API boundary change.*

**Phase 4 — rehearsal through Production.** `rehearse.Live` becomes a caller of
`Theater.Perform` with a one-attempt authority, keeping the grant, the scope checks and the
step bounds where they are. Acceptance: every existing rehearse test passes unchanged. *Package
move justified for the classifier only.*

**Phase 5 — Light is a perception configuration.** Make Accessibility-only versus fused a property
of Stage construction, not a command. `director light` keeps its name and becomes a thin reader of
`Stage.Now()`. *Semantic ownership change; no rename.*

**Phase 6 — remove the shadow copies.** Delete Director's now-unused place resolution, target
resolution and outcome classification. Acceptance: nothing references them; the suite is green.

Order matters: Phase 1 makes the disagreement impossible, Phase 2 stops the drift that already
exists, and Phases 4–6 are safe only once both hold.

## What must NOT move

- **Authority.** `NewRehearsalGrant`, the scope checks and the one-attempt rule stay in Director.
- **The matcher.** `CompareStructure`/`ExplainStructure` stay in `observe`; Theater consults them.
- **Fusion.** Theater asks it; it does not absorb it.
- **Goal reasoning.** `plan`, `goal`, `reach` are the brain.
- **Marco the language.** Theater is an act with one capability; the governance rule stands.

## God-object risk

Theater's five parts have clean contracts today (`Actor`, `Verifier`, `Theater`, `Target`,
`Production`) and total roughly 400 lines. The risk is not size but **Stage becoming a second
WorldState**. Stage must be a *projection* over the existing world account with no storage and no
sampling of its own — that is the one constraint that keeps this from producing a parallel
architecture.
