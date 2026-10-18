---
type: experiment
status: complete
date: 2026-10-17
backend:
  - accessibility
fixture: "the owner's live desktop under standing authorization for ordinary reversible operations. Windows Settings for the demonstration and the replay, and a disposable Notepad file for the wrong-application case. A cold MARCO_HOME, deleted before the run. No setting was changed: every step moved a navigation pane and nothing else."
result: "taught-named-saved-reset-replayed-repeated-and-cancelled-live-with-three-negative-cases-refusing-honestly-and-one-router-defect-found-by-the-live-run-rather-than-by-a-test"
supersedes: []
affects:
  - execution
source_paths:
  - docs/decisions/ADR-228-a-named-sequence-is-a-demonstration-somebody-kept.md
  - internal/director/demo/artifact.go
  - internal/director/demo/sequences.go
---

# Experiment 075 — a demonstration, saved, named and replayed

> The next milestone is a named, demonstrated sequence that can be saved and replayed. […] Test
> the full experience live: teach three to five steps, name the sequence, reset the UI, invoke it
> by name, repeat it a bounded number of times, and cancel an active run.

Run end to end on a cold store. Every line below is from the session.

## Teach, name, save

```
director demonstrate start                → Recording demo-20261018-005400.
  go to gaming                            → COMPLETED: "Gaming" became selected
  go to accessibility                     → COMPLETED: "Accessibility" became selected
  go to personalization                   → COMPLETED: "Personalization" became selected
director demonstrate stop                 → completed, applicationframehost

director save "my settings tour"
  my settings tour
    saved 2026-10-17 17:54
    demonstrated in Settings on 2026-10-18, as demo-20261018-005400
    asked for: "go to gaming", "go to accessibility", "go to personalization"
  Steps
    1 open "Gaming" (by name)
    2 open "Accessibility" (by name)
    3 open "Personalization" (by name)
  "my settings tour" is saved: 3 step(s).
```

Three steps, in order, under the person's own name. The goal extractor given the same recording
produces `learned applicationframehost open_file` with the first step as a parameter and the
other two frozen — one destination where the person demonstrated three.

## Reset, then invoke by name

```
go to bluetooth settings                  → COMPLETED  (the UI, reset)

run my settings tour
  3/3 observe   128 elements, coverage 0.73, actionable 0.78
  3/3 resolve   list_item "Personalization" (0.97)
  3/3 plan      open "Personalization" (medium risk)
  3/3 policy    allowed
  3/3 execute   open via double_click
  3/3 verify    "Personalization" became selected
  COMPLETED: 3 of 3 steps verified
  (3 action(s) performed)
```

A fresh observation per step, each target re-resolved from its name, and no per-step
confirmation — [[ADR-225-a-request-authorizes-the-task-it-asked-for]] applying to a replay
because a replay is an ordinary program.

## Bounded repeat, and cancel

```
run my settings tour 2 times
  1/6 "Gaming"   2/6 "Accessibility"   3/6 "Personalization"
  4/6 "Gaming"   5/6 "Accessibility"   6/6 "Personalization"
  COMPLETED: 6 of 6 steps verified     (6 action(s) performed)

run my settings tour 8 times
  FAILED: goal: saved sequence "my settings tour" expanded into a program the Director
  cannot run: the request needs 24 steps and the limit is 10; it is rejected rather than
  shortened, because running part of it would do something you did not ask for

run my settings tour 3 times     + director stop after ~3s
  cancelling "run my settings tour 3 times" — it will stop at the next safe point
  CANCELLED: stopped before step 5 of 9 (go to accessibility (run 2 of 3))
  (4 action(s) performed)
```

The eight-run refusal is the ordinary step ceiling, not a sequence rule, and its message is
better than one. The practical consequence: a three-step routine can be asked for at most three
runs. The cancellation names the run as well as the step, which is what makes `step 5 of 9`
readable.

## The three negative cases

> Include negative tests where the screen is wrong, a step cannot be reacquired, or an
> intermediate transition fails. No false completion claims.

**The wrong application.** Notepad in front, the Settings routine invoked:

