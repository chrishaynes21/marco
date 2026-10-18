---
type: adr
status: accepted
date: 2026-10-18
decision: ACCEPTED — a demonstrated input is primary evidence and is never discarded because perception could not interpret it; perception supplies corroboration, context, contradiction and outcome, and may refuse to REPLAY an action it cannot ground, but may not erase the record that the person made it. Marco spends computation before it spends the person's attention.
supersedes: []
affects:
  - perception
  - learning
source_paths:
  - internal/director/observe/clicktarget.go
  - internal/director/observe/proposal.go
  - internal/director/workflowstore/clarify.go
  - cmd/director/demonstratevisual.go
---

# ADR-234 — the person does not have to prove the click

## The owner's statement, which is the decision

> The user shouldn't have to prove to Marco that they clicked something. Marco witnessed the
> click. Its job is to determine how to repeat that action reliably — not interrogate the user
> because its visual detector couldn't name the button.
>
> That should become a permanent design principle for Director.

And the ordering it implies:

> Marco should trust the user's demonstrated actions more than its own interpretation of the
> screen, and it should spend computation before spending the user's attention.

## Why it needed saying

Three experiments in a row localised a blocker in perception and each one moved it.
[[Experiment-082-a-control-that-is-half-the-window]] read as *the click-to-target rule is wrong*;
[[Experiment-083-the-label-of-the-button-you-click-is-the-one-that-is-missing]] showed the rule
was right and the text was misleading; [[Experiment-084-the-highlight-that-hides-the-label-draws-the-box]]
showed the frames around the press were worse, not better. Each report ended by naming a missing
observation, and the cost of each was a demonstration the person had to make again.

Meanwhile the thing that was never in doubt was the press itself. The recorder had the
coordinate, the window, the look and the screen on one clock, from hardware, with provenance. The
weakest evidence in the system was being allowed to veto the strongest.

[[Experiment-085-the-graphics-tab-is-a-button]] then measured the premise of all three and found
it false: Ark's proposer does offer button-sized rectangles, and one demonstrated click resolves
to a 138 × 30 boundary holding exactly one label.

## The decision

**1. A demonstrated input is primary evidence.** The coordinate, the window-relative position,
the screen that was standing, the look, the ordering and the provenance are what physically
happened. Nothing in perception may delete them.

**2. Perception interprets; it does not adjudicate the record.** Vision and OCR supply
corroboration, context, contradiction and outcome evidence. A proposer that found no
button-shaped rectangle has failed to interpret an action, not witnessed a non-action.

**3. Two different questions, kept apart by name.**

| question | answered by |
|---|---|
| *where did the person act?* | the demonstration. Settled. |
| *is that place safe to press now?* | binding, freshness, corroboration and the foreground gate. Usually not settled. |

A demonstrated coordinate is a complete answer to the first and **no part of an answer to the
second**. So it is shown, reported and kept as evidence, and it never becomes a durable control
identity or a replay target — which is [[ADR-181-a-demonstration-marco-can-keep]] and
[[ADR-183-remembering-recognising-and-being-allowed-to-act]] unchanged, not weakened.

**4. Compute before clarification.** Before asking, Director checks what was explicitly
requested or demonstrated, reuses the context it has, examines the input timeline and the nearby
observations, uses vision and OCR where they are independent, considers a bounded set of
alternative readings, and asks only if the remaining distinction would change what Marco must do.
The default during active teaching is **no interruptions**.

**5. An imperfect demonstration is a normal demonstration.** A session carrying a stray key, a
mistaken click or an unsegmentable press is not a failed session. The parts that can be repeated
are offered; the parts that cannot are named as themselves.

## What this is not

It is **not** permission to act on a coordinate. A press with no candidate region has no boundary
to find again, so it produces no step and is refused at replay with a specific reason. The
refusal moves from *the demonstration is void* to *this one action cannot be grounded*, which is
a smaller refusal about a specific thing.

