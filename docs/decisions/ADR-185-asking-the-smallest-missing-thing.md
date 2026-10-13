---
type: decision
status: accepted
date: 2026-10-13
affects:
  - passive-observation
  - semantic-memory
---

# ADR-185 — asking the smallest missing thing

## Context

[[Experiment-035-what-a-control-looks-like]] ended a line of work rather than advancing it. The
`control_6` ambiguity — a 170x14 label inside the 352x34 button containing it — cannot be settled
by any descriptor of those rectangles, because the smaller one's pixels are a subset of the
larger one's. They are not two things that look alike. They are two extents of one thing.

The owner drew the conclusion, and it is the reason this note exists:

> We shouldn't build increasingly sophisticated visual matching just to avoid acknowledging
> uncertainty that the system has already identified.
>
> Marco shouldn't need perfect autonomous understanding to learn a useful workflow. It needs to
> know when understanding is insufficient and how to ask for the smallest missing piece.

The durable workflow already records the missing piece in words: *"clicked once, so nothing
independent confirms that this control can be found again"*. What was absent was a way for
somebody to answer.

## Decision

A **clarification** is evidence, attached to one control of one workflow, carrying who said it
and when. `Artifact.Questions` asks, `Artifact.Clarify` records, and the clarified evidence is
what reaches the matcher and the planner.

### What a question is, and when there is one

A control is asked about only when the matcher itself cannot tell it from a region that merely
contained its press — established by running `observe.ReacquireStored` over that container rather
than by restating `sameRect`, so the question and the refusal cannot drift apart.

That is narrower than "has a container", and the difference is load-bearing. `control_2` on
`rl-freeplay-1` is the TRAINING tile and its container is a slab of stadium crowd; the two are not
the same rectangle, the tile is found again on 21 looks of 28, and there is nothing to ask. An
earlier version asked anyway, which is how a person learns to click through prompts.

A control settled by a second press — [[ADR-182-two-controls-that-could-never-be-found-again]]'s
rule — has no container left at all and never reaches this.

### The question Marco actually asks

From the real artifact, on real evidence:

```
MARCO NEEDS CLARIFICATION — control_6, used by step(s) [7]

  one press was observed here, and it was inside more than one proposed region.
  Nothing in the demonstration says which of them is the control, and on a later
  look they cannot be told apart.

    [reading_1]    170x14 px at (868,620)
        the smallest region the press was inside, which is what this session
        took the control to be
    [container_1]  352x34 px at (802,610)
        a larger region that also contained the press

    [refuse]   none of these is the control
    [defer]    not now; ask me again
```

### The asymmetry, which is the interesting part

Naming the **larger** region settles the control. The person has said the button is what they
pressed; the label inside it is part of the thing rather than a region that merely contained the
press, so there is nothing left to be confused with and a proposal agreeing with the button
matches.

Naming the **smaller** region does **not** settle it, and Marco says so:

```
[reading_1]   -> still refused
    says the control is reading_1, which is recorded; but 1 larger region(s)
    still contained the press and cannot be told apart from it on a later look,
    so the control is still refused. A second demonstration would settle that.

[container_1] -> MATCHABLE
    says the control is container_1, and nothing recorded around it is larger,
    so there is no longer a region it could be confused with
```

This is not a rule about labels and buttons. It is arithmetic on two rectangles of one control:
whichever the person names, anything strictly inside it stops being a container, and anything
still larger does not. The asymmetry falls out of nested containment and is reported rather than
smoothed over, because a mechanism that claimed to settle both answers would be claiming a
discrimination [[Experiment-035-what-a-control-looks-like]] measured as absent.

### The three things a clarification is not

- **Not a rule.** The answer is about `control_6` of this workflow. A second control with the
  same shape is untouched, and a build that generalised would be deciding on a person's behalf
  about a control they were never shown.
- **Not recognition.** Choosing a region in a recorded frame says which interpretation the person
  meant. It says nothing about whether that region is on the screen now. Binding is unchanged and
  still refuses, with the three outcomes
  [[ADR-183-remembering-recognising-and-being-allowed-to-act]] gave it.
- **Not authorisation.** A clarified workflow is still unconfirmed and still unexecutable.
  Answering a question is not agreeing to press anything.

### Refusal and deferral are answers

*None of these is the control* closes the question and leaves the workflow honestly stuck. *Not
now* leaves it open, so somebody who deferred is asked again and somebody who refused is not
nagged. Both are recorded with their provenance.

## Consequences

- **Artifact version 2.** A version-1 file is refused rather than read without its
  clarifications: the field is where a person's answer about a control's identity lives, and a
  reader that ignored it would quietly match a control the person had said was something else.
- **A clarification is validated on the way in and on the way back.** An answer naming a control
  that does not exist, an interpretation never offered, or nobody at all is refused; so is an
  artifact whose stored clarification names either.
- **`control_6` is answerable now and still not answered.** Nothing in the tree has given an
  answer, because nothing should answer on a person's behalf.

## What this does not do

- It does not make the workflow executable, bindable, or recognisable.
- It does not settle `control_6` by choosing the label, which is the answer a person might
  reasonably give and which leaves the control exactly as refused as before.
- It is a contract and offline tests, not an interactive teaching interface. `perceptionreplay
  -clarify <artifact>` prints the question and what each answer would change, and records
  nothing.

## Enforced by

`internal/director/workflowstore` — `TestTheQuestionNamesTheStepTheControlAndBothReadings`,
`TestAControlSettledByASecondDemonstrationIsNotAsked`,
`TestAContainerThatCannotBeConfusedWithTheControlIsNotAsked`,
`TestChoosingTheContainingButtonMakesTheControlMatchable`,
`TestChoosingTheLabelIsRecordedAndStillRefused`,
`TestRefusingAndDeferringAreRecordedDifferently`, `TestAnAnswerIsValidatedAgainstTheQuestion`,
`TestAClarificationIsCarriedAcrossReloadWithItsProvenance`,
`TestAnArtifactWhoseClarificationNamesNothingIsRefused`,
`TestAClarificationReachesOneControlOnly`. **Mutation: 16 of 16 killed.**
