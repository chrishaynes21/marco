---
type: adr
status: accepted
date: 2026-11-02
decision: "ACCEPTED — THE LIBRARY IS THE SCREEN, AN OPEN ROUTINE IS A RUN STRIP AND A ROW OF TABS, AND ONE BUTTON IS FILLED. The Routines view opens with one header — the title, Record and a Help link — then the find box directly above the list. The lead paragraph, the three section labels, the build line, the readiness caveat and the library path are gone from it; the machine lines are Help's *About this installation*, painted by the same code as before. An active readiness PROBLEM, a library from before this one, the unrunnable-routines repair card and every refusal stay on the screen. An open routine keeps Rename beside its name and its run controls under it — the window it presses in with a one-line statement of whether its presses are checked (the full backend sentence is the chooser's title), and the values it needs. Everything else is a tab — Steps, Other names (with a count), Where it works, Source, More — none open when a routine opens, each panel directly under the row, and panels are HIDDEN rather than rebuilt so typed input survives switching. SIMPLIFY IS A VISIBLE BUTTON at the end of that row and never under More, by ruling, because it is the capability the owner asked for and twice could not find. Each review panel is titled by what it shows instead of `Nothing has been written yet` three times. EMPHASIS: a shut row's Play is outlined at the size the owner asked for in [[ADR-289-the-panel-says-less-and-play-is-bigger]]; the open routine's Play is the filled button; Record is filled only while nothing is open. An ordinary guarded routine carries no badge — `Unchecked` and `Won't run` remain. CONTRAST: `--faint` becomes #8c85a2 (dark) and #6e6680 (light) and light `--ok` becomes #277a45, so every text token reaches 4.5:1 on every surface in both themes; number fields get the field style they never had. COPY: `routine` is the user-facing noun across Studio, the overlay's listing and the README, with every CLI command, flag and protocol string unchanged (`marco plays`, `--play=`, `no play matches`). The recording band says `recording`, not `teaching`, and shows only the F9 instruction once the name field it advised about is gone. The README no longer says recording watches every application."
supersedes: []
affects:
  - execution
source_paths:
  - plugins/studio/frontend/index.html
  - plugins/studio/help.go
  - plugins/overlay/commands.go
  - README.md
---

# The library is the screen

## What happened

[[Studio-coherence-audit-2026-11-02]] measured the Routines view and found:

- the first routine started 549px down;
- an open routine with one required value was 280 visible words and 23 buttons, with six filled
  buttons on screen;
- the step editor opened below three unrelated controls.

The Architect approved the library-first layout and the compact expanded routine, with these
rulings:

> **Routine** is the user-facing noun; **Record** creates one and **Play** executes it. Align
> Studio, overlay copy and README. Preserve existing CLI commands and protocol names.
>
> **Outlined Play on collapsed rows:** approved, at the current size. The expanded routine's Play
> is filled. Record becomes secondary while a routine is open.
>
> **Tabs inside the expanded routine:** approved. Keep Rename beside the name and **Simplify…
> directly visible** — don't hide another frequently requested capability under More.
>
> Normal build information, paths and provenance can move into Help/About or Details. **Active
> installation problems, library conflicts and actionable refusals stay visible.**

## What is kept on purpose

- **Every element id.** Three were retired: `routineSecondLook`, its caption, and
  `routineWhereRow`. The tab buttons are the buttons they replace:
  - `routineStepsBtn` is Steps;
  - `routineWhereBtn` is Where it works;
  - **More is `routineMaintainSummary`**, because the desktop scenario that checks a routine's
    provenance presses it by name, and its panel is still `routineMaintain`.
- **The carried panel** of [[ADR-290-a-routine-opens-where-it-is]]. The tabs choose which part of
  the one panel is shown, and the move review is carried to where it was asked for: under the
  name box for a rename, into Where it works for a move.
- **The backend's sentence about the window.** The page shortens it using the same fact the
  backend reads (`contract`, in `handoffSays`). The full sentence is the chooser's title.
- **Protocol words.** `no play matches ` is matched by prefix between the engine and the overlay
  and is not copy. The web UI's own *Plays* tab is not Studio's and is left alone.

## Enforced by

- `plugins/studio` `TestTabsAndRedrawsKeepWhatWasTypedAndOneButtonIsFilled` checks:
  - no tab open when a routine opens;
  - Record not filled while a routine is open;
  - a shut row's Play not filled;
  - each tab opening its panel alone;
  - a typed wait and a typed phrase surviving tab switches, a forced redraw of the whole list,
    and an attempt to open another routine, which asks first.
- `plugins/studio` `TestEveryTextTokenMeetsContrastOnEverySurface` computes the ratio of every
  text token on every surface, and of each semantic colour on its own tint, from the stylesheet's
  tokens in both themes.
- `plugins/studio` `TestASecondLookAtARoutineIsOfferedWithoutOpeningADetail` checks that Simplify
  is in the tab row, is not under More, is shown on `offer`, and that the panel it opens is titled
  a review.
- `plugins/studio` `TestAStatusWordNeverClaimsAnOutcomeWasVerified` checks that a guarded row
  carries no word and the two exceptions still do.
- `plugins/studio` `TestTheJourneyPutsTheQuestionFirstAndPlayUnderWhatIsSelected` checks the
  order: Record in the header, then the find box, then the list.
- `plugins/studio` `TestEverySectionTheWindowLinksToExists` covers the two new Help sections,
  *Simplifying a routine* and *Other names for a routine*, which are each linked from where they
  apply.
- `plugins/studio` `TestARunnableConfirmationIsTheOnlyFilledButton` checks that while a run
  confirmation holds a token, *Run it* is the one filled button and the open routine's Play is
  unavailable; that a refused confirmation leaves Play live and filled as the way to ask again;
  and that Cancel brings Play back. Filled is read from the computed background, with
  transitions off.

Five mutations were run and each failed a test:

- the panel not parked before a redraw;
- the shut-row Play filled;
- the old `--faint`;
- a tab open by default;
- Record left filled.

## Correction: the run confirmation has one primary

The after-record first showed two filled buttons at the run confirmation: the open routine's
Play, repainted live as soon as the review came back, and *Run it*. The Architect ruled that
*Run it* is the sole filled primary while a confirmation is open, and that this is not an
exception to the design. `paintRoutinePlay` now treats a confirmation holding a token for the
open routine as a reason Play is unavailable; a disabled `go` button is drawn without its fill.
A refused confirmation has no *Run it* to press, so Play stays live and filled. Cancel is how
somebody returns to change a value. Two mutations were run and each failed
`TestARunnableConfirmationIsTheOnlyFilledButton`: the guard removed, and the guard applied to a
refused confirmation too.
