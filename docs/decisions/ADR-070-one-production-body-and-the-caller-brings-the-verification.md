---
type: decision
status: accepted
date: 2026-08-18
supersedes: []
affects:
  - learned-plays
  - demonstrations
  - marco-boundary
  - service
source_paths:
  - internal/production/production.go
  - internal/platform/theaterhost/perform.go
  - internal/platform/theaterhost/run.go
  - internal/director/rehearse/rehearse.go
  - internal/director/rehearse/live.go
  - cmd/director/rehearserun.go
  - cmd/marco/theaterwiring.go
---

# ADR-070 — one production body, and the caller brings the verification

There were two bodies. A saved play asking for a control reached the Theater; a live rehearsal
asking for the same control did not. Each had its own target resolution, its own execution and its
own idea of whether it had worked — and the first live run after [[ADR-068-the-theater-is-the-durable-semantic-world]]
proved they had already drifted: the Theater knew a Windows Settings navigation item is a
*selection* item, and the rehearsal, which is the path that actually runs during teaching, did not.
Every navigation step on the most obvious application anybody would teach Marco with ended at
`cannot_express: the control does not implement InvokePattern`.

The two paths were then given the same activation ladder ([[ADR-069-a-name-is-authored-and-can-be-taken-back]]
landed alongside `internal/activate`), which fixed the symptom and left the cause: two bodies that
have to be kept in step by hand. This settles the cause.

## Decision

**One production body, entered from both places, through a neutral contract.**

`internal/production` is the contract: `Request` (what to act on, which window, what the caller
expects), `Authority` (a permission that can be spent once), `Verifier` (what will check the
result), `Report` (facts, no policy) and one closed `Refusal` vocabulary. It holds types and
interfaces and no machinery, because a helper there would be a third place that knows how
productions work.

`theaterhost.Theater` implements it. The Director is handed a `production.Producer` by whoever
composed it, exactly as it is handed a `MarcoRunner` — `internal/director/rehearse` still imports
nothing that can act.

Three things follow, and each is the point of the decision rather than an implementation detail.

**An Actor expresses its part as legal Marco and performs nothing.** It writes the program;
the Production boundary runs it through an injected `MarcoRunner`. Every real input therefore
passes through Marco compilation, which returns a compile error *before any desktop mutation* —
which is what makes attempting an activation the control may not implement safe, and what gives a
dry run something to record. An Actor that called a host itself would be a second route with no
compile gate.

**Authority is spent, not asserted.** The Theater is handed something that refuses a second claim
and refuses a different target. Every bound that makes a rehearsal safe — the input budget, the
unobservable budget, one step then look — is checked before the boundary and none of it is visible
from inside it, so the Theater is given a permission rather than trusted.

**The verifier travels with the request, not with the Theater.** One Theater serves a saved play
run from a standalone runtime, which has no observation stack and honestly brings nothing, and a
live rehearsal, which brings the Director's own settled observation. A verifier installed at
construction would be one caller's answer applied to another's production — and, with both callers
live in one process, a race over a shared field. Nil is a real answer and produces `not_verified`;
it must never produce success.

**And the old path is gone.** `liveControls`, the element-id resolution in `LowerStep`, and
`pressThrough` are deleted rather than kept as a fallback. A fallback is how two bodies come back:
the first time the Theater refuses something, somebody reaches for the other one.

## Consequences

- A demonstrated click crosses the boundary as a name, a kind and a window. No runtime element id
  survives the gap between deciding and doing, which is the stale-identity failure that reads as
  flakiness.
- A rehearsal press gets window scoping, the activation ladder and the ambiguity refusal for free,
  because they are the Theater's and there is only one Theater.
- `theaterhost.Verifier` (`Changed(ctx) bool`) is gone. It was the second idea of verification in
  the same file, and with a nil verifier it made every standalone saved play report `failed`.
- A live rehearsal no longer observes twice. Its verification IS `observeOutcome`, lent to the
  production; the loop asks its own verifier whether it ran rather than taking the report's word,
  so a producer that claimed to have verified without asking cannot leave a step with no outcome.
- `production.Report` carries the program that ran, because the Actor writes it now and a dry run
  still has to show what it would have sent.
- `internal/director/uiact` still lowers control activations of its own for the UI action path.
  That is a separate caller and a separate migration; it is named here so it is not mistaken for
  having been covered.

## Recursion

A cast program is ordinary Marco, and ordinary Marco may say `do Theater's Activate`. If it could,
every level would be authorised by the one above it — unbounded, and invisible to the authority
check because nothing was refused. Two halves prevent it. The **textual** half is the Actor: a cast
program names only the concrete act. The **structural** half is `cmd/marco`: the act map a cast
program can reach is built without the Theater in it. There is no depth counter to tune.

## Enforced by

- `internal/platform/theaterhost/perform_test.go` — `TestTheaterRefusesWithoutAuthority` holds the
  order (permission before anything is resolved, so a refused production cannot reveal what it
  would have acted on); `TestTheaterCannotSpendAuthorityTwice`;
  `TestActorSuccessIsNotVerifiedSuccess` and `TestNoVerifierMeansNotVerifiedNeverSuccess` keep
  sending and succeeding apart; `TestOneRefusalVocabularyForBothCallers`;
  `TestTheWindowTravelsToTheCastProgram`.
- `internal/platform/theaterhost/ladder_test.go` — `TestAnActorNeverReachesAHostDirectly` fails if
  an Actor performs rather than casting; `TestAProductionWithoutARunnerPerformsNothing` is the
  fail-closed half; `TestACompileFailureStopsTheProductionAtOnce` is the compile gate;
  `TestACastProgramNamesOnlyTheConcreteAct` is the textual half of the recursion guarantee.
- `cmd/marco/theaterwiring_test.go` — `TestACastProgramCannotReEnterTheTheater` is the structural
  half, tested by running a program that asks for it; `TestTheCastRunnerDoesNotShareTheLiveActMap`;
  `TestTheAssembledTheaterRunsItsCastProgram` fails if this binary builds a Theater with no runner.
- `internal/director/rehearse/press_test.go` — `TestALivePressIsPutOnByTheTheater` fails if the
  rehearsal lowers a press itself; `TestARehearsalNeverPressesTwice` keeps the second ladder out;
  `TestAPressCarriesAuthorityForThatTargetOnly`; `TestAProductionRefusalKeepsItsReason`;
  `TestAPressWithNoTheaterRefusesBeforeEmitting` is the no-fallback proof.
- `internal/director/rehearse/route_test.go` — `TestADemonstratedClickRehearsesAsAProduction` is
  the two halves meeting; `TestAVerifiedProductionIsNotObservedTwice` counts observations;
  `TestAnUnverifiedProductionIsStillObserved` is its fail-open half;
  `TestAProductionThatRanAndFailedIsRecorded` holds the line between a refusal and a result.
- `internal/director/rehearse/boundary_test.go` — `TestTheDryPathCannotReachAHostByItself` still
  passes, which is what makes `internal/production` the right place for the contract.
- `cmd/director/rehearsetheaterwiring_test.go` — `TestARehearsalPressGoesThroughTheTheater` enters
  through `Runtime.Observation` and fails if this binary never wires a Theater.

## Related

[[ADR-068-the-theater-is-the-durable-semantic-world]] · [[ADR-067-a-play-may-name-a-control]] ·
[[ADR-023-rehearsal-is-attempt-scoped-authority]] · [[ADR-024-a-dry-step-is-not-evidence]] ·
[[Marco-Boundary]] · [[Learned-Plays]] · [[Demonstrations]]
