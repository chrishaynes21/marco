---
type: adr
status: accepted
date: 2026-10-29
decision: "ACCEPTED — A TEST GOES BEHIND `-tags desktop` WHEN ITS PRECONDITIONS ARE FACTS ABOUT THE OWNER'S SCREEN, AND EVERYTHING ELSE SUPPLIES ITS OWN DESKTOP. The screen-free stage has to be usable while somebody is using their computer, and it was not: fourteen tests of Director's RUN LOGIC — a deadline being announced, a question being withdrawn, a grant for the wrong Act refusing, a record naming the account it came from, a startup failure being recorded — had to manufacture a performable act first, and the only way to get one was to photograph whatever happened to be in front of the owner. So they inherited every precondition that photograph has: a window must be there, hold still across three looks, propose something control-shaped, and keep that control under the same point. With a game on screen those preconditions fail and `cmd/gate` reported eight and then six UNDECLARED SKIPS, a different set each run, for a reason not one of those tests was about. A CONDITIONAL SKIP IS THE ONE THING `gate-skips.txt` CANNOT EXPRESS — an undeclared skip fails the gate and a declaration whose test ran fails it too, so the same honest test is a finding with a game on screen and a finding without one — which is the reasoning that already put the live resolver behind `-tags resolver` and the perception corpus behind `-tags corpus`. THE PIXELS MAY BE DRAWN AND THE APPLICATION MAY NOT, which was measured rather than assumed: building the routine from a drawn window while keeping the live window's identity left all eleven passing with a game animating in front, because none of them presses anything and the two that get furthest are satisfied by the run REFUSING for want of an answer; stating the identity as well failed them all, with `this workflow was demonstrated against \"marcoprobe\" and the application resolved now is \"code\"` — so the whole desktop is stated, not only the picture. PRODUCTION PERCEPTION STILL READS THE PIXELS: `structure.Propose` finds four control-shaped regions in the drawn window and three looks agree at 1.000 against the 0.55 a run requires, where a real window in front of somebody playing agreed at 0.339. Nothing here stubs perception; it supplies pixels and lets perception answer. THE DESKTOP SEAM HAS NO INDIVIDUAL KNOBS, because a stated window list under a live handle lookup is the shape of defect that makes a test pass while the rule it names does nothing — `winprovider.DesktopForTest` answers the window list, the handle lookup, process liveness and ownership out of one list. AND ONLY ONE OF `groundbykey_test.go`'s THREE TESTS MOVED: the other two drive the production grounder over a synthetic artifact and a synthetic key desk and reach no desktop at all, so tagging the file would have taken two claims that need no screen out of the default suite, which is the quiet loss of coverage this slice exists to prevent. THE GUARD THAT HOLDS THE LIVE TESTS IS NOT TAGGED, which is the same mistake read from the other side: the guard asserting those tests cannot reach an actuator went behind the tag with the file it protected, so the guard would have been absent from every run except the one where the tests it guards are running against a real screen. It is untagged now, covers both tagged files, names them in a list, and a second test walks the directory so a new tagged file cannot be unguarded and look guarded."
supersedes: []
affects:
  - testing
source_paths:
  - internal/platform/winprovider/stated.go
  - internal/platform/winprovider/live.go
  - cmd/director/statedact_test.go
  - cmd/director/actfixture_test.go
  - cmd/director/desktoptests_test.go
  - cmd/director/groundbykeylive_test.go
  - cmd/gate/main.go
  - gate.ps1
---

# A screen-free gate, and the tests that need the screen

## What the screen-free stage was

`go run ./cmd/gate` is stage 1 of the gate and the stage with no screen in it. It is the one a
person can run while they are using their computer, and the one this session ran repeatedly.

It was not usable that way. Two consecutive runs of the same tree reported **8 findings** and
then **6**, overlapping but not equal, every one of them an undeclared skip in `cmd/director`
and every one naming the same cause:

```
"Trackmania" {0 0 1920 1080} is not holding still: looks 1 and 2 agree at 0.339,
below the 0.55 the run requires, so a routine built from one of them cannot be
performed against a look taken later
```

A game was on screen. That reading is correct, and the tests it stopped were tests of
arithmetic and bookkeeping.

