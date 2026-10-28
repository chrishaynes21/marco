---
type: adr
status: accepted
date: 2026-10-27
decision: "ACCEPTED — A ROUTINE STATES WHICH GENERATION OF GENERATOR RULES WROTE IT, AND A REBUILT VERSION IS REVIEWED BEFORE IT REPLACES ANYTHING. FOUR THINGS THE REVIEW FOUND, ALL FIXED: a replacement identified the ROUTINE rather than the review on the screen, so two preparations of one routine overwrote each other and an asynchronous page could display A while the window held B — a preparation now carries an immutable token derived from its own content and a replacement must present it, with `Replace` also checking a `Prepared` against its own token; the IMAGES ARE NOT ADDITIVE, since a rebuild produces the same `<base>-anchor-N.png` paths and overwriting them before a source write that then fails leaves the OLD source pointing at different pictures, so AND THOSE TWO WERE REOPENED AND CLOSED PROPERLY: the token digested only the CONTENT, so two routines with identical sources and identical demonstrations — the same thing saved in-app for two applications, which really is byte-for-byte identical since an in-app routine names no application — shared one, and a window holding one and handed the other's token would write the right bytes to the wrong file; the route's whole identity and the library are in the digest now, with a `Deps` pointed at another library refused on its own; and the image rule decided that nothing conflicted and then wrote them ALL, where a path already holding the right picture is now skipped rather than rewritten and a path that cannot be READ is a refusal rather than an empty space — treating unreadable as "nothing there" being the one reading that licenses writing over it — with only the missing ones written and every path decided before any is; AND CLASSIFYING IS NOT WRITING: the missing ones went through a create-or-truncate write, so another save creating one of those reusable paths in the gap would have had it truncated on the strength of a reading taken beforehand — they are created EXCLUSIVELY now, which closes the gap rather than narrowing it since the test and the write are one operation, with `WriteAssets` keeping create-or-truncate because a save owns the paths it names; and the MOMENT is tested rather than the mechanism, through an unexported seam that occupies the instant between the two steps and does what the other save would have; the unknown-generation restriction CHECKED AN EARLIER READ, so a file replaced between the listing and the preparation bypassed it, and eligibility is now read off the source `Prepare` actually read; and `current` PROMISED "it will be the same routine", which the stamp cannot support — a hand-edited routine keeps its stamp and rebuilds differently — so the sentence offers the comparison as a question and only `Prepared.Changes()` answers it. A routine keeps the shape it was generated with for ever, because nothing rewrites saved Marco — the file is the only executable truth about a routine and rewriting generated source by hand is how two things that have to agree stop agreeing. So every rule about what a generated routine DOES reaches a routine when that routine is written, and a library accumulates files written under rules that no longer match: a routine saved before [[ADR-261-typed-text-is-a-sequence-of-submissions]] goes on acting past text that never arrived and reports a run that performed. That was INVISIBLE, because nothing on disk said which rules wrote a file — and it cannot be inferred, since the shapes in a routine are a function of the demonstration as much as of the generator, which is exactly how a guarded drag read as `legacy` before `draggedTerms` existed. So the generator STAMPS it, as a comment, UNCONDITIONALLY: whatever called it, this generator wrote the file, and a condition there would be a file written by it that does not say so. NO CORE CHANGE — a comment is not a word of the language, which is the same ground `// Recorded as:` stands on. IT IS PROVENANCE AND NOT A VERDICT: it says which rules wrote the source, not that the source is guarded, unmodified or still correct — a current-generation routine can carry legacy behaviour on purpose, because a demonstration with no measured press generates exactly the unchecked route it always did, so what the presses promise stays `ContractOf`'s separate question. FOUR READINGS, NONE COLLAPSING INTO ANOTHER: current, older-and-recognised, unstated, and unknown. Unstated says ONLY that — not 'written before versioning', which is a claim a handwritten file or one somebody deleted the line from does not support. Unknown is a later build's generation, a malformed value, or the same question answered twice — not current, which would be this build claiming to have written a file it cannot read, and not unstated, which would lose the one fact the file does state. Nothing is offered for an unknown generation, because a reader that cannot say what a generation does cannot say what replacing it would change; it stays inspectable and playable under exactly the rules it always had, held against a control run of the same routine unstamped. GENERATION 1 IS WHAT THIS GENERATOR DOES, with nothing behind it: numbering the rule changes that landed before today 1..5 would place routines in a history they never stated, which is the same inference with a version number on it. THE HISTORY IS ONE LIST, so the current generation is its last entry rather than a constant beside it — `everyMeasuredTerm`'s lesson one layer up, where a generation added to a counter and not to the words would be read, found older, and never explainable — and the difference walk is exercised against SYNTHETIC future generations, because a mechanism with no live input is one nobody can tell is wired. OLDER IS NOT AFFECTED: a difference is a fact about two generators, and whether it changes anything for one routine is a fact about that routine, so a claim that a routine needs replacing is made only where its own contents bear on the rule, and otherwise a comparison is offered. THE UPDATE FLOW IS FOUR STEPS AND THREE OF THEM WRITE NOTHING: select the exact routine, prepare a version from its own stored demonstration, review both sources in full plus any refusal, then explicitly replace or cancel. `Resave` is NOT that flow and stays what it is — it repairs a routine that cannot press anything at all, so it is immediate on purpose, and it RESCOPES through the save path that chooses where a routine lives. Neither is allowed here: the scope is read off where the routine already lives, and a replacement writes the file back exactly where and as it was. A replacement re-reads both files it was built from and refuses if either moved, naming which — which is also the whole of the protection for a HAND-EDITED routine, since the edit is the left-hand pane and replacing it is then a decision. On any refusal or failed write the routine is as it was. A REBUILD TODAY'S RULES WOULD REFUSE IS A REVIEWABLE ANSWER, not an error: a recording with a press found by matching a picture was saved happily by an older build and produces no routine now. The run record carries what the file SAYS and this build's READING of it, off the same snapshot that executes — an older executor that wrote down its verdict and dropped the declared value would contradict the file it cites. ONE DESKTOP SCENARIO RAN GREEN on 2026-10-27, 1 of 1 and exit 0 on the first run: the review came up with both sources and the file was untouched, cancelling left it untouched, and after a second review Replace wrote a file differing from the one on disk by exactly the generation line — byte-identical to what the generator had written before the harness took that line out. PHYSICAL RECORDING CAPTURE REMAINS NOT EXERCISED."
supersedes: []
affects:
  - execution
  - interface
