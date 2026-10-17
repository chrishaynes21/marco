---
type: adr
status: accepted
date: 2026-10-17
decision: the supervisor's attention curve is overridden to the busy cadence while a learning interaction is live, leaving the stored curve untouched, and the phase reports whether a press could have been received
supersedes: []
affects:
  - perception
source_paths:
  - cmd/director/observeambient.go
  - cmd/director/surfacelearn.go
  - cmd/director/surfaceavailability.go
  - cmd/director/surfacelearnphases.go
---

# ADR-215 — a person reading is not a quiet desktop

## The measurement

[[Experiment-067-the-whole-loop-ran-on-a-real-desktop]] produced two eighty-five-second watches of
the same page. In one the person clicked and the whole loop closed. In the other no press arrived
at all, and the reports were indistinguishable apart from the counters.

The gaps were the reason. A clarification cannot accept a press between bounded observation
sessions, because it is being derived again from a fresh reading
([[ADR-214-a-learning-interaction-outlives-the-session-watching-it]]), and the supervisor's
`attention` grows from one second to eight while nothing on screen changes. Measured across one
watch:

| | |
|---|---|
| Gap after rollover 1 | **4s** |
| Gap after rollover 2 | **8s** |
| Gap after rollover 3 | **16s** |
| Unanswerable | **~28s of 85**, concentrated at the end |

Concentrated at the end is the part that matters. That is when somebody who has read the sentence
gets round to clicking.

## The decision

The owner's framing, which is the decision and not a gloss on it:

> It's unreasonable to ask someone to demonstrate an action while the system periodically becomes
> unable to receive the answer. … when Director has an active learning interaction, its supervisor
> should remain attentive and resume observation promptly after rollover. When nothing needs
> attention, it can return to its existing backoff behavior.

A screen nobody is touching while a question is open is not a quiet desktop. It is a person
reading. So `ambientObserver.attentionNow()` returns `ambientBusy` while a learning interaction is
live, and the stored curve otherwise.

`surfaceLearner.Expecting()` is the one question asked: a question open, or an intent between
sessions waiting to be derived again. The second is the state the measurement was about — no
question is open, so backing off there is backing off through precisely the gap that has to close.
One accessor rather than two reads of `Open` and `Awaiting`, because a rollover swaps those two
under the learner's lock and asking separately can see neither.

### Both halves, or neither works

`attentionNow` governs three things, and they are one decision because any two of them disagreeing
restores the defect:

- the **gap between sessions**, in `run`;
- the **session's own sampling cadence**, passed to the runner in `watchOnce`;
- the **supervisor's poll** inside a session, in `pollEvery`.

A prompt gap with an eight-second cadence would bank a press and not offer it until the next poll,
and a press offered after its session has ended is refused however prompt the gap was.

### It is not a burst and it grants nothing

The stored curve is untouched, so an interaction that ends — answered, lapsed, expired or
cancelled — returns the supervisor to whatever backoff it had reached. It needs no window of its
own: the question's original deadline, which ADR-214 refuses to extend, is the bound.

It changes no observation and no rule. Sessions are still twenty seconds and still bounded, the
deadline is still the person's first one, the question is still derived again from a fresh Stage,
and a press is still matched by session and screen state. There is no second observation loop and
no continuous Stage collection — the same supervisor takes the same readings, with fewer dead
seconds between them. Nothing here can make an unanswerable press answerable; it can only shorten
the interval in which no press could be answered.

## Availability is reported, not inferred

Two consequences of the measurement, both in the phase:

**Nobody is invited to click at a question that is not open.** The invitation is the one sentence a
person acts on, and issuing it while no question can accept a press asks for something that will
be refused — which reads to them as their demonstration not counting.

**The watch reports whether a press could have been received.** `availability` samples `Open()` on
the tick the phase was already taking, prints each transition as it happens, and ends with the
totals, the number of gaps and the longest one. The longest gap is the number a person feels:
twenty-eight seconds in three gaps and twenty-eight in one are not the same experience.

It is sampled rather than built from the rollover and establishment events on purpose. Those
events exist, but an account built from them would describe what the learner says happened; the
question here is what a person at the keyboard would have experienced, and a tick asking "could a
press land right now" answers that directly.

The account closes by saying how to read the handoff beside it: a run with no gaps means an absent
press is an absent press, and a run with a long gap means an absent press is not evidence that
nobody pressed. Those two conclusions were indistinguishable before, and the difference between
them is the difference between a person's choice and Marco's defect.

## Enforced by

- `cmd/director` `TestAnOpenQuestionKeepsTheSupervisorAttentive` — four states in one table,
  because the property is the difference between them: a settled desktop keeps `ambientIdle`, an
  open question and an intent between sessions both get `ambientBusy`, and an ended interaction
  returns to the backoff. Each case also asserts the stored curve did not move.
- `cmd/director` `TestTheSessionCadenceFollowsTheSameAttention` — the in-session half.
- `cmd/director` `TestNobodyIsInvitedToClickAtAQuestionThatIsNotOpen`.
- `cmd/director` `TestAvailabilityIsReportedAsWhatAPersonWouldHaveExperienced` — driven, so the
  arithmetic is the caller's and nothing waits.
