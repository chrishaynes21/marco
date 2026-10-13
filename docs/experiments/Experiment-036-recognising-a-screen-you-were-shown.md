---
type: experiment
status: complete
date: 2026-10-13
backend:
  - structure
  - vision
game: rocketleague
fixture: ".tmp/game-capture-review/rl-freeplay-1"
result: discrimination-already-works-and-every-measure-fails-the-same-way-on-absence
supersedes: []
source_paths:
  - cmd/perceptionreplay/screenid.go
  - cmd/perceptionreplay/screenidrun.go
---

# Experiment 036 — recognising a screen you were shown

**Status: RUN offline, 2026-10-13. No new capture, no inference during the measurement, no
actuation, nothing implemented. The recommendation is that nothing should be.**

## The question

> Can Marco identify a previously demonstrated screen using visual evidence saved with the
> workflow? […] Do not rely on arrangement similarity alone; Experiment-029 demonstrated that it
> can produce confidently wrong bindings.

Five measures over the same two independent folds, all reusing what is already in the tree:

| measure | what it is | where it came from |
|---|---|---|
| `arrangement` | proposals counted per 3x3 cell | the production relation, as baseline |
| `shape` | the same counts as proportions | exists, deliberately not the rule, never used for this |
| `palette` | one colour histogram of the whole frame | `internal/screen`, reused |
| `grid` | a colour histogram per 3x3 cell | palette plus where it is |
| `detector` | ScreenParser classes, counted | the recorded detections, reused |

Fold A's descriptors come from fold A's looks and fold B's from fold B's, so nothing is compared
with itself. The oracle scores answers and never produces one. No session screen id is evidence:
every measure is computed from pixels or proposals.

## The result

```
measure       rank 1  wrong absent BIND-WRONG refuse-ok worst margin
arrangement        6      0      2         2         1    +0.062  SEPARATED
shape              4      2      2         2         3    -0.087  no separation
palette            5      1      2         2         5    -0.000  no separation
grid               5      1      2         2         5    -0.002  no separation
detector           5      1      2         2         5    -0.027  no separation
```

Two findings, and the second is the one that matters.

### 1. Arrangement is the best of the five, and nothing beats it

Where a counterpart exists, the production relation puts it top **6 times out of 6**, with the
narrowest win at +0.062. Every other measure is worse: `shape` ranks two wrong, and the three
appearance-and-detector measures each rank one wrong with margins that cross zero.

The measures we went looking for in order to beat arrangement do not beat arrangement.

The appearance measures also behave exactly as [[Experiment-035-what-a-control-looks-like]]
predicted, now at screen scale: steady on screens that sit still (`visual_2` 0.99, `visual_7`
1.00) and collapsing on the ones that move (`visual_1` 0.35, `visual_5` 0.36, `visual_8` 0.27,
all with proposal counts swinging 56-373 across their own looks). A descriptor that reads
animation is reading animation whatever it is pointed at.

### 2. Every measure fails identically, and not at discrimination

**`BIND-WRONG` is 2 for all five.** The two screens fold B never saw are nominated confidently by
every measure, and the winner is **unopposed** in each case:

```
arrangement  visual_3   no counterpart;  best 0.886,  next 0.467
             visual_5   no counterpart;  best 0.559,  next 0.416
palette      visual_3   no counterpart;  best 1.000,  next ...
detector     visual_3   no counterpart;  best 0.996,  next ...
```

Those two numbers — 0.886 and 0.559 — are
[[Experiment-029-binding-a-screen-across-two-independent-folds]]'s two wrong bindings, located
precisely. That experiment's finding was never that arrangement discriminates badly. It is that
**nothing can say "none of these"**, and a measure that always nominates will nominate when the
answer is absent.

The conservative binder in [[ADR-183-remembering-recognising-and-being-allowed-to-act]] refuses
when two candidates clear the floor. Both of these are one candidate clearing the floor alone, so
it cannot refuse them — which is what that ADR already says in words, now with the numbers under
it.

### And the cost of the rule that does exist

`refuse-ok` is the correct answers the ambiguity rule would decline: **1** for arrangement,
**5** for palette, grid and detector. Switching to an appearance measure would turn five correct
recognitions into refusals and fix none of the wrong ones.

Run autonomously over every candidate, the existing binder on the existing relation gives
**5 bound correctly, 1 refused though correct, 2 bound confidently wrong** — the same shape
Experiment-029 reported, reproduced from a different direction.

## Why no measure can fix this

Each was checked against the absence cases directly:

- **A higher floor.** The wrong winner scores 0.886; a correct one scores 0.556. No floor
  separates them.
- **A margin over the runner-up.** The wrong winner beats its runner-up by 0.419; the correct
  `visual_7` beats its runner-up by 0.062. A margin test would reject correct answers and admit
  wrong ones — worse than useless.
- **Agreement between measures.** All five nominate the same wrong screen for `visual_3`. They
  agree with each other and with nothing true.

Absence is not a property of the comparison. It cannot be read out of a ranking, because a
ranking of candidates always has a top.

## The smallest justified proposal

**Do not automate binding, and do not add a measure.** Both would be answers to a question the
evidence says is not the problem.

What the measurement does support is a smaller question than the one Marco currently cannot
answer. Discrimination works when the screen is present; only presence is unknown. So:

> Marco: I think this is where "enter free play" starts — `screen_b`, at 0.901, with the next
> best at 0.407. Is this that screen?

A **yes/no about presence**, not a request to name a screen. It reuses
[[ADR-185-asking-the-smallest-missing-thing]]'s contract exactly: a question generated only from
demonstrated ambiguity, an answer that is evidence with provenance, narrow authority, and no
conversion into execution. And the measurement says what the yes is worth: on this recording,
when the screen was present the top-ranked candidate was right **6 times out of 6**.

That is one confirmation per workflow rather than per step, and it is the smallest thing that
turns a confidently wrong binding into a refusal.

**It is not implemented.** This experiment measures; the proposal is for review.

## What this does not establish

- **One recording, one split, eight screens.** Six of six is six trials, not a rate.
- **The oracle has its own uncertainty.** It is a third observer over the whole recording, and
  Experiment-029 recorded it splitting one screen into three. Ground truth here is *another
  observation*, not a fact.
- **Nothing about a later session.** Both folds share a resolution, a window, a build and a
  theme.
- **The presence question is unmeasured.** Nobody has been asked anything; what a person would
  actually answer, and how often they would be right, is not known.
