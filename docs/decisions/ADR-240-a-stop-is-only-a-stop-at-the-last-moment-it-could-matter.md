---
type: adr
status: accepted
date: 2026-10-19
decision: "ACCEPTED — twelve conditions found by four rounds of review of the supervised-run lifecycle are closed, under three rules. (1) A STOP IS ONLY A STOP AT THE LAST MOMENT IT COULD MATTER: the stop key is read once more immediately before input is sent, it is bridged onto the executor's own context, it is honoured before a runner exists, and a consent question may not consume it. (2) AUTHORISATION IS NOT EVIDENCE: the look taken after a person authorises reports what the STABILISER made of it, because the stabiliser damps a change for three looks and keeps answering with the standing screen while it settles — which is exactly the window an authorisation arrives in. An unsettled look, an unreadable one, and a settled screen with nothing under the approved point are each a refusal, and the standing claim is read off the fresh look so the order cannot be got wrong. On a screen whose identity words have been separating, a look that agrees by ARRANGEMENT ALONE is not enough either: the reading of that capture is asked for by frame and waited for inside a bounded, cancellable budget, because the reading of the frame before it is admissible while the epoch is still and agrees with the screen the plan was made on — and a request that came back having read nothing is not that evidence either, because a reading below the stabiliser's own bar for a claim could not have refused the merge however different what it said was. (3) A RECORD IS THE ATTEMPT IT NAMES: the whole evidence namespace — the record and the frames directory — is RESERVED before the reference is printed and retried on collision, `saveRun` may only write where a reservation already is, every exit past the citation finalises, and the one-word outcome is read from how far the workflow got as well as from each action's verdict."
supersedes: []
affects:
  - execution
  - safety
source_paths:
  - cmd/director/performworkflow.go
  - cmd/director/readingeyes.go
  - cmd/director/consentdesk.go
  - cmd/director/runrecord.go
  - internal/director/workflowrun/workflowrun.go
  - internal/director/workflowrun/verify.go
---

# ADR-240 — a stop is only a stop at the last moment it could matter

## The gap

The supervised-run lifecycle landed in `cec983d`, `7ae9dd1` and `22fd169`: an explicit Start, a
countdown with no input during it, a visible Stop, a readable final result, and a run reference
tied to the evidence. All of it was tested and all of the tests passed.

A review sweep of the same code then traced seven conditions none of those tests described. Six
of them are not about whether a feature works — they are about **when** it works, and the
answer in each case was "not at the moment that matters":

1. **Screen validation used stale evidence after authorisation.** The grounder measured the
   target against the look that had produced the consent question. A person takes a second or
   two to press F7; the check answered for the screen as it was *before* they did.
2. **F9 could arrive after the final stop check and still permit a click.** The stop flag was
   read, then the executor was called, and between those two statements nothing was watching.
3. **F9 during startup could dereference a nil runner.** The stop path reached for a runner the
   run had not constructed yet.
4. **A partially completed run could be recorded as `verified`.** The classifier returned
   `verified` on any positive verdict count, so a workflow whose first step verified and whose
   second refused was, as a whole, a success.
5. **Opening a consent question could discard F9.** The desk drained pending keys before posing
   its question and threw away whatever it found, including a stop.
6. **Partial positive evidence bypassed the verification deadline.** A window holding one
   qualifying reading waited for more, unconditionally — so `--verify-for` existed and could
   not expire.
7. **Evidence could be overwritten or attributed to a different run.** Two attempts in one
   second shared an id; `os.WriteFile` truncated; `--frames` wrote to one directory for every
   run.

A second round of review on the fixes for those seven found three more, and all three are the
same first fix not having gone far enough:

8. **The fresh capture could still approve a changed interface.** The look reached the observer
   and the observer, correctly, kept answering with the standing screen while the change
   settled. So the two confirmations before a click could both see the old screen and the old
   epoch. And the look carried geometry without text, so a panel of the same shape that read
   differently went on matching.
9. **Three exits past the citation still wrote no record.** The delay cancellation for an
   explicitly named application, and each of the three optional sources that can be asked for
   and be absent. The first round had fixed the exits it had enumerated.
10. **Reservation protected the JSON and not the evidence namespace.** `O_EXCL` at save time
    refuses the second writer *after* both have been printing the same reference and writing
    into the same frames directory. And the test for it demanded 64 distinct draws from 65,536,
    which is a 3% chance of failing a correct implementation while testing the wrong property.

