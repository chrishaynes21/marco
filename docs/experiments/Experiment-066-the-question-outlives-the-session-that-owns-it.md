---
type: experiment
status: complete
date: 2026-10-16
backend:
  - accessibility
  - structure
  - ocr
  - navsource
fixture: "the owner's live desktop -- the disposable Marco Test page in an isolated Chrome profile, read through the augmented configuration of ADR-211, dry-run runtime, with the production Director path as the only establishment path"
result: the-director-establishes-the-question-by-itself-from-a-fresh-reading-and-the-press-is-captured-then-refused-36-times-across-three-later-sessions-because-an-ambient-session-is-twenty-seconds-long-while-a-question-lives-ninety
supersedes: []
affects:
  - perception
source_paths:
  - cmd/director/observeambient.go
  - cmd/director/surfacelearn.go
  - internal/director/surface/pending.go
---

# Experiment 066 — the question outlives the session that owns it

One bounded live demonstration at `03e41ba`, in the disposable Marco Test application, under the
augmented configuration. Authorised as passive observation of one human click; nothing was
injected and no replay was attempted.

## What this run was for

[[ADR-213-a-question-belongs-to-the-session-that-can-answer-it]] moved establishment off the
request and onto a fresh reading, and `03e41ba` removed the harness's own establishment so that a
successful demonstration would be evidence about the Director rather than about the harness. This
was the first live run where the production path was the only path.

## What worked, and it is most of the loop

The Director established the question by itself, from a reading of its own:

```
the request ended session observe_1 and watching resumed in observe_2; the
Director will establish the question itself from a reading of its own
the Director established the question in session observe_2 on state_1, at
place subj_29d46b64b789, about "chrome"
  deferred: the session this would be stamped with (observe_1) is not the one a
    press will arrive in (none), so the question waits for a fresh reading
  established: … — a press answers it at {213 297 371 180}, whose middle is (398, 387)
```

The deferral fired for the reason ADR-213 predicted, the re-derivation found the same durable
place across both sessions (`subj_29d46b64b789`), and the geometry it opened the question with was
its own rather than the request's. The person clicked, the page navigated, and the press was
captured: 11 presses across 55 offers.

## The boundary it failed at

Every one of those presses was refused at the session gate, and not at the one ADR-213 fixed:

```
offers ................... 55 (0 carried no usable reading)
presses seen in the log .. 11
nothing was pending ...... 2
another session .......... 36
  the reading is from session observe_3 and the question was asked in observe_2
  the reading is from session observe_4 and the question was asked in observe_2
  the reading is from session observe_5 and the question was asked in observe_2
READ THIS AS: a press WAS offered and every one of them was refused.
```

Three later sessions, during one 85-second watch. The cause is a constant, not a defect:

| | |
|---|---|
| `ambientSession` | **20s** — `observeambient.go:191` |
| `HowLongAQuestionStaysOpen` | **90s** — `surface/pending.go:49` |

Ambient watching is a supervisor over **bounded** sessions. `watchOnce` starts one, samples it for
`ambientSession`, calls `endLook`, returns, and `run` starts another. So a session is twenty
seconds long by construction, and session ids roll on regardless of what anybody is being asked.

A question therefore has a 90-second life and a ~20-second window in which it can be answered —
less, because establishment consumes part of that window before the person is told to click. The
press is admitted, placed, offered and correctly refused. Nothing is broken; two constants
disagree about how long a question lives.

## Why this was not visible before

It is not reachable from the unit tests, which hold one session still because the thing under test
is the gate rather than the clock. It was not reachable from the previous live run either: that one
failed at an **earlier** boundary — a question stamped with a cancelled session — so the press
never got far enough to be refused by a *later* one. Each fix exposes the next boundary, and this
is the fourth.

## What was decided afterwards

The anchor the question needs across a rollover already exists and was observed to be stable: the
durable Place was `subj_29d46b64b789` in both sessions. Screen **state** ids are the session-local
part. Whether a question should survive a rollover by re-deriving against a stable Place, or
whether a session should instead stay open while a question is pending, is a design decision about
what an observation session is — recorded here, not taken.

The first was chosen: a learning interaction survives the rollover by being derived again from a
fresh reading, carrying only the instruction, the application, the durable Place and the person's
original absolute deadline. Both constants stayed as they were. See
[[ADR-214-a-learning-interaction-outlives-the-session-watching-it]].

## Enforced by

`cmd/director` `TestATeachingInteractionSurvivesASessionRollover` and the four tests beside it,
listed in [[ADR-214-a-learning-interaction-outlives-the-session-watching-it]].
