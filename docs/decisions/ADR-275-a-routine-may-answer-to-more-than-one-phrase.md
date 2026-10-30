---
type: adr
status: accepted
date: 2026-10-30
decision: "ACCEPTED — A ROUTINE MAY ANSWER TO PHRASES BESIDES ITS OWN NAME, KEPT IN A SIDECAR BESIDE ITS SOURCE, AND THE SLUG ALWAYS WINS. A route IS its scope and its slug, so the only way to reach one by a second phrase was to save a second routine — two files, two recordings and two things to keep in step. `<slug>.aliases.json` holds a version, the route IDENTITY it belongs to, and the phrases VERBATIM as somebody typed them, because the folding happens when they are compared and the list read back should be the list that was written. IT CARRIES NO DIGESTS, which is the deliberate difference from the step account beside it: that records a rendering basis and is bound to the bytes it rendered, where a phrase is a name for the identity — editing a step, re-simplifying, regenerating or rebuilding all change the source and change nothing about what a routine is called, and a file bound to the source would have dropped somebody's phrases every time they fixed a wait. What it IS bound to is the identity, because that is the thing that moves: a rename rewrites it as part of the move, and a file whose binding names another identity is NOT believed — its phrases answer for nothing and the state says why, fail-closed in that one direction because the cost of believing an orphan is a phrase that runs a routine nobody attached it to. FIVE STATES, and absence is `os.IsNotExist` and nothing else: a file that could not be READ is a different answer from one that was read and would not parse, and both are unreadable rather than absent — the one answer that licenses writing over it. THE INTAKE DOOR DOES NOT RESOLVE A PHRASE AT ALL, which is the correction below: it answers EXACTLY or not at all, and whether a phrase is unambiguous is a fact about the whole library that a door with nowhere to ask may not settle. `Registry.Resolve` is unchanged from before phrases existed, so a name that runs something today keeps running it; `AnsweringTo` lists every routine a phrase answers for and Director chooses, applies the scope rules and asks which. A PHRASE INHERITS ITS ROUTINE'S SCOPE through `resolveroute.Eligible`, because a scope is about what a routine can DO and not about how it was asked for. IN DIRECTOR A PHRASE IS ANOTHER LABEL FOR THE SAME CANDIDATE, so `dispatch.Decide`'s first arm answers it — exact, offline, `not-asked`, no plugin and no model server — and a library whose routines all answer to several phrases still resolves with Ollama stopped. A NAME OUTRANKS A PHRASE INSIDE DIRECTOR TOO: names and phrases are counted APART, so one routine's phrase cannot drag another routine's own name into ambiguity, and a phrase reducing to a key some routine is CALLED is never offered as a label. A phrase no name claims and more than one routine answers to is claimed by nobody and becomes the question the shared-name clarification already asked, naming WHICH phrase each routine answered by; counted on `nlu.Key` and once per routine, so a routine answering to two phrases that reduce alike is still one routine. Two routines CALLED the same thing are the question, and a third that merely answers to it is not part of it. THE ADD REFUSES A COLLISION IT CAN SEE, naming the routine the phrase already reaches, on BOTH normalizations — `routes.Slug` is the door's and `nlu.Key` is the resolver's, and a phrase that is distinct at one and ambiguous at the other is a phrase whose meaning depends on which surface asked. It is a COURTESY AND NOT THE GUARANTEE, because a rename, a move or a routine saved later produces a collision no add could have seen, and resolution clarifies regardless. A routine's own name, or a phrase that reduces to it, is SAID rather than stored. A REMOVAL NAMES THE PHRASE and never a position, so the hazard the step-editing slice needed a basis token for does not exist here rather than being guarded against. Every lifecycle row is decided: a rename carries them and rewrites the binding, a MOVE carries them, rebinds them to the destination, verifies them there before the original is removed and makes the destination's own file match the original either way — written when there are phrases, removed when there are none — and refuses when a phrase was taught between the review and the write; a delete removes them, a move's second half sweeps them and REPORTS what it could not, a FRESH CREATION CLEARS residue at the identity it is taking BEFORE publishing the source, under the write lock its callers hold, and REFUSES when that clear fails — the binding is to a place, so a file left there matches again the moment a source exists beside it, and clearing afterwards leaves an interval in which a resolver offers somebody else's phrase, a replacement KEEPS them and says so, and a step edit or a regeneration touches nothing. Studio's routine card is the surface: the phrases listed, one added, one removed by name, the backend's own refusal shown, and a refused phrase left in the box the person typed it into. Routes only — a demonstration in the demo store has its own name field and its own lookup, stated as a boundary. 32 of 33 mutations killed; the survivor is a second statement of a rule the page's redraw hook already enforces, written down as one."
supersedes: []
affects:
  - execution
