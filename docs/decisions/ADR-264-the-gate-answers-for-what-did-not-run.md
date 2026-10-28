---
type: adr
status: accepted
date: 2026-10-28
decision: "ACCEPTED — THE GATE ANSWERS FOR WHAT DID NOT RUN, AND FOR WHAT OUTLIVED IT. `go test ./...` answers one question and a release gate needs three: did anything fail, did anything DECLINE to run, and did anything the tests started OUTLIVE them. The second and third are not hypothetical and they hid each other for hours: Studio's suite printed `ok` while every test that presses Play SKIPPED, because the binary they spawn could not be rebuilt — and the reason it could not be rebuilt was eight `marco do` processes from earlier runs still holding the file open, each with the recorder's global low-level hooks still installed. A skip is green, and nothing looks for a child. So a skip is now tolerated ONLY when `gate-skips.txt` declares it, by package and test name, with a reason somebody wrote; an undeclared skip fails the gate, and so does a STALE declaration whose test ran and did not skip, because a list of excuses nobody prunes stops describing anything. A reason beginning `BROKEN:` marks a declared skip that is a DEFECT rather than a decision, counted and printed apart on every run including a green one. And a watched product binary that is running now and was not running before is a FINDING: established by comparison, because something the owner already had open is not the gate's doing; described into the run's directory BEFORE anything is done about it, because the eight that went unnoticed had their run records in temporary stores that went away with the tests that made them; and ended only when this run can SHOW it started it. An orphan was first treated as this run's on the reasoning that nothing was left which could be waiting for it, which is the absence of somebody else's claim and not ownership: start `marco do` in a shell during the gate and close the shell, and what is left is indistinguishable from a child of this run that lost its parent. So ownership is POSITIVE ONLY — something this run started, or something seen below it while that chain was still intact, which is why the lineage is sampled DURING each suite rather than reconstructed from the wreckage afterwards — and anything else is reported, preserved and LEFT RUNNING. AND A PID IS NOT A PROCESS, which is the same mistake one level down and was got wrong second: what was remembered was a bare pid, indefinitely, and Windows hands a pid back out once its process has gone — so a run that started pid 7384, watched it exit and later met that number again would have claimed somebody else{Q}s process AND EVERYTHING BELOW IT, because the walk upwards stopped at the remembered number and called the whole line its. What is remembered is an IDENTITY: the pid and the moment that pid started running. Every step of an ancestry walk is checked against it and not just the ends — a parent must be present with the identity this run established, and a child cannot have started BEFORE its parent, which is what rules out a parent id naming a replacement for the process that really did the starting — a line through a parent that has already exited is no longer followed at all, because there is nothing left to identify it by, and the kill itself opens the process ONCE and compares the start time ON THE HANDLE IT IS ABOUT TO TERMINATE, because the pid came out of a snapshot and a snapshot is a photograph. A process whose start time cannot be read is not identified, so it is reported and left running; there is no fallback to the bare pid, because that fallback is the defect. Nothing is ended until the description of it is on disk and the write has been CHECKED, because a survivor killed with nothing left behind to read is the exact state the eight were found in. The desktop harness asks the same question after EVERY scenario rather than once at the end, for attribution and for containment, and preserves that scenario's run records before ending anything; a scenario that passed and left a child running is turned into a finding, because a run that reports it finished while the process performing it is still there has not established what it says. `gate.ps1` is the whole of it in order — the suites, the packed bundle, its installation from an isolated directory, then EVERY registered scenario against those installed binaries. Packing is asked three ways, because `pack.ps1` reports a bad bundle by THROWING and `$LASTEXITCODE` then holds the zero the last successful native command left: the exception is caught, the exit code is checked, and — the only one of the three that is evidence — the bundle on disk must be one this run produced, or the gate stops before anything is installed. And every narrowing switch adds a line to the verdict: `-SuitesOnly`, `-Installed` and `-Only` each end with what the run DID NOT ASK, where all three used to end with the same sentence as a whole run. The first run found four things: three helper-process skips nobody had declared, and `TestAnUnconsumedArmDoesNotSurviveIntoTheNextRecording` skipping because two bounded-shutdown tests left `recWedged` set for the whole package — a real test silently not running, three files away. Three more declarations are BROKEN rather than expected: two mutation gates in `internal/director/observe` and one rehearsal test in `cmd/director` build a scenario, find it did not produce the condition they are about, and decline. PHYSICAL RECORDING CAPTURE REMAINS NOT EXERCISED, and the eight orphans' CAUSE REMAINS UNRESOLVED: one clean run does not close it, and what this changes is that a recurrence is exposed and contained rather than silent."
supersedes: []
affects:
  - execution
