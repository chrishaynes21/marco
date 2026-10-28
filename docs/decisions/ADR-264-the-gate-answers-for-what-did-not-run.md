---
type: adr
status: accepted
date: 2026-10-28
decision: "ACCEPTED — THE GATE ANSWERS FOR WHAT DID NOT RUN, AND FOR WHAT OUTLIVED IT. `go test ./...` answers one question and a release gate needs three: did anything fail, did anything DECLINE to run, and did anything the tests started OUTLIVE them. The second and third are not hypothetical and they hid each other for hours: Studio's suite printed `ok` while every test that presses Play SKIPPED, because the binary they spawn could not be rebuilt — and the reason it could not be rebuilt was eight `marco do` processes from earlier runs still holding the file open, each with the recorder's global low-level hooks still installed. A skip is green, and nothing looks for a child. So a skip is now tolerated ONLY when `gate-skips.txt` declares it, by package and test name, with a reason somebody wrote; an undeclared skip fails the gate, and so does a STALE declaration whose test ran and did not skip, because a list of excuses nobody prunes stops describing anything. A reason beginning `BROKEN:` marks a declared skip that is a DEFECT rather than a decision, counted and printed apart on every run including a green one. And a watched product binary that is running now and was not running before is a FINDING: established by comparison, because something the owner already had open is not the gate's doing; described into the run's directory BEFORE anything is done about it, because the eight that went unnoticed had their run records in temporary stores that went away with the tests that made them; and ended only when it is this run's to end — below this process, or an orphan whose parent is gone — with `marco do` somebody started in their own shell reported and LEFT ALONE. The desktop harness asks the same question after EVERY scenario rather than once at the end, for attribution and for containment, and preserves that scenario's run records before ending anything; a scenario that passed and left a child running is turned into a finding, because a run that reports it finished while the process performing it is still there has not established what it says. `gate.ps1` is the whole of it in order — the suites, the packed bundle, its installation from an isolated directory, then EVERY registered scenario against those installed binaries, with `-only` named in the output as a partial run rather than the gate. The first run found four things: three helper-process skips nobody had declared, and `TestAnUnconsumedArmDoesNotSurviveIntoTheNextRecording` skipping because two bounded-shutdown tests left `recWedged` set for the whole package — a real test silently not running, three files away. Three more declarations are BROKEN rather than expected: two mutation gates in `internal/director/observe` and one rehearsal test in `cmd/director` build a scenario, find it did not produce the condition they are about, and decline. PHYSICAL RECORDING CAPTURE REMAINS NOT EXERCISED, and the eight orphans' CAUSE REMAINS UNRESOLVED: one clean run does not close it, and what this changes is that a recurrence is exposed and contained rather than silent."
supersedes: []
affects:
  - execution
source_paths:
  - internal/gate/gate.go
  - internal/gate/ledger.go
  - cmd/gate/main.go
  - gate-skips.txt
  - gate.ps1
  - plugins/e2e/children.go
---

# The gate answers for what did not run

## Three questions, and `go test` asks one

	Did anything FAIL?                            `go test` says so, loudly, and always did.
	Did anything DECLINE to run?                  It prints `ok` and says nothing.
	Did anything the tests started OUTLIVE them?  It does not look.

The second and third are not hypothetical, and on 2026-10-27 they hid each other.

Eight `marco do --play=says-hello` processes were found still running on the owner's desktop, from
test runs between 18:57 and 21:30, each with the recorder's global low-level keyboard and mouse
hooks still installed. They are what the thirty-second hang of
[[ADR-263-the-alpha-is-the-studio-stack-and-it-says-what-it-is]] looks like when it never ends.

Nothing said so, and there was nothing left to read: the child's `$MARCO_HOME` is the test's
temporary directory, so the run record it reserved went away with the test that made it. The only
reason they were noticed at all is that a running binary cannot be overwritten on Windows — a
build failed — and the consequence of that lock was the other silence: `withAMarcoBeside` refuses
to spawn a stale `marco.exe`, so Studio's suite had been **skipping every test that presses
Play**, printing `ok`, for however long.

Each silence explained the other. Neither was visible.

