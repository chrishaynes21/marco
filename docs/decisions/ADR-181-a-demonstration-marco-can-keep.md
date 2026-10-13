---
type: decision
status: accepted
date: 2026-10-12
affects:
  - passive-observation
  - perception
  - navigation
  - execution
source_paths:
  - internal/director/workflow/workflow.go
  - internal/director/fastobserve/fastobserve.go
  - cmd/perceptionreplay/workflow.go
---

# ADR-181 — a demonstration Marco can keep

## Context

The owner corrected the direction, and the correction is the context:

> I've been treating Marco as a perception experiment when the whole point is to watch you do
> something once, remember how you did it, and do it for you later. […] The next milestone is not
> another observation benchmark. It is a workflow Marco can remember and eventually perform
> without making me demonstrate it again.

With it, a standing change to how this subsystem is tested:

> My recommendation is to stop requesting manual Rocket League sessions by default. […] If
> additional testing is needed, it should first determine whether an automated offline test can
> answer the question.

And the distinction that shapes the design more than anything else in the brief:

> Marco can recognize a geometric target again. It has not yet proven that it recognizes the same
> semantic control.

## Decision

`internal/director/workflow` is the missing middle: a **Draft** is the ordered record of one
demonstration with its evidence attached to the steps it belongs to, and a **Plan** is what the
Draft would do against the screen in front of it. Neither can press anything.

### Two refusals, kept apart

**A draft is not a Play.** `Step.confirmed` is unexported and `Confirm(i, by)` is the only thing
that sets it. No quantity of evidence sets it, `Confirm` needs a name because an unattributed
confirmation is indistinguishable from a default, and it refuses a step with nothing to press —
agreeing to a step whose evidence cannot support it would produce a confirmed thing that still
cannot run. `TestNoStepConfirmsItself` is what keeps it that way.

**Geometry is not identity.** `Support` has exactly two values, `none` and `geometric`. There is
no `SupportSemantic`, deliberately: nothing in Marco can currently establish that two readings of
a rectangle are the same CONTROL rather than the same place, and inventing the word before the
evidence exists is how a confident wrong answer gets a vocabulary. A planned action reports
`geometric` and the planner's refusal for an unconfirmed step says so in words.

### What makes a step repeatable, and the four ways it is not

| | |
|---|---|
| not a click | a load completing is not a thing to press; the response is to wait and verify |
| no target | the press landed on nothing the proposer found |
| no reading | nothing was kept to match against |
| no destination | an action nobody can verify is one nobody should take |

Each is a separate guard with its own test and its own words, because each sends a reader
somewhere different.

### The coordinates are not merely unused — they are out of scope

> Do not turn original click coordinates into permanent replay targets.

A `Step` holds the control's READINGS, not the press position. And `Plan` takes the reacquisition
outcomes rather than the correlator, so this package cannot reach the press evidence at all: a
planner that could read the original observations could read the original coordinates. The point
is computed inside one plan, from the look in front of it.

### It cannot act, held by the build

An import walk fails if the package can reach the lowering path, the execution pipeline, the
durable store, the action graph, the OS host or a recorder. A draft that could lower itself would
be an autonomous actuator with a reading interface. The lowering therefore happens in
`cmd/perceptionreplay`, which nothing live can reach.

The session-local tier's allowlist grew a fourth time, with the test failing first from both
sides.

### Verification has four outcomes

`arrived`, `waiting`, `elsewhere`, `unobserved`. Collapsing them is how a workflow reports success
for having pressed something: a destination that arrives by a load completing takes several looks,
and nobody-could-see is neither progress nor failure. The oldest rule in this subsystem —
delivery is not success, self-report is not proof — is the whole reason this is the screen's
answer and not the planner's.

## Measured on rl-freeplay-1

`go run ./cmd/perceptionreplay -frames -workflow .tmp/game-capture-review/rl-freeplay-1`

The draft is the demonstration, in its own order: **21 steps**, 13 of them clicks with geometric
support, 5 automatic, 3 unexplained. It reads as the route a person walked —
`visual_1 → 2 → 3 → 4 → 1 → 5 → 1 → 6 → 7 → 8 → 7 → 1 → 2 → 3 → 4 → 1 → 6 → 7 → 8 → 1 → 9 → 1` —
with `visual_1 --target_1--> visual_2` appearing at step 0 and again at step 11, which is the same
control clicked on both passes.

### The product answer: one step of twenty-one

```
      1 of 21 step(s) completed before it stopped.
      stopped at step 1 (visual_2 -> visual_3)
```

`target_2` on `visual_2` could not be resolved on **any** of the 62 looks of its own screen, so
the workflow stops at its second step. **A workflow that resolves most of its controls somewhere
is still a workflow that cannot be performed**, and that is the number to read.

### The capability answer: ten of thirteen

