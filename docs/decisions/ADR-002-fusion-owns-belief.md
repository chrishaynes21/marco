---
type: decision
status: accepted
date: 2026-08-06
supersedes: []
affects:
  - fusion
  - perception
  - world-state
---

# Fusion alone converts evidence into belief

## Context

Given [[ADR-001-observations-vs-belief]], something has to do the conversion. The question
is whether that is one stage or many.

Many is the natural drift. A provider that is *usually* right starts writing a belief
directly "just for this case"; a planner that needs a value the world state lacks reads an
observation "just this once". Each shortcut is locally reasonable and collectively fatal,
because after a few of them nobody can say what the Director believes or why.

## Decision

**There is exactly one stage that turns evidence into belief, and it is fusion.** Every
observation, from every provider, reaches world state through it or does not reach world
state at all.

Fusion owns merge criteria, identity, clustering, confidence combination, and the report
that explains what it did.

## Consequences

- One place to change how belief is formed, and one place to read to understand it.
- The fusion report is a complete account. Because nothing bypasses fusion, a report that
  does not mention something is proof that it did not become belief.
- Providers stay simple. A provider's job ends at "here is what I saw, and how sure I am".
- Fusion is a bottleneck by construction, which is a real cost on a busy screen — see the
  Cost section of [[director-perception]].

## Enforced by

- **implementation** — `internal/director/perception/fusion` (`engine.go`, `identity.go`,
  `cluster.go`, `report.go`)
- **boundary test** — `TestPerceptionCannotReachIntoTheReasoningLayers`
  (`internal/director/perception_boundary_test.go`)
- **boundary test** — `TestOnlyPerceptionKnowsWhatAnObservationIs` — a reasoning layer that
  could see an observation could bypass fusion

## Related

- [[ADR-001-observations-vs-belief]]
- [[ADR-003-evidence-authority-by-source]]
- [[Fusion]], [[Perception]]
