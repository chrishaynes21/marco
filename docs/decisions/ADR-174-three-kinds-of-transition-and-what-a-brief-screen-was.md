---
type: decision
status: accepted
date: 2026-10-12
affects:
  - passive-observation
  - navigation
  - perception
source_paths:
  - internal/director/observe/clicktarget.go
  - internal/director/observe/arrangement.go
  - cmd/perceptionreplay/transient.go
  - cmd/perceptionreplay/frames.go
---

# ADR-174 — three kinds of transition, and what a brief screen was

## Context

[[ADR-172-a-click-transition-is-a-source-a-target-and-a-destination]] attributed 13 of 21
observed changes to a press and reported the other 8 as "nobody pressed for it". The owner read
that and found the thing it was hiding:

> **The most important discovery isn't the nine screens.** It's that not every screen transition
> is caused by a click. […] A planner that assumes every edge requires an action will fail on
> ordinary applications, not just games.
>
> I would allow the graph to represent both kinds of transitions explicitly. […] Click /
> Automatic / Unknown. **The third category is especially important. Marco should be able to
> learn that something happened without pretending to know why.**
>
> But we must also avoid assuming that every unprompted visual change is a legitimate navigation
> transition. Animations, loading screens, and segmentation noise can produce the same
> appearance.

And on the short-lived screens:

> A short-lived screen could be a loading state, a genuine transient menu, or a false
> segmentation. The system should distinguish those outcomes rather than automatically deleting
> brief states. […] Preserve unknown when evidence is insufficient.

## Decision

### Three causes, and only one of them could ever license an action

| cause | what it means | what it implies |
|---|---|---|
| **click** | a press on a grounded target was followed by the change | resolve the target again, then act |
| **automatic** | no press, and it followed Marco's own ARRIVAL by click | wait and verify; press nothing |
| **unknown** | no press, and no arrival to hang it on | nothing may be executed |

`TransitionCause.Executable()` answers true for a click and nothing else. An automatic or unknown
step carries **no target at all** — a target on a step nobody pressed would be an invitation to
press it.

The automatic/unknown split is the load-after-a-click shape made into a test: a change out of a
screen Marco clicked its way into, soon after, is a consequence of that click finishing. A change
out of a screen it has merely been sitting on could be an animation, a splash, a flash of
mid-transition geometry or a real transient menu, and **nothing in this layer can tell those
apart** — which is what the third category is for. The bound is the same one a press gets,
because it is the same question one step along, and the gap is reported per occurrence.

### A brief screen is explained, not deleted — and there is no duration threshold

"Brief" is the symptom, not the cause. A loading screen is brief; so is a menu somebody passed
through quickly; so is a segmentation artefact. A number dividing them would be invented here and
would be wrong on the first application with a slower animation.

What separates them is what HAPPENED on them:

| role | evidence |
|---|---|
| **place** | somebody pressed something on it |
| **passage** | nothing pressed, and it was left automatically — the loading-state shape |
| **unexplained** | nothing pressed, and nothing explains how it was left |

The duration is reported beside the verdict and decides nothing.

## Measured outcome

### The causes, on 900 frames

**13 click, 5 automatic, 3 unknown.**

### Two of my own conclusions were wrong, and the classification corrected them

I had reported `visual_5` and `visual_9` as "short-lived screens, the shape of a load or a
splash". The evidence says otherwise:

- **`visual_5` is a place** — and more precisely a **dialog**. Every arrival at it was automatic
  and somebody pressed something on it: it appeared by itself and was dismissed with a click. 8
  looks, one press, `visual_5 --target_5--> visual_1`. A route through it is a wait and then a
  click, which is two steps in a particular order.
- **`visual_8` is the passage**, not `visual_5`. 15 looks, no presses, left automatically. That
  is the loading state.
- **`visual_9` is genuinely unexplained.** 2 looks, no press, arrived and left with nothing
  behind either change. It is not called an artefact, because the evidence cannot say that
  either.

