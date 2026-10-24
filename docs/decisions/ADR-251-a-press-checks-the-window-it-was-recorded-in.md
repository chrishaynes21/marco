---
type: adr
status: accepted
date: 2026-10-23
decision: "ACCEPTED — A COORDINATE IS A CLAIM ABOUT A PLACE ONLY WHILE THE WINDOW IT WAS MEASURED IN IS STILL THAT WINDOW, AND THE PRESS IS WHERE THAT GETS CHECKED. A press records five terms at the instant it is made: the BASIS its offset is measured from, the window's OUTER size, the window's DPI as its own term, and the file name of the program that owned it. All five reach the saved route, because saved legal Marco is the only executable truth in this stack. The basis is recorded although there is one value today, so no route already on disk is reinterpreted the day a recorder measures from the client area instead. Each term is written only when it was MEASURED — a DPI nobody could read is absent, not 96 — and MISSING IS WHAT IT STAYS. The BASIS is the contract and everything else is a measurement, so `RelTo` is written for every press a measuring recorder makes whether or not the measurements arrive, and the saved file therefore says three things apart: no basis is LEGACY and pressed exactly as it always was; a basis with every term is GUARDED; a basis with a term missing is INCOMPLETE and refused, naming what it lacks. Which state a press is in does not depend on a measurement having succeeded. A failed measurement means guarded replay is UNAVAILABLE for that recording — never that its requirements become optional, and never that it quietly becomes an ordinary point, which is pressed at its recorded coordinate having checked nothing. Review says what was not measured and whether recording it again can change the answer; saving is allowed and is not quiet. The BASIS ALONE is the opt-in — requiring an offset beside it was a bypass — and it must be a basis this knows how to check. Replay binds the WINDOW during preparation, from `Activate`, AND THE PROCESS THAT OWNED IT THEN, because a handle is not an identity: two copies of one program share an executable name, so only the process id separates the window the route prepared from a second instance that inherited the handle. Before every press it re-reads THAT HANDLE: still a window, still in front, still that process, its origin, its outer size, its DPI, and the program that owns it NOW. And the offset must lie INSIDE the recorded rectangle, because origin-plus-offset is a point on the desktop and every other check can pass while the cursor leaves the window. Every comparison is EXACT and any disagreement REFUSES, naming the term; it does not fall back to the recorded coordinate, because that is pressing a remembered pixel on a desktop just shown not to match. The window is re-read before every press and never cached: the legacy origin snapshot answers about the FIRST press of a route and exists to survive a focus flicker this path detects directly. One resolver serves clicks, Moves and the recorded point a vision-free `Find` hands back, because those are three ways into one question and a guard with a bypass is not a guard."
supersedes: []
affects:
  - execution
  - demonstrations
  - recorder
source_paths:
  - internal/recorder/recorder.go
  - internal/recorder/recorder_windows.go
  - internal/macroir/step.go
  - internal/simplify/simplify.go
  - internal/codegen/codegen.go
  - internal/osmod/os.marco
  - internal/oshost/oshost.go
  - internal/winctx/winctx_windows.go
---

# A press checks the window it was recorded in

## What a recorded coordinate is actually a claim about

Not "press pixel 842, 274". That is what gets stored, and it is only a shorthand. The claim is
*press the thing that was at 842, 274 in a 1920×1080 window of arkascended.exe drawn at 144 dpi,
whose top-left was 120, 64.* Everything after the first four numbers used to be thrown away, so
the shorthand was all there was, and a replay had no way to notice that it no longer meant
anything.

[[ADR-248-replaying-a-demonstrated-layout]] granted the narrow licence to replay a demonstrated
point at all, and it came with the condition that the window be checked. That condition is what
this is.

## The five terms, and what each one answers

| `RelTo`   | which rectangle the offset is measured from        |
| `WinW`/`WinH` | how big the window was, on the OUTSIDE        |
| `Dpi`     | what scale it was drawn at                        |
| `Process` | which program owned it                            |

**`RelTo` has one value today and is recorded anyway.** An offset from the outer frame and an
offset from the client area are different numbers for the same pixel, and nothing in a file
holding `RelX 722` says which. Without it, the day a recorder starts measuring from the client
area is the day every route already saved is silently reinterpreted — a kind of defect this
project has now hit often enough to pay a word for.

