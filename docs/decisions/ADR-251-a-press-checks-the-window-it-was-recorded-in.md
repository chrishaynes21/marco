---
type: adr
status: accepted
date: 2026-10-23
decision: "ACCEPTED — A COORDINATE IS A CLAIM ABOUT A PLACE ONLY WHILE THE WINDOW IT WAS MEASURED IN IS STILL THAT WINDOW, AND THE PRESS IS WHERE THAT GETS CHECKED. A press records five terms at the instant it is made: the BASIS its offset is measured from, the window's OUTER size, the window's DPI as its own term, and the file name of the program that owned it. All five reach the saved route, because saved legal Marco is the only executable truth in this stack. The basis is recorded although there is one value today, so no route already on disk is reinterpreted the day a recorder measures from the client area instead. Each term is written only when it was MEASURED — a DPI nobody could read is absent, not 96 — and MISSING IS WHAT IT STAYS. The BASIS is the contract and everything else is a measurement, so `RelTo` is written for every press a measuring recorder makes whether or not the measurements arrive, and the saved file therefore says three things apart: no basis is LEGACY and pressed exactly as it always was; a basis with every term is GUARDED; a basis with a term missing is INCOMPLETE and refused, naming what it lacks. Which state a press is in does not depend on a measurement having succeeded. A failed measurement means guarded replay is UNAVAILABLE for that recording — never that its requirements become optional, and never that it quietly becomes an ordinary point, which is pressed at its recorded coordinate having checked nothing. Review says what was not measured and whether recording it again can change the answer; saving is allowed and is not quiet. The BASIS ALONE is the opt-in — requiring an offset beside it was a bypass — and it must be a basis this knows how to check. AND A REFUSAL STOPS THE ROUTINE: a guarded route is generated as a CHAIN — the preparation and every guarded press each in their own capability, calling the next only when its own action went through, with the single `this is ok!` inside the success of the last — so nothing after a refusal runs and no unconditional ending can override it. A guarded press is never folded into a `repeat`, which has no way to say that. The run's answer is DATA and not a log line: a root handler is compulsory in Marco so a refused routine still ends tidily, and the host therefore keeps its own refusals for the command to fail on. New guarded recordings are saved to bring their application forward, because the scopes that switch nothing bind no window and every press in them would refuse; the choice is withdrawn rather than defaulted, an impossible scope is refused at Save rather than rescoped, and a routine already saved in that state is listed and saved again from its own demonstration. Replay binds the WINDOW during preparation, from `Activate`, AND THE PROCESS THAT OWNED IT THEN, because a handle is not an identity: two copies of one program share an executable name, so only the process id separates the window the route prepared from a second instance that inherited the handle. Before every press it re-reads THAT HANDLE: still a window, still in front, still that process, its origin, its outer size, its DPI, and the program that owns it NOW. And the offset must lie INSIDE the recorded rectangle, because origin-plus-offset is a point on the desktop and every other check can pass while the cursor leaves the window. Every comparison is EXACT and any disagreement REFUSES, naming the term; it does not fall back to the recorded coordinate, because that is pressing a remembered pixel on a desktop just shown not to match. The window is re-read before every press and never cached: the legacy origin snapshot answers about the FIRST press of a route and exists to survive a focus flicker this path detects directly. One resolver serves clicks, Moves and the recorded point a vision-free `Find` hands back, because those are three ways into one question and a guard with a bypass is not a guard."
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
  - cmd/marco/panicstop.go
  - plugins/studio/recording.go
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

## A refusal stops the routine, and a guarded route is a chain

The next press of a sequence means nothing once the one before it did not happen. A menu that
never opened has no button to press, and the coordinate that button occupied belongs to whatever
is there instead — so a routine that pressed on past a refusal would be doing the thing the check
exists to prevent, one step later and with the check's blessing.

