---
type: adr
status: accepted
date: 2026-11-03
decision: "ACCEPTED — STUDIO HAS A SECONDARY OPEN WIDGET ACTION THAT STARTS THE INSTALLER'S OWN WIDGET STACK BESIDE ITSELF, NEVER A SECOND ONE, AND NEVER ONE BUILT FROM MIXED SOURCES. The action is in the rail below the destinations and in Help (Use Marco while you play). It starts `marco serve` with the macros bridge and the overlay, exactly the installer's `Marco.exe --overlay` command (`internal/widgetstack`, held to the installer's source by a test), from the installed or the tree layout beside Studio, with Studio's library, store and marco stated to the child. Any Marco widget already running (an overlay.exe with overlay.marco beside it, from anywhere) is reused and said, and a launch already in progress is not repeated. Every piece must be present and every Go program in it must carry the same known build revision as this Studio (`buildid.Same`, read from the files without running them), so the mismatched binaries at the repository root are refused and each piece is named. States: ready, starting, running, unavailable, failed (a start refused, or a stack that ended within four seconds, with the launcher to run by hand). REPAIRED 2026-11-04: in a development tree, recognised by its source (go.mod and plugins\\studio\\build.cmd), the stack is the ROOT's from either studio.exe build.cmd makes; a tree's fix is that exact build command, an installation's is still a reinstall; and plugins\\studio\\build.cmd builds Studio and every piece of the stack from one clean commit, staged and stamp-checked against HEAD before anything is replaced."
supersedes: []
affects:
  - interface
source_paths:
  - plugins/studio/widget.go
  - plugins/studio/frontend/index.html
  - internal/widgetstack/widgetstack.go
  - internal/buildid/buildid.go
  - plugins/studio/build.cmd
---

# Studio opens the widget

The proposal's "Start the widget from Studio (R5)", run as S7 by the owner's renumbering. It is §5.7
of [[Surfaces-alignment-proposal-2026-11-02]].

## The trace

Before building anything, I traced how the widget is started today.

- **Installed:** `Marco.exe --overlay` (`plugins/marco-app` `runOverlay`). It runs `marco.exe serve
  --host OS=bridge:marco-macros.exe --host Overlay=bridge:overlay.exe overlay.marco` from
  `%LOCALAPPDATA%\Marco\bin`, with `MARCO_BIN` set to that `marco.exe` and nothing about the
  library.
- **Tree:** `overlay.cmd` at the root runs the same `serve` with the overlay under
  `plugins\overlay`, and pipes the voice plugin into it.
- With bridge hosts present, `serve` does not end when its stdin does (`internal/driver`
  `mergeSources`). A child given no stdin therefore keeps running.
- The installer is its own module, named `marco-app`, and Go's internal rule keeps it from
  importing an engine package. Its command therefore stays where it is. `internal/widgetstack`
  `Args` is the same command for a layout, and `TestTheStackIsTheOneTheInstallerStarts` reads the
  installer's source and fails if the two drift.
- **What the root binaries carry, read with `go version -m` (nothing was run):**
  - `marco.exe` and `studio.exe` are `6c0498b`;
  - `director.exe` is `922220f+dirty`;
  - `marco-macros.exe` is `fcc5f07+dirty`;
  - `overlay.exe` is `b84fd52+dirty`.

  The mismatch is in the stamps, not only in the file times.

## What was decided

- **A secondary action, discoverable.** It sits in the rail, below the destinations and quieter
  than them, and again in Help in a card that says what the widget is. The button's label is the
  state. The line under it in the rail appears only when something is wrong, and Help lists every
  problem with what to do. Every word comes from the backend.
- **The installer's stack, beside this Studio** (`widgetstack.Beside`). That is the directory
  Studio already spawns `marco.exe` and `director.exe` from, so the widget performs with the
  engine Studio plays with. There are two layouts:
  - **installed:** every piece flat;
  - **tree:** the overlay under `plugins\overlay`.

  *Corrected 2026-11-04:* in a tree, the stack is the ROOT stack from either `studio.exe`. See
  "Repaired: the launch and build contract" below.