**The size is the OUTER size**, because the origin is the outer origin: `GetWindowRect` is the
whole window, frame included. A client size beside an outer origin would be two rectangles
described as one, and a check on it would be checking a number that never meant what the offset
was added to.

**DPI is its own term and not something read off the dimensions.** An application that handles
`WM_DPICHANGED` relayouts every control inside a window whose outer rectangle never moves. Same
size, everything somewhere else. The tree being per-monitor DPI aware V2 does not help here — it
makes the coordinates physical, which is what lets the size check be exact, and says nothing
about whether the application re-laid itself out.

**`Process` is the file name, not the path.** The question is "is this still the same program".
The path answers "is this the same installation", which is a different question, makes a saved
routine useless on another machine, and writes somebody's directory layout into a file they will
share.

## Where each is read, and why there

The basis, the size, the DPI and the origin are read IN THE HOOK, at the press, from the window
the attribution already identified — one `GetForegroundWindow` per click, not two. Two reads
would be two instants, and the second could describe a window the first did not attribute, so
the process id and the geometry on one event would belong to two different windows.

The process IMAGE cannot be read there. It opens a process handle, which is the work that makes
Windows drop the hooks — the invariant in CLAUDE.md, learned the hard way. So it is resolved
once the hooks are down, from the process id that WAS read at the press, with one lookup per
distinct program rather than one per click. A program that has already exited resolves to
nothing, and nothing is what is recorded: filling it from the app poller's last sample would put
a guess exactly where the point is to have measured something.

## Absent is a different claim from a default

A DPI nobody could read must not be written as 96. A guard comparing against that would pass on
a machine where the real answer happened to be 96 and fail on every other, having checked a
number that was invented. The same for a program that had gone and a window whose size came back
zero. Each term is written only when measured.

None of them is written without the basis, though. Checking a window's size while having no idea
which of its rectangles the offset belongs to is checking the easy half.

## Three saved contracts, and the basis is what states them

The first version of this drew the conclusion that a term the DEMONSTRATION lacks is not checked
at replay, because there is nothing to compare it against, and refusing would refuse a routine for
the crime of having been recorded honestly. **That is wrong, and it was wrong in two successive
ways.** Both corrections are worth writing down, because the second one is the subtler mistake and
it looked like the fix for the first.

**The first mistake: a check skipped for want of a term.** It makes the guard's strength depend on
how much the recording happened to capture, and *nothing in the route, the log or the review says
which checks did not happen.* A press with no recorded DPI passed on a desktop redrawn at another
scale. A press with no recorded program passed in a window belonging to something else entirely.
The log line said `click: guarded` for both. That is worse than no guard, because the route reads
as checked.

**The second mistake: making the RECORDING degrade instead.** The basis went in only when every
term arrived, so a press that could not be fully measured was saved as an ordinary point. That
reads as conservative and is the same hole one layer earlier: an ordinary point is pressed at its
recorded coordinate having checked nothing, so a window that would not report its size, a Windows
without `GetDpiForWindow` (before 10/1607), or a program that had exited before the hooks came
down each turned the safeguards off for a recording somebody was about to be offered. Preserving
old routines means preserving the behaviour of **routes already on disk**. It never meant letting
a new recording run unchecked because a measurement failed. A failed measurement means guarded
replay is UNAVAILABLE for that recording — not that its requirements become optional.

So the basis is the CONTRACT and everything else is a measurement:

| no basis                   | **legacy** — about no window; pressed exactly as it always was |
| basis, every term          | **guarded** — bound, re-read, compared exactly, then pressed   |
| basis, a term missing      | **incomplete** — refused, naming what it lacks                 |

`RelTo` is written for every press a measuring recorder makes, whether or not the measurements
arrive, because it says which rectangle the press is ABOUT and that is true either way. Each
measurement is written only when it was taken; missing stays missing, and nothing substitutes for
it. The three states are therefore distinguishable in the saved file, and which state a press is
in does not depend on a measurement having succeeded: the contract is stated, and the measurements
are what the contract is then judged against.

That re-defines `RelTo` slightly and for the better. It was "what the offset is measured from";
it is now "the rectangle this press is about" — which the offset is measured from and which the
press is checked against. One meaning, and it survives the offset itself being one of the things
that can go missing.

