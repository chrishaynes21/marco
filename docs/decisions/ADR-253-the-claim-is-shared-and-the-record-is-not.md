---
type: adr
status: accepted
date: 2026-10-24
decision: "ACCEPTED — THE DISCIPLINE OF RECORDING AN ATTEMPT IS SHARED AND THE RECORD IN IT IS NOT. `internal/runs` owns the four things two kinds of run genuinely have in common: the PLACE (`internal/marcopaths.Runs`, so one reference retrieves one attempt whoever wrote it and whoever reads it), the CLAIM on a reference made exclusively before anything is sent, the PROVISIONAL RECORD left in that claim at once so an attempt that dies is retrievable as an attempt that did not finish, and the FINALISATION that cannot create — no `O_CREATE`, so a record can only be written into a claim that is already there. It is handed a record it may not look inside: the shape is the caller's own type, supplied for the id it is given, and this package never reads a field of one. Everything above is about one attempt's NAMESPACE and none of it is about what happened. The record is per kind because the vocabularies are: a supervised Act's run accounts for EVIDENCE (`verified`, `contradicted`, `unconfirmed`) and a routine's run accounts for DELIVERY (`performed`, `unverified`, `unavailable`, `clarify`); three of seven words overlap and the rest do not translate. NOTHING CONVERTS IN EITHER DIRECTION — `performed` may never be read as `verified`, because a routine's run looks at nothing afterwards and `verified` is a claim about evidence it never gathered; `unverified` is not `unconfirmed`, since the first says nobody looked and the second says somebody looked and could not tell. Absent verification evidence stays ABSENT rather than becoming three zeroes, which would read as a run that verified nothing. The reader stays the one already there: `plugins/studio/history.go` matches JSON names rather than importing a writer's type, lists a record whose kind it does not know, and shows the whole of it through `Inspect`. `cmd/director`'s `RunRecord` did not move — it became a caller, keeping its own field names, its own words and its own `buildid` stamp."
supersedes: []
affects:
  - execution
  - interface
source_paths:
  - internal/runs/runs.go
  - internal/marcopaths/marcopaths.go
  - cmd/director/runrecord.go
  - plugins/studio/history.go
---

# The claim is shared and the record is not

## What two kinds of run have in common, and what they do not

A supervised Act's run has produced a durable record since
[[ADR-246-a-trial-collects-its-own-evidence]]: one file per attempt, under a reference somebody can
paste into a bug report, written by `cmd/director`. A coordinate routine's run produces nothing.
`marco do` announces `[result] <word>` and the terminal scrollback is the whole account, so the
window says *a routine — no run record* and `Report` is unreachable for exactly the routines the
coordinate MVP is about.

The obvious move is one record for both. It is wrong, and the reason is in what each one *means*.

| | What its answer is about | Its words |
|---|---|---|
| Supervised Act | **evidence** — what the screen said afterwards | `verified`, `unconfirmed`, `contradicted`, `refused`, `cancelled`, `failed`, `started` |
| Coordinate routine | **delivery** — whether every press went out, or which term disagreed | `performed`, `clarify`, `refused`, `unavailable`, `cancelled`, `unverified`, `failed` |

Three words overlap. The rest do not translate, and the dangerous direction is obvious: a routine
presses and looks at nothing afterwards, so `verified` is a claim about evidence it never
gathered. One record shape would have to drop one of those meanings, or carry both and let each
side ignore half — and the half each side ignored would still be sitting in the file for somebody
to read later.

So the vocabularies stay apart, under an explicit kind, and **nothing converts in either
direction**. `unverified` is not `unconfirmed` either: the first says nobody looked, the second
says somebody looked and could not tell, and they send a person to two different places.

## Then what is actually shared

Four things, and every one of them is about an attempt's *namespace* rather than about what
happened in it.

**A place.** One directory, `runs` inside the store, answered by
`internal/marcopaths` — the package that exists because three commands each derived the route
library as a relative path and got three different answers. A reference is worth having only if it
retrieves the attempt it names, and it is written by two binaries and read by a third. Three string
literals spelling one folder is three chances for one of them to be spelled differently, in the one
directory whose whole job is to be found again.

**A claim, made before anything is sent.** The reference is created exclusively — `O_CREATE|O_EXCL`
— and so is this run's own frames directory when pixels were asked for. A candidate that loses
either is abandoned whole and the next one is tried, so two attempts proposing the same id end up
with two namespaces and neither can reach the other's.

