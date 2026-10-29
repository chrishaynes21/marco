---
type: adr
status: accepted
date: 2026-10-28
decision: "ACCEPTED — A RECORDING ADMITS INPUT FROM ONE WINDOW, CHOSEN BEFORE THE CAPTURE, AND THAT IS THE DEFAULT FOR STUDIO, LEARN AND LEARNAUTO. The whole desktop is retained explicitly, for a demonstration that genuinely spans applications. What a demonstration was before this was the whole desktop: every press anywhere entered it, so a message answered halfway through is in the routine — the letters of it as keystrokes, the coordinates of the window it was typed in, and a picture of whatever was clicked — and the thirty seconds somebody spent looking something up became a thirty-second wait the routine performed on every run, because `insertWaits` keeps each step’s real recorded gap. THE GATE IS AT THE INSTANT OF THE EVENT AND NOT A FILTER AT `Finish`: deciding afterwards that those steps are not wanted leaves them recorded, in the buffer, in `Draft.Events`, published to whoever was listening, and written to disk beside the routine, which is the shape this repository already has a rule about for passwords. The decision and the writes are one step under the lock the buffer is appended with, because the boundary is mutable state every event changes. AND THE LIVE CHANNEL IS GATED TOO, which the first version got wrong on the reasoning that the channel exists for the stop gesture and a consumer can filter — the same mistake one step along, because a consumer holds whatever is published. What a stop needs is the stop keys, since `StopKey.Triggered` reads nothing but an `EvKey`’s name and ignores every key outside the gesture, so that is the whole of what an off-target event may say; the capture-state markers are published so a surface can show what the capture is doing rather than off-target input. MEMBERSHIP IS THE FOREGROUND AND NOT THE RECTANGLE, which is what makes a drag out of the window and back one gesture rather than three. IDENTITY IS THE HANDLE with the owning process beside it: two windows of one application satisfy every comparison of process id, geometry and program name and are two different windows, so the process is identity rather than membership — Windows reuses handles, and a target whose owner no longer matches is GONE rather than one to keep recording, which is \"a pid is not a process\" read the other way round. Unreadable attribution is UNKNOWN and therefore not a member. The window is LATCHED rather than asked for up front, because at the moment somebody presses Record the window in front of them is Marco’s: the target is the first foreground that is neither the surface the recording was started from nor its process, which is exactly what `withoutOwner` has always assumed. A GESTURE THAT CROSSES THE BOUNDARY IS UNREPRESENTABLE AND NOT TRIMMED, because dropping a modifier and keeping the clicks it modified writes down `click` where the person performed `ctrl+click`: the draft is kept, review shows the steps and the reason, and Save refuses. EVERY KEY AND BUTTON, not only the modifiers — Q held across the edge is a held key whose press is not in the recording, and Q is \"use\" in half the games there are. AND IN THE OTHER DIRECTION, which the first draft of the rule missed: entering with Ctrl ALREADY DOWN makes a ctrl+click record as an ordinary click, a correct admission decision producing a wrong demonstration, so an admitted press or release taken while anything is held from outside is a crossing. THAT IS ALSO WHAT BOUNDS THE ACTIVATION-CLICK EXCEPTION: clicking on the window to bring it forward leaves a release of a button nothing pressed, which is dropped and refuses nothing, but the press is forgotten only when its release arrives — so an admitted action performed while it is STILL HELD has already made the recording unrepresentable. A MOVE NEVER TRIPS THE RULE, because it is a sample and not an intent, and whatever the person then does trips it. TIME INSIDE IS PRESERVED EXACTLY AND TIME AWAY IS REMOVED, leaving a documented 250ms where the absence was: a timing policy and NOT a readiness claim, since nothing is asked of the application and playback emits no activation there. Nothing is added for a trailing absence, and the leading absence — pressing Record and walking to the window — is taken out by the same rule and is not reported as an interruption. AND THE ABSENCE IS TAKEN OUT OF THE TIMESTAMPS, which is the compatibility answer and replaces the marker first proposed: a marker in a stored recording is a kind an older build would IGNORE, keeping the raw gap and regenerating a routine with a thirty-second pause in it, which is changed behaviour dressed as degradation. `inWindowClock` consumes all four markers in `Finish`, before `simplify` and before anything is written, so a stored recording holds only the kinds it always held with gaps that are already right for every reader of every vintage — there is nothing in it to reinterpret and therefore no version for an older build to refuse. A recording that crossed the boundary never reaches disk at all, because Save refuses it before `SaveRecording`. LearnAuto is confined like the others: it removes the PROMPTS, not the person, and `plugins/e2e` seeds drafts through `DraftOf` and starts no recorder, so there is no unattended caller to change. 40 of 40 mutations killed, the last three only after the cases they named existed:  AND THE WINDOW IS CHOSEN, NOT INFERRED, which the first version got wrong: it LATCHED the first foreground window that was neither the starting surface nor its process, and that is whichever window came forward first — a terminal, a notification toast, an updater — so the demonstration was admitted from a window nobody picked and a person cannot correct a choice they were never shown. `Begin` REFUSES a confined recording that was not told which window, before the hooks go on; `Deps.Pickable` is the list every surface asks out of, excluding this process, the asking surface and anything a person could not read or demonstrate in, with the foreground offered first as an ORDERING and an ambiguous name REFUSED rather than resolved by position, which is the same mistake one layer up. `marco learn --window` with a numbered prompt, a Studio control filled fresh, and the overlay passing the window the person was in — which it can answer because the HUD reads keys through its own hook and never takes the foreground. AND MEMBERSHIP IS ONE READING CARRYING BOTH HALVES: a click asked for its window while the gate asked the foreground again, so an event could be attributed to one window and admitted on the strength of another, and a key was tagged with the poller's SAMPLE and admitted on a third reading; `whose` is taken once per event, `admits` needs the handle AND the owner, and an admitted event carries the chosen window's name, resolved once at Start off the hook thread. AND WHAT WAS ALREADY HELD IS READ AT THE START: Ctrl down when Record is pressed is a press the recording never sees, so a ctrl+click was admitted and written down as an ordinary click — every key and button is scanned once and seeded as held-from-outside, and the arm that ADMITTED an unmatched release is gone, because `modifierTrouble` names modifiers and a release of anything else was a keystroke nobody made. AND THE POLLER MOVES THE BOUNDARY: markers were made only by `observe`, so leaving and returning without touching anything produced none at all and the routine waited the whole interval on every run — the defect the clock exists to prevent, through the one path that generates no input. AND THE PAUSE IS VISIBLE WHILE IT HAPPENS, which is not optional: a confined recording goes silent when somebody leaves and silence looks exactly like a recording that is still going. `waitForStop` says the transitions (the terminal prints them, the HUD streams the same child), Studio ASKS `Capture.Paused` on the poll that draws its card, and a capture whose person has not arrived yet is not paused. the key hook’s own call site (reached by building the structure Windows passes and calling `keyboardProc`), the handle check that is all a caller whose process is unknown has (`marco learn` in a terminal), and an activating release that cleared what was held beside it."
supersedes: []
affects:
  - execution
