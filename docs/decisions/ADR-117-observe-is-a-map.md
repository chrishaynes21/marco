---
type: decision
status: accepted
date: 2026-08-30
affects:
  - passive-observation
  - semantic-memory
  - editing
source_paths:
  - cmd/director/observemap.go
  - internal/director/observe/learning.go
  - cmd/marco/watchui.go
  - cmd/marco/edit.go
---

# ADR-117 — Observe is a map, and an unnamed place has one name

## Context

Dogfood reached a good place — *"I noticed you went from Home to Mouse. I think I can do this."* —
and could not build on it, because the surface showed process vocabulary rather than the model
underneath. A person could not tell what Marco thought the screen was called, what it had just
discovered, how Places related, or what any of it would let Marco do.

## Decision

### The primary object is the graph

`Runtime.Map` answers four questions and nothing else: **where am I**, **what does Marco know
around here**, **what did it just find**, **what can it reach**. It is a read that establishes
nothing, plans nothing new and cannot act.

- **The marker comes from perception, never from memory.** A remembered Place is not a visible
  Place; unknown is a valid answer and is said. A map is the surface where that mistake is most
  invisible, because being wrong looks like knowledge.
- **The neighbourhood, not the whole graph** — one step either way, because "what do you know
  around here" is a question somebody can act on. The whole topology is reported as counts, so
  growth is visible without drawing all of it.
- **Reachability is the planner's answer**, from `observe.PlanToGoal` with the canonical
  eligibility — the same call `Reach` and `PerformGoal` make. A connection is not a route, and a
  map that drew a line because two places touch would promise what the planner refuses.
- **FROM and TO, never origin and arrival.** Those are the walker's words; a person read them and
  could not tell what they meant.

### An unnamed place is called one thing

`observe.PlaceWords` floored to `DescribeStructure` — *"about back, settings, 96 things on it"*.
Its own comment already called that making somebody speak Marco's diagnostics, and two dogfood
sessions ran into it: first as `[unnamed subj_c3e77b6f]`, then as the inventory. The floor is now
`observe.Unnamed` — **"Unnamed place"** — one representation on every surface.

What the screen is MADE of is unchanged and still available: `DescribeStructure` stands, and
`KnownPlace.Describes` carries it beside the name, which is where a list telling two unnamed
screens apart reads it.

**A question is not a label.** `TestTwoRoutesFromDifferentPlacesAskDifferentQuestions` caught this
immediately: a rehearsal asking "shall I try getting from X to Y" with both ends called *Unnamed
place* describes nothing and cannot be answered. `PlaceWordsAsking` composes the label with the
description for exactly that case — still one naming function underneath.

## Consequences

- Observe shows connections rather than events. Activity remains the audit trail.
- Several unnamed places in one list all read *Unnamed place*, told apart by the description
  beside them rather than by the name.
- The scoped-affordance experiment (38B stages A–D) is **not** run here and the map does not
  depend on it.

## Enforced by

- `cmd/director` `TestTheMapMarkerComesFromPerception`, `TestTheMapDrawsTheCanonicalGraph`
- `cmd/director` `TestReachabilityComesFromThePlanner`, `TestTheDirectorAnswersWithItsMap`
- `cmd/director` `TestAnUnnamedPlaceIsCalledTheSameThingEverywhere`,
  `TestEverySurfaceNamesAPlaceTheSameWay`
- `internal/director/observe` `TestTwoRoutesFromDifferentPlacesAskDifferentQuestions`,
  `TestAnUnnamedPlaceIsStillDescribed`
- `cmd/marco` `TestTheHereViewAsksForTheMap`, `TestThePrimaryCopyDoesNotSayOriginOrArrival`

## Related

- [[ADR-116-watching-follows-the-window-not-the-executable]]
- [[ADR-056-a-goal-is-a-destination-not-a-route]]
- [[Experiment-022-the-first-dogfood]]
