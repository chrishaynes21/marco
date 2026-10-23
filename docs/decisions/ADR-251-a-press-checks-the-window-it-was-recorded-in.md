---
type: adr
status: accepted
date: 2026-10-23
decision: "ACCEPTED — A COORDINATE IS A CLAIM ABOUT A PLACE ONLY WHILE THE WINDOW IT WAS MEASURED IN IS STILL THAT WINDOW, AND THE PRESS IS WHERE THAT GETS CHECKED. A press records five terms at the instant it is made: the BASIS its offset is measured from, the window's OUTER size, the window's DPI as its own term, and the file name of the program that owned it. All five reach the saved route, because saved legal Marco is the only executable truth in this stack. The basis is recorded although there is one value today, so no route already on disk is reinterpreted the day a recorder measures from the client area instead. Each term is written only when it was MEASURED — a DPI nobody could read is absent, not 96 — and the terms go in ALL OF THEM OR NONE, because a check skipped for want of a term is a guard whose strength depends on what the recording happened to capture and which says nowhere that it was skipped. So the recording side DEGRADES to an ordinary point when it cannot measure everything, and the replay side REFUSES anything that states the basis and omits a term: missing, malformed, out of range, and unreadable at the desktop are all refusals, named. The BASIS ALONE is the opt-in — requiring an offset beside it was a bypass — and it must be a basis this knows how to check. A point without a basis is pressed by exactly the rule it was saved with. Replay binds the WINDOW during preparation, from `Activate`, AND THE PROCESS THAT OWNED IT THEN, because a handle is not an identity: two copies of one program share an executable name, so only the process id separates the window the route prepared from a second instance that inherited the handle. Before every press it re-reads THAT HANDLE: still a window, still in front, still that process, its origin, its outer size, its DPI, and the program that owns it NOW. And the offset must lie INSIDE the recorded rectangle, because origin-plus-offset is a point on the desktop and every other check can pass while the cursor leaves the window. Every comparison is EXACT and any disagreement REFUSES, naming the term; it does not fall back to the recorded coordinate, because that is pressing a remembered pixel on a desktop just shown not to match. The window is re-read before every press and never cached: the legacy origin snapshot answers about the FIRST press of a route and exists to survive a focus flicker this path detects directly. One resolver serves clicks, Moves and the recorded point a vision-free `Find` hands back, because those are three ways into one question and a guard with a bypass is not a guard."
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

## All of them, or none — and which side pays

The first version of this drew the conclusion that a term the DEMONSTRATION lacks is not checked
at replay, because there is nothing to compare it against, and refusing would refuse a routine for
the crime of having been recorded honestly. That reasoning is sound about the recording and wrong
about the guard, and the difference matters enough to be the main correction this ADR carries.

A check that is skipped when its term is absent makes the guard's strength depend on how much the
recording happened to capture, and **nothing in the route, the log or the review says which checks
did not happen.** A press with no recorded DPI passed on a desktop redrawn at another scale. A
press with no recorded program passed in a window belonging to something else entirely. The log
line said `click: guarded` for both. That is worse than no guard, because the route reads as
checked.

So the rule is all-or-none, and the two halves carry it differently:

- **The recording side degrades.** The basis goes in only with every term beside it. A window
  that would not report its size, a Windows without `GetDpiForWindow` (before 10/1607), a press
  whose offset falls outside the window it was attributed to — which is what a click that CHANGED
  the foreground looks like from inside the hook — and a program that had exited by the time the
  hooks came down all record an ordinary point, pressed by the rule every route before this was
  saved with. This is the one place where less evidence buys less checking rather than a refusal,
  and it is here because the alternative is a routine that can never run on the machine that
  recorded it.
- **The replay side refuses.** A stated basis requires every term: present, a whole number in
  range, and a program named. Missing, malformed and out-of-range are three different recordings
  and the refusal says which, because they are a truncated route, a hand-edited one, and a
  hand-edited one again, and the person reading the sentence is the one who has to tell them
  apart. There is no fallback: a point that CLAIMS the window and omits what the window was is the
  one case where falling back would be pressing a remembered pixel under cover of a guard.