source_paths:
  - internal/recorder/confine.go
  - internal/recorder/recorder_windows.go
  - internal/recorder/recorder.go
  - internal/orchestrator/inwindow.go
  - internal/orchestrator/draft.go
  - internal/orchestrator/orchestrator.go
  - internal/winctx/liveness.go
---

# Record only the chosen window

## What a demonstration was

The recorder's hooks are low-level and global, and every press anywhere on the desktop entered the
demonstration. The only thing removed was Marco's own window, by process, and that exists so a
glance at Studio mid-demonstration does not become a drag from the application to Marco and back.

So a person demonstrating was demonstrating **the whole desktop**. Answer a message halfway
through and the reply is in the routine — the letters of it, as keystrokes, and the coordinates of
the window it was typed in, and a picture of whatever was clicked. The product relied on people
not doing anything else and said so nowhere.

It also relied on them not pausing. `simplify.insertWaits` keeps each step's real recorded gap, so
the thirty seconds somebody spent looking something up became a thirty-second wait the routine
performed on every run.

## The decision

A recording admits input from **one window**, chosen before the capture, and that is the default
for Studio, `Learn` and `LearnAuto`. The whole desktop is retained explicitly, for a demonstration
that genuinely spans applications.

Four rules, and the reasons each is the way it is.

