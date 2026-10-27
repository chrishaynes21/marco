---
type: adr
status: accepted
date: 2026-10-26
decision: "ACCEPTED — A GUARDED DRAG IS TWO POINTS IN ONE WINDOW, MEASURED AT BOTH ENDS AND RESOLVED FROM ONE ANSWER. A drag already worked and was already one `do` sentence; what it lacked was a measurement, so `codegen.cannotPropagate` refused to SAVE any recording containing one in a guarded routine — truthfully, and with a sentence that described the recorder rather than the gesture. The recorder now measures BOTH EDGES of a click: a release used to carry no geometry at all, which was right while the only two-edged gesture was a click, whose release lands where its press did. A drag ends somewhere else, and the alternative — subtracting the PRESS's rectangle from a point captured later — is an offset into the window as it WAS attributed to a point from after, with nothing able to tell whether the window moved in between and no way for replay to notice. Each edge also carries the WINDOW HANDLE and its owner and the rectangle's ORIGIN, as recording evidence that never reaches a route and is never a replay target: geometry and a program name cannot say WHICH window, because two windows of one executable at one size on one display satisfy every comparison of those and are two different windows. `simplify` then has six outcomes and they are six different recordings: both edges agreeing gets the full contract; two different windows, a window that MOVED, or one that was RESIZED between the edges REFUSES the recording with a sentence naming which; a drag that ended OUTSIDE the window refuses with its own sentence, because that is something the person did and can do differently; a term the machine would not report gets the BASIS ALONE and no sentence, because telling somebody to demonstrate it again cannot help when the program had exited, and Save names the missing term instead; and a drag that declared no window at all is untouched and runs exactly as every drag saved before this does. The `Drag` set gains nine scalar fields in the host module — `FromRelX`, `FromRelY`, `ToRelX`, `ToRelY`, `RelTo`, `WinW`, `WinH`, `Dpi`, `Process` — with ONE set of window terms for both ends, because they are two points in the same window and a second set would invite a gesture to be checked against two. At replay both ends resolve from ONE `confirmWindow` answer: asking the desktop twice would let a window that moved between the answers relocate the destination, so the gesture would start where it was demonstrated and end somewhere nobody chose, each half correct against a different rectangle. AND THE WINDOW IS ASKED AGAIN IMMEDIATELY BEFORE THE BUTTON GOES DOWN, against the SNAPSHOT those two points were computed for and not merely against the recorded terms — because between resolving and pressing the gesture AIMS, an injected pointer move changes applications, and `confirmWindow` compares the live window with what was DEMONSTRATED while nothing was demonstrated about where the window would be. A change there is a REFUSAL and not a re-base, because re-basing would move the gesture after it had been planned. The order is the stop, the window, THE STOP AGAIN, then the press, with no input sent anywhere in that sequence: the confirmation reads a rectangle, a scale and a process name off a live desktop and takes time of its own, so a stop arriving while it ran found nothing between its own answer and the button going down. Whatever is last before the press is what has to be read there, including a check that was itself the fix for the instant before. Cleanup is a DEFER inside the gesture and needs no ledger and no split backend: a panic unwinding past it runs the deferred release, because the gesture is still on the stack while the unwinding happens — and WHAT IT ARMS ON IS AN ACCEPTED PRESS, not entry, so there are four states (nothing accepted, down and a release owed, released, refused) and a panic during the AIM reports nothing. A stuck button is reported through a CALLBACK and not a return value, because the path it exists for does not return. Forced termination stays outside all of it: a ledger lives inside the runner, and nothing in a terminated process runs. The fixture logs movement ONLY while a button is down — the travel of a drag, bounded by the gesture, and the one part of it no account outside the application can see — counted from the edges the window RECEIVED rather than read from the mouse's state now. Acceptance requires A RECEIVED POINT THAT IS NEITHER END and endpoint ORDER and never a COUNT, because Windows coalesces mouse moves and a number would be an assertion about the scheduler — and 'a move arrived' is weaker than it reads, since a replay that jumped to the far end and let go there sends a move of its own, at the destination. A STOP IS ORDERED AGAINST THE RELEASE BY THE FIXTURE'S OWN COUNTER, read on both sides of the request, because an outcome word is what Marco concluded and not when the application was told: read as a timestamp it excuses a stop that was IGNORED and convicts a drag nothing interrupted. So a release logged before the request is NOT RUN, one that falls inside the bracket is AMBIGUOUS and reported as neither, and an ignored stop is a finding of its own. A stopped drag's release must be ON THE TRAVEL, inside the demonstrated ends and on the line between them, because a release at a point the pointer never visited is not 'where it got to' either. And a refused release stays a headless case rather than a real button left down on somebody's desktop."
supersedes: []
affects:
  - execution
  - demonstrations
  - recorder
  - interface