## A skip is tolerated only when somebody declared it

`gate-skips.txt` names every skip this gate accepts, by package and test, with a reason. An
undeclared skip fails the gate. So does a **stale** declaration — one whose test ran and did not
skip — because the entries are read as the gate's own statement of coverage, and one that has
quietly started passing, or that names a test since renamed, is an exemption doing nothing while
the next skip of the renamed test arrives undeclared.

A reason is **required**, and a line that names a test without one is an error rather than an
entry: that is the shape a silent exemption takes. Every entry is a sentence a reader can
disagree with.

A reason beginning `BROKEN:` marks an entry that is a **defect rather than a decision**: a test
that no longer establishes what it was written for. Those are counted and printed apart, on a
green run as well, because the two kinds mean opposite things about coverage and only one of them
is somebody's choice.

## A surviving child is a finding, and ending it is a second question

A survivor is established by **comparison** — a watched product binary running now that was not
running before. A gate that failed on "a `marco.exe` exists" would fail for the owner having
their own Marco open; a gate that failed on nothing is the one that let eight of them accumulate.

Identity is the executable's **path**, not its name. The binary a test spawns and the one in
somebody's own Marco are both `marco.exe`, and a process whose path cannot be read is reported as
unknown rather than matched by name — ending somebody's Marco to report on a test's would be this
gate interfering with the work it exists to protect.

**Everything is written down before anything is done about it.** That is the whole lesson of the
eight: pid, image, parent, start time, window titles, and — in the harness, which owns the store —
that scenario's run records, copied out before the store is removed.

**And whether it may be ended is separate from whether it is a finding.** It is this run's to end
when it is below this process or when it is an **orphan** whose parent is gone; `marco do` started
by hand in a live shell is reported and left alone, with the sentence saying which it was. The
ownership walk repeats until it stops growing, because the chain breaks: a Studio that exits
leaves the `marco` it spawned with a parent id that names nothing, and a walk following only live
parents would lose exactly the process being hunted. Pids of things this run started are
remembered when they are started rather than looked up when they are wanted, for the same reason.

The harness asks after **every scenario** rather than once at the end — attribution, because the
scenario that leaked is the one worth looking at, and containment, because a Studio or a `marco`
left running changes what the next scenario sees. A scenario that passed and left a child running
becomes a finding: a run that reports it finished while the process performing it is still there
has not established what it says. Its own evidence is kept beside the finding; the verdict is
what is replaced, not the account.

## The settling period, which is Marco's own number

Twenty seconds. A recording's shutdown gives itself three bounded waits of five seconds each (see
`internal/recorder`) and the process still has to exit after them. A child alive past that has
outlived every bound Marco allows itself — which is the definition used, rather than a number
somebody liked. The wait is only entered when there is something to wait for, so a clean module
or scenario costs nothing.

## The order the stages have to run in

`gate.ps1`:

1. the suites, with what declined and what outlived them
2. the bundle, packed from this tree
3. the bundle **installed** from an isolated directory, reporting what it is made of
4. the **complete** registered harness against those installed binaries

Each stage is worth something only if the one before it passed. A harness run against binaries
whose own suite is failing tells you about the harness; a bundle validated from the tree it was
built in cannot tell you whether an installation works; and the desktop half goes last because it
is the only part that takes somebody's screen. `-Only` is printed as **a partial run, not the
gate** — the scenario that did not run is the one whose question nobody asked.

Stage 1 also builds `marco.exe` and `director.exe` first, which is not a convenience: it is the
reason Studio's Play tests decline, and a gate that did not build them would be a gate whose most
load-bearing tests were silent.

## What the first run found

- **Three helper-process skips nobody had declared**, in `plugins/overlay`: the branch that *is*
  the spawned child, declining when it is the parent. Structural, permanent, and now written down.
- **`TestAnUnconsumedArmDoesNotSurviveIntoTheNextRecording` was skipping**, three files away from
  the cause: `TestAHookThreadThatNeverStartedDoesNotWedgeTheShutdown` and
  `TestAPollerThatNeverComesBackDoesNotWedgeTheShutdown` abandon a recording, which sets
  `recWedged` for the life of the process, and neither put it back. `Start` then refuses and the
  arm test declines on the refusal. A real test not running, found by the gate on its first pass.
