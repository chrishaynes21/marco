---
type: map
status: active
updated: 2026-08-06
source_paths:
  - cmd/director
  - internal/director
  - pkg/directorapi
  - internal/platform
---

# Director

The Director builds a world model of the desktop and plans, executes and **verifies**
arbitrary semantic actions against it — "click Save", "focus the search box then type
Director and press enter", "close every Notepad window".

It is distinct from `internal/dispatch`, which picks a **saved macro** from a phrase. The
Director decides what to do to the **screen**. Neither imports the other.

Start here, then read the subsystem note you need, then its ADRs. See [[AI-CONTEXT]] for
the retrieval protocol.

## The spine

```
perception  →  world state  →  intent  →  plan  →  program  →  lower to Marco  →  verify
 (evidence)     (belief)                             (steps)      (execution)     (proof)
```

Every stage is separated by a rule that a boundary test enforces:

- Evidence never becomes belief without passing through fusion — [[ADR-001-observations-vs-belief]], [[ADR-002-fusion-owns-belief]]
- No source may claim more than it can know — [[ADR-003-evidence-authority-by-source]], [[ADR-004-vision-cannot-establish-actionability]]
- Nothing reaches the desktop except as legal Marco source — [[ADR-005-legal-marco-only]]

There is a second spine beside it, for what Marco learns by watching rather than by being told:

```
watched  →  rehearsed  →  verified  →  written down  →  saved  →  registered  →  run  →  arrived
                                        (as Marco)                  (findable)   (asked)  (checked)
```

Each arrow is a wall. Watching is not verification, verification is not permission to write it
down, saving is not registering, resolving is not permission to run, and running every step is not
succeeding. [[Learned-Plays]] is the note; ADRs 023–032 are the constraints.

## Subsystems

**Perceiving**
- [[Perception]] — the observation graph, providers, provenance
- [[Fusion]] — the only place evidence becomes belief
- [[Vision]] — the learned detector as an evidence source
- [[Windows]] — window identity, liveness, stale-capture prevention
- [[Passive-Observation]] — watching without a reachable execution path
- [[Navigation]] — what the player did, as meaning rather than as keys

**Interpreting**
- [[Hypotheses]] — cautious interpretations of the discovery evidence, with their contradictions
- [[Semantic-Memory]] — what the user confirmed, surviving a restart

**Deciding**
- [[Programs]] — sequential semantic execution
- [[Goals]] — outcome descriptions expanded through hand-written procedures
- [[Semantic-Actions]] — 33 verbs, lowered by capability ladder at execution time
- [[Collections]] — bounded semantic queries, re-run every iteration
- [[Control-Flow]] — waits, conditions, cancellation
- [[Editing]] — semantic text entry

**Acting**
- [[Marco-Boundary]] — every desktop effect lowers to legal Marco source
- [[Plays]] — the product model: a Play, a Binding, the three kinds, the three scopes, saved vs registered
- [[Learned-Plays]] — what Marco watched, becoming a play it performs and verifies
- [[Action-Graph]] — what was done, in a form replay can re-lower
- [[Demonstrations]] — recording, and extracting a procedure from what was shown

**Running**
- [[Service]] — the warm-client service and its diagnostic surface
- [[Game-Packs]] — per-application capability contributions

**Being watched**
- [[Visibility]] — the one account of what Marco is doing, rendered as Normal, Watch and
  Diagnostics

## Decisions

The full index is [[Decisions]]. The load-bearing ones are ADR-001 through ADR-005; the
rest constrain specific subsystems.

## Current evidence

- [[Experiment-001-vision-backend-comparison]] — three detectors on a frozen Rocket League corpus
- [[Experiment-002-dnfc-observation-baseline]] — the passive session that set the baseline
- [[director-vision-backend-decision]] — the decision that came out of both

## What is next

See [[Roadmap]]. In short: the vision detector's class vocabulary is the blocking
constraint on everything downstream of perception, and the benchmark cannot currently
score it, because there is no nameable-role coverage metric.

## Milestone records

The `docs/director-*.md` files are the primary detail, each ending in a **Known gaps**
section that states what is not proven. Read those before trusting a capability:
[[director-perception]], [[director-vision]], [[director-programs]],
[[director-marco-boundary]], [[director-goals]], [[director-semantic-actions]],
[[director-collections]], [[director-demonstrations]], [[director-games]],
[[director-editing]], [[director-waits]], [[director-cancellation]],
[[director-service]], [[director-windows]].
