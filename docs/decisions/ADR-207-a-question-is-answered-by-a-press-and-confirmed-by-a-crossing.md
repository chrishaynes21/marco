---
type: decision
status: accepted
date: 2026-10-16
deciders: director, owner
affects:
  - execution
  - perception
source_paths:
  - internal/director/observe/demonstratedsurface.go
  - internal/director/surface/pending.go
  - cmd/director/surfacelearn.go
  - cmd/director/observeaction.go
  - cmd/director/observeambient.go
  - cmd/director/runtime.go
---

# ADR-207 — a question is answered by a press and confirmed by a crossing

## Context

[[ADR-206-a-demonstrated-surface-is-one-fact-about-one-target]] built the reasoning and shipped
with two wires hanging: `Pipeline.Surfaces` and `Pipeline.Place` were nil in the daemon, so no
production path grounded anything, and `ConfirmationAttributed` was a string a caller handed in.
The owner named the second as the thing to fix before enabling persistence:

> `ConfirmationAttributed` must not be an assertion that an arbitrary caller can make. It should
> be derived from a validated observation/transition result, or accepted only through a narrowly
> controlled internal boundary. Otherwise the system can promote a demonstration to learned
> knowledge without evidence that the demonstrated action did anything.

And the first as production wiring rather than another investigation:

> Wire `Pipeline.Surfaces` and `Pipeline.Place` using existing composition roots and
> application/place identity mechanisms. Do not invent another screen identity system.

## Decision

**Both halves already ran, continuously, in ambient watching.** What is added is a reader of each
and a question for them to be about. There is no second input recorder, no second correlator and
no second screen identity.

```
  THE PRESS       navsource admits a hand's press, refuses an injected one, refuses one on
                  Marco's own surface and refuses one it cannot place in the window. The session
                  runner banks it STAMPED WITH THE SCREEN STATE IT WAS PERFORMED ON.
                  `ambientObserver.drain` reads that log; one call there offers it to the question.
  THE CROSSING    the ambient observer detects a screen change, asks whether a PERSON or Marco
                  caused it, and attributes the human actions filed against the screen being left.
                  One call there offers the step as a witness.
  THE PLACE       every ambient look already carries `observe.Place`, resolved "through the one
                  resolver". Its `Subject` is the durable subject id, or empty.
```

### A confirmation cannot be asserted

`observe.SurfaceConfirmation` is now a struct with no exported field and one constructor per
level — the discipline `observe.ScreenName` already keeps, for the reason its note gives: passing
something off as a confirmation is possible in the sense that any lie is possible, and it is not
possible by accident.

`TransitionAttributed(TransitionWitness)` is the only way to a confirmation that can aim anything,
and every clause is a way a witness could exist and mean nothing: no application, not attributed
to a person, **not exactly one human action**, no screens either side, the same screen both sides,
no time. A caller that wants this has to have a crossing in hand; a caller that merely wants to
say so cannot.

**The zero value confirms nothing**, so the failure mode of a future edit that forgets the field
is a refusal rather than a press. `confirmationVerified` is named and has **no constructor**: the
vocabulary keeps the owner's distinction visible, and the day something can verify an intended
outcome it gets a constructor and that comment goes.

### Exactly one human action, which is the same rule as the press

A crossing attributed to two actions does not single out the press being asked about. That is the
same refusal the press side makes of a press inside both candidates, and it is worth saying twice:
**where evidence is consistent with two explanations, this system refuses rather than picks.**

### One question, bound to one request and one place

`surface.Pending` names the request, the application, the place, the window, the observation
session and the screen state, and expires after ninety seconds. Every way a press is not an answer
is a separate clause with its own sentence:

| not an answer because | the clause |
|---|---|
| the screen had already moved | the press's own state stamp ≠ the question's |
| the observation rolled over | session ids differ, so state ids are unrelated counters |
| it was a different application | the look's application ≠ the question's |
| it was not a press | intent ≠ `NavPoint` |
| it did not tell the surfaces apart | exactly one option must contain it |
| it was somewhere else entirely | no option contains it |
| two presses disagreed | contested, and then unanswerable rather than answered |
| the question lapsed, was cancelled, or was replaced | `Close`, through one door |

A later request **closes** the outstanding question: the person asked for something else, and the
press they make next belongs to that. One question at a time, replaced rather than queued, because
two open questions would mean a press had two things it might be answering.

What this does NOT re-check is whether the press came from a hand, was inside the watched window,
or landed on Marco's own surface. `navsource` refuses all three before an event is admitted, and a
second copy of a boundary policy is how two copies come to disagree.

