---
type: adr
status: accepted
date: 2026-10-28
decision: "ACCEPTED — ONE SET OF RULES FOR KEEPING A RECORDING, AND EVERY SURFACE OBEYS THEM. Studio gained drafts and the consolidation that should have come with them did not: `Begin` → `Finish` → `Draft` → `Save` was written for the window, and `marco learn` and the widget kept a second implementation of the same pipeline with none of the rules. Both divergences were MEASURED rather than reasoned about. Learning a name twice REPLACED the first routine — the library kept one routine, the file held the second recording, and the line printed said `Learned` as though it were a fresh save; re-recording is what somebody testing a routine does all afternoon, which makes the likeliest casualty the routine they were comparing against. And answering \"only here\" or \"anywhere\" for a recording whose presses check their own window SAVED A ROUTINE THAT REFUSES EVERY PRESS, which `Deps.Unrunnable` — the same reader Studio greys the Play button with — reported immediately. So the enforcement moved DOWN into `orchestrator.Keep`: whether a scope word means anything, whether this draft can be saved under the scope asked for, what is already where it would go, and whether a replacement is the one that was agreed to. Each surface supplies only the INTERACTION. Studio shows a card that offers Replace; `marco learn` names the exact application, scope and route and asks, with ENTER MEANING NO because the two answers do not cost the same — no costs a second attempt and yes costs a routine and the demonstration beside it, neither of which comes back; and the widget's unattended path REFUSES a collision outright, names what is there and preserves it, because it saves with nobody having looked and a silent overwrite there is the worst of the three. A replacement targets the exact route consented to and is re-derived and re-checked at the instant of writing, so a routine deleted or saved again while the question was open is refused rather than overwritten. A scope the recording cannot be performed under is refused before anything is written: the terminal is not offered it and is told why, and the unattended path asks the draft what it supports instead of defaulting to the application's in-place scope. THE CAPTURE POLICY IS PRESERVED DELIBERATELY AND IS NOT PART OF THE SHARING: Studio states coordinate mode to the recorder and the terminal and the widget state nothing, which leaves the hooks doing what the environment says — the behaviour every caller before Studio had — so a saved routine from those paths still states no mode, truthfully. The owner-strip rule is the same kind of difference and is now a stated field rather than two copies of a pipeline: everywhere for a window, which is glanced at, and off the tail of a button-ended recording for everything else, which is what `TestAGestureEndedRecordingIsNotStripped` was written to record. `saveTaught` and `chooseScope` are RETAINED for `LearnVoice`, which is out of this slice: a helper whose deletion reaches another caller is kept rather than taken along for free. 13 of 13 mutations killed, held through the production entrances rather than through `Keep` alone — a test of the rule that did not enter where a person does would have passed for the whole time the rule was missing."
supersedes: []
affects:
  - execution
source_paths:
  - internal/orchestrator/keep.go
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

**A replacement targets the route consented to.** The identity is re-derived and re-checked inside
`Keep`, at the instant of writing, because between the question and the answer somebody can delete
that routine or save that name from a terminal. A surface trusting its earlier look would
overwrite at exactly that moment. The test for it interleaves for real: the reader removes the
routine as the `yes` is read.

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
- `plugins/studio` — its own save, replace and collision tests, unchanged, now running through the
  shared `Keep`.

See [[Learned-Plays]], [[Plays]], [[ADR-249-ending-a-recording-is-not-part-of-the-recording]] and
[[ADR-265-say-what-you-want-and-studio-offers-the-routine]].
