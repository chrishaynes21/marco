---
type: decision
status: accepted
date: 2026-10-16
deciders: director, owner
affects:
  - perception
  - execution
source_paths:
  - internal/platform/navsource/navsource.go
  - internal/platform/navsource/navsource_windows.go
  - cmd/director/surfacehandoff.go
  - cmd/director/surfacelearn.go
  - cmd/director/surfacelive.go
---

# ADR-212 — a silence is reported as a silence

## The run that forced this

A live demonstration on the owner's desktop ended with the three facts of
[[ADR-207-a-question-is-answered-by-a-press-and-confirmed-by-a-crossing]] printed and nothing
else: observed press false, attributed transition false, intended outcome not implemented. The
press had been made. The page it was made on navigated to its destination, which is a witness
Marco has no part in, and a later measurement showed the same desktop delivering presses into
`navsource` and placing them inside the watched window.

So an admitted press disappeared between the producer and the question, and the report could not
say where. Worse, the preconditions printed **input observation hook installed: yes** — and on
Windows that line could not be anything else. Three hours of live debugging went into the input
hook, which was working the whole time.

Two defects, both about reporting rather than about behaviour.

## One: a precondition that could not fail

`navsource`'s Windows backend returned the empty string from `unavailable()` unconditionally,
`start` returned `nil` unconditionally, and both `SetWindowsHookEx` results were discarded except
for the unhook. Nothing anywhere checked whether installation had succeeded, so `rt.navUnavailable`
was always empty and the check always passed.

Now the pump records what the OS said about each hook, publishes it before anything waits on it,
and `start` waits for that verdict rather than returning a hope. The two hooks are reported
**separately**, because one can fail while the other installs: a single flag would describe a
producer that silently sees no presses as a working one, which is the reading the live run got.
Both failing is fatal and the source is unavailable outright; one failing is not, because keyboard
navigation is the older and more load-bearing evidence — that asymmetry was already the documented
intent and is unchanged. A hook that did install is still unhooked from its own thread when the
other did not.

The Director's precondition now asks the producer what became of each hook instead of asking
whether it started.

## Two: four silent returns

`offerPresses` had four ways to drop a press without a word: a reading from another observation
session, a reading of another application, a question that had lapsed, and — the quiet one — an
outcome its switch had no case for. `Pending.Press` returns a **zero** `Answer` with a stated
reason when the press was made on a different screen state than the question is about, and a zero
`Outcome` matches neither `OutcomeUndiscriminating` nor `OutcomeElsewhere`.

Every gate is now counted and every refusal says what it refused, printed above the three facts
because it is what makes them readable. Counts are per offer rather than per distinct press: the
whole admitted log is handed over on every reading, so one press is offered again on each
subsequent one. That is reported rather than deduplicated, because deduplicating means a cursor,
and a cursor would change which presses the learner considers. A measurement must not change the
thing it measures.

## What this deliberately does not do

No gate moved. Nothing extends a question's life, relaxes an identity, adds a fallback match, or
changes what `Lessons` means — the unaccounted case is recorded where measurements live and is
still not an outcome the learner acts on. Perception, the resolver, the policy ladder and the
execution safeguards are untouched. The account separates "nobody pressed anything" from "a press
arrived and was refused for this reason", because those two send a reader to different places, and
reporting both as zeros is what sent this one to the wrong place.

## Enforced by

- `TestAProducerThatCouldInstallNothingIsUnavailable` — a backend that could install no hook is
  reported unavailable, with what the OS said.
- `TestOneMissingHookIsReportedAgainstItsOwnDevice` — one hook failing is reported against its own
  device and the other keeps working.
- `TestEveryPressOfferedToAQuestionIsAccountedFor` — settled, undiscriminating, elsewhere and the
  previously silent different-screen refusal, each recorded, through the production drain.
- `TestARefusedSessionSaysSo` — the refusal names the session it refused.
- `TestAPressWithNoQuestionOpenIsStillCounted` — a press with nothing pending is a fact, not an
  absence.
- `TestTheAccountSaysWhichSilenceThisIs` — the account distinguishes no readings, no presses, and
  a press refused.

- `TestWhatTwoHookOutcomesMeanForTheProducer` — one hook missing keeps the producer, neither
  refuses it, and the refusal names both.
- `TestTheHookCheckAsksWhatTheProducerSays` — the precondition fails for an unavailable producer
  and for a missing device, and passes only when every hook returned a handle.
- `TestTheDemonstrationReportCarriesTheHandoff` — the account is printed, and printed before the
  three facts it explains.

A mutation run over every new call site killed 14 of 16. The two survivors are the same
irreducible gap and are recorded rather than papered over: no test can make `SetWindowsHookEx`
fail, so `devices = rt.navSource.Stats().Devices` can be replaced with nil, and
`return installVerdict(...)` with `return nil`, without anything failing. Both decisions are held
by `hooksVerdict` and `installVerdict`, which is why they were extracted; what survives is the one
line that reads a live producer in each case.

## See also

- [[ADR-207-a-question-is-answered-by-a-press-and-confirmed-by-a-crossing]] — the three facts this
  account is printed above.
- [[ADR-209-the-assistant-operates-the-harness]] — the checkpoints the live runs reported against.
- [[ADR-211-production-observation-with-the-pixels-added]] — the configuration every live run used.
- [[Experiment-065-perception-and-capture-both-work-and-the-handoff-is-where-the-press-went]]