source_paths:
  - internal/codegen/codegen.go
  - internal/orchestrator/generation.go
  - internal/orchestrator/regenerate.go
  - internal/routes/registry.go
  - cmd/marco/panicstop.go
  - plugins/studio/generation.go
  - plugins/e2e/scenario_generation.go
---

# A routine states which generation wrote it

## The gap, and why nothing could see it

A routine keeps the shape it was generated with. That is deliberate and
[[ADR-251-a-press-checks-the-window-it-was-recorded-in]] says why: the saved file is the only
executable truth about a routine, and rewriting generated Marco by hand is how two things that
have to agree stop agreeing.

The consequence is that every rule about what a generated routine DOES reaches a routine at the
moment that routine is written, and never afterwards. Five slices of rules have landed:

- presses are checked against the window they were demonstrated in, and a refusal stops the
  routine (ADR-251)
- a keystroke, a chord and a held key stop what follows them
- a drag states both of its ends in one window ([[ADR-259-a-guarded-drag-is-two-points-in-one-window]])
- a press states which button it was ([[ADR-260-a-press-is-a-button-and-a-place]])
- typed text is a sequence of submissions, and each run of text stops what follows it
  ([[ADR-261-typed-text-is-a-sequence-of-submissions]])

A routine saved before the last of those has a `Type` step that is not a link. It goes on
pressing past text that never arrived, and reports a run that performed. ADR-251 said this
plainly about keystrokes at the time and scoped the answer out:

> **A ROUTINE ALREADY ON DISK KEEPS THE SHAPE IT WAS GENERATED WITH.** … it will go on pressing
> past a keystroke that did not arrive until it is recorded again or saved again.

What made it invisible is that **nothing on disk said which rules wrote a file**, and nothing can
work it out. The shapes in a routine are a function of the demonstration as much as of the
generator: a routine with no measured press may have been recorded in coordinate mode on purpose
or recorded against a flat menu with nothing to anchor to, and a routine with no `Type` step may
predate the typing rules or simply contain no typing. That inference has already failed here, in
exactly this shape — before `draggedTerms` existed, `ContractOf` knew only about a `Point`, so a
guarded drag read as **legacy**, Studio told a person a routine that checks its window checks
nothing, and four desktop scenarios failed on the precondition before anything about a drag was
exercised.

So the generation is **stated** by the only thing that knows it, at the moment it is true.

## The stamp, and the three things it is not

`codegen` writes one line into every routine it generates:

	// Generation: 1.

**Unconditionally**, which is where this differs from every other line in that header. `Context`
depends on the route and `Recorded as` depends on what the caller knew; this depends on nothing,
because whatever called it, that generator wrote the file. A condition there would be a file
written by this generator that does not say so.