```
  #    from       target     resolved  refusals
  0    visual_1   target_1        139  ambiguous=74 unobservable=235
  1    visual_2   target_2          0  unobservable=62     <-- the stop
  2    visual_3   target_3         39  unobservable=25
  3    visual_4   target_4         51  ambiguous=3 unobservable=2
  5    visual_5   target_5          6  unobservable=5
  7    visual_6   target_6          0  unobservable=181    <-- would be the next stop
  8    visual_7   target_7         18  unobservable=33
  12   visual_2   target_10         5  unobservable=57
  15   visual_1   target_11       129  ambiguous=9 unobservable=310
  17   visual_7   target_12         6  unobservable=45

      10 of 13 repeatable step(s) could be resolved on at least one look of their own screen
```

So the blockage is **two controls**, not a general weakness of matching. `target_2` and
`target_6` are the same two that ADR-180 measured as never reacquirable, and they are now the
specific thing standing between a demonstration and a performance.

`resolved` counts LOOKS, and consecutive looks of one screen are not independent trials: a step
resolved on 40 looks of one visit was resolved once, forty times. The column that means anything
is how many steps have a non-zero count at all.

### It reaches legal Marco

Step 0's planned action lowers through `marcoexec` to a complete program, from a point computed
against the window in front of the planner:

```marco
the DirectorClick is an actor.

this can Run.
this's Run does...
    the at is a Point with X 128, Y 489.
    do OS's Click with at.
    this is ok!
```

Not run. The existing execution authorisation is untouched, Director still cannot inject, and the
replay tool is what took the last step precisely because the workflow package may not.

## Two measurement mistakes, recorded

**The cursor that was not there.** The first version planned every step whose source screen
matched the standing one, so a draft with three steps out of `visual_1` planned all three on every
`visual_1` look — and at most one could be right about where the screen was going. Steps 0, 11 and
15 printed identical counts, which is the tell: not three measurements but one measurement
reported three times with different expectations attached. A workflow has a position.

**A narration that contradicted its own table.** The report said "a plan whose expectation
disagreed with what followed would be wrong about the workflow, and none did" while printing 7 in
its own `elsewhere` column. Third of its kind this session, after ADR-178's green measurement of
nothing and ADR-180's red one.

Then the cursor exposed the opposite problem: a walk that stops answers only about the prefix, and
reporting it alone would describe a 21-step workflow by its first two steps. Both questions get
asked now, separately, because reporting one as the other is how a workflow that gets one step in
reads as a workflow that works.

## Consequences

- **The record → learn → plan → lower → verify path exists and is measured end to end offline.**
  Nothing in it is live, nothing is durable, nothing is executed.
- **A draft is session-local, and that is the one remaining blocker to the product.** It names
  screens identified by a count of rectangles in nine boxes, and those identities mean nothing in
  the next session. "Tomorrow: Marco, enter Free Play" needs cross-session screen identity, which
  is explicitly not authorised — so persisting a draft is the next decision rather than part of
  this one.
- **Two controls block the Rocket League workflow**, and they are named. That is a diagnosable
  question with the pixels still on disk, and it does not need another demonstration.
- **No new live evidence was requested.** Per the owner's standing change, the existing recording
  answered this.

## Enforced by

- `internal/director/workflow` — `TestADemonstrationBecomesAnOrderedDraft`,
  `TestTheDraftFollowsTheWalkAndNotTheEvidenceOrder`,
  `TestAnAutomaticStepIsNotRepeatableAndSaysWhy`, `TestEveryWayAStepFailsToBeRepeatable`,
  `TestNoStepConfirmsItself`, `TestAConfirmationNeedsSomebodyToHaveGivenIt`,
  `TestADraftWithNothingToDoIsNotExecutable`, `TestAnAutomaticStepDoesNotBlockAnAgreedDraft`,
  `TestAStepIsRefusedOnTheWrongScreen`, `TestAnUnfindableOrAmbiguousControlStopsThePlan`,
  `TestAnUncertainIdentityIsNotPlanned`,
  `TestAStepWhoseTargetTheMatcherNeverMentionsIsRefused`,
  `TestAnOutcomeFromAnotherScreenIsNotEvidence`,
  `TestVerificationSeparatesArrivedWaitingElsewhereAndUnobserved`,
  `TestAWorkflowCannotActOrRemember`, `TestAWorkflowNamesNothingDurableOrExecutable`,
  `TestThisPackageIsPartOfTheSessionLocalTier`.
- `internal/director/observe` — `TestAVisualScreenCannotReachDurableMemory`.

## Mutation gate

24 mutants, all 24 killed. One survivor was a **mis-aimed mutant** rather than a missing test:
`!s.Repeatable()` appears in `Executable` before `Plan`, so a mutant meant for the planner hit the
draft judgement instead. Re-aiming it exposed that `Executable`'s own skip was untested — a
version requiring every step to be confirmed would report every real workflow inexecutable
forever, since a load completing can never be confirmed, and Rocket League's demonstration has
eight such steps in twenty-one.
