---
type: decision
status: accepted
date: 2026-08-27
supersedes: []
affects:
  - semantic-memory
  - learned-plays
  - demonstrations
  - passive-observation
source_paths:
  - internal/director/observe/rank.go
  - internal/director/observe/rank_test.go
  - internal/director/observe/goal.go
  - cmd/director/reach.go
  - cmd/director/rankwiring_test.go
  - cmd/director/perform.go
  - acceptance-36e.ps1
---

# ADR-098 — The planner prefers better evidence, and says why

## The claim

Graph edges carry evidence about known transitions. **Eligibility** decides whether an edge may be
planned at all. **Ranking** decides which eligible route is preferred. The two are different
questions and are kept apart.

Ranking is deterministic, bounded and explainable. Marco having walked an edge and recognised
where it arrived is stronger evidence than having watched a person walk it. Human repetition
strengthens semantic evidence, is bounded, and does **not** represent preference. Contradictions
cannot be erased by repetition. Route length remains a real cost. Goals do not own route
preference, and neither do historical demonstrations.

## What it was before

Breadth-first, shortest chain, ties broken by subject id. Every edge cost one, and the only
question asked of an edge was a boolean: may I use it?

That predicate — `plannableEdges` — already **computed** which of two kinds of knowledge an edge
had, and then threw the answer away:

```
EXECUTION-PROVEN     a completed rehearsal still vouches for it   ─┐
                                                                   ├─► true
OBSERVATIONALLY      the person demonstrated it, cleanly          ─┘
```

So Marco knew which edges it had actually performed and planned as though it did not. And
`WatchedEdge.Contradicted` — the record that one control had been seen leading two different
places — was written by the observer, kept durably, and **read by nothing** once the edge was
promoted. An edge Marco was demonstrably confused about planned exactly like one it was sure of.

## The model

Two small ordinals and a flag. No score, no weights, nothing to tune and nothing to overflow.

```
EdgeClass    ClassVerified       Marco walked it and checked
             ClassObservedOften  watched more than once
             ClassObservedOnce   watched once
             ClassNone           the zero value, and it ranks worst

EdgeRank     { Class, Contradicted bool }
```

`PathRank` compares complete routes, lexicographically:

| | | |
|---|---|---|
| 1 | **contradicted** | how many edges on this route Marco does not understand |
| 2 | **effort** | actions, plus one when the route is not fully verified |
| 3 | **weakest** | the worst edge class on the route |
| 4 | **actions** | the raw count |
| 5 | **step ids** | so an exact tie is still an answer |

### Contradiction is first, and is never traded away

A route Marco does not understand is not something to weigh against a saved keystroke. If there is
any other way, take it.

It does **not** make the edge ineligible. The disagreement is about which of two destinations a
control reaches and either might be right, so when it is the only way it is still a way — and the
plan says it goes through a contradiction. Turning a preference into a safety boundary would let a
single confused reading take a destination away.

### Verification is worth exactly one action

Both extremes are wrong. Ignoring verification throws away the only evidence Marco has about its
own ability. Letting it win outright makes Marco open four windows rather than one to save a
hypothetical, and a person watching that would rightly ask what it was doing.

One action is the smallest bounded answer that expresses *I would rather use the way I have
actually done, if it is not much further*. It is a policy, it is stated, and it is the thing to
change if it turns out to be wrong — not the shape of the comparison.

### The weakest edge, never an average

Two verified edges and one contradicted one is still a route with a bad edge in it, and the bad
edge is the one that will fail. An average hides it. Likewise two-of-two verified beats
three-of-five: a raw count of verified edges would prefer the longer route because it has more of
them, which is the arithmetic this exists to avoid.

### Repetition saturates immediately

The second sighting is the whole of what repetition tells the planner: more evidence that the graph
fact is **real**. It is not evidence that the person prefers this way. So the class saturates at
"more than once" and there is nothing above it, and even that only breaks ties — after actions.
Fourteen mornings on the long way still lose to one clean traversal of the short one.

A model that kept counting would let a habit outvote a contradiction by volume, and Marco would be
modelling somebody's routine rather than their computer.

## The search

Dijkstra over a state of `(subject, weakest class so far)`, with cost `(contradictions, actions)`.

The weakest class is in the **state** and not just the cost because `effort` adds one for a route
that is not fully verified — a path property, not an edge property. So the cheapest way to a
subject is not necessarily the cheapest way that is still fully verified, and a search keeping one
answer per subject would discard the route that eventually wins.

Four states per subject, over a topology the store already bounds. Each state settles once.

**Cycles cannot improve a route, structurally:** actions only increase and the weakest class only
falls, so no extension of a route is ever ranked better than the route itself. That is also why
the search terminates.

## Invariants, and where each is held

