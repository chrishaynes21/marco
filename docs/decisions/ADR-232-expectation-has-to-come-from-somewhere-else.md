---
type: adr
status: accepted
date: 2026-10-17
decision: ADR-186's conjunction is not three independent terms — its expectation is established by an arrangement comparison, the same operation as its agreement term, so it cannot corroborate itself; measured, the conjunction is wrong 50.5% of the time against 52.5% and 53.0% for its halves, and taking the expectation from Marco's own act instead gives 3.2% on 1 of 19 steps. No pixel execution adapter is built, and the missing evidence is named rather than scored
supersedes: []
affects:
  - perception
source_paths:
  - cmd/perceptionreplay/conjunction.go
  - docs/experiments/Experiment-078-the-conjunction-is-two-terms-wearing-three-hats.md
---

# ADR-232 — expectation has to come from somewhere else

## What was asked, and the bar

> First, measure whether expectation plus visual agreement actually separates correct targets
> from incorrect ones. […] A promising match rate isn't enough if wrong actions remain common.
> I'd require zero wrong actions in the bounded acceptance suite before permitting live pixel
> replay.
>
> If it doesn't [improve discrimination], identify the missing evidence instead of adding another
> arbitrary score term.

[[Experiment-078-the-conjunction-is-two-terms-wearing-three-hats]] is that measurement.

## What it found

Four rules over 449 looks of a fold that had never seen the workflow, with a whole-recording
oracle for truth:

| rule | ready | wrong | wrong rate | steps advanced |
|---|---|---|---|---|
| control only | 117 | 62 | 53.0% | 19, **17 wrong** |
| agreement only | 259 | 136 | 52.5% | 19, **17 wrong** |
| **conjunction** | 105 | 53 | **50.5%** | 19, **17 wrong** |
| acted, then left | 31 | 1 | **3.2%** | 1, 1 wrong |

**The conjunction does not discriminate.** Two percentage points over its halves.

## Why — the finding

The expectation term is established by comparing the look to the step's saved **destination**.
That is an arrangement comparison: the same operation as the agreement term, on the same evidence.
So the rule reads as three terms and delivers two, and the consequence is in the last column —
**17 of 19 advances are wrong**, so from step two onwards the expectation points at the wrong step
and everything after it is a confident answer about the wrong place.

A rule cannot corroborate itself. [[Experiment-029-binding-a-screen-across-two-independent-folds]]
measured that arrangement similarity nominates an absent screen as confidently as a present one;
using it twice in one rule inherits that error twice and detects it neither time.

## So the missing evidence is named, not scored

It is **an independent source of expectation**, and three exist:

1. **A person saying where they are.** `workflowstore/anchor.go`, already built, valid for one
   observation generation. [[Experiment-036-recognising-a-screen-you-were-shown]] said why: *what
   a person adds is not a better ranking — it is the one thing no measure supplied.*
2. **Marco's own act.** A replay knows something an observer does not: that it just pressed. The
   fourth arm advances when it was ready and the look no longer agrees with the step's SOURCE —
   we acted, and we left. That is the desktop verifier's `menuClosedTaking` rule, and it is
   independent of what the destination looks like. **1 wrong in 31, against 53 in 105**, and the
   only difference is where the expectation came from.
3. **The words.** Independent of layout by construction, and
   [[Experiment-076-nothing-the-artifact-carries-says-which-screen-this-is]] measured them as
   anti-correlated at SCREEN scale. At CONTROL scale they are untested, and `TargetRecord.Text` is
   where they already live.

## The decision

**No pixel execution adapter.** 1 of 31 is not zero and 53 of 105 is not close, so nothing is
authorised to press from visual evidence. `Reacquisition.Actionable()` still says *could*.

**No new score term, and no threshold moved.** Every number in the measurement is production's
own: `VisualScreenMatch`, `sameRect`, `ReacquireStored`. The experiment adds a harness and four
readings of it.

**The act-based arrival is the next thing to measure, not to ship.** It is right 30 times in 31
and ready on 1 look in 449 — a rule that is nearly always right and nearly never ready has moved
the problem rather than solved it. Moving it was progress, because *not ready* is an abstention
and *wrong* is a press on the wrong screen; shipping it would be authorising something that
completes 1 step in 19.

## And the teaching experience, which is a product debt and not a finding

> `director demonstrate-visual "enter free play" --application rocketleague --seconds 45` is fine
> for development, but it's not a viable final teaching experience. Eventually I want to say
> "Marco, watch this." […] We don't need to implement the full voice experience this session. But
> it should remain the UX contract.

One step toward it, because the measurement needed it anyway: a demonstration now ends when the
person presses ENTER rather than when a clock runs out, `--seconds` staying the ceiling so a
forgotten terminal still stops. And the report answers the question a person demonstrating
actually has — *did Marco understand what I clicked on?* — by naming each control, how many times
it was pressed, **what it was read to say**, and whether anything changed after it.

`E2E.md` carries the two-minute walkthrough. It needs a person's hands, and that is
[[ADR-029-resolution-is-not-permission]] showing up as a product requirement: injected presses are
discarded at the callback, so Marco cannot demonstrate to itself and a demonstration keeps
meaning something.

## Enforced by

`cmd/perceptionreplay` `TestTheConjunctionIsMeasuredAgainstItsHalves` — four arms, the advance
outside the ready branch, the presses fed to the learning fold, and the oracle supplying truth.
`cmd/director` `TestADemonstrationEndsWhenThePersonSaysSo`.
