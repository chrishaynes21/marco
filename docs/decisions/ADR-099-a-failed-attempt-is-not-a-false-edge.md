---
type: decision
status: accepted
date: 2026-08-27
supersedes: []
affects:
  - learned-plays
  - semantic-memory
  - demonstrations
source_paths:
  - cmd/director/recover.go
  - cmd/director/recoverwiring_test.go
  - cmd/director/perform.go
  - cmd/director/performcmd.go
  - internal/director/service/protocol.go
---

# ADR-099 — A failed attempt is not a false edge

## Three things, and confusing any two is the whole risk

```
GRAPH KNOWLEDGE    activating X from A has been observed to reach B
EXECUTION ATTEMPT  Marco tried to activate X from A, now
ATTEMPT OUTCOME    this attempt succeeded, failed, was refused, or could not be checked
```

A failed attempt is a fact about **now**. It is not, by itself, evidence that the graph fact is
false: a button that moved, a window that changed, a stale accessibility handle and a control that
was briefly disabled all produce failures, and none of them means the control leads somewhere else.

[[ADR-095-repeated-observation-may-become-knowledge]] already defines contradiction, and it is a
much stronger thing: the same semantic action from the same semantic source **positively observed**
reaching a materially different destination. Nothing in recovery may reach it.

So failure evidence is **attempt-scoped**. It lives on the stack of one `PerformGoal` call, and
when that returns it is gone.

## What it was before

`performPlan` stopped at the first edge that failed, and `PerformGoal` returned. That was right
while Marco knew one way to anywhere. [[ADR-098-the-planner-prefers-better-evidence-and-says-why]]
made it wrong: Marco usually knows several ways now, and a control that has moved since yesterday
is a reason to take the other one rather than to give up.

**Two things the walker computed and dropped.** Every step is classified — `target_moved`,
`target_unavailable`, `wrong_state`, `unobservable` — and whatever perception resolved *after* the
action is recorded as `StepRecord.Observed`. Neither reached the view, so recovery would have had a
word like `ended_unverified` and nothing else: unable to tell a stale handle from a screen that led
elsewhere, and unable to say where Marco was standing.

## The classification, over the vocabulary that already exists

Nothing new was invented. `rehearse` already names every way a step can end; what was missing was
the **reading** — which of those words describe a world that moved, and which describe a boundary
Marco must not work around.

| | |
|---|---|
| `stopHere` | cancellation, a revoked or expired grant, no authority, a spent bound, an unreadable screen, nothing sent |
| `retryMechanics` | `target_moved` — the control is there and its handle is not |
| `replanFrom` | `target_unavailable`, `wrong_state`, `unrecognised`, `ambiguous`, `input_failed`, `window_not_in_front`, `ended_unverified`, `stopped_at_step` |

**Everything unrecognised falls to `stopHere`.** A word this classification has never heard of is a
word whose meaning nobody has decided, and guessing that an unknown failure is safe to work around
is exactly the guess that turns a bug into Marco pressing things.

## The recovery cycle

```
walk fails
    ↓
classify — a boundary stops here
    ↓
where is Marco ACTUALLY standing?   the step's own observation, then a fresh look
    ↓
unreadable or unknown → stop
    ↓
bounds — replans, total steps, going round
    ↓
SAME planner, SAME graph, plus one refusal for what just failed
    ↓
SAME performer, SAME source recognition, SAME legal Marco, SAME verification
```

**Where Marco actually is** is the half that only shows up in life. A failed step may still have
moved the interface: the action ran, the destination did not appear, and something else did.
Planning from where the edge *began* would be planning from a screen Marco is not on — and the next
edge's source guard would refuse it, which looks like a second failure and is really the first one
repeated. The walker's own post-action reading is preferred to a second look, because it was taken
at the moment that matters.

**A healthy screen Marco does not know stops too**, and for a different reason from an unreadable
one. Execution is not Learn: minting a Place because recovery would find one convenient is how a Do
turns into acquisition.

## Attempt-local, layered over durable rank

Two kinds of preference, kept apart:

| | |
|---|---|
| **durable** | what the graph's evidence says about an edge — ADR-098's ranking |
| **attempt** | what just happened to it, in this execution, minutes ago |

The second wins for the current attempt and is invisible to the next one. That is the motivating
case exactly: ADR-098 prefers a verified edge, the verified edge is broken today, and Marco should
take the observed alternative **now** without forgetting that the verified one worked yesterday.

It refuses **eligibility** rather than lowering a rank, so a failed edge cannot creep back in by
being the best of a bad set. Everything else about the alternate route is ranked by exactly the
rules the first one was — **there is no weaker fallback mode.**

## Bounded

