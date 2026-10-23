---
type: adr
status: accepted
date: 2026-10-23
decision: "ACCEPTED — THE RECORDER-FIRST PATH RUNS IN STUDIO'S OWN PROCESS, AND ITS FOUR ACTIONS ARE FOUR BUTTONS. Every other child Studio starts is a child; this one is not, because `orchestrator.Record` WRITES NOTHING and a draft held across a process boundary would have to be written somewhere for the parent to review it — which is the thing it exists not to do — or need a protocol of its own, which is a second Studio interface. So the draft lives in the window until somebody saves it or drops it, and Studio passes its OWN process id (`os.Getpid`, authoritative, no guessing) and its own app name (`winctx.Self`, the same transform the poller's samples come from) as the recording-exclusion boundary. Starting is SYNCHRONOUS and separable from finishing — `Begin` returns once the recorder is watching, so a recorder that would not start is refused at the button rather than discovered at Stop. Stop ends the capture and opens review; Cancel drops the draft, and there is nothing to delete; Save persists it under the scope CHOSEN IN THE REVIEW PANEL, and an unrecognised scope is refused rather than defaulted. Nothing is brought forward and nothing counts down: the person switches when they like and that switch is the first step. Review lists EVERY step in plain words, not only the ones that press, and is drawn once rather than on every poll. Because the hooks are now in the window's process, shutdown must end a live capture and wait for the pump, and the one predicate that answers 'is something being recorded' must answer for BOTH recorders — a run started while these hooks watch would press its own clicks into somebody's draft with no process boundary to notice. The visual recorder's path is unchanged and remains the one that asks what a press was."
supersedes: []
affects:
  - demonstrations
  - recorder
source_paths:
  - plugins/studio/recording.go
  - plugins/studio/acts.go
  - plugins/studio/frontend/index.html
  - internal/orchestrator/draft.go
  - internal/winctx/winctx.go
---

# The recorder runs in the window

## Two recorders, and which question each answers

Studio could already record. It spawned `director demonstrate-visual`, which looks at the screen,
correlates each press to a control it can see, and refuses to write a coordinate as a control's
identity. That recorder answers *what did they press, and can Marco recognise it again.*

It has no answer for a menu drawn over a live 3D world, which is where the owner actually works.
[[ADR-248-replaying-a-demonstrated-layout]] is the licence for that case and the recorder-first
path is how a routine gets made for it: `internal/recorder` watches the keyboard and mouse,
`internal/simplify` makes steps with the real pauses, `internal/codegen` writes legal Marco. It
answers *what did they DO, in order.*

Both exist on purpose. This ADR is about reaching the second one from a window.

## Why it is not a child process

Every other thing Studio starts is a child: the runner, the visual recorder, the performer. The
reason this one is not is the whole point of the backend it drives.

`orchestrator.Record` returns a draft and **writes nothing**. That is what makes Stop, Cancel and
Save three acts instead of one — there is something to cancel precisely because nothing is on
disk. A draft held in a child could not do that. It would have to be written somewhere for the
parent to review it, which is the thing it exists not to do; or the two processes would need a
protocol for handing a draft back and taking a Save command, which is a second Studio interface.

So the draft lives in the window, in one field, until somebody saves it or drops it.

The cost is real and is stated rather than waved at: the low-level hooks are installed in the
window's own process. That is why `internal/recorder` has the discipline it has — callbacks
return fast, the pump BLOCKS rather than polls, shutdown wakes the pump and unhooks from the
installing thread. The overlay has run hooks inside a GUI process for two milestones on the same
terms. Two consequences follow and are handled below.

## Starting can fail; finishing cannot

`Record` was one call: start the recorder, wait for the stop, make a draft. A window cannot use
that, because the only thing that can fail is the START — and it would fail into the draft, so a
person would press Record, see "recording", demonstrate the whole thing, and be told at Stop that
it had never begun.

So `Deps.Begin` returns a `Capture` once the recorder is watching, and `Capture.Finish` returns
the draft and cannot fail. `Record` is now those two in order, for the callers that block. The
window calls `Begin` on the button thread and `Finish` on its own.

## What Studio tells the recorder about itself

Pressing a button means bringing a window forward and clicking in it, and the hooks record that
like any other click. [[ADR-249-ending-a-recording-is-not-part-of-the-recording]] is the five
rules that take it back out again, and this is the two values they need:

| `OwnerPID` | `uint32(os.Getpid())` — Studio's own process, which is the boundary that holds |
| `Owner`    | `winctx.Self()` — the same transform the app poller's samples come from        |

`winctx.Self` is new and exists so the name is not three lines copied into the caller. A copy
that drifted by one character would exclude nothing and say nothing about it.

## The two consequences of hooks in this process

**Shutdown.** A live capture ends with the window, and is waited for. Leaving it would leave
low-level keyboard and mouse hooks installed on somebody's desktop with nothing left to unhook
them — a sharper version of the rule that already abandons a running visual recording.