The degradation is not silent. Studio's review writes `(window not measured)` beside such a press,
next to the `1920×1080, 144 dpi, arkascended.exe` the measured ones carry, so the difference in
what the routine will do is visible while the person is still in front of the screen and before
the routine is kept.

**And unreadable is not matching, on the live side either.** A window that will not say what scale
it is drawn at, and a program that will not say what it is, were read as agreement for the same
reason — the comparison was conditional on both sides being known, so an unknown side meant no
comparison. Both refuse now.

## The basis alone is the opt-in

Entering the guard required a stated basis *and* a usable offset, which read as caution and was a
bypass: a point stating a window with no offset to resolve inside it missed the condition and went
down the legacy path, which presses the recorded coordinate having checked nothing. The basis
alone is the gate. Whether the rest of the terms are there is the guard's question, and its answer
is a refusal.

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
  `TestAPressWithNoReadableWindowRecordsNothingAboutOne` — driven through `newClick`, the
  production event builder. The second one holds the one-handle rule by counting the shape reads
  and checking which window was asked about.
- `internal/recorder` `TestEachPressIsNamedAndEachProgramAskedOnce`,
  `TestOnlyAPressCarriesTheProgramName`, `TestAProgramThatHasGoneUnmeasuresTheWindow`,
  `TestAPressWithNoProcessIsNotLookedUp` — the last two hold the recording side's half of
  all-or-none: a press whose program cannot be named gives up its basis and stays an ordinary
  point, keeping its coordinate and its offset.
- `internal/simplify` `TestTheWindowAPressMeasuredReachesTheStep`, `TestTheClickPointIsThePress`,
  `TestAPressThatMeasuredNothingClaimsNothing`, `TestADragClaimsNothingAboutAWindow`.
- `internal/codegen` `TestAMeasuredPressCarriesItsWindowIntoTheRoute`,
  `TestAnUnmeasuredTermIsAbsentRatherThanDefaulted`,
  `TestWithoutABasisNothingAboutTheWindowIsWritten`, `TestALegacyPointIsGeneratedUnchanged`,
  `TestAnAbsolutePointIsUntouched` — each generates the route, CHECKS it compiles and RUNS it,
  because a set field the engine parses but will not assign fails at neither of the first two.
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

The happy-path fixture puts the window somewhere OTHER than where it was demonstrated,
deliberately. The recorder computes the offset as the click minus the origin, so a window still
at its recorded position reconstructs the recorded desktop coordinate exactly — and a test on
such a window cannot tell "resolved the offset against the window" from "ignored the window and
used the coordinate". The first version of that test asserted both and contradicted itself.

## Mutations

Seventeen for the original increment and fifteen for the corrections, all killed.

The fifteen, each one a way the guard could look present and answer about less than it appears
to: the basis accepted for being merely non-empty; a missing term not refused; a malformed one not
refused; an unnamed program not refused; the offset not checked against the window at all, and
then checked with the far edges inclusive; a window bound without an owner passing; the handle's
owner never compared with the bound one; an unreadable live scale passing; an unnameable live
program passing; the guard entered only when an offset is usable, which is the bypass itself;
preparation not reading the owner; the recording side claiming a window it measured incompletely;
an unnameable program leaving the basis on; and review not saying when a press measured no window.

Seven of the fifteen are a check that was WRITTEN and does not run — a conditional whose condition
was the thing being checked. That is the shape all three findings had, and it is the shape to look
for next: not a missing comparison but a comparison guarded by its own subject.

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
- **A recording that could not measure everything degrades silently in the ROUTE.** Review says
  `(window not measured)` while the person is in front of the screen, and the saved Marco says it
  only by the absence of the terms — there is no sentence in the route itself distinguishing "this
  was recorded before any of this existed" from "this was recorded by a build that measures and
  could not". Both run by the legacy rule, which is correct, and neither says so where somebody
  reading the route a month later would see it.
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