## 1. Admission, not a filter at the end

The gate is at the instant of the event, not at `Finish`. Deciding afterwards that those steps are
not wanted leaves them **recorded**: in the buffer, in `Draft.Events`, published to whoever was
listening, and written to disk beside the routine. This repository already has a rule about that
shape for passwords.

So the decision and the writes are one step, under the lock the buffer is appended with — because
the boundary is mutable state that every event changes, and asking it outside the lock would be
two instants with another hook callback possible in between.

**And membership is decided from ONE reading of the desktop, containing both halves of the
identity.** A click used to ask `clickAttribution` for the window it landed in while the gate
asked the foreground AGAIN for itself: between two reads the foreground can change — that is what
a click on another window DOES — so an event could be attributed to one window and admitted on
the strength of another. A key was worse, tagged with the app poller's last SAMPLE and admitted
on a third reading, so a key pressed within the poll interval of a switch was labelled with the
application the person had just left. `whose{hwnd, pid, ok}` is taken once per event and decides
everything about that event; `admits()` requires the handle AND the owner to match, and an
admitted event is tagged with the chosen window's application, resolved once at `Start` and off
the hook thread because reading it opens a process handle.

**And the live channel is gated too.** The first version of this gated only the buffer, reasoning
that the channel exists for the stop gesture and a consumer can filter. That is the same mistake
one step along: a consumer holds whatever is published. What a stop actually needs is the stop
keys — `StopKey.Triggered` reads nothing but the key name of an `EvKey` and ignores every key
outside the gesture — so that is the whole of what an off-target event may say. Everything else
off-target reaches neither. The markers below are published, so a surface can show capture state
rather than off-target input.

**The boundary is the FOREGROUND, not the rectangle.** Membership asks whether the chosen window
is the foreground at that instant. That is what makes a drag out of the window and back one
gesture rather than three: a drag holds the foreground for its whole life, so every sample of it
belongs to the window it started in, wherever the pointer went. A rectangle test would refuse the
ordinary way to drag something onto a taskbar button.

## 2. The window is a window, and it is CHOSEN

Identity is the **handle**, with the owning process beside it. Not the process: two windows of one
application satisfy every comparison of process id, geometry and program name and are two
different windows — which is why `RecordedEvent.HWnd` exists at all. The process is identity
rather than membership, because Windows reuses handles, and a target whose owner no longer matches
is a target that is **gone** rather than one to keep recording. It is "a pid is not a process"
read the other way round. Attribution that cannot be read is UNKNOWN and therefore not a member.

**And it is selected before capture, by the person.** The first version LATCHED it: the first
foreground window that was neither the surface the recording was started from nor its process
became the target. That reads as reasonable — you press Record and go to the application — and it
is not a choice. Whichever window comes forward first is whatever Windows does next: a terminal,
a notification toast, an updater, the Start menu. The demonstration that followed was then
admitted from a window nobody picked, and **a person cannot correct a choice they were never
shown.**

So `Begin` REFUSES a confined recording that was not told which window, before the hooks go on,
because after them somebody is already demonstrating. The alternative to refusing is recording
the desktop, which is the one outcome this whole decision exists to prevent, arriving silently.

`Deps.Pickable()` is the list every surface asks out of. It leaves out this process and the
asking surface (the exclusion `withoutOwner` has always made, moved to where it can be a choice
rather than a repair), minimized and off-screen windows, and anything with neither a caption nor
a name — a list with blank lines in it is not a choice. The window in front is offered first,
because the person reading the list is looking for the one they were just in; that is an
ORDERING and not an answer. **An ambiguous name is refused rather than resolved by order**: two
captions containing "settings" is a question, and answering it by taking the first is the latch
again one layer up.

**And the application written down is the chosen window's, not a sample of the foreground.**
The app-switch poller is a second door into the buffer, and the gate on it closed one half of a
race: a sample taken while the person was away is refused, because the admission snapshot still
reads elsewhere. The other half is a sample taken while they were away and admitted after they
came back — the reading agrees, correctly, that they ARE in the window, and the string written
beside it still names Chrome. `simplify` turns that `EvAppSwitch` into an `Activate`, so the
routine begins by raising a browser nobody demonstrated in, and `Keep` files it under that
application. So membership and the recorded name come out of the same fact, the window that was
CHOSEN, and a confined recording therefore holds exactly one application switch: its own. A
target whose image could not be read records NONE — which is why `chosenApp` is separate from
`appOfTarget`, whose fallback to the sample is right for a TAG on an event that did happen in the
window and wrong for a STEP that claims an application was raised.

