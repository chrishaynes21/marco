---
type: decision
status: accepted
date: 2026-10-13
affects:
  - passive-observation
  - navigation
  - execution
---

# ADR-187 — knowing the whole plan, trusting only the screen you are on

## Context

[[Experiment-038-how-far-a-reloaded-workflow-gets]] found two contracts that cannot both hold
during a multi-screen workflow:

| | says |
|---|---|
| `Artifact.Bind` ([[ADR-183-remembering-recognising-and-being-allowed-to-act]]) | every screen must be established before planning |
| a binding ([[ADR-186-a-person-can-say-where-they-are]]) | is valid only in the observation generation it was established in |

Establishing the second screen expires the first, so at most one is ever live and `Bind` can only
succeed on a one-screen workflow. Neither rule is wrong. They were written three days apart
against different models of how a run proceeds, and nothing had asked them to hold at the same
time until the readiness report did.

The owner named the resolution:

> Marco should know the whole plan, but only trust the screen it's actually on.

With the distinction that keeps it honest:

> the entire workflow can be validated before starting; its future screens cannot all be grounded
> before reaching them.

## Decision

Assurance moves to the point where an action is considered. `Artifact.StepReadinessAt` examines
**one** step against **one** observation, in the order a person would want the answer:

1. **this step's screen**, established in this observation generation — not an earlier one, and
   nothing is asked about the screens the workflow visits later;
2. **is there anything to press** — a wait is `not repeatable` and is answered by arriving;
3. **was the control's identity ever settled** — an unanswered clarification is reported as one,
   before re-acquisition, because it otherwise surfaces as an `indistinct` refusal that reads
   like a matcher failure and is not;
4. **can it be seen now**, from an outcome about *this* control on *this* screen;
5. **has anybody agreed**, in this session;
6. **is the session allowed to act**, which is never inferred here.

Only then is the step `plannable`, which means a point and an expectation exist. **It is not
permission.** Nothing in `workflowstore` lowers anything, and the package's import walk keeps the
lowering path out of reach entirely.

`Bind` is untouched and keeps its all-or-nothing meaning for the question it answers — *could
this whole workflow be established right now* — and simply stops standing between a reloaded
workflow and its first step. Observation generations, detector expiry, the durable format and
clarification provenance are all unchanged.

### Confirmation moved, and why

`SessionBinding` now holds which steps were confirmed, because step-scoped planning rebuilds the
draft from the artifact whenever the observation moves, and a confirmation living on the draft
would evaporate with it. It is a fact about the session, not about where Marco is standing. It
still never reaches a file.

### Finishing and stopping safely are different words

`Progress` distinguishes a completed workflow from one that performed earlier steps and then
stopped. A run interacts with something that changes after every action, so stopping partway is
the ordinary case and reporting it as either success or failure is a lie in a different
direction. Nothing recovers or rolls back; the distinction is kept so that it can be.

## What it measured

`perceptionreplay -frames -readiness -authorise`, fold A learning and fold B fresh:

```
1. artifact reload and validation                READY
2. fresh screen establishment                    READY   6 screens, 1 confirmed, 5 inferred
3. stored-control reacquisition                  READY   3 steps found their control
4. control clarification                         READY   nothing blocked on a question
5. fresh confirmation and authorisation          READY   [SIMULATED]
6. legal Marco lowering and compile validation   READY   3 steps lowered and compiled; NOT run
7. verification plan                             REFUSED

STOPPED SAFELY after 3 of 11 steps
```

**Six of seven stages, and three steps of a reloaded workflow lowered into legal Marco and
compiled**, from an artifact that crossed between two independent observers as bytes. Without the
`-authorise` fixture the chain stops at stage 5, which is the point of having the flag.

### The next genuine blocker

Stage 7, and it is specific. `workflow.Verify` compares screen **identities**; a reloaded
workflow's destination is an artifact **label** that nothing has bound, because arriving there is
what would bind it. The comparison is between two vocabularies and can only report `elsewhere`
for a step that has not moved.

