---
type: adr
status: accepted
date: 2026-10-23
decision: "ACCEPTED — A COORDINATE IS A CLAIM ABOUT A PLACE ONLY WHILE THE WINDOW IT WAS MEASURED IN IS STILL THAT WINDOW, AND THE PRESS IS WHERE THAT GETS CHECKED. A press records five terms at the instant it is made: the BASIS its offset is measured from, the window's OUTER size, the window's DPI as its own term, and the file name of the program that owned it. All five reach the saved route, because saved legal Marco is the only executable truth in this stack. The basis is recorded although there is one value today, so no route already on disk is reinterpreted the day a recorder measures from the client area instead. Each term is written only when it was MEASURED — a DPI nobody could read is absent, not 96 — and none is written without the basis. The terms are also the OPT-IN: a point carrying them asks to be checked, and a point without them is pressed by exactly the rule it was saved with. Replay binds the WINDOW during preparation, from `Activate`, and before every press re-reads THAT HANDLE: still a window, still in front, its origin, its outer size, its DPI, and the program that owns it NOW — because handles are reused. Every comparison is EXACT and any disagreement REFUSES, naming the term; it does not fall back to the recorded coordinate, because that is pressing a remembered pixel on a desktop just shown not to match. The window is re-read before every press and never cached: the legacy origin snapshot answers about the FIRST press of a route and exists to survive a focus flicker this path detects directly. One resolver serves clicks, Moves and the recorded point a vision-free `Find` hands back, because those are three ways into one question and a guard with a bypass is not a guard."
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
zero. Each term is written only when measured, and a term the DEMONSTRATION lacks is not checked
at replay — there is nothing to compare, and refusing on it would refuse every routine for the
crime of having been recorded honestly.

None of them is written without the basis, though. Checking a window's size while having no idea
which of its rectangles the offset belongs to is checking the easy half.

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
  `TestOnlyAPressCarriesTheProgramName`, `TestAProgramThatHasGoneLeavesTheNameEmpty`,
  `TestAPressWithNoProcessIsNotLookedUp`.
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
  `TestATermTheDemonstrationLacksIsNotChecked`, `TestAMoveIsGuardedLikeAClick`,
  `TestALegacyPointIsUnaffected`, `TestTheRecordedPointAFindHandsBackIsGuarded` — all through
  `Host.Invoke`, which is what a running route calls.

The happy-path fixture puts the window somewhere OTHER than where it was demonstrated,
deliberately. The recorder computes the offset as the click minus the origin, so a window still
at its recorded position reconstructs the recorded desktop coordinate exactly — and a test on
such a window cannot tell "resolved the offset against the window" from "ignored the window and
used the coordinate". The first version of that test asserted both and contradicted itself.

## Mutations

Seventeen, all killed.

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