source_paths:
  - internal/routes/aliases.go
  - internal/routes/registry.go
  - internal/orchestrator/alias.go
  - internal/orchestrator/keep.go
  - internal/resolveroute/resolveroute.go
  - cmd/director/resolveroutewiring.go
  - plugins/studio/answers.go
  - plugins/studio/frontend/index.html
---

# A routine may answer to more than one phrase

The trace and the alternatives are [[Command-aliases-proposal-2026-10-30]].

## What a name was

One. A route IS its scope and its slug — `global/<slug>`, `<app>/context/<slug>`,
`<app>/focus/<slug>` — and `routes.Slug` folds case, punctuation, surrounding quotes and runs of
whitespace onto that one durable identity. That normalization is good and is the reason the
deterministic tier can be exact rather than generous ([[Invocation]]), and it answers exactly one
question: what does this spelling of THE NAME reduce to.

So a person who thought of their routine two ways had one option: save it twice. Two `.marco`
files, two recordings, two step accounts, and two things to keep in step every time they changed
their mind about what it should do.

## The decision

A sidecar beside the source, like the demonstration, the provenance and the step account.
`<slug>.aliases.json`: a version, the route identity it belongs to, and the phrases as somebody
typed them.

### It carries no digests, and that is the point

The step account next to it records a rendering BASIS, so it is bound to the bytes it rendered and
a moved source digest means something ([[ADR-273-a-routine-writes-down-what-it-does]]). A phrase is
not about the bytes at all. Editing a step, re-simplifying, regenerating, rebuilding — all of them
change the source and none of them changes what the routine is called. A file bound to the source
would have thrown somebody's phrases away every time they fixed a wait.

What it is bound to is the IDENTITY, because that is the thing that moves. A rename rewrites it as
part of the move; a file whose binding names another identity is not believed, and the state says
why rather than the phrases quietly answering for whatever is saved there now.

| state | is |
| --- | --- |
| `none` | no file: it answers to its own name and nothing else, which is every routine already on disk |
| `intact` | bound to the routine reading it — the only state whose phrases are offered |
| `elsewhere` | the binding names another identity: a move whose rewrite did not land, or a slug something else has since been saved to |
| `orphaned` | phrases with no routine beside them |
| `unreadable` | cannot be parsed, or states a version this build does not know |

Absence is `os.IsNotExist` and nothing else — the correction `readSteps` and `Collision` both
carry. A file that could not be READ is a different answer from one that was read and would not
parse, and both are unreadable, because absence is the single answer that licenses writing over it.

### The door does not resolve a phrase

`Registry.Resolve` answers on the slug exactly as it did before any of this, and stops. It does
NOT consult the phrases.

That is a correction. The first version did consult them, by the same four-scope priority a slug
walks, and it looked like the natural reading of "the slug always wins" — the phrases came last, so
nothing could lose its own name. What it missed is that *exact* and *unambiguous* are different
properties. A phrase is exact. Whether a phrase names ONE routine is a fact about the whole
library, and this door has nowhere to ask: a phrase two routines answered to was selected by
priority and performed, while Director — asked the same words, against the same library — asked
which was meant. One phrase, two answers, and the one that acted was the one that could not ask.

So ambiguity has a single owner, which is the arrangement
[[ADR-267-director-owns-route-resolution]] already describes: the intake is the AUTHORITY door and
its lookup is exact on purpose, and anything needing a judgement is Director's.

**What it costs is stated rather than hidden.** With Director unavailable, a phrase reaches
nothing, where a routine's own name still runs offline. That is the same boundary the widget's
learn offer already has — Director down means no resolution, and cards and Play keep working — and
it is the price of one owner for the question.

`AnsweringTo` lists every routine a phrase answers for, without filtering and without choosing,
because eligibility is about the moment a request is made and `internal/routes` knows nothing about
that.

### In Director it is another label

`internal/resolveroute` offers a phrase as one more label for the same candidate, so
`dispatch.Decide`'s first arm answers it: exact, offline, free, reported as `exact` and
`not-asked`. A library whose routines all answer to several phrases resolves with no plugin
configured and no model server running. That is the "without requiring Ollama" requirement held as
an assertion rather than left as a hope.

Three rules keep the labelling honest, and the middle one is load-bearing:

- **own names first**, so a phrase can never displace the routine that is actually called that;
- **a shared phrase is claimed by nobody** — handed to one of the routines that answers to it, the
  phrase would be an exact match on that label, answered offline, selecting whichever the listing
  put first, with neither the resolver nor the person asked;
