---
type: adr
status: accepted
date: 2026-10-30
decision: "ACCEPTED — AN EDIT CHANGES A ROUTINE'S STEP LIST, THE SOURCE IS RE-RENDERED FROM IT, AND THE STEP LIST IS WRITTEN DOWN BESIDE THE ROUTINE AS THE ACCOUNT OF WHAT ITS SOURCE IS THE RENDER OF. A step editor already existed (`cmd/marco/edit.go`) and it edits SOURCE LINES: two of the four things a step-editing contract needs are there, keyed by line index, and its remove is already known to be broken for a generated routine — the repository's own test says deleting a `do OS's Find …` row takes the header and leaves `when ok?` with no arm, which the compile gate then refuses. That shape is every press in a current-generation routine, because a step's boundary is in the IR and only approximately in the file: the chained shape makes each press its own capability called inside the previous one's `when ok?`, with the single `this is ok!` and the trailing settle wait inside the LAST link, and an anchored click in the unguarded shape is a dozen lines three indents deep. So an edit is expressed over `[]macroir.Step` and the source is rendered again through the one renderer `Save` and every review already use. TWO OPERATIONS AND NO MORE: set a wait's duration, remove one step. A gesture is removed WHOLE — `macroir` already makes a drag, a typed run, an anchored click and a folded repetition one step each, and the only gesture with two halves is a held key, so a removal resolves the pair and takes both and says so, and a half whose partner cannot be found is refused rather than guessed. A removal leaves the WAITS around it, because a wait is its own step and choosing a new duration is a second operation somebody makes on purpose. Removing the last link of a chain renders a SHORTER CHAIN, with the answer and the settle wait moving into the link that is now last; an edit that would leave no steps at all is refused with `Deleting the routine` named, because a routine that does nothing is not a smaller routine and deletion already sweeps what sits beside it. AND THE WHOLE OF IT RESTS ON THE ACCOUNT. `<slug>.steps.json` holds a version, a digest of the source it renders to, a digest of the demonstration it descends from, and the steps — four fields and NO EDITING HISTORY, because what a routine does now is necessary and a record of how it got there is not. It is read into one of six states and only `intact` licenses rendering, so a file that is old, half-written by a failed writer, from a build this one does not understand, or THERE AND UNREADABLE cannot be believed about whatever happens to be beside it; absence is `os.IsNotExist` and nothing else, because the first implementation answered absence to every read error and absence is the single answer that licenses rendering the demonstration instead and writing over whatever the file says. An account missing either digest is refused AT THE WRITE, because a file nothing can check is the hazard every other rule here exists to prevent. AND THE FILE RECORDS A BASIS, NOT AN AUTHOR: a step edit writes one and so does a re-simplification, nothing on disk tells them apart, and so nothing here says which. A MISMATCH IS DESCRIBED NEUTRALLY — a source digest that moved is equally consistent with a hand edit and with an account write that failed after the source landed, and nothing on disk tells them apart, so the sentence says the source and the stored steps no longer match and points at a rebuild review. Editing is offered only where the saved source IS what the routine's current account renders today — the stored steps when there are usable ones, the demonstration when there are none — and that is CHECKED BY RENDERING rather than asserted, which is also what keeps this from being a one-use editor: after the first save the source differs from the demonstration on purpose, and that difference is the edit. The review writes nothing, shows both sources in full, and the proposed source passes `driver.CheckSource` before it can be offered, because `codegen` is trusted everywhere else on the grounds that its input came from a recorder and after an edit it did not. The save is the version-bound path: the route's cross-process lock before the first read, all THREE digests re-read and the refusal naming which moved, then assets, then the source atomically, then the account LAST — it cannot be written first because the digest it records is of a source that does not exist yet, and a failure there leaves the routine running the edit with further editing blocked and SAID, which is the conservative half of that trade. AND EVERY OTHER OPERATION ON A ROUTINE DECIDES ABOUT IT, because a decision left for later is a conflict that ships: a MOVE renders the account rather than the demonstration and carries it rebound to the source it wrote, and the account is part of READING THE DESTINATION BACK so the original is never removed until the moved routine is editable; a REPAIR renders the account and writes it back, and REFUSES ALTOGETHER when the account cannot be read, because `Resave` is the one writer that consents to nothing and a routine being unrunnable is not evidence that somebody's edits are disposable; a GENERATION REBUILD renders the account so the generator's new rules reach an edited routine; a RE-SIMPLIFICATION renders the demonstration, says `This rebuild replaces the stored steps with a fresh simplification of the demonstration.` beside the comparison it already shows, and writes an account of what it rendered instead — because that source is made under rules that are not the default and nothing else could ever explain it, and because a sentence about "your step edits" would be a guess about a rebuild with no edit in it; the other arm, a rebuild over stored steps that no longer describe the routine, removes them and says so in its own sentence; RECORDING THE ROUTINE AGAIN removes the account and the collision question says so, which is the one place losing the edits is right; RENAMING carries the file for the same reason the provenance sidecar's rename carries its own, that the digests are over contents renaming does not change; DELETING takes it, and a move's removal takes it and names it as residue when it cannot. `marco edit` is UNCHANGED and that is stated rather than discovered: its save leaves the account mismatched, the routine still runs because execution reads only the `.marco`, step editing then refuses it, and the cost is that the person's edits are no longer the explanation of their file and that editor does not tell them. The Studio controls are a separate increment. No count of edits is ever reported, because nothing durable records them and two operations on one wait are one difference to read and two to count. AND WHAT A REPLACEMENT WOULD DO TO THE STORED STEPS IS DERIVED RATHER THAN RECORDED: `DiscardsEdits` is a method over two unexported fields, not an exported field a caller could flip to change which of three things a replacement did to a file the review never showed them, and both of those fields are sealed into the token — a derived answer may stay out of a token, what it is derived from may not. The step list a review hands out is a DEEP copy, because a `macroir.Step` carries an anchor's picture and a repetition's body by reference and the shallow version looked exactly like a copy while defending neither."
supersedes: []
affects:
  - execution
