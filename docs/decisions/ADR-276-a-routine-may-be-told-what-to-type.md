---
type: adr
status: accepted
date: 2026-10-30
decision: "ACCEPTED — A ROUTINE THAT TYPES SUPPLIED TEXT IS TOLD WHAT TO TYPE AT THE MOMENT OF RUNNING IT, AND REFUSES RATHER THAN TYPING NOTHING. The mechanism was already built end to end — a name declares arguments (`routes.SplitArgs`), a reserved key tapped during the demonstration puts the placeholder where the text goes (`simplify.ArgKey`), `codegen` keeps `{{name}}` in the literal, `routes.ArgNames` lists what a routine accepts, `routes.ApplyArgs` fills it, and `marco do`, `marco hotkey` and the overlay can all supply a value. What was missing was Studio, which had no flag and no field, so a routine whose purpose was to type what somebody supplied TYPED NOTHING and reported `performed`. MISSING IS NOT EMPTY, and the test is whether a value was SUPPLIED rather than whether it has length: an empty string somebody supplied on purpose is a value, because clearing a field is a thing people do and a routine that types nothing where they asked for nothing is correct — `len(v) == 0` refuses exactly the case they meant. A value is used as supplied, not trimmed and not collapsed. THE NAMES ARE CHECKED BOTH WAYS: a value for a name the routine does not take is refused rather than dropped, which is `performLearned`'s existing rule generalised, and a name given twice is refused rather than one of them winning — caught where the flags are still a list, because a map cannot hold the second and nothing afterwards could notice. A VALUE THIS SUBSTITUTION CANNOT WRITE DOWN IS REFUSED: `escapeArg` escapes a backslash and a quotation mark and nothing else, so a value carrying CR or LF produces a literal that does not lex, and the refusal is about the escaping as it stands rather than a claim about what a demonstration can contain — quotation marks, backslashes and text outside ASCII are exercised against the real compiler instead of assumed. VALUES TRAVEL WITH THE IDENTITY AND NEVER AS WORDS, through `--arg=name=value`, because an explicit identity deliberately never has its words read for arguments; the entrance records which values arrived as FLAGS, so one entrance may offer what another may not — a secret-typed argument is refused from a flag, since a phrase carrying `password:value` already reaches the credential store and a window filling in a command line must not become a second way to do that. THE AUTHORISED INVOCATION IS IMMUTABLE: `Resolved.WithValues` COPIES them, and the confirmation's token is recomputed from what the struct now holds rather than compared against a reading stored when it was made — sealing a mutable map by reference would leave the digest and the map moving together, which is the defect ADR-271 records about `Was`. A confirmation binds the route, the digest of the source as it then stood, and those values; a routine saved since, a value changed since, or a value added since all REFUSE. VALUES ARE NEVER STORED, and that covers the diagnostics: not in the route, the recording, a sidecar, a run record, a log line or a refusal's sentence — a diagnostic names the argument and not its text, and the confirmation is the one place a value is repeated because it is on a screen being read by whoever typed it. THE FIELDS SIT BESIDE PLAY, before its confirmation, because they are required inputs to running the routine rather than maintenance; Play is dead until every text argument has been GIVEN a value, the plain button cannot go round the confirmation, no box is offered for a secret, and nothing is prefilled or remembered. 31 of 31 mutations killed. Director's natural-language extraction stays separate and fills the same `Request.Named` against an explicit identity."
supersedes: []
affects:
  - execution
source_paths:
  - internal/routes/args.go
  - internal/orchestrator/authority.go
  - internal/orchestrator/playvalues.go
  - internal/invoke/invoke.go
  - cmd/marco/assistant.go
  - cmd/marco/intake.go
  - cmd/marco/panicstop.go
  - plugins/studio/playvalues.go
  - plugins/studio/routines.go
  - plugins/studio/frontend/index.html
---

# A routine may be told what to type

The trace, the alternatives and the rules this was built to are
[[Text-arguments-proposal-2026-10-30]].

## What was already there, and what was not