source_paths:
  - internal/recorder/recorder.go
  - internal/recorder/recorder_windows.go
  - internal/macroir/step.go
  - internal/simplify/simplify.go
  - internal/codegen/codegen.go
  - internal/osmod/os.marco
  - internal/oshost/oshost.go
  - internal/oshost/drag.go
  - plugins/fixture/Fixture.cs
  - plugins/e2e/scenario_drag.go
---

# A guarded drag is two points in one window

## What was actually missing

A drag worked end to end before any of this: captured, simplified into `StepDrag`, generated as
`do OS's Drag with drag1`, performed by `oshost.doDrag`. What it did not have was a **guard** — and
the tree said so, in one line of `codegen.cannotPropagate`:

> a DRAG resolves no offset, so it carries no window and is checked against nothing.

So `Unpropagatable` refused to SAVE any guarded recording containing a drag, by name, before
anybody pressed Save. The state was honest: a drag was either in an unguarded routine or refused.

That sentence turned out to be a statement about the **recorder** rather than about the gesture. A
drag is already ONE `do` sentence, which is exactly the property the propagation chain requires
and the reason a non-left click and an anchored press cannot join it. What it lacked was a
measurement.

## Both edges are measured, and a gesture across two rectangles is refused

The release edge recorded no geometry at all. `newClick` stamped the basis, the shape and the
offset only `if down`, and that was right while the only two-edged gesture was a click: a release
lands where its press did, so it is about no rectangle of its own.

A drag ends somewhere else. The first design derived the destination's offset from the **press's**
rectangle, and that is an unchecked inference — an offset into the window as it WAS, attributed to
a point captured after, with nothing able to tell whether the window moved in between. It would
pass every check at replay and localise the drop against a rectangle that had gone.

So both edges are measured where they happened, and each carries two things a route never sees:

- **`WinX`, `WinY`** — where the rectangle was. An offset cannot answer "is this the same
  rectangle", because an offset is what is left once the origin has been subtracted away: two
  offsets agree happily about a window that moved between them and describe a gesture no static
  window can reproduce.
- **`HWnd`** (with `PID` beside it) — **which** window. Geometry and a program name cannot say:
  two windows of one executable, the same size at the same scale, owned by the same process,
  satisfy every comparison of those and are two different windows. Identity is asked BEFORE the
  describing terms for exactly that reason.

Both are **recording evidence and never replay targets**. Nothing emits them into a route and
nothing resolves against them; a saved absolute origin is the thing the whole window-relative
design exists to avoid.

The hook cost of the second measurement was measured rather than asserted: `clickShape` is
100.5 ns/op on the development machine and both edges of a click through `newClick` are 809.5
ns/op together, against `LowLevelHooksTimeout`'s 300ms. Kept as a benchmark so the next person
adding work to that path re-runs it instead of reasoning about it.

## Six outcomes, because they are six different recordings

`simplify`'s `dragIn` is the one place that decides, and each outcome has to be distinguishable by
whatever reads the step:

| the recording | what the step gets | and the person is told |
|---|---|---|
| both edges agree about one window | the full contract: both offsets, one set of window terms | nothing — it saves and runs |
| two different windows | the basis alone | the recording is REFUSED: "starts in one window and ends in another" |
| the window MOVED between the edges | the basis alone | REFUSED: "the window moved while this drag was being demonstrated" |
| the window was RESIZED or rescaled | the basis alone | REFUSED: "changed size or scale … what is inside it moved" |
| the drag ended OUTSIDE the window | the basis alone | REFUSED: "ends outside the window it started in" |
| a term the machine would not report | the basis alone | NO sentence. Review names the missing term; Save refuses |
| neither edge declared a window | nothing | nothing — it runs exactly as every drag saved before this |

