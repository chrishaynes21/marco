---
type: adr
status: accepted
date: 2026-10-29
decision: "ACCEPTED — CHANGING WHERE A ROUTINE WORKS IS A REVIEWED MOVE BETWEEN TWO IDENTITIES, NOT A SAVE UNDER ANOTHER WORD. A route's identity is its folder — `routes/<app>/context/<slug>.marco`, `routes/<app>/focus/<slug>.marco` and `routes/global/<slug>.marco` are three files — so changing the scope creates a route at one path and ends one at another, with everything that implies: two things that can already be there, two things that can change while somebody reads a review, and a window in which both exist. Until now the only rescope was `Deps.Resave`, which is immediate on purpose because the file it replaces refuses every press: one lock, no consent, no read-back, and a delete after a save it never verifies. Useful machinery, not a move contract. So `PrepareRescope` renders the routine for the scope asked for FROM ITS OWN DEMONSTRATION and writes nothing, and `Rescope` performs the move under both locks. SEVEN RULES, EACH ONE A WAY A ROUTINE COULD OTHERWISE BE LOST. (1) The review is bound by a token to the original's source AND demonstration, the destination's identity AND what was at it, the library, the scope asked for, and the exact source that would be written — so a value assembled from parts of two reviews no longer matches its own content, and a review prepared when the destination was empty is not the same review as one prepared after a routine appeared there, which matters the moment a surface holds pending moves in a map keyed by token. (2) BOTH identities are locked, in an order taken from the identities themselves and never from the caller: two windows moving one routine in opposite directions take the same two locks in opposite orders and each waits for the one the other holds. (3) Both ends are revalidated under those locks — the original's source and demonstration against the review, and the destination against what the consent was given to. (4) A destination collision needs SEPARATE, VERSION-BOUND consent: agreeing to move this routine is not agreeing to lose that one, and the consent names the destination's identity AND digests of the source and demonstration somebody was shown. Without consent the destination is created with `O_CREATE|O_EXCL`, so a routine appearing between the revalidation and the write is the failure rather than something written over. (5) The destination is READ BACK — source and demonstration both — before anything is removed, because a write that returned no error is not a routine that is there and the original is about to be deleted on the strength of it. (6) The original is revalidated once more immediately before removal and is NEVER deleted when it is newer than the version reviewed: both locks have been held throughout, so what this protects against is the writer that does not take them — a hand edit in `marco edit`, a text editor, a sync. AND THE REMOVAL TAKES THE SOURCE FIRST. `Registry.Delete` removes the demonstration and the anchor images before the source, which is the ordinary order for a deletion -- everything is going, though not a safe one even there -- and exactly wrong for a move, which is allowed to fail at the removal and promises the original is RETAINED when it does: a failed source removal through that order left a routine whose recording and images were already gone, one that can no longer be reviewed, rebuilt or moved and with anchors one that can no longer run, while the result said it had been retained. A move uses `Registry.DeleteSource` and then `Registry.DeleteLeftovers`: a routine IS its `.marco`, so removing that is what makes this a move, nothing beside it is touched until that has succeeded, and what is left afterwards is litter rather than a routine -- it cannot fail the move and is NAMED, because a recording left under a slug is a file the next routine saved there would inherit. AND THE OUTCOME IS NAMED HONESTLY. If the destination was written and the original could not be removed, the result says `Copied; original retained:` with the reason, and never `Moved`: there are two routines, and a person told otherwise goes looking for one of them. Selecting the scope a routine already has is refused as nothing to do rather than performed — the destination identity would BE the source identity, which means one lock taken twice, an exclusive create onto the file being moved, and a removal of what had just been written. A move RE-RENDERS rather than editing the source, because the difference between the scopes is in the generated Marco — a focus route carries the `Activate` that brings its application forward and the other two have it stripped — and rewriting generated Marco by hand is how two things that have to agree stop agreeing. The cost is that a hand-edited routine comes back as a rendering of its recording, so the file as it stands is one side of the review and what would be written is the other: approving the move is approving the loss of the edit, in front of them, rather than discovering it afterwards. A routine with no stored demonstration cannot be moved at all. A routine whose presses check the window they were made in cannot be moved to a scope that brings no window forward, which is the same rule the save path applies, asked at preparation so it is a sentence in a review rather than a refusal after somebody decided. A global routine being moved into an application needs one named — its own demonstration is asked first, and a refusal naming what to supply is the answer when that says nothing. AND STUDIO OFFERS IT, in four steps of which three write nothing: the three availability choices with a reason beside any that cannot be taken, a review showing both identities and both sources, Cancel, and Move. The entrance is visible rather than inside the maintenance detail, which is the whole of why nobody found Simplify. All three choices are listed INCLUDING the ones that cannot be chosen, because a chooser showing only what is possible answers the question somebody is actually asking by removing the row that would have carried the answer, and an unavailable row carries no button. And the page cannot author the agreement that costs somebody a routine: `Acts.MoveRoutine` takes the review's token and ONE BOOLEAN, the four digests stay in the window, and `Rescoping.ConsentToReplace` turns the yes into a `Consent` inside the package that recorded them -- a step further than `ReplaceDraft`, which takes digests from the page. Nothing written is a refusal that STAYS on the screen with the review, because looking again needs the panel still there; a move is a note, a copy with the original retained is a problem card, residue is a third line, and the affected identities are read again at once rather than at the next poll. 12 of 12 mutations killed for the contract and 30 of 30 for the surface, two of them standing in instants nothing else can reach: `AfterValidating` between the revalidation and the first write, and `afterDestination` in the one interval a move has that a save does not — the destination written and the original still there. The eighth was found by a surviving mutation: the destination's version was in the token and nothing detected its removal, because the revalidation refuses the same case a moment later. It is load-bearing for the review's IDENTITY rather than for the write, and now has a test that says so. The surface's one survivor was the same shape: deleting the release of a spent preparation changed nothing, because the contract underneath refuses the second press a moment later -- so the test now asserts THIS WINDOW's own sentence rather than that something was refused."
supersedes: []
affects:
  - execution
