---
type: adr
status: accepted
date: 2026-10-24
decision: "ACCEPTED — ROUTINES LIVE IN ONE LIBRARY, FOUND THE SAME WAY BY EVERY BINARY FROM ANY LAUNCH LOCATION, AND A SAVED ROUTINE IS SELECTED AND PLAYED IN THE WINDOW IT WAS RECORDED IN. The library was `$MARCO_ROUTES, else \"routes\"`, derived independently in four places, and that is a RELATIVE path: the answer was whichever directory each process happened to be started from, so a routine saved by a Studio somebody double-clicked in `plugins\\studio` went into `plugins\\studio\\routes` and `marco do` run from the repo root reported no such routine. The documented remedy was to launch both from the same directory, which is a developer workaround and not a product. So there is one resolver — `internal/marcopaths` — the answer is ABSOLUTE always, including a relative `$MARCO_ROUTES`, and the default is `routes` inside the store every part of Marco already shares, because a library is a place and not a thing you are standing next to. Moving a default makes an old library go quiet rather than disappear, so the window names any library-shaped directory beside its launch location that holds routines and says what to do about it; nothing is adopted or moved, because choosing between two libraries is the person's. AND SAVING IS NO LONGER THE END OF THE FLOW: a routine is CHOSEN AND PERFORMED BY ITS WHOLE IDENTITY — `<app>/<scope>/<slug>`, its place in the library — through card selection, preview and Play, because a name is not unique and a slug is not either, and every lookup that travelled by name answered with the first row that matched: clicking the second of two `settings` cards displayed and performed the first one, with the full identity reaching the child correctly AFTER the wrong routine had been chosen. The backend requires an exact match and refuses a bare name rather than resolving it, and both listings are keyed by identity rather than by slug — keyed by slug, a context-scoped routine that cannot press anything disabled the valid focus-scoped routine of the same name and showed it that one's reason. The window lists every saved routine with the contract it was saved under — legacy, guarded or incomplete, read off the saved Marco by one reader tied by test to the terms the press guard requires — refuses Play for the ones that cannot press anything and says what they lack, raises the application itself because only the foreground process may, and then spawns `marco do --play` for the route's whole identity with the library named explicitly. Studio acquires no execution loop: the answer is the published outcome word read through the package that owns it, one run slot is shared with Acts and with recording, and a routine's run is rendered as what it is — no canonical reference, no phase, no question — because a view that expected a supervised performance would report a perfect routine as a run that cited nothing."
supersedes: []
affects:
  - demonstrations
  - recorder
source_paths:
  - internal/marcopaths
  - internal/orchestrator/draft.go
  - plugins/studio/routines.go
  - plugins/studio/acts.go
  - plugins/studio/build.cmd
  - plugins/studio/frontend/index.html
  - cmd/marco/assistant.go
  - cmd/director/learnedplay.go
---

# A library is a place, and a routine is played where it was made

Two gaps between a working mechanism and a product somebody can use, both found by asking what
the owner would actually do rather than by reading the code that does it.

## The library was wherever you were standing

`marco`, `director` and Studio each resolved the route store in their own words, and the words
agreed:

```go
dir := os.Getenv("MARCO_ROUTES")
if dir == "" {
	dir = "routes"
}
```

Four copies of one sentence, which is the arrangement a duplicate is usually criticised for. The
defect is not the duplication. It is that the sentence is **relative**: `routes` means a different
directory for every process depending on where it was started.

So the product had as many libraries as it had launch locations. A Studio double-clicked in
`plugins\studio` — which is where it sits, and what a person does with an executable — kept its
routines in `plugins\studio\routes`. `marco do "ark graphics"` run from a terminal at the repo
root looked in `marco\routes`, found nothing, and said there was no such routine. Both were
behaving exactly as written.

The walkthrough for the trial carried the remedy as an instruction: *launch both from the repo
root.* That is a developer working around their own tooling. Double-clicking Studio has to reopen
the same library.

**So there is one resolver and the answer is a place.** `internal/marcopaths` answers where
anything of Marco's lives, and `Routes()` is `$MARCO_ROUTES`, else `routes` inside the store —
absolute either way. A relative `$MARCO_ROUTES` is resolved too, because that is the same hazard
wearing the person's own clothes: `MARCO_ROUTES=routes` set in one shell and inherited by a child
started elsewhere is two libraries again.