**A provisional record, left in the claim at once.** Not an empty file: a real record in the
caller's own words saying the attempt had not finished. An attempt that dies between the first
press and the outcome is then retrievable as what it was. Retrieving *nothing* is
indistinguishable from a run that never happened, which is how three exits that wrote no outcome
stayed hidden for a milestone.

**A finalisation that cannot create.** No `O_CREATE` at save time, which is the whole guard: the
file EXISTING is the proof that this id belongs to this attempt. A record for an id nobody reserved
cannot be written at all, and a record for an id somebody else holds was never reachable because
they hold it.

That list is the package. It is handed a record it may not look inside — the shape is the caller's
own type, supplied as a function of the id it is given — and it never reads a field of one.

## And the executor is what claims the reference

A routine is performed by `marco do`, and that is where the reservation is made: after the
authority door, before the first input, inside the one `performOnePlay` every performance goes
through.

**Not by the surface that pressed Play.** Two front ends run the same routine — Studio spawns
`marco do` and so does the overlay — and a reference minted by whichever one happened to ask would
be a different reference for the same attempt depending on who asked, with none at all for a run
somebody started from a terminal. The thing that sends the input is the only thing present for
every run of it. So the executor claims, prints `runref.Cite`, and a surface READS that line and
reports what it was told.

**After the authority door**, because a run the door declined pressed nothing and never reached
the machine. There is no attempt to account for, and the refusal is said in the decision's own
sentence. A record for it would be a record of a thing that did not happen.

**Before the first input**, because the run this exists for is the one that dies at its first
press. That is held by measurement rather than by the order of printed lines: a host counts the
records in the store on the first action it is asked for, and finds its own already there.

### The outcome does not go on the citation line

`runref.Ended` appends the word in brackets and `runref.Parse` strips it only when the word is one
of the SEVEN SUPERVISED ones. `performed` is not, so a closing citation would print a word the
parser leaves inside the path, and a window would show a record path with `(performed)` on the end
of it. Teaching that parser a routine's words would make one parser answer for two vocabularies —
the conversion this ADR forbids, wearing a parser's clothes.

It is also unnecessary. `[result] <word>` is already the wire line a front end reads for a
routine, from the package that owns those seven words. One opening citation and one result line,
each printed by the thing that owns it.

### What a routine's record identifies

The exact route — slug, application and scope, as three fields rather than one joined string,
because an application's name is a file name and may hold anything a file name may hold. The
digest of the source that ran, the library it was read from, the build of the `marco` that
performed it, the contract, the mode, both timestamps, the outcome word and the run's own
sentence.

**The digest is of the SAVED file, not of the text with arguments substituted into it.** Two
reasons and both matter: a digest of the substituted text cites source that is nowhere on disk, so
nobody can look at what ran — and the substituted text holds whatever the person passed on the
command line, which would make a durable file out of their arguments.

**The contract is read once, by the reader that already existed.** `runRoute` already read it,
because which contract a route states is what decides whether the host's account of what did not
happen stops the run. It now returns that answer (`ran`) instead of the caller asking the file a
second time, so the record cites the same answer the rule acted on.

The first version of this change read it with `strings.Contains(src, "RelTo \"")` — a second answer
to the question, and a weaker one. `orchestrator.ContractOf` already existed, already knew all
THREE contracts of [[ADR-251-a-press-checks-the-window-it-was-recorded-in]], and already had its
list of required terms held against the guard's own requirement by test; its doc comment even
names `cmd/marco` as the other place that asks. So the record carries `legacy`, `guarded` or
`incomplete`, in the project's own words, and `unstated` is kept for the one case the reader
cannot answer: a source nobody could open.

And the rule that acts on it is `legacy` LEAVES, not `anything but guarded` leaves. The second
form looks equivalent and is not: an incomplete route states a window and REFUSES the press,
naming what it lacks, so skipping the account for it would let exactly that refusal pass as a
successful run. Nothing in `cmd/marco` covered an incomplete route, so that mutation survived the
first gate; `TestAnIncompleteRoutinesRefusalStillFailsTheCommand` is what closes it.

**The mode is `unstated`.** Nothing in a routine on disk today says how it was recorded, and
guessing from an environment variable nobody can see afterwards would be a claim rather than a
reading. An explicit coordinate mode is separately scoped work and is what will put a word there.