- **a label this package invents last**, because `settings-in-ark` is what is left when two
  routines want one name and a phrase somebody chose is what they are likely to type.

`sharedNames` counts phrases as names, and once per routine: two phrases of one routine that
reduce to the same key are still one routine and must not look like a collision with itself.

### The add refuses what it can see, and nothing more

A phrase more than one eligible routine answers to selects nothing and asks which — that holds
whatever the add does, because a rename, a move or a routine saved afterwards produces a collision
no add could have seen. What the add contributes is a refusal at the one moment somebody can
choose a different phrase instead, naming the routine the phrase already reaches.

It is a **courtesy and not the guarantee**, and saying so matters: a product that refuses at the
add tends to grow a resolution path that assumes the collision cannot happen.

It refuses on BOTH normalizations. The door folds with `routes.Slug`; the resolver compares on
`nlu.Key`, which also drops hyphens and stop words. A phrase that is distinct at the door and
ambiguous in the resolver is a phrase whose meaning depends on which surface was asked — and this
is not hypothetical: writing a Studio fixture produced `type two letters please` for a routine
called `type two letters`, which the matcher cannot tell apart, because `please` is a stop word.

### A removal names the phrase

Not its position. The step-editing slice needed a basis token because a row number means nothing
without the list it was read off ([[ADR-273-a-routine-writes-down-what-it-does]]). A phrase is its
own identity, so that hazard does not exist here rather than being guarded against — which is the
better answer when it is available.

### Every operation decides

| operation | the phrases |
| --- | --- |
| rename | carried, and the binding rewritten — the file alone is not enough |
| delete | removed |
| a move's second half | swept, and reported as residue when they cannot be |
| saving over a routine | KEPT, and the collision sentence says so: they belong to the identity, not the files |
| replace, regenerate, Simplify, a step edit | untouched |
| the legacy `marco edit` save | untouched; it knows nothing about either sidecar |

The replacement row is the one worth reading twice. A step account is LOST when somebody
demonstrates a routine again, and that is right. Phrases are KEPT, and nobody would guess that
from a sentence about files being replaced — so `Collision.Says` names them and says they would
reach whatever is saved there.

### Routes only

A demonstration in the demo store has its own name field, its own `Sequence(name)` lookup and its
own store, and the lifecycle above is a route lifecycle. Reaching both stores in one slice would
mean two alias mechanisms or one spanning two stores. Stated as a boundary rather than discovered
as an omission.

## Enforced by

- `internal/routes`: `TestPhrasesAreStoredAsTheyWereTyped`,
  `TestAnUnreadablePhraseFileIsNotTreatedAsNone` and
  `TestAPhraseFileThatCannotBeReadIsNotTreatedAsNone` — the two unreadable cases are separate
  branches and the second was written for a survivor.
  `TestPhrasesBoundToAnotherRoutineAnswerForNothing`, `TestPhrasesWithNoRoutineAreOrphaned` — the
  binding, fail-closed. `TestTheDoorDoesNotResolveAPhraseItself` and
  `TestASharedPhraseSelectsNothingAtTheDoor` — the correction above, both directions.
  `TestARoutinesOwnNameBeatsAnotherRoutinesPhrase`, `TestAPhraseIsMatchedByTheSameFoldingANameIs`,
  `TestEveryRoutineAnsweringToAPhraseIsListed`, `TestAPhraseTwoRoutinesAnswerToIsReportedAsBoth` —
  what the reader answers and what it leaves to Director.
  `TestRenamingARoutineCarriesThePhrasesItAnswersTo`,
  `TestDeletingARoutineTakesAwayItsPhrases`, `TestAMoveSweepsThePhrasesAndReportsWhatItCouldNot`,
  `TestARoutineWithNoPhrasesHasNoFile` — the lifecycle.
  `TestAFreshRoutineDoesNotInheritLeftoverPhrases` and
  `TestAReplacementKeepsThePhrasesTheIdentityAnswersTo` — the two opposite answers at one
  identity. `TestNoInstantLetsAFreshIdentityAnswerToAnOrphan` — the ORDER of the two writes,
  through the one seam that can look between them, because both end states are identical either
  way round. `TestAPhraseFileThatWillNotGoRefusesTheCreation` (Windows) — a file that can be read
  and cannot be deleted, which is the state that reads as `intact` the moment a source appears
  beside it, and the reason the removal's error is not ignored.
  `TestARefusedCreationLeavesTheRoutineAndItsPhrasesAlone` — the preservation control: a refusal
  that has changed the library is not a refusal.
  `TestAnythingAtTheSourcePathRefusesTheCreation` — renamed after a mutation showed it reaches the
  first branch and not the third, which is defensive and said to be.
  `TestARoutineThatAppearsDuringACreationIsNotOverwritten` — `O_EXCL` in the only instant it
  answers for, which nothing held once the source was checked first.