Each surface supplies only the interaction. `marco learn --window <n|handle|name>`, with a
numbered prompt when it is not given and a REFUSAL for `--auto`, which saves with nobody having
looked and so has nobody to answer a prompt. Studio shows a control filled from `Acts.Windows()`,
asked fresh every time because the person is about to open the application they mean. And the
overlay passes the handle of the window the person was in when they summoned the HUD — which it
can answer truthfully because the HUD reads the keyboard through its own global hook and never
takes the foreground, so the window in front as somebody types is the one they mean.

## 3. A gesture that crosses the boundary is unrepresentable

Not trimmed. A gesture starting inside cannot authorize recording its outside portion, and the
inside half alone is not what the person did: dropping a modifier and keeping the clicks it
modified writes down `click` where they performed `ctrl+click`. So the draft is kept, review shows
the steps and the reason, and **Save refuses** — the person is the only one who can perform it
again differently.

**Every key and button, not only the modifiers.** `simplify.modifierTrouble` is about a modifier
changing what the keys under it mean. A boundary is not that question: Q held across the edge is a
held key whose press is not in the recording, and Q is "use" in half the games there are.

**And what was already held when the recording began.** A recording starts in the middle of
somebody's day: Ctrl held when Record is pressed is a press this recording will never see.
`Boundary` tracked only what it watched go down, so it started empty and believed nothing was
held, and a ctrl+click performed in the chosen window was admitted and written down as an
ordinary `click` — the same defect as entering with Ctrl down, one step earlier, invisible to the
boundary because the press happened before there was a boundary. So the physical state of every
key and mouse button is read ONCE at `Start`, off the hook thread, and seeded as held-from-
outside: one rule then covers two facts that are the same fact at different times. The whole
keyboard and not a list of modifiers, for the reason the rule below gives. And the arm that
ADMITTED a release of something never seen going down is gone — it was the hole, because
`simplify.modifierTrouble` names modifiers and a release of anything else was recorded as a
keystroke nobody made.

**And in the other direction**, which the first draft of this rule missed: entering the window
with Ctrl ALREADY DOWN. The press is outside and excluded, so a ctrl+click in the window would be
recorded as an ordinary click — a correct admission decision producing a wrong demonstration. So
an admitted press or release taken while anything is held from outside is a crossing, which is the
same rule read the other way round.

**That is also what bounds the activation-click exception.** Clicking on the window to bring it
forward lands its press outside and its release inside, leaving a release of a button nothing
pressed; that release is dropped and nothing is refused, because it is how you get IN rather than
part of the demonstration. The exception is bounded by forgetting the press only when its release
arrives — so an admitted action performed while it is STILL HELD has already made the recording
unrepresentable. Click-to-return is dropped; entering with Ctrl down and clicking is refused.
`TestClickingBackIntoTheWindowIsDroppedAndNotRefused` and
`TestEnteringTheWindowWithCtrlHeldRefusesTheRecording` are deliberately separate cases.

A **move** never trips the rule. It is a sample of where the pointer was, not an intent, and a
rule that let one refuse would make a recording unsaveable because somebody's mouse was nudged.
Whatever they then DO trips it, and a test holds that distinction so a move cannot disarm the rule
instead of merely not tripping it.

## 4. The clock, and why the markers never reach the disk

Time spent inside is preserved exactly — a person who paused for ten seconds meant to. Time away
is removed, leaving a documented **250ms** where the absence was. Nothing is added for a trailing
absence: somebody who walks away and stops the recording from somewhere else has not asked the
routine to wait at the end. A leading absence — pressing Record and walking to the window — is
taken out by the same rule and is not reported as an interruption, because they had not arrived.

The 250ms is a **timing policy and not a readiness claim**. Nothing is asked of the application
and nothing is observed; playback does not activate the window there either. Zero would replay a
demonstration faster than it was given, and the real absence would replay somebody reading their
messages. If a routine needs the window brought forward, that is an `Activate`, and an `Activate`
is something `simplify` emits from a foreground switch that actually happened.