### Counting input honestly, and a correction

The assessment this work came from says `oshost` "already wraps every send in `delivered`". **It
does not** — four call sites against eleven sending actions — so a count taken there would have
read zero for a routine that typed its password or dragged something.

So the count is taken in `Invoke`, against a closed classification with its false answers written
out, and a test reads the switch and fails for an action nobody has classified. The number is
**asked for, not delivered**, and **actions, not primitives**: one smoothed Move puts out dozens of
small moves and is one action, `Spam` puts out keystrokes until somebody stops it and is one
action, and a press refused by its own guard still asked. `Unperformed` is the other half of that
pair and names which of them did not happen.

Confirmed delivery is still not written anywhere. `SendInput` returning success says the queue
took the press, not that the application acted on it.

**And the count is absent rather than zero when nothing can say.** Zero is a real answer — a run
that stopped at its preparation asked for nothing, and saying so is true. A host that cannot be
asked at all is a different fact, and writing zero for it would turn "nobody knows" into "nothing
happened", in the one field the record has for answering "did it touch my machine".

### And a second command writing records leaked into the owner's store

`cmd/director` has carried a `TestMain` that isolates `$MARCO_HOME` for its whole package since
its own run records leaked out of a suite. Its comment ends: *the next thing to leak that way
might not be a record.* The next thing to leak that way was a record, from `cmd/marco`.

Several tests there drive `runInvocation` end to end with a registry in a `t.TempDir()` and no
`MARCO_HOME`. They were right about everything they were testing and wrong about where they were
standing, so `go test ./cmd/marco/` wrote three run records into the directory the owner keeps
their work in, every run. Found by looking, not by a failing test — and six of them were sitting
in the owner's store by the time it was looked at.

So `cmd/marco` has the same isolation, asserted the same way. And the comparison both guards are
judged by is now one function, `marcopaths.SameStore`, because three things ask "is this the real
store" and **one of them is destructive**: `director reset-test-state` refuses to run against it,
two suites must not write into it, and the library warning must not report one directory as two.
Three implementations is three chances for the destructive one to be the weakest — and the
weakest was already there, in `marcopaths` itself: `Clean` and a case-fold with no `Abs`, so a
relative path naming the real store compared unequal to it.

### A consequence of the executor owning the reference, stated rather than solved

Every `marco do` of a route that is not a learned demonstration now writes a record — which
includes an overlay hotkey, because a binding enters the same intake. A key somebody holds down in
a game is one record per press.

A record is small and `Forget` already removes them one at a time or all at once, so nothing is at
risk. But the directory is unbounded by anything except use, and `History` reads every file in it
to show twenty-five. Retention is a product decision — a sweep, a cap, or nothing — and inventing
one here would be this increment deciding how long somebody's evidence is kept. Parked and named.

## And the reader reads each kind in its own vocabulary

Writing the record was half of it. The other half is what somebody is SHOWN, and that half was
wrong in a way no test noticed, because the reader was written when there was only one kind.

`plugins/studio/history.go` ran every record through `saidOf` and `nextOf` — the SUPERVISED seven
words. Handed a routine's `performed` they fell to their default, so a run was described, and
offered a remedy, in the language of evidence it never gathered: *demonstrate it again, resting
after the click so the result is read.* Not a conversion into `verified`, but the same mistake one
step quieter — an Act's sentence about a run that was not one.

So the record's `kind` is asked, and nowhere else is. Guessing from which fields are filled in
would be this window deciding what kind of run somebody had, and the first record that broke the
guess would be rendered in the wrong vocabulary. `saidOfRoutine` already existed and already said
the load-bearing thing; `nextOfRoutine` is new, because a routine's remedies are about the window
and about recording it again, and `performed` is given NOTHING — there is nothing to do about a run
where every press happened, and a next step would imply something was wrong with it.

**And the word is one constant.** `marco` writes `kind` and Studio reads it, and there is no
compiler between those two: a misspelling on either side is not an error, it is a record silently
read in the wrong vocabulary. `runs.KindRoutine` is imported by both.

**One key per meaning.** The supervised record's `actions_sent` is how many programs reached a
computer; a routine's `input_actions` is how many input actions were asked for. Close enough for a
reader to add up and different enough that the sum would mean nothing, so a routine's row takes
its number off its own key — reading the other one showed zero presses for a run that made four.