source_paths:
  - internal/orchestrator/rescope.go
  - internal/orchestrator/keep.go
  - internal/orchestrator/regenerate.go
  - internal/orchestrator/orchestrator.go
  - internal/routes/registry.go
---

# A rescope is a reviewed move between identities

## Where a routine works is not a property of the file

It is the file's **place**. [[ADR-252-a-library-is-a-place-and-a-routine-is-played-where-it-was-made]]
settled that a library is a place; this is the same fact one level down. The three scopes are
three folders:

```
GLOBAL   routes/global/<slug>.marco          app-less; resolves anywhere, switches nothing
CONTEXT  routes/<app>/context/<slug>.marco   only while <app> is in front; switches nothing
FOCUS    routes/<app>/focus/<slug>.marco     from anywhere; brings <app> forward first
```

So "change where this works" is not an edit. It is a **move**, and a move has a shape a save does
not: there is something at the other end that may already exist, and there is a moment when both
ends exist at once. Everything in this decision is about that moment.

## What `Resave` is, and why it is not this

`Deps.Resave` repairs a routine that cannot press anything at all — one saved to switch nothing
whose presses check their own window — and it **rescopes**, to `ScopeAppAnywhere`. It takes one
lock, consents to nothing, deletes the original after a save it never reads back, and is right to:
the file it replaces refuses every press, so there is nothing to weigh up and nothing at the
destination worth protecting.

A rescope of a routine that **works** is the opposite case in every one of those, and reusing the
machinery without the contract would be the kind of borrowing this repository has a name for.

## The seven rules

**1. The review is an identity, and the token is it.** Bound to the original's source and
demonstration, the destination's identity and what was at it, the library, the scope asked for,
and the exact source a move would write. A `Rescoping` that differs in any of those no longer
matches its own token, so nothing can be written that was not displayed — and two reviews that
saw different destinations are two reviews, which matters the moment a surface keys pending moves
by token, as Studio already does for rebuilds.

**2. Both identities are locked, in the identities' own order.** Two moves of one routine in
opposite directions take the same two locks in opposite orders and wait for each other until
`SaveWithin` runs out. `lockOrder` makes that impossible rather than unlikely: whatever the two
moves are, they ask for the same lock first. It is a pure function because the claim is about
BOTH input orders, and a test that ran one move would pass with the caller's order either way.