## Why the diagnosis needed tracing and not inferring

Variation across runs plus an animated game strongly suggests an environmental cause. It does
not establish one. What establishes it is the dependency: every message comes from one fixture,
`aRoutineFromALookAt`, which photographs the live foreground window, runs production perception
over the pixels and builds a routine from what it finds. Fourteen tests reached it — eleven
through `anActOverTheWindowInFront`, three more through sibling helpers that wanted *an
acquirable application* — and inherited its preconditions.

None of those fourteen is about perception. They are about a deadline being announced, a
question being withdrawn, a grant for another Act refusing with exit 1, a play's record being
written from its own account, a startup failure being recorded, a press boundary taking no
look. They needed a performable act, and the only way to get one was to borrow the owner's
screen.

## What was measured, in two steps

**The pixels may be drawn.** The fixture was pointed at a drawn window while keeping the live
window's identity, so the saved arrangement could not possibly match the screen. All eleven
passed, with the game still animating. The run never compares the saved arrangement with the
screen in these tests: none of them presses anything, and the two that get furthest are
satisfied by the run refusing for want of an answer.

**The application may not.** Stating the identity as well failed them, every one:

```
this workflow was demonstrated against "marcoprobe" and the application resolved
now is "code"; a workflow may only act on the application it was shown in
```

That guard is right, and it is why the whole desktop is stated rather than only the picture.

## The decision

A test goes behind `-tags desktop` when its preconditions are **facts about the owner's
screen**. Everything else supplies its own desktop and runs by default.

`winprovider.DesktopForTest` is how. It answers the window list, the handle lookup, process
liveness and ownership **out of one list**, and it has no individual knobs on purpose: a stated
window list under a live handle lookup is the shape of defect that makes a test pass while the
rule it names does nothing, which is the same defect
[[ADR-268-record-only-the-chosen-window]] fixed one layer up when a fixture window list sat
under a live foreground reading.

**Production perception still reads the pixels.** `structure.Propose` finds four
control-shaped regions in the drawn window, and three looks agree at 1.000 against the 0.55 a
run requires — where a real window in front of somebody playing agreed at 0.339. Nothing here
stubs perception. It supplies pixels and lets perception answer, so the routine the stated
fixture saves is one the production proposer agreed to.

**And no conditional skips.** `gate-skips.txt` can express neither state of an
environment-dependent skip: an undeclared skip fails the gate, and a declaration whose test ran
fails it too. The ledger is for a skip that is always a skip. `-tags resolver` and `-tags
corpus` are the precedent, and the rule in CLAUDE.md already said so: *any other test that needs
something this repository does not contain belongs behind a tag.* A quiet screen is something
this repository does not contain.

## The inventory

Every test that moved, against the claim it establishes. **Nothing in the right-hand column was
replaced by the stated fixture** — the stated fixture exists for the tests in the first table,
which never claimed anything about a live screen.

### Stayed, and now supply their own desktop (`anActOverAStatedWindow`)

| Test | The claim it establishes |
|---|---|
| `TestAGrantForAnotherActStopsTheCommand` | a grant naming another Act exits 1, before the run opens its account |
| `TestAGrantedRunAsksNobodyAnything` | a grant suppresses every question, and says what authorised it |
| `TestAStalledPlayEndsAtItsDeadline` | a run nobody answers returns within `--seconds`, with a finalized record |
| `TestThePlaysRecordIsWrittenFromItsAccount` | the record comes from the run's own account |
| `TestAnAnswerAfterTheDeadlineAuthorisesNothing` | 0 actions sent, and the consent-desk counts do not rise after the deadline |
| `TestSayingReadyEndsPreparationExplicitly` | preparation ends when the person says they are ready |
| `TestPreparationRunningOutEndsItExplicitly` | preparation ends when its delay runs out, and says so |
| `TestTheStartQuestionOffersOnlyTheKeysStartAccepts` | the start question accepts exactly the keys it offers |
| `TestTheClockOnlyPreparationIsAnnouncedAndEnded` | a clock-only preparation is announced and ended |
| `TestThePreparationDeadlineIsSaidAndAnEarlyStartIsHonoured` | the deadline is emitted by the runner, and an early start is honoured |
| `TestAQuestionThatExpiresIsWithdrawnFromTheState` | an expired question is withdrawn rather than left open |
| `TestAnAbsentOptionalSourceIsRecordedAsAStartupFailure` | an absent optional source is recorded as a startup failure |
| `TestRepairWithoutOCRIsRecordedAsAStartupFailure` | repair without OCR is recorded as a startup failure |
| `TestGeometryReplayTakesNoLookAtThePressBoundary` | geometry replay takes no look at the press boundary |