### And a row does not say it checked nothing

The history row printed `<n> presses sent · <v> checked` for every record. A routine writes no
verification counts, so `v` was zero and the row read **0 checked** — not a count but the claim
that it checked and found nothing, sitting directly beside an Act that really did check two
things. Three zeroes in the file was the thing the record was careful to avoid; one zero on the
screen says the same thing to the person. A routine's row now says how many input actions it asked
for and that nothing was checked afterwards, and that is held in a browser, because it is the one
place this rule could be broken on screen while every backend test passed.

## Reporting a problem with a routine

`Report` and `Inspect` already worked from a reference, and `Inspect` is kind-agnostic — it returns
the whole record — so the mode, library, build and reason reach the file verbatim as soon as
anything writes them. Two things were left.

`Report` ended with *THE ACT, AS IT STANDS NOW*, found by matching whichever Act this window had
open. A routine has no Act; what it has is its SOURCE, which is the executable truth about it and
the only thing a reader can judge the run against. It is found by the identity in the RECORD, not
by what is selected now: a report written an hour later, about a run from this morning, must
describe the routine that ran.

And the thing a routine's problem report can say that nothing else can: **whether the source on
disk is still the source that ran.** The record carries the digest; the library carries the file.
If they differ the routine has been saved again since that attempt, so the source printed in the
report is not the one that failed — and re-recording is exactly what somebody does between a
failure and reporting it, which makes this the likely case rather than the unlikely one. Without
the warning, a reader compares the run to a program that never ran. A routine that has gone
entirely is said so rather than left out, because a section quietly missing looks like a report
that forgot.

## Why the claim is guarded at the start and not at the end

Because it was guarded at the end, and the guard held while the thing it was protecting was
already lost.

Finalisation used to write with `O_EXCL`, so a second attempt with the same id was refused its
JSON. By then both attempts had been printing the same reference for the length of a run, and the
hover probe had been writing `01-before.png` into the same frames directory. The refusal protected
the record and not the evidence it points at, and the attempt that was refused had already
overwritten the other's images.

The two halves swapped. Exclusive at the claim, and no creation at the finalisation.

## What did not move

**`cmd/director`'s `RunRecord`.** It stays in that command's own package, with its own field names,
its own seven words and its own `buildid` stamp, and it became a *caller*: `reserveRun` supplies
the supervised provisional record, `saveRun` stamps the build and hands the record over. Moving the
whole implementation would have been a refactor nobody asked for, and the field it would have had
to generalise — the three verification counts — is the field that must not be generalised.

**The reader.** `plugins/studio/history.go` already reads records by their JSON *names* rather than
through a writer's type, because `RunRecord` lives in a `main` package and nothing can import it.
That accident is what lets one window list two kinds: a record carrying a field it does not know is
still listed, and `Inspect` shows the whole of it. It keeps that shape on purpose now rather than by
necessity.

**The citation.** `internal/director/runref` keeps the single line one process prints and another
parses. It is a wire format between two binaries and has its own reasons, written down in its own
package.

## Absent evidence stays absent

A routine's record omits the verification counts rather than writing three zeroes. Three zeroes
read as a run that verified nothing, which is a claim; omission is the truth, which is that the
question was never asked. The same rule covers confirmed delivery: `SendInput` returning success
says the queue took the press, not that the application acted on it, so that field is not written
rather than being filled in with the attempted count.

## Enforced by

### R1 — the shared claim

- `internal/runs` `TestRecordsAreKeptInTheStore` — the place is the store's, through
  `marcopaths`, so the reference a person pasted retrieves something in another process.
- `internal/runs` `TestTheStoresPlaceIsAnswerableWithNothingInTheEnvironment` — **added because
  the mutation gate found its absence.** A second derivation reading `$MARCO_HOME` and stopping
  there passed every other test here, because every other test sets it; Studio sets it too, and
  `marco do` typed into a terminal does not. `filepath.Join("", "runs")` is `runs`, a relative
  directory that is a different place for every process depending on where it was started. That
  is the defect `marcopaths` exists because of, and the records are the same hazard with the
  reference in a bug report pointing at nothing. The test reads two strings and writes nothing,
  so it does not touch the store on this machine.