source_paths:
  - internal/gate/gate.go
  - internal/gate/gate_windows.go
  - internal/gate/ownership.go
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

**And whether it may be ended is separate from whether it is a finding.** It is this run's
to end only when this run can **show** it started it: see the next section, which is where
the first version of this was wrong.

The harness asks after **every scenario** rather than once at the end — attribution, because the
scenario that leaked is the one worth looking at, and containment, because a Studio or a `marco`
left running changes what the next scenario sees. A scenario that passed and left a child running
becomes a finding: a run that reports it finished while the process performing it is still there
has not established what it says. Its own evidence is kept beside the finding; the verdict is
what is replaced, not the account.

## Absence is not ownership

The first version of this ended an **orphan** — a survivor whose parent id names nothing — on the
reasoning that nothing was left which could be waiting for it. That is not ownership. It is the
absence of somebody else's claim, and the two come apart in a case anybody can produce by
accident: start `marco do` in a shell while the gate is running, then close the shell. The process
is orphaned, it is at a watched path, it appeared during the run, and it is not the gate's to
kill. Nothing about it distinguishes it from a child of this run that has lost its parent, which
is exactly why absence cannot be the test.

So ownership is **positive only**. A pid is this run's when this run started it, or when it was
seen below something this run started **while that chain was still intact**. Everything else is a
finding that is reported, preserved and left running — which is the direction this has to be wrong
in: a gate that fails loudly about somebody else's process costs them a sentence, and a gate that
kills it costs them their work.

**Which is why the lineage is watched rather than worked out afterwards.** The chain breaks. A
Studio exits and the `marco` it spawned keeps running with a parent id naming a process nobody can
ask about any more; the test binary above that one is gone too. By the time anybody looks there is
no path from the gate to the child at all — and that is precisely the shape of the eight. Nothing
can be deduced from the wreckage, so `gate.Ownership.Note` is given a snapshot every two seconds
while a suite runs and remembers whatever is then below a pid already known. A process whose whole
ancestry both appeared and died between two samples cannot be established and is left running with
the report saying why. That is the honest limit, and it is stated rather than papered over.

**And a pid is not a process.** The same mistake one level down, and the one this got wrong
second. What was remembered was a bare pid, indefinitely — and Windows hands a pid back out once
its process has gone, soon and from a small table, so during a gate that spawns a Studio, a
director and a `marco` per scenario the same number comes round again. The label outlived the
thing it was attached to: a run that started pid 7384, watched it exit and later met pid 7384
would have claimed somebody else’s process, **and everything below it**, because the walk upwards
stopped at the remembered number and called the whole line its.

So what is remembered is an **identity** — the pid and the moment that pid started running, read
from `GetProcessTimes` through the same path for every use of it, so two readings of one process
compare exactly equal and two processes sharing a pid never do. Three consequences, every one of
them a narrowing:

- A process whose start time cannot be read is **not identified**, so it is not claimed and not
  ended. There is no fallback to the bare pid; that fallback is the defect.
- Every step of an ancestry walk is checked, not just its ends. A parent has to be in the snapshot
  **with the identity this run established**, and a child cannot have started before its parent —
  which is what rules out a parent id naming a *replacement* for the process that really started
  it. A line through a parent that has already exited is no longer followed at all, because there
  is nothing left to identify it by; what carries ownership across that gap is `Note`, which
  claims the child in its own right while its parent is still alive.
- **The kill checks the handle it is about to use.** The pid came out of a snapshot, and a
  snapshot is a photograph: between the photograph and the kill the process can exit and its
  number be handed on. So `gate.End` opens the process once, reads the start time from that
  handle, compares it to the identity written down, and terminates **that same handle** — a handle
  names a kernel object rather than a number, so nothing can be swapped underneath it. It takes an
  `Identity` and not a `uint32`, so there is no longer a way to express the old call.