- `internal/orchestrator`: `TestAddingAPhraseMakesTheRoutineAnswerToIt`,
  `TestAPhraseAnotherRoutineIsCalledIsRefused`,
  `TestAPhraseAnotherRoutineAlreadyAnswersToIsRefused`,
  `TestAPhraseThatCollidesInEitherNormalizationIsRefused`,
  `TestAddingARoutinesOwnNameIsSaidRatherThanStored`, `TestAddingAPhraseTwiceChangesNothing`,
  `TestAPhraseIsRemovedByNamingIt`, `TestRemovingAPhraseThatIsNotThereIsNotAnError`,
  `TestAPhraseWithNoWordsInItIsRefused`, `TestPhrasesBoundElsewhereAreNotAddedTo`.
  And the move: `TestAMoveCarriesThePhrasesTheRoutineAnswersTo`,
  `TestAMoveClearsPhrasesSittingAtTheDestination`,
  `TestAPhraseTaughtSinceTheMoveWasReviewedRefusesIt`,
  `TestAMoveRefusesAnUnreadablePhraseFile`.
- `internal/resolveroute`: `TestAPhraseResolvesOfflineWithNoResolverConfigured` — the
  Ollama-free claim, with no Advisor at all. `TestAPhraseResolvesToTheCanonicalRoutine`,
  `TestAPhraseIsNotOfferedWhenItsRoutineIsNotEligible`,
  `TestAPhraseForAnUnrunnableRoutineIsNotOffered`, `TestAPhraseTwoRoutinesAnswerToAsksWhich`,
  `TestAPhraseNeverDisplacesARoutinesOwnName`, `TestASharedPhraseIsOfferedAsNobodysLabel`,
  `TestOneRoutineWithTwoPhrasesThatReduceAlikeIsNotACollision`. And the precedence:
  `TestARoutinesOwnNameOutranksAnotherRoutinesPhrase`,
  `TestAPhraseThatAnotherRoutineIsCalledIsNotOffered`,
  `TestNamesakesAreTheQuestionAndAPhraseHolderIsNotPartOfIt`.
- `cmd/director`: `TestTheLibraryOffersWhatEachRoutineAnswersTo` — the wiring between the file and
  the resolver, which neither side can hold alone.
- `cmd/marco`: `TestAPhraseIsHandedToDirectorRatherThanClaimedHere`,
  `TestASharedPhraseSelectsNothingAtTheDoor`, `TestAPhraseNeverOutranksAControlWord` — the door's
  own half of the correction.
- `plugins/studio`, in two halves. Against a real library:
  `TestARoutineWithNoPhrasesIsOfferedTheControlAndNoComplaint`,
  `TestAPhraseAddedInStudioIsWhatTheDoorAnswersTo`, `TestAPhraseIsListedAsItWasTyped`,
  `TestAPhraseThatReducesToTheRoutinesOwnNameIsSaidRatherThanStored`,
  `TestACollidingPhraseIsRefusedWithTheBackendsOwnSentence`,
  `TestARefusedPhraseIsShownAsTheContractsOwnRefusal`,
  `TestAPhraseIsRemovedFromTheCardByNamingIt`, `TestThePhrasePanelHoldsOffWhileRecording`,
  `TestPhrasesForARoutineThatIsNotThereSayWhy`. Rendered: `TestTheAnswersPageDoesNotThrow`,
  `TestThePanelListsEveryPhraseWithARemove`, `TestARefusedPhraseKeepsTheWordsAndSaysWhy`,
  `TestAnAddedPhraseClearsTheBoxAndSaysSo`, `TestThePageSendsPhrasesAndNeverPositions`,
  `TestThePhrasePanelBelongsToTheRoutineItIsAbout`.

**And the corrections: 11 of 13 mutations killed.** Two wrote better tests —
`TestAReplacingMoveClearsThePhrasesOfWhatItReplaces`, because on a move to a FREE identity
`routes.Create` clears the residue on its way past and the branch only matters on the replacing
path, and `TestANameDecidesTheQuestionWhateverAnswersToItByPhrase`, because with ONE phrase holder
the counting reaches the same answer either way. One survivor is `askedAbout`'s name-precedence
arm, which no test can see removed: a sole canonical name always claims its own label, so the
offline arm answers before that guard is reached. It is written down in the function as defensive,
with the reason it is kept. And the Play-ordering claim turned out to belong in a test rather than
a mutation record: `TestAThrowingPhrasePanelLeavesPlayAlone` renders a page whose phrases binding
is MISSING, so it throws exactly where the five failures came from, and asserts Play survives it.