### Moved behind `-tags desktop`

| Test | The claim it establishes | Why no controlled counterpart |
|---|---|---|
| `TestAHotkeyRunReachesAnAuthorisedActionWithoutFrom` | a hotkey run reaches an authorised action with no `--from`, naming the screen by key | the claim is that this works against the window actually in front of somebody; a drawn window would establish the plumbing and not the claim |
| `TestTheRealBoundaryRefusesATargetThatLostTheDesktop` | the real foreground boundary refuses a target that lost the desktop | the boundary's subject is the real desktop; that is the whole test |
| `TestTheRealBoundaryRefusesAnApplicationTheWorkflowDoesNotBelongTo` | the real boundary refuses an application the workflow does not belong to | as above |

### Stayed, and never needed a screen

| Test | Why it was nearly moved |
|---|---|
| `TestAHotkeyRunEstablishesItsScreenByKey` | it sits in `groundbykey_test.go`, whose other test is live. It drives the production grounder over a synthetic artifact and a synthetic key desk and reaches no desktop at all. |
| `TestTheScreenOfferedByKeyIsTheRankedCandidateWithItsScore` | as above |

Tagging that file wholesale would have taken both out of the default suite. That is the quiet
loss of coverage this slice exists to prevent, and it was caught by checking rather than by
reading the filename.

## The guard is not tagged

The guard that asserted the live boundary tests cannot reach an actuator lived in the file it
protected, and went behind the tag with it — so the guard would
have been absent from every run except the one where the tests it guards run against a real
screen. A guard that is only present when the thing it guards is running is not a guard.

It is untagged now, as `TestADesktopTestCanObserveButNeverAct`, and it covers **both** tagged
files from a named list. `internal/winctx` is on its forbidden-import list for a new reason: a
desktop test must read the world through the provider seam rather than around it, because a test
that calls `winctx` directly cannot be given a stated desktop, and that is how this class of
test came to depend on whatever the owner had open.

And `TestEveryDesktopTaggedFileIsHeldByTheGuard` walks the directory, because a list the guard
reads is a list a new file can be missing from — which would leave it unguarded and looking
guarded.

## What the stages say now

`cmd/gate` prints the tagged suite as NOT RUN with the command that runs it, so a reader of the
screen-free stage is told they did not run rather than finding out from the absence of their
names. `gate.ps1` gained a stage: **4/5, the tests that need the screen**, before the harness
rather than after it, because these only LOOK at the screen while stage 5 performs real input
into its own fixture window — so a failure here is read against a desktop nothing has pressed
yet.

## What this does not change

No production perception behaviour. The seam is four package variables that production never
replaces, and the only way to set them is a `testing.TB` helper that restores them. No test's
assertions were weakened: the fourteen assert exactly what they asserted before, against a
desktop they state instead of one they borrow.

The three `BROKEN:` declarations in `gate-skips.txt` are untouched and remain parked findings;
they are a different problem (Roadmap 37), not an environmental one.

## One survivor, recorded rather than contrived against

**12 of 13 mutations killed.** Two of the kills are worth naming because they were survivors
first, and both for reasons this slice is itself about:

- **A skip is green.** The harness read only the exit code, so the mutation that removed the
  background window from the stated desktop SURVIVED: it turned
  `TestGeometryReplayTakesNoLookAtThePressBoundary` into a skip, and `go test` reports a skip as
  success. The same trap `cmd/gate` exists for, reappearing inside the gate for the slice that
  was fixing it. The harness reads `--- SKIP` now.
- **A guard about a broken tree cannot be established from an unbroken one.** Emptying the
  forbidden-import list and deleting either arm of the unheld-file rule both survived, because
  there was nothing in the tree for them to catch. Both rules are pure functions over facts now
  (`whatReaches`, `unguardedFiles`), with the broken trees stated in a table instead of arranged
  in the repository.

