---
type: adr
status: accepted
date: 2026-10-29
decision: "ACCEPTED — PACKAGING IS GATED ON A BUILD OF THE COMMITTED SOURCE, IN A CHECKOUT THAT HOLDS NOTHING ELSE, AND THE RESULT NAMES THE COMMIT IT TESTED. Every green result this repository had recorded was a fact about one working tree. `go build ./...`, the whole suite and `cmd/gate` all read the directory in front of the person running them, and for the entire life of `cmd/perceptionreplay/coverage.go` — 420 lines holding the walker, the classifier and the coverage report `workflow.go` calls — that directory contained a file no clone had, because `.gitignore` carried `coverage.*` and that pattern matches a `.go` file as readily as a `.out` file. NOTHING COULD HAVE SAID SO: `git status` does not list an ignored file, which is what ignoring means, so the suite, the gate and every review were reading the same tree the omission was hidden in — a check and the thing it checks sharing one blind spot. It surfaced only when three commits were checked out into empty worktrees to see whether the history could be bisected. THE IGNORE-PATTERN GATE IS NOT ENOUGH, and that is not a criticism of it: `internal/wiringcheck` TestNoIgnorePatternHidesASourceFile closes the specific accident of a pattern swallowing source, and cannot establish that every file the build needs was ever added — that is the general question and the only instrument for it is a clone. So `cmd/cleanroom` clones the commit with `--no-hardlinks`, REFUSES unless `git status --porcelain --ignored` in the checkout is empty, names the prerequisites it takes from the machine rather than from the commit, and runs the build and the screen-free gate inside it; `gate.ps1` runs that as stage 2 of 6, before packing, and refuses to pack unless the recorded verdict is PASSED and the recorded commit is the one about to be packed. THE EXIT CODE IS NOT THE RECORD AND THE RECORD IS NOT ABOUT ANY COMMIT: the three ways the stage can be satisfied by something that is not evidence — a program that failed, a program that succeeded and wrote nothing, and a passing record of SOME OTHER COMMIT — are refused separately, because a mutation dropping any one of them passed the other two. AND THE WORKING TREE MUST BE CLEAN, because cleanroom builds the COMMIT while `pack.ps1` packs the WORKING TREE, so an uncommitted change would be packed having been checked nowhere: the same shape of gap, one step along."
supersedes: []
affects:
  - testing
  - packaging
source_paths:
  - cmd/cleanroom/main.go
  - cmd/cleanroom/cleanroom_test.go
  - cmd/gate/script_windows_test.go
  - internal/wiringcheck/ignoredsource_test.go
  - gate.ps1
  - .gitignore
---

# Packaging is gated on a clean checkout

## What every green result had actually been about

One directory. `go build ./...` compiles what is on disk. `go test ./...` reads what is on disk.
`go run ./cmd/gate` — the screen-free stage, the one this project runs most — is told which tree
it is for and reads that. So is `docscheck`, and so is `internal/wiringcheck`.

That is correct for all of them and it is not a claim about the repository. It is a claim about a
machine.

## The omission, and why nothing could see it

`cmd/perceptionreplay/coverage.go` is 420 lines: the `walker` that attempts a draft in order, the
`classify` that says which of four reasons a step went unexamined, `printCoverage`,
`secondRouteStart`, `routeStartLook` and `sameTargetAcrossVisits`. `workflow.go` calls all of it.

It was never committed. `.gitignore` carried:

    # Code coverage profiles and other test artifacts
    *.out
    coverage.*

and `coverage.*` matches `coverage.go`. So the package existed on one machine and in no clone.
`go build ./...` passed here and failed in every checkout of this history, for the whole life of
the file.

Every instrument that could have noticed was pointed at the tree the file was in:

- `git status` does not list an ignored file. That is what ignoring means.
- The suite compiled the package from disk, where it was present.
- `cmd/gate` did the same, and then asked its two questions about skips and surviving processes.
- A reviewer reading the commit saw `workflow.go` calling nine functions that, as far as the
  repository was concerned, did not exist, and no tool objected.

It surfaced because three commits were checked out into empty worktrees to see whether the
history could be bisected. Nothing else in the repository would have found it.

## It had already happened once

Further down the same `.gitignore`:

    # User routes ignored — ANCHORED.
    # Unanchored `routes/` matches a directory of that name at ANY depth, so it also
    # excluded internal/routes/ — and internal/routes/origin.go, a source file the build
    # needs, was silently left out of a commit. A pattern meant for one directory must
    # say which one.

The same accident, understood, paid for, and written down as prose two lines below where
`coverage.*` was later added. A comment cannot fail. That is the argument `internal/wiringcheck`
was created to make, and this is the argument applied to the repository's own packaging.

## Why the ignore-pattern gate is not the answer on its own

`internal/wiringcheck` `TestNoIgnorePatternHidesASourceFile` now asks whether any pattern in
`.gitignore` matches a `.go`, `.ps1` or `.cmd` file that exists in the tree. It kills the specific
accident, and `matchesIgnore` is a pure function so the two real cases are stated in a table
rather than arranged in the repository.

