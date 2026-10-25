---
type: adr
status: accepted
date: 2026-10-24
decision: "ACCEPTED — A READINESS PANEL REPORTS WHAT HAS BEEN ESTABLISHED BY DOING A CHECK, AND SAYS OUT LOUD WHAT IT DID NOT ESTABLISH. Four states used to arrive at the worst available moment: the recorder refusing a platform at Stop, after a whole demonstration; a missing `marco.exe` at Play, over a game somebody had just arranged; a library that could not be written at Save, which is the one instant a recording exists nowhere else; and a window that cannot name its own build, which is the stale-binary trap being invisible. A fifth never arrived at all — `marcoBin` and `directorBin` fall back to the bare name when the sibling is missing, so the spawn resolves it on PATH and SUCCEEDS, running a binary of another age. `Acts.Ready` answers each by acting: the recorder is a platform fact (`recorder.Available`, which explicitly does NOT claim the hooks will install, because that can only be known by installing them and taking the whole desktop's keyboard); each executable is `os.Stat` beside this binary and then `exec.LookPath`, which is the resolution the spawn itself performs, giving THREE answers rather than two — beside, nowhere, or on PATH but not beside; the library is a file CREATED and removed in it, because a directory that exists is not a directory that can be written to and the difference arrives at Save; and the build is this binary's own stamp or the honest word `unknown`. Each problem carries three parts — what is wrong, what it STOPS, and what to DO — because every one of these has exactly one answer and withholding it makes a diagnostic into a riddle. `Limits` is UNCONDITIONAL: it names what was not established (the hooks installing, the application running, a press landing, the screen changing) and is present when nothing is wrong, which is precisely when a panel is read as a promise. Asked again rather than once, because a build script replaces the binaries and a drive fills up while the window is open; on a slow cadence and never during a recording or a run, because the library check writes and a recording is watching the keyboard in this same process."
supersedes: []
affects:
  - interface
source_paths:
  - plugins/studio/readiness.go
  - internal/recorder/recorder_windows.go
  - internal/recorder/recorder_other.go
  - plugins/studio/frontend/index.html
---

# Readiness is what has been checked

## Five ways of finding out too late

Every one of these was discovered by hitting it.

| what is wrong | when you used to find out |
|---|---|
| no input recorder on this platform | at **Stop**, after a whole demonstration |
| no `marco.exe` to play with | at **Play**, over a game you had just arranged |
| the library cannot be written | at **Save** — the one moment a recording exists nowhere else |
| this window cannot name its build | never; you compare file timestamps |
| **`marco.exe` is on PATH but not beside the window** | **never. It works.** |

The last row is the one this exists for. `marcoBin` falls back to the bare name when the sibling
is missing, `exec.Command` resolves it on PATH, and playback runs whichever `marco.exe` that
finds — a different build, of a different age, performing somebody's routine. Nothing fails.
Nothing is logged. It is the trap `internal/buildid` was written because of, and it cost the owner
a day of recordings.

## What the panel may say

Only what a check performed just now established:

```
the recorder     a platform fact — a backend exists, or none does
the executables  os.Stat beside this binary, then exec.LookPath — the resolution the spawn does
the library      a file CREATED and removed in it, which is what Save will do
the build        this binary's own stamp, or the honest word "unknown"
```

`recorder.Available` is a *platform* fact and says so in its own comment: it does **not** claim the
hooks will install. That can only be known by installing them, which takes the keyboard and the
mouse of the entire desktop, so a readiness check claiming it would be claiming something it had
not done.

The executable check gives **three** answers, not two, and the third is the point. `beside` is
right, `nowhere` is a failure, and `on PATH but not beside` is the silent one — reported with what
would actually happen (*a different build of a different age*) rather than as a pedantic note about
file locations.

The library check **writes**, because nothing else is an answer. A directory that exists is not a
directory that can be written to: permissions, a full disk, a path that has become a file, a drive
that has gone away — each of them reads as *the directory is there* to anything short of a write.

## Three parts to a problem, and the third is the one usually withheld

```
Says    what is wrong, as a statement about this machine
Blocks  what cannot be done until it is fixed
Do      the action that would fix it
```

Every problem this panel can report has exactly one answer. Leaving somebody to work it out turns a
diagnostic into a riddle, so the answer is named.

## And `Limits` is unconditional

This is the half that keeps the panel honest, and it is present **when nothing is wrong** — which
is exactly when a readiness panel is read as a promise:

> This is what Marco has checked on this machine just now. It does not say the input hooks will
> install, that the application you are recording is running, that a press will land where you
> pressed it, or that what you are automating will do what you want. Nothing here looks at a
> screen.

A panel that dropped that on a clean machine would be the same false claim as a green suite over an
untested moment — a tested mechanism is not a tested moment — moved to the front of the window where
somebody trusts it before they start. It is asserted on the clean path by test, in both the backend
and the rendered page.

## The cost of asking, accepted rather than hidden

Asked again rather than once at start-up, because every one of these changes while the window is
open: a build script replaces the sibling executables, a drive fills up, somebody moves the
library. A panel answered once would describe the machine as it was when Studio opened, in the
place somebody goes to rule exactly that out.