| invariant | held by |
|---|---|
| an unnecessary cycle cannot improve a route | `TestGoingRoundInACircleNeverImprovesARoute` |
| an extra equal-quality edge cannot improve a route | `TestAnExtraEqualEdgeNeverImprovesARoute` |
| a contradiction cannot be erased by good edges | `TestAGoodRouteAroundABadEdgeIsNotAveragedAway` |
| repeated evidence saturates | `TestRepetitionSaturatesAndDoesNotBuyActions` |
| exact ties are deterministic | `TestAnExactTieIsDecidedTheSameWayEveryTime` |
| the comparison is total and antisymmetric | `TestTheComparisonIsTotalAndAntisymmetric` |
| insertion order does not decide | `TestInsertionOrderDoesNotDecideTheRoute` |
| comparison is stable across a restart | `TestTheSameGraphPlansTheSameRouteAcrossARestart` |
| evidence never bypasses eligibility | `TestEvidenceCannotMakeAnIneligibleEdgeUsable` |

## Freshness is deliberately absent

A durable relationship carries **no timestamp**. The counts are cumulative and the only clock in
the evidence model lives on the candidate ledger beside it. Ranking on a time the edge does not
have would mean inventing one, and decay that made known graph facts evaporate is worse than no
decay. Left out until the evidence supports it, per this roadmap's own instruction.

## The explanation is the comparison

`PathRank.Because()` reads the same fields `BetterThan` reads, so a disagreement between them is a
bug in one line rather than a second opinion maintained separately. `director reach` prints it:

```
  step 1: Home → Bluetooth & devices
  step 2: Bluetooth & devices → Mouse

  chosen because: 2 actions, every step is one Marco has done and checked
```

`reach` also gained `--from`, because the question somebody debugging a route actually has is
usually about a place they are **not**: *would it still take the long way from the Home page?*
Until now the only answerable question was about the one screen a session happened to end on, and
on a fresh Director there is no such screen at all.

## What did NOT change

No second planner, no second graph, no second store. `reach` and `PerformGoal` call the same
`plannableEdges` and the same `PlanToGoal`, and a gate measures that they produce the same steps
and the same reasons.

Goal resolution ranks nothing — it hands over a destination and stops. Observe and Learn write
evidence and never a preferred route; there is no durable "preferred workflow" artifact and the
generated `.marco` gets no bonus for being the demonstrated way. Ranking emits no input, takes no
lease and mints no authority: **confidence in a route is not permission to walk it**, and every
edge is still verified as it is crossed through legal Marco.

Explicit Learn is still not verification. Somebody being emphatic is not Marco having performed
anything, so a taught edge and a watched edge grade alike.

## What 36E does NOT solve

Stated plainly, because each is a separate roadmap and entangling them with ranking is how both
become untestable:

- **Mid-execution replanning and recovery.** The route is chosen before execution. If edge two
  fails, the existing failure behaviour is unchanged — no next-best route, no backtracking, no
  exploring alternatives.
- **Exploration.** Marco does not execute uncertain edges to improve their ranking. Observe and
  Learn acquire knowledge; Do carries out requested work.
- **Failure evidence.** `rememberRehearsal` records only completed routes, so a failed attempt
  leaves the graph untouched — which is the right default (a target that moved is not a semantic
  contradiction) and means a repeatedly-failing edge currently ranks like any other observed one.
- **Freshness, decay, dynamic interface adaptation, probabilistic confidence, Fusion.**

## KNOWN FOLLOW-ONS

1. **Execution failure is invisible to DURABLE ranking**, and that is now a deliberate baseline
   rather than an omission: [[ADR-099-a-failed-attempt-is-not-a-false-edge]] made failure memory
   attempt-scoped, so a verified route that fails today is avoided for the rest of that attempt
   and ranks unchanged tomorrow. Whether repeated failure should eventually become durable
   evidence is a question to answer from real use.
2. **Contradiction is a flag, not a tally.** Deliberate — how many times Marco was confused about
   one screen is not a reason to prefer routes through it — but a persistent disagreement and a
   one-off misreading currently rank alike.
3. **Contradiction is only visible where the ledger row resolves.** A described end is resolved
   through `Recall`; an end whose screen memory no longer holds is skipped rather than guessed.
4. **`reach` still plans from where somebody was last seen** unless `--from` names a source.
   `PerformGoal` takes a fresh look. They agree about meaning and about ranking; they can still
   differ about where the person is.

## Related

[[ADR-097-language-names-the-outcome-the-graph-decides-the-way]] ·
[[ADR-096-observe-and-learn-are-two-doors-into-one-graph]] ·
[[ADR-095-repeated-observation-may-become-knowledge]] ·
[[ADR-089-watching-is-how-marco-learns-performing-is-how-it-proves]] ·
[[ADR-056-a-goal-is-a-destination-not-a-route]] ·
[[ADR-029-resolution-is-not-permission]] ·
[[ADR-005-legal-marco-only]] ·
[[Semantic-Memory]]