It cannot establish that every source, asset or generated file the build needs was ever added. A
file never created, never ignored and never committed defeats it; so does one removed from the
index by hand; so does a generated artifact that only exists because somebody ran something once.
That is the general question, and the only instrument that answers it is a build of the committed
source, somewhere the working tree is not.

## The decision

`cmd/cleanroom`, run as stage 2 of `gate.ps1`, before packing.

1. Resolve the commit, and SAY whether the tree it was asked from is dirty.
2. `git clone --no-hardlinks` into a temporary directory and check that commit out detached, so
   the object store is the clone's own and the working tree is built from those objects.
3. **Refuse unless the checkout is empty of anything git did not put there** —
   `git status --porcelain --ignored` must report nothing. This is the premise, checked rather
   than assumed: a checkout with an ignored file in it is a second working tree, and the whole
   program would then be measuring the same blind spot from a new directory.
4. Name the prerequisites it takes from the machine rather than from the commit, and say which
   generated artifacts this stage needs: none, because `cmd/gate` builds `marco.exe` and
   `director.exe` itself.
5. `go build ./...`, then `go run ./cmd/gate`, both inside the checkout.
6. Write a result naming the commit, because a pass that does not say what it passed is not a
   prerequisite anybody can check.

## What the stage refuses, and why the arms are separate

Three ways it could be satisfied by something that is not evidence. Each is refused on its own,
because a mutation that dropped any one of them passed the other two:

| What happened | Why the other checks miss it |
| --- | --- |
| cleanroom failed | The only arm an exit code catches. |
| cleanroom succeeded and wrote no result | The exit code says a program finished; what a later step reads is the file. `pack.ps1` already has a paragraph about this shape one stage along — a packer that reports success and produces nothing leaves whatever was there before standing in for what this run made. |
| the record names ANOTHER commit | The program succeeded and the record exists and says PASSED. A clean checkout of yesterday's commit says nothing about today's, and this is the arm a reader is most likely to assume the exit code covers. |

## And the working tree must be clean

`cleanroom` builds the **commit**. `pack.ps1` packs the **working tree**. With a change on disk
and not in git, the stage would pass on a commit that does not contain it and the bundle would
then be packed with it in — checked nowhere.

That is the same shape of gap as the ignored file, one step along, so the stage fails on a dirty
tree and names the files. It is strict on purpose: `gate.ps1` is the release gate, and
`-SuitesOnly` remains the switch for a run during development.

## What this does not change

- **It takes no screen and opens nothing.** It does not pack, install, or run any desktop
  scenario. Stages 3 to 6 do those, and stage 2 is why stage 3 is allowed to start.
- **It does not build `plugins/voice`**, which needs the Vosk C headers, and it does not run
  `-race`, which needs gcc. Both are named in its own output rather than assumed.
- **It does not rewrite history.** The three commits before the fix do not build from a clone,
  because the fix is after them; that is recorded in [[Alpha-candidate-2026-10-29]] rather than
  edited away.
- **It is not a repository cleanup.** Two files are ignored on purpose and stay ignored: the
  person's own route store under `/routes/`, and the staged bundle assets.

## Enforced by

- `cmd/cleanroom` `TestTheResultNamesTheCommitItTested` — the record names the commit and reads
  back as that commit, which is what the packing gate compares.
- `cmd/cleanroom` `TestAFailedOrSilentResultIsNotAPass` — a failed run, a record with no verdict,
  a record with no commit, an empty record and a verdict that is not the word are each refused.
- `cmd/cleanroom` `TestAnIgnoredFileCopiedIntoACheckoutIsReported` — the one claim about git the
  whole program rests on: that `git status --porcelain --ignored` in a checkout names a file
  copied in even when `.gitignore` covers it. Built as a repository of its own, with the ordinary
  status asserted to be silent about the same file first.
- `cmd/cleanroom` `TestThePrerequisitesAreNamedRatherThanAssumed` — a check that quietly needed
  something would prove the opposite of what it is for.
- `cmd/gate` `TestPackingIsRefusedUnlessACleanCheckoutBuiltThisCommit` — the three arms above,
  against the real `gate.ps1` in a stand-in tree, each asserting that nothing was packed.
- `cmd/gate` `TestAnUncommittedChangeStopsTheGateBeforeAnythingIsPacked` — the dirty-tree rule,
  and the one case in that file that must not commit first.
- `cmd/gate` `TestAWholeRunClaimsTheGate` — names all six stage headers, so a stage that stops
  running fails by name.
- `internal/wiringcheck` `TestNoIgnorePatternHidesASourceFile` and
  `TestTheIgnoreRuleCatchesBothAccidentsItIsAbout` — the specific accident, with both real cases
  stated in a table beside the patterns that must not fire.

## And the first run found that the suite could not run anywhere else

