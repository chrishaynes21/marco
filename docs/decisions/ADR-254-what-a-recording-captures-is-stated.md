---
type: adr
status: accepted
date: 2026-10-24
decision: "ACCEPTED — WHAT A RECORDING CAPTURES IS STATED BY THE THING THAT STARTS IT, AND NEVER INFERRED. `$MARCO_CV` and `$MARCO_ANCHORS` are read ONCE, into package variables, when a process starts: right for a command you set a switch and run, wrong for a window that stays open all afternoon with whatever shell launched it. With anchors inherited on, a click whose crop happened to be distinctive carried a template; a press found by matching a picture cannot be stopped at, so `codegen.Unpropagatable` refused the WHOLE guarded recording at Save. The same demonstration therefore saved or was refused depending on a variable nobody in the window could see and the pixels that happened to be under the pointer. So `recorder.Mode` is a statement about ONE recording — `Inherited`, which is every caller there was, or `Coordinates`, which captures the coordinate and the clock and nothing else — handed to the recorder BEFORE `Start`, because the hooks read it and the first press can already have been seen. A stated mode BEATS the environment at every point that would capture, crop, colour-sample or listen for the anchor key, and those points ask one function, `anchoring()`. Studio states `Coordinates`, once, to the thing that acts on it, and the sentence a person reads is a COPY of that — not the constant a second time, which the mutation gate caught as a window still promising coordinates over a recording that had gone back to inheriting them. A stated coordinate mode also keeps the recorded pauses, which `cvOff()` alone did not: a Studio launched with `$MARCO_CV=on` capped every gap to 50ms while capturing no anchor to wait on. The word travels on the DRAFT (review happens before the save), into the saved source as a comment line `// Recorded as: <mode>.`, and from there into the run record via `orchestrator.ModeOf` off the same single read as the digest and the contract. ABSENCE SURVIVES ALL OF IT: a caller that states nothing writes no line, a file with no line reads back as `Unstated`, a word this build does not know reads back as `Unstated` rather than being echoed into a record as something verified, and the page renders a blank as a blank rather than as `undefined` or as the likeliest answer. Every other caller — the overlay, `marco learn`, `director demonstrate-visual`, the CLI — states nothing and is unchanged, because turning the switch on and demonstrating something is what the switch is FOR there. AND THE STATEMENT SURVIVES A REPAIR, which it did not: `Resave` rebuilt a saved routine stating nothing, so a coordinate routine lost its line AND — because the stated mode is also what lifts the 50ms wait cap — lost its recorded pauses to a window that happened to inherit `$MARCO_CV=on`, which is a menu pressed before it has drawn, out of a repair. The mode now travels with the recording: read from the source of the file being replaced, and simplified under the mode given rather than under whatever `Deps` is holding it. Nothing is versioned and nothing is migrated."
supersedes: []
affects:
  - execution
  - interface
source_paths:
  - internal/recorder/recorder.go
  - internal/recorder/recorder_windows.go
  - internal/orchestrator/draft.go
  - internal/orchestrator/orchestrator.go
  - internal/codegen/codegen.go
  - cmd/marco/routinerecord.go
  - plugins/studio/recording.go
  - plugins/studio/routines.go
---

# What a recording captures is stated

## The defect, which is not a bug in anything

Nothing here was coded wrong. `captureAnchors` is read once at process start, from `$MARCO_CV` or
`$MARCO_ANCHORS`, and that is the correct shape for the thing it was written for. You set the
switch, you run `marco learn`, the setting is the whole of what you asked for, and the process
ends. The overlay is the same. So is `director demonstrate-visual`.

Studio is not. It is open all afternoon, holding the environment of whichever shell started it,
and the person in front of it has no way to see what that was. And the setting is not cosmetic:

| | with anchors inherited OFF | with anchors inherited ON |
|---|---|---|
| a click on a flat panel | plain coordinate | plain coordinate |
| a click on a distinctive button | plain coordinate | **carries a template** |
| the recording at Save | saves | **refused outright** |

The last row is the one that matters. A press found by matching a picture cannot be *stopped at* —
its emitted form is a ladder of locators with a click in each arm, not one `do` sentence a chain
can propagate a refusal through — so `codegen.Unpropagatable` refuses the whole guarded routine
rather than write one that reads as checked and is not. That refusal is correct. What is wrong is
that whether it fires depends on a variable nobody in the window can see **and on the pixels that
happened to be under the pointer**. Record the same menu twice and you can get two different
answers with nothing on screen to explain either.

There was a second half, quieter and worse. `learnOptions` caps every recorded gap to 50ms when CV
is on, because the anchor poll was supposed to do the real waiting. A Studio launched with
`$MARCO_CV=on` against a flat menu therefore captured **no anchor to wait on and took the pauses
out as well** — a routine with its timings removed and nothing put in their place, which presses a
menu before it has drawn, and which looks perfectly correct in review.

## The decision

A recording's mode is **stated by the thing that starts it**, and the statement wins.

```
recorder.Mode     Inherited       leave it to $MARCO_CV / $MARCO_ANCHORS
                  Coordinates     the coordinate and the clock, and nothing else
```

`Inherited` is the zero value, so every `Deps` and every recorder built without a thought about
this behaves exactly as it did. `Coordinates` is stated through `orchestrator.Deps.Mode`, handed to
the recorder through the optional `recorder.Stated` interface, and published to the hooks by
`Start` — **before** the hooks go on, because a mode stated afterwards would be stated after the
first press could already have been captured.

Three things fall out of that ordering, and all three were found by writing it down:

- `Start` also **clears any unconsumed arm**. The anchor key's arm is a one-shot consumed by the
  next click, so one left over outlives the recording it was tapped in — which would put a picture
  in the first click of the next recording, and in Studio that is a recording that asked for none.
- The gate is **one function**, `anchoring()`, asked at every point that would capture, crop,
  colour-sample or listen for the anchor key. A fourth such point cannot be added without
  answering it.
- The arm is gated on `plainCoordinates` and **not** on `anchoring()`. That distinction is a defect
  this ADR's own mutation gate caught: `anchoring()` is the conjunction, so using it there made an
  armed press do nothing whenever the global switch was off — taking the tap-then-click anchor away
  from *every* caller instead of from *this* recording. See the mutation table.

## Why the word is stated and never inferred

Because it cannot be inferred. A routine with no pictures in it may have been recorded in
coordinate mode on purpose, or recorded with the anchor stack on against a flat menu that offered
nothing distinctive to anchor to. Those are two different facts about somebody's afternoon and the
file looks identical.

So the mode is a claim, or it is absent. And absence survives the whole path, which is the lesson
of [[ADR-253-the-claim-is-shared-and-the-record-is-not]] arriving one layer up:

| where | stated | not stated |
|---|---|---|
| the draft | `Coordinates` | `Unstated` (the zero value) |
| the saved source | `// Recorded as: coordinates.` | **no line at all** |
| `orchestrator.ModeOf` | `Coordinates` | `Unstated` |
| the run record | `"mode": "coordinates"` | `"mode": "unstated"` |
| the routine's card | *Recorded as coordinates. …* | **nothing** |

A word this build does not know — a newer build's, or one typed into the file by hand — reads back
as `Unstated` rather than being echoed on. A reader that handed on whatever it found would put an
unverified string into a run record, where it reads as something this build checked.

## Why it is a comment and not a word of the language

The language governance rule: generated Marco must stay inside Core and must not name a backstage
concept. A recording mode is not something the route *does* — nothing reads it to decide anything
at run time — and a Marco program that named a recorder setting would be teaching its reader to
expect backstage in the play. So it is a comment, beside the `marco learn` header that is already
there.