It is **not the build**. ADR-251 settled that in advance: an older build does not establish that a
routine needs anything, most builds generated what today's would, and a build id answers *when*
where the question is *under which rules*.

It is **not a Core change**. A comment is not a word of the language — the same ground
`// Recorded as:` stands on, and the reason that line was a comment too. Reported as such before
implementing, per the slice's boundaries.

It is **not a verdict**. The stamp says which rules wrote the source. It does not say the source is
guarded, or unmodified, or that anything else about it is still true: a routine of the current
generation can carry legacy behaviour on purpose, because a demonstration with no measured press
generates exactly the unchecked route it always did. So what a routine's presses promise stays
`ContractOf`'s separate question, read off the shapes, and both facts are shown with neither
derived from the other.

## Four readings, and the damage from merging any two

	CURRENT   states the generation this build writes.
	OLDER     states an earlier generation this build can explain.
	UNSTATED  says nothing.
	UNKNOWN   says something this cannot explain.

**Unstated means exactly that.** Not "written before versioning", which is a claim about when a
file was made that the file does not support: a handwritten routine says nothing here, and so does
one somebody took the line out of. The wording is the backend's, for all four readings, because
the wording is where this goes wrong.

**Unknown is neither of its neighbours.** A later build's generation read as `current` would be
this build writing down that it wrote a file it has never been able to read — in the run record,
which is the one artifact a person cites when they cannot work out why a routine behaved
differently somewhere else. Read as `unstated` it would lose the one fact the file does state, and
the declared value is what identifies the routine to whoever can explain it. So the reading and
the declared value are carried separately, everywhere: in the reading, on the card, and in the
record.

Malformed, conflicting and **duplicate** declarations all read as unknown. Duplicate counts even
when the two agree: a file answering one question twice has been edited by something, and which
answer the file means is not in the file. Reading the first, or the last, or the one they agree on
would each be a guess presented as a reading.

**Nothing is offered for an unknown generation.** A reader that cannot say what a generation does
cannot say what replacing it would change. What is withheld is the offer and nothing else: the
routine stays listed, stays runnable, and runs under exactly the rules it always had — held
against a control run of the same routine with no stamp, which has to reach the same outcome for
the same stated reason.

## Generation 1 has nothing behind it, on purpose

A generation is a claim about a file, and nobody stamped the files written before this. Numbering
the five rule changes above 1..5 would place routines in a history they never stated, and the only
way to decide which one a given file belonged to would be to infer it from its shapes — the
inference this whole thing exists to avoid, with a version number on it.

So generation 1 is what this generator does, its entry lists those rules in the words a person
reads, and everything before it is unstated.

**The history is one list.** The current generation is its last entry rather than a constant beside
it, and an entry with no words is a build that does not pass its own tests. That is
`everyMeasuredTerm`'s lesson one layer up: a term added to a shape's list and not to the reporting
list is read, found absent and never named, and a generation added to a counter and not to the
history would be read, found older, and never explainable.

**The difference walk is exercised against synthetic generations.** With one real generation nothing
can read as `older`, so three exported readers would have no live input at all — and this
repository's own record is eight cases of complete code, correct in itself, never invoked. A
made-up future is the honest way to exercise the walk, and the one authority is swapped for it
under `t.Cleanup` so nothing fabricated outlives the test.

## Older is not affected

A generation difference is a fact about two **generators**. Whether it changes anything for one
routine is a fact about that **routine** — so each rule carries its own question about a saved
source, and a routine is claimed to need replacing only where its contents answer it. Everything
else is listed as a comparison.

A card that announced every difference as a defect would be telling somebody to replace a file
over a rule it never exercises, and the next time it said so about a real one they would have
learnt to ignore it.

## Prepare, review, then replace or cancel

	SELECT    one exact routine, by its own identity.
	PREPARE   render its stored demonstration again. NOTHING IS WRITTEN.
	REVIEW    both sources in full, the differences, and any refusal.
	REPLACE   or cancel. Cancelling writes nothing, because nothing was written.

`Resave` is **not** this flow and stays what it is. It repairs a routine that cannot press anything
at all, and it is immediate on purpose — there is nothing to weigh up, because the file as it
stands refuses every press. It also rescopes, through the save path that chooses where a routine
lives. Neither is allowed here: this replaces a routine that works with one that works
differently, which is a decision only its owner can make, and the scope is read off where the
routine already lives so it cannot quietly move.