**3. Both ends are revalidated under those locks.** The review was of a pair of files, and either
can move while somebody reads it.

**4. A collision at the destination needs its own version-bound consent.** Agreeing to move this
routine is not agreeing to lose that one: two routines, two recordings, two decisions. The consent
names the destination's identity and digests of the source and demonstration as they were shown —
the same rule [[ADR-266-one-set-of-rules-for-keeping-a-recording]] makes for a replacement, for
the same reason: an identity does not change when its contents do. Without consent the destination
is created with `O_CREATE|O_EXCL`, so the create is itself the check.

**5. The destination is read back before anything is removed.** Source and demonstration both. A
write that returned no error is not a routine that is there, and the original is about to be
deleted on the strength of this one reading.

**6. The original is revalidated immediately before removal, and a newer version is never
deleted.** Both locks have been held since the first revalidation, so what this protects against
is the writer that does not take them — `marco edit`, a text editor, a sync. Deleting that is
deleting work nobody reviewed.

**7. And the removal takes the SOURCE first.** `Registry.Delete` takes the demonstration and the
anchor images first and the source last. That is the ordinary order for a deletion — everything
is going — though not a safe one even there, since its own last removal can fail with the
dependencies already gone. It is exactly wrong for a move, which is allowed to fail at the
removal and promises the original is **retained** when it does. Through that order a failed source removal left a routine whose recording and images were
already gone: one that can no longer be reviewed, rebuilt or moved, and with anchors one that can
no longer run — while the result said it had been retained.

So a move calls `Registry.DeleteSource` and only then `Registry.DeleteLeftovers`. A routine **is**
its `.marco`: while that is there the routine is there, so removing it is what makes this a move,
and nothing beside it is touched until that has succeeded. What is left afterwards is litter
rather than a routine — it cannot fail the move, and it is **named**, because a recording left
under a slug is a file the next routine saved there would inherit.

And it is named in a sentence of its OWN. `Rescoped.Says` is the outcome and `Rescoped.Left` is
the residue, because they are two things to be told: the move happened, AND something is still
lying at the old place. They were one sentence, and a surface rendering the outcome and a residue
line beside it would have said one of them twice — which is the defect this window keeps
finding in itself.

## And the outcome is named honestly

A destination that was written and an original that could not be removed is **two routines**.
Calling that a move would be this flow reporting a state it is not in, and the person would go
looking for one routine and find two. So the result says

> Copied "type my address" to notepad/context/type-my-address; original retained: …

with the reason, every one of which is something they can act on. `Moved` is reserved for the
outcome where the original is gone.

## The surface, and the one thing it is not allowed to author

Studio's panel for a chosen routine offers the move in four steps, three of which write nothing:
the three availability choices with a reason beside any that cannot be taken, a review showing
both identities and both sources, Cancel, and Move.

**The entrance is visible.** Not inside the `Rebuild and re-record` detail, which is the whole of
why nobody found Simplify: a complete review-and-replace flow, with consent and a refusal when
either file moved, reachable only by somebody who already knew it was there.

**All three choices are listed, including the ones that cannot be chosen.** A chooser showing only
what is possible answers the question somebody is actually asking — *why can this not work from
anywhere?* — by removing the row that would have carried the answer. The reasons are the
backend's own words, and the row that cannot be chosen carries no button: a control that is live
and answers a press with a refusal is a window telling somebody off for pressing what it offered
them. Whether a scope is possible is asked of `WhyNotThisScope` and `rescopeTarget` through
`PrepareRescope`, one choice at a time, so the page has no copy of either rule.

**And the page cannot author the consent.** `Acts.MoveRoutine` takes the review's token and ONE
BOOLEAN. The four digests a move is checked against are the backend's, recorded when it prepared
the review; the page was shown a sentence naming the destination and what is at it, and what it
sends is agreement with what it was shown. `Rescoping.ConsentToReplace` is where that agreement
becomes a `Consent`, inside the package that holds the digests, so no surface can assemble one.