- `internal/runs` `TestAReservationIsRetrievableBeforeAnythingFinishes` — the killed-child case at
  the layer that decides it: the record is there, in the caller's own word for unfinished, and it
  names the reference it is filed under.
- `internal/runs` `TestAReservationNeedsTheRecordItLeaves` — a claim that would leave nothing
  behind is refused rather than claiming the id with an unparseable empty file.
- `internal/runs` `TestTwoAttemptsThatProposeOneReferenceGetTwo` and
  `TestTwoAttemptsInOneSecondAreTwoReferences` — two namespaces from one proposal, through a
  forced collision and through the real proposer; the second is the acceptance check *both
  repeated runs produce distinct references* at the layer that decides it.
- `internal/runs` `TestACandidateThatCannotHoldItsFramesKeepsNothing` — half a namespace is the
  state this prevents, so the losing candidate's record is removed.
- `internal/runs` `TestNothingIsRecordedAgainstAnUnreservedReference` and
  `TestFinalisingOneAttemptDoesNotTouchAnother` — the missing `O_CREATE`, and the failure it
  stands against: the attempt that finished last describing itself in the other's record.
- `internal/runs` `TestAReferenceCarriesTheMomentItWasProposedIn` — a reference a person reads,
  pastes and says aloud.
- The record this test package uses is deliberately **neither real shape**. A test written against
  `RunRecord` would not notice the day this package started reading a field of the record it is
  handed.
- `cmd/director` `TestTwoAttemptsProposingOneIdGetTwoNamespaces`,
  `TestNeitherAttemptCanWriteTheOthersEvidence`,
  `TestARecordCannotBeWrittenWhereNothingWasReserved` and
  `TestAReservationThatCannotClaimThePixelsClaimsNothing` — unchanged, and that is their value
  here: they assert the supervised runner's behaviour through its own wrappers, so the extraction
  is proven not to have changed what that command does.
- `cmd/director` `TestARunRecordKeepsTheFourAnswersApart`,
  `TestARunRecordIsWrittenWhereItsReferenceSaysItIs`,
  `TestAWindowThatCannotBeAcquiredIsRecordedAsFailed` and
  `TestAStartPromptCancellationIsRecordedAsCancelled` — the supervised record's own words and
  fields, which did not move.

### R2 — the record a routine's run leaves

- `cmd/marco` `TestARoutinesRunCitesItselfAndLeavesARecord` — the citation, the record under that
  reference, and every field of it, including that the citation carries NO outcome.
- `cmd/marco` `TestTheReferenceIsClaimedBeforeTheFirstInput` — measured from inside the run. A
  host counts the records in the store on the first action it is asked for and finds its own
  already there, which is the property; the order of printed lines is not.
- `cmd/marco` `TestARoutinesRecordSaysNothingAboutEvidence` — asserted on the KEYS, not their
  values, and on the absence of the word `verified` anywhere in the file.
- `cmd/marco` `TestTheDigestIsOfTheFileAndNotOfWhatWasTypedIntoIt` — a routine with an argument,
  performed with a value passed for it: the digest is the saved file's and the value is nowhere in
  the record.
- `cmd/marco` `TestTheRecordCitesTheSourceThatRan`,
  `TestALegacyRoutinesRecordSaysWhichContractItRanUnder` — the digest and the contract, and that a
  legacy route still reports exactly what it always did.
- `cmd/marco` `TestAnIncompleteRoutinesRefusalStillFailsTheCommand` — **added because the gate
  found its absence.** ADR-251's third contract had no coverage at the command at all, so the
  rule could be rewritten as `anything but guarded leaves` and nothing noticed, although that
  would let an incomplete route's refusal pass as a successful run. The route it uses is missing
  `Dpi` by ABSENCE rather than by an empty value, which is what a route from a build that could
  not read it looks like.
- `cmd/marco` `TestTwoRunsOfOneRoutineAreTwoRecords` — the acceptance check *both repeated runs
  produce distinct references whose reports match their respective outcomes*, at the command that
  produces them.
- `cmd/marco` `TestARunThatNeverFinishedSaysSoAndNotThatItWorked` — the killed-child case: the
  reservation is made and nothing finalises it, and what is left says the attempt began, has no
  `ended`, and nowhere reads as performed.
- `cmd/marco` `TestTheRecordSaysHowManyInputActionsWereAskedFor` and
  `TestAHostThatCannotSayHowMuchItSentLeavesTheFieldOut` — zero is written and nil is omitted.