**The replay side refuses, exactly.** A stated basis requires every term: present, a whole number
in range, and a program named. Missing, malformed and out-of-range are three different recordings
and the refusal says which, because they are a truncated route, a hand-edited one, and a
hand-edited one again, and the person reading the sentence is the one who has to tell them apart.
There is no fallback.

**And unreadable is not matching, on the live side either.** A window that will not say what scale
it is drawn at, and a program that will not say what it is, were read as agreement for the same
reason as the first mistake — the comparison was conditional on both sides being known, so an
unknown side meant no comparison. Both refuse now.

## What a person is told, before they keep it

A draft may come back incomplete, and Studio's review says so rather than leaving it to be
discovered the first time somebody runs the routine. Three things, because fewer than three is not
actionable:

- **Beside the step**, what it cannot be checked against: `(1920×1080, 144 dpi, cannot be checked
  — which program owned it was not measured)`, with what WAS measured still shown, because that
  is half of what the person is deciding about.
- **Once, for the draft**, how much of it will not run and which measurements are absent.
- **Whether recording it again would help.** This is the part somebody actually needs, and it has
  two answers. A window that would not describe itself this time, or a program that exited before
  the recording closed, is a moment that passed: do it again with the thing running in front of
  you. A Windows with no `GetDpiForWindow` is not a moment — no press on that machine will ever
  carry a scale, so a routine that checks the window it was made in cannot be made there at all.
  Sending somebody back to re-demonstrate for an answer that cannot arrive is the worse of those
  two mistakes.

**Saving is still allowed.** The draft is a true record of what somebody did, and refusing to keep
it would lose the demonstration along with the measurement. What is not allowed is keeping it
quietly as something that runs: the Save message repeats that it will refuse, the trial journal
records it with "record it again" as the next step rather than "run it", and the route itself
refuses. An explicitly offered unguarded mode would be a separate product decision, and it must
not arise from an error.

## The basis alone is the opt-in

Entering the guard required a stated basis *and* a usable offset, which read as caution and was a
bypass: a point stating a window with no offset to resolve inside it missed the condition and went
down the legacy path, which presses the recorded coordinate having checked nothing. The basis
alone is the gate. Whether the rest of the terms are there is the guard's question, and its answer
is a refusal.

The same branch existed one layer earlier, in `codegen`: a point was written in window-relative
form when it had an OFFSET, so a press that declared the window and could not be measured in it
came out as a bare absolute point — byte for byte a route from before any of this existed. The
generator now branches on the basis too.

And the basis must be one this knows how to check. `RelTo` is compared against the one supported
value rather than against emptiness: a point measured from a control, or a screen, or a monitor is
different arithmetic, and resolving it as origin-plus-offset in a window would be answering a
question it did not ask — while falling through to the legacy path would press it unchecked.

## A handle is not an identity

The process id of the bound window is read at `Activate`, kept, and compared before the press.

The program NAME cannot do this job. Windows reuses window handles, and two copies of one program
have the same executable — so a handle inherited by a second instance of the game answers the
image-name check correctly, reports a window of the recorded size and scale, and is a window the
route never brought to the front. A press in it is a press in somebody else's game.

It has to be read at preparation time. Asked at the press it would be the identity of whatever
holds the handle by then, compared against itself: a check that cannot fail, which is the shape
every one of these findings has had. A window Activate found and could not name the owner of is
refused rather than passed through as a check that cannot run.

## And the press must land inside the window

Origin plus an offset is a point on the desktop, not a point in an application, and the window's
size is the only thing that says which. Every other check can pass — same program, same process,
same size, same scale, in front, alive — while the arithmetic sends the cursor out of the window
entirely, into whatever sits beside the target: the next application, the desktop, another
monitor. Underneath a guard that has just confirmed the target is exactly as demonstrated.

The offset is required to lie inside the RECORDED rectangle, which is the one it was measured in;
by the time the press happens the live rectangle has been shown to be the same size, so there is
no second rectangle to choose between. The near edges are inclusive and the far ones are not — an
offset of exactly the width is the first pixel outside a window whose columns run 0 to width−1 —
and a press at 0,0 or at width−1,height−1 is honoured, because that is where a maximised
application's own edge controls are.

## The terms are the opt-in

Nothing else turns the guard on. A point that carries them asks to be checked; a point that does
not is pressed by the rule it was saved with — the recorded absolute coordinate, or the cached
foreground origin after an `Activate`. Every route on the owner's machine was recorded by a build
that measured none of this, and all of them read and run exactly as they did.

