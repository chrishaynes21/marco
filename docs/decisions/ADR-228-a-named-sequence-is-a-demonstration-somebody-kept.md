---
type: adr
status: accepted
date: 2026-10-17
decision: an arbitrary demonstrated routine is saved as a workflow artifact carrying a semantic body — a verb and a value per step, a role or a label per control — under a name the person chose and that means nothing to Marco; it is addressed only by that name, reached by a router question that can match only when a saved name exists, and replayed through the ordinary goal expansion so that confirmation, policy, fresh grounding, per-step verification and cancellation are the ones every request gets
supersedes: []
affects:
  - execution
  - perception
source_paths:
  - internal/director/demo/artifact.go
  - internal/director/demo/sequences.go
  - internal/director/workflowstore/workflowstore.go
  - internal/director/goal/expand.go
  - internal/director/execute/goal.go
---

# ADR-228 — a named sequence is a demonstration somebody kept

## The two halves, and why neither was the answer

[[Experiment-074-two-halves-of-teach-and-replay]] measured two learning subsystems, each holding
the half the other lacked. The owner's call:

> We shouldn't build a third system. We should connect the two we already have.
> An arbitrary demonstrated routine should not be forced into a goal template.
> "my settings tour" doesn't need to mean anything to Marco. It's simply the name of a saved
> sequence of observed actions.

A Director demonstration knows the verbs, the targets, the transitions and the verification, and
`Extract` reads it as *a better way to achieve a goal Director already has a word for* — which
generalizes the first step into a parameter and freezes the rest, so three demonstrated
destinations become one. A workflow artifact is named, durable, inert, forbidden from carrying
handles or coordinates, and already what `director perform-workflow` replays — and its `Step` had
no verb, no value and no role, because a recording over frames observes a press and a screen
change and cannot know what the person meant by pressing.

## What made the seam small

Three things that were already there, two of them documented as waiting:

- **`Validate` already accepts a step whose `From` and `To` are empty.** A step that names no
  screen names no screen that must exist. So an artifact with a semantic body and no visual walk
  validates without the loader changing.
- **`Control.Text` was already there**, described by its own doc as *the strongest evidence of
  identity the artifact can carry* and *empty in every artifact recorded so far, because no
  demonstration writes it yet*. One does now.
- **The expansion loop was separable.** `goal.Expand` is four checks and then a fifth step; the
  fifth became `goal.ExpandProcedure`, with no behaviour moved. A named sequence is not a goal,
  has no pattern and must not acquire one — what it needs is everything after the lookup.

## The format: version 3

A `Step` gained `Verb`, `Value` and `Says`; a `Control` gained `Role` and the three ways a step
addresses one without naming it — `Deictic`, `Editor`, `Focused`. Nothing was removed and nothing
a frame recording writes changed meaning: a step with no verb is a transition, which is what
those steps always were.

The refusal list is unchanged and still asserted over the serialised form. A label is semantics,
not mechanics; handles, element ids, runtime ids and coordinates are still absent, and
`TestNothingMechanicalSurvivesASavedSequence` holds that over the durable form this time.

## What a sequence refuses to save

Each with a sentence, at save time, rather than as a binding failure three steps into a replay:

| | why |
|---|---|
| a sensitive step | a password in a durable file is the one mistake here that cannot be taken back |
| a captured program value | it names something the session had, which is gone |
| an unverified step | a sequence is a record of things that worked |
| a clarified step | somebody's answer was about that moment, not about this control |
| **a step that POINTED** | "this file" is whatever the person was looking at, and nothing in a file can say which object that was tomorrow |

A capture step is dropped rather than refused: reading a value is how a demonstration learned
something, not a thing the routine has to repeat. Everything else is a refusal, because silently
dropping an action from the middle of a sequence changes what the sequence does.

## The invocation, and why it hijacks nothing

> Named macros can use a separate invocation path, such as "Marco, run my settings tour."
> Do not add broad goal patterns that hijack existing UI commands.

`run <name>` and `replay <name>`, and **the lookup is part of the parse**. A request is a
sequence invocation only when a sequence of that name exists, so *"run the installer"* asks for a
routine only if somebody saved one called "the installer" and otherwise reaches the ordinary
request path untouched. Adding this took nothing away, and that is a property rather than a
promise: `TestRunningANameNobodySavedIsAnOrdinaryRequest` holds it over seven phrasings somebody
might really say.

A bounded repeat is `3 times` on the end, stripped before the name is looked up and then tried
again unstripped — because somebody is entitled to call a routine "my three times table".

## The repeat is the steps again, not a loop

So the bound is the ordinary step ceiling, the plan a person reads shows every step that will
happen, validation covers the whole thing before the first one, and cancellation between runs is
the cancellation between steps that already works. A loop would have needed its own bound, its
own report and its own cancellation check, and all three exist.

Measured live, and the interaction is worth stating: `policy.MaxSteps` is 10, so a three-step
routine can be asked for at most **three** runs, and a fourth is refused as *"the request needs
24 steps and the limit is 10; it is rejected rather than shortened, because running part of it
would do something you did not ask for"*. `MostRunsOfASequence` (25) therefore only ever fires
for a very short routine. The step ceiling is the real bound and its message is the better one.

## Where the wiring was missing, found live and not by a test

Worth recording in full, because it is the eighth instance of the same class and the test that
would have caught it did not exist until afterwards.

`HandleRequest` routes on `IsGoal`. The sequence check was written inside `expandGoal`, which
`HandleProgram` reaches and `HandleRequest` never got to. So with the routine saved, listed by
`director sequences`, one command away:

```
director execute "run my settings tour"
  ! intent    I only understand click, focus, move, repeat and text editing so far
  FAILED: I only understand click, focus, move, repeat and text editing so far
```

This is the same regression `TestTheProductionEntryPointReachesTheGoalLayer` exists for, one
layer later. The router now asks a second question, `IsSaved`, which reads the store and observes
nothing — no focus change, no input, no capability call — which is the property `IsGoal`'s own
comment is about.

## What this does not claim

Saving is not recognition and not authority.
[[ADR-183-remembering-recognising-and-being-allowed-to-act]] is unchanged: a saved sequence says
*this is what you showed me*. Whether the screen in front of Marco is the one it was shown is
[[ADR-229-proposed-a-single-qualifying-candidate-is-not-permission]], and on the desktop the
question does not arise in that form — a replay reacquires each control by name against a fresh
observation and refuses when it is absent, which is what the live negative tests show.

## Enforced by

`internal/director/demo` `TestADemonstrationSavesAsANamedArtifact`,
`TestASavedSequenceKeepsRolesAndMergesRepeatedControls`, `TestASequenceRefusesWhatItCannotReplay`,
`TestNothingMechanicalSurvivesASavedSequence`,
`TestASavedSequenceReplaysThroughTheOrdinaryProgramPath`, `TestASavedSequenceIsFoundByItsName`,
`TestTwoNamesThatFlattenToOneFileCollide`,
`TestAnUnreadableSequenceIsReportedRatherThanSkipped`,
`TestRunningANameNobodySavedIsAnOrdinaryRequest`,
`TestABoundedRepeatRunsTheStepsThatManyTimes`,
`TestASequenceRequestBecomesAnOrdinaryProgram`; `internal/director/execute`
`TestARequestForASavedSequenceReachesTheProgramPath`, which enters through `HandleRequest`.