A third round found one more, and it is the second round's own fix one layer down:

11. **The boundary look's words were the previous frame's words.** Text was put on the boundary
    look the way the loop puts it on an ordinary one: `offer` the capture, `ready()` in the same
    breath, take whatever OCR has FINISHED. For a loop that looks ten times a second that is
    right. At a boundary it means the look carried a reading of the frame BEFORE it — and while
    the stabiliser damps a change the epoch does not move, so that reading is admissible, agrees
    with the standing screen, and confirms the screen the plan was made on. A panel swapped for
    one of the same shape then had nothing left to refuse it: the arrangement agreed, the point
    was still inside something, and the words were the words of the panel that had gone.

And a fourth round found the edge of that fix:

12. **A request that completed having read nothing satisfied the requirement.** `VisualScreens.Read`
    counts every reading it is handed, including one whose source answered and found no words —
    correctly, because "read and silent" and "never answered" are different facts about a screen.
    The boundary's term was reading that counter as "the words arrived", so OCR returning
    successfully with nothing on it cleared the press: empty text cannot contradict an identity,
    so the same-shape change went through on the strength of a request having returned. A reading
    too thin to be a claim did the same thing.

## The decision

Three rules, each of which generalises past the condition that found it.

### A stop is only a stop at the last moment it could matter

A stop that is read early and acted on late is not a stop, it is a prediction. So the flag is
read again immediately before `exec.Do`, and — because a single executor call can itself take
long enough for a person to change their mind — `stopping(ctx)` bridges the flag onto the
context the executor already watches. `stopper atomic.Pointer[workflowrun.Runner]` lets the key
handler record a stop before any runner exists and have the run honour it when one does. And
the consent desk's drain loop now inspects what it drains: a `ConsentStop` found there calls
the stop, instead of being counted and dropped.

### Authorisation is not evidence

A person's approval is permission to act on the world as it is *now*. `freshLook` takes a real
look, fed through the same observer, so the inputs `worldMoved` reasons about describe the
present rather than the moment the question was asked.

**Taking the look was not enough, and a second round of review found why.** The stabiliser
damps change on purpose — a look that differs from the standing screen is counted `Unsettled`
and the STANDING screen is returned, for `VisualSettleLooks` looks. ADR-170 measured the 150 raw
adjacent-frame changes that suppresses, and it is correct. But at ten looks a second that window
is three looks, and a person's F7 lands inside it: both confirmations before an ordinary click
could take a fresh look at a changed interface, read back the old screen and the old epoch, and
clear the check. Every term above the fresh ones answers from the standing claim, which is the
claim the plan was made from.

So the fresh look reports what the stabiliser made of IT. `Stability()` before and after says
whether the look was counted as a look at the standing screen or as something else, and the
name of the counter that moved goes into the refusal. Three refusals rather than one, because
"no evidence" and "evidence that disagrees" send a person to different places:

- **No look could be taken.** `freshEvidence` zero. Not a pass.
- **The look did not agree.** Unsettled, warming, unobserved, a replaced window, a promotion.
- **Nothing was proposed under the approved point.** A different question from screen identity:
  an interface can settle clean and have nothing under the coordinate — a list that scrolled, a
  row that went away, a panel swapped for one of the same overall shape. Asked of the fresh
  look's own proposals, which is the same evidence a control is found by.

**And the words have to be the words of THIS capture, which took a third round to get right.**
The first version of this put text on the boundary look the way the loop puts it on an ordinary
one — offer, then drain whatever has finished — and that is the defect with the capture added.
OCR costs hundreds of milliseconds against a cadence of a hundred, so what `ready()` returns
describes an earlier frame; the epoch does not move while a change is settling, so the earlier
frame's reading is admissible by the rule that exists to refuse stale evidence; and it agrees
with the standing screen, because it is a reading OF the standing screen. The one change the
geometric terms cannot see was being confirmed by the panel it replaced.

So the boundary makes a different request from the loop's: this frame, by id, waited for inside
`boundaryTextWait`, cancellable, and refused when it does not arrive. Nothing drains the loop's
queue. And it is asked **only where it could decide anything** — a screen with no corroborated
vocabulary cannot be contradicted by a reading, so there a look agreeing on pixels is the whole
of what the grounding policy has ever had, and demanding words would refuse presses for evidence
the stabiliser could not have used.

