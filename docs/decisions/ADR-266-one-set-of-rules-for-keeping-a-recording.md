---
type: adr
status: accepted
date: 2026-10-28
decision: "ACCEPTED — ONE SET OF RULES FOR KEEPING A RECORDING, AND EVERY SURFACE OBEYS THEM. Studio gained drafts and the consolidation that should have come with them did not: `Begin` → `Finish` → `Draft` → `Save` was written for the window, and `marco learn` and the widget kept a second implementation of the same pipeline with none of the rules. Both divergences were MEASURED rather than reasoned about. Learning a name twice REPLACED the first routine — the library kept one routine, the file held the second recording, and the line printed said `Learned` as though it were a fresh save; re-recording is what somebody testing a routine does all afternoon, which makes the likeliest casualty the routine they were comparing against. And answering \"only here\" or \"anywhere\" for a recording whose presses check their own window SAVED A ROUTINE THAT REFUSES EVERY PRESS, which `Deps.Unrunnable` — the same reader Studio greys the Play button with — reported immediately. So the enforcement moved DOWN into `orchestrator.Keep`: whether a scope word means anything, whether this draft can be saved under the scope asked for, what is already where it would go, and whether a replacement is the one that was agreed to. Each surface supplies only the INTERACTION. Studio shows a card that offers Replace; `marco learn` names the exact application, scope and route and asks, with ENTER MEANING NO because the two answers do not cost the same — no costs a second attempt and yes costs a routine and the demonstration beside it, neither of which comes back; and the widget's unattended path REFUSES a collision outright, names what is there and preserves it, because it saves with nobody having looked and a silent overwrite there is the worst of the three. A REPLACEMENT IS CONSENTED TO BY VERSION AND NOT BY PATH: a routine{Q}s identity is its place in the library and a place does not change when its contents do, so "the same route is still there" was true of one that had been saved again while the question was open and that newer version was overwritten. Consent carries digests of the source AND the stored demonstration as somebody was shown them, re-read at the instant of writing — the demonstration because a routine re-recorded from the same steps has the same source and a different recording, and the recording is the only thing a rebuild can be made from. AND A FRESH SAVE IS EXCLUSIVE: `Keep` asked whether the path was free and then wrote with create-or-truncate, so another writer landing in the gap was overwritten by a caller that had been told the path was free. `routes.Registry.Create` uses `O_CREATE|O_EXCL`, which closes the gap rather than narrowing it, and a replacement goes through `Registry.Replace`, which is atomic. `Collision` treated every `Stat` error as absence and now treats only not-found that way. AND THE VERSION COMPARISON IS INSIDE THE CRITICAL SECTION, because atomic writing prevents a half-written file and does not make the comparison atomic: reading the digests, rendering and writing are three operations, and another save landing between the first and the last has its version overwritten by a caller whose comparison was true when it was made. So the route is HELD across the whole of validate-and-write — the version check, the source and the demonstration — and the fresh branch takes the same hold, because serialising only the replacements would leave a create and a replace of one route racing. It is `LockFileEx` rather than a lock file of our own, because a lock file has to be cleaned up by whoever took it and the one case a lock is for is the case where that does not happen; Windows drops a handle lock when the process ends however it ends, so there is no stale state to recover and no deadline to tune. The rendezvous file sits BESIDE the route, because one lock per slug would make a save of `notepad/context/settings` wait for `notepad/focus/settings` and then refuse. Off Windows it is a goroutine lock and the file says plainly that it is not cross-process. AND EVERY WRITER IN THE CONTRACT TAKES IT, which is participation rather than mechanism: `Deps.Replace` — the path behind Studio{Q}s Simplify and its generation rebuilds — read its two digests, classified the anchor images and wrote the source OUTSIDE the lock, so a `Keep` holding the route could be committing while that rebuild validated and the rebuild would write over what it committed. A lock protects the writers that take it, and a writer that does not take it is not protected by anybody else taking it. It takes the hold before the first digest read and keeps it through the images and the source. A scope the recording cannot be performed under is refused before anything is written: the terminal is not offered it and is told why, and the unattended path asks the draft what it supports instead of defaulting to the application's in-place scope. THE CAPTURE POLICY IS PRESERVED DELIBERATELY AND IS NOT PART OF THE SHARING: Studio states coordinate mode to the recorder and the terminal and the widget state nothing, which leaves the hooks doing what the environment says — the behaviour every caller before Studio had — so a saved routine from those paths still states no mode, truthfully. The owner-strip rule is the same kind of difference and is now a stated field rather than two copies of a pipeline: everywhere for a window, which is glanced at, and off the tail of a button-ended recording for everything else, which is what `TestAGestureEndedRecordingIsNotStripped` was written to record. `saveTaught` and `chooseScope` are RETAINED for `LearnVoice`, which is out of this slice: a helper whose deletion reaches another caller is kept rather than taken along for free. 13 of 13 mutations killed, held through the production entrances rather than through `Keep` alone — a test of the rule that did not enter where a person does would have passed for the whole time the rule was missing."
supersedes: []
affects:
  - execution
