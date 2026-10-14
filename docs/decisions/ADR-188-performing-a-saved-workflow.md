---
type: decision
status: accepted
date: 2026-10-13
affects:
  - passive-observation
  - execution
  - navigation
---

# ADR-188 — performing a saved workflow, one authorised action at a time

## Context

[[ADR-187-knowing-the-plan-and-trusting-the-screen]] took a reloaded workflow as far as it can go
without acting: a screen grounded in the observation in front of it, a control re-acquired from
the file's own readings, a step reported PLANNABLE, lowered into legal Marco and compiled. And
then it stopped, because nothing in the tree was permitted to run it and `workflowstore`'s import
walk keeps it that way on purpose — a saved workflow that could lower itself would be an
executable file in the strongest sense.

So the missing thing was never a capability. It was a **coordinator**: something outside the
artifact that sequences the five operations somebody else performs, in an order that cannot be
rearranged.

## Decision

`internal/director/workflowrun` advances one saved workflow. It holds no measure, captures
nothing, asks nobody anything and cannot build a host. `Advance` does one observation's worth of
work, and the order is the guarantee:

```
stopped?  →  usable?  →  ground  →  expire  →  destination  →  readiness
          →  authorise  →  stopped?  →  re-observe the world  →  lower + run
```

Everything before the last step can refuse, and refusing at any of them means nothing was sent.

### It cannot act, until a composition root says so

`New` takes an artifact, eyes, a grounder and a granter — four things incapable of emitting — and
stays incapable until `WithExecutor` hands it a `*marcoexec.Executor`. That is the same shape
`rehearse.Live` has and it exists for the same reason: a package that defaulted to a real host
would make every test one mistake away from clicking on somebody's desktop. `Acts()` exists so a
composition root can assert it wired the thing it meant to.

Its single route outward is that executor, so every action is a lowered, compiled Marco program.
Its own import walk fails if it can reach a host, the driver, the runtime, the compiler, screen
capture, the action graph or semantic memory — **and fails if it cannot reach `marcoexec`**,
because a forbidden list guarding a package that could not emit anything anyway is a test that
passes forever.

### An authorisation is for one action

[[ADR-183-remembering-recognising-and-being-allowed-to-act]] gave the three objects three
lifetimes and this is the shortest. `Authorisation` names the step, names who granted it, carries
the observation it was granted in, and expires; `Spendable` answers four questions separately,
because a caller that gets one "no" cannot tell a person whether to look again, agree again or
stop. A `Granter` is an interface, so the answer comes from outside the run every time it is
needed.

**Agreeing to a step and permitting a press stay two sentences.** `StepUnconfirmed` is reported
and the caller asks; the coordinator never answers it. A build that put both behind one prompt
would let a person answer one and be taken to have answered the other.

### The world is re-observed at the instant of the press

This is the part that was not in the design until a test asked for it. A click **carries no
address** — it lands wherever the pointer is, in whatever window is in front at that instant — and
`marcoexec`'s own guard does not cover the case, because that guard is for operations that NAME a
window and a click names a coordinate.

Worse, and specific to this design: **asking a person for authorisation is an unbounded pause in
the middle of the one sequence that must not have one.** The look that found the control is
seconds old by the time they answer, and in those seconds a menu can animate, a match can start,
a notification can take the window. The authorisation's stamp catches a *cached* permission from
an earlier screen; it cannot catch a world that moved, because a well-behaved granter copies the
numbers it was given.

So `WithConfirm` takes the sighting the plan was made from and is asked again immediately before
the press. In `cmd/director` it compares four things — the window is still there, its generation
has not changed, the target leads the desktop, and the observation generation has not moved. A
runner nobody installed it on, or installed `nil` on, **refuses every action**: an unverifiable
precondition on an operation that clicks wherever the pointer is, is not a precondition.

### A step moves on by arriving, and is never acted on twice

One sentence covers both kinds of step. `DestinationAt` is asked first, for the step the walk is
on, and it is the only thing that advances the cursor — so a press advances by arriving and so
does a wait. A step that has been acted on is only ever **waited for** after that: a second click
on a menu that did respond is a click on whatever replaced the control.

A press whose destination never arrives is the outcome this package exists to report honestly.
The run stops safely, and says that the press was sent and nothing grounded the screen it was
supposed to produce — *which is neither a completed step nor a failed one*.

### Patience, rather than stopping at the first refusal

A control mid-animation is the ordinary case, so a refusal is retried and the run stops safely
when the retries run out. Bounded at forty observations by default — four seconds at the cadence
[[ADR-175-no-settling-rule-survives-a-four-second-cadence]] established.

### `director perform-workflow`, and what "dry" means

The composition root. Without `--live`, the Marco runner is the **real front end** over
`internal/platform/recordhost`: every program is lexed, parsed, graphed, compiled and run, and the
last step appends a line instead of pressing a mouse button. `--live` replaces one Host and
changes nothing else — same coordinator, same refusals, same prompts.