The store rule went the same way for a sharper reason than tidiness. It was written out four times
as well — once with a comment saying it *"has to stay the same rule"* — and the library is now
INSIDE the store, so two binaries disagreeing about the store is two binaries with two libraries.
One function, four callers.

### Moving a default makes the old one go quiet

Nothing is lost by this change and nothing says so, which is the worse of the two. Every library
saved under the old rule is still sitting in a `routes` folder beside whichever directory its
binary was started from; it is simply not read any more. A person opens the window, reads an empty
list, and concludes their work is gone.

So the thing that went quiet says its name: the window reports which library it is reading, and
when there is a library-shaped directory near it holding routines, it names that directory and
the count and says what to do — copy them in, or name it with `$MARCO_ROUTES`.

NEAR, not beside. A program somebody double-clicks is handed the folder it sits in as its working
directory, and a Studio in `plugins\studio` has no library beside it either way — so both that
folder and the one the binary lives in are searched, each upwards as well, as far as three levels.
The first version looked only alongside the working directory and was therefore silent in exactly
the case it was written for.

**Nothing is adopted and nothing is moved.** Deciding which of two libraries is somebody's is not
a decision code gets to make on their behalf, and a silent migration is the same defect as a
silent default with the blame moved.

## Saving was the end of the flow, and it should not have been

The recorder-first path reached disk and stopped there. The only way to run what had just been
saved was to leave the window, open a terminal, and type `marco do "ark graphics"` — which proves
a Studio-recording-to-CLI path and is not what anybody would call Record → Stop → Review → Save →
Play.

The Act surface next door does not close it either, and it is worth saying why rather than
treating the two as near enough. An **Act** is composed from a demonstration Director watched, is
pinned to that demonstration's revision, names its interactions in the person's own words, and is
performed by the supervised runner under an authorisation written at the press of Play. A
**routine** is a `.marco` file in the library that presses what somebody showed it. They are
different things, made differently, performed by different machinery, and a person who recorded
one does not find it in the list of the other.

**So the library is a list and Play is a button.** Choose a routine, read what it says about its
own presses, press Play.

### What the window does, in order

1. finds the routine, and refuses NOW if it would not run — before anything is raised
2. resolves the WINDOW it is to press in, and refuses without one
3. brings THAT WINDOW forward, because only the foreground process may
4. spawns `marco do --play=<slug>` with the route's whole identity, the library named, and the
   window as `--window=<handle>:<owner>`
5. reads the answer off the published wire line

Steps 2 and 4 are later than this decision and SUPERSEDE part of it. It said the window raises
"its application", which is what it did — and a program can own several windows, so the one
this surface raised and the one the child then bound were two different windows whenever
somebody had two. A routine naming no application had nothing raised at all, so its run began
with Studio in front. See [[ADR-272-the-window-a-run-acts-in-is-chosen-and-carried]].

Step 2 is the same split [[ADR-244-pressing-play-is-the-authorisation]] already draws for an Act
and for the same measured reason: Windows refuses `SetForegroundWindow` to a process that does not
already own the foreground, and the child is a process nobody is looking at. Studio owns the
foreground at the instant somebody clicks a button in it, and that is the one moment it may hand
it on. Nothing simulates a keypress to get around the lock.

### Why the run is a child and not a library call

Because `marco do` is the entrance, and everything a run of a routine needs is already behind it:
the authority door, the one cross-process stop, the guarded contract's account of what did not
happen ([[ADR-251-a-press-checks-the-window-it-was-recorded-in]]), the seven-word outcome, and the
exit code that carries it.

Studio must not acquire an execution loop of its own. A second place that decides what a routine's
result was is a second answer to "did that work", and not having two of those is most of what
ADR-251 is about.

Reading the child's line is **protocol, not transcript-parsing**. The outcome word is a closed set
of seven owned by `internal/outcome`, which both ends import — the same arrangement
`plugins/overlay` already has with the same binary. A surface that matched on wording would be a
surface whose verdict changed when somebody improved a message, and that is the thing this is not.

### The route's WHOLE identity, and the library by name

`--play` alone resolves to whichever scope the registry finds first, and a route IS its scope and
its slug: the same name can be saved twice. So the application and the focus flag travel with it.

And `$MARCO_ROUTES` is set on the child explicitly, even though both processes now resolve it the
same way. The window knows which library it listed the routine FROM, and that is the one the run
must read. Relying on a child to work it out again is how the first half of this ADR happened.

### A routine's run is rendered as what it is

