---
type: experiment
status: complete
date: 2026-10-16
backend:
  - accessibility
  - structure
  - ocr
  - navsource
fixture: "the owner's live desktop — a disposable local HTML page in an isolated Chrome profile, read through the augmented configuration of ADR-211, dry-run runtime, plus a sixty-line standalone hook probe written for the session and discarded"
result: perception-asks-a-well-formed-question-about-an-unnamed-container-and-input-capture-admits-and-places-a-real-press-so-neither-is-the-blocker-the-press-disappears-in-the-handoff-to-the-pending-question-which-had-four-silent-returns-and-now-has-none
supersedes: []
affects:
  - perception
  - execution
---

# Experiment-065 — perception and capture both work, and the handoff is where the press went

## What was being tested

Whether the loop of [[ADR-207-a-question-is-answered-by-a-press-and-confirmed-by-a-crossing]]
closes on a real desktop: a human demonstration, a learned surface, a fresh reacquisition, and one
authorised replay. It does not, and this records exactly how far it gets.

## The instrument

A disposable page in an isolated Chrome profile, approved by the owner on the condition that it
introduce no application-specific recognition and no special-cased resolver. The first version
used a well-formed link and Marco **resolved it outright**, asking nothing — a link takes its
accessible name from its own text, so identity and surface are one object. The owner called that
the desirable case, and it is: the feature does not exist for it. One approved change replaced it
with an unnamed container holding a separate text label, which is the shape Settings' home cards
have and an ordinary pattern in real interfaces.

## What held, live

```
identity e31 "Open Archive" role=text sources=[accessibility ocr]
             rejected="not an interactive control"
  candidate e31 {90 324 123 27}    the words          DEFENSIBLE
  candidate e30 {64 297 520 180}   the card           DEFENSIBLE
  11 further surfaces refused by the band rule
durable place subj_29d46b64b789, stable across five separate readings
request: needs_clarification, with the question attached, on the production path
where a press answers it: {213 297 371 180}, middle (398, 387)
```

The question is the one [[ADR-210-a-question-nobody-could-answer-is-not-asked]] permits: a press
exists that tells the two surfaces apart, and the gate found it. The ninety-second bound on a
question's life was also observed live, refusing a press that arrived at ninety-four seconds.

## What failed, and what that cost

A witnessed press — the page navigated to its destination, which Marco has no part in — produced
`observed press false` and **no lesson at all**. Three of the learner's outcomes are reported and
all three were absent, which means no press was ever offered to the question.

The next three hours went into the input hook, on the strength of a precondition reading
`input observation hook installed: yes` beside zero received events. That line could not be
anything else: see [[ADR-212-a-silence-is-reported-as-a-silence]]. A standalone probe then
measured the truth of it.

```
standalone probe, hook installed by hand   8478 mouse callbacks in 30s
                                           handle non-zero, no error
navsource, same desktop, witnessed press   13 admitted, 3 placed in the watched window
                                           precondition: input observation hook ACTIVE
```

So capture works. It had been working the whole time, and an earlier reading of zero came from a
measurement whose click left no witness — a flaw in how the run was operated, recorded here
because it cost more than the defect did.

## Where the press actually goes

`offerPresses` drops a press in silence four ways, and the quiet one is a press made on a screen
state other than the question's: `Pending.Press` returns a zero `Answer` with a stated reason, and
a zero outcome matches none of the learner's cases. That is the leading candidate for the failed
demonstration and it is now instrumented rather than inferred — the account prints which gate
refused a press and what it said.

## And the gate it actually met, measured live

With the account of [[ADR-212-a-silence-is-reported-as-a-silence]] in place, one bounded run
answered it on the first attempt. The press was captured and every one of them was refused by
the SESSION gate.

```
offers ................... 56
presses seen in the log .. 14
another session .......... 54      <- every press
nothing was pending ...... 2
settled / undiscriminating / elsewhere / other ... 0 0 0 0

  "the reading is from session observe_2 and the question was asked in observe_1,
   so no press in it can be matched to the question's screen"
```

The screen-state gate was never reached, so the temporal reading — a press attributed to the
destination screen after navigation rather than the source screen it was made on — is not what
happened here. The cause is earlier and deterministic.

A request makes ambient watching **stand aside**: its session is cancelled so that nothing the
Director does is folded into evidence about what the person did, and the supervisor starts a
fresh one afterwards. That is deliberate and documented at the call site. But the question is
stamped from `standing()` read immediately after the request returns, which reports the session
of the last ambient look — the one just cancelled. The question is therefore born carrying a
session that no longer exists, and every press afterwards arrives in the next one.

This is not a flaw in the rule. A press must come from the session its question was asked in,
because screen state ids are session-local counters and `state_2` in two sessions are unrelated
screens. What is wrong is that the demonstration path asks the question in a session it has
already ended. Renaming the session on the pending question would not fix it either: the
question's `State` is session-local too, so a rebind has to re-establish that the same screen is
in front rather than relabel a stamp. The durable Place is the one anchor that survives a
session, and it is already established here.

## Status after this

```
perception: identify the screen and ask a fit question       verified live
resolving outright when accessibility already names it       verified live
the bounded life of a question, enforced                     verified live
durable place across runs                                    verified live
input capture: admitted and placed in the watched window      verified live
the handoff from an admitted press to the question           MEASURED: every press refused at the session gate
learning from a human click                                  not reached
persistence, reacquisition, authorised replay                 not reached
```

## See also

- [[Experiment-064-the-augmented-configuration-works-and-settings-is-the-wrong-instrument]]
- [[ADR-209-the-assistant-operates-the-harness]]
- [[ADR-211-production-observation-with-the-pixels-added]]