> A dry run that took a shortcut past the compiler would prove that the shortcut works.

Ctrl-C stops the run rather than killing it, so a press in flight is refused and the report still
says how far it got. It writes nothing to disk and promotes nothing into durable memory.

### And the claim it will not make

`Report.Verified` is always false and carries the arrival contract's own reason, unaltered.
`Established` and `Independent` are two numbers because collapsing them would let a workflow
corroborate its own steps. The five rows the command prints are five sentences:

```
actions lowered, compiled and run          <n>
real input sent                            YES | NO — the Host wrote it down
destinations established                   <n> of <steps>
  of those, independent of the workflow    <n>
actions independently verified             NOT ESTABLISHED
```

## What it found

Two defects the design did not have, both found by writing the test before believing the code.

**The person was being asked to authorise a press without being told where it lands.** The point
only exists at `StepPlannable`, and authorisation is a precondition of being plannable — so at
the moment somebody is asked, `Action` is nil and a prompt built from it shows (0,0). Fixed where
the arithmetic lives: `StepReadiness.Found` carries the re-acquired centre from the moment the
control is found, which is before confirmation and before authorisation, and it is **not**
permission — `Action` is still the only field that means plannable. The alternative, the caller
computing the centre itself, would have been a second statement of this arithmetic, which is this
subsystem's recorded failure mode.

**A cancelled run still asked somebody a question.** A mutation survivor: deleting the stop check
at the top of `Advance` left the test passing, because the check before the press caught it — and
in between, the run grounded a screen and put a prompt to a person. The test now asserts about the
whole observation rather than only about what reached Marco.

## Consequences

- **A saved workflow can now be performed**, and only one action at a time, each one grounded,
  confirmed, authorised and re-checked.
- **`--live` is one Host.** The dry path is the same path, so what a dry run proves is worth
  something.
- **An arrival is still not a verified action**, and no caller can print the first and imply the
  second.
- **Nothing is written.** No file, no Place, no durable identity, however many steps succeed.
- **The next thing this needs is not more code.** It is a supervised live run, separately
  authorised, which is a decision rather than a change.

## Enforced by

`internal/director/workflowrun` —
`TestAWorkflowAdvancesOneGroundedActionAtATime`,
`TestEveryActionGoesThroughMarcoAsAClickAtAPoint`,
`TestThePressLandsAtTheControlOnTheDesktopAndNotInTheWindowsOwnCoordinates`,
`TestNothingIsPressedWithoutAnExecutor`,
`TestARunnerThatCannotRecheckTheWorldRefusesToAct`,
`TestAnActionIsRefusedWhenTheTargetDoesNotLeadTheDesktop`,
`TestAWorldThatMovedWhileTheQuestionWasOpenSendsNothing`,
`TestTheRecheckIsToldWhichObservationThePlanWasMadeFrom`,
`TestAStopAtThePromptPreventsThatPress`, `TestACancelledContextDoesNothingAtAll`,
`TestNobodyAuthorisedMeansNothingIsSent`,
`TestAnAuthorisationThatCannotBeSpentHereIsRefused`,
`TestALookNobodyCouldTakeIsNotActedOn`, `TestAPressedStepIsNeverPressedTwice`,
`TestAPressWhoseDestinationNeverArrivesStopsSafely`,
`TestARefusalIsRetriedAndThenStopsSafely`, `TestAProgramMarcoRefusesStopsTheRun`,
`TestAWaitAdvancesByArrivingAndPressesNothing`,
`TestAnInferredArrivalIsNotCountedAsIndependentEvidence`,
`TestNoRunReportsAVerifiedAction`, `TestRunStopsSafelyPartwayAndSaysSo`,
`TestARunnerWithNoEyesCannotBeDriven`, `TestTheOnlyRouteToAnEffectIsLegalMarco`.

`internal/director/workflowstore` —
`TestTheControlsPositionIsKnownBeforeAuthorisationIsAskedFor`,
`TestAStepThatCouldNotFindItsControlHasNoPosition`.

`cmd/director` — `TestAWorkflowStepReachesTheHostAsOneRealLeftClickThroughTheMarcoCompiler` and
`TestTheSameRealPathSendsNothingWhenNobodyAuthorisesIt`, which run the coordinator over the real
Marco front end with `recordhost` at the bottom; `TestPerformWorkflowIsDispatchedAndReachable`,
`TestPerformWorkflowInstallsItsGates`,
`TestWorldMovedRefusesEachWayTheWorldCanHaveMoved`.

**Mutation: 37 of 37**, after one real survivor (the cancelled run that still asked a question),
one mutant that had to be re-expressed because its first form was a compile failure, and one that
named text that was not there.