All of it, except the way in. A name declares the arguments, a reserved key tapped during the
demonstration puts the placeholder where the text goes, `codegen` keeps it in the literal,
`routes.ArgNames` lists what a routine accepts, `routes.ApplyArgs` fills it at run time, and three
surfaces could supply a value — `marco do` from a phrase, `marco hotkey` from a binding, the
overlay by popping the labels into the command line.

**Studio could not.** `doCommand` sends `do --play=<slug>` and nothing else, `parseDoArgs` had no
flag that filled `Request.Named`, and `performDecidedPlay` deliberately does not read an explicit
identity's words for arguments. So the one surface most people use could not say what to type —
and `ApplyArgs` leaves an unfilled placeholder EMPTY, which is right for the routines that have
none and catastrophic for one whose purpose is to type what somebody supplied: it typed nothing,
and the run reported `performed`.

The product already refused the mirror case. `performLearned` turns away arguments given to a play
that takes none, because *"dropping them silently would let somebody watch the wrong thing happen
and believe they had asked for it."* Dropping the ABSENCE of an argument is that sentence with the
subject changed, and nothing refused it.

## Missing is not empty

The rule the whole slice turns on, and the one a careless implementation gets backwards.

An empty string somebody supplied ON PURPOSE is a value. Clearing a field is a thing people do,
and a routine that types nothing where they asked for nothing is correct. So the test is whether a
value was **supplied** — which a map answers (`v, ok := named[name]`) and a length never can.
`len(v) == 0` refuses exactly the case they meant.

It reaches the page too: a box somebody typed into and cleared counts as given, because the page
records presence on input rather than reading emptiness afterwards.

And a value is used **as supplied**: not trimmed, not collapsed. A leading space in a typed field
is a leading space.

## What is refused, and where

| refused | because |
| --- | --- |
| no value for a text argument | it would type nothing and report `performed` |
| a value for a name the routine does not take | somebody expects text somewhere and there is nowhere to put it |
| the same name twice | a map cannot hold the second, so it is caught where the flags are still a list |
| a value carrying CR or LF | `escapeArg` escapes `\` and `"` and nothing else, so it produces a literal that does not lex |
| a secret-typed argument through a FLAG | a phrase already reaches the credential store; a window filling a command line must not be a second way |

The first four are `routes.CheckArgs`, asked at the door through `take`'s single read of the
source — so every entrance gets the same answer and a refusal is a refusal rather than a failure,
taken before a run reference is claimed because a declined run pressed nothing.

**The newline refusal is about the escaping and says so.** The first draft argued that a
demonstration cannot contain multiline text; nothing traced establishes that, and whether a routine
should be able to type several lines is a separate question with its own answer. What is
established is the lexer error, which is quoted in the proposal. Quotation marks, backslashes and
text outside ASCII are exercised against the real compiler rather than assumed — "two replacements
are enough" being exactly the kind of claim that holds until somebody pastes something.

## The authorised invocation is immutable

`Resolved` carries the values, COPIED. A decision holding the caller's map changes when the caller
does, and a digest taken at authorisation moves with it — the defect
[[ADR-271-a-rescope-is-a-reviewed-move-between-identities]] records about `Was`, where WHAT WAS
DISPLAYED and WHAT CURRENTLY EXISTS are two questions and a stored reading compared against a
mutable field only ever asks one.

So `Invocation.sealed` recomputes the token from whatever the struct now holds, and the
confirmation binds three things: the route, the digest of the source as it then stood, and the
values. A routine saved since, a value changed since, and a value ADDED since all refuse. The name
and value are length-delimited inside the token, because joining them plainly lets a value
containing `=` digest the same as a name containing one — a collision somebody could choose, in
the one field whose whole job is to notice a change.

**And there is a confirmation to bind, which there was not.** `Authorize` asks only about a
composed play; an authored or demonstrated routine is `Allowed` with `ReasonOrdinary` and nothing
is put to anybody. That is right for a routine whose every press was demonstrated, and not enough
for one whose text arrives at the moment of pressing. So a routine WITH arguments gets one, and a
routine with none is unchanged.

## Values are never stored, and that covers the diagnostics