- `internal/oshost` `TestEveryHostActionSaysWhetherItSendsInput` — reads the `Invoke` switch and
  fails for an action nobody classified, in both directions: an unclassified action, and a
  classification of an action the host no longer offers.
- `internal/oshost` `TestTypingAndDraggingAreAskedFor` — the two the `delivered` count would have
  missed, which is the measurement behind the correction above.
- `internal/oshost` `TestAPressRefusedByItsGuardStillAskedForIt` — asked for and not delivered,
  with `Unperformed` naming it, which is the pair.
- `internal/oshost` `TestLookingAtTheScreenIsNotAskingForInput` — the number answers "did it touch
  my machine", and sampling a colour touches nothing.
- `internal/runs` `TestBothKindsSayUnfinishedInTheSameWord` — `runs.Unfinished` and
  `runref.Started` are one word, because it is a record state rather than either vocabulary's
  outcome.
- `plugins/studio` `TestTheRunsOwnReferenceReachesTheWindow` — the window READS the citation and
  shows it, and the citation does not become the sentence explaining what went wrong. The branch
  order is load-bearing and this is what holds it.
- `cmd/marco` `TestTheseTestsDoNotRunAgainstTheRealStore` and
  `TestARunRecordWouldBeWrittenInsideTheTemporaryStore` — the isolation above, and that the thing
  which leaked actually follows it. Both halves of standing in the real store are asserted: no
  `MARCO_HOME` at all, which is how it leaked, and one pointing AT the default location, which is
  the same directory by a longer route.
- `cmd/director` `TestAResetRefusesTheRealStore` — unchanged, and that is its value here: the
  destructive guard now calls the shared comparison, and this is what says the comparison did not
  get weaker.

### R3 — what somebody is shown, and what they can send

- `plugins/studio` `TestARoutinesRunIsListedInARoutinesWords` — the sentence, the absence of an
  Act's words, that `performed` is given nothing to do, and that the count comes off its own key.
- `plugins/studio` `TestARefusedRoutinesRemedyIsAboutTheWindowAndNotADemonstration` — the run's
  own sentence reaches the person, and the remedy offers none of an Act's machinery.
- `plugins/studio` `TestAnActsRunStillReadsInTheSupervisedWords` — the regression direction, and
  the reason the kind is asked of the record rather than guessed: a supervised record carries no
  kind at all, so the ABSENCE has to keep meaning what it meant.
- `plugins/studio` `TestAnUnfinishedRoutineAttemptSaysSoAndNotThatItWorked` — **added because the
  gate found its absence.** Nothing covered an unfinished ROUTINE record, so its sentence could be
  deleted and the row fell through to "ended as started", which reads as a word somebody
  misspelled rather than as a run nobody can speak for. It is also the acceptance check *a killed
  child leaves an unfinished attempt, never a completed or successful one.*
- `plugins/studio` `TestARoutinesRowDoesNotClaimToHaveCheckedAnything` — in a BROWSER, both rows,
  both directions. The one place this rule could be broken on screen while every backend test
  passed.
- `plugins/studio` `TestReportingAProblemWithARoutineGathersItsSource` — the routine's own source
  in the file, with the build, the contract and the mode, and no section about an Act.
- `plugins/studio` `TestAReportSaysWhenTheRoutineHasBeenSavedAgainSince` — the digest comparison,
  naming both so a reader can tell which is which.
- `plugins/studio` `TestAReportSaysWhenTheRoutineIsNotInTheLibraryAnyMore` — said, not silently
  left out.
- `plugins/studio` `TestReportingAProblemWritesTheRunAndTheActIntoOneFile` and
  `TestTheRunsThisMachineKeptAreListedNewestFirst` — unchanged, and that is their value: the Act
  path reports and lists exactly as it did.

## Mutations

### R1 — seven mutations, all killed, plus a no-op control

The runs tests and the four supervised-runner tests above were run against each one.

- **M1 — the claim becomes non-exclusive** (`O_EXCL` dropped from `Reserve`) → killed ×3:
  `TestTwoAttemptsThatProposeOneReferenceGetTwo`, and `cmd/director`
  `TestTwoAttemptsProposingOneIdGetTwoNamespaces` and
  `TestNeitherAttemptCanWriteTheOthersEvidence`.