- **What the child is given.** Studio's environment, plus three variables, each stated once:
  `MARCO_BIN` (the marco beside it, as the installer sets it), `MARCO_HOME` and `MARCO_ROUTES`
  (Studio's own, already absolute from `internal/marcopaths`). The widget therefore uses the same
  library and store as Studio, and reads the same `overlay.json`, because `MARCO_OVERLAY_CONFIG`
  is inherited.
  - Nothing about the application in front is passed: the widget reads it itself, at each press.
  - The console is hidden (`hideConsole`), so no window takes the foreground.
  - The child's stdout and stderr go nowhere. Studio keeps none of the widget's output, so
    nothing typed into it is written anywhere by this.
- **Never two.** A running Marco widget, from this installation or any other, suppresses a second
  launch. Only the widget THIS window started is described with Studio's context: the overlay
  whose parent is the `marco serve` it started, while that is still running. That one reads
  *The widget is running (process N) — press <leader> over your game*. Any other reads *Widget
  already running from <location>. Its library and active settings haven't been verified.* An
  executable's location says nothing about which library it reads or which leader it listens
  for, and there is no channel to ask. It is not reconfigured or ended. Here "a Marco widget" means an `overlay.exe` with `overlay.marco`
  beside it, so an unrelated `overlay.exe` is not taken for one. A launch this window has in
  progress is not repeated.
- **Never mixed.** Every piece must exist, and every Go program among them must carry the same
  KNOWN revision as this Studio (`buildid.Of`, read from the file with `debug/buildinfo`, never by
  running it). A piece with no stamp cannot be vouched for and is refused like a different one.
  This is what keeps the root stack blocked. The check is asked again after a failed launch, so a
  retry over a piece that has since changed is refused too.
- **The states:**

  | State | Meaning |
  |---|---|
  | ready | It can be opened. |
  | starting | Started, and inside the first four seconds. |
  | running | A widget is up, whoever started it. |
  | unavailable | Each missing or mismatched piece is named. |
  | failed | The start was refused, or the stack ended inside four seconds. It names the exit and the launcher to run from a terminal to see why. It may be tried again. |

  A widget closed after it was up is not a failure: the action is ready again.

## Renders

These are renders of the real page in headless Edge, with the backend stubbed: `window.go` answers
each state. They are the page's own layout and text, not a desktop run.

| ready | running |
|---|---|
| ![ready](../images/studio-ui/s7-widget-ready.png) | ![running](../images/studio-ui/s7-widget-running.png) |
| **unavailable** (this tree's root stack) | **failed** |
| ![unavailable](../images/studio-ui/s7-widget-unavailable.png) | ![failed](../images/studio-ui/s7-widget-failed.png) |

## Enforced by

- `plugins/studio`: `TestTheWidgetIsOpenedAsTheInstallerOpensIt`, `TestTheWidgetIsNeverStartedTwice`,
  `TestTheWidgetIsOnlyOpenedWhenItCanBe`, `TestALaunchThatFailedSaysSo`,
  `TestAWidgetClosedLaterIsReadyAgain`, `TestTheTreeStackIsFoundFromEitherStudio` (since 2026-11-04, replacing a tree test that covered only a Studio at the root),
  `TestAFailedLaunchIsNotRetriedOverAMismatch`, and `TestEveryMethodThePageCallsIsOnTheBackend` for
  the page's two calls. Every outward act is a seam:
  - the directory beside, filled with stand-in files;
  - the process list;
  - the start and the wait;
  - each file's revision.

  No test starts a process.
- `internal/widgetstack`: `TestTheStackIsFoundWhereEachLayoutPutsIt`,
  `TestTheStackIsTheOneTheInstallerStarts`, `TestAnOverlayIsOneWithItsProgramBesideIt`.

Mutations: 13 runs, all killed. In Studio:
- the running-widget check;
- the in-progress check;
- the problem check on a retry *(survived first; `TestAFailedLaunchIsNotRetriedOverAMismatch`
  added)*;
- `Same` relaxed to `Differ`, which would let unstamped pieces through;
- the child's environment;
- the early-exit window;
- taking any `overlay.exe` for a widget;
- the working directory;
- keeping the widget's output;
- the missing-piece check.

In the stack package:
- the host order against the installer's;
- recognising an overlay with no program beside it.

## Corrected after commit (owner review)

The first version announced Studio's own configured leader for ANY running widget, including one
started elsewhere with other settings. Now an instance whose context cannot be verified is said to
be running from its location, with its library and settings unverified, and only Studio's own child
is given Studio's leader. `TestAWidgetStartedElsewhereIsNotGivenStudiosContext` checks three
cases: this installation's own `overlay.exe` started some other way, an unrelated overlay while
Studio's launch is alive, and an overlay outliving Studio's `marco serve`. None of them acquires
the CapsLock leader Studio is configured with, and a duplicate launch stays suppressed.

Mutations on the correction, 4 runs, all killed:

- claiming every widget;
- dropping the parent check *(survived first; the unrelated-overlay case was added)*;
- treating an ended stack as alive;
- not recording the started pid.

## Checks

Windows only, the affected checks. Passed: `go build ./...`; `buildid`, `widgetstack`, `gate`,
`wiringcheck` and `spectest`; gofmt; docscheck. Binary timestamps unchanged.

The whole Studio package went through three runs:

1. **The first run FAILED 15 page tests.** Studio's page harnesses render the real page in
   headless Edge, and treat any backend call their stub does not answer as a thrown page. That is
   the convention, and it is what caught this: the page now polls `WidgetStatus` from start-up.
2. Each harness, 21 files plus `testdata/shortcutspage.js`, gained a `WidgetStatus` answer. The
   page was not given a guard, which would have hidden a missing binding. The second run left one
   failure, the harness kept as a JS file, which had been missed.
3. The third run passed.

All three outputs are kept in `docs/experiments/Roadmap-38-output/`:
`2026-11-03-s7-{engine,studio,studio-after-stubs,studio-final}.txt`.

## Not established

- **Nothing was launched**, by instruction. The command, the environment, the hidden console and
  the four-second window were not exercised against a real stack. No rebuild was done, so the
  binaries beside this tree are still the mismatched ones, and Studio would show the widget as
  unavailable here.
- **Voice is not started.** The installer's stack has no voice plugin. The tree's `overlay.cmd`
  pipes one in, and Studio's launch does not.
- The widget is not brought forward or shown when it is already running: there is no channel to it
  for that. Studio says it is running and which leader to press.
- "A Marco widget is running" is any `overlay.exe` with `overlay.marco` beside it. One started
  from a stack Studio did not start is reported but not adopted: closing Studio leaves it, and the
  four-second failure window applies only to one Studio started.
- The revision check needs every piece built from one commit. In a development tree that is a
  whole rebuild: `marco-macros.exe` and `overlay.exe` are rarely rebuilt, and they block the action
  until they are. *(2026-11-04: `plugins\studio\build.cmd` now rebuilds them. See below.)*
- F6 (one meaning for Enter) and Roadmap 38 remain open and are unchanged, as is the
  binary-mismatch warning.

## Repaired: the launch and build contract (2026-11-04)

Open widget could not work in the development layout that Studio's own build script produced.
There were two separate faults.

1. **Studio looked only beside itself.** `plugins\studio\build.cmd` puts a `studio.exe` in
   `plugins\studio` and another at the root. From `plugins\studio`, `widgetstack.Beside` found no
   overlay beside it or under `plugins\studio\plugins\overlay`. It fell back to the installed
   layout's paths, reported every piece missing there, and said "reinstall Marco" to a developer.
2. **The build script never built the widget.** It made Studio, the director and marco, but not
   `marco-macros.exe` or `overlay.exe`. A freshly built Studio was therefore always a different
   revision from the bridge and the overlay, and the check refused it. That refusal is correct.
   In this tree, before the repair, Studio, marco and director were `1c2d35f+dirty`,
   `marco-macros.exe` was `fcc5f07+dirty` and `overlay.exe` was `b84fd52+dirty`.

### The contract now

- **Where the stack is** (`widgetstack.Beside`, `TreeRoot`):
  - **A tree is recognised by its source,** not its build outputs: a root holding `go.mod` and
    `plugins\studio\build.cmd`.
  - **Either studio.exe finds the root's stack.** From the root, or from that root's
    `plugins\studio`, the stack is the root's `marco.exe`, `director.exe` and
    `marco-macros.exe`, with `overlay.exe` and `overlay.marco` under `plugins\overlay`.
  - **The working directory and `MARCO_BIN`** are the root and the root's marco.
  - **Unbuilt pieces read as missing at their tree paths.** The outputs are what is being
    checked, so they cannot also decide which layout is checked, and an unbuilt overlay never
    reads as a missing installation.
  - **A stray flat overlay at a tree's root does not make the tree an installation.**
- **The installed layout is unchanged:** every piece flat beside Studio. Anything that is not a
  tree, including an installation that happens to sit in a folder called `plugins\studio`, is
  read as installed.
- **The checks are unchanged.** Every piece must be present, and every Go program must carry
  the same known revision as the Studio asking (`buildid.Same`).
- **What to do** (`Layout.Rebuild`):
  - **In a tree,** each problem names the exact command: close Studio, commit or set aside any
    change, then run `<root>\plugins\studio\build.cmd`.
  - **In an installation,** it is still "reinstall Marco".
  - **Never a copy.** Nothing offers to copy a piece in, because the check exists to refuse
    exactly that.
- **One build command:** `plugins\studio\build.cmd`, from the repository. It builds Studio with
  the Wails tags (`desktop,production`) and every piece of the stack, from one commit, into
  every place either Studio and the widget look. In order, it:
  1. **refuses a tree with any change** `git status` reports. A dirty tree stamps every binary
     `+dirty`, and two different dirty builds carry the same stamp.
  2. **refuses when any process is running from a destination file,** and names each one.
     Windows cannot replace a running executable, so carrying on would leave a mixed stack.
  3. **builds everything into a temporary directory:**
     - Studio from its module;
     - the director, marco and `marco-macros` from the root module;
     - the overlay from its own module (`go -C plugins\overlay`), as `pack.ps1` builds it.
  4. **checks every stamp** with `go version -m`: `vcs.revision` is HEAD and
     `vcs.modified=false`.
  5. **only then copies each piece into place:**
     - `plugins\studio\`: `studio.exe`, `studio-console.exe`, `director.exe`, `marco.exe`;
     - the root: `studio.exe`, `director.exe`, `marco.exe`, `marco-macros.exe`;
     - `plugins\overlay\`: `overlay.exe`.

     A copy that fails part-way says the stack is now mixed, and the revision check refuses it
     until the script is run again.

### Enforced by (the repair)

- `internal/widgetstack` `TestTheStackIsFoundWhereEachLayoutPutsIt`:
  - both tree locations, case-insensitively;
  - an unbuilt tree;
  - a stray flat overlay;
  - neither marker alone;
  - an installation in a `plugins\studio`.
- `plugins/studio`:
  - `TestTheTreeStackIsFoundFromEitherStudio`: the command, the working directory and
    `MARCO_BIN`, from both;
  - `TestATreeIsToldTheBuildCommand`: missing, mismatched and unstamped pieces at their tree
    paths, each with the build script and never "reinstall" or a copy; an installation's
    mismatch still says "reinstall Marco";
  - `TestTheBuildScriptMakesTheWholeStackFromOneCommit`, which reads the script's commands with
    the comments stripped:
    - each piece built from the right module, Studio with the Wails tags;
    - the refusals placed before the build;
    - the stamp check covering every staged binary;
    - no copy before it;
    - each piece copied to every path `widgetstack.Beside` resolves from both Studio locations.

  It replaces the earlier director-and-marco build-script test in `console_test.go` and keeps all of its
  requirements.

Mutations: 18, all killed. Two first attempts did not apply to the file and were rewritten. In
`widgetstack` and Studio:

- recognising the tree only from its root, checked in both packages;
- go.mod alone as the marker;
- the tree's root taken as Studio's own directory;
- `Rebuild` always "reinstall", and the fix hard-coded to it;
- the overlay looked for flat in a tree.

In the script:

- dropping the `marco-macros`, overlay or `plugins\studio` director copy;
- the dirty and running refusals not exiting;
- no `vcs.modified` check;
- a revision check that accepts any revision;
- a copy before the check;
- the overlay left out of the check;
- the Wails tags dropped;
- `marco-macros` built straight into place.

### Verified with temporary build outputs (Windows only)

Nothing in the owner's tree was rebuilt or replaced, and nothing was launched. The script ran in
a scratch clone of 17e3a7f with this change committed in the clone only (`ee08908`). Outputs are
in `docs/experiments/Roadmap-38-output/2026-11-04-widget-launch-*.txt`.

- **With one untracked file,** it refused and built nothing.
- **Clean, it built all nine outputs.** `go version -m` read `vcs.revision=ee08908` and
  `vcs.modified=false` on each. `studio.exe` does not contain the Wails guard's sentence. The
  stage directory was removed afterwards.
- **With a stand-in process running from a destination,** it refused, named the process and
  built nothing. The stand-in was a copy of Windows' `ping.exe` at the overlay's path, so no
  Marco binary ran. A first attempt at this was refused as dirty instead, because the real
  overlay had been set aside inside the clone. Its output file was overwritten by the second
  attempt, so it is not kept.
- **`WidgetStatus` on those real files.** This was a temporary test in the clone, with the
  process list stated empty and the start set to fail if called.
  - From the root and from `plugins\studio`: ready, as the tree at the root.
  - With the owner's own `marco-macros.exe` (`fcc5f07+dirty`) copied into the clone: refused,
    naming it, with the clone's build script as the fix.
  - With the overlay absent: refused at `plugins\overlay\overlay.exe`.
  - The same pieces laid out flat: ready, as installed. With the overlay removed: refused, with
    "reinstall Marco".

### Checks (the repair; Windows only, affected checks)

- **Passed:** `go build ./...`, `go vet` (`widgetstack`, `buildid`, `plugins/studio`), gofmt,
  `internal/widgetstack`, `internal/buildid`, `cmd/gate`, and docscheck.
- **The full `plugins/studio` suite** passed on its first run.
- **`internal/wiringcheck` FAILED on its first run.**
  - `TestEveryTestNamedInTheDocsVaultExists` found four lines still naming the two replaced
    tests, in "until 2026-11-04" notes in ADR-245, ADR-252 and this ADR.
  - The guard is right. The lines now describe the old tests without naming them, and the
    second run passed.
- **The full engine suite was not run.**

### Not established (the repair)

- **Not launched.** No widget has been started from either Studio location. The command,
  environment and four-second window are still unexercised against a real stack. Desktop
  confirmation is pending.
- **The owner's tree is still mixed** until `plugins\studio\build.cmd` is run there from a
  clean commit, with Studio and the widget closed.
- **A partial copy cannot be undone by the script.** If a copy fails after earlier ones
  succeeded, the stack is mixed until the script is run again. The revision check refuses it
  meanwhile, and the script says so.
- **The running-process check compares exact paths.** A process whose path Windows does not
  report to this user is not seen. Its file then fails to copy, which is the mixed case above.