source_paths:
  - internal/routes/steps.go
  - internal/orchestrator/stepedit.go
  - internal/orchestrator/regenerate.go
  - internal/orchestrator/rescope.go
  - internal/orchestrator/draft.go
  - internal/orchestrator/keep.go
  - internal/routes/registry.go
---

# A routine writes down what it does

The trace this came out of, with the alternatives considered and rejected, is
[[Step-editing-proposal-2026-10-30]].

## The thing that was missing was not the save

Version-bound replacement was already built. `internal/orchestrator/regenerate.go` is
SELECT → PREPARE → REVIEW → REPLACE, with the review held in memory, a token derived from the
content and the target, the route's cross-process lock taken before the first read, both digests
re-read, assets first and the source landed atomically
([[ADR-266-one-set-of-rules-for-keeping-a-recording]]). A rescope is the same contract across two
identities ([[ADR-271-a-rescope-is-a-reviewed-move-between-identities]]).

What was missing was everything BEFORE it — and one fact underneath all of it: **Marco could not
tell an edited routine from an unedited one.** A demonstrated routine gets no `.origin.json`
(nothing writes `routes.KindTaught`), so there was no durable digest of "the source as generated".
The only available answer was computed — render the demonstration and compare — and that answer
says "somebody edited this" and "an older generator wrote this" in the same voice.

So a rebuild of an edited routine offered to replace it with the person's own change undone, under
a sentence about generations. That is the defect this decision closes, and the account is how.

## An edit is of the step list, not of the text

`cmd/marco/edit.go` is a real step editor and it edits source LINES. Two of the operations a
contract needs are already there — `saveReq.Waits` keyed line→ms, `saveReq.Deletes` a list of line
indexes — and its remove is **already known to be broken** for a generated routine. Not predicted:
`cmd/marco/editsave_test.go` carries the case in its own words, that deleting a `do OS's Find …`
row takes the header and leaves `when ok?` with nothing to be an arm of. The compile gate refuses,
the file survives, the person's work is kept — the right answer, and not the asked-for behaviour.

That shape is not an edge case. It is what every press in a current-generation routine looks like:

```
this can Step3.
this's Step3 does...
    the p2 is a Point with X 900, Y 300, RelX 780, ... .
    do OS's Sleep with 2500.
    do OS's Click with p2...
        when ok?
            do OpenGraphics's Step4.
            this is that!
        or?
            this is that!
```

Each press is its own capability, the next is called inside the previous one's success, and the
single `this is ok!` with the trailing settle wait sits inside the LAST link. Removing a press is a
re-render. Removing the last press is a re-render that hands the answer and the wait to the link
which is now last. **Neither is line surgery**, and a wait is the one operation line surgery gets
right — which is the one the existing editor already does.

## The account

`<slug>.steps.json`, beside the routine:

```json
{
  "version": 1,
  "source": "<16 hex>",
  "demonstration": "<16 hex>",
  "steps": [ … ]
}
```

`steps` is what a render turns into the saved source. The two digests are what the account
describes. **No editing history**, and that is a decision rather than an omission: a journal was
drafted to let a discard sentence enumerate, and the review already shows both sources in full,
which is better evidence of what would be lost than any summary — and a count could not be had
honestly anyway, because two operations on one wait are one difference to read and two to count.

**It records a basis, not an author.** Two things write one: a step edit, and a re-simplification,
which has no edit in it at all. Nothing on disk tells them apart, so the presence of an account
means "the basis this source was rendered from is stored here" and nothing more — which is why no
sentence anywhere in this decision says somebody edited anything. A surface that wants to say that
needs evidence this file does not carry.

Six states, and `Usable` is true for exactly one:

| | means | offered |
| --- | --- | --- |
| `none` | no file — no edit, the demonstration is the whole account | editing, against the demonstration |
| `intact` | both digests match what is on disk | editing, against the stored steps |
| `mismatched` | the source digest moved | the rebuild review, saying the two no longer match |
| `adrift` | the demonstration digest moved | the rebuild review, saying which |
| `orphaned` | no source beside it | nothing |
| `unreadable` | unparseable, a version from the future, or there and unable to be read | nothing, and it says so |

Three things about that table are load-bearing. **Absence is one-way**: every routine already on
disk has no account, so a missing file may never read as a claim. **Unreadable is not absent** — and
this was wrong in the first implementation, which answered `none` to every error from reading the
file. A permission, a directory in its place, a handle something else held: each meant "nothing has
stored a basis" to every caller, which is the single answer that licenses rendering the
demonstration instead and writing over whatever the file says. Absence is `os.IsNotExist` and
nothing else, the digest a review binds itself to carries that third answer too, and every caller
refuses rather than proceeding. And a **mismatch is described as what it is** — a source digest that
moved is equally consistent with somebody editing the file by hand and with an account write that
failed after the source landed, and nothing on disk distinguishes them, so the sentence never names
a cause.

An account with no digests is refused at the WRITE (`Registry.SaveSteps`), because a file nothing
can check is exactly the hazard the digests exist to prevent.

## What may be edited

Only a routine whose saved source **is** what its own current account renders today, and that is
checked by rendering and comparing rather than asserted. An account's digests say it describes this
file; they do not say that rendering it today reproduces this file, which is a different question
and the one that matters.

This is also what keeps the editor from being a one-use one. After the first save the source
differs from the demonstration's rendering **on purpose**, and that difference is the edit — so the
second edit compares against the stored steps and the third against those.

## The write order, and what each failure leaves

| | why | a failure here leaves |
| --- | --- | --- |
| assets | additive, and only the missing ones | images nothing points at |
| source, atomically | the executable truth, and the only file a run reads | the edit in force with no matching account: further editing refused and SAID |
| account, atomically | describes what the source write produced | nothing; the row above is the state |

The account cannot go first: the digest it records is of a source that does not exist yet. The
middle row is therefore the one that matters, and it is the conservative state — **execution is
unaffected**, because a run reads the `.marco` and nothing else, and what is blocked is editing.
The sentence is the shape `SaveWithOrigin` already uses for the same situation.

## Everything else that touches a routine

| operation | the account |
| --- | --- |
| a move (`Rescope`) | rendered FROM, carried, rebound — and part of reading the destination back, so the original is never removed until the moved routine is editable |
| a repair (`Resave`) | rendered from and written back; **refused** when it cannot be read |
| a generation rebuild | rendered from, rewritten |
| a re-simplification | discarded, said, and replaced by an account of what it rendered |
| recording the routine again | removed, and the collision question says so first |
| a rename | carried, with the source, demonstration, provenance and images |
| a delete | removed |
| a move's removal | removed, or named as residue |
| `marco edit`'s save | left `mismatched`: unchanged behaviour, stated |

Two of those are worth their own sentence. **A repair refuses** rather than proceeding, because
`Deps.Resave` is the one writer here that consents to nothing — it decides on its own that the file
refuses every press and replaces it — and a routine being unrunnable is not evidence that
somebody's edits are disposable. **A re-simplification writes a new account** rather than simply
removing the old one: that rendering is made under rules which are not the default, so the
demonstration does not produce it and never will, and a routine with no account of it would be
permanently inexplicable — refused for a disagreement nobody has.

## Enforced by