Marco's way of saying that is the one `testdata/macro_ark_transfer` has always used: a `do` whose
two arms both END, with the rest of the work inside the one that succeeded. **It is the only way
that works**, and the reason is worth recording because the obvious alternative silently does not:
a terminal return inside a NON-exhaustive `when` arm does not return. The arm fires,
`this is failed with error "…"!` is ignored, execution falls through, and the caller is told ok.
Measured on a three-line program; it is why every fixture in the repository pairs its arms. Not
changed here — that is Marco's own semantics and a change to it is nothing to do with this
contract — but it is the reason the shape below is the shape.

Written as one block, that shape is one indent level per press, which at thirty presses is a route
nobody can read. So each propagating action gets its own capability and calls the next only when
its own action went through:

```marco
this can Run.
this's Run does...
    do OS's Activate with "arkascended"...
        when ok?
            do ArkGraphics's Step2.
            this is that!
        or?
            this is that!
```

`Run` is the first link, so `do <Actor>'s Run` still performs the routine. `this is that!` adopts
the last outcome verbatim, so a refusal three presses along comes back out of `Run` as the refusal
it was. The one `this is ok!` sits inside the success of the LAST press, so there is no path on
which a refused routine reports that it worked.

**What propagates**: the preparation, every guarded press, and every keystroke. `Activate` is in
because it is what binds the window the presses are then checked against — a routine whose
application never came forward has nothing to press in and must not try. An anchored press is NOT
in: its emitted form is a ladder of locators with a click in each arm rather than one sentence, so
there is nothing to wrap; anchors are off unless computer vision is explicitly on, so no
recorder-first recording produces one. Recorded as a gap rather than silently covered.

**A KEYSTROKE IS A LINK, AND NOT BECAUSE ANYTHING CHECKS IT.** Nothing measures a window for a key
and nothing here compares one. A key is a link because it can fail to be DELIVERED, and what
follows it then acts on a screen the key never changed: an Escape that did not arrive leaves the
menu open, the press after it lands on whatever is now under that coordinate, and the guard cannot
see the difference, because the window is the same window. **Geometry cannot detect a menu that
stayed open.** A routine that went on from there reported that it performed — the first version of
this contract stopped at presses only, and a key was an ordinary step.

That was not found by a live run and could not easily have been: it needs the machine to refuse
one keystroke and take everything else. It was found by the mutation gate on the tests for a key
beside a checked press, where promoting a key to a link of its own changed nothing any test could
see — which was the question, not the answer.

**And neither a guarded press nor a keystroke is folded into a `repeat`.** Three identical presses
used to become `repeat 3 times`, and a repeat has no way to say "stop if that one did not happen" —
the body would run again after the one that did not. For presses the folding is simply off; for
keys it cannot be, because `simplify` folds a run of the same key into one step with a count before
anything knows what the recording is for, so the generator expands that count back into one link
per keystroke. Three presses written out three times is the honest reading of three presses that
each have to be allowed.

**A route with no guarded press is generated exactly as it was**: one block, a bare `Activate`
whose status nobody reads, clicks attempted and the run carrying on. That contract is not re-read
now, and the test that holds it asserts the bare sentence rather than the absence of a second
capability — because a legacy route has one propagating action and no guarded press, so a chain
generated for everything carries no `Step2` to notice it by.

## What cannot be stopped at is not generated at all

The chain works by wrapping ONE `do` sentence in arms that both end. A step emitted as something
else cannot be stopped at, and in a routine whose presses are checked that is not a smaller
promise — it is the promise broken by the one step nobody could wrap, because whatever follows it
runs regardless.

A **non-left click** is the live case and it was generated wrong first: a guarded `Move` to aim,
then an ordinary `do OS's Click with "right".` whose result nobody read. The aim was checked and
the press was not, and a press the machine refused was followed by the next step anyway. Also a
**drag**, which resolves no offset and is checked against nothing; an **anchored press**, which is
a ladder of locators with a click in each arm; and a **repeated block**, which runs its body again
— the one thing a stop cannot survive.