One run view, two kinds of run — because a second panel showing a transcript and an outcome is a
second panel to drift. What differs is said rather than inferred: a routine's run carries no
canonical reference, publishes no phases, cannot pause and is never waiting on an answer. A view
that expected a supervised performance would have told somebody their perfectly good routine
*"cited no reference, so there is no canonical account of it"*, and the trial journal would have
filed it as a run that never finished writing one. Both now say which kind they are about.

And the run's own last sentence is in the banner beside the verdict. The word is what happened and
the sentence is why; `performed` makes no claim about the application, because nothing on this
path reads the screen afterwards. For a refused guarded press the sentence names the press that
could not be checked, which is the whole finding — it was carried as far as the function that
writes the banner and then dropped there, with the page rendering the word and the transcript and
nothing in between.

### A routine is chosen by its whole identity, not by its name

A name is not unique and a slug is not either. `settings` saved for two applications is two
routines, and so is `settings` saved twice for one application under two scopes — a route IS its
scope and its slug, which is why the command is told all three.

Everything above that travelled by NAME. The page kept `r.name`; looking a routine up returned the
first row that matched; `PlayRoutine` matched a name or a slug and took the first of those. So
clicking the second `settings` card displayed the first one's saved Marco, the first one's sentence
about its presses and the first one's Play button — and pressing it performed the first one's
route. **Handing the child the route's full identity was already right, and it was happening after
the wrong routine had been chosen**: the shape this project keeps finding, a check that is correct
about the wrong subject.

The listing had the same defect in a second place and with a worse result. "Cannot press anything"
was keyed by SLUG, and a slug is shared by every scope it was ever saved under — so
`<app>/context/settings` being unable to press anything marked `<app>/focus/settings` unable to
press anything too. A valid routine disabled by an invalid namesake, with the namesake's sentence
beside it explaining a scope it is not in.

And the repair card had it a third time: `Resave` matched a name and took the first that answered,
so the button beside one row saved the other row's routine and reported the first one's name.

So every one of them carries `<app>/<scope>/<slug>` — the route's place in the library, which is
what a route's identity is. The page keeps it through selection, preview and Play; the backend
requires an exact match and refuses a bare name rather than resolving it, because a term that
identifies two routines must not pick one of them; and both listings are keyed by it.

### Three contracts, read before Play rather than discovered during it

A saved route says one of three things about its presses — no basis is legacy, a basis with every
term is guarded, a basis with a term missing is incomplete and refuses — and which it is does not
depend on a measurement having succeeded. That is ADR-251's rule, and the list is where a person
meets it: the routine that checks every press says so, the one that cannot press anything is not
offered and says why, and the one missing a measurement names what it lacks.

`orchestrator.ContractOf` is the one reader, and it is **tied by test to the guard that refuses**.
Six terms are required at the press by `internal/oshost`; a route this called complete and the
guard then refused would be a listing that contradicts the run. So the test generates a guarded
route through the real generator, removes one term at a time, and requires every removal to change
the answer — and checks the list's length, because a reader that quietly forgot a term would pass
every remaining case.

### And the marco beside the window

Pressing Play spawns the `marco.exe` beside Studio, which is the trap CLAUDE.md already carries a
paragraph about for `director.exe`: two copies of Studio, and a sibling beside only one of them is
a current window running yesterday's binary — or no binary, and then the failure reads as a
routine that cannot run. `build.cmd` makes all of them in one breath.

## And saving does not replace a routine unless somebody says so

`Registry.Save` is a plain file write. Keeping a recording under a name already in the library
therefore replaced what was there — the source, and a moment later the demonstration beside it,
which is the copy a routine is ever saved again from. Nothing asked, nothing said, nothing kept.

The person most exposed is the one re-recording all afternoon to compare two attempts, so the
likeliest casualty is the routine they were comparing against. That is not a hypothetical shape of
accident: it is the shape the product invites as soon as it is pleasant to re-record.

**The policy sits at the Studio boundary and nowhere lower.** The orchestrator, the overlay and
`marco learn` overwrite ON PURPOSE — teaching a name again is how you correct one there, and that
is their contract rather than an oversight. Pushing a refusal down into `Deps.Save` would change
what every other caller was written against, and the mutation that does so kills the test holding
their behaviour as well as the one holding the repair path, which is a deliberate replacement too.

