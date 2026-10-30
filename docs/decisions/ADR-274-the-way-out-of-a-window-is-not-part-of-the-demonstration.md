---
type: adr
status: accepted
date: 2026-10-30
decision: "ACCEPTED — A KEY STILL HELD WHEN THE CHOSEN WINDOW IS LEFT IS A FACT THE RECORDER REPORTS, NOT A VERDICT IT REACHES. The boundary used to refuse the recording at that instant, and Alt-Tab holds two keys down — so the one obvious way to finish a recording, come back to the window with the Stop button in it and press Stop, refused every recording it ended, and the only exit that worked was a shortcut named in six grey words beside the button. The owner met this on their first recording after the library was emptied. THE INSTANT OF THE LEAVING IS THE ONE INSTANT NOBODY CAN DECIDE AT: a key held as the window is left is either a gesture the demonstration depends on, whose other half happened somewhere else, or it is the way out — and what separates them is what happens NEXT, which is nothing at that moment. So `Boundary.Arrived` and the release-outside arm of `Boundary.Pressed` queue the key, both doors drain it into the buffer as an `EvHeldAcross` marker after the `EvLeft` it belongs to, and `orchestrator.inWindowClock` decides with the whole stream in front of it. The marker is appended to the kinds rather than inserted, so no other kind's number moves, and it is a MARKER — consumed in `Finish`, never written to disk, so a stored recording still holds only the kinds it always held. THE WAY OUT IS THE ONLY THING THIS BOUNDARY TRIMS INSTEAD OF REFUSING, and three bounds keep that safe: NOTHING MAY FOLLOW IT, so everything after the first of those presses must be another of them — Alt-Tab's presses are the last things in the buffer because what happened after them happened outside the window and was never admitted, while a key held through the rest of a demonstration has that demonstration after it and refuses; KEYS ONLY, because a button stranded at the boundary is a drag whose end happened elsewhere and dropping its press would turn a drag into nothing, which is STRUCTURAL rather than a branch since presses are identified by `KeyName` and a click has none; and SOMEBODY LEFT, because a key held when the window WENT AWAY is a gesture the closing cut in half and there was no button they were walking to. A STRANDED KEY THE DEMONSTRATION CARRIED ON PAST IS PROMOTED TO A CROSSING as soon as anything is admitted after it, which is the only thing standing between a later Alt-Tab and a press taken silently out of the middle of somebody's demonstration: left in the stranded set, Q arrives at the decision beside Alt and Tab, every press in the trailing run belongs to that larger set, and the way out carries Q off with it. FIRST REASON WINS, so the crossed list is not sorted — its order is the order things crossed in. AND A REFUSED RECORDING IS SHOWN WHOLE: nothing is trimmed out of a recording that is not being saved, because all of what the person did is what tells them how to give the demonstration again. AND IT IS SAID, NEVER SILENT — `Draft.ExitSentence` names the keys, the terminal prints it through `Learn` and Studio's review card draws it above the steps, because leaving something out of somebody's demonstration is the discard this boundary exists to refuse and the only person who can disagree is the one who performed it. Confinement is untouched: nothing new enters the buffer, the admission rule is unchanged, and every other half-gesture still refuses. 17 of 18 mutations killed; the survivor is the structural rule above, which no single deletion can change. F12 remains an alternative rather than the workaround required to use the product."
supersedes: []
affects:
  - execution
source_paths:
  - internal/recorder/confine.go
  - internal/recorder/recorder.go
  - internal/recorder/recorder_windows.go
  - internal/orchestrator/inwindow.go
  - internal/orchestrator/draft.go
  - internal/orchestrator/orchestrator.go
  - plugins/studio/recording.go
  - plugins/studio/frontend/index.html
---

# The way out of a window is not part of the demonstration

## What the owner met

The library had just been emptied, so this was a first recording: `enter freeplay`, demonstrated in
Trackmania. They recorded it, came back to Studio, pressed **Stop recording**, and the Save button
was dead. The review said:

> tab was still held when the window was left, so the recording has half of a gesture in it

They asked the only question there was to ask: *how would I tab out to end the recording?*