source_paths:
  - internal/orchestrator/keep.go
  - internal/orchestrator/regenerate.go
  - internal/routes/lock_windows.go
  - internal/routes/registry.go
  - internal/orchestrator/orchestrator.go
  - internal/orchestrator/draft.go
  - plugins/studio/replace.go
---

# One set of rules for keeping a recording

## The consolidation that should have come with drafts

When Studio gained the recorder-first path it gained something the product did not have: a
**draft**. `Begin` starts the recorder, `Finish` makes a `Draft` and **writes nothing**, and
`Save` is a separate act — which is what makes Stop, Cancel and Save three things rather than one.
Around that, Studio grew the rules a save has to obey.

`marco learn` and the widget kept the implementation they already had: start the recorder, wait,
simplify, write. The same pipeline, written twice, and only one copy had the rules.

Both halves of the divergence were **measured**, not argued about.

## What the measurements found

**A collision overwrote, silently.** `marco learn "the same name"` twice, with different
recordings: the second replaced the file, the library still held one routine, and the output said
`Learned …` as though it were a fresh save. `saveTaught` went straight to `Reg.Save`, which
truncates. Re-recording is what somebody testing a routine does all afternoon, which makes the
likeliest casualty the routine they were comparing against.

**Two of the three scope answers produced a routine that refuses every press.** With a
window-checking click recorded, answering `c` (only here) or `g` (anywhere) both saved, and
`Deps.Unrunnable()` — the same production reader Studio greys the Play button with — reported the
routine that had just been written. Only `f` was sound. Studio refuses `c`/`g` with the sentence
naming the remedy; `chooseScope` took them without comment.

One thing expected and wrong: `LearnAuto` **does** refuse an unrepresentable gesture. Not a gap,
and worth recording as a thing that was checked.

## The rules are shared; the interaction is not

`orchestrator.Keep` is the one enforcement: a scope word that means nothing, a scope this draft
cannot be performed under, what is already where it would go, and whether a replacement is the one
that was agreed to. Every surface goes through it. What each one *says* is its own:

- **Studio** shows a card with the sentence and a Replace button naming one routine.
- **`marco learn`** names the exact application, scope and route — `notepad/context/the-same-name`
  — and asks. **Enter means no.** The two answers do not cost the same: no costs a second attempt,
  and yes costs a routine and the demonstration beside it, neither of which comes back. A person
  pressing Enter through a prompt they did not read must not lose work by it, which is why this is
  a second confirm function and not a flag on the existing one.
- **The widget's unattended path refuses.** Record, F12, saved — no prompt and no review. Of the
  three this is the one where a silent overwrite is worst, so it is the one that will not do it at
  all: the existing routine is named, left exactly as it is, and nothing is written.

**A replacement is consented to by VERSION, not by path.** A routine's identity is its place in
the library, and a place does not change when its contents do — so the first version of this
checked that the same route was still there, which catches it being DELETED while the question is
open and misses it being REPLACED. Another save landing there left a newer routine at the identity
somebody had agreed to overwrite, and it was overwritten. They agreed to lose the version they
were shown.

So `Consent` carries digests of the source **and** the stored demonstration as they were when the
question was asked, and `Keep` reads both again at the instant of writing. `Deps.Replace` already
worked this way for a regeneration; this is the same rule for the other way a routine gets written
over. The demonstration is half of it rather than an extra: a routine re-recorded from the same
steps has the same source and a different recording, and the recording is the only thing a rebuild
can be made from — a mutation that checked the source alone survived the first gate run, which
is how that became a case. Studio's page hands back what it was SHOWN, which is what the digests
on `Collision` are for.