**32 of 33 mutations killed** on the first delivery. Three survived and each wrote a better test:
the read-failure branch (the existing case was a parse failure, which is a different branch), the
Studio refusal path (the case it was asked about is one the contract SAYS rather than refuses, so
it never took the error path), and the panel's ownership guard, which needed an answer still in
flight to have a window at all. The remaining survivor is `drawAnswers`'s own id check, which the
redraw hook already enforces — written down in the page as a second statement rather than left
looking like an untested branch.

### And one thing the page tests gained

`TestTheAnswersPageDoesNotThrow`. Writing the harness produced a window where every counter read
zero, which is indistinguishable from a panel that does not work; it took three runs to find that
the harness declared `listed`, a name the page already declares, which made the page's whole
script a SyntaxError. The harness now keeps every error the page throws and asserts there were
none.

That turned out to matter beyond this file. The phrases panel's read was called from
`renderRoutine` BEFORE the block that decides which window Play presses in, and every existing
page harness stubbed `Routines` without it — so ten harnesses rendered a page that threw mid-render
and **five Play and target tests went red on a panel that has nothing to do with running
anything**. The gate caught it. The read is now the LAST thing `renderRoutine` does, because a
secondary panel does not belong upstream of the one control that acts, and every harness has the
binding.

## Amended before closure

Four corrections, found by review of the source rather than by anything failing — which is the
part worth recording, because a green gate said nothing about any of them.

1. **The door resolved a phrase, and picked between namesakes.** Covered above; the branch is
   gone and ambiguity has one owner.
2. **A move lost the phrases.** §4 of this ADR and of the contract both said a move writes them at
   the destination and verifies them before removing the original. `Rescope` copied the recording
   and the step account and did neither — and then `DeleteLeftovers` removed the original's file,
   so a move silently took away every other way of reaching a routine with nothing left to recover
   them from. **A rule stated in a table and absent from the code is the shape of defect this
   repository keeps finding**, and the table was written by the same hand that skipped the row.
3. **A fresh routine inherited leftover phrases.** The binding is to an identity, and an identity
   is a PLACE: a phrase file left by a sweep that could not remove it is orphaned while nothing is
   saved there, and matches again the instant something is. `Create` clears it.

   **And the first fix put the clear in the wrong place, which took a second review to catch.** It
   cleared AFTER publishing the source, reasoning that the hazard begins when the source appears.
   That is true, and it is the argument for clearing FIRST: while there is no source there is
   nothing to inherit, so clearing first closes the window where clearing afterwards opens and
   shuts it — and a resolver reading inside it offers somebody else's phrase for a routine that
   has just appeared. The removal's error was also ignored, so a readable file Windows would not
   let go of became the new routine's phrases and `Aliases` reported it `intact`: the single
   outcome this file exists to prevent, arriving through the one path that is supposed to prevent
   it. The clear now runs before the source, under the write lock both callers already hold, and a
   failure REFUSES with the file named. Avoiding a refusal cannot buy a routine that answers to a
   phrase nobody gave it.

   **And then the clear ran before the source existed was checked, which took a third review.**
   `O_EXCL` is what makes a creation exclusive and it answers LAST, so calling `Create` at an
   identity that already had a routine cleared THAT routine's legitimate phrases and then refused
   with "already exists" — a refused save that had changed the library, which is the one thing a
   refusal may not do. And it is the path rather than a corner of it: `Keep` reaches `Create`
   exactly when its own collision check found the place free and something appeared since, which
   is the race `O_EXCL` exists for. The source is asked about FIRST now, under the write lock the
   callers hold: an existing routine is `fs.ErrExist` with nothing written and nothing removed, and
   only a DEFINITE absence proceeds — the two refusals cover for each other, which is fail-closed
   and deliberate, because the question is not "does it exist" but "is it certainly not there".
   `O_EXCL` stays, and testing it needed the one instant it matters in: a writer that does not take
   the lock, standing in through the same `beforeSource` seam.
4. **A phrase could drag another routine's canonical name into ambiguity.** Names and phrases were
   counted together inside Director, so the slug-wins rule held everywhere except the one place
   that chooses. They are counted apart now.

## What this does NOT establish

That anybody has reached a routine by one of its other names **on a desktop**. Every case above is
a phrase resolving to an identity, and the press afterwards is the same press it already was,
through the same door, with the same guards. Studio has not been opened on a screen either: the
page tests render it headless against a stub, and the window the owner has open predates all of
this.
