---
type: decision
status: accepted
date: 2026-08-10
supersedes: []
affects:
  - demonstrations
  - semantic-memory
  - perception
---

# Authority must never outrun observation, and verification is derived

One authorized attempt may contain several steps. Marco acts once, then looks, and only if reality
permits may it act again. A rehearsal succeeds only when the WHOLE learned route survives that
conversation.

---
## Context

[[ADR-025-one-move-then-look]] proved the one-step lifecycle: establish the source from perception,
claim, guard, emit one semantic operation through legal Marco, settle, observe fresh, classify.
Roadmap 23 generalises that seam without replacing it — the multi-step machine orchestrates
repeated calls to the same proven primitive, and no second lowering path exists.

The authorization needed no change. `AskRehearse` already asks *"I'd like to try it once myself,
one step at a time, and stop the moment the screen isn't what I expected"* — which is a bounded
multi-step attempt described in the user's own terms, not a single keypress.

---
## Decision 1 — ACT → ACT is impossible in the transition graph

```
open --lower--> acted --observe--> open        the only loop
  |               |                  |
  +---------------+------------------+--> finished | cancelled
```

`Attempt.LowerStep` may run only from `open` and leaves the attempt in `acted`, which nothing but
`Attempt.Observed` can leave. The invariant is therefore a property of the type rather than of an
orchestrator remembering to look: a loop with a bug is REFUSED (`awaiting_observation`) where one
relying on control flow would type twice.

`Observed(mayContinue bool)` takes the orchestrator's reading of the world, because classifying a
screen needs perception and this type has none.

---
## Decision 2 — the attempt owns authorization; a step owns none

| | |
|---|---|
| an ATTEMPT | one authorization, one candidate, one application, one source, bounded, N step records, exactly one terminal outcome |
| a STEP | one proposed operation, its precondition, at most one actuation, fresh observation, one verification |

The grant is claimed once, before the first step, and is spent for the whole attempt however it
ends. It expires on any terminal outcome, cannot be resumed, cannot be reused, and does not survive
a restart.

Bounds are the plan's own: `MaxInputs` from the judgement, `maxSteps` from the plan's length,
`MaxUnobservable` from its longest contained run, `MaxDuration` from the grant. A bound failing is
a typed terminal outcome, never a truncated step.

---
## Decision 3 — containment comes from the candidate, never from the result

`progress_unobservable` permits continuing only when the CANDIDATE declared that step unobservable
before the attempt, and only while containment held: same window, same application, same screen.

A step the candidate said was directly verifiable, whose screen then did not change, is
`wrong_state`. Inferring containment afterwards from a failure to detect progress would let every
step that did nothing report itself as safely contained — and would eventually promote a procedure
that presses keys into a void.

**A route cannot succeed on containment.** The final step must be DIRECTLY verified: containment
says the screen did not change, and a destination nobody arrived at is not a destination reached.

---
## Decision 4 — verification is derived, and `Verified` stays vestigial

`ProcedureCandidate.Verified` remains always false, and is now deliberately vestigial. Nothing
stamps it, because verification is not a property of an observation.

Instead a completed attempt stores `observe.RehearsalEvidence` — bounded, semantic, and containing
**nothing executable**: which candidate, which digest, which endpoints, how many steps and inputs,
and the per-step outcome vocabulary. Reproducing any of it means lowering the candidate again
through the boundary under a fresh authorization.

Whether a candidate counts as verified is then recomputed by `CandidateAssessment.WithRehearsal`,
and three things must hold every time it is asked:

- the route COMPLETED when it ran;
- the digest still matches, so it is the same demonstration;
- memory still recognises both endpoints.

A stored boolean would go on saying yes after the demonstration was revised or a screen was
contradicted. This is the discipline [[ADR-021-a-judgement-is-recomputed-not-recorded]] established,
applied to the strongest claim in the system.

**Persistence is required**, because the milestone that lowers a verified procedure into readable
Marco cannot ask an exited process whether the route once worked, and re-rehearsing on every launch
is not a design. Load applies the same referential rule as candidates: evidence whose endpoints are
gone is dropped rather than kept vouching.

---
## Decision 5 — failure is evidence, and different failures mean different things

| outcome | what it means about the candidate |
|---|---|
| `wrong_state` | the route went somewhere else. Real contradiction evidence — but recorded, not acted on: nothing invalidates the candidate here. |
| `input_failed` | the HOST could not send. Says nothing about the procedure, and must never read as one. |
| `target_moved`, `target_unavailable` | the world moved. Not about the procedure at all. |
| `ambiguous`, `unobservable` | Marco could not see. Not about the procedure. |
| `cancelled` | the user stopped it. Not about the procedure. |
| `bounds_exceeded` | the attempt ran out of room. Not about the procedure. |

Every one of them leaves the candidate exactly as it was and requires a FRESH authorization to try
again. None of them is stored: only a completed route writes anything.

---
## Decision 6 — a dry run stops after one step

A recording host does not move the application, so sequencing past step 1 would mean classifying
screens that never changed and calling the results a route. A dry attempt lowers one step, records
what would be sent, ends `nothing_sent`, and completes nothing.

---
## Consequences

- The next milestone may ask "is this candidate verified" and get a meaningful answer across
  restarts, without anything executable having been persisted.
- No learned Marco is generated, no actor or verb is named, no capability is registered, and
  nothing rehearsed can invoke itself.
- [[ADR-005-legal-marco-only]] holds for EVERY step: there is no step-2 shortcut.

## Enforced by

- `TestAnAttemptCannotActTwiceWithoutLooking` — the governing invariant, at the type.
- `TestATwoStepRouteThatVerifiesThroughoutCompletes` — a whole route, and every step through
  legal Marco.
- `TestAContainedMiddleStepDoesNotStopARoute`,
  `TestAFailureToArriveIsNeverReinterpretedAsContainment`,
  `TestARouteEndingOnContainmentDoesNotComplete` — containment comes from the candidate, and never
  finishes a route.
- `TestAWrongStateStopsTheRouteWithNoFurtherInput`, `TestASuccessfulPrefixDoesNotCompleteARoute`,
  `TestARouteThatNeverReachesItsDestinationDoesNotComplete`.
- `TestAWindowThatMovesBeforeTheSecondStepSendsNoSecondInput`,
  `TestAWindowThatMovesAfterAnInputPreservesThatItHappened` — a fresh guard before EVERY step.
- `TestCancellingBetweenTwoStepsSendsOnlyTheFirst` — the narrow boundary.
- `TestAHostFailureMidRouteIsNotACandidateContradiction`,
  `TestPerceptionFailingMidRouteStopsTheAttempt`, `TestARouteStopsWhenItsInputBudgetRunsOut`.
- `TestADryRouteNeverCompletes`, `TestACompletedRouteDoesNotAuthoriseAnother`.
- `TestNoRehearsalEverStampsACandidateVerified`,
  `TestStoredEvidenceDoesNotVouchForARevisedDemonstration` — verification is derived, and stops
  being true when the world moves under it.