The split between the last two of the refusing cases is the one worth stating. A gesture across two
rectangles, a window that moved, a resize and a drag out of the window are all things the **person
did**, and each one has an instruction that could help. A scale Windows would not report or a
program that had exited is not; telling somebody to demonstrate it again would be telling them to
do something that cannot change the answer. So those get the basis and silence, and the review
surface names the term.

And the basis is kept in every refusing case, which is [[ADR-251-a-press-checks-the-window-it-was-recorded-in]]'s
rule unchanged: a drag that asked to be checked and cannot be must not quietly become an ordinary
drag, which runs at its recorded coordinates having checked nothing.

## One measurement, two points — and asked again at the press

Nine scalar fields on the `Drag` set in `os.marco`. Four of the names are new; five mean what they
mean on a `Point`. No Core change and no new act: nesting a `Point` inside a `Drag` is a language
change, since set fields are scalars, and `do X's Act with Y` takes one argument. `os.marco` is the
host module, which is where `Point`'s own window fields went for ADR-251.

**ONE set of window terms for both ends.** They are two points in the same window, so there is one
rectangle, measured once. Carrying a second set would be inviting a drag to be checked against two
windows, which is not a gesture. What makes one set honest is that `simplify` has already required
both edges of the demonstration to agree about it.

At replay both ends come out of one `confirmWindow` answer. Asking the desktop twice would let a
window that moved between the answers relocate the destination: the gesture would start where it
was demonstrated and end somewhere nobody chose, each half correct against a different rectangle —
the hazard this boundary exists for, wearing the boundary's clothes.

**And the window is asked again immediately before the button goes down.** Not belt-and-braces.
Between resolving and pressing, the gesture AIMS, and an injected pointer move changes
applications: a hover, a highlight, a tooltip, a panel that lays itself out differently, a window
that resizes to fit what is now under the cursor. A check made before the gesture started is a
check made before the gesture's own first event.

The second answer is compared against the **snapshot**, not just against the recorded terms.
`confirmWindow` compares the live window with what was DEMONSTRATED, and nothing was demonstrated
about where the window would be — so it passes a window that has moved since the two ends were
computed from its origin. Only the snapshot notices.

A change there is a **refusal and not a re-base.** Re-basing would move the gesture after it had
been planned against those two points, which is the same wrong answer by a different route: a drag
lands where it was demonstrated or does not happen.

The order is: the stop, then the window, **then the stop again**, then the press — with no input
sent anywhere in that sequence. The stop comes first because a run that has been stopped has no
business asking the desktop anything, and a window complaint is the wrong answer to give somebody
who pressed Escape.

**And the stop is read once more after the window answers**, which the first cut did not do and
review found. The confirmation is a read and sends nothing, which is why it is allowed to sit
between a stop check and the press at all — but it reads a rectangle, a scale and a process name
off a live desktop, and a stop arriving while it does that found nothing between its own answer
and the button going down. The window was still there, the callback said so, and the press went
out on a run that had already been stopped. It is the aim's defect one question later, and the
rule it proves is the general one: **a check belongs against the press, so whatever is last before
the press is what has to be read there** — including a check that was itself the fix for the
instant before.

## Cleanup needs a defer, and neither a ledger nor a split backend

The first proposal reached for both. Neither is necessary, and the reason is worth writing down: a
panic unwinding past the gesture runs its deferred code, so the release is deferred and the button
comes up on the way out. Nothing outside the gesture has to release, because the gesture is still
on the stack while the unwinding happens.

**What it arms on is an ACCEPTED PRESS, not entry into the gesture.** Prewriting "a button may be
down" on the way in reports a stuck button after a panic during the AIM, when nothing was ever
pressed — and a warning that fires when nothing happened is worse than none, because it teaches
people to ignore the warning. So there are four states, each with its own cleanup and its own
report: nothing accepted, down and a release owed, released, and refused. Two would be wrong in
both directions: on the way in it over-reports, and "pressed" alone cannot tell a release that
worked from one the machine refused.

One release path, `letGoNow`, and every exit goes through it including the deferred one. It is
idempotent by reading the state rather than by a second flag, so a gesture that already released
does not send a release for a button that is not down — which would be a second refusal and a
sentence naming two failures for one.