The stage's own first result, against `eca0515`: **2 failures and 9 findings** in the clone, for a
commit whose suite is green in the tree it was cloned from. Not line endings — that was the run
before, and it is fixed. These were tests whose preconditions are facts about **one machine**.

The rule applied to all of them: **a missing repository-generated artifact is not an external
prerequisite.** A model server, a captured recording, vendored weights and somebody's screen are
things this repository does not contain, and a test needing one is an integration test somebody
selects. A packed bundle, a seeded module and a generated route are things this repository
**makes**, and a test needing one must make it.

| What failed or skipped | What it had been reading | What it does now |
| --- | --- | --- |
| `internal/spectest` `TestGeneratedMarcoStaysInsideCore`, `TestGeneratedMarcoNamesNoBackstageConcept` — **failed** | `../../routes`, the person's own route library | A deterministic matrix of 24 programs produced by this build: `codegen.RouteAs` over every step kind the generator supports and the anchor forms that change what it emits, plus `marcoexec`'s play lowering and six lowered operations. Each is compiled through `driver.CheckSource`, which resolves `use os.` to the tracked canonical module. |
| `internal/osmod`'s seeded-surface test — **skipped** (the name went with the rewrite, which is why it is described here rather than cited: `internal/wiringcheck` holds this page to naming tests that exist) | `../../routes/os.marco`, one historical copy | The canonical module supplied in an isolated fixture, and the MECHANISM checked rather than one copy of it: for every capability the surface exports, a sibling carrying the canonical source resolves as the built-in does, and a sibling with that export removed makes the capability unreachable. |
| `plugins/marco-app` × 6 — **skipped** | `assets/`, staged by a `pack.ps1` run at some point in the past | Behind `-tags packaging`, which `gate.ps1` runs in the stage that stages the bundle — and which REJECTS a skip. Under the tag the precondition fails rather than skips. |
| `cmd/director` `TestBuildingTheShadowDetectorDoesNotTouchTheProcessEnvironment` — **skipped** | vendored ScreenParser weights and a gcc-built plugin | Behind `-tags vision`, failing rather than skipping when they are absent. Genuinely supplied from outside, so an explicitly selected integration test. |
| `cmd/perceptionreplay` `TestAtACoarseCadenceTheCorpusIdentifiesTheMenu` — **skipped** | an L0 recording under `$MARCO_FRAME_CORPUS` or `.tmp/` | Behind `-tags corpus` with the full-frame one, failing rather than skipping. Hundreds of megabytes of captured frames is supplied from outside. |

**The claim the two governance gates make is different now, and stronger.** It was *the routes in
this library stay inside Core*; it is *representative programs produced by this build stay inside
Core and name no backstage concept*. A leak reaches a library only after the generator has written
it; the generator is asked directly here, and the matrix's coverage is checked against the
`StepKind` constants rather than asserted by a comment — a kind that is neither exercised nor
excused by name fails.

**And the checkers are held with deliberately invalid examples.** They passed before this change,
against a library on one machine, and they would have passed with a checker that always answered
"nothing wrong" — a rule about a broken thing cannot be established from an unbroken one. Six
broken programs are stated: a sentence opening with a named extension, one Core has no word for, a
legal sentence naming a confidence, a backstage word in a comment, a stem-only match, and one that
breaks both rules at once so neither gate can rely on the other having caught it.

**One documented claim turned out to be about this machine and is corrected rather than kept.**
CLAUDE.md said `go test ./...` still gates perception through the every-tenth-frame test. It did
here, where the recording is under `.tmp/`. It gates perception through the deterministic tests in
that package; the measurement against a real recording is `-tags corpus`.

## Enforced by, for the test corrections

- `internal/spectest` `TestGeneratedMarcoStaysInsideCore` and
  `TestGeneratedMarcoNamesNoBackstageConcept` — the two gates, over the matrix.
- `internal/spectest` `TestTheMatrixCoversEveryStepKind` — the coverage rule, read from the
  `StepKind` constants, with a stale excuse failing too.
- `internal/spectest` `TestTheGovernanceCheckersObjectToProgramsThatBreakTheRules` and
  `TestTheGovernanceCheckersLeaveAnOrdinaryRouteAlone` — both directions, because a checker that
  answered "broken" to everything would pass the first.
- `internal/osmod` `TestASiblingOSModuleShadowsTheEmbeddedSurface`,
  `TestTheFixtureReadsEveryExportOfTheCanonicalModule` and
  `TestDroppingAnExportLeavesTheRestOfTheModuleIntact` — the mechanism, the fixture reading the
  whole surface, and the drifted sibling being drifted rather than broken.
- `cmd/gate` `TestAPackagingTestThatSkipsStopsTheGate` and
  `TestAPackagingStageThatRanNothingIsNotAPass` — a skip and an empty selection are each refused,
  and nothing is installed after either.
- `cmd/gate` `TestAWholeRunClaimsTheGate` — the packaging stage recorded as passed in the verdict
  table, not only printed on the way past.