`internal/orchestrator`, through `PrepareEdit` and `SaveEdit` — the two calls a surface makes —
and `internal/routes` for the file's own rules. Every routine in these tests is saved by the
production save path and rendered by the production generator; where what RUNS is the question, it
is run by the production driver over a host that records what it was asked to do.

- `TestAnEditedRoutineKeepsItsDemonstrationByteForByte` — the digest before and after. A rule with
  a test, not a property of today's path: `SaveRecording` is one call away from any writer here.
- `TestAnEditIsOfferedAgainstTheDemonstrationWhenThereIsNoAccount` and
  `TestAnAlreadyEditedRoutineCanBeEditedAgain` — the first edit, the second, and a third over the
  same wait as the first. The second is the one that fails if the precondition compares against the
  demonstration forever.
- `TestAnEditIsRefusedWhenTheSourceIsNotTheRenderOfItsAccount`,
  `TestAMismatchIsNotReportedAsAHandEdit` and
  `TestAnUnreadableAccountIsSaidAndNotTreatedAsAbsent` — the four states that refuse, the neutral
  wording, and the file that cannot answer.
- `TestARemovedHoldTakesBothHalvesOrNothing` — both directions, and the refusal for a half whose
  partner is not there. `TestARemovedStepIsRemovedWholeInBothShapes` covers the chained routine and
  the unguarded one.
- `TestRemovingTheLastLinkOfAChainRendersAShorterChain` — **not a refusal**: exactly one
  `this is ok!`, inside the link that is now last, compiled and then RUN.
- `TestAnEditThatWouldEmptyARoutineIsRefusedAndSaysToDeleteItInstead`.
- `TestAnEditedRenderThatIsNotLegalMarcoIsRefusedBeforeAnythingIsWritten` — through the
  `Deps.CheckSource` seam, because the claim is that the gate is ASKED and not that a particular
  shape fails it. Deleting the call fails this test.
- `TestAChangedWaitIsTheWaitThatRuns`, `TestAnEditedRoutinePressesWhatIsLeftInOrder` and
  `TestAFailedStepStopsWhatFollowsInAnEditedRoutine` — the duration asked for, the order of the
  remaining presses, the waits around a removed press both surviving, and a refused press ending
  the run. The last one is what says a re-render did not flatten the chain into a flat body; a
  reading of the source cannot settle it.
- `TestTheAccountIsWrittenAfterTheSourceAndNeverBefore` and
  `TestASourceWrittenWithoutItsAccountStillRunsAndRefusesTheNextEdit` — the order observed through
  its own failure, and the conservative state that failure leaves.
- `TestAnEditIsRefusedWhenTheWorldMovedWhileItWasOpen` — the source, the demonstration and the
  account, each moved on its own. The third is the case only the third digest can see: a whole
  second edit moves the source too, and the source check would answer first and say the wrong
  thing about it. `TestAnEditRevalidatesInsideTheLock` observes that the route's lock cannot be
  taken at the instant between the comparison and the write.
- `TestAnEditCannotBeSavedTwice`, `TestTwoEditsOfOneRoutineAreTwoReviews` (including a review
  assembled from two, and one saved into another library) and `TestCancellingAnEditWritesNothing`,
  which compares every file in the routine's folder. `TestTheStepsAReviewHandsOutAreACopy` holds
  that a caller cannot change what a sealed review says it is.
- `TestAReviewsTokenCoversWhatTheRoutineSaysAboutItself`,
  `TestTheStepListASaveWouldWriteIsPartOfWhatWasReviewed` and
  `TestWhatAReplacementWouldDoToTheStoredStepsIsPartOfWhatWasApproved` — **three mutations wrote
  these.** Each covers a token field that looked redundant: the account's digest, the marshalled
  step list, and the two fields a replacement branches on. See the mutation run below.
- `TestTheStepListAReviewHandsOutIsADeepCopy` — a caller cannot reach into a sealed review through
  the two references a `macroir.Step` carries: an anchor's picture and a repetition's body. The
  first version copied the outer slice only.
- `TestAnAccountThatCannotBeReadIsNotTakenForAnAbsentOne`,
  `TestAnUnreadableAccountStopsAMoveBeingPrepared` and, in `internal/routes`,
  `TestTheAccountsDigestTellsAbsentFromUnreadable` — a read failure refuses an edit, a rebuild and a
  move, each with a genuinely absent account as the control.
- `TestAReplacementIsRefusedWhenTheStoredStepsMovedWhileItWasOpen` — the third digest at the rebuild
  end.