So generation REFUSES, naming what it found, and Studio says it at review rather than letting a
generator error arrive at a button press. Supporting a non-left press properly would mean one
guarded operation carrying a button beside the point, which is a permanent word in the language
for a case no recording has needed yet; it waits for one that does.

**And "did a press ask to be checked" is a different question from "can this version wrap it".**
They came out as one first, which was a hole with the shape of every other hole here: a recording
whose only press was a right-click had no step this could wrap, so it was not a guarded route, so
the refusal never fired and it was generated as an ordinary block — a press that declared the
window it was made in, pressed without any of it being looked at. Conflating the two answers "no"
to the first whenever the second is "no".

A route where nothing asked to be checked keeps all of it: a right-click or a drag there was saved
under a contract that says a click is attempted and the run carries on.

## And the run's answer is data, not a log line

A refused routine still ENDS TIDILY. A root handler is compulsory in Marco — the compiler refuses
`App leaves status "failed" unhandled at root` — so the failure is caught, logged, and the script
finishes. `marco do` would therefore have exited successfully about a routine that pressed
nothing, which is the worst of the available outcomes: the person is told it worked.

So the host keeps an account of everything it did not do (`oshost.Host.Unperformed`) and
`runRoute` turns a non-empty list into a failed command. Not the output and not an exit code
alone: both of those are downstream of an answer that now exists.

**Three kinds, and one list, because the caller should not have to care which**: a press that could
not be CHECKED, a press the machine would not DELIVER having passed every check, and an
application that never came FORWARD for one. The middle one was missing, and that is why the list
is not named for refusals: a guard that passes and a backend that then rejects the input are the
same outcome from where a person is standing — the press did not happen — and an account holding
only refusals let the command report success about it. `SendInput` really does get blocked: by an
elevated window owning the foreground, by UIPI, by a game with raw input and a lower-integrity
sender. Nothing upstream could have noticed, either: the chain stopped correctly, the root handler
caught the failure correctly, and the program ended tidily with no press having been made.

Which contract a route carries is read from the route itself, because that is where it lives: a
point stating the window it was recorded in is the whole of the guarded contract. A route carrying
none reports exactly what it always did, which is what keeps this off every routine the owner has.

On the Act path the answer was already there and needed nothing: `marcoexec` maps a capability
that returned `failed` to `StatusRuntimeFailed` with the host's own sentence as the diagnostic, and
the tee in `marcorunner` sees a refused `Click` like any other failure.

## A guarded recording is saved to bring its application forward

Found while wiring the chain end to end, and present since the guard landed: `Save` under the
`app` scope strips the leading `Activate` and passes no application to `codegen`, because
switching nothing is that scope's whole promise. So the route binds no window and every guarded
press refuses. `everywhere` is the same, by the same line of `Save`. Only `app-anywhere` works.

For this MVP the choice is therefore WITHDRAWN rather than defaulted. Studio shows
*"Runs in Ark. Marco brings it forward before playback."* and sends `app-anywhere`; the three-way
select is hidden, because a control that looks like a choice and has one real option is worse than
a sentence. A draft with no guarded press keeps all three.

Asking for an impossible scope at Save is REFUSED and explained — never silently rescoped, which
is the thing `wantedScope` already exists to prevent — and the refusal names the scope it needs so
the person can ask again.

