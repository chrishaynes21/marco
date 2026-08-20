---
type: decision
status: accepted
date: 2026-08-17
supersedes: []
affects:
  - passive-observation
  - demonstrations
  - semantic-memory
  - learned-plays
source_paths:
  - internal/director/observe/bridge.go
  - internal/director/observe/screenstate.go
  - internal/director/observe/shadow.go
  - internal/director/observe/discoverycandidate.go
---

# ADR-064 — the order of a walk is evidence

The segmenter recorded WHICH changes happened and how often, and discarded WHAT FOLLOWED WHAT.
That was sufficient for every question about a recurring habit and insufficient for the question
Learn asks, which is about a route somebody has just walked.

## What was wrong

Every real navigation crosses a frame Marco cannot place — a half-rendered page, one sample wide.
[[ADR-049-a-change-nobody-could-read-is-still-one-change]] already recovers the
adjacency across one such frame. It could recover exactly one, per session.

A two-step demonstration crosses two. `ShadowTotals.Transitions` is keyed by `(From, To)`, so
`A → ? → B → ? → C` aggregates to:

```
A → state_unknown        count 1
state_unknown → B        count 1, unattributed 1
B → state_unknown        count 1
state_unknown → C        count 1, unattributed 1
```

Two entries into the unplaced state and two exits out of it. Which entry belongs with which exit
is genuinely not recoverable from counts — the pairing `A→C, B→B` fits the same numbers — so the
bridge refused `ambiguous_interval` and **both** adjacencies were lost.

The refusal was the correct reading of the evidence available and the wrong outcome. Measured
live on 2026-08-17, on a cold store, with a person demonstrating `Settings Home → Bluetooth &
devices → Mouse`: all three screens settled, all three carried terms, all three were made durable
after [[ADR-063-a-pass-remembers-every-place-it-settled-on]] — and the pass still learned nothing.
Fixing ADR-063 alone moved the report from `both_unresolved=2` to `destination_unresolved=2
source_unresolved=2` and produced zero edges, which is how the second cause was found: the first
had been hiding it.

**This was not an edge case.** A person navigating at ordinary human speed crosses a transition
frame at every step, so the one-shot multi-step Learn that Roadmap 34 exists to deliver was
impossible except at one step per session. Every fixture in the suite missed it, because a
scripted screen that RECURS is never separated from its neighbour by a frame nobody could place.

## The decision

**The segmenter records the session's WALK — every change, in the order it was seen — and interval
pairing reads the walk rather than the aggregate.**

- `observe.Crossing{From, To, Run}` — deliberately thin. It carries no navigation, no counts and
  no interpretation; those stay on `ScreenTransition`, which is still where anything is READ from.
  A crossing answers the one question the aggregate cannot.
- `Run` is the length of THIS gap, unlike `ScreenTransition.UnsettledRun`, which is the longest
  gap that edge ever crossed. Bridging asks about this one.
- Written in `ScreenSegmenter.note` — the single call site every change already passes through,
  so the walk cannot drift out of step with the aggregate.
- `unsettledIntervals(t)` replaces `unsettledPair(t)`: every `A → [unplaceable] → B` excursion, in
  order, each with exactly one entry and one exit **by construction**. Shared by both callers, as
  the pair was — the relationship bridge and the one-shot candidate path — so there is still one
  answer to "which transition carries the intents".
- `bridgeUnsettled` returns a slice and applies the same five conditions per interval.

### What the order buys beyond recovering the edges

The strong result rather than the safe one. `A → ? → C → ? → B` used to refuse partly to avoid
collapsing to `A → B` and inventing a route through nowhere. With the walk it yields `A → C` and
`C → B`: C stays in the middle, and the demonstration decomposes into the two reusable edges
[[ADR-056-a-goal-is-a-destination-not-a-route]] needs.

## What it still refuses, and what it does not claim

- **A truncated walk.** `MaxCrossings = 128`, and `EvictedCrossings > 0` makes `unsettledIntervals`
  return not-ok. Pairing across a dropped crossing would be a guess, so every caller refuses
  outright rather than pairing short.
- **A session that began or ended unplaced** — `no_entry` / `no_exit`, unchanged. There is no
  source to recover for an arrival and no destination for a departure, and neither may be invented.
- **An interval long enough to have hidden a screen** — `interval_too_long`, bound unchanged at
  `StatePromotionCount`, and it is now applied to the run of the interval being bridged rather
  than to the worst run that edge ever crossed.
- **Nothing about what the unplaced samples WERE.** The walk records that Marco could not read
  them and in what order that happened. `state_unknown` still mints nothing, names nothing and can
  never be a durable subject.

## Considered and rejected

- **Solve the pairing as a puzzle.** With two entries and two exits, only one pairing avoids a
  self-loop, so the live case could have been resolved by elimination. That is not evidence, it is
  a solver with a plausible answer, and with three intervals it has six candidates and no
  principle for choosing.
- **Put the navigation on the crossing too.** It would duplicate what `Sequences` already holds
  and give two answers to "what preceded this change". The aggregate already insists on an
  unambiguous order before anything may read it as a procedure (`legOf`), which is the rule that
  makes reading it safe.
- **Record a full per-inference state trail.** Strictly more information and unbounded in the
  session's length, when what was missing is the order of the CHANGES.
- **Have Learn capture the walk separately.** A second derivation of "what followed what", which
  is exactly the shape of mistake this repository has paid for before.

## Enforced by

- `internal/director/observesession/walkwiring_test.go` —
  `TestATwoStepWalkLeavesTwoDurableEdges` runs a walk `A → ? → B → ? → C` through the real session
  and requires two durable edges, both bridged, and three durable places;
  `TestAWalkThroughAMiddleScreenIsNotShortened` requires one subject to be both a destination and
  a source, so the route cannot have been read as a shortcut. Mutation-gated three ways —
  disabling the crossing log, refusing more than one interval (the old rule, which reproduces the
  live failure exactly), and narrowing establishment to the current state — each of which fails
  them, and each restored byte-identically.
- `internal/director/observe/bridge_test.go` —
  `TestAnIntermediateRecognisableScreenIsNotSkipped` now requires both halves and still forbids a
  direct `A → B`; every other refusal in that file is unchanged and still passes.
- `internal/director/observe/liveevidence_test.go` — the two real captured Settings sessions still
  replay. They predate the walk, so the loader reconstructs it, and only where the aggregate
  leaves exactly one possible ordering; anything else fails the fixture rather than guessing.

## Related

[[ADR-049-a-change-nobody-could-read-is-still-one-change]] ·
[[ADR-063-a-pass-remembers-every-place-it-settled-on]] ·
[[ADR-056-a-goal-is-a-destination-not-a-route]] ·
[[ADR-018-a-remembered-relationship-is-adjacency-not-a-route]] ·
[[Passive-Observation]] · [[Demonstrations]]
