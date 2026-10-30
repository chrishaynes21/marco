---
type: decision
status: accepted
date: 2026-10-29
decision: "ACCEPTED — WHICH WINDOW A RUN ACTS IN IS CHOSEN BEFORE PLAY AND CARRIED TO THE RUN, AND IT IS NEVER DURABLE. Studio raised an application's window and the child bound one: two steps and two independent choices. `theWindowToRaise` preferred the biggest window that was up; `winctx.Bind` was given a program NAME and took the first visible window `EnumWindows` reached. With two Explorer windows, two copies of a game or two browser profiles those are different windows — and NOTHING DOWNSTREAM COULD TELL, because `oshost.confirmWindow` compares the program, the size, the scale and the owning process of the window that was BOUND, and two windows of one program agree on the first three while the fourth compares the bound window against itself. The press landed in a window nobody chose, having passed every check there is. A routine naming no application had nothing raised at all, so its run began with Studio in front and an unguarded press went into Marco's own window. So the window is CHOSEN, out of the same list `marco learn` chooses a recording's window from (`orchestrator.Pickable`, which excludes this process and its owning surface, so Studio is never a candidate): one candidate is selected automatically and SHOWN beside Play, several is a visible choice, and none means Play is unavailable with a reason that says what to start. Studio then raises THAT HANDLE and hands the child `--window=<handle>:<owner>` — one term with BOTH HALVES, because a handle is not an identity: Windows hands the number back out, and a program name does not separate two copies of one program. The child VERIFIES what it was given — a window that is still there, still that process's, and still the program the route asks to activate — binds it, and NEVER looks for another window of the same program, which is the fallback that was the defect. It is a RUN's term and never a route's: nothing durable carries a handle, because an identity that stops existing when a window closes would be inherited by the next run. And the choice is an INITIAL HANDOFF, said plainly and not overclaimed: the window receives the foreground at the start of the run, Marco does not own the desktop, and a window that comes forward afterwards gets what is typed next — a guarded routine's presses are checked against the window they were recorded in and STOP, an unguarded routine's are not, and the sentence beside Play says which. `marco do` without `--window` is unchanged, which is the whole of the legacy CLI path. The refusal is ANNOUNCED through the outcome protocol rather than exited: the first version printed a sentence on stderr and exited 1, and Studio reported the run as having ended with a blank answer. 14 of 14 mutations killed."
source_paths:
  - plugins/studio/target.go
  - plugins/studio/routines.go
  - internal/orchestrator/pick.go
  - internal/oshost/oshost.go
  - internal/winctx/winctx_windows.go
  - cmd/marco/assistant.go
---

# The window a run acts in is chosen and carried

## Three questions, and this is the third

| question | when it is answered | durable? |
|---|---|---|
| which window a recording is **of** | before the hooks go on | no — it is a fact about the demonstration |
| where a routine is **available** | when it is saved, and at a reviewed move | **yes** — the folder IS the identity |
| which window **this run** acts in | at the instant somebody presses Play | **never** |

They are decided at different moments, by different people, about different things, and the third
one is the one that had no answer. See [[ADR-268-record-only-the-chosen-window]] for the first and
[[ADR-271-a-rescope-is-a-reviewed-move-between-identities]] for the second.

## The defect, and why no check could see it

Studio raised a window and the child bound a window. Two choices:

- `theWindowToRaise` (Studio) preferred a window that was up, then the biggest.
- `winctx.Bind` (the child) was given a program NAME and took the first visible window
  `EnumWindows` reached.

With one window those agree. With two windows of one program — two Explorer windows, two copies
of a game, two browser profiles, a launcher and its game — they are different windows.

**And nothing downstream could tell.** `oshost.confirmWindow` is the guarded boundary, and it is
thorough: the window is still alive, still in front, the same size, the same scale, the same
program, and owned by the same process it was bound to. Every one of those passes for a sibling
window, because the sibling IS the same program at the same scale, and the PID check compares the
bound window against itself. So the press landed in a window nobody chose and the run reported
success.

A routine naming no application had nothing raised at all. The run began with whatever was in
front, which at the instant somebody clicks Play in Studio is **Studio**.

## What happens now

**The window is chosen, out of a list that cannot contain Marco.** `orchestrator.Pickable` — the
same list `marco learn --window` chooses a recording's window from — excludes this process and the
surface that owns it, and leaves out minimized, off-screen and nameless windows. For a routine
that names an application, the windows of that application; for one that names none, every window,
because that is exactly what *works anywhere, switching nothing* means.

**One is selected, several is a choice, none is a refusal.** A single candidate is selected
automatically **and still shown**, so somebody is told which window rather than discovering it
from where the presses landed. Two windows of one program is not a choice this surface may make.
No window at all is Play unavailable with the sentence that says what to start — the alternative
is pressing into whatever is in front.

**The window travels as ONE term with BOTH halves.** `--window=<handle>:<owner>`. A handle is not
an identity: Windows hands the number back out, and the program name does not separate two copies
of one program. Two flags would be two things a caller can supply half of.

**The child verifies and never substitutes.** `oshost.Host.GiveWindow` refuses a handle that is no
longer a window, or no longer that process's; `activateGiven` refuses one that belongs to a
different program than the route asks to activate. Neither falls back to looking for another
window of the same program, which is the behaviour being replaced.

**Nothing durable carries a handle.** Not the route, not a preference, not a session. An identity
that stops existing when a window closes would be inherited by the next run.

## It is a handoff, and that is said rather than overclaimed