**And a routine already saved in that state has a way out.** The guarded contract arrived before
this rule, so such a routine can be on disk: it reads like a routine and refuses every press, and
the only thing that says so at run time is the refusal itself, one press at a time. Its own
demonstration is stored beside it (`Registry.LoadRecording`), so Studio lists them and saves one
again from that recording on asking, by name. The demonstration is re-simplified rather than the
source rewritten, because rewriting generated Marco by hand is how two things that have to agree
stop agreeing. The old file goes only once the new one is written.

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
- **And what a refusal does to the routine around it**: `internal/oshost`
  `TestPreparationThatFailsPressesNothing` (zero input, one refusal — three would mean the
  preparation was a line above the body rather than around it),
  `TestAFirstPressThatRefusesNeverReachesTheSecond`,
  `TestASecondPressThatRefusesKeepsTheFirstAndStillFails` (both halves: the press that happened is
  in the account, and the run is unsuccessful anyway),
  `TestEveryPressGoingThroughCompletesThePlayback` (and claims nothing about the screen
  afterwards), `TestALegacyRouteIsStillOneBlock`. Each also holds the ROUTE's own answer, which
  `Refused` does not cover: a chain whose arms both ended in `this is ok!` would stop just the
  same and report that it worked.
- `cmd/marco` `TestAGuardedRoutineThatWasRefusedFailsTheCommand`,
  `TestALegacyRoutineReportsWhatItAlwaysDid`, `TestAPressTheMachineRefusedFailsTheCommand` and
  `TestALegacyRoutineWhosePressFailedReportsWhatItAlwaysDid` — the command's answer. The first
  two run the real host on routes whose application is absent, so no press is reached; the second
  two stand in a host that takes everything but the input, which is the only way to reach a
  DELIVERY failure from here and the case an absent application cannot pose.
- `internal/oshost` `TestAPressTheMachineWillNotDeliverStopsTheRoutine`,
  `TestAnUndeliverableSecondPressKeepsTheFirst`, `TestAnAimTheMachineWillNotDeliverIsKept` — the
  same gap at the host, with the delivery error kept rather than reported as a refusal.
- `internal/codegen` `TestAStepThatCannotStopTheRoutineIsRefused`,
  `TestARightClickAndADragAreGeneratedWhenNothingAsksToBeChecked`,
  `TestARecordingOfOnlyAnUnwrappablePressIsRefused` — the last one is the hole where asking to be
  checked and being wrappable were one question.
