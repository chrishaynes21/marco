---
type: adr
status: accepted
date: 2026-10-27
decision: "ACCEPTED — THE ALPHA IS THE STUDIO STACK, SHIPPED, AND IT SAYS WHAT IT IS. `pack.ps1` built the engine, the Director, the accessibility provider and the overlay, and the launcher ran the overlay serve stack; no root script mentioned Studio and the README never did. So record → Stop → review → Save → Play, and every guard in [[ADR-251-a-press-checks-the-window-it-was-recorded-in]], [[ADR-259-a-guarded-drag-is-two-points-in-one-window]], [[ADR-260-a-press-is-a-button-and-a-place]], [[ADR-261-typed-text-is-a-sequence-of-submissions]] and [[ADR-262-a-routine-states-which-generation-wrote-it]], reached NOBODY who installed Marco. And the consequence is sharper than a missing feature: the shipped overlay can record and can simplify, and it sets the no-preview option because it cannot render two sources — so an installed Marco could WRITE a routine and could not show anybody what it was about to write, which makes every guard that depends on a review unreachable. Studio is therefore staged and OPENED, built with the Wails tags, and `pack.ps1` checks the built bytes for the guard's own message box because that is the only way to tell a windowless build from a working one without opening a window on somebody's desktop. The overlay is still shipped, still works, and starts only with `--overlay`: it is a different product surface rather than an older version of this one, and it is asked for. `marco.exe` and `director.exe` land BESIDE `studio.exe`, because Studio spawns the ones beside ITSELF — one layout satisfies that and `marco`'s own two lookups at once. AN INCOMPLETE BUNDLE IS A REFUSAL: `marcoBin`/`directorBin` answer a bare name when there is no sibling, which `exec.Command` resolves against PATH, so an installed Studio with a missing sibling performed routines through a binary nobody chose, of whatever age, with every window looking current — readiness had reported it for a while and blocked nothing, and Play, Run and Learn refuse it now. An upgrade blocked by a running Studio SAYS to close it rather than killing a window that may hold a draft existing nowhere else. A BUNDLE CARRIES A MANIFEST: every file's hash, and every Go binary's revision asked of the binary itself through `go version -m`, with a binary that carries no revision FAILING the pack rather than shipping — `unknown` is what a package built from outside its module answers, and a bundle of those makes every run record unciteable. The accessibility provider has no such stamp, being built by the .NET compiler that ships with Windows, so it is identified by its bytes; Studio reports all of it, verifies the hashes against disk because a manifest is a claim, and prints it without opening a window, since a bundle that will not start is exactly the case where the report has to be reachable without starting it. GLOBAL RECORDING IS DISCLOSED BEFORE RECORD, in the window and in the README: the hooks are global, every press and key anywhere on the desktop enters the demonstration with Studio's own interactions excepted, and recording only a chosen window is NOT something Marco can do — this alpha does not claim it. THE README LED WITH THE WRONG PRODUCT, promising 'nothing to record' and saying `performed` means Marco looked again and confirmed it; `performed` on a routine means every action was SUBMITTED and none refused, because `internal/outcome`'s own definition is conditional — 'for a Play whose arrival could be checked, it means checked' — and the README was reading the condition off. VALIDATED BY RUNNING THE ARTIFACT from an isolated directory with no repository and no development PATH, which found three defects no unit test could reach: a manifest written with a byte-order mark that Go's JSON refuses, a manifest listing the embed's own version label as an installed component, and the launcher's closing line still describing the overlay. The harness can now test an INSTALLED Marco — launch, seeded playback, cancellation, reporting, review/cancel/replace, all green against the bundle. AND THE WORK FOUND A 30-SECOND HANG ON THE SHIPPED PLAY PATH: a run that finished its program reported nothing, slot still claimed, one run in six under load. Deductively one of the two unbounded channel receives either side of the pump wake, since the wake itself has been bounded since the 600-second timeouts; both are bounded now with their own sentences, and the tree-walking test refuses any bare receive in a hook site's shutdown rather than putting the bound in its prose. AND THREE P1s THE REVIEW FOUND, THE FIRST OF THEM THIS SLICE'S OWN: the launcher SET `MARCO_HOME` and `MARCO_ROUTES` to its own installation directory, overriding an explicit setting and pointing away from the shared `%APPDATA%\marco` every other way into Marco resolves — an existing Studio user would have installed the alpha and been shown an EMPTY LIBRARY — so installation and data resolution are separate now, the launcher seeds nothing, and the older bundled installation's data is NAMED by the window through `marcopaths.Elsewhere`, which ADR-252 promised and which had no caller at all; an interrupted upgrade LEFT MIXED BINARIES, because components were written into `bin\` in turn and Windows refuses to overwrite an executing file, so each version now stages into its own directory and `bin\` is replaced only when every file is written, with the old one put back if the swap fails; and RECORDER WORKERS COULD REACH A LATER RECORDING after a bounded shutdown gave up — both are fenced by session, with the poller's check AFTER the sample because the read is what blocks, and a further recording is refused outright because the hooks are C callbacks a session number cannot reach. PHYSICAL RECORDING CAPTURE REMAINS NOT EXERCISED."
supersedes: []
affects:
  - execution
  - interface