**Preparation writes nothing** — not the source, not the images, not the recording. That is what
makes cancelling the absence of an operation rather than a tidy-up a crash can interrupt, and it
is held against a digest of the whole library rather than of the one file, because "it did not
touch the file I was looking at" is a weaker claim than the one being made.

**A replacement re-reads both files it was built from** and refuses if either moved, naming which. A
person can edit the routine in `marco edit`, or record the same name again, while the review is
open; replacing then would write a rendering of a recording that is no longer there over a source
nobody reviewed, and silently, because the result would look like an ordinary successful
replacement. This is also the whole of the protection for a hand-edited routine: the edit is the
left-hand pane, so replacing it is a decision — what is not allowed is for that to happen without
the review being current.

**`Save`'s own rendering is now the one function both callers use.** A review built from a copy of
those four lines would be a second answer to what a draft becomes, and the disagreement would be
between the source somebody approved and the source that replaced theirs.

**A rebuild today's rules would refuse is a reviewable answer.** A recording containing a press found
by matching a picture was saved happily by an older build and produces no routine now, so "what
would I get if I rebuilt this" has three answers rather than two, and the third is shown in the
generator's own words beside the source it would have replaced.

## What the desktop scenario can and cannot see

The scenario presses Studio's own controls and reads the file off the library Studio was given.
Studio's words are read too and reported as evidence — a window that says `unstated` about one
routine and replaces another is a defect this product has had — but **the verdict is the bytes**.

Its strength is not "the file changed". The harness takes the generation line out of a routine
this build just wrote, so the same generator wrote both versions and the only difference a correct
replacement can have is that line restored. A rescope, a different simplification, a stale draft
or an empty file all fail it. On the first desktop run the replaced file differed from the stripped
one by exactly that line and was **byte-identical** to what the generator had written before.

What that arrangement establishes is bounded, and the boundary is stated where it happens: it
exercises the flow against a routine that states no generation. It is not a claim that any
particular older routine in anybody's library looks like this, and nothing in it reconstructs a
generation for an unstamped file.

**No input is sent.** The fixture is open because the routine has to be a real one, generated by the
production path from real window measurements; its window is never pressed. **Physical recording
capture is NOT EXERCISED**, as everywhere: the demonstration behind the routine is seeded, and the
harness says so in its own report.

## What the mutation gate found

Three holes, and two of them were the same hole in different places. A routine this build just
wrote rebuilds **byte for byte** from its own recording — so in the two tests that used one, the
saved and rebuilt sources were identical, and that made two different wrongs invisible: a
preparation that saved its own result changed nothing a digest could see, and a replacement that
wrote the OLD source instead of the reviewed one wrote the same bytes either way. *Successful
replacement produces exactly the reviewed source* was passing without being able to fail. Both
tests now take the generation line out first and assert the two differ before relying on it.

The third was a widened lookup: `routineWithID` also matching a name or a slug is invisible until
something passes one, and nothing did. A name that two routines answer to is not an address, and
choosing one is how a person approves one routine's source and has another one's file replaced.

One mutant survives on purpose. `Registry.Replace`'s atomicity cannot be told from `Save` by any
test here: they differ only when a write fails PART WAY, and a disk full is not something a test
stages portably. It is a property of the mechanism rather than one anything checks, and the
comment beside it says so rather than letting it look held.

## Four things the review of this slice found

**[P1] A replacement identified the routine, not the review on the screen.** Studio held one
preparation per routine, so a second review of one routine overwrote the first — and these calls
are asynchronous, so the page could be displaying A's two sources while the window held B. Press
Replace and B is written: every check in `Deps.Replace` passes, the success names the routine the
person was looking at, and the bytes are ones they never saw.

So a preparation carries an immutable **token**, and a replacement must present it. The token is
derived from the content — the two files it was built from, what it would write, and why it would
write nothing — rather than counted, for two reasons. What has to be identified is the *review*,
and the content IS that: two preparations sharing a token are byte-for-byte identical in what was
displayed and in what a replacement would write, so resolving one to the other cannot write
anything nobody saw. And a counter would need somewhere to live, which is the shared state the
defect came out of. `Deps.Replace` also checks a `Prepared` against its own token, so one edited
on its way back — by a caller, by a bug, by a struct assembled from parts of two — says so.

Studio now holds reviews keyed by token, bounded at sixteen, oldest out first. An evicted review
is a refusal with a reason rather than a wrong write.

**[P1] A failed replacement could change the original's images.** The images were written first and
called additive. They are not: they are named after the routine — `<base>-anchor-1.png` — so a
rebuild produces the same paths and `WriteAssets` overwrites what is there. Write the new images,
fail to write the source, and the OLD source is still on disk pointing at paths that now hold
different pictures. A routine that presses what a different image matched, having been left alone.

