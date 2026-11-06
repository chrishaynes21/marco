---
type: adr
status: accepted
date: 2026-11-05
decision: "ACCEPTED — \"WHY DIDN'T THAT WORK?\" IS ANSWERED FROM ONE RECORD OF THE LATEST ATTEMPT. The intake (cmd/marco runInvocation), which every entrance reaches, replaces one record in the Marco home at the end of every attempt that is not a control phrase: the routine it became, the outcome, the stage it ended at, the reason in Marco's words, the next step when one is known, and the run reference when one was claimed — including refusals BEFORE a run, which left nothing before. Every sentence in it is scrubbed of the request and of each value supplied with it; the request is recorded only as a word count and an entrance. `marco last [--json]` reads it back with the answer composed, and the widget answers a fixed set of phrasings (internal/lastattempt.Asks) from it, typed or spoken, without asking any model."
supersedes: []
affects:
  - interface
source_paths:
  - internal/lastattempt/lastattempt.go
  - cmd/marco/lastattempt.go
  - cmd/marco/intake.go
  - cmd/marco/panicstop.go
  - plugins/overlay/why.go
  - plugins/overlay/acts.go
  - plugins/overlay/commands.go
---

# Why didn't that work?

The first slice of [[Skills-on-a-miss-2026-11-05]]'s Explain skill.

## What was wrong

- A routine that RAN left a run record, and Studio's History reads it.
- A command refused BEFORE it ran printed its sentence once — into a console, or a status line
  that was gone by the time anybody asked — and left nothing. That covers a value missing, the
  routine saved again since its read-back, a shortcut whose routine changed, another application
  in front, the authority declining, and nothing answering to the words. Those are the attempts
  people ask about, because nothing visible happened.
- The run records do not take them, on purpose: a refusal pressed nothing, so there is no run to
  cite (`performOnePlay`). This is a different question — not "what did this run do" but "what
  happened to what I just asked for".

## Decision

- **One record, the latest** (`internal/lastattempt`, `last-attempt.json` in the Marco home),
  replaced whole, by one writer at one moment: `keepLastAttempt` at the end of `runInvocation`.
  The arms only note what they know as they go — `noteStep` (the routine and its values),
  `noteEnd` (stage, reason, next step; the deepest arm wins), `noteRun` (the reference).
- **The stages:** `matching`, `scope`, `window`, `authority`, `version`, `values`, `reading`,
  `running`, `director`. `take()` says which of its four refusals it made, and what to do:
  press the shortcut again; ask again and check the read-back; say the value as part of the
  request; say it again with the value, naming the routine and the argument.
- **A control phrase does not replace it.** The command somebody stopped is the one they will ask
  about.
- **Nothing anybody said.** The request is recorded as a word count and an entrance. Every
  sentence is scrubbed (`lastattempt.Scrub`, whole words, ignoring case) of the request, the
  phrase Director was handed, every named and positional value on the request, and every value a
  routine step was given. The routine's own name is NOT scrubbed when the request was exactly that
  name: it is a library identity, and the answer has to say which routine.
- **The answer is a lookup.** `Attempt.Says` composes it from the record alone: what it was and
  when, why, what next, the run record. `marco last` prints it; `--json` adds the record.
- **The widget answers the question.** `why` is one of its own words, and `lastattempt.Asks` is
  the one list of phrasings — "why didn't that work", "what happened", "what went wrong", with or
  without punctuation, an apostrophe or "marco" in front. A spoken question and a typed one reach
  the same arm, which runs `marco last --json` and shows the first sentence as the status and all
  of them in the log. It never reaches the intake, and no model is asked. "why is the sky blue" is
  an ordinary request.
- **Not yet a plan.** The shared plan format waits for Chain, the first real multi-step operation;
  Explain needed none.

## Verified (Windows only)

- Through `runInvocation`: a missing value (`values`, the argument named, how to give it, no
  reference); a stale confirmation (`version`, the value given not in the file); a routine refused
  with another application in front (`scope`, the door's sentence, the hotkey entrance); nothing
  answering (`matching`, how to learn it, the words not in the file); a stop afterwards leaving
  the record as it was; a run that performed (its reference, the typed value in neither).
- The scrub over every source of a supplied value, and `marco last --json` and its empty case.
- In the widget: the question spoken, typed bare, and typed with punctuation is answered from the
  record and reaches no intake; a real `marco` built from the tree to a temporary path reads a
  record from the test's own Marco home back to the widget.
- Mutants, all killed by an assertion: the record not kept, the take() refusal not noted, the
  scope refusal not noted, the reference not noted, the scrub dropped, the control skip dropped,
  the routine's name scrubbed, a step's values not collected, the widget's arm removed.

## Not established

- Not asked live from the widget on the desktop, typed or spoken.
- The Director arm's reason is the outcome word and a pointer to Studio's History; the Director's
  own sentence is not carried back to `marco` yet.
- A routine's run that fails records the run's error as the reason, scrubbed; an error that quoted
  part of a value rather than all of it would not be caught by a whole-word scrub.
- The engine's other log lines (`intake: routed`, an unknown command's error) still name the
  phrase on `marco`'s stderr. The widget's echo of that stream redacts it
  ([[ADR-319-the-installer-ships-voice]]); a terminal running `marco do` shows it.

## Enforced by

- `cmd/marco` `TestARefusalBeforeTheRunIsWrittenDown`, `TestAStaleConfirmationIsWrittenDownWithoutItsValue`,
  `TestARoutineRefusedForWhatIsInFrontIsWrittenDown`,
  `TestNothingAnsweringIsWrittenDownAndAStopDoesNotReplaceIt`, `TestARunIsWrittenDownWithItsReference`,
  `TestTheRecordCarriesNothingSupplied`, `TestMarcoLastSaysWhatHappened`.
- `internal/lastattempt` `TestTheQuestionIsAFixedSetOfPhrasings`, `TestScrubTakesOutWhatWasSupplied`,
  `TestTheRecordIsReplacedWhole`, `TestTheAnswerSaysWhatHappenedWhyAndWhatNext`.
- `plugins/overlay` `TestWhyDidntThatWorkIsAnsweredFromTheRecord`, `TestTheWidgetReadsWhatMarcoWroteDown`.