source_paths:
  - pack.ps1
  - plugins/marco-app/main.go
  - plugins/studio/readiness.go
  - internal/recorder/recorder_windows.go
  - internal/stopsignal/stopsignal.go
  - cmd/director/runrecord.go
  - plugins/e2e/bundle.go
---

# The alpha is the Studio stack, and it says what it is

## What was shipped, and what it cost

`pack.ps1` staged `marco`, `marco-macros`, `director`, `overlay` and `uia`; `plugins/marco-app`
embedded them and the launcher ran the overlay serve stack. **No root script mentioned Studio and
the README never did.** Studio existed only as a development build from `plugins/studio/build.cmd`.

So five slices of work — the guarded press, the guarded drag, the press's button, typed text, and
the generation stamp with its review flow — shipped in nothing. A person who installed Marco got a
hotkey and a command line.

**And the consequence is sharper than "a feature is missing".** The shipped overlay *can* record
and *can* simplify, and when it spawns the simplify child it sets the no-preview option —
correctly, because it has no way to render two sources. So an installed Marco could **write** a
routine and could not show anybody what it was about to write. Every guard that depends on a
review was unreachable to whoever installed it, which is the half of the work that the last two
slices were almost entirely about.

Shipping Studio is therefore not polish. It is what makes the work reachable.

## Studio is the front door, and the overlay is asked for

The overlay is **not removed**. It is a different product surface rather than an older version of
this one: a command line over a hotkey, with no way to show anybody what a recording became before
it is saved. That made it the right thing to ship when it was the only thing there was, and it
makes it the wrong default now. It starts with `--overlay`.

**Built with the Wails tags, and the build is checked.** A plain `go build` of Studio produces a
binary that opens NO WINDOW: without `production`, Wails compiles a `CreateApp` that shows a
message box reading *Wails applications will not build without the correct build tags* and exits.
The binary is otherwise ordinary — it has the flags, it prints usage, `-acts` and `-proof` both
work, because they return before the app is created — so one sat in the tree looking current until
somebody double-clicked it. `pack.ps1` reads the built bytes back and looks for that sentence,
which is the only way to tell the two apart without opening a window on somebody's desktop.