```
maxReplans        3    a person asked for one thing; a fourth try is Marco insisting
maxAttemptSteps  12    recovery lengthens routes and lengths compound
maxEdgeAttempts   2    an edge that failed twice has said what it has to say
```

Plus loop detection: a Place seen three times in one attempt is going round. Revisiting a Place is
**not** wrong on its own — walking back to try another way out of it is exactly what recovery is
for — so it is read alongside the replan count rather than as a refusal of its own.

## What recovery is not

- **Not exploration.** It uses knowledge already in the graph. Marco does not execute uncertain
  edges to find out what they do.
- **Not learning.** No Place is minted, no edge is created, no topology grows.
- **Not permission.** The same delegation, the same authority scope, no new grant minted for the
  privilege of trying again. A revoked grant stops.
- **Not parallel.** One route at a time, serially. The desktop stays deterministic and the 35E
  machine-wide lease is untouched.
- **Not a fallback to the demonstration.** The saved `.marco` is never consulted; every route comes
  from the canonical planner.
- **Not a re-reading of the words.** `carryOn` takes the destination subject as an argument and
  never touches the goal store — a route failing says nothing about what somebody meant, and a
  recovery that decided the phrase now meant wherever Marco accidentally ended up would be
  answering a question nobody asked.

## Saying so

A success reported with no trace of the recovery would hide a broken control indefinitely — the
person would never learn that the way Marco used to take has stopped working. So the view carries
what happened, whether or not the goal was reached:

```
  the Mouse page didn't work (the control wasn't there), so from Bluetooth & devices
  I went another way — 2 more step(s).
```

Semantic, not host logs: no subject ids, no vocabulary words, no coordinates.

## What is NOT stored

No durable failure evidence at all. A stale handle today must not permanently make a valid semantic
edge worse tomorrow, and the honest baseline is attempt-local memory with no graph poisoning. The
attempt holds subject ids, edge references and failure classes — no screenshots, no coordinates, no
handles, no typed text.

## KNOWN FOLLOW-ONS

1. **`retryMechanics` is classified and not yet acted on.** `target_moved` is recognised as the one
   failure that deserves the same edge again after a fresh target resolution, and the recovery loop
   currently treats it as it treats any other recoverable failure — it replans. The classification
   is the part that had to be right first; wiring the retry is a small, separate change and the
   bound (`maxEdgeAttempts`) is already there for it.
2. **No durable execution reliability.** An edge that fails every day ranks exactly like one that
   has never been tried. That is the deliberate 36F baseline, and whether it should change is a
   question to answer from real use rather than from first principles.
3. **`PerformGoal` cannot be entered from a test**, because it goes through `winctx` to bring a
   window forward. Everything below that line is gated, including the real walker failing against a
   stalling desktop and `carryOn` reading the view it produced. The foreground half is not.
4. **Loop detection is a count of revisits**, not a model of what has been tried from each Place. A
   graph with many short cycles could spend its replan budget before its loop budget.

## Enforced by

- `cmd/director` — `TestAFailedEdgeIsNotChosenAgainInTheSameAttempt` (the motivating case);
  `TestRecoveryReplansThroughTheCanonicalPlanner`; `TestRecoveryReplansFromWhereMarcoActuallyIs`;
  `TestAFailureThatLeftMarcoNowhereItKnowsStops` (both unknowns);
  `TestAFailedAttemptDoesNotTouchTheGraph`; `TestFailureSuppressionDoesNotOutliveTheAttempt`;
  `TestBoundariesStopRecoveryRatherThanReplanningIt` (the whole taxonomy);
  `TestAnUnknownFailureStopsRatherThanRecovering`; `TestCancellationDuringRecoveryStopsIt`;
  `TestRecoveryIsBounded` (replans, steps, going round);
  `TestNoAlternativeSaysSoWithoutForgettingTheGoal`;
  `TestRecoveryKeepsTheSameGoalAndNeverRebindsIt`;
  `TestRecoveryIsReportedWhetherOrNotItWorked`.
- Through the real walker — `TestAFailedStepSaysHowItFailedAndWhereItLeftMarco`;
  `TestTheWalkFailsAndRecoveryHandsBackARouteTheWalkerCanTake`.

## Related

[[ADR-098-the-planner-prefers-better-evidence-and-says-why]] ·
[[ADR-095-repeated-observation-may-become-knowledge]] ·
[[ADR-090-a-verified-outcome-is-the-next-step-s-evidence]] ·
[[ADR-029-resolution-is-not-permission]] ·
[[ADR-023-rehearsal-is-attempt-scoped-authority]] ·
[[ADR-005-legal-marco-only]] ·
[[Learned-Plays]]
