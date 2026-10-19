---
type: adr
status: accepted
date: 2026-10-19
decision: "ACCEPTED — the one door gains a SIXTH arm: a name that exactly matches something the person demonstrated, after the play arm and before Director, resolved through the same demo store the runner reads. It is performed by spawning the ONE supervised runner — `director perform-workflow --name --application --live --hotkeys --reading` — whose transcript is passed through to the stream every front end already reads, and whose one-word outcome is taken from the run REFERENCE and translated into the seven words every surface already knows. The reference's shape and vocabulary move to `internal/director/runref` so the two binaries cannot drift. The door only claims a routine the runner can actually perform (`Artifact.PerformableBySight`), so a promise is never made that the runner then declines. And a supervised run now watches the stop generation, so the word 'stop' — typed, spoken, or the leader key — reaches it like it reaches every other running thing."
supersedes: []
affects:
  - execution
  - safety
  - learning
source_paths:
  - internal/invoke/invoke.go
  - cmd/marco/demonstrated.go
  - cmd/marco/intake.go
  - internal/director/runref/runref.go
  - internal/director/workflowstore/workflowstore.go
  - cmd/director/performworkflow.go
  - plugins/overlay/director.go
---

# ADR-241 — a name you demonstrated is a name you can use

## The gap

The architect's framing, after the fourth round of lifecycle review closed:

> These commits repair the CLI runner. They do not connect the widget's bare `ark graphics live`
> trigger to that runner.

Both halves of the product were validated and neither could reach the other.

A person shows Marco a game menu with their own hands: `director demonstrate-visual` watches them
click, saves the screens it settled on, the control they pressed, and what the press did
([[Experiment-092-the-first-human-taught-pixel-macro]]). And `director perform-workflow --name`
performs exactly that, supervised: grounded on a fresh look, authorised one action at a time,
pressed at the moment that was authorised, verified against what the demonstration saw
([[ADR-239-running-what-you-demonstrated-by-the-name-you-gave-it]],
[[ADR-240-a-stop-is-only-a-stop-at-the-last-moment-it-could-matter]]).

Typing that name into the overlay reached `marco do`, which asked the one door what the words
meant. The door knew about PLAYS, found none, and handed the words to Director — which read them
against the screen and reported that nothing matching them was present in the observed window. It
was present. It had been pointed at. The words went to the half that guesses rather than the half
that had been told.

So the only path to the thing a person had taught was a developer's command line, and the product
could be demonstrated but not used.

## The decision

### A sixth arm, and where it sits

`invoke.Decide(plays, sights, Request)` takes a second resolver. A demonstration is not a play —
it carries no verbs, it lives in another store, and only the Director's supervised runner can
perform one — so the two cannot share a store or a performer. They can and must share the DOOR:
the alternative was a front end that checked one store, then another, then fell through to
Director, which is three places deciding what a phrase means and is the defect `internal/invoke`
exists to have ended.

**After the play arm and before Director.** The order is the compatibility guarantee: every name
that runs something today runs the same thing tomorrow, and a demonstration saved under the name
of an existing play does not take it over. The door that changes is the one past it.

**And it only claims what can actually happen.** `Artifact.PerformableBySight` — one definition on
the artifact, used by the listing that tells a person how to run a routine and by the door that
decides what their words mean — is three conditions, each a different refusal: it is not a verb
routine (those belong to the ordinary path), it carries visual control evidence (otherwise there
is nothing to ground), and it is SCOPED (an unscoped routine is valid for no application and the
runner would refuse it). A door that promised one of those would have Marco accepting a request
and then arguing with itself.

### Performed by the one runner, in a child process

`marco` cannot do this work and must not learn how: grounding a control in pixels needs an
observation loop, a stabiliser, a consent desk and a run record, all of which exist, once, in the
Director. The same reasoning as `perform.go`, where a learned play is delegated rather than run.

It is a CHILD PROCESS rather than a service request because the supervised lifecycle is a command,
not a query: it owns a countdown somebody prepares during, a consent desk that reads real function
keys off the desktop, a run record reserved before anything is printed, and a transcript that is
the product's own account of what happened. `marco` already locates and launches `director.exe` —
it starts the service that way — so this is not a new dependency. Putting the lifecycle behind a
service request would mean a second home for it, which this phase was told not to build.