**And a request that returned is not evidence that arrived**, which is the same mistake in
miniature and took a fourth round. `Read` counts a reading whose source answered and found
nothing, because "read and silent" is a fact about the screen and "never answered" is not — and
the boundary was reading that counter as the words having arrived. So a successful reading of
nothing cleared the press, and so did a reading of one or two words. The term now requires the
reading to say enough to be a claim, at the stabiliser's own bar: below
`observe.TextDisagreementWords`, `TextsDisagree` declines to call either side a claim, so a
reading that thin could not have refused this merge however different what it said was. The
stabiliser is still given the reading either way; what it may not do is stand in for evidence
nothing was ever going to consult.

Nothing new is measured and no threshold is invented: reporting a counter is not a second opinion
about screen identity, the bar is the one the text rule already applies, and the rule that rules
on the reading is the stabiliser's own.

**And the standing claim comes back ON the evidence.** The look is what moves it, so reading it
before looking compares the plan against the state the plan was made in — the original defect
restated. A mutation that swapped those two statements survived every test in the package, which
is what two source-pinned strings are worth against an ordering. There is now exactly one place
either value is read, inside `freshLook` immediately after the observation, and `stillThere` no
longer holds the observer at all.

This costs one capture per authorised action, and buys the only guarantee that mattered: a click
lands on the control a person said yes to.

### A record is the attempt it names

A reference whose single job is retrieval must retrieve, and **the record is not the only thing
a reference points at**. The first version guarded only the JSON, with `O_EXCL` at save time —
so two attempts that proposed one id had both been printing the same reference for the length of
a run and the hover probe had been writing `01-before.png` into the same frames directory before
the refusal arrived. The guard protected the record and not the evidence.

So the whole namespace is **reserved** before anything is written into it and before the
reference is printed: a provisional record under `runs/`, created exclusively, and this run's own
frames directory, created with a bare `Mkdir` so an existing one is a refusal rather than a
silent share. A candidate that loses either is abandoned whole and the next is tried. `saveRun`
then opens **without** `O_CREATE`, so the file existing is the proof that this attempt holds this
id, and a record for an id nobody reserved has nowhere to go.

The provisional record says `started`, which is what makes an unfinalised exit findable: before
it, an exit that wrote nothing left the reference pointing at a file that did not exist, and that
reads as a citation bug rather than as a path that forgot to finalise. **Every exit past the
citation now finalises**, including the three usage refusals that exit 2 — `endedEarly` takes the
exit code, because folding them into exit 1 to get a record would change what the code means. A
test walks the function and requires it of every return, which is the half a newly added exit
cannot be written around.

And the one-word outcome is a property of the run, not of its best action:
`rep.Progress.Complete()` gates `verified`, `rep.Pressed > decided` catches an action nobody ever
answered for, and a verification window with partial positive evidence expires `unconfirmed` at
its deadline like any other.

## What this does not do

Two limitations were traced in the same sweep and are **not** closed here, because both are
larger than a lifecycle fix:

- **Control-centre Stop** (`cmd/marco/watchui.go:426`) reaches service cancellation without the
  standalone-play cancellation broadcast, so Stop there does not reach a standalone play.
- **Inter-step verification** (`internal/director/workflowrun/workflowrun.go`) can advance a
  different-screen step on destination arrival while a previous step's outcome verification is
  still pending.

Both are named so the next session does not rediscover them.

## Enforced by

Freshness at the boundary:

- `cmd/director/freshconfirm_test.go` — `TestASettlingChangeIsRefusedOnEveryLookOfItsSettleRun`:
  the real observer, settled on one panel and then shown another, refuses on every look of the
  damped window, and the test asserts the standing claim really did not move.
- `TestAPanelThatReadsDifferentlyIsRefusedThoughItsShapeAgrees`: same geometry, conflicting text,
  refused by the stabiliser's own text rule.
- `cmd/director/boundarytext_test.go` — the production fold over a reader whose engine this test
  starts and stops, which is the only way to describe a boundary that clicked before its answer
  arrived. `TestAPanelWhoseWordsHaveNotArrivedIsNotPressed`: identical arrangement, a finished
  reading of the loop's last frame waiting in the queue the boundary used to drain, and no answer
  for the capture — refused, and the stale reading is still sitting there.
  `TestAStepIsNotPressedWhileItsOwnWordsAreUnavailable`: the same, through the real runner, zero
  host calls. `TestWordsThatArriveLateStillRefuseTheSameShape`: the withheld answer released,
  refused by the text rule. `TestAnUnchangedPanelWhoseWordsArriveIsPressed`: the positive
  control, one move and one click, and the two boundary readings one press costs.
  `TestAScreenNobodyHasReadIsStillPressedOnItsPixels`: the limit of the rule — an unread screen
  is pressed on its pixels and no reading is asked for at all.
  `TestABoundaryWaitingForWordsIsCancellable`: a cancellation ends the wait.
