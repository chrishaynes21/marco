---
type: decision
status: accepted
date: 2026-08-06
supersedes: []
affects:
  - passive-observation
  - safety
  - privacy
---

# Passive observation has no reachable execution path

## Context

Passive observation watches a running application for minutes at a time, sampling what is on
screen so that temporal analysis has something to work with. It runs unattended, while
somebody is playing a game or using their computer for something else.

The promise is that it only watches. A promise of that kind, kept by discipline, lasts
exactly until somebody adds a convenient import — a helper that happens to live in a package
that can also click, pulled in for a coordinate conversion.

The distinction that matters is not that the code does not act. It is that the code **could
not**, because nothing it can reach knows how.

## Decision

**The observation packages may not reach anything capable of affecting the machine**, and
this is a property of the build rather than of the implementation.

## Consequences

- Observation is structurally incapable of input, so "did the session do something?" is
  answerable by inspection rather than by audit.
- Some code has to be duplicated or relocated rather than imported, which is the cost.
- The session is also held game-agnostic by the same mechanism, so a capability pack cannot
  leak into a package that is supposed to be neutral.
- Combined with the privacy classifier, an observation session produces evidence that is
  safe both to store and to share — see [[Passive-Observation]].

## Enforced by

- **boundary test** — `TestTheObservationPackageCannotAct`
  (`internal/director/observe/boundary_test.go`) walks the package's **transitive**
  dependencies and fails if anything that can touch a desktop is reachable at all
- **boundary test** — `TestTheObservationPackageStaysGameAgnostic` (same file)
- **boundary test** — `TestTheRunnerCannotAct`
  (`internal/director/observesession/boundary_test.go`)
- **guard test** — `cmd/director/observeguard_test.go`
- **implementation** — `internal/director/observe`, `internal/director/observesession`

## Related

- [[Passive-Observation]], [[Vision]]
- [[Experiment-002-dnfc-observation-baseline]]