Seven places, one passage, one unexplained — and nothing was deleted.

### Is it the same route twice?

The session settled on 22 screens in order. The longest run that repeats, **non-overlapping**:

```
visual_1 -> visual_2 -> visual_3 -> visual_4 -> visual_1     (5 screens, at positions 0 and 11)
```

Dropping passages and unexplained screens does **not** lengthen it. The two demonstrations share
that five-screen prefix exactly and then diverge — because in the first one `visual_5` appeared
and was dismissed, and in the second it did not. `visual_5` is a *place*, so collapsing it is not
licensed.

**So: the two demonstrations are not the same route end to end. They share an identical
five-screen prefix.** That is a fact about what the person did, not a defect in the reading.

### The same step, caused differently on two occasions

```
visual_1 -> visual_6   click x1   automatic x1
visual_3 -> visual_4   click x1   automatic x1
```

A load that took longer than the attribution window reads as automatic. **The route is the same
either way**, and a planner reading only the causes would conclude the two walks were different
journeys. This is the single most important caveat in this decision.

### The ambiguous target pairs, on geometry alone

Reaching the same destination is how a pair was noticed; it is deliberately not part of the
comparison, because two entries can open the same thing.

| pair | centres apart | area ratio | overlap | verdict |
|---|---|---|---|---|
| `target_2` / `target_10` on `visual_2` | 0.370 | 7.6× | yes | a control and the thing containing it |
| `target_7` / `target_12` on `visual_7` | 0.139 | 73.2× | yes | a control and the thing containing it |

**Neither pair is one control.** So the conservative identity rule chosen in ADR-172 was right,
and the candidate-set rule that would have merged them — and reported four repeated units instead
of three — would have been **wrong**. The extra unit it bought was an error.

## Consequences

- **An automatic transition can never imply an executable click**, and that is a property of the
  type rather than a discipline: the step has no target to click.
- **`unknown` is a first-class answer**, and three changes on this recording are in it.
- **Three repeated click units stands**, and the measurement now says why it is not four.
- **Route-level stability is partial**: an identical five-screen prefix, then a genuine
  divergence. Reporting it as one route would have required collapsing a screen somebody clicked
  on.
- **A cause disagreement is not a route disagreement.** Two steps of this route were caused
  differently on the two occasions, and anything that plans over this has to treat the cause as
  per-occurrence evidence rather than a property of the edge.
- Nothing is durable, nothing is executable, and the live path still calls none of it. The
  passive integration plan is [[Passive-Visual-Screen-Integration]] and is a plan, not a wiring.

## Two bugs this change found in its own code

- `compareTargets` computed a centre's x as `x + height/2`. Square fixtures hid it, because then
  the wrong term is right by accident; the test now uses rectangles whose width and height
  differ, and the mutant that reintroduces it dies.
- My first assertion about the repeated-run rule was wrong rather than the rule: `a b a b a`
  repeats `a b` twice, not `a b a`, because the two occurrences may not share an index. The
  comment now says why.

## Mutation gate

22 mutants, all 22 killed.

## Enforced by

- `internal/director/observe` — `TestAChangeAfterArrivingByClickIsAutomatic` (including that an
  arrival explains only the screen it arrived AT),
  `TestAChangeWithNoArrivalBehindItHasAnUnknownCause`,
  `TestAnArrivalTooLongAgoDoesNotExplainTheNextChange`,
  `TestTheSessionRecordsTheOrderItSettledIn`.
- `cmd/perceptionreplay` — `TestABriefScreenIsExplainedRatherThanDeleted`,
  `TestTheLongestRepeatedRunIsContiguousAndDoesNotOverlapItself`,
  `TestDroppingTransientsDoesNotInventAStep`,
  `TestAmbiguousTargetsAreComparedOnGeometryNotOnDestination`,
  `TestAPressBeforeALayoutChangeBecomesALearningUnit`.