Four flags, each load-bearing, and `TestADemonstrationIsPerformedWithTheFlagsItNeeds` holds each
one:

- `--name` and `--application`, the routine and the window to watch. Without the application the
  runner reads the FOREGROUND — which, at the moment somebody types a name into the overlay, is
  the overlay. It is a selector and not a permission; the artifact's scope is still checked
  against the window actually acquired ([[ADR-236-a-workflow-belongs-to-the-application-it-was-shown-in]]).
- `--live`, because a person asking Marco to do the thing they demonstrated is asking for it to
  happen. Every action is still authorised individually, by a key this process cannot send.
- `--hotkeys`, because the questions are answered by function key. Without it the runner asks on
  its own standard input, and a child spawned by a front end has no terminal to answer on.
- `--reading`, because the outcome expectation IS words and two visually similar menus are told
  apart by what they say ([[ADR-194-a-screen-is-also-what-it-says]]).

**Nothing authorises more than one action.** `--explore` is a measurement flag and must never be
what a typed name gets.

### One reference, read by two programs

The outcome a person is told comes from the line the runner prints about itself, so that line is
now a contract between two binaries — and there is no compiler between them. Its shape and its
seven words move to `internal/director/runref`: the runner prints through it, the front end parses
through it, and `cmd/director`'s own names (`RunVerified` and friends) are aliases of it so the
file that writes the record reads as it always did.

The parser earns its own package. A record's path is a path on somebody's machine and may well
contain a space — `C:\Users\A Person\…` is ordinary — and Windows renames a taken name to
`r (1).json`, so a parenthesised word at the end of the line is the outcome only if it IS one of
the seven.

### The seven words become the seven everybody already reads

`marcoWord` maps the run's outcome onto `internal/outcome`, and two arms are worth reading twice.
CONTRADICTED is `failed` rather than `unverified`, because the evidence did not fail to decide — it
decided against, and that is the only clear bad news the verifier produces. `started` is `failed`,
because a run that reserved a reference and wrote no outcome is a defect in the runner, and a
front end smoothing it over would hide the one thing that makes such a path findable. A word this
build does not know is never success.

### And a stop said anywhere reaches the run

Ctrl-C is a terminal's stop and F9 is a desktop's. Neither is the product's: a person types or
says "stop", which reaches `marco`'s one stop — the local generation every running Play watches,
and the Director service's own active command. A supervised run was neither of those, so the
moment the product could start one from the widget, it could start one it could not stop by the
only means the widget offers.

`stopsignal.Watch` on the run's context is the whole fix: the same mechanism, the same home, no
second broadcast, and the classifier already reads a cancelled context as cancellation rather than
failure. It is also what makes the overlay's leader key true for this path — that key raises the
generation precisely because it is "the only half that reaches a child the overlay has no handle
on", and a supervised run is exactly such a grandchild.

### What the widget shows

Everything the runner says already reaches the HUD's tail, because `marco` passes the transcript
through to the stream the overlay reads, and both streams are merged into it: the runner says some
things on stderr — a provider that is not installed, a record that could not be written, a flag it
does not know — and every one of them is part of the account of what happened.

What the always-visible line gains is the two states a person must ACT on, in the runner's own
words: the preparation countdown (bring the application in front) and a question that only a real
function key can answer. Those are the states in which nothing is happening and the reason is on a
transcript nobody is reading — a person who typed the name of something they demonstrated is, by
then, looking at the game. The keys come from `navsource.ConsentKeys`, the same map the runner
prints from, so one binding cannot be advertised while another is listened for.

## What this does not do

- **It is not a live confirmation.** Everything here is deterministic: the real runner is built
  from the tree and driven by the real door, and the run it performs is stopped before anything is
  authorised. A person pressing F7 on a real game through the widget is the owner's confirmation
  step and has not happened yet. A live run confirms; it is never the debug loop.
- **The control centre's Stop still does not reach this.** `cmd/marco/watchui.go handleStop`
  reaches service cancellation without raising the generation, so it does not reach a standalone
  play and does not reach a supervised run either. Named in ADR-240 and still open.