Both tests interleave for real: the reader deletes the routine, or saves over it, as the `yes` is
read.

**And a fresh save is exclusive.** `Keep` asked whether the path was free and then wrote with
create-or-truncate, so another writer landing between the two was overwritten by a caller that had
been told the path was free. The gap is as long as whatever happens in between, which on an
interactive surface is as long as somebody takes to answer. `routes.Registry.Create` uses
`O_CREATE|O_EXCL` — the test and the write are one operation, so the gap is closed rather than
narrowed — and a replacement goes through `Registry.Replace`, which is atomic, for the one case
where there is an original to lose.

It is tested twice on purpose, because the two gaps are different sizes. The production-path test
drives the realistic window and the collision check catches it there. The gap between `Keep`
looking and `Keep` writing has no prompt inside it and no seam to stand in, so that one is
**raced**: 240 iterations with another writer creating the target, 193 of which it won, asserting
in each of those that its bytes and its demonstration are untouched. It cannot fail wrongly,
because the assertion is only made about an iteration where their create succeeded.

**And the collision check is no longer what protects anything**, which is said in the code so that
nobody reads it as the thing holding the line: deleting it changes no behaviour, because the
exclusive create refuses the same case and reports it with the same sentence. Its gate run is
recorded as redundant rather than as a kill. It stays because it is where the sentence comes from
one syscall earlier, and because a surface offering Replace needs the refusal before it has tried
to write.

**And only not-found means nothing is there.** Every `Stat` error used to read as absence. That
branch cannot be reached by anything a test can arrange on Windows, and both arrangements were
measured: a directory in the routine's place makes `Stat` SUCCEED, and a file where a parent
directory should be reports path-not-found, which `os.IsNotExist` answers true for. So the guard is
correct, its mutation is recorded as unfalsifiable rather than as a kill — the same standing as
`Watching.has`'s empty-path check — and the test is named for what it actually arranges.

**An impossible scope is refused before anything is written.** The terminal is not offered a scope
this recording cannot be performed under, and is told why — because a person who expected "only
here" needs to know why they are not being asked for it, and a surface that silently narrowed the
choice would be the one deciding. The unattended path asks the draft what it supports rather than
defaulting to the application's in-place scope.

## What is deliberately NOT shared

**The capture policy.** Studio states coordinate mode to the recorder; the terminal and the widget
state nothing, which leaves the hooks doing what the environment says — the behaviour every caller
before Studio had. Routing them through `Begin` preserves that exactly, because `recordedAs()`
answers `Unstated` unless `Deps.Mode` is `Coordinates`, and `Unstated` tells the recorder
`Inherited`. So a routine saved from those paths still states no mode, which is the truth about how
it was recorded. **This is shared orchestration and shared save rules, and it is not an identical
capture configuration.**

**The owner strip**, which is the same kind of difference and was nearly changed by accident. The
window's rule is that the owner comes out *wherever it is*, because a window is glanced at
mid-demonstration and the glance costs the click before it as well as the one that pressed Stop
(ADR-249). The terminal's rule is the tail of a button-ended recording only, so a recording ended
by the stop gesture keeps whatever it always kept —
`TestAGestureEndedRecordingIsNotStripped` exists to record that changing *that* would be a
separate decision rather than a tidy-up. Moving the two paths onto one pipeline would have flipped
it in passing; the suite caught it. It is now a stated field, `Deps.OwnerTailOnly`, defaulting to
the window's rule because that is what `Finish` has always done, with the two adapters asking for
the other one on a copy of their own `Deps`. In production the difference is unreachable for them:
`cmd/marco` sets no owner at all. It is preserved because it was stated, not because it is
load-bearing.

**`saveTaught` and `chooseScope` are retained.** Their one remaining caller is `LearnVoice`, which
is out of this slice. A helper whose deletion reaches another caller is kept rather than taken
along for free.

## And one loop that was bounded before it shipped

