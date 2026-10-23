---
type: adr
status: accepted
date: 2026-10-23
decision: "ACCEPTED — A WINDOW THAT RECORDS MUST NOT RECORD ITSELF, AND STOP IS NOT SAVE. Studio gets a visible Stop button, and the recorder's own window is excluded from the demonstration WHEREVER it appears, not only at the tail: a window that invites being looked at will be looked at mid-demonstration. Exclusion is decided by the PROCESS that owned the foreground window at the instant of each event, read in the hook from two window-data calls, and never by the application name an interval poller sampled — a sample can be stale by up to one poll, and the one case where it is stale is a click made in the TARGET application just after looking at the recorder, which is exactly the click this exists to save. Known event-time ownership outranks a sampled name in BOTH directions: to condemn an event the sample excuses, and to spare one the sample accuses. Both edges of a click carry their process, because a button is activated on release and an unattributed release hides the press behind it. The one unattributed event the exclusion will take is the release of a press it has just removed — same button, same point, immediately after — and nothing else, because an event it cannot place is not shown to belong to anybody. App switches are matched by NAME, because a switch carries no process at all; and removing one leaves its sampled RETURN naming a window that never left the front, which `simplify.foldActivates` would collapse only AFTER deleting the demonstrated step beside it as the gesture that performed the switch — so the draft path collapses it first. `simplify` itself is unchanged and `LearnAuto` keeps the behaviour it shipped with. Finally, record, review and save are three acts: `Record` returns a draft and writes NOTHING, Cancel is dropping it, and `Save` is the only thing that touches the store and takes the scope it is GIVEN, refusing an application scope with no application rather than quietly widening it to everywhere."
supersedes: []
affects:
  - demonstrations
  - recorder
source_paths:
  - internal/orchestrator/draft.go
  - internal/orchestrator/orchestrator.go
  - internal/recorder/recorder.go
  - internal/recorder/recorder_windows.go
  - internal/winctx/winctx_windows.go
---

# Ending a recording is not part of the recording

## The ask

> We need a cleaner way to end basic recordings like this than f7, I prefer a button to end
> recording, simplify can chop off unnecessary time

and, settling the open question:

> Studio needs a visible "Stop recording" button. F12 can remain an optional shortcut, not the
> only way to finish. … The Studio click used to stop recording must **not** become part of the
> macro. Test that, including bringing Studio forward to press it.

The second half of the owner's sentence was already true before any of this: `trimEdgeWaits`
drops the leading and trailing waits, so the dead time at either end of a recording never
reached a route. What was missing was the button, and everything the button turns out to imply.

## What a button implies that a key does not

A stop KEY is pressed wherever the person already is. A stop BUTTON is in a window, so pressing
it means **bringing that window forward and clicking in it** — four or five events of pure
navigation at the end of every recording, every one of them in the recorder's own window.

So the tail comes off. That part was expected. Four things were not.

### 1. The poller cannot answer about an instant it did not sample

`EvAppSwitch` and `RecordedEvent.App` both come from an app poller that samples the foreground
every 120ms and emits a switch only when the sample CHANGES. A fallback for "the stop happened
too fast for the poller" that then consults *that same poller's last sample* is asking the one
witness that cannot have seen it.

`RecordedEvent.PID` is the answer: read in the hook at the instant of the event, from
`GetForegroundWindow` + `GetWindowThreadProcessId` — two window-data reads, which is what keeps
them legal on the hook thread. `QueryFullProcessImageName` would open a handle and is the kind
of call that makes Windows drop the hooks.

### 2. A button is activated on RELEASE

The builder stamped the process on button-down only, alongside the window-relative geometry. A
strip walking backwards from the end of the stream therefore met an unattributed release first
and stopped there, never reaching the perfectly-attributed press behind it. Both edges carry the
process now; geometry stays on the press alone, because a release lands wherever its press did
and a second offset would be a second place to replay from.

### 3. A known process outranks a sampled name in BOTH directions

The process rule stops walking at a click with a known non-owner process. The name rule, running
after it, could still delete that click because its stale `App` tag said `studio`. Two rules,
each correct alone, the second undoing the first — and the event it undid is the one with the
best evidence on it.

### 4. The exclusion cannot be only at the tail

A window with buttons on it is a window a person glances at while demonstrating. A glance leaves
a switch in the MIDDLE of the stream, and `simplify.foldActivates` drops the step immediately
before a switch as the gesture that performed it — a taskbar click is navigation, not
demonstration, and that rule predates all of this and is right. So an unshielded click made just
before looking at the recorder is read as the press that brought the recorder forward, and is
lost. The same switch also decides which application the routine is filed under, and therefore
whether it is ever offered.

Hence `withoutOwner`: the owner's events come out wherever they are, because interacting with the
recorder is never the thing being demonstrated.

### 4b. And removing a switch leaves its return behind

This is the correction that arrived last, and it is the ordinary case rather than the exotic one.

When the poller samples both sides of a visit, the stream carries a switch INTO the recorder and
a switch back OUT. Excluding the first leaves the second announcing a switch to the window that
never left the front — and `foldActivates` does its two jobs in the order that loses the
demonstration: it drops the step before an activate as the gesture that performed the switch, and
only THEN collapses an activate that re-focuses the app already in effect. The redundant return
takes the demonstrated click beside it on the way out and is then removed itself, leaving no
trace of what it cost.

`withoutRedundantReturns` does that collapse first, in the draft path, before `simplify` runs.
`simplify` is untouched.

This hid because the fixture omitted the return and emitted the owner's app twice with nothing
between — a stream the production poller cannot produce, since it emits only on change. The rare
timing (the person clicks within one poll of returning, so no return is sampled) passes with the
normalization deleted. Both timings are now fixtures, and three of the four mutations below kill
only on the ordinary one.

