---
type: decision
status: accepted
date: 2026-10-16
deciders: director, owner
affects:
  - perception
  - execution
source_paths:
  - cmd/director/surfacelearn.go
  - cmd/director/surfacelearnphases.go
  - cmd/director/runtime.go
---

# ADR-213 — a question belongs to the session that can answer it

## The measured defect

A request makes ambient watching **stand aside**: its observation session is cancelled so that
nothing the Director does is folded into evidence about what the person did, and the supervisor
starts a fresh one afterwards. That is deliberate, and the call site in `observeregistry.go` says
so in as many words — *"the session ENDS, so nothing the Director does is folded into it, and the
supervisor starts a fresh one afterwards."*

But the surface question raised by that request was stamped from `standing()`, read immediately
after the request returned, which reports the session of the **last ambient look** — the one just
cancelled. So the question was born carrying a session that no longer existed, and every press
the person then made arrived in the session that had replaced it.

Measured live, with the account from
[[ADR-212-a-silence-is-reported-as-a-silence]]:

```
offers ................... 56
presses seen in the log .. 14
another session .......... 54      every press
settled / undiscriminating / elsewhere / other ... 0 0 0 0

  "the reading is from session observe_2 and the question was asked in observe_1,
   so no press in it can be matched to the question's screen"
```

Deterministic, not a race: it happened to every surface question raised through a request while
ambient watching was on, which is the only way this path raises one. The screen-state gate was
never reached, so the earlier reading — a press attributed to the destination screen after
navigation rather than the source screen it was made on — is not what happened.

## What was NOT changed

The rule that refused the press is correct and is untouched. A press must come from the session
its question was asked in, because screen state ids are session-local counters and `state_2` in
two sessions are unrelated screens. Session matching is not relaxed, session ids are not
rewritten, and no session-local screen state is copied across a boundary. The learner, the
resolver, the policy ladder, transition attribution, the authorisation gates, the foreground
checks, cancellation and the executor safeguards are all as they were.

## What was changed

**A request no longer opens a question it cannot have answered.** When the session a question
would be stamped with is not the session that is running, the learner holds an `awaitingQuestion`
— the request, the application and the durable Place, and nothing executable. No question, no
options, no screen state: carrying those would be the stale question wearing a new label. The
durable Place is the one identity allowed across, because it is the only one that is not a
session-local counter.

**Establishment is a separate step against a fresh reading.** `Establish` opens the question from
a reading taken in the session a press will arrive in, and the caller re-derives that question
from its own observation of the screen — the same `surface.Evaluate` the pipeline uses, asked
again. It refuses if nothing is waiting, if the reading is not the running session, if the reading
placed no screen state, if the application changed, if the durable Place changed, or if the fresh
reading raises no answerable question at all. A clarification that no longer describes the screen
is void rather than transferable.

**A later request abandons a waiting intent**, exactly as it already closed an open question: the
person asked for something else, and the press they make next belongs to that.

The account names all of it. `deferred` says which session was dead and which is live;
`established` says where the question actually opened; `refused` says which way the screen had
moved on.

## Consequences

The demonstration path now reports an extra line before asking anybody to press anything — which
session ended, which resumed, and that the question will be established from a fresh reading in
it. The press point printed to the operator comes from the **established** question rather than
the request's, because those are two derivations from two readings and the person must be told
about the one that is open.

## The daemon, which is the path that matters

A harness is not the product. The same sequence has to happen during ordinary operation, so the
deferred clarification is established by the Director itself:

1. A request needs clarification, and the session it was observed in has already been cancelled.
2. The intent and the durable Place are held. Nothing executable, no options, no screen state.
3. The supervisor resumes in a fresh session and reads the screen.
4. That reading triggers one establishment, which takes a Stage of its own.
5. The place, the application, the target, the ambiguity, the session and the screen state are
   all confirmed from that Stage, and the question opens only if every check passes.
6. The person demonstrates, and the existing guarded learning path is unchanged.

**Where the trigger lives, and what it may not do there.** In the drain — the one place a reading
and the admitted input log are both in hand, beside the presses being offered. So it runs on the
supervisor thread, and all it may do there is compare a few fields: a Stage collection on that
thread would stall the watching that has to notice what the person did. The observation happens
on its own goroutine instead, **single flight**. Readings arrive about ten times a second and an
observation is a hundred times slower, so without that flag every reading would start another one
and they would queue behind the accessibility bridge until the intent expired. There is no polling
and no second observation loop: the trigger is a reading the supervisor was taking anyway, and the
observation is `pipeline.Observe` — the production path the request itself used.