It is written down because a person looking at a routine a month later is owed the answer to *how
was this made*, and because `marco do` puts it in the run record, which is where somebody
reporting a problem with that routine will look. A press aimed at a coordinate and a press found
by matching a picture fail in different ways, and a record that said `unstated` about a routine
whose own file states a mode would be withholding the answer at the moment it was asked for.

The honest limit, stated rather than hidden: the line says how the recording was **made**, which is
history, and no later edit can change history. So the line can survive a hand-edited route and
describe a file that has moved on — exactly as the `marco learn "name"` line above it still names
whatever the route was called the day it was generated. Nothing infers it, and a route with no line
says nothing.

## Two entrances to the generator, on purpose

`codegen.Route` has a dozen callers that know nothing about a mode, so `codegen.RouteAs` is a
second entrance rather than a changed signature. A caller forced to pass a value it does not have
would pass a guess, and a guess written into a saved routine is a claim about how somebody's
recording was made.

## And a repair carries the statement, including the part of it that is not a comment

Found in review after the rest of this had landed: `orchestrator.Resave` — the repair that saves a
guarded routine again under a scope that brings its window forward — rebuilt the draft stating
`Unstated`. Two things went missing, and only one of them was visible.

The visible one is the line. A routine that stated `coordinates` came out of a repair stating
nothing, so its card lost the sentence and every run record of it afterwards read `unstated`.
Absent is the honest answer for a file that never said; writing it over a file that did say is a
claim edited out of somebody's afternoon.

**The other one changes what the routine does.** The stated mode is also what keeps the recorded
gaps — `learnOptions` lifts the 50ms wait cap for a coordinate recording, because the cap only ever
made sense when an anchor poll did the real waiting. The repair ran under a `Deps` built for the
repair, which states nothing, so with `$MARCO_CV` inherited on it rebuilt the routine with every
pause capped and no anchor to wait on in their place. A menu pressed before it has drawn, out of a
repair. Studio is the window that inherits those flags all afternoon, and the repair button is in
it.

So the mode travels with the recording rather than with the process:

- `Resave` reads it from the SOURCE of the file it is about to replace, through `ModeOf`, for the
  same reason that function exists — the saved Marco is the only durable account of how a recording
  was made, and anything beside it is a second account to disagree with it. A word this build does
  not know still reads as unstated, so a repair never starts passing on an unverified claim.
- `DraftOf` simplifies under the mode it is GIVEN, and `Capture.Finish` under the mode its capture
  ran under, rather than both asking the `Deps` they happen to be holding. `learnOptionsFor` is
  that distinction, and its absence was the defect: the comment and the simplification were being
  decided from two different places.

Nothing was versioned and nothing is rewritten automatically. A routine on disk is read, not
migrated; a repair writes what the file already said.

## What did not move

- **The default is still off, and this does not answer the open question.** `cvOff()` keeps its
  note: nobody has decided what Marco *should* do, the default is off because the anchor stack was
  not reliable on the day somebody needed it out of the way, and answering it needs an experiment
  on a UI that actually moves. This slice does not flip anything — it stops one surface from
  inheriting the answer silently.
- **`$MARCO_CV=max` and the sensitivity dial are untouched.** They are the engineering switches and
  they still do what they did, for the callers that use them.
- **The visual recorder is unaffected.** `director demonstrate-visual` is a child process whose
  whole purpose is looking at the screen; it states nothing and reads the environment as before.
  Studio spawns it with the environment it has, exactly as it did.
- **The anchor-key gate in `keyboardProc` is redundant with the one in `recordClickDown`**, and is
  recorded as such in both places. Its mutation is **unfalsifiable** rather than a kill: removing
  it changes no behaviour, because the consumption gate refuses an arm in this mode anyway. It
  stays as the honest expression of the rule at the point where the key is read.

## Enforced by