- **M2 — the finalisation creates** (`O_CREATE` added to `Finalise`) → killed ×2:
  `TestNothingIsRecordedAgainstAnUnreservedReference` and `cmd/director`
  `TestARecordCannotBeWrittenWhereNothingWasReserved`.
- **M3 — the claim is left as an empty file** → killed ×4, including
  `TestFinalisingOneAttemptDoesNotTouchAnother`: an attempt that dies retrieves a record that
  will not parse, which is a worse answer than either a record or nothing.
- **M4 — the provisional record names a different id** (the next candidate rather than the one
  taken) → killed by `TestAReservationIsRetrievableBeforeAnythingFinishes`. The supervised
  runner's own tests **survive** this one, which is the measurement that justifies the new test:
  nothing before it asked whether a provisional record names the reference it is filed under.
- **M5 — a losing candidate keeps the record it already made** (the `os.Remove` dropped from both
  frames branches) → killed ×2: `TestACandidateThatCannotHoldItsFramesKeepsNothing` and
  `cmd/director` `TestAReservationThatCannotClaimThePixelsClaimsNothing`.
- **M6a — the place is derived here from the environment alone**
  (`filepath.Join(os.Getenv(marcopaths.EnvHome), marcopaths.RunsDir)`) → **survived the first
  gate**, and is why `TestTheStoresPlaceIsAnswerableWithNothingInTheEnvironment` exists. Killed
  by it now. The first attempt at this mutation removed `marcopaths` altogether and did not
  compile; a build failure is not a kill and was redone.
- **M6b — the folder is spelled differently here than in `marcopaths`** → killed ×2:
  `TestRecordsAreKeptInTheStore` and
  `TestTheStoresPlaceIsAnswerableWithNothingInTheEnvironment`.
- **CONTROL — the record is indented with a tab instead of two spaces** → SURVIVED, as intended.
  Nothing here asserts the formatting of a record, and nothing should: the reader parses JSON.
- **Recorded as a shape change and not as a kill**: widening the reference's suffix from four hex
  characters to six fails `TestAReferenceCarriesTheMomentItWasProposedIn` and nothing else. That
  test is about what a person reads, not about the discipline, and the gate above is what covers
  the discipline.


### R2 — nine mutations, all killed, plus a no-op control

- **R2-M1 — the reference is claimed AFTER the run instead of before it** → killed by
  `TestTheReferenceIsClaimedBeforeTheFirstInput`, and by nothing else. Every other test here
  passes a record written at the end, which is exactly why that test is a measurement from inside
  the run rather than an assertion about printed lines.
- **R2-M2 — the outcome is converted to a supervised word** (`verified` written for a routine's
  run) → killed ×3, including `TestARoutinesRecordSaysNothingAboutEvidence`. The rule this ADR
  exists for.
- **R2-M3 — the digest is of the text with arguments substituted in** → killed by
  `TestTheDigestIsOfTheFileAndNotOfWhatWasTypedIntoIt`. It cites source that is nowhere on disk
  AND puts what the person typed into a durable file.
- **R2-M4 — the count becomes an `int`**, so a host nobody can ask reads as a run that touched
  nothing → killed by `TestAHostThatCannotSayHowMuchItSentLeavesTheFieldOut`.
- **R2-M5 — the provisional record says when it ended** → killed by
  `TestARunThatNeverFinishedSaysSoAndNotThatItWorked`. `ended` is how a reader knows.
- **R2-M6 — the record does not say which kind it is** → killed ×2. Without the kind a reader runs
  a routine's word through the supervised vocabulary.
- **R2-M7 — the outcome goes on the citation line** (`runref.Ended` for `Cite`) → killed by
  `TestARoutinesRunCitesItselfAndLeavesARecord`, on the PATH rather than on the outcome: the
  parser leaves an unrecognised word inside the path, which is the defect, so the assertion that
  caught it is the one comparing the cited path to the record's.
- **R2-M8 — the contract written is not the one the run acted on** → killed by
  `TestARoutinesRunCitesItselfAndLeavesARecord`.
- **R2-M9 — every run proposes one fixed reference** → killed by
  `TestTwoRunsOfOneRoutineAreTwoRecords`. Recorded precisely: the second run does not overwrite
  the first, because the claim is exclusive and the retries are exhausted — it gets no reference
  at all, and the test catches that.