A conflicting image is now a **refusal**, and nothing is written at all. A path holding the same
bytes is no conflict, because writing it changes nothing — which matters, or every re-review of an
anchored routine would refuse. A path holding nothing is no conflict either: a file nobody
references is not a change to anything that runs. What a failed SOURCE write can still leave is
new image files beside an unchanged old source that does not reference them — unreferenced files,
with the routine behaving exactly as it did.

The alternative — content-addressed image names — was rejected for this slice on scope: it changes
what every newly generated routine's files are called, reaching every asset writer and every saved
file's embedded paths, and would arrive inside a provenance slice unreviewed.

**[P2] The unknown-generation refusal checked an earlier read.** `ReviewRebuilt` read the LISTING's
eligibility, and `Prepare` then read the source again without asking what THAT snapshot stated.
Replacing the file with an unknown-generation version between the two reads bypassed the one
restriction where this build refuses to say what a replacement would change. The rule is now
applied where the bytes are: `Prepare` reads the generation off the source it actually read and
refuses there. Studio's check survives as the greying of the button, which is a courtesy rather
than the enforcement.

**[P2] "Current" was presented as proof of identical regeneration.** The sentence promised "it will
be the same routine", which the stamp cannot support: the generation says which rules WROTE the
source, not that the source is still what they wrote. A routine somebody edited by hand keeps its
stamp and rebuilds differently — and that person is exactly who would have been told their
comparison was pointless. It now offers the comparison as the question it is, and only
`Prepared.Changes()` answers it, after the rebuild.

**The gate against these four: 14 mutations, 13 killed, one real hole found.** Nothing exercised
Replace from the PAGE — the rendered test reviewed and cancelled — so the page sending a routine's
identity in place of the token survived. The page now reviews again and replaces, and asserts
which term travelled in both directions.

### And two of those four were not closed by the first attempt

**The token's content argument was incomplete.** It digested only the two files, what would be
written, and why nothing would — on the argument that two preparations sharing a token are
identical in what was displayed and in what a replacement would write. That said nothing about
WHERE. Two routines with identical sources and identical demonstrations are a thing this product
produces readily: the same demonstration saved in-app for two applications is byte-for-byte
identical, because an in-app routine switches nothing so the generator is told no application and
the source never names one. They would have shared a token, and a window holding one and handed
the other's token would write the right bytes to the wrong file — with a success naming the routine
the person was looking at.

So the route's whole identity and the library are digested too, each part separately so no two
routes can run together into one string. `Focus` is in because it is the difference between a
routine that brings its application forward and one that does not, and those are two files. Two
preparations sharing a token now agree about all four questions: what was shown, what would be
written, where, and in which library. A `Deps` pointed at another library is also refused on its
own, with its own sentence, because "not the one that was prepared" would send somebody to look at
a review that is perfectly fine.

**And a refusal was not the whole of the image rule.** Deciding that no image conflicted and then
writing all of them left two of the four states wrong. A path that already holds the right picture
is now SKIPPED rather than rewritten: writing a file the bytes it already holds changes nothing
that runs and is still a write — it touches the file, it can fail, and it is one more thing between
a person's decision and the one file this is meant to replace. And a path that cannot be READ is
now a refusal rather than an empty space, which is the correction that matters: treating
unreadable as "nothing there" is the one reading that licenses writing over it, and a path this
cannot read is precisely a path it cannot say is safe to replace. Only the MISSING ones are
written, and every path is decided before any of them is written, so a refusal found on the second
image cannot leave a routine beside pictures from two different rebuilds.

### And classifying is not writing

Deciding which images were missing and then writing them left a gap, and the write was
create-or-truncate. Between the two, another save — another Studio, a `marco learn`, a sync — can
create one of those paths, and `os.WriteFile` would then destroy what they had just written on the
strength of a reading taken beforehand. Not a collision between unrelated files: the paths are
reusable, so it is the same routine's own image name, written by whatever else is working in that
library.

So the creation IS the check. `Registry.CreateAssets` opens each path with `O_CREATE|O_EXCL`,
which closes the gap rather than narrowing it — there is no instant between the test and the write
because they are one operation — and the path existing is the failure, with nothing written to it.
`WriteAssets` keeps create-or-truncate, which is correct for a save: a fresh save owns the paths
it names, and a collision on the routine's name is refused long before. Two functions rather than
one with a flag, because the two callers want opposite things for good reasons.

Files created before such a refusal are LEFT. They are paths that did not exist, nothing
references them, and removing them would mean deleting a file between creating it and tidying it
up — a narrower version of the hazard this closes.