**And the absence is taken out of the TIMESTAMPS.** This is the compatibility answer, and it
replaces the one first proposed. Markers in a stored recording are a kind an older build does not
know: it would ignore them, keep the raw timestamps, and regenerate a routine with a thirty-second
pause in it — changed behaviour dressed as degradation. An ignorable marker is exactly what must
not be relied on. So `inWindowClock` consumes all four markers in `Finish`, before `simplify` and
before anything is written, and `Draft.Events` contains none of them. A stored recording holds
only the kinds it always held, with gaps that are already right for every reader of every vintage.
There is nothing in it to reinterpret and therefore **no version for an older build to refuse**.

A recording that crossed the boundary never reaches disk at all, because `Save` refuses it before
`SaveRecording` — so the one case an older reader could mishandle does not exist as an artefact.

**And the boundary moves when the foreground does, not when the next key arrives.** The markers
were made by `observe`, which only runs when the hooks see something, so somebody who left the
window, read a page and came back WITHOUT TOUCHING ANYTHING produced no markers at all — and with
no pair around the absence the clock had nothing to take out, so the routine waited the whole
interval on every run. Exactly the defect the clock exists to prevent, arriving through the one
path that generates no input. The app-switch poller already samples the foreground, so it reports
the transition through the same fenced boundary update; `Arrived` answers only on a CHANGE, so
the two doors cannot double-count.

## And the pause is visible while it is happening

Not optional. A confined recording goes silent the moment somebody leaves the window, and silence
looks exactly like a recording that is still going: alt-tab to check something, carry on
demonstrating, and the routine is missing half its steps with nothing having said so. The review
sentence afterwards is no substitute, because by then the demonstration they have to give again
is over.

`waitForStop` already reads every event, markers included, so it says the transitions there — one
consumer of the channel and no second reader to steal from it. The terminal prints them, and the
overlay's HUD streams the same child's stdout, so both say it from one place. Studio redraws on a
poll and cannot see markers, so it ASKS: `Capture.Paused` reads the boundary under the lock that
mutates it, and the card shows a banner naming the window and how long. Before the person has
first arrived the answer is NOT paused — they have chosen a window and not gone to it yet, which
is not a pause in a recording.

## Retained means reachable, and review says what happened

`marco learn "<name>" --whole-desktop` is the way a person asks for the old behaviour, and either
way the scope is PRINTED before the recording starts — because the default changed, and
somebody whose routine is four steps when they demonstrated six is owed the reason before they
demonstrate rather than only in review. A capability nothing can reach is the pattern this
repository has eight recorded cases of, which is why the flag exists and why
`orchestrator.WholeDesktop`'s own comment names it.

**Studio has no control for it yet**, and that is a gap recorded as one rather than a decision:
its window records one window and cannot be told otherwise. Its review card DOES say what
happened — `Draft.AwaySentence` reaches both surfaces, because the draft in front of somebody is
shorter than the demonstration they gave and the clock no longer matches what happened, and
without the sentence the only available reading is that the recorder missed something.

## What this does not change

The whole-desktop scope is retained and is what every existing recording was. Nothing on disk is
rewritten, no saved routine is reinterpreted, and the stop gesture works from anywhere it ever
did. `LearnAuto` is confined like the others: it removes the PROMPTS, not the person — all three
starts have somebody at the keyboard, and `plugins/e2e` seeds drafts through `DraftOf` and starts
no recorder, so there is no unattended caller to change.

## Enforced by

### The buffer, the channel and the crossing rules

- `internal/recorder` `TestOffTargetInputReachesNeitherTheBufferNorTheChannel` — a message typed
  away from the window, asserted absent from BOTH, and an off-target press whose index is -1 so
  no screen capture is queued against it.
- `internal/recorder` `TestTheStopKeyFromElsewhereIsPublishedButNotRecorded` — published, not
  recorded, and `StopKey.Triggered` fires on what was published; with an ordinary off-target key
  beside it as the control.
- `internal/recorder` `TestACrossingIsRecordedAndPublishedOnce`,
  `TestLeavingAndReturningAreRecordedAsCaptureState`,
  `TestAnUnconfinedRecordingAdmitsEverything`.