The selected window receives the foreground **at the start of the run**. It is not confinement of
every subsequent keystroke, and the surface must not imply that it is: Marco does not own the
desktop, and a window that takes the foreground mid-run gets what is typed next.

What actually holds afterwards depends on the routine, so the sentence beside Play says which:

> Play brings *notepad — notes* to the front and presses there. Its presses are checked against
> that window and stop if something else comes forward.

> Play brings *notepad — notes* to the front and presses there. Nothing after that is confined to
> it: its presses are not checked, so whatever is in front at the time receives them.

The first is a guarded routine, whose own boundary refuses at the instant of a press
([[ADR-251-a-press-checks-the-window-it-was-recorded-in]]). The second is a legacy one, which
declined checking when it was recorded. Saying "it will press in that window" for both would be a
promise only the first can keep.

## Three things this deliberately does not do

**It does not raise from the child.** Windows refuses `SetForegroundWindow` to a process that does
not already own the foreground, and the child is a process nobody is looking at. Studio owns the
foreground at the instant somebody clicks a button in it, and that is the one moment it may hand
it on. `winctx.Raise` is called in the child too, best effort, for the person who typed
`marco do --window=3` at a terminal; a raise that did not work is kept beside the refusals, and
the guarded boundary's own "is it in front" question is what decides.

**It does not change `marco do` without `--window`.** That is the legacy CLI path — `Activate`
finds a window by program name, exactly as it always has — and preserving it is part of this
decision. A saved routine performed from a terminal or by the overlay is not somebody choosing a
window.

**It does not cover the Act surface.** Studio's Play for an Act goes through the supervised runner
under a written authorisation, and carrying a window through that means changing the grant and
`cmd/director`. An Act that names no application still grounds against whatever leads the desktop.
That is the same hole one surface over, it is NOT closed here, and it is recorded rather than
implied: see [[Roadmap]].

## And a refusal is announced, not exited

The first version of the CLI hand-over printed a sentence on stderr and exited 1. Studio read no
outcome off the wire line and reported the run as having ended with a **blank answer** — which the
end-to-end test found, and which is the whole reason that test spawns a real child. Every surface
that spawns `marco do` reads the outcome through `internal/outcome`, so the refusal is
`[result] refused` with the sentence beside it.

## Enforced by

`plugins/studio`, against a stated desktop — a live reading would make these pass or fail on what
the owner happens to have open, which is the trap `chosenwindow_test.go` already records:

- `TestTheChosenWindowIsBroughtForwardBeforeTheRunStarts` — **two windows of one program**, and
  the raise reaches the handle that was chosen rather than one this surface picked. The case the
  whole slice is about; a routine played with the other window's term fails it.
- `TestARoutineThatNamesNoApplicationIsPlayedIntoAChosenWindow` — the **opposite** of what it
  asserted before: a global routine is offered a window and raised into it. "The routine did not
  ask for a foreground" was true of the route and never of the run.
- `TestARoutineWithNowhereToPressIsNotPlayed` — a desktop holding nothing but Marco's own windows
  is a refusal naming what to start, and nothing is brought forward for it. The half that makes
  the half above safe: choosing a window only beats pressing into whatever is in front if NO
  window is a refusal rather than a fallback.
- `TestTwoRoutinesOfOneNameAreTwoRoutines` — each namesake is played into a window of ITS OWN
  application, by handle.
- `TestTheCommandCarriesTheRoutesIdentityAndTheLibrary` — the term reaches the command, written by
  `Pickable.Term` rather than spelled a second time.
- `TestARunToldToActInAWindowThatIsNotThereIsRefused` — **the cross-process half, for real**: a
  handle that cannot be a window on any desktop handed to a spawned `marco do`. The child refuses,
  announces `refused` through the outcome protocol, says it was about the window it was given, and
  does NOT run the routine anyway. Needs no window to exist, no window to close, and nothing of
  the owner's screen.
- `TestPlayingARoutineRunsItAndReportsWhatItSaid` — the control, and now explicitly the LEGACY
  shape: `startRoutine` with no window handed over spawns the command `marco do` has always been
  and still performs.
- `rescopepage_test.go`'s sibling `targetpage_test.go` renders the page: the selected window is
  shown beside Play, several candidates leave Play dead until one is chosen, no candidate replaces
  the row with the reason, and the term the page submits is the one it displayed.

`internal/oshost` — `TestAGivenWindowIsBoundAndNeverReplacedBySibling`,
`TestAGivenWindowThatHasGoneIsRefused`, `TestAGivenWindowWhoseOwnerChangedIsRefused`,
`TestActivatingAGivenWindowOfAnotherProgramIsRefused`, driven through the `windowFacts`, `imageOf`
and `raiseWindow` seams, because a window that closes, changes owner or changes program between
two instants cannot be arranged on a real desktop on purpose.

`internal/orchestrator` — `TestAWindowTermCarriesBothHalvesOrNeither` and
`TestAWindowTermIsReadBackAsTheWindowItNames`.

`cmd/marco` — `TestTheEntranceNeverPutsAFlagInThePhrase` carries `--window` now, so the flag
cannot fall into the phrase and be performed as a name.

## Related

- [[ADR-268-record-only-the-chosen-window]] — the first of the three questions
- [[ADR-271-a-rescope-is-a-reviewed-move-between-identities]] — the second
- [[ADR-251-a-press-checks-the-window-it-was-recorded-in]] — what holds after the handoff
- [[ADR-252-a-library-is-a-place-and-a-routine-is-played-where-it-was-made]] — the Play order this
  supersedes a step of
- [[Learned-Plays]] — the subsystem note
