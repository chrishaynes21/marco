---
type: decision
status: accepted
date: 2026-10-14
deciders: director
affects:
  - passive-observation
  - execution
source_paths:
  - internal/director/workflowrun/workflowrun.go
  - internal/director/workflowstore/stepplan.go
  - cmd/director/reground.go
---

# ADR-196 — a control that moved is not a control that was covered

## Context

[[ADR-193-a-control-re-grounded-from-the-workflows-own-name]] built a repair for a control the
file cannot place, and bounded when it may be asked: only on `StepControlUnlocated`, the stage
where a recorded rectangle names nowhere. The comment beside the call site argued the exclusion,
and argued it correctly:

> A step that was refused for any other reason is not re-grounded: looking for a different
> rectangle because a control was briefly covered would replace a target that works with one that
> happened to be visible.

[[Experiment-044-two-live-runs-that-pressed-nothing-and-pressed-grass]] found the case that
reasoning does not cover. Rocket League's main menu is **bottom-anchored**, and signed out it
loses two entries and both side panels, so PLAY sits 95px below where the demonstration recorded
it. The recorded rectangle still locates something — the arena behind the menu — so the step is
refused `StepControlUnresolved`, which is the stage the rule excludes.

Sixty consecutive refusals, a run stopped, and the repair that exists for exactly this was never
asked.

## Decision

**The patience window is what separates a control that was covered from one that moved.**

A control mid-animation, or behind something passing over it, is refused `unresolved` and comes
back within the window. A control that has moved is refused on every look of it. So:

- `StepControlUnlocated` is asked about **at once**. No amount of waiting repairs a rectangle
  that names nowhere.
- `StepControlUnresolved` is asked about **once, on the look the run would otherwise stop on** —
  where the alternative is not a working target but no target at all, and the reasoning the
  exclusion rests on no longer applies.

Nothing else is asked about. A step refused because nobody has said which control was meant, or
because this run may not send another action, is a question for a person, and looking for a
rectangle instead would answer it without asking.

### A repair is itself an answer to "can it be seen now"

The second half, and without it the first changes nothing. `StepReadinessAt` asked
`outcome.Actionable()` — did re-acquisition succeed — **before** it consulted the session's
repairs, so a repair could never help a control that had not been found. It could only widen a
control that had been found in the wrong shape.

That ordering was wrong for the reason the repair exists. The regrounder establishes a region by
looking at THIS screen: the regions the sources proposed on it, and the words read inside them. A
control re-grounded there **was seen**, by something other than the file's own readings — and the
file's own readings are the thing that failed.

It does not skip `Locates`. A repair that named a band across the window is refused exactly as a
recorded one is.

### What the repair is told

Two stages now reach the regrounder and they are different findings. One sentence for both would
read as the first and send whoever reads it to the artifact, to look at a rectangle that is
perfectly well-formed.

```
  step 0: control_1 locates nothing. Looking for it on the screen instead.
  step 0: control_1 was not found on any look while the run waited for it. Looking ...
```

## Consequences

This is a repair reaching a case it could not reach, not a new power. The repair's own bounds are
unchanged: one control, one session, written nowhere, refused unless exactly one candidate on the
screen is worded in the workflow's own name, and reported as the workflow agreeing with itself —
which is weaker than somebody pointing at it.

**It did not make the measured run succeed, and the reason is the finding that matters.** On the
next live run a proposer happened to box a patch of pitch inside the old rectangle, re-acquisition
**matched**, and the step pressed grass 75px above the real button. Re-acquisition is
`sameControl`: mutual centre containment between a proposal and a saved reading, and it consults
no pixels and no text. So *"re-acquired on this look"* means only **a proposer drew a box in the
same place**, and a wrong match reaches the press ahead of the repair that would have been right.

That is [[ADR-197-a-geometric-match-nominates-and-does-not-identify]], which refuses such a match
before it becomes a press and hands it to the repair this ADR made reachable — in a single look,
because a contradiction needs no patience window to tell it from a transient.

## Enforced by

`internal/director/workflowrun` — `TestAControlNotSeenYetIsNotReGroundedWhileThereIsStillPatience`
(the exclusion ADR-193 defended, kept), `TestAControlNeverSeenIsReGroundedOnTheLastLookAndThenPressed`,
`TestTheRepairIsToldWhichRefusalItIsAnswering`,
`TestAStepThatIsMerelyUnauthorisedIsNeverReGrounded` (every plannable step passes through this
stage, so a rule that re-grounded whatever refusal it met would look for a new rectangle before
any step was ever pressed), `TestAStepRefusedForAnotherReasonIsNotRegrounded`.

`internal/director/workflowstore` — `TestARepairAnswersAControlThatCouldNotBeSeenAtAll`,
`TestARepairThatNamesAnotherBandAcrossTheWindowIsRefused` (unchanged, and the bound on the above).

## Related

- [[ADR-193-a-control-re-grounded-from-the-workflows-own-name]] — the repair, and the exclusion
  this qualifies.
- [[ADR-190-the-pointer-arrives-before-the-button-goes-down]] — the hover probe, which measured
  0.0/255 on the grass it pressed and did not stop it.
- [[Experiment-044-two-live-runs-that-pressed-nothing-and-pressed-grass]] — both runs.
