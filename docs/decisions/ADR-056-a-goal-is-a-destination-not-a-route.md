---
type: decision
status: accepted
date: 2026-08-17
supersedes: []
affects:
  - semantic-memory
  - demonstrations
  - learned-plays
source_paths:
  - internal/director/observe/goal.go
  - internal/director/observe/discoverycandidate.go
  - internal/director/observesession/runner.go
  - internal/director/learn/learn.go
  - internal/director/semanticmemory/store.go
  - cmd/director/reach.go
---

# ADR-056 — a goal is a destination, not a route

Learning was route-centric: `Learn` memorised how the person got from A to B, and the
capability it produced was effectively tied to A. The live E2E made the cost concrete —
exact starting screens, returns to START, waits for observation phases, retries around the
ritual. The person was adapting to Marco's capture protocol.

## The decision

```
Learn "open mouse settings"
    demonstration:  A --action--> B
    Marco learns:   goal = B            (the outcome, in the person's words)
                    edge A→B            (one KNOWN way in — evidence, never the definition)
```

- **A goal has no start, structurally.** `observe.Goal` is a name, an application and a
  destination subject; there is no field a start could go in, under any name a start might
  wear.
- **A demonstration is evidence and decomposes.** `A → B → C` contributes one candidate per
  grown edge, never one monolithic macro. The leg that arrived where the person stopped is
  the one the teaching tail carries forward; the others are already durable route knowledge.
- **No waypoint is inferred from one example.** One demonstration `A → B → C` proves the
  route succeeded, not that B is required for C. A later direct `A → C` coexists with both
  edges and wins on length.
- **Planning starts from wherever the person is.** `observe.PlanToGoal` is breadth-first
  over the remembered topology, filtered by the caller's usable-edge predicate. Already at
  the goal is `satisfied`; no chain is the honest sentence — *"I know what you want to
  reach, but I don't yet know how to get there from here."* Never a walk back to the
  demonstrated START.
- **The refusals that anchored Learn to its start are gone.** `start_not_recognised` no
  longer ends an attempt (the session watches anyway and says what route evidence was
  lost); `left_the_start` no longer exists at all.

## What this does not change

Authority. A goal is knowledge, and knowledge licenses nothing: every edge still earns
execution through its own rehearsal and its own yes, `reach` is a read that plans only over
edges a completed rehearsal still vouches for, and an unrehearsed edge appears in no plan.
[[ADR-018-a-remembered-relationship-is-adjacency-not-a-route]] stands unchanged underneath —
the topology this plans over is still adjacency, and the planner's output is still a claim
about what is KNOWN, enacted only through a saved play's resolve → authorize → run.

## Enforced by

- `TestGoalHasNoRequiredStart`, `TestCurrentBCanUseBToC`, `TestCurrentXCanComposeXToBToC`,
  `TestCurrentCIsAlreadySatisfied`, `TestUnknownRouteToCRefusesHonestly`,
  `TestOneDemonstrationDoesNotMakeAWaypointRequired`
  (`internal/director/observe/goal_test.go`)
- `TestAMultiLegDemonstrationDecomposesIntoReusableEdges`
  (`internal/director/observesession/goalwiring_test.go`)
- `TestAnUnrecognisableStartStillWatches`, `TestARouteFromSomewhereElseIsStillLearned`,
  `TestLearningRecordsTheDestinationAsAGoal` (`internal/director/learn/learn_test.go`)
- `TestReachPlansOverTheVerifiedEdgeFromWhereThePersonStands`,
  `TestAnObservedEdgeCanBePlannedOver`
  (`cmd/director/reachwiring_test.go`)

> **Amendment, 2026-08-20 — what makes an edge PLANNABLE widened; what makes it PERMITTED did not.**
>
> This decision was written when Learn could not finish without rehearsing every edge, so the only
> way an edge could exist unrehearsed was for something to have gone wrong. `verifiedEdges` — the
> predicate handed to `PlanToGoal` — therefore accepted execution-proven edges only, and
> TestAKnownGoalWithoutARehearsedRoute… (unbackticked so this note does not itself
> become a citation) held that line.
>
> Roadmap 35B removed the ceremony: a clean demonstration is admitted without Marco replaying it,
> so a route can now be perfectly well known and never have been walked. A planner that refused
> those would refuse the knowledge Learn had just acquired — "I learned that" followed by "I don't
> know how".
>
> The predicate is now `plannableEdges` and accepts either kind of knowing: execution-proven, or
> observationally admitted (`CandidateConsistent` with nothing `Blocking()` — the same rule Learn
> admits on, read from the same assessment, so the two cannot disagree about one demonstration).
>
> **Nothing about permission moved.** This decision's own words still hold: a plan "says a route is
> KNOWN, never that performing it is authorised". Authority is minted per invocation at the
> ordinary door ([[ADR-029-resolution-is-not-permission]]), the foreground must lead before input
> is emitted, and every edge is positively verified as it is walked. An edge that was only ever
> observed proves itself the first time somebody asks for it — or refuses honestly.

## Related

[[ADR-018-a-remembered-relationship-is-adjacency-not-a-route]] ·
[[ADR-048-learn-teach-and-do-are-three-different-sentences]] ·
[[ADR-051-one-demonstration-and-an-attempt]] · [[Semantic-Memory]] · [[Demonstrations]]