- **R2-M10 — the count is only taken for actions that were not refused** → killed by
  `internal/oshost` `TestAPressRefusedByItsGuardStillAskedForIt`.
- **R2-M11 — an action is left unclassified** (`drag` removed from `sendsInput`) → killed ×2:
  `TestEveryHostActionSaysWhetherItSendsInput` and `TestTypingAndDraggingAreAskedFor`.
- **S-M1 — the citation is read after the sentence it is not** (the two branches of
  `readRoutine` swapped) → killed by `plugins/studio` `TestTheRunsOwnReferenceReachesTheWindow`.
- **S-M2 — the reference is read and never shown** → killed by the same test.
- **R2-M12 — the contract rule becomes `anything but guarded leaves`** → killed ONLY by
  `TestAnIncompleteRoutinesRefusalStillFailsTheCommand`, which is why that test exists: it
  survived the first gate because nothing in the package used an incomplete route.
- **R2-M13 — the contract is read a second, weaker way** (`strings.Contains(src, "RelTo \"")` in
  place of `orchestrator.ContractOf`) → killed by the same test, on the CONTRACT it recorded: a
  press stating a window while lacking a term was called `guarded`.
- **L-M1 — the package no longer isolates its store** (the `TestMain` guard made unconditional)
  → killed by `TestTheseTestsDoNotRunAgainstTheRealStore`, which is the point: the leak it closes
  was found by looking at the owner's store, and now a test says it.
- **CONTROL — the record's `version` is bumped to 2** → SURVIVED, as intended. Nothing here reads
  it, and nothing should until there are two versions to tell apart.
- **Unfalsifiable, recorded as such**: reading the contract a second time in `keepRoutineRun`
  instead of carrying it on `ran` is behaviourally identical for every route that does not change
  on disk between the read and the save. It is a structural choice about having one answer, held
  by the comment at its old site rather than by a test, and no test was written to pretend
  otherwise.

### R3 — nine mutations, all killed, plus a no-op control

- **R3-M1 — every record is read in the supervised words again** → killed ×2. The defect itself.
- **R3-M2 — every record is read as a routine** → killed ×2, including
  `TestTheRunsThisMachineKeptAreListedNewestFirst`: a branch can be wrong either way round.
- **R3-M3 — a routine's count comes off `actions_sent`** → killed. Zero presses for a run that
  made four.
- **R3-M4 — the report never gathers a routine** → killed ×3.
- **R3-M5 — the digest is never compared** → killed by
  `TestAReportSaysWhenTheRoutineHasBeenSavedAgainSince`. The report prints source that did not run
  and says nothing about it.
- **R3-M6 — the report describes whichever routine the library lists first** rather than the one
  the record names → killed. A report about this morning's run describing what is selected now.
- **R3-M7 — a run where every press happened is given a remedy** → killed.
- **R3-M8 — an unfinished attempt reads as "ended as started"** → **survived the first gate**, and
  is why `TestAnUnfinishedRoutineAttemptSaysSoAndNotThatItWorked` exists. Killed by it now.
- **R3-M9 — the kind word is misspelled on the reading side** → killed ×2, which is the risk
  `runs.KindRoutine` removes rather than guards: `marco` writes the word and Studio reads it, with
  no compiler between them.
- **S-M3 — every row counts presses and checks**, as it did → killed by the browser test, which
  reported all four of its assertions at once.
- **CONTROL — the report's timestamp format changes** → SURVIVED, as intended.
- **Recorded as a kill and not a control**: rewording the report's section heading fails two
  tests. Those assertions are deliberate — a problem report is a document somebody greps and
  pastes, so its section names are part of what it is — but it is a kill, not a no-op, and is not
  claimed as one.
## Related

- [[ADR-246-a-trial-collects-its-own-evidence]] — the supervised record, and the reference it is
  cited by.
- [[ADR-240-a-stop-is-only-a-stop-at-the-last-moment-it-could-matter]] — where the reference and
  its seven words came from.
- [[ADR-252-a-library-is-a-place-and-a-routine-is-played-where-it-was-made]] — the same argument
  about the library: a place, answered once, absolute.
- [[ADR-251-a-press-checks-the-window-it-was-recorded-in]] — the contract a coordinate routine
  runs under, and what its answer therefore means.
- [[Demonstrations]] — the subsystem note for both paths.
