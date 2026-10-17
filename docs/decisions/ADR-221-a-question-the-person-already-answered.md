---
type: adr
status: accepted
date: 2026-10-17
decision: two deterministic capabilities that need no model — a tie is settled when the kind of control the person named fits exactly one of the candidates in question, and a request whose state the application already reports as current performs nothing and says so, with the no-op resting on a structured source's own selection report and applying to a person's request rather than to a step of a procedure
supersedes: []
affects:
  - perception
  - execution
source_paths:
  - internal/director/target/target.go
  - internal/director/satisfied
  - internal/director/execute/pipeline.go
  - internal/director/program/program.go
---

# ADR-221 — a question the person already answered

Two capabilities, both deterministic, both from evidence already on the Stage, and both
identified as concrete gaps by
[[Experiment-070-the-interpreter-knows-some-meaning-and-does-not-know-what-it-does-not-know]].
Neither needs a language model.

## One: a tie the person's own sentence breaks

Paint shows a `Zoom` slider and a `Zoom` combo box. Asked for `"the zoom slider"`, resolution
matched both — the word is in both labels and in the request — and asked which was meant. The
person had already said which. They said slider.

[[ADR-218-a-request-reworded-in-the-screens-own-words]] permits exactly this and only this: a
tie may be broken by evidence that DISTINGUISHES the candidates, never by a preference. A kind
noun in the request is that evidence when it fits one contender and not the others, and it is
nothing at all when it fits several.

`Resolver.settle`, reached on AMBIGUOUS, with two conditions:

- **The kind must fit exactly one contender.** Two buttons both called Close, asked for as
  "the close button", are still two buttons: the person has narrowed nothing and the question
  stands. A word that is set aside but names no kind worth constraining by — "control",
  "area", "window" — is not distinguishing evidence either.
- **The re-asking must RESOLVE, and must land on one of the contenders.** The question was
  about those controls; answering it with a different one would be changing the subject. A
  lower-scoring control that only wins once the contenders are role-rejected was never what
  the person was being asked to choose between.

Together those mean settling can only ever turn one of the offered choices into the answer. It
cannot invent a third, cannot refuse, and cannot resolve a tie the person's words left tied.

Measured over the expanded regression sets: ambiguity preserved **3/3** on Settings and
**6/6** held out, with the two cases a named kind settles answered. Held-out accuracy
**79% → 83%**.

## Two: a request that is already satisfied

Measured during Alpha 1 on a live Settings window. Asked to open Apps while the Apps page was
already open, Marco found two operable controls labelled "Apps" and asked which was meant.
Every answer to that question was wrong in the same way — pressing either changes nothing —
and the question itself was the wrong thing to say.

Now: `ResultNothingToDo`, with the sentence *"Apps" is already the one you are on, so nothing
needs pressing*. Nothing is planned, no policy verdict is taken, no action record is written,
and the actuator is never reached.

`ResultNothingToDo` already existed, unused, documented as "the input asked for nothing". Its
doc is widened rather than a tenth status added: both cases mean no action was performed and
none was needed, which is the only thing a caller branches on.

### A no-op success must be supported by observed state

This is the part that needed care, because the failure mode is a system that reports success
by doing nothing.

The evidence is `Element.Selected` **and** `StateEvidence["selected"]` from a STRUCTURED
source — the application's own selection report, carried from the accessibility pattern
through fusion with the observation id that produced it. Two things could otherwise set that
boolean: a value nobody wrote, and a visual detector filling the gap from what a region LOOKS
like, which fusion records honestly as coming from vision.

Appearance is good enough to say "this looks like the current tab". It is not good enough to
tell somebody their request is complete. That is the same line
[[ADR-202-an-instruction-may-license-one-action-on-a-prop-only-pixels-describe]] draws for
operability, drawn again for accounting.

The answer carries its receipt — which Prop was read, and which sensor said so — because
"nothing needed doing" without one is indistinguishable from "I did not try".

### And it applies to a person's request, not to a step of a procedure

Found by the rename procedure, whose first step selects the file the person pointed at. In a
window where that file was already selected, satisfaction ended the request before F2 was ever
pressed, and *"Alpha.txt" is already the one you are on* was a true sentence about a rename
that did not happen.

A person's "open Apps" has no next step. A procedure's step one has three, and a precondition
already met means the procedure should CONTINUE. So `program.Context.WithinProgram` says which
this is, set by `runStep` and by an iteration's turn, and it exists for this one decision.

**The limitation that leaves:** a program step whose state is already set still performs its
action. Teaching every program to treat a no-op step as a success it can build on is a larger
change — later steps read the record the skipped step would have left — and it is not needed
to answer the question this exists for. Recorded rather than hidden.

### Why satisfaction is asked before the clarification

Because both the question and the plan are about HOW TO ACT, and the finding is that acting is
not called for. Asking which of two controls was meant, when neither needs pressing, is the
measured failure this fixes.

## What is deliberately not done

- No application-specific rules, synonyms or special cases. The kind nouns are the same UI
  vocabulary list ADR-218 uses, and selection is read from the one field the application sets.
- No inference from a page header repeating a navigation label. A rule like that would be a
  rule about applications that happen to print their page name, and the first one that did not
  would be told its request was satisfied when it was not — which does nothing and reports
  success.
- No change to `MinScore`, to substring scoring, or to any ambiguity the person's words do not
  distinguish.
- Verbs are limited to `open`, `select` and `choose`. "Click" is mechanical: a person who says
  click wants a press, and a selected control can be pressed again.

## Enforced by

- `internal/director/target` `TestAQuestionThePersonAlreadyAnsweredIsNotAsked`,
  `TestAKindThatFitsEveryCandidateSettlesNothing`.
- `internal/director/satisfied` `TestOnlyTheApplicationsOwnReportCanSayARequestIsDone`,
  `TestAnAmbiguousResolutionIsSatisfiedWhenOneChoiceAlreadyIs`,
  `TestOnlyADestinationRequestCanBeAlreadySatisfied`,
  `TestNothingIsInferredFromALabelThatMerelyRepeats`.
- `internal/director/execute` `TestARequestForAPageThatIsOpenPerformsNothingAndSaysSo` (over
  the real pipeline with a recording actuator: no clicks, no operations, no focus, no plan, no
  policy verdict, no record), `TestARequestForAPageThatIsNotOpenIsHandledAsBefore`,
  `TestAppearanceAloneCannotReportARequestAsAlreadyDone`,
  `TestAProgramStepWhoseStateIsAlreadySetStillRuns`.
- `cmd/director` `TestSatisfactionIsDecidedInExactlyOnePlace`,
  `TestRewordingNeverChoosesTheWrongControl` over the expanded sets.