## And nothing is ended until the evidence is on disk

The description used to be assembled *as* the processes were killed and saved afterwards, with the
save's error discarded — so the one run that mattered was the one that lost everything. A gate
whose diagnostics directory could not be written would have killed every survivor and left exactly
the silence the eight were found in.

So the description is written and the write is **checked** first; only then is anything ended, and
what was ended or left running is appended to the same file afterwards. A failed write means
nothing is touched at all, and the finding says so. Evidence is the only thing that licenses
ending a survivor, so a run that cannot keep it has not earned the right.

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

**And each stage has to establish that the stage before it happened.** `pack.ps1` validates what
it staged and reports a bad bundle by `throw`ing — a missing `director.exe`, a Studio built
without the Wails tags, a binary carrying no VCS revision. `$LASTEXITCODE` is then whatever the
last native command that SUCCEEDED left behind, which is zero. A `pack.ps1` that is missing, or
that swallows a failure of its own with `-ErrorAction`, raises nothing that reaches the caller and
sets no exit code either. In both shapes an exit-code check passes and the next stage installs and
gates **whatever `dist\Marco.exe` was already there**, which could be from any build at all — and
the installed-bundle result, which is the whole provenance claim of stages 3 and 4, would be
about a bundle nobody can name. So the exception is caught, the exit code is checked, and the
bundle itself is asked whether it is one this run produced: written during the packing, or at
least not the file that was there before it began. Two arms, because `go build -o` dates its
output now and `Copy-Item` PRESERVES a write time, so neither covers the other — and what a
packer that did nothing cannot produce is a change.

**A narrowed run does not get the sentence a whole one gets.** `-SuitesOnly`, `-Installed` and
`-Only` each leave a question nobody asked, and each now adds a line to the verdict saying which:
a partial run used to end with "the suites, the packaged bundle, its installation, and every
registered scenario", which is a claim about a bundle it never packed and scenarios it never
selected. The pass is real; what it is a pass OF is the part that was being overstated. There is
now one place the script can end, so a failing stage cannot exit past the stage table either —
which is how a run that died in packing used to leave nothing behind saying where it got to.

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

## What it reports

Run twice on 2026-10-28, from the working tree and then from the committed one so the result is
reproducible from a clean checkout:

- **the suites**: 0 failures, 0 findings, 30 tolerated skips, every one declared with a reason
- **the bundle**: `88de884-20261028005038`, every Go binary stamped `88de884` with no `+dirty`
- **its installation**, from an isolated directory with `LOCALAPPDATA` and `APPDATA` pointed away
  and PATH cut to the system directories: no problems reported
- **the harness**: `desktop 23 of 23`, `recording 1 of 1` — all 24 registered scenarios, against
  those installed binaries, with **no scenario leaving a process running**

The eleven-scenario run recorded above was a selection. The twelve this adds are the guarded drag
in four forms, the chord, the three holds, the right and middle press with its moved and resized
refusals, and the recording path.

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
- `internal/gate` `TestOwnershipIsEstablishedWhileTheChainIsAlive` — one sample below this run
  establishes the three processes under it and nothing that was never under it; the chain is then
  broken and the child is still recognised; and a run that never watched declines to claim it.
  Which is also the case for a child that OUTLIVES its parent: its ownership was established in
  its own right while both were alive, and stays established afterwards.
- `internal/gate` `TestAReusedPidIsNotTheProcessThisRunStarted` — a pid this run started and saw,
  then exited and given to somebody else’s shell: neither the replacement nor the `marco do` under
  it is claimed, watching learns nothing from the snapshot it appears in, and a process older than
  the process its parent id names is refused as well — which is the same reuse in the middle of
  a line, and is what a walk that checked only its ends would have claimed.
- `internal/gate` `TestAProcessWithNoStartTimeIsNotEnded` — a survivor the platform would not
  identify is reported and left running, with the sentence saying it cannot be told apart from
  another process given the same pid; and two identities neither of which is known are not equal.