**One predicate for two recorders.** Four places must refuse while the screen is being watched: a
run, a Scene deleted, everything forgotten, and a second recording. Each asked
`a.learn != nil && !a.learn.over` by hand, so a second kind of recording would have been
invisible to all four. A run started under these hooks presses the runner's own clicks into
somebody's draft, and there is no process boundary to notice it. `recordingLocked` is now the one
question, asked in all four places.

A draft in review is deliberately NOT a recording: the hooks are gone and the screen is free, so
a run is allowed again. It does still block a second capture, because there is one draft and
taking another would drop it unasked.

## What review shows, and what it does not

Every step, in plain words, in order — including the waits. A review that listed only the presses
would be a review of a shorter routine than the one being saved.

Not the generated Marco, and that is a deliberate deferral rather than an omission. The source
depends on the scope, and the scope is chosen in the same panel; a preview rendered before that
choice would be a preview of something else. Showing the Marco for the scope CURRENTLY selected,
regenerated when it changes, is the better review and is the next thing to build here. A preview
that drifts from what gets saved is worse than words.

## Enforced by

- `plugins/studio` `TestStopOpensReviewAndWritesNothing` — through `StartRecording` and
  `StopRecording`, asserting the routes directory is still empty and that review is open.
- `plugins/studio` `TestPressingStopInStudioDoesNotBecomeAStep` — the Architect's case, with
  Studio brought forward and the button pressed in it, no keystroke anywhere, and the poller
  sampling both sides of a mid-demonstration glance. The fixture's stopping click carries
  `os.Getpid()` and its switches carry `winctx.Self()`, so a Studio that passed the wrong owner
  or none fails here. Mutations killed: no owner name (every click lost), no owner process (the
  Stop click becomes a step), and the button not wired to `StopWhen` (Stop times out).
- `plugins/studio` `TestCancelDiscardsTheDraft`, `TestSaveKeepsTheRecordingUnderTheStatedScope`,
  `TestAnUnknownScopeIsRefusedAndTheDraftSurvives`.
- `plugins/studio` `TestTheWindowRecordsAgainAfterSaving` — found by reading, not by a failing
  test, and worth recording as such. Every other test here ends its recording and none started a
  second one after KEEPING the first, so nothing noticed that the guard against a second capture
  asked only whether a draft existed: a Studio that had saved one routine refused to record
  another for the rest of its life unless somebody pressed Cancel over a routine they had just
  kept. The same test holds that saving twice is refused rather than written again, because the
  second Save would use whichever scope happened to be selected then — a routine rescoped by a
  double-click.
- `plugins/studio` `TestARecordingWithNothingInItIsNotOffered` — Record pressed, mind changed,
  Stop pressed: every event is excluded, review still OPENS and says why, and Save refuses. A
  routine saved from that would be offered to somebody and then do nothing.
- `plugins/studio` `TestARecordingAndARunExcludeEachOther` — both directions, through `Play`,
  with a real Act so the refusal is the recording guard and not "there is no Act called that".
- `plugins/studio` `TestEveryMethodThePageCallsIsOnTheBackend` — reflection over `*Acts` against
  every `acts.X` in the page, plus the five this flow needs by name. There is no compiler between
  those two files: Wails binds whatever `*Acts` exports, so a misspelling is
  `undefined is not a function` inside a `.then` chain nobody watches, in a window that looks
  finished. Mutations killed: a misspelled call, Stop wired to a name that does not exist, and
  Cancel with no call at all.
- `plugins/studio` `TestTheReviewAStopButtonOpensIsRenderedOnce` — the real page in headless
  Edge over a stubbed draft, counting the steps as list items, the three scope choices, the Save
  button's state, and the rebuilds of the scope control. Mutations killed: the review card never
  shown, the waits filtered out of the list, the redraw guard made inert (30 rebuilds), and Save
  disabled regardless.

## Known gaps

- **The trial journal does not say which recorder a note came from.** The recorder-first path
  writes the same `recording` and `recorded` notes the visual one does, with a sentence on the
  closing note that names the path. A note kind of its own needs a reader to match, and no record
  at all while that was built was the worse option.
- **No Marco preview in review**, as above.
- ~~**Line A and B are not here**~~ — landed since: a press now records its window's outer size,
  its DPI as its own term, the process image and the explicit basis of the offset, and all of it
  reaches the saved route. Review shows them beside the click. The ADR for that contract is
  written with its other half; see below.
- **Line C is not here**: there is no press-moment guard on this path yet, so the evidence a
  press records is carried and not yet checked. Until it is, a route carrying it replays exactly
  as a legacy one does.

## Related

[[ADR-249-ending-a-recording-is-not-part-of-the-recording]] is the exclusion boundary this
supplies the two values for, and the record of the five corrections it took.
[[ADR-248-replaying-a-demonstrated-layout]] is the mode these recordings replay in.
[[ADR-066-stop-is-a-product-event]] decided that the person is the only reliable source of "I am
done"; the button is that, for this recorder.
[[ADR-245-one-task-at-a-time]] is the reservation these refusals sit beside.