- `internal/recorder` `TestAStatedCoordinateModeOverridesInheritedAnchors` — drives
  `recordClickDown`, the function the mouse hook calls, with the desktop read substituted, and
  watches what reaches the capture queue. All three directions: anchors inherited on + coordinates
  stated captures nothing; anchors inherited on + nothing stated still captures, so the CV stack is
  not quietly gone for the overlay and `marco learn`; anchors off + nothing stated still captures
  nothing.
- `internal/recorder` `TestACoordinateRecordingCannotBeArmedIntoCapturingAPicture` — a
  **right-click**, which is the one press the arm alone decides, since the global switch anchors
  left clicks whatever armed them. Three directions again, and the middle one is the assertion that
  caught the first version of the rule.
- `internal/recorder` `TestAnUnconsumedArmDoesNotSurviveIntoTheNextRecording` — `Start` clears the
  arm and publishes the stated mode.
- `internal/orchestrator` `TestTheRecorderIsToldTheModeBeforeItStarts` — the wiring claim, with a
  stand-in that records the mode **at Start** rather than at `Capture`, so a recorder told after the
  hooks went on is not counted as told. Falsified by deleting the hand-off: every other test in the
  file is about a draft and a file, and both still exist when nothing told the recorder anything.
- `internal/orchestrator` `TestACallerThatStatesNothingLeavesItToTheEnvironment` — the preservation
  claim, with the switches ON.
- `internal/orchestrator` `TestTheModeASaveWritesIsTheModeThisReads` — the writer is `codegen` and
  the reader is `orchestrator`, and they cannot share a constant because `codegen` may not import
  this package. Held as a pair here instead, the same discipline `measuredTerms` is under.
- `internal/orchestrator` `TestARoutineThatStatesNoModeReadsBackAsUnstated` — no line is written,
  nothing is read back, and `codegen.Route`'s old entrance invents nothing either.
- `internal/orchestrator` `TestAModeThisBuildDoesNotKnowReadsBackAsUnstated` — three unknown words.
- `internal/orchestrator` `TestAStatedCoordinateModeKeepsTheRecordedPauses` — with `$MARCO_CV=on`,
  and the second half checks the switch still does what it did for a caller that states nothing.
- `internal/orchestrator` `TestARepairKeepsWhatTheRoutineSaysItRecorded` /
  `TestARepairKeepsTheRecordedPausesOfACoordinateRoutine` — **the pair, and it has to be a pair.**
  Through the production repair (`Unrunnable` then `Resave`) with the sensing flags inherited ON. A
  repair that carried the comment and dropped the gaps passes the first and fails the second,
  measurably: simplifying under the `Deps`' mode instead of the stated one leaves the word in place
  and takes the routine's longest pause from 800ms to 250ms.
- `internal/orchestrator` `TestARepairStatesNothingWhenTheRoutineStatedNothing` /
  `TestARepairDoesNotCarryOnAWordThisBuildDoesNotKnow` — absence survives a repair as absence, and
  is not inferred from the geometry: this demonstration has no pictures in it, which is what a
  coordinate recording looks like AND what anchors-on against a flat menu looks like. An unknown
  word is not echoed either, because a repair writes the file a run record quotes.
- `internal/orchestrator` `TestARepairStillRepairsAndWritesTheSameRoutine` — the behaviour the line
  travels with, beside it: the same slug and application, now brought forward; one routine left in
  the library; nothing still unable to press; the demonstration still stored beside it; and the
  contract compared against what it was rather than against a word written in the test.
- `plugins/studio` `TestSavingAgainKeepsWhatTheRoutineSaysItRecorded` — the card after the repair
  button, in the window that inherits the flags.
- `cmd/marco` `TestTheRecordOfARepairedRoutineSaysWhatItWasRecordedAs` — the end of the chain, and
  the reason the loss would have been found by somebody reading a bug report rather than by a test:
  the record is read off the source, so a repair that edits the source changes every record after
  it without anything in `cmd/marco` changing.