A stuck button is reported through a **callback** and not a return value, because the path it
exists for does not return: a panic skips `doDrag`, so `delivered` never runs and the gesture's
error reaches nobody. The sentence is written from inside the gesture by code still on the stack.

What a ledger would actually buy is the route-end sweep's SECOND attempt after a first refusal.
That is worth having and is not needed here. **Forced termination stays outside all of it**: a
ledger lives inside the runner, so it reaches ordinary returns and the unwinding where deferred
code still runs, and nothing in a terminated process runs at all. Recovering from a kill needs an
owner that outlives the runner.

## The fixture logs the travel, and the acceptance stays inside what it can see

A drag is not its two ends — those are two clicks — and the travel between them is the one part of
the gesture no account outside the application can see. So the fixture logs movement, and **only
while a button is down**: a replay aims before every click and a person moves the pointer
continuously, so logging all of it would bury the presses in thousands of records and stop the log
being evidence.

Whether a button is down is counted from the edges the window RECEIVED, not read from
`MouseButtons` — that is the state of the mouse now, which is after the instant being logged, and
a witness answering about a later instant is the defect the whole harness exists to prevent. A
count rather than a flag, because two buttons can be down at once; floored at zero, because one
release seen without its press would otherwise make every later move invisible.

What the scenarios deliberately do not assert, and what review found they had to:

- **Not every point, and no count.** Windows coalesces mouse moves — a message is generated from
  the queue's current position when somebody asks for one — so a gesture that sent twenty-four
  moves can arrive as six, and how many depends on how busy the machine is. A scenario asserting a
  number would be asserting the scheduler and would fail on a loaded machine with nothing wrong.
  What is required is **a received point that is neither end**, and the ends in order around it.
  "A move arrived" is weaker than it reads, which review found: a replay that jumped to the far
  end and let go there sends a move of its own, at the destination, and satisfies any reader that
  only counted them. The travel is the part BETWEEN the ends, and that is what no count can stand
  in for. The same log has a second reading the oracle cannot separate — an application that never
  ran during the travel and was told the pointer's position once, at the end — so the failing
  sentence names both, because one of them is a reason to look at the machine and not at Marco.
- **A stop that arrived after the release is NOT RUN**, not a pass — and what decides it is the
  stop's place in the fixture's own counter, read on **both sides of the request**. Review found
  the first cut inferring that from the outcome word and the existence of movement, and neither
  orders anything: an outcome says what Marco concluded, not when the application was told. Read
  as a timestamp it fails both ways — a stop Marco IGNORED finishes as `performed` and is excused
  as "too late", which files the most serious thing this scenario can find as "nothing happened";
  and a stop that landed after a completed gesture still records `cancelled`, which convicts a
  drag nothing interrupted of dropping at its destination. So the press has to be logged before
  the request and the release after it, an ordering that falls inside the request is reported as
  **ambiguous** rather than counted either way, and an ignored stop is a finding with its own
  sentence.
- **"Anywhere but the destination" is not "where it got to."** The other half of the same review:
  a release at a point the pointer never visited passed, and "it let go somewhere else" is not the
  claim. The release has to be ON the travel — inside the two demonstrated ends and on the line
  between them, to within the band the gesture's own integer stepping produces and no wider. What
  the oracle cannot say is WHICH point of the travel the pointer had reached when the stop landed:
  coalescing means the log does not name the pointer's last position, and the instant a stop
  arrives is not observable from outside the run. So the passing sentence claims an order and a
  place, and not an instant.
- **No exemption is applied before a held button is reported.** The claims above are each true or
  false of a log on their own, and review found the ORDER of them carrying a defect of its own: a
  log with a press and nothing else has no movement in it, so the scope exemption answered first,
  filed the run as NOT RUN, and never asked whether the button came up. A left button held down on
  somebody's desktop is the worst thing this scenario can find and it was the one thing it could
  not report. So the held button is asked FIRST — above both exemptions, since an unestablished
  ordering would have swallowed it exactly as the travel check did — and it is silent about a log
  with no press in it, which is what lets it be asked there at all. The composed decision lives in
  one function so that its order can be tested; this is the third time this slice has found a
  defect behind its own excuse, which is why the composition is now a claim and not a list.