This is a step further than Studio's older `ReplaceDraft`, which takes digests from the page. That
is sound — they are compared against disk at the write — and it still leaves the page as the
author of a value whose whole job is to say what somebody saw.

**Three outcomes, three renderings.** Nothing written is a refusal that stays on the screen with
the review, because the way forward is to look at what is there now and choose once more and that
needs the panel still there to press the button beside. A move is a note. A copy with the original
retained is a PROBLEM card, because two routines exist and somebody has to decide about the one
that was supposed to go. Residue is a third line either way. Then the affected identities are read
again at once rather than at the next poll: the identity the panel was showing does not exist after
a move.

## Two things this deliberately does not do

**It does not edit the source.** The difference between the scopes is in the generated Marco — the
`Activate` a focus route carries and the other two have stripped — so a move is a re-render from
the stored demonstration. Rewriting generated Marco by hand is how two things that have to agree
stop agreeing, which is the same reasoning `Resave` records.

**It does not preserve a hand edit.** It cannot: the rendering comes from the recording, and
inventing source nobody wrote would be worse. What it does instead is show them — the file as it
stands on one side, what would be written on the other — so approving the move is approving the
loss, in front of them, rather than discovering it a week later.

## Amended 2026-10-30

A move's token binds the source it SHOWED as well as a digest of the file it read. `Was` is
exported and mutable, and `fromSourceAt` is a stored reading, so a caller that rewrote `Was` after
preparation left the token matching and the move went ahead having displayed something nobody read
off disk. Rule (1) above now covers both questions. The move's token also carries the routine's
stored step list, which the same slice introduced. See
[[ADR-273-a-routine-writes-down-what-it-does]], and
`TestAMoveReviewIsBoundToTheSourceItShowed`.

## Enforced by

`internal/orchestrator`, through `PrepareRescope` and `Rescope` — the two calls a surface makes —
with the two interleavings driven from seams that exist for exactly these instants.

- `TestAMoveLandsTheRoutineAtItsNewScopeAndEndsTheOld` — the successful control, and it earns its
  place: every other test here asserts that something did NOT happen, and a contract that refuses
  everything passes all of them. The destination holds the source for the scope asked for (no
  `Activate`), its demonstration came with it, and the original's source and recording are both
  gone.
- `TestMovingARoutineToTheScopeItAlreadyHasIsNothingToDo` — refused before anything is read, and
  the library is byte-identical afterwards.
- `TestAMoveOntoAnOccupiedIdentityNeedsItsOwnConsent` and
  `TestConsentToReplaceTheDestinationIsBoundToTheVersionShown` — no consent, a consent naming
  another routine, and a consent to a version that has since been saved again; each refused with
  both ends untouched. `TestAMoveReplacesTheDestinationWhenTheConsentMatchesIt` is their control.
- `TestARoutineThatAppearsAtTheDestinationMidMoveIsNotWrittenOver` — a routine created inside the
  lock, through `AfterValidating`, after the revalidation said the path was free. The exclusive
  create refuses it and what the other writer saved is still there.
- `TestAMoveIsRefusedWhenTheOriginalChangedAfterItWasReviewed` — its source edited, its recording
  replaced, and the routine deleted; each refused with nothing written at the destination.
- `TestAMoveNeverDeletesAnOriginalThatChangedAndSaysItCopied` — an edit landing in the one interval
  where both routines exist, through `afterDestination`. The edit survives, the result says
  `Copied` and `original retained`, and **both locks are observed to be held at that instant** —
  a move that locked only one end passes every other test in the file.
- `TestAMoveReadsTheDestinationBackBeforeRemovingTheOriginal` — the destination's source, and then
  its demonstration, replaced in that same interval; the original stays and the result is a copy.
- `TestAMoveThatCannotRemoveTheOriginalLeavesItWhole` — the removal made to fail through
  `removeSource`, a seam rather than a locked file because the real causes are a different answer
  on every platform and what is held is the ORDER. The destination is complete (source and
  demonstration), the original's source, recording **and anchor image** are byte-identical, and
  the result says copied. Reverting to `Registry.Delete` fails it on all three.
  `TestAMoveThatRemovesTheOriginalTakesWhatSatBesideIt` is its control — the ordinary move still
  ends the original and leaves nothing under the old slug, so "never remove anything" is not a
  way to pass.