And asking has a cost: the library check creates and removes a file in somebody's routine library.
So the page polls readiness on its **own slow cadence** — ten seconds, against 400ms and 2.5s for
the others — and **never while a recording or a run is in flight**. A recording watches the
keyboard and the mouse in this very process; a run has a game in front. Neither is a moment to
touch somebody's library for a diagnostic. `Registry.Save` performs the same write, and `Routines`
skips the file either way: it is not a `.marco` and has no entrance.

## What did not move

- **No button is disabled.** The ask was to *show* actionable problems, and a control greyed out
  with the explanation somewhere else is worse than a control that refuses and says why. The
  existing refusals at Record, Save and Play are unchanged and still fire.
- **Nothing runs a child to ask its build.** `marco.exe` and `director.exe` have no version flag,
  and a panel polled for as long as the window is open must not start a process that performs real
  input. So the panel names the executables' **locations** and says nothing about their builds; a
  run record names the `marco` that performed it, which is where that answer lives. Held by
  `TestTheReadinessCheckStartsNothing`.
- **The recorder's refusal at `Start` stays.** This is an earlier, cheaper answer and not a
  replacement: `Begin` still returns *can't record on this platform*, because a platform fact
  checked at poll time and a hook that will not install are two different claims.

## Enforced by

- `plugins/studio` `TestAReadyWindowSaysWhatItIsAndPromisesNothing` — a machine with everything in
  place reports **no** problems, names its build, its two executables and its library, and still
  carries the limits. The last assertion is the one that matters: it checks three specific things
  the caveat must mention, on the clean path.
- `plugins/studio` `TestASiblingOnPathButNotBesideTheWindowIsReported` — the silent case, with a
  `marco.exe` planted on PATH and none beside the binary. Asserts the sentence says *PATH* and
  *different build*, because *not beside this binary* alone reads as pedantry.
- `plugins/studio` `TestAMissingPlaybackExecutableBlocksPlaybackAndSaysWhatToDo` — nowhere at all:
  blocks playback, in those words, with an action.
- `plugins/studio` `TestALibraryThatCannotBeWrittenIsReportedBeforeSave` — the library's path is a
  **file**, a state `os.Stat` reports as perfectly present.
- `plugins/studio` `TestALibraryThatRefusesAWriteIsCaughtByWriting` — **written because a mutation
  survived.** The test above is caught by `MkdirAll` failing, so it never exercises the write at
  all: replacing the probe with `os.Stat` passed it. Here the directory is real, `MkdirAll` succeeds
  without touching it, and an `icacls` deny means only putting a file in it finds out. It skips if
  the deny will not take, and checks that it took before relying on it.
- `plugins/studio` `TestTheLibraryCheckLeavesNothingBehind` — asked twice, nothing left, and nothing
  it writes is ever listed as a routine.
- `plugins/studio` `TestAWindowThatCannotNameItsBuildReportsIt` — both directions, so the claim is
  about the stamp rather than about the panel always complaining.
- `plugins/studio` `TestTheReadinessCheckStartsNothing` — no run slot claimed, no child started, and
  what the window was doing is unchanged by having been asked.
- `plugins/studio` `TestACleanMachineShowsNoProblemCardAndStillSaysWhatItDidNotCheck` /
  `TestAProblemIsShownWithWhatItStopsAndWhatToDo` — **rendered** in headless Edge or Chrome. A page
  that showed the card unconditionally would leave an empty red box above Record on every clean
  machine, and one that dropped the caveat when nothing was wrong would withhold it at the moment
  it is believed. Both pass a source guard.

## Mutations

Seven applied, all killed, plus a no-op control that survived. One of them changed the **tests**
rather than the code, which is the second time in this slice that the gate found the gap rather
than confirming the fix.

| | mutation | result |
|---|---|---|
| N1 | the playback executable is never checked | **killed**, both sibling tests |
| N2 | the library is checked with `os.Stat` instead of a write | **SURVIVED on its first run** — the only library test then in place was caught by `MkdirAll`. `TestALibraryThatRefusesAWriteIsCaughtByWriting` was written, and N2 is **killed** by it |
| N3 | `Limits` is set only when there are problems | **killed** on the clean path, in the backend and the page |
| N4 | the page shows the problem card unconditionally | **killed** — an empty card above Record |
| N5 | *on PATH but not beside* is treated as fine | **killed** |
| N6 | the write probe does not remove its file | **killed** |
| N7 | the page drops the *what to do* line | **killed** |
| CONTROL | the wording of one `Do` sentence is changed | **survived**, as intended |

And one defect was found by the browser test rather than by a mutation: starting `pollReady` inside
the bindings callback threw a `ReferenceError`, because that callback runs *synchronously* part-way
down a script whose later `let` declarations are still in their temporal dead zone — and
`pollReady` reads `runActive` before doing anything else. It took **the whole bootstrap** with it,
every other poll included, and silently, because a `-H windowsgui` binary has no console to print
it to. The other five polls survive that ordering only because everything they touch sits inside a
`.then`. The readiness poll now starts on the next tick, with the reason written down beside it.