- **And a keystroke beside a checked press, which nothing combined before**: `internal/codegen`
  `TestAKeyAndACheckedPressAreOneRoutine` (the owner's own routine — Escape, then the press),
  `TestAKeyBeforeAPressWaitsForTheApplication`, `TestAKeyBetweenTwoPressesHappensBetweenThem`,
  `TestAKeyBetweenTwoPressesIsInsideTheFirstOnesSuccess` and
  `TestAKeyAfterTheLastPressIsInsideItsSuccess` (both paths) — generated by the real generator and
  performed by the real runtime through a host that writes down what it was asked to do and
  refuses one nominated thing, because the claims are which things happened and in what order.
  Every chain test before these used presses alone and every key test used a route with nothing
  guarded in it.
- **And a keystroke that did not happen**: `internal/codegen`
  `TestAKeyThatDidNotHappenStopsEverythingAfterIt` — before the first press, between two presses
  and after the last one, each with controls around it the machine takes without complaint, so
  what stopped the routine is the keystroke and not the arrangement. In the last case the settle
  wait is the tell: it is the only thing after the final keystroke, so its absence is what says
  the tail went with the key. `TestARepeatedKeyIsEachOfItsPresses` refuses the second of three and
  `TestARepeatedKeyPressesOncePerRepetition` is its control, without which it passes on a routine
  that presses the key once or never.
- **And the same thing with a real recording and a real host**: `internal/oshost`
  `TestAKeyTheMachineWillNotDeliverStopsTheRoutine` takes the owner's own shape — a press, Escape,
  another press — through the production recorder, simplifier, generator, registry, compiler and
  `Host`, with the backend rejecting the keystroke the way `SendInput` really is rejected. It
  asserts the first press landed, the key was asked for once, NOTHING after it was sent, and the
  run's own ACCOUNT names the key and keeps what the machine said — the half a status cannot
  carry. `TestAKeyTheMachineTakesLetsTheRoutineGoOn` is the control, without which it passes on a
  routine that refuses everything.
- **And the command's answer, because that is what Studio reads**: `cmd/marco`
  `TestAKeyTheMachineRefusedFailsTheCommand` (the command fails, keeps the machine's words, and
  NO press was attempted after the key) and `TestALegacyRoutineWhoseKeyFailedReportsWhatItAlwaysDid`
  — the same host, the same rejection, the other contract, asserting the press after the key was
  still attempted rather than merely that no error came back. A simulated host inside one package
  cannot establish that a reason reaches the window; the command's error is the wire.
- `plugins/studio` `TestARecordingWithAStepThatCannotBeStoppedAtIsNotSaved`,
  `TestAnOrdinaryRecordingIsSavedWithoutComplaint`,
  `TestARightClickInALegacyRecordingIsStillSaved`.
- `internal/simplify` `TestGuardedPressesAreNotFoldedIntoARepeat`,
  `TestUnmeasuredPressesAreStillFolded`.
- `plugins/studio` `TestAGuardedRecordingIsSavedToBringItsApplicationForward`,
  `TestALegacyRecordingKeepsTheChoice`,
  `TestSavingAGuardedRecordingUnderAnImpossibleScopeIsRefused` (both impossible scopes, nothing
  written, and the one it needs still accepted afterwards),
  `TestARoutineThatCannotPressAnythingIsListedAndSavedAgain`.
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
contract, sixteen for the execution path, ten for the two result gaps, five for a keystroke beside
a checked press and seven for making one stop the routine. All killed but one, recorded below as
unfalsifiable rather than as a hole, plus three no-op controls that survived as they should.

The ten: a delivery failure not kept, and then not kept for a press and for an aim in turn; a step
that cannot be stopped at generated anyway; nothing ever unpropagatable; asking to be checked and
being wrappable treated as one question; a non-left click allowed; a drag allowed; the command
ignoring what did not happen; and review not saying a recording cannot be saved.

The sixteen: a guarded route not generated as a chain, and a chain generated for every route; a
refusal reported as success; the preparation not propagating; the next link called whatever
happened; guarded presses folded into a repeat; the run not keeping its refusals; a preparation
that failed not kept; a refused routine reporting success from the command; a legacy routine
judged by the guarded contract; an impossible scope accepted at Save; review not saying how the
routine will be performed; a legacy draft having its scope decided for it; a routine that already
brings its window forward listed as broken; the old copy left on disk after a resave; and a resave
under the scope that could not work.

Two of those first SURVIVED and the tests were wrong rather than the code: "every route is a
chain" went unnoticed because a legacy route has one propagating action and no guarded press, so
its chain carries no `Step2` to notice — the assertion had to become the bare `Activate`
sentence. And "the preparation does not propagate" went unnoticed because the test read only the
FIRST refusal, which is the same either way; it had to count them, because the difference is one
refusal against three.

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
- On a keystroke beside a checked press: the key moved inside its own press's success instead of
  the previous one's; the key promoted to a link of its own; whatever follows the last press
  written into its refusal as well as its success; and the preparation no longer the first link.
  The middle two are the ones that matter — each leaves a routine that performs correctly whenever
  nothing refuses, so only a refusing path can tell. The no-op control was the settle wait's
  duration, which nothing in that file asserts.

  One of those four was killed only after a new test was written for what it exposed: a key that
  cannot be performed does not stop the routine. That became the correction above, and the gate is
  how it was found rather than a live run.

  And one SURVIVED and was recorded rather than closed: every key flushed into the PREVIOUS link's
  success instead of the next link's preamble. It was unfalsifiable, not a hole — those two places
  were the same instant, after the earlier press and skipped when it refused — so a test telling
  them apart would have been holding a shape with no consequence. The first version of that file
  had such a test, with a comment claiming the refusing path would distinguish them. It would not,
  and the test was removed. The mutation no longer exists: a key is a link now, so there is no
  preamble for one to sit in.
- On making a keystroke stop the routine: a keystroke no longer a link, which is the original gap
  restored (killed in two packages); a repeated key left as a `repeat` block; a repeated key losing
  one of its presses; a key failure not kept in the account; the account kept but the status
  reported ok, so nothing stopped; the command no longer reading the account; and the contract read
  backwards, so a guarded route is treated as legacy and a legacy one as guarded — the only
  mutation that falsifies the legacy half, and it kills all six command cases. The no-op control
  was the `Count <= 1` guard in `propagating`, which `keyByKey` has already made true by the time
  anything asks.

  The gate is also what separates the three levels, and it should be read that way. Not keeping the
  failure in the account is invisible to the generator's tests and fatal to the host's; the command
  no longer reading the account is invisible to both and fatal only at the command. Each level was
  mutated, and each killed only what it should.

Two suite guards also caught real misses while this landed, neither of them from a test written
for it: `routes/os.marco` had drifted from the canonical act surface it shadows, so the new
`Point` fields would have been declared and unreachable where routes run; and `spectest`'s
backstage-concept gate rejected the word "evidence" in an act surface comment — Marco says what
it intends to do.

## Known gaps

- **Shown on a live desktop, once.** The owner ran E2E.md §63 end to end: a routine recorded in
  Studio, saved, selected and played into a window that had been MOVED, pressing where it was
  shown; the same routine refusing after that window was RESIZED, naming the term that disagreed;
  and Stop mid-run ending as cancelled. The Stop-button question was the same trial's step 3 and
  is settled in [[ADR-249-ending-a-recording-is-not-part-of-the-recording]]. Everything else here
  is still from the automated suites, and one run is one run: it says the contract holds on that
  desktop, in that application, at that scale.
- **CONTEXT AND GLOBAL SCOPES STILL CANNOT CARRY A GUARDED PRESS.** Resolved for this MVP by
  withdrawing the choice (above), not by making those scopes work. What preparation means for a
  scope with no preparation step is still open: such a routine could bind the foreground once at
  the start of its run, which is that scope's own premise and would make its first press checked
  against whatever is in front rather than against something prepared. Not taken, and
  `TestAContextScopedRecordingHasNoWindowToCheckAgainst` pins what a route saved that way does.
- **A NON-LEFT PRESS, A DRAG, AN ANCHORED PRESS AND A REPEATED BLOCK CANNOT BE SAVED AS PART OF A
  GUARDED ROUTINE.** Refused at generation rather than half-guarded, and said at review. Each
  could be supported: the first needs one guarded operation carrying a button beside the point,
  which is a permanent word in the language for a case no recording has needed yet. The last two
  are unreachable today anyway — anchors are off unless computer vision is turned on, and loop
  folding is off for a recording with a guarded press.
- **TYPED TEXT AND HELD KEYS STILL DO NOT STOP A ROUTINE.** A keystroke does, now (above), and
  these two are the rest of the keyboard. `Type` is one step that can emit SEVERAL sentences — a
  run of text, a declared argument, a secret resolved at run time — so it is not one `do` to wrap,
  and what a partly typed line means is its own question. `KeyDown`/`KeyUp` are a hold, which
  brackets the steps between them; making the pair propagate means deciding what the release owes
  the press when the press did not happen, and a hold left on is worse than a hold never taken.
  Neither is authorized here and neither is pretended to be covered. And the gap is the whole gap,
  not half of it: their delivery failures are not in the account either (`doType` and `doKeyDown`
  return the status without recording anything), so a guarded routine whose typing was dropped
  carries on AND reports that it performed. That is what a keystroke did until this increment.
- **A terminal return inside a non-exhaustive `when` arm does not return.** Marco's own
  semantics, measured here and not changed: the arm fires, the ending is ignored, execution falls
  through, and the caller is told ok. Every fixture in the repository pairs its arms, so nothing
  depends on the other behaviour — but a program written the obvious way would silently report
  success, and that is worth somebody knowing before they write one.
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
