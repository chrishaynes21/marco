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

## Mutations

Seven mutations, all killed, plus a no-op control that survived. The runs tests and the four
supervised-runner tests above were run against each one.

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