- **Three parked findings**: two mutation gates in `internal/director/observe` and one rehearsal
  test in `cmd/director` that build a scenario, discover it did not produce the condition they are
  about, and decline — one of them with a doc comment that says *"deleting `g.unsettled++` must
  fail this"*, which it cannot. Declared `BROKEN:` rather than fixed or deleted: they are
  perception and rehearsal work, not alpha work. See [[Roadmap]] item 37.

## What this does not establish

- **The eight orphans' cause.** Nothing here diagnoses it, and one clean run does not close it.
  What changed is that a recurrence is exposed and contained instead of silent.
- **Physical recording capture.** Still NOT EXERCISED, for the reason it always was: Marco's hooks
  discard injected input by design.
- **That the declared skips are harmless.** They are declared, which means somebody wrote a
  sentence about each one. A ledger is an argument, not a proof.

## Enforced by

- `internal/gate` `TestASurvivorIsEstablishedByComparison` — a watched program somebody already
  had open is not a survivor, one that appeared is, a process whose image could not be read is not
  matched by name, and an unwatched program is not reported at all.
- `internal/gate` `TestDescendantsSurviveADeadParent` — the walk reaches a child whose parent has
  exited, which is the case it exists for, and does not reach somebody else's process.
- `internal/gate` `TestOnlyThisRunsChildrenAreEnded` — a child of this run is ours, an orphan is
  ours, and a process with a live foreign parent is a finding that is LEFT ALONE, with the
  sentence naming the parent.
- `internal/gate` `TestASurvivorIsDescribedBeforeItIsEnded` — pid, image, parent, when it started
  and that it started during this run.
- `internal/gate` `TestTheLedgerRequiresAReason` — comments and blank lines, both dash forms, a
  `BROKEN:` entry read apart from an ordinary one, and a bare test name REFUSED.
- `internal/gate` `TestWhatTheLedgerDoesAndDoesNotCover` — an undeclared skip is the finding, a
  declaration whose test did not skip is another, and an entry for a package this run never
  reached is neither.
- `internal/gate` `TestTheSnapshotFindsThisProcess` — the platform asked for real: this process,
  its path, its parent, its start time, and itself found as a survivor of its own executable.
- `cmd/gate` `TestWhatIsReadOutOfATestRun` — the skip carries its reason and not the framework
  line around it, the failure carries what it said, and every package the stream mentioned is
  recorded as covered.
- `cmd/gate` `TestAModuleThatDoesNotBuildIsNotACleanRun` — a package-level failure names no test,
  which is why the suite's exit code is checked as well.
- `cmd/gate` `TestASurvivingChildIsFoundDescribedAndEnded` — against a REAL process: started,
  found by comparison, described into the run's directory with "started by this run", then ended
  and reaped. The watched set is a parameter so a test can watch something harmless rather than
  point the gate at the product and hope.
- `cmd/gate` `TestTheGateCallsWhatItIsFor` — the wiring, by walking the command: `suite` takes a
  snapshot and looks for survivors, `run` reads the ledger, runs the modules, and builds the
  engine first.
- `cmd/gate` `TestTheLedgerInThisTreeReads` — this tree's own ledger parses, declares nothing
  twice, and names only packages in this repository.
- `plugins/e2e` `TestASurvivingChildIsAFindingAndNotEndedBlind` — a survivor whose parent is a
  live foreign process: the result becomes a finding, the scenario's own evidence is kept, the
  description and the run records are written out first, and nothing is ended.
- `plugins/e2e` `TestAScenarioThatLeftNothingRunningIsReportedAsItWas` — the other half: a clean
  scenario is untouched, and a check that could not be made does not read as one that found
  nothing.
- `plugins/e2e` `TestThisRunRemembersWhatItStarted` and
  `TestEveryProcessThisHarnessStartsIsRememberedAndSweptFor` — every function in the package that
  starts a process also remembers it, something calls the sweep, and `main` takes the two
  snapshots it needs. By walking the package rather than naming the two places, because naming
  one place is how the second was missed the last time.