Not in the route, the recording, a sidecar, a run record, a log line or a refusal's sentence. The
leak is easier through a log than through a file: `panicstop.go` already logs `len(s.named)`, a
count, and `oshost.SetArgs` logs the keys — both the shape to follow. A diagnostic names the
ARGUMENT and never its text.

The CONFIRMATION is the exception and is not a diagnostic: it is on a screen, read by the person
who typed the text, and showing it is the whole point.

## The fields sit beside Play

Not below the step panel and the phrases panel. Those are maintenance; these are required inputs
to running the routine, so they sit next to Play and the window it will press in. Play is dead
until every text argument has been given a value, with the reason beside it; **the plain Play
button cannot go round the confirmation** — it refuses and says where to put the text, because a
window offering the fields and leaving the plain button live would be offering two buttons one of
which is wrong; no box is offered for a secret, since the credential store answers for it and a
field here would put it on a command line; and nothing is prefilled or remembered, so choosing
another routine drops the text with the fields.

## Enforced by

- `internal/routes`: `TestARoutineSaysWhichValuesAreTypedAndWhichAreSecret`,
  `TestARoutineNeedingTextIsRefusedWithoutIt`, `TestAnExplicitlyEmptyValueIsAValue`,
  `TestAValueIsUsedExactlyAsSupplied`, `TestAValueForAnUnknownNameIsRefused`,
  `TestMoreValuesThanArgumentsIsRefused`, `TestAPositionalValueSuppliesTheNameItLandsOn`,
  `TestASecretArgumentIsNotRequiredAsText`, `TestAValueSpanningLinesIsRefused`,
  `TestQuotesBackslashesAndUnicodeSurviveSubstitution` (checked by the real compiler),
  `TestASecretCannotBeSuppliedThroughAFlag`.
- `internal/orchestrator`: `TestTheAuthorisedValuesAreACopyOfWhatWasGiven`,
  `TestAnInvocationSaysWhatItWouldType`, `TestAnInvocationWithoutItsValueCannotBeConfirmed`,
  `TestAnExplicitlyEmptyValueIsConfirmableAndSaidInWords`,
  `TestARoutineSavedSinceTheConfirmationRefuses`,
  `TestAValueChangedAfterTheConfirmationRefuses`, `TestAValueAddedAfterTheConfirmationRefuses`,
  `TestAnInvocationNobodyPreparedCannotBeConfirmed`, `TestARoutineThatNeedsNothingIsUnchanged`,
  `TestAValueTheRoutineDoesNotTakeIsNotShownAsSomethingItWillType`,
  `TestTwoDifferentSetsOfValuesDoNotSealAlike`, `TestAValidValueBesideAMisspeltOneRefuses`.
- `cmd/marco`: `TestARoutineNeedingTextIsRefusedAtTheDoor`, `TestARoutineGivenItsTextTypesIt`,
  `TestAnExplicitlyEmptyValueRunsAndTypesNothing`,
  `TestTheSuppliedTextIsNotInAnyDurableRecord` (which walks the whole store rather than naming
  the records it knows about), `TestARefusalNamesTheArgumentAndNotTheValue`,
  `TestASecretGivenThroughTheFlagIsRefusedAtTheDoor`,
  `TestTheEntranceNeverPutsAFlagInThePhrase`, `TestADuplicateOrMalformedValueFlagIsRefused`.
  And the version pin: `TestASaveBetweenTheConfirmationAndTheReadRunsNothing`,
  `TestAConfirmedVersionThatStillMatchesRuns`,
  `TestAnEntranceThatConfirmedNothingPinsNothing`.
