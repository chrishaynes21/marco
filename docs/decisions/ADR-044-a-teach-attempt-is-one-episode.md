---
type: decision
status: accepted
date: 2026-08-11
supersedes: []
affects:
  - semantic-memory
  - passive-observation
source_paths:
  - internal/director/observe/relationship.go
  - internal/director/observesession/runner.go
  - internal/director/semanticmemory/store.go
  - cmd/director/learnsessionwiring.go
---

# ADR-044 — a teach attempt is one episode, not three sightings

> *Editorial note, added 2026-08-20.* Written before
> [[ADR-048-learn-teach-and-do-are-three-different-sentences]]. **A "teach attempt" here is a Learn
> episode** — the person demonstrates and Marco acquires — not the reserved **Teach** feature, in
> which Marco guides a person through something it already knows. The record is left in the
> vocabulary of its date; the rename is [[ADR-086-one-acquisition-one-word-one-request]].

[[ADR-043-teaching-is-two-passes-not-a-new-capture]] left an open concern, recorded rather than
resolved: teaching runs several bounded observation passes back to back, and every session that
sees a durable edge increments `RememberedRelationship.Sessions`.

That counter is not a tally of observations — `Observations` is. It means **separate times**:

> Twenty observations in one sitting and the same edge on five different days are different kinds
> of evidence, and only the second has survived a restart, a different window generation and
> whatever else changed in between.

`DefaultLearningThresholds.MinSessions` is 2, and it is what stops Marco offering to learn a habit
it has seen once. Three teach passes in one sitting would satisfy it outright.

So the answer to the audit's question — *can one explicit teach attempt satisfy a policy intended
to mean independent real-world recurrence?* — was **yes, it could**. Marco would have been
manufacturing its own corroboration and then offering to learn from it.

## The decision

`RelationshipObservation.SameEpisode`. A session may declare that it belongs to an episode whose
corroboration has already been counted; the store folds its evidence and does not increment
`Sessions`.

**One teach attempt is one episode.** It claims its single sighting at the first pass that actually
contributed a durable edge; every pass after it declares itself part of the same episode.

### Why the flag defaults to counting

The zero value corroborates. Every existing caller is unchanged, and a caller that forgets the
field cannot silently stop passive observation from accumulating evidence — the failure mode
points the safe way.

### Why it is stamped in the runner

Whether two sessions are one episode is a fact about **why** they were run. The store cannot know
it; the coordinator that ran them can. So the flag travels on `observesession.Config`, the runner
stamps every observation it is about to hand over, and the store applies one rule.

## The other audit: a busy start

The second recorded concern was that a start screen with history might make direct teaching
unusable — Teach refuses when more than one route leaves the start.

It does not. Teach picks the route by **diffing the durable topology across the discovery pass**,
so what decides is what the person did in the last minute, not what the screen has ever done. A
start with twenty historical routes out of it is unambiguous; two routes appearing *during the
demonstration* are genuinely ambiguous and still refuse.

Characterised rather than changed: `TestHistoryAtTheStartDoesNotMakeTeachingUnusable`. No policy
was touched, because the counters showed no mismatch to fix.

## Enforced by

- `internal/director/semanticmemory/store_test.go` —
  `TestATeachingEpisodeClaimsOneSessionAndAnOrdinaryOneStillClaimsItsOwn`: three passes of one
  episode fold six observations and claim one session; two ordinary sittings after it claim their
  own. Both the rule and its default survived deliberate deletion and inversion.
- `internal/director/observesession/relationshipwiring_test.go` —
  `TestASessionInAnEpisodeClaimsNoFurtherCorroboration`, through the production `Run` path.
- `cmd/director/learnepisode_test.go` —
  `TestATeachEpisodeClaimsOneCorroborationThroughTheProductionPass` drives `teachPasses.Observe`
  and asserts the flag it hands the runner, so deleting the production line fails.

## Related

[[ADR-043-teaching-is-two-passes-not-a-new-capture]] ·
[[ADR-018-a-remembered-relationship-is-adjacency-not-a-route]] · [[Semantic-Memory]]