- **It does not change the demonstration half.** Nothing here records, scopes or edits a routine.
- **Two actions are still not a workflow.** One authorised action at a time, as before; the
  two-action acceptance trial is the next piece of work and is not this one.

## Enforced by

The door:

- `internal/invoke/invoke_test.go` — `TestADemonstrationIsPerformedBySight` (the arm, the saved
  name rather than the typed words, the application, and the trace line);
  `TestAPlayStillWinsOverADemonstrationOfTheSameName` (the order, and that the demonstration store
  is never even consulted); `TestADemonstrationIsNotConsultedAboveItsArm` (a control phrase, an
  answer to a question, an explicit identity and silence each decided without asking);
  `TestAnEntranceWithNoSightOfDemonstrationsFallsThroughToDirector`.

The front end:

- `cmd/marco/demonstrated_test.go` — `TestADemonstratedRoutineIsReachedByItsName` and
  `TestTheSavedSpellingIsWhatThePerformerIsGiven` drive the production `runInvocation`;
  `TestADemonstrationIsPerformedWithTheFlagsItNeeds` holds every flag and that none authorises a
  session; `TestAVerbRoutineIsNotClaimedBySight` and
  `TestADemonstrationTheRunnerWouldRefuseIsNotClaimed` hold the three conditions;
  `TestEveryWayARunCanEndBecomesAWordTheSurfacesKnow` sweeps the whole vocabulary rather than a
  list written beside it; `TestTheRunsOwnOutcomeIsWhatTheInvocationReports`,
  `TestARunThatPrintedNoOutcomeIsNotPerformed`, `TestARunnerThatCannotBeStartedIsUnavailable`,
  `TestTheTranscriptIsPassedThroughAndTheReferenceReadOutOfIt`.

Both binaries, from the same revision:

- `cmd/marco/demonstratedlive_test.go` — `TestTypingADemonstratedNameReachesTheRealRunner` builds
  `director` from the working tree and drives the production path into it: the reference comes
  back, the run's own word becomes Marco's, a stop raised from a third place ends it, and the
  record — read as JSON by a program that did not write it — says cancelled with zero actions
  sent. `TestWhatTheRunnerSaysOnStandardErrorReachesThePerson` proves the complaint about a flag
  the runner does not understand reaches the person, which is what gives that test's flag check
  its teeth: `perform-workflow` parses with the error ignored, so a renamed flag is not a refusal
  but a run carrying on with the flag unset.
- `cmd/marco/packaging_test.go` — `TestThePackagedLayoutIsWhatMarcoLooksFor` already holds that
  `director.exe` ships beside `marco.exe`, which is what makes the lookup this path depends on
  true of an installed Marco.

The reference:

- `internal/director/runref/runref_test.go` — `TestAReferenceComesBackAsWhatWasPrinted` (both
  shapes, a path with a space in it, and a path the filesystem renamed to `r (1).json`);
  `TestAReferenceWithNoRecordCarriesNoPath`; `TestOrdinaryTranscriptLinesAreNotReferences`;
  `TestEveryWordARunCanEndInIsKnown`.

The stop, and the widget:

- `cmd/director/runlifecycle_test.go` — `TestAStopSaidAnywhereReachesASupervisedRun`: the word,
  raised the way `marco do stop` raises it, ends a run waiting at its start prompt, and the record
  says cancelled with nothing sent.
- `plugins/overlay/intake_test.go` — `TestASupervisedRunSaysWhatItIsWaitingFor` (both of the
  runner's questions, built from the same key map it prints them from; the countdown; a question
  that closed unanswered stops asking; and the reference is NOT status, because it is not
  something to act on) and `TestASupervisedRunsTranscriptReachesTheLog`, through the real child
  streamer.

## Links

- [[Invocation]]
- [[ADR-239-running-what-you-demonstrated-by-the-name-you-gave-it]]
- [[ADR-240-a-stop-is-only-a-stop-at-the-last-moment-it-could-matter]]
- [[ADR-029-resolution-is-not-permission]]
- [[Experiment-092-the-first-human-taught-pixel-macro]]