- `plugins/studio`, against a real library: `TestThePlayRowSaysWhatTheRoutineNeeds`,
  `TestPlayRefusesARoutineThatNeedsTextAndSaysWhereToPutIt`,
  `TestReviewingAPlaySaysWhatItWouldType`, `TestReviewingWithoutAValueOffersNothingToConfirm`,
  `TestAnEmptyValueIsConfirmableFromTheWindow`,
  `TestOnlyATokenThisWindowPreparedCanBeConfirmed`,
  `TestARoutineSavedSinceTheConfirmationIsRefusedInStudio`,
  `TestTheValuesTravelAsFlagsOnTheRunsOwnCommand`,
  `TestARoutineThatNeedsNothingIsOfferedNoFields`, `TestThePlayFieldsHoldOffWhileRecording`,
  `TestAMisspeltValueIsRefusedByTheWindow`,
  `TestTheWindowHandsTheConfirmedVersionToTheChild`,
  `TestAConfirmedRunIsPinnedToTheVersionItWasSealedWith`.
  Rendered: `TestThePlayValuesPageDoesNotThrow`,
  `TestThePlayRowOffersABoxForTextAndNotForASecret`,
  `TestPlayWaitsForTheTextToBeGivenAndACountsAsGivenWhenCleared`,
  `TestPressingPlayShowsWhatItWouldTypeFirst`, `TestARefusedPlayReviewIsShownAndCannotBeRun`,
  `TestThePageSendsValuesOnceAndConfirmsByToken`,
  `TestThePlayFieldsBelongToTheRoutineTheyAreAbout`.

**31 of 31 mutations killed.** Three survived the first run and each wrote a better test or a
better note: the token's length delimiter (the case first chosen differed as strings anyway — the
pair that collides moves the separator rather than the boundary), the page's field ownership (the
panel being hidden is `drawArgs`' own doing, so the discriminating claim is that the TEXT is
dropped), and `performOnePlay` reading the authorised values rather than the caller's map, which no
test can see swapped because the two are equal by construction at that line — written down in the
code as structural, with the reason it is written that way.

### And a defect the page tests found

Returning to a routine reused its old field DOM, text and all. The rebuild is skipped when the
fields are unchanged, so that a poll does not take the caret out of a box somebody is typing into —
and clearing only the HTML left the signature claiming those fields were still drawn. The result
was a box that looked filled while the value behind it had been dropped: Play dead, and nothing on
screen saying why.

## Amended before closure

Two gaps, both found by reading the source rather than by anything failing, and the gate that
passed in between established neither.

**[P1] The confirmed version stopped at Studio.** `ConfirmPlay` checked the digest and then started
a child that READ THE FILE AGAIN. A save landing between those two moments ran a version nobody
confirmed — with the values they supplied, which is the worst of it, because the text was approved
for other steps. The version now travels (`invoke.Request.Expect`, carried as `--expect=<digest>`)
and the child compares it against the ONE read its snapshot is made from: the same bytes `filled`
is built out of and the same bytes that execute. Asking the file again there would be a second
answer about one file and would reopen the gap. It is the version the invocation was SEALED with,
never a fresh reading — a reading taken at confirmation time would be a reading of whatever is on
disk now, which is the thing being caught. An empty pin pins nothing, which is every entrance that
confirmed nothing, and is unchanged behaviour rather than a hole.

**[P2] Unknown arguments were discarded before validation.** `PrepareInvocation` filtered the
values to the names the routine takes and then asked `CheckArgs` about the FILTERED map, so the
unknown-name arm could never fire from that path: `message` plus a misspelt `mesage` dropped the
second and ran happily with the first. The filter was doing the ignoring before the check could see
it. The names are validated AS SUPPLIED now, and the narrowing to what will be typed happens after
— at which point nothing is being dropped, because everything passed.

**8 of 8 mutations killed** on the two corrections. Two survived first, and both were uncovered
lines rather than wrong ones: the entrance's reading of the new flag (the flag-in-phrase guard did
not pass it, which matters because an unparsed `--expect=` would fall into the words and be
performed — the version pin becoming the thing that removes every check it was added to make), and
the single line in `ConfirmPlay` that decides what a run is pinned to. Nothing could reach that
one: everything after it raises a window and starts a child, which is real input. It is a seam now
(`Acts.playing`, the same shape as `recording`), so the decision is assertable and the desktop half
is not reached — which also closed the larger gap that the confirmed path from token to command had
no test at all.

## What this does NOT establish

That a routine typed the right text **into a real application**. Everything above is a value
reaching a substitution, a substitution reaching the compiler, and a confirmation reaching a
screen that is rendered headless. What the characters do when they arrive is a live trial, and the
one in front of this is still the recording trial that has not happened.
