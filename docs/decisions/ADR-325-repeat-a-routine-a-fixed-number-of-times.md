---
type: adr
status: accepted
date: 2026-11-06
decision: "ACCEPTED — A SAVED ROUTINE CAN BE REPEATED A FIXED NUMBER OF TIMES, AND ONLY AFTER THE COUNT HAS BEEN READ BACK. \"repeat open social five times\" (also without \"repeat\", and with twice/once) is read on the widget's command line as the routine and the count, shown as \"Open social ×5\", and Enter runs `marco do --play=… --expect=… --times=5`. Each run is the ordinary one with every check; the first that does not perform ends the repeat with its outcome, and a Stop ends it, including between runs. `[ran]` says \"open social ×5\", or \"open social ×2 of 5\" when it ended early. Words in that form reaching the intake are proposed and never run; a description is named by Director's resolver from the routine's words alone, and the count travels beside its answer. At most 100. A routine Marco runs by name only; no \"repeat that\", no \"repeat until\"."
supersedes: []
affects:
  - interface
source_paths:
  - cmd/marco/repeat.go
  - cmd/marco/line.go
  - cmd/marco/intake.go
  - cmd/marco/assistant.go
  - internal/invoke/invoke.go
  - plugins/overlay/line.go
  - plugins/overlay/resolve.go
  - plugins/overlay/director.go
---

# Repeat a routine a fixed number of times

Builds on [[ADR-301-the-command-line-reads-back-what-it-will-run]], [[ADR-322-resolve-before-reading-the-screen]] and [[ADR-324-the-history-shows-what-ran]].

## Why

With chains working, the owner asked for repeat next: something done mid-game that took a clumsy
chain ("open social then open social then …"). The first slice they set: a named routine and a
fixed count, confirmed before it runs, one run at a time with the usual checks, stopping on a
failure or on Stop and saying how many completed. The explicit form read locally; the model only
where the routine is described rather than named.

## Decision

- **The form.** `parseRepeat`: an optional leading `repeat`, the routine's words, then a count and
  `times` (digits, or the words a recogniser writes, up to "a hundred"), or `twice` / `once` /
  `one time`. A routine's exact name wins over it, and without the word `repeat` so does any
  reading of the whole line, so "notepad type I said it 3 times" still types what was said.
- **The read-back.** `marco line` reads the routine's words through the ordinary reading
  (`readOnce`) and adds `times`, with "×5" on the read-back. So the routine's own checks apply
  unchanged: one bare value only for a routine taking one, a missing value asked for, a proposal's
  identity and version.
- **Confirmation.** The widget treats any reading with a count as one it reviews, even for a
  routine that takes no values: the same rule as a value, the read-back must have been on screen
  before Enter runs it, and a spoken repeat always waits for that Enter.
- **Running.** Only `--times` on an identity runs a repeat (`performRepeated`). Each run is
  `performOnePlay`: scope, authority, the confirmed version and the values, every time. The first
  run that does not perform ends it with that outcome. The stop generation is taken when the repeat
  begins and checked before each run, because a run watches for Stop only while it runs. A count
  on anything that is not a routine is refused.
- **Saying what happened.** `[repeat 2/5] Open social` before each run (the widget's status:
  "running 2 of 5: Open social"); a final `[ran] ` of "open social ×5", or "open social ×2 of 5"
  after "stopped after 2 of 5". The history keeps the last `[ran] `.
- **Words reaching the intake** in the form are proposed on `[proposed] ` with `times` beside the
  answer, never run: named locally when the words name a routine, otherwise Director's resolver is
  asked about the routine's words alone ("open the social menu"), so the count is never the model's
  to read. The widget opens the line on the explicit form ("repeat open social 5 times") for the
  same review, and a repeat it cannot read back is not offered as a yes, which would run once.
- **At most 100**, refused rather than run once.

## Not established

- Not tried live.
- Nothing waits between runs beyond what the routine itself waits; a routine whose last step needs
  a moment to settle before the next first step may need one added to the routine.
- A demonstration performed by sight (`KindDemonstration`) is not repeatable; only a routine in the
  library is.

## Enforced by

- `cmd/marco` `TestTheRepeatFormIsRead`, `TestARepeatIsReadBackWithItsCount`, `TestARepeatRunsTheRoutineThatManyTimes`, `TestEachRunOfARepeatIsCheckedAgain`, `TestARepeatStopsAtTheFirstRunThatDoesNotPerform`, `TestAStopBetweenRunsEndsTheRepeat`, `TestARepeatInWordsIsProposedNotRun`, `TestARepeatIsOnlyOfARoutine`, `TestTheTimesFlagIsACount`.
- `plugins/overlay` `TestARepeatRunsOnlyFromItsReadBackWithItsCount`, `TestARepeatIsDrawnWithItsCount`, `TestAProposedRepeatIsReviewedWithItsCount`, `TestARepeatsProgressIsTheStatus`.
