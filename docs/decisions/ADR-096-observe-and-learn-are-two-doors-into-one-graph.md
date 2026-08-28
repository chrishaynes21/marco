---
type: decision
status: accepted
date: 2026-08-27
supersedes: []
affects:
  - passive-observation
  - demonstrations
  - semantic-memory
  - learned-plays
source_paths:
  - cmd/director/onegraph_test.go
  - cmd/director/observerecent.go
  - cmd/director/observepromote.go
  - cmd/director/perform.go
  - cmd/director/reach.go
  - internal/director/observesession/runner.go
  - internal/director/observe/relationship.go
  - internal/director/observe/goal.go
  - acceptance-36c2.ps1
---

# ADR-096 — Observe and Learn are two doors into one graph

## The claim

There is **one semantic graph of the computer**: Places, the Targets on them, and the Edges
between them, in one store, read by one planner.

Observe teaches it while somebody uses their machine. Learn teaches it when somebody says *this
matters*. Marco executing and verifying an edge strengthens it. Planning reads it. Goals and Plays
sit above it and refer into it.

There is no Observe graph, no Learn graph, no Play graph, no demonstration graph and no replay
graph. **Observe and Learn differ in acquisition INTENT, not in what they produce.**

## What this ADR actually decided

Mostly: *that the architecture already said this, and that nothing had ever asked it to prove so.*

36C.1 established the model on the ambient side — one clean traversal is one graph edge, and edges
watched apart compose into routes nobody demonstrated. This roadmap audited the explicit Learn path
end to end looking for the false boundary, expecting to correct it, and found the boundary was not
there. Learn already writes canonical Places, canonical Targets and canonical Edges, through the
same store methods and the same identity test ambient watching uses:

| | prospective Learn | retrospective Learn | ambient Observe |
|---|---|---|---|
| licence | `observesession.LearnLicence()` | `observesession.LearnLicence()` | zero, then the same on promotion |
| places | `PlaceStore.EstablishPlace` | `PlaceStore.EstablishPlace` | `PlaceStore.EstablishPlace` |
| identity | `Memory.Recall` → `CompareStructure` | same | same |
| edges | `Memory.RememberRelationships`, one per transition | same, one per leg | same, one per traversal |
| candidates | one per grown edge | one per leg | one per promoted edge |
| planner | `observe.PlanToGoal` | same | same |

An edge's identity is `(application, from-subject, to-subject)` and nothing else. Not the name
somebody gave the episode. Not the episode. Not the session. Not a timestamp, and not a coordinate.

So this ADR is mostly a **gate**, and the gates are the deliverable: eighteen deterministic tests
in `cmd/director/onegraph_test.go` driving both production doors against one store, plus a mutation
run that shows each of them bites.

## The two things that were genuinely at risk

**The seam had never been crossed in a test.** Every Learn test drove Learn; every Observe test
drove Observe. Nothing in the tree had ever taught one edge through one door and the next edge
through the other and asked the planner to walk both — which is precisely the case a person hits
on their second afternoon, and precisely the case a second graph would break. The mutation that
namespaces explicit Learn into `settings_learned` used to survive the entire suite.

**Prospective Learn establishes places at a different call site.** Retrospective Learn shares the
promotion boundary with ambient watching by construction, so those two could not drift. A live
`marco learn "…"` does not use that boundary at all: it runs an ordinary observation session, and
the session establishes through its own call site in the runner. Two call sites, one store. Nothing
held them together, and the symptom of drift would be a second copy of every screen somebody had
both walked past and been shown.

## Where the name goes, and why it is not in the graph

`marco learn "mouse settings"` produces two different kinds of fact:

```
GRAPH      Home --Bluetooth & devices--> Bluetooth
           Bluetooth --Mouse--> Mouse           (how the computer moves)

GOAL       "mouse settings" → the Mouse page    (what the person wants)
```

A [[ADR-056-a-goal-is-a-destination-not-a-route|goal has no start]] — not an empty start, no field
a start could go in. So "mouse settings" does not mean *Home → Bluetooth → Mouse*; it means *the
Mouse page*, and how to get there is decided when somebody asks.

That is measurable rather than aspirational, and it is measured: the same goal asked from Home and
from Printers & scanners produces two different first steps, because `PerformGoal` resolves the
name to a destination, takes a fresh look at where the person actually is, and asks the planner for
a way from THERE. Two names over the same screens — "mouse settings" and "change mouse settings" —
duplicate no Place, no Target and no Edge, and both names survive.

## What a Play is, precisely

Audited rather than assumed, because this was the likeliest place to find private route knowledge.

A learned Play is **a registered phrase plus a readable generated artifact**. The `.marco` file
carries an entry guard on one starting screen and a fixed sequence of presses — genuinely
route-shaped, and honestly so: it is a record of what was watched, written in a language somebody
can open and edit.

**It is not the execution path.** Asking for a learned play by name goes:

```
marco do "open mouse settings"
    → the routes resolver finds the registered phrase
    → performLearned refuses to run the file locally, and delegates
    → PerformGoal: name → goal → destination subject
    → foreground the application, take a FRESH look at where you are
    → observe.PlanToGoal over the canonical topology
    → walk, verifying each edge as it goes
```

