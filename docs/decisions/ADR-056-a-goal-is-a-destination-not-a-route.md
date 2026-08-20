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
  - internal/director/teach/teach.go
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
  `TestTeachingRecordsTheDestinationAsAGoal` (`internal/director/teach/teach_test.go`)
- `TestReachPlansOverTheVerifiedEdgeFromWhereThePersonStands`,
  `TestAKnownGoalWithoutARehearsedRouteRefusesHonestly`
  (`cmd/director/reachwiring_test.go`)

## Related

[[ADR-018-a-remembered-relationship-is-adjacency-not-a-route]] ·
[[ADR-048-learn-teach-and-do-are-three-different-sentences]] ·
[[ADR-051-one-demonstration-and-an-attempt]] · [[Semantic-Memory]] · [[Demonstrations]]