**And the moment is tested, not just the mechanism.** `Deps.afterClassifying` is an unexported
seam that occupies the instant between the two steps, and what it does is what the other save
would have done. Nothing in production can set it — a `Deps` is built from its exported fields
everywhere — and nothing has a reason to be notified of that instant, which is the point: the only
way to show a gap is closed is to put something in it.

## What is unchanged

- **No saved routine is rewritten or migrated.** Nothing is automatic, and the person decides.
- **Nothing is refused over a comment.** An unknown generation runs as it always did.
- `ContractOf`, `ModeOf` and the three saved contracts are untouched.
- `os.marco` is **unstamped**: it is the act surface, not a generated routine, and two tests police
  it byte for byte.
- No generation's rules changed. This slice adds a statement about what the generator does and
  changes nothing it does.

## Enforced by

- `internal/orchestrator` `TestTheGenerationASaveWritesIsTheGenerationThisReads` — the writer and
  the reader cannot share a constant, because `codegen` may not import `orchestrator`, so the pair
  is held here: a routine saved through the production path reads back as current, at the number
  `codegen` says it writes, and says so in words a person opening the file would see.
- `internal/orchestrator` `TestEveryEntranceToTheGeneratorStampsTheGeneration` — the dozen callers
  that know nothing about a recording still produce a routine that says which rules wrote it,
  which is where this differs from the stated mode.
- `internal/orchestrator` `TestTheHistoryIsTheOneAuthorityForTheVersionAndTheWords` — the current
  generation is the last entry, the entries are the contiguous 1..n a reading walks, every one has
  words, and every generation in range can be explained.
- `internal/orchestrator` `TestARoutineThatStatesNoGenerationReadsAsUnstated` — including the
  wording: the sentence for an unstated routine may not say "before", "older" or "out of date"
  about a file that states nothing.
- `internal/orchestrator` `TestAStampThisCannotReadIsUnknownAndNeitherOfTheOthers` — six ways a
  declaration is unreadable, each one unknown rather than current or unstated, each preserving what
  the file said, and none of them offered a rebuild. Duplicates included, agreeing ones among them.
- `internal/orchestrator` `TestANewerStampIsNotRelabelledCurrentOrLost` — the reading an older
  executor gets of a later build's routine, with the declared value kept and nothing walked from a
  generation this build has no account of.
- `internal/orchestrator` `TestTheGenerationAndTheContractAreIndependentFacts` — a routine today's
  generator wrote reading `current` and `legacy` at once, and a measured one reading `current` and
  `guarded`. Two readers, neither derived from the other.
- `internal/orchestrator` `TestARoutineThatTypesTheGenerationLineStatesNothing` — a `Type` step
  carrying the text of this very line is not a declaration, and the same routine with a real stamp
  still reads it.
- `internal/orchestrator` `TestTheDifferencesAreTheGenerationsAfterTheOneStated`,
  `TestOlderDoesNotMeanAffectedUnlessTheContentsShowIt` — the walk and the contents gate, against a
  synthetic history of three, with a control routine that is older and claimed to need nothing.
- `internal/orchestrator` `TestTheExportedReadersWalkTheHistoryAndSayWhatChanged`,
  `TestAnUnknownReadingIsNotOfferableUnderAnyHistory` — the same mechanism through the exported
  door every caller uses, with the one authority swapped for a made-up future and restored on
  cleanup. Without these, three delegations nothing can enter today would be unwired and green.
- `internal/orchestrator` `TestPreparingAndCancellingAVersionWritesNothing` — against a digest of
  the whole library, and against a routine whose rebuild DIFFERS, so a preparation that wrote its
  own result is visible. The degenerate case is asserted away first.
- `internal/orchestrator` `TestAReplacementWritesExactlyTheSourceThatWasReviewed` — the claim the
  flow rests on, with the two sources known to differ, plus: the file stays where it was, its
  demonstration survives, and rebuilding the replaced routine changes nothing.
- `internal/orchestrator` `TestAReplacementAfterTheRoutineChangedRefusesAndLeavesItAlone`,
  `TestAReplacementAfterTheRecordingChangedRefusesAndLeavesItAlone` — both files checked, both
  refusals naming which moved, the routine left exactly as it was, and a fresh review comparing
  against the routine as it now stands.
- `internal/orchestrator` `TestTheScopeAReplacementUsesIsTheOneTheRoutineAlreadyHas`,
  `TestAReplacementKeepsAScopeThatChangesWhatIsGenerated` — all three scopes read off where a
  routine lives, and the context-scoped case where the scope changes what is GENERATED: no
  `Activate` goes in, and the file does not move.