`performLearned` says why the local fallback is refused: the generated play catches its own
first-line refusal, so running it outside the Director would exit 0 for a play that never ran. The
consequence for this ADR is that the frozen sequence in the artifact is never what executes.

**The compatibility debt, stated plainly:** the saved `.marco` still describes one way in, from one
starting screen, and regenerating it after the graph grows a better route is not something anything
does today. Somebody reading the file will read the demonstration rather than what Marco would now
do. That is a readability gap, not an ownership gap — the file owns nothing — and it belongs to
whatever eventually decides how a Play is presented. Recorded here so it is not discovered.

## What did NOT change

No new store. No new planner. No new graph. No new licence. No widening of authority: retrospective
Learn still emits no input, takes no desktop lease and opens no performance slot, and the
performance counter says so. No change to what is persisted — the graph holds counts, structures,
subject ids and the two words the interface put on things, and no screenshot, transcript, keystroke,
coordinate or secret.

Explicit Learn still records `EdgeObserved`, never `EdgeVerified`. Somebody being emphatic is not
Marco having executed anything; verification is still something Marco earns by walking an edge and
recognising where it arrived. A clean demonstration is learned without rehearsal, per
[[ADR-089-watching-is-how-marco-learns-performing-is-how-it-proves]], and an optional rehearsal
strengthens the same edge rather than making another.

## KNOWN FOLLOW-ONS

1. **The saved Play is not regenerated as the graph improves.** See the debt above. It is the one
   place where route-shaped information outlives the moment it was true.
2. **The semantic store has no delete path at all.** Forgetting a play unregisters a phrase and
   cannot reach the topology, because there is nothing to reach it with. The gate that holds
   "forgetting a play is not forgetting what Marco saw" is therefore holding a property that is
   currently structural — worth keeping, because the first delete anybody adds is when it stops
   being.
3. **The language layer above this is [[ADR-097-language-names-the-outcome-the-graph-decides-the-way]].**
   It is where a phrase becomes a destination, and it must never come to know about routes.
4. **Ranking is shortest-chain and nothing else.** A later direct edge wins over a two-edge route
   because it is shorter, not because anything weighed evidence. Preferring an edge Marco has
   verified over one it has only watched is a real question and is not this roadmap's.
5. **Live acceptance is UNMEASURED**, as it is for 36A, 36B and 36C. `acceptance-36c2.ps1` is the
   north-star check and it is deliberately small: teach one way into a screen, walk another
   without teaching it, and read the graph. Everything else this ADR claims is held by
   deterministic tests, which is where a claim belongs when a test can hold it.

## Enforced by

`cmd/director/onegraph_test.go`, in four groups:

- **The modes compose** — `TestTwoLearnEpisodesComposeIntoARouteNobodyDemonstrated`;
  `TestObserveAndLearnComposeInBothDirections` (both orders);
  `TestTheSameEdgeTaughtBothWaysIsOneEdge` (both orders, zero topology growth).
- **The name is meaning above the graph** — `TestTwoLearnNamesOverOneRouteDoNotDuplicateAnything`;
  `TestALearnedGoalNamesTheDestinationOnly`.
- **A learned route is not a route** — `TestALearnedRouteCanBeEnteredHalfway`;
  `TestAnotherWayInFoundLaterNeedsNoSecondLearn`;
  `TestAShorterWayFoundLaterIsNotBlockedByTheDemonstration`;
  `TestTheRouteIsChosenWhenSomebodyAsksNotWhenTheyDemonstrated`.
- **The knowledge outlives the artifact** — `TestLearnedEdgesOutliveTheEpisodeAndTheProcess`;
  `TestTheGraphDoesNotLiveInThePlayFile`; `TestForgettingAPlayIsNotForgettingWhatMarcoSaw`;
  `TestExplicitLearnDoesNotRelearnAPromotedEdge`.
- **The live door writes the same places** —
  `TestAPlaceEstablishedByALiveLearnPassIsOneAmbientWatchingKnows`;
  `TestALiveLearnPassRecognisesAPlaceWatchingEstablished`;
  `TestAnExplicitLearnOverFamiliarScreensEstablishesNothing` (the second afternoon, where the
  trail carries subject ids rather than descriptions — the arm a cold walk cannot reach).
- **Learning is not proving, and not acting** —
  `TestExplicitLearnRecordsObservationAndNotVerification`;
  `TestLearningRecentEvidenceTouchesNothingThatActs`.

And, already in the tree, the decomposition this rests on:
`internal/director/observesession` — `TestAMultiLegDemonstrationDecomposesIntoReusableEdges`
(one candidate per grown edge through the production runner, and no monolithic a→c record);
`internal/director/learn` — the multi-edge review suite.

## Related

[[ADR-095-repeated-observation-may-become-knowledge]] ·
[[ADR-094-observe-gathers-evidence-learn-promotes-it]] ·
[[ADR-093-observe-is-attention-not-recording]] ·
[[ADR-089-watching-is-how-marco-learns-performing-is-how-it-proves]] ·
[[ADR-056-a-goal-is-a-destination-not-a-route]] ·
[[ADR-027-what-marco-learned-becomes-marco]] ·
[[ADR-005-legal-marco-only]] ·
[[Passive-Observation]]