- **No stuck button is manufactured on a desktop.** A release the machine refuses is arranged
  headlessly, where a machine can be told to refuse. Doing it on a desktop would mean leaving a
  real mouse button held down on somebody's computer to see whether the report says so.

**Physical recording capture is not exercised and cannot be.** The hooks discard injected input,
so every scenario seeds a demonstration and plays it. What they establish is that a drag ARRIVED
as one.

## What this does not change

- **Existing saved Marco.** A saved `the drag1 is a Drag with FromX … Button "left"` has no
  `RelTo`, so it takes the legacy path exactly as it did: four ints, absolute, no window question
  asked. Nothing rewrites anything; explicit regeneration picks the guarded form up.
- **Non-left clicks and anchored presses** still cannot propagate, for their own reasons, which
  are unchanged and still listed in `cannotPropagate`.
- **A repeated block** still cannot, which is the one thing a stop cannot survive.

## Enforced by

- `internal/recorder` `TestBothEdgesOfAClickRecordWhereTheyLanded` — both edges measured, with the
  handle and the origin. It replaces a test that asserted the opposite and PASSED after the change,
  because its release coordinate fell outside the window the fixture stood in front, so the
  containment test declined the offset and the claim held by accident.
- `internal/recorder` `TestOnlyThePressRecordsTheWindowsTitle` — the half of that claim which
  survives, and about a different field for a different reason: `Window` is an anchor's context
  check, resolved at the press.
- `internal/recorder` `TestOnlyWhatDeclaredAWindowCarriesTheProgramName` — the program lookup
  follows the BASIS and not the edge, which is what it always did; the old name described the
  stream rather than the condition.
- `internal/recorder` `BenchmarkClickShape` / `BenchmarkNewClickBothEdges` — the hook cost as a
  number, never asserted. A timing assertion in the default suite is a flake on a busy machine.
- `internal/simplify` `TestADragCarriesTheWindowItsEdgesAgreeAbout` — all seven outcomes, each
  with the step it produces and whether the recording is refused. Mutation: dropping the identity
  comparison, the origin comparison, the shape comparison, the out-of-window case, or the basis in
  the unmeasurable case each fails exactly one sub-case.
- `internal/codegen` `TestAMeasuredDragCarriesOneWindowAndBothEnds` /
  `TestADragThatCouldNotBeMeasuredStillSaysWhichWindow` — the emission, inside the package,
  because they were written while Save still refused a guarded drag.
- `internal/codegen` `TestAnUnmeasuredDragIsUnchanged` — the compatibility claim, generated,
  compiled AND run.
- `internal/codegen` `TestAMeasuredDragIsAGuardedLink` — the switch, asserted on the shape of a
  LINK (`do … ...` followed by `when ok?`) rather than on the drag being in the route: a drag
  written as an ordinary step inside a chain has the same words, performs the same gesture, and
  everything after it runs whether the gesture happened or not.
- `internal/codegen` `TestAStepThatCannotStopTheRoutineIsRefused` — the two drags that stay
  refused, beside the right-click.
- `internal/oshost` `TestAGuardedDragResolvesBothEndsAgainstTheLiveWindow` — both ends re-based,
  against a window that is not where the drag was demonstrated, so "resolved the offset" is
  distinguishable from "used the coordinate".
- `internal/oshost` `TestADragIntoADisagreeingWindowSendsNothing` /
  `TestADragMissingATermIsRefused` / `TestADragEndOutsideTheWindowIsRefused` — the press's cases
  asked of a drag, both ends, with the refusal arriving before the machine is touched.
- `internal/oshost` `TestAnUnguardedDragIsUnchanged` — the compatibility claim and the control for
  all of the above: a host that refused every drag would satisfy them.
- `internal/oshost` `TestAGuardedDragIsCheckedAgainBeforeTheButtonGoesDown` /
  `TestAWindowThatMovedBeforeTheButtonWentDownRefusesTheDrag` — the revalidation and that it is
  compared against the snapshot. Removing it, or comparing only the recorded terms, each fails.
- `internal/oshost` `TestADragRefusedAtTheLastInstantPressesNothing` /
  `TestNothingIsSentBetweenTheWindowCheckAndThePress` — the gesture's own half, asserted on an
  ORDER: a check followed by an injected event is a check made before that event.