## And Stop is not Save

`Learn` and `LearnAuto` record and save in one call. That is right for a terminal and for a HUD
with one key. It is wrong for a window, because it collapses three decisions a person expects to
make separately:

| Stop   | end the capture and SHOW ME what you got |
| Cancel | throw it away                            |
| Save   | keep it, as this                         |

Routing a Stop button through `LearnAuto` would make Stop and Save the same event by
construction — there would be nothing to cancel, because it would already be on disk.

So `Record` returns an unsaved `*Draft` and writes nothing; Cancel is dropping that draft; `Save`
is the only thing in the path that touches the store. And `Save` takes the scope it is GIVEN
rather than inferring one, because a window can ask the question. An application scope with no
application is REFUSED: a routine quietly rescoped to everywhere works in places the caller was
told it would not, which is the larger harm.

## What this does not change

- **`simplify` is unchanged.** Every path that excludes no window sees exactly the stream it saw
  before.
- **`LearnAuto` keeps its shipped behaviour.** It strips only the tail, so a mid-visit glance
  still leaves its switch in the stream and the click before it still goes. That is a real
  limitation of the legacy path, recorded rather than fixed: Studio's recording goes through the
  draft.
- **`foldActivates`' own rule stands.** A click immediately before a deliberate switch away is
  still read as the gesture that performed it. The draft path removes the owner's switches; it
  does not teach that rule to tell a taskbar press from a demonstration.
- **The strip's own tests shield their click with a keystroke** to isolate what they are about.
  That is acceptable for them and it establishes nothing about click-only behaviour, which is why
  the draft's acceptance case has no keystrokes anywhere.

## Enforced by

- `internal/recorder` `TestBothEdgesOfAClickCarryTheirOwner`,
  `TestTheAppTagCanDisagreeWithTheProcess`, `TestOnlyThePressRecordsWhereItLanded` — driven
  through `newClick`, the production event builder, with the poller still on the game. These
  exist because a test that builds its own events cannot catch a builder that stamps less than
  the fixture assumed, and twice it did not.
- `internal/orchestrator` `TestTheClickThatStoppedTheRecordingIsNotInTheMacro`,
  `TestAStopInsideTheSampleIntervalIsStillCut`, `TestTheProcessBoundaryHoldsWithNoOwnerName`,
  `TestAStaleNameDoesNotDeleteAClickTheEventPlacesInTheGame`,
  `TestAnUnattributedReleaseDoesNotHideThePressBehindIt`,
  `TestAnUnownedEventAfterAnOwnerClickIsNotAssumedToBeTheirs`,
  `TestTheStripKeepsWhatItCannotPlace`, `TestAGestureEndedRecordingIsNotStripped`.
- `internal/orchestrator` `TestStopEndsTheCaptureAndWritesNothing`,
  `TestCancelLeavesNothingBehind`, `TestSavePersistsTheDraftUnderTheScopeItIsGiven`,
  `TestSaveRefusesAnAppScopeWithNoApp`, `TestSaveRefusesADraftWithNoSteps`.
- `internal/orchestrator` `TestAnUnshieldedClickOnlyDemonstrationSurvivesAGlanceAtMarco` — the
  acceptance case, click-only and unshielded, run against BOTH poller timings, asserting both
  clicks, no Stop click, no Activate naming the recorder, the game as the draft's application,
  and a Save under the stated game scope with the whole demonstration in it.
- `internal/orchestrator` `TestWhatTheExclusionWillNotClaim`, `TestTheExclusionIgnoresAStaleName`,
  `TestTheReturnNormalizationKeepsARealSwitchBack` — the boundaries, driven directly, because no
  draft fixture produces the shapes they are about.

## Mutations

- Four on the return normalization, all killed: the pass made a no-op; the app in front never
  remembered; the comparison inverted so only redundant switches survive; and the call site
  reverted to the shipped `withoutOwner` alone. Three of the four kill ONLY on the
  both-sides-sampled timing, which is the measurement that the old single fixture could not have
  caught this.
- Eight on the exclusion, all killed. Three died through `Record`; the release-match clause
  needed its four shapes driven directly, because no draft fixture produces an unowned release at
  a different point — the whole clause could have been `true` and the acceptance tests would
  still have passed.
- Four on the strip's ownership precedence, all killed — but only after the first version of that
  test killed NONE of them: every stream in it ended with a complete owner click pair, so the
  predecessor of the release was always another release and the matching clause was never
  reached. Rewritten so each stream ends with the owner's press.

Each of the five corrections in this ADR has the same shape: **the fixture was easier than
production.** A poller sample that could not be stale, a release that carried what only a press
carries, a name that agreed with its process, a glance with no return, a stream with no clicks in
it. Mutation testing found four of the five; reading found none of them.

## Related

[[ADR-065-operating-marco-is-not-demonstrating-to-it]] decided this principle two months earlier,
and is the reason this ADR is worth reading twice. The sentence is the same — *the surface is in
front, and it is not the subject* — and the Director's overlay has held it since August. The
widget recorder underneath never got it, because it had no surface of its own to press: it ended
on a key, pressed wherever the person already was. Giving it a button gave it a surface, and the
whole class of defect arrived at once, in a stack that shares nothing with the overlay's —
process ids read in a hook, a 120ms sampler, `simplify`'s gesture rule. A decision recorded in
one subsystem is not a decision enforced in another.

[[ADR-066-stop-is-a-product-event]] is the same change on the Director side: the person is the
only reliable source of "I am done." The button is that, for the recorder.

[[ADR-248-replaying-a-demonstrated-layout]] is the mode these recordings are replayed in.
[[ADR-086-one-acquisition-one-word-one-request]] is the acquisition this enters beside.