This is deliberate rather than convenient. A flag would have been a second thing to get right,
and a route whose evidence and whose contract could disagree is a route where one of them is a
lie. The presence of the evidence IS the request for it to be checked.

## Binding, and the two instants

`Activate` hands back the window it settled on — `winctx.Bind` — and the host keeps the handle.
That is the preparation half.

It matters because "activate the app, then press inside it" is two instants and the desktop
belongs to somebody else in between. A dialog, an overlay or the person can change what is in
front. Re-asking "which window is in front" at the press answers a different question from the
one the route prepared, and an offset added to that window has been added to a window nobody
checked.

With the handle kept, the press asks about THAT window. Including who owns it now — window
handles are reused, so a window that answers every other question can belong to something else
by then, and everything it said would be true of that something else.

## Refusing, exactly, and never falling back

Every comparison is exact. A tolerance would be a number nobody measured: a window one pixel
narrower either has had something relayout inside it or has not, this cannot tell which, and the
honest answer is that the demonstration no longer describes the window. Same decision, same
reason, as ADR-248's window check.

Any disagreement REFUSES and says which term it was. It does not fall back to the recorded
coordinate. That fallback is right for a route that measured nothing — the coordinate is all it
has, it is exact for a fullscreen application, and the alternative is doing nothing — and it is
wrong here, because it would press a remembered pixel on a desktop just shown not to match. The
owner's standing instruction is the rule: *refuse rather than search elsewhere or guess.*

## The cache, investigated

The approved half-day question was whether the origin snapshot should stay. The answer is that it
stays for the legacy path and is not used by the guarded one, and the reason is what the cache is
FOR: it survives a momentary focus flicker that the legacy path has no way to detect. The guarded
path detects one directly, through `Front`, so it has no need to paper over it — and the cache is
actively wrong there, because it answers about the FIRST press of the route and is kept for the
rest of it, so a window moved halfway through sends every later press to the old origin.

So the guarded path re-reads before every press, which is what "immediately before input" means,
and the legacy path is untouched.

## One resolver, three ways in

A click, a `Move`, and the recorded point a vision-free `Find` hands back all resolve through the
same function.

- **`Move`** because `codegen` emits `Move` then `Click "right"`: a guard covering only the click
  would leave every press that is not left-handed unchecked, and the click that follows a Move
  takes the cursor's position, so the Move is the aiming.
- **`Find`** because with computer vision off there is nothing to match, so it short-circuits to
  the demonstrated point — and the route then clicks what it got back. That point is the press,
  one step later. Left alone it was the one bypass: the early return handed back a bare X,Y and
  the terms were dropped on the way out.

`anchorPoint` keeps its unguarded form on purpose. Its other job is to centre the search box when
there IS matching to do, and refusing there would refuse to go looking.

## Enforced by

- `internal/recorder` `TestAPressRecordsTheWindowItLandedIn`,
  `TestThePressMeasuresTheWindowItAttributed`,
  `TestAPressWithNoReadableWindowStillSaysWhichWindow`,
  `TestAnIncompletelyMeasuredPressKeepsItsBasis` (five desktops that answer partly) — driven
  through `newClick`, the production event builder. The second holds the one-handle rule by
  counting the shape reads and checking which window was asked about; the last two hold the basis
  against every way a measurement can fail.
- `internal/recorder` `TestEachPressIsNamedAndEachProgramAskedOnce`,
  `TestOnlyAPressCarriesTheProgramName`, `TestAProgramThatHasGoneLeavesTheNameMissing`,
  `TestAPressWithNoProcessIsNotLookedUp`, `TestAPressWithNoOffsetIsStillNamed` — the program
  lookup runs after the capture and follows the BASIS, not the offset: a press it cannot answer
  for keeps everything it did measure and loses only the name.
- `internal/simplify` `TestTheWindowAPressMeasuredReachesTheStep`, `TestTheClickPointIsThePress`,
  `TestAPressThatMeasuredNothingClaimsNothing`, `TestADragClaimsNothingAboutAWindow`.