- `internal/orchestrator` `TestARebuildTodaysRulesWouldRefuseIsShownAndCannotBeReplacedWith` — a
  recording with a press found by matching a picture: a reviewable answer in the generator's words,
  the saved source still shown beside it, and a replacement refused carrying that reason.
- `internal/orchestrator` `TestARoutineWithNoDemonstrationIsExplainedAndCannotBeRebuilt`,
  `TestAReplacementFromNothingIsRefused`, `TestTheRegistryRefusesToReplaceSomethingWithNoName` —
  the explanation available where the rebuild is not, and the two ways a replacement can be asked
  for with nothing to write.
- `internal/orchestrator` `TestTwoPreparationsOfOneRoutineAreTwoReviews` — two preparations of one
  routine built from different sources have different tokens, and the EARLIER one presented after
  the second was prepared is refused with the routine left alone, while the current one goes
  through. The out-of-order case, which is about which response arrived last rather than which was
  asked for first.
- `internal/orchestrator` `TestAPreparationThatWasAlteredDoesNotMatchItsToken` — a `Prepared` whose
  source was swapped after it was prepared no longer matches its own token, with the control that
  the unaltered one still goes through.
- `internal/orchestrator` `TestARoutineThatBecameUnexplainableBetweenTheReadsIsRefused` — the
  interval: a routine that was eligible when it was listed and states a generation this build
  cannot explain when it is read. Refused, with nothing written, and the arrangement asserting it
  WAS eligible first so the refusal is about the gap.
- `internal/orchestrator` `TestAReplacementThatWouldOverwriteADifferentImageIsRefused` — against a
  real anchored routine whose rebuild produces the same image path: the refusal names the image,
  and the source AND the image are both exactly as they were. The second half is what the finding
  was about.
- `internal/orchestrator` `TestAnImageThatIsAlreadyTheRightOneIsNoConflict` — the control, in both
  directions: the ordinary case where a rebuild renders the same picture, and a path holding
  nothing. Without it the rule refuses every re-review of an anchored routine.
- `internal/orchestrator` `TestTheSentenceForACurrentRoutineOffersTheComparisonAndPromisesNothing`
  — it offers the comparison as a question and promises nothing about the result, with the
  hand-edited current routine that is the reason.
- `plugins/studio` `TestTwoReviewsOfOneRoutineAreTwoReviews` — through the Acts: two reviews of one
  routine, the earlier token reaching its own review and being refused for the right reason (only
  its source digest is stale, so a token resolving to the other would have gone through), the
  current one writing its own source, and cancelling a review nobody made leaving an open one
  alone.
- `cmd/marco` `TestTheRecordSaysWhichGenerationWroteTheRoutine`,
  `TestTheRecordSaysUnstatedWhenTheRoutineNamesNoGeneration`,
  `TestTheRecordKeepsAMalformedStampVerbatim`,
  `TestTheRecordInFlightAlreadySaysWhichGenerationWroteIt`,
  `TestARoutineThatCouldNotBeReadStatesNoGenerationEitherWay` — the record's two fields, off the
  same snapshot that executes, present before the first input like the rest of the identity, and
  absent entirely for a file nobody read.
- `cmd/marco` `TestTheRecordDoesNotRelabelALaterGenerationAsItsOwn` — the silent one, with a
  CONTROL: the same routine unstamped has to reach the same outcome for the same reason and ask for
  the same number of input actions, so nothing is refused or diverted over a comment.
- `internal/orchestrator` `TestTwoRoutinesWithIdenticalContentsDoNotShareAToken` — the same
  demonstration saved in-app for two applications, which really is byte-for-byte identical, with
  the precondition asserting so: different tokens, and replacing one leaves the other exactly as
  it was. The first version of this test used two SCOPES and failed its own precondition, because
  `app-anywhere` prepends an `Activate` and `app` strips it.
- `internal/orchestrator` `TestAPreparationCannotBeReplacedWithInAnotherLibrary` — against a second
  library really holding a routine of that name, so the write would have landed on a real file;
  the refusal names the library, and the library it WAS prepared in still takes it. The token's own
  claim about the library is asserted directly beside it, because the refusal would otherwise be
  the only thing holding it and the token is a public value with a documented meaning.
- `internal/orchestrator` `TestAnImageThatAlreadyHoldsTheRightPictureIsNotWrittenAgain` — by making
  the write IMPOSSIBLE: the file is left read-only, so a replacement that writes it fails and one
  that skips it succeeds. Comparing timestamps would not do — the two writes are the same bytes
  microseconds apart.
