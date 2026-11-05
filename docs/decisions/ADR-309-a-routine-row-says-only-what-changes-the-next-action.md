---
type: adr
status: accepted
date: 2026-11-04
decision: "ACCEPTED — A ROUTINE'S ROW SAYS ONLY WHAT CHANGES WHAT SOMEBODY DOES NEXT. The window checks are how Marco works, not a status, so neither `Unchecked` nor `Window-checked` appears on a row, and the badge does not read the routine's contract at all. A ready routine has no badge. One that cannot run is badged `Can't run`, with a short reason and what to do beside its Play (`Routine.Cant`), and the same sentence leads the open panel. Where it runs and the window it will bring forward stay beside Play, which now just says `Brings this window forward.` How a routine's presses are checked, and the whole refusal, are under the routine's More tab, with what is checked in Help. The checks themselves are unchanged."
supersedes: []
affects:
  - interface
source_paths:
  - plugins/studio/frontend/index.html
  - plugins/studio/routines.go
  - plugins/studio/help.go
---

# A routine's row says only what changes the next action

## What a person saw

A routine saved before Marco measured windows carried an `Unchecked` badge on its row, and the
open panel said *Its presses are not checked* above Play and again beside it. An ordinary routine
said *Brings this window forward, and checks it before each press.* The owner:

> Window checks are implementation detail, not a useful routine status. Showing "Unchecked" makes
> people think they've missed a verification step.

There was no step to take. A legacy routine is one somebody chose to save that way, and a guarded
one checks its window whether or not it says so.

`Window-checked` had already gone from rows for a related reason: on every ordinary row it was the
badge people learn to stop reading ([[Studio-coherence-audit-2026-11-02]], finding 12).

## The decision

- **No badge for the window checks, in either direction.** The line that chooses a row's badge no
  longer reads `contract`. No other classification takes its place.
- **Ready has no badge.** That holds for guarded and legacy routines alike.
- **`Can't run` is the one badge,** and the row says beside its Play what is wrong and what to do.
  The backend writes that sentence (`Routine.Cant`) next to the whole refusal (`Routine.Why`), one
  per refusal:

  | Refusal | Short reason and remedy |
  |---|---|
  | saved to switch nothing | *It was saved without a window to run in. Save it again from its demonstration.* |
  | incomplete recording | *Its recording is incomplete. Record it again.* |
  | no longer legal Marco | *Its saved program has an error. Fix it in the source, or record it again.* |

  The open panel's red banner leads with the same sentence.
- **Where it runs stays visible.** The row still names where it works, and beside Play the panel
  names the window the run will bring forward: *Brings this window forward.* The backend's longer
  sentence is the chooser's title, as before.
- **The technical explanation is under More and in Help.** The legacy sentence (`routineChecks`)
  and the whole refusal (`routineWhyFull`, *Why it can't run: …*) are in the More tab. What is
  checked is Help's `checks` section, still linked beside Play.

## What did not change

- **The checks.** `orchestrator.ContractOf`, the guarded press and its refusal are untouched.
  `Routine.Contract` and `Routine.Checks` are still sent.
- **A refusal is still never collapsed.** `routineWhy` is outside every `<details>`. The checks
  line is no longer a blocking message, so it is no longer in that test's list.
- **No word on a row claims an outcome.** Marco presses and reports what the press returned.

## Consequence

A legacy routine's open panel no longer warns, above Play, that its presses land where they were
recorded whatever is there now. That is in its More tab and in Help. This was the owner's call:
it is how the routine works, and it is not something to act on before pressing Play.

## Verified (Windows only)

- **Rendered rows and panel** in headless Edge, with stub bindings
  (`TestARoutineRowShowsABadgeOnlyWhenItCannotRun`).
- **Mutants,** each applied alone and the file restored. All 11 were killed:
  - the `Unchecked` badge put back;
  - a `Ready` badge on every row;
  - the row showing the first sentence of the full refusal instead of `cant`;
  - the banner showing the full account;
  - the checks sentence put back beside Play;
  - the checks line moved back above Play;
  - the full refusal dropped from More;
  - each of the three `Cant` sentences removed;
  - a `Cant` set on a ready routine.

  The `switches nothing` mutant was killed by `TestARoutineThatCannotPressAnythingIsNotOfferedForPlay`
  once that test was included in the run. The first pass ran without it and reported the mutant
  as surviving.

## Not established

- **Not seen on the desktop.** No Studio window was opened for this. The page was rendered
  headless with stub bindings.

## Enforced by

- `plugins/studio` `TestARoutineRowSaysOnlyWhatChangesTheNextAction`: the badge line is
  `!r.runnable ? ['no', "Can't run"] : null`, does not read `contract`, and names no check or
  outcome. The line beside Play just names the window.
- `plugins/studio` `TestARoutineRowShowsABadgeOnlyWhenItCannotRun`, rendered:
  - guarded and legacy ready rows have no badge;
  - the row that can't run says `Can't run` with its short reason;
  - nothing in the list says "check";
  - the checks line and the whole refusal are under More;
  - the banner is the short sentence.
- `plugins/studio` `TestARoutineThatCannotPressAnythingIsNotOfferedForPlay`,
  `TestARoutineMissingAMeasurementIsNotOfferedForPlay`, `TestARoutineThatNoLongerChecksSaysWhatToDo`
  and `TestASavedRoutineIsListedAndReadyToPlay`: each refusal's `Cant`, and none on a ready routine.
- `plugins/studio` `TestThePageNamesTheOneWindowARunWillPressIn` and
  `TestThePageWillNotPlayUntilOneOfSeveralWindowsIsChosen`: beside Play is *Brings this window
  forward.*, with the backend's sentence on the chooser.