- `internal/codegen` `TestAMeasuredPressCarriesItsWindowIntoTheRoute`,
  `TestAnUnmeasuredTermIsAbsentRatherThanDefaulted`,
  `TestWithoutABasisNothingAboutTheWindowIsWritten`, `TestALegacyPointIsGeneratedUnchanged`,
  `TestAnAbsolutePointIsUntouched`, `TestAPressThatCouldNotBeMeasuredStillDeclaresItsWindow` —
  each generates the route, CHECKS it compiles and RUNS it, because a set field the engine parses
  but will not assign fails at neither of the first two. The last one is the third contract: a
  press that declared the window and could not be measured in it writes its basis and nothing
  else, which is what keeps it from being byte-identical to a legacy point.
- `internal/oshost` `TestAGuardedPressLandsAtTheOffsetInsideTheBoundWindow`,
  `TestAGuardedPressRefusesEveryWindowThatIsNotTheOne` (six windows, one changed thing each),
  `TestAGuardedPressWithNothingBoundRefuses`, `TestTheWindowIsReadAgainBeforeEveryPress`,
  `TestAMoveIsGuardedLikeAClick`, `TestALegacyPointIsUnaffected`,
  `TestTheRecordedPointAFindHandsBackIsGuarded` — all through `Host.Invoke`, which is what a
  running route calls.
- And the four corrections above: `TestAGuardedPressRefusesEveryTermItCannotCheck` (twelve
  recordings — each term missing, each malformed one way, and a basis this does not know),
  `TestAGuardedPressRefusesWhatTheDesktopWillNotReport` (an unreadable live DPI and an unnameable
  live program), `TestAGuardedPressRefusesAnOffsetOutsideItsOwnWindow` (all four edges, plus the
  two corners that must still be honoured),
  `TestAGuardedPressRefusesTheSameProgramInAnotherProcess` and
  `TestAGuardedPressRefusesAWindowBoundWithoutAnOwner`. Every case asserts ZERO INPUT, because a
  refusal that still moved the cursor is the same finding one step later.
- **And the whole chain, which nothing else walked**: `internal/oshost`
  `TestASavedRecordingThatCouldNotBeMeasuredPressesNothing` records four partly-measured presses
  (one per measurement that can fail), takes each through the production simplifier, generator,
  registry, compiler and host, and asserts the saved route declares its window, omits what nobody
  measured, sends NO INPUT and does not fall back to the demonstrated coordinate.
  `TestASavedRecordingThatWasMeasuredPressesInsideTheWindow` is the control, without which the
  first passes on a chain that refuses everything. `TestAContextScopedRecordingHasNoWindowToCheck
  Against` pins the gap below.
- `plugins/studio` `TestReviewSaysWhatThePressMeasured`,
  `TestReviewExplainsAMeasurementThatFailed` (step, draft, and the Save message),
  `TestReviewNamesWhicheverMeasurementFailed` (four, one at a time),
  `TestAScaleThisComputerCannotReportIsNotRepairable` (the branch this machine cannot reach), and
  `TestReviewIsQuietWhenEverythingWasMeasured` — the control, because a warning on every
  recording is a warning nobody reads.

The happy-path fixture puts the window somewhere OTHER than where it was demonstrated,
deliberately. The recorder computes the offset as the click minus the origin, so a window still
at its recorded position reconstructs the recorded desktop coordinate exactly — and a test on
such a window cannot tell "resolved the offset against the window" from "ignored the window and
used the coordinate". The first version of that test asserted both and contradicted itself.

## Mutations

Seventeen for the original increment, twelve for the replay-side corrections, fourteen for the
contract. All killed.

The twelve, each one a way the guard could look present and answer about less than it appears to:
the basis accepted for being merely non-empty; a missing term not refused; a malformed one not
refused; an unnamed program not refused; the offset not checked against the window at all, and
then checked with the far edges inclusive; a window bound without an owner passing; the handle's
owner never compared with the bound one; an unreadable live scale passing; an unnameable live
program passing; the guard entered only when an offset is usable, which is the bypass itself; and
preparation not reading the owner.

The fourteen, each one a way a failed measurement could turn the safeguards off or go unsaid: the
basis written only when the window answered; an unread scale defaulting to 96; the program lookup
following the offset instead of the basis; a point written window-relative only when it has an
offset; an unmeasured offset written as 0,0; the step not saying what it cannot be checked
against; the draft not reporting that it will not run; the Save message not saying it; each of the
four missing measurements going unnamed in turn; an unreportable scale still answered with "record
it again"; and a legacy press reported as incomplete, which would put the warning on every route
there is.

