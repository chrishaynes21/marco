---
type: decision
status: accepted
date: 2026-09-02
affects:
  - passive-observation
source_paths:
  - internal/director/semanticmemory/learning.go
  - internal/director/semanticmemory/store.go
  - internal/director/observe/relationship.go
  - cmd/director/observepromote.go
  - cmd/director/learningfeed.go
  - cmd/marco/edit.go
---

# ADR-122 — a movement is not a way

## Context

Read off a live store, the JUST LEARNED strip told somebody:

```
learned way  Home -> Mouse
```

Marco could not do `Home → Mouse`, and had *explicitly declined to claim it*. That crossing spanned
a page it could not place, a second press happened inside the gap, and
[[ADR-120-a-crossing-that-spans-two-interactions-carries-neither]] refused to credit the arrival to
any action.

The store still recorded the **adjacency**, which is right — that is how the map grows. The feed
then announced the adjacency in the words of a route.

This is worse than noisy UX. The distinction it collapsed is the one the store is built around, and
`store.go` says so out loud about the two fields:

> *"being separate types behind separate interfaces is what keeps 'I have seen this' from becoming
> 'I know this' by accident"*

`relationships` are places that border each other. `candidates` are how you get across. A feed that
reports the first in the vocabulary of the second undoes that separation at the only point where it
reaches a person.

## Decision

**The durable graph is untouched.** The same relationship record is written either way; the
topology, the counts and everything that plans over them are unchanged. What changes is what the
announcement is entitled to SAY.

| what happened | announced as |
|---|---|
| relationship created, route evidence beside it | `learned` / `edge` |
| relationship created, nothing can say how | `saw` / `movement` |
| relationship corroborated | `strengthened` / `edge` |

`RelationshipObservation.Routed` carries the distinction and reaches nothing durable. Its zero value
is the cautious one on purpose: a caller that forgets it understates what Marco knows, which is the
direction a wrong answer should fall.

The three callers of `promotion.relate` divide cleanly, which is why the flag is honest rather than
a guess. `admitWatched` and `learnRecent` both go through `resolvePlaces` and write a candidate per
step. `strengthen` hand-builds its step to fold a re-sighting into an edge that already exists, and
writes no route evidence of its own — so when the edge turns out not to exist, it creates the
relationship with nothing beside it. Four relationships and three candidates in the live store is
that path, seen from the outside.

**Said, not hidden.** Going quiet would close the lie by concealing how the map grows, and a strip
that reports nothing while Marco is watching is its own defect. A movement renders as *"watched you
go Home -> Mouse"*, and is demoted out of the headline for the same reason a re-sighting is: the one
line somebody actually reads must not be the one thing Marco does not know.

## Consequences

- The `Home → Mouse` entry from Experiment-022 finding 9 now reads as movement.
- Ambient affordance acquisition (38D step 2) will push considerably more durable activity through
  this strip. It needs the same separation again — *saw an affordance* against *learned a
  transition* — and this ADR is the shape that answers to.
- Nothing downstream consumes the feed's `kind`, so `movement` adds a value without a migration.

## Enforced by

- `cmd/director` `TestAMovementWithNoRouteEvidenceIsNotAnnouncedAsALearnedEdge`
- `cmd/director` `TestACrossingWithRouteEvidenceIsStillALearnedEdge` — the control
- `cmd/director` `TestWalkingAKnownEdgeAgainIsStrengthenedRatherThanLearned`
- `cmd/marco` `TestAMovementIsNotWordedAsAWay`
- `cmd/marco` `TestTheHeadlineIsNewKnowledgeRatherThanTheLatestEvent`

## Related

- [[ADR-120-a-crossing-that-spans-two-interactions-carries-neither]]
- [[ADR-095-repeated-observation-may-become-knowledge]]
- [[ADR-123-a-control-you-can-see-is-worth-knowing-about]]
- [[ADR-111-a-demonstration-takes-the-slot-from-watching]]
- [[Experiment-022-the-first-dogfood]]