- `internal/oshost` `TestADragStoppedWhileItAimsDoesNotPress` /
  `TestADragStoppedWhileTheWindowIsCheckedDoesNotPress` /
  `TestADragStoppedBeforeItAimedAsksTheDesktopNothing` — the three instants between the entrance
  and the press, one test each. The middle one is review's finding: a confirmation that cancels
  the run and then AGREES, which is the narrowest arrangement of it. The last holds the other
  side — a run already stopped asks the desktop nothing — which is why the stop is read before the
  window as well as after it.
- `internal/oshost` `TestAPanicBeforeTheButtonWentDownSaysNothing` /
  `TestAPanicDuringTheTravelStillLetsGo` / `TestAPanicWhoseReleaseIsRefusedSaysSo` /
  `TestAnOrdinaryDragReleasesOnce` — the four cleanup states, and the control that holds the
  deferred release to exactly one.
- `plugins/fixture` `TestTheFixtureLogsMovementOnlyWhileAButtonIsDown` — the gate, the count, the
  floor, and that the message filter actually calls it. It lives beside the C# because
  `plugins/e2e` is a module of its own and `go test` caches straight past a change to a file
  outside the module it is testing — measured: the gate was deleted and the suite said
  `ok (cached)`.
- `plugins/e2e` `TestTheHarnessReadsAMoveRecord` — the reading half, through the production record
  type and its printer.
- `plugins/e2e` `TestTheDragScenariosAreGatedUntilTheyHaveRun` — the arrangement, including that
  no other family's `-only` selector sweeps them up. It FAILED on the first names, which contained
  "window", and it asks about every registered family now rather than the one that was obviously
  a risk.
- `plugins/e2e` `TestTheGestureShapeIsAskedOfTheWholeLog` /
  `TestTheTravelIsRequiredAndNotCounted` / `TestEveryPointOfTheGestureIsHeldInsideTheWindow` /
  `TestARefusedDragIsAssertedOnNotOneEvent` — what three of the four scenarios' verdicts rest on,
  without a desktop. The travel one carries review's case: one move at the destination, and one
  that never left the press, are both teleports.
- `plugins/e2e` `TestTheStopIsOrderedAgainstTheReleaseByTheFixturesLog` /
  `TestAnIgnoredStopIsReportedRatherThanExcused` /
  `TestAStoppedDragThatNeverTravelledIsOutOfScope` — the ordering, in its five readings, and the
  two things that must NOT come out as NOT RUN: a stop that was ignored, and a button that never
  came up.
- `plugins/e2e` `TestTheReleaseHasToBeOnTheTravelAndNotAtItsEnd` — both halves of where a stopped
  drag may let go, including a release at a corner of the box the ends make, which the first cut
  passed.
- `plugins/e2e` `TestTheHeldButtonIsReportedBeforeAnyExemption` — the composed decision, which is
  where the order of those claims lives and where review found the next defect: a press with
  nothing after it must name the missing release, and the two exemptions must still exempt a
  release with no travel, a drag that had already finished, and a log with nothing in it.
- `plugins/e2e` `TestTheStoppedDragScenarioBracketsItsStop` — that the scenario TAKES that
  evidence, read off the source the way the hold scenario's absence claim is: the lower bound
  after the press arrives, the upper bound after Stop is pressed, the composed decision reached
  after both, and BOTH dispositions downstream of it — a scenario that called only `fail` would
  turn every NOT RUN into a failure, and one that called only `skip` would turn a held button into
  nothing happened. A bracket taken at the wrong moment is not a bracket, and no headless test can
  run that function body.
- `plugins/studio` `TestADemonstratedDragIsOfferedAndSaved` /
  `TestADragWhoseWindowMovedIsRefusedWithTheReason` /
  `TestADragThatEndedOutsideTheWindowSaysSo` / `TestADragMissingAMeasurementIsNamedAtReview` —
  the review, through the production recorder, simplifier and generator, which is where a person
  finds out before the button.
- `plugins/e2e` `drag/a drag arrives as a drag`, `drag/it follows an app that moved`,
  `drag/a resized app is refused with nothing sent`, `drag/a stopped drag lets go where it got to`
  — **WRITTEN AND NEVER RUN.** Gated with `onlyWhenAsked`, for the holds' reason. Until they pass
  on a desktop, nothing here is established about what an application receives.