**Seven of the first twelve, and five of the fourteen, are a check that was WRITTEN and does not
run** — a conditional whose condition was the thing being checked. Every finding in this area has
had that shape, at three different layers: the guard asking "if both sides know it", the recorder
asking "if every term arrived", and the generator asking "if there is an offset". It is the shape
to look for next: not a missing comparison but a comparison guarded by its own subject.

- On the record-and-carry half: simplify dropping the measurement; the click point reverting to
  the release; codegen writing no measurement; an unread DPI defaulting to 96; the basis omitted
  from the route; the basis no longer required for the rest; releases given a program name; and
  the shape read taking whatever is in front instead of the attributed window.
- On the guard: frontness, liveness, size, DPI and program each left unchecked in turn; a closed
  window falling back to the recorded coordinate; the opt-in ignored so legacy points are guarded
  too; the binding ignored so the foreground is read; and `Find` bypassing the resolver.

Two suite guards also caught real misses while this landed, neither of them from a test written
for it: `routes/os.marco` had drifted from the canonical act surface it shadows, so the new
`Point` fields would have been declared and unreachable where routes run; and `spectest`'s
backstage-concept gate rejected the word "evidence" in an act surface comment — Marco says what
it intends to do.

## Known gaps

- **Not tried on a live desktop.** Every claim above is from the automated suites. Guarded
  coordinate replay is outstanding until a live acceptance run, and so is the owner's Stop-button
  trial, which is a separate question about recording and the window.
- **A CONTEXT-SCOPED RECORDING CANNOT RUN A GUARDED PRESS, AND THAT IS STUDIO'S DEFAULT SCOPE.**
  Found while wiring the end-to-end chain; not caused by it, and present since the guard landed.
  `Save` under the `app` scope strips the leading Activate and passes no application to `codegen`,
  because that scope's whole promise is that it switches nothing — it runs only while its
  application is already in front. So the route contains no `do OS's Activate`, nothing binds a
  window, and every press that declared a window refuses with *nothing to measure it against*. The
  `everywhere` scope is the same, by the same line of `Save`. Only `app-anywhere` works.

  The resolution is a decision about what PREPARATION means for a scope with no preparation step:
  either such a routine binds the foreground once at the start of its run — which is that scope's
  own premise, and would make its first press checked against whatever is in front rather than
  against something prepared — or recordings whose presses declared a window must be saved as
  `app-anywhere`, and Studio should say so at Save. Both change the guarded contract, so neither
  is taken here. `TestAContextScopedRecordingHasNoWindowToCheckAgainst` pins the current state so
  it is deliberate and found by a suite rather than by a live trial.
- **A REFUSED PRESS DOES NOT FAIL THE ROUTINE.** `codegen` emits a bare `do OS's Click with p1.`,
  and a `do` whose status nobody reads does not abort the interaction — so a routine whose every
  press is refused runs to `this is ok!` and reports done. Zero input is sent and the refusal is
  logged, but the person is told it worked. This is the pre-existing behaviour of every failing
  host call in a generated route rather than anything about this contract, and it is why
  `played` in the chain test reads the host's log as well as the route's output. It matters more
  now: before, a press that could not be checked was pressed anyway, so "done" was at least about
  something that happened.
- **A drag resolves no offset at all**, so it carries no window and is checked against none. That
  is consistent — window terms with no offset to resolve would be opting into a guard with
  nothing to apply — and it means a demonstrated drag is still a pair of absolute coordinates.
- **`anchorPoint` is unguarded as a search hint**, which is correct, and it means a route recorded
  with vision on and replayed with vision on resolves its matched point by matching rather than
  by this contract. A template match is its own evidence; the owner's instruction to keep
  localization and outcome verification separate is why these are not merged.
- **No calibration.** Nothing here chooses a tolerance from measured variation, because nothing
  here has a tolerance. See [[ADR-247-what-a-click-is-evidence-of]] for the calibration this
  deliberately does not use.

## Related

[[ADR-248-replaying-a-demonstrated-layout]] is the licence this satisfies the window condition of.
[[ADR-250-the-recorder-runs-in-the-window]] is the window these recordings are made from.
[[ADR-249-ending-a-recording-is-not-part-of-the-recording]] is how the recorder's own window is
kept out of them.