**Three ways in, one place that writes.** Ordinary Save refuses a collision, naming the routine
that is there and when it was kept. Save as keeps both, and refuses a collision of its own,
because the other name can be taken too and saving over THAT one is the same loss by a longer
route. Replace is explicit and carries the identity somebody agreed to replace.

**And the identity is the generator's own.** `orchestrator.Deps.Target` is the route a draft WOULD
be saved as; `Save` calls it and so does the collision check, so the question and the write cannot
disagree. Deriving it a second time in the window would be a second answer to which routine this
is — the shape of defect this repository keeps finding — and the mutation that drops the scope
from that derivation kills five tests, because a collision looked for in the wrong place is no
protection at all.

**The window's earlier look is not what protects the library.** `Collision` exists so the page can
offer the choice before anybody presses anything, and every save asks again at the save. Between
the question and the answer somebody can save that name from a terminal, and a window trusting its
own earlier look would overwrite at exactly that moment. Replace also checks that the identity it
would write is the one it was told to replace: agreeing to replace one routine is not agreeing to
replace whatever is at the target by then.

**The ordinary button cannot reach a replacement.** It asks for a save and nothing else — it does
not look first and then decide for somebody. Only once the backend has refused does the page ask
what is there, so it can show the two ways on. A page that looked and then called the replacement
itself would pass every backend test while doing the one thing they exist to prevent, which is why
that wiring is held in a browser.

No version history, no automatic renaming, and nothing to do with generation-contract versioning,
which stays separately scoped.

## Enforced by

- `internal/marcopaths` — `TestTheLibraryIsTheSameFromAnyDirectory` (the defect itself: two
  working directories, each with a `routes` folder in it, one answer),
  `TestTheLibraryIsAnAbsolutePath`, `TestTheOverrideNamesTheLibraryAndIsResolvedOnce`,
  `TestTheStoreCarriesTheLibrary`, `TestTheDefaultStoreIsNotTheOneTheEnvironmentNames`,
  `TestALibraryLeftBesideTheLaunchDirectoryIsReported`,
  `TestALibraryAboveTheLaunchDirectoryIsStillFound`, `TestTheSearchDoesNotClimbForever`,
  `TestEveryPlaceGivenIsSearched`, `TestTheLibraryIsNotReportedAsSomewhereElse`,
  `TestAnEmptyDirectoryIsNotSomebodysLostLibrary`
- `cmd/marco` — `TestThisCommandReadsTheOneLibrary`, `TestTheLibraryCanStillBeNamedOutright`
- `cmd/director` — `TestTheDirectorReadsTheOneLibrary`
- `internal/orchestrator` — `TestEveryTermTheGuardNeedsIsOneThisReads`,
  `TestTheTermsReadAreTheWholeList`, `TestARouteWithEveryTermIsGuarded`,
  `TestARouteThatStatesNoWindowIsLegacy`,
  `TestAnOrdinaryPressBesideAGuardedOneIsNotAMissingMeasurement`
- `plugins/studio` — `TestASavedRoutineIsListedAndReadyToPlay`,
  `TestAnActIsNotListedAsARoutine`, `TestAModuleBesideTheRoutinesIsNotOfferedAsOne`,
  `TestARoutineThatCannotPressAnythingIsNotOfferedForPlay`,
  `TestARoutineMissingAMeasurementIsNotOfferedForPlay`,
  `TestPlayingAnUnknownRoutineNamesTheLibrary`, `TestPlayingARoutineByNameAloneIsRefused`,
  `TestTwoRoutinesOfOneNameAreTwoRoutines`,
  `TestAnUnrunnableNamesakeDoesNotDisableTheValidRoutine`,
  `TestSavingOneOfTwoNamesakesAgainRepairsTheOneAskedFor`,
  `TestARoutineWillNotRunWhileSomethingElseIs`,
  `TestARoutineWillNotRunWhileSomethingIsBeingRecorded`,
  `TestTheChosenWindowIsBroughtForwardBeforeTheRunStarts`,
  `TestARoutineThatNamesNoApplicationIsPlayedIntoAChosenWindow`,
  `TestARoutineWithNowhereToPressIsNotPlayed`,
  `TestTheCommandCarriesTheRoutesIdentityAndTheLibrary`,
  `TestAContextRoutineIsNotAskedForAsAFocusOne`, `TestTheAnswerComesFromTheRunsOwnWord`,
  `TestARefusedPressReachesTheWindowWithItsReason`,
  `TestARunThatAnnouncedNothingIsNotAnAnswer`,
  `TestAWordThatIsNotOneOfTheSevenIsNotAnAnswer`, `TestARoutinesRunSaysItIsOne`,
  `TestTheTrialJournalRecordsARoutinesRun`, `TestTheWindowSaysWhichLibraryItIsReading`,
  `TestALibraryLeftBesideTheLaunchDirectoryIsNamed`,
  `TestPlayingARoutineRunsItAndReportsWhatItSaid`,
  `TestEveryMethodThePageCallsIsOnTheBackend`,
  `TestTheSavedRoutinesAreDrawnAndPlayIsOfferedForTheOneThatCanRun`,
  `TestTheBuildScriptMakesTheWholeStackFromOneCommit` (since 2026-11-04, replacing the earlier director-and-marco build-script test),
  `TestTheTrialNoteShapeMovesWithItsVersion`