It is **not** a licence to infer intent. A key whose effect was never observed is neither
promoted to a deliberate step nor deleted; it stays recorded and unresolved. Where causality is
ambiguous the ambiguity is preserved rather than resolved by a default.

It is **not** a new subsystem. No question-policy framework, no second question system, no new
workflow engine or artifact format.

## Enforced by

- `internal/director/observe` `TestAPressNoProposalContainedIsKeptAsWhatThePersonDid` — the
  production correlator retains a blind press as an observation with its coordinate, screen and
  look, and `len(BlindPresses) == Blind` so the arithmetic cannot drift from the evidence.
- `cmd/director` `TestTheReportSaysWhatYouPressedThatMarcoCouldNotSee` — the demonstration report
  names each such press as an act, and says plainly that it cannot be replayed.
- `cmd/director` `TestATeachingSessionWithAMistakeAsksNothing` — a session carrying a stray key
  and an unsegmentable press still produces its usable step and opens no question.
- `internal/director/observe` `DefaultProposalThresholds` holds `MaxOpen: 1`, and
  `internal/director/workflowstore` `Artifact.Questions` asks only where the matcher itself would
  refuse the control as indistinct.

## Appendix — the audit, done before any new abstraction

The owner's instruction was to trace the real paths before writing a policy. Every question
Director can put to a person comes from one of seven places. There is no eighth, and nothing here
needed a new framework.

| raised at | kind | trigger | gate it already has |
|---|---|---|---|
| `observe/proposal.go` | `AskSemantic` | a hypothesis earned validation | materiality digest; re-ask only if the judgement changes shape |
| `observe/learning.go` | `AskLearnRelationship` | a transition Marco could characterise | named refusal for every case it declines to ask |
| `observe/assessment.go` | `AskSecondDemonstration` | one example cannot be generalised | only after the person already asked to learn |
| `observe/rehearsal.go` | `AskRehearse` | a route is eligible and unauthorised | already-granted, already-asked, declined-on-same-evidence |
| `observe/screennaming.go` | `AskNameScreen` | a verified play cannot be written without a name | already-named, subject no longer held, budget |
| `workflowstore/clarify.go` | `Questions()` | the matcher itself would refuse the control as indistinct | a control settled by a second press never reaches it |
| `surface/question.go` | surface choice | competing readings a single press could separate | only when a press could actually tell them apart |

Two things were already true and are worth stating, because the direction being set here could
otherwise be read as a criticism of code that does not deserve one.

**`DefaultProposalThresholds` is `MaxOpen: 1`.** At most one question is open across a whole
session, and a declined one returns only when the judgement's own digest changes. The budget for
interruptions was already one.

**And the measured count on real work is zero.** `Artifact.Questions()` over all three saved Ark
workflows — `ark_twice`, `ark_menu_2`, `ark_menu_test` — returns nothing open on every one. The
path designed to ask about an ambiguous control asks about none of them, because each control was
either settled by a second press or was never confusable with its container.

So the cost the owner experienced was not questions. It was three things that are not questions:

1. **Re-demonstration.** Each of Experiments 082–084 ended by naming a missing observation, and
   the cost of each was a session the person had to perform again. [[Experiment-085-the-graphics-tab-is-a-button]]
   is what one of those finally measured, and it contradicted the premise of all three.
2. **A discarded press.** A click no proposal contained was an integer. Fixed here.
3. **A fact repeated per step.** `Draft.Unresolved` printed one sentence about one control once
   for every step that pressed it, so a control demonstrated twice produced two identical lines.
   A person reading two identical lines counts two problems and has one. Merged here, naming
   every step the one answer settles — which is what `workflowstore.Question.Steps` already does
   on the other side of the same idea.

What was NOT changed, deliberately: no question was deleted on the grounds that it is annoying.
Every one of the seven paths asks about something that would change what Marco does, and the one
that could be measured asks nothing on real data.