The first version of the terminal's save re-asked for a scope when the one it got was impossible.
`readLine` on a closed stdin answers `""` for ever, so a terminal whose input had gone would have
sat there asking itself the same question until somebody killed it. It refuses and returns
instead; `askScope` already declines to offer a scope the recording cannot use, so reaching that
refusal is a disagreement between the two and not a person's mistake.

## And the comparison is inside the critical section too

`Registry.Replace` makes the WRITE atomic, so nobody reads a half-written routine. That says
nothing about the gap before it. Reading the digests, rendering and writing are three operations,
and another save landing between the first and the last has its version overwritten by a caller
whose comparison was true when it was made and is not true any more. **Making a comparison atomic
is not something a file write can do.**

What closes it is holding the route for the whole of validate-and-write — the version check, the
source, and the demonstration — so two saves of one routine serialise and the second revalidates
against what the first actually left. The fresh branch takes the same hold: serialising only the
replacements would leave a create and a replace of one route racing each other, where now they
queue. The exclusive create would still refuse rather than overwrite — it is its own critical
section — and the lock is what makes the pair of them wait instead of one of them failing.

**The operating system's lock, not a lock file of our own.** A lock file has to be cleaned up by
whoever took it, and the one case a lock is for is the case where that does not happen: a process
killed mid-save leaves its file behind and every later save waits for a holder that does not exist.
The remedies are worse than the problem — a timeout long enough to be safe is long enough to be a
hang, and a stale-lock breaker is a second race with a second set of rules. `LockFileEx` is held
by a HANDLE, which Windows drops when the handle closes and therefore when the process exits for
any reason at all. No stale state, no deadline to tune. The rendezvous file is left on disk on
purpose: it holds nothing, so its presence means nothing and nobody has to tidy it — though
`Registry.Delete` sweeps it when the route goes, so a library does not accumulate litter.

**And every writer in the contract takes it.** That is participation rather than mechanism, and
it is where this was still short. `Deps.Replace` — the path behind Studio's **Simplify** and its
**generation rebuilds** — read its two digests, classified the anchor images and wrote the source
outside the lock. Three operations with two decisions in them: a `Keep` holding the route could be
committing while that rebuild validated, and the rebuild would then write over what it committed,
having compared against the version before it. Not a smaller gap than the one the lock closed —
the same gap, from the side that had not joined. It takes the hold before the first digest read
and keeps it through the images and the source, for the same reason `Keep` does: what has to be
atomic is the comparison and the write *together*.

It sits **beside the route**, keyed by the whole identity. A route is its scope and its slug, so
one lock per slug would make a save of `notepad/context/settings` wait for
`notepad/focus/settings` — a save queueing behind an unrelated one and then refusing. That came
out of a surviving mutation: all the locks in one directory passed every test there was, because
none of them used two routines.

**Off Windows it is a stub and says so.** There is no portable advisory lock in the standard
library and the engine has no dependencies to reach for, so the other build gets a real lock
between goroutines and not between processes. Stated in the file rather than implied, because a
lock that does not lock is worse than one that refuses.

## Enforced by

Through the **production entrances** — `Deps.Learn` and `Deps.LearnAuto`, the ones `marco learn`
and the widget call — rather than through `Keep` alone. A test of the rule that did not enter where
a person does would have passed for the whole time the rule was missing: both surfaces had `Save`
underneath them and neither reached these checks.

- `internal/orchestrator` `TestLearningANameTwiceAsksBeforeItReplacesAnything` — the first save,
  then Enter at the replace prompt leaving the file byte-identical, the question naming
  `notepad/context/the-same-name` and `[y]es / [N]o`, then an explicit yes replacing it and saying
  `Replaced`, with the library still holding ONE routine.
- `internal/orchestrator` `TestAutoLearnRefusesACollisionAndKeepsWhatIsThere` — the unattended
  path leaves the file unchanged, says what is there, returns no error (the recording happened;
  what could not happen is the save), and never says it learned anything.
- `internal/orchestrator` `TestAReplacementRefusesWhenTheRoutineChangedWhileTheQuestionWasOpen` —
  the routine is deleted *as the consent is read*, and the replacement is refused with nothing
  written in its place, because an ordinary save is a different thing nobody agreed to.
