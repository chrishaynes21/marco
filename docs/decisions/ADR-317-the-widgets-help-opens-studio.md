---
type: adr
status: accepted
date: 2026-11-05
decision: "ACCEPTED — DISCOVERABILITY, NOT NEW CAPABILITY. (1) A routine's collapsed row in Studio says what it asks for after its name, the way it is typed: `cloud pull ‹resource›`. The names are read by `routes.Arguments` (the reader Play and `marco args` use) and written by `routes.Slots`, now shared with the widget's line, so the list and the line cannot spell it differently. A secret is not shown, because the credential store answers it. (2) The widget's `help` opens Studio on its Help (`studio.exe -view help`, found beside `$MARCO_BIN` or named by `$MARCO_STUDIO`), not the browser control centre. Studio is one window per library (Wails' single-instance lock, keyed by the library's path), so with Studio already open that window comes forward on Help and the second launch exits. A harness or proof launch is never locked. With no Studio installed, `help` opens the control centre's help and says why."
supersedes: []
affects:
  - interface
source_paths:
  - plugins/studio/routines.go
  - plugins/studio/single.go
  - plugins/studio/help.go
  - plugins/studio/main.go
  - plugins/studio/frontend/index.html
  - plugins/overlay/acts.go
  - internal/routes/args.go
---

# The widget's help opens Studio

## The request

> The capabilities are accounted for; we don't need another porting exercise. What remains is
> discoverability and consistency: show `cloud pull ‹resource›` on the collapsed row, using its
> declared argument name, and make `m help` open Studio's Help, so the normal entry point leads to
> the interface you're using.

## Decision

- **The row says what the routine asks for.**
  - `Routine.Takes` is written after the name, muted.
  - The names come from the source by `routes.Arguments`, the same reader Play's boxes and
    `marco args` use.
  - They are written by `routes.Slots`, which `cmd/marco/line.go` now calls too: `‹resource›` for
    one value, `name:‹name› message:‹message›` for several. That is what the widget shows while
    the name is typed.
  - A secret is left out. No box is offered for one; the credential store answers it.
- **`help` opens Studio's Help.**
  - The widget starts `studio.exe -view help`. It finds Studio by `$MARCO_STUDIO`, or beside
    `$MARCO_BIN`. `overlay.cmd` and the installed launcher both set `$MARCO_BIN` to an absolute
    `marco.exe`, and Studio is beside it in both layouts.
  - `-view` opens on a view the rail offers, and only on one. `Features.Start` carries it to
    the page.
  - With no Studio there, `help` opens the control centre's help and says so.
- **One Studio window per library.**
  - Without this, `help` with Studio open would make a second window onto one library: two
    recorders, and two step editors that can save the same routine.
  - A second launch on the same library hands its arguments to the open window and exits. The
    open window is unminimised, shown, and sent `studio:view` with the view asked for. `show`
    refuses a view the rail withholds, whoever asks.
  - The lock is keyed by the library's path, case-insensitively, so Studios on two stores do not
    stand in for each other.
  - A launch with `-isolate`, `-seconds`, `-evidence`, `-autostart` or `-autocancel` is never
    locked. Those are harnesses and proofs, and must report on their own window.
- **Not the control centre's hook guard.** The widget starts the control centre with
  `MARCO_NO_PANIC_STOP`, but not Studio. Studio is the person's own window and outlives the
  widget, so its Plays keep the stop key they have when Studio is opened any other way.

## Verified (Windows only)

- **Studio:**
  - Through `Acts.Routines` on a real library: one value reads `‹resource›`, two read each
    named, none reads nothing, and a secret is not shown.
  - Rendered in headless Edge: the row reads `cloud pull ‹item›`, the placeholder is muted, and a
    routine with no values has none.
  - Rendered: a window opened with `start: help` is on Help. A later launch's `studio:view` moves
    it to Routines and back, and cannot open a withheld view.
  - `viewArg` reads the three spellings `flag` accepts. The lock exists for an ordinary launch
    and is absent for each harness flag. Two spellings of one path share it, and two libraries
    do not. A later launch's arguments reach the open window.
- **Widget:** `help`, `h`, `?` and `manual` start `studio.exe -view help`. With no Studio they
  start `marco ui help`, and the line says why. Studio is found beside `$MARCO_BIN`, and
  `$MARCO_STUDIO` wins.
- **Mutants:** 16 behavioural, all killed:
  - Studio: the row never says it, a secret on the row, not the typed form, the page dropping
    it, not muted, the start view ignored, a later launch not heard, any view offered, a
    harness launch handing off, path case splitting the lock, arguments not handed on;
  - widget: help still the control centre, Studio not on Help, not looked for beside marco,
    the override ignored, the fallback unexplained.

## Not established

- **Not seen on the desktop.** Nothing here has been launched. In particular:
  - whether the open Studio actually comes to the FRONT is unproven. Windows' foreground rules
    may only flash its taskbar button, because the request comes from a process that was not in
    front;
  - an exit-0 hand-off is reported by the widget as *already open* and has only been reasoned
    about, never observed.
- **`main` passing the lock to Wails is not under test**, because `main` runs the window.
- A Studio built before this does not know `-view`, exits with a flag error, and the widget says
  it could not open Studio's Help. Rebuild with `plugins\studio\build.cmd`.

## Enforced by

- `plugins/studio` `TestARoutineRowSaysWhatItAsksFor`,
  `TestARoutineRowDrawsWhatItAsksForAfterItsName`, `TestStudioOpensOnTheViewItWasAskedFor`,
  `TestALaunchNamesTheViewItWants`, `TestOnlyAnOfferedViewCanBeOpenedOn`,
  `TestStudioIsOneWindowPerLibraryAndAHarnessHasItsOwn`,
  `TestALaterLaunchsArgumentsReachTheOpenWindow`.
- `plugins/overlay` `TestTheHelpWordOpensStudioOnItsHelp`,
  `TestTheHelpWordOpensTheControlCentreOnlyWithoutStudio`, `TestStudioIsFoundBesideMarco`.