```
[1/3] go to gaming — failed
  ! 1/3 resolve absent: nothing matching "Gaming" is present in the observed window
FAILED: step 1 of 3 (go to gaming) did not complete: nothing matching "Gaming" is present
```

Nothing was performed. No binding to "whatever is there", no partial run.

**A step that cannot be reacquired.** The saved artifact's second control edited by hand to name
a pane that does not exist — which is a thing a person can do, because the artifact is a file
they can read:

```
[1/3] go to gaming — nothing needed doing
[2/3] go to accessibility — failed
FAILED: step 2 of 3 did not complete: nothing matching "Nonexistent Pane" is present in the
observed window
```

Step one reported *nothing needed doing* — the already-satisfied path from
[[ADR-221-a-question-the-person-already-answered]] working inside a replay, because Marco was
already on Gaming. Step two failed with the real reason and step three never ran.

**An intermediate transition.** Covered by the same mechanism and visible above: each step's
outcome is verified against the world before the next one is planned, so there is no step whose
success is assumed from the one before it.

## Human interventions: one, and it was a defect

The first live invocation did this, with the routine saved and listed one command away:

```
director execute "run my settings tour"
  ! intent    I only understand click, focus, move, repeat and text editing so far
```

`HandleRequest` routes on `IsGoal`, and the sequence check had been written inside `expandGoal` —
which `HandleProgram` reaches and `HandleRequest` never got to. The eighth recorded instance of
complete code that production never invokes, and the first in a while that a live run found
before a test did. `TestARequestForASavedSequenceReachesTheProgramPath` now enters through
`HandleRequest`; deleting either the router's second question or the `expandSequence` call fails
it.

Apart from that: zero interventions. No confirmation prompts, no clarifications, no manual
grounding.

## What remains before this artifact replays in a pixel-only game menu

Stated against what the format now holds rather than in general.

**The format is ready.** A step carries a verb and a value; a control carries a role, or the words
it was read to say, or one of three ways of being addressed without a name. No handles, no element
ids, no coordinates — asserted over the serialised form. A game menu supplies no role and no
element id; what OCR reads off it is `Control.Text`, which is the field a saved sequence already
fills from a desktop demonstration.

**Three things are not.**

1. **No live capture writes one.** `director capture-game-session` records frames and input on one
   clock, and `perceptionreplay -frames -workflow -save` turns a recording into an artifact — but
   `director` itself never writes one, and that file carries no verbs because a frame recording
   cannot know what a press meant. Writing `Control.Text` from a recording is the measurable next
   step.
2. **Per-control refusal has nothing to refuse with.** The desktop replay's honesty comes entirely
   from *nothing matching "Gaming" is present* — a per-control statement. A pixel artifact's
   controls are regions, so the equivalent statement does not exist yet.
3. **Screen identity is not the way round it.**
   [[Experiment-076-nothing-the-artifact-carries-says-which-screen-this-is]] measured four
   discriminators and none of them separates a correct binding from an incorrect one. The
   conclusion is to stop grounding on screens, not to score them better — see
   [[ADR-229-proposed-a-single-qualifying-candidate-is-not-permission]].

## The mutation run

Fifteen mutations across the save path, the invocation parse, the router and the report. All
fifteen killed — after four rounds, and the four survivors are the interesting part.

| it survived because | the fix |
|---|---|
| `expandSequence` returned a hardcoded `true` for "I recognised this", so deleting its recognition check changed no outcome — the router's identical question was the only thing between that and claiming every request | return the second answer honestly, and pin the remaining property (recognition must cost no observation) through `HandleProgram`, the one entry the router does not guard |
| the run ceiling, replaced by a silent clamp to 25, still errored — from the program validator, about the number of STEPS | assert the refusal says *runs of*, because somebody who asked for a hundred and got twenty-five would be told their routine finished |
| the report's two new sections were tested by calling them directly, so deleting either from `printBinding` changed nothing | enter through `printBinding` |

The third is the one worth naming: a test that exercises a function instead of the report that
calls it is the same vacuous shape as a test that exercises a constructor instead of the
production path, and it had been written twice in one session.