- **And that saving does not replace a routine unless somebody says so**: `plugins/studio`
  `TestSavingOverARoutineIsRefusedAndLeavesItAlone` (the refusal names what is there, and the
  BYTES of both the source and the demonstration are unchanged — a refusal that returned a
  sentence and wrote the file anyway would pass a test that read only the sentence),
  `TestSavingUnderAnotherNameKeepsBoth` (both routines, the first untouched, and an alternative
  name that is also taken refused), `TestReplacingChangesOnlyTheOneChosen` (the same name in two
  applications; the namesake is byte for byte what it was),
  `TestReplacingSomethingElseIsRefused` (both ways the window's earlier look can be stale by the
  save: an identity this save would not write, and a target that has gone), and
  `TestTheOrchestratorStillOverwritesForEverybodyElse` — the half that keeps the policy confined
  to this window.
- `plugins/studio` `TestOrdinarySaveNeverReplacesAndTheChoiceIsPressed` — in a browser, over a
  backend whose Save refuses: pressing the ordinary button must not have reached `ReplaceDraft`
  or `SaveDraftAs`, the choice must be shown, and pressing Replace must carry the identity the
  BACKEND named. A page that looked first and replaced for somebody would pass every test above.

## Mutations

Forty-eight across two rounds, with one deliberate no-op control, plus seven for replacement
protection, all killed, with a second no-op control.

**The second round** is the identity, after review found that selecting a routine lost its scope:
eight mutations, one survivor. `Resave` was made to accept a name as well as an identity and
nothing failed, because the test handed it an identity and never asked what a name would do —
closed by requiring a name to repair nothing. One mutation is recorded as unfalsifiable rather
than killed: the listing sorts with a tiebreak on identity, and `Registry.List` already returns a
total order, so the sort never sees equal keys and removing the tiebreak changes no observable
behaviour. It stays because the order must not depend on an upstream sort being total, and the
comment on it says that rather than claiming a defect anybody saw.

**The first round** was forty, with six survivors, five of them a weak test rather than weak
code: the child's environment was asserted to CONTAIN the library, which
`os.Environ()` already satisfied whenever the two agreed — and in a test they agree by
construction; the module filter was tested with `os.marco`, which `routes.routeSlug` already
excludes by name everywhere, so the fixture asked nothing; nothing tested that the window brings
the application forward, which is what the foreground lock makes load-bearing; nothing read what
`-trial` prints, so the branch that prints a routine's run could be deleted whole; and the refusal
sentence was asserted from a two-line fixture that could not tell the first sentence from the last.

The sixth was a defect in the page. **Two places decided whether Play was live** — the click and
the poll — and the poll's copy overwrote the click's four hundred milliseconds later, so making
the click's half always-enabled changed nothing and survived its own test. A duplicate whose later
writer silently wins is the arrangement that makes a test like that unfalsifiable; one function
decides now, asked from both.

## Known gaps

- **A routine's run cannot be dry.** `marco do` has no such switch; the dry path is
  `marco run --host dryrun <file>`, which performs a file rather than a routine. The button says
  it presses for real rather than offering a tick-box the command could not honour.
- **The end-to-end spawn test skips without a marco binary beside the module.** The two halves it
  would cover — what the window hands the command, and what it makes of the answer — are tested
  unconditionally, which is why this is a gap and not a hole.
- **Nothing migrates an old library.** It is named and counted and left alone.
- **A routine's run writes no run record**, so `Report a problem` and `Inspect` have nothing to
  read for one. They are disabled, which is correct and not yet explained in the window.