The answer at the time was that you do not — you press F12 in the game, which
[[ADR-268-record-only-the-chosen-window]] made the only safe exit without ever saying so. The
window offered **Stop recording** as its primary control and six grey words, *or press F12*, beside
it. Alt-Tab holds Alt and Tab down while the switcher takes the foreground, so the boundary was
crossed by the gesture that reached the button, and **every recording ended the obvious way was
refused.**

It is the same defect `TestAVisitToStudioLeavesARecordingThatStillSaves` closed for the mouse, one
modality along. Coming back to Studio by CLICKING on it has been excused since the activation-click
exception; coming back by Alt-Tab was not, and nothing connected the two.

## Why the boundary could not answer

[[ADR-268-record-only-the-chosen-window]]'s rule is right and stays: *a gesture that crosses the
boundary is unrepresentable, not trimmed.* Dropping a modifier and keeping the clicks it modified
writes `click` where somebody performed `ctrl+click`, and saving that silently is worse than
refusing.

What was wrong was WHERE the rule was applied. `Boundary.Arrived` called `cross` the moment the
window was left with something held — and at that moment there are two possibilities and no way to
tell them apart:

- a gesture the demonstration depends on, whose release happened in another window;
- the Alt-Tab somebody used to reach the Stop button.

They call for opposite answers, and what separates them is **what happens next**: either the rest
of a demonstration, or nothing at all. The boundary is a state machine stepped one event at a time;
the future is exactly what it does not have. So it answered the first way every time.

This is a correct rule evaluated at the
wrong instant: a tested mechanism is not a tested moment.

## The decision

**The recorder reports the fact. The review decides what it meant.**

`EvHeldAcross` carries the KEY rather than a sentence, appended to the event kinds so no other
kind's number moves. Both doors that move the boundary drain it — `observe` for an event-driven
leaving and `noteBoundary` for the foreground poller, which is the one that usually notices, since
it is the switcher taking the foreground that it sees rather than the key that was still down. The
marker lands after the `EvLeft` it belongs to, so a reader of the stream can tell which absence
stranded what.

It is a MARKER, which means `inWindowClock` consumes it in `Finish` and `Draft.Events` contains
none: a stored recording still holds only the kinds it always held, so there is nothing in it for
an older build to interpret differently and no version for one to refuse. That is the same
compatibility answer ADR-268 gave for the absence, for the same reason.

### What the review decides, and the three bounds on it

`exitGesture` is the whole of it. The way out is the trailing run of unreleased key presses before
the recording's last leaving — and each bound below is load-bearing:

| bound | why |
| --- | --- |
| **nothing may follow them** | everything after the first of those presses must be another of them. Alt-Tab's presses are the last things in the buffer because what came after them happened outside the window and was never admitted; a key held through the rest of a demonstration has that demonstration after it, and refuses. |
| **keys only** | a button stranded at the boundary is a drag whose end happened elsewhere, and dropping its press turns a drag into nothing at all. STRUCTURAL rather than a branch: presses are identified by `KeyName` and a click has none — the boundary calls it `the left button`. |
| **somebody left** | a key held when the window WENT AWAY is a gesture the closing cut in half. Nobody navigated anywhere and there was no button they were walking to. |

**And a stranded key the demonstration carried on past is promoted to a crossing** the moment
anything is admitted after it. This is the one thing standing between a later Alt-Tab and a press
taken silently out of the middle of a demonstration: Q held, the window left, the person comes back
and finishes with Alt-Tab. Leave Q in the stranded set and all three arrive at the decision
together, every press in the trailing run belongs to that larger set, the check passes, and the way
out carries Q away with it. Promote Q and the recording refuses, naming Q.

It took two attempts to test. The first put a click after Q, which the trailing-run check refuses
on its own account — so deleting the promotion changed nothing and the test passed either way. The
separating case has nothing but KEYS after the stranded one.

**First reason wins**, so the crossed list is not sorted: its order is the order things crossed in,
and the first is the one the person can do something about.

**And a refused recording is shown whole.** Nothing is trimmed out of a recording that is not being
saved: the person is about to be told it cannot be kept, and all of what they did is what tells
them how to give it again. Trimming it would also have review saying *these were left out* about a
recording that is going nowhere.

### It is said, never silent

This is the only thing the boundary takes out instead of refusing, so review names it:
`Draft.ExitSentence` says which keys, the terminal prints it through `Learn` beside the absence
sentence, and Studio's card draws it ABOVE the steps, because it is about steps that are not there.