Verifying a reloaded step therefore means asking whether the NEW observation establishes as the
step's destination label — screen establishment run once more, not a comparison. That is the next
change and it is not made here.

## Stage 7: a destination established, never an action verified

The blocker above is closed, and the words matter more than the code. `Artifact.DestinationAt`
asks the establishment path its own question one observation later — is this screen the saved
`screen_c` — and **establishes nothing itself**. Thresholds, expiry and the three outcomes are
untouched.

What it returns is a **destination established**. It is not an action verified, and `Verified()`
is a method that always returns false with the reason attached, so no caller can print the first
and imply the second:

> nothing was executed, so there is no post-action evidence; and arriving where a step expected
> is not by itself proof that the step's action caused it

### The circularity, labelled rather than argued away

A destination can be established two ways, and here they are not equal evidence even though they
are elsewhere:

- **confirmed** — somebody looked and said it is `screen_c`. They did not consult the workflow,
  so this is independent of the thing being checked.
- **inferred** — the workflow demonstrates a step into `screen_c` and the arrangement agrees. The
  expectation came from the **same workflow whose step is being checked**, so the only
  independent part is the arrangement agreement — and
  [[Experiment-036-recognising-a-screen-you-were-shown]] measured that arrangement agreement
  nominates an absent screen as confidently as a present one.

`Arrival.Independent()` is true only for the first. A report that collapsed the two would let a
workflow corroborate its own expectations.

### What the run actually shows

```
action lowered and compiled                ready
action executed                            NOT EXECUTED
simulated action boundary                  present
destination established                    2 of 3
  of those, independent of the workflow    0
action independently verified              NOT ESTABLISHED
```

**Zero of the three arrivals were established by somebody looking.** Every one was the workflow's
own expectation agreeing with an arrangement. That is the weakest of the two grounds, it is the
one the circularity warning is about, and it is the number this stage exists to print rather
than hide.

All seven stages now complete, and the chain still STOPS SAFELY after 3 of 11 steps.

## Consequences

- **A step on an unestablished screen is refused, and says which of the two reasons it is**:
  nothing established about it, or established as something else.
- **A revisit needs establishing again.** Unchanged, and now reached through the planning path.
- **Nothing pre-authorises a later screen.** Knowing the plan is not standing on it.
- **Authorisation is passed in and labelled.** The readiness experiment will not manufacture one,
  and the run without it is what shows what it is worth.

## Enforced by

`internal/director/workflowstore` —
`TestAMultiScreenWorkflowPlansItsFirstStepWithTheRestUnbound`,
`TestTheFirstScreensBindingExpiresAndTheSecondStepStillPlans`,
`TestAStepOnAnUnestablishedScreenIsRefused`,
`TestAScreenThatChangedSinceReacquisitionIsRefused`,
`TestARevisitedScreenNeedsEstablishingAgain`,
`TestAStepWithAnUnresolvedClarificationSaysSo`,
`TestConfirmationAndAuthorisationAreSeparateRefusals`,
`TestAnAmbiguousOutcomeRefusesTheStep`, `TestAnOutcomeForAnotherControlIsNotUsed`,
`TestAWaitStepIsNotPlannable`, `TestStoppingPartwayIsItsOwnOutcome`,
`TestPlannableIsNotPermission`.

Arrival — `TestAConfirmedDestinationIsIndependentEvidence`,
`TestAnInferredDestinationIsEstablishedButNotIndependent`,
`TestAnUngroundedDestinationIsNotEstablished`,
`TestADestinationMustBeEstablishedInTheNewObservation`,
`TestADestinationEstablishedAsAnotherScreenIsNotAnArrival`,
`TestAStepWithNoDestinationCannotArrive`, `TestAnActionIsNeverVerified`.

**Mutation: 12 of 12 on the step planner and 7 of 7 on the arrival adapter**, after two genuinely untested cases were found (an outcome for
another control, and a wait step) and one guard that was being killed by the guard after it — the
test now asserts which refusal it got rather than only that it got one.