- `cmd/marco` `TestTheRecordSaysWhatTheRoutineWasRecordedAs` /
  `TestTheRecordSaysUnstatedWhenTheRoutineDoesNotSay` — the word reaches the run record, and
  absence reaches it as `unstated`.
- `cmd/marco` `TestTheRecordInFlightAlreadySaysHowItWasRecorded` — measured **from inside the first
  host action**, because the attempt that most needs identifying is the one that never finished;
  and the finished record must agree with the one in flight.
- `cmd/marco` `TestTheRecordedModeIsNotTakenFromThisProcessEnvironment` — `marco do` is not the
  process that recorded anything, so `$MARCO_CV` may not reach a durable record.
- `plugins/studio` `TestStudioStatesCoordinateModeRatherThanInheritingIt` — the wiring claim and the
  visible one together, with the switches ON. Either alone is the defect wearing the other's
  clothes.
- `plugins/studio` `TestTheModeIsStatedBeforeAnythingIsRecorded` — the line is beside Record before
  anybody presses it, which is where the choice being described is still a choice.
- `plugins/studio` `TestASavedRoutineSaysHowItWasRecorded` — the card, the sentence, and the saved
  Marco itself.
- `plugins/studio` `TestARoutineThatSaysNothingAboutItsModeIsShownAsSayingNothing` — an older
  routine, written by hand as every library's are.
- `plugins/studio` `TestThePageSaysWhatARecordingWillCapture` /
  `TestARoutineThatStatesNoModeIsShownWithoutOne` — **rendered** in headless Edge or Chrome, because
  a source guard passes as long as the field is named in the file. The second one is the blank, in
  three ways: not a sentence, not `undefined`, not *Recorded as* anything.

## Mutations

Nine applied, eight killed, one recorded as unfalsifiable, plus a no-op control that survived as
intended. Two of them changed the code rather than the tests, which is the point of running them.

| | mutation | result |
|---|---|---|
| M1 | `Begin` never tells the recorder | **killed** — orchestrator and Studio both |
| M2 | `anchoring()` returns `captureAnchors`, ignoring the statement | **killed**, both recorder tests |
| M3 | the arm escapes the stated mode (`armed := anchorArmed.Load()`) | **killed** — *and its first run exposed the opposite defect in the production rule: the gate had been written as `anchoring() && …`, which took the anchor key away from every caller. The rule was corrected and the test rebuilt around a right-click so the arm is what decides.* |
| M4 | Studio stops stating the mode on its `Deps` | **killed** — *and its first run showed the page still promising coordinates over a recording that had gone back to inheriting them, because the constant was written at two sites. The page's word is now a copy of what the recorder was told.* |
| M5 | `codegen` writes `coordinates` when told nothing | **killed** — the absence assertions, through both entrances |
| M6 | `ModeOf` echoes whatever word it finds | **killed** — two of the three unknown words |
| M7 | `learnOptions` drops the stated-mode clause | **killed**, and the second half of the same test confirms `$MARCO_CV=off` still works |
| M8 | the run record always says `unstated` | **killed** by the in-flight test |
| M9 | the routine panel prints the sentence unconditionally | **killed** in the browser, and it rendered `undefined` — which is what makes the positive assertion necessary |
| M10 | the anchor-key gate in `keyboardProc` is removed | **unfalsifiable**, recorded as such: redundant with M3's gate, changes no behaviour |
| CONTROL | the wording of `capturesSentence` is changed | **survived**, as intended |

## Still to come, and not started

The mode is `coordinates` or nothing. There is no second mode to choose between, and no control
that offers one — Studio records coordinates because that is what the path *is*, not because
somebody picked it from a list. A surface that let a person choose would need a second mode worth
choosing, which is the anchor stack, which is the open question `cvOff()` names and which needs an
experiment rather than a dropdown.