The keys are NAMED rather than counted. *Two keys were left out* is a sentence somebody can only
act on by guessing, and the whole point of saying it is that the person who performed the gesture
is the only one who can decide it was not the way out.

## What is NOT changed

Confinement. Nothing new enters the buffer, the admission decision is untouched, the live channel
is gated as it was, and every other half-gesture still refuses — a key released outside with the
demonstration carrying on, an action taken with something held from outside, a drag that left the
window, a window that closed mid-gesture. ADR-268's rule is intact; what changed is the instant it
is applied at.

And the guidance half of the first ruling on this is retired by the fix rather than delivered: with
**Stop recording** working, making F12 primary would be promoting the workaround, and warning that
Alt-Tab can leave an unfinished gesture would be warning about something that no longer happens.
F12 stays where it is, as an alternative.

## Enforced by

- `internal/recorder`: `TestLeavingTheWindowWithAKeyHeldIsReportedForJudgingLater` — the boundary
  reports and does not refuse, from both doors, and drains what it reported.
  `TestLeavingReportsEveryKeyItStranded` — every key, not whichever one a map handed back first,
  because Alt-Tab strands two and a reported one leaves the other in the recording.
  `TestAKeyStrandedByLeavingReachesTheRecordingAsAFact` — the wiring between the boundary and the
  buffer, which no test of either alone can hold, and the half that says no crossing is written
  there.
- `internal/orchestrator`: `TestAltTabbingBackToStopTheRecordingSaves` and
  `TestTheKeysUsedToLeaveAreNotInTheDemonstrationAndWhatPrecededThemIs` — the headline case through
  `Finish` and `Save`, and both halves of what it leaves behind.
  `TestAHeldKeyTheDemonstrationCarriedOnPastStillRefuses`,
  `TestAKeyHeldUnderTheDemonstrationIsNotExcusedByLeavingAfterwards`,
  `TestAKeyHeldWhenTheWindowWentAwayIsStillACrossing` and
  `TestAMouseButtonStrandedAtTheBoundaryIsNeverTheWayOut` — the three bounds.
  `TestAStrandedKeyIsNotCarriedOffByALaterWayOut` — the promotion, in the only case where it is the
  only thing that answers.
  `TestAKeyStrandedBeforeTheDemonstrationCarriedOnIsNotTakenOutByTheWayOut` is the first attempt at
  that case and is kept: it is what a refused recording being shown whole looks like.
  `TestOnlyAnOpenPressCanBeTheWayOut` — the function's own rule, and
  `TestTheWayOutIsRecognisedWhicheverOrderTheDesktopReportsIt` — all four orderings the desktop can
  report this in, because which one happens is not a choice this code makes.
  `TestTheTerminalSaysWhatTheWayOutLeftOut` — `marco learn` says it, which a payload field Studio
  reads is not.
- `plugins/studio`: `TestComingBackByAltTabLeavesARecordingThatSaves` and
  `TestAStrandedKeyTheDemonstrationUsedIsStillRefusedInStudio` — both halves at the surface the
  owner met this on. `TestTheReviewCardSaysWhatTheWayOutLeftOut` and
  `TestAReviewThatLeftNothingOutShowsNoSuchBox` — rendered, because a sentence in a payload nobody
  draws is a sentence nobody reads.

**17 of 18 mutations killed.** Four survived the first run and each wrote a better test: the
promotion (two attempts — the first case was refused by the trailing-run check anyway), the
release-tracking inside `exitGesture` (covered by the promotion through the whole clock, so the
test asks the function its own question), and the terminal's print, which nothing in this
repository held. The remaining survivor is the structural keys-only rule, which no single deletion
can change, and it is written down in `exitGesture`'s own comment as structural rather than left
looking like an untested branch.

## What this does NOT establish

That any of it works **on a desktop**. Every case above hands the decision a stream of events, and
the shape of the real one — the exact order in which the switcher takes the foreground, the poller
notices, and the keys go down — came from reading `observe` and `noteBoundary` rather than from a
recording made by a hand. The owner's own next recording is the test that matters, and Studio has
to be rebuilt and reopened before it is: this fix is in the recorder Studio runs **in its own
process**, so a window left open across the build records through the old one.