- `internal/recorder` `TestClickingBackIntoTheWindowIsDroppedAndNotRefused` versus
  `TestEnteringTheWindowWithCtrlHeldRefusesTheRecording`,
  `TestAnOrdinaryKeyHeldFromOutsideRefusesTheRecording` and
  `TestTheActivatingClickDoesNotExcuseWhatIsStillHeldBesideIt` — the bound on the exception, as
  separate cases, because the version that cleared what was held beside an activating release
  survived a gate of every other case in the file.
- `internal/recorder` `TestLeavingTheWindowWithAKeyHeldRefusesTheRecording`,
  `TestADragThatLeavesTheRectangleStaysOneGesture`,
  `TestAPointerMoveWhileSomethingIsHeldOutsideRefusesNothing`,
  `TestTheFirstCrossingIsTheReasonGiven`,
  `TestAReleaseOfSomethingNeverSeenGoingDownIsNotRecorded`.

### Which window, and what decides membership

- `internal/orchestrator` `TestTheWindowListLeavesOutWhatNobodyCouldChoose`,
  `TestAWindowCanBeNamedByPositionHandleOrName`, `TestTheWindowInFrontIsOfferedFirst`,
  `TestANameThatMatchesNothingIsRefusedWithWhat` and
  `TestANameThatMatchesTwoWindowsIsRefused` — the last one is the latch rule read one layer up:
  answering an ambiguous name by position is choosing a window nobody picked.
- `internal/orchestrator` `TestAConfinedRecordingWithNoChosenWindowIsRefusedBeforeItStarts` — it
  is refused, the recorder was not told to confine itself to nothing, and it was not started;
  with the whole desktop beside it so the refusal is about the missing CHOICE.
- `internal/orchestrator` `TestWhyNoWindowNamesWhatIsMissingAndHowToSayIt`,
  `TestTheRecordingIsConfinedToOneWindowUnlessTheCallerSaysOtherwise` — the second is the
  mutation that matters most: delete the `Only` call from `Begin` and every other case still
  passes, because they all hand `Finish` a recording that was already gated.
- `internal/recorder` `TestOnlyTheChosenWindowIsAdmitted` — the chosen window, another window of
  the same application, the SAME HANDLE UNDER A DIFFERENT OWNER, three kinds of unreadable
  attribution, and a window that has gone.
- `internal/recorder` `TestOneReadingDecidesBothTheAdmissionAndTheEvent` — a desktop that answers
  differently every time it is asked: the press reads it once, the recorded window is the window
  that was admitted, and the edge that reads as elsewhere is not recorded.
- `internal/recorder` `TestAnAdmittedEventIsTaggedWithTheChosenWindowsApplication` — with the
  poller's sample deliberately lagging and wrong, and an unconfined recording beside it still
  reading that sample.
- `internal/recorder` `TestAConfinedApplicationSwitchNamesTheChosenWindow` — the stale sample
  admitted after the person came back, a second sample adding nothing, a target whose image could
  not be read recording no switch at all, and an unconfined recording still writing its sample.
  The first assertion is the one the gate on the poller does NOT cover.
- `internal/orchestrator` `TestTheWindowInFrontIsOfferedFirst` states BOTH halves of the desktop,
  which is the point: a fixture list under a live foreground reading orders by a handle no row
  has, so the swap never happens and the rule under test does nothing. With an unreadable
  foreground and a foreground outside the list beside it, and `ForegroundForTest` exported for
  the same reason `WindowsForTest` is.
- `plugins/studio` `TestTheWindowListReachesThePage` asks the act the page calls, against a stated
  desktop and then an empty one. It used to read the live list and return early when there was
  nothing on it, so a machine with no desktop exercised none of it.

### What was already held, and the clock

- `internal/recorder` `TestSomethingHeldBeforeTheRecordingBeganRefusesTheRecording`,
  `TestTheReleaseOfSomethingHeldBeforeTheRecordingIsDropped` and
  `TestNothingHeldAtTheStartRefusesNothing` — the third is the control: a seeding that always
  refused would pass the first two and no recording would ever be saveable.
- `internal/recorder` `TestLeavingAndReturningWithNoInputStillMovesTheClock` — an absence with no
  hook event in it at all, which is the path that produced no markers and kept its full playback
  delay.