- `TestASuccessfulReadingOfNothingIsNotEvidence` and `TestAReadingTooThinToBeAClaimIsNotEvidence`:
  a request that completed and read nothing, and one that read fewer words than it takes to be a
  claim — refused, with the first asserting that the stabiliser's counter DID move, which is the
  trap. `TestAStepIsNotPressedOnAnEmptyOrThinReading` carries both through the real runner for
  zero host calls. The bar is load-bearing in both directions: the positive control clears on
  exactly `observe.TextDisagreementWords` words.
- `TestAPointWithNothingUnderItIsRefusedOnASettledScreen`: a settled screen with nothing under
  the approved coordinate.
- `TestAChangedPanelUnderAnAuthorisedStepSendsNoInput`: the real runner, with that boundary
  installed — zero host calls.
- `TestABoundaryWithNoFreshEvidenceRefuses`, `TestFreshOfNamesWhatTheStabiliserCountedThisLookAs`,
  `TestHoldsPointIsBoundedOnEveryEdge`: no look is not a pass; one case per stabiliser counter;
  one case per edge of the region.

Stop:

- `cmd/director/consentstop_test.go` — `TestAStopQueuedAsAQuestionOpensIsNotDiscarded`.
- `internal/director/workflowrun/laststop_test.go` — `TestAStopFromTheHoverProbeStopsTheClick`
  and `TestAStopFromTheWorldCheckStopsTheClick`: a stop arriving after the last check sends no
  input.
- `cmd/director/runlifecycle_test.go` — `TestAStartPromptCancellationIsRecordedAsCancelled`:
  a stop before a runner exists.

Classification and the deadline:

- `cmd/director/runrecord_test.go` — `TestARunRecordKeepsTheFourAnswersApart`: an incomplete run
  with one verified action is not `verified`; a pressed action with no verdict is `unconfirmed`.
- `internal/director/workflowrun/deadlinehold_test.go` —
  `TestOneQualifyingReadingDoesNotWaitForever`: one qualifying reading and then silence expires
  `unconfirmed` with no cursor advance.

Evidence and the exits:

- `cmd/director/runevidence_test.go` — `TestTwoAttemptsProposingOneIdGetTwoNamespaces`,
  `TestNeitherAttemptCanWriteTheOthersEvidence`,
  `TestARecordCannotBeWrittenWhereNothingWasReserved`,
  `TestAReservationThatCannotClaimThePixelsClaimsNothing`: every one of them forces a collision
  rather than hoping for none.
- `cmd/director/runexits_test.go` — `TestEveryExitPastTheReferenceIsFinalised` walks the
  function and requires every exit past the citation to finalise;
  `TestAnAbsentOptionalSourceIsRecordedAsAStartupFailure` and
  `TestRepairWithoutOCRIsRecordedAsAStartupFailure` drive two of them through the production
  command with the source forced absent, and the second also asserts the frames directory the
  record points at;
  `TestAReservationThatIsNeverFinalisedIsVisibleAsSuch` keeps `started` legible as a defect.
- `cmd/director/runlifecycle_test.go` — `TestAWindowThatCannotBeAcquiredIsRecordedAsFailed`.
- `cmd/director/performworkflowwiring_test.go` — `TestPerformWorkflowInstallsItsGates` holds what
  no behavioural test in the package can reach: that the standing claim is read off the fresh
  look in one statement after the observation; that the boundary is given a reader and asks it
  for the words of its own frame, with the production budget; and that the capture carries an id
  the reading is compared against, which matters on the day somebody routes this back through
  the shared queue.

## Links

- [[ADR-235-three-records-and-only-one-of-them-may-act]]
- [[ADR-238-a-verdict-needs-both-sides-and-a-window-of-its-own]]
- [[ADR-239-running-what-you-demonstrated-by-the-name-you-gave-it]]
- [[Experiment-093-seven-conditions-a-green-suite-did-not-describe]]