- `TestAnOperationThatRefersToNothingIsAnError` and
  `TestEveryPositionInOneEditIsIntoTheListThatWasShown` — the split between a refusal somebody can
  read and an operation there is nothing to show about, and the rule that stops two operations in
  one review moving under each other.
- The lifecycle, one per row: `TestAMoveOfAnEditedRoutineKeepsItsEdits`,
  `TestAMoveOfAHandEditedRoutineStillShowsTheLoss` (so the new behaviour cannot be read as
  weakening the warning the move contract already carries),
  `TestAMoveVerifiesTheAccountAtTheDestinationBeforeRemovingTheOriginal` (the write failing, and
  the account overwritten in the one instant a move has that a save does not — through
  `afterDestination`), `TestAMoveIsRefusedWhenTheRoutineWasEditedWhileItWasOpen`,
  `TestARepairOfAnEditedRoutineKeepsItsEdits`,
  `TestARepairIsRefusedWhenTheAccountCannotBeRead` (one case per state, plus the control that a
  routine with no account repairs exactly as it always did),
  `TestARebuildOfAnEditedRoutineKeepsTheEdits`, `TestAReSimplificationSaysItDiscardsTheStepEdits`,
  `TestARebuildOverAnAccountThatNoLongerDescribesTheRoutineSweepsIt` and
  `TestRecordingARoutineAgainRemovesItsAccountAndSaysSo`.
- `internal/routes`: `TestAnAccountIsBelievedOnlyWhileItDescribesWhatIsOnDisk` (all six states),
  `TestAnAccountThatSaysNothingAboutWhatItDescribesIsNotWritten`, `TestTheWriterStatesTheVersion`,
  `TestRenamingAnEditedRoutineCarriesItsAccount`, `TestDeletingARoutineTakesItsAccount`,
  `TestAMovesRemovalTakesTheAccountOrReportsIt` and
  `TestNothingAboutRunningARoutineReadsItsAccount`.
- `cmd/marco`: `TestTheLegacyEditorsSaveLeavesTheAccountMismatchedAndTheRoutineRunnable` — the
  unchanged behaviour pinned down where it lives, in both directions: the account is not believed,
  and it is not swept by a surface that knows nothing about it either.

## The mutation run

**30 of 30 killed**, over three rounds: 22 sites on the first implementation, 30 after the
corrections. Every survivor was a token field rather than a decision, and every one of them wrote
a test:

- the account's digest in the edit review's token and in the rebuild review's token. Both are
  load-bearing for what a review IS, and in production nothing moves the account without moving
  the source as well — so the source digest answers a moment later and the field looks redundant.
  Killed by a test that constructs the situations which separate them: the account's bytes changed
  and its meaning unchanged, which is what another build rewriting that file would do, and a
  rebuild that would REMOVE the stored steps rather than render them, where nothing else in the
  token moves with the file. The same shape as the eighth rule of
  [[ADR-271-a-rescope-is-a-reviewed-move-between-identities]].
- the marshalled step list a save would write. Its digest looked redundant with the rendered
  source's, since the steps produce the source — but `codegen` ignores fields it has no use for, so
  two step lists can render identical Marco and still be two different things to write down.
- `fromAccount` and `DiscardsEdits` in the rebuild token. These were **removed** after the first
  round, on the argument that they are a function of things already in the token. That was
  half-right and the wrong half: `DiscardsEdits` was an EXPORTED, mutable field that `Replace`
  branched on, so a caller could flip it and change which of three things a replacement did to a
  file the review never showed them. It is a method now, derived from two unexported fields, and
  those are back in the token with a test that damages each of them. **A derived answer may stay
  out of a token; what it is derived from may not.**

## What this does NOT establish

That an edited routine presses anything **on a desktop**. Everything above is headless: the host
answers in memory, which settles order, duration and the stop-on-failure chain, and settles nothing
about a real press at a real coordinate in a real window. That is a live trial.

And there is no surface yet. The Studio controls are the next increment; until then nothing a
person can reach calls any of this.

## The cost, stated

An account carries the anchor images a second time — they are `[]byte` on `macroir.Step.Template`,
and `.rec.json` already holds the same PNGs on the click events they came from — so an edited
routine's footprint roughly doubles. The alternative was an amendment list keyed by index and
fingerprint, which refuses exactly when the simplification rules move, which is exactly when
somebody wants a rebuild.

And until `marco edit` joins this, two surfaces can change one routine and only one of them keeps
the explanation.
