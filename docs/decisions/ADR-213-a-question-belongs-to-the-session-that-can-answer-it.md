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

The daemon path holds an intent rather than opening an unanswerable question. Nothing establishes
one for it automatically: establishment needs a fresh Stage, and taking one inside the ambient
observer's own glance would put a full observation on the thread that has to stay responsive. For
now that path records `deferred` and says why, which is strictly better than a question no press
could ever reach, and it is recorded here as the remaining gap rather than described as finished.

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

A mutation run over the change killed 11 of 12. The survivor is recorded at the call site rather
than papered over: replacing the active-session read in `runtime.go` with the look's own session
makes the learner never defer and every test still passes, because no test can enter `Handle` with
a live observation registry. Every decision this change makes is held; what survives is that one
read.

## See also

- [[ADR-207-a-question-is-answered-by-a-press-and-confirmed-by-a-crossing]] — the three facts.
- [[ADR-212-a-silence-is-reported-as-a-silence]] — the account that found this in one run.
- [[Experiment-065-perception-and-capture-both-work-and-the-handoff-is-where-the-press-went]]