- `internal/recorder` `TestAConfinedRecordingBeginsWithAnAbsence`,
  `TestAnApplicationSwitchFromElsewhereIsNotRecorded` — the poller is a second door into the
  buffer and the gate is on it too — and `TestAnOffTargetPressQueuesNoScreenCapture`.
- `internal/recorder` `TestTheKeyHookDoesNotRecordKeysFromAnotherWindow` — the one call site a test
  of `observe` cannot reach: the keys go through `keyboardProc`, a C callback, so it builds the
  structure Windows passes and calls it.
- `internal/orchestrator` `TestTheClockKeepsTimeInTheWindowAndDropsTimeAway`,
  `TestAnAbsenceShorterThanTheSettleIsLeftAlone`, `TestATrailingAbsenceAddsNoTimeAtAll`,
  `TestNoCaptureMarkerSurvivesIntoTheRecording`.
- `internal/orchestrator` `TestADraftExcludesTheTimeSpentAwayAndSaysSo`,
  `TestACrossingDraftIsKeptForReviewAndRefusedAtSave`,
  `TestTheCrossingIsNamedAheadOfSimplifysOwnRefusal` — with the control, so it is an ordering and
  not a crossing that swallows everything.
- `internal/orchestrator` `TestAClosedWindowEndsTheRecordingAndIsNamedOnTheDraft`,
  `TestAWindowThatClosesMidGestureRefusesTheSave`,
  `TestTheChosenWindowClosingEndsARecordingNobodyStopped` and
  `TestAStopPressedOutsideTheWindowEndsTheRecording` — both on a stream nobody closes, so what
  ended the recording is what the case is about.

### Two survivors, recorded rather than contrived against

**27 of 29 mutations killed.** The two that survive are written down because a gate that reports
only kills is a gate nobody can read:

- `whose.admits`'s unreadable arm is UNFALSIFIABLE from production. `Start` sets `confined` only
  when the target carries both halves, so the comparison after it already refuses a reading with
  a zero in it. It stays because it is the honest expression of "unknown is not membership" at
  the point the answer is read, and because the two guards are about different things — the
  reading, and the target.
- `alreadyHeld`'s bit test (`0x8000`, "down now", against `0x0001`, "pressed since last asked")
  cannot be exercised headlessly: establishing it needs a key physically held while the scan
  runs. The same class of gap as physical capture, and recorded as one. `plugins/e2e`
  `isHeldDown` reads the same bit for the same reason, and that harness does hold keys down.

### The surfaces

- `internal/orchestrator` `TestLeavingTheWindowIsSaidWhileItIsHappening` — the transitions, the
  return and the crossing, all said as they happen rather than at review.
- `internal/orchestrator` `TestACaptureAnswersWhetherItIsPaused` and `internal/recorder`
  `TestPausedIsAnsweredFromTheBoundary` — including that a capture whose person has not arrived
  yet is NOT paused, which is the state every confined recording opens in.
- `plugins/studio` `TestTheRecordingCardSaysWhenTheCaptureIsPaused`,
  `TestRecordingRefusesAWindowThatIsNotThere` and `TestTheWindowListReachesThePage` — through the
  acts the page actually calls, because what was missing was never a list function: it was
  anybody asking.
- `internal/orchestrator` `TestTheTerminalSaysTheWindowWasLeft` and `plugins/studio`
  `TestReviewSaysTheWindowWasLeft` — both review surfaces, because the sentence existed and
  nothing said it for as long as it took to write the first of them.
- `cmd/marco` `TestTheWholeDesktopCanBeAskedForAndIsNotTheDefault`,
  `TestTheWholeDesktopFlagIsWiredToTheRecording`,
  `TestTheChosenWindowFlagIsParsedAndReachesTheRecording` and
  `TestAnUnattendedRecordingWillNotBeAskedWhichWindow` — the last one is why `--auto` is TOLD and
  not asked: a prompt there hangs a HUD on a question nothing can see. The two source checks are
  there for the reason `howMuchToRecord` and `chooseTheWindow` are separate functions:
  `runAssistantLearn` ends in `os.Exit`, so the only other way to establish that a flag reaches
  `Deps` is to run the real recorder on somebody's desktop.