**One layout for three lookups.** Studio spawns the `marco.exe` and `director.exe` beside ITSELF;
`marco` looks for `director.exe` beside its own executable and the provider at
`plugins\uia\uia.exe` under it. All of them land in `bin\`, which satisfies every one of those at
once.

## An incomplete bundle is a refusal, not a fallback

`marcoBin` and `directorBin` answer an absolute path when the sibling is there or when somebody
named one, and a **bare name** when neither is true. A bare name handed to `exec.Command` is
resolved against PATH — so an installed Studio with a missing sibling performed routines through a
binary nobody chose, of whatever age, while every window in sight looked current. That is the
defect that cost the owner a day of recordings, arriving from the one direction a packaged install
can take it.

`Ready` has reported this for a while and **blocked nothing**: the problem it raised for the
on-PATH case named what it blocked as nothing, so the window warned and then spawned it anyway.
Play, Run and Learn refuse it now. The explicit `$MARCO_BIN`/`$DIRECTOR_BIN` overrides still win —
they are statements by whoever started the window.

**And an upgrade blocked by a running Studio says so.** Windows refuses to overwrite an executing
file, so an upgrade while Studio is open fails during extraction, and "Access is denied" sends
somebody to look at permissions for a thing that is simply open. Nothing is killed: a window that
is recording, or holding a draft that exists nowhere else, is not something an installer may close
on somebody's behalf.

## A bundle says what it is made of

`version.txt` is a label this script chose. A label is not an identity: it says what the repository
was at pack time, and it would say the same about a bundle whose `studio.exe` had been replaced by
hand.

So each **Go** binary is asked what IT says, through `go version -m`, which reads back the VCS
stamp `go build` left. A binary carrying no revision **fails the pack** rather than shipping:
`unknown` is what a package built from outside its own module answers — the mistake `build.cmd`
made with the director once — and a bundle of those makes every run record unciteable, when
`build` is the field that would have named the stale recorder.

The one piece that is **not Go** has no such stamp. The accessibility provider is built by the
.NET Framework `csc.exe` that ships with Windows, so it is identified by the hash of its bytes,
which is the only identity it has. Every file gets a hash regardless: a revision says which SOURCE
a binary came from, a hash says whether THIS is that binary.

Studio reports all of it and **verifies the hashes against disk**, because a manifest is a claim
and a claim about files anybody can replace is worth what the check behind it is worth. It prints
it **without opening a window**, which is the right way round: a bundle that will not start is
exactly the case where the report has to be reachable without starting it.

A development build has no manifest and never will. That is reported as what it is rather than as
a problem — a diagnostic that cried about a development build is one nobody reads on an
installation.

## Two claims the product was making and should not have been

**Recording is not bounded to a window.** The hooks are global: every press and key anywhere on
the desktop enters the demonstration, attributed to whatever window was in the foreground at that
instant, with Studio's own interactions the one exception. A person demonstrating is demonstrating
the whole desktop — answer a message halfway through and the reply is in the routine — and the
product said so nowhere. The window says it before Record now, and says the other half too:
recording only a chosen window is not something Marco can do yet. See [[Demonstrations]] and
Roadmap 36 for why the playback window CHECK is a different thing and does not cover it.

**`performed` does not mean Marco checked.** `internal/outcome` defines it conditionally — *"for a
Play whose arrival could be checked, it means checked"* — and the README was reading the condition
off, promising that `performed` means Marco looked again and confirmed. For a coordinate routine
nothing can check: `performed` means every action was **submitted to the machine and none was
refused**. What a routine *does* promise is stated instead, because it is worth having: a measured
press follows a window that moved, refuses one that changed, and stops what follows it.

The semantic path keeps its promise and is now named as a **different** one: an accessible control
can be read back afterwards, so there `performed` can mean checked. One replays a demonstration,
the other recognises a control, and only the second is in a position to confirm anything.

## Validated by running the artifact

From an **isolated directory**, with `LOCALAPPDATA` pointed away from the owner's own install, no
repository, and PATH reduced to the system directories. That found three defects, none of which
any test in this repository could have reached:

- a manifest written with a **byte-order mark**, which Go's `json.Unmarshal` refuses — so the
  first packed bundle reported *"this installation's manifest cannot be read — reinstall Marco"*
  about a manifest that was perfectly correct, on a fresh install;
- a manifest listing **`version.txt`** as an installed component, which is the embed's own label
  and is never extracted — so a correct installation reported a missing file and told its owner to
  reinstall;
- the launcher's closing line still saying it **launches the overlay**.

And the harness can now test an **installed** Marco rather than only this tree: `-installed <bin>`
takes the product binaries from an installation, requires every one by name, checks the installed
Studio for the Wails guard — a different claim from checking the build `pack.ps1` just made, since
a bundle can be packed correctly and installed from an older copy — and says in its report
"taken from an installed Marco at …" rather than "built from this tree". A run against a bundle
that claimed to have built its own binaries would be the most misleading report this harness could
produce. The fixture stays this tree's, and the report says why: it is the harness's own window,
not a shipped component.

## The thirty-second hang this turned up

The `plugins/studio` suite failed intermittently at ~95s, which had been recorded as unexplained.
It is a real defect on the **shipped Play path**, caught with the product's own diagnostic: the
child ran the program — its last transcript line was the routine's own log — and then said nothing
for thirty seconds, with no outcome word and the window's one run slot still claimed. Roughly one
run in six under load; half a second in isolation.

Deductively it could only be one of the two **unbounded channel receives** either side of the pump
wake, because the wake itself has been bounded at 5 seconds since the 600-second timeouts that
found it. Both are bounded now, each with its own sentence, so the next occurrence says which
expired. The evidence points at the app-switch poller — its one blocking call is a
foreground-window read, which blocks when the window's owning thread is not pumping, and a desktop
full of browser windows is exactly that desktop — but **that is a pointer, not a proof**, and
nothing here claims the cause is resolved. What is fixed is that a run which finished its program
now reports that it finished.

**And sometimes it never ends at all.** On 2026-10-27, eight `marco do --play=says-hello`
processes were found still running on this desktop from test runs between 18:57 and 21:30 — each
with the recorder's global hooks installed, hours after the test that spawned it had finished.
That is the same symptom with no end to it, and it was invisible for a reason worth writing down:
the child's `$MARCO_HOME` is the test's temporary directory, so the run record it reserved went
away with the test that made it. There was nothing left to read. They also hold the repo-root
`marco.exe` open, which is what makes them visible at all — `withAMarcoBeside` had been SKIPPING
every Play test in Studio's suite, because a locked binary cannot be rebuilt and a stale one is
not run. Stopped, rebuilt, and the suite then ran with no skips and left no child behind. One
clean run is not a fix and nothing here claims the cause; what is new is that the symptom has a
name and somewhere to look for it.

**And the rule is generalised rather than patched.**
`TestAPumpWakeupIsRetriedRatherThanPostedOnce` demanded the wakeup be posted in a loop and put the
BOUND in its own failure message — prose, not a check. So a site could satisfy every test and
still wedge its caller two lines away from the one that was checked, which is what happened. The
tree-walking test now refuses any bare receive in a hook site's shutdown, scoped to the function
that wakes the pump because that is by definition the one taking the hooks off, and skipping a
`select`'s comm clauses because `case <-ch:` wraps its receive exactly like a bare one — a walk
that missed that flagged every bounded wait in the tree.

## The parked items, as ruled

- **`failed` versus `refused`** — the wire vocabulary is unchanged for this alpha. A guard's
  refusal is a decision rather than a malfunction and still reaches a person as `failed`, because
  every execution error maps onto that word. The distinction is carried by the REASON, which is
  printed, stored in the record's `because` and shown on the card. The limitation is documented
  where the words live. Reclassifying changes what every surface does and what every record
  already on disk means by comparison, and is a separate change.
- **The stop that could not be raised** — reproduced deterministically: `os.Rename` over the
  generation file fails with *Access is denied* whenever ANY process holds it open, and a watcher
  polling the generation holds it open every 100ms. So the stop is most likely to fail at precisely
  the moment it matters. One hypothesis ruled out: opening the reader with `FILE_SHARE_DELETE` does
  not help, because `MoveFileEx` with REPLACE_EXISTING still refuses an open target. **Nothing is
  retried** — that is a policy about how hard to try to stop somebody's computer and is not a
  decision for an error path. The sentence is fixed instead.
- **The Director's provisional record** — five fields are pointers and `ended` is omitempty, so an
  attempt nobody can speak for claims nothing. It was publishing `actions_sent: 0`, `live: false`
  and three verification zeroes about a killed run that may have pressed a dozen things for real.
- **`anchorArmed`'s comment** — corrected; no behaviour change.
- **The encoding damage** — repaired in both named files, with a tool that refuses what it cannot
  verify.
- **Orphan images, anchored-press propagation, the CV default** — still parked.

## Three things the review found, and the first was this slice's own

**The launcher decided where the data was.** Increment 1 had it set `MARCO_HOME` and
`MARCO_ROUTES` to its own installation directory, on the reasoning that the launcher states which
store this installation uses. That reasoning was wrong twice over: it overrode an explicit
`$MARCO_HOME` somebody had set, and it pointed away from the shared default `%APPDATA%\marco` that
every other way into Marco resolves. **An existing Studio user would have installed the alpha and
been shown an empty library**, with their routines exactly where they left them and nothing saying
so.

Installation and data resolution are separate now. The launcher sets neither, and seeds no starter
routine — which needed the library resolved and wrote into somebody's data from an installer.
`MARCO_BIN` stays, because that names an executable: it is the marco this bundle unpacked, and a
child that found another would be a different build.

`plugins/marco-app` is its own module and cannot import `internal/marcopaths`, which is the reason
the answer must not be derived there at all — a second resolver is exactly
[[ADR-252-a-library-is-a-place-and-a-routine-is-played-where-it-was-made]]'s defect. The guard is
therefore an assertion about the ABSENCE of a thing, by name, in the one file that could
reintroduce it, ignoring comments: the note that records this fix necessarily names what it must
not do.

**And the conflict is named by the window, which ADR-252 promised and never shipped.** An older
bundled installation does have data under `%LOCALAPPDATA%\Marco\routes`, because the launcher put
it there. `marcopaths.Elsewhere` was written for exactly that case — *"the window names any
library-shaped directory beside its launch location that holds routines and says what to do about
it; nothing is adopted or moved, because choosing between two libraries is the person's"* — and
**nothing ever called it**. A complete mechanism with no caller, which is this repository's own
recorded failure mode. It is wired into readiness now, with both paths, the one setting that
chooses between them, and nothing moved or merged by anybody.

**An interrupted upgrade left mixed binaries.** Components were written into `bin\` in turn, and
Windows refuses to overwrite an executing file — so an upgrade while anything from the last one
was running failed PART WAY, with the files before the failure already replaced, while the error
said *"nothing has been changed"*. A new Studio beside an old `marco` and an old `director` is
precisely the mixed installation the sibling discipline exists to prevent, and it looks current
from every direction.

Each version now stages into its own directory — `bin-<version>` — and `bin\` is replaced only
once every file is written. Staging can fail halfway and cost nothing, because nothing is using
it. The activation is a directory swap, which is the one step that can still fail against running
binaries, so on failure the old `bin\` is put back and the message says to close Marco. **Either
the whole new version is in use or the whole old one is.** Previous versions are left on disk:
a directory that was in use a moment ago is not something to remove while its processes may still
be exiting, and it is what makes the fallback possible.

**Recorder workers could reach a later recording.** `Stop` is bounded, so a shutdown that cannot
complete gives up and returns — and the app-switch poller and the capture worker are still
running, and both publish through this package's global state. A late foreground read appended an
app switch to somebody else's demonstration as a step they never performed; an old capture could
write a picture into whatever event was at its index now.

Both are fenced by **session**, taken when the recording starts and compared where the publication
is made — and for the poller that is AFTER the sample rather than before it, because
`winctx.Active()` is the call that blocks and the instant that matters is the one it returns in.
The decision the poller makes twice is one function, because the fence went into one of the two
call sites first and a mutation found it. The first version of this compared *immediately before*
publishing, which was still two moments; see the review below.

**And a further recording is refused outright**, because the hooks cannot be fenced from Go at all.
They are C callbacks with no context of their own; they gate on `recActive`, and the next `Start`
would set that true again — so an abandoned recording's hooks, still installed because the thread
that owns them never answered, would feed the new one. A session number cannot reach a C callback.
So recording is over for the life of the process and says a restart is what fixes it, which is
what the shutdown's own message already said: *they stay installed until this process exits.*

## Three boundaries the second review found

**The recorder fence was checked before publication rather than with it.** Each worker asked
whether the global state was still its own and then wrote into it — two moments, and a recording
can be replaced between them. A worker could pass its check, be descheduled, and publish after a
later `Start` had emptied the buffer. The session was also raised BESIDE the lock rather than under
it, so the replacement itself was two steps and a worker could be let through between those as
well. Checking first and writing second is not a fence, however correct each half is on its own —
and the finding is reachable from a worker timing out alone, since `recWedged` only covers hooks
that were abandoned.

`recMu` now guards the session as well as the state it describes. `beginRecording` raises the
session and empties the buffer under one acquisition, and each publication — `recordAppSwitch` for
the poller, `attach` for the capture worker — asks the fence and makes its writes under that same
lock. The poller's `last` advances only when publication succeeded, because a sample remembered as
recorded when it was dropped would silence the next real switch to that application. And the
foreground the hooks tag every press with is published inside the fence too: an abandoned poller
that wrote it would have the NEXT recording's presses attributed to an application nobody was in.

**Activation silently ignored a failed recovery.** It ignored the clearing of the recovery
directory and it ignored the rollback's own error. So an activation that failed AND a rollback that
failed reported itself as the case where the rollback worked — *"the version you had is still the
one in use"* — about an installation that was not there. That is the one sentence a person in that
position must not be told: it sends them to close a Marco that is not running instead of looking on
their own disk, where their previous version is sitting under another name. Both errors are handled
now, and the message for a failed rollback names the recovery directory and what to do with it. The
recovery directory is cleared only when there IS an installation to move into it, so a rollback
that failed leaves the one copy of the previous version alone until something is activated over it.

**A relative data override resolved against the installation.** The children are started from
`bin\` so that a sibling they spawn by a bare name cannot be resolved against whatever directory
somebody double-clicked the launcher in — and that moved the meaning of a RELATIVE `$MARCO_HOME`
or `$MARCO_ROUTES` with it. `MARCO_ROUTES=routes` in a terminal means `.\routes` THERE; the same
words through the launcher meant `bin\routes`. A different library from the same invocation, which
is exactly
[[ADR-252-a-library-is-a-place-and-a-routine-is-played-where-it-was-made]]'s defect arriving by the
back door.

So an override SOMEBODY SET is made absolute against the directory this process was started in,
before any child is started from anywhere else. That is not a second resolver and it answers
nothing: an override nobody set stays unset, an absolute one is untouched, and `marcopaths` decides
what any of it means as it always did. The only thing done is to spell a path somebody else wrote
against the directory they wrote it in, which is the one fact the launcher has and the child does
not.

## What is unchanged

- The CLI. `marco do`, `marco learn`, `marco simplify` and the rest are untouched, and
  `$MARCO_BIN`/`$DIRECTOR_BIN`/`$MARCO_ROUTES` keep their meanings.
- The store and library rules. `internal/marcopaths` still answers where both are, and the
  launcher answers nothing: it states which marco it unpacked and no more. A relative
  `$MARCO_HOME` or `$MARCO_ROUTES` keeps the directory it named where it was typed.
- User data across an upgrade. Nothing is seeded, moved or rewritten, by an installer or by
  anybody else.
- The seven outcome words, and every record already on disk.
- `Resave`, which is still the immediate repair for a routine that cannot press anything.

## Enforced by

- `cmd/marco` `TestTheBundleShipsStudioAsItsFrontDoor` — Studio extracted beside the two children
  it spawns, built with the Wails tags, the built bytes checked for the guard's own message box,
  every Go binary's revision asked for at pack time, a manifest written and extracted, and the
  overlay still shipped with an explicit way to ask for it. By source text, because
  `plugins/marco-app` is a separate module with no go.work and a guard living there is a guard
  nobody executes.
- `cmd/marco` `TestThePackagedLayoutIsWhatMarcoLooksFor` — unchanged and still holding the two
  lookups it was written for.
- `plugins/studio` `TestAnInstalledMarcoSaysWhatItIsMadeOf` — a tree reported as a tree rather than
  as a fault, an installation reporting each piece, a piece with no revision said to have none, a
  replaced file and a missing file as two different problems, an unreadable manifest saying so, and
  a byte-order mark NOT treated as a broken manifest.
- `plugins/studio` `TestTheWindowSaysRecordingWatchesEverythingBeforeRecord` — the disclosure
  RENDERED, because a sentence nobody draws is not a disclosure, with all four of its claims
  including that recording only a chosen window is not something Marco can do.
- `internal/recorder` `TestAHookThreadThatNeverStartedDoesNotWedgeTheShutdown`,
  `TestAPollerThatNeverComesBackDoesNotWedgeTheShutdown` — neither wait can wedge a caller, and the
  shutdown is bounded rather than merely each part of it.
- `internal/platform/navsource` `TestAHookSiteShutdownHasNoUnboundedWait` — the rule generalised:
  no bare receive in the function that wakes the pump, at any hook site in the tree.
- `internal/stopsignal` `TestAStopThatCouldNotBeRaisedSaysWhatItMeans` — the words, that the
  generation did NOT move for a failed raise, that no temporary file is left behind, and that a
  raise with nothing holding the file still works.
- `cmd/director` `TestARunRecordIsWrittenWhereItsReferenceSaysItIs` — the three counts as three
  keys with three DISTINCT values, a finished record carrying a real zero, and an unfinished one
  carrying none of them.
- `plugins/marco-app` `TestTheLauncherDoesNotDecideWhereTheDataIs` — the ABSENCE of
  `Setenv("MARCO_HOME")`, `Setenv("MARCO_ROUTES")` and `seedHello`, by name, in the one file that
  could reintroduce them, ignoring comments — because the note recording the fix necessarily names
  what it must not do. With the control that `MARCO_BIN` is still named, or the test is satisfied
  by a launcher that says nothing at all.
- `plugins/marco-app` `TestInstallingPutsTheWholeBundleWhereItIsLookedFor`,
  `TestAFailedUpgradeLeavesThePreviousBundleWhole` — every staged component where the table says,
  the stamp inside the version it describes, a same-version install rewriting nothing, and a
  failure on a LATER component leaving every file of the previous version as it was. The failure is
  arranged with a directory where a file has to go, which fails on any platform and needs no
  running process, and it lands after the first component — the only case that could ever have
  produced a mixture.
- `plugins/studio` `TestASecondLibraryIsNamedOnceAndNothingIsMoved` — a library beside the binary
  that is not the one in use, named with both paths and the one setting that chooses, the other
  library's routine still there afterwards, and nothing said when the library in use IS that one.
  `marcopaths.Elsewhere` had no caller at all before this.
  **ONCE**: it was then named TWICE, by `Acts.Library` over the routine list and again by a
  readiness problem above Record — 100 words for one fact, on the screen of anybody with an
  older library beside their tree. The readiness panel is for what is WRONG, and two libraries is
  not wrong, so the notice lives with the routines it is about and the test fails if the panel
  says it too. `Acts.Library` looks beside the working directory as well as beside the executable,
  so nothing is lost with the duplicate.
- `internal/recorder` `TestAWorkerHeldAcrossATimeoutCannotReachALaterRecording` — a shutdown that
  gives up, a later recording REFUSED with what to do about it, and an abandoned recording's late
  publish dropped even so. The refusal protects a real process; the fence makes the workers
  correct, and a build that only refused would be one line from being wrong again.
- `internal/recorder` `TestAnAbandonedPollersSampleIsNotRecorded` — the decision the poller makes
  twice, asked directly, because the goroutine that asks it cannot be started in a test and a
  mutation taking the fence out of ONE call site survived a test that only exercised `mine`. The
  fence is no longer one of those questions: see the three below.
- `internal/recorder` `TestAPausedWorkerPublishesIntoNothing` — a worker paused AFTER its
  eligibility check, the next recording started, and the worker released only then: neither the
  poller's switch nor the capture's picture reaches the later recording, its press is not moved,
  and its foreground is not re-decided. The test this replaced changed the session BEFORE asking
  eligibility, which the broken shape passes too.
- `internal/recorder` `TestARecordingIsNotReplacedWhileAWorkerIsPublishing` — the other half of
  "under the same synchronization": a replacement cannot complete while the publication lock is
  held, and does complete once it is released. A negative assertion with a window, which is the
  honest shape for "this does not happen" — what it rejects is an atomic add, which waits for
  nothing, so the failure is deterministic.
- `internal/recorder` `TestTheSessionFenceIsReadUnderTheLock` — and WHERE in each function the
  fence is read, which neither test above can see: move it one line up, above `recMu.Lock()`, and
  both still pass while the gap is open again. No test can pause a goroutine between two adjacent
  lines, so the rule is checked where it is written — by name, with a deferred unlock not counted
  as an unlock, and with a control that there are two publications to find.
- `plugins/marco-app` `TestActivationPutsTheStagedVersionInUse`,
  `TestAFailedActivationPutsThePreviousVersionBack`,
  `TestAFailedRollbackSaysWhereThePreviousVersionIs`,
  `TestARecoveryDirectoryThatCannotBeClearedIsNotIgnored` — the four outcomes of an activation:
  the staged version in use with the previous one kept, a failure with the previous version back
  and whole, a failure whose rollback also failed naming where their Marco is and NOT claiming one
  is in use, and a recovery directory that cannot be emptied refused before anything is moved. The
  failed rollback needs a seam, because it is an ORDER of two renames rather than a state on disk;
  the other three need none. A first install that fails claims no recovery, because there was none.
- `plugins/marco-app` `TestARelativeLibraryOverrideKeepsTheDirectoryItNamed`,
  `TestTheLauncherInventsNoLibraryWhenNobodyNamedOne` — a relative override from an unrelated
  launch directory resolving to the populated library that was THERE, with the routine in it and
  nothing under the installation; and the line that keeps this from being a resolver: nobody's
  override stays nobody's, and an absolute one is returned exactly as it was.
- `plugins/e2e` `TestOnlyAStaleElementIsRetried` — a stale element retried once and a control that
  was never there, one that is disabled, a window that has gone and a failing bridge each returned
  as findings.
- `plugins/e2e` **the installed bundle, run on a desktop on 2026-10-27**: `play` 1 of 1, `runs`
  3 of 3, `generation` 2 of 2, `text` 1 of 1, `button` 4 of 4 — launch, seeded playback,
  cancellation, reporting, review/cancel/replace, typed text and the guarded press, every run
  record naming the INSTALLED binary's revision rather than the tree's, which is itself the
  evidence that the bundle was what answered. Run again at `cec9e65` after the three closures
  below, with the same eleven results and nothing left running afterwards.
- **And the artifact, from an isolated directory** with `LOCALAPPDATA` and `APPDATA` pointed away
  from the owner's install, no repository and PATH cut to the system directories. A first install
  reporting every Go binary's revision and no problems. A relative `MARCO_ROUTES=routes` typed in
  an unrelated directory resolving to THAT directory's library — against the counterfactual, which
  is the same words given to the child started in `bin\` as the launcher starts it, resolving to
  `bin\routes`. The shared default still found with nothing set at all. And an upgrade attempted
  while a file in `bin\` was held open: refused, with `bin\` still whole at the version it had
  and the new one left staged beside it; then, with the file released, activated whole with the
  previous version kept in `bin-previous`. The rollback path itself is not reachable this way and
  has its own test — it is an order of two renames, not a state on disk.
