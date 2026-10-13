---
type: experiment
status: complete
date: 2026-10-13
backend:
  - structure
  - vision
game: rocketleague
fixture: ".tmp/game-capture-review/rl-freeplay-1"
result: nineteen-of-twenty-one-steps-verified-none-unresolved-and-the-second-route-completes-from-its-own-start
supersedes: []
source_paths:
  - cmd/perceptionreplay/coverage.go
  - cmd/perceptionreplay/workflow.go
---

# Experiment 032 — what the recording can and cannot say

**Status: RUN offline, 2026-10-13, on the existing recording and recorded detections. No new
capture, no inference, no actuation, nothing durable written.**

## The question

[[Experiment-031-the-workflow-gets-nine-steps-in]] reported a prefix of 9 of 21 and a stop whose
reason was "the recording ended". The owner asked what that actually leaves unknown:

> Step 9 stops because the available demonstration ends while waiting for an automatic
> transition. That is a recording-coverage limit, not necessarily a planning failure.

With four questions, and one instruction that shapes the whole answer:

> Do not treat a missing recorded continuation as a successful transition.

## Why a prefix is not an assessment

A prefix stops, and everything after the stop is unexamined. "Unexamined" has at least three
meanings and reporting them as one hides whichever matters:

| verdict | meaning | is it a defect |
|---|---|---|
| `verified` | some walk planned it, acted, and the screen became what the step expected | no — it worked |
| `unresolved` | its screen stood and the control was never resolvable on it | **yes** |
| `not-reached` | resolvable, but no walk arrived — an earlier step stopped or the recording ran out | no |
| `no-evidence` | its starting screen never stood again anywhere | no |

## The result

```
                       pixels only        with detections
verified                   11                  19
unresolved                  1  <- target_2      0
not reached                 9                   2
no evidence                 0                   0
```

**Nothing is unresolved any more.** Every step the recording could evaluate, it evaluated, and
every control resolved. The two `not-reached` steps are 9 and 10 — the tail of the first route,
where the recording ends.

### The second route completes, from its own starting observation

```
route 1 (steps 0-10)    9 of 11, stopped at 9: the recording ended
route 2 (steps 11-20)   10 of 10, completed
```

The second route is walked with a fresh cursor, **and not before look 356** — the look at which
the demonstration arrived at the screen that route starts from. That gate is the measurement.
Without it the second walk starts at look 1 and plans its opening steps against the FIRST
route's observations, because steps 11-14 are the same screens and the same controls as 0-3; the
answer would be real and about the wrong thing.

Getting the gate wrong is not hypothetical. Gating on the route's own first transition instead
of the arrival into it produced **0 of 10** — the walk started after its opening step had
already happened, sat waiting for a screen it had just left, caught the next visit to that
screen and reported a failure that was entirely an artefact of the gate.
`TestARouteStartsWhenItsFirstScreenWasArrivedAt` is that mistake, held.

### And the pixel-only run had the same second route all along

This is the finding that justifies the whole exercise. **Pixel-only, route 2 also completes 10
of 10.** It was never reported, because route 1 stopped at step 1 and a prefix cannot look past
its own stop. The headline `1 of 21` was true and was hiding a route the recording walks end to
end.

### What the detector actually bought, stated precisely

Pixel-only, the two demonstrations of the TRAINING tile were **two different identities**:

```
pixels:      step 1  target_2   unresolved   0 of 62 looks   <- the stadium slab
             step 12 target_10  verified     5 of 62 looks   <- a different identity

detections:  step 1  target_2   verified    26 of 62 looks
             step 12 target_2   verified    26 of 62 looks   <- the SAME control
```

So the detector did not only make the tile findable. It made two presses on one control resolve
to **one control**, where the pixel proposer had learned the first press as a slab of stadium
crowd and the second as something else entirely.

### The same learned targets, on both visits

All four targets used by two steps are verified at both, with detections:

```
target_1   step 0: verified (134 of 448)   step 11: verified (134 of 448)
target_2   step 1: verified ( 26 of  62)   step 12: verified ( 26 of  62)
target_4   step 3: verified ( 51 of  56)   step 14: verified ( 51 of  56)
target_6   step 7: verified (  7 of 181)   step 16: verified (  7 of 181)
```

Pixel-only there are only three such targets, and `target_2` is not among them — because the
two visits were not the same target.

## One thing worth knowing about how a walk stops

A refusal does **not** stop a walk. A screen settles before its contents are proposed, so a run
waits on its own screen and gives up only when the screen becomes something the step did not
expect, or when the recording ends. That is why nearly every stop reads as "the recording
ended" rather than "the control was refused", and it is deliberate rather than accidental.

While extracting the walk into a `walker` a dead guard was removed: a refusal was followed by
`if standing != step.From { stop }`, which the line above it makes unreachable. Behaviour is
unchanged; the code now says what it does.

## What this does not establish

- **Nine, or nineteen, planned steps are not executed UI actions.** The owner's distinction, and
  it stands: this validates planning, lowering and verification against recorded observations.
  Nothing was performed.
- **Zero unresolved on one recording is not a guarantee.** One demonstration, one resolution,
  one window.
- **Route 2 completing does not mean a run would complete it.** It means the recording contains
  enough evidence to plan and verify every step of it in order.

## Enforced by

`cmd/perceptionreplay` — `TestAStepsVerdictSaysWhichEvidenceWasMissing`,
`TestAStepCompletedByTheSecondRouteIsNotCalledUnreached`,
`TestTheSecondRouteStartsWhereTheOpeningStepRecurs`,
`TestARouteStartsWhenItsFirstScreenWasArrivedAt`,
`TestARouteGatesOnItsOwnArrivalNotTheFirstOne`, `TestAWalkDoesNothingBeforeItsStartingLook`,
`TestAWaitAdvancesOnlyWhenTheDestinationStands`,
`TestAWalkStopsWhenItArrivesSomewhereUnexpected`, `TestARecordingThatEndsMidStepIsNotASuccess`.

**Mutation: 11 of 11 killed**, after one survivor — the gate test was built on an unconfirmed
step, which `Plan` refuses for a different reason, so it passed whether the gate existed or not.