- `internal/gate` `TestTheProcessEndedIsTheOneThatWasWrittenDown` — against a REAL process, which
  is where the handle is the whole question: an identity whose start time is not the one on the
  handle is REFUSED and the process is still running afterwards, a bare pid with no start time is
  refused too, and then the identity that was written down IS ended — without which every
  refusal above is satisfied by an End that ends nothing at all.
- `internal/gate` `TestOnlyThisRunsChildrenAreEnded` — a child of this run is ours, a process with
  a live foreign parent is LEFT ALONE with the sentence naming the parent, an orphan nothing
  connects to this run is LEFT ALONE TOO, the same orphan becomes ours once this run can show it
  started what started it — and only while that process is THERE to be identified, because a
  number this run remembers is not on its own a reason — and a sweep with no ownership at all ends
  nothing.
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
- `cmd/gate` `TestNothingIsEndedUntilTheEvidenceIsOnDisk` — against a REAL process, with a
  diagnostics directory that cannot be written: the finding says nothing was done to any of them,
  and the child is still running afterwards.
- `cmd/gate` `TestAnOrphanThatIsNobodysChildIsNotEnded` — a survivor at a watched path whose
  parent is gone and whose ancestry reaches nothing this run started: reported, written down as
  `LEFT RUNNING`, and not ended.
- `cmd/gate` `TestTheGateCallsWhatItIsFor` — the wiring, by walking the command: `suite` takes a
  snapshot, records the suite it spawned, starts the lineage watcher and looks for survivors;
  `lookForSurvivors` hands over to `endWhatItMay`, which writes before it ends; `run` reads the
  ledger, runs the modules, and builds the engine first.
- `cmd/gate` `TestAPackerThatThrowsStopsTheGateBeforeAnythingIsInstalled` — the real script, run
  against a stand-in tree whose `pack.ps1` succeeds at a native command and then throws, with a
  bundle from before the run on disk: the gate fails at `pack` and stages 3 and 4 do not happen.
- `cmd/gate` `TestAnOldBundleIsNotMistakenForTheOneThisRunPacked` — a `pack.ps1` that swallows
  its own failure and exits 0 without producing anything: the bundle itself refuses the run.
- `cmd/gate` `TestANarrowedRunSaysWhatItDidNotAsk` and `TestASuitesOnlyRunDoesNotClaimTheGate`
  — `-Installed -Only` and `-SuitesOnly` each pass, each say "PASSED WHAT IT RAN, WHICH WAS NOT
  THE GATE", and each name their own narrowing in the verdict; neither claims every registered
  scenario.
- `cmd/gate` `TestAWholeRunClaimsTheGate` — the other half, without which a verdict that said
  "not the gate" unconditionally would pass every assertion above and claim nothing ever again.
- `cmd/gate` `TestTheLedgerInThisTreeReads` — this tree's own ledger parses, declares nothing
  twice, and names only packages in this repository.
- `plugins/e2e` `TestASurvivingChildIsAFindingAndNotEndedBlind` — a survivor whose parent is a
  live foreign process: the result becomes a finding, the scenario's own evidence is kept, the
  description and the run records are written out first, and nothing is ended.
- `plugins/e2e` `TestAScenarioThatLeftNothingRunningIsReportedAsItWas` — the other half: a clean
  scenario is untouched, and a check that could not be made does not read as one that found
  nothing.
- `plugins/e2e` `TestNothingIsEndedWhenTheEvidenceCannotBeKept` — a survivor this run does own,
  with an artifacts directory that cannot be written: nothing is ended, and the finding does not
  claim otherwise.
- `plugins/e2e` `TestThisRunRemembersWhatItStarted` — against a REAL process this run starts:
  its identity is remembered, the same pid with a different start time is NOT it, a child seen
  below it stays this run’s once its parent has gone, and an orphan nothing connects to this run
  is a finding rather than a kill.
- `plugins/e2e` `TestEveryProcessThisHarnessStartsIsRememberedAndSweptFor` — every function in the
  package that starts a process also remembers it, something calls the sweep, and `main` takes the
  two snapshots it needs and starts the lineage watcher exactly once. By walking the package
  rather than naming the two places, because naming one place is how the second was missed the
  last time.
