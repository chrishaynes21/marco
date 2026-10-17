---
type: adr
status: accepted
date: 2026-10-16
decision: a learning interaction survives an observation session rollover by being derived again from a fresh reading, carrying only the instruction, the application, the durable Place and the person's original deadline
supersedes: []
affects:
  - perception
source_paths:
  - cmd/director/surfacelearn.go
  - cmd/director/observeambient.go
  - internal/director/surface/pending.go
---

# ADR-214 — a learning interaction outlives the session watching it

## The measurement

[[Experiment-066-the-question-outlives-the-session-that-owns-it]], live, at `6fe28d5`. The
production Director established a sound question by itself in `observe_2`, from a reading of its
own, with the durable Place confirmed across the boundary. The person pressed. The press was
captured — eleven presses across fifty-five offers — and refused thirty-six times, naming
`observe_3`, `observe_4` and `observe_5`.

Nothing was broken. Two constants disagreed:

| | |
|---|---|
| `ambientSession` | **20s** — ambient watching is a supervisor over bounded sessions |
| `HowLongAQuestionStaysOpen` | **90s** — how long a person has to answer |

`watchOnce` starts a session, samples it for twenty seconds, ends it, and the supervisor starts
another. Session ids therefore roll on regardless of what anybody is being asked, and a question
had a ninety-second life with a twenty-second window in which it could be answered — less whatever
establishment spent getting there.

## The decision

The owner named the principle, and it is the decision rather than a gloss on it:

> Observation sessions are implementation details. User intentions are not. A person shouldn't
> have to click within an arbitrary 20-second window just because Marco rotates its observation
> session.

So a **learning interaction** survives a rollover and a **question** does not.

The question stays the property of exactly one session and keeps strict session and screen-state
matching for every press it accepts. That is evidence integrity and it is untouched: screen state
ids are session-local counters, so `state_2` in two sessions are unrelated screens, and a press
can only ever be attributed to the screen it was performed on.

What crosses the boundary is the interaction: the instruction, the application, the durable Place,
and **the person's original absolute deadline**. In the next session the question is derived again
from a fresh Stage by the deferred-establishment path
[[ADR-213-a-question-belongs-to-the-session-that-can-answer-it]] already built — the same
re-derivation, the same identity checks, the same refusals.

### The deadline is set once

A question derived again is a new question, and a new question naively given the full bound would
renew the person's time every twenty seconds. The only limit on a learning interaction would then
be reset by the very thing it was meant to limit, and a request could stay answerable all
afternoon. So `learningInteraction.Deadline` is absolute, set when the interaction first opens a
question, and carried unchanged through every rollover. `surface.NewPendingUntil` exists for no
other reason.

Absolute rather than a remaining duration because a duration has to be subtracted at each
rollover, and a subtraction skipped once is a renewal nobody authorised.

### The press at the boundary is refused out loud

An observed press is never carried across. It is counted at the session gate, named in the
handoff with the two session ids, and then the interaction rolls — in that order, so a reader sees
both that the press was refused and that the learning went on. Reattaching it to the session that
replaced the question's would be inventing evidence about which screen somebody pressed.

### Retirement happens when the session ends, not at the next reading

Between one ambient session and the next there is a gap of the current attention — up to eight
seconds on a quiet desktop. A question that went on looking open across that gap would be
inviting presses nothing could attribute. So the supervisor retires it as it ends the look, and
`offerPresses` retires it too, because a reading from another session is independent evidence that
the question's session is gone and the supervisor may not have been the one to end it. Both doors
call `retireForRollover`, so there is one statement of what a rollover does.

### A settled question does not roll

It is no longer waiting for a press; it is waiting for the correlator to attribute a crossing to
the press that settled it, and that conclusion routinely arrives after the session the press was
made in has ended. Rolling it over would discard a demonstration the person had already given.
Its own deadline still bounds it.

## What this does not do

It does not relax session or screen-state matching, rewrite a session id, copy a session-local
screen state across a boundary, extend a question's life, add a fallback match, or change the
learner's rules, transition attribution, authorisation, the foreground protections or the
executor. It adds no observation loop: the trigger is a reading the supervisor was taking anyway,
and establishment is still single-flight on its own goroutine, off the thread that drains the
input log.

It also does not change `ambientSession` or `HowLongAQuestionStaysOpen`. Both numbers were right
about their own concern; neither was the span a learning interaction needed.

## What it costs

A press made in the gap between sessions, or while the question is being derived again, is
refused. The person presses once, so that window is a real exposure and it is reported rather
than hidden: the handoff names it as a press that arrived while the clarification was between
sessions. The gap is the current attention, which backs off to eight seconds on a screen nobody
is touching — so the residual exposure on a static screen is roughly the attention plus one
observation. Closing it further would mean driving the supervisor's cadence from whether somebody
has been asked a question, which is a separate decision about what watching costs and is not taken
here.

## Enforced by

- `cmd/director` `TestATeachingInteractionSurvivesASessionRollover` — two real registry sessions,
  a press at the boundary refused, re-derivation through the drain, and a second press settling
  the question in the session it belongs to.
- `cmd/director` `TestTheSupervisorRetiresTheQuestionWhenItsSessionEnds` — driven through
  `watchOnce`, the real supervisor step.
- `cmd/director` `TestTheOriginalDeadlineSurvivesEveryRollover` — three rollovers, one deadline.
- `cmd/director` `TestAnInteractionBetweenSessionsExpiresAtTheOriginalDeadline` — both doors that
  can meet the deadline while an interaction is between sessions.
- `cmd/director` `TestASettledQuestionIsNotRolledOver`.
- `cmd/director` `TestTheWatchDoesNotEndAtARollover` — the demonstration phase stopped watching as
  soon as no question was open, which was sound while a question could only end by being answered,
  lapsing or being cancelled. A rollover is none of those, so the watch would have ended about
  twenty seconds in, telling the person their demonstration was not seen while the Director was
  deriving the question for them.