**The intent is bounded.** Thirty seconds, which is two orders of magnitude more than the
supervisor needs to resume. It exists because the screen the person asked about can disappear
while nothing is watching — they close the window, a game takes the foreground, watching stops —
and an unbounded intent would sit there until some later request happened to replace it, then
establish a question about a screen that had been gone for an hour. A reading that merely does not
match is not a refusal: the person may be elsewhere for a moment, so the trigger reports not-yet
and the expiry decides. A failed observation leaves the intent waiting too, because missing
evidence is not contradictory evidence; what ends it is the bound, not an invented refusal.

**The operator is told the new geometry.** The `established` note carries the rectangle a press
would answer at, from the question that actually opened — not the one the request produced, which
came from a different reading of a session that has ended.

## One path, because two would not be evidence

The demonstration harness could establish a question too, and being synchronous it would usually
win the race. A demonstration completed that way would be evidence about the harness rather than
about the Director anybody will use — the owner's words, and the reason for the final shape:

> the harness is not the product.

So the harness establishes nothing. It waits for the Director, reports the question the Director
opened — its session, its screen state, its place, and the geometry it derived — and stops at the
authorisation checkpoints as before. There is one establishment path, and it is the production
one. `surfaceHypothesisFor` and `Establish` are untouched and shared.

The wait outlasts the intent on purpose: an intent expires after thirty seconds, so a wait that
ended first would report a stall where the Director was about to report a reason. A refusal is
noticed as soon as it happens rather than waited out, because the Director records a reason when
a place moved, an application changed or an ambiguity resolved itself, and that reason is the
answer.

## Enforced by

- `TestAQuestionRaisedInAnEndedSessionOpensNothing` — the live shape exactly: `observe_1` on the
  look, `observe_2` running, nothing opened, the intent held, the account naming both.
- `TestAQuestionRaisedInTheLiveSessionOpensAtOnce` — the deferral is for a dead stamp only.
- `TestADeferredQuestionIsEstablishedAndThenAnswered` — end to end: deferred, established from a
  fresh reading, and a press from that session settles it through the production drain.
- `TestAPressFromAnUnrelatedSessionIsStillRefusedAfterEstablishment` — the guard is not weakened
  by any of this.
- `TestEstablishmentRefusesAScreenThatMovedOn` — the place changed, the application changed, the
  reading is not the running session, the reading placed no screen state.
- `TestEstablishmentRefusesAQuestionNobodyCouldAnswer` — the ambiguity resolved itself.
- `TestALaterRequestAbandonsADeferredIntent` — cancellation reaches the intent, and nothing can be
  established afterwards.
- `TestEstablishmentRefusesWhenNothingIsWaiting`.
- `TestTheDaemonEstablishesADeferredClarificationFromAFreshReading` — the production sequence end
  to end, with a real session in the registry: deferred, the supervisor reading again, the
  question established in that session, and the new geometry in the account.
- `TestTheDoorHandsOverTheRunningSession` — the line that survived the first mutation run, now
  entered with a live observation registry.
- `TestADeferredClarificationExpires` — a clarification nobody could establish does not wait
  forever, and says how long it waited.
- `TestOnlyAReadingAPressCouldArriveInTriggersEstablishment` — six readings that must not start
  one, and the intent left waiting rather than discarded by any of them.
- `TestAValidReadingTriggersEstablishment` — and the one that must.
- `TestAFailedObservationLeavesTheClarificationWaiting`.
- `TestALaterRequestThroughTheDoorAbandonsTheIntent`.
- `TestTryItOutWaitsForTheQuestionTheDirectorEstablished` — the harness reports the question the
  Director established, triggered through the drain as in production.
- `TestTheHarnessEstablishesNoQuestionItself` — read from the phase source, because what is held
  is the ABSENCE of a call: a second path would fail no test, it would simply win the race.
- `TestOnlyAQuestionFromANewSessionCountsAsEstablished` — a question still stamped with the
  session the request ended is not one the Director established.
- `TestARefusalIsNoticedRatherThanWaitedOut` and `TestTheWaitEndsAsSoonAsTheDirectorHasRefused`.

- `TestOnlyOneEstablishmentRunsAtATime` — twenty readings of the same screen start exactly one
  observation.
- `TestTheAmbiguityIsAskedOfTheReadingItself` — the derivation both readers share, and a reading
  with nothing in it raising nothing.

Three mutation runs over this work, 11 of 12 then 12 of 14 then **14 of 14**. The first run's lone
survivor was the active-session read: nothing could enter `Handle` with a live observation
registry, so handing the look's own session over made the learner never defer with the whole suite
green. That door is now a function a test enters with a real session started in the registry, and
the mutant dies. The other two survivors — the single-flight flag and the phase's derivation —
were closed the same way, by entering the thing rather than trusting it.

## See also

- [[ADR-207-a-question-is-answered-by-a-press-and-confirmed-by-a-crossing]] — the three facts.
- [[ADR-212-a-silence-is-reported-as-a-silence]] — the account that found this in one run.
- [[Experiment-065-perception-and-capture-both-work-and-the-handoff-is-where-the-press-went]]