### A screen with no durable identity is asked and not learned

A semantic target is `{label, kind, place}` and `Discriminating` refuses one with no place. So a
question on a screen Marco does not recognise durably is still **asked** — the person is not at
fault for Marco not recognising it — and the sentence says the answer cannot be remembered. The
press is kept as `PressWasObserved`, which `Usable` refuses wherever it is read.

### The licence

A person asked Marco to act, Marco asked them where, and they pointed. That is the human semantic
event the durable writes hang off — the shape `promotion` keeps for retrospective Learn, and the
same sentence: what is licensed is this operation, now, on the part of that evidence the person
just pointed at. `learnLicence` in `surfacelearn.go` is the only place one is made on this path,
and `NameActivatedTargets` is the permission that matters, because the durable target is
identified by the word on the control the person's own input landed on.

## Consequences

**All five halves now demonstrated** — the loop measured in
[[Experiment-059-one-demonstration-from-a-person-into-durable-memory]]:

```
  production request wiring     Pipeline.Surfaces and Pipeline.Place are non-nil in the daemon;
                                the place seam reads ambient watching's own resolver
  genuine input attribution     a press through ambientObserver.drain settles the question;
                                one on the words does not; one on another screen or another
                                observation session is not an answer at all
  persistence after attribution a human crossing with exactly one action writes the record;
                                Marco's own crossing, two actions, and no change at all each
                                keep it as an unconfirmed demonstration that cannot aim
  reload and fresh regrounding  the production grounder finds it on a freshly built Stage, on a
                                game menu and on a Windows Settings page
  guarded execution             left click sent at (320,778) through the unchanged resolver,
                                licence, policy, ladder and executor
```

**Still not reached: `verified`.** An attributed crossing says the interaction did something and
not that the intended page opened. Nothing constructs the third level, and the report says so
rather than letting `attributed` drift into meaning it.

**One honest gap.** The question carries its `WindowID` and nothing compares it to a live
foreground reading, because `ambientLook` does not carry one. What stands in its place is the
screen-state stamp, the application check, and `navsource`'s own refusal of any press it cannot
place inside the watched window — a different window is a different screen state. Named here
rather than claimed as a check that is made.

## Enforced by

`internal/director/surface` — `TestAPressThatTellsTheSurfacesApartAnswersTheQuestion`,
`TestTwoPressesOnTwoSurfacesContestEachOther`,
`TestASecondPressOnTheSameSurfaceChangesNothing`, `TestAPressOnAnotherScreenIsNotAnAnswer`,
`TestOnlyAPressAnswersASurfaceQuestion`, `TestALapsedOrClosedQuestionRefuses`,
`TestACrossingThatConfirmsNothing`, `TestACrossingWithNoAnswerLearnsNothing`,
`TestAQuestionWithNoPlaceIsNotLearned`, `TestTheZeroConfirmationConfirmsNothing`,
`TestAConfirmationNeedsItsEvidence`.

`cmd/director`, through the real correlator — `TestAGenuinePressSettlesAnOpenQuestion`,
`TestAPressFromAnotherSessionDoesNotAnswer`,
`TestAPressIsLearnedOnlyAfterAnAttributedCrossing`,
`TestAPressNothingConfirmedIsKeptAndCannotAim`,
`TestTheDaemonOpensAQuestionItCannotSettleItself`,
`TestAQuestionWithNoDurablePlaceCannotBeLearned`, `TestALapsedQuestionIsNotAnswerable`,
`TestTheDaemonGroundsADemonstratedSurface`,
`TestTheProductionSamplePathLearnsADemonstration`.

`cmd/director`, under `-tags corpus` over real recorded frames —
`TestOneDemonstrationGroundsARecordedGameMenu`,
`TestOneDemonstrationGroundsARecordedSettingsPage`.

`internal/director/observe` — `TestNothingCapturedCanReachDurableMemory` and
`TestTheDurableTextExceptionsAreTheOnesThatWereDecided`, both walking the durable record.

## Related

- [[Experiment-059-one-demonstration-from-a-person-into-durable-memory]] — the measurement, and
  the defect its own negative case found.
- [[ADR-206-a-demonstrated-surface-is-one-fact-about-one-target]] — the reasoning this wires.
- [[ADR-183-remembering-recognising-and-being-allowed-to-act]] — the three lifetimes, and why an
  authorisation expires.
- [[ADR-047-a-place-is-remembered-a-meaning-is-answered]] — the place identity this reuses.
- [[ADR-068-the-theater-is-the-durable-semantic-world]] — the durable target, and the label
  licence this spends.