- `internal/orchestrator` `TestNoSurfaceSavesARoutineThatRefusesEveryPress` — a window-checking
  recording through the terminal for all three answers and through the unattended path:
  `Unrunnable` is empty every time, it lands where it can run, and the terminal is told why the
  other scopes were not offered. Plus the **control**: an ordinary recording still goes to
  whichever of the three scopes is asked for, without which every assertion above is satisfied by
  a surface that has stopped honouring anybody's choice.
- `internal/orchestrator` `TestTheTerminalStatesNoModeAndStillDoesNot` — the recorder is told
  `Inherited` on both paths and the saved routine states no mode, read back through `ModeOf`.
- `internal/orchestrator` `TestNothingIsWrittenWhenNothingIsKept` — declining the save, and
  demonstrating nothing: no routine, and nothing left in the library at all.
- `internal/orchestrator` `TestAGestureEndedRecordingIsNotStripped` — unchanged, and now the thing
  that holds `Deps.OwnerTailOnly` for the paths that asked for it.
- `internal/orchestrator` `TestAReplacementRefusesWhenTheContentsChangedUnderTheSameIdentity` —
  the source changed under the same identity as the consent is read, and then the DEMONSTRATION
  changed with the source untouched. Both refuse, and the other writer's bytes are intact.
- `internal/orchestrator` `TestAFreshSaveDoesNotOverwriteAConcurrentSave` — another writer creates
  the target while the scope answer is being read: their routine and their demonstration survive
  unchanged, and nothing claims to have learned anything.
- `internal/orchestrator` `TestWhenTheGapIsRacedTheOtherWritersFileSurvives` — the gap no prompt
  sits inside, raced 240 times. Only iterations where the other writer's create succeeded are
  asserted about, so it cannot fail wrongly.
- `internal/orchestrator` `TestAPathThatIsNotARoutineIsNotReportedAsFree` — a directory where the
  routine should be: occupied, not written through, and no demonstration left beside it.
- `internal/orchestrator` `TestAFocusPlayActivatesEvenWhenTheDemonstrationDidNot` — unchanged, and
  what caught `Keep` growing its own `Target` call without `Save`'s application fallback: a focus
  routine recorded from inside its own application came out with no Activate, which is a routine
  that types into whatever happens to be in front. There is one preamble now, shared, with a
  mutation on it.
- `plugins/studio` `TestAReplacementRefusesWhenTheRoutineChangedSinceTheCardWasDrawn` — the
  window's round trip: the card is drawn, somebody else saves over the routine, and the replacement
  is refused with the newer version intact.
- `internal/orchestrator` `TestTwoSavesOfOneRoutineSerialiseAndTheStaleConsentRefuses` —
  DETERMINISTIC rather than raced, which the fresh-save case could not be: `AfterValidating`
  stands in the one instant that exists — inside the lock, after the comparison, before the
  write — so the second save CANNOT proceed and nothing depends on timing to be true. Two saves,
  both with consent valid when they read it: the first pauses, the second must not get past the
  lock (asserted on the FILE, not the clock), the first finishes, and the second revalidates,
  finds its version gone and refuses with the first save's work intact. With the control — a
  replacement whose consent is current still goes through — which also proves every hold was
  given back, since a leaked one would have made it wait out `SaveWithin`.
- `internal/orchestrator` `TestARebuildWaitsForASaveAndThenRefusesStaleConsent` — the cross-path
  case, deterministic in the same way: `Keep` pauses inside its lock, a prepared rebuild is
  attempted with digests that are still entirely current, it cannot proceed (asserted on the FILE,
  not the clock), the save finishes, and the rebuild revalidates and refuses with the save's work
  intact. With the control — a rebuild prepared against what is on disk now goes through —
  whose assertion is that the file is the version that was REVIEWED rather than that it changed,
  because a rebuild of a routine already written under current rules can legitimately come out
  byte-identical.
- `internal/orchestrator` `TestHoldingOneRoutineDoesNotHoldItsNamesake` — `settings` in two scopes
  are two routines: holding one and saving the other finishes well inside the bound, and the held
  one is untouched.
- `plugins/studio` — its own save, replace and collision tests, now running through the shared
  `Keep` and handing back the version they were shown.

See [[Learned-Plays]], [[Plays]], [[ADR-249-ending-a-recording-is-not-part-of-the-recording]] and
[[ADR-265-say-what-you-want-and-studio-offers-the-routine]].