- `internal/orchestrator` `TestAnImageThatCannotBeReadIsARefusal` — something that is not a file
  where the image belongs, which is the one unreadable path a test can arrange on any machine.
- `internal/orchestrator` `TestARefusalOnOneImageWritesNoneOfThem` — a conflicting image and a
  missing one together: nothing is written, including the missing one, whichever order the map
  happens to be walked in; with the control that without the conflict the missing one IS created.
- `plugins/studio` `TestTwoRoutinesWithTheSameContentsAreTwoReviews` — the same finding at the layer
  the slot lives in, against a pair seeded through the registry because Studio rightly refuses to
  SAVE a guarded routine in-app — a guard about saving, not about what a library can contain.
- `internal/orchestrator` `TestAnImageThatAppearsAfterClassificationIsRefused` — the MOMENT, not
  the mechanism: `afterClassifying` occupies the instant between deciding a path was missing and
  creating it, and writes what another save would have. The refusal says the image appeared, the
  image holds what that other save wrote, and the source is untouched — with the control that the
  same replacement goes through when nothing appears, so it is not a function that refuses
  whenever an image is absent.
- `internal/orchestrator` `TestCreatingAssetsRefusesAnExistingPathAndWriteAssetsDoesNot` — the
  mechanism under that moment: `CreateAssets` answers `fs.ErrExist` and leaves the file exactly as
  it was, creates a path that is free, and `WriteAssets` still truncates — which is why the two
  are separate functions rather than one with a flag.
- `plugins/studio` `TestTheListingSaysWhichGenerationWroteARoutine`,
  `TestARoutineWithNoDemonstrationIsListedAsNotOfferable`,
  `TestARoutineFromALaterBuildIsNotOfferedAReviewAndIsUntouched` — the card's fields off a file
  this build really wrote, the two different reasons a rebuild is not offered, and the later
  build's routine still listed, still runnable, its source untouched by being looked at.
- `plugins/studio` `TestNothingIsReplacedWithoutAReviewInHand`,
  `TestCancellingAReviewLetsGoOfTheVersion` — the rule that makes the review more than decoration,
  with the control that it goes through when there IS one, and the spent review that cannot be
  written twice.
- `plugins/studio` `TestAReviewIsOfTheRoutineThatWasAskedAbout`,
  `TestAReviewOfTwoSameNamedRoutinesIsOfTheOneAddressed` — by identity, with the other routines
  untouched, and a name that two routines answer to REFUSED rather than resolved.
- `plugins/studio` `TestThePageSaysWhichGenerationWroteEachRoutine`,
  `TestThePageOffersARebuiltVersionOnlyWhereThereIsOneToOffer`,
  `TestAReviewShowsBothSourcesAndCancellingWritesNothing` — the page RENDERED, including which
  term travels on the way out and the way back: it reviews, cancels, reviews again and replaces,
  and asserts the REVIEW'S token reached `CancelReview` and `ReplaceWithRebuilt` rather than the
  routine's identity. Nothing exercised Replace from the page before, so the mutation that sends
  the identity survived, because a source
  guard passes as long as a field is named somewhere in the file: all four sentences, no
  `undefined`, the button live for the two readings that may be offered and dead for the two that
  may not, both panes with their own labels, and a cancel that lets go of the version and replaces
  nothing.
- `plugins/e2e` `TestTheReviewScenarioRunsLikeAnyOther` — it is in every default run, and the name
  claim that outlived the gate: asked of every registered family, so no other family's selector
  takes it.
- `plugins/e2e` `TestAReplacementThatChangedAnythingElseIsAFinding`,
  `TestAFileThatChangedBeforeReplaceIsAFinding` — the scenario's arithmetic, headlessly: four ways
  a replacement can be wrong and still have changed the file, and a file written to before Replace
  reported with both versions in the sentence.
- `plugins/e2e` `TestTheReviewScenarioAsksEveryQuestionItsVerdictRestsOn` — the wiring, by AST
  because a text search finds each claim's own definition further down the file: every
  disagreement, both waits, all three controls pressed, and Review pressed exactly twice.
- `plugins/e2e` `generation/a reviewed rebuild replaces the routine` — **RAN GREEN ON A DESKTOP ON
  2026-10-27**, 1 of 1, exit 0 on the first run (`plugins\e2e\run.cmd -only generation`). The
  review came up with both sources in it and the saved routine was untouched; cancelling left it
  untouched; and after a second review, Replace wrote a file differing from the one on disk by
  exactly the generation line and byte-identical to what the generator had written before the
  harness removed it. Artifacts `20261027-162205`.