- `internal/routes` `TestAMovesRemovalTakesTheSourceFirstAndNamesWhatItLeaves` — the two halves
  at their own level: `DeleteSource` leaves the demonstration and the images where they are and
  is idempotent, `DeleteLeftovers` takes them, and something it cannot take is NAMED rather than
  swallowed. The failure is a directory in the recording's place, which `os.Remove` refuses on
  every platform with no permissions, no open handle and no second process.
- `TestBothEndsOfAMoveAreLockedInTheSameOrderWhicheverWayItGoes` — `lockOrder` read from both
  input orders, over all three pairs of scopes.
- `TestAGuardedRoutineCannotBeMovedWhereItsPressesWouldRefuse` — refused at preparation for both
  scopes that bring no window forward, and a refused review cannot be acted on.
- `TestAMoveOfferedWithAnAlteredReviewIsRefused` — the source it would write, where it would go,
  and the library it was prepared in, each altered after preparation.
- `TestTwoMovesThatSawDifferentDestinationsAreNotTheSameReview` — the test the eighth mutation
  asked for: the destination's version is load-bearing for the review's identity even though the
  revalidation refuses the same case a moment later.
- `TestARoutineWithNoDemonstrationCannotBeMoved`, `TestAHandEditIsShownBeforeAMoveReplacesIt` and
  `TestMovingAGlobalRoutineIntoAnApplicationNeedsOneNamed` — the three refusals a person meets
  before any of the above matters.
- `TestAMoveNamesWhatItCouldNotSweepAndStillMoves` — a move with litter beside the original it
  cannot sweep: the move still happens, the residue is named, and the outcome sentence is NOT also
  carrying it. The failure is a non-empty directory where an anchor image goes, which `os.Remove`
  refuses everywhere — an anchor rather than the demonstration on purpose, because the
  revalidation immediately before the removal reads the demonstration and would report a copy.

And the surface, in `plugins/studio`. Both halves, because neither substitutes for the other: a
greyed-out button proves nothing about a backend that would have obliged, and a refusing backend
proves nothing about a button that offers it.

- `rescopeacts_test.go` against a real library: `TestEveryScopeIsAnsweredForAChosenRoutine`
  (all three, the current one marked and not offered, and where-it-works and where-it-can-be-asked
  -for as two sentences), `TestAGuardedRoutineIsRefusedTheScopesThatBringNoWindow` and
  `TestARoutineWithNoDemonstrationIsOfferedNoMove` (unavailable WITH the reason),
  `TestAPreparedMoveShowsBothEndsAndWritesNothing`,
  `TestAReviewedMoveLandsAtTheNewIdentityAndSaysSo` (the control, and the preparation is spent —
  asserted on THIS window's own sentence, because a kept review is refused a moment later by the
  contract underneath and "something was refused" passes either way),
  `TestAMoveWithATokenThisWindowNeverPreparedIsRefused`,
  `TestAMoveOntoAnOccupiedIdentityNeedsItsOwnYes`,
  `TestAYesToReplaceDoesNotCoverARoutineSavedSince` — the whole reason the call takes a boolean
  — and `TestAMoveOfARoutineSavedAgainSinceTheReviewIsRefused`.
- `rescopepage_test.go`, the page RENDERED in headless Chromium against a stubbed backend, because
  a source guard passes as long as a field is named somewhere in the file:
  `TestThePageOffersEveryScopeAndSaysWhyOneIsUnavailable`,
  `TestThePageShowsBothEndsOfAMoveBeforeAnythingIsWritten` (and that Cancel writes nothing and
  lets go by token), `TestThePageWillNotMoveOntoAnOccupiedIdentityWithoutAgreement`,
  `TestThePageMovesWithTheReviewsOwnTokenAndNothingElse` (what the page submitted, recorded call by
  call: two arguments, the review's token, and the agreement following the tick box),
  `TestARefusedMoveKeepsTheReviewAndSaysWhat`,
  `TestThePageRendersMovedCopiedAndResidueDistinctly`, and
  `TestThePageReadsTheMovedIdentityBackAndSelectsIt`.