And the stage in `gate.ps1` was wired by nothing at all. **Deleting the whole of it left every
case in `cmd/gate`'s script test passing**: the verdict still claimed the gate, `-SuitesOnly`
still listed the stage among what it had not asked, and the only evidence it had gone was its
absence from output nobody read. Three assertions now hold it — every stage named in a whole
run, a mark on disk left by the tagged test rather than a header in the output, and a
`//go:build !desktop` test in the stand-in tree that fails if the stage ever runs the default
suite instead of the tagged one.

The survivors:

- `DesktopForTest`'s **ownership arm** is unfalsifiable. The handles a fixture states do not
  exist, so the real `IsOwnedSurface` reads a property off nothing and already answers false for
  them; deleting the line changes no behaviour a test can reach. It stays because the two
  falses mean different things — the real one is "the property could not be read", this one is
  "a stated window IS a window the person could have meant" — and because a fixture handle
  colliding with a live Marco surface would otherwise vanish from its own desktop, which is not
  a collision a test can arrange.
- The stand-in tree's **mark-writing test being tagged** is behaviourally equivalent. Moving the
  mark into an untagged file still establishes both things under test: that the stage ran
  `./cmd/director`, and that it ran it WITH the tag — the second from the `//go:build !desktop`
  test that would otherwise have failed. Which file leaves the mark is not part of the claim.

## Enforced by

- `cmd/director` `TestADesktopTestCanObserveButNeverAct` — both tagged files, asserted over
  imports rather than text, because a text scan finds the forbidden words in the list that
  forbids them.
- `cmd/director` `TestTheImportRuleCatchesAFileThatCouldAct` — the rule itself, against a file
  that imports an actuator and one that reads the desktop around the seam, with a file that only
  observes beside them and the handed-nothing case. The guard above cannot establish its own
  rule: the tagged files do not import an actuator, which is the point of it.
- `cmd/director` `TestEveryDesktopTaggedFileIsHeldByTheGuard` — walks the directory, so a tagged
  file missing from the guard's list is a failure rather than a silent gap.
- `cmd/director` `TestTheGuardsRuleCatchesBothWaysOfEscapingIt` — a tagged file nobody named, a
  named file that is not tagged, and a tree in order. Same reason as the import rule: both arms
  are about a tree somebody has just broken.
- `cmd/gate` `TestAWholeRunClaimsTheGate` — every one of the five stages named, plus the mark
  the tagged test leaves on disk. Deleting the stage from `gate.ps1` must fail this.
- `cmd/gate` `TestADesktopTestThatFailsStopsTheGate` — the stage's failure path, which nothing
  exercised: a failing tagged test stops the run, the stage is named, and the harness that
  presses things never runs. Ignoring the stage's exit code must fail this.
- `cmd/gate` `TestASuitesOnlyRunDoesNotClaimTheGate` — each piece of what a partial run did not
  ask, asserted separately, so a stage added later either appears in the verdict or fails by
  name rather than changing one sentence nobody can diff.
- `cmd/director` `TestTheDrawnWindowIsSomethingPerceptionCanRead` — four regions, every one
  control-shaped, and three looks agreeing at 1.000. The numbers quoted in this note were
  measured and then asserted by nothing, and a fixture needs only ONE control-shaped region to
  manufacture an act — so this picture could have degraded to something that no longer resembles
  a window while every test above kept passing.
- `cmd/director` `TestAGrantForAnotherActStopsTheCommand` and the thirteen others in the first
  table above — all fourteen run in the default suite, against
  `winprovider.DesktopForTest`. Deleting the stated desktop must fail
  `TestAQuestionThatExpiresIsWithdrawnFromTheState`, which is the mutation that was run: the
  tests that assert a REFUSAL still pass without it, because the refusal then happens for
  another reason.
- `cmd/director` `TestAHotkeyRunEstablishesItsScreenByKey` and
  `TestTheScreenOfferedByKeyIsTheRankedCandidateWithItsScore` — in the default suite, which is
  the half of the split that would have been lost silently.
